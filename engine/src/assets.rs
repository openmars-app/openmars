//! Scene-owned asset handles. Readiness follows decoded dependencies, actual
//! render-world residency and queue completion, as in the sibling renderer.
use crate::nuclear::material::{NuclearMaskMaterial, NuclearMaterial};
use crate::particles::material::ParticleMaterial;
use crate::structures::materials::StructureMaterial;
use crate::structures::sites::material::SiteMaterial;
use crate::structures::special::accessory_material::AccessoryMaterial;
use crate::structures::special::material::SpecialMaterial;
use crate::structures::world_tree::{WorldTreeMaterial, ground::MossMaterial};
use crate::terrain::TerrainState;
use crate::terrain::materials::{GridMaterial, GroundMaterial, LakeMaskMaterial, OceanMaterial};
use crate::vegetation::materials::FoliageMaterial;
use bevy::ecs::system::SystemParam;
use bevy::image::{
    ImageAddressMode, ImageFilterMode, ImageLoaderSettings, ImageSampler, ImageSamplerDescriptor,
};
use bevy::{
    asset::{
        AssetMetaCheck, LoadState, RecursiveDependencyLoadState,
        io::{
            AssetSourceBuilder,
            memory::{Dir, MemoryAssetReader},
        },
    },
    camera::RenderTarget,
    gltf::Gltf,
    pbr::PreparedMaterial,
    prelude::*,
    render::{
        RenderApp, erased_render_asset::ErasedRenderAssets, mesh::RenderMesh,
        render_asset::RenderAssets, texture::GpuImage,
    },
};
use openmars_scene::{
    assets::{AssetDemand, AssetKind, RENDERER_ASSETS, requirements},
    view::{AssetView, FocusTarget},
};
use std::path::Path;

enum Slot {
    Model(Handle<Gltf>),
    Texture(Handle<Image>),
    // The board font has private shaping/atlas custody, independent of UI fonts.
    Font(Font),
}
impl Slot {
    fn id(&self) -> Option<bevy::asset::UntypedAssetId> {
        match self {
            Self::Model(h) => Some(h.id().untyped()),
            Self::Texture(h) => Some(h.id().untyped()),
            Self::Font(_) => None,
        }
    }
}

#[derive(Resource, Default)]
pub struct WorldAssets {
    directory: Dir,
    slots: Vec<(String, String, Slot)>,
    spawned: bool,
    published: bool,
    request: u32,
    focus: FocusTarget,
    encoded_pending: bool,
    pub(crate) view: AssetView,
}
#[derive(Component)]
pub(crate) struct AssetCamera;
#[derive(Component)]
struct AssetWorld;
#[derive(Component)]
pub(crate) struct AssetGlobe;

pub fn configure(app: &mut App) {
    let assets = WorldAssets::default();
    let directory = assets.directory.clone();
    app.register_asset_source(
        "world",
        AssetSourceBuilder::new(move || {
            Box::new(MemoryAssetReader {
                root: directory.clone(),
            })
        }),
    );
    app.insert_resource(assets);
}

pub fn plugin() -> AssetPlugin {
    AssetPlugin {
        meta_check: AssetMetaCheck::Never,
        ..default()
    }
}

pub fn setup(mut commands: Commands) {
    commands.spawn((
        AssetCamera,
        bevy::pbr::ScreenSpaceTransmission::default(),
        bevy::anti_alias::fxaa::Fxaa::default(),
        Camera3d {
            depth_texture_usages: (bevy::render::render_resource::TextureUsages::RENDER_ATTACHMENT
                | bevy::render::render_resource::TextureUsages::TEXTURE_BINDING)
                .into(),
            ..default()
        },
        bevy::core_pipeline::prepass::DepthPrepass,
        Camera {
            is_active: false,
            ..default()
        },
        Msaa::Off,
        bevy::core_pipeline::tonemapping::Tonemapping::None,
        RenderTarget::TextureView(super::SURFACE),
        Transform::from_xyz(0.0, 0.0, 14.0).looking_at(Vec3::ZERO, Vec3::Y),
    ));
}

