//! Bounded real GPU mip generation on the same ordered queue as its consumers.
use bevy::{
    asset::{AssetId, RenderAssetUsages},
    image::{ImageAddressMode, ImageFilterMode, ImageSampler, ImageSamplerDescriptor},
    prelude::*,
    render::{
        render_asset::RenderAssets,
        render_resource::{
            Extent3d, TextureDimension, TextureFormat, TextureUsages, TextureViewDescriptor,
            TextureViewDimension,
        },
        renderer::{RenderDevice, RenderQueue},
        texture::GpuImage,
    },
};
fn error(message: &str) -> String {
    message.into()
}
pub(crate) fn mip_bytes(width: u32, height: u32, layers: u32, levels: u32) -> Result<u64, String> {
    if width == 0
        || height == 0
        || layers == 0
        || levels == 0
        || levels > width.max(height).ilog2() + 1
    {
        return Err(error("invalid mip extent"));
    }
    let mut bytes = 0u64;
    for mip in 0..levels {
        bytes = bytes
            .checked_add(
                u64::from((width >> mip).max(1))
                    * u64::from((height >> mip).max(1))
                    * u64::from(layers)
                    * 4,
            )
            .ok_or_else(|| error("mip byte overflow"))?;
    }
    Ok(bytes)
}
pub(crate) fn target_image(width: u32, height: u32, layers: u32, levels: u32, srgb: bool) -> Image {
    let mut image = Image::new_uninit(
        Extent3d {
            width,
            height,
            depth_or_array_layers: layers,
        },
        TextureDimension::D2,
        if srgb {
            TextureFormat::Rgba8UnormSrgb
        } else {
            TextureFormat::Rgba8Unorm
        },
        RenderAssetUsages::MAIN_WORLD | RenderAssetUsages::RENDER_WORLD,
    );
    image.data = None;
    image.texture_descriptor.mip_level_count = levels;
    image.texture_descriptor.usage |=
        TextureUsages::RENDER_ATTACHMENT | TextureUsages::TEXTURE_BINDING;
    image.texture_view_descriptor = Some(TextureViewDescriptor {
        dimension: Some(if layers == 1 {
            TextureViewDimension::D2
        } else {
            TextureViewDimension::D2Array
        }),
        ..default()
    });
    image.sampler = ImageSampler::Descriptor(ImageSamplerDescriptor {
        address_mode_u: ImageAddressMode::Repeat,
        address_mode_v: ImageAddressMode::Repeat,
        mag_filter: ImageFilterMode::Linear,
        min_filter: ImageFilterMode::Linear,
        mipmap_filter: ImageFilterMode::Linear,
        anisotropy_clamp: 4,
        ..default()
    });
    image
}

