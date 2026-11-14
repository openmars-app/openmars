//! Original facade canvases, prepared in bounded rows without per-frame work.
use bevy::{
    asset::RenderAssetUsages,
    image::{ImageAddressMode, ImageFilterMode, ImageSampler, ImageSamplerDescriptor},
    prelude::*,
    render::render_resource::{Extent3d, TextureDimension, TextureFormat},
};
pub const SOURCES: [(&str, bool); 15] = [
    ("textures/terrain/concrete-tile-facade-color", true),
    ("textures/terrain/concrete-tile-facade-normal", false),
    ("textures/terrain/concrete-tile-facade-roughness", false),
    ("textures/terrain/rectangular-facade-tiles-color", true),
    ("textures/terrain/rectangular-facade-tiles-normal", false),
    ("textures/terrain/rectangular-facade-tiles-roughness", false),
    ("textures/terrain/ribbed-concrete-wall-color", true),
    ("textures/terrain/ribbed-concrete-wall-normal", false),
    ("textures/terrain/ribbed-concrete-wall-roughness", false),
    ("textures/terrain/concrete", true),
    ("textures/terrain/grass", true),
    ("textures/terrain/sand", true),
    ("textures/terrain/noise-mid", false),
    ("textures/terrain/noise-high", false),
    ("textures/effects/smoke", false),
];
#[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord)]
pub enum Slot {
    Raw(usize, usize),
    Facade(usize, usize),
    Shared(usize),
    Pool,
}
#[derive(Clone, Copy, Debug)]
pub enum Role {
    Color,
    Normal,
    Roughness,
    Emission,
    Pool,
}
#[derive(Clone, Copy, Debug)]
pub struct Overlay {
    pub variant: usize,
    pub bright: bool,
}
#[derive(Clone, Copy, Debug)]
pub struct Spec {
    pub slot: Slot,
    pub source: Option<usize>,
    pub width: u32,
    pub height: u32,
    pub srgb: bool,
    pub role: Role,
    pub overlay: Option<Overlay>,
}
pub const GENERATED_IMAGES: usize = 34;
pub fn plans() -> Vec<Spec> {
    let mut specs = Vec::new();
    for family in 0..3 {
        for role in 0..3 {
            specs.push(Spec {
                slot: Slot::Raw(family, role),
                source: Some(family * 3 + role),
                width: 512,
                height: 1024,
                srgb: role == 0,
                role: [Role::Color, Role::Normal, Role::Roughness][role],
                overlay: None,
            });
        }
    }
    for facade in 0..6 {
        for role in 0..4 {
            if facade >= 3 && (role == 1 || role == 2) {
                continue;
            }
            specs.push(Spec {
                slot: Slot::Facade(facade, role),
                source: if role == 3 {
                    None
                } else {
                    Some(facade % 3 * 3 + role)
                },
                width: 512,
                height: 1024,
                srgb: role == 0 || role == 3,
                role: [Role::Color, Role::Normal, Role::Roughness, Role::Emission][role],
                overlay: Some(Overlay {
                    variant: facade % 3,
                    bright: facade >= 3,
                }),
            });
        }
    }
    for shared in 0..5 {
        specs.push(Spec {
            slot: Slot::Shared(shared),
            source: Some(9 + shared),
            width: 512,
            height: 512,
            srgb: shared < 3,
            role: Role::Color,
            overlay: None,
        });
    }
    // Preserve the approved non-square source, alpha and linear colour data.
    specs.push(Spec {
        slot: Slot::Shared(5),
        source: Some(14),
        width: 377,
        height: 329,
        srgb: false,
        role: Role::Color,
        overlay: None,
    });
    specs.push(Spec {
        slot: Slot::Pool,
        source: None,
        width: 64,
        height: 64,
        srgb: true,
        role: Role::Pool,
        overlay: None,
    });
    specs
}
#[derive(Clone, Copy, PartialEq, Eq)]
enum Stage {
    Source,
    Overlay,
    Post,
    Flip,
    Done,
}
pub struct CpuJob {
    pub spec: Spec,
    pixels: Vec<u8>,
    row: u32,
    stage: Stage,
}
impl CpuJob {
    pub fn new(spec: Spec) -> Result<Self, String> {
        if spec.width == 0
            || spec.height == 0
            || spec.width > 1024
            || spec.height > 2048
            || spec.source.is_some_and(|s| s >= SOURCES.len())
            || spec.overlay.is_some_and(|v| v.variant >= 3)
        {
            return Err("Structure raster admission".into());
        }
        Ok(Self {
            spec,
            pixels: vec![0; (spec.width * spec.height * 4) as usize],
            row: 0,
            stage: Stage::Source,
        })
    }
    pub fn progress(&self) -> (u32, u32) {
        let height = self.spec.height;
        let offset = match self.stage {
            Stage::Source => 0,
            Stage::Overlay => height,
            Stage::Post => height,
            Stage::Flip => height * 2,
            Stage::Done => height * 2 + height / 2,
        };
        (offset + self.row, height * 2 + height / 2)
    }
    /// At most64 source/post rows or32 row swaps per call. No decoded source clone.
    pub fn advance(&mut self, image: Option<&Image>) -> Result<bool, String> {
        let spec = self.spec;
        let end = (self.row + 64).min(spec.height);
        match self.stage {
            Stage::Source => {
                let source = if spec.source.is_some() {
                    Some(Source::new(
                        image.ok_or("Missing structure raster source")?,
                    )?)
                } else {
                    None
                };
                if let Some(source) = &source
                    && source.srgb != spec.srgb
                {
                    return Err("Structure source color space mismatch".into());
                }
                for y in self.row..end {
                    for x in 0..spec.width {
                        let pixel = if let Some(source) = &source {
                            source.sample(
                                [
                                    (f64::from(x) + 0.5) * f64::from(source.width)
                                        / f64::from(spec.width)
                                        - 0.5,
                                    (f64::from(y) + 0.5) * f64::from(source.height)
                                        / f64::from(spec.height)
                                        - 0.5,
                                ],
                                spec.srgb,
                            )
                        } else if matches!(spec.role, Role::Pool) {
                            let radius =
                                ((f64::from(x) + 0.5 - 32.).hypot(f64::from(y) + 0.5 - 32.) / 32.)
                                    .min(1.);
                            let a = if radius < 0.35 {
                                0.8 + (0.4 - 0.8) * radius / 0.35
                            } else {
                                0.4 * (1. - (radius - 0.35) / 0.65)
                            };
                            [255, 255, 255, (a * 255.).round() as u8]
                        } else {
                            [0, 0, 0, 255]
                        };
                        let at = ((y * spec.width + x) * 4) as usize;
                        self.pixels[at..at + 4].copy_from_slice(&pixel);
                    }
                }
                self.row = end;
                if end == spec.height {
                    self.row = 0;
                    self.stage = Stage::Overlay;
                }
            }
            Stage::Overlay => {
                if let Some(overlay) = spec.overlay {
                    window_overlay(
                        &mut self.pixels,
                        spec.width,
                        spec.height,
                        overlay.variant,
                        overlay.bright,
                        match spec.role {
                            Role::Normal => 1,
                            Role::Roughness => 2,
                            _ => 0,
                        },
                        matches!(spec.role, Role::Emission),
                    );
                }
                self.stage = Stage::Post;
            }
            Stage::Post => {
                for pixel in self.pixels
                    [(self.row * spec.width * 4) as usize..(end * spec.width * 4) as usize]
                    .as_chunks_mut::<4>()
                    .0
                {
                    if matches!(spec.role, Role::Normal) {
                        let strength = if spec.overlay.is_some() { 0.45 } else { 0.35 };
                        let normal = Vec3::new(
                            (f32::from(pixel[0]) / 255. * 2. - 1.) * strength,
                            (f32::from(pixel[1]) / 255. * 2. - 1.) * strength,
                            f32::from(pixel[2]) / 255. * 2. - 1.,
                        )
                        .normalize_or_zero();
                        for (value, channel) in pixel.iter_mut().zip([normal.x, normal.y, normal.z])
                        {
                            *value = ((channel * 0.5 + 0.5) * 255.).round() as u8;
                        }
                    } else if matches!(spec.role, Role::Roughness) {
                        pixel[0] = 255;
                        pixel[2] = 255;
                    }
                }
                self.row = end;
                if end == spec.height {
                    self.row = 0;
                    self.stage = Stage::Flip;
                }
            }
            Stage::Flip => {
                // THREE's ordinary/CanvasTexture uploads used flipY=true. Flip
                // stored rows, retaining original UVs and tangent handedness.
                let end = (self.row + 32).min(spec.height / 2);
                let stride = (spec.width * 4) as usize;
                for y in self.row..end {
                    let low = (y as usize) * stride;
                    let high = ((spec.height - 1 - y) as usize) * stride;
                    let (a, b) = self.pixels.split_at_mut(high);
                    a[low..low + stride].swap_with_slice(&mut b[..stride]);
                }
                self.row = end;
                if end == spec.height / 2 {
                    self.row = 0;
                    self.stage = Stage::Done;
                }
            }
            Stage::Done => return Ok(true),
        }
        Ok(self.stage == Stage::Done)
    }
    pub fn finish(self) -> Result<Image, String> {
        if self.stage != Stage::Done {
            return Err("Unfinished structure raster".into());
        }
        let spec = self.spec;
        let mut image = Image::new(
            Extent3d {
                width: spec.width,
                height: spec.height,
                depth_or_array_layers: 1,
            },
            TextureDimension::D2,
            self.pixels,
            if spec.srgb {
                TextureFormat::Rgba8UnormSrgb
            } else {
                TextureFormat::Rgba8Unorm
            },
            RenderAssetUsages::RENDER_WORLD,
        );
        let address = if (matches!(spec.role, Role::Pool) || spec.slot == Slot::Shared(5)) {
            ImageAddressMode::ClampToEdge
        } else {
            ImageAddressMode::Repeat
        };
        image.sampler = ImageSampler::Descriptor(ImageSamplerDescriptor {
            address_mode_u: address,
            address_mode_v: address,
            mag_filter: ImageFilterMode::Linear,
            min_filter: ImageFilterMode::Linear,
            mipmap_filter: ImageFilterMode::Linear,
            anisotropy_clamp: 4,
            ..default()
        });
        Ok(image)
    }
}
static SRGB_LINEAR: std::sync::OnceLock<[f64; 256]> = std::sync::OnceLock::new();
struct Source<'a> {
    data: &'a [u8],
    width: u32,
    height: u32,
    channels: usize,
    srgb: bool,
    decode: Option<&'static [f64; 256]>,
}
impl<'a> Source<'a> {
    fn new(image: &'a Image) -> Result<Self, String> {
        let size = image.size();
        let format = image.texture_descriptor.format;
        let channels = match format {
            TextureFormat::R8Unorm => 1,
            TextureFormat::Rg8Unorm => 2,
            TextureFormat::Rgba8Unorm | TextureFormat::Rgba8UnormSrgb => 4,
            _ => return Err("Structure source must be eight-bit pixels".into()),
        };
        let data = image
            .data
            .as_ref()
            .ok_or("Missing structure decoded pixels")?;
        if size.x == 0
            || size.y == 0
            || size.x > 4096
            || size.y > 4096
            || data.len() != size.x as usize * size.y as usize * channels
            || image.texture_descriptor.size.depth_or_array_layers != 1
        {
            return Err("Structure source bounds".into());
        }
        Ok(Self {
            data,
            width: size.x,
            height: size.y,
            channels,
            srgb: format.is_srgb(),
            decode: format.is_srgb().then(|| {
                SRGB_LINEAR.get_or_init(|| std::array::from_fn(|v| linear(v as f64 / 255.)))
            }),
        })
    }
    fn sample(&self, point: [f64; 2], target_srgb: bool) -> [u8; 4] {
        let [x, y] = point;
        let (ix, iy) = (x.floor() as i64, y.floor() as i64);
        let (tx, ty) = (x - x.floor(), y - y.floor());
        std::array::from_fn(|c| {
            let read = |dx: i64, dy: i64| {
                let x = (ix + dx).rem_euclid(i64::from(self.width)) as u32;
                let y = (iy + dy).rem_euclid(i64::from(self.height)) as u32;
                let pixel =
                    &self.data[(y * self.width + x) as usize * self.channels..][..self.channels];
                let value = match (self.channels, c) {
                    (1 | 3, 3) => 255,
                    (1 | 2, 0..=2) => pixel[0],
                    (2, 3) => pixel[1],
                    _ => pixel[c],
                };
                let v = f64::from(value) / 255.;
                if c < 3
                    && let Some(decode) = self.decode
                {
                    decode[value as usize]
                } else {
                    v
                }
            };
            let value = (read(0, 0) * (1. - tx) + read(1, 0) * tx) * (1. - ty)
                + (read(0, 1) * (1. - tx) + read(1, 1) * tx) * ty;
            let value = if target_srgb && c < 3 {
                encode(value)
            } else {
                value
            };
            (value.clamp(0., 1.) * 255.).round() as u8
        })
    }
}
fn linear(v: f64) -> f64 {
    if v <= 0.04045 {
        v / 12.92
    } else {
        ((v + 0.055) / 1.055).powf(2.4)
    }
}
fn encode(v: f64) -> f64 {
    if v <= 0.0031308 {
        v * 12.92
    } else {
        1.055 * v.powf(1. / 2.4) - 0.055
    }
}

