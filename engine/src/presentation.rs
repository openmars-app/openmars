//! Scene-scoped renderer-to-host audio events.
use bevy::prelude::*;
use openmars_scene::presentation::{SceneSound, SoundAction, SoundKind};
use std::collections::BTreeMap;

#[derive(Resource, Default)]
pub struct SoundRuntime {
    active_sounds: BTreeMap<String, String>,
    reported_sounds: BTreeMap<String, String>,
}

impl SoundRuntime {
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
    pub fn clear(&mut self) {
        self.active_sounds.clear();
    }
    pub fn diagnostics(&self) -> serde_json::Value {
        serde_json::json!({"sounds":self.active_sounds.len()})
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn sound_start_stop_and_resync_cleanup_are_once_only() {
        let mut r = SoundRuntime::default();
        r.start_sound("game", "blast").unwrap();
        r.start_sound("game", "blast").unwrap();
        assert_eq!(r.take_sounds().len(), 1);
        r.clear();
        r.clear();
        let ended = r.take_sounds();
        assert_eq!(ended.len(), 1);
        assert_eq!(ended[0].action, SoundAction::Stop);
        r.stop_sound("blast");
        assert!(r.take_sounds().is_empty());
    }
}
