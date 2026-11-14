//! Public presentation cues. These never authorize gameplay or carry private data.
use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SceneCue {
    pub game_id: String,
    pub id: String,
    pub seed: u32,
    pub kind: CueKind,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "kebab-case")]
pub enum CueKind {
    Asteroid,
}

impl SceneCue {
    pub fn validate(&self) -> Result<(), &'static str> {
        if self.game_id.is_empty()
            || self.game_id.len() > 128
            || self.id.is_empty()
            || self.id.len() > 256
        {
            return Err("Invalid presentation cue identity");
        }
        Ok(())
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "camelCase")]
pub struct SceneSound {
    pub game_id: String,
    pub id: String,
    pub sound: SoundKind,
    pub action: SoundAction,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "kebab-case")]
pub enum SoundKind {
    NuclearBlast,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "kebab-case")]
pub enum SoundAction {
    Start,
    Stop,
}