pub fn reset(world: &mut World, view: AssetView) -> Result<(), String> {
    let request = world
        .resource::<WorldAssets>()
        .request
        .checked_add(1)
        .ok_or("Asset request generation exhausted")?;
    crate::space::scene::reset(world);
    crate::menu::reset(world);
    crate::terrain::surface::reset(world);
    world.insert_resource(TerrainState::default());
    let cameras: Vec<_> = world
        .query_filtered::<Entity, With<AssetCamera>>()
        .iter(world)
        .collect();
    for camera in cameras {
        world
            .entity_mut(camera)
            .remove::<bevy::light::EnvironmentMapLight>();
    }
    let roots: Vec<_> = world
        .query_filtered::<Entity, With<AssetWorld>>()
        .iter(world)
        .collect();
    for root in roots {
        world.despawn(root);
    }
    let mut assets = world.resource_mut::<WorldAssets>();
    for (_, path, _) in std::mem::take(&mut assets.slots) {
        assets.directory.remove_asset(Path::new(&path));
    }
    assets.spawned = false;
    assets.published = false;
    assets.request = request;
    assets.view = view;
    assets.focus = FocusTarget::default();
    assets.encoded_pending = false;
    Ok(())
}

pub(crate) fn model(world: &World, id: &str) -> Option<Handle<Gltf>> {
    world
        .resource::<WorldAssets>()
        .slots
        .iter()
        .find_map(|(key, _, slot)| {
            if key == id {
                if let Slot::Model(handle) = slot {
                    Some(handle.clone())
                } else {
                    None
                }
            } else {
                None
            }
        })
}
pub(crate) fn font(world: &World) -> Option<Font> {
    world
        .resource::<WorldAssets>()
        .slots
        .iter()
        .find_map(|(id, _, slot)| match slot {
            Slot::Font(font) if id == "fonts/prototype" => Some(font.clone()),
            _ => None,
        })
}
pub(crate) fn texture(world: &World, id: &str) -> Option<Handle<Image>> {
    world
        .resource::<WorldAssets>()
        .slots
        .iter()
        .find_map(|(key, _, slot)| {
            if key == id
                && let Slot::Texture(image) = slot
            {
                Some(image.clone())
            } else {
                None
            }
        })
}

/// Latch initial publication only after actual scene resources are GPU-ready.
/// Subsequent live changes keep the current view visible while they prepare.
pub fn publish(world: &mut World) {
    let mut assets = world.resource_mut::<WorldAssets>();
    if assets.encoded_pending {
        for (_, path, _) in &assets.slots {
            assets.directory.remove_asset(Path::new(path));
        }
        assets.encoded_pending = false;
    }
    assets.published = true;
}
pub fn view_active(world: &mut World) -> bool {
    world
        .query_filtered::<&Camera, With<AssetCamera>>()
        .iter(world)
        .any(|camera| camera.is_active)
}

pub fn view(world: &World) -> AssetView {
    world.resource::<WorldAssets>().view
}
pub fn demand(world: &World) -> AssetDemand {
    let assets = world.resource::<WorldAssets>();
    AssetDemand {
        request: assets.request,
        assets: requirements(assets.view)
            .into_iter()
            .filter(|a| !assets.slots.iter().any(|(id, _, _)| id == a.id))
            .collect(),
    }
}

/// Camera travel does not mutate the prepared world's residency or request generation.
pub fn change_focus(world: &mut World, focus: FocusTarget) -> Result<bool, String> {
    let mut assets = world.resource_mut::<WorldAssets>();
    if assets.view != AssetView::Game {
        return Err("Camera focus requires a game scene".into());
    }
    let changed = assets.focus != focus;
    assets.focus = focus;
    Ok(changed)
}
pub fn focus(world: &World) -> FocusTarget {
    world.resource::<WorldAssets>().focus
}
pub fn source_count(world: &World) -> usize {
    world.resource::<WorldAssets>().slots.len()
}

