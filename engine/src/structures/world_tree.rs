//! Original world-tree shader relief with stable, scene-owned GPU uniforms.
pub mod ground;
mod mesh;
use bevy::{
    camera::{
        RenderTarget,
        visibility::{NoFrustumCulling, RenderLayers},
    },
    core_pipeline::tonemapping::Tonemapping,
    ecs::system::SystemParamItem,
    light::{NotShadowCaster, NotShadowReceiver},
    mesh::MeshVertexBufferLayoutRef,
    pbr::{MaterialPipeline, MaterialPipelineKey, PreparedMaterial},
    prelude::*,
    render::{
        RenderApp,
        erased_render_asset::ErasedRenderAssets,
        mesh::RenderMesh,
        render_asset::RenderAssets,
        render_resource::*,
        renderer::{RenderDevice, RenderQueue},
    },
    shader::ShaderRef,
};
use openmars_scene::geometry::{AxialPoint, BodyProjection};
use std::collections::{BTreeMap, BTreeSet};
type Coordinate = (i32, i32, i32);
const MAX_TREES: usize = 512;
#[derive(Clone, Copy, Debug, ShaderType)]
pub struct WorldTreeUniform {
    pub geometry: Vec4,
    pub clock: Vec4,
    pub destruction: Vec4,
    pub sun: Vec4,
    pub sun_color: Vec4,
    pub ground_destruction: Vec4,
}
#[derive(Asset, TypePath, Clone, Debug)]
pub struct WorldTreeMaterial {
    pub parameters: Buffer,
    pub atmosphere: bevy::render::render_resource::Buffer,
}
impl AsBindGroup for WorldTreeMaterial {
    type Data = ();
    type Param = ();
    fn label() -> &'static str {
        "WorldTreeMaterial"
    }
    fn bind_group_data(&self) {}
    fn build_bind_group(
        &self,
        _: &BindGroupLayout,
        _: &RenderDevice,
        _: &mut SystemParamItem<'_, '_, Self::Param>,
        _: bool,
        out: &mut BindGroupBuilder,
    ) -> Result<(), AsBindGroupError> {
        out.push((
            0,
            UnpreparedBindingResource::Buffer(self.parameters.clone()),
        ));
        crate::atmosphere::material::bind_haze(&self.atmosphere, out);
        Ok(())
    }
    fn bind_group_layout_entries(_: &RenderDevice, _: bool) -> Vec<BindGroupLayoutEntry> {
        vec![
            BindGroupLayoutEntry {
                binding: 0,
                visibility: ShaderStages::VERTEX_FRAGMENT,
                ty: BindingType::Buffer {
                    ty: BufferBindingType::Uniform,
                    has_dynamic_offset: false,
                    min_binding_size: Some(WorldTreeUniform::min_size()),
                },
                count: None,
            },
            crate::atmosphere::material::haze_layout(),
        ]
    }
}
impl Material for WorldTreeMaterial {
    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/structures/shaders/world_tree.wesl".into()
    }
    fn fragment_shader() -> ShaderRef {
        Self::vertex_shader()
    }
    fn alpha_mode(&self) -> AlphaMode {
        AlphaMode::Blend
    }
    fn enable_prepass() -> bool {
        false
    }
    fn enable_shadows() -> bool {
        false
    }
    fn specialize(
        _: &MaterialPipeline,
        d: &mut RenderPipelineDescriptor,
        _: &MeshVertexBufferLayoutRef,
        _: MaterialPipelineKey<Self>,
    ) -> Result<(), SpecializedMeshPipelineError> {
        d.primitive.cull_mode = None;
        if let Some(depth) = &mut d.depth_stencil {
            depth.depth_write_enabled = Some(true);
            depth.depth_compare = Some(CompareFunction::GreaterEqual);
        }
        Ok(())
    }
}
pub struct WorldTreePlugin;
impl Plugin for WorldTreePlugin {
    fn build(&self, app: &mut App) {
        bevy::shader::load_shader_library!(app, "shaders/world_tree_shape.wesl");
        bevy::asset::embedded_asset!(app, "shaders/world_tree.wesl");
        bevy::asset::embedded_asset!(app, "shaders/world_tree_ground.wesl");
        app.add_plugins(MaterialPlugin::<ground::MossMaterial>::default());
        app.add_plugins(MaterialPlugin::<WorldTreeMaterial>::default())
            .init_resource::<WorldTreeRuntime>();
    }
}
#[derive(Clone, Copy, Debug)]
struct Lifetime {
    born: f64,
    born_motion: f64,
    destroy: Option<f64>,
    destroy_motion: Option<f64>,
}
impl Lifetime {
    fn ended(self, seconds: f64, reduced: bool) -> bool {
        self.destroy
            .is_some_and(|d| seconds >= d + if reduced { 0.15 } else { 128. / 60. })
    }
    fn parameters(
        self,
        seed: u32,
        radius: f32,
        seconds: f64,
        motion_seconds: f64,
        reduced: bool,
        depths: Vec2,
    ) -> WorldTreeUniform {
        let age = (self
            .destroy
            .map_or(seconds, |d| seconds.min(d + if reduced { 0. } else { 1. }))
            - self.born)
            .max(0.);
        let emergence = if reduced {
            1.
        } else {
            1. - (1. - (age / 1.5).clamp(0., 1.)).powi(3)
        };
        let part = super::geometry::collapse_params(u64::from(seed) + 97);
        let collapse = if reduced {
            0.
        } else {
            self.destroy.map_or(0., |d| {
                ((((seconds - d - 1.) * 0.8 - f64::from(part.delay_s)) / 0.75).clamp(0., 1.))
                    .powi(2)
            })
        };
        let mut alpha = if reduced {
            (age / 0.15).clamp(0., 1.)
        } else {
            1.
        };
        if reduced && let Some(d) = self.destroy {
            alpha *= 1. - ((seconds - d) / 0.15).clamp(0., 1.);
        }
        WorldTreeUniform {
            geometry: Vec4::new(radius, 0.1, seed as f32, self.born_motion as f32),
            clock: Vec4::new(
                motion_seconds as f32,
                emergence as f32,
                collapse as f32,
                alpha as f32,
            ),
            destruction: if reduced {
                Vec4::ZERO
            } else {
                Vec4::new(
                    part.direction_rad.cos(),
                    part.direction_rad.sin(),
                    part.tilt_rad * collapse as f32,
                    depths.y * collapse as f32,
                )
            },
            ground_destruction: {
                let part =
                    openmars_scene::geometry::collapse::CollapseParameters::new(u64::from(seed));
                let progress = if reduced {
                    0.
                } else {
                    self.destroy
                        .map_or(0., |d| part.progress((seconds - d) * 0.8 - 0.8))
                } as f32;
                Vec4::new(
                    part.direction.cos() as f32,
                    part.direction.sin() as f32,
                    part.tilt as f32 * progress,
                    depths.x * progress,
                )
            },
            sun: crate::lighting::sun_direction().extend(0.65),
            sun_color: crate::lighting::sun_rgb().extend(if reduced { 0. } else { 1. }),
        }
    }
}
struct Plot {
    seed: u32,
    root: Entity,
    material: Handle<WorldTreeMaterial>,
    ground: Entity,
    ground_material: Handle<ground::MossMaterial>,
    uniform: UniformBuffer<WorldTreeUniform>,
    id: BufferId,
    life: Lifetime,
    warm: u32,
    depths: Vec2,
}
pub struct Input {
    pub trees: BTreeMap<Coordinate, u32>,
    pub nuclear: BTreeSet<Coordinate>,
    pub projection: BodyProjection,
    pub step: u64,
    pub motion_step: u64,
    pub reduced: bool,
    pub settled: bool,
    pub warm: Handle<Image>,
}
#[derive(Resource, Default)]
pub struct WorldTreeRuntime {
    plots: BTreeMap<Coordinate, Plot>,
    mesh: Option<Handle<Mesh>>,
    ground_mesh: Option<Handle<Mesh>>,
    retired: Vec<Buffer>,
    warm_camera: Option<Entity>,
    ready: bool,
    pub revision: u64,
    writes: u64,
    retirements: u64,
    builds: u64,
}
impl WorldTreeRuntime {
    pub fn ready(&self) -> bool {
        self.ready
    }
    pub fn covers(&self, c: Coordinate) -> bool {
        self.plots.get(&c).is_some_and(|p| p.warm >= 2)
    }
    pub fn progress(&self) -> Option<String> {
        (!self.ready).then(|| format!("Preparing world trees ({} canopies)", self.plots.len()))
    }
    pub fn diagnostics(&self) -> serde_json::Value {
        serde_json::json!({"ready":self.ready,"trees":self.plots.len(),"uniforms":self.plots.len()+self.retired.len(),"uniformWrites":self.writes,"retiredUniforms":self.retirements,"geometryBuilds":self.builds,"groundMesh":self.ground_mesh.as_ref().map(|m|format!("{:?}",m.id())),"mesh":self.mesh.as_ref().map(|m|format!("{:?}",m.id())),"sample":self.plots.iter().map(|(c,p)|serde_json::json!({"coordinate":c,"seed":p.seed,"entity":p.root.to_string(),"buffer":format!("{:?}",p.id),"born":p.life.born,"destroy":p.life.destroy,"collapse":p.uniform.get().destruction.to_array(),"groundCollapse":p.uniform.get().ground_destruction.to_array(),"depths":p.depths.to_array(),"motionClock":p.uniform.get().clock.x,"warmFrames":p.warm})).collect::<Vec<_>>()})
    }
    fn update(
        &mut self,
        world: &mut World,
        input: Input,
        uploaded: &BTreeSet<Entity>,
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<(), String> {
        if input.trees.len() > MAX_TREES {
            return Err("World-tree plot limit exceeded".into());
        }
        let seconds = input.step as f64 / 60.;
        let motion_seconds = input.motion_step as f64 / 60.;
        let mut remove = Vec::new();
        for (&c, p) in &mut self.plots {
            if input.settled && input.trees.get(&c) == Some(&p.seed) {
                p.life = Lifetime {
                    born: -1000.,
                    born_motion: -1000.,
                    destroy: None,
                    destroy_motion: None,
                };
            }
            if input.trees.get(&c) == Some(&p.seed) && p.life.destroy.is_some() {
                p.life = Lifetime {
                    born: seconds,
                    born_motion: motion_seconds,
                    destroy: None,
                    destroy_motion: None,
                };
            }
            if input.trees.get(&c) != Some(&p.seed) {
                if input.nuclear.contains(&c)
                    && !input.settled
                    && world
                        .resource::<crate::nuclear::timeline::Timeline>()
                        .live(c)
                {
                    if p.life.destroy.is_none()
                        && let Some(start) = world
                            .resource::<crate::nuclear::timeline::Timeline>()
                            .start(c)
                    {
                        let (seconds, motion_seconds) = start.seconds();
                        p.life.destroy = Some(seconds);
                        p.life.destroy_motion = Some(motion_seconds);
                    }
                } else {
                    remove.push(c);
                }
            }
            if p.life.ended(seconds, input.reduced) {
                remove.push(c);
            }
        }
        remove.sort_unstable();
        remove.dedup();
        for c in remove {
            if let Some(p) = self.plots.remove(&c) {
                world.despawn(p.root);
                self.retired.push(p.uniform.buffer().unwrap().clone());
                self.revision += 1;
            }
        }
        let noises = if input.trees.is_empty() {
            None
        } else {
            let library = world.resource::<super::texture_library::TextureLibrary>();
            library
                .get(super::textures::Slot::Shared(3))
                .zip(library.get(super::textures::Slot::Shared(4)))
        };
        if !input.trees.is_empty() && noises.is_none() {
            self.ready = false;
            return Ok(());
        }
        if !input.trees.is_empty() && self.mesh.is_none() {
            self.mesh = Some(
                world
                    .resource_mut::<Assets<Mesh>>()
                    .add(mesh::canopy_mesh()),
            );
            self.ground_mesh = Some(
                world
                    .resource_mut::<Assets<Mesh>>()
                    .add(mesh::ground_mesh()),
            );
            self.builds += 1;
        }
        let incoming: Vec<_> = input
            .trees
            .iter()
            .filter(|(c, _)| !self.plots.contains_key(c))
            .take(8.min(MAX_TREES.saturating_sub(self.plots.len() + self.retired.len())))
            .map(|(c, s)| (*c, *s))
            .collect();
        for (c, seed) in incoming {
            let life = Lifetime {
                born: if !input.settled { seconds } else { -1000. },
                born_motion: if !input.settled {
                    motion_seconds
                } else {
                    -1000.
                },
                destroy: None,
                destroy_motion: None,
            };
            let normal = input
                .projection
                .normal(input.projection.hex_center(AxialPoint { q: c.0, r: c.1 }))
                .map_err(|e| e.to_string())?;
            let rotation = bevy::math::DQuat::from_rotation_arc(
                bevy::math::DVec3::Z,
                bevy::math::DVec3::from_array(normal),
            );
            let meshes = world.resource::<Assets<Mesh>>();
            let depths = Vec2::new(
                mesh::collapse_depth(
                    meshes.get(self.ground_mesh.as_ref().unwrap()).unwrap(),
                    rotation,
                    0.,
                ),
                mesh::collapse_depth(
                    meshes.get(self.mesh.as_ref().unwrap()).unwrap(),
                    rotation,
                    0.15,
                ),
            );
            let parameters = life.parameters(
                seed,
                input.projection.content_radius() as f32,
                seconds,
                motion_seconds,
                input.reduced,
                depths,
            );
            let mut uniform = UniformBuffer::from(parameters);
            uniform.set_label(Some("world tree parameters"));
            uniform.write_buffer(device, queue);
            let id = uniform.buffer().unwrap().id();
            let atmosphere = world
                .resource::<crate::atmosphere::AtmosphereRuntime>()
                .binding()?;
            let material =
                world
                    .resource_mut::<Assets<WorldTreeMaterial>>()
                    .add(WorldTreeMaterial {
                        atmosphere: atmosphere.clone(),
                        parameters: uniform.buffer().unwrap().clone(),
                    });
            let point = Vec3::from_array(
                input
                    .projection
                    .project_content(
                        input.projection.hex_center(AxialPoint { q: c.0, r: c.1 }),
                        0.,
                    )
                    .map_err(|e| e.to_string())?
                    .map(|v| v as f32),
            );
            let root = world
                .spawn((
                    Mesh3d(self.mesh.as_ref().unwrap().clone()),
                    MeshMaterial3d(material.clone()),
                    Transform::from_translation(point)
                        .with_rotation(Quat::from_rotation_arc(Vec3::Z, point.normalize())),
                    Visibility::Inherited,
                    RenderLayers::layer(2),
                    NoFrustumCulling,
                    NotShadowCaster,
                    NotShadowReceiver,
                ))
                .id();
            let (mid, high) = noises.as_ref().unwrap();
            let ground_material =
                world
                    .resource_mut::<Assets<ground::MossMaterial>>()
                    .add(ground::material(
                        uniform.buffer().unwrap().clone(),
                        atmosphere.clone(),
                        mid.clone(),
                        high.clone(),
                    ));
            let ground = world
                .spawn((
                    Mesh3d(self.ground_mesh.as_ref().unwrap().clone()),
                    MeshMaterial3d(ground_material.clone()),
                    Transform::IDENTITY,
                    Visibility::Inherited,
                    RenderLayers::layer(2),
                    NoFrustumCulling,
                    NotShadowCaster,
                    NotShadowReceiver,
                    ChildOf(root),
                    crate::transparent::TransparentOrder::WORLD_TREE_GROUND,
                ))
                .id();
            world
                .entity_mut(root)
                .insert(crate::transparent::TransparentOrder::WORLD_TREE_CANOPY);
            self.plots.insert(
                c,
                Plot {
                    seed,
                    root,
                    material,
                    ground,
                    ground_material,
                    uniform,
                    id,
                    life,
                    warm: 0,
                    depths,
                },
            );
        }
        for p in self.plots.values_mut() {
            if uploaded.contains(&p.root) && p.warm < 2 {
                p.warm += 1;
                if p.warm == 2 {
                    world.entity_mut(p.root).insert(RenderLayers::default());
                    world.entity_mut(p.ground).insert(RenderLayers::default());
                    self.revision += 1;
                }
            }
            // Freeze the intact canopy at the impact instant before sinking it.
            let decorative = p.life.destroy_motion.map_or(motion_seconds, |d| {
                motion_seconds.min(d + if input.reduced { 0. } else { 1. })
            });
            p.uniform.set(p.life.parameters(
                p.seed,
                input.projection.content_radius() as f32,
                seconds,
                decorative,
                input.reduced,
                p.depths,
            ));
            p.uniform.write_buffer(device, queue);
            self.writes += 1;
            if p.uniform.buffer().unwrap().id() != p.id {
                return Err("World-tree uniform storage changed".into());
            }
        }
        let warming = self.plots.values().any(|p| p.warm < 2);
        if warming && self.warm_camera.is_none() {
            self.warm_camera = Some(
                world
                    .spawn((
                        crate::terrain::surface::SurfaceCamera::Warm,
                        Camera3d::default(),
                        bevy::core_pipeline::prepass::DepthPrepass,
                        Camera {
                            order: -6,
                            ..default()
                        },
                        Msaa::Off,
                        Tonemapping::None,
                        RenderTarget::Image(input.warm.into()),
                        RenderLayers::layer(2),
                        Transform::from_xyz(0., 0., 6.8).looking_at(Vec3::ZERO, Vec3::Y),
                    ))
                    .id(),
            );
        } else if !warming && let Some(c) = self.warm_camera.take() {
            world.despawn(c);
        }
        self.ready = input.trees.keys().all(|c| self.covers(*c));
        if self.plots.is_empty() {
            self.mesh = None;
            self.ground_mesh = None;
        }
        Ok(())
    }
}
/// Source Tile entrance affects published actors; warm draws remain full size.
pub fn apply_entry(world: &mut World) {
    world.resource_scope(|world, runtime: Mut<WorldTreeRuntime>| {
        for (&coordinate, plot) in &runtime.plots {
            let scale = if plot.warm < 2 {
                1.
            } else {
                crate::entry::tile_scale(world, coordinate)
            };
            world.get_mut::<Transform>(plot.root).unwrap().scale = Vec3::splat(scale.max(0.000001));
            crate::entry::publish(world, plot.root, scale);
        }
    });
}

pub fn prepare_queued(
    app: &mut App,
    input: Option<Input>,
    device: &RenderDevice,
    queue: &RenderQueue,
) -> Result<(), String> {
    let _profile = crate::profiling::span(app.world(), "world_tree.prepare");
    let render = app.sub_app(RenderApp).world();
    let runtime = app.world().resource::<WorldTreeRuntime>();
    let mut uploaded = BTreeSet::new();
    if render
        .resource::<PipelineCache>()
        .waiting_pipelines()
        .next()
        .is_none()
        && let (Some(meshes), Some(materials)) = (
            render.get_resource::<RenderAssets<RenderMesh>>(),
            render.get_resource::<ErasedRenderAssets<PreparedMaterial>>(),
        )
    {
        for p in runtime.plots.values() {
            if runtime
                .mesh
                .as_ref()
                .is_some_and(|h| meshes.get(h.id()).is_some())
                && runtime
                    .ground_mesh
                    .as_ref()
                    .is_some_and(|h| meshes.get(h.id()).is_some())
                && materials.get(p.material.id().untyped()).is_some()
                && materials.get(p.ground_material.id().untyped()).is_some()
            {
                uploaded.insert(p.root);
            }
        }
    }
    let mut runtime = app
        .world_mut()
        .remove_resource::<WorldTreeRuntime>()
        .unwrap();
    for buffer in runtime.retired.drain(..) {
        buffer.destroy();
        runtime.retirements += 1;
    }
    let result = if let Some(input) = input {
        runtime.update(app.world_mut(), input, &uploaded, device, queue)
    } else {
        Ok(())
    };
    app.world_mut().insert_resource(runtime);
    result
}
pub fn reset(world: &mut World) {
    let Some(mut r) = world.remove_resource::<WorldTreeRuntime>() else {
        return;
    };
    for (_, p) in std::mem::take(&mut r.plots) {
        world.despawn(p.root);
        r.retired.push(p.uniform.buffer().unwrap().clone());
    }
    if let Some(c) = r.warm_camera.take() {
        world.despawn(c);
    }
    world.insert_resource(WorldTreeRuntime {
        retired: r.retired,
        revision: r.revision + 1,
        writes: r.writes,
        retirements: r.retirements,
        builds: r.builds,
        ..default()
    });
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn original_disc_has_exact_rings_uvs_winding_and_bounded_storage() {
        let mesh = mesh::canopy_mesh();
        let positions = mesh
            .attribute(Mesh::ATTRIBUTE_POSITION)
            .unwrap()
            .as_float3()
            .unwrap();
        assert_eq!(positions.len(), 1569);
        let Some(bevy::mesh::VertexAttributeValues::Float32x2(uvs)) =
            mesh.attribute(Mesh::ATTRIBUTE_UV_0)
        else {
            panic!("UVs")
        };
        let Some(bevy::mesh::Indices::U32(indices)) = mesh.indices() else {
            panic!("Indices")
        };
        assert_eq!(indices.len(), 9240);
        assert_eq!(positions[0], [0.; 3]);
        assert_eq!(uvs[0], [0.5; 2]);
        for (p, uv) in positions.iter().zip(uvs) {
            assert!((p[0].hypot(p[1])) <= 0.140001);
            assert!((uv[0] - (0.5 + p[0] / 0.28)).abs() < 1e-6);
            assert!((uv[1] - (0.5 + p[1] / 0.28)).abs() < 1e-6);
        }
        for triangle in indices.as_chunks::<3>().0 {
            let p = [triangle[0], triangle[1], triangle[2]]
                .map(|i| Vec3::from_array(positions[i as usize]));
            assert!((p[1] - p[0]).cross(p[2] - p[0]).z > 0.);
        }
        assert!(positions.len() * 32 + indices.len() * 4 < 128 * 1024);
    }
    #[test]
    fn moss_disc_keeps_original_resolution_rotation_and_unrotated_uvs() {
        let mesh = mesh::ground_mesh();
        let positions = mesh
            .attribute(Mesh::ATTRIBUTE_POSITION)
            .unwrap()
            .as_float3()
            .unwrap();
        let Some(bevy::mesh::VertexAttributeValues::Float32x2(uvs)) =
            mesh.attribute(Mesh::ATTRIBUTE_UV_0)
        else {
            panic!("UVs")
        };
        assert_eq!(positions.len(), 361);
        assert_eq!(mesh.indices().unwrap().len(), 2052);
        assert!(positions[1][0].abs() < 1e-7);
        assert!((positions[1][1] - 0.02988).abs() < 1e-7);
        assert_eq!(uvs[1], [0.55, 0.5]);
        for p in positions {
            assert!(p[0].hypot(p[1]) <= 0.298801);
        }
    }
    #[test]
    fn placement_uses_canonical_age_and_independent_decorative_birth() {
        let life = Lifetime {
            born: 10.,
            born_motion: 4.,
            destroy: None,
            destroy_motion: None,
        };
        let full = life.parameters(37, 2.02, 10.75, 4.75, false, Vec2::new(0.1114822, 0.215));
        assert!((full.clock.y - 0.875).abs() < 1e-6);
        assert_eq!(full.geometry.w, 4.);
        assert_eq!(full.clock.x, 4.75);
        let reduced = life.parameters(37, 2.02, 10.075, 4., true, Vec2::new(0.1114822, 0.215));
        assert_eq!(reduced.clock.y, 1.);
        assert!((reduced.clock.w - 0.5).abs() < 1e-5);
        assert_eq!(reduced.sun_color.w, 0.);
        assert_eq!(reduced.destruction, Vec4::ZERO);
        let initial = Lifetime {
            born: -1000.,
            born_motion: -1000.,
            destroy: None,
            destroy_motion: None,
        };
        assert_eq!(
            initial
                .parameters(37, 2.02, 0., 0., false, Vec2::new(0.1114822, 0.215))
                .clock
                .y,
            1.
        );
        assert_eq!(
            initial
                .parameters(37, 2.02, 0., 0., true, Vec2::new(0.1114822, 0.215))
                .clock
                .w,
            1.
        );
    }
    #[test]
    fn nuclear_collapse_and_reduced_crossfade_end_at_original_bounds() {
        let life = Lifetime {
            born: -1000.,
            born_motion: -1000.,
            destroy: Some(10.),
            destroy_motion: Some(4.),
        };
        let p = life.parameters(37, 2.02, 11.5, 4., false, Vec2::new(0.1114822, 0.215));
        assert!(p.clock.z > 0.);
        assert!(p.destruction.w > 0.);
        assert_eq!(p.clock.x, 4.);
        assert!(!life.ended(12., false));
        assert!(life.ended(10. + 128. / 60., false));
        let reduced = life.parameters(37, 2.02, 10.075, 4., true, Vec2::new(0.1114822, 0.215));
        assert!((reduced.clock.w - 0.5).abs() < 1e-5);
        assert_eq!(reduced.destruction, Vec4::ZERO);
        assert!(life.ended(10.15, true));
    }
}
