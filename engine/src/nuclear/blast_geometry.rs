//! Original local missile and radially subdivided shockwave, with stateless
//! timing and the compositor's real view-space culling/scissor bounds.

use bevy::{
    asset::RenderAssetUsages,
    mesh::{Indices, PrimitiveTopology},
    prelude::*,
};

pub const MAX_BLASTS: usize = 512;
pub const CLOUD_STEPS: u32 = 48;
pub const CLOUD_END: f32 = 6.2;
pub const WAVE_RADIUS: f32 = 0.98;
pub const WAVE_SECONDS: f32 = 2.8;
pub const DUST_SECONDS: f32 = 0.8;

#[derive(Clone, Copy, Debug, PartialEq)]
pub struct BlastSample {
    pub age: f32,
    pub missile: Option<Transform>,
    pub wave: bool,
    pub cloud: bool,
    pub complete: bool,
}
pub fn sample(start: u64, now: u64, reduced: bool) -> BlastSample {
    let elapsed = now.saturating_sub(start) as f64 / 60. * 0.8;
    let age = (elapsed - 0.8) as f32;
    let full = !reduced && now >= start;
    let from = Vec3::new(-0.48, 0.18, 0.85);
    let to = Vec3::new(0., 0., -0.008);
    BlastSample {
        age,
        missile: (full && now - start < 60).then(|| Transform {
            translation: from.lerp(to, (elapsed / 0.8).min(1.).powf(1.5) as f32),
            rotation: Quat::from_rotation_arc(Vec3::Y, (to - from).normalize()),
            scale: Vec3::ONE,
        }),
        wave: full && (0. ..WAVE_SECONDS + DUST_SECONDS).contains(&age),
        cloud: full && (0. ..CLOUD_END).contains(&age),
        complete: now >= start && now - start >= if reduced { 9 } else { 525 },
    }
}
pub fn shake(age: f32) -> Vec2 {
    if !(0. ..0.35).contains(&age) {
        return Vec2::ZERO;
    }
    let amplitude = 1.5 * (1. - age / 0.35).powi(2);
    Vec2::new(
        (age * 90.).sin() * amplitude * 0.8,
        (age * 113.).sin() * amplitude * 0.6,
    )
}
pub fn clamp_shake(shake: Vec2) -> Vec2 {
    shake.clamp_length_max(1.5)
}
/// Read the current authored hierarchy before PostUpdate propagation. This
/// avoids a one-frame camera/body lag while the ordinary celestial rig moves.
pub fn current_matrix(world: &World, mut entity: Entity) -> Option<Mat4> {
    let mut matrix = Mat4::IDENTITY;
    for _ in 0..64 {
        if world.get_entity(entity).is_err() {
            return None;
        }
        matrix = world
            .get::<Transform>(entity)
            .map(Transform::to_matrix)
            .unwrap_or(Mat4::IDENTITY)
            * matrix;
        if let Some(parent) = world.get::<ChildOf>(entity) {
            entity = parent.parent();
        } else {
            return matrix.is_finite().then_some(matrix);
        }
    }
    None
}