pub fn cpu_array_count(world: &World) -> usize {
    let images = world.resource::<Assets<Image>>();
    world
        .resource::<WorldAssets>()
        .slots
        .iter()
        .filter(|(id, _, slot)| {
            RENDERER_ASSETS
                .iter()
                .any(|asset| asset.id == id && asset.array_layers > 0)
                && matches!(slot, Slot::Texture(handle) if images.get(handle.id()).is_some_and(|image| image.data.as_ref().is_some_and(|pixels| !pixels.is_empty())))
        })
        .count()
}

/// Actual GPU array residency, including the authored layer and mip counts.
pub fn array_stats(world: &World, render: &World) -> (usize, u32, u32) {
    let Some(images) = render.get_resource::<RenderAssets<GpuImage>>() else {
        return (0, 0, 0);
    };
    let mut result = (0, 0, 0);
    for (id, _, slot) in &world.resource::<WorldAssets>().slots {
        if RENDERER_ASSETS
            .iter()
            .any(|asset| asset.id == id && asset.array_layers > 0)
            && let Slot::Texture(handle) = slot
            && let Some(image) = images.get(handle.id())
        {
            result.0 += 1;
            result.1 += image.texture_descriptor.size.depth_or_array_layers;
            result.2 += image.texture_descriptor.mip_level_count;
        }
    }
    result
}

pub fn install(
    world: &mut World,
    request: u32,
    id: String,
    filename: String,
    bytes: Vec<u8>,
) -> Result<(), String> {
    let _profile = crate::profiling::span(world, "assets.install")
        .arg("request", request)
        .arg("bytes", bytes.len());
    let assets = world.resource::<WorldAssets>();
    if request != assets.request {
        return Err("Stale asset request".into());
    }
    let wanted = requirements(assets.view);
    if !wanted.iter().any(|asset| asset.id == id) {
        return Err("Asset is outside the requested view".into());
    }
    if bytes.is_empty()
        || bytes.len() > openmars_scene::assets::max_file_bytes(&id) as usize
        || filename.contains(['/', '\\', ':'])
        || !filename
            .bytes()
            .all(|c| c.is_ascii_alphanumeric() || matches!(c, b'.' | b'-' | b'_'))
    {
        return Err("Invalid world asset input".into());
    }
    if world
        .resource::<WorldAssets>()
        .slots
        .iter()
        .any(|(existing, _, _)| existing == &id)
    {
        return Err("Duplicate world asset identity".into());
    }
    let requirement = RENDERER_ASSETS
        .iter()
        .find(|asset| asset.id == id && filename.ends_with(asset.extension))
        .ok_or("Unknown world asset identity or format")?;
    let font = matches!(requirement.kind, AssetKind::Font).then(|| Font::from_bytes(bytes.clone()));
    world
        .resource::<WorldAssets>()
        .directory
        .insert_asset(Path::new(&filename), bytes);
    let server = world.resource::<AssetServer>();
    let path = format!("world://{filename}");
    let slot = match requirement.kind {
        AssetKind::Font => Slot::Font(font.expect("Font bytes prepared")),
        AssetKind::Model => Slot::Model(server.load::<Gltf>(path)),
        AssetKind::Image => {
            let requirement = *requirement;
            Slot::Texture(
                server
                    .load_builder()
                    .with_settings(move |settings: &mut ImageLoaderSettings| {
                        settings.is_srgb = requirement.srgb;
                        if requirement.array_layers > 0
                            || requirement.id == "textures/skyboxes/starmap"
                            || (requirement.id.starts_with("textures/planets/")
                                && requirement.id != "textures/planets/mars/surface")
                        {
                            // Atlas and display-sky pixels are GPU-only after upload. Transfer
                            // their decoded mip chains instead of retaining a
                            // second full copy in the main world.
                            settings.asset_usage = bevy::asset::RenderAssetUsages::RENDER_WORLD;
                        }
                        if requirement.repeat {
                            settings.sampler = ImageSampler::Descriptor(ImageSamplerDescriptor {
                                address_mode_u: ImageAddressMode::Repeat,
                                address_mode_v: ImageAddressMode::Repeat,
                                mag_filter: ImageFilterMode::Linear,
                                min_filter: ImageFilterMode::Linear,
                                mipmap_filter: ImageFilterMode::Linear,
                                anisotropy_clamp: 4,
                                ..default()
                            });
                        }
                    })
                    .load(path),
            )
        }
    };
    let mut assets = world.resource_mut::<WorldAssets>();
    assets.slots.push((id, filename, slot));
    assets.encoded_pending = true;
    Ok(())
}

