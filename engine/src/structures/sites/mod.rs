//! Mining workings and claimed land, driven only by public scene appearance.
mod frame;
mod geometry;
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
use geometry::WorksiteWork;
use lifetime::{Life, Pose};
use material::{SiteMaterial, SiteUniform, Textures};
use openmars_scene::{SceneSnapshot, SceneTile, geometry::AxialPoint};
use std::collections::{BTreeMap, BTreeSet};
type Coordinate = (i32, i32, i32);
const MAX_SITES: usize = 512;
const MAX_MESH_BYTES: usize = 64 * 1024 * 1024;

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum Kind {
    Mining,
    Reserved,
}
impl Kind {
    fn from_tile(tile: &SceneTile) -> Option<Self> {
        match tile.occupant.as_deref() {
            Some("mining-tile") => Some(Self::Mining),
            Some("restricted-tile") => Some(Self::Reserved),
            None if tile.reserved_by.is_some() => Some(Self::Reserved),
            _ => None,
        }
    }
}
#[derive(Clone, Copy, Debug)]
struct Spec {
    kind: Kind,
    seed: u32,
    color: Vec4,
}
impl Spec {
    fn same_geometry(self, other: Self) -> bool {
        self.kind == other.kind && (self.kind == Kind::Reserved || self.seed == other.seed)
    }
}
pub struct Input {
    pub snapshot: std::sync::Arc<openmars_scene::landscape::LandscapeSnapshot>,
    pub step: u64,
    pub motion_step: u64,
    pub reduced: bool,
    pub settled: bool,
    pub warm: Handle<Image>,
}
fn desired(scene: &SceneSnapshot) -> BTreeMap<Coordinate, Spec> {
    scene
        .tiles
        .iter()
        .filter(|t| crate::world::is_mars(t))
        .filter_map(|tile| {
            let kind = Kind::from_tile(tile)?;
            let owner = tile.owner_id.as_ref().or(tile.reserved_by.as_ref());
            let color = owner
                .and_then(|id| scene.players.iter().find(|p| &p.id == id))
                .and_then(|p| Srgba::hex(&p.color).ok())
                .unwrap_or(Srgba::rgb_u8(136, 136, 136));
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
                    color: Vec4::from_array(LinearRgba::from(color).to_f32_array()),
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
    parts: [Entity; 2],
    mesh: Handle<Mesh>,
    materials: [Handle<SiteMaterial>; 2],
    uniform: UniformBuffer<SiteUniform>,
    bytes: usize,
    warm: u32,
    collapse_sources: Vec<crate::nuclear::collapse::PartSource>,
    collapse_capture: Option<Vec<crate::nuclear::collapse::MotionFacts>>,
}
struct Building {
    coordinate: Coordinate,
    spec: Spec,
    frame: Frame,
    work: WorksiteWork,
}
struct Retired {
    buffer: Buffer,
    bytes: usize,
    frames: u8,
}
#[derive(Resource, Default)]
pub struct SiteRuntime {
    plots: BTreeMap<Coordinate, Plot>,
    building: Option<Building>,
    highlights: Option<UniformBuffer<crate::terrain::materials::HighlightUniform>>,
    retired: Vec<Retired>,
    camera: Option<Entity>,
    ready: bool,
    pub revision: u64,
    writes: u64,
    builds: u64,
    retirements: u64,
}
impl SiteRuntime {
    pub fn ready(&self) -> bool {
        self.ready
    }
    pub fn covers(&self, coordinate: Coordinate) -> bool {
        self.plots.get(&coordinate).is_some_and(|p| p.warm >= 2)
    }
    pub fn progress(&self) -> Option<String> {
        (!self.ready).then(|| format!("Preparing worksites ({} sites)", self.plots.len()))
    }
    pub fn diagnostics(&self) -> serde_json::Value {
        serde_json::json!({"ready":self.ready,"sites":self.plots.len(),"building":self.building.is_some(),
            "uniforms":self.plots.len()+usize::from(self.highlights.is_some())+self.retired.len(),
            "meshBytes":self.plots.values().map(|p|p.bytes).sum::<usize>()+self.retired.iter().map(|p|p.bytes).sum::<usize>(),
            "geometryBuilds":self.builds,"uniformWrites":self.writes,"retiredUniforms":self.retirements,
            "highlightBuffer":self.highlights.as_ref().and_then(|b|b.buffer()).map(|b|format!("{:?}",b.id())),
            "sample":self.plots.iter().map(|(c,p)|serde_json::json!({"coordinate":c,"kind":format!("{:?}",p.spec.kind),"seed":p.spec.seed,
                "entity":p.root.to_string(),"mesh":format!("{:?}",p.mesh.id()),"buffer":format!("{:?}",p.uniform.buffer().unwrap().id()),
                "born":p.life.born,"destroy":p.life.destroyed.map(|d|d.0),"cart":p.uniform.get().cart.to_array(),"collapseParts":p.collapse_sources.len(),"collapse":p.uniform.get().collapse_parts.iter().take(p.collapse_sources.len()).map(|v|v.to_array()).collect::<Vec<_>>(),"color":p.spec.color.to_array(),"warmFrames":p.warm})).collect::<Vec<_>>()})
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
            return Err("Worksite count limit exceeded".into());
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
        if !desired.is_empty() && noises.is_none() {
            self.ready = false;
            return Ok(());
        }
        if !desired.is_empty() && self.highlights.is_none() {
            let mut buffer: UniformBuffer<crate::terrain::materials::HighlightUniform> = world
                .resource::<crate::terrain::grid::GridRuntime>()
                .uniform()
                .into();
            buffer.set_label(Some("worksite highlights"));
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
            if retained + WorksiteWork::PEAK_BYTES as usize > MAX_MESH_BYTES {
                return Err("Worksite working-set limit exceeded".into());
            }
            let point = input.snapshot.projection().hex_center(AxialPoint {
                q: coordinate.0,
                r: coordinate.1,
            });
            let height = world
                .resource::<crate::terrain::TerrainState>()
                .height(point);
            let frame = Frame::new(
                input.snapshot.projection(),
                [coordinate.0, coordinate.1, coordinate.2],
                height,
            )?;
            self.building = Some(Building {
                coordinate,
                spec,
                work: WorksiteWork::new(spec.kind, frame.clone(), spec.seed)?,
                frame,
            });
        }
        let mut complete = false;
        if let Some(building) = &mut self.building {
            // Each substantive geometry part is bounded; at most four per completed frame.
            for _ in 0..4 {
                if building.work.advance()? {
                    complete = true;
                    break;
                }
            }
        }
        if complete {
            let Building {
                coordinate,
                spec,
                frame,
                work,
            } = self.building.take().unwrap();
            let (mesh, collapse_sources) = work.take_mesh()?;
            let bytes = mesh.get_vertex_buffer_size()
                + mesh.get_index_buffer_bytes().map_or(0, |v| v.len())
                + collapse_sources.len()
                    * (std::mem::size_of::<crate::nuclear::collapse::PartSource>()
                        + std::mem::size_of::<crate::nuclear::collapse::MotionFacts>())
                + SiteUniform::min_size().get() as usize;
            let mesh = world.resource_mut::<Assets<Mesh>>().add(mesh);
            let life = if input.settled {
                Life::settled()
            } else {
                Life {
                    born: seconds,
                    born_motion: motion_seconds,
                    destroyed: None,
                }
            };
            let mut uniform = UniformBuffer::from(life.parameters(
                spec,
                &frame,
                Pose {
                    radius: input.snapshot.projection().content_radius() as f32,
                    seconds,
                    motion_seconds,
                    reduced: input.reduced,
                },
                &[],
            )?);
            uniform.set_label(Some("worksite parameters"));
            uniform.write_buffer(device, queue);
            let (mid, high) = noises.as_ref().unwrap();
            let data = world.resource::<crate::terrain::grid::GridRuntime>().data();
            let atmosphere = world
                .resource::<crate::atmosphere::AtmosphereRuntime>()
                .binding()?;
            let materials = [true, false].map(|ground| {
                world
                    .resource_mut::<Assets<SiteMaterial>>()
                    .add(SiteMaterial {
                        atmosphere: atmosphere.clone(),
                        parameters: uniform.buffer().unwrap().clone(),
                        highlights: self.highlights.as_ref().unwrap().buffer().unwrap().clone(),
                        textures: Textures {
                            ground,
                            mid: mid.clone(),
                            high: high.clone(),
                            highlight_data: data.clone(),
                        },
                    })
            });
            let root = world
                .spawn((Transform::default(), Visibility::Inherited))
                .id();
            let parts = materials.clone().map(|material| {
                world
                    .spawn((
                        Mesh3d(mesh.clone()),
                        MeshMaterial3d(material),
                        Transform::default(),
                        NoFrustumCulling,
                        NotShadowCaster,
                        NotShadowReceiver,
                        RenderLayers::layer(2),
                        ChildOf(root),
                    ))
                    .id()
            });
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
                    uniform,
                    bytes,
                    warm: 0,
                    collapse_sources,
                    collapse_capture: None,
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
            if let Some((_, motion_at)) = plot.life.destroyed {
                if plot.collapse_capture.is_none() {
                    use bevy::math::{DMat4, DQuat, DVec3};
                    let q = DQuat::from_array(plot.frame.rotation);
                    let origin = DVec3::from_array(plot.frame.origin);
                    let angle = geometry::cave_angle(plot.spec.seed);
                    let along = q * DVec3::new(-angle.sin(), angle.cos(), 0.);
                    let radial = q * DVec3::Z;
                    let cart = geometry::cart_pose(motion_at)?;
                    let parent = DMat4::from_rotation_translation(
                        DQuat::from_axis_angle(along, cart.tilt),
                        origin + along * cart.position.y + radial * cart.position.z,
                    ) * DMat4::from_translation(-origin);
                    plot.collapse_capture = Some(
                        plot.collapse_sources
                            .iter()
                            .map(|s| s.capture(if s.cart { parent } else { DMat4::IDENTITY }))
                            .collect(),
                    );
                }
            } else {
                plot.collapse_capture = None;
            }
            let values = plot.life.parameters(
                plot.spec,
                &plot.frame,
                Pose {
                    radius: input.snapshot.projection().content_radius() as f32,
                    seconds,
                    motion_seconds,
                    reduced: input.reduced,
                },
                plot.collapse_capture.as_deref().unwrap_or(&[]),
            )?;
            plot.uniform.set(values);
            plot.uniform.write_buffer(device, queue);
            self.writes += 1;
            for handle in &plot.materials {
                let mut materials = world.resource_mut::<Assets<SiteMaterial>>();
                if materials.get(handle).unwrap().textures.highlight_data != data {
                    materials.get_mut(handle).unwrap().textures.highlight_data = data.clone();
                }
            }
            if plot.warm < 2 && uploaded.contains(&plot.root) {
                plot.warm += 1;
                if plot.warm == 2 {
                    for part in plot.parts {
                        world.entity_mut(part).insert(RenderLayers::default());
                    }
                    self.revision += 1;
                }
            }
        }
        let warming = self.plots.values().any(|p| p.warm < 2);
        if warming && self.camera.is_none() {
            let shared = world
                .query::<(&Camera, &RenderLayers)>()
                .iter(world)
                .any(|(c, l)| c.is_active && l == &RenderLayers::layer(2));
            if !shared {
                self.camera = Some(
                    world
                        .spawn((
                            Camera3d::default(),
                            bevy::core_pipeline::prepass::DepthPrepass,
                            bevy::core_pipeline::tonemapping::DebandDither::Disabled,
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
        } else if !warming && let Some(camera) = self.camera.take() {
            world.despawn(camera);
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
    world.resource_scope(|world, runtime: Mut<SiteRuntime>| {
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
    let _profile = crate::profiling::span(app.world(), "worksites.prepare");
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
        for plot in app.world().resource::<SiteRuntime>().plots.values() {
            if meshes.get(plot.mesh.id()).is_some()
                && plot
                    .materials
                    .iter()
                    .all(|h| materials.get(h.id().untyped()).is_some())
            {
                uploaded.insert(plot.root);
            }
        }
    }
    let mut runtime = app.world_mut().remove_resource::<SiteRuntime>().unwrap();
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
        runtime.update(app.world_mut(), input, &uploaded, device, queue)
    } else {
        Ok(())
    };
    app.world_mut().insert_resource(runtime);
    result
}
pub fn reset(world: &mut World) {
    let Some(mut runtime) = world.remove_resource::<SiteRuntime>() else {
        return;
    };
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
    if let Some(camera) = runtime.camera.take() {
        world.despawn(camera);
    }
    world.insert_resource(SiteRuntime {
        retired: runtime.retired,
        writes: runtime.writes,
        builds: runtime.builds,
        retirements: runtime.retirements,
        ..default()
    });
}
pub struct SitePlugin;
impl Plugin for SitePlugin {
    fn build(&self, app: &mut App) {
        bevy::asset::embedded_asset!(app, "site.wesl");
        app.add_plugins(MaterialPlugin::<SiteMaterial>::default())
            .init_resource::<SiteRuntime>();
    }
}
