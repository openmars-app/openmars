//! Charged low-resolution nuclear volume and depth-aware full-view composition.
//! Uses the current Core3d post-process schedule, never private GL framebuffers.
use super::blast_geometry::{self as geometry, MAX_BLASTS};
use bevy::{
    core_pipeline::{Core3dSystems, schedule::Core3d},
    prelude::*,
    render::{
        Render, RenderApp, RenderStartup, RenderSystems,
        extract_component::{ExtractComponent, ExtractComponentPlugin},
        render_asset::RenderAssets,
        render_resource::{binding_types::*, *},
        renderer::{RenderAdapter, RenderContext, RenderDevice, RenderQueue, ViewQuery},
        texture::GpuImage,
        view::{ExtractedView, ViewDepthStencilTexture, ViewTarget},
    },
};
use std::sync::{
    Arc,
    atomic::{AtomicBool, Ordering},
};

#[derive(Clone, Copy, Debug)]
pub struct Volume {
    pub local: Mat4,
    pub age: f32,
    pub seed: u32,
}
#[derive(Component, Clone, ExtractComponent)]
#[extract_app(RenderApp)]
pub struct NuclearView {
    pub target: Handle<Image>,
    pub extent: [u32; 2],
    pub world_from_board: Mat4,
    pub volumes: Vec<Volume>,
    pub encoded: Arc<AtomicBool>,
    pub probe: Arc<Probe>,
}
#[derive(Default, Debug)]
pub struct Probe {
    pub allocations: std::sync::atomic::AtomicU64,
    pub retirements: std::sync::atomic::AtomicU64,
    pub frames: std::sync::atomic::AtomicU64,
    pub draws: std::sync::atomic::AtomicU64,
    pub bytes: std::sync::atomic::AtomicU64,
    pub buffer: std::sync::Mutex<String>,
}
#[derive(Resource, Default)]
struct RetiredBuffers(Vec<(Buffer, Arc<Probe>, u8)>);
/// The original1.5 CSS-pixel shake is confined to the world camera. The mask
/// follows the same subview afterwards; HUD/UI cameras never receive it.
#[derive(Component, Default)]
pub struct NuclearShake {
    pub offset: Vec2,
}
pub fn apply_shake(mut cameras: Query<(&mut Camera, &NuclearShake)>) {
    // World framing writes an absolute subview every frame before this system.
    for (mut camera, shake) in &mut cameras {
        if let Some(sub) = &mut camera.sub_camera_view {
            // Bevy's subview Y grows downward. These signs reproduce the
            // original +2*shake/css displacement of projection columns 8/9.
            sub.offset += Vec2::new(shake.offset.x, -shake.offset.y);
        }
    }
}

