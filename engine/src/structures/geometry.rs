//! Shared procedural structure meshes. Authored layout facts select reusable
//! features, never card IDs. Coordinates remain in the original board plane
//! until the final projection onto the actual celestial content sphere.
use bevy::{
    asset::RenderAssetUsages,
    math::{DQuat, DVec3 as V},
    mesh::{Indices, PrimitiveTopology},
    platform::collections::HashMap,
    prelude::Mesh,
};
use openmars_scene::structure::layout::{
    CAPITAL_WALL_APOTHEM, inside_building, wall_apothem, wall_distance,
};
use openmars_scene::{
    geometry::{AxialPoint, BoardPoint as Point, BodyProjection as Projection},
    structure::*,
};
use std::{
    collections::BTreeMap,
    f64::consts::{PI, TAU},
};
const MAX_VERTICES: usize = 2_000_000;
const MAX_PARTS: usize = 32768;
pub(super) const MATERIALS: usize = 19;
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum GeometryError {
    Bounds,
    NonFinite,
    Projection,
}
#[derive(Clone, Copy, Debug)]
struct Vertex {
    p: V,
    n: V,
    uv: [f64; 2],
}
struct Part {
    material: usize,
    vertices: Vec<Vertex>,
    group: u32,
    pivot: V,
    tint: [f32; 4],
}
#[derive(Default)]
struct Builder {
    parts: Vec<Part>,
    count: usize,
    failed: bool,
    count_only: bool,
    part_count: usize,
}
fn p(x: f64, y: f64, z: f64) -> V {
    V::new(x, y, z)
}
fn rot(angle: f64) -> DQuat {
    DQuat::from_rotation_z(angle)
}
impl Builder {
    fn count_part(&mut self, material: usize, count: usize) -> bool {
        if material >= MATERIALS
            || self.part_count >= MAX_PARTS
            || self.count.saturating_add(count) > MAX_VERTICES
        {
            self.failed = true;
            return false;
        }
        self.count += count;
        self.part_count += 1;
        true
    }
    fn append(&mut self, material: usize, vertices: Vec<Vertex>) {
        if !self.count_part(material, vertices.len()) || self.count_only {
            return;
        }
        self.parts.push(Part {
            material,
            vertices,
            group: 0,
            pivot: V::ZERO,
            tint: [1.; 4],
        });
    }
    fn quad(&mut self, material: usize, points: [V; 4], uv: [[f64; 2]; 4]) {
        if self.count_only {
            // Six is the conservative bound even for a collapsed corner.
            self.count_part(material, 6);
            return;
        }
        let mut v = Vec::with_capacity(6);
        for indices in [[0, 1, 2], [0, 2, 3]] {
            let [a, b, c] = indices.map(|i| points[i]);
            let normal = (b - a).cross(c - a).normalize_or_zero();
            if normal.length_squared() < 0.5 {
                continue;
            }
            for i in indices {
                v.push(Vertex {
                    p: points[i],
                    n: normal,
                    uv: uv[i],
                });
            }
        }
        self.append(material, v);
    }
    fn patch(&mut self, material: usize, rows: usize, columns: usize, f: impl Fn(f64, f64) -> V) {
        if rows == 0 || columns == 0 || rows > 256 || columns > 256 {
            self.failed = true;
            return;
        }
        if self.count_only {
            self.count_part(material, rows * columns * 6);
            return;
        }
        let mut v = Vec::with_capacity(rows * columns * 6);
        for r in 0..rows {
            for c in 0..columns {
                let u = c as f64 / columns as f64;
                let u1 = (c + 1) as f64 / columns as f64;
                let t = r as f64 / rows as f64;
                let t1 = (r + 1) as f64 / rows as f64;
                let uv = [[u, t], [u1, t], [u1, t1], [u, t1]];
                let ps = uv.map(|[u, t]| f(u, t));
                for ids in [[0, 1, 2], [0, 2, 3]] {
                    let [a, b, c] = ids.map(|i| ps[i]);
                    let normal = (b - a).cross(c - a).normalize_or_zero();
                    if normal.length_squared() < 0.5 {
                        continue;
                    }
                    for i in ids {
                        v.push(Vertex {
                            p: ps[i],
                            n: normal,
                            uv: uv[i],
                        });
                    }
                }
            }
        }
        self.append(material, v);
    }
    fn transform(&mut self, start: usize, rotation: DQuat, origin: V, scale: V) {
        for part in &mut self.parts[start..] {
            for v in &mut part.vertices {
                v.p = origin + rotation * (v.p * scale);
                v.n = (rotation * (v.n / scale)).normalize_or_zero();
            }
        }
    }
    fn rotate_about(&mut self, start: usize, angle: f64, center: V) {
        let q = rot(angle);
        for part in &mut self.parts[start..] {
            for v in &mut part.vertices {
                v.p = center + q * (v.p - center);
                v.n = q * v.n;
            }
        }
    }
    fn box_at(&mut self, at: V, size: V, material: usize) {
        let center = at + p(0., 0., size.z / 2.);
        let half = size / 2.;
        for (axis, u, v) in [
            (V::X, V::Y, V::Z),
            (-V::X, -V::Y, V::Z),
            (V::Y, -V::X, V::Z),
            (-V::Y, V::X, V::Z),
            (V::Z, V::X, V::Y),
            (-V::Z, -V::X, V::Y),
        ] {
            let w = half.dot(u.abs()) * 2.;
            let d = half.dot(v.abs()) * 2.;
            let base = center + axis * half.dot(axis.abs());
            self.patch(
                material,
                (d / 0.012).ceil().max(1.) as usize,
                (w / 0.012).ceil().max(1.) as usize,
                |x, y| base + u * ((x - 0.5) * w) + v * ((y - 0.5) * d),
            );
        }
    }
    fn beam(&mut self, a: V, b: V, width: f64, height: f64, material: usize) {
        let delta = b - a;
        if delta.length() < 1e-10 {
            return;
        }
        let start = self.parts.len();
        self.box_at(
            p(0., 0., -height / 2.),
            p(delta.length(), width, height),
            material,
        );
        self.transform(
            start,
            DQuat::from_rotation_arc(V::X, delta.normalize()),
            (a + b) / 2.,
            V::ONE,
        );
    }
    fn lathe(
        &mut self,
        profile: &[[f64; 2]],
        segments: usize,
        start: f64,
        arc: f64,
        material: usize,
    ) {
        for line in profile.windows(2) {
            let a = line[0];
            let b = line[1];
            self.patch(material, 1, segments, |u, t| {
                let angle = start + u * arc;
                let radius = a[0] + (b[0] - a[0]) * t;
                p(
                    angle.cos() * radius,
                    angle.sin() * radius,
                    a[1] + (b[1] - a[1]) * t,
                )
            });
        }
    }
    fn cylinder(&mut self, at: V, radius: f64, height: f64, material: usize, sides: usize) {
        let start = self.parts.len();
        self.lathe(
            &[
                [0., -height / 2.],
                [radius, -height / 2.],
                [radius, height / 2.],
                [0., height / 2.],
            ],
            sides,
            0.,
            TAU,
            material,
        );
        self.transform(start, DQuat::IDENTITY, at, V::ONE);
    }
    fn ring(&mut self, at: V, radius: f64, thickness: f64, material: usize, arc: f64) {
        self.torus(at, radius, thickness, material, [0., arc], DQuat::IDENTITY);
    }
    fn torus(
        &mut self,
        at: V,
        radius: f64,
        thickness: f64,
        material: usize,
        sweep: [f64; 2],
        rotation: DQuat,
    ) {
        let [start, arc] = sweep;
        let sides = if radius > 0.07 { 128 } else { 24 };
        let tubes = if radius > 0.07 { 6 } else { 4 };
        self.patch(material, tubes, sides, |u, t| {
            let theta = start + u * arc;
            let phi = t * TAU;
            at + rotation
                * p(
                    theta.cos() * (radius + thickness * phi.cos()),
                    theta.sin() * (radius + thickness * phi.cos()),
                    thickness * phi.sin(),
                )
        });
    }
    fn sphere(
        &mut self,
        at: V,
        radius: f64,
        scale: V,
        theta: f64,
        segments: usize,
        material: usize,
    ) {
        self.patch(material, segments / 2, segments, |u, t| {
            let a = u * TAU;
            let b = t * theta;
            at + p(-a.cos() * b.sin(), a.sin() * b.sin(), b.cos()) * radius * scale
        });
    }
    fn dome(&mut self, at: V, radius: f64, flatten: f64, material: usize) {
        let large = radius > 0.05;
        let segments = if large {
            64
        } else if radius > 0.01 {
            20
        } else {
            12
        };
        self.sphere(at, radius, p(1., 1., flatten), PI / 2., segments, material);
        let ribs = if radius < 0.006 {
            0
        } else if large {
            6
        } else {
            3
        };
        for i in 0..ribs {
            let start = self.parts.len();
            self.torus(
                V::ZERO,
                radius + 0.0005,
                0.0008,
                4,
                [0., PI],
                DQuat::from_rotation_x(PI / 2.),
            );
            self.transform(
                start,
                rot(i as f64 * PI / ribs as f64),
                at,
                p(1., 1., flatten),
            );
        }
    }
    fn tube(&mut self, points: &[V], radius: f64, segments: usize, sides: usize, material: usize) {
        if points.len() < 2 {
            return;
        }
        let curve = |t: f64| catmull(points, t);
        self.patch(material, sides, segments, |u, v| {
            let at = curve(u);
            let tangent =
                (curve((u + 0.0001).min(1.)) - curve((u - 0.0001).max(0.))).normalize_or_zero();
            let basis = if tangent.z.abs() < 0.9 { V::Z } else { V::Y };
            let a = tangent.cross(basis).normalize_or_zero();
            let b = a.cross(tangent).normalize_or_zero();
            at + (a * (v * TAU).cos() + b * (v * TAU).sin()) * radius
        });
    }
    fn facade(&mut self, b: &StructureBuilding, base: f64, width: f64, depth: f64, height: f64) {
        let start = self.parts.len();
        self.box_at(
            p(b.x, b.y, base),
            p(width, depth, height),
            b.facade as usize,
        );
        for part in &mut self.parts[start..] {
            for v in &mut part.vertices {
                let side = if v.n.x.abs() > 0.5 { depth } else { width };
                v.uv = [v.uv[0] * side / 0.036, v.uv[1] * height / 0.088];
            }
        }
        self.box_at(
            p(b.x, b.y, base + height),
            p(width + 0.001, depth + 0.001, 0.002),
            4,
        );
    }
}
/// Centripetal Catmull-Rom, matching the reference TubeGeometry default. The
/// extrapolated endpoints avoid a zero knot interval at either open end.
fn catmull(points: &[V], t: f64) -> V {
    let index =
        (t.clamp(0., 1.) * (points.len() - 1) as f64).min((points.len() - 1) as f64 - 1e-10);
    let i = index.floor() as usize;
    let f = index - i as f64;
    let p1 = points[i];
    let p2 = points[i + 1];
    let p0 = if i == 0 { p1 * 2. - p2 } else { points[i - 1] };
    let p3 = points.get(i + 2).copied().unwrap_or(p2 * 2. - p1);
    let dt0 = (p1 - p0).length_squared().powf(0.25).max(1e-4);
    let dt1 = (p2 - p1).length_squared().powf(0.25).max(1e-4);
    let dt2 = (p3 - p2).length_squared().powf(0.25).max(1e-4);
    let a = ((p1 - p0) / dt0 - (p2 - p0) / (dt0 + dt1) + (p2 - p1) / dt1) * dt1;
    let b = ((p2 - p1) / dt1 - (p3 - p1) / (dt1 + dt2) + (p3 - p2) / dt2) * dt1;
    p1 + a * f + (-p1 * 3. + p2 * 3. - a * 2. - b) * f * f + (p1 * 2. - p2 * 2. + a + b) * f * f * f
}
#[derive(Debug)]
pub struct MeshBatch {
    pub material: usize,
    pub positions: Vec<[f32; 3]>,
    pub normals: Vec<[f32; 3]>,
    pub uv: Vec<[f32; 2]>,
    pub colors: Vec<[f32; 4]>,
    pub frost: Vec<f32>,
    pub pivots: Vec<[f32; 3]>,
    pub collapse: Vec<[f32; 4]>,
}
impl MeshBatch {
    fn new(material: usize) -> Self {
        Self {
            material,
            positions: vec![],
            normals: vec![],
            uv: vec![],
            colors: vec![],
            frost: vec![],
            pivots: vec![],
            collapse: vec![],
        }
    }
    pub fn bytes(&self) -> usize {
        self.positions.len() * (3 + 3 + 2 + 4 + 1 + 3 + 4) * 4
    }
    /// Share only vertices with identical surface AND animation facts. Adjacent
    /// buildings never lose their independent collapse pivot by welding.
    pub fn indexed(&self, group_base: u32) -> Result<IndexedBatch, GeometryError> {
        let mut groups = Vec::new();
        let mut group_ids = HashMap::<[u32; 7], u32>::default();
        let mut vertices = HashMap::<[u32; 20], u32>::default();
        let mut indices = Vec::with_capacity(self.positions.len());
        let (mut positions, mut normals, mut uv, mut colors, mut animation) =
            (Vec::new(), Vec::new(), Vec::new(), Vec::new(), Vec::new());
        for i in 0..self.positions.len() {
            let mut key = [0u32; 20];
            for (n, value) in self.positions[i]
                .into_iter()
                .chain(self.normals[i])
                .chain(self.uv[i])
                .chain(self.colors[i])
                .chain([self.frost[i]])
                .chain(self.pivots[i])
                .chain(self.collapse[i])
                .enumerate()
            {
                if !value.is_finite() {
                    return Err(GeometryError::NonFinite);
                }
                key[n] = value.to_bits();
            }
            if let Some(&index) = vertices.get(&key) {
                indices.push(index);
                continue;
            }
            let group_key = std::array::from_fn(|n| {
                if n < 3 {
                    self.pivots[i][n].to_bits()
                } else {
                    self.collapse[i][n - 3].to_bits()
                }
            });
            let group = match group_ids.get(&group_key) {
                Some(&id) => id,
                None => {
                    let id = group_base
                        .checked_add(groups.len().try_into().map_err(|_| GeometryError::Bounds)?)
                        .filter(|id| *id < 16_777_216)
                        .ok_or(GeometryError::Bounds)?;
                    groups.push(GeometryGroup {
                        pivot: self.pivots[i],
                        motion: self.collapse[i],
                    });
                    group_ids.insert(group_key, id);
                    id
                }
            };
            let index = u32::try_from(positions.len()).map_err(|_| GeometryError::Bounds)?;
            vertices.insert(key, index);
            indices.push(index);
            positions.push(self.positions[i]);
            normals.push(self.normals[i]);
            uv.push(self.uv[i]);
            colors.push(self.colors[i]);
            animation.push([self.frost[i], group as f32]);
        }
        let bytes = (positions.len() * (3 + 3 + 2 + 4 + 2 + 4) * 4
            + indices.len() * 4
            + groups.len() * 7 * 4) as u64;
        let mut mesh = Mesh::new(
            PrimitiveTopology::TriangleList,
            RenderAssetUsages::MAIN_WORLD | RenderAssetUsages::RENDER_WORLD,
        )
        .with_inserted_attribute(Mesh::ATTRIBUTE_POSITION, positions)
        .with_inserted_attribute(Mesh::ATTRIBUTE_NORMAL, normals)
        .with_inserted_attribute(Mesh::ATTRIBUTE_UV_0, uv)
        .with_inserted_attribute(Mesh::ATTRIBUTE_UV_1, animation)
        .with_inserted_attribute(Mesh::ATTRIBUTE_COLOR, colors)
        .with_inserted_indices(Indices::U32(indices));
        tangents(&mut mesh)?;
        Ok(IndexedBatch {
            mesh,
            groups,
            bytes,
        })
    }
}
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct GeometryGroup {
    pub pivot: [f32; 3],
    pub motion: [f32; 4],
}
pub struct IndexedBatch {
    pub mesh: Mesh,
    pub groups: Vec<GeometryGroup>,
    pub bytes: u64,
}
#[derive(Debug)]
pub struct StructureGeometry {
    pub batches: Vec<MeshBatch>,
    pub emergence_depth: f32,
    pub bounds: [[f32; 3]; 2],
}
impl StructureGeometry {
    pub fn bytes(&self) -> usize {
        self.batches.iter().map(MeshBatch::bytes).sum()
    }
}
/// Local board-plane geometry is reusable at any tile coordinate. Projection
/// is deliberately separate from the source layout cache.
/// Count procedural work without allocating vertex buffers. Includes builder,
/// final arrays, index hash tables, transfer copies and finite part metadata.
pub fn working_bound(layout: &StructureLayout) -> Result<u64, GeometryError> {
    layout.validate().map_err(|_| GeometryError::Bounds)?;
    let mut builder = Builder {
        count_only: true,
        ..Default::default()
    };
    buildings(&mut builder, layout);
    fixtures(&mut builder, layout);
    perimeter(&mut builder, layout);
    details(&mut builder, layout);
    if builder.failed {
        return Err(GeometryError::Bounds);
    }
    (builder.count as u64)
        .checked_mul(320)
        .and_then(|n| n.checked_add(builder.part_count as u64 * 4096 + 1024 * 1024))
        .ok_or(GeometryError::Bounds)
}
pub fn generate(layout: &StructureLayout) -> Result<StructureGeometry, GeometryError> {
    layout.validate().map_err(|_| GeometryError::Bounds)?;
    let mut builder = Builder::default();
    buildings(&mut builder, layout);
    fixtures(&mut builder, layout);
    perimeter(&mut builder, layout);
    details(&mut builder, layout);
    finish(builder, layout)
}
fn color(hex: &str) -> [f32; 4] {
    let n = u32::from_str_radix(hex.trim_start_matches('#'), 16).unwrap_or(0xffffff);
    let mut out = [1.; 4];
    for (i, shift) in [16, 8, 0].into_iter().enumerate() {
        let v = ((n >> shift) & 255) as f32 / 255.;
        out[i] = if v <= 0.04045 {
            v / 12.92
        } else {
            ((v + 0.055) / 1.055).powf(2.4)
        }
    }
    out
}
fn finish(builder: Builder, layout: &StructureLayout) -> Result<StructureGeometry, GeometryError> {
    if builder.failed {
        return Err(GeometryError::Bounds);
    }
    let mut bounds: BTreeMap<u32, (V, V)> = BTreeMap::new();
    for part in &builder.parts {
        for vertex in &part.vertices {
            if !vertex.p.is_finite() || !vertex.n.is_finite() {
                return Err(GeometryError::NonFinite);
            }
            let b = bounds
                .entry(part.group)
                .or_insert((V::splat(f64::INFINITY), V::splat(f64::NEG_INFINITY)));
            b.0 = b.0.min(vertex.p);
            b.1 = b.1.max(vertex.p);
        }
    }
    let mut batches: Vec<MeshBatch> = (0..MATERIALS).map(MeshBatch::new).collect();
    let (mut low, mut high) = (V::splat(f64::INFINITY), V::splat(f64::NEG_INFINITY));
    for part in builder.parts {
        let b = bounds[&part.group];
        let depth = (b.1.z - part.pivot.z).max(0.) + (b.1 - b.0).length() * 0.12 + 0.08;
        let motion = collapse_params(u64::from(layout.seed) + u64::from(part.group) * 97);
        let batch = &mut batches[part.material];
        for v in part.vertices {
            low = low.min(v.p);
            high = high.max(v.p);
            let r = v.p.x.hypot(v.p.y);
            let mut exposure = 1.;
            if layout.style.cover != StructureCover::None && part.material != 10 {
                let radius = if layout.style.cover == StructureCover::FlatGlass
                    && layout.style.ground == StructureGround::Recessed
                {
                    0.054
                } else {
                    0.113
                };
                let roof = if layout.style.cover == StructureCover::Dome {
                    0.024 + (1. - (r / radius).powi(2)).max(0.).sqrt() * radius * 0.92
                } else if layout.style.ground == StructureGround::Recessed {
                    0.033
                } else {
                    0.025
                };
                if r < radius - 0.002 && v.p.z < roof - 0.001 {
                    exposure = 0.1;
                }
            }
            let mut uv = v.uv;
            if [3, 4, 5, 7, 13, 16].contains(&part.material) {
                let repeat = if part.material == 16 { 12. } else { 35. };
                uv = if v.n.z.abs() > 0.65 {
                    [v.p.x * repeat, v.p.y * repeat]
                } else if v.n.x.abs() > v.n.y.abs() {
                    [v.p.y * repeat, v.p.z * repeat]
                } else {
                    [v.p.x * repeat, v.p.z * repeat]
                };
            }
            batch.positions.push(v.p.as_vec3().to_array());
            batch.normals.push(v.n.as_vec3().to_array());
            batch.uv.push(uv.map(|n| n as f32));
            batch.colors.push(part.tint);
            batch.frost.push(exposure);
            batch.pivots.push(part.pivot.as_vec3().to_array());
            batch.collapse.push([
                motion.delay_s,
                motion.direction_rad.cos() * motion.tilt_rad,
                motion.direction_rad.sin() * motion.tilt_rad,
                depth as f32,
            ]);
        }
    }
    batches.retain(|b| !b.positions.is_empty());
    if !low.is_finite() || !high.is_finite() {
        return Err(GeometryError::Bounds);
    }
    Ok(StructureGeometry {
        batches,
        emergence_depth: (high.z
            + f64::from(openmars_scene::geometry::terrain::RELIEF_DEPTH)
            + 0.015)
            .max(0.08) as f32,
        bounds: [low.as_vec3().to_array(), high.as_vec3().to_array()],
    })
}
impl StructureGeometry {
    pub fn project(
        &mut self,
        projection: Projection,
        coordinate: AxialPoint,
    ) -> Result<(), GeometryError> {
        let center = projection.hex_center(coordinate);
        // Unindexed triangles repeat corners, and every building repeats its
        // collapse pivot. Preserve the exact public projection while evaluating
        // each distinct input only once during this placement.
        let mut projected = HashMap::<[u32; 3], [f32; 3]>::default();
        let mut project = |position: [f32; 3]| -> Result<[f32; 3], GeometryError> {
            let key = position.map(f32::to_bits);
            if let Some(&point) = projected.get(&key) {
                return Ok(point);
            }
            let point = projection
                .project_content(
                    Point {
                        x: center.x + f64::from(position[0]),
                        y: center.y + f64::from(position[1]),
                    },
                    f64::from(position[2]) - 0.007,
                )
                .map_err(|_| GeometryError::Projection)?
                .map(|n| n as f32);
            projected.insert(key, point);
            Ok(point)
        };
        for batch in &mut self.batches {
            for (position, pivot) in batch.positions.iter_mut().zip(&mut batch.pivots) {
                *position = project(*position)?;
                *pivot = project(*pivot)?;
            }
            for (points, normals) in batch
                .positions
                .as_chunks::<3>()
                .0
                .iter()
                .zip(batch.normals.as_chunks_mut::<3>().0.iter_mut())
            {
                let [a, b, c] = [0, 1, 2].map(|i| V::from_array(points[i].map(f64::from)));
                let n = (b - a)
                    .cross(c - a)
                    .normalize_or_zero()
                    .as_vec3()
                    .to_array();
                normals.fill(n);
            }
        }
        Ok(())
    }
}
fn buildings(g: &mut Builder, l: &StructureLayout) {
    let mut rng = openmars_scene::geometry::placement::random_sequence(7321);
    let mut dishes = l
        .buildings
        .iter()
        .enumerate()
        .map(|(i, b)| (i, b, rng()))
        .filter(|(i, b, _)| !b.round && i % 2 == 1)
        .collect::<Vec<_>>();
    dishes.sort_by(|a, b| a.2.total_cmp(&b.2));
    let selected = dishes
        .iter()
        .take((dishes.len() as f64 * 0.7).round() as usize)
        .map(|(i, _, _)| *i)
        .collect::<std::collections::BTreeSet<_>>();
    for (index, b) in l.buildings.iter().enumerate() {
        let start = g.parts.len();
        let at = p(b.x, b.y, b.z);
        let base = b.z + 0.005;
        match (b.landmark, l.style.landmark) {
            (true, StructureLandmark::Headquarters) => {
                g.box_at(at, p(b.width, b.depth, 0.003), 5);
                for (x, y, z, w, d, h) in [
                    (0., 0.026, 0., 0.116, 0.046, 0.03),
                    (-0.009, 0.029, 0.032, 0.085, 0.035, 0.024),
                    (-0.025, 0.032, 0.058, 0.033, 0.027, b.height - 0.058),
                    (-0.046, -0.024, 0., 0.027, 0.054, 0.024),
                    (0.046, -0.014, 0., 0.025, 0.036, 0.035),
                ] {
                    let mut office = b.clone();
                    office.x += x;
                    office.y += y;
                    office.facade = 2;
                    g.facade(&office, base + z, w, d, h);
                    g.box_at(
                        p(office.x, office.y, base + z + h + 0.002),
                        p(w * 0.9, d * 0.88, 0.0015),
                        17,
                    );
                }
                for x in [-0.056, -0.026, 0.017, 0.055] {
                    g.box_at(p(b.x + x, b.y + 0.002, base), p(0.003, 0.004, 0.031), 18);
                }
                for (x, y, z, w, d, h, m) in [
                    (-0.004, 0.001, 0.002, 0.026, 0.003, 0.016, 8),
                    (-0.004, -0.002, 0.018, 0.034, 0.009, 0.002, 17),
                    (-0.004, -0.006, 0.017, 0.018, 0.001, 0.001, 11),
                ] {
                    g.box_at(p(b.x + x, b.y + y, base + z), p(w, d, h), m);
                }
                g.box_at(
                    p(b.x - 0.004, b.y - 0.025, b.z + 0.003),
                    p(0.017, 0.051, 0.001),
                    4,
                );
                for (x, y, z) in [
                    (0.028, 0.03, 0.034),
                    (-0.045, -0.027, 0.028),
                    (0.046, -0.015, 0.039),
                ] {
                    g.box_at(p(b.x + x, b.y + y, base + z), p(0.008, 0.011, 0.004), 5);
                }
            }
            (true, StructureLandmark::Hall) => {
                g.cylinder(at + p(0., 0., 0.0025), 0.031, 0.005, 4, 48);
                let corners = [
                    p(-0.022, -0.022, 0.),
                    p(0.022, -0.022, 0.),
                    p(0.022, 0.022, 0.),
                    p(-0.022, 0.022, 0.),
                ];
                for side in 0..4 {
                    let a = corners[side];
                    let c = corners[(side + 1) % 4];
                    let point = |t: f64, z: f64| {
                        let xy = a.lerp(c, t) * (1. - z / (b.height * 1.035));
                        p(xy.x + b.x, xy.y + b.y, z + base)
                    };
                    let (left, right, shoulder, tip) = (
                        point(0.02, 0.),
                        point(0.98, 0.),
                        point(0.98, b.height * (0.85 + side as f64 * 0.012)),
                        point(0.08, b.height * [1., 0.945, 0.98, 0.915][side]),
                    );
                    g.quad(
                        9,
                        [left, right, shoulder, tip],
                        [[0., 0.], [1., 0.], [1., 1.], [0., 1.]],
                    );
                    for (a, c) in [(left, tip), (right, shoulder)] {
                        g.beam(a, c, 0.00045, 0.00045, 17);
                    }
                    let mut z = 0.003;
                    while z < b.height * 0.83 {
                        g.beam(point(0.025, z), point(0.975, z), 0.00018, 0.00025, 17);
                        z += 0.0035;
                    }
                    for t in [0.25, 0.5, 0.75] {
                        g.beam(point(t, 0.), point(t, b.height * 0.84), 0.0002, 0.0002, 17);
                    }
                    let top = point(0.5, b.height * 0.86);
                    g.quad(
                        8,
                        [point(0., 0.), point(1., 0.), top, top],
                        [[0., 0.], [1., 0.], [1., 1.], [0., 1.]],
                    );
                }
            }
            (true, StructureLandmark::Globe) => {
                let radius = b.width * 0.42;
                let center = base + 0.006 + radius;
                g.cylinder(p(b.x, b.y, base + 0.002), b.width / 2., 0.005, 17, 48);
                g.cylinder(p(b.x, b.y, base + 0.005), radius * 0.6, 0.006, 4, 32);
                for (scale, m) in [(0.975, 9), (1., 10)] {
                    g.sphere(p(b.x, b.y, center), radius * scale, V::ONE, PI, 40, m);
                }
                for i in 0..6 {
                    g.torus(
                        p(b.x, b.y, center),
                        radius + 0.0004,
                        0.0005,
                        17,
                        [0., TAU],
                        rot(i as f64 * PI / 6.) * DQuat::from_rotation_x(PI / 2.),
                    );
                }
                for latitude in [-0.5_f64, 0., 0.5] {
                    g.ring(
                        p(b.x, b.y, center + radius * latitude),
                        radius * (1. - latitude * latitude).sqrt() + 0.0004,
                        0.0005,
                        17,
                        TAU,
                    );
                }
            }
            _ => {
                g.box_at(at, p(b.width + 0.002, b.depth + 0.002, 0.005), 5);
                g.quad(
                    14,
                    [
                        p(
                            b.x - b.width / 2. - 0.007,
                            b.y - b.depth / 2. - 0.007,
                            b.z + 0.0008,
                        ),
                        p(
                            b.x + b.width / 2. + 0.007,
                            b.y - b.depth / 2. - 0.007,
                            b.z + 0.0008,
                        ),
                        p(
                            b.x + b.width / 2. + 0.007,
                            b.y + b.depth / 2. + 0.007,
                            b.z + 0.0008,
                        ),
                        p(
                            b.x - b.width / 2. - 0.007,
                            b.y + b.depth / 2. + 0.007,
                            b.z + 0.0008,
                        ),
                    ],
                    [[0., 0.], [1., 0.], [1., 1.], [0., 1.]],
                );
                if b.landmark && l.style.landmark == StructureLandmark::Factory {
                    g.box_at(p(b.x, b.y, base), p(b.width, b.depth, b.height), 17);
                    for side in [-1., 1.] {
                        for rib in 1..7 {
                            g.box_at(
                                p(
                                    b.x - b.width / 2. + rib as f64 * b.width / 7.,
                                    b.y + side * b.depth / 2.,
                                    base,
                                ),
                                p(0.001, 0.001, b.height),
                                5,
                            );
                        }
                    }
                    let roof = base + b.height + 0.002;
                    let bay = b.depth / 3.;
                    for i in 0..3 {
                        let y = b.y - b.depth / 2. + bay * (i as f64 + 0.5);
                        for side in [-1., 1.] {
                            let start = g.parts.len();
                            g.box_at(
                                p(0., 0., -0.00075),
                                p(b.width + 0.002, bay * 0.57, 0.0015),
                                17,
                            );
                            g.transform(
                                start,
                                DQuat::from_rotation_x(side * 0.35),
                                p(b.x, y + side * bay * 0.24, roof + 0.002),
                                V::ONE,
                            );
                        }
                        g.box_at(p(b.x, y, roof + 0.004), p(b.width * 0.75, 0.002, 0.001), 9);
                    }
                    for side in [-1., 1.] {
                        g.box_at(
                            p(
                                b.x + side * b.width * 0.25,
                                b.y - b.depth / 2. - 0.001,
                                base,
                            ),
                            p(b.width * 0.23, 0.001, b.height * 0.64),
                            8,
                        );
                    }
                } else if b.habitat {
                    g.cylinder(p(b.x, b.y, base - 0.001), b.width * 0.53, 0.003, 17, 24);
                    g.sphere(
                        p(b.x, b.y, base + b.height * 0.35),
                        1.,
                        p(b.width * 0.52, b.depth * 0.52, b.height * 0.72),
                        PI * 0.72,
                        16,
                        18,
                    );
                    g.dome(p(b.x, b.y, base + b.height * 0.88), b.width * 0.22, 0.5, 9);
                    g.ring(
                        p(b.x, b.y, base + b.height * 0.35),
                        b.width * 0.52,
                        0.0005,
                        17,
                        TAU,
                    );
                    g.box_at(
                        p(b.x, b.y - b.depth * 0.48, base),
                        p(b.width * 0.35, 0.004, 0.008),
                        17,
                    );
                    g.box_at(
                        p(b.x, b.y - b.depth * 0.51, base + 0.001),
                        p(b.width * 0.23, 0.001, 0.005),
                        8,
                    );
                } else if b.round {
                    let start = g.parts.len();
                    g.cylinder(
                        p(b.x, b.y, base + b.height / 2.),
                        b.width / 2.,
                        b.height,
                        1,
                        20,
                    );
                    for part in &mut g.parts[start..] {
                        for v in &mut part.vertices {
                            v.uv = [v.uv[0] * PI * b.width / 0.036, v.uv[1] * b.height / 0.088];
                        }
                    }
                    g.cylinder(
                        p(b.x, b.y, base + b.height),
                        b.width / 2. + 0.003,
                        0.004,
                        4,
                        16,
                    );
                    if b.landmark || index % 3 == 0 {
                        g.dome(p(b.x, b.y, base + b.height + 0.002), b.width / 2., 0.8, 9);
                    } else {
                        g.cylinder(
                            p(b.x, b.y, base + b.height + 0.003),
                            b.width * 0.4,
                            0.003,
                            5,
                            16,
                        );
                        g.cylinder(
                            p(b.x, b.y, base + b.height + 0.006),
                            b.width * 0.18,
                            0.004,
                            8,
                            16,
                        );
                    }
                    g.box_at(p(b.x, b.y - b.width / 2., base), p(0.006, 0.001, 0.007), 8);
                } else {
                    let (mut w, mut d, mut roof) = (b.width, b.depth, base + b.height + 0.002);
                    if b.stepped && b.height > 0.045 {
                        let lower = (b.height * 0.55 / 0.011).round() * 0.011;
                        g.facade(b, base, w, d, lower);
                        w *= 0.7;
                        d *= 0.7;
                        g.facade(b, base + 0.002 + lower, w, d, b.height - lower);
                        roof += 0.002;
                    } else {
                        g.facade(b, base, w, d, b.height);
                    }
                    for (at, size, m) in [
                        (p(b.x, b.y, roof), p(w * 0.75, d * 0.75, 0.001), 5),
                        (
                            p(b.x - w * 0.16, b.y, roof + 0.001),
                            p(w * 0.35, d * 0.58, 0.002),
                            8,
                        ),
                        (
                            p(b.x + w * 0.26, b.y, roof + 0.001),
                            p(w * 0.18, d * 0.3, 0.003),
                            3,
                        ),
                        (
                            p(b.x, b.y - b.depth / 2. - 0.002, base + 0.007),
                            p(b.width * 0.35, 0.008, 0.002),
                            6,
                        ),
                        (
                            p(b.x, b.y - b.depth / 2. - 0.001, base),
                            p(0.007, 0.001, 0.007),
                            8,
                        ),
                    ] {
                        g.box_at(at, size, m);
                    }
                    if l.style.details.contains(&StructureDetail::Antennas) && index % 2 == 0 {
                        g.cylinder(p(b.x + w * 0.28, b.y, roof + 0.015), 0.0007, 0.029, 4, 8);
                        g.box_at(
                            p(b.x + w * 0.28, b.y, roof + 0.024),
                            p(0.012, 0.001, 0.001),
                            6,
                        );
                    }
                    if l.style.details.contains(&StructureDetail::Dishes)
                        && selected.contains(&index)
                    {
                        dish(g, p(b.x, b.y, roof), index);
                    }
                    if l.style.details.contains(&StructureDetail::Vents) {
                        for i in 0..3 {
                            let x = b.x - 0.008 + i as f64 * 0.008;
                            g.cylinder(p(x, b.y, roof + 0.006), 0.002, 0.012, 3, 12);
                            g.cylinder(p(x, b.y, roof + 0.012), 0.003, 0.002, 6, 12);
                        }
                    }
                }
            }
        }
        g.rotate_about(start, b.rotation, p(b.x, b.y, 0.));
        for part in &mut g.parts[start..] {
            part.group = index as u32 + 1;
            part.pivot = at;
            if part.material < 3 || part.material == 18 {
                part.tint = color(&b.tint);
            }
        }
        if let Some(target) = b.entrance_road {
            let to = p(target.x, target.y, b.z + 0.0004);
            let center = p(b.x, b.y, b.z + 0.0004);
            let direction = (to - center).normalize_or_zero();
            let from = center + direction * b.depth / 2.;
            let end = to - direction * (target.half_width + 0.0005);
            if (end - from).dot(direction) > 0. {
                g.beam(from, end, 0.004, 0.0008, 4);
            }
        }
    }
}
fn dish(g: &mut Builder, at: V, index: usize) {
    g.cylinder(at + p(0., 0., 0.007), 0.0012, 0.014, 17, 12);
    let mut random = openmars_scene::geometry::placement::random_sequence(931 + index as u32 * 137);
    let angle = random() * TAU;
    let tilt = 0.25 + random() * 0.6;
    let direction = p(
        tilt.sin() * angle.cos(),
        tilt.sin() * angle.sin(),
        tilt.cos(),
    );
    let q = DQuat::from_rotation_arc(V::Z, direction);
    let origin = at + p(0., 0., 0.014);
    let start = g.parts.len();
    g.lathe(
        &[
            [0., 0.],
            [0.003, 0.0004],
            [0.006, 0.0016],
            [0.009, 0.0036],
            [0.012, 0.0064],
        ],
        40,
        0.,
        TAU,
        18,
    );
    g.ring(p(0., 0., 0.0064), 0.012, 0.0005, 17, TAU);
    g.transform(start, q, origin, V::ONE);
    let receiver = origin + q * p(0., 0., 0.014);
    for i in 0..3 {
        let angle = i as f64 * TAU / 3.;
        let anchor = origin + q * p(angle.cos() * 0.011, angle.sin() * 0.011, 0.0055);
        g.beam(anchor, receiver, 0.0005, 0.0005, 17);
    }
    g.sphere(receiver, 0.0012, V::ONE, PI, 8, 5);
}
fn fixtures(g: &mut Builder, l: &StructureLayout) {
    for (index, f) in l.fixtures.iter().enumerate() {
        let start = g.parts.len();
        let at = p(f.x, f.y, f.z);
        let (r, h) = (f.radius, f.height);
        match f.kind {
            StructureFixtureKind::Smokestack => {
                g.cylinder(at + p(0., 0., h / 2.), r, h, 5, 16);
                for t in [0.15, 0.8] {
                    g.ring(at + p(0., 0., h * t), r + 0.0004, 0.0007, 17, TAU);
                }
                g.cylinder(at + p(0., 0., h), r * 0.75, 0.0005, 8, 16);
                g.ring(at + p(0., 0., h + 0.0003), r * 0.9, 0.001, 17, TAU);
            }
            StructureFixtureKind::Tank => {
                g.cylinder(at + p(0., 0., 0.001), r + 0.002, 0.002, 4, 20);
                g.cylinder(at + p(0., 0., h / 2.), r, h, 17, 20);
                g.dome(at + p(0., 0., h), r, 0.28, 17);
                g.ring(at + p(0., 0., h * 0.2), r + 0.0002, 0.0005, 5, TAU);
            }
            StructureFixtureKind::GroundExhaust => {
                let angle = f.rotation.unwrap_or(0.);
                let direction = p(angle.cos(), angle.sin(), 0.);
                g.cylinder(at + p(0., 0., 0.0015), r, 0.003, 4, 16);
                g.ring(at + p(0., 0., 0.003), r * 0.6, 0.001, 17, TAU);
                let mouth = at + p(0., 0., h) + direction * r * 0.8;
                g.tube(
                    &[at + p(0., 0., 0.002), at + p(0., 0., h * 0.7), mouth],
                    r * 0.34,
                    12,
                    10,
                    17,
                );
                let q = DQuat::from_rotation_arc(V::Z, direction);
                g.torus(mouth, r * 0.35, 0.0007, 5, [0., TAU], q);
                for (scale, m) in [(0.34, 8), (0.12, 11)] {
                    let start = g.parts.len();
                    g.lathe(&[[0., 0.], [r * scale, 0.]], 16, 0., TAU, m);
                    g.transform(
                        start,
                        q,
                        mouth + direction * 0.0001 * if m == 11 { 2. } else { 1. },
                        V::ONE,
                    );
                }
            }
            StructureFixtureKind::Shaft => {
                g.cylinder(at + p(0., 0., h / 2.), r, h, 5, 24);
                g.cylinder(at + p(0., 0., h), r * 0.82, 0.0005, 8, 24);
                g.ring(at + p(0., 0., h + 0.0005), r * 0.9, 0.001, 17, TAU);
                g.cylinder(at + p(0., 0., h + 0.001), r * 0.17, 0.002, 17, 12);
                for blade in 0..5 {
                    let start = g.parts.len();
                    g.box_at(p(0., 0., -0.0004), p(r * 0.57, r * 0.22, 0.0008), 17);
                    g.transform(start, rot(0.5), p(r * 0.48, 0., 0.), V::ONE);
                    g.transform(
                        start,
                        rot(blade as f64 * TAU / 5.),
                        at + p(0., 0., h + 0.0008),
                        V::ONE,
                    );
                }
            }
            StructureFixtureKind::AccentLight => {
                let start = g.parts.len();
                g.box_at(
                    p(0., 0., -r),
                    p(h, r, r * 2.),
                    if f.accent == Some(FixtureAccent::Cyan) {
                        9
                    } else {
                        11
                    },
                );
                g.transform(start, rot(f.rotation.unwrap_or(0.)), at, V::ONE);
            }
        }
        for part in &mut g.parts[start..] {
            part.group = (l.buildings.len() + index + 1) as u32;
            part.pivot = at;
        }
    }
    for pipe in &l.pipes {
        let points = pipe
            .points
            .iter()
            .map(|p| V::new(p.x, p.y, p.z))
            .collect::<Vec<_>>();
        g.tube(&points, pipe.radius, 8, 8, 17);
        for t in [0.15, 0.5, 0.85] {
            let point = catmull(&points, t);
            let tangent =
                (catmull(&points, t + 0.0001) - catmull(&points, t - 0.0001)).normalize_or_zero();
            g.torus(
                point,
                pipe.radius + 0.0004,
                0.0004,
                6,
                [0., TAU],
                DQuat::from_rotation_arc(V::Z, tangent),
            );
            g.box_at(
                p(point.x, point.y, 0.007),
                p(0.004, 0.003, (point.z - 0.007).max(0.001)),
                5,
            );
        }
    }
    if l.style.connections != StructureConnections::Paths {
        let buildings = l
            .buildings
            .iter()
            .filter(|b| !b.landmark)
            .collect::<Vec<_>>();
        let stride = if l.style.connections == StructureConnections::Skybridges {
            6
        } else {
            2
        };
        for i in (0..buildings.len().saturating_sub(1)).step_by(stride) {
            let (a, b) = (buildings[i], buildings[i + 1]);
            let (mut from, mut to) = (p(a.x, a.y, a.z + 0.008), p(b.x, b.y, b.z + 0.008));
            if l.style.connections == StructureConnections::Skybridges {
                let z = ((a.z + a.height).min(b.z + b.height) * 0.6 + 0.012)
                    .max(a.z + 0.01)
                    .max(b.z + 0.01);
                from.z = z;
                to.z = z;
            }
            g.beam(from, to, 0.01, 0.003, 4);
            let perpendicular = p(-(b.y - a.y), b.x - a.x, 0.).normalize_or_zero() * 0.0045;
            for side in [-1., 1.] {
                g.beam(
                    from + perpendicular * side + p(0., 0., 0.004),
                    to + perpendicular * side + p(0., 0., 0.004),
                    0.001,
                    0.006,
                    8,
                );
            }
            if l.style.connections == StructureConnections::Enclosed {
                g.beam(
                    from + p(0., 0., 0.008),
                    to + p(0., 0., 0.008),
                    0.012,
                    0.002,
                    4,
                );
            }
        }
    }
}
fn perimeter(g: &mut Builder, l: &StructureLayout) {
    let has_wall =
        l.style.perimeter != StructurePerimeter::Open && l.style.cover != StructureCover::Dome;
    if has_wall {
        if let Some(apothem) = wall_apothem(l) {
            let height = if l.style.perimeter == StructurePerimeter::HighWall {
                0.025
            } else {
                0.012
            };
            let gatehouses = l.style.details.contains(&StructureDetail::Gatehouses);
            let half = apothem * (PI / 6.).tan();
            for side in 0..6 {
                let angle = side as f64 * PI / 3.;
                let radial = p(angle.cos(), angle.sin(), 0.);
                let tangent = p(-radial.y, radial.x, 0.);
                let gate = l.entrances.iter().any(|e| (e - angle).abs() < 1e-6);
                let spans = if gate {
                    vec![(-half, -0.01), (0.01, half)]
                } else {
                    vec![(-half, half)]
                };
                for (a, b) in spans {
                    for (width, z, thickness, m) in [
                        (if gatehouses { 0.007 } else { 0.004 }, 0.007, height, 4),
                        (
                            if gatehouses { 0.008 } else { 0.005 },
                            0.007 + height,
                            0.0015,
                            17,
                        ),
                    ] {
                        let points = [(a, -1.), (b, -1.), (b, 1.), (a, 1.)].map(|(along, edge)| {
                            let reach = apothem + edge * width / 2.;
                            let end = if (along.abs() - half).abs() < 1e-8 {
                                along.signum() * reach * (PI / 6.).tan()
                            } else {
                                along
                            };
                            radial * reach + tangent * end
                        });
                        prism(g, &points, z, thickness, m);
                    }
                }
                if gate && gatehouses {
                    let start = g.parts.len();
                    for offset in [-0.019, 0.019] {
                        for (at, size, m) in [
                            (
                                p(apothem - 0.007, offset, 0.007),
                                p(0.024, 0.018, height + 0.009),
                                5,
                            ),
                            (
                                p(apothem - 0.007, offset, 0.007 + height + 0.009),
                                p(0.026, 0.02, 0.003),
                                17,
                            ),
                            (
                                p(apothem + 0.0051, offset, 0.018),
                                p(0.0008, 0.011, 0.006),
                                8,
                            ),
                            (
                                p(apothem + 0.006, offset.signum() * 0.011, 0.014),
                                p(0.001, 0.001, 0.005),
                                11,
                            ),
                        ] {
                            g.box_at(at, size, m);
                        }
                    }
                    g.box_at(
                        p(apothem - 0.007, 0., 0.007 + height),
                        p(0.022, 0.028, 0.007),
                        17,
                    );
                    g.rotate_about(start, angle, V::ZERO);
                    for part in &mut g.parts[start..] {
                        part.group = (l.buildings.len() + l.fixtures.len() + side + 1) as u32;
                        part.pivot = radial * apothem + p(0., 0., 0.007);
                    }
                } else if gate {
                    for offset in [-0.011, 0.011] {
                        let at = radial * apothem + tangent * offset;
                        g.box_at(p(at.x, at.y, 0.007), p(0.004, 0.004, height + 0.004), 17);
                    }
                    g.beam(
                        radial * apothem - tangent * 0.011 + p(0., 0., 0.007 + height + 0.003),
                        radial * apothem + tangent * 0.011 + p(0., 0., 0.007 + height + 0.003),
                        0.005,
                        0.003,
                        17,
                    );
                }
            }
        } else {
            let height = if l.style.perimeter == StructurePerimeter::HighWall {
                0.025
            } else {
                0.01
            };
            let inner = 0.118;
            let outer = 0.124;
            let gap = (0.009_f64 / inner).asin();
            let mut entrances = l.entrances.clone();
            entrances.sort_by(f64::total_cmp);
            for (i, &current) in entrances.iter().enumerate() {
                let next = entrances.get(i + 1).copied().unwrap_or(entrances[0] + TAU);
                let start = current + gap;
                let arc = next - current - gap * 2.;
                let profile = [
                    [inner, 0.007],
                    [outer, 0.007],
                    [outer, 0.007 + height],
                    [inner, 0.007 + height],
                    [inner, 0.007],
                ];
                g.lathe(&profile, 64, start, arc, 4);
                g.torus(
                    p(0., 0., 0.007 + height),
                    0.121,
                    0.0015,
                    17,
                    [start, arc],
                    DQuat::IDENTITY,
                );
                for angle in [start, start + arc] {
                    let q = rot(angle);
                    g.quad(
                        4,
                        [
                            q * p(inner, 0., 0.007),
                            q * p(outer, 0., 0.007),
                            q * p(outer, 0., 0.007 + height),
                            q * p(inner, 0., 0.007 + height),
                        ],
                        [[0., 0.], [1., 0.], [1., 1.], [0., 1.]],
                    );
                }
                let radial = p(current.cos(), current.sin(), 0.);
                let tangent = p(-radial.y, radial.x, 0.);
                for side in [-1., 1.] {
                    let at = radial * 0.121 + tangent * side * 0.01;
                    g.box_at(p(at.x, at.y, 0.007), p(0.004, 0.004, height + 0.002), 17);
                }
                let from = radial * 0.121 - tangent * 0.012 + p(0., 0., 0.007 + height + 0.002);
                let to = radial * 0.121 + tangent * 0.012 + p(0., 0., 0.007 + height + 0.002);
                g.beam(from, to, 0.005, 0.003, 17);
                g.beam(
                    from + tangent * 0.005 - p(0., 0., 0.002),
                    to - tangent * 0.005 - p(0., 0., 0.002),
                    0.002,
                    0.0007,
                    11,
                );
            }
        }
    }
    match l.style.cover {
        StructureCover::Dome => {
            g.lathe(
                &[
                    [0.111, 0.007],
                    [0.117, 0.007],
                    [0.117, 0.024],
                    [0.111, 0.024],
                    [0.111, 0.007],
                ],
                192,
                0.,
                TAU,
                4,
            );
            g.dome(p(0., 0., 0.024), 0.113, 0.92, 10);
            g.ring(p(0., 0., 0.024), 0.114, 0.0012, 6, TAU);
            g.ring(p(0., 0., 0.076), 0.098, 0.0007, 4, TAU);
            for &angle in &l.entrances {
                let start = g.parts.len();
                g.box_at(p(0.114, 0., 0.007), p(0.025, 0.024, 0.022), 4);
                g.box_at(p(0.127, 0., 0.01), p(0.001, 0.015, 0.016), 8);
                g.box_at(p(0.128, 0., 0.027), p(0.002, 0.018, 0.001), 11);
                g.rotate_about(start, angle, V::ZERO);
            }
        }
        StructureCover::FlatGlass if l.style.landmark != StructureLandmark::Hall => {
            let radius = if l.style.ground == StructureGround::Recessed {
                0.054
            } else {
                0.113
            };
            let z = if l.style.ground == StructureGround::Recessed {
                0.033
            } else {
                0.025
            };
            g.lathe(&[[radius, z], [0., z]], 96, 0., TAU, 10);
            g.ring(p(0., 0., z + 0.001), radius, 0.002, 17, TAU);
            for i in -2..=2 {
                let offset = i as f64 * radius / 3.;
                let reach = (radius * radius - offset * offset).sqrt();
                g.box_at(p(offset, 0., z), p(0.002, reach * 2., 0.003), 17);
                g.box_at(p(0., offset, z), p(reach * 2., 0.002, 0.003), 17);
                for j in -2..=2 {
                    let y = j as f64 * radius / 3.;
                    if offset.hypot(y) < radius - 0.004 {
                        g.box_at(p(offset, y, z + 0.003), p(0.004, 0.004, 0.001), 5);
                        g.cylinder(p(offset, y, z + 0.004), 0.0008, 0.001, 17, 8);
                    }
                }
            }
            for i in 0..12 {
                let angle = i as f64 * TAU / 12.;
                let at = p(
                    angle.cos() * (radius + 0.002),
                    angle.sin() * (radius + 0.002),
                    z,
                );
                g.cylinder(at + p(0., 0., 0.002), 0.0025, 0.002, 5, 12);
                g.cylinder(at + p(0., 0., 0.0035), 0.001, 0.001, 17, 8);
            }
            g.ring(p(0., 0., z - 0.006), radius - 0.005, 0.0006, 11, TAU);
        }
        _ => {}
    }
    if l.style.landmark == StructureLandmark::Hall {
        let roof = |angle: f64, t: f64| {
            let (x, y) = (angle.cos(), angle.sin());
            let outer = CAPITAL_WALL_APOTHEM
                / x.abs()
                    .max((x * 0.5 + y * 3_f64.sqrt() / 2.).abs())
                    .max((x * 0.5 - y * 3_f64.sqrt() / 2.).abs());
            let hole = 0.018 / x.abs().max(y.abs());
            let r = hole + (outer - hole) * t;
            p(x * r, y * r, 0.05 + 0.01 * (1. - t * t))
        };
        g.patch(10, 10, 96, |u, t| roof(u * TAU, t));
        g.patch(10, 1, 96, |u, t| {
            let mut v = roof(u * TAU, 1.);
            v.z = v.z + (0.019 - v.z) * t;
            v
        });
        for rib in 0..6 {
            let angle = PI / 6. + rib as f64 * PI / 3.;
            for step in 0..10 {
                g.beam(
                    roof(angle, step as f64 / 10.),
                    roof(angle, (step + 1) as f64 / 10.),
                    0.00065,
                    0.00065,
                    17,
                );
            }
            let corner = roof(angle, 1.);
            g.beam(corner, p(corner.x, corner.y, 0.019), 0.0007, 0.0007, 17);
            g.beam(corner, roof(angle + PI / 3., 1.), 0.0008, 0.0008, 17);
        }
        for side in 0..4 {
            let angle = PI / 4. + side as f64 * PI / 2.;
            g.beam(roof(angle, 0.), roof(angle + PI / 2., 0.), 0.001, 0.001, 17);
        }
    }
}
fn prism(g: &mut Builder, points: &[V; 4], z: f64, height: f64, material: usize) {
    let uv = [[0., 0.], [1., 0.], [1., 1.], [0., 1.]];
    g.quad(material, points.map(|v| v + p(0., 0., z + height)), uv);
    g.quad(
        material,
        [points[3], points[2], points[1], points[0]].map(|v| v + p(0., 0., z)),
        uv,
    );
    for i in 0..4 {
        let (a, b) = (points[i], points[(i + 1) % 4]);
        g.quad(
            material,
            [
                a + p(0., 0., z),
                b + p(0., 0., z),
                b + p(0., 0., z + height),
                a + p(0., 0., z + height),
            ],
            uv,
        );
    }
}
fn nearest(point: V, roads: &[RoadPath]) -> (f64, f64) {
    let mut best = (f64::INFINITY, 0.);
    for road in roads {
        for line in road.points.windows(2) {
            let a = p(line[0].x, line[0].y, 0.);
            let b = p(line[1].x, line[1].y, 0.);
            let d = b - a;
            let t = if d.length_squared() > 0. {
                ((point - a).dot(d) / d.length_squared()).clamp(0., 1.)
            } else {
                0.
            };
            let distance = (point - (a + d * t)).length();
            if distance - road.width / 2. < best.0 - best.1 {
                best = (distance, road.width / 2.);
            }
        }
    }
    best
}
fn entrance(b: &StructureBuilding, point: V, padding: f64) -> bool {
    let Some(e) = b.entrance_road else {
        return false;
    };
    let a = p(b.x, b.y, 0.);
    let d = p(e.x, e.y, 0.) - a;
    let length = d.length_squared();
    if length < 1e-6 {
        return false;
    }
    let t = ((point - a).dot(d) / length).clamp(0., 1.);
    (point - a - d * t).length() < padding
}
fn road_ribbon(g: &mut Builder, roads: &[RoadPath], extra: f64, height: f64, material: usize) {
    for road in roads {
        let Some(first) = road.points.first() else {
            continue;
        };
        let mut points = vec![p(first.x, first.y, height)];
        for line in road.points.windows(2) {
            let a = p(line[0].x, line[0].y, height);
            let b = p(line[1].x, line[1].y, height);
            let length = (b - a).length();
            if length < 1e-10 {
                continue;
            }
            let steps = (length / 0.006).ceil().max(1.) as usize;
            for i in 1..=steps {
                points.push(a.lerp(b, i as f64 / steps as f64));
            }
        }
        if points.len() < 2 {
            continue;
        }
        let closed = (points[0] - *points.last().unwrap()).length() < 1e-8;
        let mut lengths = vec![0.];
        for pair in points.windows(2) {
            lengths.push(lengths.last().unwrap() + (pair[1] - pair[0]).length());
        }
        let total = lengths.last().copied().unwrap_or(1.).max(1e-10);
        let edges = points
            .iter()
            .enumerate()
            .map(|(i, at)| {
                let (before, after) = if closed && (i == 0 || i == points.len() - 1) {
                    (points[points.len() - 2], points[1])
                } else {
                    (
                        points[i.saturating_sub(1)],
                        points[(i + 1).min(points.len() - 1)],
                    )
                };
                let mut a = *at - before;
                let mut b = after - *at;
                if a.length() > 1e-10 {
                    a = a.normalize();
                } else {
                    a = b.normalize();
                }
                if b.length() > 1e-10 {
                    b = b.normalize();
                } else {
                    b = a;
                }
                let sum = a + b;
                let normal = p(-sum.y, sum.x, 0.) / sum.length().max(1e-10);
                let width = (road.width
                    + (road.end_width.unwrap_or(road.width) - road.width) * lengths[i] / total)
                    / 2.
                    + extra;
                let miter = width / normal.dot(p(-b.y, b.x, 0.)).max(0.65);
                [*at - normal * miter, *at + normal * miter]
            })
            .collect::<Vec<_>>();
        for pair in edges.windows(2) {
            let [a, b] = pair[0];
            let [c, d] = pair[1];
            g.quad(
                material,
                [a, c, d, b],
                [
                    [a.x * 4., a.y * 4.],
                    [c.x * 4., c.y * 4.],
                    [d.x * 4., d.y * 4.],
                    [b.x * 4., b.y * 4.],
                ],
            );
        }
    }
}
fn details(g: &mut Builder, l: &StructureLayout) {
    road_ribbon(g, &l.roads, 0.0012, 0.0076, 4);
    road_ribbon(g, &l.roads, 0., 0.008, 13);
    if l.style.ground == StructureGround::Recessed {
        g.lathe(
            &[
                [0.054, 0.007],
                [0.067, 0.007],
                [0.067, 0.031],
                [0.054, 0.031],
                [0.054, 0.007],
            ],
            48,
            0.,
            TAU,
            16,
        );
        g.cylinder(p(0., 0., 0.008), 0.054, 0.002, 12, 48);
        g.ring(p(0., 0., 0.032), 0.054, 0.002, 6, TAU);
        for x in [-0.032, 0.032] {
            g.box_at(p(x, 0.022, 0.009), p(0.024, 0.01, 0.017), 12);
            g.box_at(p(x, 0.0165, 0.012), p(0.018, 0.001, 0.009), 11);
            g.box_at(p(x, 0.021, 0.026), p(0.028, 0.015, 0.003), 17);
        }
    }
    if l.style.details.contains(&StructureDetail::Pipes) {
        if l.style.ground == StructureGround::Recessed {
            for side in [-1., 1.] {
                for run in 0..2 {
                    let run = run as f64;
                    let x = side * (0.074 + run * 0.007);
                    let points = [
                        p(side * 0.079, -0.05, 0.022 + run * 0.004),
                        p(x, -0.035, 0.016),
                        p(x, 0.019, 0.016),
                        p(side * 0.065, 0.033 + run * 0.006, 0.019),
                        p(side * 0.045, 0.03 + run * 0.006, 0.029),
                    ];
                    g.tube(&points, 0.0018, 40, 8, 17);
                    // The reference collars use arc-length positions, not uniform knots.
                    let mut samples = Vec::with_capacity(201);
                    let mut length = 0.;
                    let mut before = catmull(&points, 0.);
                    samples.push((0., before));
                    for i in 1..=200 {
                        let at = catmull(&points, i as f64 / 200.);
                        length += (at - before).length();
                        samples.push((length, at));
                        before = at;
                    }
                    for t in [0.12, 0.4, 0.7, 0.95] {
                        let wanted = t * length;
                        let index = samples
                            .partition_point(|(distance, _)| *distance < wanted)
                            .clamp(1, 200);
                        let (a, b) = (samples[index - 1], samples[index]);
                        let f = (wanted - a.0) / (b.0 - a.0).max(1e-10);
                        let point = a.1.lerp(b.1, f);
                        let u = (index as f64 - 1. + f) / 200.;
                        let direction = (catmull(&points, u + 0.0001)
                            - catmull(&points, u - 0.0001))
                        .normalize_or_zero();
                        g.torus(
                            point,
                            0.0023,
                            0.0006,
                            6,
                            [0., TAU],
                            DQuat::from_rotation_arc(V::Z, direction),
                        );
                    }
                    for y in [-0.02, 0.01] {
                        g.box_at(p(x, y, 0.007), p(0.005, 0.003, 0.007), 5);
                    }
                }
                g.cylinder(p(side * 0.074, -0.006, 0.021), 0.0008, 0.008, 17, 8);
                g.ring(p(side * 0.074, -0.006, 0.025), 0.004, 0.0007, 6, TAU);
                g.box_at(p(side * 0.074, -0.006, 0.0245), p(0.008, 0.001, 0.001), 17);
            }
        } else {
            let mut count = 0;
            for (i, a) in l.buildings.iter().enumerate().filter(|(_, b)| b.habitat) {
                let mut nearby = l
                    .buildings
                    .iter()
                    .enumerate()
                    .filter(|(j, b)| *j != i && !b.landmark)
                    .collect::<Vec<_>>();
                nearby.sort_by(|(_, b), (_, c)| {
                    (b.x - a.x)
                        .hypot(b.y - a.y)
                        .total_cmp(&(c.x - a.x).hypot(c.y - a.y))
                });
                for (j, b) in nearby {
                    if (b.x - a.x).hypot(b.y - a.y) > 0.047 {
                        break;
                    }
                    let blocked = (0..12).any(|step| {
                        let t = step as f64 / 11.;
                        let point = p(a.x + (b.x - a.x) * t, a.y + (b.y - a.y) * t, 0.);
                        let road = nearest(point, &l.roads);
                        road.0 < road.1 + 0.002
                            || l.buildings.iter().enumerate().any(|(k, c)| {
                                k != i && k != j && inside_building(c, point.x, point.y, 0.002)
                            })
                    });
                    if blocked {
                        continue;
                    }
                    g.tube(
                        &[
                            p(a.x, a.y, a.z + 0.008),
                            p(
                                a.x + (b.x - a.x) * 0.2,
                                a.y + (b.y - a.y) * 0.2,
                                a.z + 0.004,
                            ),
                            p(
                                a.x + (b.x - a.x) * 0.8,
                                a.y + (b.y - a.y) * 0.8,
                                b.z + 0.004,
                            ),
                            p(b.x, b.y, b.z + 0.008),
                        ],
                        0.0012,
                        12,
                        6,
                        17,
                    );
                    count += 1;
                    break;
                }
                if count >= 3 {
                    break;
                }
            }
        }
    }
    if l.style.details.contains(&StructureDetail::Shafts) && l.style.plan != StructurePlan::Grid {
        for x in [-0.079, 0.079] {
            g.cylinder(p(x, -0.05, 0.017), 0.012, 0.02, 5, 12);
            g.cylinder(p(x, -0.05, 0.028), 0.016, 0.003, 6, 12);
            g.ring(p(x, -0.05, 0.029), 0.01, 0.001, 11, TAU);
            g.box_at(p(x, -0.05, 0.03), p(0.004, 0.004, 0.015), 4);
        }
    }
    if l.style.lighting == StructureLighting::Bright {
        for x in [-0.051, 0.051] {
            g.box_at(p(x, 0., 0.009), p(0.0015, 0.215, 0.001), 11);
        }
    }
    if l.style.exposure == StructureExposure::Surface {
        let mut candidates = vec![];
        for road in &l.roads {
            let mut accumulated = 0.;
            for line in road.points.windows(2) {
                let (a, b) = (line[0], line[1]);
                let dx = b.x - a.x;
                let dy = b.y - a.y;
                let length = dx.hypot(dy);
                if length < 1e-6 {
                    continue;
                }
                let (nx, ny) = (-dy / length, dx / length);
                let spacing = if l.style.lighting == StructureLighting::Bright {
                    0.028
                } else {
                    0.038
                };
                let mut distance = spacing - accumulated;
                while distance < length {
                    for side in [-1., 1.] {
                        let across = side * (road.width / 2. + 0.0025);
                        candidates.push((
                            p(
                                a.x + dx * distance / length + nx * across,
                                a.y + dy * distance / length + ny * across,
                                0.,
                            ),
                            p(-nx * side, -ny * side, 0.),
                        ));
                    }
                    distance += spacing;
                }
                accumulated = (accumulated + length) % spacing;
            }
        }
        let mut placed: Vec<V> = vec![];
        for (at, direction) in candidates {
            let hit = nearest(at, &l.roads);
            if hit.0 < hit.1 + 0.002
                || l.buildings
                    .iter()
                    .any(|b| inside_building(b, at.x, at.y, 0.006) || entrance(b, at, 0.004))
                || l.plants
                    .iter()
                    .any(|b| (b.x - at.x).hypot(b.y - at.y) < 0.004)
                || placed.iter().any(|v| (*v - at).length() < 0.018)
                || (l.style.cover == StructureCover::Dome && at.length() > 0.102)
                || (l.style.landmark == StructureLandmark::Hall
                    && wall_distance(l, at.x, at.y) > -0.008)
            {
                continue;
            }
            placed.push(at);
            g.beam(
                at - direction * 0.001 + p(0., 0., 0.0074),
                at + direction * 0.0015 + p(0., 0., 0.0074),
                0.0035,
                0.0008,
                4,
            );
            g.cylinder(at + p(0., 0., 0.0165), 0.0006, 0.018, 5, 8);
            g.beam(
                at + p(0., 0., 0.025),
                at + direction * 0.005 + p(0., 0., 0.025),
                0.001,
                0.001,
                5,
            );
            g.beam(
                at + direction * 0.003 + p(0., 0., 0.0244),
                at + direction * 0.006 + p(0., 0., 0.0244),
                0.002,
                0.0007,
                11,
            );
            let start = g.parts.len();
            g.quad(
                15,
                [
                    p(-0.007, -0.004, 0.),
                    p(0.007, -0.004, 0.),
                    p(0.007, 0.004, 0.),
                    p(-0.007, 0.004, 0.),
                ],
                [[0., 0.], [1., 0.], [1., 1.], [0., 1.]],
            );
            g.transform(
                start,
                rot(direction.y.atan2(direction.x)),
                at + direction * 0.006 + p(0., 0., 0.009),
                V::ONE,
            );
        }
    }
}

