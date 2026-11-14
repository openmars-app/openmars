//! Renderer-local event ownership. Snapshots establish a baseline; only live
//! arrivals may start an effect, after its actual GPU dependencies are ready.
use bevy::prelude::*;
use std::collections::{BTreeMap, BTreeSet};
type Coordinate = (i32, i32, i32);
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Start {
    pub step: u64,
    pub motion_step: u64,
}
impl Start {
    pub fn seconds(self) -> (f64, f64) {
        (self.step as f64 / 60., self.motion_step as f64 / 60.)
    }
    pub fn coverage(self, step: u64, reduced: bool) -> f32 {
        let age = step.saturating_sub(self.step);
        if reduced {
            (age as f32 / 9.).min(1.)
        } else if age < 60 {
            0.
        } else {
            1.
        }
    }
    pub fn ended(self, step: u64, reduced: bool) -> bool {
        step.saturating_sub(self.step) >= if reduced { 9 } else { 128 }
    }
}
#[derive(Resource, Default)]
pub struct Timeline {
    seen: BTreeSet<Coordinate>,
    events: BTreeMap<Coordinate, Option<Start>>,
}
impl Timeline {
    pub fn observe(&mut self, keys: BTreeSet<Coordinate>, quiet: bool) {
        if quiet {
            self.events.clear();
        } else {
            self.events.retain(|c, _| keys.contains(c));
            for c in keys.difference(&self.seen) {
                self.events.insert(*c, None);
            }
        }
        self.seen = keys;
    }
    pub fn advance(&mut self, ready: &BTreeSet<Coordinate>, now: Start, reduced: bool) {
        self.events.retain(|_, start| {
            start.is_none_or(|s| now.step.saturating_sub(s.step) < if reduced { 9 } else { 525 })
        });
        for (c, start) in &mut self.events {
            if start.is_none() && ready.contains(c) {
                *start = Some(now);
            }
        }
    }
    pub fn blocks_field(&self, c: Coordinate, step: u64, reduced: bool) -> bool {
        self.events
            .get(&c)
            .is_some_and(|start| start.is_none_or(|s| !reduced && step.saturating_sub(s.step) < 60))
    }
    pub fn diagnostics(&self) -> serde_json::Value {
        serde_json::json!({"events":self.events.iter().map(|(c,s)|serde_json::json!({"coordinate":c,"start":s.map(|s|s.step),"motionStart":s.map(|s|s.motion_step)})).collect::<Vec<_>>()})
    }
    pub fn coordinates(&self) -> impl Iterator<Item = Coordinate> + '_ {
        self.events.keys().copied()
    }
    pub fn live(&self, c: Coordinate) -> bool {
        self.events.contains_key(&c)
    }
    pub fn start(&self, c: Coordinate) -> Option<Start> {
        self.events.get(&c).copied().flatten()
    }
    pub fn coverage(&self, c: Coordinate, step: u64, reduced: bool) -> f32 {
        self.events
            .get(&c)
            .map_or(1., |start| start.map_or(0., |s| s.coverage(step, reduced)))
    }
}
pub fn observe(world: &mut World, quiet: bool) {
    let keys = world
        .resource::<crate::scene::SceneState>()
        .0
        .as_ref()
        .map_or_else(BTreeSet::new, |s| {
            s.tiles
                .iter()
                .filter(|t| {
                    crate::world::is_mars(t) && t.occupant.as_deref() == Some("nuclear-zone-tile")
                })
                .map(|t| (t.q, t.r, t.s))
                .collect()
        });
    world.resource_mut::<Timeline>().observe(keys, quiet);
}
#[cfg(test)]
mod tests {
    use super::*;
    const C: Coordinate = (0, 0, 0);
    fn keys() -> BTreeSet<Coordinate> {
        [C].into()
    }
    #[test]
    fn quiet_snapshots_and_duplicate_live_deliveries_never_replay() {
        let mut t = Timeline::default();
        t.observe(keys(), true);
        t.observe(keys(), false);
        assert!(!t.live(C));
        t.observe(BTreeSet::new(), false);
        t.observe(keys(), false);
        assert!(t.live(C));
        assert!(t.start(C).is_none());
        let at = Start {
            step: 100,
            motion_step: 33,
        };
        t.advance(&keys(), at, false);
        assert_eq!(t.start(C), Some(at));
        t.observe(keys(), false);
        t.advance(
            &keys(),
            Start {
                step: 101,
                motion_step: 34,
            },
            false,
        );
        assert_eq!(t.start(C), Some(at));
        t.observe(keys(), true);
        assert!(!t.live(C));
        assert_eq!(t.coverage(C, 101, false), 1.);
    }
    #[test]
    fn preparation_waits_for_this_coordinate_and_cancellation_cannot_start_later() {
        let mut t = Timeline::default();
        t.observe(keys(), false);
        t.advance(
            &BTreeSet::new(),
            Start {
                step: 900,
                motion_step: 2,
            },
            false,
        );
        assert!(t.start(C).is_none());
        assert_eq!(t.coverage(C, 900, false), 0.);
        t.observe(BTreeSet::new(), false);
        t.advance(
            &keys(),
            Start {
                step: 901,
                motion_step: 3,
            },
            false,
        );
        assert!(!t.live(C));
        t.observe(keys(), false);
        t.advance(
            &keys(),
            Start {
                step: 902,
                motion_step: 4,
            },
            false,
        );
        assert_eq!(t.start(C).unwrap().step, 902);
    }
    #[test]
    fn full_impact_and_reduced_fade_share_canonical_time_without_replay() {
        let at = Start {
            step: 100,
            motion_step: 7,
        };
        assert_eq!(at.seconds(), (100. / 60., 7. / 60.));
        let mut field = Timeline::default();
        field.observe(keys(), false);
        assert!(field.blocks_field(C, 999, false));
        assert!(field.blocks_field(C, 999, true));
        field.advance(&keys(), at, false);
        assert!(field.blocks_field(C, 159, false));
        assert!(!field.blocks_field(C, 160, false));
        assert!(!field.blocks_field(C, 100, true));
        assert_eq!(at.coverage(159, false), 0.);
        assert_eq!(at.coverage(160, false), 1.);
        assert!(!at.ended(227, false));
        assert!(at.ended(228, false));
        assert_eq!(at.coverage(100, true), 0.);
        assert_eq!(at.coverage(103, true), 1. / 3.);
        assert_eq!(at.coverage(109, true), 1.);
        let mut t = Timeline::default();
        t.observe(keys(), false);
        t.advance(&keys(), at, true);
        t.advance(
            &keys(),
            Start {
                step: 109,
                motion_step: 7,
            },
            true,
        );
        assert!(!t.live(C));
        t.observe(keys(), false);
        t.advance(
            &keys(),
            Start {
                step: 110,
                motion_step: 8,
            },
            false,
        );
        assert!(!t.live(C));
    }
}
