//! Presentation snapshot and the menu greeting. World entities live in `world`.
use bevy::prelude::*;
use openmars_scene::SceneSnapshot;

#[derive(Resource, Default)]
pub struct SceneState(pub Option<SceneSnapshot>);

/// Local browser accessibility preference, independent of authoritative DTOs.
#[derive(Resource, Default)]
pub struct MotionPreference(pub bool);

/// Loading and reconnect snapshots establish a quiet visual baseline. Live
/// effects start only after that baseline has completed a GPU submission.
#[derive(Resource)]
pub struct PresentationState {
    pub settling: bool,
}
impl Default for PresentationState {
    fn default() -> Self {
        Self { settling: true }
    }
}

/// React's app phase controls entry; game snapshots carry only public state.
#[derive(Resource, Default)]
pub struct WorldEntryState(pub openmars_scene::view::WorldEntry);

#[derive(Component)]
pub struct Greeting;

pub fn update(
    menu: Res<crate::menu::MenuState>,
    scene: Res<SceneState>,
    mut greeting: Query<&mut TextColor, With<Greeting>>,
) {
    if scene.is_changed() || menu.is_changed() {
        // Retain the greeting and its text assets between views. Its camera
        // only renders when no authored world is active.
        for mut color in &mut greeting {
            color.0 = Color::WHITE.with_alpha(if scene.0.is_some() || menu.0.is_some() {
                0.0
            } else {
                1.0
            });
        }
    }
}
