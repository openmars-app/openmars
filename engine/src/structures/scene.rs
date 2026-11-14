//! Stable material chunks driven by the exact published landscape layouts.
use super::{
    geometry::{self, GeometryGroup},
    groups::{self, GROUPS, GroupPool},
    materials::{
        self, MaterialKey, StructureBuffer, StructureExtension, StructureMaterial,
        StructureTextures,
    },
    texture_library::TextureLibrary,
    textures::Slot,
};
use bevy::{
    asset::AssetId,
    camera::{
        RenderTarget,
        visibility::{NoFrustumCulling, RenderLayers},
    },
    core_pipeline::tonemapping::Tonemapping,
    light::{NotShadowCaster, NotShadowReceiver},
    mesh::VertexAttributeValues,
    pbr::PreparedMaterial,
    prelude::*,
    render::{
        RenderApp,
        erased_render_asset::ErasedRenderAssets,
        mesh::RenderMesh,
        render_asset::RenderAssets,
        render_resource::{Buffer, PipelineCache},
        renderer::{RenderDevice, RenderQueue},
        texture::GpuImage,
    },
};
use openmars_scene::{
    geometry::AxialPoint,
    landscape::LandscapeSnapshot,
    structure::{StructureLayoutRef, StructureLighting},
};
use std::{
    collections::{BTreeMap, BTreeSet},
    ops::Range,
    sync::Arc,
};
mod arrival;
mod compact;
mod library;
fn err(message: &str) -> String {
    message.into()
}
type Coordinate = (i32, i32, i32);
const MAX_CHUNK_VERTICES: usize = 65536;
const MAX_CHUNKS: usize = 128;
const MAX_MESH_BYTES: u64 = 64 * 1024 * 1024;
#[derive(Clone)]
struct Slice {
    vertices: Range<usize>,
    indices: Range<usize>,
}
#[derive(Clone)]
struct Chunk {
    key: MaterialKey,
    root: Entity,
    mesh: Handle<Mesh>,
    members: BTreeMap<Coordinate, Slice>,
}
struct Pending {
    id: usize,
    next: Chunk,
    previous: Option<Chunk>,
}
#[derive(Clone)]
struct Plot {
    layout: StructureLayoutRef,
    snapshot: Arc<LandscapeSnapshot>,
    parts: Vec<usize>,
    groups: Vec<u32>,
}
pub struct Input {
    pub snapshot: Arc<LandscapeSnapshot>,
    pub step: u64,
    pub frost: f32,
    pub reduced: bool,
    pub settled: bool,
    pub warm: Handle<Image>,
}
#[derive(Resource, Default)]
pub struct StructureRuntime {
    source: Option<Arc<LandscapeSnapshot>>,
    root: Option<Entity>,
    group_image: Option<Handle<Image>>,
    group_pool: Option<GroupPool>,
    buffers: Vec<StructureBuffer>,
    materials: BTreeMap<MaterialKey, Handle<StructureMaterial>>,
    chunks: BTreeMap<usize, Chunk>,
    pending: Vec<Pending>,
    next_chunk: usize,
    plots: BTreeMap<Coordinate, Plot>,
    births: BTreeMap<Coordinate, f64>,
    pending_births: BTreeSet<Coordinate>,
    retired_groups: Vec<Vec<u32>>,
    retired_meshes: Vec<AssetId<Mesh>>,
    retired: Vec<Buffer>,
    warm_camera: Option<Entity>,
    warm_frames: u32,
    library_warm: Option<library::WarmLibrary>,
    library_warmed: bool,
    library_retiring: Option<AssetId<Mesh>>,
    ready: bool,
    seconds: f64,
    pub revision: u64,
    pub uniform_writes: u64,
    pub retired_uniforms: u64,
    pub geometry_builds: u64,
}
impl StructureRuntime {
    pub fn ready(&self) -> bool {
        self.ready
    }
    pub fn covers(&self, coordinate: Coordinate) -> bool {
        self.plots.get(&coordinate).is_some_and(|plot| {
            plot.parts.iter().all(|id| {
                self.chunks.contains_key(id)
                    || self.pending.iter().any(|p| {
                        p.id == *id
                            && p.previous
                                .as_ref()
                                .is_some_and(|old| old.members.contains_key(&coordinate))
                    })
            })
        })
    }
    pub fn progress(&self) -> Option<String> {
        (!self.ready).then(|| {
            if !self.library_warmed {
                return format!(
                    "Preparing city materials ({} surfaces)",
                    self.materials.len()
                );
            }
            format!(
                "Preparing cities ({} plots, {} chunks)",
                self.plots.len(),
                self.chunks.len() + self.pending.len()
            )
        })
    }
    pub fn diagnostics(&self) -> serde_json::Value {
        serde_json::json!({"pending":self.pending.len(),"retiringMeshes":self.retired_meshes.len(),"warmFrames":self.warm_frames,"ready":self.ready,"meshIds":self.chunks.values().map(|c|format!("{:?}",c.mesh.id())).collect::<Vec<_>>(),"bufferIds":self.buffers.iter().map(|b|format!("{:?}",b.buffer().id())).collect::<Vec<_>>(),"groupImage":self.group_image.as_ref().map(|h|format!("{:?}",h.id())),"plots":self.plots.len(),"chunks":self.chunks.len()+self.pending.len(),"materials":self.materials.len(),"groups":self.group_pool.as_ref().map_or(0,|p|GROUPS-p.free.len()),"uniforms":self.buffers.len()+self.retired.len(),"groupImages":usize::from(self.group_image.is_some()),"textureWrites":self.group_pool.as_ref().map_or(0,|p|p.writes),"uniformWrites":self.uniform_writes,"retiredUniforms":self.retired_uniforms,"geometryBuilds":self.geometry_builds,"sample":self.plots.iter().take(32).map(|(key,p)|serde_json::json!({"coordinate":key,"name":p.layout.layout().name,"parts":p.parts,"groups":p.groups.len(),"born":self.births.get(key),"collapseTimes":p.groups.iter().map(|id|self.group_pool.as_ref().unwrap().rows[*id as usize*6+2][1]).collect::<Vec<_>>()})).collect::<Vec<_>>()})
    }
    fn initialize(&mut self, world: &mut World, device: &RenderDevice, queue: &RenderQueue) {
        if self.root.is_some() {
            return;
        }
        self.root = Some(
            world
                .spawn((Transform::IDENTITY, Visibility::Inherited))
                .id(),
        );
        self.group_image = Some(world.resource_mut::<Assets<Image>>().add(groups::image()));
        self.group_pool = Some(GroupPool::new());
        self.buffers = (0..5)
            .map(|class| {
                StructureBuffer::new(
                    materials::parameters(
                        class,
                        Vec4::new(self.seconds as f32, self.seconds as f32, 0., 0.),
                    ),
                    device,
                    queue,
                )
            })
            .collect();
    }
    fn material(
        &mut self,
        world: &mut World,
        key: MaterialKey,
    ) -> Result<Handle<StructureMaterial>, String> {
        if let Some(handle) = self.materials.get(&key) {
            return Ok(handle.clone());
        }
        let library = world.resource::<TextureLibrary>();
        let base = materials::surface(library, key)?;
        let noise = library
            .get(Slot::Shared(3))
            .ok_or("Missing structure frost noise")?;
        let material = StructureMaterial {
            base,
            extension: StructureExtension {
                atmosphere: world
                    .resource::<crate::atmosphere::AtmosphereRuntime>()
                    .binding()?,
                parameters: self.buffers[materials::buffer_class(key.index)].buffer(),
                textures: StructureTextures {
                    groups: self.group_image.as_ref().unwrap().clone(),
                    frost_noise: noise,
                },
            },
        };
        let handle = world
            .resource_mut::<Assets<StructureMaterial>>()
            .add(material);
        self.materials.insert(key, handle.clone());
        Ok(handle)
    }
    fn mesh_bytes(&self, world: &World) -> u64 {
        self.chunks
            .values()
            .chain(
                self.pending
                    .iter()
                    .flat_map(|p| std::iter::once(&p.next).chain(p.previous.iter())),
            )
            .filter_map(|c| world.resource::<Assets<Mesh>>().get(&c.mesh))
            .map(mesh_bytes)
            .sum()
    }
    fn uploaded(&self, render: &World) -> bool {
        let meshes = render.resource::<RenderAssets<RenderMesh>>();
        let materials = render.resource::<ErasedRenderAssets<PreparedMaterial>>();
        self.pending
            .iter()
            .all(|p| meshes.get(p.next.mesh.id()).is_some())
            && self
                .library_warm
                .as_ref()
                .is_none_or(|warm| meshes.get(warm.mesh.id()).is_some())
            && self
                .materials
                .values()
                .all(|h| materials.get(h.id().untyped()).is_some())
            && self.group_image.as_ref().is_some_and(|h| {
                render
                    .resource::<RenderAssets<GpuImage>>()
                    .get(h.id())
                    .is_some()
            })
            && render
                .resource::<PipelineCache>()
                .waiting_pipelines()
                .next()
                .is_none()
    }
    fn update(
        &mut self,
        world: &mut World,
        input: Input,
        uploaded: bool,
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<(), String> {
        if input.step as f64 / 60. < self.seconds || !input.frost.is_finite() {
            return Err(err("Structure clock/climate invalid"));
        }
        if input.settled {
            self.pending_births.clear();
            self.births.values_mut().for_each(|born| *born = -1000.);
            if let Some(pool) = &mut self.group_pool {
                for id in self.plots.values().flat_map(|plot| &plot.groups) {
                    let timing = &mut pool.rows[*id as usize * 6 + 2];
                    if *timing != [-1000., 1e9, 1., 0.] {
                        *timing = [-1000., 1e9, 1., 0.];
                        pool.dirty.insert(*id);
                    }
                }
            }
        }
        let previous = self.seconds;
        self.seconds = input.step as f64 / 60.;
        if self
            .source
            .as_ref()
            .is_none_or(|s| s.source_identity() != input.snapshot.source_identity())
        {
            self.source = Some(input.snapshot.clone());
            self.ready = false;
        }
        let desired: BTreeMap<Coordinate, StructureLayoutRef> = input
            .snapshot
            .sources()
            .iter()
            .filter_map(|s| {
                s.layout.as_ref().map(|l| {
                    (
                        (s.coordinate[0], s.coordinate[1], s.coordinate[2]),
                        l.clone(),
                    )
                })
            })
            .collect();
        self.initialize(world, device, queue);
        if !self.prepare_library(world, input.warm.clone(), uploaded)? {
            self.ready = false;
            return Ok(());
        }
        if desired.is_empty() && self.plots.is_empty() && self.pending.is_empty() {
            self.ready = true;
            return Ok(());
        }
        for buffer in &mut self.buffers {
            buffer.write(
                Vec4::new(
                    self.seconds as f32,
                    previous as f32,
                    input.frost.clamp(0., 1.),
                    if input.reduced { 1. } else { 0. },
                ),
                device,
                queue,
            )?;
            self.uniform_writes += 1;
        }
        let timeline = world.resource::<crate::nuclear::timeline::Timeline>();
        let mut held = std::collections::BTreeSet::new();
        for (c, plot) in &self.plots {
            let live = !input.settled && timeline.live(*c);
            let start = timeline.start(*c);
            if live && start.is_none_or(|s| !s.ended(input.step, input.reduced)) {
                held.insert(*c);
            }
            if let Some(pool) = &mut self.group_pool {
                let death = if live {
                    start.map_or(1e9, |s| {
                        s.step as f32 / 60. + if input.reduced { 0. } else { 1. }
                    })
                } else {
                    1e9
                };
                for id in &plot.groups {
                    let timing = &mut pool.rows[*id as usize * 6 + 2];
                    if timing[1] != death {
                        timing[1] = death;
                        pool.dirty.insert(*id);
                    }
                }
            }
        }
        if !self.pending.is_empty() {
            let shared = world
                .query::<(&Camera, &RenderLayers)>()
                .iter(world)
                .any(|(c, l)| c.is_active && l == &RenderLayers::layer(2));
            if !shared {
                self.warm_camera = Some(
                    world
                        .spawn((
                            Camera3d::default(),
                            bevy::core_pipeline::prepass::DepthPrepass,
                            Camera {
                                order: -4,
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
            self.warm_frames += 1;
            if uploaded && self.warm_frames >= 2 {
                self.activate(world)?;
                if let Some(camera) = self.warm_camera.take() {
                    world.despawn(camera);
                }
            }
            return Ok(());
        }
        if !self.retired_meshes.is_empty() {
            return Ok(());
        }
        for ids in self.retired_groups.drain(..) {
            for id in ids {
                self.group_pool.as_mut().unwrap().release(id);
            }
        }
        for (key, plot) in &mut self.plots {
            if desired.get(key) == Some(&plot.layout) {
                plot.layout = desired[key].clone();
                plot.snapshot = input.snapshot.clone();
            }
        }
        if let Some(key) = self
            .plots
            .iter()
            .find(|(key, plot)| desired.get(*key) != Some(&plot.layout) && !held.contains(*key))
            .map(|(key, _)| *key)
        {
            self.ready = false;
            self.remove(world, key)?;
            self.revision += 1;
            return Ok(());
        }
        if let Some((key, layout)) = desired
            .iter()
            .find(|(key, _)| !self.plots.contains_key(*key))
        {
            self.ready = false;
            self.births
                .insert(*key, if !input.settled { self.seconds } else { -1000. });
            self.add(world, *key, layout.clone())?;
            if !input.settled {
                self.pending_births.insert(*key);
            }
            self.warm_frames = 0;
            self.geometry_builds += 1;
            return Ok(());
        }
        self.ready = true;
        Ok(())
    }
    fn add(
        &mut self,
        world: &mut World,
        key: Coordinate,
        layout: StructureLayoutRef,
    ) -> Result<(), String> {
        let _profile = crate::profiling::span(world, "structures.geometry_and_chunks")
            .arg("q", key.0)
            .arg("r", key.1);
        // The procedural census determines the complete worst-case CPU peak
        // before the generator, indexing table or mesh copies can allocate.
        let peak = {
            let _profile = crate::profiling::span(world, "structures.bound");
            geometry::working_bound(layout.layout()).map_err(|_| err("Structure geometry bound"))?
        };
        if peak > 96 * 1024 * 1024 {
            return Err(err("Structure scratch capacity"));
        }
        let mut geometry = {
            let _profile = crate::profiling::span(world, "structures.generate");
            geometry::generate(layout.layout()).map_err(|_| err("structure geometry invalid"))?
        };
        // Skip only the fully buried portion of the existing cubic rise.
        // The projected mesh has a -0.007 foundation bias; published city pads
        // sit at +0.0003. The extra margin starts just above that surface.
        let arrival_lead = arrival::visible_lead(geometry.bounds[1][2], geometry.emergence_depth);
        let scalar = self
            .source
            .as_ref()
            .ok_or("Missing published structure layout")?
            .projection();
        {
            let _profile = crate::profiling::span(world, "structures.project");
            geometry
                .project(scalar, AxialPoint { q: key.0, r: key.1 })
                .map_err(|_| err("structure projection failed"))?;
        }
        let center = scalar.hex_center(AxialPoint { q: key.0, r: key.1 });
        let origin = Vec3::from_array(
            scalar
                .project_content(center, 0.)
                .map_err(|_| err("structure origin"))?
                .map(|n| n as f32),
        );
        let up = origin.normalize();
        let rotation = Quat::from_rotation_arc(Vec3::Z, up);
        let right = rotation * Vec3::X;
        let forward = rotation * Vec3::Y;
        let mut group_ids = BTreeMap::<[u32; 7], u32>::new();
        let mut parts = Vec::new();
        for batch in &geometry.batches {
            let mut indexed = {
                let _profile = crate::profiling::span(world, "structures.index")
                    .arg("material", batch.material)
                    .arg("vertices", batch.positions.len());
                batch
                    .indexed(0)
                    .map_err(|_| err("structure indexing failed"))?
            };
            let mut remap = Vec::new();
            for group in &indexed.groups {
                let hash = group_key(group);
                let id = match group_ids.get(&hash) {
                    Some(&id) => id,
                    None => {
                        let id = self.group_pool.as_mut().unwrap().allocate()?;
                        group_ids.insert(hash, id);
                        self.group_pool.as_mut().unwrap().set(
                            id,
                            [
                                [
                                    group.pivot[0],
                                    group.pivot[1],
                                    group.pivot[2],
                                    group.motion[0],
                                ],
                                [
                                    group.motion[1],
                                    group.motion[2],
                                    group.motion[3],
                                    geometry.emergence_depth,
                                ],
                                [
                                    *self.births.get(&key).unwrap_or(&-1000.) as f32,
                                    1e9,
                                    1.,
                                    arrival_lead,
                                ],
                                [up.x, up.y, up.z, origin.x],
                                [right.x, right.y, right.z, origin.y],
                                [forward.x, forward.y, forward.z, origin.z],
                            ],
                        );
                        id
                    }
                };
                remap.push(id);
            }
            let Some(VertexAttributeValues::Float32x2(animation)) =
                indexed.mesh.attribute_mut(Mesh::ATTRIBUTE_UV_1)
            else {
                return Err(err("structure animation attribute absent"));
            };
            for values in animation {
                values[1] = *remap
                    .get(values[1] as usize)
                    .ok_or_else(|| err("structure group remap"))?
                    as f32;
            }
            let material_key = MaterialKey {
                index: batch.material,
                bright: batch.material < 3
                    && layout.layout().style.lighting == StructureLighting::Bright,
            };
            let material = self.material(world, material_key)?;
            let chosen = self
                .chunks
                .iter()
                .find(|(_, chunk)| {
                    chunk.key == material_key
                        && material_key.index != 10
                        && world
                            .resource::<Assets<Mesh>>()
                            .get(&chunk.mesh)
                            .is_some_and(|old| {
                                old.count_vertices() + indexed.mesh.count_vertices()
                                    <= MAX_CHUNK_VERTICES
                            })
                })
                .map(|(id, _)| *id);
            let id = match chosen {
                Some(id) => id,
                None => {
                    if self.chunks.len() + self.pending.len() >= MAX_CHUNKS {
                        return Err(err("structure chunk capacity"));
                    }
                    let id = self.next_chunk;
                    self.next_chunk = self
                        .next_chunk
                        .checked_add(1)
                        .ok_or_else(|| err("structure chunk identity exhausted"))?;
                    id
                }
            };
            let old = self.chunks.get(&id).cloned();
            let vstart = old
                .as_ref()
                .map(|chunk| {
                    world
                        .resource::<Assets<Mesh>>()
                        .get(&chunk.mesh)
                        .map(Mesh::count_vertices)
                        .unwrap_or(0)
                })
                .unwrap_or(0);
            let istart = old
                .as_ref()
                .map(|chunk| {
                    world
                        .resource::<Assets<Mesh>>()
                        .get(&chunk.mesh)
                        .and_then(Mesh::indices)
                        .map(|v| v.len())
                        .unwrap_or(0)
                })
                .unwrap_or(0);
            let vertex_count = indexed.mesh.count_vertices();
            let index_count = indexed.mesh.indices().map(|v| v.len()).unwrap_or(0);
            if vertex_count > MAX_CHUNK_VERTICES {
                return Err(err("structure part exceeds one bounded chunk"));
            }
            let previous = old.as_ref().map(|chunk| chunk.mesh.clone());
            let bytes = indexed.bytes
                + old
                    .as_ref()
                    .and_then(|chunk| world.resource::<Assets<Mesh>>().get(&chunk.mesh))
                    .map(mesh_bytes)
                    .unwrap_or(0);
            if self.mesh_bytes(world).saturating_add(bytes) > MAX_MESH_BYTES {
                return Err(err("Structure mesh residency capacity"));
            }
            let next = match previous {
                Some(old) => {
                    let mut mesh = world
                        .resource::<Assets<Mesh>>()
                        .get(&old)
                        .ok_or("Previous structure chunk absent")?
                        .clone();
                    mesh.merge(&indexed.mesh)
                        .map_err(|_| err("Structure chunk attribute mismatch"))?;
                    mesh
                }
                None => indexed.mesh,
            };
            let new_mesh = world.resource_mut::<Assets<Mesh>>().add(next);
            let root = world
                .spawn((
                    Mesh3d(new_mesh.clone()),
                    MeshMaterial3d(material),
                    Transform::IDENTITY,
                    Visibility::Inherited,
                    RenderLayers::layer(2),
                    NoFrustumCulling,
                    ChildOf(self.root.unwrap()),
                ))
                .id();
            if [10, 14, 15].contains(&batch.material) {
                world
                    .entity_mut(root)
                    .insert((NotShadowCaster, NotShadowReceiver));
            }
            let mut members = old
                .as_ref()
                .map(|chunk| chunk.members.clone())
                .unwrap_or_default();
            members.insert(
                key,
                Slice {
                    vertices: vstart..vstart + vertex_count,
                    indices: istart..istart + index_count,
                },
            );
            self.chunks.remove(&id);
            self.pending.push(Pending {
                id,
                next: Chunk {
                    key: material_key,
                    root,
                    mesh: new_mesh,
                    members,
                },
                previous: old,
            });
            parts.push(id);
        }
        self.plots.insert(
            key,
            Plot {
                layout,
                snapshot: self.source.as_ref().unwrap().clone(),
                parts,
                groups: group_ids.values().copied().collect(),
            },
        );
        Ok(())
    }
    fn remove(&mut self, world: &mut World, key: Coordinate) -> Result<(), String> {
        let plot = self
            .plots
            .get(&key)
            .cloned()
            .ok_or_else(|| err("structure plot absent"))?;
        for id in plot.parts {
            let mut chunk = self
                .chunks
                .get(&id)
                .cloned()
                .ok_or_else(|| err("structure chunk absent"))?;
            self.retired_meshes.push(chunk.mesh.id());
            chunk
                .members
                .remove(&key)
                .ok_or_else(|| err("structure slice absent"))?;
            if chunk.members.is_empty() {
                self.chunks.remove(&id);
                let _ = world.despawn(chunk.root);
                continue;
            }
            let mesh = world
                .resource::<Assets<Mesh>>()
                .get(&chunk.mesh)
                .ok_or_else(|| err("structure mesh absent"))?;
            let bytes = mesh_bytes(mesh);
            let old_handle = chunk.mesh.clone();
            let old_root = chunk.root;
            let members = chunk.members.clone();
            let material = self.material(world, chunk.key)?;
            if self.mesh_bytes(world).saturating_add(bytes) > MAX_MESH_BYTES {
                return Err(err("Structure compaction capacity"));
            }
            let mesh = world
                .resource::<Assets<Mesh>>()
                .get(&old_handle)
                .ok_or("Structure mesh absent")?;
            let (next, updated) = compact::compact_mesh(mesh, &members)?;
            let next_members = updated;
            let new_mesh = world.resource_mut::<Assets<Mesh>>().add(next);
            let root = world
                .spawn((
                    Mesh3d(new_mesh.clone()),
                    MeshMaterial3d(material),
                    Transform::IDENTITY,
                    Visibility::Inherited,
                    RenderLayers::layer(2),
                    NoFrustumCulling,
                    ChildOf(self.root.unwrap()),
                ))
                .id();
            if [10, 14, 15].contains(&chunk.key.index) {
                world
                    .entity_mut(root)
                    .insert((NotShadowCaster, NotShadowReceiver));
            }
            chunk.root = old_root;
            self.chunks.remove(&id);
            self.pending.push(Pending {
                id,
                next: Chunk {
                    key: chunk.key,
                    root,
                    mesh: new_mesh,
                    members: next_members,
                },
                previous: Some(chunk),
            });
        }
        self.plots.remove(&key);
        self.retired_groups.push(plot.groups);
        self.births.remove(&key);
        self.pending_births.remove(&key);
        self.warm_frames = 0;
        Ok(())
    }
    pub fn activate(&mut self, world: &mut World) -> Result<(), String> {
        let _event = crate::profiling::instant(world, "structures.publish")
            .arg(
                "landscapeRevision",
                self.source.as_ref().map_or(0, |s| s.revision()),
            )
            .arg("chunks", self.pending.len())
            .arg("plots", self.plots.len());
        // GPU preparation must not consume a live city's visible animation.
        // Reset only newly added plots, leaving existing merged members intact.
        for coordinate in std::mem::take(&mut self.pending_births) {
            let Some(born) = self.births.get_mut(&coordinate) else {
                continue;
            };
            if *born < 0. {
                continue;
            }
            *born = self.seconds;
            for group in &self.plots[&coordinate].groups {
                let pool = self.group_pool.as_mut().unwrap();
                pool.rows[*group as usize * 6 + 2][0] = self.seconds as f32;
                pool.dirty.insert(*group);
            }
        }
        for pending in self.pending.drain(..) {
            world
                .entity_mut(pending.next.root)
                .insert((Visibility::Inherited, RenderLayers::default()));
            if let Some(old) = pending.previous {
                let _ = world.despawn(old.root);
                if !self.retired_meshes.contains(&old.mesh.id()) {
                    self.retired_meshes.push(old.mesh.id());
                }
            }
            self.chunks.insert(pending.id, pending.next);
        }
        self.revision += 1;
        Ok(())
    }
}
fn mesh_bytes(mesh: &Mesh) -> u64 {
    mesh.count_vertices() as u64 * mesh.get_vertex_size()
        + mesh.indices().map_or(0, |i| i.len() as u64 * 4)
}

pub fn apply_entry(world: &mut World) {
    let visible = world
        .resource::<crate::entry::WorldEntrance>()
        .tiles
        .shown();
    let runtime = world.resource::<StructureRuntime>();
    let entities: Vec<_> = runtime
        .chunks
        .values()
        .chain(
            runtime
                .pending
                .iter()
                .filter_map(|pending| pending.previous.as_ref()),
        )
        .map(|chunk| chunk.root)
        .collect();
    for entity in entities {
        let visibility = if visible {
            Visibility::Inherited
        } else {
            Visibility::Hidden
        };
        if world.get::<Visibility>(entity) != Some(&visibility) {
            world.entity_mut(entity).insert(visibility);
        }
    }
}
fn group_key(group: &GeometryGroup) -> [u32; 7] {
    std::array::from_fn(|n| {
        if n < 3 {
            group.pivot[n].to_bits()
        } else {
            group.motion[n - 3].to_bits()
        }
    })
}
pub fn prepare_queued(
    app: &mut App,
    input: Option<Input>,
    device: &RenderDevice,
    queue: &RenderQueue,
) -> Result<(), String> {
    let _profile = crate::profiling::span(app.world(), "structures.prepare");
    let mut runtime = app
        .world_mut()
        .remove_resource::<StructureRuntime>()
        .unwrap();
    let result = (|| {
        for buffer in runtime.retired.drain(..) {
            buffer.destroy();
            runtime.retired_uniforms += 1;
        }
        let render = app.sub_app(RenderApp).world();
        runtime.retired_meshes.retain(|id| {
            render
                .resource::<RenderAssets<RenderMesh>>()
                .get(*id)
                .is_some()
        });
        let uploaded = runtime.uploaded(render);
        if app.world().resource::<TextureLibrary>().ready() {
            if let Some(input) = input {
                runtime.update(app.world_mut(), input, uploaded, device, queue)?;
            }
            if let Some(image) = &runtime.group_image
                && let Some(gpu) = app
                    .sub_app(RenderApp)
                    .world()
                    .resource::<RenderAssets<GpuImage>>()
                    .get(image.id())
            {
                runtime.group_pool.as_mut().unwrap().publish(gpu, queue)?;
            }
        }
        Ok(())
    })();
    app.world_mut().insert_resource(runtime);
    result
}
pub fn reset(world: &mut World) {
    let Some(mut runtime) = world.remove_resource::<StructureRuntime>() else {
        return;
    };
    if let Some(root) = runtime.root.take() {
        world.despawn(root);
    }
    if let Some(camera) = runtime.warm_camera.take() {
        world.despawn(camera);
    }
    runtime.retire_library(world);
    for buffer in runtime.buffers.drain(..) {
        runtime.retired.push(buffer.buffer());
    }
    world.insert_resource(StructureRuntime {
        retired: runtime.retired,
        revision: runtime.revision + 1,
        uniform_writes: runtime.uniform_writes,
        retired_uniforms: runtime.retired_uniforms,
        geometry_builds: runtime.geometry_builds,
        ..default()
    });
}
