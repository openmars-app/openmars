// Adapted from the sibling renderer; graphics-only scalar sampling.
//! Immutable, CPU-only sampling of already prepared terrain fields.
//! This is not the later board-state-to-landscape builder or a GPU renderer.
use super::{BOARD_SCALE, BoardPoint, BodyProjection};
use serde::{Deserialize, Serialize};
use std::{collections::BTreeMap, fmt, sync::Arc};

pub const PATCH_SIZE: f64 = 0.6 * BOARD_SCALE;
pub const FIELD_SAMPLES: usize = 128;
pub const FIELD_BORDER: usize = 2;
pub const FIELD_SIZE: usize = FIELD_SAMPLES + FIELD_BORDER * 2;
pub const FIELD_STEP: f64 = PATCH_SIZE / (FIELD_SAMPLES - 1) as f64;
pub const EMPTY_SHORE: f32 = 0.2;
pub const RELIEF_DEPTH: f32 = 0.036;
pub const MAX_RELIEF_PIXELS: usize = 512 * 256;
pub const MAX_TERRAIN_PATCHES: usize = 256;

/// Lake floor and dry-bank grading from landscapeFields.ts. This samples an
/// explicit shore field; it does not decide whether a gameplay tile is water.
pub fn basin_height(shore: f64, beach_width: f64, land_height: f64) -> Result<f64, TerrainError> {
    if ![shore, beach_width, land_height]
        .into_iter()
        .all(f64::is_finite)
    {
        return Err(TerrainError::NonFinite);
    }
    const WATER: f64 = -0.008;
    const BED: f64 = -0.024;
    const REACH: f64 = 0.075 + 0.04;
    if beach_width <= 0.0 || beach_width >= REACH {
        return Err(TerrainError::OutsideDomain);
    }
    let smooth = super::noise::smooth;
    let result = if shore < 0.0 {
        WATER + (BED - WATER) * smooth(0.0, 0.07, -shore)
    } else {
        let bank_height = land_height.max(WATER + 0.002);
        let bank = WATER + (bank_height - WATER) * smooth(0.0, beach_width, shore);
        bank + (land_height - bank) * smooth(beach_width, REACH, shore)
    };
    if !result.is_finite() {
        return Err(TerrainError::NonFinite);
    }
    Ok(result)
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum TerrainError {
    NonFinite,
    InvalidDimensions,
    InvalidVolcano,
    InvalidNormal,
    OutsideDomain,
    DuplicatePatch,
    InconsistentSeam,
    InvalidHeight,
    ResourceLimit,
}
impl fmt::Display for TerrainError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{self:?}")
    }
}
impl std::error::Error for TerrainError {}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum SurfaceClass {
    Ground,
    Beach,
    Water,
    Basin,
    Excluded,
    Volcano,
    Crater,
}

