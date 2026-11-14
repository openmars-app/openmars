//! Incremental legacy-oriented relief globe. Scalar contact is shared with tiles.
use bevy::{
    asset::RenderAssetUsages,
    math::DVec3,
    mesh::{Indices, PrimitiveTopology},
    prelude::*,
    render::render_resource::TextureFormat,
};
use openmars_scene::geometry::{BoardPoint, MARS_RADIUS, terrain::ReliefMap};
use std::sync::Arc;
pub mod bake;
pub mod grid;
pub mod materials;
pub mod mips;
pub mod surface;
pub mod upload;
pub use materials::TerrainMaterialPlugin;
const WIDTH: usize = 512;
const HEIGHT: usize = 256;
const VERTICES: usize = (WIDTH + 1) * (HEIGHT + 1);
const INDICES: usize = WIDTH * (HEIGHT - 1) * 6;
fn error(message: &str) -> String {
    message.into()
}

#[derive(Resource, Default)]
pub struct TerrainState {
    pub relief: Option<Arc<ReliefMap>>,
    pub revision: u64,
    work: Option<ReliefMeshWork>,
    pub error: Option<String>,
    pub contact: Option<surface::Contact>,
}
impl TerrainState {
    pub fn height(&self, point: BoardPoint) -> f64 {
        if let Some(contact) = &self.contact {
            return contact.height(point).unwrap_or(0.0);
        }
        self.relief
            .as_ref()
            .and_then(|map| {
                let direction = openmars_scene::geometry::BodyProjection::MARS
                    .project(point, 0.0)
                    .ok()?;
                map.sample_direction(direction).ok()
            })
            .unwrap_or(0.0)
    }
    pub fn progress(&self) -> Option<(usize, usize)> {
        self.work.as_ref().map(|work| {
            let done = match work.phase {
                0 => work.row,
                1 => HEIGHT + 1 + work.row,
                2 => 2 * HEIGHT + 1 + work.row,
                _ => 3 * HEIGHT + 2,
            };
            (done, 3 * HEIGHT + 2)
        })
    }
    pub fn start(&mut self, image: &Image) -> Result<(), String> {
        let rgba = sample_color(image)?;
        let relief =
            Arc::new(ReliefMap::from_rgba(WIDTH, HEIGHT, &rgba).map_err(|e| e.to_string())?);
        self.work = Some(ReliefMeshWork::new(relief.clone()));
        self.relief = Some(relief);
        self.revision += 1;
        Ok(())
    }
    pub fn advance(&mut self) -> Result<Option<Mesh>, String> {
        let Some(work) = self.work.as_mut() else {
            return Ok(None);
        };
        // Bound construction to twelve rows per frame, including normal work.
        for _ in 0..12 {
            if work.step()? {
                return self.work.take().map(ReliefMeshWork::finish).transpose();
            }
        }
        Ok(None)
    }
}

fn sample_color(image: &Image) -> Result<Vec<u8>, String> {
    if !matches!(
        image.texture_descriptor.format,
        TextureFormat::Rgba8UnormSrgb | TextureFormat::Rgba8Unorm
    ) {
        return Err(error("Mars relief requires RGBA8 pixels"));
    }
    let width = image.width() as usize;
    let height = image.height() as usize;
    if width < WIDTH || height < HEIGHT || width > 8192 || height > 8192 {
        return Err(error("Mars relief image dimensions are outside bounds"));
    }
    let source = image
        .data
        .as_ref()
        .ok_or_else(|| error("Missing decoded Mars pixels"))?;
    if source.len() != width * height * 4 {
        return Err(error("Invalid decoded Mars pixel size"));
    }
    let mut rgba = vec![0; WIDTH * HEIGHT * 4];
    // Average encoded sRGB pixels before the legacy scalar map's sRGB decode,
    // mirroring canvas image reduction without retaining a second full image.
    for y in 0..HEIGHT {
        for x in 0..WIDTH {
            let (x0, x1) = (x * width / WIDTH, (x + 1) * width / WIDTH);
            let (y0, y1) = (y * height / HEIGHT, (y + 1) * height / HEIGHT);
            let mut sum = 0u32;
            let count = ((x1 - x0) * (y1 - y0)) as u32;
            for row in y0..y1 {
                for column in x0..x1 {
                    sum += u32::from(source[(row * width + column) * 4]);
                }
            }
            rgba[(y * WIDTH + x) * 4] = ((sum + count / 2) / count) as u8;
            rgba[(y * WIDTH + x) * 4 + 3] = 255;
        }
    }
    Ok(rgba)
}

