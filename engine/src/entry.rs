//! Source tile entrance: 15ms per board ordinal, 400ms cubic scale-in.
use bevy::prelude::*;
use openmars_scene::view::TileEntry;
use openmars_scene::view::WorldLight;
#[derive(Clone, Copy, Debug, Default)]
pub enum Sample {
    Hidden,
    #[default]
    Settled,
    Reveal(f64),
}

#[derive(Default)]
struct Sunrise {
    mode: WorldLight,
    start: Option<u64>,
    pending: bool,
    quiet: bool,
    initialized: bool,
}
impl Sunrise {
    fn sample(&mut self, mode: WorldLight, quiet: bool, reduced: bool, step: u64) -> f32 {
        let resync = self.initialized && quiet && !self.quiet;
        self.quiet = quiet;
        self.initialized = true;
        if mode != self.mode {
            self.mode = mode;
            self.start = None;
            self.pending = mode == WorldLight::Sunrise;
        }
        if mode == WorldLight::Dark {
            return 0.;
        }
        if mode == WorldLight::Steady || reduced || resync {
            self.start = None;
            self.pending = false;
            return 1.;
        }
        if self.pending {
            if quiet {
                return 0.;
            }
            self.start = Some(step);
            self.pending = false;
        }
        self.start.map_or(1., |start| {
            let t = (step.saturating_sub(start) as f64 / 150.).min(1.);
            (1. - (1. - t).powi(2)) as f32
        })
    }
}

/// Shared Mars tile clock; all native owners consume the same sampled phase.
#[derive(Resource)]
pub struct WorldEntrance {
    entrance: Entrance,
    sunrise: Sunrise,
    pub tiles: Sample,
    pub light: f32,
}
impl Default for WorldEntrance {
    fn default() -> Self {
        Self {
            entrance: Entrance::default(),
            sunrise: Sunrise::default(),
            tiles: Sample::Settled,
            light: 1.,
        }
    }
}
impl WorldEntrance {
    pub fn diagnostics(&self) -> serde_json::Value {
        serde_json::json!({"light":self.light,"lightStart":self.sunrise.start,"tileScales":[self.tiles.scale(0),self.tiles.scale(30),self.tiles.scale(60)]})
    }
}
pub fn prepare(world: &mut World) {
    if world.resource::<crate::scene::SceneState>().0.is_none() {
        *world.resource_mut::<WorldEntrance>() = WorldEntrance::default();
        return;
    }
    let cue = world.resource::<crate::scene::WorldEntryState>().0;
    let quiet = world.resource::<crate::scene::PresentationState>().settling;
    let reduced = world.resource::<crate::scene::MotionPreference>().0;
    let step = world
        .resource::<crate::terrain::surface::SurfaceRuntime>()
        .motion_step();
    let mut entry = world.resource_mut::<WorldEntrance>();
    entry.tiles = entry.entrance.sample(cue.tiles, quiet, reduced, step);
    entry.light = entry.sunrise.sample(cue.lighting, quiet, reduced, step);
}
impl Sample {
    pub fn shown(self) -> bool {
        !matches!(self, Self::Hidden)
    }
    pub fn scale(self, ordinal: usize) -> f32 {
        match self {
            Self::Hidden => 0.,
            Self::Settled => 1.,
            Self::Reveal(ms) => {
                let t = ((ms - ordinal as f64 * 15.) / 400.).clamp(0., 1.);
                (1. - (1. - t).powi(3)) as f32
            }
        }
    }
}

/// Source Tile order, excluding offworld and empty padding cells. Retained
/// outgoing actors at an occupied coordinate share the current tile's clock.
pub fn tile_scale(world: &World, coordinate: (i32, i32, i32)) -> f32 {
    let sample = world.resource::<WorldEntrance>().tiles;
    world
        .resource::<crate::scene::SceneState>()
        .0
        .as_ref()
        .and_then(|scene| {
            scene
                .tiles
                .iter()
                .filter(|tile| crate::world::is_mars(tile))
                .enumerate()
                .find(|(_, tile)| (tile.q, tile.r, tile.s) == coordinate)
                .map(|(ordinal, _)| sample.scale(ordinal))
        })
        .unwrap_or(1.)
}

/// A warm pass must keep full-size geometry and inherited visibility. Published
/// zero-scale actors are hidden rather than sending singular normal matrices.
pub fn publish(world: &mut World, root: Entity, scale: f32) {
    if world.get::<ActorScale>(root) != Some(&ActorScale(scale)) {
        world.entity_mut(root).insert(ActorScale(scale));
    }
    let visibility = if scale > 0. {
        Visibility::Inherited
    } else {
        Visibility::Hidden
    };
    if world.get::<Visibility>(root) != Some(&visibility) {
        world.entity_mut(root).insert(visibility);
    }
}