fn fill(pixels: &mut [u8], size: [u32; 2], rect: [f64; 4], color: [u8; 4]) {
    let [width, height] = size;
    let [x, y, w, h] = rect;
    let scale = f64::from(width) / 256.;
    let (x, y, w, h) = (x * scale, y * scale, w * scale, h * scale);
    for py in y.max(0.).floor() as u32..((y + h).ceil() as u32).min(height) {
        for px in x.max(0.).floor() as u32..((x + w).ceil() as u32).min(width) {
            pixels[((py * width + px) * 4) as usize..((py * width + px + 1) * 4) as usize]
                .copy_from_slice(&color);
        }
    }
}
fn window_overlay(
    pixels: &mut [u8],
    width: u32,
    height: u32,
    variant: usize,
    bright: bool,
    kind: usize,
    emission: bool,
) {
    let mut rng = openmars_scene::geometry::placement::random_sequence(475 + variant as u32 * 31);
    let w = [30., 36., 22.][variant];
    let h = [32., 26., 40.][variant];
    for row in 0..8 {
        for column in 0..4 {
            let x = column as f64 * 64. + (64. - w) / 2.;
            let y = row as f64 * 64. + (64. - h) / 2.;
            let lit = rng() > if bright { 0.15 } else { 0.58 };
            if emission {
                if lit {
                    fill(pixels, [width, height], [x, y, w, h], [156, 174, 184, 255]);
                    fill(
                        pixels,
                        [width, height],
                        [x + 2., y + 2., w - 4., 5.],
                        [240, 207, 137, 255],
                    );
                }
            } else if kind == 0 {
                fill(
                    pixels,
                    [width, height],
                    [x - 2., y - 2., w + 4., h + 4.],
                    [82, 99, 107, 255],
                );
                fill(
                    pixels,
                    [width, height],
                    [x, y, w, h],
                    if lit {
                        [185, 203, 208, 255]
                    } else {
                        [35, 62, 80, 255]
                    },
                );
                fill(
                    pixels,
                    [width, height],
                    [x + 2., y + 2., w - 4., 5.],
                    if lit {
                        [231, 217, 169, 255]
                    } else {
                        [65, 100, 117, 255]
                    },
                );
            } else {
                fill(
                    pixels,
                    [width, height],
                    [x - 2., y - 2., w + 4., h + 4.],
                    if kind == 1 {
                        [128, 128, 255, 255]
                    } else {
                        [85, 85, 85, 255]
                    },
                );
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    fn image(data: Vec<u8>, size: [u32; 2], format: TextureFormat) -> Image {
        Image::new(
            Extent3d {
                width: size[0],
                height: size[1],
                depth_or_array_layers: 1,
            },
            TextureDimension::D2,
            data,
            format,
            RenderAssetUsages::MAIN_WORLD,
        )
    }
    #[test]
    fn resampling_keeps_linear_light_data_alpha_and_wrapping_distinct() {
        let bytes = vec![0, 0, 0, 20, 255, 255, 255, 220];
        let srgb = image(bytes.clone(), [2, 1], TextureFormat::Rgba8UnormSrgb);
        let linear_image = image(bytes, [2, 1], TextureFormat::Rgba8Unorm);
        let source = Source::new(&srgb).unwrap();
        assert_eq!(source.sample([0.5, 0.], true), [188, 188, 188, 120]);
        assert_eq!(source.sample([-0.5, 0.], true), [188, 188, 188, 120]);
        assert_eq!(
            Source::new(&linear_image).unwrap().sample([0.5, 0.], false),
            [128, 128, 128, 120]
        );
        let luma = image(vec![43, 127], [1, 1], TextureFormat::Rg8Unorm);
        assert_eq!(
            Source::new(&luma).unwrap().sample([0., 0.], false),
            [43, 43, 43, 127]
        );
    }
    #[test]
    fn preparation_is_bounded_and_flips_texture_rows_without_changing_uvs() {
        let source = image(
            vec![255, 0, 0, 20, 0, 0, 255, 220],
            [1, 2],
            TextureFormat::Rgba8UnormSrgb,
        );
        let spec = Spec {
            slot: Slot::Shared(0),
            source: Some(0),
            width: 1,
            height: 2,
            srgb: true,
            role: Role::Color,
            overlay: None,
        };
        assert!(CpuJob::new(spec).unwrap().finish().is_err());
        let mut job = CpuJob::new(spec).unwrap();
        for _ in 0..20 {
            if job.advance(Some(&source)).unwrap() {
                break;
            }
        }
        assert_eq!(job.progress().0, job.progress().1);
        let prepared = job.finish().unwrap();
        assert_eq!(prepared.data.unwrap(), [0, 0, 255, 220, 255, 0, 0, 20]);
        let mut job = CpuJob::new(Spec {
            height: 192,
            ..spec
        })
        .unwrap();
        assert!(!job.advance(Some(&source)).unwrap());
        assert_eq!(job.progress().0, 64);
        assert!(
            CpuJob::new(Spec {
                width: 1025,
                ..spec
            })
            .is_err()
        );
        assert!(CpuJob::new(spec).unwrap().advance(None).is_err());
        let mut invalid = source;
        invalid.data = Some(vec![]);
        assert!(Source::new(&invalid).is_err());
    }

    #[test]
    fn plans_define_resolution_map_roles_and_shared_normal_roughness() {
        let specs = plans();
        assert_eq!(specs.len(), GENERATED_IMAGES);
        let unique: std::collections::BTreeSet<_> = specs.iter().map(|s| s.slot).collect();
        assert_eq!(unique.len(), GENERATED_IMAGES);
        assert_eq!(SOURCES.len(), 15);
        for spec in specs {
            if matches!(spec.slot, Slot::Raw(..) | Slot::Facade(..)) {
                assert_eq!((spec.width, spec.height), (512, 1024));
            }
            if matches!(spec.role, Role::Normal | Role::Roughness) {
                assert!(!spec.srgb);
            }
            if let Some(source) = spec.source {
                assert_eq!(SOURCES[source].1, spec.srgb);
            }
            if let Slot::Facade(family, role) = spec.slot {
                assert!(family < 3 || role == 0 || role == 3);
            }
        }
    }
}
