use bevy::{
    asset::RenderAssetUsages,
    mesh::{Indices, PrimitiveTopology},
    prelude::*,
};
pub(super) fn canopy_mesh() -> Mesh {
    disc(0.14, 28, 56, false)
}
pub(super) fn ground_mesh() -> Mesh {
    disc(0.166 * 1.8, 10, 36, true)
}
fn disc(outer_radius: f64, rings: usize, segments: usize, rotate: bool) -> Mesh {
    let mut positions = Vec::with_capacity(1 + rings * segments);
    let mut uv = Vec::with_capacity(1 + rings * segments);
    positions.push([0.; 3]);
    uv.push([0.5; 2]);
    for ring in 1..=rings {
        let radius = ring as f64 / rings as f64 * outer_radius;
        for a in 0..segments {
            let angle = a as f64 / segments as f64 * std::f64::consts::TAU;
            let (x, y) = (angle.cos() * radius, angle.sin() * radius);
            positions.push(if rotate {
                [-y as f32, x as f32, 0.]
            } else {
                [x as f32, y as f32, 0.]
            });
            uv.push([
                (0.5 + x / (outer_radius * 2.)) as f32,
                (0.5 + y / (outer_radius * 2.)) as f32,
            ]);
        }
    }
    let mut indices = Vec::with_capacity(segments * 3 + (rings - 1) * segments * 6);
    for a in 0..segments {
        indices.extend([0, 1 + a as u32, 1 + ((a + 1) % segments) as u32]);
    }
    for ring in 1..rings {
        let inner = 1 + (ring - 1) * segments;
        let outer = 1 + ring * segments;
        for a in 0..segments {
            let n = (a + 1) % segments;
            indices.extend(
                [
                    inner + a,
                    outer + a,
                    outer + n,
                    inner + a,
                    outer + n,
                    inner + n,
                ]
                .map(|n| n as u32),
            );
        }
    }
    Mesh::new(
        PrimitiveTopology::TriangleList,
        RenderAssetUsages::MAIN_WORLD | RenderAssetUsages::RENDER_WORLD,
    )
    .with_inserted_attribute(Mesh::ATTRIBUTE_NORMAL, vec![[0., 0., 1.]; positions.len()])
    .with_inserted_attribute(Mesh::ATTRIBUTE_POSITION, positions)
    .with_inserted_attribute(Mesh::ATTRIBUTE_UV_0, uv)
    .with_inserted_indices(Indices::U32(indices))
}

pub(super) fn collapse_depth(mesh: &Mesh, rotation: bevy::math::DQuat, minimum: f64) -> f32 {
    use bevy::math::DVec3;
    let bounds = mesh
        .attribute(Mesh::ATTRIBUTE_POSITION)
        .unwrap()
        .as_float3()
        .unwrap()
        .iter()
        .map(|p| DVec3::from_array(p.map(f64::from)))
        .fold(
            [DVec3::splat(f64::INFINITY), DVec3::splat(f64::NEG_INFINITY)],
            |a, p| [a[0].min(p), a[1].max(p)],
        );
    crate::nuclear::collapse::depth(bounds, rotation, minimum)
}