fn tangents(mesh: &mut Mesh) -> Result<(), GeometryError> {
    use bevy::{math::Vec3, mesh::VertexAttributeValues};
    let Some(VertexAttributeValues::Float32x3(positions)) =
        mesh.attribute(Mesh::ATTRIBUTE_POSITION)
    else {
        return Err(GeometryError::Bounds);
    };
    let Some(VertexAttributeValues::Float32x3(normals)) = mesh.attribute(Mesh::ATTRIBUTE_NORMAL)
    else {
        return Err(GeometryError::Bounds);
    };
    let Some(VertexAttributeValues::Float32x2(uv)) = mesh.attribute(Mesh::ATTRIBUTE_UV_0) else {
        return Err(GeometryError::Bounds);
    };
    let Some(Indices::U32(indices)) = mesh.indices() else {
        return Err(GeometryError::Bounds);
    };
    let mut accum = vec![(Vec3::ZERO, Vec3::ZERO); positions.len()];
    for tri in indices.as_chunks::<3>().0 {
        let [a, b, c] = [tri[0] as usize, tri[1] as usize, tri[2] as usize];
        let e1 = Vec3::from_array(positions[b]) - Vec3::from_array(positions[a]);
        let e2 = Vec3::from_array(positions[c]) - Vec3::from_array(positions[a]);
        let x1 = uv[b][0] - uv[a][0];
        let x2 = uv[c][0] - uv[a][0];
        let y1 = uv[b][1] - uv[a][1];
        let y2 = uv[c][1] - uv[a][1];
        let determinant = x1 * y2 - x2 * y1;
        if determinant.abs() > 1e-12 {
            let t = (e1 * y2 - e2 * y1) / determinant;
            let b = (e2 * x1 - e1 * x2) / determinant;
            for i in tri {
                accum[*i as usize].0 += t;
                accum[*i as usize].1 += b;
            }
        }
    }
    let mut values = Vec::with_capacity(positions.len());
    for (i, (t, b)) in accum.into_iter().enumerate() {
        let n = Vec3::from_array(normals[i]).normalize_or_zero();
        let mut t = (t - n * t.dot(n)).normalize_or_zero();
        if t.length_squared() < 0.5 {
            t = n
                .cross(if n.z.abs() < 0.9 { Vec3::Z } else { Vec3::Y })
                .normalize_or_zero();
        }
        values.push([t.x, t.y, t.z, if n.cross(t).dot(b) < 0. { -1. } else { 1. }]);
    }
    mesh.insert_attribute(Mesh::ATTRIBUTE_TANGENT, values);
    Ok(())
}

// Same deterministic original collapse facts; consumed by per-group animation.
pub(super) struct CollapseParams {
    pub(super) delay_s: f32,
    pub(super) tilt_rad: f32,
    pub(super) direction_rad: f32,
}
pub(super) fn collapse_params(seed: u64) -> CollapseParams {
    let p = openmars_scene::geometry::collapse::CollapseParameters::new(seed);
    CollapseParams {
        delay_s: p.delay as f32,
        tilt_rad: p.tilt as f32,
        direction_rad: p.direction as f32,
    }
}
