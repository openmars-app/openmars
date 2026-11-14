// Adapted from ../web-assembly/crates/asset-pipeline/src/transform/environment.rs.
//! Explicit linear-BT709 EXR interpretation and deterministic CPU cubemap recipes.
//! The recipe is proposed numeric policy; encoding does not confer content/device approval.
use crate::TransformLimits;
use crate::error::PipelineError;
use exr::prelude::*;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::result::Result;
use std::{collections::BTreeMap, f32::consts::PI, io::Cursor};

#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct SourceBinding {
    pub sha256: String,
    pub absent_chromaticities_are_linear_bt709: bool,
}
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum Recipe {
    OrdinaryLinear,
    NormalizedIrradiance,
    Ggx,
}
/// GPU-sampleable linear HDR storage. Lighting convolutions retain binary16;
/// the detailed display sky uses a shared exponent without an alpha channel.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum CubeEncoding {
    Rgba16Float,
    Rgb9e5Ufloat,
}
impl CubeEncoding {
    fn max_face_size(self) -> u32 {
        match self {
            Self::Rgba16Float => 1024,
            Self::Rgb9e5Ufloat => 2048,
        }
    }
    fn bytes(self) -> u64 {
        match self {
            Self::Rgba16Float => 8,
            Self::Rgb9e5Ufloat => 4,
        }
    }
    fn format(self) -> ktx2::Format {
        match self {
            Self::Rgba16Float => ktx2::Format::R16G16B16A16_SFLOAT,
            Self::Rgb9e5Ufloat => ktx2::Format::E5B9G9R9_UFLOAT_PACK32,
        }
    }
}
#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct CubeRequest {
    pub encoding: CubeEncoding,
    pub id: String,
    pub face_size: u32,
    pub levels: u32,
    pub recipe: Recipe,
    /// Explicit seed-zero Hammersley count; ordinary background uses fixed16 stratified taps.
    pub samples: u32,
    pub diffuse_grid: Option<GridSpec>,
}
#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct GridSpec {
    pub width: u32,
    pub height: u32,
}
impl GridSpec {
    fn cells(self) -> Result<u64, PipelineError> {
        if self.width == 0 || self.height == 0 || self.width > 256 || self.height > 128 {
            return Err(invalid("explicit diffuse grid outside32768-cell bound"));
        }
        Ok(u64::from(self.width) * u64::from(self.height))
    }
}
#[derive(Clone, Debug)]
struct Cell {
    radiance: [f32; 3],
    direction: [f64; 3],
    omega: f64,
}
#[derive(Clone, Debug)]
pub struct DiffuseGrid {
    cells: Vec<Cell>,
}
impl DiffuseGrid {
    /// Every source pixel contributes with its exact latitude-band solid angle.
    /// No point-sampling, resampling, per-output rotated sequence or sample matrix.
    pub fn from_image(
        source: &LinearImage,
        grid: GridSpec,
        limits: &TransformLimits,
    ) -> Result<Self, PipelineError> {
        limits.validate()?;
        source.validate()?;
        let cells = grid.cells()?;
        if source.width > limits.max_dimension
            || source.height > limits.max_dimension
            || !source.width.is_multiple_of(grid.width)
            || !source.height.is_multiple_of(grid.height)
        {
            return Err(invalid(
                "unsupported diffuse recipe: source dimensions must divide explicit grid",
            ));
        }
        let reservation = cells
            .checked_mul(std::mem::size_of::<Cell>() as u64)
            .and_then(|n| n.checked_add((source.pixels.len() as u64).checked_mul(12)?))
            .and_then(|n| n.checked_add(limits.max_output_bytes.checked_mul(3)?))
            .and_then(|n| n.checked_add(16 * 1024 * 1024))
            .ok_or_else(|| invalid("diffuse reservation overflow"))?;
        if reservation > limits.job_bytes() {
            return Err(invalid("diffuse source/grid/output reservation cannot fit"));
        }
        let mut result = Vec::with_capacity(cells as usize);
        let dx = source.width / grid.width;
        let dy = source.height / grid.height;
        let pi = std::f64::consts::PI;
        for y in 0..grid.height {
            let theta = pi * (f64::from(y) + 0.5) / f64::from(grid.height);
            let omega = (2. * pi / f64::from(grid.width))
                * ((pi * f64::from(y) / f64::from(grid.height)).cos()
                    - (pi * f64::from(y + 1) / f64::from(grid.height)).cos());
            for x in 0..grid.width {
                let mut sum = [0f64; 3];
                let mut weights = 0.;
                for sy in y * dy..(y + 1) * dy {
                    let weight = (2. * pi / f64::from(source.width))
                        * ((pi * f64::from(sy) / f64::from(source.height)).cos()
                            - (pi * f64::from(sy + 1) / f64::from(source.height)).cos());
                    for sx in x * dx..(x + 1) * dx {
                        let p = source.pixels[sy as usize * source.width as usize + sx as usize];
                        sum[0] += f64::from(p[0]) * weight;
                        sum[1] += f64::from(p[1]) * weight;
                        sum[2] += f64::from(p[2]) * weight;
                        weights += weight;
                    }
                }
                if !weights.is_finite() || weights <= 0. {
                    return Err(invalid("invalid source-cell weight"));
                }
                let phi = 2. * pi * (f64::from(x) + 0.5) / f64::from(grid.width) - pi;
                result.push(Cell {
                    radiance: checked(sum.map(|v| (v / weights) as f32))?,
                    direction: [
                        theta.sin() * phi.cos(),
                        theta.cos(),
                        theta.sin() * phi.sin(),
                    ],
                    omega,
                });
            }
        }
        Ok(Self { cells: result })
    }
    pub fn irradiance(&self, n: [f64; 3]) -> Result<[f32; 3], PipelineError> {
        if n.iter().any(|v| !v.is_finite())
            || (n[0] * n[0] + n[1] * n[1] + n[2] * n[2] - 1.).abs() > 1e-8
        {
            return Err(invalid("unit finite diffuse direction required"));
        }
        let mut sum = [0f64; 3];
        let mut weights = 0.;
        for cell in &self.cells {
            let w = cell.omega
                * (n[0] * cell.direction[0] + n[1] * cell.direction[1] + n[2] * cell.direction[2])
                    .max(0.);
            weights += w;
            sum[0] += f64::from(cell.radiance[0]) * w;
            sum[1] += f64::from(cell.radiance[1]) * w;
            sum[2] += f64::from(cell.radiance[2]) * w;
        }
        if !weights.is_finite() || weights <= 0. {
            return Err(invalid("invalid diffuse normalization"));
        }
        checked(sum.map(|s| (s / weights) as f32))
    }
}
pub fn build_environment_cube(
    source: &LinearImage,
    request: &CubeRequest,
    limits: &TransformLimits,
) -> Result<Cube, PipelineError> {
    preflight_cube(request, limits)?;
    if request.recipe != Recipe::NormalizedIrradiance {
        return build_cube(&|d| source.sample(d), request, limits);
    }
    let grid = DiffuseGrid::from_image(
        source,
        request
            .diffuse_grid
            .ok_or_else(|| invalid("explicit diffuse grid required"))?,
        limits,
    )?;
    let n = request.face_size;
    let mut pixels = Vec::with_capacity(n as usize * n as usize * 6);
    for face in 0..6 {
        for y in 0..n {
            for x in 0..n {
                pixels.push(grid.irradiance(face_direction64(
                    face,
                    2. * (f64::from(x) + 0.5) / f64::from(n) - 1.,
                    2. * (f64::from(y) + 0.5) / f64::from(n) - 1.,
                ))?);
            }
        }
    }
    Ok(Cube {
        encoding: request.encoding,
        size: n,
        levels: vec![pixels],
    })
}
pub fn face_direction64(face: usize, s: f64, t: f64) -> [f64; 3] {
    let q = match face {
        0 => [1., -t, -s],
        1 => [-1., -t, s],
        2 => [s, 1., t],
        3 => [s, -1., -t],
        4 => [s, -t, 1.],
        5 => [-s, -t, -1.],
        _ => return [f64::NAN; 3],
    };
    let r = (q[0] * q[0] + q[1] * q[1] + q[2] * q[2]).sqrt();
    [q[0] / r, q[1] / r, -q[2] / r]
}

