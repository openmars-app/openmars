//! Rendering-only contract. No game rules, private hand data or network transport.
use serde::{Deserialize, Serialize};
pub mod assets;
pub mod atmosphere;
pub mod climate;
pub mod clock;
pub mod geometry;
pub mod highlights;
pub mod offworld;
pub mod presentation;
pub mod view;

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SceneSnapshot {
    pub offworld: offworld::OffworldScene,
    pub game_id: String,
    pub map_id: String,
    pub generation: u32,
    pub temperature: i32,
    pub oxygen: u32,
    pub oceans: u32,
    pub max_oceans: u32,
    pub venus: u32,
    pub tiles: Vec<SceneTile>,
    pub players: Vec<ScenePlayer>,
    pub selected_tile: Option<String>,
    pub selectable_tiles: Vec<String>,
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SceneTile {
    pub id: String,
    pub q: i32,
    pub r: i32,
    pub s: i32,
    pub location: String,
    pub terrain: String,
    pub display_name: Option<String>,
    pub tags: Vec<String>,
    pub bonuses: Vec<SceneTileBonus>,
    pub occupant: Option<String>,
    pub occupant_display_name: Option<String>,
    pub occupant_tags: Vec<String>,
    pub visual: Option<SceneTileVisual>,
    pub owner_id: Option<String>,
    pub reserved_by: Option<String>,
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SceneTileVisual {
    pub seed: u32,
    pub structure: Option<structure::StructureStyleRequest>,
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(deny_unknown_fields)]
pub struct SceneTileBonus {
    pub r#type: String,
    pub amount: i32,
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct ScenePlayer {
    pub id: String,
    pub color: String,
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(tag = "type", rename_all = "kebab-case", deny_unknown_fields)]
#[cfg_attr(feature = "export-types", ts(tag = "type", rename_all = "kebab-case"))]
pub enum SceneEvent {
    BodySelected {
        #[serde(rename = "gameId")]
        game_id: String,
        body: view::BodyId,
    },
    BodyHovered {
        #[serde(rename = "gameId")]
        game_id: String,
        body: view::BodyId,
    },
    TileHovered {
        #[serde(rename = "gameId")]
        game_id: String,
        #[serde(rename = "tileId")]
        tile_id: Option<String>,
        x: f32,
        y: f32,
    },
    TileSelected {
        #[serde(rename = "gameId")]
        game_id: String,
        #[serde(rename = "tileId")]
        tile_id: String,
    },
    TileInspected {
        #[serde(rename = "gameId")]
        game_id: String,
        #[serde(rename = "tileId")]
        tile_id: String,
        x: f32,
        y: f32,
    },
}

impl SceneSnapshot {
    /// Structural bounds protect the renderer; placement legality stays in Go.
    pub fn validate(&self) -> Result<(), &'static str> {
        self.offworld.validate(&self.players)?;
        if self.game_id.is_empty()
            || self.game_id.len() > 128
            || self.tiles.len() > 4096
            || self.players.len() > 64
        {
            return Err("Scene identity or size is invalid");
        }
        let mut ids = std::collections::BTreeSet::new();
        for tile in &self.tiles {
            let bounded = |value: &str| value.len() <= 128;
            if ![tile.location.as_str(), tile.terrain.as_str()]
                .into_iter()
                .all(bounded)
                || [
                    &tile.display_name,
                    &tile.occupant,
                    &tile.occupant_display_name,
                ]
                .into_iter()
                .flatten()
                .any(|s| !bounded(s))
                || tile.tags.len() > 64
                || tile.occupant_tags.len() > 64
                || tile
                    .tags
                    .iter()
                    .chain(&tile.occupant_tags)
                    .any(|s| !bounded(s))
                || tile.bonuses.len() > 32
                || tile
                    .bonuses
                    .iter()
                    .any(|b| !bounded(&b.r#type) || b.amount.unsigned_abs() > 4096)
                || tile
                    .visual
                    .as_ref()
                    .and_then(|v| v.structure.as_ref())
                    .is_some_and(|s| !s.is_bounded())
            {
                return Err("Scene visual metadata exceeds renderer bounds");
            }
            if tile.q.unsigned_abs() > 4096
                || tile.r.unsigned_abs() > 4096
                || tile.s.unsigned_abs() > 4096
                || i64::from(tile.q) + i64::from(tile.r) + i64::from(tile.s) != 0
            {
                return Err("Invalid scene coordinates");
            }
            if tile.id != format!("{},{},{}", tile.q, tile.r, tile.s) || !ids.insert(&tile.id) {
                return Err("Invalid or duplicate tile identity");
            }
        }
        if self.selectable_tiles.iter().any(|id| !ids.contains(id))
            || self
                .selected_tile
                .as_ref()
                .is_some_and(|id| !ids.contains(id))
        {
            return Err("Selection references an unknown tile");
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    fn snapshot() -> SceneSnapshot {
        SceneSnapshot {
            game_id: "game".into(),
            map_id: "tharsis".into(),
            generation: 1,
            temperature: -30,
            oxygen: 0,
            oceans: 0,
            max_oceans: 9,
            venus: 0,
            tiles: vec![SceneTile {
                id: "0,0,0".into(),
                q: 0,
                r: 0,
                s: 0,
                location: "mars".into(),
                terrain: "land".into(),
                display_name: None,
                tags: vec![],
                bonuses: vec![],
                occupant: None,
                occupant_display_name: None,
                occupant_tags: vec![],
                visual: None,
                owner_id: None,
                reserved_by: None,
            }],
            players: vec![],
            selected_tile: None,
            selectable_tiles: vec![],
            offworld: Default::default(),
        }
    }
    #[test]
    fn preserves_snapshot_json_contract() {
        let scene = snapshot();
        scene.validate().unwrap();
        let value = serde_json::to_value(&scene).unwrap();
        assert_eq!(value["gameId"], "game");
        assert_eq!(
            serde_json::from_value::<SceneSnapshot>(value).unwrap(),
            scene
        );
    }
    #[test]
    fn rejects_private_or_unknown_fields() {
        let mut value = serde_json::to_value(snapshot()).unwrap();
        value["hand"] = serde_json::json!([]);
        assert!(serde_json::from_value::<SceneSnapshot>(value).is_err());
    }
    #[test]
    fn presentation_callers_supply_the_current_offworld_contract() {
        let mut value = serde_json::to_value(snapshot()).unwrap();
        value.as_object_mut().unwrap().remove("offworld");
        assert!(serde_json::from_value::<SceneSnapshot>(value).is_err());
    }
    #[test]
    fn authored_visuals_round_trip_and_reject_unbounded_metadata() {
        let mut scene = snapshot();
        scene.max_oceans = 14;
        scene.tiles[0].visual = Some(SceneTileVisual {
            seed: u32::MAX,
            structure: Some(structure::StructureStyleRequest {
                landmark: Some("hall".into()),
                ..Default::default()
            }),
        });
        scene.tiles[0].occupant_display_name = Some("Capital".into());
        scene.validate().unwrap();
        let json = serde_json::to_string(&scene).unwrap();
        assert_eq!(serde_json::from_str::<SceneSnapshot>(&json).unwrap(), scene);
        scene.tiles[0]
            .visual
            .as_mut()
            .unwrap()
            .structure
            .as_mut()
            .unwrap()
            .details = vec!["pipes".into(); 33];
        assert!(scene.validate().is_err());
        scene.tiles[0].visual = None;
        scene.tiles[0].tags = vec!["tag".into(); 65];
        assert!(scene.validate().is_err());
    }
    #[test]
    fn checks_coordinates_ids_and_selection_references() {
        let mut scene = snapshot();
        scene.tiles.push(scene.tiles[0].clone());
        assert!(scene.validate().is_err());
        scene.tiles.pop();
        scene.selectable_tiles.push("1,0,-1".into());
        assert!(scene.validate().is_err());
        scene.selectable_tiles.clear();
        scene.tiles[0].s = 1;
        assert!(scene.validate().is_err());
    }
    #[test]
    fn event_names_match_browser_contract() {
        let event = SceneEvent::TileSelected {
            game_id: "game".into(),
            tile_id: "0,0,0".into(),
        };
        assert_eq!(
            serde_json::to_value(event).unwrap(),
            serde_json::json!({"type":"tile-selected","gameId":"game","tileId":"0,0,0"})
        );
    }
}

pub mod landscape;
pub mod structure;
