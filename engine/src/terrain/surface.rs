//! Immutable terrain batches, real shader warming, and explicit GPU lifetime.
use super::{
    TerrainState,
    materials::*,
    upload::{field_images, patch_mesh},
};
use crate::{
    assets::{AssetCamera, AssetGlobe},
    scene::SceneState,
};
use bevy::{
    asset::UntypedAssetId,
    camera::{
        CompositingSpace, RenderTarget,
        visibility::{NoFrustumCulling, RenderLayers},
    },
    core_pipeline::tonemapping::{DebandDither, Tonemapping},
    pbr::PreparedMaterial,
    prelude::*,
    render::{
        RenderApp,
        erased_render_asset::ErasedRenderAssets,
        mesh::RenderMesh,
        render_asset::RenderAssets,
        render_resource::{BufferId, Extent3d, PipelineCache, TextureFormat},
        renderer::{RenderDevice, RenderQueue},
        texture::GpuImage,
    },
};
use openmars_scene::{
    SceneSnapshot,
    climate::{ClimateParameters, ClimateRuntime},
    clock::SceneClock,
    geometry::{
        BoardPoint,
        terrain::{FIELD_SIZE, PATCH_SIZE},
    },
    landscape::{
        LandscapeBuilder, LandscapeInput, LandscapeSnapshot, LandscapeSource, PreparationState,
        publication::{FieldSlot, FieldTimeline},
        upload::{FieldUpload, FieldUploadBuilder, MAX_UPLOAD_BYTES},
    },
};
use std::{collections::BTreeMap, num::NonZeroUsize, sync::Arc};

const MASK: usize = 1;
const WARM: usize = 2;
const WARM_MASK: usize = 3;
// Masks store only 0/1 channel bits, whose sRGB endpoints are exact. Using an
// sRGB attachment avoids Bevy requesting alternate texture views for unorm
// camera targets, which WebGL2 cannot provide. Shader values stay binary.
const MASK_FORMAT: TextureFormat = TextureFormat::Rgba8UnormSrgb;

pub struct Contact {
    pub next: Arc<LandscapeSnapshot>,
    timeline: FieldTimeline,
    step: u64,
}
impl Contact {
    pub fn sample_revision(&self) -> (u64, Option<u64>, u64) {
        self.timeline.sample_revision(self.step)
    }
    pub fn height(&self, point: BoardPoint) -> Result<f64, String> {
        self.timeline
            .height(point, self.step)
            .map_err(|e| e.to_string())
    }
}

#[derive(Component)]
pub(crate) enum SurfaceCamera {
    Mask,
    Warm,
    WarmMask,
}
#[derive(Component)]
struct SurfaceRoot;

struct Targets {
    mask: Handle<Image>,
    warm: Handle<Image>,
    warm_mask: Handle<Image>,
    blank: Handle<Image>,
    mask_camera: Entity,
}
struct Batch {
    root: Entity,
    ground_entities: Vec<Entity>,
    mask_entity: Entity,
    warm_globe: Entity,
    warm_cameras: [Entity; 2],
    fields: [Handle<Image>; 2],
    mesh: Handle<Mesh>,
    ground: [Handle<GroundMaterial>; 3],
    globe: Handle<GroundMaterial>,
    ocean: Handle<OceanMaterial>,
    mask: Handle<LakeMaskMaterial>,
    buffers: TerrainBuffers,
    buffer_ids: [BufferId; 2],
    next: Arc<LandscapeSnapshot>,
    timeline: FieldTimeline,
    capacity: usize,
    activation: Option<u64>,
    frames: u32,
    context: u32,
}
impl Batch {
    fn material_ids(&self) -> Vec<UntypedAssetId> {
        self.ground
            .iter()
            .map(|h| h.id().untyped())
            .chain([
                self.globe.id().untyped(),
                self.ocean.id().untyped(),
                self.mask.id().untyped(),
            ])
            .collect()
    }
    fn release(self, world: &mut World) -> TerrainBuffers {
        world.despawn(self.root);
        self.buffers
    }
    fn parameters(
        &self,
        climate: &ClimateRuntime,
        step: u64,
        camera: Vec3,
        intensity: f32,
    ) -> TerrainUniform {
        let mut p = TerrainUniform::from_climate(
            climate.current(),
            camera,
            crate::lighting::sun_direction(),
            crate::lighting::sun_rgb(),
            intensity,
        );
        p.geometry.x = self.next.projection().radius() as f32;
        p.geometry.y = PATCH_SIZE as f32;
        p.geometry.z = self.capacity as f32;
        p.geometry.w = step as f32 / 60.;
        p.bake.y = if self.timeline.transitioning() {
            self.activation.unwrap_or(step) as f32 / 60.
        } else {
            -1.
        };
        p.water_extra.y = climate.pulses().shimmer as f32;
        p
    }
}

