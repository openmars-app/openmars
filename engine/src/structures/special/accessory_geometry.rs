//! Original rim, stripe, three swept pipes and complete merged GLB fence.
//! Positions stay in body-local plane space until the common vertex shader.
use super::frame::Frame;
use bevy::{
    asset::RenderAssetUsages,
    math::{DQuat, DVec3, EulerRot},
    mesh::{Indices, PrimitiveTopology, VertexAttributeValues},
    prelude::*,
};
fn error(s: &str) -> String {
    s.into()
}
const TAU: f64 = std::f64::consts::TAU;
pub(super) struct AccessoryWork {
    frame: Frame,
    seed: u32,
    turn: usize,
    raw_bounds: [[DVec3; 2]; 3],
    fence_next: usize,
    layout: Vec<[f64; 3]>,
    positions: Vec<[f32; 3]>,
    normals: Vec<[f32; 3]>,
    uv: Vec<[f32; 2]>,
    kinds: Vec<[f32; 2]>,
    colors: Vec<[f32; 4]>,
    growth: Vec<[f32; 4]>,
    indices: Vec<u32>,
}
pub(super) fn fence_layout(width: f64) -> Result<Vec<[f64; 3]>, String> {
    if !width.is_finite() || width <= 0. {
        return Err(error("fence width"));
    }
    let mut out = Vec::with_capacity(24);
    for side in 0..6 {
        let point = |i: usize| {
            let a = i as f64 * TAU / 6. + std::f64::consts::FRAC_PI_6;
            [a.cos() * 0.098, a.sin() * 0.098]
        };
        let a = point(side);
        let b = point(side + 1);
        let length = (b[0] - a[0]).hypot(b[1] - a[1]);
        let n = (length / width).round().clamp(1., 4.) as usize;
        let angle = (b[1] - a[1]).atan2(b[0] - a[0]);
        for j in 0..n {
            let t = (j as f64 + 0.5) / n as f64;
            out.push([a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t, angle]);
        }
    }
    Ok(out)
}
impl AccessoryWork {
    pub const PEAK_BYTES: u64 = 8 * 1024 * 1024;
    pub fn new(frame: Frame, seed: u32, width: f64) -> Result<Self, String> {
        Ok(Self {
            frame,
            seed,
            turn: 0,
            raw_bounds: [[DVec3::splat(f64::INFINITY), DVec3::splat(f64::NEG_INFINITY)]; 3],
            fence_next: 0,
            layout: fence_layout(width)?,
            positions: Vec::with_capacity(8192),
            normals: Vec::with_capacity(8192),
            uv: Vec::with_capacity(8192),
            kinds: Vec::with_capacity(8192),
            colors: Vec::with_capacity(8192),
            growth: Vec::with_capacity(8192),
            indices: Vec::with_capacity(32768),
        })
    }
    fn vertex(
        &mut self,
        p: DVec3,
        n: DVec3,
        uv: [f32; 2],
        kind_angle: [f32; 2],
        color: [f32; 4],
        growth: DVec3,
    ) {
        if kind_angle[0] < 3. {
            let i = kind_angle[0] as usize;
            let raw = p.as_vec3().as_dvec3();
            self.raw_bounds[i][0] = self.raw_bounds[i][0].min(raw);
            self.raw_bounds[i][1] = self.raw_bounds[i][1].max(raw);
        }
        let q = DQuat::from_array(self.frame.rotation);
        self.positions.push(
            (DVec3::from_array(self.frame.origin) + q * p)
                .as_vec3()
                .to_array(),
        );
        self.normals.push((q * n).as_vec3().to_array());
        self.uv.push(uv);
        self.kinds.push(kind_angle);
        self.colors.push(color);
        self.growth
            .push((q * growth).as_vec3().extend(0.).to_array());
    }
    fn ring(&mut self, inner: f64, outer: f64, rings: usize, kind: f32) {
        let base = self.positions.len() as u32;
        for r in 0..=rings {
            let radius = inner + (outer - inner) * r as f64 / rings as f64;
            for a in 0..=56 {
                let angle = a as f64 * TAU / 56.;
                let x = radius * angle.cos();
                let y = radius * angle.sin();
                self.vertex(
                    DVec3::new(x, y, 0.),
                    DVec3::Z,
                    [
                        (x / outer * 0.5 + 0.5) as f32,
                        (y / outer * 0.5 + 0.5) as f32,
                    ],
                    [kind, 0.],
                    [1.; 4],
                    DVec3::ZERO,
                );
            }
        }
        for r in 0..rings {
            for a in 0..56 {
                let i = base + (r * 57 + a) as u32;
                self.indices
                    .extend_from_slice(&[i, i + 57, i + 58, i, i + 58, i + 1]);
            }
        }
    }
    fn pipe(&mut self, i: usize) {
        let seed = f64::from(self.seed);
        let random = |j: usize| {
            let x = (seed * 127.1 + j as f64 * 311.7).sin() * 43758.5453;
            x - x.floor()
        };
        let angle = i as f64 * TAU / 3. + (random(i * 3) - 0.5) * 0.8;
        let (s, c) = angle.sin_cos();
        let point = |r, z| DVec3::new(c * r, s * r, z);
        let wall = 0.046;
        let tip = 0.074 + random(i * 3 + 1) * 0.008;
        let path = [
            Curve::line(point(wall, -0.072), point(wall, -0.003)),
            Curve::quadratic(
                point(wall, -0.003),
                point(wall, 0.005),
                point(wall + 0.008, 0.005),
            ),
            Curve::line(point(wall + 0.008, 0.005), point(tip, 0.005)),
            Curve::quadratic(point(tip, 0.005), point(tip, 0.005), point(tip, -0.003)),
            Curve::line(point(tip, -0.003), point(tip, -0.03)),
        ];
        let total = path.iter().map(|c| c.length).sum::<f64>();
        let at = |u: f64| {
            let d = u.clamp(0., 1.) * total;
            let mut previous = 0.;
            for curve in &path {
                if d <= previous + curve.length {
                    return curve.at_distance(d - previous);
                }
                previous += curve.length;
            }
            path.last().unwrap().point(1.)
        };
        let mut tangents = [DVec3::ZERO; 49];
        for (j, t) in tangents.iter_mut().enumerate() {
            let u = j as f64 / 48.;
            *t = (at((u + 0.0001).min(1.)) - at((u - 0.0001).max(0.))).normalize();
        }
        let first = tangents[0];
        let mut axis = DVec3::ZERO;
        let mut min = f64::INFINITY;
        for k in 0..3 {
            if first[k].abs() <= min {
                min = first[k].abs();
                axis = DVec3::ZERO;
                axis[k] = 1.;
            }
        }
        let mut normal = first.cross(first.cross(axis).normalize()).normalize();
        let base = self.positions.len() as u32;
        for j in 0..=48 {
            if j > 0 {
                let cross = tangents[j - 1].cross(tangents[j]);
                if cross.length() > f64::EPSILON {
                    normal = DQuat::from_axis_angle(
                        cross.normalize(),
                        tangents[j - 1].dot(tangents[j]).clamp(-1., 1.).acos(),
                    ) * normal;
                }
            }
            let binormal = tangents[j].cross(normal);
            let center = at(j as f64 / 48.);
            for a in 0..=5 {
                let theta = a as f64 * TAU / 5.;
                let n = -normal * theta.cos() + binormal * theta.sin();
                self.vertex(
                    center + n * 0.002,
                    n,
                    [j as f32 / 48., a as f32 / 5.],
                    [2., 0.],
                    [1.; 4],
                    DVec3::ZERO,
                );
            }
        }
        for j in 0..48 {
            for a in 0..5 {
                let i = base + (j * 6 + a) as u32;
                self.indices
                    .extend_from_slice(&[i, i + 6, i + 1, i + 6, i + 7, i + 1]);
            }
        }
    }
    fn fence_piece(&mut self, mesh: &Mesh, layout: [f64; 3]) -> Result<(), String> {
        let positions = mesh
            .attribute(Mesh::ATTRIBUTE_POSITION)
            .and_then(|v| v.as_float3())
            .ok_or_else(|| error("fence positions"))?;
        let normals = mesh
            .attribute(Mesh::ATTRIBUTE_NORMAL)
            .and_then(|v| v.as_float3())
            .ok_or_else(|| error("fence normals"))?;
        let uv = match mesh.attribute(Mesh::ATTRIBUTE_UV_0) {
            Some(VertexAttributeValues::Float32x2(v)) => v,
            _ => return Err(error("fence UV")),
        };
        let colors = match mesh.attribute(Mesh::ATTRIBUTE_COLOR) {
            Some(VertexAttributeValues::Float32x4(v)) => v,
            _ => return Err(error("fence colors")),
        };
        let rotation = DQuat::from_euler(
            EulerRot::XYZ,
            std::f64::consts::FRAC_PI_2,
            layout[2] + std::f64::consts::FRAC_PI_2,
            0.,
        );
        let base = self.positions.len() as u32;
        for (i, p) in positions.iter().enumerate() {
            let p = DVec3::from_array(p.map(f64::from));
            self.vertex(
                rotation * p + DVec3::new(layout[0], layout[1], 0.005),
                rotation * DVec3::from_array(normals[i].map(f64::from)),
                uv[i],
                [3., layout[2] as f32],
                colors[i],
                rotation * DVec3::new(0., p.y, 0.),
            );
        }
        self.indices.extend(
            mesh.indices()
                .ok_or_else(|| error("fence indices"))?
                .iter()
                .map(|i| base + i as u32),
        );
        Ok(())
    }
    /// One ring, one pipe, or at most four complete fence pieces per turn.
    pub fn advance(&mut self, fence: &Mesh) -> Result<bool, String> {
        match self.turn {
            0 => self.ring(0.056, 0.1596, 8, 0.),
            1 => self.ring(0.055, 0.063, 1, 1.),
            2..=4 => self.pipe(self.turn - 2),
            _ => {
                for _ in 0..4 {
                    if self.fence_next == self.layout.len() {
                        break;
                    }
                    self.fence_piece(fence, self.layout[self.fence_next])?;
                    self.fence_next += 1;
                }
            }
        }
        self.turn += 1;
        if self.positions.len() > 16384 || self.indices.len() > 65536 {
            return Err(error("mohole accessory geometry bound"));
        }
        Ok(self.turn >= 5 && self.fence_next == self.layout.len())
    }
    pub fn take_mesh(self) -> Result<(Mesh, [[DVec3; 2]; 3]), String> {
        if self.turn < 5 || self.fence_next != self.layout.len() {
            return Err(error("mohole accessories incomplete"));
        }
        let mut this = self;
        this.positions.shrink_to_fit();
        this.normals.shrink_to_fit();
        this.uv.shrink_to_fit();
        this.kinds.shrink_to_fit();
        this.colors.shrink_to_fit();
        this.growth.shrink_to_fit();
        this.indices.shrink_to_fit();
        Ok((
            mesh(
                this.positions,
                this.normals,
                this.uv,
                this.kinds,
                this.colors,
                this.growth,
                this.indices,
            ),
            this.raw_bounds,
        ))
    }
}
/// Keep preparation and live accessories on exactly the same indexed layout.
pub(super) fn mesh(
    positions: Vec<[f32; 3]>,
    normals: Vec<[f32; 3]>,
    uv: Vec<[f32; 2]>,
    kinds: Vec<[f32; 2]>,
    colors: Vec<[f32; 4]>,
    growth: Vec<[f32; 4]>,
    indices: Vec<u32>,
) -> Mesh {
    Mesh::new(
        PrimitiveTopology::TriangleList,
        RenderAssetUsages::MAIN_WORLD | RenderAssetUsages::RENDER_WORLD,
    )
    .with_inserted_attribute(Mesh::ATTRIBUTE_POSITION, positions)
    .with_inserted_attribute(Mesh::ATTRIBUTE_NORMAL, normals)
    .with_inserted_attribute(Mesh::ATTRIBUTE_UV_0, uv)
    .with_inserted_attribute(Mesh::ATTRIBUTE_UV_1, kinds)
    .with_inserted_attribute(Mesh::ATTRIBUTE_COLOR, colors)
    .with_inserted_attribute(Mesh::ATTRIBUTE_TANGENT, growth)
    .with_inserted_indices(Indices::U32(indices))
}
struct Curve {
    points: [DVec3; 3],
    quadratic: bool,
    lengths: [f64; 201],
    length: f64,
}
impl Curve {
    fn line(a: DVec3, b: DVec3) -> Self {
        Self::new([a, b, b], false)
    }
    fn quadratic(a: DVec3, b: DVec3, c: DVec3) -> Self {
        Self::new([a, b, c], true)
    }
    fn new(points: [DVec3; 3], quadratic: bool) -> Self {
        let mut c = Self {
            points,
            quadratic,
            lengths: [0.; 201],
            length: 0.,
        };
        let mut previous = points[0];
        for i in 1..=200 {
            let p = c.point(i as f64 / 200.);
            c.lengths[i] = c.lengths[i - 1] + p.distance(previous);
            previous = p;
        }
        c.length = c.lengths[200];
        c
    }
    fn point(&self, t: f64) -> DVec3 {
        if self.quadratic {
            self.points[0] * (1. - t).powi(2)
                + self.points[1] * (2. * (1. - t) * t)
                + self.points[2] * t * t
        } else {
            self.points[0].lerp(self.points[1], t)
        }
    }
    fn at_distance(&self, d: f64) -> DVec3 {
        let d = d.clamp(0., self.length);
        let high = self.lengths.partition_point(|v| *v < d).clamp(1, 200);
        let low = high - 1;
        let span = self.lengths[high] - self.lengths[low];
        let f = if span > 0. {
            (d - self.lengths[low]) / span
        } else {
            0.
        };
        self.point((low as f64 + f) / 200.)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use openmars_scene::geometry::BodyProjection;
    #[test]
    fn full_rim_stripe_and_three_seeded_swept_pipes_keep_original_topology() {
        let frame = Frame::new(BodyProjection::MARS, [0, 0, 0]).unwrap();
        for seed in [0, 37, u32::MAX] {
            let mut work = AccessoryWork::new(frame.clone(), seed, 0.01).unwrap();
            work.ring(0.056, 0.1596, 8, 0.);
            work.ring(0.055, 0.063, 1, 1.);
            for i in 0..3 {
                work.pipe(i);
            }
            assert_eq!(work.positions.len(), 1509);
            assert_eq!(work.indices.len(), 7344);
            assert!(
                work.indices
                    .iter()
                    .all(|i| (*i as usize) < work.positions.len())
            );
            assert!(
                work.positions
                    .iter()
                    .flatten()
                    .chain(work.uv.iter().flatten())
                    .all(|x| x.is_finite())
            );
            assert!(
                work.normals
                    .iter()
                    .all(|n| (Vec3::from_array(*n).length() - 1.).abs() < 1e-6)
            );
            let pipes = &work.positions[627..];
            assert!(pipes.iter().any(|v| v[2] < 1.965));
            assert!(pipes.iter().any(|v| v[2] > 2.033));
        }
        assert_eq!(fence_layout(0.01).unwrap().len(), 24);
        assert_eq!(fence_layout(1.).unwrap().len(), 6);
        assert!(fence_layout(0.).is_err());
    }
}
