//! Stable original landscape plant cells. Placement reads the same complete
//! terrain sampler as rendered ground; climate gates never reroll candidates.
use super::{
    LandscapeKind, LandscapeSampler, TILE_RADIUS,
    fields::{fractal, hex_distance},
    smooth,
};
use crate::{
    geometry::{
        AxialPoint, BoardPoint,
        placement::{GreeneryKind, greenery_cell, greenery_rank, kept_at_foliage_density},
        terrain::{FIELD_STEP, PATCH_SIZE, TerrainError},
    },
    structure::StructureLandmark,
};
use std::sync::Arc;

pub const MAX_PLANTS_PER_PATCH: usize = 4096;
const MAX_ROCKS: usize = 64;
const MAX_CHECKS: usize = 16384;
const PASSES: [(GreeneryKind, f64, bool); 8] = [
    (GreeneryKind::Rock, 0.095, false),
    (GreeneryKind::Tree, 0.016, false),
    (GreeneryKind::Pine, 0.014, false),
    (GreeneryKind::Bush, 0.014, false),
    (GreeneryKind::Clover, 0.031, false),
    (GreeneryKind::Flower, 0.046, false),
    (GreeneryKind::Tree, 0.01, true),
    (GreeneryKind::Bush, 0.012, true),
];
#[derive(
    Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord, Hash, serde::Serialize, serde::Deserialize,
)]
pub struct PlantId {
    pub kind: GreeneryKind,
    pub cell: [i32; 2],
    pub capital_only: bool,
}
#[derive(Clone, Copy, Debug, PartialEq, serde::Serialize, serde::Deserialize)]
pub struct LandscapePlant {
    pub id: PlantId,
    pub tile: [i32; 3],
    pub point: BoardPoint,
    pub height: f64,
    pub seed: u32,
    pub scale: f64,
    pub rank: f64,
}
impl LandscapePlant {
    pub fn visible(&self, gate: f64, density: f64) -> Result<bool, TerrainError> {
        if !gate.is_finite() || !(0.0..=1.0).contains(&gate) {
            return Err(TerrainError::OutsideDomain);
        }
        let kept =
            kept_at_foliage_density(self.seed, density).map_err(|_| TerrainError::OutsideDomain)?;
        Ok(self.rank <= gate && kept)
    }
    pub fn variant(&self) -> usize {
        self.seed as usize
            % match self.id.kind {
                GreeneryKind::Tree => 4,
                GreeneryKind::Pine => 1,
                GreeneryKind::Bush | GreeneryKind::Clover => 5,
                GreeneryKind::Flower => 3,
                GreeneryKind::Rock => 1,
            }
    }
}
/// Cooperative cursor only. The builder reserves the complete output capacity
/// before its buffer exists, then moves that buffer into the immutable patch.
pub struct PlantScatter {
    sampler: Arc<LandscapeSampler>,
    origin: BoardPoint,
    capital: bool,
    pass: usize,
    cell: usize,
    checked: usize,
    rocks: [(BoardPoint, f64); MAX_ROCKS],
    rock_count: usize,
}
impl PlantScatter {
    pub fn new(sampler: Arc<LandscapeSampler>, key: [i32; 2]) -> Result<Self, TerrainError> {
        let origin = BoardPoint {
            x: f64::from(key[0]) * PATCH_SIZE,
            y: f64::from(key[1]) * PATCH_SIZE,
        };
        if origin.x.abs() > 1e6 || origin.y.abs() > 1e6 {
            return Err(TerrainError::OutsideDomain);
        }
        let reach = PATCH_SIZE * 1.1 + FIELD_STEP * 2.;
        let capital = sampler.input().sources().iter().any(|source| {
            let center = sampler.input().projection.hex_center(AxialPoint {
                q: source.coordinate[0],
                r: source.coordinate[1],
            });
            source.kind == LandscapeKind::Structure
                && source
                    .layout
                    .as_ref()
                    .is_some_and(|l| l.layout().style.landmark == StructureLandmark::Hall)
                && center.x >= origin.x - reach
                && center.x <= origin.x + PATCH_SIZE + reach
                && center.y >= origin.y - reach
                && center.y <= origin.y + PATCH_SIZE + reach
        });
        Ok(Self {
            sampler,
            origin,
            capital,
            pass: 0,
            cell: 0,
            checked: 0,
            rocks: [(BoardPoint { x: 0., y: 0. }, 0.); MAX_ROCKS],
            rock_count: 0,
        })
    }
    pub fn complete(&self) -> bool {
        self.pass == PASSES.len()
    }
    /// Counts attempted lattice cells, including exclusions, against the same
    /// finite per-poll budget as ground preparation. No hidden unbounded sweep.
    pub fn step(
        &mut self,
        plants: &mut Vec<LandscapePlant>,
        budget: usize,
    ) -> Result<usize, TerrainError> {
        if budget == 0 || budget > 1024 || plants.len() > MAX_PLANTS_PER_PATCH {
            return Err(TerrainError::ResourceLimit);
        }
        let mut spent = 0;
        while self.pass < PASSES.len() && spent < budget {
            let (kind, spacing, capital_only) = PASSES[self.pass];
            if capital_only && !self.capital {
                self.pass += 1;
                self.cell = 0;
                continue;
            }
            let low = [
                (self.origin.x / spacing).floor() as i32,
                (self.origin.y / spacing).floor() as i32,
            ];
            let high = [
                ((self.origin.x + PATCH_SIZE) / spacing).ceil() as i32,
                ((self.origin.y + PATCH_SIZE) / spacing).ceil() as i32,
            ];
            let columns = (high[1] - low[1] + 1) as usize;
            let count = (high[0] - low[0] + 1) as usize * columns;
            if count > 4096 {
                return Err(TerrainError::ResourceLimit);
            }
            if self.cell == count {
                self.pass += 1;
                self.cell = 0;
                continue;
            }
            let ix = low[0] + (self.cell / columns) as i32;
            let iy = low[1] + (self.cell % columns) as i32;
            self.cell += 1;
            self.checked += 1;
            spent += 1;
            if self.checked > MAX_CHECKS {
                return Err(TerrainError::ResourceLimit);
            }
            let seed = self.sampler.input().seed;
            let cell = greenery_cell(seed, kind, ix, iy, spacing)
                .map_err(|_| TerrainError::OutsideDomain)?;
            let point = cell.point;
            if point.x < self.origin.x
                || point.x >= self.origin.x + PATCH_SIZE
                || point.y < self.origin.y
                || point.y >= self.origin.y + PATCH_SIZE
            {
                continue;
            }
            let sample = self.sampler.sample(point, true)?;
            let Some(tile) = sample.source else {
                continue;
            };
            if sample.blocked
                || self.rocks[..self.rock_count]
                    .iter()
                    .any(|(p, r)| super::planar_length(point.x - p.x, point.y - p.y) < r + 0.008)
            {
                continue;
            }
            let source = self
                .sampler
                .input()
                .sources()
                .iter()
                .find(|s| s.coordinate == tile)
                .ok_or(TerrainError::OutsideDomain)?;
            let structure = source.kind == LandscapeKind::Structure;
            let capital = structure
                && source
                    .layout
                    .as_ref()
                    .is_some_and(|l| l.layout().style.landmark == StructureLandmark::Hall);
            if matches!(kind, GreeneryKind::Tree | GreeneryKind::Bush) && capital != capital_only {
                continue;
            }
            let center = self.sampler.input().projection.hex_center(AxialPoint {
                q: tile[0],
                r: tile[1],
            });
            let edge = hex_distance(point.x - center.x, point.y - center.y, TILE_RADIUS);
            let threshold = match kind {
                GreeneryKind::Tree => {
                    (if capital {
                        sample.coverage * 0.95
                    } else {
                        sample.forest * 0.665
                    }) * if structure {
                        1.
                    } else {
                        1. - 0.92 * smooth(-0.025, 0.006, edge)
                    }
                }
                GreeneryKind::Pine if !structure && source.kind != LandscapeKind::Volcano => {
                    sample.coverage
                        * 0.8
                        * (0.1
                            + 0.9
                                * smooth(
                                    0.38,
                                    0.64,
                                    fractal(point.x * 11., point.y * 11., seed.wrapping_add(41)),
                                ))
                        * (1. - smooth(-0.03, 0., edge))
                }
                GreeneryKind::Bush => {
                    sample.coverage
                        * if capital {
                            0.9
                        } else {
                            0.12 + sample.forest * 0.18
                        }
                }
                GreeneryKind::Clover => sample.lush * 0.45,
                GreeneryKind::Flower if source.kind == LandscapeKind::SpecialGreenery => {
                    sample.coverage * 0.5
                }
                GreeneryKind::Rock if !structure && source.kind != LandscapeKind::Volcano => {
                    sample.coverage * 0.11
                }
                _ => 0.,
            };
            let Some(rank) = greenery_rank(cell.selector, threshold, structure, kind)
                .map_err(|_| TerrainError::OutsideDomain)?
            else {
                continue;
            };
            let mut scale = if structure {
                0.27 + cell.jitter * 0.12
            } else {
                0.75 + cell.jitter * 0.35
            };
            if !structure && kind == GreeneryKind::Tree {
                scale *= 0.6;
            }
            if kind == GreeneryKind::Pine {
                scale *= 0.7;
            }
            if !structure && kind == GreeneryKind::Bush {
                scale *= 0.75;
            }
            if plants.len() == MAX_PLANTS_PER_PATCH {
                return Err(TerrainError::ResourceLimit);
            }
            plants.push(LandscapePlant {
                id: PlantId {
                    kind,
                    cell: [ix, iy],
                    capital_only,
                },
                tile,
                point,
                height: sample.height,
                seed: cell.seed,
                scale,
                rank,
            });
            if kind == GreeneryKind::Rock {
                if self.rock_count == MAX_ROCKS {
                    return Err(TerrainError::ResourceLimit);
                }
                self.rocks[self.rock_count] = (point, 0.025 * scale);
                self.rock_count += 1;
            }
        }
        Ok(spent)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{
        geometry::BodyProjection,
        landscape::{BodyId, LandscapeInput, LandscapeSource},
    };
    use std::collections::BTreeSet;
    fn sampler() -> Arc<LandscapeSampler> {
        Arc::new(LandscapeSampler::new(
            LandscapeInput::new(
                BodyId::Mars,
                BodyProjection::MARS,
                1,
                42,
                None,
                vec![
                    LandscapeSource {
                        coordinate: [0, 0, 0],
                        kind: LandscapeKind::SpecialGreenery,
                        seed: 114,
                        layout: None,
                    },
                    LandscapeSource {
                        coordinate: [1, 0, -1],
                        kind: LandscapeKind::Ocean,
                        seed: 115,
                        layout: None,
                    },
                ],
            )
            .unwrap(),
        ))
    }
    fn scatter(
        sampler: Arc<LandscapeSampler>,
        key: [i32; 2],
        budget: usize,
    ) -> Vec<LandscapePlant> {
        let mut cursor = PlantScatter::new(sampler, key).unwrap();
        let mut plants = Vec::new();
        let mut checks = 0;
        while !cursor.complete() {
            let spent = cursor.step(&mut plants, budget).unwrap();
            assert!(spent <= budget);
            checks += spent;
            assert!(checks <= MAX_CHECKS);
        }
        plants
    }
    #[test]
    fn frame_partition_does_not_change_any_original_cell_or_plant_fact() {
        let s = sampler();
        let a = scatter(s.clone(), [0, 0], 1);
        assert!(!a.is_empty());
        assert_eq!(a, scatter(s.clone(), [0, 0], 1024));
        for plant in &a {
            let f = s.sample(plant.point, true).unwrap();
            assert!(!f.blocked);
            assert_eq!(plant.tile, f.source.unwrap());
            assert_eq!(plant.height, f.height);
            assert!(
                plant.scale > 0. && plant.rank.is_finite() && (0.0..=1.0).contains(&plant.rank)
            );
            if plant.id.kind == GreeneryKind::Rock {
                assert_eq!(plant.rank, 0.);
            }
        }
    }
    #[test]
    fn neighbouring_patches_never_duplicate_cells_and_climate_reveals_stable_ranks() {
        let s = sampler();
        let mut ids = BTreeSet::new();
        let mut all = Vec::new();
        for key in [[-1, -1], [-1, 0], [0, -1], [0, 0]] {
            let plants = scatter(s.clone(), key, 113);
            for p in plants {
                assert!(ids.insert(p.id));
                assert!(
                    p.point.x >= f64::from(key[0]) * PATCH_SIZE
                        && p.point.x < (f64::from(key[0]) + 1.) * PATCH_SIZE
                );
                assert!(
                    p.point.y >= f64::from(key[1]) * PATCH_SIZE
                        && p.point.y < (f64::from(key[1]) + 1.) * PATCH_SIZE
                );
                assert!(p.visible(1., 1.).unwrap());
                assert!(!p.visible(1., 0.).unwrap());
                if p.visible(0.3, 0.7).unwrap() {
                    assert!(p.visible(0.8, 0.7).unwrap());
                }
                assert!(p.visible(f64::NAN, 1.).is_err());
                all.push(p);
            }
        }
        let rocks = all
            .iter()
            .filter(|p| p.id.kind == GreeneryKind::Rock)
            .collect::<Vec<_>>();
        assert!(!rocks.is_empty());
        for p in all.iter().filter(|p| p.id.kind != GreeneryKind::Rock) {
            // Original exclusion is patch-local; a rock belonging to another
            // patch does not silently change this patch's deterministic stream.
            for r in &rocks {
                let same = (p.point.x / PATCH_SIZE).floor() == (r.point.x / PATCH_SIZE).floor()
                    && (p.point.y / PATCH_SIZE).floor() == (r.point.y / PATCH_SIZE).floor();
                if same {
                    assert!(
                        (p.point.x - r.point.x).hypot(p.point.y - r.point.y)
                            >= 0.025 * r.scale + 0.008
                    );
                }
            }
        }
    }
    #[test]
    fn invalid_domain_and_unbounded_poll_are_errors_without_partial_plant_output() {
        let mut cursor = PlantScatter::new(sampler(), [0, 0]).unwrap();
        let mut plants = Vec::new();
        assert!(cursor.step(&mut plants, 0).is_err());
        assert!(cursor.step(&mut plants, 1025).is_err());
        assert!(plants.is_empty());
        assert!(PlantScatter::new(sampler(), [i32::MAX, 0]).is_err());
    }
}
