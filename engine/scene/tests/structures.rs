use openmars_scene::structure::layout::generate;

#[test]
fn all_authored_go_card_styles_generate_bounded_layouts() {
    let cards: serde_json::Value =
        serde_json::from_slice(include_bytes!("../../../backend/assets/cards.json")).unwrap();
    let mut count = 0;
    for card in cards.as_array().unwrap() {
        if let Some(request) = card.get("style").and_then(|s| s.get("tile")) {
            let request = serde_json::from_value(request.clone()).unwrap();
            let name = card["name"].as_str().unwrap();
            let layout = generate(0x91b751, &request, name)
                .unwrap_or_else(|error| panic!("{name}: {error:?}"));
            layout.layout().validate().unwrap();
            assert!(!layout.layout().roads.is_empty(), "{name}");
            assert!(!layout.layout().buildings.is_empty(), "{name}");
            count += 1;
        }
    }
    assert!(count >= 12);
}
