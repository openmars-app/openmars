use openmars_scene::{
    SceneSnapshot, SceneTile, SceneTileVisual,
    landscape::{LandscapeKind, visual_seed},
    structure::{StructureLandmark, StructureStyleRequest},
};

fn tile(q: i32, occupant: &str) -> SceneTile {
    SceneTile {
        id: format!("{q},0,{}", -q),
        q,
        r: 0,
        s: -q,
        location: "mars".into(),
        terrain: "land".into(),
        display_name: None,
        tags: vec![],
        bonuses: vec![],
        occupant: Some(occupant.into()),
        occupant_display_name: None,
        occupant_tags: vec![],
        visual: None,
        owner_id: None,
        reserved_by: None,
    }
}
fn scene(tiles: Vec<SceneTile>) -> SceneSnapshot {
    SceneSnapshot {
        game_id: "game".into(),
        map_id: "tharsis".into(),
        generation: 1,
        temperature: -30,
        oxygen: 0,
        oceans: 0,
        max_oceans: 9,
        venus: 0,
        tiles,
        players: vec![],
        selected_tile: None,
        selectable_tiles: vec![],
        offworld: Default::default(),
    }
}

#[test]
fn actual_dto_occupants_select_only_their_visual_field_families() {
    let types = [
        "ocean-tile",
        "greenery-tile",
        "city-tile",
        "volcano-tile",
        "world-tree-tile",
        "mining-tile",
        "ecological-zone-tile",
        "natural-preserve-tile",
    ];
    let value = scene(
        types
            .into_iter()
            .enumerate()
            .map(|(q, kind)| tile(q as i32, kind))
            .collect(),
    );
    let input = value.landscape_input(None, 1).unwrap();
    assert_eq!(
        input.sources().iter().map(|s| s.kind).collect::<Vec<_>>(),
        [
            LandscapeKind::Ocean,
            LandscapeKind::Greenery,
            LandscapeKind::Structure,
            LandscapeKind::Volcano,
            LandscapeKind::Excluded,
            LandscapeKind::Excluded,
            LandscapeKind::SpecialGreenery,
            LandscapeKind::SpecialGreenery
        ]
    );
    assert_eq!(input.sources()[4].seed, 0);
    assert_eq!(input.sources()[5].seed, 0);
    let mut offworld = tile(0, "city-tile");
    offworld.location = "ganymede".into();
    assert!(
        scene(vec![offworld])
            .landscape_input(None, 1)
            .unwrap()
            .sources()
            .is_empty()
    );
}

#[test]
fn dto_seed_and_style_drive_geometry_while_display_names_do_not_choose_it() {
    let mut city = tile(0, "city-tile");
    city.occupant_display_name = Some("Capital".into());
    let plain = scene(vec![city.clone()]).landscape_input(None, 1).unwrap();
    assert_eq!(
        plain.sources()[0]
            .layout
            .as_ref()
            .unwrap()
            .layout()
            .style
            .landmark,
        StructureLandmark::None
    );
    city.visual = Some(SceneTileVisual {
        seed: u32::MAX,
        structure: Some(StructureStyleRequest {
            landmark: Some("hall".into()),
            ..Default::default()
        }),
    });
    let authored = scene(vec![city]).landscape_input(None, 2).unwrap();
    let source = &authored.sources()[0];
    assert_eq!(source.seed, u32::MAX);
    assert_eq!(source.layout.as_ref().unwrap().layout().seed, u32::MAX);
    assert_eq!(
        source.layout.as_ref().unwrap().layout().style.landmark,
        StructureLandmark::Hall
    );
}

#[test]
fn visual_hash_and_unrelated_snapshot_updates_keep_landscape_identity() {
    assert_eq!(visual_seed("hello"), 0x4f9f2cab);
    let mut value = scene(vec![tile(0, "ocean-tile"), tile(1, "greenery-tile")]);
    let before = value.clone();
    let a = value.landscape_input(None, 1).unwrap();
    assert_eq!(value, before);
    value.generation += 1;
    value.temperature = 8;
    value.max_oceans = 14;
    value.selected_tile = Some("0,0,0".into());
    value.tiles.reverse();
    let b = value.landscape_input(None, 2).unwrap();
    assert_eq!(a.sources(), b.sources());
    assert_eq!(a.seed, b.seed);
}
