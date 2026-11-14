//! Tile.tsx/TileGrid.tsx presentation rules, including compact mode exceptions.
use openmars_scene::SceneTile;

pub const FONT: &str = "fonts/prototype";
pub const FONT_SIZE: f64 = 0.045;
pub const OUTLINE: f64 = 0.004;
pub const MAX_WIDTH: f64 = 0.18;
pub const CREDIT_SIZE: f64 = 0.025;
pub const ICON_SIZE: f64 = 0.05;
pub const ICON_TINT: [f32; 3] = [0.7; 3];

#[derive(Clone, Debug, PartialEq)]
pub struct Icon {
    pub asset: &'static str,
    pub x: f64,
    pub credit: Option<i32>,
}
#[derive(Clone, Debug, Default, PartialEq)]
pub struct Content {
    pub name: Option<String>,
    pub icons: Vec<Icon>,
    pub icon_y: f64,
}
/// Identical aliases and wild fallback to the old texture owner. No game rules.
pub fn icon_asset(kind: &str) -> &'static str {
    match kind {
        "steel" => "icons/resources/steel",
        "titanium" => "icons/resources/titanium",
        "plants" | "plant" => "icons/resources/plant",
        "cards" | "card-draw" => "icons/resources/card",
        "credit" => "icons/resources/megacredit",
        "heat" => "icons/resources/heat",
        "energy" | "energy-production" => "icons/resources/power",
        "microbe" => "icons/resources/microbe",
        "animal" => "icons/resources/animal",
        "science" => "icons/resources/science",
        "data" => "icons/resources/data",
        "delegate" => "icons/resources/director",
        "temperature" => "icons/parameters/temperature",
        "ocean" | "ocean-placement" => "icons/placements/ocean",
        _ => "icons/resources/wild",
    }
}
/// Source getTileData synthesizes labels only for its generic special branch.
/// Placed displayName describes the occupant in inspections, not the board label.
pub fn display_name(tile: &SceneTile) -> Option<&str> {
    match tile.occupant.as_deref() {
        Some("commercial-district-tile") => Some("Commercial District"),
        Some(kind)
            if !matches!(
                kind,
                "ocean-tile"
                    | "city-tile"
                    | "greenery-tile"
                    | "volcano-tile"
                    | "nuclear-zone-tile"
                    | "ecological-zone-tile"
                    | "natural-preserve-tile"
                    | "industrial-center-tile"
                    | "mining-tile"
                    | "restricted-tile"
                    | "world-tree-tile"
                    | "mohole-tile"
            ) =>
        {
            Some(kind)
        }
        _ => tile.display_name.as_deref(),
    }
}
impl Content {
    /// The renderer supplies actual structure/impact ownership, including outgoing
    /// nuclear visuals; metadata alone must not guess whether a mesh is shown.
    pub fn new(tile: &SceneTile, has_structure: bool, compact: bool, impacted: bool) -> Self {
        if impacted {
            return Self::default();
        }
        let name = if compact || (has_structure && tile.occupant.as_deref() != Some("city-tile")) {
            None
        } else {
            display_name(tile)
                .filter(|s| !s.is_empty())
                .map(str::to_owned)
        };
        let mut groups: Vec<(&str, i32)> = Vec::new();
        if !has_structure
            && !matches!(
                tile.occupant.as_deref(),
                Some(
                    "greenery-tile"
                        | "ecological-zone-tile"
                        | "natural-preserve-tile"
                        | "world-tree-tile"
                )
            )
        {
            // convertBackendBonuses overwrites duplicates without changing their
            // first insertion order; sorting these groups changes the old layout.
            for bonus in &tile.bonuses {
                if let Some((_, value)) = groups.iter_mut().find(|(kind, _)| *kind == bonus.r#type)
                {
                    *value = bonus.amount;
                } else {
                    groups.push((&bonus.r#type, bonus.amount));
                }
            }
        }
        let count = |kind: &str, amount: i32| if kind == "credit" { 1 } else { amount };
        let width: f64 = groups
            .iter()
            .enumerate()
            .map(|(i, (kind, amount))| {
                let n = count(kind, *amount);
                f64::from(n) * ICON_SIZE
                    + f64::from((n - 1).max(0)) * 0.005
                    + if i > 0 { 0.01 } else { 0. }
            })
            .sum();
        let mut x = -width / 2.;
        let mut icons = Vec::new();
        for (i, (kind, amount)) in groups.into_iter().enumerate() {
            if i > 0 {
                x += 0.01;
            }
            for j in 0..count(kind, amount) {
                if j > 0 {
                    x += 0.005;
                }
                icons.push(Icon {
                    asset: icon_asset(kind),
                    x: x + ICON_SIZE / 2.,
                    credit: (kind == "credit" && !compact).then_some(amount),
                });
                x += ICON_SIZE;
            }
        }
        let icon_y = if name.is_some() { -0.08 } else { 0. };
        Self {
            name,
            icons,
            icon_y,
        }
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    fn tile() -> SceneTile {
        serde_json::from_value(serde_json::json!({"id":"0,0,0","q":0,"r":0,"s":0,"location":"mars","terrain":"land","displayName":"Olympus Mons","tags":[],"bonuses":[{"type":"plant","amount":2},{"type":"credit","amount":3},{"type":"steel","amount":1}],"occupant":null,"occupantDisplayName":"A different placed name","occupantTags":[],"visual":null,"ownerId":null,"reservedBy":null})).unwrap()
    }
    #[test]
    fn original_names_order_spacing_and_compact_credit_exception() {
        let t = tile();
        let c = Content::new(&t, false, false, false);
        assert_eq!(c.name.as_deref(), Some("Olympus Mons"));
        assert_eq!(c.icon_y, -0.08);
        assert_eq!(
            c.icons.iter().map(|i| i.asset).collect::<Vec<_>>(),
            [
                "icons/resources/plant",
                "icons/resources/plant",
                "icons/resources/megacredit",
                "icons/resources/steel"
            ]
        );
        for (icon, x) in c.icons.iter().zip([-0.0875, -0.0325, 0.0275, 0.0875]) {
            assert!((icon.x - x).abs() < 1e-14);
        }
        assert_eq!(c.icons[2].credit, Some(3));
        let c = Content::new(&t, false, true, false);
        assert_eq!(c.name, None);
        assert_eq!(c.icon_y, 0.);
        assert_eq!(c.icons.len(), 4);
        assert!(c.icons.iter().all(|i| i.credit.is_none()));
    }
    #[test]
    fn actual_structures_hide_bonuses_and_only_city_keeps_its_board_name() {
        let mut t = tile();
        t.occupant = Some("city-tile".into());
        let city = Content::new(&t, true, false, false);
        assert_eq!(city.name.as_deref(), Some("Olympus Mons"));
        assert!(city.icons.is_empty());
        t.occupant = Some("commercial-district-tile".into());
        assert_eq!(Content::new(&t, true, false, false), Content::default());
        assert_eq!(
            Content::new(&t, false, false, false).name.as_deref(),
            Some("Commercial District")
        );
        t.occupant = Some("industrial-center-tile".into());
        assert_eq!(display_name(&t), Some("Olympus Mons"));
        t.occupant = Some("unknown-special-tile".into());
        assert_eq!(display_name(&t), Some("unknown-special-tile"));
        for kind in [
            "greenery-tile",
            "ecological-zone-tile",
            "natural-preserve-tile",
            "world-tree-tile",
        ] {
            t.occupant = Some(kind.into());
            assert!(Content::new(&t, false, false, false).icons.is_empty());
        }
        assert_eq!(Content::new(&t, false, false, true), Content::default());
    }
    #[test]
    fn duplicate_bonus_overwrites_amount_without_reordering_and_unknown_uses_wild() {
        let mut t = tile();
        t.bonuses.push(openmars_scene::SceneTileBonus {
            r#type: "plant".into(),
            amount: 1,
        });
        t.bonuses.push(openmars_scene::SceneTileBonus {
            r#type: "unknown".into(),
            amount: 1,
        });
        let c = Content::new(&t, false, false, false);
        assert_eq!(c.icons.len(), 4);
        assert_eq!(c.icons[0].asset, "icons/resources/plant");
        assert_eq!(c.icons[1].credit, Some(3));
        assert_eq!(c.icons[3].asset, "icons/resources/wild");
        for (a, b) in [
            ("plants", "plant"),
            ("cards", "card-draw"),
            ("energy", "energy-production"),
            ("ocean", "ocean-placement"),
        ] {
            assert_eq!(icon_asset(a), icon_asset(b));
        }
    }
}
