//! Persistent source craters. Quiet snapshots publish final surfaces without replay.
pub mod blast;
pub mod blast_compositor;
pub mod blast_geometry;
pub mod blast_material;
pub(crate) mod collapse;
pub mod debris;
pub mod debris_material;
mod debris_scene;
mod geometry;
pub mod material;
pub mod timeline;
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
use material::{NuclearMaskMaterial, NuclearMaterial, NuclearPass, NuclearUniform, Textures};
use openmars_scene::{
    SceneSnapshot,
    geometry::{
        GeometryError,
        nuclear::NuclearProfile,
        nuclear_mesh::{DiscKind, DiscWork},
        nuclear_site::{BoardFrame, CraterGround},
    },
    landscape::LandscapeSnapshot,
};
use std::{
    collections::{BTreeMap, BTreeSet},
    num::NonZeroUsize,
    sync::Arc,
};
type Coordinate = (i32, i32, i32);
const MAX_SITES: usize = 512;
const MAX_BYTES: usize = 64 * 1024 * 1024;
// Includes all three source arrays, their final vertex layouts and conversion scratch.
const BUILD_BYTES: usize = 4 * 1024 * 1024;
const CPU_TURNS: usize = 16;
const CPU_BUDGET: std::time::Duration = std::time::Duration::from_millis(4);
#[derive(Clone, Debug, PartialEq, Eq)]
struct Spec {
    seed: u32,
    source: [u8; 32],
    nearby: Vec<(Coordinate, bool)>,
}
pub struct Input {
    pub snapshot: Arc<LandscapeSnapshot>,
    pub landscape_ready: bool,
    pub step: u64,
    pub motion_step: u64,
    pub reduced: bool,
    pub warm: Handle<Image>,
    pub warm_mask: Handle<Image>,
    pub climate: openmars_scene::climate::ClimateState,
}
struct Plot {
    spec: Spec,
    root: Entity,
    parts: Vec<(Entity, bool)>,
    meshes: Vec<Handle<Mesh>>,
    materials: Vec<Handle<NuclearMaterial>>,
    mask: Handle<NuclearMaskMaterial>,
    uniform: UniformBuffer<NuclearUniform>,
    bytes: usize,
    warm: u32,
    debris: Vec<debris_scene::Instance>,
    debris_shared: Arc<debris_scene::Shared>,
    visible: bool,
    debris_settled: bool,
}
struct Building {
    coordinate: Coordinate,
    spec: Spec,
    frame: BoardFrame,
    nearby: Vec<BoardFrame>,
    snapshot: Arc<LandscapeSnapshot>,
    profile: NuclearProfile,
    disc: DiscWork,
    index: usize,
    meshes: Vec<Mesh>,
    fragment_work: debris::FragmentWork,
    fragments: Option<Vec<debris::Fragment>>,
}
impl Building {
    fn advance_cpu(&mut self) -> Result<(), String> {
        let kinds = [DiscKind::Mask, DiscKind::Bowl, DiscKind::Scorch];
        // Spend a bounded CPU slice independently of slow GPU submissions.
        // Raw meshes remain unpublished until the admitted render update uploads them.
        let started = bevy::platform::time::Instant::now();
        for turn in 0..CPU_TURNS {
            if turn > 0 && started.elapsed() >= CPU_BUDGET {
                break;
            }
            if self.fragments.is_some() {
                break;
            }
            let terrain = |point| {
                self.snapshot
                    .sampler()
                    .sample(point)
                    .map(|s| s.height)
                    .map_err(|_| GeometryError::InvalidHeight)
            };
            let ground = CraterGround {
                frame: &self.frame,
                nearby: &self.nearby,
                radius: self.snapshot.projection().radius(),
                terrain: &terrain,
            };
            if self.index == 3 {
                if self.fragment_work.advance(&self.profile, &ground)? {
                    self.fragments = self.fragment_work.take_ready();
                }
                continue;
            }
            if self
                .disc
                .step(NonZeroUsize::new(256).unwrap(), |x, y| ground.sample(x, y))
                .map_err(|e| e.to_string())?
            {
                let data = self.disc.take_ready().ok_or("Crater disc incomplete")?;
                self.meshes.push(geometry::mesh(
                    data,
                    &self.frame,
                    self.snapshot.projection().radius(),
                ));
                self.index += 1;
                if self.index < 3 {
                    self.disc = DiscWork::new(
                        &self.profile,
                        kinds[self.index],
                        self.frame.tile_frame().map_err(|e| e.to_string())?,
                    )
                    .map_err(|e| e.to_string())?;
                }
            }
        }
        Ok(())
    }
}
/// Geometry can progress while the GPU is still rendering the prior frame.
/// Cancellations discard only unpublished CPU storage; GPU retirement stays gated.
pub fn advance_cpu(world: &mut World) -> Result<(), String> {
    let mut runtime = world
        .remove_resource::<NuclearRuntime>()
        .ok_or("Missing crater runtime")?;
    if runtime.building.as_ref().is_some_and(|b| {
        world
            .resource::<crate::scene::SceneState>()
            .0
            .as_ref()
            .is_none_or(|s| {
                !s.tiles.iter().any(|t| {
                    crate::world::is_mars(t)
                        && (t.q, t.r, t.s) == b.coordinate
                        && t.occupant.as_deref() == Some("nuclear-zone-tile")
                })
            })
    }) {
        runtime.building = None;
    }
    let result = runtime
        .building
        .as_mut()
        .map_or(Ok(()), Building::advance_cpu);
    world.insert_resource(runtime);
    result
}
struct Retired {
    plot: Plot,
    frames: u8,
}
struct RetiredHighlight {
    buffer: Buffer,
    frames: u8,
}
#[derive(Resource, Default)]
pub struct NuclearRuntime {
    plots: BTreeMap<Coordinate, Plot>,
    pending: BTreeMap<Coordinate, Plot>,
    building: Option<Building>,
    retired: Vec<Retired>,
    retired_highlights: Vec<RetiredHighlight>,
    highlights: Option<UniformBuffer<crate::terrain::materials::HighlightUniform>>,
    debris: Option<Arc<debris_scene::Shared>>,
    cameras: Vec<Entity>,
    ready: bool,
    pub revision: u64,
    builds: u64,
    writes: u64,
    retirements: u64,
}
fn desired(
    scene: &SceneSnapshot,
    snapshot: &LandscapeSnapshot,
) -> Result<BTreeMap<Coordinate, Spec>, String> {
    let frames: Vec<_> = scene
        .tiles
        .iter()
        .filter(|t| crate::world::is_mars(t))
        .map(|t| {
            let allowed = (t.occupant.is_none() && t.reserved_by.is_none())
                || t.occupant.as_deref() == Some("greenery-tile");
            BoardFrame::new(snapshot.projection(), [t.q, t.r, t.s], allowed)
                .map_err(|e| e.to_string())
        })
        .collect::<Result<_, _>>()?;
    if frames.len() > MAX_SITES {
        return Err("Crater board frame limit exceeded".into());
    }
    let mut result = BTreeMap::new();
    for t in scene
        .tiles
        .iter()
        .filter(|t| crate::world::is_mars(t) && t.occupant.as_deref() == Some("nuclear-zone-tile"))
    {
        let frame = frames
            .iter()
            .find(|f| f.coordinate == [t.q, t.r, t.s])
            .ok_or("Crater frame missing")?;
        let nearby = frames
            .iter()
            .filter(|f| {
                f.origin
                    .iter()
                    .zip(frame.origin)
                    .map(|(a, b)| (a - b).powi(2))
                    .sum::<f64>()
                    < 0.36
            })
            .map(|f| {
                (
                    (f.coordinate[0], f.coordinate[1], f.coordinate[2]),
                    f.allowed,
                )
            })
            .collect();
        result.insert(
            (t.q, t.r, t.s),
            Spec {
                seed: openmars_scene::landscape::visual_seed(&format!(
                    "{}:{},{},{}",
                    scene.game_id, t.q, t.r, t.s
                )),
                source: snapshot.source_identity(),
                nearby,
            },
        );
    }
    Ok(result)
}
impl NuclearRuntime {
    pub fn ready(&self) -> bool {
        self.ready
    }
    pub fn covers(&self, c: Coordinate) -> bool {
        self.plots.contains_key(&c)
    }
    pub fn progress(&self) -> Option<String> {
        (!self.ready).then(|| format!("Preparing craters ({} sites)", self.plots.len()))
    }
    pub fn diagnostics(&self) -> serde_json::Value {
        serde_json::json!({"ready":self.ready,"sites":self.plots.len(),"pending":self.pending.len(),"building":self.building.is_some(),"fragmentCandidates":self.building.as_ref().map_or(0,|b|b.fragment_work.progress()),"meshBytes":self.bytes(),"geometryBuilds":self.builds,"uniformWrites":self.writes,"retiredUniforms":self.retirements,"uniforms":self.plots.len()+self.pending.len()+self.retired.len()+usize::from(self.highlights.is_some())+self.retired_highlights.len(),"sample":self.plots.iter().map(|(c,p)|serde_json::json!({"coordinate":c,"seed":p.spec.seed,"entity":p.root.to_string(),"buffer":format!("{:?}",p.uniform.buffer().unwrap().id()),"meshes":p.meshes.iter().map(|m|format!("{:?}",m.id())).collect::<Vec<_>>(),"warmFrames":p.warm,"coverage":p.uniform.get().surface.z,"entryScale":p.uniform.get().entrance.w,"fragments":p.debris.len(),"fragmentSample":p.debris.iter().take(4).map(|f|serde_json::json!({"entity":f.root.to_string(),"settledPosition":f.fragment.end.to_array(),"seedColor":f.fragment.color})).collect::<Vec<_>>()})).collect::<Vec<_>>()})
    }
    pub fn debris_poses(&self, world: &World) -> serde_json::Value {
        serde_json::json!(self.plots.iter().flat_map(|(c,p)|p.debris.iter().take(4).filter_map(move |f|world.get::<Transform>(f.root).map(|t|serde_json::json!({"coordinate":c,"entity":f.root.to_string(),"position":t.translation.to_array(),"rotation":t.rotation.to_array(),"scale":t.scale.to_array(),"tag":f.children.first().and_then(|c|world.get::<bevy::mesh::MeshTag>(*c)).map(|t|t.value)})))).take(64).collect::<Vec<_>>())
    }
    fn bytes(&self) -> usize {
        self.plots
            .values()
            .chain(self.pending.values())
            .map(|p| p.bytes)
            .sum::<usize>()
            + self.retired.iter().map(|p| p.plot.bytes).sum::<usize>()
            + usize::from(self.building.is_some()) * BUILD_BYTES
    }
    fn retire(&mut self, world: &mut World, plot: Plot) {
        world.despawn(plot.root);
        self.retired.push(Retired { plot, frames: 2 });
        self.revision += 1;
    }
    fn update(
        &mut self,
        world: &mut World,
        input: Input,
        uploaded: &BTreeSet<Entity>,
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<(), String> {
        let Some(scene) = world.resource::<crate::scene::SceneState>().0.as_ref() else {
            return Ok(());
        };
        let wanted = desired(scene, &input.snapshot)?;
        for c in self
            .plots
            .keys()
            .filter(|c| !wanted.contains_key(c))
            .copied()
            .collect::<Vec<_>>()
        {
            let p = self.plots.remove(&c).unwrap();
            self.retire(world, p);
        }
        for c in self
            .pending
            .iter()
            .filter(|(c, p)| wanted.get(c) != Some(&p.spec))
            .map(|(c, _)| *c)
            .collect::<Vec<_>>()
        {
            let p = self.pending.remove(&c).unwrap();
            self.retire(world, p);
        }
        if self
            .building
            .as_ref()
            .is_some_and(|b| wanted.get(&b.coordinate) != Some(&b.spec))
        {
            self.building = None;
        }
        let mars = crate::assets::texture(world, "textures/planets/mars/surface");
        if !wanted.is_empty() && mars.is_none() {
            self.ready = false;
            return Ok(());
        }
        if !wanted.is_empty() && self.highlights.is_none() {
            let mut u: UniformBuffer<crate::terrain::materials::HighlightUniform> = world
                .resource::<crate::terrain::grid::GridRuntime>()
                .uniform()
                .into();
            u.set_label(Some("crater highlights"));
            u.write_buffer(device, queue);
            self.highlights = Some(u);
        }
        if self.building.is_none()
            && self.plots.len() + self.pending.len() + self.retired.len() < MAX_SITES * 3
            && let Some((&c, spec)) = wanted.iter().find(|(c, s)| {
                !self.pending.contains_key(c)
                    && self.plots.get(c).is_none_or(|p| &p.spec != *s)
                    && input.snapshot.sources().iter().any(|source| {
                        source.coordinate == [c.0, c.1, c.2]
                            && source.kind == openmars_scene::landscape::LandscapeKind::Excluded
                    })
            })
        {
            if self.bytes() + BUILD_BYTES > MAX_BYTES {
                return Err("Crater working-set limit exceeded".into());
            }
            let frame = BoardFrame::new(input.snapshot.projection(), [c.0, c.1, c.2], true)
                .map_err(|e| e.to_string())?;
            let nearby = spec
                .nearby
                .iter()
                .map(|(c, a)| {
                    BoardFrame::new(input.snapshot.projection(), [c.0, c.1, c.2], *a)
                        .map_err(|e| e.to_string())
                })
                .collect::<Result<_, _>>()?;
            let profile = NuclearProfile::new(spec.seed);
            let disc = DiscWork::new(
                &profile,
                DiscKind::Mask,
                frame.tile_frame().map_err(|e| e.to_string())?,
            )
            .map_err(|e| e.to_string())?;
            self.building = Some(Building {
                coordinate: c,
                spec: spec.clone(),
                frame,
                nearby,
                snapshot: input.snapshot.clone(),
                profile,
                disc,
                index: 0,
                meshes: Vec::with_capacity(3),
                fragment_work: debris::FragmentWork::new(spec.seed),
                fragments: None,
            });
        }
        if self.debris.is_none() && !wanted.is_empty() {
            self.debris = debris_scene::prepare(world);
        }
        if self
            .building
            .as_ref()
            .is_some_and(|b| b.fragments.is_some())
            && let Some(shared) = self.debris.clone()
        {
            let b = self.building.take().unwrap();
            let fragments = b.fragments.unwrap();
            let fragment_bytes = debris_scene::bytes(&shared, fragments.len());
            let bytes: usize = b
                .meshes
                .iter()
                .map(|m| {
                    m.get_vertex_buffer_size() + m.get_index_buffer_bytes().map_or(0, |v| v.len())
                })
                .sum::<usize>()
                + fragment_bytes;
            if self.bytes() + bytes > MAX_BYTES {
                return Err("Crater fragment working-set limit exceeded".into());
            }
            let mut uniform = UniformBuffer::from(NuclearUniform {
                surface: Vec4::new(b.spec.seed as f32, (1. - input.climate.sky) as f32, 1., 0.),
                gouges: b
                    .profile
                    .gouges
                    .map(|v| Vec4::from_array(v.map(|v| v as f32))),
                entrance: Vec3::from_array(b.frame.origin.map(|v| v as f32)).extend(1.),
            });
            uniform.set_label(Some("crater shared parameters"));
            uniform.write_buffer(device, queue);
            let meshes: Vec<_> = b
                .meshes
                .into_iter()
                .map(|m| world.resource_mut::<Assets<Mesh>>().add(m))
                .collect();
            let data = world.resource::<crate::terrain::grid::GridRuntime>().data();
            let atmosphere = world
                .resource::<crate::atmosphere::AtmosphereRuntime>()
                .binding()?;
            let materials: Vec<_> = [NuclearPass::Bowl, NuclearPass::Scorch]
                .into_iter()
                .map(|pass| {
                    world
                        .resource_mut::<Assets<NuclearMaterial>>()
                        .add(NuclearMaterial {
                            atmosphere: atmosphere.clone(),
                            parameters: uniform.buffer().unwrap().clone(),
                            highlights: self.highlights.as_ref().unwrap().buffer().unwrap().clone(),
                            textures: Textures {
                                pass,
                                mars: mars.as_ref().unwrap().clone(),
                                highlight_data: data.clone(),
                            },
                        })
                })
                .collect();
            let mask =
                world
                    .resource_mut::<Assets<NuclearMaskMaterial>>()
                    .add(NuclearMaskMaterial {
                        parameters: uniform.buffer().unwrap().clone(),
                    });
            let root = world
                .spawn((Transform::default(), Visibility::Inherited))
                .id();
            let mut parts = Vec::new();
            for (i, mat) in materials.iter().enumerate() {
                let e = world
                    .spawn((
                        Mesh3d(meshes[i + 1].clone()),
                        MeshMaterial3d(mat.clone()),
                        Transform::default(),
                        NoFrustumCulling,
                        NotShadowCaster,
                        NotShadowReceiver,
                        RenderLayers::layer(2),
                        ChildOf(root),
                    ))
                    .id();
                parts.push((e, false));
            }
            let e = world
                .spawn((
                    Mesh3d(meshes[0].clone()),
                    MeshMaterial3d(mask.clone()),
                    Transform::default(),
                    NoFrustumCulling,
                    NotShadowCaster,
                    NotShadowReceiver,
                    RenderLayers::layer(3),
                    ChildOf(root),
                ))
                .id();
            parts.push((e, true));
            let debris = debris_scene::spawn(world, root, fragments, &shared, &mut parts);
            self.pending.insert(
                b.coordinate,
                Plot {
                    spec: b.spec,
                    root,
                    parts,
                    meshes,
                    materials,
                    mask,
                    uniform,
                    bytes,
                    warm: 0,
                    debris,
                    debris_shared: shared,
                    visible: false,
                    debris_settled: false,
                },
            );
            self.builds += 1;
        }
        for p in self.pending.values_mut() {
            if p.warm < 2 && uploaded.contains(&p.root) {
                p.warm += 1;
            }
        }
        for c in self
            .pending
            .iter()
            .filter(|(_, p)| p.warm >= 2)
            .map(|(c, _)| *c)
            .collect::<Vec<_>>()
        {
            let mut p = self.pending.remove(&c).unwrap();
            // Loading snapshots warm each actual mesh, then park it until the
            // whole board is prepared. Repeatedly drawing every finished crater
            // behind the loading screen makes dense saved boards progressively
            // slower. Live additions still publish as soon as they are ready.
            if world.resource::<crate::scene::PresentationState>().settling {
                for &(e, _) in &p.parts {
                    world.entity_mut(e).insert(RenderLayers::none());
                }
            } else {
                show(world, &mut p);
            }
            if let Some(old) = self.plots.insert(c, p) {
                self.retire(world, old);
            }
            self.revision += 1;
        }
        let ready = if input.landscape_ready {
            self.plots
                .iter()
                .filter(|(c, p)| {
                    wanted.get(c) == Some(&p.spec)
                        && (input.reduced
                            || !world.resource::<timeline::Timeline>().live(**c)
                            || world.resource::<blast::BlastRuntime>().allows(**c))
                })
                .map(|(c, _)| *c)
                .collect()
        } else {
            BTreeSet::new()
        };
        world.resource_mut::<timeline::Timeline>().advance(
            &ready,
            timeline::Start {
                step: input.step,
                motion_step: input.motion_step,
            },
            input.reduced,
        );
        if let Some(u) = &mut self.highlights {
            u.set(
                world
                    .resource::<crate::terrain::grid::GridRuntime>()
                    .uniform(),
            );
            u.write_buffer(device, queue);
        }
        let data = world.resource::<crate::terrain::grid::GridRuntime>().data();
        for (c, p) in self.plots.iter_mut().chain(self.pending.iter_mut()) {
            let live = world.resource::<timeline::Timeline>().live(*c);
            if live || !p.debris_settled {
                debris_scene::update(world, *c, &p.debris, input.step, input.reduced, p.warm >= 2);
                p.debris_settled = !live && p.warm >= 2;
            }
            p.uniform.get_mut().surface.y = (1. - input.climate.sky) as f32;
            p.uniform.get_mut().entrance.w = if p.warm < 2 {
                1.
            } else {
                crate::entry::tile_scale(world, *c)
            };
            p.uniform.get_mut().surface.z = if p.warm < 2 {
                1.
            } else {
                world
                    .resource::<timeline::Timeline>()
                    .coverage(*c, input.step, input.reduced)
            };
            p.uniform.write_buffer(device, queue);
            self.writes += 1;
            for h in &p.materials {
                let mut assets = world.resource_mut::<Assets<NuclearMaterial>>();
                if assets.get(h).unwrap().textures.highlight_data != data {
                    assets.get_mut(h).unwrap().textures.highlight_data = data.clone();
                }
            }
        }
        if !self.pending.is_empty() {
            ensure_cameras(world, &mut self.cameras, input.warm, input.warm_mask);
        } else {
            for e in self.cameras.drain(..) {
                world.despawn(e);
            }
        }
        self.ready = world.resource::<blast::BlastRuntime>().ready()
            && self.building.is_none()
            && self.pending.is_empty()
            && wanted
                .iter()
                .all(|(c, s)| self.plots.get(c).is_some_and(|p| &p.spec == s));
        if self.ready {
            let mut changed = false;
            for p in self.plots.values_mut().filter(|p| !p.visible) {
                show(world, p);
                changed = true;
            }
            self.revision += u64::from(changed);
        }
        if self.plots.is_empty() && self.pending.is_empty() && self.building.is_none() {
            self.debris = None;
            if let Some(u) = self.highlights.take() {
                self.retired_highlights.push(RetiredHighlight {
                    buffer: u.buffer().unwrap().clone(),
                    frames: 2,
                });
            }
        }
        Ok(())
    }
}
fn show(world: &mut World, plot: &mut Plot) {
    for &(entity, mask) in &plot.parts {
        world.entity_mut(entity).insert(if mask {
            RenderLayers::layer(1)
        } else {
            RenderLayers::default()
        });
    }
    plot.visible = true;
}
fn ensure_cameras(
    world: &mut World,
    cameras: &mut Vec<Entity>,
    warm: Handle<Image>,
    mask: Handle<Image>,
) {
    for (layer, target) in [(2, warm), (3, mask)] {
        if world
            .query::<(&Camera, &RenderLayers)>()
            .iter(world)
            .any(|(c, l)| c.is_active && l == &RenderLayers::layer(layer))
        {
            continue;
        }
        let mut e = world.spawn((
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
            e.insert((
                bevy::core_pipeline::prepass::DepthPrepass,
                crate::terrain::surface::SurfaceCamera::Warm,
            ));
        } else {
            e.insert((
                bevy::camera::CompositingSpace::Linear,
                crate::terrain::surface::SurfaceCamera::WarmMask,
            ));
        }
        cameras.push(e.id());
    }
}
/// Source crater surfaces inherit Tile visibility, while the independent
/// NuclearDebrisRenderer remains outside that source Tile group.
pub fn apply_entry(world: &mut World) {
    world.resource_scope(|world, runtime: Mut<NuclearRuntime>| {
        for (&coordinate, plot) in runtime.plots.iter().chain(runtime.pending.iter()) {
            let scale = if plot.warm < 2 {
                1.
            } else {
                crate::entry::tile_scale(world, coordinate)
            };
            for &(part, _) in &plot.parts {
                if world
                    .get::<ChildOf>(part)
                    .is_some_and(|parent| parent.parent() == plot.root)
                {
                    crate::entry::publish(world, part, scale);
                }
            }
        }
    });
}

pub fn prepare_queued(
    app: &mut App,
    input: Option<Input>,
    device: &RenderDevice,
    queue: &RenderQueue,
) -> Result<(), String> {
    let _profile = crate::profiling::span(app.world(), "nuclear.prepare");
    let mut uploaded = BTreeSet::new();
    let render = app.sub_app(RenderApp).world();
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
        for p in app.world().resource::<NuclearRuntime>().pending.values() {
            if p.meshes.iter().all(|h| meshes.get(h.id()).is_some())
                && p.materials
                    .iter()
                    .all(|h| materials.get(h.id().untyped()).is_some())
                && materials.get(p.mask.id().untyped()).is_some()
                && materials
                    .get(p.debris_shared.material.id().untyped())
                    .is_some()
                && p.debris_shared
                    .meshes
                    .iter()
                    .all(|h| meshes.get(h.id()).is_some())
            {
                uploaded.insert(p.root);
            }
        }
    }
    let mut runtime = app.world_mut().remove_resource::<NuclearRuntime>().unwrap();
    for r in &mut runtime.retired {
        r.frames = r.frames.saturating_sub(1);
    }
    runtime.retired.retain(|r| {
        if r.frames == 0 {
            r.plot.uniform.buffer().unwrap().destroy();
            runtime.retirements += 1;
            false
        } else {
            true
        }
    });
    for r in &mut runtime.retired_highlights {
        r.frames = r.frames.saturating_sub(1);
    }
    runtime.retired_highlights.retain(|r| {
        if r.frames == 0 {
            r.buffer.destroy();
            runtime.retirements += 1;
            false
        } else {
            true
        }
    });
    let result = if let Some(input) = input {
        runtime.update(app.world_mut(), input, &uploaded, device, queue)
    } else {
        Ok(())
    };
    app.world_mut().insert_resource(runtime);
    result
}
pub fn reset(world: &mut World) {
    blast::reset(world);
    world.insert_resource(timeline::Timeline::default());
    let Some(mut r) = world.remove_resource::<NuclearRuntime>() else {
        return;
    };
    for (_, p) in std::mem::take(&mut r.plots)
        .into_iter()
        .chain(std::mem::take(&mut r.pending))
    {
        r.retire(world, p);
    }
    if let Some(u) = r.highlights.take() {
        r.retired_highlights.push(RetiredHighlight {
            buffer: u.buffer().unwrap().clone(),
            frames: 2,
        });
    }
    for e in r.cameras.drain(..) {
        world.despawn(e);
    }
    world.insert_resource(NuclearRuntime {
        retired: r.retired,
        retired_highlights: r.retired_highlights,
        builds: r.builds,
        writes: r.writes,
        retirements: r.retirements,
        ..default()
    });
}
pub struct NuclearPlugin;
impl Plugin for NuclearPlugin {
    fn build(&self, app: &mut App) {
        app.add_plugins((
            material::NuclearMaterialPlugin,
            blast::BlastPlugin,
            debris_material::DebrisMaterialPlugin,
        ))
        .init_resource::<NuclearRuntime>()
        .init_resource::<timeline::Timeline>();
    }
}
#[cfg(test)]
mod cpu_tests {
    use super::*;
    fn preparing_world() -> World {
        use openmars_scene::{
            SceneTile,
            geometry::BodyProjection,
            landscape::{
                BodyId, LandscapeBuilder, LandscapeInput, LandscapeKind, LandscapeSource,
                PreparationState,
            },
        };
        let mut builder = LandscapeBuilder::new();
        builder
            .begin(
                LandscapeInput::new(
                    BodyId::Mars,
                    BodyProjection::MARS,
                    1,
                    37,
                    None,
                    vec![LandscapeSource {
                        coordinate: [0, 0, 0],
                        kind: LandscapeKind::Excluded,
                        seed: 0,
                        layout: None,
                    }],
                )
                .unwrap(),
            )
            .unwrap();
        while matches!(builder.state(), PreparationState::Preparing { .. }) {
            builder.step(NonZeroUsize::new(1024).unwrap()).unwrap();
        }
        let snapshot = builder.take_ready().unwrap();
        let frame = BoardFrame::new(snapshot.projection(), [0, 0, 0], true).unwrap();
        let profile = NuclearProfile::new(37);
        let disc = DiscWork::new(&profile, DiscKind::Mask, frame.tile_frame().unwrap()).unwrap();
        let building = Building {
            coordinate: (0, 0, 0),
            spec: Spec {
                seed: 37,
                source: snapshot.source_identity(),
                nearby: vec![((0, 0, 0), false)],
            },
            nearby: vec![frame.clone()],
            frame,
            profile,
            disc,
            index: 0,
            meshes: Vec::with_capacity(3),
            fragment_work: debris::FragmentWork::new(37),
            fragments: None,
            snapshot,
        };
        let scene = SceneSnapshot {
            game_id: "cpu-crater".into(),
            map_id: "tharsis".into(),
            generation: 1,
            temperature: -30,
            oxygen: 0,
            oceans: 0,
            max_oceans: 9,
            venus: 0,
            tiles: vec![SceneTile {
                id: "0,0,0".into(),
                q: 0,
                r: 0,
                s: 0,
                location: "mars".into(),
                terrain: "land".into(),
                display_name: None,
                tags: vec![],
                bonuses: vec![],
                occupant: Some("nuclear-zone-tile".into()),
                occupant_display_name: None,
                occupant_tags: vec![],
                visual: None,
                owner_id: None,
                reserved_by: None,
            }],
            players: vec![],
            selected_tile: None,
            selectable_tiles: vec![],
            offworld: Default::default(),
        };
        let mut world = World::new();
        world.insert_resource(crate::scene::SceneState(Some(scene)));
        world.insert_resource(NuclearRuntime {
            building: Some(building),
            ..default()
        });
        world
    }
    fn vertices(b: &Building) -> usize {
        let kinds = [DiscKind::Mask, DiscKind::Bowl, DiscKind::Scorch];
        kinds[..b.index].iter().map(|k| k.vertices()).sum::<usize>()
            + if b.index == 3 { 0 } else { b.disc.progress().0 }
    }
    #[test]
    fn bounded_geometry_finishes_without_a_device_or_completed_gpu_frame() {
        let mut world = preparing_world();
        let mut previous = 0;
        for _ in 0..64 {
            advance_cpu(&mut world).unwrap();
            let runtime = world.resource::<NuclearRuntime>();
            let b = runtime.building.as_ref().unwrap();
            let completed = vertices(b);
            assert!(completed >= previous && completed - previous <= CPU_TURNS * 256);
            previous = completed;
            assert!(runtime.plots.is_empty() && runtime.pending.is_empty());
            assert!(runtime.bytes() <= MAX_BYTES);
            if b.index == 3 {
                break;
            }
        }
        let b = world
            .resource::<NuclearRuntime>()
            .building
            .as_ref()
            .unwrap();
        assert_eq!(b.index, 3);
        assert_eq!(previous, 8256);
        assert_eq!(
            b.meshes.iter().map(Mesh::count_vertices).sum::<usize>(),
            8256
        );
        assert_eq!(
            b.meshes
                .iter()
                .map(|m| m.indices().unwrap().len())
                .sum::<usize>(),
            46848
        );
        assert!(!world.contains_resource::<Assets<Mesh>>());
        let mut candidates = b.fragment_work.progress();
        for _ in 0..32 {
            advance_cpu(&mut world).unwrap();
            let b = world
                .resource::<NuclearRuntime>()
                .building
                .as_ref()
                .unwrap();
            let next = b.fragment_work.progress();
            assert!(next >= candidates && next - candidates <= CPU_TURNS * 8);
            candidates = next;
            if b.fragments.is_some() {
                break;
            }
        }
        let b = world
            .resource::<NuclearRuntime>()
            .building
            .as_ref()
            .unwrap();
        assert_eq!(candidates, 144);
        assert!(b.fragments.is_some());
        assert!(!world.contains_resource::<Assets<Mesh>>());
    }
    #[test]
    fn a_removed_tile_cancels_cpu_work_before_any_gpu_publication() {
        let mut world = preparing_world();
        advance_cpu(&mut world).unwrap();
        assert!(
            vertices(
                world
                    .resource::<NuclearRuntime>()
                    .building
                    .as_ref()
                    .unwrap()
            ) > 0
        );
        world
            .resource_mut::<crate::scene::SceneState>()
            .0
            .as_mut()
            .unwrap()
            .tiles
            .clear();
        advance_cpu(&mut world).unwrap();
        let runtime = world.resource::<NuclearRuntime>();
        assert!(
            runtime.building.is_none() && runtime.plots.is_empty() && runtime.pending.is_empty()
        );
        assert_eq!(runtime.bytes(), 0);
    }
}