pub fn cameras(
    menu: Res<crate::menu::MenuRuntime>,
    assets: Res<WorldAssets>,
    terrain: Res<TerrainState>,
    mut cameras: Query<&mut Camera, With<AssetCamera>>,
    mut overlay: Query<&mut Camera, (With<Camera2d>, Without<AssetCamera>)>,
) {
    let active = assets.published
        && match assets.view {
            AssetView::Idle => false,
            AssetView::Menu => menu.ready(),
            AssetView::Game => terrain.contact.is_some(),
        };
    for mut camera in &mut cameras {
        if camera.is_active != active {
            camera.is_active = active;
        }
    }
    for mut camera in &mut overlay {
        // The greeting is the only Bevy UI. React owns the game interface, so
        // an active 3D view does not need an empty UI/2D render pipeline.
        if camera.is_active == active {
            camera.is_active = !active;
        }
        if camera.order != 1 {
            camera.order = 1;
        }
        if !matches!(camera.clear_color, ClearColorConfig::Default) {
            camera.clear_color = ClearColorConfig::Default;
        }
    }
}

#[derive(SystemParam)]
pub struct SourceAssets<'w> {
    images: Res<'w, Assets<Image>>,
    terrain: ResMut<'w, TerrainState>,
    skies: ResMut<'w, Assets<crate::sky::SkyMaterial>>,
    atmosphere: Res<'w, crate::atmosphere::AtmosphereRuntime>,
}

pub fn synchronize(
    mut commands: Commands,
    state: Res<super::scene::SceneState>,
    mut assets: ResMut<WorldAssets>,
    server: Res<AssetServer>,
    sources: SourceAssets,
    mut meshes: ResMut<Assets<Mesh>>,
    mut materials: ResMut<Assets<StandardMaterial>>,
) {
    let SourceAssets {
        images,
        mut terrain,
        mut skies,
        atmosphere,
    } = sources;
    let active = state.0.is_some() && assets.view == AssetView::Game;
    if !active
        || assets.spawned
        || assets.slots.len() != requirements(assets.view).len()
        || assets.slots.iter().any(|(_, _, slot)| {
            slot.id()
                .is_some_and(|id| !server.is_loaded_with_dependencies(id))
        })
    {
        return;
    }
    let Some(texture) = assets.slots.iter().find_map(|(id, _, slot)| {
        if id != "textures/planets/mars/surface" {
            return None;
        }
        if let Slot::Texture(handle) = slot {
            Some(handle.clone())
        } else {
            None
        }
    }) else {
        return;
    };
    if terrain.error.is_some() {
        return;
    }
    if terrain.relief.is_none() {
        let Some(image) = images.get(&texture) else {
            return;
        };
        if let Err(error) = terrain.start(image) {
            terrain.error = Some(error);
            return;
        }
    }
    let globe = match terrain.advance() {
        Ok(Some(mesh)) => mesh,
        Ok(None) => return,
        Err(error) => {
            terrain.error = Some(error);
            return;
        }
    };
    let Some(sky) = assets.slots.iter().find_map(|(id, _, slot)| match slot {
        Slot::Texture(handle) if id == "textures/skyboxes/starmap" => Some(handle.clone()),
        _ => None,
    }) else {
        return;
    };
    let Ok(atmosphere) = atmosphere.binding() else {
        return;
    };
    assets.spawned = true;
    // Readers have completed. Keep decoded handles, release their encoded copies.
    for (_, path, _) in &assets.slots {
        assets.directory.remove_asset(Path::new(path));
    }
    commands
        .spawn((AssetWorld, Transform::default(), Visibility::default()))
        .with_children(|parent| {
            parent.spawn((
                AssetGlobe,
                Mesh3d(meshes.add(globe)),
                MeshMaterial3d(materials.add(StandardMaterial {
                    base_color_texture: Some(texture),
                    unlit: true,
                    ..default()
                })),
                Transform::default(),
            ));
            crate::sky::spawn(parent, &mut meshes, &mut skies, sky, atmosphere);
        });
}