#[derive(Resource)]
pub struct SurfaceRuntime {
    clock: SceneClock,
    motion_clock: SceneClock,
    climate: Option<ClimateRuntime>,
    seen: Option<SceneSnapshot>,
    sources: Option<Vec<LandscapeSource>>,
    desired: Option<LandscapeInput>,
    builder: LandscapeBuilder,
    remote: bool,
    worker_ticket: u64,
    worker_sent: bool,
    worker_relief: Option<[u8; 32]>,
    packing: Option<(FieldTimeline, FieldUploadBuilder)>,
    active: Option<Batch>,
    pending: Option<Batch>,
    targets: Option<Targets>,
    retired: Vec<TerrainBuffers>,
    serial: u64,
    desired_context: u32,
    contexts: BTreeMap<u64, u32>,
    reported_ready: u64,
    reported_wait: u64,
    pub uniform_writes: u64,
    pub retired_uniforms: u64,
}
impl Default for SurfaceRuntime {
    fn default() -> Self {
        Self {
            clock: SceneClock::default(),
            motion_clock: SceneClock::default(),
            climate: None,
            seen: None,
            sources: None,
            desired: None,
            builder: LandscapeBuilder::new(),
            remote: cfg!(target_arch = "wasm32"),
            worker_ticket: 0,
            worker_sent: false,
            worker_relief: None,
            packing: None,
            active: None,
            pending: None,
            targets: None,
            retired: Vec::new(),
            serial: 0,
            desired_context: 0,
            contexts: BTreeMap::new(),
            reported_ready: 0,
            reported_wait: 0,
            uniform_writes: 0,
            retired_uniforms: 0,
        }
    }
}
impl SurfaceRuntime {
    pub fn warm_target(&self) -> Option<Handle<Image>> {
        self.targets.as_ref().map(|t| t.warm.clone())
    }
    pub fn ready(&self) -> bool {
        self.active.is_some()
            && self.pending.is_none()
            && self.packing.is_none()
            && self.desired.is_none()
            && matches!(
                self.builder.state(),
                PreparationState::Idle | PreparationState::Cancelled
            )
    }
    pub fn motion_step(&self) -> u64 {
        self.motion_clock.step()
    }
    pub fn step(&self) -> u64 {
        self.clock.step()
    }
    pub fn batches(&self) -> usize {
        usize::from(self.active.is_some()) + usize::from(self.pending.is_some())
    }
    pub fn uniforms(&self) -> usize {
        (self.batches() + self.retired.len()) * 2
    }
    pub fn revision(&self) -> u64 {
        self.active.as_ref().map_or(0, |b| b.next.revision())
    }
    pub fn requested_revision(&self) -> u64 {
        self.serial
    }
    pub fn transitioning(&self) -> bool {
        self.active
            .as_ref()
            .is_some_and(|b| b.timeline.transitioning())
    }
    pub fn patches(&self) -> usize {
        self.active.as_ref().map_or(0, |b| b.next.patches().count())
    }
    pub fn progress(&self) -> Option<String> {
        if let Some((_, work)) = &self.packing {
            let (done, total) = work.progress();
            return Some(format!("Packing terrain ({done}/{total} rows)"));
        }
        if let PreparationState::Preparing { completed, total } = self.builder.state() {
            return Some(format!("Preparing landscape ({completed}/{total} samples)"));
        }
        self.pending
            .as_ref()
            .map(|_| "Preparing terrain shaders".into())
    }
    /// New tiles may wait behind one useful in-flight publication. Replacing
    /// that work on every addition starves arrivals during continuous play.
    /// Keep only the newest successor; edits/removals still invalidate old work.
    fn queue_input(&mut self, input: LandscapeInput) -> (bool, Option<Batch>) {
        let next: std::collections::BTreeMap<_, _> = input
            .sources()
            .iter()
            .map(|source| (source.coordinate, source))
            .collect();
        let additive = self.sources.as_ref().is_some_and(|previous| {
            previous.len() < next.len()
                && previous.iter().all(|source| {
                    next.get(&source.coordinate)
                        .is_some_and(|next| *next == source)
                })
                && previous
                    .iter()
                    .filter(|s| s.kind == openmars_scene::landscape::LandscapeKind::Excluded)
                    .count()
                    == next
                        .values()
                        .filter(|s| s.kind == openmars_scene::landscape::LandscapeKind::Excluded)
                        .count()
        });
        self.sources = Some(input.sources().to_vec());
        self.desired = Some(input);
        if additive {
            (true, None)
        } else {
            self.builder.cancel();
            self.packing = None;
            (false, self.pending.take())
        }
    }
    fn begin_queued(&mut self) -> Result<Option<u64>, String> {
        // Ready is NOT idle: the completed snapshot must be taken and staged
        // before another begin() can replace the builder's completed output.
        if !matches!(
            self.builder.state(),
            PreparationState::Idle | PreparationState::Cancelled
        ) {
            return Ok(None);
        }
        let Some(mut input) = self.desired.take() else {
            return Ok(None);
        };
        self.serial += 1;
        let active = self.active.as_ref().map(|b| b.next.revision());
        let pending = self.pending.as_ref().map(|b| b.next.revision());
        let packing = self
            .packing
            .as_ref()
            .map(|(timeline, _)| timeline.snapshot().revision());
        self.contexts.retain(|revision, _| {
            Some(*revision) == active || Some(*revision) == pending || Some(*revision) == packing
        });
        self.contexts.insert(self.serial, self.desired_context);
        input.revision = self.serial;
        self.builder.begin(input).map_err(|e| e.to_string())?;
        static TICKET: std::sync::atomic::AtomicU64 = std::sync::atomic::AtomicU64::new(1);
        self.worker_ticket = TICKET.fetch_add(1, std::sync::atomic::Ordering::Relaxed);
        self.worker_sent = false;
        Ok(Some(self.serial))
    }
    /// Pure preparation can progress while the preceding GPU frame is busy.
    /// Device allocations, publication, writes and retirement stay in advance.
    fn step_cpu(&mut self) -> Result<(), String> {
        let started = bevy::platform::time::Instant::now();
        // Keep each scalar/scratch chunk bounded, but use the available CPU
        // slice even when a slow GPU lowers the browser's frame frequency.
        for chunk in 0..16 {
            let building =
                !self.remote && matches!(self.builder.state(), PreparationState::Preparing { .. });
            let packing = self.packing.as_ref().is_some_and(|(_, work)| {
                let (done, total) = work.progress();
                done < total
            });
            if building && (!packing || chunk % 2 == 0) {
                self.builder
                    .step(NonZeroUsize::new(1024).unwrap())
                    .map_err(|e| e.to_string())?;
            } else if let Some((_, work)) = &mut self.packing {
                let (done, total) = work.progress();
                if done == total {
                    break;
                }
                work.step(NonZeroUsize::new(FIELD_SIZE).unwrap())
                    .map_err(|e| e.to_string())?;
            } else {
                break;
            }
            if started.elapsed().as_secs_f64() >= 0.006 {
                break;
            }
        }
        Ok(())
    }
    fn report_ready(&mut self, profiler: Option<&crate::profiling::Profiler>) {
        if let Some(next) = self.builder.prepared() {
            let revision = next.revision();
            if self.reported_ready != revision {
                self.reported_ready = revision;
                if let Some(profiler) = profiler {
                    let _event = profiler
                        .instant("terrain.build.ready")
                        .arg(
                            "context",
                            self.contexts.get(&revision).copied().unwrap_or(0),
                        )
                        .arg("landscapeRevision", revision)
                        .arg("patches", next.patches().count());
                }
            }
        }
    }
    fn targets(
        &mut self,
        world: &mut World,
        size: UVec2,
        format: TextureFormat,
    ) -> Result<(), String> {
        if self.targets.is_none() {
            let _profile = crate::profiling::span(world, "terrain.targets.allocate")
                .arg("width", size.x)
                .arg("height", size.y);
            let (mask, warm, warm_mask, blank) = {
                let mut images = world.resource_mut::<Assets<Image>>();
                (
                    images.add(Image::new_target_texture(size.x, size.y, MASK_FORMAT, None)),
                    images.add(Image::new_target_texture(32, 32, format, None)),
                    images.add(Image::new_target_texture(32, 32, MASK_FORMAT, None)),
                    images.add(Image::new_fill(
                        Extent3d {
                            width: 1,
                            height: 1,
                            depth_or_array_layers: 1,
                        },
                        bevy::render::render_resource::TextureDimension::D2,
                        &[0, 0, 0, 255],
                        TextureFormat::Rgba8Unorm,
                        bevy::asset::RenderAssetUsages::MAIN_WORLD
                            | bevy::asset::RenderAssetUsages::RENDER_WORLD,
                    )),
                )
            };
            let mask_camera =
                spawn_camera(world, SurfaceCamera::Mask, mask.clone(), MASK, -1, None);
            self.targets = Some(Targets {
                mask,
                warm,
                warm_mask,
                blank,
                mask_camera,
            });
        }
        let target = self.targets.as_ref().unwrap();
        let image = world
            .resource::<Assets<Image>>()
            .get(&target.mask)
            .ok_or("Missing surface mask")?;
        let resized = image.width() != size.x || image.height() != size.y;
        if resized {
            let _profile = crate::profiling::span(world, "terrain.mask.resize")
                .arg("width", size.x)
                .arg("height", size.y);
            world
                .resource_mut::<Assets<Image>>()
                .get_mut(&target.mask)
                .unwrap()
                .resize(Extent3d {
                    width: size.x,
                    height: size.y,
                    depth_or_array_layers: 1,
                });
        }
        if resized {
            // A resized image receives a new GPU view. Reprepare texture bind
            // groups explicitly while preserving their shared uniform buffers.
            for batch in [&self.active, &self.pending].into_iter().flatten() {
                let mut materials = world.resource_mut::<Assets<GroundMaterial>>();
                for handle in batch.ground.iter().chain([&batch.globe]) {
                    if let Some(mut material) = materials.get_mut(handle) {
                        material.textures.surface_mask = target.mask.clone();
                    }
                }
                if let Some(mut material) = world
                    .resource_mut::<Assets<OceanMaterial>>()
                    .get_mut(&batch.ocean)
                {
                    material.textures.surface_mask = target.mask.clone();
                }
            }
        }
        Ok(())
    }
    fn stage(
        &mut self,
        world: &mut World,
        timeline: FieldTimeline,
        upload: FieldUpload,
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<(), String> {
        let next = timeline.snapshot().clone();
        let context = self.contexts.get(&next.revision()).copied().unwrap_or(0);
        let _profile = crate::profiling::span(world, "terrain.stage_gpu")
            .arg("context", context)
            .arg("landscapeRevision", next.revision())
            .arg("patches", upload.keys.len());
        let targets = self.targets.as_ref().ok_or("Missing terrain targets")?;
        let mars = crate::assets::texture(world, "textures/planets/mars/surface")
            .ok_or("Missing Mars image")?;
        let color_atlas = crate::assets::texture(world, "textures/terrain/color-array")
            .ok_or("Missing color array")?;
        let detail_atlas = crate::assets::texture(world, "textures/terrain/detail-array")
            .ok_or("Missing detail array")?;
        let globe_mesh = world
            .query_filtered::<&Mesh3d, With<AssetGlobe>>()
            .iter(world)
            .next()
            .map(|m| m.0.clone())
            .ok_or("Missing globe geometry")?;
        let capacity = upload.keys.len();
        let mesh = patch_mesh(&upload.keys, &upload.slots, next.projection())?;
        let images = field_images(upload)?;
        let fields = {
            let mut storage = world.resource_mut::<Assets<Image>>();
            images.map(|image| storage.add(image))
        };
        let mesh = world.resource_mut::<Assets<Mesh>>().add(mesh);
        let mut parameters = TerrainUniform::from_climate(
            self.climate
                .as_ref()
                .ok_or("Missing terrain climate")?
                .current(),
            Vec3::new(0., 0., 6.8),
            crate::lighting::sun_direction(),
            crate::lighting::sun_rgb(),
            crate::lighting::intensity(world),
        );
        parameters.geometry.z = capacity as f32;
        let buffers = TerrainBuffers::new(parameters, device, queue)?;
        let buffer_ids = buffers.ids();
        let textures = GroundTextures {
            highlight_data: world.resource::<super::grid::GridRuntime>().data(),
            terrain: fields[0].clone(),
            surface_fields: fields[1].clone(),
            color_atlas: color_atlas.clone(),
            detail_atlas: detail_atlas.clone(),
            mars,
            plant_shade: targets.blank.clone(),
            surface_mask: targets.mask.clone(),
            bake_albedo: targets.blank.clone(),
            bake_surface: targets.blank.clone(),
            pass: GroundPass::Opaque,
        };
        let atmosphere = world
            .resource::<crate::atmosphere::AtmosphereRuntime>()
            .binding()?;
        let mut materials = world.resource_mut::<Assets<GroundMaterial>>();
        let mut ground = Vec::new();
        for pass in [GroundPass::Opaque, GroundPass::Edge, GroundPass::Basin] {
            let mut textures = textures.clone();
            textures.pass = pass;
            ground.push(materials.add(GroundMaterial {
                textures,
                parameters: buffers.parameters(),
                highlights: buffers.highlights(),
                atmosphere: atmosphere.clone(),
            }));
        }
        let mut globe_textures = textures;
        globe_textures.pass = GroundPass::Globe;
        let globe = materials.add(GroundMaterial {
            textures: globe_textures,
            parameters: buffers.parameters(),
            highlights: buffers.highlights(),
            atmosphere: atmosphere.clone(),
        });
        let ground: [Handle<GroundMaterial>; 3] =
            ground.try_into().map_err(|_| "Ground pass count")?;
        let highlight_data = world.resource::<super::grid::GridRuntime>().data();
        let ocean = world
            .resource_mut::<Assets<OceanMaterial>>()
            .add(OceanMaterial {
                textures: OceanTextures {
                    highlight_data,
                    terrain: fields[0].clone(),
                    color_atlas,
                    detail_atlas,
                    surface_mask: targets.mask.clone(),
                },
                parameters: buffers.parameters(),
                highlights: buffers.highlights(),
                atmosphere,
            });
        let mask = world
            .resource_mut::<Assets<LakeMaskMaterial>>()
            .add(LakeMaskMaterial {
                textures: LakeMaskTextures {
                    terrain: fields[0].clone(),
                },
                parameters: buffers.parameters(),
            });
        let root = world
            .spawn((SurfaceRoot, Transform::default(), Visibility::default()))
            .id();
        let mut ground_entities = Vec::new();
        for material in &ground {
            ground_entities.push(
                world
                    .spawn((
                        Mesh3d(mesh.clone()),
                        MeshMaterial3d(material.clone()),
                        crate::transparent::TransparentOrder::TERRAIN_EDGE,
                        Transform::default(),
                        NoFrustumCulling,
                        RenderLayers::layer(WARM),
                        ChildOf(root),
                    ))
                    .id(),
            );
        }
        ground_entities.push(
            world
                .spawn((
                    Mesh3d(mesh.clone()),
                    MeshMaterial3d(ocean.clone()),
                    crate::transparent::TransparentOrder::OCEAN,
                    Transform::default(),
                    NoFrustumCulling,
                    RenderLayers::layer(WARM),
                    ChildOf(root),
                ))
                .id(),
        );
        let mask_entity = world
            .spawn((
                Mesh3d(mesh.clone()),
                MeshMaterial3d(mask.clone()),
                Transform::default(),
                NoFrustumCulling,
                RenderLayers::layer(WARM_MASK),
                ChildOf(root),
            ))
            .id();
        let warm_globe = world
            .spawn((
                Mesh3d(globe_mesh),
                MeshMaterial3d(globe.clone()),
                Transform::default(),
                NoFrustumCulling,
                RenderLayers::layer(WARM),
                ChildOf(root),
            ))
            .id();
        let warm_cameras = [
            spawn_camera(
                world,
                SurfaceCamera::Warm,
                targets.warm.clone(),
                WARM,
                -3,
                Some(root),
            ),
            spawn_camera(
                world,
                SurfaceCamera::WarmMask,
                targets.warm_mask.clone(),
                WARM_MASK,
                -2,
                Some(root),
            ),
        ];
        self.pending = Some(Batch {
            root,
            ground_entities,
            mask_entity,
            warm_globe,
            warm_cameras,
            fields,
            mesh,
            ground,
            globe,
            ocean,
            mask,
            buffers,
            buffer_ids,
            timeline,
            next,
            capacity,
            activation: None,
            frames: 0,
            context,
        });
        Ok(())
    }
    fn activate(&mut self, world: &mut World) -> Result<(), String> {
        let mut batch = self.pending.take().ok_or("Missing pending terrain")?;
        let step = self.clock.step();
        batch.activation = Some(step);
        batch.timeline.activate(
            step,
            world.resource::<crate::scene::PresentationState>().settling,
        );
        for entity in &batch.ground_entities {
            world.entity_mut(*entity).insert(RenderLayers::default());
        }
        world
            .entity_mut(batch.mask_entity)
            .insert(RenderLayers::layer(MASK));
        world.despawn(batch.warm_globe);
        let globe = world
            .query_filtered::<Entity, With<AssetGlobe>>()
            .iter(world)
            .next()
            .ok_or("Missing globe")?;
        world
            .entity_mut(globe)
            .remove::<MeshMaterial3d<StandardMaterial>>()
            .insert(MeshMaterial3d(batch.globe.clone()));
        for entity in batch.warm_cameras {
            world.despawn(entity);
        }
        let mut terrain = world.resource_mut::<TerrainState>();
        terrain.contact = Some(Contact {
            next: batch.next.clone(),
            timeline: batch.timeline.clone(),
            step,
        });
        terrain.revision += 1;
        if let Some(old) = self.active.take() {
            self.retired.push(old.release(world));
        }
        let _event = crate::profiling::instant(world, "terrain.publish")
            .arg("context", batch.context)
            .arg("stage", "render_submission")
            .arg("landscapeRevision", batch.next.revision())
            .arg("patches", batch.capacity);
        self.active = Some(batch);
        Ok(())
    }
    fn bake_source(&self, world: &World) -> Result<Option<super::bake::BakeSource>, String> {
        // The published batch still owns an unchanged surface while its
        // replacement is being prepared. Keep its valid cache available until
        // activation; otherwise slow live shading also starves CPU preparation.
        if self.transitioning()
            || self.climate.as_ref().is_none_or(|climate| {
                climate.current().ground_channels() != climate.target().ground_channels()
            })
        {
            return Ok(None);
        }
        let Some(batch) = &self.active else {
            return Ok(None);
        };
        let mut keys = batch.next.patches().map(|patch| patch.key());
        let Some(first) = keys.next() else {
            return Ok(None);
        };
        let (mut min, mut max) = (first, first);
        for key in keys {
            for i in 0..2 {
                min[i] = min[i].min(key[i]);
                max[i] = max[i].max(key[i]);
            }
        }
        let textures = world
            .resource::<Assets<GroundMaterial>>()
            .get(&batch.ground[0])
            .ok_or("Missing bake source material")?
            .textures
            .clone();
        Ok(Some(super::bake::BakeSource {
            batch: batch.next.revision(),
            mesh: batch.mesh.clone(),
            textures,
            parameters: {
                let mut parameters = batch.parameters(
                    self.climate.as_ref().unwrap(),
                    self.clock.step(),
                    world.resource::<crate::world::WorldView>().eye(),
                    crate::lighting::intensity(world),
                );
                if let Some((_, bounds, _)) = world
                    .resource::<crate::vegetation::FoliageRuntime>()
                    .shade()
                {
                    parameters.shade_bounds = bounds;
                }
                parameters
            },
            bounds: [
                BoardPoint {
                    x: f64::from(min[0]) * PATCH_SIZE,
                    y: f64::from(min[1]) * PATCH_SIZE,
                },
                BoardPoint {
                    x: (f64::from(max[0]) + 1.) * PATCH_SIZE,
                    y: (f64::from(max[1]) + 1.) * PATCH_SIZE,
                },
            ],
            shade_revision: world
                .resource::<crate::vegetation::FoliageRuntime>()
                .shade()
                .map_or(0, |v| v.2),
        }))
    }
    fn write_uniforms(
        &mut self,
        world: &World,
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<(), String> {
        let camera = world.resource::<crate::world::WorldView>().eye();
        let step = self.clock.step();
        for batch in [&mut self.active, &mut self.pending].into_iter().flatten() {
            let mut parameters = batch.parameters(
                self.climate.as_ref().unwrap(),
                step,
                camera,
                crate::lighting::intensity(world),
            );
            parameters.camera.w = self.motion_clock.seconds() as f32;
            world
                .resource::<super::bake::BakeRuntime>()
                .configure(batch.next.revision(), &mut parameters);
            if let Some((_, bounds, _)) = world
                .resource::<crate::vegetation::FoliageRuntime>()
                .shade()
            {
                parameters.shade_bounds = bounds;
            }
            let highlights = world.resource::<super::grid::GridRuntime>().uniform();
            batch.buffers.write(parameters, highlights, device, queue)?;
            if batch.buffer_ids != batch.buffers.ids() {
                return Err("Terrain uniform storage changed during a frame".into());
            }
            self.uniform_writes += 1;
        }
        Ok(())
    }
    fn advance(
        &mut self,
        world: &mut World,
        uploaded: bool,
        size: UVec2,
        format: TextureFormat,
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<(), String> {
        for retired in self.retired.drain(..) {
            retired.retire();
            self.retired_uniforms += 2;
        }
        let Some(scene) = world.resource::<SceneState>().0.as_ref() else {
            return Ok(());
        };
        let next_scene = (self.seen.as_ref() != Some(scene)).then(|| scene.clone());
        let relief = world.resource::<TerrainState>().relief.clone();
        if relief.is_none()
            || world
                .query_filtered::<Entity, With<AssetGlobe>>()
                .iter(world)
                .next()
                .is_none()
        {
            return Ok(());
        }
        if self.targets.is_some() {
            self.targets(world, size, format)?;
        }
        let step = self.clock.step();
        let settling = world.resource::<crate::scene::PresentationState>().settling;
        if let Some(scene) = next_scene {
            let parameters = ClimateParameters {
                temperature: f64::from(scene.temperature),
                oxygen: f64::from(scene.oxygen),
                oceans: f64::from(scene.oceans),
                max_oceans: f64::from(scene.max_oceans),
            };
            if let Some(climate) = &mut self.climate {
                climate
                    .retarget(parameters, step)
                    .map_err(|e| e.to_string())?;
            } else {
                self.climate =
                    Some(ClimateRuntime::new(parameters, step).map_err(|e| e.to_string())?);
            }
            let input = scene
                .landscape_input(relief, self.serial + 1)
                .map_err(|e| e.to_string())?;
            if self.sources.as_deref() != Some(input.sources()) {
                let sources = input.sources().len();
                self.desired_context = world
                    .get_resource::<crate::profiling::Profiler>()
                    .map_or(0, |p| p.context());
                let (additive, obsolete) = self.queue_input(input);
                let _event = crate::profiling::instant(world, "terrain.request")
                    .arg("landscapeRevision", self.serial + 1)
                    .arg("sources", sources)
                    .arg("additive", additive);
                if let Some(old) = obsolete {
                    self.retired.push(old.release(world));
                }
            }
            self.seen = Some(scene);
        }
        if settling && let Some(climate) = &mut self.climate {
            climate
                .snap(climate.parameters(), step)
                .map_err(|e| e.to_string())?;
        }
        self.climate
            .as_mut()
            .ok_or("Missing climate")?
            .advance(step)
            .map_err(|e| e.to_string())?;
        let impact_wait = self.pending.as_ref().is_some_and(|p| {
            p.next.sources().iter().any(|source| {
                source.kind == openmars_scene::landscape::LandscapeKind::Excluded
                    && world
                        .resource::<crate::nuclear::timeline::Timeline>()
                        .blocks_field(
                            (
                                source.coordinate[0],
                                source.coordinate[1],
                                source.coordinate[2],
                            ),
                            step,
                            world.resource::<crate::scene::MotionPreference>().0,
                        )
            })
        });
        if uploaded && self.pending.is_some() && !impact_wait {
            let batch = self.pending.as_ref().unwrap();
            let _event = crate::profiling::instant(world, "terrain.upload.ready")
                .arg("context", batch.context)
                .arg("landscapeRevision", batch.next.revision());
            self.activate(world)?;
        }
        if let Some(batch) = &mut self.active {
            let mut terrain = world.resource_mut::<TerrainState>();
            if batch.timeline.settle(step, settling) {
                terrain.revision += 1;
            }
            if let Some(contact) = &mut terrain.bypass_change_detection().contact {
                contact.timeline.settle(step, settling);
                contact.step = step;
            }
        }
        if let Some(revision) = self.begin_queued()? {
            let _event = crate::profiling::instant(world, "terrain.build.begin")
                .arg(
                    "context",
                    self.contexts.get(&revision).copied().unwrap_or(0),
                )
                .arg("landscapeRevision", revision);
        }
        self.report_ready(world.get_resource::<crate::profiling::Profiler>());
        // CPU work is allowed alongside uploads and earlier animations. A
        // completed successor only waits for its own overlapping patch clocks.
        let can_pack = self.pending.is_none()
            && self.packing.is_none()
            && self.builder.prepared().is_some_and(|next| {
                settling
                    || self
                        .active
                        .as_ref()
                        .is_none_or(|old| old.timeline.can_replace(next, step))
            });
        if !can_pack
            && let Some(next) = self.builder.prepared()
            && self.reported_wait != next.revision()
        {
            self.reported_wait = next.revision();
            let _event = crate::profiling::instant(world, "terrain.publication.wait")
                .arg(
                    "context",
                    self.contexts.get(&next.revision()).copied().unwrap_or(0),
                )
                .arg("landscapeRevision", next.revision())
                .arg(
                    "reason",
                    if self.pending.is_some() || self.packing.is_some() {
                        "upload in progress"
                    } else {
                        "overlapping region"
                    },
                );
        }
        if can_pack {
            let next = self.builder.take_ready().unwrap();
            let _event = crate::profiling::instant(world, "terrain.pack.begin")
                .arg(
                    "context",
                    self.contexts.get(&next.revision()).copied().unwrap_or(0),
                )
                .arg("landscapeRevision", next.revision());
            self.targets(world, size, format)?;
            let previous = (!settling)
                .then(|| self.active.as_ref().map(|b| &b.timeline))
                .flatten();
            let timeline =
                FieldTimeline::prepare(previous, next, step).map_err(|e| e.to_string())?;
            if let Some(work) = FieldUploadBuilder::new(
                timeline.clone(),
                device.limits().max_texture_array_layers,
                MAX_UPLOAD_BYTES,
            )
            .map_err(|e| e.to_string())?
            {
                self.packing = Some((timeline, work));
            } else {
                // No surface patches: only the globe is visible. The dummy
                // field binds both transition sides and warms the same passes.
                let cells = FIELD_SIZE * FIELD_SIZE;
                let terrain = [0xea, 0x0c, 0x66, 0x32, 0, 0, 0, 0].repeat(cells);
                let surface = vec![0; cells * 8];
                self.stage(
                    world,
                    timeline,
                    FieldUpload {
                        keys: vec![[0, 0]],
                        slots: vec![FieldSlot::settled(0)],
                        terrain,
                        surface,
                    },
                    device,
                    queue,
                )?;
            }
        }
        if let Some((_, work)) = &mut self.packing {
            let (done, total) = work.progress();
            if done == total {
                let (timeline, work) = self.packing.take().unwrap();
                let _event = crate::profiling::instant(world, "terrain.pack.ready")
                    .arg(
                        "context",
                        self.contexts
                            .get(&timeline.snapshot().revision())
                            .copied()
                            .unwrap_or(0),
                    )
                    .arg("landscapeRevision", timeline.snapshot().revision());
                self.targets(world, size, format)?;
                self.stage(
                    world,
                    timeline,
                    work.finish().map_err(|e| e.to_string())?,
                    device,
                    queue,
                )?;
            }
        }
        if let Some(pending) = &mut self.pending {
            pending.frames += 1;
        }
        Ok(())
    }
}

pub fn reset(world: &mut World) {
    world.insert_resource(crate::entry::WorldEntrance::default());
    crate::labels::scene::reset(world);
    crate::offworld::scene::reset(world);
    crate::atmosphere::reset(world);
    crate::asteroid::reset(world);
    crate::structures::sites::reset(world);
    crate::structures::special::reset(world);
    crate::particles::reset(world);
    crate::nuclear::reset(world);
    crate::structures::world_tree::reset(world);
    crate::fauna::reset(world);
    crate::structures::scene::reset(world);
    world.insert_resource(crate::structures::texture_library::TextureLibrary::default());
    crate::vegetation::reset(world);
    super::grid::reset(world);
    let Some(mut runtime) = world.remove_resource::<SurfaceRuntime>() else {
        super::bake::reset(world);
        return;
    };
    for batch in [runtime.active.take(), runtime.pending.take()]
        .into_iter()
        .flatten()
    {
        runtime.retired.push(batch.release(world));
    }
    if let Some(targets) = runtime.targets.take() {
        world.despawn(targets.mask_camera);
    }
    // Future ground consumers are removed before their direct cache storage retires.
    super::bake::reset(world);
    let retired = std::mem::take(&mut runtime.retired);
    let uniform_writes = runtime.uniform_writes;
    let retired_uniforms = runtime.retired_uniforms;
    world.insert_resource(SurfaceRuntime {
        retired,
        uniform_writes,
        retired_uniforms,
        ..default()
    });
}

/// Only published landscape/mask draws follow the Mars entry gate. Pending
/// color/mask warm passes must remain visible so hidden loading can complete.
pub fn apply_entry(world: &mut World) {
    let visible = world
        .resource::<crate::entry::WorldEntrance>()
        .tiles
        .shown();
    let entities: Vec<_> = world
        .resource::<SurfaceRuntime>()
        .active
        .as_ref()
        .map(|batch| {
            batch
                .ground_entities
                .iter()
                .copied()
                .chain([batch.mask_entity])
                .collect()
        })
        .unwrap_or_default();
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

fn uploaded_batch(batch: &Batch, targets: &Targets, render: &World) -> bool {
    let Some(images) = render.get_resource::<RenderAssets<GpuImage>>() else {
        return false;
    };
    let Some(meshes) = render.get_resource::<RenderAssets<RenderMesh>>() else {
        return false;
    };
    let Some(materials) = render.get_resource::<ErasedRenderAssets<PreparedMaterial>>() else {
        return false;
    };
    let pipelines = render.resource::<PipelineCache>();
    batch
        .fields
        .iter()
        .chain([
            &targets.mask,
            &targets.warm,
            &targets.warm_mask,
            &targets.blank,
        ])
        .all(|h| images.get(h.id()).is_some())
        && meshes.get(batch.mesh.id()).is_some()
        && batch
            .material_ids()
            .iter()
            .all(|id| materials.get(*id).is_some())
        && pipelines.waiting_pipelines().next().is_none()
}

pub fn prepare(app: &mut App, size: UVec2, format: TextureFormat) -> Result<(), String> {
    let _profile = crate::profiling::span(app.world(), "terrain.prepare");
    crate::atmosphere::begin(app)?;
    crate::menu::prepare(app, format)?;
    crate::space::scene::prepare(app, format)?;
    crate::offworld::scene::prepare(app)?;
    let device = app.world().resource::<RenderDevice>().clone();
    let queue = app.world().resource::<RenderQueue>().clone();
    let step = app.world().resource::<SurfaceRuntime>().step();
    super::grid::prepare(app, step, &device, &queue)?;
    // A map extent replacement changes the texture once; animation writes its
    // existing GPU storage and never dirties the materials.
    let highlight_data = app.world().resource::<super::grid::GridRuntime>().data();
    let world = app.world_mut();
    let ground: Vec<_> = world
        .resource::<Assets<GroundMaterial>>()
        .iter()
        .filter_map(|(id, material)| {
            (material.textures.highlight_data != highlight_data).then_some(id)
        })
        .collect();
    for id in ground {
        world
            .resource_mut::<Assets<GroundMaterial>>()
            .get_mut(id)
            .unwrap()
            .textures
            .highlight_data = highlight_data.clone();
    }
    let ocean: Vec<_> = world
        .resource::<Assets<OceanMaterial>>()
        .iter()
        .filter_map(|(id, material)| {
            (material.textures.highlight_data != highlight_data).then_some(id)
        })
        .collect();
    for id in ocean {
        world
            .resource_mut::<Assets<OceanMaterial>>()
            .get_mut(id)
            .unwrap()
            .textures
            .highlight_data = highlight_data.clone();
    }
    let uploaded = app
        .world()
        .resource::<SurfaceRuntime>()
        .pending
        .as_ref()
        .is_some_and(|batch| {
            app.world()
                .resource::<SurfaceRuntime>()
                .targets
                .as_ref()
                .is_some_and(|targets| {
                    uploaded_batch(batch, targets, app.sub_app(RenderApp).world())
                })
        });
    let mut runtime = app
        .world_mut()
        .remove_resource::<SurfaceRuntime>()
        .ok_or("Missing terrain runtime")?;
    let result = (|| {
        runtime.advance(app.world_mut(), uploaded, size, format, &device, &queue)?;
        let nuclear = runtime
            .pending
            .as_ref()
            .or(runtime.active.as_ref())
            .map(|batch| crate::nuclear::Input {
                snapshot: batch.next.clone(),
                landscape_ready: runtime.pending.as_ref().is_none_or(|pending| {
                    runtime.targets.as_ref().is_some_and(|targets| {
                        uploaded_batch(pending, targets, app.sub_app(RenderApp).world())
                    })
                }),
                step: runtime.clock.step(),
                motion_step: runtime.motion_clock.step(),
                reduced: app.world().resource::<crate::scene::MotionPreference>().0,
                warm: runtime.targets.as_ref().unwrap().warm.clone(),
                warm_mask: runtime.targets.as_ref().unwrap().warm_mask.clone(),
                climate: runtime.climate.as_ref().unwrap().current(),
            });
        crate::nuclear::blast::prepare_queued(app, nuclear.as_ref())?;
        crate::nuclear::prepare_queued(app, nuclear, &device, &queue)?;
        let input = runtime
            .active
            .as_ref()
            .map(|batch| crate::vegetation::Input {
                context: batch.context,
                snapshot: batch.next.clone(),
                current: runtime.climate.as_ref().unwrap().current(),
                reduced: app.world().resource::<crate::scene::MotionPreference>().0,
                settled: app
                    .world()
                    .resource::<crate::scene::PresentationState>()
                    .settling,
                target: runtime.climate.as_ref().unwrap().target(),
                step: runtime.clock.step(),
                warm: runtime.targets.as_ref().unwrap().warm.clone(),
            });
        crate::structures::texture_library::prepare_queued(app, &device, &queue)?;
        crate::vegetation::prepare_queued(app, input, &device, &queue)?;
        let structures = runtime
            .active
            .as_ref()
            .map(|batch| crate::structures::scene::Input {
                snapshot: batch.next.clone(),
                step: runtime.clock.step(),
                frost: runtime.climate.as_ref().unwrap().current().frost as f32,
                reduced: app.world().resource::<crate::scene::MotionPreference>().0,
                settled: app
                    .world()
                    .resource::<crate::scene::PresentationState>()
                    .settling,
                warm: runtime.targets.as_ref().unwrap().warm.clone(),
            });
        crate::structures::scene::prepare_queued(app, structures, &device, &queue)?;
        let fauna = runtime.active.as_ref().and_then(|batch| {
            app.world()
                .resource::<SceneState>()
                .0
                .as_ref()
                .map(|scene| crate::fauna::Input {
                    flocks: scene
                        .tiles
                        .iter()
                        .filter(|t| {
                            t.location == "mars"
                                && t.occupant
                                    .as_deref()
                                    .is_some_and(crate::fauna::is_bird_tile)
                        })
                        .map(|t| (t.q, t.r, t.s))
                        .collect(),
                    projection: batch.next.projection(),
                    step: runtime.motion_clock.step(),
                    density: runtime.climate.as_ref().unwrap().current().birds,
                    warm: runtime.targets.as_ref().unwrap().warm.clone(),
                })
        });
        crate::fauna::prepare_queued(app, fauna)?;
        let trees = runtime.active.as_ref().and_then(|batch| {
            app.world()
                .resource::<SceneState>()
                .0
                .as_ref()
                .map(|scene| {
                    let mut trees = std::collections::BTreeMap::new();
                    let mut nuclear = std::collections::BTreeSet::new();
                    for tile in scene.tiles.iter().filter(|t| t.location == "mars") {
                        let c = (tile.q, tile.r, tile.s);
                        if tile.occupant.as_deref() == Some("nuclear-zone-tile") {
                            nuclear.insert(c);
                        }
                        if tile.occupant.as_deref() == Some("world-tree-tile")
                            && batch
                                .next
                                .sources()
                                .iter()
                                .any(|s| s.coordinate == [tile.q, tile.r, tile.s])
                        {
                            trees.insert(
                                c,
                                tile.visual.as_ref().map_or_else(
                                    || {
                                        openmars_scene::landscape::visual_seed(&format!(
                                            "{}:{}",
                                            scene.game_id, tile.id
                                        ))
                                    },
                                    |v| v.seed,
                                ),
                            );
                        }
                    }
                    crate::structures::world_tree::Input {
                        trees,
                        nuclear,
                        projection: batch.next.projection(),
                        step: runtime.clock.step(),
                        motion_step: runtime.motion_clock.step(),
                        reduced: app.world().resource::<crate::scene::MotionPreference>().0,
                        settled: app
                            .world()
                            .resource::<crate::scene::PresentationState>()
                            .settling,
                        warm: runtime.targets.as_ref().unwrap().warm.clone(),
                    }
                })
        });
        crate::structures::world_tree::prepare_queued(app, trees, &device, &queue)?;
        let sites = runtime
            .active
            .as_ref()
            .map(|batch| crate::structures::sites::Input {
                snapshot: batch.next.clone(),
                step: runtime.clock.step(),
                motion_step: runtime.motion_clock.step(),
                reduced: app.world().resource::<crate::scene::MotionPreference>().0,
                settled: app
                    .world()
                    .resource::<crate::scene::PresentationState>()
                    .settling,
                warm: runtime.targets.as_ref().unwrap().warm.clone(),
            });
        crate::structures::sites::prepare_queued(app, sites, &device, &queue)?;
        let special = runtime
            .active
            .as_ref()
            .map(|batch| crate::structures::special::Input {
                snapshot: batch.next.clone(),
                step: runtime.clock.step(),
                motion_step: runtime.motion_clock.step(),
                reduced: app.world().resource::<crate::scene::MotionPreference>().0,
                settled: app
                    .world()
                    .resource::<crate::scene::PresentationState>()
                    .settling,
                warm: runtime.targets.as_ref().unwrap().warm.clone(),
                warm_mask: runtime.targets.as_ref().unwrap().warm_mask.clone(),
                climate: runtime.climate.as_ref().unwrap().current(),
            });
        crate::structures::special::prepare_queued(app, special, &device, &queue)?;
        let particles = runtime
            .active
            .as_ref()
            .map(|batch| crate::particles::Input {
                snapshot: batch.next.clone(),
                step: runtime.clock.step(),
                motion_step: runtime.motion_clock.step(),
                reduced: app.world().resource::<crate::scene::MotionPreference>().0,
                settled: app
                    .world()
                    .resource::<crate::scene::PresentationState>()
                    .settling,
                warm: runtime.targets.as_ref().unwrap().warm.clone(),
            });
        crate::particles::prepare_queued(app, particles, &device, &queue)?;
        let asteroid = runtime.active.as_ref().map(|batch| crate::asteroid::Input {
            step: runtime.clock.step(),
            radius: batch.next.projection().radius(),
            reduced: app.world().resource::<crate::scene::MotionPreference>().0,
            quiet: app
                .world()
                .resource::<crate::scene::PresentationState>()
                .settling,
            warm: runtime.targets.as_ref().unwrap().warm.clone(),
        });
        crate::asteroid::prepare_queued(app, asteroid)?;
        let atmosphere = runtime
            .pending
            .as_ref()
            .or(runtime.active.as_ref())
            .and_then(|batch| {
                let (camera, _, fov) =
                    crate::world::framed_camera(app.world().resource::<crate::world::WorldView>())?;
                let climate = runtime.climate.as_ref()?;
                let profile = openmars_scene::atmosphere::mars_profile(
                    climate.current().sky,
                    climate.pulses().warm,
                )
                .ok()?;
                let celestial = app
                    .world()
                    .resource::<crate::space::scene::CelestialRuntime>();
                let mut bodies = vec![crate::atmosphere::BodySpec {
                    id: "mars".into(),
                    body: openmars_scene::atmosphere::Body {
                        center: [0.; 3],
                        radius: batch.next.projection().radius(),
                        profile,
                        follows_mesh: false,
                    },
                    rotation: Quat::IDENTITY,
                    hull: None,
                }];
                bodies.extend(celestial.atmosphere());
                Some(crate::atmosphere::Input {
                    bodies,
                    // Source Sun at the origin, Mars at orbit angle270/radius250.
                    // The orbital rig will supply moving body/light transforms.
                    sun: Vec3::new(0., 0., 250.),
                    sun_color: Vec3::new(1., 0.93, 0.85),
                    intensity: f64::from(crate::lighting::intensity(app.world())),
                    ambient: crate::lighting::ambient_radiance(),
                    sky_rotation: celestial.sky_rotation(),
                    sky_brightness: app
                        .world()
                        .resource::<crate::graphics::GraphicsState>()
                        .0
                        .background_brightness,
                    seconds: runtime.motion_clock.step() as f32 / 60.,
                    camera,
                    projection_y: 1. / (f64::from(fov) * 0.5).tan(),
                    height: f64::from(app.world().resource::<crate::world::WorldView>().size.y),
                    warm: runtime.targets.as_ref()?.warm.clone(),
                })
            });
        crate::atmosphere::prepare_queued(
            app,
            atmosphere.or_else(|| crate::menu::atmosphere_input(app)),
        )?;

        if let Some((texture, _, _)) = app
            .world()
            .resource::<crate::vegetation::FoliageRuntime>()
            .shade()
        {
            let handles: Vec<_> = [&runtime.active, &runtime.pending]
                .into_iter()
                .flatten()
                .flat_map(|b| b.ground.iter().chain([&b.globe]))
                .cloned()
                .collect();
            let mut materials = app.world_mut().resource_mut::<Assets<GroundMaterial>>();
            for handle in handles {
                if let Some(material) = materials.get(&handle)
                    && material.textures.plant_shade != texture
                {
                    materials.get_mut(&handle).unwrap().textures.plant_shade = texture.clone();
                }
            }
        }
        let source = if app
            .world()
            .resource::<crate::vegetation::FoliageRuntime>()
            .ready()
            && app
                .world()
                .resource::<crate::structures::texture_library::TextureLibrary>()
                .ready()
        {
            if app
                .world()
                .resource::<crate::structures::scene::StructureRuntime>()
                .ready()
            {
                runtime.bake_source(app.world())?
            } else {
                None
            }
        } else {
            None
        };
        let warm = runtime
            .pending
            .as_ref()
            .or(runtime.active.as_ref())
            .zip(runtime.climate.as_ref())
            .map(|(batch, climate)| {
                (
                    batch.mesh.clone(),
                    batch.parameters(
                        climate,
                        runtime.clock.step(),
                        app.world().resource::<crate::world::WorldView>().eye(),
                        crate::lighting::intensity(app.world()),
                    ),
                )
            });
        super::bake::prepare_queued(app, source, warm, &device, &queue)?;
        runtime.write_uniforms(app.world(), &device, &queue)
    })();
    app.world_mut().insert_resource(runtime);
    result
}

type SurfaceCameras<'w, 's> = Query<
    'w,
    's,
    (
        &'static mut Transform,
        &'static mut Projection,
        &'static mut Camera,
    ),
    (With<SurfaceCamera>, Without<AssetCamera>),
>;
pub(crate) fn cameras(
    main: Query<(&Transform, &Projection, &Camera), With<AssetCamera>>,
    mut targets: SurfaceCameras,
) {
    let Some((transform, projection, camera)) = main.iter().next() else {
        return;
    };
    for (mut target_transform, mut target_projection, mut target_camera) in &mut targets {
        *target_transform = *transform;
        *target_projection = projection.clone();
        target_camera.sub_camera_view = camera.sub_camera_view;
    }
}

pub fn advance_clock(world: &mut World, delta: f64) -> Result<(), String> {
    let mut profile = crate::profiling::span(world, "terrain.clock_and_cpu");
    let reduced = world.resource::<crate::scene::MotionPreference>().0;
    let mut runtime = world.resource_mut::<SurfaceRuntime>();
    profile.set_arg("revision", runtime.serial);
    runtime.clock.advance(delta).map_err(str::to_string)?;
    if !reduced {
        runtime
            .motion_clock
            .advance(delta)
            .map_err(str::to_string)?;
    }
    runtime.step_cpu()
}

/// Bounded pure work may run between animation frames. It never advances time,
/// submits GPU work, changes the published field or retires device resources.
pub fn advance_cpu(world: &mut World) -> Result<bool, String> {
    let mut profile = crate::profiling::span(world, "terrain.cpu");
    let profiler = world.get_resource::<crate::profiling::Profiler>().cloned();
    let mut runtime = world.resource_mut::<SurfaceRuntime>();
    if profile.enabled() {
        profile.set_arg("revision", runtime.serial);
        if let PreparationState::Preparing { completed, total } = runtime.builder.state() {
            profile.set_arg("samples_before", completed);
            profile.set_arg("samples_total", total);
        }
        if let Some((_, work)) = &runtime.packing {
            let (rows, total) = work.progress();
            profile.set_arg("rows_before", rows);
            profile.set_arg("rows_total", total);
        }
    }
    runtime.step_cpu()?;
    runtime.report_ready(profiler.as_ref());
    if profile.enabled() {
        if let PreparationState::Preparing { completed, .. } = runtime.builder.state() {
            profile.set_arg("samples_after", completed);
        }
        if let Some((_, work)) = &runtime.packing {
            profile.set_arg("rows_after", work.progress().0);
        }
    }
    Ok(
        (!runtime.remote && matches!(runtime.builder.state(), PreparationState::Preparing { .. }))
            || runtime.packing.as_ref().is_some_and(|(_, work)| {
                let (done, total) = work.progress();
                done < total
            }),
    )
}

/// Worker ownership is identified across scene resets, not merely by a reused revision.
pub fn worker_request(world: &mut World) -> Result<Vec<u8>, String> {
    let profiler = world.get_resource::<crate::profiling::Profiler>().cloned();
    let mut runtime = world.resource_mut::<SurfaceRuntime>();
    if !runtime.remote
        || runtime.worker_sent
        || !matches!(runtime.builder.state(), PreparationState::Preparing { .. })
    {
        return Ok(vec![]);
    }
    let _profile = profiler.as_ref().map(|p| {
        p.span("terrain.worker.request")
            .arg(
                "context",
                runtime.contexts.get(&runtime.serial).copied().unwrap_or(0),
            )
            .arg("landscapeRevision", runtime.serial)
    });
    let relief = runtime.builder.pending_relief_hash();
    let bytes = runtime.builder.worker_request(
        runtime.worker_ticket,
        runtime.contexts.get(&runtime.serial).copied().unwrap_or(0),
        relief != runtime.worker_relief,
    )?;
    runtime.worker_relief = relief;
    runtime.worker_sent = true;
    Ok(bytes)
}
pub fn accept_worker(world: &mut World, bytes: &[u8]) -> Result<bool, String> {
    let profiler = world.get_resource::<crate::profiling::Profiler>().cloned();
    let settling = world.resource::<crate::scene::PresentationState>().settling;
    let mut runtime = world
        .remove_resource::<SurfaceRuntime>()
        .ok_or("Missing terrain runtime")?;
    let _profile = profiler.as_ref().map(|p| {
        p.span("terrain.worker.receive")
            .arg(
                "context",
                runtime.contexts.get(&runtime.serial).copied().unwrap_or(0),
            )
            .arg("landscapeRevision", runtime.serial)
    });
    let result = (|| {
        if !runtime
            .builder
            .accept_worker(runtime.worker_ticket, bytes)?
        {
            return Ok(false);
        }
        runtime.report_ready(profiler.as_ref());
        let step = runtime.clock.step();
        let can_pack = runtime.pending.is_none()
            && runtime.packing.is_none()
            && runtime.builder.prepared().is_some_and(|next| {
                next.patches().next().is_some()
                    && (settling
                        || runtime
                            .active
                            .as_ref()
                            .is_none_or(|old| old.timeline.can_replace(next, step)))
            });
        if can_pack {
            let next = runtime.builder.take_ready().unwrap();
            if let Some(profiler) = &profiler {
                let _event = profiler
                    .instant("terrain.pack.begin")
                    .arg(
                        "context",
                        runtime.contexts.get(&next.revision()).copied().unwrap_or(0),
                    )
                    .arg("landscapeRevision", next.revision());
            }
            let previous = if settling {
                None
            } else {
                runtime.active.as_ref().map(|b| &b.timeline)
            };
            let timeline =
                FieldTimeline::prepare(previous, next, step).map_err(|e| e.to_string())?;
            let max_layers = world
                .resource::<RenderDevice>()
                .limits()
                .max_texture_array_layers;
            if let Some(work) =
                FieldUploadBuilder::new(timeline.clone(), max_layers, MAX_UPLOAD_BYTES)
                    .map_err(|e| e.to_string())?
            {
                runtime.packing = Some((timeline, work));
            }
            runtime.step_cpu()?;
        }
        Ok(true)
    })();
    world.insert_resource(runtime);
    result
}

fn spawn_camera(
    world: &mut World,
    kind: SurfaceCamera,
    target: Handle<Image>,
    layer: usize,
    order: isize,
    parent: Option<Entity>,
) -> Entity {
    let mut camera = world.spawn((
        SurfaceRoot,
        kind,
        Camera3d::default(),
        Camera {
            order,
            clear_color: ClearColorConfig::Custom(Color::BLACK),
            ..default()
        },
        RenderTarget::Image(target.into()),
        Msaa::Off,
        Tonemapping::None,
        DebandDither::Disabled,
        RenderLayers::layer(layer),
        Transform::from_xyz(0., 0., 6.8).looking_at(Vec3::ZERO, Vec3::Y),
    ));
    if layer == WARM {
        camera.insert(bevy::core_pipeline::prepass::DepthPrepass);
    }
    if layer != WARM {
        camera.insert(CompositingSpace::Linear);
    }
    if let Some(parent) = parent {
        camera.insert(ChildOf(parent));
    }
    camera.id()
}

#[cfg(test)]
mod tests {
    use super::*;
    use openmars_scene::{
        geometry::BodyProjection,
        landscape::{BodyId, LandscapeKind},
    };
    fn queued_input(cells: &[(i32, LandscapeKind, u32)]) -> LandscapeInput {
        LandscapeInput::new(
            BodyId::Mars,
            BodyProjection::MARS,
            1,
            73,
            None,
            cells
                .iter()
                .map(|&(q, kind, seed)| LandscapeSource {
                    coordinate: [q, 0, -q],
                    kind,
                    seed,
                    layout: None,
                })
                .collect(),
        )
        .unwrap()
    }
    #[test]
    fn additions_finish_in_flight_work_and_keep_only_the_latest_successor() {
        let mut runtime = SurfaceRuntime::default();
        let mut sources = vec![(0, LandscapeKind::Greenery, 3)];
        let first = queued_input(&sources);
        runtime.sources = Some(first.sources().to_vec());
        runtime.builder.begin(first).unwrap();
        let mut completed = 0;
        for q in 1..=4 {
            runtime
                .builder
                .step(NonZeroUsize::new(32).unwrap())
                .unwrap();
            let progress = runtime.builder.state();
            let PreparationState::Preparing {
                completed: next, ..
            } = progress
            else {
                panic!("Expected incremental work");
            };
            assert!(next >= completed);
            completed = next;
            sources.push((q, LandscapeKind::Greenery, q as u32));
            let (additive, obsolete) = runtime.queue_input(queued_input(&sources));
            assert!(additive);
            assert!(obsolete.is_none());
            assert_eq!(runtime.builder.state(), progress);
            assert_eq!(
                runtime.desired.as_ref().unwrap().sources().len(),
                sources.len()
            );
        }
        while matches!(runtime.builder.state(), PreparationState::Preparing { .. }) {
            runtime
                .builder
                .step(NonZeroUsize::new(1024).unwrap())
                .unwrap();
        }
        assert_eq!(runtime.begin_queued().unwrap(), None);
        assert_eq!(runtime.desired.as_ref().unwrap().sources().len(), 5);
        assert_eq!(runtime.builder.take_ready().unwrap().sources().len(), 1);
        assert!(runtime.begin_queued().unwrap().is_some());
        while matches!(runtime.builder.state(), PreparationState::Preparing { .. }) {
            runtime
                .builder
                .step(NonZeroUsize::new(1024).unwrap())
                .unwrap();
        }
        assert_eq!(runtime.builder.take_ready().unwrap().sources().len(), 5);
    }
    #[test]
    fn removals_replacements_and_exclusions_cancel_obsolete_work() {
        for next in [
            vec![],
            vec![(0, LandscapeKind::Greenery, 4)],
            vec![(0, LandscapeKind::Ocean, 3)],
            vec![
                (0, LandscapeKind::Greenery, 3),
                (1, LandscapeKind::Excluded, 0),
            ],
        ] {
            let mut runtime = SurfaceRuntime::default();
            let first = queued_input(&[(0, LandscapeKind::Greenery, 3)]);
            runtime.sources = Some(first.sources().to_vec());
            runtime.builder.begin(first).unwrap();
            runtime
                .builder
                .step(NonZeroUsize::new(32).unwrap())
                .unwrap();
            let (additive, _) = runtime.queue_input(queued_input(&next));
            assert!(!additive);
            assert_eq!(runtime.builder.state(), PreparationState::Cancelled);
            assert_eq!(
                runtime.desired.as_ref().unwrap().sources().len(),
                next.len()
            );
        }
    }
    fn snapshot(ocean: bool, revision: u64) -> Arc<LandscapeSnapshot> {
        let mut builder = LandscapeBuilder::new();
        let sources = if ocean {
            vec![LandscapeSource {
                coordinate: [0, 0, 0],
                kind: LandscapeKind::Ocean,
                seed: 3,
                layout: None,
            }]
        } else {
            vec![]
        };
        builder
            .begin(
                LandscapeInput::new(
                    BodyId::Mars,
                    BodyProjection::MARS,
                    revision,
                    73,
                    None,
                    sources,
                )
                .unwrap(),
            )
            .unwrap();
        while matches!(builder.state(), PreparationState::Preparing { .. }) {
            builder.step(NonZeroUsize::new(1024).unwrap()).unwrap();
        }
        builder.take_ready().unwrap()
    }
    #[test]
    fn reduced_motion_freezes_decorative_time_without_stalling_scene_time() {
        let mut world = World::new();
        world.init_resource::<SurfaceRuntime>();
        world.insert_resource(crate::scene::MotionPreference(true));
        for _ in 0..60 {
            advance_clock(&mut world, 1. / 60.).unwrap();
        }
        assert_eq!(world.resource::<SurfaceRuntime>().step(), 60);
        assert_eq!(world.resource::<SurfaceRuntime>().motion_step(), 0);
        world.resource_mut::<crate::scene::MotionPreference>().0 = false;
        for _ in 0..30 {
            advance_clock(&mut world, 1. / 60.).unwrap();
        }
        assert_eq!(world.resource::<SurfaceRuntime>().motion_step(), 30);
        world.resource_mut::<crate::scene::MotionPreference>().0 = true;
        for _ in 0..60 {
            advance_clock(&mut world, 1. / 60.).unwrap();
        }
        assert_eq!(world.resource::<SurfaceRuntime>().step(), 150);
        assert_eq!(world.resource::<SurfaceRuntime>().motion_step(), 30);
        assert!(advance_clock(&mut world, f64::NAN).is_err());
        advance_clock(&mut world, 0.).unwrap();
        assert_eq!(world.resource::<SurfaceRuntime>().step(), 150);
        assert_eq!(world.resource::<SurfaceRuntime>().motion_step(), 30);
    }
    #[test]
    fn cpu_work_advances_without_a_device_or_completed_gpu_frame() {
        let input = LandscapeInput::new(
            BodyId::Mars,
            BodyProjection::MARS,
            1,
            73,
            None,
            vec![LandscapeSource {
                coordinate: [0, 0, 0],
                kind: LandscapeKind::Ocean,
                seed: 3,
                layout: None,
            }],
        )
        .unwrap();
        let mut runtime = SurfaceRuntime::default();
        runtime.builder.begin(input).unwrap();
        let mut world = World::new();
        world.insert_resource(runtime);
        world.init_resource::<crate::scene::MotionPreference>();
        let mut completed = false;
        for _ in 0..1024 {
            if !advance_cpu(&mut world).unwrap() {
                completed = true;
                break;
            }
        }
        assert!(
            completed,
            "bounded preparation must complete without GPU frames"
        );
        let mut runtime = world.resource_mut::<SurfaceRuntime>();
        assert!(runtime.builder.take_ready().is_some());
        assert_eq!(runtime.step(), 0);
        assert_eq!(runtime.motion_step(), 0);
        assert_eq!(runtime.batches(), 0);
        assert_eq!(runtime.uniforms(), 0);
    }
    #[test]
    fn contact_holds_the_published_field_and_follows_the_same_removal_front() {
        let previous = snapshot(true, 1);
        let next = snapshot(false, 2);
        let point = BoardPoint { x: 0., y: 0. };
        let old_height = f64::from(previous.sampler().height(point).unwrap());
        let next_height = f64::from(next.sampler().height(point).unwrap());
        let previous = FieldTimeline::prepare(None, previous, 0).unwrap();
        let mut timeline = FieldTimeline::prepare(Some(&previous), next.clone(), 100).unwrap();
        timeline.activate(100, false);
        let mut contact = Contact {
            next,
            timeline,
            step: 100,
        };
        assert_eq!(contact.height(point).unwrap(), old_height);
        contact.step = 118;
        assert!((contact.height(point).unwrap() - (old_height + next_height) * 0.5).abs() < 1e-10);
        contact.step = 136;
        assert_eq!(contact.height(point).unwrap(), next_height);
        contact.timeline.settle(136, false);
        assert_eq!(contact.height(point).unwrap(), next_height);
        assert!(contact.height(BoardPoint { x: f64::NAN, y: 0. }).is_err());
    }
    #[test]
    fn successor_prepares_while_older_upload_is_packing_without_starving_either() {
        let timeline = FieldTimeline::prepare(None, snapshot(true, 1), 0).unwrap();
        let packing = FieldUploadBuilder::new(timeline.clone(), 256, MAX_UPLOAD_BYTES)
            .unwrap()
            .unwrap();
        let mut runtime = SurfaceRuntime {
            packing: Some((timeline, packing)),
            desired: Some(queued_input(&[(1, LandscapeKind::Greenery, 17)])),
            ..default()
        };
        assert!(runtime.begin_queued().unwrap().is_some());
        runtime.step_cpu().unwrap();
        assert!(
            matches!(runtime.builder.state(),PreparationState::Preparing{completed,..} if completed>0)
        );
        assert!(runtime.packing.as_ref().unwrap().1.progress().0 > 0);
    }
}
