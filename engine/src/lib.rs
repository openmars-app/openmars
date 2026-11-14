//! Browser world renderer. React owns the DOM, frame scheduling, and game UI.

#[cfg(target_arch = "wasm32")]
mod web;
#[cfg(target_arch = "wasm32")]
pub use web::Engine;

#[cfg(all(feature = "webgpu", feature = "webgl2"))]
compile_error!("Build WebGPU and WebGL2 as separate artifacts");

use bevy::{
    camera::{ManualTextureViewHandle, RenderTarget},
    prelude::*,
};
mod assets;
pub mod atmosphere;
pub mod entry;
pub mod fauna;
#[cfg(any(target_arch = "wasm32", test))]
mod frame_pacing;
mod graphics;
pub mod labels;
mod lighting;
mod menu;
pub mod nuclear;
pub mod offworld;
pub mod particles;
pub mod presentation;
mod profiling;
mod scene;
mod sky;
pub mod space;
pub mod structures;
pub mod terrain;
pub mod transparent;
pub mod vegetation;
mod world;

const SURFACE: ManualTextureViewHandle = ManualTextureViewHandle(0);

fn setup(mut commands: Commands) {
    let camera = commands
        .spawn((Camera2d, Msaa::Off, RenderTarget::TextureView(SURFACE)))
        .id();
    commands
        .spawn((
            Node {
                width: percent(100),
                height: percent(100),
                align_items: AlignItems::Center,
                justify_content: JustifyContent::Center,
                ..default()
            },
            UiTargetCamera(camera),
        ))
        .with_children(|parent| {
            parent.spawn((
                scene::Greeting,
                Text::new("Hello world"),
                TextFont {
                    font_size: FontSize::Px(40.0),
                    ..default()
                },
                TextColor(Color::WHITE),
            ));
        });
}
