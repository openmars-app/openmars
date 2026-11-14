//! Shared renderer asset requirements. Catalog processing owns physical URLs;
//! both hosts use this one list for download, decoding and readiness.
use crate::view::AssetView;
use serde::Serialize;

#[derive(Clone, Copy, Debug, Serialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "kebab-case")]
pub enum AssetKind {
    Model,
    Image,
    Font,
}

#[derive(Clone, Copy, Debug, Serialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "camelCase")]
pub struct AssetRequirement {
    pub id: &'static str,
    pub extension: &'static str,
    pub kind: AssetKind,
    pub srgb: bool,
    pub repeat: bool,
    pub array_layers: u32,
    /// Zero chooses the largest adapter-supported prepared variant.
    pub pixels: u32,
    /// Keep encoded scene bytes in the bounded download cache.
    pub cache: bool,
    #[serde(skip)]
    #[cfg_attr(feature = "export-types", ts(skip))]
    scope: Scope,
}
#[derive(Clone, Copy, Debug)]
enum Scope {
    Board,
    Common,
    Phobos,
}
#[derive(Clone, Debug, Serialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "camelCase")]
pub struct AssetDemand {
    pub request: u32,
    pub assets: Vec<AssetRequirement>,
}
pub fn requirements(view: AssetView) -> Vec<AssetRequirement> {
    RENDERER_ASSETS
        .iter()
        .copied()
        .filter(|asset| match view {
            AssetView::Idle => false,
            AssetView::Menu => matches!(asset.scope, Scope::Common | Scope::Phobos),
            AssetView::Game => true,
        })
        .collect()
}
const fn planet(id: &'static str, scope: Scope, pixels: u32) -> AssetRequirement {
    AssetRequirement {
        id,
        extension: ".webp",
        kind: AssetKind::Image,
        srgb: true,
        repeat: false,
        array_layers: 0,
        pixels,
        cache: true,
        scope,
    }
}
const fn icon(id: &'static str) -> AssetRequirement {
    planet(id, Scope::Board, 256)
}
pub const RENDERER_ASSETS: &[AssetRequirement] = &[
    AssetRequirement {
        id: "fonts/prototype",
        extension: ".ttf",
        kind: AssetKind::Font,
        srgb: false,
        repeat: false,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    icon("icons/resources/steel"),
    icon("icons/resources/titanium"),
    icon("icons/resources/plant"),
    icon("icons/resources/megacredit"),
    icon("icons/resources/card"),
    icon("icons/resources/heat"),
    icon("icons/resources/power"),
    icon("icons/resources/microbe"),
    icon("icons/resources/animal"),
    icon("icons/resources/science"),
    icon("icons/resources/data"),
    icon("icons/resources/director"),
    icon("icons/parameters/temperature"),
    icon("icons/placements/ocean"),
    icon("icons/resources/wild"),
    AssetRequirement {
        id: "models/spaceship",
        extension: ".glb",
        kind: AssetKind::Model,
        srgb: false,
        repeat: false,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "models/satellite",
        extension: ".glb",
        kind: AssetKind::Model,
        srgb: false,
        repeat: false,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "models/asteroid",
        extension: ".glb",
        kind: AssetKind::Model,
        srgb: false,
        repeat: false,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/effects/smoke",
        extension: ".png",
        kind: AssetKind::Image,
        srgb: false,
        repeat: false,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "models/fence",
        extension: ".glb",
        kind: AssetKind::Model,
        srgb: false,
        repeat: false,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/terrain/noise-high",
        extension: ".png",
        kind: AssetKind::Image,
        srgb: false,
        repeat: true,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "models/bird",
        extension: ".glb",
        kind: AssetKind::Model,
        srgb: false,
        repeat: false,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "models/rock",
        extension: ".glb",
        kind: AssetKind::Model,
        srgb: false,
        repeat: false,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/planets/mars/surface",
        extension: ".webp",
        kind: AssetKind::Image,
        srgb: true,
        repeat: false,
        scope: Scope::Common,
        pixels: 2048,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/skyboxes/starmap",
        extension: ".ktx2",
        kind: AssetKind::Image,
        srgb: false,
        repeat: false,
        scope: Scope::Common,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/skyboxes/starmap-diffuse",
        extension: ".ktx2",
        kind: AssetKind::Image,
        srgb: false,
        repeat: false,
        scope: Scope::Common,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/skyboxes/starmap-specular",
        extension: ".ktx2",
        kind: AssetKind::Image,
        srgb: false,
        repeat: false,
        scope: Scope::Common,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/terrain/color-array",
        extension: ".ktx2",
        kind: AssetKind::Image,
        srgb: true,
        repeat: true,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 11,
    },
    AssetRequirement {
        id: "textures/terrain/detail-array",
        extension: ".ktx2",
        kind: AssetKind::Image,
        srgb: false,
        repeat: true,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 9,
    },
    AssetRequirement {
        id: "models/trees",
        extension: ".glb",
        kind: AssetKind::Model,
        srgb: false,
        repeat: false,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "models/pines",
        extension: ".glb",
        kind: AssetKind::Model,
        srgb: false,
        repeat: false,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "models/flowers",
        extension: ".glb",
        kind: AssetKind::Model,
        srgb: false,
        repeat: false,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/terrain/rock",
        extension: ".jpg",
        kind: AssetKind::Image,
        srgb: true,
        repeat: true,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/terrain/concrete-tile-facade-color",
        extension: ".jpg",
        kind: AssetKind::Image,
        srgb: true,
        repeat: true,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/terrain/concrete-tile-facade-normal",
        extension: ".png",
        kind: AssetKind::Image,
        srgb: false,
        repeat: true,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/terrain/concrete-tile-facade-roughness",
        extension: ".png",
        kind: AssetKind::Image,
        srgb: false,
        repeat: true,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/terrain/rectangular-facade-tiles-color",
        extension: ".jpg",
        kind: AssetKind::Image,
        srgb: true,
        repeat: true,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/terrain/rectangular-facade-tiles-normal",
        extension: ".png",
        kind: AssetKind::Image,
        srgb: false,
        repeat: true,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/terrain/rectangular-facade-tiles-roughness",
        extension: ".png",
        kind: AssetKind::Image,
        srgb: false,
        repeat: true,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/terrain/ribbed-concrete-wall-color",
        extension: ".jpg",
        kind: AssetKind::Image,
        srgb: true,
        repeat: true,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/terrain/ribbed-concrete-wall-normal",
        extension: ".png",
        kind: AssetKind::Image,
        srgb: false,
        repeat: true,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/terrain/ribbed-concrete-wall-roughness",
        extension: ".png",
        kind: AssetKind::Image,
        srgb: false,
        repeat: true,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/terrain/concrete",
        extension: ".jpg",
        kind: AssetKind::Image,
        srgb: true,
        repeat: true,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/terrain/grass",
        extension: ".jpg",
        kind: AssetKind::Image,
        srgb: true,
        repeat: true,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/terrain/sand",
        extension: ".jpg",
        kind: AssetKind::Image,
        srgb: true,
        repeat: true,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "textures/terrain/noise-mid",
        extension: ".png",
        kind: AssetKind::Image,
        srgb: false,
        repeat: true,
        scope: Scope::Board,
        pixels: 0,
        cache: true,
        array_layers: 0,
    },
    AssetRequirement {
        id: "models/phobos",
        extension: ".glb",
        kind: AssetKind::Model,
        srgb: false,
        repeat: false,
        pixels: 0,
        cache: true,
        array_layers: 0,
        scope: Scope::Phobos,
    },
    planet("textures/planets/mercury/surface", Scope::Board, 0),
    planet("textures/planets/venus/surface", Scope::Board, 0),
    planet("textures/planets/earth/surface", Scope::Board, 0),
    planet("textures/planets/moon/surface", Scope::Board, 0),
    planet("textures/planets/ceres/surface", Scope::Board, 0),
    planet("textures/planets/jupiter/surface", Scope::Board, 0),
    planet("textures/planets/ganymede/surface", Scope::Board, 0),
    planet("textures/planets/saturn/surface", Scope::Board, 0),
    planet("textures/planets/uranus/surface", Scope::Board, 0),
    planet("textures/planets/neptune/surface", Scope::Board, 0),
    planet("textures/planets/earth-clouds/surface", Scope::Board, 0),
    planet("textures/planets/sun/surface", Scope::Board, 1024),
];

pub const MAX_ASSET_FILE_BYTES: u32 = 64 * 1024 * 1024;
pub const DISPLAY_SKY_FILE_BYTES: u32 = 128 * 1024 * 1024;
pub fn max_file_bytes(id: &str) -> u32 {
    if id == "textures/skyboxes/starmap" {
        DISPLAY_SKY_FILE_BYTES
    } else {
        MAX_ASSET_FILE_BYTES
    }
}
/// Bounded encoded-byte cache; large sky assets may evict earlier entries.
pub const SCENE_ASSET_CACHE_BYTES: u32 = 160 * 1024 * 1024;
const _: () = assert!(MAX_ASSET_FILE_BYTES <= SCENE_ASSET_CACHE_BYTES);
const _: () = assert!(DISPLAY_SKY_FILE_BYTES <= SCENE_ASSET_CACHE_BYTES);

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn view_requests_share_authored_resources_without_loading_the_board_for_menus() {
        assert!(requirements(AssetView::Idle).is_empty());
        let menu = requirements(AssetView::Menu);
        let game = requirements(AssetView::Game);
        assert_eq!(menu.len(), 5);
        assert_eq!(game.len(), 60);
        for id in ["models/spaceship", "models/satellite"] {
            assert!(game.iter().any(|asset| asset.id == id));
        }
        assert!(menu.iter().any(|r| r.id == "models/phobos"));
        assert!(!menu.iter().any(|r| r.array_layers > 0));
        assert_eq!(
            menu.iter()
                .filter(|a| game.iter().any(|b| a.id == b.id))
                .count(),
            5
        );
        let json = serde_json::to_value(AssetDemand {
            request: 7,
            assets: menu,
        })
        .unwrap();
        assert_eq!(json["request"], 7);
        assert!(json["assets"][0].get("scope").is_none());
    }
    #[test]
    fn game_prepares_every_destination_surface_before_travel() {
        let game = requirements(AssetView::Game);
        for name in [
            "mercury",
            "venus",
            "earth",
            "moon",
            "ceres",
            "jupiter",
            "ganymede",
            "saturn",
            "uranus",
            "neptune",
            "earth-clouds",
        ] {
            assert!(
                game.iter()
                    .any(|a| a.id == format!("textures/planets/{name}/surface"))
            );
        }
        assert!(!game.iter().any(|a| a.id.ends_with("/overview")));
        assert_eq!(requirements(AssetView::Menu).len(), 5);
    }
    #[test]
    fn requirements_match_prepared_catalog_array_layout_and_color_space() {
        let catalog: serde_json::Value =
            serde_json::from_str(include_str!("../../../assets/catalog.json")).unwrap();
        let profiles: serde_json::Value =
            serde_json::from_str(include_str!("../../../assets/profiles.json")).unwrap();
        let mut ids = std::collections::BTreeSet::new();
        for requirement in RENDERER_ASSETS {
            assert!(ids.insert(requirement.id));
            let entry = catalog["assets"]
                .as_array()
                .unwrap()
                .iter()
                .find(|entry| entry["id"] == requirement.id)
                .unwrap();
            if entry["profile"] == "copy" {
                assert!(
                    entry["source"]
                        .as_str()
                        .unwrap()
                        .ends_with(requirement.extension),
                    "{}",
                    requirement.id
                );
            }
            if requirement.array_layers > 0 {
                assert_eq!(
                    entry["layers"].as_array().unwrap().len(),
                    requirement.array_layers as usize
                );
                let profile = &profiles[entry["profile"].as_str().unwrap()]["textureArray"];
                assert_eq!(profile["srgb"], requirement.srgb);
                assert!(profile["size"].as_u64().unwrap() <= 1024);
            }
        }
    }
}
