//! Compile every special-surface pass before gameplay, using the production
//! vertex layouts. Temporary draws have no plot, seed selection or birth clock.
use super::*;
use bevy::asset::UntypedAssetId;

struct Warm {
    root: Entity,
    cameras: Vec<Entity>,
    meshes: Vec<Handle<Mesh>>,
    surfaces: Vec<Handle<SpecialMaterial>>,
    accessories: Vec<Handle<AccessoryMaterial>>,
    flow: Handle<Image>,
    buffers: Vec<Buffer>,
    frames: u8,
}
struct Retiring {
    meshes: Vec<AssetId<Mesh>>,
    materials: Vec<UntypedAssetId>,
    flow: AssetId<Image>,
    buffers: Vec<Buffer>,
    frames: u8,
}
#[derive(Default)]
pub(super) struct Library {
    warm: Option<Warm>,
    retiring: Option<Retiring>,
    pub(super) ready: bool,
}
impl Warm {
    fn remove(self, world: &mut World) -> Retiring {
        world.despawn(self.root);
        for camera in self.cameras {
            world.despawn(camera);
        }
        let meshes = self.meshes.iter().map(Handle::id).collect::<Vec<_>>();
        for id in &meshes {
            world.resource_mut::<Assets<Mesh>>().remove(*id);
        }
        let mut materials = Vec::new();
        for handle in self.surfaces {
            materials.push(handle.id().untyped());
            world
                .resource_mut::<Assets<SpecialMaterial>>()
                .remove(handle.id());
        }
        for handle in self.accessories {
            materials.push(handle.id().untyped());
            world
                .resource_mut::<Assets<AccessoryMaterial>>()
                .remove(handle.id());
        }
        world.resource_mut::<Assets<Image>>().remove(self.flow.id());
        Retiring {
            meshes,
            materials,
            flow: self.flow.id(),
            buffers: self.buffers,
            frames: 2,
        }
    }
}
impl Library {
    pub(super) fn gpu_state(&self, render: &World) -> (bool, bool) {
        let (Some(meshes), Some(materials), Some(images)) = (
            render.get_resource::<RenderAssets<RenderMesh>>(),
            render.get_resource::<ErasedRenderAssets<PreparedMaterial>>(),
            render.get_resource::<RenderAssets<bevy::render::texture::GpuImage>>(),
        ) else {
            return (false, false);
        };
        let uploaded = self.warm.as_ref().is_some_and(|w| {
            render
                .resource::<PipelineCache>()
                .waiting_pipelines()
                .next()
                .is_none()
                && w.meshes.iter().all(|h| meshes.get(h.id()).is_some())
                && w.surfaces
                    .iter()
                    .all(|h| materials.get(h.id().untyped()).is_some())
                && w.accessories
                    .iter()
                    .all(|h| materials.get(h.id().untyped()).is_some())
                && images.get(w.flow.id()).is_some()
        });
        // Only our temporary assets belong to this barrier. Live plot replacement
        // and unrelated shader work must never prevent their own activation.
        let removed = self.retiring.as_ref().is_some_and(|r| {
            r.meshes.iter().all(|id| meshes.get(*id).is_none())
                && r.materials.iter().all(|id| materials.get(*id).is_none())
                && images.get(r.flow).is_none()
        });
        (uploaded, removed)
    }
    fn finish_retirement(&mut self, removed: bool) -> bool {
        let Some(retiring) = self.retiring.as_mut() else {
            return self.ready;
        };
        retiring.frames = retiring.frames.saturating_sub(1);
        if retiring.frames == 0 && removed {
            for buffer in self.retiring.take().unwrap().buffers {
                buffer.destroy();
            }
            self.ready = true;
        }
        self.ready
    }
    pub(super) fn cancel(&mut self, world: &mut World) -> Vec<Buffer> {
        if let Some(warm) = self.warm.take() {
            self.retiring = Some(warm.remove(world));
        }
        let buffers = self.retiring.take().map_or_else(Vec::new, |r| r.buffers);
        self.ready = false;
        buffers
    }
    pub(super) fn prepare(
        &mut self,
        world: &mut World,
        input: &Input,
        state: (bool, bool),
        fence: &fence::Fence,
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<bool, String> {
        if self.ready {
            return Ok(true);
        }
        if self.retiring.is_some() {
            return Ok(self.finish_retirement(state.1));
        }
        if let Some(warm) = &mut self.warm {
            warm.frames = warm.frames.saturating_add(1);
            if state.0 && warm.frames >= 2 {
                self.retiring = Some(self.warm.take().unwrap().remove(world));
            }
            return Ok(false);
        }
        let Some(colors) = crate::assets::texture(world, "textures/terrain/color-array") else {
            return Ok(false);
        };
        let library = world.resource::<super::super::texture_library::TextureLibrary>();
        let Some((mid, high)) = library
            .get(super::super::textures::Slot::Shared(3))
            .zip(library.get(super::super::textures::Slot::Shared(4)))
        else {
            return Ok(false);
        };
        let atmosphere = world
            .resource::<crate::atmosphere::AtmosphereRuntime>()
            .binding()?;
        let data = world.resource::<crate::terrain::grid::GridRuntime>().data();
        let radius = input.snapshot.projection().content_radius() as f32;
        let mut parameters = UniformBuffer::from(AccessoryUniform {
            base: material::SpecialUniform {
                geometry: Vec4::new(radius, 0., 0., 1.),
                seeds: [Vec4::ONE; 4],
                profile: Vec4::new(0.06, 0.4, 0., 0.14),
                clock: Vec4::new(0., 1., 1., 1.),
                climate: Vec4::ZERO,
                sun: Vec4::new(0., 0., 1., 0.65),
                sun_color: Vec4::ONE,
                local_x: Vec4::X,
                local_y: Vec4::Y,
                origin: Vec4::new(0., 0., radius + 0.01, 1.),
                motion: Vec4::ZERO,
                collapse_parts: [Vec4::ZERO; 5],
            },
            stages: Vec4::ONE,
            fence_surface: fence.surface,
        });
        parameters.set_label(Some("special surface preparation parameters"));
        parameters.write_buffer(device, queue);
        let mut highlights = UniformBuffer::from(
            world
                .resource::<crate::terrain::grid::GridRuntime>()
                .uniform(),
        );
        highlights.set_label(Some("special surface preparation highlights"));
        highlights.write_buffer(device, queue);
        let parameter_buffer = parameters.buffer().unwrap().clone();
        let highlight_buffer = highlights.buffer().unwrap().clone();
        let flow = world.resource_mut::<Assets<Image>>().add(Image::new(
            Extent3d {
                width: 1,
                height: 1,
                depth_or_array_layers: 1,
            },
            TextureDimension::D2,
            0_f32.to_le_bytes().to_vec(),
            TextureFormat::R32Float,
            bevy::asset::RenderAssetUsages::RENDER_WORLD,
        ));
        let positions = vec![
            [-0.01, -0.01, radius],
            [0.01, -0.01, radius],
            [0.01, 0.01, radius],
            [-0.01, 0.01, radius],
        ];
        let normals = vec![[0., 0., 1.]; 4];
        let uv = vec![[0., 0.], [1., 0.], [1., 1.], [0., 1.]];
        let indices = vec![0, 1, 2, 0, 2, 3];
        let surface = geometry::mesh(
            positions.clone(),
            normals.clone(),
            uv.clone(),
            indices.clone(),
        );
        let accessory = accessory_geometry::mesh(
            positions,
            normals,
            uv,
            vec![[0.; 2]; 4],
            vec![[1.; 4]; 4],
            vec![[0.; 4]; 4],
            indices,
        );
        let meshes = [surface, accessory]
            .into_iter()
            .map(|m| world.resource_mut::<Assets<Mesh>>().add(m))
            .collect::<Vec<_>>();
        let root = world
            .spawn((Transform::default(), Visibility::Inherited))
            .id();
        let mut surfaces = Vec::new();
        for pass in [
            SpecialPass::Core,
            SpecialPass::Fringe,
            SpecialPass::MoholeMask,
        ] {
            let handle = world
                .resource_mut::<Assets<SpecialMaterial>>()
                .add(SpecialMaterial {
                    atmosphere: atmosphere.clone(),
                    parameters: parameter_buffer.clone(),
                    highlights: highlight_buffer.clone(),
                    textures: Textures {
                        pass,
                        colors: colors.clone(),
                        flow: flow.clone(),
                        highlight_data: data.clone(),
                    },
                });
            world.spawn((
                Mesh3d(meshes[0].clone()),
                MeshMaterial3d(handle.clone()),
                Transform::default(),
                NoFrustumCulling,
                NotShadowCaster,
                NotShadowReceiver,
                RenderLayers::layer(if pass == SpecialPass::MoholeMask {
                    3
                } else {
                    2
                }),
                ChildOf(root),
            ));
            surfaces.push(handle);
        }
        let mut accessories = Vec::new();
        for ground in [true, false] {
            let handle = world
                .resource_mut::<Assets<AccessoryMaterial>>()
                .add(AccessoryMaterial {
                    atmosphere: atmosphere.clone(),
                    parameters: parameter_buffer.clone(),
                    highlights: highlight_buffer.clone(),
                    textures: accessory_material::Textures {
                        ground,
                        colors: colors.clone(),
                        mid: mid.clone(),
                        high: high.clone(),
                        fence: fence.texture.clone(),
                        highlight_data: data.clone(),
                    },
                });
            world.spawn((
                Mesh3d(meshes[1].clone()),
                MeshMaterial3d(handle.clone()),
                Transform::default(),
                NoFrustumCulling,
                NotShadowCaster,
                NotShadowReceiver,
                RenderLayers::layer(2),
                ChildOf(root),
            ));
            accessories.push(handle);
        }
        let mut cameras = Vec::new();
        for (layer, target) in [(2, input.warm.clone()), (3, input.warm_mask.clone())] {
            let mut camera = world.spawn((
                Camera3d::default(),
                Camera {
                    order: if layer == 2 { -4 } else { -2 },
                    ..default()
                },
                RenderTarget::Image(target.into()),
                Msaa::Off,
                Tonemapping::None,
                bevy::core_pipeline::tonemapping::DebandDither::Disabled,
                RenderLayers::layer(layer),
                Transform::from_xyz(0., 0., 6.8).looking_at(Vec3::ZERO, Vec3::Y),
            ));
            if layer == 2 {
                camera.insert((
                    bevy::core_pipeline::prepass::DepthPrepass,
                    crate::terrain::surface::SurfaceCamera::Warm,
                ));
            } else {
                camera.insert((
                    bevy::camera::CompositingSpace::Linear,
                    crate::terrain::surface::SurfaceCamera::WarmMask,
                ));
            }
            cameras.push(camera.id());
        }
        self.warm = Some(Warm {
            root,
            cameras,
            meshes,
            surfaces,
            accessories,
            flow,
            buffers: vec![parameter_buffer, highlight_buffer],
            frames: 0,
        });
        Ok(false)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn retirement_waits_for_its_gpu_assets_after_the_native_delay() {
        let mut world = World::new();
        world.init_resource::<Assets<Image>>();
        let image = world.resource_mut::<Assets<Image>>().add(Image::default());
        let mut library = Library {
            retiring: Some(Retiring {
                meshes: Vec::new(),
                materials: Vec::new(),
                flow: image.id(),
                buffers: Vec::new(),
                frames: 2,
            }),
            ..default()
        };
        assert!(!library.finish_retirement(true));
        assert!(!library.finish_retirement(false));
        assert!(!library.ready);
        assert!(library.finish_retirement(true));
        assert!(library.retiring.is_none());
    }
    #[test]
    fn cancellation_removes_only_owned_views_meshes_and_materials() {
        let mut world = World::new();
        world.init_resource::<Assets<Mesh>>();
        world.init_resource::<Assets<Image>>();
        world.init_resource::<Assets<SpecialMaterial>>();
        world.init_resource::<Assets<AccessoryMaterial>>();
        let mesh = world.resource_mut::<Assets<Mesh>>().add(Cuboid::default());
        let flow = world.resource_mut::<Assets<Image>>().add(Image::default());
        let root = world.spawn_empty().id();
        let child = world.spawn(ChildOf(root)).id();
        let camera = world.spawn_empty().id();
        let unrelated = world.spawn_empty().id();
        let mut library = Library {
            warm: Some(Warm {
                root,
                cameras: vec![camera],
                meshes: vec![mesh.clone()],
                surfaces: Vec::new(),
                accessories: Vec::new(),
                flow: flow.clone(),
                buffers: Vec::new(),
                frames: 0,
            }),
            ..default()
        };
        assert!(library.cancel(&mut world).is_empty());
        for entity in [root, child, camera] {
            assert!(world.get_entity(entity).is_err());
        }
        assert!(world.get_entity(unrelated).is_ok());
        assert!(world.resource::<Assets<Mesh>>().get(mesh.id()).is_none());
        assert!(world.resource::<Assets<Image>>().get(flow.id()).is_none());
        assert!(!library.ready);
        assert!(library.warm.is_none());
        assert!(library.retiring.is_none());
    }
}
