//! One scene-owned, portable atmosphere block shared by halos and surface haze.
pub mod material;
pub mod standard;
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
        RenderApp,
        erased_render_asset::ErasedRenderAssets,
        mesh::RenderMesh,
        render_asset::RenderAssets,
        render_resource::*,
        renderer::{RenderDevice, RenderQueue},
    },
};
use material::HaloMaterial;
use openmars_scene::atmosphere::{Body, MAX_BODIES};
use std::collections::{BTreeMap, BTreeSet};

#[derive(Clone, Debug, ShaderType)]
pub struct AtmosphereUniform {
    pub bodies: [Vec4; MAX_BODIES],
    pub colors: [Vec4; MAX_BODIES],
    pub shadows: [Vec4; MAX_BODIES],
    pub profiles: [Vec4; MAX_BODIES],
    pub sun: Vec4,
    pub sun_color: Vec4,
    /// Ambient radiance, sky rotation about Y, display sky brightness, decorative seconds.
    pub lighting: Vec4,
    pub metadata: UVec4,
}
impl Default for AtmosphereUniform {
    fn default() -> Self {
        Self {
            bodies: [Vec4::ZERO; MAX_BODIES],
            colors: [Vec4::ZERO; MAX_BODIES],
            shadows: [Vec4::ZERO; MAX_BODIES],
            profiles: [Vec4::ZERO; MAX_BODIES],
            sun: Vec4::ZERO,
            sun_color: Vec4::ONE,
            lighting: Vec4::new(crate::lighting::ambient_radiance(), 0., 0.65, 0.),
            metadata: UVec4::ZERO,
        }
    }
}
pub struct BodySpec {
    pub id: String,
    pub body: Body,
    pub rotation: Quat,
    /// Unit-radius source radial hull, required for an irregular atmosphere.
    pub hull: Option<Handle<Mesh>>,
}
pub struct Input {
    pub bodies: Vec<BodySpec>,
    pub sun: Vec3,
    pub sun_color: Vec3,
    pub intensity: f64,
    pub ambient: f32,
    pub sky_rotation: f32,
    pub sky_brightness: f32,
    pub seconds: f32,
    pub camera: Transform,
    pub projection_y: f64,
    pub height: f64,
    pub warm: Handle<Image>,
}
struct Halo {
    hull: Option<bevy::asset::AssetId<Mesh>>,
    entities: Vec<Entity>,
    mesh: Handle<Mesh>,
    warm: u8,
}
struct Shared {
    mesh: Handle<Mesh>,
    material: Handle<HaloMaterial>,
}
struct Retired {
    buffer: Option<Buffer>,
    _shared: Option<Shared>,
    _meshes: Vec<Handle<Mesh>>,
    frames: u8,
}
#[derive(Resource, Default)]
pub struct AtmosphereRuntime {
    uniform: Option<UniformBuffer<AtmosphereUniform>>,
    shared: Option<Shared>,
    halos: BTreeMap<String, Halo>,
    retired: Vec<Retired>,
    camera: Option<Entity>,
    ready: bool,
    allocations: u64,
    retirements: u64,
    writes: u64,
}
impl AtmosphereRuntime {
    pub fn ready(&self) -> bool {
        self.ready
    }
    pub fn binding(&self) -> Result<Buffer, String> {
        self.uniform
            .as_ref()
            .and_then(|u| u.buffer())
            .cloned()
            .ok_or("Atmosphere storage is not prepared".into())
    }
    pub fn progress(&self) -> Option<String> {
        (!self.ready).then(|| format!("Preparing atmosphere ({} bodies)", self.halos.len()))
    }
    pub fn diagnostics(&self) -> serde_json::Value {
        serde_json::json!({"ready":self.ready,"bodies":self.uniform.as_ref().map_or(0,|u|u.get().metadata.x),"uniforms":usize::from(self.uniform.is_some()),"bytes":self.uniform.as_ref().and_then(|u|u.buffer()).map_or(0,|b|b.size()),"buffer":self.uniform.as_ref().and_then(|u|u.buffer()).map(|b|format!("{:?}",b.id())),"allocations":self.allocations,"retirements":self.retirements,"retired":self.retired.len(),"writes":self.writes,"sample":self.halos.iter().map(|(id,h)|serde_json::json!({"id":id,"entities":h.entities.iter().map(Entity::to_string).collect::<Vec<_>>(),"warmFrames":h.warm})).collect::<Vec<_>>(),"profiles":self.uniform.as_ref().map(|u|u.get().profiles[..u.get().metadata.x as usize].iter().map(|p|p.to_array()).collect::<Vec<_>>())})
    }
    fn collect(&mut self) {
        for r in &mut self.retired {
            r.frames = r.frames.saturating_sub(1);
        }
        self.retired.retain(|r| {
            if r.frames == 0 {
                if let Some(b) = &r.buffer {
                    b.destroy();
                    self.retirements += 1;
                }
                false
            } else {
                true
            }
        });
    }
    fn clear(&mut self, world: &mut World) {
        let mut meshes = Vec::new();
        for (_, h) in std::mem::take(&mut self.halos) {
            for e in h.entities {
                world.despawn(e);
            }
            meshes.push(h.mesh);
        }
        if let Some(e) = self.camera.take() {
            world.despawn(e);
        }
        let buffer = self.uniform.take().and_then(|u| u.buffer().cloned());
        let shared = self.shared.take();
        if buffer.is_some() || shared.is_some() || !meshes.is_empty() {
            self.retired.push(Retired {
                buffer,
                _shared: shared,
                _meshes: meshes,
                frames: 2,
            });
        }
        self.ready = true;
    }
    fn update(
        &mut self,
        world: &mut World,
        input: Option<Input>,
        uploaded: &BTreeSet<String>,
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<(), String> {
        let Some(input) = input else {
            return Ok(());
        };
        if input.bodies.len() > MAX_BODIES
            || !input.intensity.is_finite()
            || !(0. ..=16.).contains(&input.intensity)
            || ![
                input.ambient,
                input.sky_rotation,
                input.sky_brightness,
                input.seconds,
            ]
            .into_iter()
            .all(f32::is_finite)
            || !(0. ..=16.).contains(&input.ambient)
            || !(0. ..=16.).contains(&input.sky_brightness)
            || input.seconds < 0.
            || !input.sun.is_finite()
            || !input.sun_color.is_finite()
            || !input.camera.to_matrix().is_finite()
        {
            return Err("Invalid atmosphere view or body bound".into());
        }
        let mut wanted = BTreeSet::new();
        for b in &input.bodies {
            b.body.validate()?;
            if b.id.is_empty()
                || b.id.len() > 128
                || !wanted.insert(b.id.clone())
                || !b.rotation.is_finite()
                || b.rotation.length_squared() < 0.99
                || b.rotation.length_squared() > 1.01
                || (b.body.follows_mesh && b.hull.is_none())
            {
                return Err("Invalid atmosphere identity or hull".into());
            }
        }
        for id in self
            .halos
            .keys()
            .filter(|id| !wanted.contains(*id))
            .cloned()
            .collect::<Vec<_>>()
        {
            let h = self.halos.remove(&id).unwrap();
            for e in h.entities {
                world.despawn(e);
            }
            self.retired.push(Retired {
                buffer: None,
                _shared: None,
                _meshes: vec![h.mesh],
                frames: 2,
            });
        }
        if self.shared.is_none() && !input.bodies.is_empty() {
            let mesh = world
                .resource_mut::<Assets<Mesh>>()
                .add(Sphere::new(1.).mesh().uv(64, 32));
            let material = world
                .resource_mut::<Assets<HaloMaterial>>()
                .add(HaloMaterial {
                    parameters: self.binding()?,
                });
            self.shared = Some(Shared { mesh, material });
        }
        let mut data = AtmosphereUniform {
            sun: input.sun.extend(input.intensity as f32),
            sun_color: input.sun_color.extend(1.),
            lighting: Vec4::new(
                input.ambient,
                input.sky_rotation,
                input.sky_brightness,
                input.seconds,
            ),
            metadata: UVec4::new(input.bodies.len() as u32, 0, 0, 0),
            ..default()
        };
        let view = input.camera.to_matrix().inverse();
        for (index, spec) in input.bodies.into_iter().enumerate() {
            let b = spec.body;
            let center = Vec3::from_array(b.center.map(|v| v as f32));
            let radius = b.radius as f32;
            let view_z = f64::from(view.transform_point3(center).z);
            let visibility = openmars_scene::atmosphere::visibility(
                b.radius,
                view_z,
                input.projection_y,
                input.height,
            )?;
            let p = b.profile;
            data.bodies[index] = center.extend(radius);
            data.colors[index] =
                Vec3::from_array(p.color.map(|v| v as f32)).extend(f32::from(view_z <= b.radius));
            data.shadows[index] = Vec3::from_array(p.shadow.map(|v| v as f32)).extend(0.);
            data.profiles[index] = Vec4::new(
                p.thickness as f32,
                (p.intensity * input.intensity * visibility) as f32,
                p.halo as f32,
                f32::from(b.follows_mesh),
            );
            let hull = spec.hull.as_ref().map(Handle::id);
            if self.halos.get(&spec.id).is_some_and(|h| h.hull != hull) {
                let old = self.halos.remove(&spec.id).unwrap();
                for entity in old.entities {
                    world.despawn(entity);
                }
                self.retired.push(Retired {
                    buffer: None,
                    _shared: None,
                    _meshes: vec![old.mesh],
                    frames: 2,
                });
            }
            if !self.halos.contains_key(&spec.id) {
                let shared = self.shared.as_ref().unwrap();
                let mesh = spec.hull.clone().unwrap_or_else(|| shared.mesh.clone());
                let count = if b.follows_mesh { 16 } else { 1 };
                let mut entities = Vec::with_capacity(count);
                for _ in 0..count {
                    entities.push(
                        world
                            .spawn((
                                Mesh3d(mesh.clone()),
                                MeshMaterial3d(shared.material.clone()),
                                MeshTag::new(index as u32),
                                Transform::IDENTITY,
                                RenderLayers::layer(2),
                                NoFrustumCulling,
                                NotShadowCaster,
                                NotShadowReceiver,
                            ))
                            .id(),
                    );
                }
                self.halos.insert(
                    spec.id.clone(),
                    Halo {
                        hull,
                        entities,
                        mesh,
                        warm: 0,
                    },
                );
            }
            let halo = self.halos.get_mut(&spec.id).unwrap();
            if uploaded.contains(&spec.id) {
                halo.warm = halo.warm.saturating_add(1).min(2);
            }
            for (layer, &entity) in halo.entities.iter().enumerate() {
                let thickness = if b.follows_mesh {
                    p.thickness * (layer + 1) as f64 / 16.
                } else {
                    p.thickness
                };
                world.entity_mut(entity).insert((
                    Transform {
                        translation: center,
                        rotation: spec.rotation,
                        scale: Vec3::splat(radius * (1. + thickness as f32)),
                    },
                    MeshTag::new(index as u32),
                    if halo.warm >= 2 {
                        RenderLayers::default()
                    } else {
                        RenderLayers::layer(2)
                    },
                ));
            }
        }
        let u = self
            .uniform
            .as_mut()
            .ok_or("Missing shared atmosphere storage")?;
        u.set(data);
        u.write_buffer(device, queue);
        self.writes += 1;
        self.ready = self.halos.values().all(|h| h.warm >= 2);
        if self.ready {
            if let Some(e) = self.camera.take() {
                world.despawn(e);
            }
        } else if self.camera.is_none()
            && !world
                .query::<(&Camera, &RenderLayers)>()
                .iter(world)
                .any(|(camera, layers)| camera.is_active && layers == &RenderLayers::layer(2))
        {
            self.camera = Some(
                world
                    .spawn((
                        Camera3d::default(),
                        Camera {
                            order: -6,
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
        Ok(())
    }
}
/// Submitted-resource ownership: invalidate old buffers before admitting new
/// bindings. The GPU retains storage needed by previously submitted uses.
pub fn begin(app: &mut App) -> Result<(), String> {
    let mut r = app
        .world_mut()
        .remove_resource::<AtmosphereRuntime>()
        .unwrap();
    r.collect();
    if crate::assets::view(app.world()) == openmars_scene::view::AssetView::Idle {
        r.clear(app.world_mut());
    } else if r.uniform.is_none() {
        let mut u = UniformBuffer::from(AtmosphereUniform::default());
        u.set_label(Some("planet haze parameters"));
        u.write_buffer(
            app.world().resource::<RenderDevice>(),
            app.world().resource::<RenderQueue>(),
        );
        r.uniform = Some(u);
        r.allocations += 1;
        r.ready = false;
    }
    app.world_mut().insert_resource(r);
    Ok(())
}
pub fn prepare_queued(app: &mut App, input: Option<Input>) -> Result<(), String> {
    let mut uploaded = BTreeSet::new();
    let render = app.sub_app(RenderApp).world();
    let meshes = render.resource::<RenderAssets<RenderMesh>>();
    let materials = render.resource::<ErasedRenderAssets<PreparedMaterial>>();
    let r = app.world().resource::<AtmosphereRuntime>();
    if render
        .resource::<PipelineCache>()
        .waiting_pipelines()
        .next()
        .is_none()
        && r.shared
            .as_ref()
            .is_some_and(|s| materials.get(s.material.id().untyped()).is_some())
    {
        for (id, h) in &r.halos {
            if meshes.get(h.mesh.id()).is_some() {
                uploaded.insert(id.clone());
            }
        }
    }
    let device = app.world().resource::<RenderDevice>().clone();
    let queue = app.world().resource::<RenderQueue>().clone();
    let mut r = app
        .world_mut()
        .remove_resource::<AtmosphereRuntime>()
        .unwrap();
    let result = r.update(app.world_mut(), input, &uploaded, &device, &queue);
    app.world_mut().insert_resource(r);
    result
}
pub fn reset(world: &mut World) {
    if let Some(mut r) = world.remove_resource::<AtmosphereRuntime>() {
        r.clear(world);
        world.insert_resource(r);
    }
}
pub struct AtmospherePlugin;
impl Plugin for AtmospherePlugin {
    fn build(&self, app: &mut App) {
        app.init_resource::<AtmosphereRuntime>()
            .add_plugins((material::HaloMaterialPlugin, standard::HazeMaterialPlugin));
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn portable_layout_is_one_1088_byte_block_for_all_sixteen_bodies() {
        assert_eq!(AtmosphereUniform::min_size().get(), 1088);
        let data = AtmosphereUniform::default();
        assert_eq!(data.bodies.len(), 16);
        assert_eq!(data.metadata.x, 0);
    }
}
