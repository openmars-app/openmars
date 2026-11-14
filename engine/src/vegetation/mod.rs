//! Stable approved plant variants and bounded, scene-owned instancing.
pub mod materials;
pub mod models;
mod shade;
use crate::terrain::mips::{self, MipRenderer, MipWork};
use bevy::{
    asset::AssetId,
    camera::{
        RenderTarget,
        visibility::{NoFrustumCulling, RenderLayers},
    },
    core_pipeline::tonemapping::Tonemapping,
    image::{ImageFilterMode, ImageSampler, ImageSamplerDescriptor},
    mesh::MeshTag,
    pbr::{ExtendedMaterial, PreparedMaterial},
    prelude::*,
    render::{
        RenderApp,
        erased_render_asset::ErasedRenderAssets,
        mesh::RenderMesh,
        render_asset::RenderAssets,
        render_resource::{Buffer, BufferId, PipelineCache, UniformBuffer},
        renderer::{RenderDevice, RenderQueue},
        texture::GpuImage,
    },
};
use materials::{FoliageExtension, FoliageMaterial, FoliageUniform};
use models::{REQUESTS, Variant};
use openmars_scene::{
    climate::ClimateState,
    geometry::{BodyProjection, placement::GreeneryKind},
    landscape::{
        LandscapeSnapshot,
        vegetation::{LandscapePlant, PlantId},
    },
};
use std::{
    collections::{BTreeMap, BTreeSet, VecDeque},
    sync::Arc,
};
type MaterialKey = (usize, bool, Option<AssetId<Image>>);
const MAX_PLANTS: usize = 18 * 4096 + 16384;
pub struct Input {
    pub context: u32,
    pub snapshot: Arc<LandscapeSnapshot>,
    pub current: ClimateState,
    pub reduced: bool,
    pub settled: bool,
    pub target: ClimateState,
    pub step: u64,
    pub warm: Handle<Image>,
}
struct Texture {
    _source: Handle<Image>,
    sampled: Handle<Image>,
    work: Option<MipWork>,
    ready: bool,
}
struct Instance {
    plant: LandscapePlant,
    root: Entity,
    children: Vec<Entity>,
    born: f64,
    retire: Option<f64>,
    transform: Transform,
    visible: bool,
    first_frame: Option<(u64, u32)>,
}
struct WarmPlants {
    context: u32,
    required_frames: u64,
    frame: u64,
    revision: u64,
    ids: Vec<PlantId>,
}
struct WarmLibrary {
    root: Entity,
    camera: Entity,
    frame: u64,
}
impl WarmPlants {
    fn ready(&self, frame: u64, uploaded: bool) -> bool {
        uploaded && frame.saturating_sub(self.frame) >= self.required_frames
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
struct PoseRevision {
    terrain: u64,
    contact: Option<(u64, Option<u64>, u64)>,
    tree_scale: u64,
    reduced: bool,
    settled: bool,
}

#[derive(Resource, Default)]
pub struct FoliageRuntime {
    variants: Vec<Variant>,
    textures: BTreeMap<AssetId<Image>, Texture>,
    materials: BTreeMap<MaterialKey, Handle<FoliageMaterial>>,
    instances: BTreeMap<PlantId, Instance>,
    desired: BTreeMap<PlantId, LandscapePlant>,
    selection: Option<([u8; 32], [u8; 4])>,
    incoming: VecDeque<LandscapePlant>,
    animating: BTreeSet<PlantId>,
    pose_revision: Option<PoseRevision>,
    pose_step: Option<u64>,
    scene_tick: Option<u32>,
    nuclear: BTreeSet<[i32; 3]>,
    nuclear_timeline: Vec<([i32; 3], Option<crate::nuclear::timeline::Start>)>,
    shade_dirty: bool,
    root: Option<Entity>,
    uniform: Option<UniformBuffer<FoliageUniform>>,
    uniform_id: Option<BufferId>,
    retired: Vec<Buffer>,
    retired_shade: Vec<Buffer>,
    shade: shade::Shade,
    staged: VecDeque<WarmPlants>,
    warm_camera: Option<Entity>,
    frame: u64,
    warmed_variants: BTreeSet<usize>,
    library_warm: Option<WarmLibrary>,
    library_warmed: bool,
    ready: bool,
    pub uniform_writes: u64,
    pub retired_uniforms: u64,
    pub mip_draws: u64,
    pub spawns: u64,
    pub pose_evaluations: u64,
    pub transform_writes: u64,
    pub shade_rebuilds: u64,
    pub selection_rebuilds: u64,
    pub idle_pose_frames: u64,
}
impl FoliageRuntime {
    pub fn ready(&self) -> bool {
        self.ready
    }
    pub fn diagnostics(&self) -> serde_json::Value {
        serde_json::json!({"poseEvaluations":self.pose_evaluations,"transformWrites":self.transform_writes,"shadeRebuilds":self.shade_rebuilds,"selectionRebuilds":self.selection_rebuilds,"idlePoseFrames":self.idle_pose_frames,"pendingPlants":self.incoming.len(),"animatedPlants":self.animating.len(),"models":self.variants.iter().enumerate().map(|(i,v)|serde_json::json!({"name":REQUESTS[i].name,"extent":REQUESTS[i].extent,"min":v.bounds[0].to_array(),"max":v.bounds[1].to_array(),"vertices":v.vertices,"indices":v.indices,"parts":v.parts.len()})).collect::<Vec<_>>(),"sample":self.instances.iter().take(32).map(|(id,v)|serde_json::json!({"kind":format!("{:?}",id.kind),"cell":id.cell,"entity":v.root.to_string(),"variant":variant(&v.plant),"born":v.born})).collect::<Vec<_>>()})
    }
    pub fn poses(&self, world: &World) -> serde_json::Value {
        serde_json::json!(self.instances.values().take(64).filter_map(|p|world.get::<Transform>(p.root).map(|pose|serde_json::json!({"tile":p.plant.tile,"entity":p.root.to_string(),"kind":format!("{:?}",p.plant.id.kind),"position":pose.translation.to_array(),"rotation":pose.rotation.to_array(),"scale":pose.scale.to_array()}))).collect::<Vec<_>>())
    }
    pub fn shade(&self) -> Option<(Handle<Image>, Vec4, u64)> {
        self.shade.published()
    }
    pub fn shade_stats(&self) -> (usize, u64, u32, u64) {
        (
            self.shade.buffers() + self.retired_shade.len(),
            self.shade.revision,
            self.shade.shapes,
            self.shade.draws,
        )
    }
    fn shade_blobs(&self) -> Vec<shade::Blob> {
        self.instances
            .values()
            .filter_map(|instance| {
                if self.nuclear.contains(&instance.plant.tile) {
                    return None;
                }
                let scale = instance.transform.scale;
                let size = (scale.x * 500.).round() / 500.;
                if size <= 0. {
                    return None;
                }
                let (radius, height, strength) = match instance.plant.id.kind {
                    GreeneryKind::Tree => (0.03, 0.08, 0.8),
                    GreeneryKind::Pine => (0.026, 0.075, 0.75),
                    GreeneryKind::Bush => (0.016, 0.035, 0.6),
                    GreeneryKind::Clover => (0.006, 0.012, 0.3),
                    GreeneryKind::Flower => (0.008, 0.025, 0.25),
                    GreeneryKind::Rock => (0.018, 0.03, 0.55),
                };
                Some(shade::Blob {
                    point: Vec2::new(instance.plant.point.x as f32, instance.plant.point.y as f32),
                    radius: radius * size,
                    length: height * size,
                    strength,
                })
            })
            .collect()
    }
    /// Crater debris uses the same approved, normalized rock geometry and mips.
    pub(crate) fn rock(&self, world: &World) -> Option<(Vec<Handle<Mesh>>, Handle<Image>)> {
        let index = REQUESTS.iter().position(|r| r.rock)?;
        let variant = self.variants.get(index)?;
        let source = crate::assets::texture(world, "textures/terrain/rock")?;
        let texture = self.textures.get(&source.id())?;
        texture.ready.then(|| {
            (
                variant.parts.iter().map(|p| p.mesh.clone()).collect(),
                texture.sampled.clone(),
            )
        })
    }
    pub fn variants(&self) -> usize {
        self.variants.len()
    }
    pub fn plants(&self) -> usize {
        self.instances.len()
    }
    pub fn uniforms(&self) -> usize {
        usize::from(self.uniform.is_some()) + self.retired.len()
    }
    pub fn mesh_vertices(&self) -> usize {
        self.variants.iter().map(|v| v.vertices).sum()
    }
    pub fn mesh_indices(&self) -> usize {
        self.variants.iter().map(|v| v.indices).sum()
    }
    pub fn textures(&self) -> usize {
        self.textures.values().filter(|t| t.ready).count()
    }
    pub fn progress(&self) -> Option<String> {
        (!self.ready).then(|| {
            format!(
                "Preparing vegetation ({} / {} models, {} plants)",
                self.variants.len(),
                REQUESTS.len(),
                self.instances.len()
            )
        })
    }
    fn uploaded(&self, render: &World) -> bool {
        let Some(meshes) = render.get_resource::<RenderAssets<RenderMesh>>() else {
            return false;
        };
        let Some(materials) = render.get_resource::<ErasedRenderAssets<PreparedMaterial>>() else {
            return false;
        };
        self.variants.len() == REQUESTS.len()
            && self
                .variants
                .iter()
                .flat_map(|v| &v.parts)
                .all(|p| meshes.get(p.mesh.id()).is_some())
            && self
                .materials
                .values()
                .all(|h| materials.get(h.id().untyped()).is_some())
            && render
                .resource::<PipelineCache>()
                .waiting_pipelines()
                .next()
                .is_none()
    }
    fn prepare_models(&mut self, world: &mut World) -> Result<(), String> {
        if self.variants.len() < REQUESTS.len() {
            if let Some(variant) = models::prepare_variant(world, REQUESTS[self.variants.len()])? {
                for part in &variant.parts {
                    if let Some(source) = &part.texture {
                        self.add_texture(world, source.clone())?;
                    }
                }
                self.variants.push(variant);
            }
            return Ok(());
        }
        let rock = crate::assets::texture(world, "textures/terrain/rock");
        if let Some(rock) = rock
            && world.resource::<Assets<Image>>().get(&rock).is_some()
        {
            self.add_texture(world, rock)?;
        }
        Ok(())
    }
    fn add_texture(&mut self, world: &mut World, source: Handle<Image>) -> Result<(), String> {
        if self.textures.contains_key(&source.id()) {
            return Ok(());
        }
        if self.textures.len() >= 32 {
            return Err("Foliage texture admission exceeded".into());
        }
        let image = world
            .resource::<Assets<Image>>()
            .get(&source)
            .ok_or("Missing decoded foliage texture")?;
        let size = image.size();
        if size.x == 0 || size.y == 0 || size.x > 4096 || size.y > 4096 {
            return Err("Foliage texture bounds exceeded".into());
        }
        let mut target = mips::target_image(
            size.x,
            size.y,
            1,
            size.x.max(size.y).ilog2() + 1,
            image.texture_descriptor.format.is_srgb(),
        );
        if target.texture_descriptor.format != image.texture_descriptor.format {
            return Err("Foliage texture color space mismatch".into());
        }
        let mut sampler = match &image.sampler {
            ImageSampler::Descriptor(d) => d.clone(),
            ImageSampler::Default => ImageSamplerDescriptor::default(),
        };
        sampler.mag_filter = ImageFilterMode::Linear;
        sampler.min_filter = ImageFilterMode::Linear;
        sampler.mipmap_filter = ImageFilterMode::Linear;
        sampler.anisotropy_clamp = 4;
        target.sampler = ImageSampler::Descriptor(sampler);
        let sampled = world.resource_mut::<Assets<Image>>().add(target);
        let work = MipWork::for_images(world, &[(source.clone(), sampled.clone())])?;
        self.textures.insert(
            source.id(),
            Texture {
                _source: source,
                sampled,
                work: Some(work),
                ready: false,
            },
        );
        Ok(())
    }
    fn material(
        &mut self,
        world: &mut World,
        variant: usize,
        part: &models::Part,
    ) -> Result<Handle<FoliageMaterial>, String> {
        let source = if variant == 18 {
            Some(
                crate::assets::texture(world, "textures/terrain/rock")
                    .ok_or("Missing rock texture")?,
            )
        } else {
            part.texture.clone()
        };
        let key = (
            usize::from(variant == 18),
            part.double_sided,
            source.as_ref().map(|h| h.id()),
        );
        if let Some(material) = self.materials.get(&key) {
            return Ok(material.clone());
        }
        let texture = source
            .map(|h| {
                self.textures
                    .get(&h.id())
                    .filter(|t| t.ready)
                    .map(|t| t.sampled.clone())
                    .ok_or("Foliage mip chain not complete")
            })
            .transpose()?;
        let atmosphere = world
            .resource::<crate::atmosphere::AtmosphereRuntime>()
            .binding()?;
        let material = world
            .resource_mut::<Assets<FoliageMaterial>>()
            .add(ExtendedMaterial {
                base: StandardMaterial {
                    base_color: Color::WHITE,
                    base_color_texture: texture.clone(),
                    emissive_texture: texture,
                    emissive: LinearRgba::WHITE * if variant == 18 { 0.15 } else { 0.5 },
                    alpha_mode: if variant == 18 {
                        AlphaMode::Opaque
                    } else {
                        AlphaMode::Mask(0.15)
                    },
                    double_sided: part.double_sided,
                    cull_mode: if part.double_sided {
                        None
                    } else {
                        Some(bevy::render::render_resource::Face::Back)
                    },
                    perceptual_roughness: 1.,
                    metallic: 0.,
                    ..default()
                },
                extension: FoliageExtension {
                    atmosphere,
                    parameters: self.uniform.as_ref().unwrap().buffer().unwrap().clone(),
                },
            });
        self.materials.insert(key, material.clone());
        Ok(material)
    }
    /// Compile the real shared foliage layouts before gameplay, independently
    /// of which species the initial climate happens to make visible.
    fn warm_library(
        &mut self,
        world: &mut World,
        target: Handle<Image>,
        uploaded: bool,
    ) -> Result<bool, String> {
        if self.library_warmed {
            return Ok(true);
        }
        if let Some(warm) = &self.library_warm {
            if !uploaded || self.frame.saturating_sub(warm.frame) < 2 {
                return Ok(false);
            }
            let warm = self.library_warm.take().unwrap();
            world.despawn(warm.root);
            world.despawn(warm.camera);
            self.warmed_variants.extend(0..REQUESTS.len());
            self.library_warmed = true;
            return Ok(true);
        }
        let mut parts = Vec::new();
        for (index, request) in REQUESTS.iter().enumerate() {
            let model = self.variants[index].parts.clone();
            let transform = Transform {
                translation: Vec3::new((index % 5) as f32 - 2., (index / 5) as f32 - 1.5, 0.),
                scale: Vec3::splat(0.8 / request.extent as f32),
                ..default()
            };
            for part in model {
                let material = self.material(world, index, &part)?;
                parts.push((part.mesh, material, transform));
            }
        }
        let root = world
            .spawn((Transform::default(), Visibility::Inherited))
            .id();
        for (mesh, material, transform) in parts {
            world.spawn((
                Mesh3d(mesh),
                MeshMaterial3d(material),
                MeshTag::new(37 | (255 << 8)),
                NoFrustumCulling,
                RenderLayers::layer(2),
                transform,
                ChildOf(root),
            ));
        }
        let camera = world
            .spawn((
                Camera3d::default(),
                bevy::core_pipeline::prepass::DepthPrepass,
                Camera {
                    order: -3,
                    ..default()
                },
                RenderTarget::Image(target.into()),
                Msaa::Off,
                Tonemapping::None,
                RenderLayers::layer(2),
                Transform::from_xyz(0., 0., 6.8).looking_at(Vec3::ZERO, Vec3::Y),
            ))
            .id();
        self.library_warm = Some(WarmLibrary {
            root,
            camera,
            frame: self.frame,
        });
        Ok(false)
    }
    /// Update only moving or explicitly invalidated instances. A settled
    /// landscape has no per-plant pose work between actual visual changes.
    fn refresh_poses(
        &mut self,
        world: &mut World,
        input: &Input,
        selection_changed: bool,
    ) -> Result<(), String> {
        let seconds = input.step as f64 / 60.;
        let revision = {
            let terrain = world.resource::<crate::terrain::TerrainState>();
            PoseRevision {
                terrain: terrain.revision,
                contact: terrain
                    .contact
                    .as_ref()
                    .map(|contact| contact.sample_revision()),
                tree_scale: if input.reduced || input.settled {
                    input.target.tree_scale
                } else {
                    input.current.tree_scale
                }
                .to_bits(),
                reduced: input.reduced,
                settled: input.settled,
            }
        };
        // The public scene owns shade exclusion; it can change before the new
        // landscape publishes. Read its tile list only when that resource changes.
        let scene_tick = world
            .get_resource_ref::<crate::scene::SceneState>()
            .map(|scene| scene.last_changed().get());
        if self.scene_tick != scene_tick {
            self.scene_tick = scene_tick;
            let nuclear = world
                .resource::<crate::scene::SceneState>()
                .0
                .as_ref()
                .map_or_else(BTreeSet::new, |scene| {
                    scene
                        .tiles
                        .iter()
                        .filter(|tile| {
                            crate::world::is_mars(tile)
                                && tile.occupant.as_deref() == Some("nuclear-zone-tile")
                        })
                        .map(|tile| [tile.q, tile.r, tile.s])
                        .collect()
                });
            if nuclear != self.nuclear {
                self.nuclear = nuclear;
                self.shade_dirty = true;
            }
        }
        let timeline = world.resource::<crate::nuclear::timeline::Timeline>();
        let nuclear_timeline: Vec<_> = timeline
            .coordinates()
            .map(|(q, r, s)| ([q, r, s], timeline.start((q, r, s))))
            .collect();
        let all_dirty = selection_changed
            || self.pose_revision != Some(revision)
            || self.nuclear_timeline != nuclear_timeline;
        self.pose_revision = Some(revision);
        self.nuclear_timeline = nuclear_timeline;
        let work: Vec<_> = if all_dirty {
            self.instances.keys().copied().collect()
        } else if self.pose_step != Some(input.step) {
            self.animating.iter().copied().collect()
        } else {
            Vec::new()
        };
        self.pose_step = Some(input.step);
        if work.is_empty() {
            self.idle_pose_frames += 1;
        }
        let mut remove = Vec::new();
        let mut first_frames: BTreeMap<(u64, u32), BTreeSet<[i32; 3]>> = BTreeMap::new();
        for id in work {
            let Some(instance) = self.instances.get_mut(&id) else {
                continue;
            };
            self.animating.remove(&id);
            // Warm poses are hidden. Their visible animation starts only at
            // publication, not while later chunks are still being prepared.
            if !instance.visible {
                if instance.retire.is_some() {
                    remove.push(id);
                }
                continue;
            }
            let c = instance.plant.tile;
            let c = (c[0], c[1], c[2]);
            let timeline = world.resource::<crate::nuclear::timeline::Timeline>();
            if !input.settled
                && timeline.live(c)
                && let Some(start) = timeline.start(c)
            {
                if start.ended(input.step, input.reduced) {
                    remove.push(id);
                    continue;
                }
                self.animating.insert(id);
                if input.reduced {
                    let alpha = ((1. - start.coverage(input.step, true)) * 255.).round() as u32;
                    for child in &instance.children {
                        world
                            .entity_mut(*child)
                            .insert(MeshTag::new((instance.plant.seed % 100) | (alpha << 8)));
                    }
                    continue;
                }
                let age = input.step.saturating_sub(start.step) as f64 / 60. * 0.8 - 0.8;
                if age >= 0. {
                    self.pose_evaluations += 1;
                    let transform = collapsing_pose(
                        instance.plant,
                        input.current,
                        self.variants[variant(&instance.plant)].sphere_radius,
                        age,
                    )?;
                    if instance.transform != transform {
                        if instance.transform.scale != transform.scale {
                            self.shade_dirty = true;
                        }
                        instance.transform = transform;
                        world.entity_mut(instance.root).insert(transform);
                        self.transform_writes += 1;
                    }
                    continue;
                }
            }
            if (input.reduced || input.settled) && instance.retire.is_some() {
                remove.push(id);
                continue;
            }
            if input.settled {
                instance.born = -1000.;
            }
            let t = ((seconds - instance.born) / 0.6).clamp(0., 1.);
            if t > 0.
                && let Some(revision) = instance.first_frame.take()
            {
                first_frames
                    .entry(revision)
                    .or_default()
                    .insert(instance.plant.tile);
            }
            let growth = if input.reduced || input.settled {
                1.
            } else if let Some(start) = instance.retire {
                let t = ((seconds - start) / 1.1).clamp(0., 1.);
                if t >= 1. {
                    remove.push(id);
                }
                1. - t.powi(3)
            } else {
                1. - (1. - t).powi(3)
            };
            if instance.retire.is_some() || (!input.reduced && !input.settled && t < 1.) {
                self.animating.insert(id);
            }
            let height = world
                .resource::<crate::terrain::TerrainState>()
                .height(instance.plant.point)
                - instance
                    .retire
                    .map_or(0., |start| ((seconds - start) / 1.1).clamp(0., 1.) * 0.004);
            self.pose_evaluations += 1;
            let transform = pose(
                instance.plant,
                if input.reduced || input.settled {
                    input.target
                } else {
                    input.current
                },
                growth,
                height,
            )?;
            if instance.transform != transform {
                let before = (instance.transform.scale.x * 500.).round();
                let after = (transform.scale.x * 500.).round();
                self.shade_dirty |= before != after;
                instance.transform = transform;
                world.entity_mut(instance.root).insert(transform);
                self.transform_writes += 1;
            }
        }
        for ((revision, context), tiles) in first_frames {
            for [q, r, s] in tiles {
                let _event = crate::profiling::instant(world, "vegetation.animation.frame")
                    .arg("context", context)
                    .arg("landscapeRevision", revision)
                    .arg("q", q)
                    .arg("r", r)
                    .arg("s", s)
                    .arg("animationStep", input.step)
                    .arg("stage", "render_submission");
            }
        }
        for id in remove {
            if let Some(instance) = self.instances.remove(&id) {
                world.despawn(instance.root);
                self.animating.remove(&id);
                self.shade_dirty = true;
                for batch in &mut self.staged {
                    batch.ids.retain(|plant| *plant != id);
                }
            }
        }
        Ok(())
    }
    fn publish_batch(
        &mut self,
        world: &mut World,
        input: &Input,
        batch: WarmPlants,
    ) -> Result<(), String> {
        let seconds = input.step as f64 / 60.;
        let mut entities = 0;
        let mut tiles = BTreeSet::new();
        for id in batch.ids {
            let Some(instance) = self.instances.get_mut(&id) else {
                continue;
            };
            if instance.retire.is_some() {
                continue;
            }
            self.warmed_variants.insert(variant(&instance.plant));
            instance.visible = true;
            instance.first_frame =
                (!input.settled && !input.reduced).then_some((batch.revision, batch.context));
            instance.born = if input.settled || input.reduced {
                -1000.
            } else {
                seconds
            };
            let height = world
                .resource::<crate::terrain::TerrainState>()
                .height(instance.plant.point);
            let transform = pose(
                instance.plant,
                if input.settled || input.reduced {
                    input.target
                } else {
                    input.current
                },
                if input.settled || input.reduced {
                    1.
                } else {
                    0.
                },
                height,
            )?;
            instance.transform = transform;
            world.entity_mut(instance.root).insert(transform);
            self.animating.insert(id);
            for entity in &instance.children {
                if let Ok(mut entity) = world.get_entity_mut(*entity) {
                    entity.insert(RenderLayers::default());
                    entities += 1;
                }
            }
            tiles.insert(instance.plant.tile);
        }
        for &[q, r, s] in &tiles {
            let _event = crate::profiling::instant(world, "vegetation.animation.begin")
                .arg("context", batch.context)
                .arg("landscapeRevision", batch.revision)
                .arg("q", q)
                .arg("r", r)
                .arg("s", s)
                .arg("animationStep", input.step)
                .arg("animated", !input.settled && !input.reduced);
        }
        let _event = crate::profiling::instant(world, "vegetation.publish")
            .arg("context", batch.context)
            .arg("landscapeRevision", batch.revision)
            .arg("entities", entities)
            .arg("plants", self.instances.len())
            .arg("tiles", tiles.len())
            .arg("animationStep", input.step);
        Ok(())
    }
    fn update(
        &mut self,
        world: &mut World,
        input: Input,
        uploaded: bool,
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<(), String> {
        self.frame += 1;
        let seconds = input.step as f64 / 60.;
        if self.uniform.is_none() {
            let mut buffer: UniformBuffer<FoliageUniform> =
                FoliageUniform { frost: Vec4::ZERO }.into();
            buffer.set_label(Some("foliage climate"));
            buffer.write_buffer(device, queue);
            self.uniform_writes += 1;
            self.uniform_id = buffer.buffer().map(|b| b.id());
            self.uniform = Some(buffer);
        }
        let buffer = self.uniform.as_mut().unwrap();
        let frost = input.current.plant_frost as f32;
        if buffer.get().frost.x != frost {
            buffer.get_mut().frost.x = frost;
            buffer.write_buffer(device, queue);
            self.uniform_writes += 1;
        }
        if buffer.buffer().map(|b| b.id()) != self.uniform_id {
            return Err("Foliage climate buffer replaced".into());
        }
        if self.variants.len() != REQUESTS.len()
            || self.textures.values().any(|t| !t.ready)
            || !self.textures.contains_key(
                &crate::assets::texture(world, "textures/terrain/rock")
                    .ok_or("Missing rock image")?
                    .id(),
            )
        {
            self.ready = false;
            return Ok(());
        }
        if !self.warm_library(world, input.warm.clone(), uploaded)? {
            self.ready = false;
            return Ok(());
        }
        let gates = [
            input.target.tree,
            input.target.bush,
            input.target.grass,
            input.target.flower,
        ]
        .map(|v| (v.clamp(0., 1.) * 20.).round() as u8);
        let selection = (input.snapshot.source_identity(), gates);
        let selection_changed = self.selection != Some(selection);
        if selection_changed {
            self.selection_rebuilds += 1;
            self.shade_dirty = true;
            let mut desired = BTreeMap::new();
            let mut counts = [0usize; 19];
            for plant in input.snapshot.patches().flat_map(|patch| patch.plants()) {
                let gate = f64::from(match plant.id.kind {
                    GreeneryKind::Tree => gates[0],
                    GreeneryKind::Bush => gates[1],
                    GreeneryKind::Clover => gates[2],
                    GreeneryKind::Flower => gates[3],
                    _ => 20,
                }) / 20.;
                if plant.visible(gate, 1.).map_err(|e| e.to_string())? {
                    counts[variant(plant)] += 1;
                    desired.insert(plant.id, *plant);
                }
            }
            if desired.len() > MAX_PLANTS
                || counts
                    .iter()
                    .enumerate()
                    .any(|(i, n)| *n > if i == 14 { 16384 } else { 4096 })
            {
                return Err("Foliage instance admission exceeded".into());
            }
            for (id, instance) in &mut self.instances {
                if let Some(plant) = desired.get(id) {
                    if instance.plant != *plant {
                        self.shade_dirty = true;
                    }
                    instance.plant = *plant;
                    instance.retire = None;
                } else {
                    instance.retire = Some(seconds);
                    self.animating.insert(*id);
                }
            }
            self.incoming = desired
                .iter()
                .filter(|(id, _)| !self.instances.contains_key(id))
                .map(|(_, plant)| *plant)
                .collect();
            self.desired = desired;
            self.selection = Some(selection);
            self.ready = false;
        }
        let incoming: Vec<_> = (0..256).filter_map(|_| self.incoming.pop_front()).collect();
        let root = *self.root.get_or_insert_with(|| {
            world
                .spawn((Transform::default(), Visibility::Inherited))
                .id()
        });
        let arrivals = !incoming.is_empty();
        let reusable = incoming
            .iter()
            .all(|p| self.warmed_variants.contains(&variant(p)));
        let mut staged = Vec::with_capacity(incoming.len());
        for plant in incoming {
            let v = variant(&plant);
            let parts = self.variants[v].parts.clone();
            let born = if !input.settled && !input.reduced {
                seconds
            } else {
                -1000.
            };
            let pose = pose(
                plant,
                input.current,
                1.,
                world
                    .resource::<crate::terrain::TerrainState>()
                    .height(plant.point),
            )?;
            let entity = world
                .spawn((pose, Visibility::Inherited, ChildOf(root)))
                .id();
            let mut children = Vec::new();
            for part in parts {
                let material = self.material(world, v, &part)?;
                let child = world
                    .spawn((
                        Mesh3d(part.mesh),
                        MeshMaterial3d(material),
                        MeshTag::new((plant.seed % 100) | (255 << 8)),
                        NoFrustumCulling,
                        RenderLayers::layer(2),
                        Transform::IDENTITY,
                        ChildOf(entity),
                    ))
                    .id();
                children.push(child);
            }
            self.instances.insert(
                plant.id,
                Instance {
                    plant,
                    root: entity,
                    children,
                    born,
                    retire: None,
                    transform: pose,
                    visible: false,
                    first_frame: None,
                },
            );
            self.spawns += 1;
            self.transform_writes += 1;
            self.pose_evaluations += 1;
            self.shade_dirty = true;
            self.animating.insert(plant.id);
            staged.push(plant.id);
        }
        if !staged.is_empty() {
            self.staged.push_back(WarmPlants {
                context: input.context,
                required_frames: u64::from(!reusable),
                frame: self.frame,
                revision: input.snapshot.revision(),
                ids: staged,
            });
        }
        self.refresh_poses(world, &input, selection_changed || arrivals)?;
        if !self.staged.is_empty() {
            let shared = world
                .query::<(&Camera, &RenderLayers)>()
                .iter(world)
                .any(|(camera, layers)| camera.is_active && layers == &RenderLayers::layer(2));
            if !shared {
                let camera = world
                    .spawn((
                        Camera3d::default(),
                        bevy::core_pipeline::prepass::DepthPrepass,
                        Camera {
                            order: -3,
                            ..default()
                        },
                        RenderTarget::Image(input.warm.clone().into()),
                        Msaa::Off,
                        Tonemapping::None,
                        RenderLayers::layer(2),
                        Transform::from_xyz(0., 0., 6.8).looking_at(Vec3::ZERO, Vec3::Y),
                    ))
                    .id();
                self.warm_camera = Some(camera);
            }
            while self
                .staged
                .front()
                .is_some_and(|batch| batch.ready(self.frame, uploaded))
            {
                let batch = self.staged.pop_front().unwrap();
                self.publish_batch(world, &input, batch)?;
            }
            if self.staged.is_empty()
                && let Some(camera) = self.warm_camera.take()
            {
                world.despawn(camera);
            }
        }
        self.ready = self.incoming.is_empty() && self.staged.is_empty() && uploaded;
        Ok(())
    }
}
fn variant(plant: &LandscapePlant) -> usize {
    match plant.id.kind {
        GreeneryKind::Tree => plant.variant(),
        GreeneryKind::Bush => 4 + plant.variant(),
        GreeneryKind::Clover => 9 + plant.variant(),
        GreeneryKind::Pine => 14,
        GreeneryKind::Flower => 15 + plant.variant(),
        GreeneryKind::Rock => 18,
    }
}
fn pose(
    plant: LandscapePlant,
    climate: ClimateState,
    growth: f64,
    height: f64,
) -> Result<Transform, String> {
    let point = BodyProjection::MARS
        .project(plant.point, height)
        .map_err(|e| e.to_string())?;
    let mut position = Vec3::from_array(point.map(|v| v as f32));
    let normal = position.normalize();
    let size = match plant.id.kind {
        GreeneryKind::Tree | GreeneryKind::Bush => climate.tree_scale,
        GreeneryKind::Pine => 0.85 + 0.15 * ((climate.tree_scale - 0.6) / 0.4),
        _ => 1.,
    };
    let base = plant.scale * growth;
    let sink = match plant.id.kind {
        GreeneryKind::Tree => 0.0024,
        GreeneryKind::Pine => 0.0022,
        GreeneryKind::Bush => 0.001,
        _ => 0.,
    };
    position -= normal * (sink * base) as f32;
    Ok(Transform {
        translation: position,
        rotation: Quat::from_rotation_arc(Vec3::Z, normal)
            * Quat::from_rotation_z((plant.seed % 6283) as f32 / 1000.),
        scale: Vec3::splat((base * size).max(1e-5) as f32),
    })
}
fn collapsing_pose(
    plant: LandscapePlant,
    climate: ClimateState,
    radius: f64,
    age: f64,
) -> Result<Transform, String> {
    let mut transform = pose(plant, climate, 1., plant.height)?;
    let part = openmars_scene::geometry::collapse::CollapseParameters::new(u64::from(plant.seed));
    let progress = part.progress(age);
    let normal = transform.translation.normalize();
    transform.translation -= normal * ((radius * 2. * plant.scale + 0.065) * progress) as f32;
    transform.rotation *= Quat::from_axis_angle(
        Vec3::new(part.direction.cos() as f32, part.direction.sin() as f32, 0.),
        (part.tilt * progress) as f32,
    );
    Ok(transform)
}
pub fn prepare_queued(
    app: &mut App,
    input: Option<Input>,
    device: &RenderDevice,
    queue: &RenderQueue,
) -> Result<(), String> {
    let _profile = crate::profiling::span(app.world(), "vegetation.prepare");
    let uploaded = app
        .world()
        .resource::<FoliageRuntime>()
        .uploaded(app.sub_app(RenderApp).world());
    let mut runtime = app.world_mut().remove_resource::<FoliageRuntime>().unwrap();
    let result = (|| {
        for buffer in runtime.retired.drain(..) {
            buffer.destroy();
            runtime.retired_uniforms += 1;
        }
        for buffer in runtime.retired_shade.drain(..) {
            buffer.destroy();
        }
        if app
            .world()
            .resource::<crate::scene::SceneState>()
            .0
            .is_none()
        {
            return Ok(());
        }
        if !app
            .world()
            .resource::<crate::structures::texture_library::TextureLibrary>()
            .ready()
        {
            return Ok(());
        }
        runtime.prepare_models(app.world_mut())?;
        if let Some(texture) = runtime.textures.values_mut().find(|texture| !texture.ready) {
            let mut renderer = app.world_mut().remove_resource::<MipRenderer>().unwrap();
            let before = renderer.draws;
            let result = renderer.advance_queued(
                texture.work.as_mut().unwrap(),
                app.sub_app(RenderApp)
                    .world()
                    .resource::<RenderAssets<GpuImage>>(),
                device,
                queue,
            );
            runtime.mip_draws += renderer.draws - before;
            app.world_mut().insert_resource(renderer);
            if result? {
                texture.ready = true;
                texture.work = None;
            }
        }
        if let Some(input) = input {
            runtime.update(app.world_mut(), input, uploaded, device, queue)?;
            if runtime.ready {
                let blobs = runtime.shade_dirty.then(|| {
                    runtime.shade_rebuilds += 1;
                    runtime.shade_blobs()
                });
                runtime.ready = runtime.shade.advance_queued(app, blobs, device, queue)?;
                if runtime.ready {
                    runtime.shade_dirty = false;
                }
            }
        }
        Ok(())
    })();
    app.world_mut().insert_resource(runtime);
    result
}
pub fn reset(world: &mut World) {
    let mut runtime = world.remove_resource::<FoliageRuntime>().unwrap();
    if let Some(root) = runtime.root.take() {
        world.despawn(root);
    }
    if let Some(camera) = runtime.warm_camera.take() {
        world.despawn(camera);
    }
    if let Some(warm) = runtime.library_warm.take() {
        world.despawn(warm.root);
        world.despawn(warm.camera);
    }
    if let Some(buffer) = runtime.uniform.take() {
        runtime.retired.push(buffer.buffer().unwrap().clone());
    }
    if let Some(buffer) = runtime.shade.retire() {
        runtime.retired_shade.push(buffer);
    }
    let retired_shade = std::mem::take(&mut runtime.retired_shade);
    let retired = std::mem::take(&mut runtime.retired);
    world.insert_resource(FoliageRuntime {
        retired,
        retired_shade,
        ..default()
    });
}
#[cfg(test)]
mod collapse_tests {
    use super::*;
    use openmars_scene::{geometry::BoardPoint, landscape::vegetation::PlantId};
    #[test]
    fn resetting_during_library_warming_retires_the_view_and_its_geometry() {
        let mut world = World::new();
        let root = world.spawn_empty().id();
        let child = world.spawn(ChildOf(root)).id();
        let camera = world.spawn_empty().id();
        world.insert_resource(FoliageRuntime {
            library_warm: Some(WarmLibrary {
                root,
                camera,
                frame: 0,
            }),
            ..default()
        });
        reset(&mut world);
        assert!(world.get_entity(root).is_err());
        assert!(world.get_entity(child).is_err());
        assert!(world.get_entity(camera).is_err());
        assert!(!world.resource::<FoliageRuntime>().library_warmed);
    }
    #[test]
    fn plant_collapse_keeps_authored_form_and_uses_source_model_radius_for_sink() {
        let climate = openmars_scene::climate::from_parameters(
            openmars_scene::climate::ClimateParameters::TERRAFORMED,
        )
        .unwrap();
        let plant = LandscapePlant {
            id: PlantId {
                kind: GreeneryKind::Tree,
                cell: [0, 0],
                capital_only: false,
            },
            tile: [0, 0, 0],
            point: BoardPoint { x: 0.3, y: -0.2 },
            height: 0.03,
            seed: 37,
            scale: 0.8,
            rank: 0.5,
        };
        let intact = pose(plant, climate, 1., plant.height).unwrap();
        let before = collapsing_pose(plant, climate, 0.03, -0.01).unwrap();
        assert_eq!(before, intact);
        let sunk = collapsing_pose(plant, climate, 0.03, 2.).unwrap();
        assert_eq!(sunk.scale, intact.scale);
        let expected = 0.03 * 2. * plant.scale + 0.065;
        assert!((f64::from(sunk.translation.distance(intact.translation)) - expected).abs() < 2e-7);
        assert!(
            (sunk.translation - intact.translation)
                .normalize()
                .dot(-intact.translation.normalize())
                > 0.999999
        );
        let part = openmars_scene::geometry::collapse::CollapseParameters::new(37);
        assert!((f64::from(sunk.rotation.angle_between(intact.rotation)) - part.tilt).abs() < 1e-5);
    }
}

#[cfg(test)]
mod cache_tests {
    use super::*;
    #[test]
    fn warming_does_not_consume_the_live_growth_animation() {
        let (mut world, mut runtime, mut input, id) = fixture();
        let profiler = crate::profiling::Profiler::default();
        profiler.set_enabled(true, 0.);
        profiler.set_context(99);
        world.insert_resource(profiler.clone());
        input.context = 99;
        input.step = 120;
        input.settled = false;
        input.reduced = false;
        runtime.instances.get_mut(&id).unwrap().visible = false;
        runtime.instances.get_mut(&id).unwrap().born = 0.;
        runtime.refresh_poses(&mut world, &input, true).unwrap();
        runtime
            .publish_batch(
                &mut world,
                &input,
                WarmPlants {
                    context: 7,
                    required_frames: 1,
                    frame: 0,
                    revision: 1,
                    ids: vec![id],
                },
            )
            .unwrap();
        assert_eq!(runtime.instances[&id].born, 2.);
        let initial = world
            .get::<Transform>(runtime.instances[&id].root)
            .unwrap()
            .scale
            .x;
        assert!(initial < 0.0001);
        input.step += 1;
        runtime.refresh_poses(&mut world, &input, false).unwrap();
        assert!(
            world
                .get::<Transform>(runtime.instances[&id].root)
                .unwrap()
                .scale
                .x
                > initial
        );
        assert_eq!(runtime.instances[&id].first_frame, None);
        let captured: serde_json::Value = serde_json::from_str(&profiler.drain(100)).unwrap();
        let events: Vec<_> = captured["events"]
            .as_array()
            .unwrap()
            .iter()
            .filter(|e| e["name"].as_str().unwrap().starts_with("vegetation."))
            .collect();
        assert_eq!(events.len(), 3);
        for event in events {
            assert_eq!(event["args"]["context"], 7);
            assert_eq!(event["args"]["landscapeRevision"], 1);
        }
    }
    #[test]
    fn continuous_arrivals_do_not_reset_earlier_publication_readiness() {
        let mut chunks = VecDeque::new();
        for frame in 0..10 {
            chunks.push_back(WarmPlants {
                context: frame as u32,
                required_frames: 1,
                frame,
                revision: frame,
                ids: Vec::new(),
            });
            if frame >= 1 {
                assert!(chunks.front().unwrap().ready(frame, true));
                assert_eq!(chunks.pop_front().unwrap().frame, frame - 1);
            }
            assert!(!chunks.front().unwrap().ready(frame, false));
        }
        assert_eq!(chunks.len(), 1);
    }
    #[test]
    fn resident_variants_skip_frame_wait_but_never_skip_gpu_readiness() {
        let reused = WarmPlants {
            context: 7,
            required_frames: 0,
            frame: 10,
            revision: 1,
            ids: vec![],
        };
        assert!(reused.ready(10, true));
        assert!(!reused.ready(10, false));
        let new = WarmPlants {
            required_frames: 1,
            ..reused
        };
        assert!(!new.ready(10, true));
        assert!(new.ready(11, true));
        assert!(!new.ready(11, false));
    }
    use openmars_scene::{
        geometry::BoardPoint,
        landscape::{BodyId, LandscapeBuilder, LandscapeInput, PreparationState},
    };
    use std::num::NonZeroUsize;

    fn fixture() -> (World, FoliageRuntime, Input, PlantId) {
        let mut builder = LandscapeBuilder::new();
        builder
            .begin(
                LandscapeInput::new(BodyId::Mars, BodyProjection::MARS, 1, 7, None, vec![])
                    .unwrap(),
            )
            .unwrap();
        while matches!(builder.state(), PreparationState::Preparing { .. }) {
            builder.step(NonZeroUsize::new(1024).unwrap()).unwrap();
        }
        let climate = openmars_scene::climate::from_parameters(
            openmars_scene::climate::ClimateParameters::TERRAFORMED,
        )
        .unwrap();
        let plant = LandscapePlant {
            id: PlantId {
                kind: GreeneryKind::Tree,
                cell: [0, 0],
                capital_only: false,
            },
            tile: [0, 0, 0],
            point: BoardPoint { x: 0., y: 0. },
            height: 0.,
            seed: 0,
            scale: 1.,
            rank: 0.,
        };
        let transform = pose(plant, climate, 1., 0.).unwrap();
        let mut world = World::new();
        world.insert_resource(crate::terrain::TerrainState::default());
        world.insert_resource(crate::scene::SceneState::default());
        world.insert_resource(crate::nuclear::timeline::Timeline::default());
        let root = world.spawn(transform).id();
        let mut runtime = FoliageRuntime::default();
        runtime.instances.insert(
            plant.id,
            Instance {
                plant,
                root,
                children: vec![],
                born: -1000.,
                retire: None,
                transform,
                visible: true,
                first_frame: None,
            },
        );
        runtime.desired.insert(plant.id, plant);
        runtime.variants.push(Variant {
            parts: vec![],
            bounds: [Vec3::ZERO; 2],
            sphere_radius: 0.03,
            vertices: 0,
            indices: 0,
        });
        let input = Input {
            context: 7,
            snapshot: builder.take_ready().unwrap(),
            current: climate,
            target: climate,
            reduced: false,
            settled: false,
            step: 100,
            warm: Handle::default(),
        };
        runtime.refresh_poses(&mut world, &input, false).unwrap();
        runtime.shade_dirty = false;
        (world, runtime, input, plant.id)
    }

    #[test]
    fn unchanged_settled_plants_do_no_pose_work_or_transform_writes() {
        let (mut world, mut runtime, mut input, _) = fixture();
        let evaluated = runtime.pose_evaluations;
        let writes = runtime.transform_writes;
        for step in 101..401 {
            input.step = step;
            runtime.refresh_poses(&mut world, &input, false).unwrap();
        }
        assert_eq!(runtime.pose_evaluations, evaluated);
        assert_eq!(runtime.transform_writes, writes);
        assert!(!runtime.shade_dirty);
        assert_eq!(runtime.idle_pose_frames, 300);
    }

    #[test]
    fn climate_and_contact_revisions_invalidate_once_without_writing_equal_transforms() {
        let (mut world, mut runtime, mut input, id) = fixture();
        let evaluated = runtime.pose_evaluations;
        input.current.tree_scale = 0.8;
        runtime.refresh_poses(&mut world, &input, false).unwrap();
        assert_eq!(runtime.pose_evaluations, evaluated + 1);
        assert_eq!(runtime.transform_writes, 1);
        assert_eq!(runtime.instances[&id].transform.scale, Vec3::splat(0.8));
        assert!(runtime.shade_dirty);
        runtime.shade_dirty = false;
        world
            .resource_mut::<crate::terrain::TerrainState>()
            .revision += 1;
        runtime.refresh_poses(&mut world, &input, false).unwrap();
        assert_eq!(runtime.pose_evaluations, evaluated + 2);
        assert_eq!(runtime.transform_writes, 1);
        assert!(!runtime.shade_dirty);
        runtime.refresh_poses(&mut world, &input, false).unwrap();
        assert_eq!(runtime.pose_evaluations, evaluated + 2);
    }

    #[test]
    fn growth_and_retirement_run_to_completion_then_leave_the_animation_set() {
        let (mut world, mut runtime, mut input, id) = fixture();
        runtime.instances.get_mut(&id).unwrap().born = input.step as f64 / 60.;
        runtime.animating.insert(id);
        input.step += 1;
        runtime.refresh_poses(&mut world, &input, false).unwrap();
        let first_growth = runtime.instances[&id].transform.scale.x;
        assert!(first_growth > 0. && first_growth < 1.);
        let evaluations = runtime.pose_evaluations;
        runtime.refresh_poses(&mut world, &input, false).unwrap();
        assert_eq!(
            runtime.pose_evaluations, evaluations,
            "same canonical step does not advance growth"
        );
        for _ in 0..40 {
            input.step += 1;
            runtime.refresh_poses(&mut world, &input, false).unwrap();
        }
        assert!(runtime.animating.is_empty());
        assert_eq!(runtime.instances[&id].transform.scale, Vec3::ONE);
        let evaluations = runtime.pose_evaluations;
        input.step += 1;
        runtime.refresh_poses(&mut world, &input, false).unwrap();
        assert_eq!(runtime.pose_evaluations, evaluations);
        runtime.instances.get_mut(&id).unwrap().retire = Some(input.step as f64 / 60.);
        runtime.animating.insert(id);
        for _ in 0..70 {
            input.step += 1;
            runtime.refresh_poses(&mut world, &input, false).unwrap();
        }
        assert!(runtime.instances.is_empty());
        assert!(runtime.animating.is_empty());
    }

    #[test]
    fn nuclear_start_invalidates_idle_plants_and_preserves_collapse_and_removal() {
        let (mut world, mut runtime, mut input, id) = fixture();
        let keys = [(0, 0, 0)].into();
        let at = crate::nuclear::timeline::Start {
            step: input.step,
            motion_step: input.step,
        };
        let mut timeline = world.resource_mut::<crate::nuclear::timeline::Timeline>();
        timeline.observe(keys, false);
        timeline.advance(&[(0, 0, 0)].into(), at, false);
        runtime.refresh_poses(&mut world, &input, false).unwrap();
        assert!(runtime.animating.contains(&id));
        input.step = at.step + 90;
        runtime.refresh_poses(&mut world, &input, false).unwrap();
        let instance = &runtime.instances[&id];
        let age = input.step.saturating_sub(at.step) as f64 / 60. * 0.8 - 0.8;
        assert_eq!(
            instance.transform,
            collapsing_pose(instance.plant, input.current, 0.03, age).unwrap()
        );
        input.step = at.step + 128;
        runtime.refresh_poses(&mut world, &input, false).unwrap();
        assert!(runtime.instances.is_empty());
    }

    #[test]
    fn new_arrival_is_initialized_even_when_canonical_time_has_not_advanced() {
        let (mut world, mut runtime, input, id) = fixture();
        runtime.instances.get_mut(&id).unwrap().born = input.step as f64 / 60.;
        runtime.animating.insert(id);
        runtime.refresh_poses(&mut world, &input, true).unwrap();
        assert_eq!(runtime.instances[&id].transform.scale, Vec3::splat(1e-5));
        assert!(runtime.animating.contains(&id));
    }

    #[test]
    fn reduced_motion_cancels_growth_but_keeps_nuclear_fade_lifetime() {
        let (mut world, mut runtime, mut input, id) = fixture();
        runtime.instances.get_mut(&id).unwrap().born = input.step as f64 / 60.;
        runtime.animating.insert(id);
        input.reduced = true;
        runtime.refresh_poses(&mut world, &input, false).unwrap();
        assert_eq!(runtime.instances[&id].transform.scale, Vec3::ONE);
        assert!(runtime.animating.is_empty());
        let at = crate::nuclear::timeline::Start {
            step: input.step,
            motion_step: input.step,
        };
        let mut timeline = world.resource_mut::<crate::nuclear::timeline::Timeline>();
        timeline.observe([(0, 0, 0)].into(), false);
        timeline.advance(&[(0, 0, 0)].into(), at, true);
        runtime.refresh_poses(&mut world, &input, false).unwrap();
        input.step += 8;
        runtime.refresh_poses(&mut world, &input, false).unwrap();
        assert!(runtime.instances.contains_key(&id));
        input.step += 1;
        runtime.refresh_poses(&mut world, &input, false).unwrap();
        assert!(runtime.instances.is_empty());
    }
}
