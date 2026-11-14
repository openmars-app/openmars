//! Normal scene entities, shared source shading and bounded sequential GPU warming.
use super::{
    config::{self, BODIES, BodyPose},
    geometry,
    material::{Mode, SpaceMaterial, Textures},
    phobos,
};
use crate::atmosphere::{
    BodySpec,
    standard::{HazeMaterial, SourceMaterials},
};
use bevy::{
    camera::{
        RenderTarget,
        visibility::{NoFrustumCulling, RenderLayers},
    },
    core_pipeline::{prepass::DepthPrepass, tonemapping::Tonemapping},
    pbr::PreparedMaterial,
    prelude::*,
    render::{
        RenderApp, erased_render_asset::ErasedRenderAssets, mesh::RenderMesh,
        render_asset::RenderAssets, render_resource::*, texture::GpuImage,
    },
};
use openmars_scene::view::{AssetView, BodyId, FocusTarget};
use std::collections::{BTreeMap, BTreeSet};
struct Body {
    root: Entity,
    surface: Option<Entity>,
    cloud: Option<(Entity, Handle<HazeMaterial>)>,
    parts: Vec<Entity>,
    grid_parts: Vec<Entity>,
    grid_revision: u64,
    warm: u8,
    pose: BodyPose,
}
#[derive(Resource, Default)]
pub struct CelestialRuntime {
    root: Option<Entity>,
    bodies: BTreeMap<BodyId, Body>,
    sphere: Option<Handle<Mesh>>,
    solar: Option<Entity>,
    sun: Vec<Entity>,
    orbits: Vec<Entity>,
    sun_warm: u8,
    orbit_warm: u8,
    model: Option<Entity>,
    model_materials: SourceMaterials,
    phobos_prepared: Option<phobos::Prepared>,
    shape: Option<phobos::Shape>,
    warm: Option<Handle<Image>>,
    camera: Option<Entity>,
    warming: Option<usize>,
    focus: FocusTarget,
    ready: bool,
    seconds: f64,
}
impl CelestialRuntime {
    pub fn ready(&self) -> bool {
        self.ready
    }
    pub fn marker_centers(&self) -> Vec<(BodyId, Vec3)> {
        if !self.ready || self.focus != FocusTarget::SolarSystem {
            return Vec::new();
        }
        std::iter::once((BodyId::Mars, Vec3::ZERO))
            .chain(self.bodies.iter().filter_map(|(id, body)| {
                (id.focus_group() == *id).then_some((*id, body.pose.center.as_vec3()))
            }))
            .collect()
    }
    pub fn grid_frame(
        &self,
        id: BodyId,
    ) -> Option<(openmars_scene::geometry::BodyProjection, Transform)> {
        let body = self.bodies.get(&id)?;
        let projection =
            super::picking::projection(id, self.shape.as_ref().map(|s| s.content_radius))?;
        Some((
            projection,
            Transform::from_translation(body.pose.center.as_vec3())
                .with_rotation(body.pose.rotation.as_quat()),
        ))
    }
    pub fn diagnostics(
        &self,
        camera: Option<(&Camera, &GlobalTransform)>,
        scale: Vec2,
    ) -> serde_json::Value {
        let project = |position: Vec3| {
            camera
                .and_then(|(c, t)| c.world_to_viewport(t, position).ok())
                .map(|point| (point * scale).to_array())
        };
        let sun = config::in_mars_frame(
            BodyPose {
                center: bevy::math::DVec3::ZERO,
                rotation: bevy::math::DQuat::IDENTITY,
            },
            self.seconds,
        )
        .ok()
        .map(|pose| pose.center.as_vec3());
        let sun_point = sun.and_then(project);
        let sun_radius = sun
            .and_then(|position| {
                camera.and_then(|(_, pose)| project(position + pose.right() * 22.))
            })
            .zip(sun_point)
            .map(|(edge, center)| Vec2::from_array(edge).distance(Vec2::from_array(center)));

        serde_json::json!({"ready":self.ready,"focus":self.focus,"seconds":self.seconds,"bodies":self.bodies.iter().map(|(id,b)|serde_json::json!({"body":id,"entity":b.root.to_string(),"position":b.pose.center.to_array(),"rotation":b.pose.rotation.to_array(),"point":project(b.pose.center.as_vec3()),"parts":b.parts.len(),"surface":b.surface.map(|e|e.to_string()),"warmFrames":b.warm})).collect::<Vec<_>>(),"sunParts":self.sun.len(),"sun":{"point":sun_point,"radius":sun_radius},"sunWarmFrames":self.sun_warm,"orbitLines":self.orbits.len(),"orbitWarmFrames":self.orbit_warm,"warmCamera":self.camera.is_some(),"phobos":self.shape.as_ref().map(|s|serde_json::json!({"radius":s.radius,"contentRadius":s.content_radius,"vertices":s.points.len(),"triangles":s.indices.len()/3}))})
    }
    fn clear(&mut self, world: &mut World) {
        if let Some(e) = self.root.take() {
            world.despawn(e);
        }
        if let Some(e) = self.camera.take() {
            world.despawn(e);
        }
        *self = Self::default();
    }
    fn spawn(&mut self, world: &mut World, format: TextureFormat) -> Result<(), String> {
        let root = *self.root.get_or_insert_with(|| {
            world
                .spawn((Transform::default(), Visibility::default()))
                .id()
        });
        let sphere = self
            .sphere
            .get_or_insert_with(|| {
                world
                    .resource_mut::<Assets<Mesh>>()
                    .add(geometry::sphere(1.))
            })
            .clone();
        if self.warm.is_none() {
            self.warm = Some(
                world
                    .resource_mut::<Assets<Image>>()
                    .add(Image::new_target_texture(128, 128, format, None)),
            );
        }
        for id in BODIES {
            if id == BodyId::Mars || self.bodies.contains_key(&id) {
                continue;
            }
            let cfg = config::config(id);
            if id == BodyId::Phobos {
                let entity = world
                    .spawn((Transform::default(), Visibility::default(), ChildOf(root)))
                    .id();
                self.bodies.insert(
                    id,
                    Body {
                        root: entity,
                        surface: None,
                        cloud: None,
                        parts: vec![],
                        grid_parts: Vec::new(),
                        grid_revision: 0,
                        warm: 0,
                        pose: BodyPose {
                            center: bevy::math::DVec3::ZERO,
                            rotation: bevy::math::DQuat::IDENTITY,
                        },
                    },
                );
                continue;
            }
            let name = format!("textures/planets/{}/surface", cfg.texture);
            let Some(texture) = crate::assets::texture(world, &name) else {
                continue;
            };
            if id == BodyId::Earth
                && crate::assets::texture(world, "textures/planets/earth-clouds/surface").is_none()
            {
                continue;
            }
            let material = crate::atmosphere::standard::material(
                world,
                StandardMaterial {
                    base_color_texture: Some(texture),
                    perceptual_roughness: if config::parent(id) != id { 0.9 } else { 0.8 },
                    ..default()
                },
            )?;
            let entity = world
                .spawn((Transform::default(), Visibility::default(), ChildOf(root)))
                .id();
            let surface = world
                .spawn((
                    Mesh3d(sphere.clone()),
                    MeshMaterial3d(material.clone()),
                    Transform::from_scale(Vec3::splat(cfg.radius as f32)),
                    NoFrustumCulling,
                    ChildOf(entity),
                ))
                .id();
            let cloud = if id == BodyId::Earth {
                if let Some(texture) =
                    crate::assets::texture(world, "textures/planets/earth-clouds/surface")
                {
                    let material = crate::atmosphere::standard::material(
                        world,
                        StandardMaterial {
                            base_color: Color::srgba(1., 1., 1., 0.4),
                            base_color_texture: Some(texture),
                            alpha_mode: AlphaMode::Blend,
                            perceptual_roughness: 1.,
                            ..default()
                        },
                    )?;
                    let cloud = world
                        .spawn((
                            Mesh3d(sphere.clone()),
                            MeshMaterial3d(material.clone()),
                            Transform::from_scale(Vec3::splat(cfg.radius as f32 * 1.005)),
                            NoFrustumCulling,
                            ChildOf(entity),
                        ))
                        .id();
                    Some((cloud, material))
                } else {
                    None
                }
            } else {
                None
            };
            let mut parts = vec![surface];
            if let Some((cloud, _)) = &cloud {
                parts.push(*cloud);
            }
            self.bodies.insert(
                id,
                Body {
                    root: entity,
                    surface: Some(surface),
                    cloud,
                    parts,
                    grid_parts: Vec::new(),
                    grid_revision: 0,
                    warm: 0,
                    pose: BodyPose {
                        center: bevy::math::DVec3::ZERO,
                        rotation: bevy::math::DQuat::IDENTITY,
                    },
                },
            );
        }
        if self.solar.is_none()
            && let Some(texture) = crate::assets::texture(world, "textures/planets/sun/surface")
        {
            let solar = world
                .spawn((Transform::default(), Visibility::default(), ChildOf(root)))
                .id();
            self.solar = Some(solar);
            for (mode, scale) in [(Mode::Sun, 22.), (Mode::Corona, 33.)] {
                let material = self.material(
                    world,
                    mode,
                    Some(texture.clone()),
                    Vec4::new(if mode == Mode::Sun { 0. } else { 1. }, 0., 1., 0.),
                )?;
                self.sun.push(
                    world
                        .spawn((
                            Mesh3d(sphere.clone()),
                            MeshMaterial3d(material),
                            Transform::from_scale(Vec3::splat(scale)),
                            NoFrustumCulling,
                            ChildOf(solar),
                        ))
                        .id(),
                );
            }
            let arc = world
                .resource_mut::<Assets<Mesh>>()
                .add(geometry::prominence());
            let material =
                self.material(world, Mode::Prominence, None, Vec4::new(2., 0., 1., 0.))?;
            for (angle, tilt, scale) in [
                (0.45, 0.15, 1.),
                (2.1, -0.35, 0.75),
                (3.6, 0.4, 1.15),
                (5.2, -0.15, 0.6),
            ] {
                let rotation = Quat::from_euler(EulerRot::XYZ, 0., tilt, angle);
                self.sun.push(
                    world
                        .spawn((
                            Mesh3d(arc.clone()),
                            MeshMaterial3d(material.clone()),
                            Transform::from_translation(rotation * Vec3::Y * 21.82)
                                .with_rotation(rotation)
                                .with_scale(Vec3::splat(scale)),
                            NoFrustumCulling,
                            ChildOf(solar),
                        ))
                        .id(),
                );
            }
            for id in BODIES.into_iter().filter(|id| config::parent(*id) == *id) {
                let cfg = config::config(id);
                let mesh = world
                    .resource_mut::<Assets<Mesh>>()
                    .add(geometry::orbit(cfg.orbit_radius as f32));
                let material = self.material(
                    world,
                    Mode::Orbit,
                    None,
                    Vec4::new(
                        3.,
                        cfg.angle_deg.to_radians() as f32,
                        cfg.period as f32,
                        f32::from(id == BodyId::Mars),
                    ),
                )?;
                self.orbits.push(
                    world
                        .spawn((
                            Mesh3d(mesh),
                            MeshMaterial3d(material),
                            Transform::default(),
                            NoFrustumCulling,
                            RenderLayers::none(),
                            ChildOf(solar),
                        ))
                        .id(),
                );
            }
        }
        if self.model.is_none()
            && let Some(body) = self.bodies.get(&BodyId::Phobos)
            && let Some(source) = phobos::source(world)?
        {
            self.model = Some(
                world
                    .spawn((
                        WorldAssetRoot(source),
                        Transform::from_scale(Vec3::splat(0.027)),
                        ChildOf(body.root),
                    ))
                    .id(),
            );
        }
        if self.phobos_prepared.is_none()
            && let Some(root) = self.model
            && let Some(prepared) = phobos::prepare(world, root, &mut self.model_materials)?
        {
            self.shape = Some(phobos::game_shape(world, root, &prepared.parts)?);
            self.bodies.get_mut(&BodyId::Phobos).unwrap().parts = prepared.parts.clone();
            self.phobos_prepared = Some(prepared);
        }
        Ok(())
    }
    fn material(
        &self,
        world: &mut World,
        mode: Mode,
        surface: Option<Handle<Image>>,
        parameters: Vec4,
    ) -> Result<Handle<SpaceMaterial>, String> {
        let atmosphere = world
            .resource::<crate::atmosphere::AtmosphereRuntime>()
            .binding()?;
        Ok(world
            .resource_mut::<Assets<SpaceMaterial>>()
            .add(SpaceMaterial {
                textures: Textures {
                    surface,
                    parameters,
                    mode,
                },
                atmosphere,
            }))
    }
    fn update(
        &mut self,
        world: &mut World,
        format: TextureFormat,
        uploaded: &BTreeSet<Entity>,
        warm_allowed: bool,
    ) -> Result<(), String> {
        if crate::assets::view(world) != AssetView::Game {
            if self.root.is_some() {
                self.clear(world)
            }
            return Ok(());
        }
        self.seconds = world
            .resource::<crate::terrain::surface::SurfaceRuntime>()
            .motion_step() as f64
            / 60.;
        self.spawn(world, format)?;
        let requested = crate::assets::focus(world);
        for (&id, body) in &mut self.bodies {
            let (revision, parts) = world
                .resource::<crate::terrain::grid::GridRuntime>()
                .celestial_parts(id, world);
            if revision != body.grid_revision {
                body.grid_revision = revision;
                body.grid_parts = parts;
                body.warm = 0;
            }
            body.pose = config::in_mars_frame(config::body_pose(id, self.seconds)?, self.seconds)?;
            world.entity_mut(body.root).insert(
                Transform::from_translation(body.pose.center.as_vec3())
                    .with_rotation(body.pose.rotation.as_quat()),
            );
            let cfg = config::config(id);
            if let Some((cloud, _)) = &body.cloud {
                world.entity_mut(*cloud).insert(
                    Transform::from_scale(Vec3::splat(cfg.radius as f32 * 1.005))
                        .with_rotation(Quat::from_rotation_y(self.seconds as f32 * 0.02)),
                );
            }
        }
        if let Some(solar) = self.solar {
            let pose = config::in_mars_frame(
                BodyPose {
                    center: bevy::math::DVec3::ZERO,
                    rotation: bevy::math::DQuat::IDENTITY,
                },
                self.seconds,
            )?;
            world.entity_mut(solar).insert(
                Transform::from_translation(pose.center.as_vec3())
                    .with_rotation(pose.rotation.as_quat()),
            );
        }
        self.warm(world, uploaded, warm_allowed)?;
        self.ready = self.bodies.len() == 11
            && self.shape.is_some()
            && self.bodies.values().all(|b| b.warm >= 2)
            && self.sun_warm >= 2
            && self.orbit_warm >= 2;
        if self.ready && requested != self.focus {
            self.focus = requested;
            let home = config::home(requested);
            let mut view = world.resource_mut::<crate::world::WorldView>();
            view.focus = requested;
            view.yaw = home.theta as f32;
            view.pitch = (std::f64::consts::FRAC_PI_2 - home.phi) as f32;
            view.distance = if requested == FocusTarget::default() {
                6.8
            } else {
                home.radius as f32
            };
            view.hovered = None;
        }
        let binding =
            config::in_mars_frame(config::focus_pose(self.focus, self.seconds)?, self.seconds)?;
        world.resource_mut::<crate::world::WorldView>().binding =
            Transform::from_translation(binding.center.as_vec3())
                .with_rotation(binding.rotation.as_quat());
        let orbit_warming = self.warming == Some(self.bodies.len() + 1);
        for &entity in &self.orbits {
            let layers = if self.focus == FocusTarget::SolarSystem {
                RenderLayers::layer(0)
            } else {
                RenderLayers::none()
            };
            world.entity_mut(entity).insert(if orbit_warming {
                layers.with(2)
            } else {
                layers
            });
        }
        Ok(())
    }
    fn warm(
        &mut self,
        world: &mut World,
        uploaded: &BTreeSet<Entity>,
        allowed: bool,
    ) -> Result<(), String> {
        // At most one celestial warm view, after the board's warm views retire.
        let mut batches: Vec<_> = self
            .bodies
            .iter()
            .map(|(id, b)| {
                (
                    b.parts
                        .iter()
                        .chain(&b.grid_parts)
                        .copied()
                        .collect::<Vec<_>>(),
                    b.pose.center.as_vec3(),
                    config::config(*id).radius as f32,
                    b.warm,
                )
            })
            .collect();
        batches.push((self.sun.clone(), Vec3::Z * 250., 33., self.sun_warm));
        batches.push((self.orbits.clone(), Vec3::Z * 250., 900., self.orbit_warm));
        let mut completed = None;
        if let Some(index) = self.warming {
            let batch = &batches[index];
            let environment = self
                .camera
                .and_then(|e| world.get::<bevy::light::EnvironmentMapLight>(e))
                .is_some();
            if !batch.0.is_empty() && environment && batch.0.iter().all(|e| uploaded.contains(e)) {
                if index < self.bodies.len() {
                    self.bodies.values_mut().nth(index).unwrap().warm = (batch.3 + 1).min(2);
                } else if index == self.bodies.len() {
                    self.sun_warm = (batch.3 + 1).min(2);
                } else {
                    self.orbit_warm = (batch.3 + 1).min(2);
                }
                if batch.3 + 1 >= 2 {
                    for &e in &batch.0 {
                        world.entity_mut(e).insert(RenderLayers::layer(0));
                    }
                    if let Some(camera) = self.camera.take() {
                        world.despawn(camera);
                    }
                    self.warming = None;
                    completed = Some(index);
                }
            }
            if let Some(camera) = self.camera {
                let target = batch.1;
                world.entity_mut(camera).insert(
                    Transform::from_translation(target + Vec3::Z * batch.2.max(0.1) * 4.)
                        .looking_at(target, Vec3::Y),
                );
            }
        }
        if allowed
            && self.warming.is_none()
            && let Some((index, batch)) = batches
                .iter()
                .enumerate()
                .find(|(i, b)| Some(*i) != completed && !b.0.is_empty() && b.3 < 2)
        {
            for &e in &batch.0 {
                world.entity_mut(e).insert(RenderLayers::layer(0).with(2));
            }
            self.camera = Some(
                world
                    .spawn((
                        Camera3d::default(),
                        Projection::Perspective(PerspectiveProjection {
                            near: 0.01,
                            far: 6000.,
                            ..default()
                        }),
                        DepthPrepass,
                        Camera {
                            order: -21,
                            ..default()
                        },
                        RenderTarget::Image(self.warm.clone().unwrap().into()),
                        Msaa::Off,
                        Tonemapping::None,
                        RenderLayers::layer(2),
                        Transform::from_translation(batch.1 + Vec3::Z * batch.2.max(0.1) * 4.)
                            .looking_at(batch.1, Vec3::Y),
                    ))
                    .id(),
            );
            self.warming = Some(index);
        }
        Ok(())
    }
    pub fn atmosphere(&self) -> Vec<BodySpec> {
        self.bodies
            .iter()
            .filter_map(|(&id, b)| {
                let cfg = config::config(id);
                let p = cfg.atmosphere;
                let rgb = |c: [u8; 3]| {
                    c.map(|v| openmars_scene::atmosphere::srgb_to_linear(f64::from(v) / 255.))
                };
                let (radius, hull) = if id == BodyId::Phobos {
                    let shape = self.shape.as_ref()?;
                    (shape.radius, Some(shape.hull.clone()))
                } else {
                    (cfg.radius, None)
                };
                Some(BodySpec {
                    id: format!("body-{}", id.location()),
                    body: openmars_scene::atmosphere::Body {
                        center: b.pose.center.to_array(),
                        radius,
                        profile: openmars_scene::atmosphere::Profile {
                            color: rgb(p.color),
                            shadow: rgb(p.shadow),
                            thickness: f64::from(p.thickness),
                            intensity: f64::from(p.intensity),
                            halo: f64::from(p.halo),
                        },
                        follows_mesh: hull.is_some(),
                    },
                    rotation: b.pose.rotation.as_quat(),
                    hull,
                })
            })
            .collect()
    }
    pub fn sky_rotation(&self) -> f32 {
        let pose = config::body_pose(BodyId::Mars, self.seconds).unwrap();
        let direction = pose.rotation * bevy::math::DVec3::Z;
        direction.x.atan2(direction.z) as f32
    }
}
/// Keep the actual globe/GLB PBR material bound to the same live highlight
/// storage as its grid. Assignment dirties the material only on lease changes.
pub fn bind_grid_highlights(
    world: &mut World,
    body: BodyId,
    binding: Option<(Buffer, Handle<Image>)>,
) {
    let Some(runtime) = world.get_resource::<CelestialRuntime>() else {
        return;
    };
    let Some(body) = runtime.bodies.get(&body) else {
        return;
    };
    let materials: std::collections::BTreeSet<_> = body
        .parts
        .iter()
        .filter_map(|&e| world.get::<MeshMaterial3d<HazeMaterial>>(e).map(|h| h.id()))
        .collect();
    let (buffer, data) =
        binding.unwrap_or_else(|| crate::atmosphere::standard::disabled_highlights(world));
    let mut assets = world.resource_mut::<Assets<HazeMaterial>>();
    for id in materials {
        let Some(material) = assets.get(id) else {
            continue;
        };
        if material.extension.highlights.id() != buffer.id()
            || material.extension.highlight_data.id() != data.id()
        {
            let mut material = assets.get_mut(id).unwrap();
            material.extension.highlights = buffer.clone();
            material.extension.highlight_data = data.clone();
        }
    }
}
pub fn reset(world: &mut World) {
    if let Some(mut r) = world.remove_resource::<CelestialRuntime>() {
        r.clear(world);
        world.insert_resource(r);
    }
}
pub fn prepare(app: &mut App, format: TextureFormat) -> Result<(), String> {
    let _profile = crate::profiling::span(app.world(), "space.prepare");
    let world = app.world();
    let render = app.sub_app(RenderApp).world();
    let mut uploaded = BTreeSet::new();
    let meshes = render.resource::<RenderAssets<RenderMesh>>();
    let materials = render.resource::<ErasedRenderAssets<PreparedMaterial>>();
    let images = render.resource::<RenderAssets<GpuImage>>();
    if render
        .resource::<PipelineCache>()
        .waiting_pipelines()
        .next()
        .is_none()
    {
        let r = world.resource::<CelestialRuntime>();
        for &e in r
            .bodies
            .values()
            .flat_map(|b| b.parts.iter().chain(&b.grid_parts))
            .chain(r.sun.iter())
            .chain(r.orbits.iter())
        {
            let Some(mesh) = world.get::<Mesh3d>(e) else {
                continue;
            };
            if meshes.get(mesh.id()).is_none() {
                continue;
            }
            let material = world
                .get::<MeshMaterial3d<HazeMaterial>>(e)
                .map(|m| m.id().untyped())
                .or_else(|| {
                    world
                        .get::<MeshMaterial3d<SpaceMaterial>>(e)
                        .map(|m| m.id().untyped())
                })
                .or_else(|| {
                    world
                        .get::<MeshMaterial3d<crate::terrain::materials::GridMaterial>>(e)
                        .map(|m| m.id().untyped())
                });
            if !material.is_some_and(|id| materials.get(id).is_some()) {
                continue;
            }
            let texture = world
                .get::<MeshMaterial3d<HazeMaterial>>(e)
                .and_then(|m| world.resource::<Assets<HazeMaterial>>().get(m.id()))
                .and_then(|m| m.base.base_color_texture.as_ref())
                .or_else(|| {
                    world
                        .get::<MeshMaterial3d<SpaceMaterial>>(e)
                        .and_then(|m| world.resource::<Assets<SpaceMaterial>>().get(m.id()))
                        .and_then(|m| m.textures.surface.as_ref())
                });
            if texture.is_none_or(|h| images.get(h.id()).is_some()) {
                uploaded.insert(e);
            }
        }
    }
    let allowed = crate::assets::board_ready(world);
    let mut r = app
        .world_mut()
        .remove_resource::<CelestialRuntime>()
        .unwrap();
    let result = r.update(app.world_mut(), format, &uploaded, allowed);
    app.world_mut().insert_resource(r);
    result?;
    super::focus::prepare(app.world_mut());
    Ok(())
}