#[derive(Clone, Debug)]
pub struct LinearImage {
    pub width: u32,
    pub height: u32,
    pub pixels: Vec<[f32; 3]>,
}
#[derive(Clone, Debug)]
pub struct Cube {
    pub encoding: CubeEncoding,
    pub size: u32,
    /// Mip-major; within each mip, +X,-X,+Y,-Y,+Z,-Z, top-origin rows.
    pub levels: Vec<Vec<[f32; 3]>>,
}
#[derive(Clone, Debug)]
pub struct ExrHeader {
    pub width: u32,
    pub height: u32,
    pub header_bytes: usize,
    pub chromaticities: Option<[f32; 8]>,
}
fn invalid(m: impl Into<String>) -> PipelineError {
    PipelineError::invalid("HDR", m)
}
fn u32le(b: &[u8]) -> u32 {
    u32::from_le_bytes(b[..4].try_into().unwrap())
}
fn string<'a>(b: &'a [u8], p: &mut usize) -> Result<&'a str, PipelineError> {
    let n = b
        .get(*p..)
        .and_then(|s| s.iter().take(256).position(|v| *v == 0))
        .ok_or_else(|| invalid("bounded EXR string missing"))?;
    let s = std::str::from_utf8(&b[*p..*p + n]).map_err(|_| invalid("EXR string invalid"))?;
    *p += n + 1;
    Ok(s)
}
/// Closed scanline RGB HALF/FLOAT ZIP/NONE profile, preflight before native EXR parsing.
pub fn inspect_exr(bytes: &[u8], limits: &TransformLimits) -> Result<ExrHeader, PipelineError> {
    limits.validate()?;
    if bytes.len() < 9
        || bytes.len() as u64 > limits.max_source_bytes
        || u32le(bytes) != 20000630
        || u32le(&bytes[4..]) != 2
    {
        return Err(invalid("unsupported EXR version/flags/size"));
    }
    let mut p = 8;
    let mut attrs = BTreeMap::new();
    while *bytes
        .get(p)
        .ok_or_else(|| invalid("truncated EXR header"))?
        != 0
    {
        if p > 1024 * 1024 || attrs.len() >= 64 {
            return Err(invalid("EXR header bound exceeded"));
        }
        let name = string(bytes, &mut p)?;
        let kind = string(bytes, &mut p)?;
        if ![
            "channels",
            "compression",
            "dataWindow",
            "displayWindow",
            "lineOrder",
            "pixelAspectRatio",
            "screenWindowCenter",
            "screenWindowWidth",
            "chromaticities",
            "chunkCount",
            "type",
        ]
        .contains(&name)
        {
            return Err(invalid(format!(
                "attribute outside qualified EXR profile: {name}"
            )));
        }

        let size = bytes
            .get(p..p + 4)
            .map(u32le)
            .ok_or_else(|| invalid("truncated EXR size"))? as usize;
        p += 4;
        if size > 65536 {
            return Err(invalid("EXR attribute bound exceeded"));
        }
        let value = bytes
            .get(p..p + size)
            .ok_or_else(|| invalid("truncated EXR attribute"))?;
        p += size;
        if attrs.insert(name, (kind, value)).is_some() {
            return Err(invalid("duplicate EXR attribute"));
        }
    }
    p += 1;
    let (kind, window) = attrs
        .get("dataWindow")
        .ok_or_else(|| invalid("missing dataWindow"))?;
    if *kind != "box2i" || window.len() != 16 || u32le(window) != 0 || u32le(&window[4..]) != 0 {
        return Err(invalid("origin-zero dataWindow required"));
    }
    let width = u32le(&window[8..])
        .checked_add(1)
        .ok_or_else(|| invalid("width overflow"))?;
    let height = u32le(&window[12..])
        .checked_add(1)
        .ok_or_else(|| invalid("height overflow"))?;
    if width == 0 || height == 0 || width > limits.max_dimension || height > limits.max_dimension {
        return Err(invalid("dimensions exceed bounds"));
    }
    let (kind, channels) = attrs
        .get("channels")
        .ok_or_else(|| invalid("missing channels"))?;
    if *kind != "chlist" {
        return Err(invalid("invalid channels"));
    }
    let mut c = 0;
    let mut names = Vec::new();
    while channels.get(c) != Some(&0) {
        let name = string(channels, &mut c)?;
        let data = channels
            .get(c..c + 16)
            .ok_or_else(|| invalid("truncated channel"))?;
        c += 16;
        if !matches!(u32le(data), 1 | 2) || u32le(&data[8..]) != 1 || u32le(&data[12..]) != 1 {
            return Err(invalid("only unit-sampled HALF/FLOAT channels"));
        }
        names.push(name);
        if names.len() > 3 {
            return Err(invalid("exactly RGB required"));
        }
    }
    if c + 1 != channels.len() || names != ["B", "G", "R"] {
        return Err(invalid("exactly sorted B,G,R required"));
    }
    let compression = attrs
        .get("compression")
        .ok_or_else(|| invalid("missing compression"))?;
    if compression.0 != "compression"
        || compression.1.len() != 1
        || !matches!(compression.1[0], 0 | 3)
    {
        return Err(invalid("only NONE/ZIP compression qualified"));
    }
    let chunks = if compression.1[0] == 3 {
        height.div_ceil(16)
    } else {
        height
    };
    if let Some((kind, count)) = attrs.get("chunkCount")
        && (*kind != "int" || count.len() != 4 || u32le(count) != chunks)
    {
        return Err(invalid(
            "chunkCount disagrees with bounded scanline profile",
        ));
    }
    if let Some((kind, value)) = attrs.get("type")
        && (*kind != "string" || *value != b"scanlineimage")
    {
        return Err(invalid("only scanlineimage type qualified"));
    }
    let table_end = p
        .checked_add(chunks as usize * 8)
        .ok_or_else(|| invalid("offset overflow"))?;
    if table_end > bytes.len() {
        return Err(invalid("truncated chunk table"));
    }
    for offset in bytes[p..table_end].as_chunks::<8>().0 {
        let offset = u64::from_le_bytes(*offset);
        if offset < table_end as u64 || offset > bytes.len().saturating_sub(8) as u64 {
            return Err(invalid("invalid EXR chunk offset"));
        }
        let offset = offset as usize;
        let n = u32le(&bytes[offset + 4..]) as usize;
        if n > bytes.len() - offset - 8 {
            return Err(invalid("truncated EXR chunk"));
        }
    }
    if attrs.contains_key("colorInteropID") {
        return Err(invalid(
            "colorInteropID requires a separately qualified interpretation",
        ));
    }
    let chromaticities = match attrs.get("chromaticities") {
        None => None,
        Some(("chromaticities", b)) if b.len() == 32 => {
            let mut c = [0.; 8];
            for (i, v) in b.as_chunks::<4>().0.iter().enumerate() {
                c[i] = f32::from_le_bytes(*v);
            }
            Some(c)
        }
        _ => return Err(invalid("invalid chromaticities")),
    };
    // 40 bytes/pixel covers 12-byte destination + native channel/decoder/copy reserves;
    // two encoded copies, three bounded output buffers, and header/chunk overhead.
    // Single-threaded EXR decompression keeps at most one ZIP block live.
    let reservation = u64::from(width)
        .checked_mul(u64::from(height))
        .and_then(|n| n.checked_mul(40))
        .and_then(|n| n.checked_add((bytes.len() as u64).checked_mul(2)?))
        .and_then(|n| n.checked_add(limits.max_output_bytes.checked_mul(3)?))
        .and_then(|n| n.checked_add(16 * 1024 * 1024))
        .ok_or_else(|| invalid("working arithmetic overflow"))?;
    if reservation > limits.job_bytes() {
        return Err(invalid("complete EXR working reservation cannot fit"));
    }
    Ok(ExrHeader {
        width,
        height,
        header_bytes: p,
        chromaticities,
    })
}
pub fn decode_exr(
    bytes: &[u8],
    binding: &SourceBinding,
    limits: &TransformLimits,
) -> Result<LinearImage, PipelineError> {
    let header = inspect_exr(bytes, limits)?;
    if format!("{:x}", Sha256::digest(bytes)) != binding.sha256 {
        return Err(invalid("source hash does not match explicit color binding"));
    }
    let bt709 = [0.64, 0.33, 0.3, 0.6, 0.15, 0.06, 0.3127, 0.329];
    match header.chromaticities {
        Some(c) if c.iter().zip(bt709).all(|(a, b)| (a - b).abs() < 1e-6) => {}
        None if binding.absent_chromaticities_are_linear_bt709 => {}
        _ => return Err(invalid("source color interpretation is unbound/non-BT709")),
    }
    let image = read()
        .no_deep_data()
        .largest_resolution_level()
        .rgba_channels(
            |size, _| LinearImage {
                width: size.width() as u32,
                height: size.height() as u32,
                pixels: vec![[0.; 3]; size.area()],
            },
            |image: &mut LinearImage, pos, (r, g, b, _): (f32, f32, f32, f32)| {
                image.pixels[pos.y() * image.width as usize + pos.x()] = [r, g, b];
            },
        )
        .first_valid_layer()
        .all_attributes()
        .non_parallel()
        .pedantic()
        .from_buffered(Cursor::new(bytes))
        .map_err(|e| invalid(format!("EXR decode: {e}")))?;
    let image = image.layer_data.channel_data.pixels;
    image.validate()?;
    Ok(image)
}
impl LinearImage {
    pub fn validate(&self) -> Result<(), PipelineError> {
        if self.width == 0
            || self.height == 0
            || u64::from(self.width) * u64::from(self.height) != self.pixels.len() as u64
            || self
                .pixels
                .iter()
                .flatten()
                .any(|v| !v.is_finite() || v.abs() > 65504.)
        {
            return Err(invalid("invalid/nonfinite/unrepresentable linear pixels"));
        }
        Ok(())
    }
    pub fn sample(&self, d: [f32; 3]) -> [f32; 3] {
        let d = normalize(d);
        let u = if d[0].abs() + d[2].abs() < 1e-7 {
            0.5
        } else {
            (d[2].atan2(d[0]) / (2. * PI) + 0.5).rem_euclid(1.)
        };
        let v = d[1].clamp(-1., 1.).acos() / PI;
        let x = u * self.width as f32 - 0.5;
        let y = v * self.height as f32 - 0.5;
        let ix = x.floor() as i64;
        let iy = y.floor() as i64;
        let fx = x - x.floor();
        let fy = y - y.floor();
        let mut out = [0.; 3];
        for (j, wy) in [(0, 1. - fy), (1, fy)] {
            for (i, wx) in [(0, 1. - fx), (1, fx)] {
                let x = (ix + i).rem_euclid(i64::from(self.width)) as usize;
                let y = (iy + j).clamp(0, i64::from(self.height) - 1) as usize;
                for (c, o) in out.iter_mut().enumerate() {
                    *o += self.pixels[y * self.width as usize + x][c] * wx * wy;
                }
            }
        }
        out
    }
}
pub fn normalize(v: [f32; 3]) -> [f32; 3] {
    let n = dot(v, v).sqrt();
    v.map(|x| x / n)
}
fn dot(a: [f32; 3], b: [f32; 3]) -> f32 {
    a[0] * b[0] + a[1] * b[1] + a[2] * b[2]
}
fn cross(a: [f32; 3], b: [f32; 3]) -> [f32; 3] {
    [
        a[1] * b[2] - a[2] * b[1],
        a[2] * b[0] - a[0] * b[2],
        a[0] * b[1] - a[1] * b[0],
    ]
}
/// Source world direction at a continuous face coordinate (including matched edges).
pub fn face_direction(face: usize, s: f32, t: f32) -> [f32; 3] {
    let q = match face {
        0 => [1., -t, -s],
        1 => [-1., -t, s],
        2 => [s, 1., t],
        3 => [s, -1., -t],
        4 => [s, -t, 1.],
        5 => [-s, -t, -1.],
        _ => return [f32::NAN; 3],
    };
    let q = normalize(q);
    [q[0], q[1], -q[2]]
}
fn frame(n: [f32; 3], local: [f32; 3]) -> [f32; 3] {
    let helper = if n[2].abs() < 0.999 {
        [0., 0., 1.]
    } else {
        [1., 0., 0.]
    };
    let t = normalize(cross(helper, n));
    let b = cross(n, t);
    std::array::from_fn(|i| t[i] * local[0] + b[i] * local[1] + n[i] * local[2])
}
/// Fixed Hammersley sequence; seed zero, no scrambling.
pub const MAX_SAMPLES: u32 = 16384;
pub fn hammersley(i: u32, count: u32) -> Result<[f32; 2], PipelineError> {
    if count == 0 || count > MAX_SAMPLES || i >= count {
        return Err(invalid("sample sequence index/count outside hard bounds"));
    }
    Ok([
        i as f32 / count as f32,
        i.reverse_bits() as f32 * 2.328_306_4e-10,
    ])
}
pub fn convolve<F: Fn([f32; 3]) -> [f32; 3]>(
    sample: &F,
    n: [f32; 3],
    recipe: Recipe,
    roughness: f32,
    samples: u32,
) -> Result<[f32; 3], PipelineError> {
    if recipe == Recipe::NormalizedIrradiance {
        return Err(invalid(
            "diffuse requires the global source-cell grid, not a directional sampler",
        ));
    }
    if samples == 0
        || samples > MAX_SAMPLES
        || n.iter().any(|x| !x.is_finite())
        || (dot(n, n) - 1.).abs() > 1e-4
        || !roughness.is_finite()
        || !(0.0..=1.0).contains(&roughness)
    {
        return Err(invalid("invalid convolution input"));
    }
    if recipe == Recipe::OrdinaryLinear || recipe == Recipe::Ggx && roughness == 0. {
        return checked(sample(n));
    }
    let mut sum = [0.; 3];
    let mut correction = [0.; 3];
    let mut weights = 0.;
    let mut weight_correction = 0.;
    for i in 0..samples {
        let xi = hammersley(i, samples)?;
        let alpha = roughness * roughness;
        let cos = ((1. - xi[1]) / (1. + (alpha * alpha - 1.) * xi[1])).sqrt();
        let sin = (1. - cos * cos).max(0.).sqrt();
        let p = 2. * PI * xi[0];
        let h = frame(n, [sin * p.cos(), sin * p.sin(), cos]);
        let nh = dot(n, h);
        let d = std::array::from_fn(|j| 2. * nh * h[j] - n[j]);
        let w = dot(n, d).max(0.);
        if w > 0. {
            let l = checked(sample(d))?;
            for c in 0..3 {
                let y = l[c] * w - correction[c];
                let total = sum[c] + y;
                correction[c] = (total - sum[c]) - y;
                sum[c] = total;
            }
            let y = w - weight_correction;
            let total = weights + y;
            weight_correction = (total - weights) - y;
            weights = total;
        }
    }
    if !weights.is_finite() || weights <= 0. {
        return Err(invalid("empty/nonfinite convolution weights"));
    }
    checked(sum.map(|s| s / weights))
}
fn checked(v: [f32; 3]) -> Result<[f32; 3], PipelineError> {
    if v.iter().any(|x| !x.is_finite() || x.abs() > 65504.) {
        Err(invalid("nonfinite/binary16 overflow; no radiance clamp"))
    } else {
        Ok(v)
    }
}
/// Validate the whole output/evaluation reservation before source decode.
pub fn preflight_cube(
    request: &CubeRequest,
    limits: &TransformLimits,
) -> Result<u64, PipelineError> {
    limits.validate()?;
    let n = request.face_size;
    let levels = request.levels;
    if request.id.is_empty()
        || request.id.len() > 160
        || n == 0
        || !n.is_power_of_two()
        || n > limits.max_dimension.min(request.encoding.max_face_size())
        || levels == 0
        || levels > n.ilog2() + 1
        || request.recipe == Recipe::NormalizedIrradiance && levels != 1
        || request.recipe != Recipe::NormalizedIrradiance
            && (request.samples == 0
                || request.samples > MAX_SAMPLES
                || request.diffuse_grid.is_some())
        || request.recipe == Recipe::NormalizedIrradiance
            && (request.samples != 0 || request.diffuse_grid.is_none())
        || request.encoding == CubeEncoding::Rgb9e5Ufloat
            && request.recipe != Recipe::OrdinaryLinear
        || request.recipe == Recipe::OrdinaryLinear && request.samples != 16
    {
        return Err(invalid("invalid explicit cube recipe/sampling bounds"));
    }
    let count = (0..levels)
        .try_fold(0u64, |a, m| a.checked_add(u64::from(n >> m).pow(2) * 6))
        .ok_or_else(|| invalid("cube arithmetic overflow"))?;
    let calls = count
        .checked_mul(if let Some(grid) = request.diffuse_grid {
            grid.cells()?
        } else {
            u64::from(request.samples)
        })
        .ok_or_else(|| invalid("evaluation count overflow"))?;
    if calls > 4_294_967_296 {
        return Err(invalid("convolution evaluation safety ceiling exceeded"));
    }
    let output = count
        .checked_mul(request.encoding.bytes())
        .and_then(|n| n.checked_add(4096))
        .ok_or_else(|| invalid("cube output overflow"))?;
    if output > limits.max_output_bytes
        || count.checked_mul(32).is_none_or(|n| n > limits.job_bytes())
    {
        return Err(invalid("cube reservation/output exceeds bounds"));
    }
    Ok(output)
}
pub fn build_cube<F: Fn([f32; 3]) -> [f32; 3]>(
    sample: &F,
    request: &CubeRequest,
    limits: &TransformLimits,
) -> Result<Cube, PipelineError> {
    preflight_cube(request, limits)?;
    if request.recipe == Recipe::NormalizedIrradiance {
        return Err(invalid(
            "diffuse requires build_environment_cube with every source pixel",
        ));
    }
    let n = request.face_size;
    let levels = request.levels;
    if n == 0
        || !n.is_power_of_two()
        || n > limits.max_dimension.min(request.encoding.max_face_size())
        || levels == 0
        || levels > n.ilog2() + 1
        || request.recipe == Recipe::NormalizedIrradiance && levels != 1
    {
        return Err(invalid("invalid cube dimensions/mips/recipe"));
    }
    let count = (0..levels)
        .try_fold(0u64, |a, m| {
            a.checked_add(u64::from((n >> m).max(1)).pow(2) * 6)
        })
        .ok_or_else(|| invalid("cube arithmetic overflow"))?;
    if count
        .checked_mul(request.encoding.bytes())
        .is_none_or(|n| n + 4096 > limits.max_output_bytes)
        || count.checked_mul(32).is_none_or(|n| n > limits.job_bytes())
    {
        return Err(invalid("cube reservation/output exceeds bounds"));
    }
    let mut result = Cube {
        encoding: request.encoding,
        size: n,
        levels: Vec::new(),
    };
    for m in 0..levels {
        let size = n >> m;
        let mut pixels = Vec::with_capacity(size as usize * size as usize * 6);
        for face in 0..6 {
            for y in 0..size {
                for x in 0..size {
                    let value = if request.recipe == Recipe::OrdinaryLinear && m > 0 {
                        let previous = &result.levels[m as usize - 1];
                        let psize = size * 2;
                        let mut out = [0.; 3];
                        for j in 0..2 {
                            for i in 0..2 {
                                let p = previous[(face * psize as usize + (y * 2 + j) as usize)
                                    * psize as usize
                                    + (x * 2 + i) as usize];
                                for c in 0..3 {
                                    out[c] += p[c] * 0.25;
                                }
                            }
                        }
                        out
                    } else if request.recipe == Recipe::OrdinaryLinear {
                        let mut out = [0.; 3];
                        for j in 0..4 {
                            for i in 0..4 {
                                let s = 2. * (x as f32 + (i as f32 + 0.5) / 4.) / size as f32 - 1.;
                                let t = 2. * (y as f32 + (j as f32 + 0.5) / 4.) / size as f32 - 1.;
                                let p = checked(sample(face_direction(face, s, t)))?;
                                for c in 0..3 {
                                    out[c] += p[c] / 16.;
                                }
                            }
                        }
                        out
                    } else {
                        let s = 2. * (x as f32 + 0.5) / size as f32 - 1.;
                        let t = 2. * (y as f32 + 0.5) / size as f32 - 1.;
                        convolve(
                            sample,
                            face_direction(face, s, t),
                            request.recipe,
                            if levels > 1 {
                                m as f32 / (levels - 1) as f32
                            } else {
                                0.
                            },
                            request.samples,
                        )?
                    };
                    pixels.push(checked(value)?);
                }
            }
        }
        result.levels.push(pixels);
    }
    Ok(result)
}
pub fn half_bits(v: f32) -> Result<u16, PipelineError> {
    if !v.is_finite() || v.abs() > 65504. {
        return Err(invalid("binary16 overflow/nonfinite"));
    }
    Ok(half::f16::from_f32(v).to_bits())
}
/// Complete KTX2 with generated Khronos basic DFD and 8-byte aligned level data.
pub fn encode_ktx2(cube: &Cube, limits: &TransformLimits) -> Result<Vec<u8>, PipelineError> {
    limits.validate()?;
    let levels = cube.levels.len();
    if levels == 0
        || levels > 32
        || cube.size == 0
        || cube.size > limits.max_dimension.min(cube.encoding.max_face_size())
        || !cube.size.is_power_of_two()
        || levels > cube.size.ilog2() as usize + 1
    {
        return Err(invalid("invalid cube mip chain"));
    }
    let (basic, _) = ktx2::dfd::Basic::from_format(cube.encoding.format())
        .map_err(|e| invalid(e.to_string()))?;
    let block = ktx2::dfd::Block::Basic(basic).to_vec();
    let dfdlen = block.len() + 4;
    let dfdoff = 80 + 24 * levels;
    let mut offset = (dfdoff + dfdlen + 7) & !7;
    let mut indices: Vec<_> = (0..levels)
        .map(|_| ktx2::LevelIndex {
            byte_offset: 0,
            byte_length: 0,
            uncompressed_byte_length: 0,
        })
        .collect();
    let compressed = cube.encoding == CubeEncoding::Rgb9e5Ufloat;
    let mut encoded = vec![Vec::new(); levels];
    // KTX2 physical mip data is smallest-first; its level index is base-first.
    // https://registry.khronos.org/KTX/specs/2.0/ktxspec.v2.html#_mip_level_array
    for (m, pixels) in cube.levels.iter().enumerate().rev() {
        let n = cube.size >> m;
        if pixels.len() != n as usize * n as usize * 6 {
            return Err(invalid("invalid cube level length"));
        }
        let len = pixels
            .len()
            .checked_mul(cube.encoding.bytes() as usize)
            .ok_or_else(|| invalid("container overflow"))?;
        if len as u64 > limits.max_output_bytes {
            return Err(invalid("uncompressed mip exceeds byte bound"));
        }
        let mut raw = Vec::with_capacity(len);
        for rgb in pixels {
            match cube.encoding {
                CubeEncoding::Rgba16Float => {
                    for v in [rgb[0], rgb[1], rgb[2], 1.] {
                        raw.extend_from_slice(&half_bits(v)?.to_le_bytes());
                    }
                }
                CubeEncoding::Rgb9e5Ufloat => {
                    raw.extend_from_slice(&pack_rgb9e5(*rgb)?.to_le_bytes())
                }
            }
        }
        let bytes = if compressed {
            use std::io::Write;
            let mut encoder =
                flate2::write::ZlibEncoder::new(Vec::new(), flate2::Compression::default());
            encoder
                .write_all(&raw)
                .map_err(|e| invalid(e.to_string()))?;
            encoder.finish().map_err(|e| invalid(e.to_string()))?
        } else {
            raw
        };
        offset = (offset + 7) & !7;
        indices[m] = ktx2::LevelIndex {
            byte_offset: offset as u64,
            byte_length: bytes.len() as u64,
            uncompressed_byte_length: len as u64,
        };
        offset = offset
            .checked_add(bytes.len())
            .ok_or_else(|| invalid("container overflow"))?;
        if offset as u64 > limits.max_output_bytes {
            return Err(invalid("compressed container exceeds byte bound"));
        }
        encoded[m] = bytes;
    }
    if offset as u64 > limits.max_output_bytes {
        return Err(invalid("container output bound exceeded"));
    }
    let header = ktx2::Header {
        format: Some(cube.encoding.format()),
        type_size: match cube.encoding {
            CubeEncoding::Rgba16Float => 2,
            CubeEncoding::Rgb9e5Ufloat => 4,
        },
        pixel_width: cube.size,
        pixel_height: cube.size,
        pixel_depth: 0,
        layer_count: 0,
        face_count: 6,
        level_count: levels as u32,
        supercompression_scheme: compressed.then_some(ktx2::SupercompressionScheme::ZLIB),
        index: ktx2::Index {
            dfd_byte_offset: dfdoff as u32,
            dfd_byte_length: dfdlen as u32,
            kvd_byte_offset: 0,
            kvd_byte_length: 0,
            sgd_byte_offset: 0,
            sgd_byte_length: 0,
        },
    };
    let mut bytes = vec![0; offset];
    bytes[..80].copy_from_slice(&header.as_bytes());
    for (m, index) in indices.iter().enumerate() {
        bytes[80 + m * 24..104 + m * 24].copy_from_slice(&index.as_bytes());
    }
    bytes[dfdoff..dfdoff + 4].copy_from_slice(&(dfdlen as u32).to_le_bytes());
    bytes[dfdoff + 4..dfdoff + dfdlen].copy_from_slice(&block);
    for (level, index) in encoded.iter().zip(&indices) {
        let start = index.byte_offset as usize;
        bytes[start..start + level.len()].copy_from_slice(level);
    }
    ktx2::Reader::new(&bytes).map_err(|e| invalid(format!("KTX2 verification: {e:?}")))?;
    Ok(bytes)
}

