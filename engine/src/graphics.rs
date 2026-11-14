//! Rendering policy shared by the main and preparation cameras. Applied before
//! initial preparation; changing resident quality creates a fresh renderer.
use bevy::{anti_alias::fxaa::Fxaa, prelude::*};
use openmars_scene::view::{GraphicsSettings, Quality};

#[derive(Resource, Default)]
pub struct GraphicsState(pub GraphicsSettings);

/// Warm color views must specialize the same PBR pipelines as the game view.
/// Transmission quality is part of Bevy's view key even for opaque materials.
pub fn prepare_views(world: &mut World) {
    use bevy::{camera::visibility::RenderLayers, pbr::ScreenSpaceTransmission};
    let Some(transmission) = world
        .query_filtered::<&ScreenSpaceTransmission, With<crate::assets::AssetCamera>>()
        .iter(world)
        .next()
        .cloned()
    else {
        return;
    };
    let views: Vec<_> = world
        .query_filtered::<(Entity, &RenderLayers, Option<&ScreenSpaceTransmission>), With<Camera3d>>()
        .iter(world)
        .filter(|(_, layers, old)| {
            **layers == RenderLayers::layer(2)
                && old.is_none_or(|old| {
                    old.quality != transmission.quality || old.steps != transmission.steps
                })
        })
        .map(|(entity, _, _)| entity)
        .collect();
    for entity in views {
        world.entity_mut(entity).insert(transmission.clone());
    }
}

pub fn antialiasing(settings: Res<GraphicsState>, mut cameras: Query<&mut Fxaa>) {
    for mut fxaa in &mut cameras {
        if fxaa.enabled != settings.0.antialiasing {
            fxaa.enabled = settings.0.antialiasing;
        }
    }
}

pub fn effect_count(world: &World, full: usize) -> usize {
    count(world.resource::<GraphicsState>().0.effects, full)
}
fn count(quality: Quality, full: usize) -> usize {
    let divisor = match quality {
        Quality::Low => 3,
        Quality::Medium => 2,
        Quality::High => 1,
    };
    full.div_ceil(divisor)
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn color_preparation_uses_the_game_view_quality_without_touching_masks() {
        use bevy::{
            camera::visibility::RenderLayers,
            pbr::{ScreenSpaceTransmission, ScreenSpaceTransmissionQuality},
        };
        let mut world = World::new();
        world.spawn((
            crate::assets::AssetCamera,
            ScreenSpaceTransmission {
                steps: 2,
                quality: ScreenSpaceTransmissionQuality::High,
            },
        ));
        let warm = world
            .spawn((Camera3d::default(), RenderLayers::layer(2)))
            .id();
        let mask = world
            .spawn((Camera3d::default(), RenderLayers::layer(1)))
            .id();
        prepare_views(&mut world);
        let prepared = world.get::<ScreenSpaceTransmission>(warm).unwrap();
        assert_eq!(prepared.quality, ScreenSpaceTransmissionQuality::High);
        assert_eq!(prepared.steps, 2);
        assert!(world.get::<ScreenSpaceTransmission>(mask).is_none());
    }
    #[test]
    fn quality_reduces_real_pool_counts_without_losing_small_effects() {
        assert_eq!(
            [
                count(Quality::Low, 45),
                count(Quality::Medium, 45),
                count(Quality::High, 45)
            ],
            [15, 23, 45]
        );
        assert_eq!(count(Quality::Low, 1), 1);
        assert_eq!(count(Quality::Low, 0), 0);
    }
}