/// Serialise celestial warm views after the board owners have finished theirs.
/// This bounds simultaneous view-light buffers during cold preparation.
pub fn board_ready(world: &World) -> bool {
    world.resource::<WorldAssets>().spawned
        && world.resource::<crate::nuclear::NuclearRuntime>().ready()
        && world
            .resource::<crate::particles::ParticleRuntime>()
            .ready()
        && world
            .resource::<crate::structures::special::SpecialRuntime>()
            .ready()
        && world
            .resource::<crate::terrain::grid::GridRuntime>()
            .ready()
        && world
            .resource::<crate::structures::sites::SiteRuntime>()
            .ready()
        && world
            .resource::<crate::structures::world_tree::WorldTreeRuntime>()
            .ready()
        && world.resource::<crate::fauna::FaunaRuntime>().ready()
        && world
            .resource::<crate::structures::scene::StructureRuntime>()
            .ready()
        && world
            .resource::<crate::structures::texture_library::TextureLibrary>()
            .ready()
        && world
            .resource::<crate::vegetation::FoliageRuntime>()
            .ready()
        && world
            .resource::<crate::terrain::surface::SurfaceRuntime>()
            .ready()
}
/// Every scene mesh, material and image must exist in Bevy's real GPU stores.
/// This is deliberately separate from AssetServer's CPU/dependency load state.
pub fn prepared(app: &mut App) -> Result<bool, String> {
    if view(app.world()) == AssetView::Idle {
        return Ok(true);
    }
    if let Some(error) = &app.world().resource::<TerrainState>().error {
        return Err(format!("Could not prepare terrain: {error}"));
    }
    let assets = app.world().resource::<WorldAssets>();
    let server = app.world().resource::<AssetServer>();
    for (id, _, slot) in &assets.slots {
        let Some(asset_id) = slot.id() else {
            continue;
        };
        if let Some(LoadState::Failed(error)) = server.get_load_state(asset_id) {
            return Err(format!("Could not decode {id}: {error}"));
        }
        if let Some(RecursiveDependencyLoadState::Failed(error)) =
            server.get_recursive_dependency_load_state(asset_id)
        {
            return Err(format!("Could not decode {id}: {error}"));
        }
    }
    if view(app.world()) == AssetView::Menu
        && !app.world().resource::<crate::menu::MenuRuntime>().ready()
    {
        return Ok(false);
    }
    if view(app.world()) == AssetView::Game {
        if !app
            .world()
            .resource::<crate::labels::scene::LabelRuntime>()
            .ready()
        {
            return Ok(false);
        }
        if !app
            .world()
            .resource::<crate::offworld::scene::OffworldRuntime>()
            .ready()
        {
            return Ok(false);
        }
        if !app
            .world()
            .resource::<crate::space::scene::CelestialRuntime>()
            .ready()
        {
            return Ok(false);
        }
        if !app
            .world()
            .resource::<crate::nuclear::NuclearRuntime>()
            .ready()
        {
            return Ok(false);
        }
        if !app
            .world()
            .resource::<crate::particles::ParticleRuntime>()
            .ready()
        {
            return Ok(false);
        }
        if !app
            .world()
            .resource::<crate::structures::special::SpecialRuntime>()
            .ready()
        {
            return Ok(false);
        }
        if !app
            .world()
            .resource::<crate::terrain::grid::GridRuntime>()
            .ready()
            || !app
                .world()
                .resource::<crate::structures::sites::SiteRuntime>()
                .ready()
            || !assets.spawned
            || !app
                .world()
                .resource::<crate::structures::world_tree::WorldTreeRuntime>()
                .ready()
            || !app.world().resource::<crate::fauna::FaunaRuntime>().ready()
            || !app
                .world()
                .resource::<crate::structures::scene::StructureRuntime>()
                .ready()
            || !app
                .world()
                .resource::<crate::structures::texture_library::TextureLibrary>()
                .ready()
            || !app
                .world()
                .resource::<crate::vegetation::FoliageRuntime>()
                .ready()
            || !app
                .world()
                .resource::<crate::terrain::surface::SurfaceRuntime>()
                .ready()
        {
            return Ok(false);
        }
    }
    let array_images: Vec<_> = assets
        .slots
        .iter()
        .filter_map(|(id, _, slot)| {
            let requirement = RENDERER_ASSETS
                .iter()
                .find(|asset| asset.id == id && asset.array_layers > 0)?;
            if let Slot::Texture(handle) = slot {
                Some((*requirement, handle.id()))
            } else {
                None
            }
        })
        .collect();
    let mesh_ids = scene_mesh_ids(app.world_mut())?;
    if mesh_ids.len() < 2 {
        return Ok(false);
    }
    let image_ids: Vec<_> = app.world().resource::<Assets<Image>>().ids().collect();
    let render = app.sub_app(RenderApp).world();
    let Some(meshes) = render.get_resource::<RenderAssets<RenderMesh>>() else {
        return Ok(false);
    };
    let Some(materials) = render.get_resource::<ErasedRenderAssets<PreparedMaterial>>() else {
        return Ok(false);
    };
    let Some(images) = render.get_resource::<RenderAssets<GpuImage>>() else {
        return Ok(false);
    };
    for (requirement, id) in array_images {
        let Some(image) = images.get(id) else {
            return Ok(false);
        };
        let descriptor = &image.texture_descriptor;
        let format = if requirement.srgb {
            bevy::render::render_resource::TextureFormat::Rgba8UnormSrgb
        } else {
            bevy::render::render_resource::TextureFormat::Rgba8Unorm
        };
        if descriptor.size.depth_or_array_layers != requirement.array_layers
            || descriptor.size.width != descriptor.size.height
            || descriptor.mip_level_count != descriptor.size.width.ilog2() + 1
            || descriptor.format != format
        {
            return Err(format!(
                "Prepared texture array has invalid layers, mips or color space: {}",
                requirement.id
            ));
        }
    }
    Ok(app
        .world()
        .resource::<crate::atmosphere::AtmosphereRuntime>()
        .ready()
        && mesh_ids.iter().all(|(mesh, material)| {
            meshes.get(*mesh).is_some() && material.is_some_and(|id| materials.get(id).is_some())
        })
        && image_ids.iter().all(|id| images.get(*id).is_some()))
}

