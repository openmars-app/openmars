//! Public visual inputs for ships and the authored orbital-station model.
use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct OffworldScene {
    pub fleets_enabled: bool,
    pub colonies: Vec<SceneColony>,
    pub fleets: Vec<SceneFleet>,
    pub station: Option<SceneStation>,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SceneColony {
    pub id: String,
    pub location: String,
    pub traded_this_gen: bool,
    pub trader_id: Option<String>,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SceneFleet {
    pub player_id: String,
    pub total: u32,
    pub available: u32,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SceneStation {
    pub name: String,
    pub filled_seats: u32,
    pub total_seats: u32,
    pub completed: bool,
}

impl OffworldScene {
    /// Renderer allocation/identity bounds; gameplay rules remain in Go.
    pub fn validate(&self, players: &[crate::ScenePlayer]) -> Result<(), &'static str> {
        if self.colonies.len() > 128 || self.fleets.len() > 64 {
            return Err("Offworld scene exceeds renderer bounds");
        }
        let bounded = |s: &str| !s.is_empty() && s.len() <= 128;
        let mut colonies = std::collections::BTreeSet::new();
        for colony in &self.colonies {
            if !bounded(&colony.id)
                || !bounded(&colony.location)
                || colony.trader_id.as_ref().is_some_and(|s| !bounded(s))
                || !colonies.insert(&colony.id)
            {
                return Err("Invalid offworld colony identity");
            }
        }
        let mut fleets = std::collections::BTreeSet::new();
        let mut total = 0u32;
        for fleet in &self.fleets {
            if !bounded(&fleet.player_id)
                || !players.iter().any(|p| p.id == fleet.player_id)
                || !fleets.insert(&fleet.player_id)
                || fleet.total > 64
                || fleet.available > fleet.total
            {
                return Err("Invalid offworld fleet presentation");
            }
            total += fleet.total;
        }
        if total > 1024
            || self.station.as_ref().is_some_and(|s| {
                !bounded(&s.name) || s.total_seats > 256 || s.filled_seats > s.total_seats
            })
        {
            return Err("Offworld scene exceeds renderer bounds");
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    fn scene() -> OffworldScene {
        OffworldScene {
            fleets_enabled: true,
            colonies: vec![SceneColony {
                id: "titan".into(),
                location: "venus".into(),
                traded_this_gen: true,
                trader_id: Some("alice".into()),
            }],
            fleets: vec![SceneFleet {
                player_id: "alice".into(),
                total: 3,
                available: 2,
            }],
            station: Some(SceneStation {
                name: "Orbital station".into(),
                filled_seats: 2,
                total_seats: 6,
                completed: false,
            }),
        }
    }
    #[test]
    fn explicit_public_visual_fields_round_trip_without_inferred_destinations() {
        let scene = scene();
        let players = [crate::ScenePlayer {
            id: "alice".into(),
            color: "#ffaa00".into(),
        }];
        scene.validate(&players).unwrap();
        let mut json = serde_json::to_value(&scene).unwrap();
        assert_eq!(json["colonies"][0]["location"], "venus");
        assert_eq!(json["fleets"][0]["available"], 2);
        assert_eq!(
            serde_json::from_value::<OffworldScene>(json.clone()).unwrap(),
            scene
        );
        let mut missing = json.clone();
        missing.as_object_mut().unwrap().remove("fleetsEnabled");
        assert!(serde_json::from_value::<OffworldScene>(missing).is_err());
        json["colonies"][0]["privateTradeOptions"] = serde_json::json!([]);
        assert!(serde_json::from_value::<OffworldScene>(json).is_err());
    }
    #[test]
    fn rejects_duplicate_unknown_and_unbounded_allocations() {
        let players = [crate::ScenePlayer {
            id: "alice".into(),
            color: "#ffaa00".into(),
        }];
        let mut s = scene();
        s.fleets[0].available = 4;
        assert!(s.validate(&players).is_err());
        let mut s = scene();
        s.fleets[0].total = 65;
        assert!(s.validate(&players).is_err());
        let mut s = scene();
        s.fleets.push(s.fleets[0].clone());
        assert!(s.validate(&players).is_err());
        let mut s = scene();
        s.colonies.push(s.colonies[0].clone());
        assert!(s.validate(&players).is_err());
        assert!(scene().validate(&[]).is_err());
        let mut s = scene();
        s.station.as_mut().unwrap().filled_seats = 7;
        assert!(s.validate(&players).is_err());
    }
}
