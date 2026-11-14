//! Authored Phobos hierarchy shared by menu framing and game surface geometry.
use bevy::{
    camera::{
        primitives::Aabb,
        visibility::{NoFrustumCulling, RenderLayers},
    },
    light::{NotShadowCaster, NotShadowReceiver},
    prelude::*,
    world_serialization::WorldInstance,
};
pub fn source(world: &World) -> Result<Option<Handle<WorldAsset>>, String> {
    let Some(handle) = crate::assets::model(world, "models/phobos") else {
        return Ok(None);
    };
    if !world
        .resource::<AssetServer>()
        .is_loaded_with_dependencies(handle.id())
    {
        return Ok(None);
    }
    let models = world.resource::<Assets<bevy::gltf::Gltf>>();
    let source = models.get(&handle).ok_or("Missing Phobos source")?;
    if source.nodes.len() > 128
        || source.meshes.len() > 64
        || !source.skins.is_empty()
        || !source.animations.is_empty()
    {
        return Err("Invalid static Phobos hierarchy".into());
    }
    Ok(Some(
        source.default_scene.clone().ok_or("Missing Phobos scene")?,
    ))
}
pub struct Prepared {
    pub parts: Vec<Entity>,
    pub center: Vec3,
    pub scale: f32,
}
pub fn prepare(
    world: &mut World,
    root: Entity,
    materials: &mut crate::atmosphere::standard::SourceMaterials,
) -> Result<Option<Prepared>, String> {
    let Some(instance) = world.get::<WorldInstance>(root) else {
        return Ok(None);
    };
    let spawner = world.resource::<WorldInstanceSpawner>();
    if !spawner.instance_is_ready(**instance) {
        return Ok(None);
    }
    let children: Vec<_> = spawner.iter_instance_entities(**instance).collect();
    if children.len() > 256 {
        return Err("Phobos instance hierarchy limit exceeded".into());
    }
    let matrix =
        crate::nuclear::blast_geometry::current_matrix(world, root).ok_or("Invalid Phobos root")?;
    if matrix.determinant().abs() < 1e-12 {
        return Err("Singular Phobos root".into());
    }
    let inverse = matrix.inverse();
    let (mut low, mut high) = (Vec3::splat(f32::INFINITY), Vec3::splat(f32::NEG_INFINITY));
    let mut parts = Vec::new();
    for &child in &children {
        if world.get::<Mesh3d>(child).is_none() {
            continue;
        }
        let Some(aabb) = world.get::<Aabb>(child) else {
            return Ok(None);
        };
        let local = inverse
            * crate::nuclear::blast_geometry::current_matrix(world, child)
                .ok_or("Invalid Phobos node")?;
        let min: Vec3 = aabb.min().into();
        let max: Vec3 = aabb.max().into();
        for x in [min.x, max.x] {
            for y in [min.y, max.y] {
                for z in [min.z, max.z] {
                    let point = local.transform_point3(Vec3::new(x, y, z));
                    if !point.is_finite() {
                        return Err("Nonfinite Phobos bounds".into());
                    }
                    low = low.min(point);
                    high = high.max(point);
                }
            }
        }
        parts.push(child);
    }
    let extent = (high - low).max_element();
    if parts.is_empty()
        || parts.len() > 64
        || !extent.is_finite()
        || extent <= 0.
        || extent > 10000.
    {
        return Err("Invalid Phobos extent".into());
    }
    crate::atmosphere::standard::wrap_sources(world, &parts, materials)?;
    for &part in &parts {
        world.entity_mut(part).insert((
            NoFrustumCulling,
            NotShadowCaster,
            NotShadowReceiver,
            RenderLayers::layer(0).with(2),
        ));
    }
    Ok(Some(Prepared {
        parts,
        center: (low + high) * 0.5,
        scale: 0.32 / extent,
    }))
}

