//! Bounded regional field histories. A new edit can overlap an earlier blend
//! without restarting it, resampling it, or waiting for the whole board.
use super::{LandscapeSnapshot, MAX_PATCHES, transition::lake_blend_weight};
use crate::geometry::{
    BoardPoint,
    terrain::{PATCH_SIZE, TerrainError},
};
use std::{
    collections::{BTreeMap, BTreeSet},
    sync::Arc,
};
const STEPS: u64 = 36;
pub const MAX_FIELD_LAYERS: usize = MAX_PATCHES * 2;
#[derive(Clone)]
struct Target {
    snapshot: Arc<LandscapeSnapshot>,
    activation: Option<u64>,
}
#[derive(Clone)]
struct PatchTransition {
    base: Arc<LandscapeSnapshot>,
    targets: Vec<Target>,
}
impl PatchTransition {
    fn remaining(&self, step: u64) -> usize {
        self.targets
            .iter()
            .rev()
            .take_while(|t| {
                t.activation
                    .is_none_or(|start| step.saturating_sub(start) < STEPS)
            })
            .count()
    }
    fn prune(&mut self, step: u64, force: bool) {
        let finished = self.targets.len() - if force { 0 } else { self.remaining(step) };
        if finished > 0 {
            self.base = self.targets[finished - 1].snapshot.clone();
            self.targets.drain(..finished);
        }
    }
}
/// Three byte-sized layer indices fit exactly in one float vertex attribute.
/// -1 starts on upload activation, -2 is complete, -3 is an absent second blend.
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct FieldSlot {
    pub layers: [u32; 3],
    pub births: [f32; 2],
}
impl FieldSlot {
    pub fn settled(layer: u32) -> Self {
        Self {
            layers: [layer; 3],
            births: [-2., -3.],
        }
    }
    pub fn packed_layers(self) -> Option<f32> {
        (self.layers.iter().all(|i| *i < MAX_FIELD_LAYERS as u32)
            && self.births.iter().all(|v| v.is_finite()))
        .then(|| (self.layers[0] | self.layers[1] << 8 | self.layers[2] << 16) as f32)
    }
}
#[derive(Clone)]
pub struct FieldTimeline {
    next: Arc<LandscapeSnapshot>,
    patches: BTreeMap<[i32; 2], PatchTransition>,
}
impl FieldTimeline {
    fn changed(previous: &LandscapeSnapshot, next: &LandscapeSnapshot, key: [i32; 2]) -> bool {
        previous.patch(key).map(|p| p.signature()) != next.patch(key).map(|p| p.signature())
    }
    fn keys_for(
        previous: Option<&Self>,
        next: &LandscapeSnapshot,
        step: u64,
    ) -> BTreeSet<[i32; 2]> {
        let mut keys: BTreeSet<_> = next.patches().map(|p| p.key()).collect();
        if let Some(old) = previous {
            for (key, p) in &old.patches {
                if p.remaining(step) > 0 || old.next.patch(*key).is_some() {
                    keys.insert(*key);
                }
            }
        }
        keys
    }
    pub fn can_replace(&self, next: &LandscapeSnapshot, step: u64) -> bool {
        let keys = Self::keys_for(Some(self), next, step);
        if keys.len() > MAX_PATCHES {
            return false;
        }
        let mut layers = 0;
        for key in keys {
            let remaining = self.patches.get(&key).map_or(0, |p| p.remaining(step));
            let changed = Self::changed(&self.next, next, key);
            if changed && remaining >= 2 {
                return false;
            }
            layers += 1 + remaining + usize::from(changed);
        }
        layers <= MAX_FIELD_LAYERS
    }
    pub fn prepare(
        previous: Option<&Self>,
        next: Arc<LandscapeSnapshot>,
        step: u64,
    ) -> Result<Self, TerrainError> {
        if previous.is_some_and(|old| {
            old.next.projection() != next.projection()
                || old.next.sampler().body() != next.sampler().body()
                || !old.can_replace(&next, step)
        }) {
            return Err(TerrainError::OutsideDomain);
        }
        let keys = Self::keys_for(previous, &next, step);
        if keys.len() > MAX_PATCHES {
            return Err(TerrainError::ResourceLimit);
        }
        let mut patches = BTreeMap::new();
        for key in keys {
            let mut patch = previous
                .and_then(|old| old.patches.get(&key))
                .cloned()
                .unwrap_or_else(|| PatchTransition {
                    base: previous.map_or_else(|| next.clone(), |old| old.next.clone()),
                    targets: Vec::new(),
                });
            patch.prune(step, false);
            if previous.is_some_and(|old| Self::changed(&old.next, &next, key)) {
                patch.targets.push(Target {
                    snapshot: next.clone(),
                    activation: None,
                });
            }
            if patch.targets.is_empty() {
                patch.base = next.clone();
            }
            patches.insert(key, patch);
        }
        Ok(Self { next, patches })
    }
    pub fn snapshot(&self) -> &Arc<LandscapeSnapshot> {
        &self.next
    }
    pub fn keys(&self) -> impl Iterator<Item = [i32; 2]> + '_ {
        self.patches.keys().copied()
    }
    pub fn layers(&self) -> impl Iterator<Item = ([i32; 2], &Arc<LandscapeSnapshot>)> + '_ {
        self.patches.iter().flat_map(|(key, p)| {
            std::iter::once(&p.base)
                .chain(p.targets.iter().map(|t| &t.snapshot))
                .map(move |s| (*key, s))
        })
    }
    pub fn slots(&self) -> Vec<FieldSlot> {
        let mut layer = 0u32;
        self.patches
            .values()
            .map(|p| {
                let mut slot = FieldSlot::settled(layer);
                for (i, t) in p.targets.iter().enumerate() {
                    slot.layers[i + 1] = layer + i as u32 + 1;
                    slot.births[i] = t.activation.map_or(-1., |s| s as f32 / 60.);
                }
                if p.targets.len() == 1 {
                    slot.layers[2] = slot.layers[1];
                }
                layer += 1 + p.targets.len() as u32;
                slot
            })
            .collect()
    }
    pub fn activate(&mut self, step: u64, settled: bool) {
        for p in self.patches.values_mut() {
            for t in &mut p.targets {
                t.activation.get_or_insert(step);
            }
        }
        self.settle(step, settled);
    }
    pub fn settle(&mut self, step: u64, force: bool) -> bool {
        let mut changed = false;
        for p in self.patches.values_mut() {
            let count = p.targets.len();
            p.prune(step, force);
            changed |= count != p.targets.len();
            if p.targets.is_empty() {
                p.base = self.next.clone();
            }
        }
        changed
    }
    pub fn transitioning(&self) -> bool {
        self.patches.values().any(|p| !p.targets.is_empty())
    }
    pub fn sample_revision(&self, step: u64) -> (u64, Option<u64>, u64) {
        let previous = self
            .patches
            .values()
            .find(|p| !p.targets.is_empty())
            .map(|p| p.base.revision());
        (
            self.next.revision(),
            previous,
            if previous.is_some() { step } else { 0 },
        )
    }
    pub fn height(&self, point: BoardPoint, step: u64) -> Result<f64, TerrainError> {
        if !point.x.is_finite() || !point.y.is_finite() {
            return Err(TerrainError::NonFinite);
        }
        let key = [
            (point.x / PATCH_SIZE).floor() as i32,
            (point.y / PATCH_SIZE).floor() as i32,
        ];
        let Some(p) = self.patches.get(&key) else {
            return self.next.sampler().height(point).map(f64::from);
        };
        let initial = p.base.height_shore(point)?;
        let (mut height, mut shore) = (f64::from(initial[0]), f64::from(initial[1]));
        for t in &p.targets {
            let next = t.snapshot.height_shore(point)?;
            let elapsed = t
                .activation
                .map_or(0., |start| step.saturating_sub(start) as f64 / 36.);
            let w = lake_blend_weight(shore, f64::from(next[1]), elapsed)?;
            height += (f64::from(next[0]) - height) * w;
            shore += (f64::from(next[1]) - shore) * w;
        }
        Ok(height)
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    use crate::{
        geometry::{AxialPoint, BodyProjection},
        landscape::*,
    };
    use std::num::NonZeroUsize;
    #[test]
    fn layer_indices_round_trip_exactly_through_float_vertex_storage() {
        for layers in [[0, 0, 0], [1, 127, 255], [255, 255, 255]] {
            let slot = FieldSlot {
                layers,
                births: [0., -1.],
            };
            let bits = slot.packed_layers().unwrap() as u32;
            assert_eq!([bits & 255, (bits >> 8) & 255, (bits >> 16) & 255], layers);
        }
        assert!(
            FieldSlot {
                layers: [0, 1, 256],
                births: [0.; 2]
            }
            .packed_layers()
            .is_none()
        );
        assert!(
            FieldSlot {
                layers: [0; 3],
                births: [f32::NAN, 0.]
            }
            .packed_layers()
            .is_none()
        );
    }
    fn snapshot(revision: u64, cells: &[(i32, i32, LandscapeKind)]) -> Arc<LandscapeSnapshot> {
        let sources = cells
            .iter()
            .map(|&(q, r, kind)| LandscapeSource {
                coordinate: [q, r, -q - r],
                kind,
                seed: 17,
                layout: None,
            })
            .collect();
        let mut builder = LandscapeBuilder::new();
        builder
            .begin(
                LandscapeInput::new(
                    BodyId::Mars,
                    BodyProjection::MARS,
                    revision,
                    73,
                    None,
                    sources,
                )
                .unwrap(),
            )
            .unwrap();
        while matches!(builder.state(), PreparationState::Preparing { .. }) {
            builder.step(NonZeroUsize::new(1024).unwrap()).unwrap();
        }
        builder.take_ready().unwrap()
    }
    #[test]
    fn disjoint_edits_preserve_existing_clocks_and_only_overlap_waits() {
        use LandscapeKind::{Greenery, Ocean};
        let initial = snapshot(1, &[(-3, 0, Ocean)]);
        let weak = Arc::downgrade(&initial);
        let base = FieldTimeline::prepare(None, initial.clone(), 0).unwrap();
        let first = snapshot(2, &[(-3, 0, Ocean), (3, 0, Ocean)]);
        let mut growing = FieldTimeline::prepare(Some(&base), first, 100).unwrap();
        growing.activate(100, false);
        let second = snapshot(3, &[(-3, 0, Ocean), (3, 0, Ocean), (-3, 1, Greenery)]);
        assert!(growing.transitioning());
        assert!(growing.can_replace(&second, 110));
        let mut combined = FieldTimeline::prepare(Some(&growing), second, 110).unwrap();
        let point = BodyProjection::MARS.hex_center(AxialPoint { q: 3, r: 0 });
        let key = [
            (point.x / PATCH_SIZE).floor() as i32,
            (point.y / PATCH_SIZE).floor() as i32,
        ];
        let slot = combined.slots()[combined.keys().position(|k| k == key).unwrap()];
        assert_eq!(slot.births[0], 100. / 60.);
        combined.activate(110, false);
        for step in [110, 115, 125, 135] {
            assert_eq!(
                combined.height(point, step).unwrap(),
                growing.height(point, step).unwrap(),
                "carrying a region must not restart or snap it"
            );
        }
        let overlap = snapshot(
            4,
            &[
                (-3, 0, Ocean),
                (3, 0, Ocean),
                (-3, 1, Greenery),
                (3, -1, Greenery),
            ],
        );
        assert!(combined.can_replace(&overlap, 120));
        let mut overlapping = FieldTimeline::prepare(Some(&combined), overlap, 120).unwrap();
        assert_eq!(
            overlapping.height(point, 120).unwrap(),
            combined.height(point, 120).unwrap(),
            "the new blend starts from the exact still-moving surface"
        );
        overlapping.activate(120, false);
        let third = snapshot(
            5,
            &[
                (-3, 0, Ocean),
                (3, 0, Ocean),
                (-3, 1, Greenery),
                (3, -1, Greenery),
                (3, 1, Greenery),
            ],
        );
        assert!(!overlapping.can_replace(&third, 125));
        assert!(
            overlapping.can_replace(&third, 140),
            "an unrelated region cannot block the far region"
        );
        drop(overlapping);
        drop(initial);
        drop(base);
        drop(growing);
        assert!(weak.upgrade().is_some());
        assert!(combined.settle(136, false));
        assert!(combined.transitioning());
        assert!(
            weak.upgrade().is_none(),
            "finished regions release their history independently"
        );
        combined.settle(146, false);
        assert!(!combined.transitioning());
        assert_eq!(combined.sample_revision(150).2, 0);
    }
    #[test]
    fn quiet_snapshots_are_complete_and_removals_keep_their_original_front() {
        let initial = snapshot(1, &[(0, 0, LandscapeKind::Ocean)]);
        let base = FieldTimeline::prepare(None, initial.clone(), 0).unwrap();
        let empty = snapshot(2, &[]);
        let mut removal = FieldTimeline::prepare(Some(&base), empty.clone(), 100).unwrap();
        assert!(removal.keys().count() > 0);
        assert!(removal.slots().iter().all(|slot| slot.births[0] == -1.));
        let point = BoardPoint { x: 0., y: 0. };
        removal.activate(100, false);
        assert_eq!(
            removal.height(point, 100).unwrap(),
            f64::from(initial.sampler().height(point).unwrap())
        );
        removal.settle(118, true);
        assert_eq!(
            removal.height(point, 118).unwrap(),
            f64::from(empty.sampler().height(point).unwrap())
        );
        assert!(!removal.transitioning());
        assert_eq!(
            FieldTimeline::prepare(Some(&removal), empty, 140)
                .unwrap()
                .keys()
                .count(),
            0
        );
        assert!(
            removal
                .height(BoardPoint { x: f64::NAN, y: 0. }, 140)
                .is_err()
        );
    }
}
