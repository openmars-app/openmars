//! Original radial topology, with the existing source-qualified CPU volcano
//! field supplying all heights and derivative metadata. Admission precedes work.
use super::frame::Frame;
use bevy::{
    asset::RenderAssetUsages,
    mesh::{Indices, PrimitiveTopology},
    prelude::*,
};
use openmars_scene::geometry::volcano::{VOLCANO_DEFAULT, VolcanoParams, volcano_surface_sample};
use std::f64::consts::TAU;
fn error() -> String {
    "Invalid special surface geometry".into()
}
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum SurfaceKind {
    Volcano,
    Mohole,
}
impl SurfaceKind {
    pub fn from_type(s: &str) -> Option<Self> {
        match s {
            "volcano-tile" => Some(Self::Volcano),
            "mohole-tile" => Some(Self::Mohole),
            _ => None,
        }
    }
    pub fn index(self) -> f32 {
        match self {
            Self::Volcano => 0.,
            Self::Mohole => 1.,
        }
    }
    pub fn extent(self) -> (f64, usize, usize) {
        if self == Self::Volcano {
            (0.15, 32, 64)
        } else {
            (0.14, 28, 56)
        }
    }
}
pub struct SurfaceWork {
    kind: SurfaceKind,
    frame: Frame,
    params: VolcanoParams,
    next: usize,
    positions: Vec<[f32; 3]>,
    metadata: Vec<[f32; 3]>,
    uv: Vec<[f32; 2]>,
    indices: Vec<u32>,
    taken: bool,
}
impl SurfaceWork {
    pub fn new(kind: SurfaceKind, frame: Frame, seed: u32) -> Result<Self, String> {
        let n = frame.normal.iter().map(|v| v * v).sum::<f64>();
        if !n.is_finite()
            || (n - 1.).abs() > 1e-10
            || frame
                .origin
                .iter()
                .zip(frame.normal)
                .map(|(v, n)| v * n)
                .sum::<f64>()
                <= 0.
        {
            return Err(error());
        }
        let (_, rings, segments) = kind.extent();
        let n = 1 + rings * segments;
        Ok(Self {
            kind,
            frame,
            params: VolcanoParams {
                seed: seed as f32,
                ..VOLCANO_DEFAULT
            },
            next: 0,
            positions: Vec::with_capacity(n),
            metadata: Vec::with_capacity(n),
            uv: Vec::with_capacity(n),
            indices: Vec::with_capacity(segments * 3 + (rings - 1) * segments * 6),
            taken: false,
        })
    }
    pub fn allocation_bytes(kind: SurfaceKind) -> u64 {
        let (_, rings, segments) = kind.extent();
        ((1 + rings * segments) * 32 + (segments * 3 + (rings - 1) * segments * 6) * 4) as u64
            + 65536
    }
    pub fn advance(&mut self) -> Result<bool, String> {
        if self.taken {
            return Err(error());
        }
        let (radius, rings, segments) = self.kind.extent();
        let n = 1 + rings * segments;
        for i in self.next..(self.next + 256).min(n) {
            let (r, a) = if i == 0 {
                (0., 0.)
            } else {
                (
                    ((i - 1) / segments + 1) as f64 / rings as f64 * radius,
                    ((i - 1) % segments) as f64 / segments as f64 * TAU,
                )
            };
            let (x, y) = (a.cos() * r, a.sin() * r);
            let d = self.frame.direction(x, y);
            // Store the original plane point: the shader applies emergence,
            // sphere projection and then collapse in that exact order.
            let direction_scale = (self
                .frame
                .origin
                .iter()
                .zip(self.frame.normal)
                .map(|(a, b)| a * b)
                .sum::<f64>())
                / d.iter()
                    .zip(self.frame.normal)
                    .map(|(a, b)| a * b)
                    .sum::<f64>();
            self.positions.push(d.map(|v| (v * direction_scale) as f32));
            let uv = [
                (0.5 + x / radius / 2.) as f32,
                (0.5 + y / radius / 2.) as f32,
            ];
            self.uv.push(uv);
            let meta = if self.kind == SurfaceKind::Volcano {
                let (cx, cy) = ((x / radius) as f32, (y / radius) as f32);
                volcano_surface_sample(cx, cy, &self.params).map_err(|_| error())?
            } else {
                [0.; 3]
            };
            self.metadata.push(meta);
            self.next = i + 1;
        }
        if self.next == n && self.indices.is_empty() {
            for a in 0..segments {
                self.indices
                    .extend([0, 1 + a as u32, 1 + ((a + 1) % segments) as u32]);
            }
            for r in 1..rings {
                let inner = 1 + (r - 1) * segments;
                let outer = 1 + r * segments;
                for a in 0..segments {
                    let next = (a + 1) % segments;
                    self.indices.extend([
                        (inner + a) as u32,
                        (outer + a) as u32,
                        (outer + next) as u32,
                        (inner + a) as u32,
                        (outer + next) as u32,
                        (inner + next) as u32,
                    ]);
                }
            }
        }
        Ok(self.next == n)
    }
    pub fn take_ready(&mut self) -> Option<Mesh> {
        let (_, rings, segments) = self.kind.extent();
        if self.taken || self.next != 1 + rings * segments {
            return None;
        }
        self.taken = true;
        Some(mesh(
            std::mem::take(&mut self.positions),
            std::mem::take(&mut self.metadata),
            std::mem::take(&mut self.uv),
            std::mem::take(&mut self.indices),
        ))
    }
}
/// The production vertex layout is also used for shared pipeline preparation.
pub(super) fn mesh(
    positions: Vec<[f32; 3]>,
    metadata: Vec<[f32; 3]>,
    uv: Vec<[f32; 2]>,
    indices: Vec<u32>,
) -> Mesh {
    Mesh::new(
        PrimitiveTopology::TriangleList,
        RenderAssetUsages::MAIN_WORLD | RenderAssetUsages::RENDER_WORLD,
    )
    .with_inserted_attribute(Mesh::ATTRIBUTE_POSITION, positions)
    .with_inserted_attribute(Mesh::ATTRIBUTE_NORMAL, metadata)
    .with_inserted_attribute(Mesh::ATTRIBUTE_UV_0, uv)
    .with_inserted_indices(Indices::U32(indices))
}
#[cfg(test)]
mod tests {
    use super::*;
    use openmars_scene::geometry::BodyProjection;
    #[test]
    fn original_radial_topology_and_boundary_derivatives_are_complete() {
        for (kind, vertices, indices) in [
            (SurfaceKind::Volcano, 2049, 12096),
            (SurfaceKind::Mohole, 1569, 9240),
        ] {
            let frame = Frame::new(BodyProjection::MARS, [0, 0, 0]).unwrap();
            let mut work = SurfaceWork::new(kind, frame, 37).unwrap();
            let mut turns = 1;
            while !work.advance().unwrap() {
                turns += 1;
            }
            assert!(turns <= 9);
            let mesh = work.take_ready().unwrap();
            assert!(work.take_ready().is_none());
            assert_eq!(mesh.count_vertices(), vertices);
            assert_eq!(mesh.indices().unwrap().len(), indices);
            let meta = mesh
                .attribute(Mesh::ATTRIBUTE_NORMAL)
                .unwrap()
                .as_float3()
                .unwrap();
            assert!(meta.iter().flatten().all(|v| v.is_finite()));
            if kind == SurfaceKind::Volcano {
                assert!(meta.iter().any(|p| p[0] > 0.1));
                assert!(meta.iter().any(|p| p[1].abs() > 0.01));
                for i in [0, 256, vertices - 1] {
                    let uv = match mesh.attribute(Mesh::ATTRIBUTE_UV_0).unwrap() {
                        bevy::mesh::VertexAttributeValues::Float32x2(v) => v[i],
                        _ => panic!(),
                    };
                    let expected = volcano_surface_sample(
                        (uv[0] - 0.5) * 2.,
                        (uv[1] - 0.5) * 2.,
                        &VolcanoParams {
                            seed: 37.,
                            ..VOLCANO_DEFAULT
                        },
                    )
                    .unwrap();
                    for (a, b) in meta[i].iter().zip(expected) {
                        assert!((a - b).abs() < 2e-5);
                    }
                }
            }
        }
    }
}
