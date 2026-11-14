//! Captions retain the outgoing board name during a live missile flight. Impact
//! timing belongs to the shared visual timeline, never to replicated game rules.
use super::content::{Content, display_name};
use crate::nuclear::timeline::Timeline;
use openmars_scene::SceneTile;
use std::collections::BTreeMap;

#[derive(Default, PartialEq)]
struct Caption {
    name: Option<String>,
    impacted: bool,
}
#[derive(Default)]
pub struct Captions(BTreeMap<String, Caption>);
impl Captions {
    pub fn reconcile(
        &mut self,
        tiles: &[SceneTile],
        previous: &[SceneTile],
        timeline: &Timeline,
        step: u64,
        reduced: bool,
    ) -> bool {
        let mut next = BTreeMap::new();
        for tile in tiles.iter().filter(|t| {
            crate::world::is_mars(t)
                && t.occupant.as_deref() == Some("nuclear-zone-tile")
                && timeline.live((t.q, t.r, t.s))
        }) {
            let name = self.0.get(&tile.id).map_or_else(
                || {
                    previous
                        .iter()
                        .find(|old| old.id == tile.id)
                        .and_then(display_name)
                        .map(str::to_owned)
                },
                |caption| caption.name.clone(),
            );
            let impacted = timeline
                .start((tile.q, tile.r, tile.s))
                .is_some_and(|start| reduced || step.saturating_sub(start.step) >= 60);
            next.insert(tile.id.clone(), Caption { name, impacted });
        }
        let changed = next != self.0;
        self.0 = next;
        changed
    }
    pub fn content(&self, tile: &SceneTile, compact: bool) -> Content {
        let transition = self.0.get(&tile.id);
        let has_structure = transition.is_none()
            && (tile.occupant.as_deref() == Some("city-tile")
                || tile.visual.as_ref().is_some_and(|v| v.structure.is_some()));
        let mut content = Content::new(
            tile,
            has_structure,
            compact,
            transition.is_some_and(|c| c.impacted),
        );
        if let Some(caption) = transition.filter(|c| !c.impacted && !compact) {
            content.name = caption.name.clone().filter(|s| !s.is_empty());
            content.icon_y = if content.name.is_some() { -0.08 } else { 0. };
        }
        content
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::nuclear::timeline::Start;
    #[test]
    fn flight_retains_outgoing_name_with_current_bonuses_then_impact_hides_everything() {
        let old: SceneTile = serde_json::from_value(serde_json::json!({
            "id":"0,0,0","q":0,"r":0,"s":0,"location":"mars","terrain":"land",
            "displayName":"Arsia Mons","occupant":"commercial-district-tile",
            "tags":[],"bonuses":[],"occupantTags":[]
        }))
        .unwrap();
        let mut tile = old.clone();
        tile.occupant = Some("nuclear-zone-tile".into());
        tile.bonuses =
            serde_json::from_value(serde_json::json!([{"type":"credit","amount":7}])).unwrap();
        let keys = [(0, 0, 0)].into();
        let mut timeline = Timeline::default();
        timeline.observe(keys, false);
        let mut captions = Captions::default();
        assert!(captions.reconcile(&[tile.clone()], &[old], &timeline, 900, false));
        let content = captions.content(&tile, false);
        assert_eq!(content.name.as_deref(), Some("Commercial District"));
        assert_eq!(content.icons[0].credit, Some(7));
        assert_eq!(content.icon_y, -0.08);
        assert_eq!(captions.content(&tile, true).name, None);
        assert_eq!(captions.content(&tile, true).icons[0].credit, None);
        timeline.advance(
            &[(0, 0, 0)].into(),
            Start {
                step: 900,
                motion_step: 20,
            },
            false,
        );
        assert!(!captions.reconcile(&[tile.clone()], &[tile.clone()], &timeline, 959, false));
        assert!(captions.reconcile(&[tile.clone()], &[tile.clone()], &timeline, 960, false));
        assert_eq!(captions.content(&tile, false), Content::default());
        timeline.observe([(0, 0, 0)].into(), true);
        assert!(captions.reconcile(&[tile.clone()], &[], &timeline, 961, false));
        assert_eq!(
            captions.content(&tile, false).name.as_deref(),
            Some("Arsia Mons")
        );
        timeline.observe([(0, 0, 0)].into(), false);
        assert!(!captions.reconcile(&[tile.clone()], &[], &timeline, 999, false));
        assert_eq!(captions.content(&tile, false).icons.len(), 1);
    }
}
