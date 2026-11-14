//! Scene-scoped, bounded once-only effects and renderer-to-React audio cues.
use bevy::prelude::*;
use openmars_scene::presentation::{SceneCue, SceneSound, SoundAction, SoundKind};
use std::collections::{BTreeMap, VecDeque};

const MAX_SEEN: usize = 16384;
const MAX_PENDING: usize = 32;

#[derive(Resource, Default)]
pub struct PresentationRuntime {
    seen: BTreeMap<String, SceneCue>,
    pending: VecDeque<SceneCue>,
    active_sounds: BTreeMap<String, String>,
    reported_sounds: BTreeMap<String, String>,
}

impl PresentationRuntime {
    pub fn accept(&mut self, cue: SceneCue, quiet: bool) -> Result<bool, String> {
        cue.validate()?;
        if let Some(previous) = self.seen.get(&cue.id) {
            return if previous == &cue {
                Ok(false)
            } else {
                Err("Presentation event identity conflict".into())
            };
        }
        if self.seen.len() >= MAX_SEEN {
            return Err("Presentation event ledger limit exceeded".into());
        }
        if !quiet && self.pending.len() >= MAX_PENDING {
            return Err("Presentation event queue limit exceeded".into());
        }
        self.seen.insert(cue.id.clone(), cue.clone());
        if !quiet {
            self.pending.push_back(cue);
        }
        Ok(!quiet)
    }
    pub fn pop(&mut self) -> Option<SceneCue> {
        self.pending.pop_front()
    }
    pub fn pending(&self) -> bool {
        !self.pending.is_empty()
    }
    pub fn discard_pending(&mut self) {
        self.pending.clear();
    }
    pub fn start_sound(&mut self, game: &str, id: &str) -> Result<(), String> {
        if self.active_sounds.contains_key(id) {
            return Ok(());
        }
        if self.active_sounds.len() >= 512 {
            return Err("Active sound limit exceeded".into());
        }
        self.active_sounds.insert(id.into(), game.into());
        Ok(())
    }
    pub fn stop_sound(&mut self, id: &str) {
        self.active_sounds.remove(id);
    }
    pub fn take_sounds(&mut self) -> Vec<SceneSound> {
        let mut events = Vec::new();
        for (id, game_id) in &self.reported_sounds {
            if self.active_sounds.get(id) != Some(game_id) {
                events.push(SceneSound {
                    game_id: game_id.clone(),
                    id: id.clone(),
                    sound: SoundKind::NuclearBlast,
                    action: SoundAction::Stop,
                });
            }
        }
        for (id, game_id) in &self.active_sounds {
            if self.reported_sounds.get(id) != Some(game_id) {
                events.push(SceneSound {
                    game_id: game_id.clone(),
                    id: id.clone(),
                    sound: SoundKind::NuclearBlast,
                    action: SoundAction::Start,
                });
            }
        }
        self.reported_sounds.clone_from(&self.active_sounds);
        events
    }
    pub fn clear(&mut self, replaced: bool) {
        self.pending.clear();
        self.active_sounds.clear();
        if replaced {
            self.seen.clear();
        }
    }
    pub fn diagnostics(&self) -> serde_json::Value {
        serde_json::json!({"seen":self.seen.len(),"queued":self.pending.len(),"sounds":self.active_sounds.len()})
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use openmars_scene::presentation::CueKind;
    fn cue(id: usize) -> SceneCue {
        SceneCue {
            game_id: "game".into(),
            id: id.to_string(),
            seed: id as u32,
            kind: CueKind::Asteroid,
        }
    }
    #[test]
    fn delivery_is_bounded_ordered_once_and_quiet_resync_never_replays() {
        let mut r = PresentationRuntime::default();
        assert!(!r.accept(cue(0), true).unwrap());
        assert!(!r.accept(cue(0), false).unwrap());
        for id in 1..=MAX_PENDING {
            assert!(r.accept(cue(id), false).unwrap());
        }
        assert!(r.accept(cue(100), false).is_err());
        assert_eq!(r.pop(), Some(cue(1)));
        assert!(r.accept(cue(100), false).unwrap());
        let mut conflict = cue(2);
        conflict.seed += 1;
        assert!(r.accept(conflict, false).is_err());
        r.clear(false);
        assert!(r.pop().is_none());
        assert!(!r.accept(cue(2), false).unwrap());
        r.clear(true);
        assert!(r.accept(cue(2), false).unwrap());
    }
    #[test]
    fn sound_start_stop_and_resync_cleanup_are_once_only() {
        let mut r = PresentationRuntime::default();
        r.start_sound("game", "blast").unwrap();
        r.start_sound("game", "blast").unwrap();
        assert_eq!(r.take_sounds().len(), 1);
        r.clear(false);
        r.clear(false);
        let ended = r.take_sounds();
        assert_eq!(ended.len(), 1);
        assert_eq!(ended[0].action, SoundAction::Stop);
        r.stop_sound("blast");
        assert!(r.take_sounds().is_empty());
    }
}
