//! Spherical board presentation and local camera interaction. No game rules.
use crate::{assets::AssetCamera, scene::SceneState};
use bevy::{camera::SubCameraView, prelude::*};
use openmars_scene::{
    SceneEvent, SceneTile,
    geometry::{AxialPoint, BodyProjection, HudInsets, Ray3, camera_frame},
};

#[derive(Resource)]
pub struct WorldView {
    pub placement: crate::space::focus::PlacementCamera,
    pub binding: Transform,
    pub focus: openmars_scene::view::FocusTarget,
    pub yaw: f32,
    pub pitch: f32,
    pub distance: f32,
    pub size: Vec2,
    pub physical: UVec2,
    pub insets: HudInsets,
    pub compact: bool,
    pub hovered: Option<String>,
}
impl Default for WorldView {
    fn default() -> Self {
        Self {
            placement: Default::default(),
            binding: Transform::IDENTITY,
            focus: openmars_scene::view::FocusTarget::default(),
            yaw: 0.0,
            pitch: 0.0,
            distance: 6.8,
            size: Vec2::ONE,
            physical: UVec2::ONE,
            insets: HudInsets::default(),
            compact: false,
            hovered: None,
        }
    }
}
impl WorldView {
    pub fn eye(&self) -> Vec3 {
        self.binding.transform_point(
            Vec3::new(
                self.yaw.sin() * self.pitch.cos(),
                self.pitch.sin(),
                self.yaw.cos() * self.pitch.cos(),
            ) * self.distance,
        )
    }
}
#[derive(Component)]
pub struct BoardRoot;
#[derive(Component)]
pub struct BoardTile(pub SceneTile);

pub fn reset(world: &mut World) {
    let roots: Vec<_> = world
        .query_filtered::<Entity, With<BoardRoot>>()
        .iter(world)
        .collect();
    for entity in roots {
        world.despawn(entity);
    }
    let view = &mut *world.resource_mut::<WorldView>();
    view.binding = Transform::IDENTITY;
    view.placement = Default::default();
    view.focus = openmars_scene::view::FocusTarget::default();
    view.yaw = 0.0;
    view.pitch = 0.0;
    view.distance = 6.8;
    view.hovered = None;
}

pub(crate) fn framed_camera(view: &WorldView) -> Option<(Transform, SubCameraView, f32)> {
    let framing = camera_frame(
        view.size.x.into(),
        view.size.y.into(),
        view.insets,
        view.compact,
    )?;
    Some((
        Transform::from_translation(view.eye()).looking_at(view.binding.translation, Vec3::Y),
        SubCameraView {
            full_size: view.physical,
            size: view.physical,
            offset: -Vec2::new(framing.offset_x as f32, framing.offset_y as f32)
                * view.physical.as_vec2()
                / view.size,
        },
        framing.fov_radians as f32,
    ))
}
pub fn camera(
    view: Res<WorldView>,
    assets: Res<crate::assets::WorldAssets>,
    menu: Res<crate::menu::MenuRuntime>,
    motion: Res<crate::scene::MotionPreference>,
    mut cameras: Query<(&mut Transform, &mut Projection, &mut Camera), With<AssetCamera>>,
) {
    if assets.view == openmars_scene::view::AssetView::Menu {
        for (mut transform, mut projection, mut camera) in &mut cameras {
            *transform = menu.pose(motion.0);
            camera.sub_camera_view = None;
            if let Projection::Perspective(p) = &mut *projection {
                p.fov = 42f32.to_radians();
            }
        }
        return;
    }
    let Some((pose, sub, fov)) = framed_camera(&view) else {
        return;
    };
    for (mut transform, mut projection, mut camera) in &mut cameras {
        *transform = pose;
        if let Projection::Perspective(perspective) = &mut *projection {
            perspective.fov = fov;
            perspective.far = 6000.;
        }
        camera.sub_camera_view = Some(sub);
    }
}

pub fn is_mars(tile: &SceneTile) -> bool {
    tile.location == "mars" && tile.terrain != "empty"
}