#[derive(Component, PartialEq)]
struct ActorScale(f32);

pub fn actors(world: &World) -> serde_json::Value {
    serde_json::Value::Array(world.iter_entities().filter_map(|entity| {
        let scale = entity.get::<ActorScale>()?.0;
        Some(serde_json::json!({"entity":entity.id().to_string(),"scale":scale,"hidden":entity.get::<Visibility>() == Some(&Visibility::Hidden)}))
    }).collect())
}

/// World-space geometry inherits the source Tile's scale around its tangent
/// origin, rather than shrinking around the center of Mars.
pub fn around(origin: Vec3, scale: f32) -> Transform {
    Transform::from_translation(origin * (1. - scale)).with_scale(Vec3::splat(scale.max(0.000001)))
}
#[derive(Default)]
pub struct Entrance {
    mode: TileEntry,
    start: Option<u64>,
    pending: bool,
    quiet: bool,
    initialized: bool,
}
impl Entrance {
    pub fn sample(&mut self, mode: TileEntry, quiet: bool, reduced: bool, step: u64) -> Sample {
        let resync = self.initialized && quiet && !self.quiet;
        self.quiet = quiet;
        self.initialized = true;
        if mode != self.mode {
            self.mode = mode;
            self.start = None;
            self.pending = mode == TileEntry::Reveal;
        }
        if reduced || resync {
            self.start = None;
            self.pending = false;
        }
        if mode == TileEntry::Hidden {
            return Sample::Hidden;
        }
        if self.pending {
            if quiet {
                return Sample::Hidden;
            }
            self.pending = false;
            self.start = Some(step);
        }
        self.start.map_or(Sample::Settled, |start| {
            Sample::Reveal(step.saturating_sub(start) as f64 / 60. * 1000.)
        })
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn parent_scale_keeps_tile_anchor_fixed_instead_of_shrinking_the_planet() {
        let anchor = Vec3::new(0., 0., 2.01);
        let transform = around(anchor, 0.5);
        assert!(transform.transform_point(anchor).abs_diff_eq(anchor, 1e-6));
        assert!(
            transform
                .transform_point(anchor + Vec3::X * 0.2)
                .abs_diff_eq(anchor + Vec3::X * 0.1, 1e-6)
        );
        assert!(around(anchor, 0.).to_matrix().inverse().is_finite());
    }

    #[test]
    fn source_sunrise_waits_for_readiness_and_uses_150_step_quadratic_rise() {
        let mut light = Sunrise::default();
        assert_eq!(light.sample(WorldLight::Dark, true, false, 0), 0.);
        assert_eq!(light.sample(WorldLight::Sunrise, true, false, 700), 0.);
        assert_eq!(light.sample(WorldLight::Sunrise, false, false, 800), 0.);
        assert_eq!(light.sample(WorldLight::Sunrise, false, false, 875), 0.75);
        assert_eq!(light.sample(WorldLight::Sunrise, false, false, 950), 1.);
        assert_eq!(light.sample(WorldLight::Sunrise, true, false, 951), 1.);
        assert_eq!(light.sample(WorldLight::Sunrise, false, false, 952), 1.);
        light.sample(WorldLight::Dark, true, true, 1000);
        assert_eq!(light.sample(WorldLight::Sunrise, false, true, 1001), 1.);
        assert_eq!(light.sample(WorldLight::Sunrise, false, false, 1002), 1.);
    }
    #[test]
    fn source_stagger_waits_for_initial_readiness_and_never_replays_reconnect_or_reduced() {
        let mut entry = Entrance::default();
        assert_eq!(entry.sample(TileEntry::Hidden, true, false, 0).scale(0), 0.);
        assert_eq!(
            entry.sample(TileEntry::Reveal, true, false, 100).scale(0),
            0.
        );
        assert_eq!(
            entry.sample(TileEntry::Reveal, true, false, 300).scale(0),
            0.
        );
        assert_eq!(
            entry.sample(TileEntry::Reveal, false, false, 400).scale(0),
            0.
        );
        let half = entry.sample(TileEntry::Reveal, false, false, 412);
        assert_eq!(half.scale(0), 0.875);
        assert_eq!(half.scale(20), 0.);
        assert_eq!(
            entry.sample(TileEntry::Reveal, false, false, 424).scale(0),
            1.
        );
        assert_eq!(
            entry.sample(TileEntry::Reveal, true, false, 430).scale(20),
            1.
        );
        assert_eq!(
            entry.sample(TileEntry::Reveal, false, false, 431).scale(20),
            1.
        );
        entry.sample(TileEntry::Hidden, false, false, 500);
        assert_eq!(
            entry.sample(TileEntry::Reveal, false, true, 501).scale(20),
            1.
        );
        assert_eq!(
            entry.sample(TileEntry::Reveal, false, false, 502).scale(20),
            1.
        );
    }
}