pub struct ReliefMeshWork {
    relief: Arc<ReliefMap>,
    positions: Vec<[f32; 3]>,
    normals: Vec<Vec3>,
    uv: Vec<[f32; 2]>,
    indices: Vec<u32>,
    row: usize,
    phase: u8,
}
impl ReliefMeshWork {
    pub fn new(relief: Arc<ReliefMap>) -> Self {
        Self {
            relief,
            positions: Vec::with_capacity(VERTICES),
            normals: vec![Vec3::ZERO; VERTICES],
            uv: Vec::with_capacity(VERTICES),
            indices: Vec::with_capacity(INDICES),
            row: 0,
            phase: 0,
        }
    }
    /// One bounded row at a time, including area-weighted normals and the
    /// duplicated longitude seam. The caller yields to the canonical frame.
    pub fn step(&mut self) -> Result<bool, String> {
        match self.phase {
            0 => {
                let v = self.row as f64 / HEIGHT as f64;
                let theta = v * std::f64::consts::PI;
                for x in 0..=WIDTH {
                    let u = x as f64 / WIDTH as f64;
                    let phi = u * std::f64::consts::TAU;
                    // Use identical seam/pole coordinates, avoiding sin(TAU)
                    // roundoff becoming a crack in generated normals.
                    let direction = if self.row == 0 {
                        DVec3::Y
                    } else if self.row == HEIGHT {
                        -DVec3::Y
                    } else if x == WIDTH {
                        DVec3::new(-theta.sin(), theta.cos(), 0.)
                    } else {
                        DVec3::new(
                            -phi.cos() * theta.sin(),
                            theta.cos(),
                            phi.sin() * theta.sin(),
                        )
                    };
                    let r = MARS_RADIUS
                        + self
                            .relief
                            .sample_direction(direction.to_array())
                            .map_err(|_| error("globe relief sample"))?;
                    self.positions.push((direction * r).as_vec3().to_array());
                    // Bevy's decoded image origin is top-left, unlike the
                    // source WebGL texture upload's flipped V convention.
                    self.uv.push([u as f32, v as f32]);
                }
                self.row += 1;
                if self.row > HEIGHT {
                    self.row = 0;
                    self.phase = 1;
                }
            }
            1 => {
                for x in 0..WIDTH {
                    let b = self.row * (WIDTH + 1) + x;
                    let a = b + 1;
                    let c = b + WIDTH + 1;
                    let d = c + 1;
                    if self.row != 0 {
                        self.triangle([a, b, d]);
                    }
                    if self.row != HEIGHT - 1 {
                        self.triangle([b, c, d]);
                    }
                }
                self.row += 1;
                if self.row == HEIGHT {
                    self.row = 0;
                    self.phase = 2;
                }
            }
            2 => {
                let start = self.row * (WIDTH + 1);
                let seam = (self.normals[start] + self.normals[start + WIDTH]).normalize_or_zero();
                for x in 0..=WIDTH {
                    let i = start + x;
                    self.normals[i] = if x == 0 || x == WIDTH {
                        seam
                    } else {
                        self.normals[i].normalize_or_zero()
                    };
                    if self.normals[i].length_squared() < 0.5 {
                        return Err(error("degenerate globe normal"));
                    }
                }
                self.row += 1;
                if self.row > HEIGHT {
                    self.phase = 3;
                }
            }
            _ => {}
        }
        Ok(self.phase == 3)
    }
    fn triangle(&mut self, triangle: [usize; 3]) {
        let [a, b, c] = triangle.map(|i| Vec3::from_array(self.positions[i]));
        let normal = (b - a).cross(c - a);
        for i in triangle {
            self.indices.push(i as u32);
            self.normals[i] += normal;
        }
    }
    pub fn finish(self) -> Result<Mesh, String> {
        if self.phase != 3 || self.positions.len() != VERTICES || self.indices.len() != INDICES {
            return Err(error("globe mesh incomplete"));
        }
        Ok(Mesh::new(
            PrimitiveTopology::TriangleList,
            RenderAssetUsages::MAIN_WORLD | RenderAssetUsages::RENDER_WORLD,
        )
        .with_inserted_attribute(Mesh::ATTRIBUTE_POSITION, self.positions)
        .with_inserted_attribute(
            Mesh::ATTRIBUTE_NORMAL,
            self.normals
                .into_iter()
                .map(|normal| normal.to_array())
                .collect::<Vec<_>>(),
        )
        .with_inserted_attribute(Mesh::ATTRIBUTE_UV_0, self.uv)
        .with_inserted_indices(Indices::U32(self.indices)))
    }
}
