//! Original local reveal/completion timing, separate from authoritative state.
use openmars_scene::view::TileEntry;
fn cubic(t: f64) -> f32 {
    (1. - (1. - t.clamp(0., 1.)).powi(3)) as f32
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
    pub fn started(&self) -> Option<u64> {
        self.start
    }
    pub fn sample(&mut self, mode: TileEntry, quiet: bool, reduced: bool, step: u64) -> f32 {
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
            return 0.;
        }
        if self.pending {
            if quiet {
                return 0.;
            }
            self.pending = false;
            self.start = Some(step);
        }
        let Some(start) = self.start else {
            return 1.;
        };
        let age = step.saturating_sub(start);
        if age >= 96 {
            self.start = None;
            return 1.;
        }
        cubic(age.saturating_sub(72) as f64 / 24.)
    }
}
#[derive(Default)]
pub struct Completion {
    previous: Option<bool>,
    start: Option<u64>,
}
impl Completion {
    pub fn started(&self) -> Option<u64> {
        self.start
    }
    pub fn sample(&mut self, complete: Option<bool>, quiet: bool, reduced: bool, step: u64) -> f32 {
        if complete == Some(true) && self.previous == Some(false) && !quiet && !reduced {
            self.start = Some(step);
        }
        self.previous = complete;
        if complete != Some(true) {
            self.start = None;
            return 0.;
        }
        if quiet || reduced {
            self.start = None;
        }
        let Some(start) = self.start else {
            return 1.;
        };
        let age = step.saturating_sub(start);
        if age >= 50 {
            self.start = None;
            return 1.;
        }
        cubic(age as f64 / 50.)
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn entry_uses_explicit_phase_source_delay_and_once_only_reconnect_settlement() {
        let mut e = Entrance::default();
        assert_eq!(e.sample(TileEntry::Hidden, true, false, 0), 0.);
        assert_eq!(e.sample(TileEntry::Reveal, true, false, 100), 0.);
        assert_eq!(
            e.sample(TileEntry::Reveal, true, false, 101),
            0.,
            "initial warm baseline must not cancel reveal"
        );
        assert_eq!(
            e.sample(TileEntry::Reveal, true, false, 300),
            0.,
            "asset preparation cannot consume reveal time"
        );
        assert!(e.started().is_none());
        assert_eq!(e.sample(TileEntry::Reveal, false, false, 301), 0.);
        assert_eq!(e.sample(TileEntry::Reveal, false, false, 373), 0.);
        assert_eq!(e.sample(TileEntry::Reveal, false, false, 385), 0.875);
        assert_eq!(e.sample(TileEntry::Reveal, false, false, 385), 0.875);
        assert_eq!(e.sample(TileEntry::Reveal, false, false, 397), 1.);
        assert_eq!(e.sample(TileEntry::Reveal, false, false, 400), 1.);
        assert_eq!(e.sample(TileEntry::Hidden, false, false, 500), 0.);
        assert_eq!(e.sample(TileEntry::Reveal, false, false, 600), 0.);
        assert_eq!(e.sample(TileEntry::Reveal, true, false, 610), 1.);
        assert_eq!(e.sample(TileEntry::Reveal, false, false, 620), 1.);
        let mut e = Entrance::default();
        assert_eq!(e.sample(TileEntry::Reveal, true, true, 0), 1.);
        assert_eq!(e.sample(TileEntry::Reveal, false, false, 1), 1.);
    }
    #[test]
    fn station_completion_only_animates_live_edges_and_settles_history_and_reduced_motion() {
        let mut c = Completion::default();
        assert_eq!(c.sample(Some(true), true, false, 100), 1.);
        assert_eq!(c.sample(Some(false), false, false, 101), 0.);
        assert_eq!(c.sample(Some(true), false, false, 200), 0.);
        assert_eq!(c.sample(Some(true), false, false, 225), 0.875);
        assert_eq!(c.sample(Some(true), false, false, 225), 0.875);
        assert_eq!(c.sample(Some(true), false, false, 250), 1.);
        assert_eq!(c.sample(Some(false), false, false, 300), 0.);
        assert_eq!(c.sample(Some(true), true, false, 400), 1.);
        assert_eq!(c.sample(Some(false), false, false, 500), 0.);
        assert_eq!(c.sample(Some(true), false, true, 600), 1.);
        assert_eq!(c.sample(Some(true), false, false, 601), 1.);
        assert_eq!(c.sample(None, false, false, 700), 0.);
        assert_eq!(
            c.sample(Some(true), false, false, 701),
            1.,
            "first-seen completed projects are settled"
        );
    }
}
