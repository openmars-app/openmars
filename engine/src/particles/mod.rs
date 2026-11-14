//! Bounded scene-owned placement dust and local emissions. React/Go retain all
//! authoritative state; quiet snapshots reconcile without replaying bursts.
mod emissions;
mod local;
pub mod material;
use crate::structures::special::frame::Frame;
use bevy::{
    camera::{
        RenderTarget,
        visibility::{NoFrustumCulling, RenderLayers},
    },
    core_pipeline::{
        prepass::DepthPrepass,
        tonemapping::{DebandDither, Tonemapping},
    },
    light::{NotShadowCaster, NotShadowReceiver},
    math::{DQuat, DVec3},
    mesh::MeshTag,
    pbr::PreparedMaterial,
    prelude::*,
    render::{
        RenderApp,
        erased_render_asset::ErasedRenderAssets,
        mesh::RenderMesh,
        render_asset::RenderAssets,
        render_resource::*,
        renderer::{RenderDevice, RenderQueue},
        texture::GpuImage,
    },
};
use local::{Kind, Pool};
use material::{Palette, ParticleMaterial, Textures};
use openmars_scene::{landscape::LandscapeSnapshot, structure::StructureLayoutRef};
use std::collections::{BTreeMap, BTreeSet};
type Coordinate = (i32, i32, i32);
type Key = (Coordinate, Kind);
const MAX_PARTICLES: usize = 65536;
#[derive(Clone, Copy)]
struct Birth {
    step: u64,
}
struct Entry {
    root: Entity,
    nodes: Vec<Entity>,
    pool: Pool,
    seed: u32,
    birth: Birth,
    warm: u8,
    fresh: bool,
    computed: bool,
    color: u32,
    death: Option<u64>,
    frozen: Option<(u64, f64)>,
}
struct Vents {
    root: Entity,
    nodes: Vec<(Entity, usize, usize)>,
    outlets: Vec<emissions::Outlet>,
    layout: StructureLayoutRef,
    birth: Option<Birth>,
    warm: u8,
    fresh: bool,
    death: Option<u64>,
}
struct Shared {
    mesh: Handle<Mesh>,
    material: Handle<ParticleMaterial>,
    image: Handle<Image>,
    palette: UniformBuffer<Palette>,
}
struct Retired {
    buffer: Buffer,
    frames: u8,
}
pub struct Input {
    pub snapshot: std::sync::Arc<LandscapeSnapshot>,
    pub step: u64,
    pub motion_step: u64,
    pub reduced: bool,
    pub settled: bool,
    pub warm: Handle<Image>,
}
#[derive(Resource, Default)]
pub struct ParticleRuntime {
    entries: BTreeMap<Key, Entry>,
    vents: BTreeMap<Coordinate, Vents>,
    baseline: BTreeMap<Coordinate, String>,
    pending: BTreeSet<Coordinate>,
    births: BTreeMap<Coordinate, Birth>,
    shared: Option<Shared>,
    retired: Vec<Retired>,
    camera: Option<Entity>,
    ready: bool,
    pub revision: u64,
    pub spawns: u64,
    pub retirements: u64,
}
fn ambient(kind: &str) -> Option<Kind> {
    match kind {
        "volcano-tile" => Some(Kind::Volcano),
        "mining-tile" => Some(Kind::Mining),
        "mohole-tile" => Some(Kind::Mohole),
        _ => None,
    }
}
fn dusty(kind: &str) -> bool {
    !matches!(
        kind,
        "ocean-tile"
            | "greenery-tile"
            | "ecological-zone-tile"
            | "natural-preserve-tile"
            | "nuclear-zone-tile"
    )
}
fn duration(kind: &str, structure: bool) -> f64 {
    if structure || matches!(kind, "world-tree-tile" | "volcano-tile") {
        3.
    } else {
        match kind {
            "restricted-tile" => 2.,
            "mining-tile" => 2.5,
            "mohole-tile" => 2.8,
            _ => 2.6,
        }
    }
}
fn dust_color(kind: &str) -> u32 {
    match kind {
        "world-tree-tile" => 4,
        "restricted-tile" => 5,
        "volcano-tile" => 6,
        "mohole-tile" => 7,
        "mining-tile" => 8,
        _ => 0,
    }
}
fn palette() -> Palette {
    let mut colors = [Vec4::ZERO; 16];
    for kind in [Kind::Placement, Kind::Volcano, Kind::Mining, Kind::Mohole] {
        colors[kind.index() as usize] = Vec3::from_array(kind.color()).extend(1.);
    }
    for (i, c) in [
        [0.15, 0.3, 0.08],
        [0.5, 0.35, 0.2],
        [0.55, 0.18, 0.08],
        [0.35, 0.15, 0.1],
        [0.45, 0.3, 0.15],
    ]
    .into_iter()
    .enumerate()
    {
        colors[i + 4] = Vec3::from_array(c).extend(1.);
    }
    colors[9] = Vec4::from_array(LinearRgba::from(Srgba::rgb_u8(202, 213, 216)).to_f32_array());
    colors[10] = Vec4::from_array(LinearRgba::from(Srgba::rgb_u8(137, 144, 151)).to_f32_array());
    Palette { colors }
}
fn root(world: &mut World, frame: &Frame) -> Entity {
    world
        .spawn((
            Transform {
                translation: DVec3::from_array(frame.origin).as_vec3(),
                rotation: DQuat::from_array(frame.rotation).as_quat(),
                ..default()
            },
            Visibility::Inherited,
        ))
        .id()
}
fn sprite(world: &mut World, parent: Entity, shared: &Shared, order: [i64; 6]) -> Entity {
    world
        .spawn((
            Mesh3d(shared.mesh.clone()),
            MeshMaterial3d(shared.material.clone()),
            MeshTag::new(1 << 31),
            crate::transparent::TransparentOrder::particle(order),
            Transform::default(),
            NoFrustumCulling,
            NotShadowCaster,
            NotShadowReceiver,
            RenderLayers::layer(2),
            ChildOf(parent),
        ))
        .id()
}
fn actor_ready(world: &World, c: Coordinate, kind: &str) -> bool {
    match kind {
        "world-tree-tile" => world
            .resource::<crate::structures::world_tree::WorldTreeRuntime>()
            .covers(c),
        "mining-tile" | "restricted-tile" => world
            .resource::<crate::structures::sites::SiteRuntime>()
            .covers(c),
        "volcano-tile" | "mohole-tile" => world
            .resource::<crate::structures::special::SpecialRuntime>()
            .covers(c),
        "city-tile" => world
            .resource::<crate::structures::scene::StructureRuntime>()
            .covers(c),
        _ => true,
    }
}
fn set_pose(world: &mut World, node: Entity, position: DVec3, scale: f64, tag: u32) {
    world.entity_mut(node).insert((
        Transform::from_translation(position.as_vec3()).with_scale(Vec3::splat(scale as f32)),
        MeshTag::new(tag),
    ));
}
impl ParticleRuntime {
    pub fn ready(&self) -> bool {
        self.ready
    }
    pub fn progress(&self) -> Option<String> {
        (!self.ready).then(|| format!("Preparing local particles ({} sprites)", self.count()))
    }
    fn count(&self) -> usize {
        self.entries.values().map(|e| e.nodes.len()).sum::<usize>()
            + self.vents.values().map(|e| e.nodes.len()).sum::<usize>()
    }
    pub fn diagnostics(&self, world: &World) -> serde_json::Value {
        serde_json::json!({"imageMips":self.shared.as_ref().and_then(|s|world.resource::<Assets<Image>>().get(&s.image)).map(|i|i.texture_descriptor.mip_level_count),"imageSize":self.shared.as_ref().and_then(|s|world.resource::<Assets<Image>>().get(&s.image)).map(|i|[i.width(),i.height()]),"ventSample":self.vents.iter().map(|(c,v)|serde_json::json!({"coordinate":c,"warmFrames":v.warm,"sprites":v.nodes.len(),"sample":v.nodes.iter().take(2).map(|(node,_,_)|serde_json::json!({"node":node.to_string(),"position":world.get::<Transform>(*node).unwrap().translation.to_array(),"scale":world.get::<Transform>(*node).unwrap().scale.x,"tag":world.get::<MeshTag>(*node).unwrap().value})).collect::<Vec<_>>()})).collect::<Vec<_>>(),"ready":self.ready,"sprites":self.count(),"pools":self.entries.len(),"vents":self.vents.len(),"pending":self.pending.len(),"uniforms":usize::from(self.shared.is_some())+self.retired.len(),"spawns":self.spawns,"retiredUniforms":self.retirements,
   "paletteBuffer":self.shared.as_ref().and_then(|s|s.palette.buffer()).map(|b|format!("{:?}",b.id())),"sample":self.entries.iter().map(|((c,k),e)|serde_json::json!({"coordinate":c,"kind":format!("{k:?}"),"seed":e.seed,"born":e.birth.step,"poolBorn":e.pool.born(),"warmFrames":e.warm,"positions":e.pool.particles().iter().take(2).map(|p|p.position.to_array()).collect::<Vec<_>>() })).collect::<Vec<_>>()})
    }
    fn remove(&mut self, world: &mut World, key: Key) {
        if let Some(e) = self.entries.remove(&key) {
            world.despawn(e.root);
            self.revision += 1;
        }
    }
    fn update(
        &mut self,
        world: &mut World,
        input: Input,
        uploaded: bool,
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<(), String> {
        let scene = world
            .resource::<crate::scene::SceneState>()
            .0
            .as_ref()
            .ok_or("Missing particle scene")?;
        let mut desired = BTreeMap::new();
        let mut specs = BTreeMap::new();
        let mut nuclear = BTreeSet::new();
        for tile in scene.tiles.iter().filter(|t| crate::world::is_mars(t)) {
            let c = (tile.q, tile.r, tile.s);
            let kind = tile
                .occupant
                .clone()
                .or_else(|| tile.reserved_by.as_ref().map(|_| "restricted-tile".into()));
            if let Some(kind) = kind {
                if kind == "nuclear-zone-tile" {
                    nuclear.insert(c);
                }
                let seed = tile.visual.as_ref().map_or_else(
                    || {
                        openmars_scene::landscape::visual_seed(&format!(
                            "{}:{}",
                            scene.game_id, tile.id
                        ))
                    },
                    |v| v.seed,
                );
                specs.insert(c, (kind.clone(), seed));
                desired.insert(c, kind);
            }
        }
        if input.settled {
            self.pending.clear();
            self.births.clear();
            let bursts: Vec<_> = self
                .entries
                .keys()
                .filter(|(_, k)| *k == Kind::Placement)
                .copied()
                .collect();
            for key in bursts {
                self.remove(world, key);
            }
        } else {
            for (&c, kind) in &desired {
                if self.baseline.get(&c) != Some(kind) && dusty(kind) {
                    self.pending.insert(c);
                }
            }
        }
        self.baseline = desired.clone();
        self.pending
            .retain(|c| desired.get(c).is_some_and(|k| dusty(k)));
        let available: BTreeSet<_> = input
            .snapshot
            .sources()
            .iter()
            .map(|s| (s.coordinate[0], s.coordinate[1], s.coordinate[2]))
            .collect();
        for &c in &self.pending.clone() {
            if (available.contains(&c) || desired[&c] == "restricted-tile")
                && actor_ready(world, c, &desired[&c])
            {
                if !input.reduced {
                    self.births.insert(c, Birth { step: input.step });
                }
                self.pending.remove(&c);
            }
        }
        self.births.retain(|c, b| {
            desired.get(c).is_some_and(|k| {
                input.step.saturating_sub(b.step) < (duration(k, true) * 60.) as u64
            })
        });

        let mut wanted = BTreeMap::new();
        for (&c, (kind, seed)) in &specs {
            if !available.contains(&c) && kind != "restricted-tile" {
                continue;
            }
            if !actor_ready(world, c, kind) {
                continue;
            }
            if let Some(k) = ambient(kind) {
                let birth = self.births.get(&c).copied();
                let delay = match k {
                    Kind::Volcano => 72,
                    Kind::Mining => 54,
                    Kind::Mohole => 90,
                    _ => 0,
                };
                if birth.is_none_or(|b| input.step >= b.step + delay) {
                    wanted.insert((c, k), (*seed, birth, 0., k.index()));
                }
            }
            if let Some(b) = self.births.get(&c).copied().filter(|_| {
                !input.reduced
                    || self
                        .entries
                        .get(&(c, Kind::Placement))
                        .is_some_and(|e| e.frozen.is_none_or(|(at, _)| input.step < at + 9))
            }) {
                let structure = input
                    .snapshot
                    .sources()
                    .iter()
                    .find(|s| s.coordinate == [c.0, c.1, c.2])
                    .is_some_and(|s| s.layout.is_some());
                let seconds = duration(kind, structure);
                if input.step.saturating_sub(b.step) < (seconds * 60.) as u64 {
                    wanted.insert(
                        (c, Kind::Placement),
                        (*seed, Some(b), seconds, dust_color(kind)),
                    );
                }
            }
        }
        let remove: Vec<_> = self
            .entries
            .iter_mut()
            .filter_map(|(&key, e)| {
                if input.settled {
                    e.death = None;
                    if !wanted.contains_key(&key) {
                        return Some(key);
                    }
                }
                if wanted.get(&key).is_some_and(|s| s.0 == e.seed) {
                    e.death = None;
                    return None;
                }
                if nuclear.contains(&key.0)
                    && !input.settled
                    && world
                        .resource::<crate::nuclear::timeline::Timeline>()
                        .live(key.0)
                {
                    let start = world
                        .resource::<crate::nuclear::timeline::Timeline>()
                        .start(key.0)?;
                    let at = *e.death.get_or_insert(start.step);
                    if input.step < at + if input.reduced { 9 } else { 60 } {
                        return None;
                    }
                }
                Some(key)
            })
            .collect();
        for key in remove {
            self.remove(world, key);
        }
        let layouts: BTreeMap<_, _> = input
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
        let remove_vents: Vec<_> = self
            .vents
            .iter_mut()
            .filter_map(|(&c, v)| {
                if layouts.get(&c) == Some(&v.layout) {
                    v.death = None;
                    return None;
                }
                if nuclear.contains(&c)
                    && !input.settled
                    && world
                        .resource::<crate::nuclear::timeline::Timeline>()
                        .live(c)
                {
                    let start = world
                        .resource::<crate::nuclear::timeline::Timeline>()
                        .start(c)?;
                    let at = *v.death.get_or_insert(start.step);
                    if input.step < at + if input.reduced { 9 } else { 60 } {
                        return None;
                    }
                }
                Some(c)
            })
            .collect();
        for c in remove_vents {
            world.despawn(self.vents.remove(&c).unwrap().root);
        }
        let needed = !wanted.is_empty()
            || layouts
                .values()
                .any(|l| l.layout().fixtures.iter().any(|f| f.emitter.is_some()));
        let missing = wanted.keys().any(|k| !self.entries.contains_key(k))
            || layouts.iter().any(|(c, l)| {
                !self.vents.contains_key(c)
                    && l.layout().fixtures.iter().any(|f| f.emitter.is_some())
            });
        if missing && self.shared.is_none() {
            let Some(image) = world
                .resource::<crate::structures::texture_library::TextureLibrary>()
                .get(crate::structures::textures::Slot::Shared(5))
            else {
                self.ready = false;
                return Ok(());
            };
            let mesh = world
                .resource_mut::<Assets<Mesh>>()
                .add(Mesh::from(Rectangle::new(1., 1.)));
            let mut palette = UniformBuffer::from(palette());
            palette.set_label(Some("particle palette"));
            palette.write_buffer(device, queue);
            let material = world
                .resource_mut::<Assets<ParticleMaterial>>()
                .add(ParticleMaterial {
                    textures: Textures {
                        image: image.clone(),
                    },
                    parameters: palette.buffer().unwrap().clone(),
                });
            self.shared = Some(Shared {
                mesh,
                material,
                image,
                palette,
            });
        }
        if let Some(shared) = &self.shared {
            for (&key, &(seed, birth, seconds, color)) in &wanted {
                if self.entries.contains_key(&key) {
                    continue;
                }
                let mut frame =
                    Frame::new(input.snapshot.projection(), [key.0.0, key.0.1, key.0.2])?;
                if key.1 == Kind::Volcano {
                    for (p, n) in frame.origin.iter_mut().zip(frame.normal) {
                        *p += n * 0.14 * 0.85;
                    }
                }
                let at = if key.1 == Kind::Placement {
                    birth.unwrap().step
                } else {
                    input.motion_step
                };
                let count = crate::graphics::effect_count(world, usize::from(key.1.count())) as u16;
                let pool = Pool::new(
                    key.1,
                    seed,
                    count,
                    at,
                    birth.is_some(),
                    if seconds > 0. { seconds } else { 3. },
                )?;
                if self.count() + usize::from(count) > MAX_PARTICLES {
                    return Err("Particle scene working-set bound".into());
                }
                let parent = root(world, &frame);
                let nodes = (0..count)
                    .map(|i| {
                        sprite(
                            world,
                            parent,
                            shared,
                            [
                                i64::from(key.0.0),
                                i64::from(key.0.1),
                                i64::from(key.0.2),
                                i64::from(key.1.index()),
                                0,
                                i64::from(i),
                            ],
                        )
                    })
                    .collect();
                self.entries.insert(
                    key,
                    Entry {
                        root: parent,
                        nodes,
                        pool,
                        seed,
                        birth: birth.unwrap_or(Birth { step: input.step }),
                        warm: 0,
                        fresh: true,
                        computed: false,
                        color,
                        death: None,
                        frozen: None,
                    },
                );
                self.spawns += 1;
            }
            for (&c, layout) in &layouts {
                if self.vents.contains_key(&c) || !actor_ready(world, c, "city-tile") {
                    continue;
                }
                let frame = Frame::new(input.snapshot.projection(), [c.0, c.1, c.2])?;
                let mut outlets = emissions::prepare(
                    layout,
                    [c.0, c.1, c.2],
                    &frame,
                    input.snapshot.projection(),
                )?;
                for outlet in &mut outlets {
                    outlet
                        .phases
                        .truncate(crate::graphics::effect_count(world, outlet.phases.len()));
                }
                if outlets.is_empty() {
                    continue;
                }
                let count = outlets.iter().map(|o| o.phases.len()).sum::<usize>();
                if self.count() + count > MAX_PARTICLES {
                    return Err("Fixture particle scene bound".into());
                }
                let parent = root(world, &frame);
                let mut nodes = Vec::with_capacity(count);
                for (i, o) in outlets.iter().enumerate() {
                    for p in 0..o.phases.len() {
                        nodes.push((
                            sprite(
                                world,
                                parent,
                                shared,
                                [
                                    i64::from(c.0),
                                    i64::from(c.1),
                                    i64::from(c.2),
                                    9,
                                    i as i64,
                                    p as i64,
                                ],
                            ),
                            i,
                            p,
                        ));
                    }
                }
                self.vents.insert(
                    c,
                    Vents {
                        root: parent,
                        nodes,
                        outlets,
                        layout: layout.clone(),
                        birth: self.births.get(&c).copied(),
                        warm: 0,
                        fresh: true,
                        death: None,
                    },
                );
                self.spawns += 1;
            }
        }
        for (&(_, kind), e) in &mut self.entries {
            let target = if kind == Kind::Placement {
                input.step
            } else {
                input.motion_step
            };
            if kind == Kind::Placement && input.reduced {
                e.frozen.get_or_insert((input.step, 1.));
                e.computed = true;
            } else {
                e.frozen = None;
                e.computed = false;
                for _ in 0..4 {
                    if e.pool.advance(target)? {
                        e.computed = true;
                        break;
                    }
                }
            }
            let fade = e.death.map_or(1., |at| {
                if input.reduced {
                    (1. - input.step.saturating_sub(at) as f64 / 9.).clamp(0., 1.)
                } else {
                    1.
                }
            });
            for (node, p) in e.nodes.iter().zip(e.pool.particles()) {
                let mut tag = p.tag(kind, true);
                tag = (tag & !(15 << 26)) | (e.color << 26);
                let mut opacity = p.opacity * fade;
                if let Some((at, _)) = e.frozen {
                    opacity *= (1. - input.step.saturating_sub(at) as f64 / 9.).clamp(0., 1.);
                }
                tag = (tag & !4095) | ((opacity.clamp(0., 1.) * 4095.).round() as u32);
                set_pose(world, *node, p.position, p.scale, tag);
            }
            if e.fresh {
                e.fresh = false;
            } else if uploaded && e.computed && e.warm < 2 {
                e.warm += 1;
                if e.warm == 2 {
                    for &node in &e.nodes {
                        world.entity_mut(node).insert(RenderLayers::default());
                    }
                }
            }
        }
        for v in self.vents.values_mut() {
            if input.settled {
                v.birth = None;
                v.death = None;
            }
            let fade = v.birth.map_or(1., |b| {
                (input.step.saturating_sub(b.step) as f64 / 60. - 1.12).clamp(0., 1.)
            });
            for &(node, o, p) in &v.nodes {
                let mut pose =
                    emissions::sample(&v.outlets[o], p, input.motion_step as f64 / 60., fade)?;
                if input.reduced
                    && let Some(at) = v.death
                {
                    pose.opacity *= (1. - input.step.saturating_sub(at) as f64 / 9.).clamp(0., 1.);
                }
                set_pose(
                    world,
                    node,
                    pose.position,
                    pose.scale,
                    emissions::tag(pose, v.outlets[o].vapor),
                );
            }
            if v.fresh {
                v.fresh = false;
            } else if uploaded && v.warm < 2 {
                v.warm += 1;
                if v.warm == 2 {
                    for &(node, _, _) in &v.nodes {
                        world.entity_mut(node).insert(RenderLayers::default());
                    }
                }
            }
        }
        let warming =
            self.entries.values().any(|e| e.warm < 2) || self.vents.values().any(|e| e.warm < 2);
        if warming && self.camera.is_none() {
            let shared = world
                .query::<(&Camera, &RenderLayers)>()
                .iter(world)
                .any(|(c, l)| c.is_active && l == &RenderLayers::layer(2));
            if !shared {
                self.camera = Some(
                    world
                        .spawn((
                            crate::terrain::surface::SurfaceCamera::Warm,
                            Camera3d::default(),
                            DepthPrepass,
                            Camera {
                                order: -4,
                                ..default()
                            },
                            RenderTarget::Image(input.warm.into()),
                            Msaa::Off,
                            Tonemapping::None,
                            DebandDither::Disabled,
                            RenderLayers::layer(2),
                            Transform::from_xyz(0., 0., 6.8).looking_at(Vec3::ZERO, Vec3::Y),
                        ))
                        .id(),
                );
            }
        } else if !warming && let Some(c) = self.camera.take() {
            world.despawn(c);
        }
        self.ready = !warming
            && self.pending.is_empty()
            && wanted.keys().all(|k| self.entries.contains_key(k))
            && layouts
                .iter()
                .filter(|(_, l)| l.layout().fixtures.iter().any(|f| f.emitter.is_some()))
                .all(|(c, _)| self.vents.contains_key(c));
        if self.entries.is_empty()
            && self.vents.is_empty()
            && !needed
            && let Some(s) = self.shared.take()
        {
            self.retired.push(Retired {
                buffer: s.palette.buffer().unwrap().clone(),
                frames: 2,
            });
        }
        Ok(())
    }
}
/// Source Tile entrance affects published actors; warm draws remain full size.
pub fn apply_entry(world: &mut World) {
    world.resource_scope(|world, runtime: Mut<ParticleRuntime>| {
        for (&(coordinate, _), entry) in &runtime.entries {
            let scale = if entry.warm < 2 {
                1.
            } else {
                crate::entry::tile_scale(world, coordinate)
            };
            world.get_mut::<Transform>(entry.root).unwrap().scale =
                Vec3::splat(scale.max(0.000001));
            crate::entry::publish(world, entry.root, scale);
        }
        for (&coordinate, vent) in &runtime.vents {
            let scale = if vent.warm < 2 {
                1.
            } else if runtime
                .baseline
                .get(&coordinate)
                .is_some_and(|kind| kind == "city-tile")
            {
                if world
                    .resource::<crate::entry::WorldEntrance>()
                    .tiles
                    .shown()
                {
                    1.
                } else {
                    0.
                }
            } else {
                crate::entry::tile_scale(world, coordinate)
            };
            world.get_mut::<Transform>(vent.root).unwrap().scale = Vec3::splat(scale.max(0.000001));
            crate::entry::publish(world, vent.root, scale);
        }
    });
}

pub fn prepare_queued(
    app: &mut App,
    input: Option<Input>,
    device: &RenderDevice,
    queue: &RenderQueue,
) -> Result<(), String> {
    let _profile = crate::profiling::span(app.world(), "particles.prepare");
    let uploaded = app
        .world()
        .resource::<ParticleRuntime>()
        .shared
        .as_ref()
        .is_some_and(|s| {
            let w = app.sub_app(RenderApp).world();
            w.resource::<PipelineCache>()
                .waiting_pipelines()
                .next()
                .is_none()
                && w.resource::<RenderAssets<RenderMesh>>()
                    .get(s.mesh.id())
                    .is_some()
                && w.resource::<ErasedRenderAssets<PreparedMaterial>>()
                    .get(s.material.id().untyped())
                    .is_some()
                && w.resource::<RenderAssets<GpuImage>>()
                    .get(s.image.id())
                    .is_some()
        });
    let mut runtime = app
        .world_mut()
        .remove_resource::<ParticleRuntime>()
        .unwrap();
    for retired in &mut runtime.retired {
        retired.frames = retired.frames.saturating_sub(1);
    }
    runtime.retired.retain(|r| {
        if r.frames == 0 {
            r.buffer.destroy();
            runtime.retirements += 1;
            false
        } else {
            true
        }
    });
    let result = if let Some(input) = input {
        runtime.update(app.world_mut(), input, uploaded, device, queue)
    } else {
        Ok(())
    };
    app.world_mut().insert_resource(runtime);
    result
}
pub fn reset(world: &mut World) {
    let Some(mut r) = world.remove_resource::<ParticleRuntime>() else {
        return;
    };
    for (_, e) in r.entries {
        world.despawn(e.root);
    }
    for (_, v) in r.vents {
        world.despawn(v.root);
    }
    if let Some(c) = r.camera {
        world.despawn(c);
    }
    if let Some(s) = r.shared {
        r.retired.push(Retired {
            buffer: s.palette.buffer().unwrap().clone(),
            frames: 2,
        });
    }
    world.insert_resource(ParticleRuntime {
        retired: r.retired,
        spawns: r.spawns,
        retirements: r.retirements,
        ..default()
    });
}
pub struct ParticlePlugin;
impl Plugin for ParticlePlugin {
    fn build(&self, app: &mut App) {
        app.add_plugins(material::ParticleMaterialPlugin)
            .init_resource::<ParticleRuntime>();
    }
}
