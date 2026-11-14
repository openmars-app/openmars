//! Shared approved rock resources and stable per-fragment transforms.
use super::{debris::Fragment, debris_material::DebrisMaterial, timeline::Timeline};
use bevy::{
    camera::visibility::{NoFrustumCulling, RenderLayers},
    light::{NotShadowCaster, NotShadowReceiver},
    mesh::MeshTag,
    pbr::ExtendedMaterial,
    prelude::*,
};
use std::sync::Arc;
pub(super) struct Shared {
    pub meshes: Vec<Handle<Mesh>>,
    pub material: Handle<DebrisMaterial>,
}
pub(super) struct Instance {
    pub fragment: Fragment,
    pub root: Entity,
    pub children: Vec<Entity>,
}
pub(super) fn prepare(world: &mut World) -> Option<Arc<Shared>> {
    let (meshes, texture) = world
        .resource::<crate::vegetation::FoliageRuntime>()
        .rock(world)?;
    let atmosphere = world
        .resource::<crate::atmosphere::AtmosphereRuntime>()
        .binding()
        .ok()?;
    let material = world
        .resource_mut::<Assets<DebrisMaterial>>()
        .add(ExtendedMaterial {
            base: StandardMaterial {
                base_color: Color::WHITE,
                base_color_texture: Some(texture.clone()),
                emissive: LinearRgba::new(0.13 * 0.15, 0.07 * 0.15, 0.035 * 0.15, 1.),
                emissive_texture: Some(texture),
                perceptual_roughness: 1.,
                reflectance: 0.,
                metallic: 0.,
                ..default()
            },
            extension: super::debris_material::DebrisExtension { atmosphere },
        });
    Some(Arc::new(Shared { meshes, material }))
}
pub(super) fn bytes(shared: &Shared, count: usize) -> usize {
    // Descriptor/vector storage plus conservative ECS transforms, bounds,
    // visibility and mesh/material/tag state for every actual source part.
    count
        * (std::mem::size_of::<Fragment>()
            + std::mem::size_of::<Instance>()
            + (1 + shared.meshes.len()) * 512)
        + 4096
}
pub(super) fn spawn(
    world: &mut World,
    parent: Entity,
    fragments: Vec<Fragment>,
    shared: &Shared,
    parts: &mut Vec<(Entity, bool)>,
) -> Vec<Instance> {
    fragments
        .into_iter()
        .map(|fragment| {
            let root = world
                .spawn((Transform::IDENTITY, Visibility::Inherited, ChildOf(parent)))
                .id();
            let children = shared
                .meshes
                .iter()
                .map(|mesh| {
                    let entity = world
                        .spawn((
                            Mesh3d(mesh.clone()),
                            MeshMaterial3d(shared.material.clone()),
                            MeshTag::new(fragment.tag(false)),
                            Transform::IDENTITY,
                            Visibility::Inherited,
                            RenderLayers::layer(2),
                            NotShadowCaster,
                            NotShadowReceiver,
                            NoFrustumCulling,
                            ChildOf(root),
                        ))
                        .id();
                    parts.push((entity, false));
                    entity
                })
                .collect();
            Instance {
                fragment,
                root,
                children,
            }
        })
        .collect()
}
pub(super) fn update(
    world: &mut World,
    coordinate: (i32, i32, i32),
    instances: &[Instance],
    step: u64,
    reduced: bool,
    warm: bool,
) {
    let timeline = world.resource::<Timeline>();
    let live = timeline.live(coordinate);
    let start = timeline.start(coordinate);
    for instance in instances {
        let (pose, visible) = instance.fragment.pose(start.map(|s| s.step), step, reduced);
        let visible = warm && visible && (!live || start.is_some());
        let mut transform = world.get_mut::<Transform>(instance.root).unwrap();
        if *transform != pose {
            *transform = pose;
        }
        let tag = instance.fragment.tag(visible);
        for child in &instance.children {
            let mut value = world.get_mut::<MeshTag>(*child).unwrap();
            if value.value != tag {
                value.value = tag;
            }
        }
    }
}