/// Data-only sample; normal is a body-local outward unit vector. Surface is
/// supplied by the immutable field owner, not inferred as gameplay legality.
#[derive(Clone, Copy, Debug, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields, try_from = "SampleWire")]
pub struct TerrainSample {
    pub height: f32,
    pub normal: [f32; 3],
    pub shore: f32,
    pub surface: SurfaceClass,
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct SampleWire {
    height: f32,
    normal: [f32; 3],
    shore: f32,
    surface: SurfaceClass,
}
impl TryFrom<SampleWire> for TerrainSample {
    type Error = TerrainError;
    fn try_from(v: SampleWire) -> Result<Self, Self::Error> {
        let sample = Self {
            height: v.height,
            normal: v.normal,
            shore: v.shore,
            surface: v.surface,
        };
        sample.validate()?;
        Ok(sample)
    }
}
impl TerrainSample {
    pub fn validate(self) -> Result<(), TerrainError> {
        if ![self.height, self.shore]
            .into_iter()
            .chain(self.normal)
            .all(f32::is_finite)
        {
            return Err(TerrainError::NonFinite);
        }
        let length = self
            .normal
            .into_iter()
            .map(|x| f64::from(x).powi(2))
            .sum::<f64>()
            .sqrt();
        if (length - 1.0).abs() > 2e-6 {
            return Err(TerrainError::InvalidNormal);
        }
        Ok(())
    }
}

/// A prepared equirectangular depth field. u wraps, v points north; storage
/// rows point south, matching marsReliefAtUv. Pole rows must be longitude-free.
#[derive(Clone, Debug)]
pub struct ReliefMap {
    width: usize,
    height: usize,
    depths: Arc<[f32]>,
    identity: Arc<std::sync::OnceLock<[u8; 32]>>,
}
impl ReliefMap {
    /// Decode the red sRGB channel, 3x3 [1,2,1] filter, and equalized poles
    /// exactly as createMarsRelief. No texture, canvas, or GPU initialization.
    pub fn from_rgba(width: usize, height: usize, rgba: &[u8]) -> Result<Self, TerrainError> {
        let len = width
            .checked_mul(height)
            .ok_or(TerrainError::ResourceLimit)?;
        if len > MAX_RELIEF_PIXELS {
            return Err(TerrainError::ResourceLimit);
        }
        if width < 2 || height < 2 || len.checked_mul(4) != Some(rgba.len()) {
            return Err(TerrainError::InvalidDimensions);
        }
        let red: Vec<f32> = rgba
            .as_chunks::<4>()
            .0
            .iter()
            .map(|p| {
                let v = f64::from(p[0]) / 255.0;
                (if v <= 0.04045 {
                    v / 12.92
                } else {
                    ((v + 0.055) / 1.055).powf(2.4)
                }) as f32
            })
            .collect();
        let mut depths = vec![0.0_f32; len];
        let kernel = [1.0, 2.0, 1.0];
        for y in 0..height {
            for x in 0..width {
                let mut sum = 0.0;
                for dy in -1_i64..=1 {
                    for dx in -1_i64..=1 {
                        let row = (y as i64 + dy).clamp(0, height as i64 - 1) as usize;
                        let col = (x as i64 + dx).rem_euclid(width as i64) as usize;
                        sum += f64::from(red[row * width + col])
                            * kernel[(dx + 1) as usize]
                            * kernel[(dy + 1) as usize]
                            / 16.0;
                    }
                }
                depths[y * width + x] = (0.036 * (1.0 - (sum / 0.65).min(1.0).powf(0.85))) as f32;
            }
        }
        for row in [0, height - 1] {
            let mean = depths[row * width..(row + 1) * width]
                .iter()
                .map(|v| f64::from(*v) / width as f64)
                .sum::<f64>() as f32;
            depths[row * width..(row + 1) * width].fill(mean);
        }
        Self::new(width, height, depths)
    }
    pub fn new(width: usize, height: usize, depths: Vec<f32>) -> Result<Self, TerrainError> {
        let len = width
            .checked_mul(height)
            .ok_or(TerrainError::ResourceLimit)?;
        if width < 2 || height < 2 || len != depths.len() {
            return Err(TerrainError::InvalidDimensions);
        }
        if len > MAX_RELIEF_PIXELS {
            return Err(TerrainError::ResourceLimit);
        }
        if depths.iter().any(|v| !v.is_finite()) {
            return Err(TerrainError::NonFinite);
        }
        if depths.iter().any(|v| *v < 0.0 || *v > RELIEF_DEPTH) {
            return Err(TerrainError::InvalidHeight);
        }
        for row in [0, height - 1] {
            if depths[row * width..(row + 1) * width]
                .iter()
                .any(|v| *v != depths[row * width])
            {
                return Err(TerrainError::InconsistentSeam);
            }
        }
        Ok(Self {
            width,
            height,
            depths: depths.into(),
            identity: Default::default(),
        })
    }
    pub fn sample_uv(&self, u: f64, v: f64) -> Result<f64, TerrainError> {
        if !u.is_finite() || !v.is_finite() {
            return Err(TerrainError::NonFinite);
        }
        let x = u.rem_euclid(1.0) * self.width as f64 - 0.5;
        let y = ((1.0 - v) * self.height as f64 - 0.5).clamp(0.0, (self.height - 1) as f64);
        let ix = x.floor();
        let iy = y.floor() as usize;
        let x0 = (ix as i64).rem_euclid(self.width as i64) as usize;
        let x1 = (x0 + 1) % self.width;
        let y1 = (iy + 1).min(self.height - 1);
        let tx = x - ix;
        let ty = y - iy as f64;
        let at = |x, y| f64::from(self.depths[y * self.width + x]);
        let a = at(x0, iy) * (1.0 - tx) + at(x1, iy) * tx;
        let b = at(x0, y1) * (1.0 - tx) + at(x1, y1) * tx;
        Ok(-(a * (1.0 - ty) + b * ty))
    }
    pub fn sample_direction(&self, d: [f64; 3]) -> Result<f64, TerrainError> {
        let n = super::ray::unit(d).ok_or(TerrainError::OutsideDomain)?;
        let u = n[2].atan2(-n[0]) / std::f64::consts::TAU;
        let v = 1.0 - n[1].clamp(-1.0, 1.0).acos() / std::f64::consts::PI;
        self.sample_uv(u, v)
    }
    /// Cache identity of the actual admitted scalar relief, independent of its URL.
    pub fn dimensions(&self) -> [usize; 2] {
        [self.width, self.height]
    }
    pub fn depths(&self) -> &[f32] {
        &self.depths
    }
    pub fn content_hash(&self) -> [u8; 32] {
        *self.identity.get_or_init(|| {
            use sha2::{Digest, Sha256};
            let mut h = Sha256::new();
            h.update((self.width as u64).to_le_bytes());
            h.update((self.height as u64).to_le_bytes());
            for value in self.depths.iter() {
                h.update(value.to_bits().to_le_bytes());
            }
            h.finalize().into()
        })
    }
}

/// Already baked binary16 RGBA terrain input. R=height, G=shore; B/A are
/// retained by the renderer owner and are deliberately not reinterpreted here.
/// Field row increases board y, with a two-texel border on every side.
#[derive(Clone, Debug)]
pub struct TerrainPatch {
    key: [i32; 2],
    rg: Arc<[[f32; 2]]>,
    surface: SurfaceClass,
}
impl TerrainPatch {
    pub fn from_half_rgba(
        key: [i32; 2],
        rgba: &[u16],
        surface: SurfaceClass,
    ) -> Result<Self, TerrainError> {
        if rgba.len() != FIELD_SIZE * FIELD_SIZE * 4 {
            return Err(TerrainError::InvalidDimensions);
        }
        let mut rg = Vec::with_capacity(FIELD_SIZE * FIELD_SIZE);
        for cell in rgba.as_chunks::<4>().0 {
            let value = [half(cell[0]), half(cell[1])];
            if !value.into_iter().all(f32::is_finite) {
                return Err(TerrainError::NonFinite);
            }
            rg.push(value);
        }
        Ok(Self {
            key,
            rg: rg.into(),
            surface,
        })
    }
    pub fn key(&self) -> [i32; 2] {
        self.key
    }
    fn sample(&self, p: BoardPoint) -> Result<[f64; 2], TerrainError> {
        let x = (p.x - f64::from(self.key[0]) * PATCH_SIZE) / FIELD_STEP + FIELD_BORDER as f64;
        let y = (p.y - f64::from(self.key[1]) * PATCH_SIZE) / FIELD_STEP + FIELD_BORDER as f64;
        if !x.is_finite() || !y.is_finite() {
            return Err(TerrainError::NonFinite);
        }
        if x < 0.0 || y < 0.0 || x > (FIELD_SIZE - 1) as f64 || y > (FIELD_SIZE - 1) as f64 {
            return Err(TerrainError::OutsideDomain);
        }
        let ix = (x.floor() as usize).min(FIELD_SIZE - 2);
        let iy = (y.floor() as usize).min(FIELD_SIZE - 2);
        let tx = x - ix as f64;
        let ty = y - iy as f64;
        Ok(std::array::from_fn(|c| {
            let at = |dx: usize, dy: usize| f64::from(self.rg[(iy + dy) * FIELD_SIZE + ix + dx][c]);
            let a = at(0, 0) * (1.0 - tx) + at(1, 0) * tx;
            let b = at(0, 1) * (1.0 - tx) + at(1, 1) * tx;
            a * (1.0 - ty) + b * ty
        }))
    }
}

/// Immutable low-level prepared-field snapshot. Body identity adapters and
/// source-to-field construction belong to later terrain owners. Seed/revision
/// are carried explicitly; samples never advance random state.
#[derive(Clone, Debug)]
pub struct TerrainField {
    projection: BodyProjection,
    revision: u64,
    seed: u32,
    relief: Option<ReliefMap>,
    patches: BTreeMap<[i32; 2], TerrainPatch>,
}
impl TerrainField {
    pub fn new(
        projection: BodyProjection,
        revision: u64,
        seed: u32,
        relief: Option<ReliefMap>,
        patches: Vec<TerrainPatch>,
    ) -> Result<Self, TerrainError> {
        if patches.len() > MAX_TERRAIN_PATCHES {
            return Err(TerrainError::ResourceLimit);
        }
        let mut map = BTreeMap::new();
        for patch in patches {
            if patch
                .rg
                .iter()
                .any(|v| projection.radius() + f64::from(v[0]) <= 0.0)
            {
                return Err(TerrainError::InvalidHeight);
            }
            if map.insert(patch.key, patch).is_some() {
                return Err(TerrainError::DuplicatePatch);
            }
        }
        // Shared border inputs must agree, including corner-only neighbors.
        // Otherwise sampling either side could give different ground contact.
        for (key, a) in &map {
            for delta in [[1, 0], [0, 1], [1, 1], [1, -1]] {
                let Some(nx) = key[0].checked_add(delta[0]) else {
                    continue;
                };
                let Some(ny) = key[1].checked_add(delta[1]) else {
                    continue;
                };
                if let Some(b) = map.get(&[nx, ny]) {
                    // Only the shared border can overlap. Scanning the entire
                    // interior repeated thousands of rejected coordinate tests.
                    let dx = delta[0] * (FIELD_SAMPLES - 1) as i32;
                    let dy = delta[1] * (FIELD_SAMPLES - 1) as i32;
                    for y in dy.max(0)..(FIELD_SIZE as i32 + dy).min(FIELD_SIZE as i32) {
                        for x in dx.max(0)..(FIELD_SIZE as i32 + dx).min(FIELD_SIZE as i32) {
                            let bx = x - dx;
                            let by = y - dy;
                            if a.rg[y as usize * FIELD_SIZE + x as usize]
                                != b.rg[by as usize * FIELD_SIZE + bx as usize]
                            {
                                return Err(TerrainError::InconsistentSeam);
                            }
                        }
                    }
                }
            }
        }
        if let Some(ref r) = relief
            && r.depths
                .iter()
                .any(|v| projection.radius() - f64::from(*v) + 0.0003 <= 0.0)
        {
            return Err(TerrainError::InvalidHeight);
        }
        Ok(Self {
            projection,
            revision,
            seed,
            relief,
            patches: map,
        })
    }
    pub fn revision(&self) -> u64 {
        self.revision
    }
    pub fn seed(&self) -> u32 {
        self.seed
    }
    pub fn projection(&self) -> BodyProjection {
        self.projection
    }
    pub fn bounds(&self) -> Option<[BoardPoint; 2]> {
        let mut keys = self.patches.keys();
        let first = keys.next()?;
        let (mut min, mut max) = (*first, *first);
        for key in keys {
            for c in 0..2 {
                min[c] = min[c].min(key[c]);
                max[c] = max[c].max(key[c]);
            }
        }
        Some([
            BoardPoint {
                x: f64::from(min[0]) * PATCH_SIZE,
                y: f64::from(min[1]) * PATCH_SIZE,
            },
            BoardPoint {
                x: (f64::from(max[0]) + 1.0) * PATCH_SIZE,
                y: (f64::from(max[1]) + 1.0) * PATCH_SIZE,
            },
        ])
    }
    fn patch(&self, p: BoardPoint) -> Result<Option<&TerrainPatch>, TerrainError> {
        if !p.x.is_finite() || !p.y.is_finite() {
            return Err(TerrainError::NonFinite);
        }
        let x = (p.x / PATCH_SIZE).floor();
        let y = (p.y / PATCH_SIZE).floor();
        // Finite points outside the stored key domain still have valid relief.
        if x < f64::from(i32::MIN)
            || x > f64::from(i32::MAX)
            || y < f64::from(i32::MIN)
            || y > f64::from(i32::MAX)
        {
            return Ok(None);
        }
        Ok(self.patches.get(&[x as i32, y as i32]))
    }
    pub(crate) fn fallback_height(&self, p: BoardPoint) -> Result<f64, TerrainError> {
        let n = self
            .projection
            .normal(p)
            .map_err(|_| TerrainError::OutsideDomain)?;
        Ok(match &self.relief {
            Some(r) => r.sample_direction(n)?,
            None => 0.0,
        } + 0.0003)
    }
    pub(crate) fn height_shore(&self, p: BoardPoint) -> Result<[f32; 2], TerrainError> {
        match self.patch(p)? {
            Some(patch) => Ok(patch.sample(p)?.map(|v| v as f32)),
            None => Ok([self.fallback_height(p)? as f32, EMPTY_SHORE]),
        }
    }
    pub fn height(&self, p: BoardPoint) -> Result<f32, TerrainError> {
        Ok(match self.patch(p)? {
            Some(patch) => patch.sample(p)?[0],
            None => self.fallback_height(p)?,
        } as f32)
    }
    pub fn sample(&self, p: BoardPoint) -> Result<TerrainSample, TerrainError> {
        let patch = self.patch(p)?;
        let at = |p| match patch {
            Some(field) => Ok(field.sample(p)?[0]),
            None => self.fallback_height(p),
        };
        let height = at(p)?;
        let shore = match patch {
            Some(field) => field.sample(p)?[1] as f32,
            None => EMPTY_SHORE,
        };
        let step = FIELD_STEP.min(self.projection.radius() * 0.001);
        if step <= 0.0 {
            return Err(TerrainError::InvalidNormal);
        }
        let point = |x, y| -> Result<[f64; 3], TerrainError> {
            let p = BoardPoint { x, y };
            let n = self
                .projection
                .normal(p)
                .map_err(|_| TerrainError::OutsideDomain)?;
            // Scale positions before subtraction to avoid radius² overflow.
            let scale = 1.0 + at(p)? / self.projection.radius();
            Ok(n.map(|v| v * scale))
        };
        let left = point(p.x - step, p.y)?;
        let right = point(p.x + step, p.y)?;
        let down = point(p.x, p.y - step)?;
        let up = point(p.x, p.y + step)?;
        let x = std::array::from_fn::<_, 3, _>(|i| right[i] - left[i]);
        let y = std::array::from_fn::<_, 3, _>(|i| up[i] - down[i]);
        let cross = [
            x[1] * y[2] - x[2] * y[1],
            x[2] * y[0] - x[0] * y[2],
            x[0] * y[1] - x[1] * y[0],
        ];
        let mut normal = super::ray::unit(cross).ok_or(TerrainError::InvalidNormal)?;
        let radial = self
            .projection
            .normal(p)
            .map_err(|_| TerrainError::OutsideDomain)?;
        if normal.iter().zip(radial).map(|(a, b)| a * b).sum::<f64>() < 0.0 {
            normal = normal.map(|v| -v);
        }
        let sample = TerrainSample {
            height: height as f32,
            normal: normal.map(|v| v as f32),
            shore,
            surface: patch.map_or(SurfaceClass::Ground, |p| p.surface),
        };
        sample.validate()?;
        Ok(sample)
    }
    /// Ground contact uses the identical sampled height and selected body radius.
    pub fn contact(&self, p: BoardPoint) -> Result<[f64; 3], TerrainError> {
        self.projection
            .project(p, f64::from(self.height(p)?))
            .map_err(|_| TerrainError::InvalidHeight)
    }
}

fn half(bits: u16) -> f32 {
    let sign = u32::from(bits & 0x8000) << 16;
    let exponent = (bits >> 10) & 31;
    let fraction = u32::from(bits & 1023);
    if exponent == 0 {
        let value = fraction as f32 * 2.0_f32.powi(-24);
        if sign == 0 { value } else { -value }
    } else {
        f32::from_bits(
            sign | if exponent == 31 {
                0x7f800000 | (fraction << 13)
            } else {
                ((u32::from(exponent) + 112) << 23) | (fraction << 13)
            },
        )
    }
}
