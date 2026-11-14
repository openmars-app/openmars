//! Owned authored hulls, pads and satellite, with real sequential GPU warming.
use super::{
    fleet::{self, Fleet},
    flight,
    materials::{Parameters, ShipPadMaterial, ShipParticleMaterial, Textures},
    models, pads,
};
use crate::atmosphere::standard::{HazeMaterial, SourceMaterials};
use bevy::{
    camera::{
        RenderTarget,
        visibility::{NoFrustumCulling, RenderLayers},
    },
    core_pipeline::{prepass::DepthPrepass, tonemapping::Tonemapping},
    light::{NotShadowCaster, NotShadowReceiver},
    pbr::{PreparedMaterial, ScreenSpaceTransmission},
    prelude::*,
    render::{
        RenderApp, erased_render_asset::ErasedRenderAssets, mesh::RenderMesh,
        render_asset::RenderAssets, render_resource::*,
    },
};
use openmars_scene::view::AssetView;
use std::collections::{BTreeMap, BTreeSet};
#[derive(Resource, Default)]
pub struct OffworldRuntime {
    root: Option<Entity>,
    ship_source: Option<Entity>,
    station: Option<Entity>,
    station_parts: Vec<Entity>,
    hull: Option<Handle<Mesh>>,
    hull_material: Option<Handle<HazeMaterial>>,
    station_materials: SourceMaterials,
    pads: Vec<Handle<ShipPadMaterial>>,
    meshes: Vec<Handle<Mesh>>,
    actors: Vec<Entity>,
    hulls: BTreeMap<fleet::Key, Entity>,
    fleet: Fleet,
    effects: super::effects::Effects,
    entrance: super::transitions::Entrance,
    completion: super::transitions::Completion,
    entrance_scale: f32,
    station_scale: f32,
    signature: String,
    camera: Option<Entity>,
    warm: u8,
    ready: bool,
}
impl OffworldRuntime {
    pub fn ready(&self) -> bool {
        self.ready
    }
    pub fn station_center(&self, world: &World) -> Option<Vec3> {
        if !self.ready
            || !world
                .resource::<crate::scene::SceneState>()
                .0
                .as_ref()?
                .offworld
                .station
                .as_ref()?
                .completed
        {
            return None;
        }
        Some(world.get::<GlobalTransform>(self.station?)?.translation())
    }
    pub fn diagnostics(&self, world: &World) -> serde_json::Value {
        let materials = world.resource::<Assets<HazeMaterial>>();
        serde_json::json!({"root":self.root.map(|e|e.to_string()),"stationEntity":self.station.map(|e|e.to_string()),"ready":self.ready,"warmFrames":self.warm,"ships":self.hulls.len(),"entranceScale":self.entrance_scale,"entranceStart":self.entrance.started(),"stationStart":self.completion.started(),"stationScale":self.station_scale,"effects":self.effects.diagnostics(world),"fleet":self.fleet.order.iter().filter_map(|key|self.fleet.ships.get(key).map(|s|serde_json::json!({"owner":key.owner,"ordinal":key.ordinal,"entity":self.hulls.get(key).map(|e|e.to_string()),"status":format!("{:?}",s.status),"serial":s.serial,"position":s.pose.position.to_array(),"rotation":s.pose.rotation.to_array(),"started":s.journey.as_ref().map(|j|j.start),"destination":s.landing.as_ref().map(|l|l.body).or_else(||s.journey.as_ref().map(|j|j.landing.body))}))).collect::<Vec<_>>(),"stationParts":self.station_parts.len(),"stationMaterials":self.station_materials.values().filter_map(|h|materials.get(h)).map(|m|serde_json::json!({"ior":m.base.ior,"transmission":m.base.specular_transmission,"reflectance":m.base.reflectance})).collect::<Vec<_>>(),"warmCamera":self.camera.is_some()})
    }
    fn parts(&self) -> Vec<Entity> {
        self.station_parts
            .iter()
            .chain(&self.actors)
            .chain(self.hulls.values())
            .chain(self.effects.warm_nodes())
            .copied()
            .collect()
    }
    fn clear(&mut self, world: &mut World) {
        self.effects.clear(world);
        if let Some(e) = self.root.take() {
            world.despawn(e);
        }
        if let Some(e) = self.camera.take() {
            world.despawn(e);
        }
        for mesh in self.meshes.drain(..) {
            world.resource_mut::<Assets<Mesh>>().remove(mesh.id());
        }
        if let Some(mesh) = self.hull.take() {
            world.resource_mut::<Assets<Mesh>>().remove(mesh.id());
        }
        for material in self.pads.drain(..) {
            world
                .resource_mut::<Assets<ShipPadMaterial>>()
                .remove(material.id());
        }
        if let Some(material) = self.hull_material.take() {
            world
                .resource_mut::<Assets<HazeMaterial>>()
                .remove(material.id());
        }
        for (_, material) in std::mem::take(&mut self.station_materials) {
            world
                .resource_mut::<Assets<HazeMaterial>>()
                .remove(material.id());
        }
        *self = Self::default();
    }
    fn update(&mut self, world: &mut World, uploaded: &BTreeSet<Entity>) -> Result<(), String> {
        if crate::assets::view(world) != AssetView::Game {
            if self.root.is_some() {
                self.clear(world);
            }
            return Ok(());
        }
        let root = *self.root.get_or_insert_with(|| {
            world
                .spawn((Transform::IDENTITY, Visibility::Inherited))
                .id()
        });
        if self.hull.is_none() {
            if self.ship_source.is_none() {
                if let Some(source) = models::source(world, "models/spaceship")? {
                    self.ship_source = Some(
                        world
                            .spawn((
                                WorldAssetRoot(source),
                                Transform::IDENTITY,
                                Visibility::Hidden,
                                ChildOf(root),
                            ))
                            .id(),
                    );
                }
                return Ok(());
            }
            let Some(mesh) = models::ship(world, self.ship_source.unwrap())? else {
                return Ok(());
            };
            self.hull = Some(world.resource_mut::<Assets<Mesh>>().add(mesh));
            world.despawn(self.ship_source.take().unwrap());
            self.hull_material = Some(crate::atmosphere::standard::material(
                world,
                StandardMaterial {
                    base_color: Color::linear_rgb(0.21, 0.21, 0.2294),
                    metallic: 0.7,
                    perceptual_roughness: 0.35,
                    ..default()
                },
            )?);
        }
        if self.station.is_none() {
            if let Some(source) = models::source(world, "models/satellite")? {
                self.station = Some(
                    world
                        .spawn((
                            WorldAssetRoot(source),
                            Transform::from_scale(Vec3::splat(0.006)),
                            Visibility::Hidden,
                            ChildOf(root),
                        ))
                        .id(),
                );
            }
            return Ok(());
        }
        if self.station_parts.is_empty() {
            let Some(parts) =
                models::station(world, self.station.unwrap(), &mut self.station_materials)?
            else {
                return Ok(());
            };
            self.station_parts = parts;
            for &e in &self.station_parts {
                world.entity_mut(e).insert((
                    RenderLayers::layer(2),
                    NoFrustumCulling,
                    NotShadowCaster,
                    NotShadowReceiver,
                ));
            }
            world
                .entity_mut(self.station.unwrap())
                .insert(Visibility::Inherited);
        }
        if self.pads.is_empty() {
            let Some(concrete) = crate::assets::texture(world, "textures/terrain/concrete") else {
                return Ok(());
            };
            let Some(noise) = crate::assets::texture(world, "textures/terrain/noise-mid") else {
                return Ok(());
            };
            let Some(high_noise) = crate::assets::texture(world, "textures/terrain/noise-high")
            else {
                return Ok(());
            };
            let atmosphere = world
                .resource::<crate::atmosphere::AtmosphereRuntime>()
                .binding()?;
            for warning in [false, true] {
                self.pads
                    .push(
                        world
                            .resource_mut::<Assets<ShipPadMaterial>>()
                            .add(ShipPadMaterial {
                                textures: Textures {
                                    warning,
                                    parameters: Parameters {
                                        mode: UVec4::new(u32::from(warning), 0, 0, 0),
                                    },
                                    concrete: concrete.clone(),
                                    noise: noise.clone(),
                                    high_noise: high_noise.clone(),
                                },
                                atmosphere: atmosphere.clone(),
                            }),
                    );
            }
        }
        if !self.effects.initialize(world, root) {
            return Ok(());
        }
        let scene = world
            .resource::<crate::scene::SceneState>()
            .0
            .as_ref()
            .ok_or("Missing offworld scene")?;
        let roster: Vec<_> = scene
            .players
            .iter()
            .map(|p| {
                let count = if scene.offworld.fleets_enabled {
                    scene
                        .offworld
                        .fleets
                        .iter()
                        .find(|f| f.player_id == p.id)
                        .map_or(1, |f| f.total)
                } else {
                    0
                };
                (p.id.clone(), p.color.clone(), count)
            })
            .collect();
        let signature = serde_json::to_string(&roster).map_err(|e| e.to_string())?;
        if signature != self.signature {
            self.ready = false;
            self.warm = 0;
            self.signature = signature;
            for e in self.actors.drain(..) {
                world.despawn(e);
            }
            for mesh in self.meshes.drain(..) {
                world.resource_mut::<Assets<Mesh>>().remove(mesh.id());
            }
            let colors: Vec<_> = roster
                .iter()
                .flat_map(|(_, color, count)| {
                    let c =
                        Color::Srgba(Srgba::hex(color).unwrap_or(Srgba::new(0.5, 0.5, 0.5, 1.)))
                            .to_linear()
                            .to_f32_array();
                    std::iter::repeat_n([c[0], c[1], c[2]], *count as usize)
                })
                .collect();
            let parking = flight::parking(colors.len(), 2.02)?;
            for (index, warning) in [false, true]
                .into_iter()
                .enumerate()
                .filter(|_| !colors.is_empty())
            {
                let mesh = world
                    .resource_mut::<Assets<Mesh>>()
                    .add(pads::geometry(&parking, &colors, 2.02, warning)?);
                let entity = world
                    .spawn((
                        Mesh3d(mesh.clone()),
                        MeshMaterial3d(self.pads[index].clone()),
                        Transform::IDENTITY,
                        Visibility::Inherited,
                        RenderLayers::layer(2),
                        NoFrustumCulling,
                        NotShadowCaster,
                        NotShadowReceiver,
                        ChildOf(root),
                    ))
                    .id();
                self.actors.push(entity);
                self.meshes.push(mesh);
            }
            let keys: Vec<_> = roster
                .iter()
                .flat_map(|(owner, _, total)| {
                    (0..*total).map(|ordinal| fleet::Key {
                        owner: owner.clone(),
                        ordinal,
                    })
                })
                .collect();
            self.hulls.retain(|key, entity| {
                if keys.contains(key) {
                    true
                } else {
                    world.despawn(*entity);
                    false
                }
            });
            for (key, park) in keys.into_iter().zip(parking) {
                if self.hulls.contains_key(&key) {
                    continue;
                }
                let entity = world
                    .spawn((
                        Mesh3d(self.hull.as_ref().unwrap().clone()),
                        MeshMaterial3d(self.hull_material.as_ref().unwrap().clone()),
                        park.ship.transform(),
                        Visibility::Inherited,
                        RenderLayers::layer(2),
                        NoFrustumCulling,
                        NotShadowCaster,
                        NotShadowReceiver,
                        ChildOf(root),
                    ))
                    .id();
                self.hulls.insert(key, entity);
            }
        }
        if self.warm < 2 {
            if !crate::assets::board_ready(world)
                || !world
                    .resource::<crate::space::scene::CelestialRuntime>()
                    .ready()
            {
                return Ok(());
            }
            if self.camera.is_none() {
                let Some(target) = world
                    .resource::<crate::terrain::surface::SurfaceRuntime>()
                    .warm_target()
                else {
                    return Ok(());
                };
                self.camera = Some(
                    world
                        .spawn((
                            Camera3d::default(),
                            Camera {
                                order: -22,
                                ..default()
                            },
                            ScreenSpaceTransmission::default(),
                            DepthPrepass,
                            RenderTarget::Image(target.into()),
                            Msaa::Off,
                            Tonemapping::None,
                            RenderLayers::layer(2),
                            Transform::from_xyz(0., 0., 6.8).looking_at(Vec3::ZERO, Vec3::Y),
                        ))
                        .id(),
                );
            }
            if self
                .camera
                .is_some_and(|e| world.get::<bevy::light::EnvironmentMapLight>(e).is_some())
                && self.parts().iter().all(|e| uploaded.contains(e))
            {
                self.warm += 1;
            }
            if self.warm < 2 {
                return Ok(());
            }
            self.effects.finish_warm(world);
            for e in self.parts() {
                world.entity_mut(e).insert(RenderLayers::layer(0));
            }
            if let Some(e) = self.camera.take() {
                world.despawn(e);
            }
            self.ready = true;
        }
        let step = world
            .resource::<crate::terrain::surface::SurfaceRuntime>()
            .motion_step();
        let quiet = world.resource::<crate::scene::PresentationState>().settling
            || world.resource::<crate::scene::MotionPreference>().0;
        self.fleet.reconcile(
            world
                .resource::<crate::scene::SceneState>()
                .0
                .as_ref()
                .ok_or("Missing fleet scene")?,
            quiet,
            step,
        )?;
        let reduced = world.resource::<crate::scene::MotionPreference>().0;
        let entry = world.resource::<crate::scene::WorldEntryState>().0;
        self.entrance_scale = self.entrance.sample(entry.tiles, quiet, reduced, step);
        for &entity in &self.actors {
            world
                .entity_mut(entity)
                .insert(if self.entrance_scale > 0. {
                    Visibility::Inherited
                } else {
                    Visibility::Hidden
                });
        }
        self.effects
            .stage(world, root, &mut self.fleet, step, quiet)?;
        self.fleet.advance(step)?;
        self.effects.sample(world, &self.fleet, step)?;
        for (key, ship) in &self.fleet.ships {
            if let Some(entity) = self.hulls.get(key) {
                world.entity_mut(*entity).insert((
                    ship.pose
                        .transform()
                        .with_scale(Vec3::splat(self.entrance_scale)),
                    if ship.status == fleet::Status::Unavailable || self.entrance_scale <= 0. {
                        Visibility::Hidden
                    } else {
                        Visibility::Inherited
                    },
                ));
            }
        }
        let complete = world
            .resource::<crate::scene::SceneState>()
            .0
            .as_ref()
            .and_then(|s| s.offworld.station.as_ref())
            .map(|s| s.completed);
        self.station_scale = self.completion.sample(complete, quiet, reduced, step);
        let seconds = world
            .resource::<crate::terrain::surface::SurfaceRuntime>()
            .motion_step() as f64
            / 60.;
        let mut source = crate::space::config::station_pose(seconds)?;
        source.rotation *= bevy::math::DQuat::from_rotation_y(std::f64::consts::PI)
            * bevy::math::DQuat::from_rotation_z(seconds * 0.3);
        let pose = crate::space::config::in_mars_frame(source, seconds)?;
        world.entity_mut(self.station.unwrap()).insert((
            Transform {
                translation: pose.center.as_vec3(),
                rotation: pose.rotation.as_quat(),
                scale: Vec3::splat(0.006 * self.station_scale),
            },
            if complete == Some(true) && self.station_scale > 0. {
                Visibility::Inherited
            } else {
                Visibility::Hidden
            },
        ));
        Ok(())
    }
}
pub fn reset(world: &mut World) {
    if let Some(mut r) = world.remove_resource::<OffworldRuntime>() {
        r.clear(world);
        world.insert_resource(r);
    }
}
pub fn prepare(app: &mut App) -> Result<(), String> {
    let mut uploaded = BTreeSet::new();
    let render = app.sub_app(RenderApp).world();
    if render
        .resource::<PipelineCache>()
        .waiting_pipelines()
        .next()
        .is_none()
    {
        let meshes = render.resource::<RenderAssets<RenderMesh>>();
        let materials = render.resource::<ErasedRenderAssets<PreparedMaterial>>();
        let world = app.world();
        for e in world.resource::<OffworldRuntime>().parts() {
            let mesh = world.get::<Mesh3d>(e).map(|m| m.id());
            let material = world
                .get::<MeshMaterial3d<HazeMaterial>>(e)
                .map(|m| m.id().untyped())
                .or_else(|| {
                    world
                        .get::<MeshMaterial3d<ShipPadMaterial>>(e)
                        .map(|m| m.id().untyped())
                })
                .or_else(|| {
                    world
                        .get::<MeshMaterial3d<ShipParticleMaterial>>(e)
                        .map(|m| m.id().untyped())
                });
            if mesh.is_some_and(|id| meshes.get(id).is_some())
                && material.is_some_and(|id| materials.get(id).is_some())
            {
                uploaded.insert(e);
            }
        }
    }
    let mut r = app
        .world_mut()
        .remove_resource::<OffworldRuntime>()
        .unwrap();
    let result = r.update(app.world_mut(), &uploaded);
    app.world_mut().insert_resource(r);
    result
}
