//! Persistent empty cluster bindings for the engine's explicit sunlight and environment lighting.
use bevy::{
    light::cluster::GlobalClusterSettings,
    pbr::ViewClusterBindings,
    prelude::*,
    render::{RenderApp, render_resource::BindingResource},
};

// Direct sun and fill are evaluated by the shared material shader. Bevy's
// WebGL2 light array supports only one directional light; explicit evaluation
// preserves both lights, their BRDF and source decay=0 on every backend.
pub const SUN_INTENSITY: f32 = 0.65;
pub fn intensity(world: &World) -> f32 {
    SUN_INTENSITY * world.resource::<crate::entry::WorldEntrance>().light
}
pub fn ambient_radiance() -> f32 {
    Color::srgb_u8(42, 42, 42).to_linear().red * 0.4
}
pub fn sun_rgb() -> Vec3 {
    Vec3::new(1., 0.93, 0.85)
}
/// Artist-authored micro-normal light, distinct from the actual solar position.
pub fn sun_direction() -> Vec3 {
    Vec3::new(0.9, 0., 0.8).normalize()
}

pub fn configure(app: &mut App) {
    use bevy::ecs::schedule::{ScheduleCleanupPolicy, Schedules};
    use bevy::render::{Render, RenderSystems};
    app.init_resource::<EnvironmentViews>()
        .insert_resource(GlobalAmbientLight::NONE)
        .add_systems(First, configure_camera_clusters);
    app.world_mut()
        .resource_mut::<GlobalClusterSettings>()
        .gpu_clustering = None;
    let world = app.sub_app_mut(RenderApp).world_mut();
    world.init_resource::<ClusterAllocations>();
    world.resource_scope(|world, mut schedules: Mut<Schedules>| {
        let schedule = schedules.get_mut(Render).expect("Bevy render schedule");
        let removed = schedule
            .remove_systems_in_set(
                bevy::pbr::prepare_clusters_for_cpu_clustering,
                world,
                ScheduleCleanupPolicy::RemoveSystemsOnly,
            )
            .expect("Pinned Bevy CPU cluster preparation");
        assert_eq!(
            removed, 1,
            "Expected one Bevy CPU cluster preparation system"
        );
        schedule.add_systems(prepare_empty_clusters.in_set(RenderSystems::PrepareResources));
    });
}

type UnsupportedLights = Or<(
    With<PointLight>,
    With<SpotLight>,
    With<RectLight>,
    With<bevy::light::LightProbe>,
    With<bevy::light::IrradianceVolume>,
    With<bevy::light::cluster::ClusteredDecal>,
)>;

fn configure_camera_clusters(
    mut commands: Commands,
    cameras: Query<Entity, (With<Camera3d>, Without<EmptyClusters>)>,
    lights: Query<Entity, UnsupportedLights>,
) {
    assert!(
        lights.is_empty(),
        "Open Mars uses custom sunlight; clustered local lights require an explicit renderer change"
    );
    for entity in &cameras {
        commands
            .entity(entity)
            .insert((bevy::light::cluster::ClusterConfig::None, EmptyClusters));
    }
}
#[derive(Component)]
struct EmptyClusters;
#[derive(Resource, Default)]
struct ClusterAllocations(u64);
#[derive(Component)]
struct OwnedClusterBuffers([wgpu::Buffer; 2]);
impl Drop for OwnedClusterBuffers {
    fn drop(&mut self) {
        for buffer in &self.0 {
            buffer.destroy();
        }
    }
}
fn prepare_empty_clusters(
    mut commands: Commands,
    views: Query<
        Entity,
        (
            With<bevy::pbr::ExtractedClusterConfig>,
            Without<OwnedClusterBuffers>,
        ),
    >,
    pipeline: Res<bevy::pbr::MeshPipeline>,
    device: Res<bevy::render::renderer::RenderDevice>,
    queue: Res<bevy::render::renderer::RenderQueue>,
    mut allocations: ResMut<ClusterAllocations>,
) {
    for entity in &views {
        let mut bindings = ViewClusterBindings::new(pipeline.clustered_forward_buffer_binding_type);
        bindings.write_buffers(&device, &queue);
        let buffers = [
            bindings.clusterable_object_index_lists_binding(),
            bindings.offsets_and_counts_binding(),
        ]
        .map(|binding| {
            let Some(BindingResource::Buffer(binding)) = binding else {
                panic!("Missing empty cluster binding")
            };
            binding.buffer.clone()
        });
        allocations.0 += 2;
        commands
            .entity(entity)
            .insert((bindings, OwnedClusterBuffers(buffers)));
    }
}