pub struct BlastCompositorPlugin;
impl Plugin for BlastCompositorPlugin {
    fn build(&self, app: &mut App) {
        bevy::asset::embedded_asset!(app, "blast_cloud.wesl");
        bevy::asset::embedded_asset!(app, "blast_composite.wesl");
        app.add_plugins(ExtractComponentPlugin::<NuclearView>::default());
        let Some(render) = app.get_sub_app_mut(RenderApp) else {
            return;
        };
        render
            .init_resource::<RetiredBuffers>()
            .add_systems(RenderStartup, initialize)
            .add_systems(Render, prepare.in_set(RenderSystems::PrepareBindGroups))
            .add_systems(
                Core3d,
                encode
                    .in_set(Core3dSystems::PostProcess)
                    .after(bevy::core_pipeline::tonemapping::tonemapping)
                    .before(bevy::anti_alias::fxaa::fxaa),
            );
    }
}
#[derive(Clone, Copy, ShaderType)]
struct Uniform {
    inverse_projection: Mat4,
    camera_world: Mat4,
    world_to_local: Mat4,
    values: Vec4,
}
#[derive(Resource)]
struct Pipelines {
    cloud_layout: BindGroupLayoutDescriptor,
    composite_layout: BindGroupLayoutDescriptor,
    cloud: [Option<CachedRenderPipelineId>; 2],
    composite: std::collections::HashMap<TextureFormat, CachedRenderPipelineId>,
    composite_descriptor: RenderPipelineDescriptor,
    sampler: Sampler,
}
pub(crate) fn cloud_format(features: TextureFormatFeatures) -> TextureFormat {
    if features
        .allowed_usages
        .contains(TextureUsages::RENDER_ATTACHMENT | TextureUsages::TEXTURE_BINDING)
        && features
            .flags
            .contains(TextureFormatFeatureFlags::FILTERABLE | TextureFormatFeatureFlags::BLENDABLE)
    {
        TextureFormat::Rgba16Float
    } else {
        TextureFormat::Rgba8Unorm
    }
}
fn initialize(
    mut commands: Commands,
    device: Res<RenderDevice>,
    adapter: Res<RenderAdapter>,
    server: Res<AssetServer>,
    cache: Res<PipelineCache>,
) {
    let depth = if cfg!(feature = "webgl2") {
        texture_2d(TextureSampleType::Float { filterable: false })
    } else {
        texture_depth_2d()
    };
    let cloud_layout = BindGroupLayoutDescriptor::new(
        "nuclear volume layout",
        &BindGroupLayoutEntries::sequential(
            ShaderStages::FRAGMENT,
            (uniform_buffer::<Uniform>(true), depth),
        ),
    );
    let composite_layout = BindGroupLayoutDescriptor::new(
        "nuclear composite layout",
        &BindGroupLayoutEntries::sequential(
            ShaderStages::FRAGMENT,
            (
                uniform_buffer::<Uniform>(true),
                depth,
                texture_2d(TextureSampleType::Float { filterable: true }),
                sampler(SamplerBindingType::Filtering),
                texture_2d(TextureSampleType::Float { filterable: true }),
            ),
        ),
    );
    let cloud_shader = server.load("embedded://openmars_engine/nuclear/blast_cloud.wesl");
    let composite_shader = server.load("embedded://openmars_engine/nuclear/blast_composite.wesl");
    let defs = if cfg!(feature = "webgl2") {
        vec![bevy::shader::ShaderDefVal::Bool("WEBGL_DEPTH".into(), true)]
    } else {
        vec![]
    };
    let descriptor = |shader: Handle<bevy::shader::Shader>,
                      layout: &BindGroupLayoutDescriptor,
                      format,
                      blend| RenderPipelineDescriptor {
        label: Some("Open Mars nuclear compositor".into()),
        layout: vec![layout.clone()],
        vertex: VertexState {
            shader: shader.clone(),
            entry_point: Some("vertex".into()),
            shader_defs: defs.clone(),
            ..default()
        },
        fragment: Some(FragmentState {
            shader,
            entry_point: Some("fragment".into()),
            shader_defs: defs.clone(),
            targets: vec![Some(ColorTargetState {
                format,
                blend,
                write_mask: ColorWrites::ALL,
            })],
            ..default()
        }),
        ..default()
    };
    let float_supported =
        cloud_format(adapter.get_texture_format_features(TextureFormat::Rgba16Float))
            == TextureFormat::Rgba16Float;
    let cloud = [TextureFormat::Rgba16Float, TextureFormat::Rgba8Unorm].map(|format| {
        (format != TextureFormat::Rgba16Float || float_supported).then(|| {
            cache.queue_render_pipeline(descriptor(
                cloud_shader.clone(),
                &cloud_layout,
                format,
                Some(BlendState::PREMULTIPLIED_ALPHA_BLENDING),
            ))
        })
    });
    let composite_descriptor = descriptor(
        composite_shader,
        &composite_layout,
        TextureFormat::Rgba8UnormSrgb,
        None,
    );
    let sampler = device.create_sampler(&SamplerDescriptor {
        mag_filter: FilterMode::Linear,
        min_filter: FilterMode::Linear,
        ..default()
    });
    commands.insert_resource(Pipelines {
        cloud_layout,
        composite_layout,
        cloud,
        composite: default(),
        composite_descriptor,
        sampler,
    });
}
#[derive(Component)]
struct Prepared {
    target: Handle<Image>,
    uniforms: DynamicUniformBuffer<Uniform>,
    draws: Vec<(u32, [u32; 4])>,
    order: Vec<usize>,
    cloud: Option<BindGroup>,
    composite: Option<[(TextureViewId, BindGroup); 2]>,
    identities: Option<(
        BufferId,
        TextureViewId,
        TextureViewId,
        TextureViewId,
        TextureViewId,
    )>,
    extent: [u32; 2],
    pipeline: usize,
    composite_pipeline: Option<CachedRenderPipelineId>,
    probe: Arc<Probe>,
}
impl Prepared {
    fn new(target: Handle<Image>, probe: Arc<Probe>) -> Self {
        Self {
            target,
            uniforms: DynamicUniformBuffer::default(),
            draws: Vec::with_capacity(MAX_BLASTS),
            order: Vec::with_capacity(MAX_BLASTS),
            cloud: None,
            composite: None,
            identities: None,
            extent: [0; 2],
            pipeline: 0,
            composite_pipeline: None,
            probe,
        }
    }
    fn update(
        &mut self,
        frame: &NuclearView,
        view: &ExtractedView,
        depth: &ViewDepthStencilTexture,
        target: &ViewTarget,
        image: &GpuImage,
        gpu: (&Pipelines, &RenderDevice, &RenderQueue, &PipelineCache),
    ) {
        let (pipelines, device, queue, cache) = gpu;
        self.target = frame.target.clone();
        self.extent = frame.extent;
        self.pipeline = usize::from(image.texture_descriptor.format == TextureFormat::Rgba8Unorm);
        let camera_world = view.world_from_view.to_matrix();
        let camera = camera_world.w_axis.truncate();
        let clip_from_world = view.clip_from_view * camera_world.inverse();
        let inverse_projection = view.clip_from_view.inverse();
        self.order.clear();
        self.order.extend((0..frame.volumes.len()).filter(|&i| {
            geometry::visible(
                frame.world_from_board * frame.volumes[i].local,
                clip_from_world,
                camera,
            )
        }));
        self.order.sort_by(|&a, &b| {
            let distance = |i: usize| {
                (frame.world_from_board * frame.volumes[i].local)
                    .w_axis
                    .truncate()
                    .distance_squared(camera)
            };
            distance(b).total_cmp(&distance(a)).then(a.cmp(&b))
        });
        if self.uniforms.buffer().is_none() {
            self.uniforms
                .set_label(Some("nuclear compositor parameters"));
            let zero = Uniform {
                inverse_projection: Mat4::IDENTITY,
                camera_world: Mat4::IDENTITY,
                world_to_local: Mat4::IDENTITY,
                values: Vec4::ZERO,
            };
            for _ in 0..=MAX_BLASTS {
                self.uniforms.push(&zero);
            }
            self.uniforms.write_buffer(device, queue);
            frame.probe.allocations.fetch_add(1, Ordering::AcqRel);
        }
        self.uniforms.clear();
        self.draws.clear();
        self.uniforms.push(&Uniform {
            inverse_projection,
            camera_world,
            world_to_local: Mat4::IDENTITY,
            values: Vec4::new(0., 0., frame.extent[0] as f32, frame.extent[1] as f32),
        });
        for &i in &self.order {
            let volume = frame.volumes[i];
            let model = frame.world_from_board * volume.local;
            let Some(rect) = geometry::scissor(clip_from_world * model, frame.extent) else {
                continue;
            };
            let offset = self.uniforms.push(&Uniform {
                inverse_projection,
                camera_world,
                world_to_local: model.inverse(),
                values: Vec4::new(volume.age, volume.seed as f32, 0., 0.),
            });
            self.draws.push((offset, rect));
        }
        self.uniforms.write_buffer(device, queue);
        let buffer = self.uniforms.buffer().unwrap();
        frame.probe.bytes.store(buffer.size(), Ordering::Release);
        *frame.probe.buffer.lock().unwrap() = format!("{:?}", buffer.id());
        let binding = self.uniforms.binding().expect("nonempty nuclear uniforms");
        let depth_view = &depth.attachment.texture.default_view;
        let a = target.main_texture_view();
        let b = target.main_texture_other_view();
        let identity = (
            self.uniforms.buffer().unwrap().id(),
            depth_view.id(),
            image.texture_view.id(),
            a.id(),
            b.id(),
        );
        if self.identities == Some(identity) {
            return;
        }
        self.cloud = Some(device.create_bind_group(
            "nuclear volume",
            &cache.get_bind_group_layout(&pipelines.cloud_layout),
            &BindGroupEntries::sequential((binding.clone(), depth_view)),
        ));
        self.composite = Some([a, b].map(|texture| {
            (
                texture.id(),
                device.create_bind_group(
                    "nuclear composite",
                    &cache.get_bind_group_layout(&pipelines.composite_layout),
                    &BindGroupEntries::sequential((
                        binding.clone(),
                        depth_view,
                        &image.texture_view,
                        &pipelines.sampler,
                        texture,
                    )),
                ),
            )
        }));
        self.identities = Some(identity);
    }
}
type Views<'w, 's> = Query<
    'w,
    's,
    (
        Entity,
        &'static NuclearView,
        &'static ExtractedView,
        &'static ViewDepthStencilTexture,
        &'static ViewTarget,
        Option<&'static mut Prepared>,
    ),
