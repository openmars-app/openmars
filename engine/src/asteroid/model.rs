//! Keep the approved GLB hierarchy/materials and the original unshifted pivot.
use bevy::{
    camera::{
        primitives::Aabb,
        visibility::{NoFrustumCulling, RenderLayers},
    },
    light::{NotShadowCaster, NotShadowReceiver},
    prelude::*,
    world_serialization::WorldInstance,
};

pub(super) fn source(world: &World) -> Result<Option<Handle<WorldAsset>>, String> {
    let Some(handle) = crate::assets::model(world, "models/asteroid") else {
        return Ok(None);
    };
    if !world
        .resource::<AssetServer>()
        .is_loaded_with_dependencies(handle.id())
    {
        return Ok(None);
    }
    let models = world.resource::<Assets<bevy::gltf::Gltf>>();
    let model = models.get(&handle).ok_or("Missing asteroid model")?;
    if model.nodes.len() > 64
        || model.meshes.len() > 32
        || !model.skins.is_empty()
        || !model.animations.is_empty()
    {
        return Err("Static asteroid source hierarchy limit exceeded".into());
    }
    Ok(Some(
        model
            .default_scene
            .clone()
            .ok_or("Missing asteroid default scene")?,
    ))
}

pub(super) fn prepare(
    world: &mut World,
    root: Entity,
    materials: &mut crate::atmosphere::standard::SourceMaterials,
) -> Result<Option<(Vec<Entity>, f32)>, String> {
    let Some(instance) = world.get::<WorldInstance>(root) else {
        return Ok(None);
    };
    let spawner = world.resource::<WorldInstanceSpawner>();
    if !spawner.instance_is_ready(**instance) {
        return Ok(None);
    }
    let children: Vec<_> = spawner.iter_instance_entities(**instance).collect();
    if children.len() > 128 {
        return Err("Asteroid instance hierarchy limit exceeded".into());
    }
    let mut parts = Vec::new();
    let (mut low, mut high) = (Vec3::splat(f32::INFINITY), Vec3::splat(f32::NEG_INFINITY));
    for &child in &children {
        if world.get::<Mesh3d>(child).is_none() {
            continue;
        }
        let Some(bounds) = world.get::<Aabb>(child) else {
            return Ok(None);
        };
        let matrix = crate::nuclear::blast_geometry::current_matrix(world, child)
            .ok_or("Invalid asteroid hierarchy transform")?;
        let min: Vec3 = bounds.min().into();
        let max: Vec3 = bounds.max().into();
        for x in [min.x, max.x] {
            for y in [min.y, max.y] {
                for z in [min.z, max.z] {
                    let point = matrix.transform_point3(Vec3::new(x, y, z));
                    if !point.is_finite() {
                        return Err("Nonfinite asteroid bounds".into());
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
        return Err("Invalid asteroid geometry extent".into());
    }
    crate::atmosphere::standard::wrap_sources(world, &parts, materials)?;
    for &child in &parts {
        world.entity_mut(child).insert((
            RenderLayers::layer(2),
            NoFrustumCulling,
            NotShadowCaster,
            NotShadowReceiver,
        ));
    }
    // The old renderer scales the clone without centering its geometry.
    let scale = 0.15 / extent;
    world.entity_mut(root).insert((
        Transform::from_scale(Vec3::splat(scale)),
        Visibility::Inherited,
    ));
    Ok(Some((parts, scale)))
}