pub fn missile() -> Mesh {
    let mut body = Cylinder::new(1., 1.)
        .mesh()
        .resolution(8)
        .segments(1)
        .build();
    // A cone frustum has different top and bottom radii. Derive the two ring
    // radii directly and retain the source's caps and eight-sided silhouette.
    if let Some(bevy::mesh::VertexAttributeValues::Float32x3(p)) =
        body.attribute_mut(Mesh::ATTRIBUTE_POSITION)
    {
        for p in p {
            let radius = if p[1] > 0. { 0.0035 } else { 0.004 };
            p[0] *= radius;
            p[2] *= radius;
            p[1] *= 0.043;
        }
    }
    // Recompute the side normals for the actual taper while caps stay planar.
    if let Some(bevy::mesh::VertexAttributeValues::Float32x3(n)) =
        body.attribute_mut(Mesh::ATTRIBUTE_NORMAL)
    {
        for n in n {
            if n[1].abs() < 0.5 {
                *n = Vec3::new(n[0], 0.0005 / 0.043, n[2]).normalize().to_array();
            }
        }
    }
    let mut nose = Cone::new(0.0035, 0.012).mesh().resolution(8).build();
    nose.translate_by(Vec3::Y * 0.0275);
    body.merge(&nose)
        .expect("matching built-in triangle attributes");
    body
}
pub fn wave() -> Mesh {
    let mut p = Vec::with_capacity(129 * 41);
    let mut uv = Vec::with_capacity(p.capacity());
    let mut normal = Vec::with_capacity(p.capacity());
    for row in 0..=40 {
        let radius = (WAVE_RADIUS + 0.08) * row as f32 / 40.;
        for col in 0..=128 {
            let theta = col as f32 * std::f32::consts::TAU / 128.;
            let x = theta.cos() * radius;
            let y = theta.sin() * radius;
            p.push([x, y, 0.006]);
            normal.push([0., 0., 1.]);
            uv.push([
                x / (WAVE_RADIUS + 0.08) * 0.5 + 0.5,
                y / (WAVE_RADIUS + 0.08) * 0.5 + 0.5,
            ]);
        }
    }
    let mut indices = Vec::with_capacity(128 * 40 * 6);
    for row in 0..40 {
        for col in 0..128 {
            let a = row * 129 + col;
            let b = a + 129;
            indices.extend_from_slice(&[a, b, a + 1, b, b + 1, a + 1]);
        }
    }
    Mesh::new(
        PrimitiveTopology::TriangleList,
        RenderAssetUsages::MAIN_WORLD | RenderAssetUsages::RENDER_WORLD,
    )
    .with_inserted_attribute(Mesh::ATTRIBUTE_POSITION, p)
    .with_inserted_attribute(Mesh::ATTRIBUTE_NORMAL, normal)
    .with_inserted_attribute(Mesh::ATTRIBUTE_UV_0, uv)
    .with_inserted_indices(Indices::U32(indices))
}
/// Same half-resolution/960-pixel cap as the original compositor.
pub fn cloud_extent([w, h]: [u32; 2]) -> Option<[u32; 2]> {
    if w == 0 || h == 0 || w > 8192 || h > 8192 {
        return None;
    }
    let scale = 0.5_f64.min(960. / w as f64);
    Some([
        (w as f64 * scale).round().max(1.) as u32,
        (h as f64 * scale).round().max(1.) as u32,
    ])
}
/// Top-left WGPU viewport coordinates. Near-plane crossings conservatively
/// keep the whole view; boxes entirely outside never allocate a draw rectangle.
pub fn scissor(clip_from_local: Mat4, extent: [u32; 2]) -> Option<[u32; 4]> {
    let mut lo = Vec2::ONE;
    let mut hi = Vec2::NEG_ONE;
    for i in 0..8 {
        let p = clip_from_local
            * Vec4::new(
                if i & 1 != 0 { 0.36 } else { -0.36 },
                if i & 2 != 0 { 0.36 } else { -0.36 },
                if i & 4 != 0 { 0.5 } else { -0.06 },
                1.,
            );
        if !p.is_finite() {
            return None;
        }
        if p.w <= 0. || p.z < 0. || p.z > p.w {
            return Some([0, 0, extent[0], extent[1]]);
        }
        let xy = Vec2::new(p.x / p.w, -p.y / p.w);
        lo = lo.min(xy);
        hi = hi.max(xy);
    }
    lo = (lo + Vec2::ONE) * 0.5 * Vec2::from_array(extent.map(|x| x as f32));
    hi = (hi + Vec2::ONE) * 0.5 * Vec2::from_array(extent.map(|x| x as f32));
    let x = lo.x.floor().max(0.).min(extent[0] as f32) as u32;
    let y = lo.y.floor().max(0.).min(extent[1] as f32) as u32;
    let r = hi.x.ceil().max(0.).min(extent[0] as f32) as u32;
    let b = hi.y.ceil().max(0.).min(extent[1] as f32) as u32;
    (r > x && b > y).then_some([x, y, r.saturating_sub(x), b.saturating_sub(y)])
}
pub fn visible(world_from_local: Mat4, clip_from_world: Mat4, camera: Vec3) -> bool {
    let origin = world_from_local.transform_point3(Vec3::ZERO);
    let up = world_from_local
        .transform_vector3(Vec3::Z)
        .normalize_or_zero();
    if up.dot(camera - origin) <= 0. {
        return false;
    }
    let center = world_from_local
        .transform_point3(Vec3::new(0., 0., 0.2))
        .extend(1.);
    // Homogeneous frustum sphere test, with plane normalization preserving the
    // source0.5 world-unit radius even at oblique angles and wide aspect ratios.
    let m = clip_from_world.transpose();
    let rows = [m.x_axis, m.y_axis, m.z_axis, m.w_axis];
    [
        rows[3] + rows[0],
        rows[3] - rows[0],
        rows[3] + rows[1],
        rows[3] - rows[1],
        rows[2],
        rows[3] - rows[2],
    ]
    .into_iter()
    .all(|p| p.dot(center) >= -0.5 * p.truncate().length())
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn source_timing_shapes_and_reduced_motion() {
        assert!(sample(100, 99, false).missile.is_none());
        assert_eq!(
            sample(100, 100, false).missile.unwrap().translation,
            Vec3::new(-0.48, 0.18, 0.85)
        );
        assert!(sample(100, 159, false).missile.is_some());
        let hit = sample(100, 160, false);
        assert!(hit.missile.is_none() && hit.wave && hit.cloud);
        assert!(hit.age.abs() < 1e-6);
        assert!(sample(100, 429, false).wave);
        assert!(!sample(100, 430, false).wave);
        assert!(!sample(100, 625, false).cloud);
        assert!(sample(100, 625, false).complete);
        let reduced = sample(100, 100, true);
        assert!(!reduced.cloud && !reduced.wave && reduced.missile.is_none());
        assert!(sample(100, 109, true).complete);
        let w = wave();
        assert_eq!(w.count_vertices(), 5289);
        assert_eq!(w.indices().unwrap().len(), 30720);
    }
    #[test]
    fn bounds_and_shake_are_source_sized() {
        assert_eq!(cloud_extent([3840, 2160]), Some([960, 540]));
        assert_eq!(cloud_extent([801, 601]), Some([401, 301]));
        assert_eq!(cloud_extent([0, 1]), None);
        assert_eq!(shake(-0.1), Vec2::ZERO);
        assert_eq!(shake(0.35), Vec2::ZERO);
        assert!(clamp_shake(Vec2::splat(10.)).length() <= 1.500001);
        assert!(scissor(Mat4::IDENTITY, [100, 100]).is_some());
    }
}