pub struct Shape {
    pub hull: Handle<Mesh>,
    pub radius: f64,
    pub content_radius: f64,
    pub points: Vec<Vec3>,
    pub indices: Vec<u32>,
}
impl Shape {
    /// Real mesh surface for ray picking, not the atmosphere's enclosing sphere.
    pub fn hit(&self, origin: Vec3, direction: Vec3) -> Option<f32> {
        let mut nearest = f32::INFINITY;
        for triangle in self.indices.as_chunks::<3>().0 {
            let [a, b, c] = [
                self.points[triangle[0] as usize],
                self.points[triangle[1] as usize],
                self.points[triangle[2] as usize],
            ];
            let e1 = b - a;
            let e2 = c - a;
            let p = direction.cross(e2);
            let determinant = e1.dot(p);
            if determinant.abs() < 1e-10 {
                continue;
            }
            let inverse = determinant.recip();
            let t = origin - a;
            let u = t.dot(p) * inverse;
            if !(0. ..=1.).contains(&u) {
                continue;
            }
            let q = t.cross(e1);
            let v = direction.dot(q) * inverse;
            if v < 0. || u + v > 1. {
                continue;
            }
            let distance = e2.dot(q) * inverse;
            if distance >= 0. {
                nearest = nearest.min(distance);
            }
        }
        nearest.is_finite().then_some(nearest)
    }
}
pub fn game_shape(world: &mut World, root: Entity, parts: &[Entity]) -> Result<Shape, String> {
    use bevy::{
        asset::RenderAssetUsages,
        mesh::{Indices, PrimitiveTopology, VertexAttributeValues},
    };
    let inverse = crate::nuclear::blast_geometry::current_matrix(world, root)
        .ok_or("Missing Phobos transform")?
        .inverse();
    let meshes = world.resource::<Assets<Mesh>>();
    let (mut points, mut normals, mut indices) = (Vec::new(), Vec::new(), Vec::new());
    for &part in parts {
        let handle = world.get::<Mesh3d>(part).ok_or("Missing Phobos mesh")?;
        let mesh = meshes
            .get(handle.id())
            .ok_or("Missing Phobos source mesh")?;
        if mesh.primitive_topology() != PrimitiveTopology::TriangleList {
            return Err("Phobos source is not triangles".into());
        }
        let Some(VertexAttributeValues::Float32x3(vertices)) =
            mesh.attribute(Mesh::ATTRIBUTE_POSITION)
        else {
            return Err("Missing Phobos vertices".into());
        };
        let Some(VertexAttributeValues::Float32x3(source_normals)) =
            mesh.attribute(Mesh::ATTRIBUTE_NORMAL)
        else {
            return Err("Missing Phobos normals".into());
        };
        if vertices.len() != source_normals.len() || points.len() + vertices.len() > 1_000_000 {
            return Err("Phobos mesh bound exceeded".into());
        }
        let matrix = inverse
            * crate::nuclear::blast_geometry::current_matrix(world, part)
                .ok_or("Missing Phobos node transform")?;
        let normal_matrix = matrix.inverse().transpose();
        let offset = points.len() as u32;
        for (p, n) in vertices.iter().zip(source_normals) {
            let point = matrix.transform_point3(Vec3::from_array(*p)) * 0.027;
            let normal = normal_matrix
                .transform_vector3(Vec3::from_array(*n))
                .normalize();
            if !point.is_finite() || !normal.is_finite() {
                return Err("Invalid Phobos surface".into());
            }
            points.push(point);
            normals.push(normal.to_array());
        }
        if let Some(source) = mesh.indices() {
            for i in source.iter() {
                if i >= vertices.len() {
                    return Err("Invalid Phobos index".into());
                }
                indices.push(offset + i as u32)
            }
        } else {
            indices.extend(offset..offset + vertices.len() as u32);
        }
        if indices.len() > 6_000_000 {
            return Err("Phobos triangle bound exceeded".into());
        }
    }
    if points.is_empty() || !indices.len().is_multiple_of(3) {
        return Err("Invalid Phobos topology".into());
    }
    let low = points
        .iter()
        .copied()
        .fold(Vec3::splat(f32::INFINITY), Vec3::min);
    let high = points
        .iter()
        .copied()
        .fold(Vec3::splat(f32::NEG_INFINITY), Vec3::max);
    let center = (low + high) * 0.5;
    let radius = points
        .iter()
        .map(|p| p.distance(center))
        .fold(0f32, f32::max)
        + center.length();
    if !radius.is_finite() || !(0.01..=10.).contains(&radius) {
        return Err("Invalid Phobos hull radius".into());
    }
    let mut shape = Shape {
        hull: Handle::default(),
        radius: f64::from(radius),
        content_radius: 0.,
        points,
        indices,
    };
    let hit = shape
        .hit(Vec3::Z * 100., -Vec3::Z)
        .ok_or("Phobos has no forward settlement surface")?;
    shape.content_radius = f64::from(100. - hit - 0.01);
    if shape.content_radius <= 0. || shape.content_radius > shape.radius {
        return Err("Invalid Phobos settlement radius".into());
    }
    let hull = Mesh::new(
        PrimitiveTopology::TriangleList,
        RenderAssetUsages::RENDER_WORLD,
    )
    .with_inserted_attribute(
        Mesh::ATTRIBUTE_POSITION,
        shape
            .points
            .iter()
            .map(|p| (*p / radius).to_array())
            .collect::<Vec<_>>(),
    )
    .with_inserted_attribute(Mesh::ATTRIBUTE_NORMAL, normals)
    .with_inserted_indices(Indices::U32(shape.indices.clone()));
    shape.hull = world.resource_mut::<Assets<Mesh>>().add(hull);
    Ok(shape)
}
