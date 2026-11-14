//! Public-log asteroid presentation, using the approved source model.
use crate::atmosphere::standard::{HazeMaterial, SourceMaterials};
pub mod material;
mod model;
use bevy::{
    camera::{
        RenderTarget,
        visibility::{NoFrustumCulling, RenderLayers},
    },
    core_pipeline::tonemapping::Tonemapping,
    light::{NotShadowCaster, NotShadowReceiver},
    mesh::MeshTag,
    pbr::PreparedMaterial,
    prelude::*,
    render::{
        RenderApp, erased_render_asset::ErasedRenderAssets, mesh::RenderMesh,
        render_asset::RenderAssets, render_resource::PipelineCache,
    },
};
use material::AsteroidDustMaterial;
use motion::{FLIGHT_STEPS, Flight, TOTAL_STEPS};

pub struct Input {
    pub step: u64,
    pub radius: f64,
    pub quiet: bool,
    pub reduced: bool,
    pub warm: Handle<Image>,
}
struct Shared {
    materials: SourceMaterials,
    scene: Handle<WorldAsset>,
    mesh: Handle<Mesh>,
    material: Handle<AsteroidDustMaterial>,
}
struct Active {
    id: String,
    root: Entity,
    model: Entity,
    dust: Entity,
    sprites: Vec<Entity>,
    parts: Vec<Entity>,
    flight: Flight,
    start: Option<u64>,
    warm: u8,
    scale: f32,
}
#[derive(Resource, Default)]
pub struct AsteroidRuntime {
    shared: Option<Shared>,
    retired: Vec<(Shared, u8)>,
    active: Option<Active>,
    camera: Option<Entity>,
    started: u64,
    completed: u64,
    canceled: u64,
}
impl AsteroidRuntime {
    pub fn diagnostics(&self, world: &World) -> serde_json::Value {
        serde_json::json!({"started":self.started,"completed":self.completed,"canceled":self.canceled,"retired":self.retired.len(),"shared":self.shared.is_some(),"active":self.active.as_ref().map(|a|serde_json::json!({"id":a.id,"root":a.root.to_string(),"start":a.start,"warmFrames":a.warm,"scale":a.scale,"parts":a.parts.len(),"sprites":a.sprites.len(),"impact":a.flight.impact.to_array(),"position":world.get::<Transform>(a.root).map(|t|t.translation.to_array()),"dustSample":a.sprites.iter().take(3).map(|e|serde_json::json!({"entity":e.to_string(),"position":world.get::<Transform>(*e).map(|t|t.translation.to_array()),"tag":world.get::<MeshTag>(*e).map(|t|t.value)})).collect::<Vec<_>>()}))})
    }
    fn park(&mut self, world: &mut World) {
        if let Some(a) = self.active.take() {
            world.despawn(a.root);
            world.despawn(a.dust);
            self.canceled += 1;
        }
        if let Some(e) = self.camera.take() {
            world.despawn(e);
        }
        if let Some(shared) = self.shared.take() {
            self.retired.push((shared, 2));
        }
    }
    fn advance(
        &mut self,
        world: &mut World,
        input: Option<Input>,
        uploaded: bool,
    ) -> Result<(), String> {
        for (_, frames) in &mut self.retired {
            *frames = frames.saturating_sub(1);
        }
        self.retired.retain(|(_, frames)| *frames > 0);
        let Some(input) = input.filter(|i| !i.quiet && !i.reduced) else {
            self.park(world);
            world
                .resource_mut::<crate::presentation::PresentationRuntime>()
                .discard_pending();
            return Ok(());
        };
        if self.active.as_ref().is_some_and(|a| {
            a.start
                .is_some_and(|s| input.step.saturating_sub(s) >= TOTAL_STEPS)
        }) {
            let a = self.active.take().unwrap();
            world.despawn(a.root);
            world.despawn(a.dust);
            self.completed += 1;
        }
        if self.active.is_none()
            && !world
                .resource::<crate::presentation::PresentationRuntime>()
                .pending()
        {
            self.park(world);
            return Ok(());
        }
        if self.shared.is_none() {
            let Some(scene) = model::source(world)? else {
                return Ok(());
            };
            let Some(image) = world
                .resource::<crate::structures::texture_library::TextureLibrary>()
                .get(crate::structures::textures::Slot::Shared(5))
            else {
                return Ok(());
            };
            let mesh = world
                .resource_mut::<Assets<Mesh>>()
                .add(Mesh::from(Rectangle::new(0.2, 0.2)));
            let material = world
                .resource_mut::<Assets<AsteroidDustMaterial>>()
                .add(AsteroidDustMaterial { image });
            self.shared = Some(Shared {
                materials: SourceMaterials::new(),
                scene,
                mesh,
                material,
            });
        }
        if self.active.is_none() {
            let cue = world
                .resource_mut::<crate::presentation::PresentationRuntime>()
                .pop()
                .unwrap();
            let flight = Flight::new(cue.seed, input.radius, 60)?;
            let root = world
                .spawn((Transform::IDENTITY, Visibility::Inherited))
                .id();
            let shared = self.shared.as_ref().unwrap();
            let model = world
                .spawn((
                    WorldAssetRoot(shared.scene.clone()),
                    Transform::IDENTITY,
                    Visibility::Hidden,
                    ChildOf(root),
                ))
                .id();
            let dust = world
                .spawn((
                    Transform::from_translation(flight.impact.as_vec3()),
                    Visibility::Inherited,
                ))
                .id();
            let sprites = (0..60)
                .map(|i| {
                    world
                        .spawn((
                            Mesh3d(shared.mesh.clone()),
                            MeshMaterial3d(shared.material.clone()),
                            MeshTag::new(0x80000000),
                            Transform::IDENTITY,
                            RenderLayers::layer(2),
                            NoFrustumCulling,
                            NotShadowCaster,
                            NotShadowReceiver,
                            crate::transparent::TransparentOrder::particle([
                                0,
                                0,
                                0,
                                32,
                                self.started as i64,
                                i,
                            ]),
                            ChildOf(dust),
                        ))
                        .id()
                })
                .collect();
            self.active = Some(Active {
                id: cue.id,
                root,
                model,
                dust,
                sprites,
                parts: Vec::new(),
                flight,
                start: None,
                warm: 0,
                scale: 0.,
            });
            self.started += 1;
        }
        let a = self.active.as_mut().unwrap();
        if a.parts.is_empty() {
            if let Some((parts, scale)) =
                model::prepare(world, a.model, &mut self.shared.as_mut().unwrap().materials)?
            {
                a.parts = parts;
                a.scale = scale;
            } else {
                return Ok(());
            }
        }
        if a.warm < 2 {
            if self.camera.is_none() {
                self.camera = Some(
                    world
                        .spawn((
                            Camera3d::default(),
                            Camera {
                                order: -5,
                                clear_color: ClearColorConfig::Custom(Color::BLACK),
                                ..default()
                            },
                            RenderTarget::Image(input.warm.into()),
                            Msaa::Off,
                            Tonemapping::None,
                            RenderLayers::layer(2),
                            Transform::from_xyz(0., 0., 6.8).looking_at(Vec3::ZERO, Vec3::Y),
                        ))
                        .id(),
                );
            }
            if uploaded {
                a.warm += 1;
            }
            if a.warm < 2 {
                return Ok(());
            }
            for &e in a.parts.iter().chain(&a.sprites) {
                world.entity_mut(e).insert(RenderLayers::default());
            }
            if let Some(e) = self.camera.take() {
                world.despawn(e);
            }
            a.start = Some(input.step);
        }
        let elapsed = input.step.saturating_sub(a.start.unwrap());
        *world.get_mut::<Transform>(a.root).unwrap() = a.flight.pose(elapsed);
        *world.get_mut::<Visibility>(a.model).unwrap() = if elapsed < FLIGHT_STEPS {
            Visibility::Inherited
        } else {
            Visibility::Hidden
        };
        for (i, &e) in a.sprites.iter().enumerate() {
            let dust = a.flight.dust[i].sample(elapsed.saturating_sub(FLIGHT_STEPS));
            world.entity_mut(e).insert((
                Transform {
                    translation: dust.position.as_vec3(),
                    rotation: Quat::from_rotation_z(dust.rotation as f32),
                    scale: Vec3::splat(dust.scale as f32),
                },
                MeshTag::new(if elapsed >= FLIGHT_STEPS {
                    (dust.opacity.clamp(0., 1.) * 65535.).round() as u32
                } else {
                    0x80000000
                }),
            ));
        }
        Ok(())
    }
}
pub fn prepare_queued(app: &mut App, input: Option<Input>) -> Result<(), String> {
    let world = app.world();
    let r = world.resource::<AsteroidRuntime>();
    let render = app.sub_app(RenderApp).world();
    let uploaded = r
        .active
        .as_ref()
        .filter(|a| !a.parts.is_empty())
        .is_some_and(|a| {
            let meshes = render.resource::<RenderAssets<RenderMesh>>();
            let materials = render.resource::<ErasedRenderAssets<PreparedMaterial>>();
            render
                .resource::<PipelineCache>()
                .waiting_pipelines()
                .next()
                .is_none()
                && a.parts.iter().all(|e| {
                    world
                        .get::<Mesh3d>(*e)
                        .is_some_and(|m| meshes.get(m.id()).is_some())
                        && world
                            .get::<MeshMaterial3d<HazeMaterial>>(*e)
                            .is_some_and(|m| materials.get(m.id().untyped()).is_some())
                })
                && r.shared.as_ref().is_some_and(|s| {
                    meshes.get(s.mesh.id()).is_some()
                        && materials.get(s.material.id().untyped()).is_some()
                })
        });
    let mut r = app
        .world_mut()
        .remove_resource::<AsteroidRuntime>()
        .unwrap();
    let result = r.advance(app.world_mut(), input, uploaded);
    app.world_mut().insert_resource(r);
    result
}
pub fn reset(world: &mut World) {
    let Some(mut r) = world.remove_resource::<AsteroidRuntime>() else {
        return;
    };
    r.park(world);
    world.insert_resource(r);
}
pub struct AsteroidPlugin;
impl Plugin for AsteroidPlugin {
    fn build(&self, app: &mut App) {
        app.init_resource::<AsteroidRuntime>()
            .add_plugins(material::AsteroidDustMaterialPlugin);
    }
}
pub mod motion;
