//! Original48-segment pad and56-segment hazard ring, projected to the body's
//! radius before upload. All players share two meshes/materials; colors are data.
use super::flight::Parking;
use bevy::{
    asset::RenderAssetUsages,
    mesh::{Indices, PrimitiveTopology},
    prelude::*,
};
pub fn geometry(
    parking: &[Parking],
    colors: &[[f32; 3]],
    radius: f64,
    warning: bool,
) -> Result<Mesh, String> {
    if parking.len() != colors.len()
        || parking.len() > super::flight::MAX_SHIPS
        || !radius.is_finite()
        || radius <= 0.
        || parking.iter().any(|p| {
            !p.pad.position.is_finite()
                || p.pad.position.length_squared() < 1e-20
                || !p.pad.rotation.is_finite()
                || (p.pad.rotation.length_squared() - 1.).abs() > 1e-6
                || !p.normal.is_finite()
                || (p.normal.length_squared() - 1.).abs() > 1e-6
        })
        || colors
            .iter()
            .flatten()
            .any(|v| !v.is_finite() || *v < 0. || *v > 1.)
    {
        return Err("ship pad inputs".into());
    }
    let (mut positions, mut normals, mut uv, mut local, mut rgba, mut data, mut indices) = (
        Vec::new(),
        Vec::new(),
        Vec::new(),
        Vec::new(),
        Vec::new(),
        Vec::new(),
        Vec::new(),
    );
    for (park, color) in parking.iter().zip(colors) {
        let offset = positions.len() as u32;
        let segments = if warning { 56 } else { 48 };
        let mut vertex = |x: f64, y: f64, u: f64, v: f64| {
            let p = park.pad.position + park.pad.rotation * bevy::math::DVec3::new(x, y, 0.);
            positions.push(
                (p.normalize() * (radius + if warning { 0.007 } else { 0.004 }))
                    .as_vec3()
                    .to_array(),
            );
            normals.push(park.normal.as_vec3().to_array());
            uv.push([u as f32, v as f32]);
            local.push([x as f32, y as f32]);
            rgba.push([color[0], color[1], color[2], 1.]);
            data.push([
                (park.pad.position.x * 37.) as f32,
                (park.pad.position.y * 37.) as f32,
                0.,
                0.,
            ]);
        };
        if warning {
            for r in [0.051, 0.059] {
                for i in 0..=segments {
                    let a = i as f64 / segments as f64 * std::f64::consts::TAU;
                    let (x, y) = (a.cos() * r, a.sin() * r);
                    vertex(x, y, x / 0.059 * 0.5 + 0.5, y / 0.059 * 0.5 + 0.5);
                }
            }
            for i in 0..segments {
                let a = offset + i;
                let b = a + segments + 1;
                indices.extend([a, b, a + 1, b, b + 1, a + 1]);
            }
        } else {
            vertex(0., 0., 0.5, 0.5);
            for i in 0..=segments {
                let a = i as f64 / segments as f64 * std::f64::consts::TAU;
                let (x, y) = (a.cos() * 0.264, a.sin() * 0.264);
                vertex(x, y, x / 0.264 * 0.5 + 0.5, y / 0.264 * 0.5 + 0.5);
            }
            for i in 0..segments {
                indices.extend([offset, offset + i + 1, offset + i + 2]);
            }
        }
    }
    Ok(Mesh::new(
        PrimitiveTopology::TriangleList,
        RenderAssetUsages::MAIN_WORLD | RenderAssetUsages::RENDER_WORLD,
    )
    .with_inserted_attribute(Mesh::ATTRIBUTE_POSITION, positions)
    .with_inserted_attribute(Mesh::ATTRIBUTE_NORMAL, normals)
    .with_inserted_attribute(Mesh::ATTRIBUTE_UV_0, uv)
    .with_inserted_attribute(Mesh::ATTRIBUTE_UV_1, local)
    .with_inserted_attribute(Mesh::ATTRIBUTE_COLOR, rgba)
    .with_inserted_attribute(Mesh::ATTRIBUTE_TANGENT, data)
    .with_inserted_indices(Indices::U32(indices)))
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn merged_pads_preserve_source_segments_projection_colors_and_flat_normals() {
        let parking = super::super::flight::parking(8, 2.02).unwrap();
        let colors = vec![[0.2, 0.4, 0.8]; 8];
        for (warning, vertices, indices) in [(false, 50 * 8, 144 * 8), (true, 114 * 8, 336 * 8)] {
            let mesh = geometry(&parking, &colors, 2.02, warning).unwrap();
            assert_eq!(mesh.count_vertices(), vertices);
            assert_eq!(mesh.indices().unwrap().len(), indices);
            let bevy::mesh::VertexAttributeValues::Float32x3(positions) =
                mesh.attribute(Mesh::ATTRIBUTE_POSITION).unwrap()
            else {
                panic!()
            };
            assert!(positions.iter().all(|p| {
                (Vec3::from_array(*p).length() - (2.02 + if warning { 0.007 } else { 0.004 })).abs()
                    < 1e-6
            }));
            let bevy::mesh::VertexAttributeValues::Float32x3(normals) =
                mesh.attribute(Mesh::ATTRIBUTE_NORMAL).unwrap()
            else {
                panic!()
            };
            assert!(
                normals[..vertices / 8]
                    .iter()
                    .all(|n| *n == parking[0].normal.as_vec3().to_array())
            );
        }
        let mut invalid = parking;
        invalid[0].normal.x = f64::NAN;
        assert!(geometry(&invalid, &colors, 2.02, false).is_err());
        assert!(geometry(&[], &[], f64::NAN, false).is_err());
    }
}
