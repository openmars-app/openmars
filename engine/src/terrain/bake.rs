//! Direct, bounded ground-cache passes share the live ground composition shader.
//! Four stripes and their mips publish on the same ordered queue as consumers.
use super::{
    materials::*,
    mips::{self, MipRenderer, MipWork},
};
use crate::{assets::AssetCamera, world::WorldView};
use bevy::{
    asset::AssetId,
    mesh::MeshVertexBufferLayoutRef,
    prelude::*,
    render::{
        RenderApp,
        mesh::{RenderMesh, allocator::MeshAllocator},
        render_asset::RenderAssets,
        render_resource::{binding_types::*, *},
        renderer::{RenderDevice, RenderQueue},
        texture::GpuImage,
    },
    shader::ShaderDefVal,
};
use openmars_scene::geometry::{BoardPoint, BodyProjection, Ray3};
const STRIPS: u32 = 4;
// Direct RGBA8 bases and sampled mip chains, plus bounded uniform headroom.
// No HDR ping-pong textures, depth buffers or temporary camera attachments.
const CACHE_BYTES: u64 = 128 * 1024 * 1024;
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct BakeView {
    pub bounds: [f64; 4],
    pub density: f64,
}
impl BakeView {
    fn drift(self, other: Self) -> f64 {
        (0..4)
            .map(|i| (self.bounds[i] - other.bounds[i]).abs())
            .fold(0., f64::max)
    }
    fn covers(self, other: Self) -> bool {
        self.bounds[0] <= other.bounds[0]
            && self.bounds[1] <= other.bounds[1]
            && self.bounds[0] + self.bounds[2] >= other.bounds[0] + other.bounds[2]
            && self.bounds[1] + self.bounds[3] >= other.bounds[1] + other.bounds[3]
    }
}
#[derive(Clone, PartialEq, Eq)]
struct Identity {
    batch: u64,
    climate: [u32; 5],
    textures: [AssetId<Image>; 4],
    shade_bounds: [u32; 4],
    shade_revision: u64,
}
pub struct BakeSource {
    pub batch: u64,
    pub mesh: Handle<Mesh>,
    pub textures: GroundTextures,
    pub parameters: TerrainUniform,
    pub bounds: [BoardPoint; 2],
    pub shade_revision: u64,
}
impl BakeSource {
    fn identity(&self) -> Identity {
        Identity {
            batch: self.batch,
            climate: [
                self.parameters.climate.x.to_bits(),
                self.parameters.climate.y.to_bits(),
                self.parameters.climate.z.to_bits(),
                self.parameters.climate.w.to_bits(),
                self.parameters.water_extra.w.to_bits(),
            ],
            textures: [
                self.textures.color_atlas.id(),
                self.textures.detail_atlas.id(),
                self.textures.mars.id(),
                self.textures.plant_shade.id(),
            ],
            shade_bounds: self.parameters.shade_bounds.to_array().map(f32::to_bits),
            shade_revision: self.shade_revision,
        }
    }
}
pub struct GroundBakePlugin;
impl Plugin for GroundBakePlugin {
    fn build(&self, app: &mut App) {
        app.init_resource::<BakeRuntime>()
            .init_resource::<MipRenderer>();
    }
}
struct Targets {
    bases: [Handle<Image>; 2],
    sampled: [Handle<Image>; 2],
    width: u32,
    height: u32,
    gpu: Option<[Texture; 4]>,
}
struct Pending {
    identity: Identity,
    view: BakeView,
    strip: u32,
    source: BakeSource,
    mips: Option<MipWork>,
}
struct Active {
    identity: Identity,
    view: BakeView,
    density: f64,
}
#[derive(Clone, PartialEq)]
struct ViewKey {
    projection: Mat4,
    transform: Mat4,
    viewport: Option<Rect>,
    physical: UVec2,
    bounds: [BoardPoint; 2],
}
struct MeasuredView {
    key: ViewKey,
    view: Option<BakeView>,
}
struct DirectPass {
    layout: BindGroupLayoutDescriptor,
    empty: BindGroupLayoutDescriptor,
    pipelines: [CachedRenderPipelineId; 2],
    mesh_layout: MeshVertexBufferLayoutRef,
    uniform: UniformBuffer<TerrainUniform>,
    bindings: Option<([AssetId<Image>; 6], BindGroup)>,
}
impl DirectPass {
    fn new(
        app: &App,
        mesh_layout: MeshVertexBufferLayoutRef,
        device: &RenderDevice,
        queue: &RenderQueue,
        parameters: TerrainUniform,
    ) -> Result<Self, String> {
        let uniform = uniform_buffer::<TerrainUniform>(false);
        let array = || texture_2d_array(TextureSampleType::Float { filterable: true });
        let image = || texture_2d(TextureSampleType::Float { filterable: true });
        let filter = || sampler(SamplerBindingType::Filtering);
        let layout = BindGroupLayoutDescriptor::new(
            "terrain direct bake",
            &BindGroupLayoutEntries::with_indices(
                ShaderStages::VERTEX_FRAGMENT,
                (
                    (0, uniform),
                    (1, array()),
                    (2, filter()),
                    (3, array()),
                    (5, array()),
                    (6, filter()),
                    (7, array()),
                    (8, image()),
                    (9, filter()),
                    (10, image()),
                    (11, filter()),
                ),
            ),
        );
        let empty = BindGroupLayoutDescriptor::new("terrain direct bake unused group", &[]);
        let shader = app
            .world()
            .resource::<AssetServer>()
            .load("embedded://openmars_engine/terrain/shaders/ground.wesl");
        let vertex_layout = mesh_layout
            .0
            .get_layout(&[
                Mesh::ATTRIBUTE_UV_0.at_shader_location(0),
                Mesh::ATTRIBUTE_COLOR.at_shader_location(1),
                Mesh::ATTRIBUTE_UV_1.at_shader_location(2),
            ])
            .map_err(|e| e.to_string())?;
        let cache = app.sub_app(RenderApp).world().resource::<PipelineCache>();
        let pipelines = [TextureFormat::Rgba8UnormSrgb, TextureFormat::Rgba8Unorm].map(|format| {
            let defs = vec![
                ShaderDefVal::Bool("TERRAIN_BAKE_DIRECT".into(), true),
                ShaderDefVal::Bool(
                    if format.is_srgb() {
                        "TERRAIN_BAKE_ALBEDO"
                    } else {
                        "TERRAIN_BAKE_SURFACE"
                    }
                    .into(),
                    true,
                ),
            ];
            cache.queue_render_pipeline(RenderPipelineDescriptor {
                label: Some("terrain direct bake".into()),
                layout: vec![empty.clone(), empty.clone(), empty.clone(), layout.clone()],
                vertex: VertexState {
                    shader: shader.clone(),
                    shader_defs: defs.clone(),
                    entry_point: Some("bake_vertex".into()),
                    buffers: vec![vertex_layout.clone()],
                    ..default()
                },
                fragment: Some(FragmentState {
                    shader: shader.clone(),
                    shader_defs: defs,
                    entry_point: Some("bake_fragment".into()),
                    targets: vec![Some(ColorTargetState {
                        format,
                        blend: None,
                        write_mask: ColorWrites::ALL,
                    })],
                    ..default()
                }),
                primitive: PrimitiveState {
                    cull_mode: None,
                    ..default()
                },
                depth_stencil: None,
                ..default()
            })
        });
        let mut uniform: UniformBuffer<TerrainUniform> = parameters.into();
        uniform.set_label(Some("terrain direct bake parameters"));
        uniform.write_buffer(device, queue);
        Ok(Self {
            layout,
            empty,
            pipelines,
            mesh_layout,
            uniform,
            bindings: None,
        })
    }
    fn draw(
        &mut self,
        app: &App,
        pending: &Pending,
        targets: &mut Targets,
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<bool, String> {
        let render = app.sub_app(RenderApp).world();
        let images = render.resource::<RenderAssets<GpuImage>>();
        if targets.gpu.is_none() {
            let handles = [
                &targets.bases[0],
                &targets.bases[1],
                &targets.sampled[0],
                &targets.sampled[1],
            ];
            let gpu: Option<Vec<_>> = handles
                .into_iter()
                .map(|handle| images.get(handle.id()).map(|image| image.texture.clone()))
                .collect();
            let Some(gpu) = gpu else { return Ok(false) };
            targets.gpu = Some(gpu.try_into().map_err(|_| "Terrain bake target count")?);
        }
        let cache = render.resource::<PipelineCache>();
        let (Some(albedo), Some(surface)) = (
            cache.get_render_pipeline(self.pipelines[0]),
            cache.get_render_pipeline(self.pipelines[1]),
        ) else {
            return Ok(false);
        };
        let meshes = render.resource::<RenderAssets<RenderMesh>>();
        let Some(mesh) = meshes.get(pending.source.mesh.id()) else {
            return Ok(false);
        };
        if mesh.layout != self.mesh_layout {
            return Err("Terrain bake mesh layout changed".into());
        }
        let allocator = render.resource::<MeshAllocator>();
        let (Some(vertices), Some(indices), Some(index_format)) = (
            allocator.mesh_vertex_slice(&pending.source.mesh.id()),
            allocator.mesh_index_slice(&pending.source.mesh.id()),
            mesh.index_format(),
        ) else {
            return Ok(false);
        };
        let t = &pending.source.textures;
        let ids = [
            t.terrain.id(),
            t.surface_fields.id(),
            t.color_atlas.id(),
            t.detail_atlas.id(),
            t.mars.id(),
            t.plant_shade.id(),
        ];
        let textures: Option<Vec<_>> = ids.iter().map(|id| images.get(*id)).collect();
        let Some(textures) = textures else {
            return Ok(false);
        };
        let (Some(output_a), Some(output_b)) = (
            images.get(targets.bases[0].id()),
            images.get(targets.bases[1].id()),
        ) else {
            return Ok(false);
        };
        let mut parameters = pending.source.parameters;
        parameters.bake_bounds = Vec4::from_array(pending.view.bounds.map(|v| v as f32));
        parameters.bake = Vec4::new(
            0.,
            parameters.bake.y,
            pending.strip as f32 / STRIPS as f32,
            1. / STRIPS as f32,
        );
        self.uniform.set(parameters);
        self.uniform.write_buffer(device, queue);
        if self
            .bindings
            .as_ref()
            .is_none_or(|(prior, _)| *prior != ids)
        {
            self.bindings = Some((
                ids,
                device.create_bind_group(
                    "terrain direct bake",
                    &cache.get_bind_group_layout(&self.layout),
                    &BindGroupEntries::with_indices((
                        (0, self.uniform.binding().unwrap()),
                        (1, &textures[0].texture_view),
                        (2, &textures[0].sampler),
                        (3, &textures[1].texture_view),
                        (5, &textures[2].texture_view),
                        (6, &textures[2].sampler),
                        (7, &textures[3].texture_view),
                        (8, &textures[4].texture_view),
                        (9, &textures[4].sampler),
                        (10, &textures[5].texture_view),
                        (11, &textures[5].sampler),
                    )),
                ),
            ));
        }
        let empty = device.create_bind_group(
            "terrain bake empty",
            &cache.get_bind_group_layout(&self.empty),
            &[],
        );
        let mut encoder = device.create_command_encoder(&wgpu::CommandEncoderDescriptor {
            label: Some("terrain direct bake stripe"),
        });
        for (pipeline, output) in [(albedo, output_a), (surface, output_b)] {
            let mut pass = encoder.begin_render_pass(&wgpu::RenderPassDescriptor {
                label: Some("terrain direct bake"),
                color_attachments: &[Some(wgpu::RenderPassColorAttachment {
                    view: &output.texture_view,
                    depth_slice: None,
                    resolve_target: None,
                    ops: wgpu::Operations {
                        load: if pending.strip == 0 {
                            wgpu::LoadOp::Clear(wgpu::Color::TRANSPARENT)
                        } else {
                            wgpu::LoadOp::Load
                        },
                        store: wgpu::StoreOp::Store,
                    },
                })],
                depth_stencil_attachment: None,
                timestamp_writes: None,
                occlusion_query_set: None,
                multiview_mask: None,
            });
            let stripe_height = targets.height / STRIPS;
            pass.set_viewport(
                0.,
                (pending.strip * stripe_height) as f32,
                targets.width as f32,
                stripe_height as f32,
                0.,
                1.,
            );
            pass.set_scissor_rect(
                0,
                pending.strip * stripe_height,
                targets.width,
                stripe_height,
            );
            pass.set_pipeline(pipeline);
            pass.set_bind_group(0, &empty, &[]);
            pass.set_bind_group(1, &empty, &[]);
            pass.set_bind_group(2, &empty, &[]);
            pass.set_bind_group(3, &self.bindings.as_ref().unwrap().1, &[]);
            pass.set_vertex_buffer(0, *vertices.buffer.slice(..));
            pass.set_index_buffer(*indices.buffer.slice(..), index_format);
            pass.draw_indexed(indices.range.clone(), vertices.range.start as i32, 0..1);
        }
        queue.submit([encoder.finish()]);
        Ok(true)
    }
}
#[derive(Resource, Default)]
pub struct BakeRuntime {
    targets: Option<Targets>,
    extent: Option<[u32; 2]>,
    pending: Option<Pending>,
    active: Option<Active>,
    wanted: Option<(Identity, BakeView, u8)>,
    direct: Option<DirectPass>,
    measured: Option<MeasuredView>,
    pub mip_draws: u64,
    pub stripes: u64,
    pub publications: u64,
    pub cancellations: u64,
    pub uniform_writes: u64,
    pub view_measurements: u64,
    pub view_cache_hits: u64,
    pub draw_calls: u64,
}
impl BakeRuntime {
    pub fn uniforms(&self) -> usize {
        usize::from(
            self.direct
                .as_ref()
                .is_some_and(|direct| direct.uniform.buffer().is_some()),
        )
    }
    pub fn valid(&self) -> bool {
        self.active.is_some()
    }
    pub fn extent(&self) -> [u32; 2] {
        self.extent.unwrap_or([0, 0])
    }
    pub fn pending_strip(&self) -> u32 {
        self.pending.as_ref().map_or(0, |p| p.strip + 1)
    }
    pub fn revision(&self) -> u64 {
        self.active.as_ref().map_or(0, |a| a.identity.batch)
    }
    pub fn configure(&self, revision: u64, p: &mut TerrainUniform) {
        if let Some(active) = &self.active
            && active.identity.batch == revision
        {
            p.bake.x = 1.;
            p.bake_bounds = Vec4::from_array(active.view.bounds.map(|v| v as f32));
        }
    }
    fn cancel(&mut self) {
        self.active = None;
        self.wanted = None;
        if self.pending.take().is_some() {
            self.cancellations += 1;
        }
    }
    fn targets(&mut self, world: &mut World, limit: u32) -> Result<(), String> {
        if self.targets.is_some() {
            return Ok(());
        }
        let width = choose_extent(limit, CACHE_BYTES, self.extent)
            .ok_or("Insufficient terrain bake cache budget")?;
        let height = width / 2;
        let _profile = crate::profiling::span(world, "terrain.bake.targets.allocate")
            .arg("width", width)
            .arg("height", height);
        self.extent = Some([width, height]);
        let mut images = world.resource_mut::<Assets<Image>>();
        let bases = [
            images.add(mips::target_image(width, height, 1, 1, true)),
            images.add(mips::target_image(width, height, 1, 1, false)),
        ];
        let sampled = [
            images.add(mips::target_image(
                width,
                height,
                1,
                width.ilog2() + 1,
                true,
            )),
            images.add(mips::target_image(
                width,
                height,
                1,
                width.ilog2() + 1,
                false,
            )),
        ];
        self.targets = Some(Targets {
            bases,
            sampled,
            width,
            height,
            gpu: None,
        });
        Ok(())
    }
    fn start(
        &mut self,
        world: &mut World,
        source: BakeSource,
        view: BakeView,
        device: &RenderDevice,
    ) -> Result<(), String> {
        let _profile =
            crate::profiling::span(world, "terrain.bake.start").arg("revision", source.batch);
        self.targets(world, device.limits().max_texture_dimension_2d)?;
        self.pending = Some(Pending {
            identity: source.identity(),
            view,
            strip: 0,
            source,
            mips: None,
        });
        Ok(())
    }
    fn measure(&mut self, world: &mut World, bounds: [BoardPoint; 2]) -> Option<BakeView> {
        let (camera, transform) = world
            .query_filtered::<(&Camera, &GlobalTransform), With<AssetCamera>>()
            .iter(world)
            .next()
            .map(|(c, t)| (c.clone(), *t))?;
        let physical = world.resource::<WorldView>().physical;
        let key = ViewKey {
            projection: camera.clip_from_view(),
            transform: transform.to_matrix(),
            viewport: camera.logical_viewport_rect(),
            physical,
            bounds,
        };
        if let Some(prior) = &self.measured
            && prior.key == key
        {
            self.view_cache_hits += 1;
            return prior.view;
        }
        let view = measure_view(&camera, &transform, physical, bounds);
        self.view_measurements += 1;
        self.measured = Some(MeasuredView { key, view });
        view
    }
}
pub fn prepare_queued(
    app: &mut App,
    source: Option<BakeSource>,
    warm: Option<(Handle<Mesh>, TerrainUniform)>,
    device: &RenderDevice,
    queue: &RenderQueue,
) -> Result<(), String> {
    let _profile = crate::profiling::span(app.world(), "terrain.bake.prepare");
    let mut runtime = app
        .world_mut()
        .remove_resource::<BakeRuntime>()
        .ok_or("Missing bake runtime")?;
    let result = (|| {
        // The empty board already carries the real terrain vertex layout.
        // Compile the shared bake passes now, without allocating a terrain
        // cache or inventing a placed tile just to trigger them.
        if runtime.direct.is_none()
            && let Some((mesh, parameters)) = warm.and_then(|(handle, parameters)| {
                app.sub_app(RenderApp)
                    .world()
                    .resource::<RenderAssets<RenderMesh>>()
                    .get(handle.id())
                    .map(|mesh| (mesh.layout.clone(), parameters))
            })
        {
            runtime.direct = Some(DirectPass::new(app, mesh, device, queue, parameters)?);
        }
        advance_queued(app, &mut runtime, source, device, queue)
    })();
    app.world_mut().insert_resource(runtime);
    result
}
fn advance_queued(
    app: &mut App,
    runtime: &mut BakeRuntime,
    source: Option<BakeSource>,
    device: &RenderDevice,
    queue: &RenderQueue,
) -> Result<(), String> {
    let Some(source) = source else {
        runtime.cancel();
        return Ok(());
    };
    let Some(view) = runtime.measure(app.world_mut(), source.bounds) else {
        runtime.cancel();
        return Ok(());
    };
    let identity = source.identity();
    if let Some(active) = &runtime.active {
        let targets = runtime.targets.as_ref().unwrap();
        let achievable = (f64::from(targets.width) / view.bounds[2])
            .min(f64::from(targets.height) / view.bounds[3]);
        if active.identity == identity
            && active.view.covers(view)
            && !(view.density > active.density * 1.5 && achievable > active.density * 1.5)
        {
            return Ok(());
        }
    }
    runtime.active = None;
    if runtime
        .pending
        .as_ref()
        .is_some_and(|p| p.identity != identity || !p.view.covers(view))
    {
        runtime.cancel();
    }
    if let Some(pending) = &mut runtime.pending {
        if let Some(work) = &mut pending.mips {
            let mut renderer = app.world_mut().remove_resource::<MipRenderer>().unwrap();
            let before = renderer.draws;
            let result = renderer.advance_queued(
                work,
                app.sub_app(RenderApp)
                    .world()
                    .resource::<RenderAssets<GpuImage>>(),
                device,
                queue,
            );
            runtime.mip_draws += renderer.draws - before;
            app.world_mut().insert_resource(renderer);
            if result? {
                let pending = runtime.pending.take().unwrap();
                let targets = runtime.targets.as_ref().unwrap();
                let density = (f64::from(targets.width) / pending.view.bounds[2])
                    .min(f64::from(targets.height) / pending.view.bounds[3]);
                bind_targets(app.world_mut(), &source, &targets.sampled)?;
                runtime.active = Some(Active {
                    identity: pending.identity,
                    view: pending.view,
                    density,
                });
                runtime.publications += 1;
            }
            return Ok(());
        }
        let Some(direct) = runtime.direct.as_mut() else {
            return Ok(());
        };
        if direct.draw(
            app,
            pending,
            runtime.targets.as_mut().unwrap(),
            device,
            queue,
        )? {
            runtime.stripes += 1;
            runtime.draw_calls += 2;
            runtime.uniform_writes += 1;
            pending.strip += 1;
            if pending.strip == STRIPS {
                let targets = runtime.targets.as_ref().unwrap();
                pending.mips = Some(MipWork::new(app.world(), &targets.bases, &targets.sampled)?);
            }
        }
        return Ok(());
    }
    let stable = match &mut runtime.wanted {
        Some((old, prior, stable))
            if *old == identity
                && prior.drift(view) <= view.bounds[2].max(view.bounds[3]) * 0.02 =>
        {
            *stable = stable.saturating_add(1);
            *prior = view;
            *stable
        }
        _ => {
            runtime.wanted = Some((identity, view, 1));
            1
        }
    };
    if stable >= 3 {
        runtime.start(app.world_mut(), source, view, device)?;
    }
    Ok(())
}
fn bind_targets(
    world: &mut World,
    source: &BakeSource,
    targets: &[Handle<Image>; 2],
) -> Result<(), String> {
    let ids: Vec<_> = world
        .resource::<Assets<GroundMaterial>>()
        .iter()
        .filter_map(|(id, material)| {
            (material.textures.terrain == source.textures.terrain).then_some(id)
        })
        .collect();
    for id in ids {
        let mut materials = world.resource_mut::<Assets<GroundMaterial>>();
        let mut material = materials.get_mut(id).ok_or("Missing bake consumer")?;
        if material.textures.bake_albedo != targets[0]
            || material.textures.bake_surface != targets[1]
        {
            material.textures.bake_albedo = targets[0].clone();
            material.textures.bake_surface = targets[1].clone();
        }
    }
    Ok(())
}
pub fn reset(world: &mut World) {
    let mut runtime = world.remove_resource::<BakeRuntime>().unwrap();
    runtime.cancel();
    if let Some(targets) = runtime.targets.take()
        && let Some(gpu) = targets.gpu
    {
        for texture in gpu {
            texture.destroy();
        }
    }
    runtime.measured = None;
    if let Some(direct) = &mut runtime.direct {
        direct.bindings = None;
        if let Some(buffer) = direct.uniform.buffer() {
            buffer.destroy();
        }
        direct.uniform = UniformBuffer::from(*direct.uniform.get());
        direct
            .uniform
            .set_label(Some("terrain direct bake parameters"));
    }
    world.insert_resource(runtime);
}
fn choose_extent(max: u32, available: u64, existing: Option<[u32; 2]>) -> Option<u32> {
    let max = max.min(4096);
    if max < 256 {
        return None;
    }
    let fits = |width: u32| {
        let height = width / 2;
        let images = 2
            * (mips::mip_bytes(width, height, 1, 1).ok()?
                + mips::mip_bytes(width, height, 1, width.ilog2() + 1).ok()?);
        Some(images + 6 * 65536 <= available)
    };
    if let Some([width, height]) = existing {
        return (width <= max && height == width / 2 && fits(width)?).then_some(width);
    }
    let mut width = 1u32 << (31 - max.leading_zeros());
    while width >= 256 {
        if fits(width)? {
            return Some(width);
        }
        width /= 2;
    }
    None
}
fn measure_view(
    camera: &Camera,
    transform: &GlobalTransform,
    physical: UVec2,
    bounds: [BoardPoint; 2],
) -> Option<BakeView> {
    let projection = BodyProjection::MARS;
    let mut points = [None; 49];
    let mut min = [f64::INFINITY; 2];
    let mut max = [f64::NEG_INFINITY; 2];
    let mut density = 0.0_f64;
    for y in 0usize..7 {
        for x in 0usize..7 {
            let ray = camera
                .viewport_to_world(
                    transform,
                    Vec2::new(
                        physical.x as f32 * x as f32 / 6.,
                        physical.y as f32 * y as f32 / 6.,
                    ),
                )
                .ok();
            let Some(point) = ray
                .and_then(|ray| {
                    Ray3 {
                        origin: ray.origin.to_array().map(f64::from),
                        dir: ray.direction.to_array().map(f64::from),
                    }
                    .intersect_sphere(projection.radius())
                })
                .and_then(|hit| projection.unproject(hit.point))
            else {
                continue;
            };
            let i = y * 7 + x;
            points[i] = Some(point);
            min[0] = min[0].min(point.x);
            min[1] = min[1].min(point.y);
            max[0] = max[0].max(point.x);
            max[1] = max[1].max(point.y);
            for (previous, pixels) in [
                (
                    x.checked_sub(1).map(|x| y * 7 + x),
                    f64::from(physical.x) / 6.,
                ),
                (
                    y.checked_sub(1).map(|y| y * 7 + x),
                    f64::from(physical.y) / 6.,
                ),
            ] {
                if let Some(a) = previous.and_then(|j| points[j]) {
                    let distance = (a.x - point.x).hypot(a.y - point.y);
                    if distance > 1e-8 {
                        density = density.max(pixels / distance);
                    }
                }
            }
        }
    }
    let x = min[0].max(bounds[0].x);
    let y = min[1].max(bounds[0].y);
    let w = max[0].min(bounds[1].x) - x;
    let h = max[1].min(bounds[1].y) - y;
    if w <= 0. || h <= 0. || !density.is_finite() {
        return None;
    }
    let view = BakeView {
        bounds: [x - w * 0.15, y - h * 0.15, w * 1.3, h * 1.3],
        density,
    };
    view.bounds.iter().all(|v| v.is_finite()).then_some(view)
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn cache_budget_admits_exact_linear_and_srgb_images_without_hdr_attachments() {
        assert_eq!(choose_extent(4096, CACHE_BYTES, None), Some(2048));
        assert_eq!(choose_extent(4096, 512 * 1024, None), None);
        assert_eq!(choose_extent(1024, CACHE_BYTES, Some([2048, 1024])), None);
        let bytes = 2
            * (mips::mip_bytes(2048, 1024, 1, 1).unwrap()
                + mips::mip_bytes(2048, 1024, 1, 12).unwrap());
        assert_eq!(bytes, 39_146_840);
        assert!(bytes < CACHE_BYTES);
    }
    #[test]
    fn actual_bake_shader_reuses_live_composition_without_any_view_bindings() {
        use bevy::shader::{Shader, ShaderCache, ShaderCacheSource};
        let mut cache = ShaderCache::new((), |_, source, _| match source {
            ShaderCacheSource::Wgsl(wgsl) => Ok(wgsl),
            _ => panic!("WGSL output expected"),
        });
        let id = |n| AssetId::Uuid {
            uuid: bevy::asset::uuid::Uuid::from_u128(n),
        };
        cache.set_shader(
            id(1),
            Shader::from_wesl(
                include_str!("shaders/field.wesl"),
                "embedded://openmars_engine/terrain/shaders/field.wesl",
            ),
        );
        cache.set_shader(
            id(2),
            Shader::from_wesl(
                include_str!("shaders/ground.wesl"),
                "embedded://openmars_engine/terrain/shaders/ground.wesl",
            ),
        );
        for flag in ["TERRAIN_BAKE_ALBEDO", "TERRAIN_BAKE_SURFACE"] {
            let output = cache
                .get(
                    0,
                    id(2),
                    &[
                        ShaderDefVal::Bool("TERRAIN_BAKE_DIRECT".into(), true),
                        ShaderDefVal::Bool(flag.into(), true),
                    ],
                )
                .unwrap();
            let module = wgpu::naga::front::wgsl::parse_str(&output).unwrap();
            wgpu::naga::valid::Validator::new(
                wgpu::naga::valid::ValidationFlags::all(),
                wgpu::naga::valid::Capabilities::empty(),
            )
            .validate(&module)
            .unwrap();
            assert_eq!(module.entry_points.len(), 2);
            assert!(
                module
                    .entry_points
                    .iter()
                    .any(|p| p.name.ends_with("bake_vertex"))
            );
            assert!(
                module
                    .entry_points
                    .iter()
                    .any(|p| p.name.ends_with("bake_fragment"))
            );
            assert!(
                module
                    .global_variables
                    .iter()
                    .filter_map(|(_, v)| v.binding.as_ref())
                    .all(|binding| binding.group == 3)
            );
            assert!(!output.contains("struct View"));
        }
    }
    #[test]
    fn unchanged_actual_camera_values_skip_all_49_rays_and_replacement_values_invalidate() {
        use bevy::camera::{ComputedCameraValues, RenderTargetInfo};
        let mut world = World::new();
        world.insert_resource(WorldView {
            physical: UVec2::new(1280, 720),
            ..default()
        });
        let camera = world
            .spawn((
                AssetCamera,
                Camera {
                    computed: ComputedCameraValues {
                        clip_from_view: Mat4::perspective_infinite_reverse_rh(
                            0.7,
                            1280. / 720.,
                            0.1,
                        ),
                        target_info: Some(RenderTargetInfo {
                            physical_size: UVec2::new(1280, 720),
                            scale_factor: 1.,
                        }),
                        ..default()
                    },
                    ..default()
                },
                GlobalTransform::from(
                    Transform::from_xyz(0., 0., 6.8).looking_at(Vec3::ZERO, Vec3::Y),
                ),
            ))
            .id();
        let bounds = [BoardPoint { x: -1., y: -1. }, BoardPoint { x: 1., y: 1. }];
        let mut runtime = BakeRuntime::default();
        let first = runtime.measure(&mut world, bounds);
        assert!(first.is_some());
        assert_eq!(runtime.measure(&mut world, bounds), first);
        assert_eq!((runtime.view_measurements, runtime.view_cache_hits), (1, 1));
        // Change detection ticks alone do not invalidate the cache.
        let clone = world.get::<Camera>(camera).unwrap().clone();
        world.entity_mut(camera).insert(clone);
        assert_eq!(runtime.measure(&mut world, bounds), first);
        assert_eq!((runtime.view_measurements, runtime.view_cache_hits), (1, 2));
        world
            .get_mut::<Camera>(camera)
            .unwrap()
            .computed
            .clip_from_view = Mat4::perspective_infinite_reverse_rh(0.8, 1280. / 720., 0.1);
        runtime.measure(&mut world, bounds);
        assert_eq!(runtime.view_measurements, 2);
        world.resource_mut::<WorldView>().physical = UVec2::new(1920, 1080);
        runtime.measure(&mut world, bounds);
        assert_eq!(runtime.view_measurements, 3);
        runtime.measure(&mut world, [bounds[0], BoardPoint { x: 1.2, y: 1. }]);
        assert_eq!(runtime.view_measurements, 4);
        world.entity_mut(camera).insert(GlobalTransform::from(
            Transform::from_xyz(0.2, 0., 6.8).looking_at(Vec3::ZERO, Vec3::Y),
        ));
        runtime.measure(&mut world, bounds);
        assert_eq!(runtime.view_measurements, 5);
    }
}