#[derive(Resource, Default)]
struct EnvironmentViews(Vec<Entity>);
const ENVIRONMENT_MAPS: [(&str, u32, u32); 2] = [
    ("textures/skyboxes/starmap-diffuse", 32, 1),
    ("textures/skyboxes/starmap-specular", 128, 8),
];
/// Color warm views and the published view use the same physical environment.
/// Mask/bake cameras stay outside this audience, and scene exit drops all leases.
pub fn prepare_environment(app: &mut App) -> Result<(), String> {
    let _profile = crate::profiling::span(app.world(), "lighting.environment");
    use bevy::{
        camera::visibility::RenderLayers,
        light::EnvironmentMapLight,
        render::render_resource::{TextureFormat, TextureViewDimension},
    };
    let world = app.world_mut();
    if crate::assets::view(world) == openmars_scene::view::AssetView::Idle {
        let views = std::mem::take(&mut world.resource_mut::<EnvironmentViews>().0);
        for entity in views {
            if let Ok(mut e) = world.get_entity_mut(entity) {
                e.remove::<EnvironmentMapLight>();
            }
        }
        return Ok(());
    }
    let mut maps = Vec::new();
    for (id, size, mips) in ENVIRONMENT_MAPS {
        let Some(handle) = crate::assets::texture(world, id) else {
            return Ok(());
        };
        let Some(image) = world.resource::<Assets<Image>>().get(&handle) else {
            return Ok(());
        };
        let descriptor = &image.texture_descriptor;
        if descriptor.size.width != size
            || descriptor.size.height != size
            || descriptor.size.depth_or_array_layers != 6
            || descriptor.mip_level_count != mips
            || descriptor.format != TextureFormat::Rgba16Float
            || image
                .texture_view_descriptor
                .as_ref()
                .and_then(|v| v.dimension)
                != Some(TextureViewDimension::Cube)
        {
            return Err(format!("Invalid prepared lighting cube: {id}"));
        }
        maps.push(handle);
    }
    let views: Vec<_> = world
        .query_filtered::<(Entity, Option<&RenderLayers>), With<Camera3d>>()
        .iter(world)
        .filter(|(e, layers)| {
            world.get::<crate::assets::AssetCamera>(*e).is_some()
                || layers.is_some_and(|layers| layers == &RenderLayers::layer(2))
        })
        .map(|(e, _)| e)
        .collect();
    // Bevy applies camera exposure to IBL; the source EXR already stores the
    // intended scene radiance. Cancel that physical unit conversion once.
    let intensity = bevy::camera::Exposure::default().exposure().recip();
    for &entity in &views {
        if world.get::<EnvironmentMapLight>(entity).is_none_or(|old| {
            old.diffuse_map != maps[0] || old.specular_map != maps[1] || old.intensity != intensity
        }) {
            world.entity_mut(entity).insert(EnvironmentMapLight {
                diffuse_map: maps[0].clone(),
                specular_map: maps[1].clone(),
                intensity,
                ..default()
            });
        }
    }
    world.resource_mut::<EnvironmentViews>().0 = views;
    Ok(())
}
pub fn environment_stats(app: &App) -> serde_json::Value {
    use bevy::{
        light::EnvironmentMapLight,
        render::{render_asset::RenderAssets, texture::GpuImage},
    };
    let world = app.world();
    let render = app.sub_app(RenderApp).world();
    let Some(images) = render.get_resource::<RenderAssets<GpuImage>>() else {
        return serde_json::json!({"views":0,"maps":[],"background":null});
    };
    let maps:Vec<_>=ENVIRONMENT_MAPS.into_iter().filter_map(|(id,_,_)| {
        let handle=crate::assets::texture(world,id)?;let image=images.get(handle.id())?;
        let d=&image.texture_descriptor;
        let bytes:u64=(0..d.mip_level_count).map(|m|u64::from((d.size.width>>m).max(1))*u64::from((d.size.height>>m).max(1))*6*8).sum();
        Some(serde_json::json!({"id":id,"size":d.size.width,"mips":d.mip_level_count,"faces":d.size.depth_or_array_layers,"format":format!("{:?}",d.format),"bytes":bytes}))
    }).collect();
    let background_cpu_bytes = crate::assets::texture(world, "textures/skyboxes/starmap")
        .and_then(|h| world.resource::<Assets<Image>>().get(&h))
        .and_then(|i| i.data.as_ref())
        .map_or(0, Vec::len);
    let background=crate::assets::texture(world,"textures/skyboxes/starmap").and_then(|h|images.get(h.id())).map(|image| {
        let d=&image.texture_descriptor;
        let bytes:u64=(0..d.mip_level_count).map(|m|u64::from((d.size.width>>m).max(1))*u64::from((d.size.height>>m).max(1))*u64::from(d.size.depth_or_array_layers)*u64::from(d.format.block_copy_size(None).unwrap_or(0))).sum();
        serde_json::json!({"size":d.size.width,"mips":d.mip_level_count,"faces":d.size.depth_or_array_layers,"format":format!("{:?}",d.format),"bytes":bytes,"cpuBytes":background_cpu_bytes})
    });
    let views = world
        .resource::<EnvironmentViews>()
        .0
        .iter()
        .filter(|&&entity| {
            world.get::<Camera>(entity).is_some_and(|c| c.is_active)
                && world.get::<EnvironmentMapLight>(entity).is_some()
        })
        .count();
    serde_json::json!({"views":views,"maps":maps,"background":background,"clusterBufferAllocations":render.resource::<ClusterAllocations>().0,"clusterViews":render.iter_entities().filter(|e| e.contains::<OwnedClusterBuffers>()).count()})
}

#[cfg(test)]
mod cluster_tests {
    use super::*;
    #[test]
    fn every_new_3d_view_disables_unused_cluster_calculation() {
        let mut app = App::new();
        app.add_systems(Update, configure_camera_clusters);
        let first = app.world_mut().spawn(Camera3d::default()).id();
        app.update();
        assert!(matches!(
            app.world()
                .get::<bevy::light::cluster::ClusterConfig>(first),
            Some(bevy::light::cluster::ClusterConfig::None)
        ));
        let second = app.world_mut().spawn(Camera3d::default()).id();
        app.update();
        assert!(matches!(
            app.world()
                .get::<bevy::light::cluster::ClusterConfig>(second),
            Some(bevy::light::cluster::ClusterConfig::None)
        ));
        assert!(app.world().get::<EmptyClusters>(first).is_some());
        assert!(app.world().get::<EmptyClusters>(second).is_some());
    }
    #[test]
    #[should_panic(expected = "clustered local lights require an explicit renderer change")]
    fn cluster_dependent_lights_cannot_silently_lose_lighting() {
        let mut app = App::new();
        app.add_systems(Update, configure_camera_clusters);
        app.world_mut().spawn(PointLight::default());
        app.update();
    }
}
