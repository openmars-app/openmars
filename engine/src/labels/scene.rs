//! Owned native labels, ordered GPU warming and submitted-resource retirement.
use super::{
    atlas::{Book, RANGE, SIDE},
    content::Content,
    font::Shaper,
    geometry::{self, Key},
    material::{LabelMaterial, Parameters},
    pose,
};
use crate::entry::Sample;
use bevy::{
    asset::RenderAssetUsages,
    camera::{
        RenderTarget,
        visibility::{NoFrustumCulling, RenderLayers},
    },
    core_pipeline::tonemapping::Tonemapping,
    image::{ImageAddressMode, ImageFilterMode, ImageSampler, ImageSamplerDescriptor},
    light::{NotShadowCaster, NotShadowReceiver},
    mesh::MeshTag,
    pbr::PreparedMaterial,
    prelude::*,
    render::{
        RenderApp, erased_render_asset::ErasedRenderAssets, mesh::RenderMesh,
        render_asset::RenderAssets, render_resource::*, texture::GpuImage,
    },
};
use openmars_scene::{
    SceneTile,
    geometry::AxialPoint,
    view::{AssetView, BodyId},
};
use std::collections::{BTreeMap, BTreeSet};
struct Part {
    entity: Entity,
    mesh: Handle<Mesh>,
    key: Key,
}
struct Entry {
    root: Entity,
    parts: Vec<Part>,
    tile: SceneTile,
    content: Content,
    ordinal: usize,
}
#[derive(Resource, Default)]
pub struct LabelRuntime {
    entries: BTreeMap<String, Entry>,
    retiring: Vec<Entry>,
    shaper: Option<Shaper>,
    book: Book,
    images: Vec<Handle<Image>>,
    materials: BTreeMap<Key, Handle<LabelMaterial>>,
    probes: Option<Entry>,
    previous: Vec<SceneTile>,
    compact: bool,
    sync_pending: bool,
    wanted: usize,
    camera: Option<Entity>,
    warm: u8,
    ready: bool,
    captions: super::transitions::Captions,
}
impl LabelRuntime {
    pub fn ready(&self) -> bool {
        self.ready
    }
    fn parts(&self) -> impl Iterator<Item = &Part> {
        self.entries
            .values()
            .chain(self.probes.iter())
            .flat_map(|entry| &entry.parts)
    }
    fn remove_entry(world: &mut World, entry: Entry) {
        world.despawn(entry.root);
        for part in entry.parts {
            world.resource_mut::<Assets<Mesh>>().remove(part.mesh.id());
        }
    }
    fn clear(&mut self, world: &mut World) {
        for (_, entry) in std::mem::take(&mut self.entries) {
            Self::remove_entry(world, entry);
        }
        for entry in std::mem::take(&mut self.retiring) {
            Self::remove_entry(world, entry);
        }
        if let Some(entry) = self.probes.take() {
            Self::remove_entry(world, entry);
        }
        if let Some(camera) = self.camera.take() {
            world.despawn(camera);
        }
        for (_, material) in std::mem::take(&mut self.materials) {
            world
                .resource_mut::<Assets<LabelMaterial>>()
                .remove(material.id());
        }
        for image in self.images.drain(..) {
            world.resource_mut::<Assets<Image>>().remove(image.id());
        }
        *self = Self::default();
    }
    fn upload_pages(&mut self, world: &mut World) {
        for page in std::mem::take(&mut self.book.dirty) {
            let pixels = self.book.pages[page].pixels.clone();
            if let Some(handle) = self.images.get(page) {
                world
                    .resource_mut::<Assets<Image>>()
                    .get_mut(handle)
                    .unwrap()
                    .data = Some(pixels);
            } else {
                let mut image = Image::new(
                    Extent3d {
                        width: SIDE as u32,
                        height: SIDE as u32,
                        depth_or_array_layers: 1,
                    },
                    TextureDimension::D2,
                    pixels,
                    TextureFormat::Rgba8Unorm,
                    RenderAssetUsages::MAIN_WORLD | RenderAssetUsages::RENDER_WORLD,
                );
                image.sampler = ImageSampler::Descriptor(ImageSamplerDescriptor {
                    address_mode_u: ImageAddressMode::ClampToEdge,
                    address_mode_v: ImageAddressMode::ClampToEdge,
                    mag_filter: ImageFilterMode::Linear,
                    min_filter: ImageFilterMode::Linear,
                    ..default()
                });
                self.images
                    .push(world.resource_mut::<Assets<Image>>().add(image));
            }
        }
    }
    fn material(&mut self, world: &mut World, key: Key) -> Result<Handle<LabelMaterial>, String> {
        if let Some(handle) = self.materials.get(&key) {
            return Ok(handle.clone());
        }
        let (image, mode, stroke) = match key {
            Key::Name(page) => (
                self.images
                    .get(page)
                    .ok_or("Name atlas page is missing")?
                    .clone(),
                0.,
                (super::content::OUTLINE / super::content::FONT_SIZE) as f32
                    * super::font::RASTER_SIZE,
            ),
            Key::Credit(page) => (
                self.images
                    .get(page)
                    .ok_or("Credit atlas page is missing")?
                    .clone(),
                1.,
                0.,
            ),
            Key::Icon(id) => (
                crate::assets::texture(world, id).ok_or("Bonus texture is missing")?,
                2.,
                0.,
            ),
        };
        let handle = world
            .resource_mut::<Assets<LabelMaterial>>()
            .add(LabelMaterial {
                parameters: Parameters {
                    data: Vec4::new(mode, RANGE, stroke, 0.),
                },
                image,
            });
        self.materials.insert(key, handle.clone());
        Ok(handle)
    }
    fn build_entry(
        &mut self,
        world: &mut World,
        tile: SceneTile,
        content: Content,
        ordinal: usize,
    ) -> Result<Entry, String> {
        let dimensions: BTreeMap<_, _> = content
            .icons
            .iter()
            .filter_map(|icon| {
                let handle = crate::assets::texture(world, icon.asset)?;
                let image = world.resource::<Assets<Image>>().get(&handle)?;
                Some((icon.asset, UVec2::new(image.width(), image.height())))
            })
            .collect();
        let meshes = geometry::build(
            &content,
            self.shaper.as_mut().ok_or("Label font is missing")?,
            &mut self.book,
            |id| dimensions.get(id).copied(),
        )?;
        self.upload_pages(world);
        let root = world
            .spawn((Transform::IDENTITY, Visibility::Inherited))
            .id();
        let mut parts = Vec::new();
        for (key, mesh) in meshes {
            let mesh = world.resource_mut::<Assets<Mesh>>().add(mesh);
            let material = self.material(world, key)?;
            let order = if matches!(key, Key::Credit(_)) {
                crate::transparent::TransparentOrder::CREDIT
            } else {
                crate::transparent::TransparentOrder::LABEL
            };
            let entity = world
                .spawn((
                    Mesh3d(mesh.clone()),
                    MeshMaterial3d(material),
                    MeshTag::new(65535),
                    Transform::IDENTITY,
                    Visibility::Inherited,
                    RenderLayers::layer(2),
                    NoFrustumCulling,
                    NotShadowCaster,
                    NotShadowReceiver,
                    order,
                    ChildOf(root),
                ))
                .id();
            parts.push(Part { entity, mesh, key });
        }
        Ok(Entry {
            root,
            parts,
            tile,
            content,
            ordinal,
        })
    }
    fn pose_entries(&self, world: &mut World, sample: Sample, warming: bool) {
        let eye = world.resource::<crate::world::WorldView>().eye().as_dvec3();
        for entry in self.entries.values() {
            let Some(body) = BodyId::from_location(&entry.tile.location) else {
                continue;
            };
            let Some((projection, frame, visibility)) = world
                .resource::<crate::terrain::grid::GridRuntime>()
                .label_frame(body, world)
            else {
                continue;
            };
            let point = projection.hex_center(AxialPoint {
                q: entry.tile.q,
                r: entry.tile.r,
            });
            let Ok(surface) = projection.project(point, 0.) else {
                continue;
            };
            let surface = bevy::math::DVec3::from_array(surface);
            let scale = if warming || body != BodyId::Mars {
                1.
            } else {
                sample.scale(entry.ordinal)
            };
            let Some(pose) = pose::at(
                surface,
                surface.normalize(),
                crate::space::config::BodyPose {
                    center: frame.translation.as_dvec3(),
                    rotation: frame.rotation.as_dquat(),
                },
                eye,
                f64::from(scale),
            ) else {
                continue;
            };
            let opacity = if warming {
                1.
            } else {
                (pose.opacity as f32 * visibility).clamp(0., 1.)
            };
            let transform = Transform {
                translation: pose.position.as_vec3(),
                rotation: pose.rotation.as_quat(),
                scale: Vec3::splat(scale),
            };
            if world.get::<Transform>(entry.root) != Some(&transform) {
                world.entity_mut(entry.root).insert(transform);
            }
            let tag = MeshTag::new((opacity * 65535.).round() as u32);
            for part in &entry.parts {
                if world.get::<MeshTag>(part.entity) != Some(&tag) {
                    world.entity_mut(part.entity).insert(tag.clone());
                }
                let visible = if opacity > 0.001 && scale > 0. {
                    Visibility::Inherited
                } else {
                    Visibility::Hidden
                };
                if world.get::<Visibility>(part.entity) != Some(&visible) {
                    world.entity_mut(part.entity).insert(visible);
                }
            }
        }
    }
    fn update(&mut self, world: &mut World, uploaded: &BTreeSet<Entity>) -> Result<(), String> {
        if crate::assets::view(world) != AssetView::Game {
            if self.shaper.is_some() {
                self.clear(world);
            }
            return Ok(());
        }
        if !crate::assets::board_ready(world)
            || !world
                .resource::<crate::space::scene::CelestialRuntime>()
                .ready()
            || !world
                .resource::<crate::offworld::scene::OffworldRuntime>()
                .ready()
        {
            // Admission of new glyphs remains serial, but captions already on
            // the main view must follow the camera and shared Tile clock while
            // another owner is preparing its next frame.
            if self.ready {
                self.pose_entries(
                    world,
                    world.resource::<crate::entry::WorldEntrance>().tiles,
                    false,
                );
            }
            return Ok(());
        }
        let compact = world.resource::<crate::world::WorldView>().compact;
        if self.shaper.is_none() {
            let Some(font) = crate::assets::font(world) else {
                return Ok(());
            };
            self.shaper = Some(Shaper::new(font.data.as_ref().to_vec())?);
            let tile = world
                .resource::<crate::scene::SceneState>()
                .0
                .as_ref()
                .and_then(|s| s.tiles.first())
                .cloned();
            if let Some(tile) = tile {
                let probe = Content {
                    name: Some("M8".into()),
                    icon_y: 0.,
                    icons: vec![super::content::Icon {
                        asset: "icons/resources/wild",
                        x: 0.,
                        credit: Some(8),
                    }],
                };
                self.probes = Some(self.build_entry(world, tile, probe, 0)?);
            }
        }
        let current = &world
            .resource::<crate::scene::SceneState>()
            .0
            .as_ref()
            .ok_or("Missing label scene")?
            .tiles;
        let captions_changed = self.captions.reconcile(
            current,
            &self.previous,
            world.resource::<crate::nuclear::timeline::Timeline>(),
            world
                .resource::<crate::terrain::surface::SurfaceRuntime>()
                .step(),
            world.resource::<crate::scene::MotionPreference>().0,
        );
        if captions_changed || *current != self.previous || compact != self.compact {
            self.previous = current.clone();
            self.compact = compact;
            self.sync_pending = true;
        }
        if self.sync_pending {
            let tiles = self.previous.clone();
            self.sync_pending = false;
            let mut work = 0;
            let mut counts = BTreeMap::<BodyId, usize>::new();
            let mut wanted = BTreeSet::new();
            for tile in &tiles {
                let Some(body) = BodyId::from_location(&tile.location) else {
                    continue;
                };
                if tile.terrain == "empty"
                    || world
                        .resource::<crate::terrain::grid::GridRuntime>()
                        .label_frame(body, world)
                        .is_none()
                {
                    continue;
                }
                let ordinal = *counts.entry(body).or_default();
                *counts.get_mut(&body).unwrap() += 1;
                let content = self.captions.content(tile, compact);
                if content.name.is_none() && content.icons.is_empty() {
                    continue;
                }
                wanted.insert(tile.id.clone());
                if let Some(entry) = self.entries.get_mut(&tile.id).filter(|e| {
                    e.tile.location == tile.location
                        && e.tile.q == tile.q
                        && e.tile.r == tile.r
                        && e.tile.s == tile.s
                        && e.content == content
                        && e.ordinal == ordinal
                }) {
                    entry.tile = tile.clone();
                    continue;
                }
                if work >= 16 {
                    self.sync_pending = true;
                    continue;
                }
                if let Some(old) = self.entries.remove(&tile.id) {
                    // Keep the last published caption while replacement glyphs,
                    // atlas uploads and material bindings become GPU resident.
                    self.retiring.push(old);
                }
                let entry = self.build_entry(world, tile.clone(), content, ordinal)?;
                self.entries.insert(tile.id.clone(), entry);
                self.warm = 0;
                self.ready = false;
                work += 1;
            }
            let removed: Vec<_> = self
                .entries
                .keys()
                .filter(|id| !wanted.contains(*id))
                .cloned()
                .collect();
            for id in removed {
                Self::remove_entry(world, self.entries.remove(&id).unwrap());
            }
            self.wanted = wanted.len();
            if self
                .entries
                .values()
                .map(|e| {
                    e.parts
                        .iter()
                        .map(|p| {
                            world
                                .resource::<Assets<Mesh>>()
                                .get(&p.mesh)
                                .map_or(0, Mesh::count_vertices)
                        })
                        .sum::<usize>()
                })
                .sum::<usize>()
                > 1_048_576
            {
                return Err("Board label geometry exceeds renderer bounds".into());
            }
        }
        if self.warm < 2 {
            if self.camera.is_none() {
                let Some(target) = world
                    .resource::<crate::terrain::surface::SurfaceRuntime>()
                    .warm_target()
                else {
                    return Ok(());
                };
                self.camera = Some(
                    world
                        .spawn((
                            Camera3d::default(),
                            Camera {
                                order: -6,
                                clear_color: ClearColorConfig::Custom(Color::BLACK),
                                ..default()
                            },
                            RenderTarget::Image(target.into()),
                            Msaa::Off,
                            Tonemapping::None,
                            RenderLayers::layer(2),
                            Transform::from_xyz(0., 0., 6.8).looking_at(Vec3::ZERO, Vec3::Y),
                        ))
                        .id(),
                );
            }
            if !self.sync_pending
                && self.wanted == self.entries.len()
                && self.parts().all(|p| uploaded.contains(&p.entity))
            {
                self.warm += 1;
            }
            if self.warm >= 2 {
                for old in std::mem::take(&mut self.retiring) {
                    Self::remove_entry(world, old);
                }
                if let Some(probe) = self.probes.take() {
                    Self::remove_entry(world, probe);
                }
                for part in self.parts() {
                    world.entity_mut(part.entity).insert(RenderLayers::layer(0));
                }
                if let Some(camera) = self.camera.take() {
                    world.despawn(camera);
                }
                self.ready = true;
            }
        }
        let sample = world.resource::<crate::entry::WorldEntrance>().tiles;
        self.pose_entries(world, sample, !self.ready);
        Ok(())
    }
    pub fn diagnostics(&self, world: &World) -> serde_json::Value {
        serde_json::json!({"ready":self.ready,"warmFrames":self.warm,"warmCamera":self.camera.is_some(),"glyphs":self.book.glyphs(),"atlasPages":self.images.len(),"fontMaskBytes":self.shaper.as_ref().map_or(0,Shaper::bytes),"materials":self.materials.len(),"entries":self.entries.iter().map(|(id,e)|serde_json::json!({"id":id,"root":e.root.to_string(),"ordinal":e.ordinal,"name":e.content.name,"icons":e.content.icons.len(),"credits":e.content.icons.iter().filter_map(|i|i.credit).collect::<Vec<_>>(),"parts":e.parts.len(),"scale":world.get::<Transform>(e.root).map(|t|t.scale.x),"opacity":e.parts.first().and_then(|p|world.get::<MeshTag>(p.entity)).map(|t|t.value as f64/65535.),"fontParts":e.parts.iter().filter(|p|matches!(p.key,Key::Name(_)|Key::Credit(_))).count()})).collect::<Vec<_>>()})
    }
}
pub fn reset(world: &mut World) {
    if let Some(mut runtime) = world.remove_resource::<LabelRuntime>() {
        runtime.clear(world);
        world.insert_resource(runtime);
    }
}
pub fn prepare(app: &mut App) -> Result<(), String> {
    let _profile = crate::profiling::span(app.world(), "labels.prepare");
    let mut uploaded = BTreeSet::new();
    let render = app.sub_app(RenderApp).world();
    if render
        .resource::<PipelineCache>()
        .waiting_pipelines()
        .next()
        .is_none()
    {
        let meshes = render.resource::<RenderAssets<RenderMesh>>();
        let materials = render.resource::<ErasedRenderAssets<PreparedMaterial>>();
        let images = render.resource::<RenderAssets<GpuImage>>();
        let world = app.world();
        let runtime = world.resource::<LabelRuntime>();
        if runtime
            .images
            .iter()
            .all(|image| images.get(image.id()).is_some())
        {
            for part in runtime.parts() {
                let material = world.get::<MeshMaterial3d<LabelMaterial>>(part.entity);
                if meshes.get(part.mesh.id()).is_some()
                    && material.is_some_and(|m| materials.get(m.id().untyped()).is_some())
                {
                    uploaded.insert(part.entity);
                }
            }
        }
    }
    let mut runtime = app.world_mut().remove_resource::<LabelRuntime>().unwrap();
    let result = runtime.update(app.world_mut(), &uploaded);
    app.world_mut().insert_resource(runtime);
    result
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn owned_font_geometry_materials_and_atlas_return_to_exact_native_baseline() {
        let mut world = World::new();
        world.insert_resource(Assets::<Image>::default());
        world.insert_resource(Assets::<Mesh>::default());
        world.insert_resource(Assets::<LabelMaterial>::default());
        let baseline = world.query::<Entity>().iter(&world).count();
        let tile:SceneTile=serde_json::from_value(serde_json::json!({"id":"0,0,0","q":0,"r":0,"s":0,"location":"mars","terrain":"land","displayName":"Arsia Mons","tags":[],"bonuses":[],"occupant":null,"occupantDisplayName":null,"occupantTags":[],"visual":null,"ownerId":null,"reservedBy":null})).unwrap();
        let mut runtime = LabelRuntime {
            shaper: Some(
                Shaper::new(
                    include_bytes!("../../../assets/original/fonts/prototype.ttf").to_vec(),
                )
                .unwrap(),
            ),
            ..default()
        };
        let entry = runtime
            .build_entry(
                &mut world,
                tile.clone(),
                Content::new(&tile, false, false, false),
                0,
            )
            .unwrap();
        let entity = entry.root;
        runtime.entries.insert(tile.id.clone(), entry);
        assert!(world.get_entity(entity).is_ok());
        assert_eq!(world.resource::<Assets<Image>>().len(), 1);
        assert_eq!(world.resource::<Assets<Mesh>>().len(), 1);
        assert_eq!(world.resource::<Assets<LabelMaterial>>().len(), 1);
        let replacement = runtime
            .build_entry(
                &mut world,
                tile.clone(),
                Content::new(&tile, false, false, false),
                0,
            )
            .unwrap();
        runtime
            .retiring
            .push(runtime.entries.remove(&tile.id).unwrap());
        runtime.entries.insert(tile.id.clone(), replacement);
        assert!(world.get_entity(entity).is_ok());
        assert_eq!(world.resource::<Assets<Mesh>>().len(), 2);
        assert_eq!(world.resource::<Assets<Image>>().len(), 1);
        assert_eq!(world.resource::<Assets<LabelMaterial>>().len(), 1);
        runtime.clear(&mut world);
        assert!(world.get_entity(entity).is_err());
        assert_eq!(world.resource::<Assets<Image>>().len(), 0);
        assert_eq!(world.resource::<Assets<Mesh>>().len(), 0);
        assert_eq!(world.resource::<Assets<LabelMaterial>>().len(), 0);
        assert_eq!(world.query::<Entity>().iter(&world).count(), baseline);
        assert!(runtime.shaper.is_none());
        assert_eq!(runtime.book.glyphs(), 0);
    }
}
