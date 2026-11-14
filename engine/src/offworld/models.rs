//! Authored hierarchy preparation and the source merged .096-unit ship hull.
use bevy::{
    asset::RenderAssetUsages,
    mesh::{Indices, PrimitiveTopology, VertexAttributeValues},
    prelude::*,
    world_serialization::WorldInstance,
};

pub fn source(world: &World, id: &str) -> Result<Option<Handle<WorldAsset>>, String> {
    let Some(handle) = crate::assets::model(world, id) else {
        return Ok(None);
    };
    if !world
        .resource::<AssetServer>()
        .is_loaded_with_dependencies(handle.id())
    {
        return Ok(None);
    }
    let models = world.resource::<Assets<bevy::gltf::Gltf>>();
    let model = models.get(&handle).ok_or("Missing offworld model")?;
    if model.nodes.len() > 128
        || model.meshes.len() > 64
        || !model.skins.is_empty()
        || !model.animations.is_empty()
    {
        return Err("Invalid static offworld hierarchy".into());
    }
    Ok(Some(
        model
            .default_scene
            .clone()
            .ok_or("Missing offworld default scene")?,
    ))
}

fn hierarchy(world: &World, root: Entity) -> Result<Option<Vec<Entity>>, String> {
    let Some(instance) = world.get::<WorldInstance>(root) else {
        return Ok(None);
    };
    if !world
        .resource::<WorldInstanceSpawner>()
        .instance_is_ready(**instance)
    {
        return Ok(None);
    }
    let mut result = Vec::new();
    let mut stack = vec![root];
    while let Some(entity) = stack.pop() {
        if result.len() >= 256 {
            return Err("Offworld hierarchy limit exceeded".into());
        }
        result.push(entity);
        if let Some(children) = world.get::<Children>(entity) {
            stack.extend(children.iter().rev());
        }
    }
    Ok(Some(result))
}

/// Remove the source backdrop/text before allowing the satellite to render.
pub fn station(
    world: &mut World,
    root: Entity,
    materials: &mut crate::atmosphere::standard::SourceMaterials,
) -> Result<Option<Vec<Entity>>, String> {
    let Some(children) = hierarchy(world, root)? else {
        return Ok(None);
    };
    for &entity in &children {
        let Some(name) = world.get::<Name>(entity) else {
            continue;
        };
        if matches!(name.as_str(), "Earth" | "Text") {
            world.despawn(entity);
        } else if name.as_str() == "Satlite" {
            world
                .get_mut::<Transform>(entity)
                .ok_or("Satellite transform absent")?
                .translation = Vec3::ZERO;
        }
    }
    let Some(children) = hierarchy(world, root)? else {
        return Ok(None);
    };
    let parts: Vec<_> = children
        .into_iter()
        .filter(|e| world.get::<Mesh3d>(*e).is_some())
        .collect();
    if parts.is_empty() || parts.len() > 64 {
        return Err("Invalid satellite mesh count".into());
    }
    crate::atmosphere::standard::wrap_sources(world, &parts, materials)?;
    // MeshPhysicalMaterial derives dielectric F0 from IOR. Bevy keeps IOR and
    // reflectance independent; preserve the source GLB's physical reflectance.
    for (original, handle) in materials.iter() {
        let source = world
            .resource::<Assets<StandardMaterial>>()
            .get(*original)
            .ok_or("Missing satellite source material")?;
        let reflectance = physical_reflectance(source.reflectance, source.ior)?;
        let mut assets = world.resource_mut::<Assets<crate::atmosphere::standard::HazeMaterial>>();
        if assets
            .get(handle)
            .ok_or("Missing satellite haze material")?
            .base
            .reflectance
            != reflectance
        {
            assets.get_mut(handle).unwrap().base.reflectance = reflectance;
        }
    }
    Ok(Some(parts))
}

pub fn physical_reflectance(reflectance: f32, ior: f32) -> Result<f32, String> {
    if !reflectance.is_finite() || !ior.is_finite() || reflectance < 0. || ior < 1. {
        return Err("Invalid physical source material".into());
    }
    Ok(reflectance * ((ior - 1.) / (ior + 1.)) / 0.2)
}

pub struct HullPart<'a> {
    pub mesh: &'a Mesh,
    pub transform: Mat4,
}

