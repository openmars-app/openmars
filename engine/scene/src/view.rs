//! Local presentation choices; these carry no game authority.
use serde::{Deserialize, Serialize};

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "kebab-case")]
pub enum Quality {
    Low,
    Medium,
    #[default]
    High,
}

/// Client-owned rendering preferences. Never serialized in authoritative games.
#[derive(Clone, Copy, Debug, PartialEq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct GraphicsSettings {
    pub texture_quality: Quality,
    pub render_scale: f32,
    pub antialiasing: bool,
    pub effects: Quality,
    pub frame_limit: u16,
    pub background_brightness: f32,
}
impl Default for GraphicsSettings {
    fn default() -> Self {
        Self {
            texture_quality: Quality::High,
            render_scale: 1.,
            antialiasing: true,
            effects: Quality::High,
            frame_limit: 0,
            background_brightness: 0.65,
        }
    }
}
impl GraphicsSettings {
    pub fn validate(&self) -> Result<(), &'static str> {
        if !self.render_scale.is_finite()
            || !(0.5..=1.5).contains(&self.render_scale)
            || !self.background_brightness.is_finite()
            || !(0.1..=1.).contains(&self.background_brightness)
            || !matches!(self.frame_limit, 0 | 30 | 60 | 90 | 120 | 144 | 165 | 240)
        {
            return Err("Invalid graphics settings");
        }
        Ok(())
    }
}
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "kebab-case")]
pub enum AssetView {
    #[default]
    Idle,
    Menu,
    Game,
}
#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "kebab-case")]
pub enum MenuPlacement {
    Landing,
    Panel,
    Lobby,
}

/// Local app-phase choreography; never inferred from a game generation.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "kebab-case")]
pub enum TileEntry {
    Hidden,
    Reveal,
    #[default]
    Settled,
}

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "kebab-case")]
pub enum WorldLight {
    Dark,
    Sunrise,
    #[default]
    Steady,
}

/// Orthogonal local cues: light rises during lobby fade before tile reveal.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(deny_unknown_fields)]
pub struct WorldEntry {
    pub tiles: TileEntry,
    pub lighting: WorldLight,
}

#[cfg(test)]
mod entry_tests {
    use super::*;
    #[test]
    fn explicit_light_and_tiles_contract_rejects_the_old_or_partial_shape() {
        let value = WorldEntry {
            tiles: TileEntry::Hidden,
            lighting: WorldLight::Sunrise,
        };
        assert_eq!(
            serde_json::from_str::<WorldEntry>(r#"{"tiles":"hidden","lighting":"sunrise"}"#)
                .unwrap(),
            value
        );
        assert!(serde_json::from_str::<WorldEntry>(r#""hidden""#).is_err());
        assert!(serde_json::from_str::<WorldEntry>(r#"{"tiles":"hidden"}"#).is_err());
        assert!(
            serde_json::from_str::<WorldEntry>(
                r#"{"tiles":"hidden","lighting":"dark","generation":1}"#
            )
            .is_err()
        );
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "kebab-case")]
pub enum BodyId {
    Mars,
    Phobos,
    Mercury,
    Venus,
    Earth,
    Luna,
    Ceres,
    Jupiter,
    Ganymede,
    Saturn,
    Uranus,
    Neptune,
}
#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[cfg_attr(
    feature = "export-types",
    ts(tag = "type", content = "body", rename_all = "kebab-case")
)]
#[serde(
    tag = "type",
    content = "body",
    rename_all = "kebab-case",
    deny_unknown_fields
)]
pub enum FocusTarget {
    Body(BodyId),
    OrbitalStation,
    SolarSystem,
}

/// CSS-pixel projections of native world anchors, never authoritative state.
#[derive(Clone, Debug, Serialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
pub struct BodyMarker {
    pub body: BodyId,
    pub point: [f32; 2],
    #[serde(rename = "hitSize")]
    pub hit_size: f32,
}

#[derive(Clone, Debug, Serialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "camelCase")]
pub struct StationMarker {
    pub point: [f32; 2],
    pub hit_size: f32,
    pub details: crate::offworld::SceneStation,
}

#[derive(Clone, Debug, Default, Serialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
pub struct OverlayFrame {
    #[serde(rename = "gameId")]
    pub game_id: Option<String>,
    pub bodies: Vec<BodyMarker>,
    pub station: Option<StationMarker>,
}

impl BodyId {
    pub fn focus_group(self) -> Self {
        match self {
            Self::Phobos => Self::Mars,
            Self::Luna => Self::Earth,
            Self::Ganymede => Self::Jupiter,
            other => other,
        }
    }
    pub fn location(self) -> &'static str {
        match self {
            Self::Mars => "mars",
            Self::Phobos => "phobos",
            Self::Mercury => "mercury",
            Self::Venus => "venus",
            Self::Earth => "earth",
            Self::Luna => "moon",
            Self::Ceres => "ceres",
            Self::Jupiter => "jupiter",
            Self::Ganymede => "ganymede",
            Self::Saturn => "saturn",
            Self::Uranus => "uranus",
            Self::Neptune => "neptune",
        }
    }
    pub fn from_location(value: &str) -> Option<Self> {
        Some(match value {
            "mars" => Self::Mars,
            "phobos" => Self::Phobos,
            "mercury" => Self::Mercury,
            "venus" => Self::Venus,
            "earth" => Self::Earth,
            "moon" => Self::Luna,
            "ceres" => Self::Ceres,
            "jupiter" => Self::Jupiter,
            "ganymede" => Self::Ganymede,
            "saturn" => Self::Saturn,
            "uranus" => Self::Uranus,
            "neptune" => Self::Neptune,
            _ => return None,
        })
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn local_focus_contract_is_explicit_and_rejects_unknown_targets() {
        let target = FocusTarget::Body(BodyId::Luna);
        assert_eq!(
            serde_json::to_value(target).unwrap(),
            serde_json::json!({"type":"body","body":"luna"})
        );
        for target in [
            target,
            FocusTarget::SolarSystem,
            FocusTarget::OrbitalStation,
        ] {
            assert_eq!(
                serde_json::from_value::<FocusTarget>(serde_json::to_value(target).unwrap())
                    .unwrap(),
                target
            );
        }
        for bad in [
            serde_json::json!({"type":"body","body":"unknown"}),
            serde_json::json!({"type":"solar-system","secret":1}),
            serde_json::json!("mars"),
        ] {
            assert!(serde_json::from_value::<FocusTarget>(bad).is_err());
        }
        assert_eq!(BodyId::from_location("moon"), Some(BodyId::Luna));
        assert_eq!(BodyId::Luna.location(), "moon");
    }
}

impl Default for FocusTarget {
    fn default() -> Self {
        Self::Body(BodyId::Mars)
    }
}
