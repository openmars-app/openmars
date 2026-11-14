//! Complete source mining and land-claim geometry. One substantive part is
//! prepared per turn; actual working/retained bytes are admitted by its scene.
use super::{Frame, Kind};
use bevy::{
    asset::RenderAssetUsages,
    math::{DMat4, DQuat, DVec3},
    mesh::{Indices, PrimitiveTopology, VertexAttributeValues},
    prelude::*,
};
use openmars_scene::geometry::placement::random_sequence;
const TAU: f64 = std::f64::consts::TAU;
fn error(s: &str) -> String {
    s.into()
}
pub(super) fn cave_angle(seed: u32) -> f64 {
    random_sequence(seed.wrapping_add(10))() * TAU - std::f64::consts::FRAC_PI_2
}
pub(super) fn track(t: f64) -> DVec3 {
    DVec3::new(0., -0.13 * (1. - t), 0.004 - 0.024 * t)
}
fn slope() -> DQuat {
    DQuat::from_rotation_x((-0.024_f64).atan2(0.13))
}
#[derive(Clone, Copy, Debug)]
pub(super) struct CartPose {
    pub position: DVec3,
    pub tilt: f64,
    pub ore: bool,
}
/// The original ping-pong track and dump window, evaluated directly from the
/// canonical clock. Direction never depends on the previous rendered frame.
pub(super) fn cart_pose(seconds: f64) -> Result<CartPose, String> {
    if !seconds.is_finite() || seconds < 0. {
        return Err(error("mine cart clock"));
    }
    let cycle = ((seconds * 1.2).sin() + 1.) * 0.5;
    let returning = (seconds * 1.2).cos() <= 0.;
    let t = cycle * 1.3;
    let position = track(t.min(1.))
        + DVec3::new(
            0.,
            (t - 1.).max(0.) * 0.13 * 0.3,
            0.005 - (t - 1.).max(0.) * 0.024 * 0.3,
        );
    Ok(CartPose {
        position,
        tilt: if cycle < 0.1 && returning {
            (1. - cycle / 0.1) * 0.8
        } else {
            0.
        },
        ore: returning && cycle > 0.12,
    })
}
fn srgb(hex: u32) -> [f32; 4] {
    let linear = |v: u32| {
        let v = (v & 255) as f64 / 255.;
        (if v <= 0.04045 {
            v / 12.92
        } else {
            ((v + 0.055) / 1.055).powf(2.4)
        }) as f32
    };
    [linear(hex >> 16), linear(hex >> 8), linear(hex), 1.]
}
fn hsl(h: f64, s: f64, l: f64) -> [f32; 4] {
    let q = if l <= 0.5 {
        l * (1. + s)
    } else {
        l + s - l * s
    };
    let p = 2. * l - q;
    let hue = |t: f64| {
        let t = t.rem_euclid(1.);
        if t < 1. / 6. {
            p + (q - p) * 6. * t
        } else if t < 0.5 {
            q
        } else if t < 2. / 3. {
            p + (q - p) * 6. * (2. / 3. - t)
        } else {
            p
        }
    };
    [
        hue(h + 1. / 3.) as f32,
        hue(h) as f32,
        hue(h - 1. / 3.) as f32,
        1.,
    ]
}
pub(super) struct WorksiteWork {
    kind: Kind,
    frame: Frame,
    seed: u32,
    turn: usize,
    part: usize,
    part_bounds: Vec<[DVec3; 2]>,
    part_caves: Vec<bool>,
    part_sources: Vec<Option<crate::nuclear::collapse::PartSource>>,
    positions: Vec<[f32; 3]>,
    normals: Vec<[f32; 3]>,
    uv: Vec<[f32; 2]>,
    kinds: Vec<[f32; 2]>,
    colors: Vec<[f32; 4]>,
    indices: Vec<u32>,
}
impl WorksiteWork {
    pub const PEAK_BYTES: u64 = 8 * 1024 * 1024;
    pub fn new(kind: Kind, frame: Frame, seed: u32) -> Result<Self, String> {
        Ok(Self {
            kind,
            frame,
            seed,
            turn: 0,
            part: 0,
            part_bounds: vec![[DVec3::splat(f64::INFINITY), DVec3::splat(f64::NEG_INFINITY)]],
            part_caves: vec![false],
            part_sources: vec![None],
            positions: Vec::with_capacity(4096),
            normals: Vec::with_capacity(4096),
            uv: Vec::with_capacity(4096),
            kinds: Vec::with_capacity(4096),
            colors: Vec::with_capacity(4096),
            indices: Vec::with_capacity(16384),
        })
    }
    fn begin_part(&mut self, cave: bool) {
        self.part += 1;
        self.part_bounds
            .push([DVec3::splat(f64::INFINITY), DVec3::splat(f64::NEG_INFINITY)]);
        self.part_caves.push(cave);
        self.part_sources.push(None);
    }
    fn vertex(&mut self, p: DVec3, n: DVec3, uv: [f32; 2], class: u8, color: [f32; 4], cave: bool) {
        self.part_bounds[self.part][0] = self.part_bounds[self.part][0].min(p);
        self.part_bounds[self.part][1] = self.part_bounds[self.part][1].max(p);
        let q = DQuat::from_array(self.frame.rotation)
            * if cave {
                DQuat::from_rotation_z(cave_angle(self.seed))
            } else {
                DQuat::IDENTITY
            };
        self.positions.push(
            (DVec3::from_array(self.frame.origin) + q * p)
                .as_vec3()
                .to_array(),
        );
        self.normals.push((q * n).as_vec3().to_array());
        self.uv.push(uv);
        self.kinds.push([f32::from(class), self.part as f32]);
        self.colors.push(color);
    }
    fn mesh(
        &mut self,
        mesh: Mesh,
        offset: DVec3,
        rotation: DQuat,
        class: u8,
        color: [f32; 4],
        cave: bool,
    ) -> Result<(), String> {
        self.begin_part(cave);
        let positions = mesh
            .attribute(Mesh::ATTRIBUTE_POSITION)
            .and_then(|v| v.as_float3())
            .ok_or_else(|| error("worksite part positions"))?;
        let bounds = positions
            .iter()
            .map(|p| DVec3::from_array(p.map(f64::from)))
            .fold(
                [DVec3::splat(f64::INFINITY), DVec3::splat(f64::NEG_INFINITY)],
                |a, p| [a[0].min(p), a[1].max(p)],
            );
        let tile = DMat4::from_rotation_translation(
            DQuat::from_array(self.frame.rotation),
            DVec3::from_array(self.frame.origin),
        );
        let cave_matrix = if cave {
            DMat4::from_rotation_z(cave_angle(self.seed))
        } else {
            DMat4::IDENTITY
        };
        self.part_sources[self.part] = Some(crate::nuclear::collapse::PartSource {
            seed: u64::from(self.seed) + self.part as u64 * 97,
            bounds,
            body_from_mesh: tile * cave_matrix * DMat4::from_rotation_translation(rotation, offset),
            tile_from_body: tile.inverse(),
            cart: (13..=15).contains(&class),
            minimum: 0.,
        });
        let normals = mesh
            .attribute(Mesh::ATTRIBUTE_NORMAL)
            .and_then(|v| v.as_float3())
            .ok_or_else(|| error("worksite part normals"))?;
        let uv = match mesh.attribute(Mesh::ATTRIBUTE_UV_0) {
            Some(VertexAttributeValues::Float32x2(v)) => Some(v),
            _ => None,
        };
        let base = self.positions.len() as u32;
        for (i, p) in positions.iter().enumerate() {
            self.vertex(
                rotation * DVec3::from_array(p.map(f64::from)) + offset,
                rotation * DVec3::from_array(normals[i].map(f64::from)),
                uv.map_or([0.; 2], |v| v[i]),
                class,
                color,
                cave,
            );
        }
        if let Some(indices) = mesh.indices() {
            self.indices.extend(indices.iter().map(|i| base + i as u32));
        } else {
            self.indices.extend(base..base + positions.len() as u32);
        }
        Ok(())
    }
    fn triangle(&mut self, p: [DVec3; 3], class: u8, color: [f32; 4], cave: bool) {
        let normal = (p[1] - p[0]).cross(p[2] - p[0]).normalize_or_zero();
        let base = self.positions.len() as u32;
        for (i, p) in p.into_iter().enumerate() {
            self.vertex(
                p,
                normal,
                [[0., 0.], [1., 0.], [0., 1.]][i],
                class,
                color,
                cave,
            );
        }
        self.indices.extend([base, base + 1, base + 2]);
    }
    fn ground(&mut self) {
        let class = if self.kind == Kind::Reserved { 4 } else { 7 };
        let color = if self.kind == Kind::Reserved {
            [0.4, 0.28, 0.18, 1.]
        } else {
            [0.35, 0.22, 0.12, 1.]
        };
        let radius = 0.166 * 1.8;
        self.vertex(DVec3::ZERO, DVec3::Z, [0.5; 2], class, color, false);
        for r in 1..=10 {
            for a in 0..36 {
                let angle = a as f64 * TAU / 36.;
                let x = angle.cos() * radius * r as f64 / 10.;
                let y = angle.sin() * radius * r as f64 / 10.;
                self.vertex(
                    DVec3::new(-y, x, 0.),
                    DVec3::Z,
                    [
                        (0.5 + x / radius / 2.) as f32,
                        (0.5 + y / radius / 2.) as f32,
                    ],
                    class,
                    color,
                    false,
                );
            }
        }
        for a in 0..36 {
            self.indices.extend([0, 1 + a, 1 + (a + 1) % 36]);
        }
        for r in 1..10 {
            for a in 0..36 {
                let i = 1 + (r - 1) * 36 + a;
                let o = 1 + r * 36 + a;
                let next = (a + 1) % 36;
                let ni = 1 + (r - 1) * 36 + next;
                let no = 1 + r * 36 + next;
                self.indices.extend([i, o, no, i, no, ni]);
            }
        }
    }
    fn posts(&mut self) -> Result<(), String> {
        for i in 0..6 {
            let a = i as f64 * TAU / 6. + std::f64::consts::FRAC_PI_6;
            self.mesh(
                Cylinder::new(0.003, 0.05).mesh().resolution(6).build(),
                DVec3::new(a.cos() * 0.145, a.sin() * 0.145, 0.025),
                DQuat::from_rotation_x(std::f64::consts::FRAC_PI_2),
                5,
                [1.; 4],
                false,
            )?;
        }
        Ok(())
    }
    fn reservation_rails(&mut self) -> Result<(), String> {
        for i in 0..6 {
            let point = |j| {
                let a = j as f64 * TAU / 6. + std::f64::consts::FRAC_PI_6;
                DVec3::new(a.cos() * 0.145, a.sin() * 0.145, 0.)
            };
            let a = point(i);
            let b = point(i + 1);
            let mid = (a + b) * 0.5;
            let angle = (b.y - a.y).atan2(b.x - a.x) - std::f64::consts::FRAC_PI_2;
            for fraction in [0.25, 0.5, 0.75] {
                self.mesh(
                    Cylinder::new(0.0015, a.distance(b) as f32)
                        .mesh()
                        .resolution(4)
                        .build(),
                    mid + DVec3::Z * (0.05 * fraction),
                    DQuat::from_rotation_z(angle),
                    6,
                    [1.; 4],
                    false,
                )?;
            }
        }
        Ok(())
    }
    fn arch(&mut self) {
        self.begin_part(true);
        let w = 0.035;
        let h = 0.04;
        let mut profile = Vec::with_capacity(28);
        profile.push([-w, 0.]);
        profile.push([-w, h * 0.5]);
        for i in 1..=12 {
            let t = i as f64 / 12.;
            profile.push([
                -w * (1. - t * t),
                0.5 * h * (1. - t).powi(2) + 2. * (1. - t) * t * h + t * t * h,
            ]);
        }
        for i in 1..=12 {
            let t = i as f64 / 12.;
            profile.push([
                w * (2. * t - t * t),
                h * (1. - t).powi(2) + 2. * (1. - t) * t * h + 0.5 * h * t * t,
            ]);
        }
        profile.push([w, 0.]);
        let point = |i: usize, y: f64| DVec3::new(profile[i][0], y, profile[i][1] - 0.02);
        for i in 1..profile.len() - 1 {
            self.triangle(
                [point(0, 0.), point(i, 0.), point(i + 1, 0.)],
                9,
                [0.25, 0.16, 0.09, 1.],
                true,
            );
            self.triangle(
                [point(0, -0.04), point(i + 1, -0.04), point(i, -0.04)],
                9,
                [0.25, 0.16, 0.09, 1.],
                true,
            );
        }
        for i in 0..profile.len() {
            let j = (i + 1) % profile.len();
            let a = point(i, 0.);
            let b = point(j, 0.);
            let c = point(i, -0.04);
            let d = point(j, -0.04);
            self.triangle([a, c, b], 9, [0.25, 0.16, 0.09, 1.], true);
            self.triangle([b, c, d], 9, [0.25, 0.16, 0.09, 1.], true);
        }
    }
    fn cave(&mut self) -> Result<(), String> {
        self.mesh(
            Mesh::from(Cuboid::new(0.1, 0.06, 0.09)),
            DVec3::new(0., 0.015, -0.005),
            DQuat::IDENTITY,
            8,
            [0.32, 0.2, 0.11, 1.],
            true,
        )?;
        self.arch();
        let p = [
            DVec3::new(-0.0325, 0.038, -0.022),
            DVec3::new(0.0325, 0.038, -0.022),
            DVec3::new(-0.0325, 0.038, 0.018),
            DVec3::new(0.0325, 0.038, 0.018),
        ];
        self.begin_part(true);
        self.triangle([p[0], p[1], p[2]], 17, [0.03, 0.02, 0.01, 1.], true);
        self.triangle([p[2], p[1], p[3]], 17, [0.03, 0.02, 0.01, 1.], true);
        for side in [-1., 1.] {
            self.begin_part(true);
            for i in 0..10 {
                let a = track(i as f64 / 10.);
                let b = track((i + 1) as f64 / 10.);
                let bl = DVec3::new(side * 0.018, a.y, a.z - 0.003);
                let br = DVec3::new(side * 0.018, b.y, b.z - 0.003);
                let tl = DVec3::new(side * 0.018, a.y, (a.z + 0.001).max(0.004));
                let tr = DVec3::new(side * 0.018, b.y, (b.z + 0.001).max(0.004));
                let triangles = if side > 0. {
                    [[bl, br, tl], [tl, br, tr]]
                } else {
                    [[bl, tl, br], [tl, tr, br]]
                };
                for p in triangles {
                    self.triangle(p, 10, [0.3, 0.18, 0.1, 1.], true);
                }
            }
        }
        Ok(())
    }
    fn tracks(&mut self) -> Result<(), String> {
        for i in 0..12 {
            let t = (i as f64 + 0.5) / 12.;
            self.mesh(
                Mesh::from(Cuboid::new(0.028, 0.004, 0.0015)),
                track(t),
                slope(),
                11,
                srgb(0x3a2a1a),
                true,
            )?;
        }
        for x in [-0.009, 0.009] {
            self.mesh(
                Mesh::from(Cuboid::new(0.003, 0.13, 0.002)),
                track(0.5) + DVec3::new(x, 0., 0.001),
                slope(),
                12,
                srgb(0x5a5a5a),
                true,
            )?;
        }
        Ok(())
    }
    fn ore(&mut self, center: DVec3, radius: f64, color: [f32; 4]) {
        self.begin_part(true);
        let phi = (1. + 5_f64.sqrt()) / 2.;
        let t = 1. / phi;
        let v = [
            [-1., -1., -1.],
            [-1., -1., 1.],
            [-1., 1., -1.],
            [-1., 1., 1.],
            [1., -1., -1.],
            [1., -1., 1.],
            [1., 1., -1.],
            [1., 1., 1.],
            [0., -t, -phi],
            [0., -t, phi],
            [0., t, -phi],
            [0., t, phi],
            [-t, -phi, 0.],
            [-t, phi, 0.],
            [t, -phi, 0.],
            [t, phi, 0.],
            [-phi, 0., -t],
            [phi, 0., -t],
            [-phi, 0., t],
            [phi, 0., t],
        ]
        .map(|p| DVec3::from_array(p).normalize() * radius);
        let triangles = [
            [3, 11, 7],
            [3, 7, 15],
            [3, 15, 13],
            [7, 19, 17],
            [7, 17, 6],
            [7, 6, 15],
            [17, 4, 8],
            [17, 8, 10],
            [17, 10, 6],
            [8, 0, 16],
            [8, 16, 2],
            [8, 2, 10],
            [0, 12, 1],
            [0, 1, 18],
            [0, 18, 16],
            [6, 10, 2],
            [6, 2, 13],
            [6, 13, 15],
            [2, 16, 18],
            [2, 18, 3],
            [2, 3, 13],
            [18, 1, 9],
            [18, 9, 11],
            [18, 11, 3],
            [4, 14, 12],
            [4, 12, 0],
            [4, 0, 8],
            [11, 9, 5],
            [11, 5, 19],
            [11, 19, 7],
            [19, 5, 14],
            [19, 14, 4],
            [19, 4, 17],
            [1, 12, 14],
            [1, 14, 5],
            [1, 5, 9],
        ];
        for t in triangles {
            self.triangle(t.map(|i| slope() * (v[i] + center)), 15, color, true);
        }
    }
    fn cart(&mut self) -> Result<(), String> {
        self.mesh(
            Mesh::from(Cuboid::new(0.02, 0.016, 0.012)),
            DVec3::ZERO,
            slope(),
            13,
            srgb(0x666666),
            true,
        )?;
        for (x, y) in [
            (-0.009, -0.006),
            (0.009, -0.006),
            (-0.009, 0.006),
            (0.009, 0.006),
        ] {
            self.mesh(
                Cylinder::new(0.003, 0.003).mesh().resolution(6).build(),
                slope() * DVec3::new(x, y, -0.004),
                slope() * DQuat::from_rotation_y(std::f64::consts::FRAC_PI_2),
                14,
                srgb(0x333333),
                true,
            )?;
        }
        let mut random = random_sequence(self.seed.wrapping_add(500));
        for _ in 0..5 {
            let x = (random() - 0.5) * 0.012;
            let y = (random() - 0.5) * 0.008;
            let radius = 0.003 + random() * 0.003;
            let color = hsl(
                0.07 + random() * 0.05,
                0.4 + random() * 0.3,
                0.3 + random() * 0.2,
            );
            self.ore(DVec3::new(x, y, 0.008), radius, color);
        }
        Ok(())
    }
    fn pile(&mut self) {
        self.begin_part(true);
        let rotation = DQuat::from_rotation_x(-std::f64::consts::FRAC_PI_2);
        let base = self.positions.len() as u32;
        for y in 0..=6 {
            for x in 0..=8 {
                let phi = x as f64 * TAU / 8.;
                let theta = y as f64 * std::f64::consts::FRAC_PI_2 / 6.;
                let normal = DVec3::new(
                    -phi.cos() * theta.sin(),
                    theta.cos(),
                    phi.sin() * theta.sin(),
                );
                self.vertex(
                    rotation * (normal * 0.015) + DVec3::new(0.03, -0.12, 0.004),
                    rotation * normal,
                    [x as f32 / 8., 1. - y as f32 / 6.],
                    16,
                    [0.35, 0.25, 0.15, 1.],
                    true,
                );
            }
        }
        for y in 0..6 {
            for x in 0..8 {
                let a = base + (y * 9 + x) as u32;
                let b = a + 1;
                let c = a + 9;
                let d = c + 1;
                if y != 0 {
                    self.indices.extend([a, b, c]);
                }
                self.indices.extend([b, d, c]);
            }
        }
    }
    pub fn advance(&mut self) -> Result<bool, String> {
        if self.kind == Kind::Reserved {
            match self.turn {
                0 => self.ground(),
                1 => self.posts()?,
                2 => self.reservation_rails()?,
                _ => return Err(error("worksite already complete")),
            }
        } else {
            match self.turn {
                0 => self.ground(),
                1 => self.cave()?,
                2 => self.tracks()?,
                3 => self.cart()?,
                4 => self.pile(),
                _ => return Err(error("worksite already complete")),
            }
        }
        self.turn += 1;
        if self.positions.len() > 16384 || self.indices.len() > 65536 {
            return Err(error("worksite geometry bound"));
        }
        Ok(self.turn == if self.kind == Kind::Reserved { 3 } else { 5 })
    }
    pub fn take_mesh(self) -> Result<(Mesh, Vec<crate::nuclear::collapse::PartSource>), String> {
        if self.turn != if self.kind == Kind::Reserved { 3 } else { 5 } {
            return Err(error("worksite incomplete"));
        }
        if self.part >= 32 {
            return Err(error("worksite collapse part bound"));
        }
        let tile = DMat4::from_rotation_translation(
            DQuat::from_array(self.frame.rotation),
            DVec3::from_array(self.frame.origin),
        );
        let sources = self
            .part_sources
            .iter()
            .enumerate()
            .map(|(i, source)| {
                source
                    .clone()
                    .unwrap_or_else(|| crate::nuclear::collapse::PartSource {
                        seed: u64::from(self.seed) + i as u64 * 97,
                        bounds: self.part_bounds[i],
                        body_from_mesh: tile
                            * if self.part_caves[i] {
                                DMat4::from_rotation_z(cave_angle(self.seed))
                            } else {
                                DMat4::IDENTITY
                            },
                        tile_from_body: tile.inverse(),
                        cart: self
                            .kinds
                            .iter()
                            .any(|k| k[1] == i as f32 && (13. ..=15.).contains(&k[0])),
                        minimum: 0.,
                    })
            })
            .collect();
        Ok((
            Mesh::new(
                PrimitiveTopology::TriangleList,
                RenderAssetUsages::MAIN_WORLD | RenderAssetUsages::RENDER_WORLD,
            )
            // Builder capacity is covered by PEAK_BYTES while this handoff runs.
            // The retained mesh must not inherit the worst-case mining workspace:
            // a small reservation site uses only a fraction of those buffers.
            .with_inserted_attribute(
                Mesh::ATTRIBUTE_POSITION,
                self.positions.into_boxed_slice().into_vec(),
            )
            .with_inserted_attribute(
                Mesh::ATTRIBUTE_NORMAL,
                self.normals.into_boxed_slice().into_vec(),
            )
            .with_inserted_attribute(Mesh::ATTRIBUTE_UV_0, self.uv.into_boxed_slice().into_vec())
            .with_inserted_attribute(
                Mesh::ATTRIBUTE_UV_1,
                self.kinds.into_boxed_slice().into_vec(),
            )
            .with_inserted_attribute(
                Mesh::ATTRIBUTE_COLOR,
                self.colors.into_boxed_slice().into_vec(),
            )
            .with_inserted_indices(Indices::U32(self.indices.into_boxed_slice().into_vec())),
            sources,
        ))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use openmars_scene::geometry::{AxialPoint, BodyProjection};
    use std::collections::BTreeMap;
    #[test]
    fn small_reservation_releases_builder_slack_before_render_admission() {
        let frame = Frame::new(
            BodyProjection::new(2.02, 2.02, -1, AxialPoint::default()).unwrap(),
            [1, 0, -1],
            0.,
        )
        .unwrap();
        let mut work = WorksiteWork::new(Kind::Reserved, frame, 37).unwrap();
        while !work.advance().unwrap() {}
        assert!(work.positions.capacity() > work.positions.len());
        assert!(work.indices.capacity() > work.indices.len());
        let (mesh, parts) = work.take_mesh().unwrap();
        assert_eq!(parts.len(), 25);
        let mut retained = 0;
        for (_, values) in mesh.attributes() {
            let (length, capacity, stride) = match values {
                VertexAttributeValues::Float32x2(v) => (v.len(), v.capacity(), 8),
                VertexAttributeValues::Float32x3(v) => (v.len(), v.capacity(), 12),
                VertexAttributeValues::Float32x4(v) => (v.len(), v.capacity(), 16),
                _ => panic!("unexpected worksite attribute"),
            };
            assert_eq!(capacity, length, "retained mesh carries builder slack");
            retained += capacity * stride;
        }
        let Some(Indices::U32(indices)) = mesh.indices() else {
            panic!("worksite indices")
        };
        assert_eq!(indices.capacity(), indices.len());
        retained += indices.capacity() * 4;
        let gpu = mesh.get_vertex_buffer_size() + mesh.get_index_buffer_bytes().unwrap().len();
        assert_eq!(retained, gpu);
        assert!(retained * 2 < WorksiteWork::PEAK_BYTES as usize);
    }
    #[test]
    fn complete_original_parts_have_finite_topology_and_seeded_ore() {
        let frame = Frame::new(
            BodyProjection::new(2.02, 2.02, -1, AxialPoint::default()).unwrap(),
            [1, 0, -1],
            0.,
        )
        .unwrap();
        for kind in [Kind::Mining, Kind::Reserved] {
            let mut work = WorksiteWork::new(kind, frame.clone(), 37).unwrap();
            let mut turns = 1;
            while !work.advance().unwrap() {
                turns += 1;
            }
            assert_eq!(turns, if kind == Kind::Mining { 5 } else { 3 });
            let (mesh, parts) = work.take_mesh().unwrap();
            assert_eq!(parts.len(), if kind == Kind::Reserved { 25 } else { 31 });
            assert_eq!(
                parts.iter().filter(|p| p.cart).count(),
                if kind == Kind::Mining { 10 } else { 0 }
            );
            let facts: Vec<_> = parts
                .iter()
                .enumerate()
                .map(|(i, p)| {
                    assert_eq!(p.seed, 37 + i as u64 * 97);
                    let f = p.capture(DMat4::IDENTITY);
                    assert!(f.depth.is_finite() && f.depth >= 0.065);
                    f
                })
                .collect();
            let moving = crate::nuclear::collapse::animated::<32>(&facts, 0.4);
            assert!(
                moving[..parts.len()]
                    .windows(2)
                    .any(|pair| pair[0] != pair[1])
            );
            assert!(
                crate::nuclear::collapse::animated::<32>(&facts, -0.01)
                    .iter()
                    .all(|v| v.z == 0. && v.w == 0.)
            );

            let positions = mesh
                .attribute(Mesh::ATTRIBUTE_POSITION)
                .unwrap()
                .as_float3()
                .unwrap();
            let normals = mesh
                .attribute(Mesh::ATTRIBUTE_NORMAL)
                .unwrap()
                .as_float3()
                .unwrap();
            assert!(positions.iter().flatten().all(|v| v.is_finite()));
            assert!(
                normals
                    .iter()
                    .all(|v| (Vec3::from_array(*v).length() - 1.).abs() < 2e-5)
            );
            assert!(mesh.indices().unwrap().iter().all(|i| i < positions.len()));
            let classes = match mesh.attribute(Mesh::ATTRIBUTE_UV_1).unwrap() {
                VertexAttributeValues::Float32x2(v) => v,
                _ => panic!(),
            };
            assert!(
                classes
                    .iter()
                    .all(|v| v[1] >= 0. && v[1] < parts.len() as f32)
            );
            let mut counts = BTreeMap::new();
            for c in classes {
                *counts.entry(c[0] as u8).or_insert(0usize) += 1;
            }
            if kind == Kind::Reserved {
                assert_eq!(counts.keys().copied().collect::<Vec<_>>(), vec![4, 5, 6]);
                assert_eq!(counts[&4], 361);
                assert!(counts[&5] > 0 && counts[&6] > 0);
            } else {
                assert_eq!(
                    counts.keys().copied().collect::<Vec<_>>(),
                    (7..=17).collect::<Vec<_>>()
                );
                assert_eq!(counts[&7], 361);
                assert_eq!(counts[&11], 12 * 24);
                assert_eq!(counts[&12], 2 * 24);
                assert_eq!(counts[&13], 24);
                assert_eq!(counts[&15], 5 * 36 * 3);
                assert_eq!(counts[&16], 63);
            }
        }
        let mut a = WorksiteWork::new(Kind::Mining, frame.clone(), 37).unwrap();
        let mut b = WorksiteWork::new(Kind::Mining, frame.clone(), 37).unwrap();
        while !a.advance().unwrap() {}
        while !b.advance().unwrap() {}
        assert_eq!(a.positions, b.positions);
        assert_eq!(a.colors, b.colors);
        let mut c = WorksiteWork::new(Kind::Mining, frame, 38).unwrap();
        while !c.advance().unwrap() {}
        assert_ne!(a.positions, c.positions);
        assert_ne!(a.colors, c.colors);
    }
    #[test]
    fn cart_track_dump_and_ore_come_from_clock_not_frame_history() {
        let first = cart_pose(0.).unwrap();
        assert!((first.position.y + 0.0455).abs() < 1e-12);
        assert!((first.position.z + 0.0066).abs() < 1e-12);
        assert_eq!(first.tilt, 0.);
        assert!(!first.ore);
        let returning = cart_pose(std::f64::consts::PI / 1.2).unwrap();
        assert!(returning.ore);
        assert!(returning.position.distance(first.position) < 1e-12);
        let dump = cart_pose(3. * std::f64::consts::FRAC_PI_2 / 1.2).unwrap();
        assert!((dump.tilt - 0.8).abs() < 1e-12);
        assert!(!dump.ore);
        assert!((dump.position.y + 0.13).abs() < 1e-12);
        assert!((dump.position.z - 0.009).abs() < 1e-12);
        let far = cart_pose(12345.5).unwrap();
        cart_pose(2.).unwrap();
        let direct = cart_pose(12345.5).unwrap();
        assert_eq!(far.position, direct.position);
        assert_eq!(far.ore, direct.ore);
        assert!(cart_pose(f64::NAN).is_err());
        assert!(cart_pose(-1.).is_err());
    }
}