/// Khronos EXT_texture_shared_exponent, conversion in section3.8.1.
/// Reject unsupported radiance rather than silently clipping the authored HDR.
fn pack_rgb9e5(rgb: [f32; 3]) -> Result<u32, PipelineError> {
    if rgb
        .into_iter()
        .any(|v| !v.is_finite() || !(0. ..=65408.).contains(&v))
    {
        return Err(invalid("radiance outside RGB9E5 range"));
    }
    let max = rgb.into_iter().fold(0f32, f32::max);
    let mut exponent = (((max.to_bits() >> 23) & 255) as i32 - 127).max(-16) + 16;
    let mut denominator = 2f64.powi(exponent - 24);
    if (f64::from(max) / denominator + 0.5).floor() >= 512. {
        exponent += 1;
        denominator *= 2.;
    }
    let mantissa = rgb.map(|v| (f64::from(v) / denominator + 0.5).floor() as u32);
    Ok(mantissa[0] | (mantissa[1] << 9) | (mantissa[2] << 18) | ((exponent as u32) << 27))
}
#[cfg(test)]
mod packed_tests {
    use super::*;
    fn unpack(bits: u32) -> [f64; 3] {
        let scale = 2f64.powi((bits >> 27) as i32 - 24);
        [bits & 511, (bits >> 9) & 511, (bits >> 18) & 511].map(|m| f64::from(m) * scale)
    }
    #[test]
    fn packed_hdr_preserves_channel_order_range_and_rounding_across_exponents() {
        for (input, word) in [
            ([0.; 3], 0),
            ([1.; 3], 0x84020100),
            ([65408.; 3], 0xffffffff),
            ([2f32.powi(-24), 0., 0.], 1),
        ] {
            let actual = pack_rgb9e5(input).unwrap();
            assert_eq!(actual, word);
            assert_eq!(unpack(actual), input.map(f64::from));
        }
        for exponent in -24..15 {
            let v = 2f32.powi(exponent);
            for value in [
                v,
                f32::from_bits(v.to_bits() - 1),
                f32::from_bits(v.to_bits() + 1),
            ] {
                for ratio in [0.01, 0.1, 0.49, 0.5, 0.99, 1.] {
                    let source = [value, value * ratio, value * 0.17];
                    let word = pack_rgb9e5(source).unwrap();
                    let decoded = unpack(word);
                    let error = 2f64.powi((word >> 27) as i32 - 24) * 0.5;
                    for (a, b) in source.into_iter().zip(decoded) {
                        assert!((f64::from(a) - b).abs() <= error + 1e-10);
                    }
                }
            }
        }
        for bad in [
            [-1., 0., 0.],
            [f32::NAN, 0., 0.],
            [f32::INFINITY, 0., 0.],
            [65504., 0., 0.],
        ] {
            assert!(pack_rgb9e5(bad).is_err());
        }
    }
    #[test]
    fn detailed_cube_budget_and_container_are_explicit_and_bounded() {
        let limits = TransformLimits::default();
        let mut request = CubeRequest {
            id: "stars".into(),
            encoding: CubeEncoding::Rgb9e5Ufloat,
            face_size: 1024,
            levels: 11,
            recipe: Recipe::OrdinaryLinear,
            samples: 16,
            diffuse_grid: None,
        };
        let reservation = preflight_cube(&request, &limits).unwrap();
        assert!(reservation > 32 * 1024 * 1024 && reservation < 33 * 1024 * 1024);
        request.face_size = 2048;
        request.levels = 12;
        let reservation = preflight_cube(&request, &limits).unwrap();
        assert!(reservation > 128 * 1024 * 1024 && reservation < 129 * 1024 * 1024);
        request.encoding = CubeEncoding::Rgba16Float;
        assert!(preflight_cube(&request, &limits).is_err());
        request.encoding = CubeEncoding::Rgb9e5Ufloat;
        request.face_size = 4096;
        request.levels = 13;
        assert!(preflight_cube(&request, &limits).is_err());
        request.face_size = 4;
        request.levels = 3;
        let cube = build_cube(&|_| [0.125, 0.25, 2.], &request, &limits).unwrap();
        let bytes = encode_ktx2(&cube, &limits).unwrap();
        let reader = ktx2::Reader::new(&bytes).unwrap();
        assert_eq!(
            reader.header().format,
            Some(ktx2::Format::E5B9G9R9_UFLOAT_PACK32)
        );
        assert_eq!(reader.header().type_size, 4);
        assert_eq!(reader.header().face_count, 6);
        assert_eq!(
            reader.header().supercompression_scheme,
            Some(ktx2::SupercompressionScheme::ZLIB)
        );
        for (m, level) in reader.levels().enumerate() {
            use std::io::Read;
            let mut raw = Vec::new();
            flate2::read::ZlibDecoder::new(level.data)
                .read_to_end(&mut raw)
                .unwrap();
            assert_eq!(raw.len(), (4usize >> m).pow(2) * 6 * 4);
            for word in raw.as_chunks::<4>().0 {
                assert_eq!(unpack(u32::from_le_bytes(*word)), [0.125, 0.25, 2.]);
            }
        }
        request.recipe = Recipe::Ggx;
        assert!(preflight_cube(&request, &limits).is_err());
    }
}
