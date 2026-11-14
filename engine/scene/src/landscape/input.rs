//! Adapt the Go DTO presentation vocabulary to continuous visual fields.
use super::*;
use crate::{
    SceneSnapshot,
    geometry::BodyProjection,
    structure::{StructureStyleRequest, layout},
};
use std::{fmt, sync::Arc};

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum LandscapeInputError {
    Snapshot(&'static str),
    Layout(layout::LayoutError),
    Field(crate::geometry::terrain::TerrainError),
}
impl fmt::Display for LandscapeInputError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{self:?}")
    }
}
impl std::error::Error for LandscapeInputError {}

/// FNV-1a over UTF-8, matching the former Three.js landscapeGeometry.hashSeed.
pub fn visual_seed(value: &str) -> u32 {
    value.as_bytes().iter().fold(2166136261_u32, |hash, byte| {
        (hash ^ u32::from(*byte)).wrapping_mul(16777619)
    })
}

impl SceneSnapshot {
    pub fn landscape_input(
        &self,
        relief: Option<Arc<crate::geometry::terrain::ReliefMap>>,
        revision: u64,
    ) -> Result<LandscapeInput, LandscapeInputError> {
        self.validate().map_err(LandscapeInputError::Snapshot)?;
        let mut sources = Vec::new();
        for tile in self
            .tiles
            .iter()
            .filter(|tile| tile.location == "mars" && tile.terrain != "empty")
        {
            let Some(occupant) = tile.occupant.as_deref() else {
                continue;
            };
            let request = tile
                .visual
                .as_ref()
                .and_then(|visual| visual.structure.as_ref());
            let fallback_seed = visual_seed(&format!("{}:{}", self.game_id, tile.id));
            let seed = tile
                .visual
                .as_ref()
                .map_or(fallback_seed, |visual| visual.seed);
            let kind = if request.is_some() || occupant == "city-tile" {
                LandscapeKind::Structure
            } else {
                match occupant {
                    "ocean-tile" => LandscapeKind::Ocean,
                    "greenery-tile" => LandscapeKind::Greenery,
                    "volcano-tile" => LandscapeKind::Volcano,
                    "ecological-zone-tile" | "natural-preserve-tile" => {
                        LandscapeKind::SpecialGreenery
                    }
                    _ => LandscapeKind::Excluded,
                }
            };
            let layout = if kind == LandscapeKind::Structure {
                let defaults = StructureStyleRequest::default();
                let name = tile
                    .occupant_display_name
                    .as_deref()
                    .or(tile.display_name.as_deref())
                    .unwrap_or("Structure");
                Some(
                    layout::generate(seed, request.unwrap_or(&defaults), name)
                        .map_err(LandscapeInputError::Layout)?,
                )
            } else {
                None
            };
            sources.push(LandscapeSource {
                coordinate: [tile.q, tile.r, tile.s],
                kind,
                seed: match kind {
                    LandscapeKind::Excluded => 0,
                    LandscapeKind::Ocean | LandscapeKind::Volcano => fallback_seed,
                    _ => seed,
                },
                layout,
            });
        }
        LandscapeInput::new(
            BodyId::Mars,
            BodyProjection::MARS,
            revision,
            visual_seed(&self.game_id),
            relief,
            sources,
        )
        .map_err(LandscapeInputError::Field)
    }
}