>;
type Gpu<'w> = (
    Option<ResMut<'w, Pipelines>>,
    Res<'w, RenderAssets<GpuImage>>,
    Res<'w, RenderDevice>,
    Res<'w, RenderQueue>,
    Res<'w, PipelineCache>,
);
fn prepare(
    mut commands: Commands,
    mut views: Views,
    abandoned: Query<(Entity, &Prepared), Without<NuclearView>>,
    mut retired: ResMut<RetiredBuffers>,
    gpu: Gpu,
) {
    let (pipelines, images, device, queue, cache) = gpu;
    for (buffer, probe, frames) in &mut retired.0 {
        *frames = frames.saturating_sub(1);
        if *frames == 0 {
            buffer.destroy();
            probe.retirements.fetch_add(1, Ordering::AcqRel);
        }
    }
    retired.0.retain(|(_, _, frames)| *frames > 0);
    for (entity, prepared) in &abandoned {
        if let Some(buffer) = prepared.uniforms.buffer() {
            retired.0.push((buffer.clone(), prepared.probe.clone(), 2));
        }
        commands.entity(entity).remove::<Prepared>();
    }
    let Some(mut pipelines) = pipelines else {
        return;
    };
    for (entity, frame, view, depth, target, old) in &mut views {
        if frame.volumes.is_empty() || frame.volumes.len() > MAX_BLASTS {
            continue;
        }
        let Some(image) = images.get(frame.target.id()) else {
            continue;
        };
        let format = target.main_texture_format();
        let composite = if let Some(id) = pipelines.composite.get(&format) {
            *id
        } else {
            let mut descriptor = pipelines.composite_descriptor.clone();
            descriptor.fragment.as_mut().unwrap().targets[0]
                .as_mut()
                .unwrap()
                .format = format;
            let id = cache.queue_render_pipeline(descriptor);
            pipelines.composite.insert(format, id);
            id
        };
        if let Some(mut prepared) = old {
            prepared.composite_pipeline = Some(composite);
            prepared.update(
                frame,
                view,
                depth,
                target,
                image,
                (&pipelines, &device, &queue, &cache),
            );
        } else {
            let mut prepared = Prepared::new(frame.target.clone(), frame.probe.clone());
            prepared.composite_pipeline = Some(composite);
            prepared.update(
                frame,
                view,
                depth,
                target,
                image,
                (&pipelines, &device, &queue, &cache),
            );
            commands.entity(entity).insert(prepared);
        }
    }
}
fn encode(
    view: ViewQuery<(&NuclearView, &ViewTarget, &Prepared)>,
    pipelines: Option<Res<Pipelines>>,
    cache: Res<PipelineCache>,
    images: Res<RenderAssets<GpuImage>>,
    mut context: RenderContext,
) {
    let Some(pipelines) = pipelines else {
        return;
    };
    let (frame, target, prepared) = view.into_inner();
    // A resized image may still be uploading. Never acknowledge the new
    // generation using the previous target or its old bind groups.
    if prepared.target != frame.target || prepared.extent != frame.extent {
        return;
    }
    let (
        Some(volume_pipeline),
        Some(composite_pipeline),
        Some(volume_group),
        Some(groups),
        Some(image),
    ) = (
        pipelines.cloud[prepared.pipeline].and_then(|id| cache.get_render_pipeline(id)),
        prepared
            .composite_pipeline
            .and_then(|id| cache.get_render_pipeline(id)),
        prepared.cloud.as_ref(),
        prepared.composite.as_ref(),
        images.get(prepared.target.id()),
    )
    else {
        return;
    };
    {
        let mut pass = context
            .command_encoder()
            .begin_render_pass(&RenderPassDescriptor {
                label: Some("Open Mars nuclear volumes"),
                color_attachments: &[Some(RenderPassColorAttachment {
                    view: &image.texture_view,
                    depth_slice: None,
                    resolve_target: None,
                    ops: Operations {
                        load: LoadOp::Clear(bevy::color::LinearRgba::NONE.into()),
                        store: StoreOp::Store,
                    },
                })],
                depth_stencil_attachment: None,
                timestamp_writes: None,
                occlusion_query_set: None,
                multiview_mask: None,
            });
        pass.set_pipeline(volume_pipeline);
        for &(offset, [x, y, w, h]) in &prepared.draws {
            pass.set_scissor_rect(x, y, w, h);
            pass.set_bind_group(0, volume_group, &[offset]);
            pass.draw(0..3, 0..1);
        }
    }
    // Do not flip the scene's target until EVERY dependency is present. From
    // this point onward the destination is always written, including no draws.
    let output = target.post_process_write();
    let group = if groups[0].0 == output.source.id() {
        &groups[0].1
    } else {
        &groups[1].1
    };
    {
        let mut pass = context
            .command_encoder()
            .begin_render_pass(&RenderPassDescriptor {
                label: Some("Open Mars depth-aware nuclear composite"),
                color_attachments: &[Some(RenderPassColorAttachment {
                    view: output.destination,
                    depth_slice: None,
                    resolve_target: None,
                    ops: Operations {
                        load: LoadOp::Clear(default()),
                        store: StoreOp::Store,
                    },
                })],
                depth_stencil_attachment: None,
                timestamp_writes: None,
                occlusion_query_set: None,
                multiview_mask: None,
            });
        pass.set_pipeline(composite_pipeline);
        pass.set_bind_group(0, group, &[0]);
        pass.draw(0..3, 0..1);
    }
    // Encoded is a pipeline readiness observation, not GPU completion or an
    // actual inspected frame. The image's typed retirement owns queue cleanup.
    frame.encoded.store(true, Ordering::Release);
    frame.probe.frames.fetch_add(1, Ordering::AcqRel);
    frame
        .probe
        .draws
        .store(prepared.draws.len() as u64, Ordering::Release);
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn shake_follows_absolute_world_framing_without_drift() {
        use bevy::ecs::system::RunSystemOnce;
        let mut world = World::new();
        world.init_resource::<crate::assets::WorldAssets>();
        world.init_resource::<crate::menu::MenuRuntime>();
        world.init_resource::<crate::scene::MotionPreference>();
        world.insert_resource(crate::world::WorldView {
            size: Vec2::new(800., 600.),
            physical: UVec2::new(1600, 1200),
            ..default()
        });
        let camera = world
            .spawn((
                crate::assets::AssetCamera,
                Camera::default(),
                Projection::default(),
                Transform::IDENTITY,
                NuclearShake {
                    offset: Vec2::new(2., -1.),
                },
            ))
            .id();
        world.run_system_once(crate::world::camera).unwrap();
        let base = world
            .get::<Camera>(camera)
            .unwrap()
            .sub_camera_view
            .unwrap()
            .offset;
        let projection = world.get::<Projection>(camera).unwrap();
        let base_clip = projection.get_clip_from_view_for_sub(
            &world
                .get::<Camera>(camera)
                .unwrap()
                .sub_camera_view
                .unwrap(),
        );
        for _ in 0..10 {
            world.run_system_once(crate::world::camera).unwrap();
            world.run_system_once(apply_shake).unwrap();
            assert_eq!(
                world
                    .get::<Camera>(camera)
                    .unwrap()
                    .sub_camera_view
                    .unwrap()
                    .offset,
                base + Vec2::new(2., 1.)
            );
            let clip = world
                .get::<Projection>(camera)
                .unwrap()
                .get_clip_from_view_for_sub(
                    &world
                        .get::<Camera>(camera)
                        .unwrap()
                        .sub_camera_view
                        .unwrap(),
                );
            assert!((clip.z_axis.x - base_clip.z_axis.x - 4. / 1600.).abs() < 1e-6);
            assert!((clip.z_axis.y - base_clip.z_axis.y + 2. / 1200.).abs() < 1e-6);
        }
    }
    #[test]
    fn unsupported_float_targets_never_select_a_float_pipeline() {
        let mut features = TextureFormatFeatures {
            allowed_usages: TextureUsages::RENDER_ATTACHMENT | TextureUsages::TEXTURE_BINDING,
            flags: TextureFormatFeatureFlags::FILTERABLE | TextureFormatFeatureFlags::BLENDABLE,
        };
        assert_eq!(cloud_format(features), TextureFormat::Rgba16Float);
        features.flags.remove(TextureFormatFeatureFlags::BLENDABLE);
        assert_eq!(cloud_format(features), TextureFormat::Rgba8Unorm);
        features.flags.insert(TextureFormatFeatureFlags::BLENDABLE);
        features.flags.remove(TextureFormatFeatureFlags::FILTERABLE);
        assert_eq!(cloud_format(features), TextureFormat::Rgba8Unorm);
        features.flags.insert(TextureFormatFeatureFlags::FILTERABLE);
        features
            .allowed_usages
            .remove(TextureUsages::RENDER_ATTACHMENT);
        assert_eq!(cloud_format(features), TextureFormat::Rgba8Unorm);
    }
}