struct CopyPipelines {
    layout: bevy::render::render_resource::BindGroupLayout,
    linear: bevy::render::render_resource::RenderPipeline,
    srgb: bevy::render::render_resource::RenderPipeline,
    sampler: bevy::render::render_resource::Sampler,
}
impl CopyPipelines {
    fn new(device: &RenderDevice) -> Self {
        let device = device.wgpu_device();
        let layout: bevy::render::render_resource::BindGroupLayout = device
            .create_bind_group_layout(&wgpu::BindGroupLayoutDescriptor {
                label: Some("terrain GPU copy"),
                entries: &[
                    wgpu::BindGroupLayoutEntry {
                        binding: 0,
                        visibility: wgpu::ShaderStages::FRAGMENT,
                        ty: wgpu::BindingType::Texture {
                            sample_type: wgpu::TextureSampleType::Float { filterable: true },
                            view_dimension: wgpu::TextureViewDimension::D2,
                            multisampled: false,
                        },
                        count: None,
                    },
                    wgpu::BindGroupLayoutEntry {
                        binding: 1,
                        visibility: wgpu::ShaderStages::FRAGMENT,
                        ty: wgpu::BindingType::Sampler(wgpu::SamplerBindingType::Filtering),
                        count: None,
                    },
                ],
            })
            .into();
        let shader = device.create_shader_module(wgpu::ShaderModuleDescriptor {
            label: Some("terrain GPU copy"),
            source: wgpu::ShaderSource::Wgsl(include_str!("shaders/copy.wgsl").into()),
        });
        let pipeline_layout = device.create_pipeline_layout(&wgpu::PipelineLayoutDescriptor {
            label: Some("terrain GPU copy"),
            bind_group_layouts: &[Some(&layout)],
            ..default()
        });
        let pipeline = |format| {
            device.create_render_pipeline(&wgpu::RenderPipelineDescriptor {
                label: Some("terrain GPU copy"),
                layout: Some(&pipeline_layout),
                vertex: wgpu::VertexState {
                    module: &shader,
                    entry_point: Some("vertex"),
                    buffers: &[],
                    compilation_options: default(),
                },
                primitive: default(),
                depth_stencil: None,
                multisample: default(),
                fragment: Some(wgpu::FragmentState {
                    module: &shader,
                    entry_point: Some("fragment"),
                    compilation_options: default(),
                    targets: &[Some(wgpu::ColorTargetState {
                        format,
                        blend: None,
                        write_mask: wgpu::ColorWrites::ALL,
                    })],
                }),
                multiview_mask: None,
                cache: None,
            })
        };
        let linear = pipeline(wgpu::TextureFormat::Rgba8Unorm).into();
        let srgb = pipeline(wgpu::TextureFormat::Rgba8UnormSrgb).into();
        let sampler = device
            .create_sampler(&wgpu::SamplerDescriptor {
                label: Some("terrain GPU copy"),
                address_mode_u: wgpu::AddressMode::Repeat,
                address_mode_v: wgpu::AddressMode::Repeat,
                mag_filter: wgpu::FilterMode::Linear,
                min_filter: wgpu::FilterMode::Linear,
                lod_min_clamp: 0.0,
                lod_max_clamp: 0.0,
                ..default()
            })
            .into();
        Self {
            layout,
            linear,
            srgb,
            sampler,
        }
    }
}
#[derive(Clone, Copy)]
struct CopyPass {
    source: AssetId<Image>,
    target: AssetId<Image>,
    source_mip: u32,
    target_mip: u32,
    srgb: bool,
}
pub struct MipWork {
    passes: Vec<CopyPass>,
    next: usize,
}
impl MipWork {
    pub fn new(
        world: &World,
        bases: &[Handle<Image>; 2],
        targets: &[Handle<Image>; 2],
    ) -> Result<Self, String> {
        Self::for_images(
            world,
            &bases
                .iter()
                .cloned()
                .zip(targets.iter().cloned())
                .collect::<Vec<_>>(),
        )
    }
    pub fn for_images(
        world: &World,
        pairs: &[(Handle<Image>, Handle<Image>)],
    ) -> Result<Self, String> {
        if pairs.is_empty() || pairs.len() > 2 {
            return Err("Mip image-pair admission exceeded".into());
        }
        let images = world.resource::<Assets<Image>>();
        let mut passes = Vec::new();
        for (base, target) in pairs {
            let b = &images
                .get(base)
                .ok_or("Missing bake base")?
                .texture_descriptor;
            let t = &images
                .get(target)
                .ok_or("Missing bake target")?
                .texture_descriptor;
            if base.id() == target.id()
                || b.size != t.size
                || b.format != t.format
                || b.mip_level_count != 1
                || t.mip_level_count < 2
                || t.size.depth_or_array_layers != 1
                || !matches!(
                    t.format,
                    TextureFormat::Rgba8Unorm | TextureFormat::Rgba8UnormSrgb
                )
                || !t
                    .usage
                    .contains(TextureUsages::RENDER_ATTACHMENT | TextureUsages::TEXTURE_BINDING)
            {
                return Err(error("Bake mip descriptor mismatch"));
            }
            passes.push(CopyPass {
                source: base.id(),
                target: target.id(),
                source_mip: 0,
                target_mip: 0,
                srgb: t.format.is_srgb(),
            });
            for mip in 1..t.mip_level_count {
                passes.push(CopyPass {
                    source: target.id(),
                    target: target.id(),
                    source_mip: mip - 1,
                    target_mip: mip,
                    srgb: t.format.is_srgb(),
                });
            }
        }
        if passes.len() > 26 {
            return Err(error("Bake mip pass bound exceeded"));
        }
        Ok(Self { passes, next: 0 })
    }
    pub fn progress(&self) -> (usize, usize) {
        (self.next, self.passes.len())
    }
}
#[derive(Resource, Default)]
pub struct MipRenderer {
    pipelines: Option<CopyPipelines>,
    pub draws: u64,
}
impl MipRenderer {
    /// A true result is returned on the admitted frame AFTER the last copy
    /// submission. Subsequent texture users execute after these copies on the
    /// same queue; this does not claim CPU-observed GPU completion.
    pub fn advance_queued(
        &mut self,
        work: &mut MipWork,
        images: &RenderAssets<GpuImage>,
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<bool, String> {
        if work.next == work.passes.len() {
            return Ok(true);
        }
        let end = (work.next + 12).min(work.passes.len());
        if work.passes[work.next..end]
            .iter()
            .any(|p| images.get(p.source).is_none() || images.get(p.target).is_none())
        {
            return Ok(false);
        }
        let p = self
            .pipelines
            .get_or_insert_with(|| CopyPipelines::new(device));
        let mut encoder = device.create_command_encoder(&wgpu::CommandEncoderDescriptor {
            label: Some("terrain bounded bake mips"),
        });
        for pass in &work.passes[work.next..end] {
            let source = images.get(pass.source).unwrap();
            let target = images.get(pass.target).unwrap();
            let s = &source.texture_descriptor;
            let t = &target.texture_descriptor;
            if pass.source_mip >= s.mip_level_count
                || pass.target_mip >= t.mip_level_count
                || t.format.is_srgb() != pass.srgb
            {
                return Err(error("Bake GPU mip descriptor mismatch"));
            }
            let view = |image: &GpuImage, mip| {
                image.texture.create_view(&TextureViewDescriptor {
                    dimension: Some(TextureViewDimension::D2),
                    base_mip_level: mip,
                    mip_level_count: Some(1),
                    base_array_layer: 0,
                    array_layer_count: Some(1),
                    ..default()
                })
            };
            let source_view = view(source, pass.source_mip);
            let target_view = view(target, pass.target_mip);
            let bind = device
                .wgpu_device()
                .create_bind_group(&wgpu::BindGroupDescriptor {
                    label: Some("terrain bake mip"),
                    layout: &p.layout,
                    entries: &[
                        wgpu::BindGroupEntry {
                            binding: 0,
                            resource: wgpu::BindingResource::TextureView(&source_view),
                        },
                        wgpu::BindGroupEntry {
                            binding: 1,
                            resource: wgpu::BindingResource::Sampler(&p.sampler),
                        },
                    ],
                });
            {
                let mut draw = encoder.begin_render_pass(&wgpu::RenderPassDescriptor {
                    label: Some("terrain bake mip"),
                    color_attachments: &[Some(wgpu::RenderPassColorAttachment {
                        view: &target_view,
                        depth_slice: None,
                        resolve_target: None,
                        ops: wgpu::Operations {
                            load: wgpu::LoadOp::Clear(wgpu::Color::TRANSPARENT),
                            store: wgpu::StoreOp::Store,
                        },
                    })],
                    depth_stencil_attachment: None,
                    timestamp_writes: None,
                    occlusion_query_set: None,
                    multiview_mask: None,
                });
                draw.set_pipeline(if pass.srgb { &p.srgb } else { &p.linear });
                draw.set_bind_group(0, &bind, &[]);
                draw.draw(0..3, 0..1);
            }
        }
        queue.submit([encoder.finish()]);
        self.draws += (end - work.next) as u64;
        work.next = end;
        Ok(false)
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn allocated_outputs_cannot_publish_before_real_mip_submissions() {
        let mut world = World::new();
        world.insert_resource(Assets::<Image>::default());
        let mut images = world.resource_mut::<Assets<Image>>();
        let bases = [
            images.add(target_image(256, 128, 1, 1, true)),
            images.add(target_image(256, 128, 1, 1, false)),
        ];
        let targets = [
            images.add(target_image(256, 128, 1, 9, true)),
            images.add(target_image(256, 128, 1, 9, false)),
        ];
        assert_eq!(
            MipWork::new(&world, &bases, &targets).unwrap().progress(),
            (0, 18)
        );
        assert!(MipWork::new(&world, &targets, &targets).is_err());
        assert!(MipWork::new(&world, &[bases[1].clone(), bases[0].clone()], &targets).is_err());
    }
    #[test]
    fn actual_copy_shader_validates_without_a_gpu() {
        let module = wgpu::naga::front::wgsl::parse_str(include_str!("shaders/copy.wgsl")).unwrap();
        wgpu::naga::valid::Validator::new(
            wgpu::naga::valid::ValidationFlags::all(),
            wgpu::naga::valid::Capabilities::empty(),
        )
        .validate(&module)
        .unwrap();
        assert_eq!(module.entry_points.len(), 2);
        assert_eq!(mip_bytes(256, 128, 1, 9).unwrap(), 174764);
    }
}