macro_rules! scene_material_types {
    ($apply:ident) => {
        $apply!(StandardMaterial);
        $apply!(crate::atmosphere::standard::HazeMaterial);
        $apply!(crate::atmosphere::material::HaloMaterial);
        $apply!(crate::sky::SkyMaterial);
        $apply!(crate::space::material::SpaceMaterial);
        $apply!(crate::offworld::materials::ShipPadMaterial);
        $apply!(crate::offworld::materials::ShipParticleMaterial);
        $apply!(crate::labels::material::LabelMaterial);
        $apply!(crate::asteroid::material::AsteroidDustMaterial);
        $apply!(GroundMaterial);
        $apply!(OceanMaterial);
        $apply!(LakeMaskMaterial);
        $apply!(GridMaterial);
        $apply!(FoliageMaterial);
        $apply!(StructureMaterial);
        $apply!(WorldTreeMaterial);
        $apply!(MossMaterial);
        $apply!(SiteMaterial);
        $apply!(SpecialMaterial);
        $apply!(AccessoryMaterial);
        $apply!(ParticleMaterial);
        $apply!(NuclearMaterial);
        $apply!(NuclearMaskMaterial);
        $apply!(crate::nuclear::blast_material::WaveMaterial);
        $apply!(crate::nuclear::debris_material::DebrisMaterial);
    };
}