/// Retain public tile identities for interaction and diagnostics. Visible world
/// layers own their actual meshes; the scaffold's proxy solids are retired.
pub fn tiles(
    mut commands: Commands,
    state: Res<SceneState>,
    roots: Query<Entity, With<BoardRoot>>,
    existing: Query<(Entity, &BoardTile)>,
) {
    if !state.is_changed() {
        return;
    }
    let Some(scene) = &state.0 else {
        return;
    };
    let root = roots.iter().next().unwrap_or_else(|| {
        commands
            .spawn((BoardRoot, Transform::default(), Visibility::default()))
            .id()
    });
    let active: std::collections::HashSet<_> = scene
        .tiles
        .iter()
        .filter(|tile| is_mars(tile))
        .map(|tile| tile.id.as_str())
        .collect();
    for (entity, tile) in &existing {
        if !active.contains(tile.0.id.as_str()) {
            commands.entity(entity).despawn();
        }
    }
    for tile in scene.tiles.iter().filter(|tile| is_mars(tile)) {
        if let Some((entity, old)) = existing.iter().find(|(_, old)| old.0.id == tile.id) {
            if old.0 != *tile {
                commands.entity(entity).insert(BoardTile(tile.clone()));
            }
        } else {
            let entity = commands
                .spawn((
                    Name::new(format!("tile:{}", tile.id)),
                    BoardTile(tile.clone()),
                    Transform::default(),
                    Visibility::default(),
                ))
                .id();
            commands.entity(root).add_child(entity);
        }
    }
}

fn screen_ray(world: &mut World, x: f32, y: f32) -> Option<Ray3> {
    if !x.is_finite() || !y.is_finite() || !(0.0..=1.0).contains(&x) || !(0.0..=1.0).contains(&y) {
        return None;
    }
    let physical = world.resource::<WorldView>().physical.as_vec2();
    let (camera, transform) = world
        .query_filtered::<(&Camera, &GlobalTransform), With<AssetCamera>>()
        .single(world)
        .ok()?;
    let ray = camera
        .viewport_to_world(transform, Vec2::new(x, y) * physical)
        .ok()?;
    Some(Ray3 {
        origin: ray.origin.to_array().map(f64::from),
        dir: ray.direction.to_array().map(f64::from),
    })
}
pub fn pick_body(world: &mut World, x: f32, y: f32) -> Option<openmars_scene::view::BodyId> {
    let ray = screen_ray(world, x, y)?;
    let seconds = world
        .resource::<crate::terrain::surface::SurfaceRuntime>()
        .motion_step() as f64
        / 60.;
    crate::space::picking::body(ray, seconds, world.resource::<WorldView>().focus)
}
pub fn pick(world: &mut World, x: f32, y: f32) -> Option<String> {
    let ray = screen_ray(world, x, y)?;
    let scene = world.resource::<SceneState>().0.as_ref()?;
    let space = world.resource::<crate::space::scene::CelestialRuntime>();
    let focus = world.resource::<WorldView>().focus;
    crate::space::config::BODIES
        .into_iter()
        .filter(|id| crate::space::picking::active(*id, focus))
        .filter_map(|id| {
            let coordinates: Vec<_> = scene
                .tiles
                .iter()
                .filter(|t| t.location == id.location() && t.terrain != "empty")
                .map(|t| AxialPoint { q: t.q, r: t.r })
                .collect();
            let (projection, transform) = if id == openmars_scene::view::BodyId::Mars {
                (BodyProjection::MARS, Transform::IDENTITY)
            } else {
                space.grid_frame(id)?
            };
            let pose = crate::space::config::BodyPose {
                center: transform.translation.as_dvec3(),
                rotation: transform.rotation.as_dquat(),
            };
            crate::space::picking::tile(ray, projection, pose, &coordinates)
        })
        .min_by(|a, b| a.1.total_cmp(&b.1).then(a.0.cmp(&b.0)))
        .map(|(tile, _)| format!("{},{},{}", tile.q, tile.r, -tile.q - tile.r))
}
pub fn pointer(
    world: &mut World,
    x: f32,
    y: f32,
    screen: Vec2,
    inspect: bool,
) -> Option<SceneEvent> {
    if !screen.is_finite() {
        return None;
    }
    if !inspect && let Some(body) = pick_body(world, x, y) {
        return Some(SceneEvent::BodySelected {
            game_id: world.resource::<SceneState>().0.as_ref()?.game_id.clone(),
            body,
        });
    }
    let tile_id = pick(world, x, y)?;
    let scene = world.resource::<SceneState>().0.as_ref()?;
    Some(if !inspect && scene.selectable_tiles.contains(&tile_id) {
        SceneEvent::TileSelected {
            game_id: scene.game_id.clone(),
            tile_id,
        }
    } else {
        SceneEvent::TileInspected {
            game_id: scene.game_id.clone(),
            tile_id,
            x: screen.x,
            y: screen.y,
        }
    })
}
