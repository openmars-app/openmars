//! Authored contact/cast blobs. Double-buffered maps publish after their draw is
//! submitted; consumers follow it on the same ordered GPU queue.
use bevy::{
    asset::RenderAssetUsages,
    image::{ImageFilterMode, ImageSampler, ImageSamplerDescriptor},
    prelude::*,
    render::{
        RenderApp,
        render_asset::RenderAssets,
        render_resource::{Buffer, Extent3d, TextureDimension, TextureFormat, TextureUsages},
        renderer::{RenderDevice, RenderQueue},
        texture::GpuImage,
    },
};
const CAPACITY: usize = 65536;
const RESOLUTION: u32 = 2048;
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Blob {
    pub point: Vec2,
    pub radius: f32,
    pub length: f32,
    pub strength: f32,
}
struct Layout {
    bounds: Vec4,
    data: Vec<u8>,
    count: u32,
}
fn layout(blobs: &[Blob], sun: Vec3) -> Result<Layout, String> {
    if !sun.is_finite() {
        return Err("Nonfinite shade sun".into());
    }
    let across = sun.xy().length();
    let direction = if across > 1e-4 {
        -sun.xy() / across
    } else {
        Vec2::X
    };
    let slope = (across / sun.z.abs().max(0.05)).min(2.);
    let mut shapes = Vec::with_capacity(blobs.len().min(CAPACITY / 2) * 2);
    let (mut low, mut high) = (Vec2::splat(f32::INFINITY), Vec2::splat(f32::NEG_INFINITY));
    for blob in blobs.iter().take(CAPACITY / 2) {
        if !blob.point.is_finite()
            || !blob.radius.is_finite()
            || blob.radius < 0.
            || !blob.length.is_finite()
            || blob.length < 0.
            || !blob.strength.is_finite()
            || !(0. ..=1.).contains(&blob.strength)
        {
            return Err("Invalid plant shade blob".into());
        }
        let reach = blob.length * slope + blob.radius;
        let center = blob.point + direction * reach * 0.5;
        shapes.push([
            blob.point.x,
            blob.point.y,
            1.,
            0.,
            blob.radius * 2.,
            blob.radius * 2.,
            blob.strength,
            0.,
        ]);
        shapes.push([
            center.x,
            center.y,
            direction.x,
            direction.y,
            reach + blob.radius,
            blob.radius * 1.7,
            blob.strength * 0.85,
            0.75,
        ]);
        let extent = Vec2::splat(reach * 0.5 + blob.radius);
        low = low
            .min(blob.point - Vec2::splat(blob.radius))
            .min(center - extent);
        high = high
            .max(blob.point + Vec2::splat(blob.radius))
            .max(center + extent);
    }
    let bounds = if shapes.is_empty() {
        Vec4::new(0., 0., 1., 1.)
    } else {
        let size = (high - low).max_element() + 0.1;
        Vec4::new(low.x - 0.05, low.y - 0.05, size, size)
    };
    let count = shapes.len() as u32;
    let mut data = Vec::with_capacity(shapes.len() * 32);
    for mut shape in shapes {
        shape[0] = (shape[0] - bounds.x) / bounds.z;
        shape[1] = (shape[1] - bounds.y) / bounds.w;
        shape[4] /= bounds.z;
        shape[5] /= bounds.w;
        data.extend(shape.into_iter().flat_map(f32::to_le_bytes));
    }
    Ok(Layout {
        bounds,
        data,
        count,
    })
}
struct Pipeline {
    pipeline: bevy::render::render_resource::RenderPipeline,
}
impl Pipeline {
    fn new(device: &RenderDevice) -> Self {
        let d = device.wgpu_device();
        let shader = d.create_shader_module(wgpu::ShaderModuleDescriptor {
            label: Some("plant shade"),
            source: wgpu::ShaderSource::Wgsl(include_str!("shaders/shade.wgsl").into()),
        });
        let layout = d.create_pipeline_layout(&wgpu::PipelineLayoutDescriptor {
            label: Some("plant shade"),
            bind_group_layouts: &[],
            ..default()
        });
        let attributes = wgpu::vertex_attr_array![0=>Float32x4,1=>Float32x4];
        let max = wgpu::BlendComponent {
            src_factor: wgpu::BlendFactor::One,
            dst_factor: wgpu::BlendFactor::One,
            operation: wgpu::BlendOperation::Max,
        };
        Self {
            pipeline: d
                .create_render_pipeline(&wgpu::RenderPipelineDescriptor {
                    label: Some("plant shade"),
                    layout: Some(&layout),
                    vertex: wgpu::VertexState {
                        module: &shader,
                        entry_point: Some("vertex"),
                        buffers: &[Some(wgpu::VertexBufferLayout {
                            array_stride: 32,
                            step_mode: wgpu::VertexStepMode::Instance,
                            attributes: &attributes,
                        })],
                        compilation_options: default(),
                    },
                    fragment: Some(wgpu::FragmentState {
                        module: &shader,
                        entry_point: Some("fragment"),
                        compilation_options: default(),
                        targets: &[Some(wgpu::ColorTargetState {
                            format: wgpu::TextureFormat::R8Unorm,
                            blend: Some(wgpu::BlendState {
                                color: max,
                                alpha: max,
                            }),
                            write_mask: wgpu::ColorWrites::RED,
                        })],
                    }),
                    primitive: wgpu::PrimitiveState {
                        cull_mode: None,
                        ..default()
                    },
                    depth_stencil: None,
                    multisample: default(),
                    multiview_mask: None,
                    cache: None,
                })
                .into(),
        }
    }
}
struct Published {
    index: usize,
    bounds: Vec4,
    blobs: Vec<Blob>,
}
#[derive(Default)]
pub struct Shade {
    pipeline: Option<Pipeline>,
    buffer: Option<Buffer>,
    targets: Option<[Handle<Image>; 2]>,
    active: Option<Published>,
    pending: Option<Published>,
    pub revision: u64,
    pub draws: u64,
    pub shapes: u32,
}
impl Shade {
    pub fn published(&self) -> Option<(Handle<Image>, Vec4, u64)> {
        self.active.as_ref().map(|v| {
            (
                self.targets.as_ref().unwrap()[v.index].clone(),
                v.bounds,
                self.revision,
            )
        })
    }
    pub fn buffers(&self) -> usize {
        usize::from(self.buffer.is_some())
    }
    pub fn retire(&mut self) -> Option<Buffer> {
        self.buffer.take()
    }
    pub fn advance_queued(
        &mut self,
        app: &mut App,
        blobs: Option<Vec<Blob>>,
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<bool, String> {
        if let Some(pending) = self.pending.take() {
            self.active = Some(pending);
            self.revision += 1;
        }
        let Some(blobs) = blobs else {
            return Ok(self.active.is_some());
        };
        if self.active.as_ref().is_some_and(|v| v.blobs == blobs) {
            return Ok(true);
        }
        let packed = layout(&blobs, crate::lighting::sun_direction())?;
        if self.targets.is_none() {
            let mut images = app.world_mut().resource_mut::<Assets<Image>>();
            self.targets = Some(std::array::from_fn(|_| {
                let mut image = Image::new_uninit(
                    Extent3d {
                        width: RESOLUTION,
                        height: RESOLUTION,
                        depth_or_array_layers: 1,
                    },
                    TextureDimension::D2,
                    TextureFormat::R8Unorm,
                    RenderAssetUsages::MAIN_WORLD | RenderAssetUsages::RENDER_WORLD,
                );
                image.data = None;
                image.texture_descriptor.usage =
                    TextureUsages::TEXTURE_BINDING | TextureUsages::RENDER_ATTACHMENT;
                image.sampler = ImageSampler::Descriptor(ImageSamplerDescriptor {
                    mag_filter: ImageFilterMode::Linear,
                    min_filter: ImageFilterMode::Linear,
                    ..default()
                });
                images.add(image)
            }));
        }
        let index = self.active.as_ref().map_or(0, |v| 1 - v.index);
        let images = app
            .sub_app(RenderApp)
            .world()
            .resource::<RenderAssets<GpuImage>>();
        let Some(image) = images.get(self.targets.as_ref().unwrap()[index].id()) else {
            return Ok(false);
        };
        let pipeline = self.pipeline.get_or_insert_with(|| Pipeline::new(device));
        let buffer = self.buffer.get_or_insert_with(|| {
            device.create_buffer(&wgpu::BufferDescriptor {
                label: Some("plant shade instances"),
                size: (CAPACITY * 32) as u64,
                usage: wgpu::BufferUsages::VERTEX | wgpu::BufferUsages::COPY_DST,
                mapped_at_creation: false,
            })
        });
        if !packed.data.is_empty() {
            queue.write_buffer(buffer, 0, &packed.data)
        }
        let mut encoder = device.create_command_encoder(&wgpu::CommandEncoderDescriptor {
            label: Some("plant contact and cast shade"),
        });
        {
            let mut pass = encoder.begin_render_pass(&wgpu::RenderPassDescriptor {
                label: Some("plant contact and cast shade"),
                color_attachments: &[Some(wgpu::RenderPassColorAttachment {
                    view: &image.texture_view,
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
            pass.set_pipeline(&pipeline.pipeline);
            pass.set_vertex_buffer(0, *buffer.slice(..));
            pass.draw(0..6, 0..packed.count);
        }
        queue.submit([encoder.finish()]);
        self.draws += 1;
        self.shapes = packed.count;
        self.pending = Some(Published {
            index,
            bounds: packed.bounds,
            blobs,
        });
        Ok(false)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn contact_cast_layout_preserves_reference_footprints_and_bounds() {
        let blob = Blob {
            point: Vec2::new(2., 3.),
            radius: 0.03,
            length: 0.08,
            strength: 0.8,
        };
        let packed = layout(&[blob], Vec3::new(-4., 5., 8.)).unwrap();
        assert_eq!(packed.count, 2);
        assert_eq!(packed.data.len(), 64);
        let values: Vec<f32> = packed
            .data
            .as_chunks::<4>()
            .0
            .iter()
            .map(|v| f32::from_le_bytes(*v))
            .collect();
        assert!((values[0] * packed.bounds.z + packed.bounds.x - 2.).abs() < 1e-6);
        assert!((values[1] * packed.bounds.w + packed.bounds.y - 3.).abs() < 1e-6);
        assert!((values[4] * packed.bounds.z - 0.06).abs() < 1e-6);
        assert_eq!(values[6], 0.8);
        assert_eq!(values[7], 0.);
        assert!(values[10] > 0. && values[11] < 0.);
        assert!((values[14] - 0.8 * 0.85).abs() < 1e-6);
        assert_eq!(values[15], 0.75);
        assert_eq!(packed.bounds.z, packed.bounds.w);
        let reach = 0.08 * (41f32.sqrt() / 8.) + 0.03;
        assert!((values[12] * packed.bounds.z - (reach + 0.03)).abs() < 1e-6);
    }
    #[test]
    fn bounds_empty_maps_rejects_invalid_data_and_caps_reference_capacity() {
        let packed = layout(&[], Vec3::Z).unwrap();
        assert_eq!(packed.bounds, Vec4::new(0., 0., 1., 1.));
        assert_eq!(packed.count, 0);
        let valid = Blob {
            point: Vec2::ZERO,
            radius: 0.03,
            length: 0.08,
            strength: 0.8,
        };
        let packed = layout(&vec![valid; 40000], Vec3::Z).unwrap();
        assert_eq!(packed.count, CAPACITY as u32);
        assert_eq!(packed.data.len(), CAPACITY * 32);
        assert!(
            layout(
                &[Blob {
                    radius: f32::NAN,
                    ..valid
                }],
                Vec3::Z
            )
            .is_err()
        );
        assert!(
            layout(
                &[Blob {
                    strength: 1.1,
                    ..valid
                }],
                Vec3::Z
            )
            .is_err()
        );
    }
    #[test]
    fn portable_shade_shader_validates_without_optional_gpu_features() {
        let module =
            wgpu::naga::front::wgsl::parse_str(include_str!("shaders/shade.wgsl")).unwrap();
        wgpu::naga::valid::Validator::new(
            wgpu::naga::valid::ValidationFlags::all(),
            wgpu::naga::valid::Capabilities::empty(),
        )
        .validate(&module)
        .unwrap();
    }
}