/// Count the same complete family set used by the real GPU readiness gate.
pub fn material_count(world: &World) -> usize {
    let mut count = 0;
    macro_rules! count_type {
        ($material:ty) => {
            count += world
                .get_resource::<Assets<$material>>()
                .map_or(0, Assets::len);
        };
    }
    scene_material_types!(count_type);
    count
}

type MeshUpload = (
    bevy::asset::AssetId<Mesh>,
    Option<bevy::asset::UntypedAssetId>,
);
/// Include every current scene material family. GPU residency is still checked
/// against PreparedMaterial, regardless of the CPU asset's concrete type.
fn scene_mesh_ids(world: &mut World) -> Result<Vec<MeshUpload>, String> {
    let mut materials = std::collections::HashMap::new();
    fn collect<M: Material>(
        world: &mut World,
        materials: &mut std::collections::HashMap<Entity, bevy::asset::UntypedAssetId>,
    ) -> Result<(), String> {
        for (e, m) in world
            .query_filtered::<(Entity, &MeshMaterial3d<M>), With<Mesh3d>>()
            .iter(world)
        {
            if materials.insert(e, m.0.id().untyped()).is_some() {
                return Err("Scene mesh has multiple material families".into());
            }
        }
        Ok(())
    }
    macro_rules! collect_type {
        ($material:ty) => {
            collect::<$material>(world, &mut materials)?;
        };
    }
    scene_material_types!(collect_type);
    Ok(world
        .query::<(Entity, &Mesh3d)>()
        .iter(world)
        .map(|(e, m)| (m.0.id(), materials.get(&e).copied()))
        .collect())
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn travel_retains_prepared_destinations_and_request_generation() {
        use openmars_scene::view::BodyId;
        let mut world = World::new();
        let board = world.spawn(Transform::default()).id();
        world.insert_resource(WorldAssets {
            request: 3,
            view: AssetView::Game,
            focus: FocusTarget::Body(BodyId::Venus),
            spawned: true,
            slots: vec![
                (
                    "textures/planets/mars/surface".into(),
                    "mars.webp".into(),
                    Slot::Texture(Handle::default()),
                ),
                (
                    "textures/planets/venus/surface".into(),
                    "venus.webp".into(),
                    Slot::Texture(Handle::default()),
                ),
            ],
            ..default()
        });
        assert!(change_focus(&mut world, FocusTarget::Body(BodyId::Earth)).unwrap());
        assert!(world.get_entity(board).is_ok());
        assert!(world.resource::<WorldAssets>().spawned);
        assert_eq!(world.resource::<WorldAssets>().request, 3);
        assert_eq!(world.resource::<WorldAssets>().slots.len(), 2);
        let demand = demand(&world);
        assert!(
            !demand
                .assets
                .iter()
                .any(|a| a.id == "textures/planets/mars/surface")
        );
        assert!(
            demand
                .assets
                .iter()
                .any(|a| a.id == "textures/planets/moon/surface")
        );
        assert!(!change_focus(&mut world, FocusTarget::Body(BodyId::Earth)).unwrap());
        assert_eq!(world.resource::<WorldAssets>().request, 3);
    }
    #[test]
    fn stale_and_out_of_audience_installs_are_rejected_before_decoding() {
        let mut world = World::new();
        world.insert_resource(WorldAssets {
            request: 2,
            view: AssetView::Menu,
            ..default()
        });
        assert_eq!(
            install(
                &mut world,
                1,
                "models/phobos".into(),
                "phobos.glb".into(),
                vec![1]
            )
            .unwrap_err(),
            "Stale asset request"
        );
        assert_eq!(
            install(
                &mut world,
                2,
                "models/rock".into(),
                "rock.glb".into(),
                vec![1]
            )
            .unwrap_err(),
            "Asset is outside the requested view"
        );
        assert_eq!(source_count(&world), 0);
    }
    #[test]
    fn mesh_readiness_accepts_each_rendered_material_family_and_rejects_ambiguity() {
        let mut world = World::new();
        let mesh = || Mesh3d(Handle::default());
        world.spawn((
            mesh(),
            MeshMaterial3d::<StandardMaterial>(Handle::default()),
        ));
        world.spawn((mesh(), MeshMaterial3d::<GroundMaterial>(Handle::default())));
        world.spawn((mesh(), MeshMaterial3d::<OceanMaterial>(Handle::default())));
        world.spawn((
            mesh(),
            MeshMaterial3d::<LakeMaskMaterial>(Handle::default()),
        ));
        world.spawn((mesh(), MeshMaterial3d::<GridMaterial>(Handle::default())));
        world.spawn((mesh(), MeshMaterial3d::<FoliageMaterial>(Handle::default())));
        world.spawn((
            mesh(),
            MeshMaterial3d::<StructureMaterial>(Handle::default()),
        ));
        world.spawn((
            mesh(),
            MeshMaterial3d::<WorldTreeMaterial>(Handle::default()),
        ));
        world.spawn((mesh(), MeshMaterial3d::<MossMaterial>(Handle::default())));
        world.spawn((mesh(), MeshMaterial3d::<SiteMaterial>(Handle::default())));
        world.spawn((mesh(), MeshMaterial3d::<SpecialMaterial>(Handle::default())));
        world.spawn((
            mesh(),
            MeshMaterial3d::<AccessoryMaterial>(Handle::default()),
        ));
        world.spawn((
            mesh(),
            MeshMaterial3d::<ParticleMaterial>(Handle::default()),
        ));
        world.spawn((mesh(), MeshMaterial3d::<NuclearMaterial>(Handle::default())));
        world.spawn((
            mesh(),
            MeshMaterial3d::<NuclearMaskMaterial>(Handle::default()),
        ));
        world.spawn((
            mesh(),
            MeshMaterial3d::<crate::nuclear::blast_material::WaveMaterial>(Handle::default()),
        ));
        world.spawn((
            mesh(),
            MeshMaterial3d::<crate::nuclear::debris_material::DebrisMaterial>(Handle::default()),
        ));
        world.spawn((
            mesh(),
            MeshMaterial3d::<crate::asteroid::material::AsteroidDustMaterial>(Handle::default()),
        ));
        world.spawn((
            mesh(),
            MeshMaterial3d::<crate::atmosphere::material::HaloMaterial>(Handle::default()),
        ));
        world.spawn((
            mesh(),
            MeshMaterial3d::<crate::atmosphere::standard::HazeMaterial>(Handle::default()),
        ));
        world.spawn((
            mesh(),
            MeshMaterial3d::<crate::sky::SkyMaterial>(Handle::default()),
        ));
        world.spawn((
            mesh(),
            MeshMaterial3d::<crate::space::material::SpaceMaterial>(Handle::default()),
        ));
        world.spawn((
            mesh(),
            MeshMaterial3d::<crate::offworld::materials::ShipPadMaterial>(Handle::default()),
        ));
        world.spawn((
            mesh(),
            MeshMaterial3d::<crate::offworld::materials::ShipParticleMaterial>(Handle::default()),
        ));
        world.spawn((
            Mesh3d(Handle::default()),
            MeshMaterial3d::<crate::labels::material::LabelMaterial>(Handle::default()),
        ));
        let ids = scene_mesh_ids(&mut world).unwrap();
        assert_eq!(ids.len(), 25);
        assert!(ids.iter().all(|(_, material)| material.is_some()));
        let types: std::collections::HashSet<_> =
            ids.iter().map(|(_, material)| material.unwrap()).collect();
        assert_eq!(
            types.len(),
            25,
            "erased IDs retain their material asset types"
        );
        world.spawn(mesh());
        assert_eq!(
            scene_mesh_ids(&mut world)
                .unwrap()
                .iter()
                .filter(|(_, material)| material.is_none())
                .count(),
            1
        );
        world.spawn((
            mesh(),
            MeshMaterial3d::<StandardMaterial>(Handle::default()),
            MeshMaterial3d::<GroundMaterial>(Handle::default()),
        ));
        assert_eq!(
            scene_mesh_ids(&mut world).unwrap_err(),
            "Scene mesh has multiple material families"
        );
    }
}
