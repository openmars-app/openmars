//! Ecological flocks use real shared GLTF meshes and independent skeletons.
//! Admission and pose writes share the bounded, ordered rendering queue.
use crate::atmosphere::standard::{HazeMaterial, SourceMaterials};
mod flight;
use bevy::{
    camera::{
        RenderTarget,
        visibility::{NoFrustumCulling, RenderLayers},
    },
    core_pipeline::tonemapping::Tonemapping,
    gltf::{Gltf, GltfNode, GltfSkin},
    light::{NotShadowCaster, NotShadowReceiver},
    math::DVec3,
    mesh::skinning::SkinnedMesh,
    pbr::{PreparedMaterial, SkinUniforms},
    prelude::*,
    render::{
        RenderApp, erased_render_asset::ErasedRenderAssets, mesh::RenderMesh,
        render_asset::RenderAssets, render_resource::PipelineCache, sync_world::MainEntity,
    },
    world_serialization::{WorldAsset, WorldAssetRoot, WorldInstance, WorldInstanceSpawner},
};
use flight::{Flight, flight_transform, flights, sample_noise};
use openmars_scene::geometry::{AxialPoint, BodyProjection};
use std::collections::{BTreeMap, BTreeSet};
type Coordinate = (i32, i32, i32);
const MAX_BIRDS: usize = 512 * 6;
struct Model {
    scene: Handle<WorldAsset>,
    graph: Handle<AnimationGraph>,
    node: AnimationNodeIndex,
    duration: f64,
}
struct Bird {
    root: Entity,
    coordinate: Coordinate,
    index: usize,
    count: usize,
    flight: Flight,
    center: DVec3,
    player: Option<Entity>,
    skin: Option<Entity>,
    wing: f64,
    last: f64,
    warm_frames: u32,
    children: Vec<Entity>,
}
pub struct Input {
    pub flocks: BTreeSet<Coordinate>,
    pub projection: BodyProjection,
    pub step: u64,
    pub density: f64,
    pub warm: Handle<Image>,
}
#[derive(Resource, Default)]
pub struct FaunaRuntime {
    model: Option<Model>,
    materials: SourceMaterials,
    birds: BTreeMap<(Coordinate, usize), Bird>,
    desired: BTreeSet<Coordinate>,
    ready: bool,
    warm_camera: Option<Entity>,
    pub spawns: u64,
}
pub fn is_bird_tile(kind: &str) -> bool {
    matches!(kind, "ecological-zone-tile" | "natural-preserve-tile")
}
impl FaunaRuntime {
    pub fn ready(&self) -> bool {
        self.ready
    }
    pub fn progress(&self) -> Option<String> {
        (!self.ready).then(|| format!("Preparing birds ({} skeletons)", self.birds.len()))
    }
    pub fn diagnostics(&self, world: &World, render: &World) -> serde_json::Value {
        let skins = render.get_resource::<SkinUniforms>();
        serde_json::json!({
            "ready":self.ready,"birds":self.birds.len(),"spawns":self.spawns,
            "graphs":world.resource::<Assets<AnimationGraph>>().len(),
            "clips":world.resource::<Assets<AnimationClip>>().len(),
            "worldAssets":world.resource::<Assets<WorldAsset>>().len(),
            "inverseBindposes":world.resource::<Assets<bevy::mesh::skinning::SkinnedMeshInverseBindposes>>().len(),
            "flocks":self.desired.len(),"clipDuration":self.model.as_ref().map(|m|m.duration),
            "gpuSkins":skins.map(|s|s.all_skins().count()).unwrap_or(0),
            "skinBufferIds":skins.map(|s|[format!("{:?}",s.current_buffer.id()),format!("{:?}",s.prev_buffer.id())]),
            "skinBufferBytes":skins.map(|s|s.current_buffer.size()+s.prev_buffer.size()).unwrap_or(0),
            "sample":self.birds.values().take(32).map(|b| {
                let joints=b.skin.and_then(|e|world.get::<SkinnedMesh>(e)).map(|s|s.joints.iter().filter_map(|e|world.get::<Transform>(*e).map(|t|t.rotation.to_array())).collect::<Vec<_>>());
                serde_json::json!({"tile":b.coordinate,"index":b.index,"root":b.root.to_string(),"player":b.player.map(|e|e.to_string()),"skin":b.skin.map(|e|e.to_string()),"wing":b.wing,"joints":joints,"position":world.get::<Transform>(b.root).map(|t|t.translation.to_array()),"visible":world.get::<Visibility>(b.root).is_some_and(|v|*v!=Visibility::Hidden),"warmFrames":b.warm_frames})
            }).collect::<Vec<_>>()
        })
    }
    fn model(&mut self, world: &mut World) -> Result<bool, String> {
        if self.model.is_some() {
            return Ok(true);
        }
        let Some(handle) = crate::assets::model(world, "models/bird") else {
            return Ok(false);
        };
        let models = world.resource::<Assets<Gltf>>();
        let Some(model) = models.get(&handle) else {
            return Ok(false);
        };
        if model.nodes.len() > 64
            || model.meshes.len() != 1
            || model.skins.len() != 1
            || model.animations.len() != 1
            || !model.named_animations.contains_key("Take 001")
        {
            return Err("Bird GLTF does not match the approved animated profile".into());
        }
        let nodes = world.resource::<Assets<GltfNode>>();
        if model.nodes.iter().any(|h| {
            nodes
                .get(h)
                .is_none_or(|n| !n.transform.to_matrix().is_finite())
        }) {
            return Err("Invalid bird transform hierarchy".into());
        }
        let skin = world
            .resource::<Assets<GltfSkin>>()
            .get(&model.skins[0])
            .ok_or("Missing bird skeleton")?;
        if skin.joints.len() != 7 {
            return Err("Invalid bird joint count".into());
        }
        let clip = model.animations[0].clone();
        let duration = f64::from(
            world
                .resource::<Assets<AnimationClip>>()
                .get(&clip)
                .ok_or("Missing bird wing animation")?
                .duration(),
        );
        if !(0.01..=60.).contains(&duration) {
            return Err("Invalid bird clip duration".into());
        }
        let scene = model
            .default_scene
            .clone()
            .ok_or("Missing bird default scene")?;
        let (graph, node) = AnimationGraph::from_clip(clip);
        let graph = world.resource_mut::<Assets<AnimationGraph>>().add(graph);
        self.model = Some(Model {
            scene,
            graph,
            node,
            duration,
        });
        Ok(true)
    }
    fn update(
        &mut self,
        world: &mut World,
        input: Input,
        uploaded: &BTreeSet<Entity>,
    ) -> Result<(), String> {
        self.desired = input.flocks;
        if self.desired.len() > MAX_BIRDS / 6 {
            return Err("Bird flock limit exceeded".into());
        }
        let removed: Vec<_> = self
            .birds
            .keys()
            .filter(|(c, _)| !self.desired.contains(c))
            .copied()
            .collect();
        for key in removed {
            if let Some(b) = self.birds.remove(&key) {
                world.despawn(b.root);
            }
        }
        if self.desired.is_empty() {
            if let Some(camera) = self.warm_camera.take() {
                world.despawn(camera);
            }
            self.model = None;
            self.materials.clear();
            self.ready = true;
            return Ok(());
        }
        self.ready = false;
        if !self.model(world)? {
            return Ok(());
        }
        let model = self.model.as_ref().unwrap();
        let t = input.step as f64 / 60.;
        // Do not queue scene clones while a previous clone is still spawning.
        let waiting = self.birds.values().any(|b| b.player.is_none());
        if !waiting {
            'admit: for &coordinate in &self.desired {
                let flock = flights(coordinate, model.duration);
                let count = flock.len();
                for (index, flight) in flock.into_iter().enumerate() {
                    let key = (coordinate, index);
                    if self.birds.contains_key(&key) {
                        continue;
                    }
                    let center = DVec3::from_array(
                        input
                            .projection
                            .project_content(
                                input.projection.hex_center(AxialPoint {
                                    q: coordinate.0,
                                    r: coordinate.1,
                                }),
                                0.,
                            )
                            .map_err(|e| e.to_string())?,
                    );
                    let root = world
                        .spawn((
                            WorldAssetRoot(model.scene.clone()),
                            Transform::IDENTITY,
                            Visibility::Hidden,
                        ))
                        .id();
                    self.birds.insert(
                        key,
                        Bird {
                            root,
                            coordinate,
                            index,
                            count,
                            flight,
                            center,
                            player: None,
                            skin: None,
                            wing: flight.animation_start,
                            last: t,
                            warm_frames: 0,
                            children: Vec::new(),
                        },
                    );
                    self.spawns += 1;
                    break 'admit;
                }
            }
        }
        let noise = crate::assets::texture(world, "textures/terrain/noise-mid")
            .ok_or("Missing bird noise source")?;
        // Clone the asset handle, never the image's CPU pixels. The existing raw
        // PNG remains top-first; the generated facade texture is flipped.
        for bird in self.birds.values_mut() {
            if bird.player.is_none() {
                let Some(instance) = world.get::<WorldInstance>(bird.root) else {
                    continue;
                };
                let spawner = world.resource::<WorldInstanceSpawner>();
                if !spawner.instance_is_ready(**instance) {
                    continue;
                }
                let children: Vec<_> = spawner.iter_instance_entities(**instance).collect();
                if children.len() > 64 {
                    return Err("Bird instance node limit exceeded".into());
                }
                let players: Vec<_> = children
                    .iter()
                    .copied()
                    .filter(|e| world.get::<AnimationPlayer>(*e).is_some())
                    .collect();
                let skins: Vec<_> = children
                    .iter()
                    .copied()
                    .filter(|e| world.get::<SkinnedMesh>(*e).is_some())
                    .collect();
                if players.len() != 1 || skins.len() != 1 {
                    return Err("Bird instance lacks an independent animated skeleton".into());
                }
                crate::atmosphere::standard::wrap_sources(world, &children, &mut self.materials)?;
                for &child in &children {
                    world.entity_mut(child).insert((
                        RenderLayers::layer(2),
                        NoFrustumCulling,
                        NotShadowCaster,
                        NotShadowReceiver,
                    ));
                }
                bird.children = children;
                bird.player = Some(players[0]);
                bird.skin = Some(skins[0]);
                world
                    .entity_mut(players[0])
                    .insert(AnimationGraphHandle(model.graph.clone()));
                world
                    .get_mut::<AnimationPlayer>(players[0])
                    .unwrap()
                    .play(model.node)
                    .repeat()
                    .pause()
                    .set_seek_time(bird.wing as f32);
            }
            let visible = bird.index < (bird.count as f64 * input.density).round() as usize;
            if visible {
                bird.wing = (bird.wing + (t - bird.last).max(0.) * bird.flight.animation_speed)
                    .rem_euclid(model.duration);
            }
            bird.last = t;
            let transform = {
                let images = world.resource::<Assets<Image>>();
                let image = images.get(&noise).ok_or("Bird noise has not decoded")?;
                let size = image.texture_descriptor.size;
                let pixels = image.data.as_ref().ok_or("Bird noise CPU data missing")?;
                if size.width == 0
                    || size.height == 0
                    || size.width > 2048
                    || size.height > 2048
                    || size.depth_or_array_layers != 1
                    || !matches!(
                        image.texture_descriptor.format,
                        bevy::render::render_resource::TextureFormat::Rgba8Unorm
                            | bevy::render::render_resource::TextureFormat::Rgba8UnormSrgb
                    )
                    || pixels.len() != size.width as usize * size.height as usize * 4
                {
                    return Err("Invalid bird noise profile".into());
                }
                flight_transform(bird.flight, bird.center, t, |u, v| {
                    sample_noise(pixels, size.width, size.height, u, v)
                })
            };
            *world.get_mut::<Transform>(bird.root).unwrap() = transform;
            world
                .get_mut::<AnimationPlayer>(bird.player.unwrap())
                .unwrap()
                .animation_mut(model.node)
                .ok_or("Bird animation stopped")?
                .set_seek_time(bird.wing as f32);
            if uploaded.contains(&bird.root) {
                bird.warm_frames = (bird.warm_frames + 1).min(2);
            }
            let warm = bird.warm_frames < 2;
            *world.get_mut::<Visibility>(bird.root).unwrap() = if warm || visible {
                Visibility::Inherited
            } else {
                Visibility::Hidden
            };
            for &child in &bird.children {
                let layers = RenderLayers::layer(if warm { 2 } else { 0 });
                if world.get::<RenderLayers>(child) != Some(&layers) {
                    world.entity_mut(child).insert(layers);
                }
            }
        }
        let warming = self.birds.values().any(|b| b.warm_frames < 2);
        if warming && self.warm_camera.is_none() {
            let camera = world
                .spawn((
                    crate::terrain::surface::SurfaceCamera::Warm,
                    Camera3d::default(),
                    bevy::core_pipeline::prepass::DepthPrepass,
                    Camera {
                        order: -5,
                        ..default()
                    },
                    RenderTarget::Image(input.warm.into()),
                    Msaa::Off,
                    Tonemapping::None,
                    RenderLayers::layer(2),
                    Transform::from_xyz(0., 0., 6.8).looking_at(Vec3::ZERO, Vec3::Y),
                ))
                .id();
            self.warm_camera = Some(camera);
        } else if !warming && let Some(camera) = self.warm_camera.take() {
            world.despawn(camera);
        }
        let expected: usize = self
            .desired
            .iter()
            .map(|c| flights(*c, model.duration).len())
            .sum();
        self.ready =
            self.birds.len() == expected && self.birds.values().all(|b| b.warm_frames >= 2);
        Ok(())
    }
}
#[derive(Resource)]
struct PaletteCache;
pub fn prepare_queued(app: &mut App, input: Option<Input>) -> Result<(), String> {
    let _profile = crate::profiling::span(app.world(), "fauna.prepare");
    let mut runtime = app
        .world_mut()
        .remove_resource::<FaunaRuntime>()
        .ok_or("Missing fauna runtime")?;
    let result = if let Some(input) = input {
        if !input.flocks.is_empty()
            && !app
                .sub_app(RenderApp)
                .world()
                .contains_resource::<PaletteCache>()
        {
            // Bevy normally grows skin buffers during extraction. Reserve the
            // bounded maximum once, after prior uses are submitted, and retire its two
            // initial buffers explicitly. This renderer cache survives clears.
            let render = app.sub_app_mut(RenderApp).world_mut();
            let device = render
                .resource::<bevy::render::renderer::RenderDevice>()
                .clone();
            if let Some(mut skins) = render.get_resource_mut::<SkinUniforms>() {
                let usage = if bevy::pbr::skins_use_uniform_buffers(&device.limits()) {
                    bevy::render::render_resource::BufferUsages::UNIFORM
                } else {
                    bevy::render::render_resource::BufferUsages::STORAGE
                } | bevy::render::render_resource::BufferUsages::COPY_DST;
                let make = || {
                    device.create_buffer(&bevy::render::render_resource::BufferDescriptor {
                        label: Some("Open Mars bounded bird joint cache"),
                        size: ((MAX_BIRDS * 8 + bevy::pbr::MAX_JOINTS) * 64) as u64,
                        usage,
                        mapped_at_creation: false,
                    })
                };
                skins.current_buffer.destroy();
                skins.prev_buffer.destroy();
                skins.current_buffer = make();
                skins.prev_buffer = make();
                render.insert_resource(PaletteCache);
            }
        }
        let render = app.sub_app(RenderApp).world();
        let mut uploaded = BTreeSet::new();
        if render
            .resource::<PipelineCache>()
            .waiting_pipelines()
            .next()
            .is_none()
            && let (Some(meshes), Some(materials), Some(skins)) = (
                render.get_resource::<RenderAssets<RenderMesh>>(),
                render.get_resource::<ErasedRenderAssets<PreparedMaterial>>(),
                render.get_resource::<SkinUniforms>(),
            )
        {
            for bird in runtime.birds.values() {
                if let Some(e) = bird.skin
                    && let Some(mesh) = app.world().get::<Mesh3d>(e)
                    && let Some(material) = app.world().get::<MeshMaterial3d<HazeMaterial>>(e)
                    && meshes.get(mesh.id()).is_some()
                    && materials.get(material.id().untyped()).is_some()
                    && skins.skin_index(MainEntity::from(e)).is_some()
                {
                    uploaded.insert(bird.root);
                }
            }
        }
        runtime.update(app.world_mut(), input, &uploaded)
    } else {
        Ok(())
    };
    app.world_mut().insert_resource(runtime);
    result
}
pub fn reset(world: &mut World) {
    let Some(mut runtime) = world.remove_resource::<FaunaRuntime>() else {
        return;
    };
    for (_, bird) in std::mem::take(&mut runtime.birds) {
        world.despawn(bird.root);
    }
    if let Some(camera) = runtime.warm_camera.take() {
        world.despawn(camera);
    }
    world.insert_resource(FaunaRuntime {
        spawns: runtime.spawns,
        ..default()
    });
}
