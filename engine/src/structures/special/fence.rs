//! The approved GLB is merged under its first material, as in MoholeTile.
use bevy::{
    asset::RenderAssetUsages,
    gltf::{GltfMaterial, GltfMesh, GltfNode},
    math::{DMat4, DVec3},
    mesh::{Indices, PrimitiveTopology, VertexAttributeValues},
    prelude::*,
};
use std::collections::BTreeSet;
pub(super) struct Fence {
    pub mesh: Mesh,
    pub width: f64,
    pub texture: Option<Handle<Image>>,
    pub surface: Vec4,
    pub bytes: usize,
}
pub(super) fn prepare(world: &World) -> Result<Option<Fence>, String> {
    let Some(handle) = crate::assets::model(world, "models/fence") else {
        return Ok(None);
    };
    if !world
        .resource::<AssetServer>()
        .is_loaded_with_dependencies(handle.id())
    {
        return Ok(None);
    }
    let models = world.resource::<Assets<bevy::gltf::Gltf>>();
    let model = models.get(&handle).ok_or("Missing fence model")?;
    if model.nodes.len() > 128 || !model.skins.is_empty() || !model.animations.is_empty() {
        return Err("Static fence hierarchy bound".into());
    }
    let nodes = world.resource::<Assets<GltfNode>>();
    let meshes = world.resource::<Assets<GltfMesh>>();
    let sources = world.resource::<Assets<Mesh>>();
    let mut children = BTreeSet::new();
    for h in &model.nodes {
        children.extend(
            nodes
                .get(h)
                .ok_or("Missing fence node")?
                .children
                .iter()
                .map(|c| c.id()),
        );
    }
    let mut stack: Vec<_> = model
        .nodes
        .iter()
        .filter(|h| !children.contains(&h.id()))
        .rev()
        .map(|h| (h.clone(), DMat4::IDENTITY))
        .collect();
    let mut seen = BTreeSet::new();
    let mut primitives = Vec::new();
    let (mut low, mut high) = (DVec3::splat(f64::INFINITY), DVec3::splat(f64::NEG_INFINITY));
    let mut vertices = 0;
    let mut count = 0;
    while let Some((h, parent)) = stack.pop() {
        if !seen.insert(h.id()) {
            return Err("Repeated fence node".into());
        }
        let node = nodes.get(&h).ok_or("Missing fence node")?;
        let matrix = parent * node.transform.to_matrix().as_dmat4();
        if !matrix.is_finite() {
            return Err("Nonfinite fence transform".into());
        }
        for child in node.children.iter().rev() {
            stack.push((child.clone(), matrix));
        }
        if let Some(h) = &node.mesh {
            for primitive in &meshes.get(h).ok_or("Missing fence primitive")?.primitives {
                let mesh = sources.get(&primitive.mesh).ok_or("Missing fence mesh")?;
                let positions = mesh
                    .attribute(Mesh::ATTRIBUTE_POSITION)
                    .and_then(|v| v.as_float3())
                    .ok_or("Missing fence positions")?;
                if mesh.primitive_topology() != PrimitiveTopology::TriangleList {
                    return Err("Fence requires triangles".into());
                }
                vertices += positions.len();
                count += mesh.indices().map_or(positions.len(), |i| i.len());
                if vertices > 16384 || count > 65536 {
                    return Err("Fence source working-set bound".into());
                }
                for p in positions {
                    let v = matrix
                        .transform_point3(DVec3::from_array(p.map(f64::from)))
                        .as_vec3()
                        .as_dvec3();
                    if !v.is_finite() {
                        return Err("Nonfinite fence vertex".into());
                    }
                    low = low.min(v);
                    high = high.max(v);
                }
                primitives.push((primitive.mesh.clone(), primitive.material.clone(), matrix));
            }
        }
    }
    let height = high.y - low.y;
    if !height.is_finite() || height <= 0. || primitives.is_empty() {
        return Err("Invalid fence extent".into());
    }
    let scale = 0.018 / height;
    let material = primitives[0]
        .1
        .as_ref()
        .and_then(|h| world.resource::<Assets<GltfMaterial>>().get(h))
        .ok_or("Missing first fence material")?;
    if material.base_color_channel != bevy::mesh::UvChannel::Uv0 {
        return Err("Fence must use UV0".into());
    }
    let color = material.base_color.to_linear().to_f32_array();
    let mut positions = Vec::with_capacity(vertices);
    let mut uv = Vec::with_capacity(vertices);
    let mut indices = Vec::with_capacity(count);
    for (h, _, matrix) in primitives {
        let mesh = sources.get(&h).unwrap();
        let source = mesh
            .attribute(Mesh::ATTRIBUTE_POSITION)
            .unwrap()
            .as_float3()
            .unwrap();
        let tex = match mesh.attribute(Mesh::ATTRIBUTE_UV_0) {
            Some(VertexAttributeValues::Float32x2(v)) if v.len() == source.len() => v,
            _ => return Err("Fence UV mismatch".into()),
        };
        let start = positions.len() as u32;
        for (p, t) in source.iter().zip(tex) {
            let p = matrix
                .transform_point3(DVec3::from_array(p.map(f64::from)))
                .as_vec3()
                .as_dvec3();
            positions.push((p * scale).as_vec3().to_array());
            uv.push(
                material
                    .uv_transform
                    .transform_point2(Vec2::from_array(*t))
                    .to_array(),
            );
        }
        let local: Vec<_> = mesh
            .indices()
            .map_or_else(|| (0..source.len()).collect(), |i| i.iter().collect());
        if !local.len().is_multiple_of(3) || local.iter().any(|i| *i >= source.len()) {
            return Err("Invalid fence indices".into());
        }
        indices.extend(local.into_iter().map(|i| start + i as u32));
    }
    let mut mesh = Mesh::new(
        PrimitiveTopology::TriangleList,
        RenderAssetUsages::MAIN_WORLD,
    )
    .with_inserted_attribute(Mesh::ATTRIBUTE_POSITION, positions)
    .with_inserted_attribute(Mesh::ATTRIBUTE_UV_0, uv)
    .with_inserted_attribute(Mesh::ATTRIBUTE_COLOR, vec![color; vertices])
    .with_inserted_indices(Indices::U32(indices));
    mesh.compute_normals();
    let bytes =
        mesh.get_vertex_buffer_size() + mesh.get_index_buffer_bytes().map_or(0, |v| v.len());
    Ok(Some(Fence {
        mesh,
        width: (high.x - low.x) * scale,
        texture: material.base_color_texture.clone(),
        surface: Vec4::new(material.perceptual_roughness, material.metallic, 0., 0.),
        bytes,
    }))
}
