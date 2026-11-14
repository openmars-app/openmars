//! Source volcano and mohole surfaces, with shared immutable pass geometry.
mod accessory_geometry;
pub mod accessory_material;
mod fence;
pub(crate) mod frame;
use accessory_geometry::AccessoryWork;
use accessory_material::{AccessoryMaterial, AccessoryUniform};
mod geometry;
mod library;
mod lifetime;
pub mod material;
use bevy::{
    camera::{
        RenderTarget,
        visibility::{NoFrustumCulling, RenderLayers},
    },
    core_pipeline::tonemapping::Tonemapping,
    light::{NotShadowCaster, NotShadowReceiver},
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
use frame::Frame;
use geometry::{SurfaceKind, SurfaceWork};
use lifetime::{Life, Pose};
use material::{SpecialMaterial, SpecialPass, Textures};
use openmars_scene::SceneSnapshot;
use openmars_scene::geometry::volcano::{VOLCANO_DEFAULT, VolcanoFlowWork, VolcanoParams};
use std::collections::{BTreeMap, BTreeSet};
type Coordinate = (i32, i32, i32);
const MAX_SITES: usize = 512;
const MAX_MESH_BYTES: usize = 64 * 1024 * 1024;

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
struct Spec {
    kind: SurfaceKind,
    seed: u32,
}
impl Spec {
    fn same_geometry(self, other: Self) -> bool {
        self == other
    }
}
pub struct Input {
    pub snapshot: std::sync::Arc<openmars_scene::landscape::LandscapeSnapshot>,
    pub step: u64,
    pub motion_step: u64,
    pub reduced: bool,
    pub settled: bool,
    pub warm: Handle<Image>,
    pub warm_mask: Handle<Image>,
    pub climate: openmars_scene::climate::ClimateState,
}
fn desired(scene: &SceneSnapshot) -> BTreeMap<Coordinate, Spec> {
    scene
        .tiles
        .iter()
        .filter(|t| crate::world::is_mars(t))
        .filter_map(|tile| {
            let kind = SurfaceKind::from_type(tile.occupant.as_deref()?)?;
            Some((
                (tile.q, tile.r, tile.s),
                Spec {
                    kind,
                    seed: tile.visual.as_ref().map_or_else(
                        || {
                            openmars_scene::landscape::visual_seed(&format!(
                                "{}:{}",
                                scene.game_id, tile.id
                            ))
                        },
                        |v| v.seed,
                    ),
                },
            ))
        })
        .collect()
}
struct Plot {
    spec: Spec,
    frame: Frame,
    life: Life,
    root: Entity,
    parts: Vec<(Entity, bool)>,
    mesh: Handle<Mesh>,
    materials: Vec<Handle<SpecialMaterial>>,
    flow: Handle<Image>,
    uniform: UniformBuffer<AccessoryUniform>,
    accessories: Option<(Handle<Mesh>, Vec<Handle<AccessoryMaterial>>)>,
    bytes: usize,
    warm: u32,
    collapse_facts: [crate::nuclear::collapse::MotionFacts; 5],
}
struct Building {
    coordinate: Coordinate,
    spec: Spec,
    frame: Frame,
    work: SurfaceWork,
    flow: Option<VolcanoFlowWork>,
    geometry_done: bool,
    flow_done: bool,
    accessories: Option<AccessoryWork>,
    accessories_done: bool,
}
struct Retired {
    buffer: Buffer,
    bytes: usize,
    frames: u8,
}
#[derive(Resource, Default)]
pub struct SpecialRuntime {
    library: library::Library,
    plots: BTreeMap<Coordinate, Plot>,
    fence: Option<fence::Fence>,
    building: Option<Building>,
    highlights: Option<UniformBuffer<crate::terrain::materials::HighlightUniform>>,
    retired: Vec<Retired>,
    cameras: Vec<Entity>,
    ready: bool,
    pub revision: u64,
    writes: u64,
    builds: u64,
    retirements: u64,
}
impl SpecialRuntime {
    pub fn ready(&self) -> bool {
        self.ready
    }
    pub fn covers(&self, coordinate: Coordinate) -> bool {
        self.plots.get(&coordinate).is_some_and(|p| p.warm >= 2)
    }
    pub fn progress(&self) -> Option<String> {
        if !self.library.ready {
            return Some("Preparing special-surface materials".into());
        }
        (!self.ready).then(|| format!("Preparing special surfaces ({} sites)", self.plots.len()))
    }
    pub fn diagnostics(&self) -> serde_json::Value {
        serde_json::json!({"accessoryMeshes":self.plots.values().filter(|p|p.accessories.is_some()).count(),"fenceBytes":self.fence.as_ref().map_or(0,|f|f.bytes),"ready":self.ready,"sites":self.plots.len(),"building":self.building.is_some(),
            "uniforms":self.plots.len()+usize::from(self.highlights.is_some())+self.retired.len(),
            "meshBytes":self.plots.values().map(|p|p.bytes).sum::<usize>()+self.retired.iter().map(|p|p.bytes).sum::<usize>(),
            "geometryBuilds":self.builds,"uniformWrites":self.writes,"retiredUniforms":self.retirements,
            "highlightBuffer":self.highlights.as_ref().and_then(|b|b.buffer()).map(|b|format!("{:?}",b.id())),
            "sample":self.plots.iter().map(|(c,p)|serde_json::json!({"coordinate":c,"kind":format!("{:?}",p.spec.kind),"seed":p.spec.seed,
                "entity":p.root.to_string(),"mesh":format!("{:?}",p.mesh.id()),"buffer":format!("{:?}",p.uniform.buffer().unwrap().id()),
                "born":p.life.born,"destroy":p.life.destroyed.map(|d|d.0),"clock":p.uniform.get().base.clock.to_array(),"collapse":p.uniform.get().base.collapse_parts.map(|v|v.to_array()),"warmFrames":p.warm})).collect::<Vec<_>>()})
    }
    fn remove(&mut self, world: &mut World, coordinate: Coordinate) {
        if let Some(plot) = self.plots.remove(&coordinate) {
            world.despawn(plot.root);
            self.retired.push(Retired {
                buffer: plot.uniform.buffer().unwrap().clone(),
                bytes: plot.bytes,
                frames: 2,
            });
            self.revision += 1;
        }
    }
    fn update(
        &mut self,
        world: &mut World,
        input: Input,
        uploaded: &BTreeSet<Entity>,
        library_state: (bool, bool),
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<(), String> {
        let Some(scene) = world.resource::<crate::scene::SceneState>().0.as_ref() else {
            return Ok(());
        };
        let desired = desired(scene);
        // Occupied sites follow the published field. Empty claims have no
        // landscape source and need only the underlying bare surface.
        let available: BTreeSet<Coordinate> = input
            .snapshot
            .sources()
            .iter()
            .map(|s| (s.coordinate[0], s.coordinate[1], s.coordinate[2]))
            .chain(
                scene
                    .tiles
                    .iter()
                    .filter(|t| {
                        crate::world::is_mars(t) && t.occupant.is_none() && t.reserved_by.is_some()
                    })
                    .map(|t| (t.q, t.r, t.s)),
            )
            .collect();
        if desired.len() > MAX_SITES {
            return Err("Special surface count limit exceeded".into());
        }
        let nuclear: BTreeSet<_> = scene
            .tiles
            .iter()
            .filter(|t| {
                crate::world::is_mars(t) && t.occupant.as_deref() == Some("nuclear-zone-tile")
            })
            .map(|t| (t.q, t.r, t.s))
            .collect();
        let seconds = input.step as f64 / 60.;
        let motion_seconds = input.motion_step as f64 / 60.;
        if !self.library.ready {
            if self.fence.is_none() {
                self.fence = fence::prepare(world)?;
            }
            let Some(fence) = &self.fence else {
                self.ready = false;
                return Ok(());
            };
            if !self
                .library
                .prepare(world, &input, library_state, fence, device, queue)?
            {
                self.ready = false;
                return Ok(());
            }
        }
        let mut remove = Vec::new();
        for (&coordinate, plot) in &mut self.plots {
            if let Some(spec) = desired
                .get(&coordinate)
                .filter(|s| s.same_geometry(plot.spec))
            {
                plot.spec = *spec;
                if input.settled {
                    plot.life = Life::settled();
                } else if plot.life.destroyed.is_some() {
                    plot.life = Life {
                        born: seconds,
                        born_motion: motion_seconds,
                        destroyed: None,
                    };
                }
            } else if nuclear.contains(&coordinate)
                && !input.settled
                && world
                    .resource::<crate::nuclear::timeline::Timeline>()
                    .live(coordinate)
            {
                if let Some(start) = world
                    .resource::<crate::nuclear::timeline::Timeline>()
                    .start(coordinate)
                {
                    plot.life.destroyed.get_or_insert(start.seconds());
                }
            } else {
                remove.push(coordinate);
            }
            if plot.life.ended(seconds, input.reduced) {
                remove.push(coordinate);
            }
        }
        remove.sort_unstable();
        remove.dedup();
        for coordinate in remove {
            self.remove(world, coordinate);
        }
        if self.building.as_ref().is_some_and(|b| {
            desired
                .get(&b.coordinate)
                .is_none_or(|s| !s.same_geometry(b.spec))
        }) {
            self.building = None;
        }
        let noises = {
            let library = world.resource::<super::texture_library::TextureLibrary>();
            library
                .get(super::textures::Slot::Shared(3))
                .zip(library.get(super::textures::Slot::Shared(4)))
        };
        if desired.values().any(|s| s.kind == SurfaceKind::Mohole) && noises.is_none() {
            self.ready = false;
            return Ok(());
        }
        let colors = crate::assets::texture(world, "textures/terrain/color-array");
        if !desired.is_empty() && colors.is_none() {
            self.ready = false;
            return Ok(());
        }
        if !desired.is_empty() && self.highlights.is_none() {
            let mut buffer: UniformBuffer<crate::terrain::materials::HighlightUniform> = world
                .resource::<crate::terrain::grid::GridRuntime>()
                .uniform()
                .into();
            buffer.set_label(Some("special surface highlights"));
            buffer.write_buffer(device, queue);
            self.highlights = Some(buffer);
        }
        if self.building.is_none()
            && self.plots.len() + self.retired.len() < MAX_SITES
            && let Some((&coordinate, &spec)) = desired
                .iter()
                .find(|(c, _)| !self.plots.contains_key(c) && available.contains(c))
        {
            let retained = self.plots.values().map(|p| p.bytes).sum::<usize>()
                + self.retired.iter().map(|p| p.bytes).sum::<usize>();
            if retained
                + SurfaceWork::allocation_bytes(spec.kind) as usize
                + VolcanoFlowWork::PEAK_BYTES
                + if spec.kind == SurfaceKind::Mohole {
                    AccessoryWork::PEAK_BYTES as usize
                } else {
                    0
                }
                > MAX_MESH_BYTES
            {
                return Err("Special surface working-set limit exceeded".into());
            }
            let frame = Frame::new(
                input.snapshot.projection(),
                [coordinate.0, coordinate.1, coordinate.2],
            )?;
            self.building = Some(Building {
                coordinate,
                spec,
                work: SurfaceWork::new(spec.kind, frame.clone(), spec.seed)?,
                flow: if spec.kind == SurfaceKind::Volcano {
                    Some(
                        VolcanoFlowWork::new(VolcanoParams {
                            seed: spec.seed as f32,
                            ..VOLCANO_DEFAULT
                        })
                        .map_err(|e| e.to_string())?,
                    )
                } else {
                    None
                },
                accessories: if spec.kind == SurfaceKind::Mohole {
                    Some(AccessoryWork::new(
                        frame.clone(),
                        spec.seed,
                        self.fence.as_ref().unwrap().width,
                    )?)
                } else {
                    None
                },
                accessories_done: spec.kind != SurfaceKind::Mohole,
                frame,
                geometry_done: false,
                flow_done: spec.kind != SurfaceKind::Volcano,
            });
        }
        let mut complete = false;
        if let Some(b) = &mut self.building {
            for _ in 0..4 {
                if !b.geometry_done {
                    b.geometry_done = b.work.advance()?;
                } else if !b.flow_done {
                    b.flow_done = b
                        .flow
                        .as_mut()
                        .unwrap()
                        .advance()
                        .map_err(|e| e.to_string())?;
                } else if !b.accessories_done {
                    b.accessories_done = b
                        .accessories
                        .as_mut()
                        .unwrap()
                        .advance(&self.fence.as_ref().unwrap().mesh)?;
                }
                if b.geometry_done && b.flow_done && b.accessories_done {
                    complete = true;
                    break;
                }
            }
        }
        if complete {
            let mut b = self.building.take().unwrap();
            let coordinate = b.coordinate;
            let spec = b.spec;
            let frame = b.frame;
            let mesh = b
                .work
                .take_ready()
                .ok_or("Special surface geometry incomplete")?;
            let mut bytes = mesh.get_vertex_buffer_size()
                + mesh.get_index_buffer_bytes().map_or(0, |v| v.len());
            let mesh = world.resource_mut::<Assets<Mesh>>().add(mesh);
            let (size, values) = if let Some(work) = &mut b.flow {
                (64, work.take_ready().ok_or("Volcano flow incomplete")?)
            } else {
                (1, vec![0.])
            };
            bytes += values.len() * 4;
            let flow = world.resource_mut::<Assets<Image>>().add(Image::new(
                Extent3d {
                    width: size,
                    height: size,
                    depth_or_array_layers: 1,
                },
                TextureDimension::D2,
                values.into_iter().flat_map(f32::to_le_bytes).collect(),
                TextureFormat::R32Float,
                bevy::asset::RenderAssetUsages::RENDER_WORLD,
            ));
            let life = if input.settled {
                Life::settled()
            } else {
                Life {
                    born: seconds,
                    born_motion: motion_seconds,
                    destroyed: None,
                }
            };
            let extra = b
                .accessories
                .take()
                .map(AccessoryWork::take_mesh)
                .transpose()?;
            use bevy::math::{DMat4, DQuat, DVec3};
            let tile = DMat4::from_rotation_translation(
                DQuat::from_array(frame.rotation),
                DVec3::from_array(frame.origin),
            );
            let seed = u64::from(spec.seed);
            let mut collapse_facts = [crate::nuclear::collapse::MotionFacts {
                parameters: openmars_scene::geometry::collapse::CollapseParameters::new(seed),
                depth: 0.25,
            }; 5];
            let core = if spec.kind == SurfaceKind::Mohole {
                1
            } else {
                0
            };
            let radius = spec.kind.extent().0 as f32 as f64;
            collapse_facts[core] = crate::nuclear::collapse::PartSource {
                seed: seed + core as u64 * 97,
                bounds: [
                    DVec3::new(-radius, -radius, 0.),
                    DVec3::new(radius, radius, 0.),
                ],
                body_from_mesh: tile,
                tile_from_body: tile.inverse(),
                cart: false,
                minimum: if spec.kind == SurfaceKind::Volcano {
                    0.21
                } else {
                    0.
                },
            }
            .capture(DMat4::IDENTITY);
            if let Some((_, bounds)) = &extra {
                for (i, bounds) in bounds.iter().enumerate() {
                    collapse_facts[i + 2] = crate::nuclear::collapse::PartSource {
                        seed: seed + (i as u64 + 2) * 97,
                        bounds: *bounds,
                        body_from_mesh: tile,
                        tile_from_body: tile.inverse(),
                        cart: false,
                        minimum: 0.,
                    }
                    .capture(DMat4::IDENTITY);
                }
            }
            let uniform = UniformBuffer::from(life.parameters(
                spec,
                &frame,
                Pose {
                    radius: input.snapshot.projection().content_radius() as f32,
                    seconds,
                    motion_seconds,
                    reduced: input.reduced,
                    climate: input.climate,
                },
                &collapse_facts,
            )?);
            let mut uniform = UniformBuffer::from(AccessoryUniform {
                base: *uniform.get(),
                stages: life.stages(seconds, input.reduced),
                fence_surface: self.fence.as_ref().map_or(Vec4::ZERO, |f| f.surface),
            });
            uniform.set_label(Some("special surface parameters"));
            uniform.write_buffer(device, queue);
            let data = world.resource::<crate::terrain::grid::GridRuntime>().data();
            let passes = if spec.kind == SurfaceKind::Mohole {
                vec![
                    SpecialPass::Core,
                    SpecialPass::Fringe,
                    SpecialPass::MoholeMask,
                ]
            } else {
                vec![SpecialPass::Core, SpecialPass::Fringe]
            };
            let atmosphere = world
                .resource::<crate::atmosphere::AtmosphereRuntime>()
                .binding()?;
            let materials: Vec<_> = passes
                .iter()
                .map(|&pass| {
                    world
                        .resource_mut::<Assets<SpecialMaterial>>()
                        .add(SpecialMaterial {
                            atmosphere: atmosphere.clone(),
                            parameters: uniform.buffer().unwrap().clone(),
                            highlights: self.highlights.as_ref().unwrap().buffer().unwrap().clone(),
                            textures: Textures {
                                pass,
                                colors: colors.as_ref().unwrap().clone(),
                                flow: flow.clone(),
                                highlight_data: data.clone(),
                            },
                        })
                })
                .collect();
            let root = world
                .spawn((Transform::default(), Visibility::Inherited))
                .id();
            let mut parts: Vec<(Entity, bool)> = materials
                .iter()
                .zip(passes)
                .map(|(material, pass)| {
                    let mask = pass == SpecialPass::MoholeMask;
                    (
                        world
                            .spawn((
                                Mesh3d(mesh.clone()),
                                MeshMaterial3d(material.clone()),
                                Transform::default(),
                                NoFrustumCulling,
                                NotShadowCaster,
                                NotShadowReceiver,
                                RenderLayers::layer(if mask { 3 } else { 2 }),
                                ChildOf(root),
                            ))
                            .id(),
                        mask,
                    )
                })
                .collect();
            let accessories = if let Some((extra, _)) = extra {
                bytes += extra.get_vertex_buffer_size()
                    + extra.get_index_buffer_bytes().map_or(0, |v| v.len());
                let extra = world.resource_mut::<Assets<Mesh>>().add(extra);
                let (mid, high) = noises.as_ref().unwrap();
                let mats: Vec<_> = [true, false]
                    .into_iter()
                    .map(|ground| {
                        world
                            .resource_mut::<Assets<AccessoryMaterial>>()
                            .add(AccessoryMaterial {
                                atmosphere: atmosphere.clone(),
                                parameters: uniform.buffer().unwrap().clone(),
                                highlights: self
                                    .highlights
                                    .as_ref()
                                    .unwrap()
                                    .buffer()
                                    .unwrap()
                                    .clone(),
                                textures: accessory_material::Textures {
                                    ground,
                                    colors: colors.as_ref().unwrap().clone(),
                                    mid: mid.clone(),
                                    high: high.clone(),
                                    fence: self.fence.as_ref().unwrap().texture.clone(),
                                    highlight_data: data.clone(),
                                },
                            })
                    })
                    .collect();
                for mat in &mats {
                    parts.push((
                        world
                            .spawn((
                                Mesh3d(extra.clone()),
                                MeshMaterial3d(mat.clone()),
                                Transform::default(),
                                NoFrustumCulling,
                                NotShadowCaster,
                                NotShadowReceiver,
                                RenderLayers::layer(2),
                                ChildOf(root),
                            ))
                            .id(),
                        false,
                    ));
                }
                Some((extra, mats))
            } else {
                None
            };
            bytes += std::mem::size_of::<[crate::nuclear::collapse::MotionFacts; 5]>()
                + AccessoryUniform::min_size().get() as usize;
            self.plots.insert(
                coordinate,
                Plot {
                    spec,
                    frame,
                    life,
                    root,
                    parts,
                    mesh,
                    materials,
                    flow,
                    uniform,
                    accessories,
                    bytes,
                    warm: 0,
                    collapse_facts,
                },
            );
            self.builds += 1;
            self.revision += 1;
        }
        let highlights = world
            .resource::<crate::terrain::grid::GridRuntime>()
            .uniform();
        let data = world.resource::<crate::terrain::grid::GridRuntime>().data();
        if let Some(buffer) = &mut self.highlights {
            buffer.set(highlights);
            buffer.write_buffer(device, queue);
        }
        for plot in self.plots.values_mut() {
            let values = plot.life.parameters(
                plot.spec,
                &plot.frame,
                Pose {
                    radius: input.snapshot.projection().content_radius() as f32,
                    seconds,
                    motion_seconds,
                    reduced: input.reduced,
                    climate: input.climate,
                },
                &plot.collapse_facts,
            )?;
            plot.uniform.set(AccessoryUniform {
                base: values,
                stages: plot.life.stages(seconds, input.reduced),
                fence_surface: self.fence.as_ref().map_or(Vec4::ZERO, |f| f.surface),
            });
            plot.uniform.write_buffer(device, queue);
            self.writes += 1;
            for handle in &plot.materials {
                let mut materials = world.resource_mut::<Assets<SpecialMaterial>>();
                if materials.get(handle).unwrap().textures.highlight_data != data {
                    materials.get_mut(handle).unwrap().textures.highlight_data = data.clone();
                }
            }
            if let Some((_, handles)) = &plot.accessories {
                for handle in handles {
                    let mut materials = world.resource_mut::<Assets<AccessoryMaterial>>();
                    if materials.get(handle).unwrap().textures.highlight_data != data {
                        materials.get_mut(handle).unwrap().textures.highlight_data = data.clone();
                    }
                }
            }
            if plot.warm < 2 && uploaded.contains(&plot.root) {
                plot.warm += 1;
                if plot.warm == 2 {
                    for &(part, mask) in &plot.parts {
                        world.entity_mut(part).insert(if mask {
                            RenderLayers::layer(1)
                        } else {
                            RenderLayers::default()
                        });
                    }
                    self.revision += 1;
                }
            }
        }
        let warming = self.plots.values().any(|p| p.warm < 2);
        if warming {
            for (layer, target) in [(2, input.warm), (3, input.warm_mask)] {
                let shared = world
                    .query::<(&Camera, &RenderLayers)>()
                    .iter(world)
                    .any(|(c, l)| c.is_active && l == &RenderLayers::layer(layer));
                if !shared {
                    let mut entity = world.spawn((
                        Camera3d::default(),
                        Camera {
                            order: if layer == 2 { -4 } else { -2 },
                            clear_color: ClearColorConfig::Custom(Color::BLACK),
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
                        entity.insert((
                            bevy::core_pipeline::prepass::DepthPrepass,
                            crate::terrain::surface::SurfaceCamera::Warm,
                        ));
                    } else {
                        entity.insert((
                            bevy::camera::CompositingSpace::Linear,
                            crate::terrain::surface::SurfaceCamera::WarmMask,
                        ));
                    }
                    self.cameras.push(entity.id());
                }
            }
        } else if !warming {
            for camera in self.cameras.drain(..) {
                world.despawn(camera);
            }
        }
        self.ready = self.building.is_none()
            && desired.iter().all(|(c, s)| {
                self.plots
                    .get(c)
                    .is_some_and(|p| p.spec.same_geometry(*s) && p.warm >= 2)
            });
        if self.plots.is_empty()
            && self.building.is_none()
            && let Some(buffer) = self.highlights.take()
        {
            self.retired.push(Retired {
                buffer: buffer.buffer().unwrap().clone(),
                bytes: 0,
                frames: 2,
            });
        }
        Ok(())
    }
}
/// Source Tile entrance affects published actors; warm draws remain full size.
pub fn apply_entry(world: &mut World) {
    world.resource_scope(|world, runtime: Mut<SpecialRuntime>| {
        for (&coordinate, plot) in &runtime.plots {
            let scale = if plot.warm < 2 {
                1.
            } else {
                crate::entry::tile_scale(world, coordinate)
            };
            world.entity_mut(plot.root).insert(crate::entry::around(
                Vec3::from_array(plot.frame.origin.map(|v| v as f32)),
                scale,
            ));
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
    let _profile = crate::profiling::span(app.world(), "special_surfaces.prepare");
    let mut uploaded = BTreeSet::new();
    let render = app.sub_app(RenderApp).world();
    let library_state = app
        .world()
        .resource::<SpecialRuntime>()
        .library
        .gpu_state(render);
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
        for plot in app.world().resource::<SpecialRuntime>().plots.values() {
            if meshes.get(plot.mesh.id()).is_some()
                && render
                    .get_resource::<RenderAssets<bevy::render::texture::GpuImage>>()
                    .is_some_and(|images| images.get(plot.flow.id()).is_some())
                && plot
                    .materials
                    .iter()
                    .all(|h| materials.get(h.id().untyped()).is_some())
                && plot.accessories.as_ref().is_none_or(|(mesh, handles)| {
                    meshes.get(mesh.id()).is_some()
                        && handles
                            .iter()
                            .all(|h| materials.get(h.id().untyped()).is_some())
                })
            {
                uploaded.insert(plot.root);
            }
        }
    }
    let mut runtime = app.world_mut().remove_resource::<SpecialRuntime>().unwrap();
    for retired in &mut runtime.retired {
        retired.frames = retired.frames.saturating_sub(1);
    }
    runtime.retired.retain(|retired| {
        if retired.frames == 0 {
            retired.buffer.destroy();
            runtime.retirements += 1;
            false
        } else {
            true
        }
    });
    let result = if let Some(input) = input {
        runtime.update(
            app.world_mut(),
            input,
            &uploaded,
            library_state,
            device,
            queue,
        )
    } else {
        Ok(())
    };
    app.world_mut().insert_resource(runtime);
    result
}
pub fn reset(world: &mut World) {
    let Some(mut runtime) = world.remove_resource::<SpecialRuntime>() else {
        return;
    };
    for buffer in runtime.library.cancel(world) {
        runtime.retired.push(Retired {
            buffer,
            bytes: 0,
            frames: 2,
        });
    }
    let coordinates: Vec<_> = runtime.plots.keys().copied().collect();
    for c in coordinates {
        runtime.remove(world, c);
    }
    if let Some(buffer) = runtime.highlights.take() {
        runtime.retired.push(Retired {
            buffer: buffer.buffer().unwrap().clone(),
            bytes: 0,
            frames: 2,
        });
    }
    for camera in runtime.cameras.drain(..) {
        world.despawn(camera);
    }
    world.insert_resource(SpecialRuntime {
        retired: runtime.retired,
        writes: runtime.writes,
        builds: runtime.builds,
        retirements: runtime.retirements,
        ..default()
    });
}
pub struct SpecialPlugin;
impl Plugin for SpecialPlugin {
    fn build(&self, app: &mut App) {
        app.add_plugins(material::SpecialMaterialPlugin);
        bevy::asset::embedded_asset!(app, "accessories.wesl");
        app.add_plugins(MaterialPlugin::<AccessoryMaterial>::default());
        app.init_resource::<SpecialRuntime>();
    }
}