/// Three's Box3 local-AABB bounds, separate f32 writes, Y flip and new normals.
pub fn hull(parts: &[HullPart<'_>]) -> Result<Mesh, String> {
    if parts.is_empty() || parts.len() > 64 {
        return Err("Invalid ship mesh count".into());
    }
    let (mut low, mut high) = (Vec3::splat(f32::INFINITY), Vec3::splat(f32::NEG_INFINITY));
    let mut count = 0usize;
    for part in parts {
        if part.mesh.primitive_topology() != PrimitiveTopology::TriangleList
            || !part.transform.is_finite()
        {
            return Err("Invalid ship source geometry".into());
        }
        let Some(VertexAttributeValues::Float32x3(vertices)) =
            part.mesh.attribute(Mesh::ATTRIBUTE_POSITION)
        else {
            return Err("Missing ship positions".into());
        };
        count += vertices.len();
        if vertices.is_empty() || count > 1_000_000 {
            return Err("Ship vertex bound exceeded".into());
        }
        let (mut a, mut b) = (Vec3::splat(f32::INFINITY), Vec3::splat(f32::NEG_INFINITY));
        for vertex in vertices {
            let point = Vec3::from_array(*vertex);
            if !point.is_finite() {
                return Err("Nonfinite ship vertex".into());
            }
            a = a.min(point);
            b = b.max(point);
        }
        for x in [a.x, b.x] {
            for y in [a.y, b.y] {
                for z in [a.z, b.z] {
                    let point = part.transform.transform_point3(Vec3::new(x, y, z));
                    low = low.min(point);
                    high = high.max(point);
                }
            }
        }
    }
    let extent = (high - low).max_element();
    if !extent.is_finite() || extent <= 0. {
        return Err("Invalid ship source extent".into());
    }
    let center = (high + low) * 0.5;
    let scale = 0.096 / extent;
    let rotation = Mat4::from_rotation_y(std::f32::consts::PI);
    let (mut positions, mut uv, mut indices) = (
        Vec::with_capacity(count),
        Vec::with_capacity(count),
        Vec::new(),
    );
    for part in parts {
        let VertexAttributeValues::Float32x3(vertices) =
            part.mesh.attribute(Mesh::ATTRIBUTE_POSITION).unwrap()
        else {
            unreachable!()
        };
        let texture = match part.mesh.attribute(Mesh::ATTRIBUTE_UV_0) {
            Some(VertexAttributeValues::Float32x2(values)) if values.len() == vertices.len() => {
                Some(values)
            }
            None => None,
            _ => return Err("Invalid ship UVs".into()),
        };
        let offset = positions.len() as u32;
        for (i, vertex) in vertices.iter().enumerate() {
            let world = part.transform.transform_point3(Vec3::from_array(*vertex));
            let centered = world - center;
            let scaled = centered * scale;
            positions.push(rotation.transform_point3(scaled).to_array());
            uv.push(texture.map_or([0.; 2], |values| values[i]));
        }
        let source: Vec<_> = part
            .mesh
            .indices()
            .map_or_else(|| (0..vertices.len()).collect(), |i| i.iter().collect());
        if source.len() % 3 != 0 || source.iter().any(|i| *i >= vertices.len()) {
            return Err("Invalid ship triangles".into());
        }
        indices.extend(source.into_iter().map(|i| offset + i as u32));
    }
    let mut mesh = Mesh::new(
        PrimitiveTopology::TriangleList,
        RenderAssetUsages::MAIN_WORLD | RenderAssetUsages::RENDER_WORLD,
    )
    .with_inserted_attribute(Mesh::ATTRIBUTE_POSITION, positions)
    .with_inserted_attribute(Mesh::ATTRIBUTE_UV_0, uv)
    .with_inserted_indices(Indices::U32(indices));
    mesh.compute_area_weighted_normals();
    Ok(mesh)
}

pub fn ship(world: &World, root: Entity) -> Result<Option<Mesh>, String> {
    let Some(children) = hierarchy(world, root)? else {
        return Ok(None);
    };
    let matrix =
        crate::nuclear::blast_geometry::current_matrix(world, root).ok_or("Invalid ship root")?;
    if matrix.determinant().abs() < 1e-12 {
        return Err("Singular ship root".into());
    }
    let inverse = matrix.inverse();
    let meshes = world.resource::<Assets<Mesh>>();
    let mut parts = Vec::new();
    for entity in children {
        if let Some(handle) = world.get::<Mesh3d>(entity) {
            let Some(mesh) = meshes.get(handle.id()) else {
                return Ok(None);
            };
            parts.push(HullPart {
                mesh,
                transform: inverse
                    * crate::nuclear::blast_geometry::current_matrix(world, entity)
                        .ok_or("Invalid ship hierarchy")?,
            });
        }
    }
    Ok(Some(hull(&parts)?))
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn merged_hull_centers_all_axes_scales_source_bounds_and_recomputes_normals() {
        let triangle = Mesh::new(
            PrimitiveTopology::TriangleList,
            RenderAssetUsages::MAIN_WORLD,
        )
        .with_inserted_attribute(
            Mesh::ATTRIBUTE_POSITION,
            vec![[0., 0., 0.], [2., 0., 0.], [0., 1., 0.]],
        )
        .with_inserted_attribute(Mesh::ATTRIBUTE_UV_0, vec![[0., 0.], [1., 0.], [0., 1.]])
        .with_inserted_indices(Indices::U32(vec![0, 1, 2]));
        let mesh = hull(&[HullPart {
            mesh: &triangle,
            transform: Mat4::from_translation(Vec3::new(7., -3., 2.)),
        }])
        .unwrap();
        let VertexAttributeValues::Float32x3(p) = mesh.attribute(Mesh::ATTRIBUTE_POSITION).unwrap()
        else {
            panic!()
        };
        assert!((p[0][0] - 0.048).abs() < 1e-7);
        assert!((p[0][1] + 0.024).abs() < 1e-7);
        assert!(p.iter().all(|p| p[2].abs() < 1e-7));
        assert_eq!(mesh.indices().unwrap().len(), 3);
        let VertexAttributeValues::Float32x3(n) = mesh.attribute(Mesh::ATTRIBUTE_NORMAL).unwrap()
        else {
            panic!()
        };
        assert!(
            n.iter()
                .all(|n| (Vec3::from_array(*n) + Vec3::Z).length() < 1e-6)
        );
        assert!(hull(&[]).is_err());
    }
    #[test]
    fn authored_ior_maps_to_the_original_dielectric_fresnel() {
        assert_eq!(physical_reflectance(0.5, 1.5).unwrap(), 0.5);
        let value = physical_reflectance(0.5, 1.45).unwrap();
        let f0 = 0.16 * value * value;
        assert!((f0 - ((1.45_f32 - 1.) / (1.45 + 1.)).powi(2)).abs() < 1e-7);
        assert!(physical_reflectance(0.5, f32::NAN).is_err());
    }
}
