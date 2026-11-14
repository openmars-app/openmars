//! One cooperative source disc at a time; mesh publication is all-or-nothing.
use super::material::DIRECTION;
use bevy::{
    asset::RenderAssetUsages,
    mesh::{Indices, PrimitiveTopology},
    prelude::*,
};
use openmars_scene::geometry::{nuclear_mesh::DiscGeometry, nuclear_site::BoardFrame};
pub(super) fn mesh(data: DiscGeometry, frame: &BoardFrame, radius: f64) -> Mesh {
    let uv1: Vec<[f32; 2]> = data.positions.iter().map(|p| [p[0], p[1]]).collect();
    let positions: Vec<[f32; 3]> = data
        .positions
        .iter()
        .map(|p| {
            let direction = frame
                .direction(f64::from(p[0]), f64::from(p[1]))
                .expect("validated source disc");
            direction.map(|v| (v * (radius + f64::from(p[2]))) as f32)
        })
        .collect();
    let colors: Vec<[f32; 4]> = data
        .coverage
        .into_iter()
        .zip(data.apron)
        .map(|(c, a)| [c, a, 0., 1.])
        .collect();
    let mut mesh = Mesh::new(
        PrimitiveTopology::TriangleList,
        RenderAssetUsages::RENDER_WORLD,
    );
    mesh.insert_attribute(Mesh::ATTRIBUTE_POSITION, positions);
    mesh.insert_attribute(Mesh::ATTRIBUTE_UV_0, data.uv);
    mesh.insert_attribute(Mesh::ATTRIBUTE_UV_1, uv1);
    mesh.insert_attribute(Mesh::ATTRIBUTE_COLOR, colors);
    mesh.insert_attribute(DIRECTION, data.planet_directions);
    mesh.insert_indices(Indices::U32(data.indices));
    mesh.compute_area_weighted_normals();
    mesh
}
