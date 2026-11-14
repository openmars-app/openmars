use super::{
    footprint::*,
    support::{Bounds, Connection, WaterTriangle},
    *,
};
use crate::{
    geometry::{
        AxialPoint, BoardPoint, BodyProjection, landscape_noise,
        terrain::{PATCH_SIZE, ReliefMap, TerrainError, basin_height},
    },
    structure::{StructureGround, StructureLandmark, StructureLandscaping, StructureLayoutRef},
};
use serde::{Deserialize, Serialize};
use std::{collections::BTreeSet, sync::Arc};

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub enum LandscapeKind {
    Structure,
    Greenery,
    SpecialGreenery,
    Ocean,
    Volcano,
    Excluded,
}
impl LandscapeKind {
    fn green(self) -> bool {
        matches!(self, Self::Greenery | Self::SpecialGreenery | Self::Volcano)
    }
}
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct LandscapeSource {
    pub coordinate: [i32; 3],
    pub kind: LandscapeKind,
    pub seed: u32,
    pub layout: Option<StructureLayoutRef>,
}
#[derive(Clone)]
pub struct LandscapeInput {
    pub body: BodyId,
    pub projection: BodyProjection,
    pub revision: u64,
    pub seed: u32,
    pub relief: Option<Arc<ReliefMap>>,
    pub(super) sources: Vec<LandscapeSource>,
}
impl LandscapeInput {
    pub fn new(
        body: BodyId,
        projection: BodyProjection,
        revision: u64,
        seed: u32,
        relief: Option<Arc<ReliefMap>>,
        mut sources: Vec<LandscapeSource>,
    ) -> Result<Self, TerrainError> {
        if sources.len() > MAX_SOURCES {
            return Err(TerrainError::ResourceLimit);
        }
        let mut coordinates = BTreeSet::new();
        for source in &sources {
            let point = AxialPoint::from_cube(source.coordinate)
                .map_err(|_| TerrainError::OutsideDomain)?;
            let center = projection.hex_center(point);
            if center.x.abs() > 1e6 || center.y.abs() > 1e6 {
                return Err(TerrainError::OutsideDomain);
            }
            // All actual board extents fit comfortably inside this finite field domain.
            // This also bounds key arithmetic and deterministic noise lattice conversion.
            if source.coordinate.iter().any(|v| v.unsigned_abs() > 4096) {
                return Err(TerrainError::OutsideDomain);
            }
            if !coordinates.insert(point) {
                return Err(TerrainError::DuplicatePatch);
            }
            if let Some(layout) = &source.layout {
                layout
                    .layout()
                    .validate()
                    .map_err(|_| TerrainError::OutsideDomain)?;
            }
            if source.layout.is_some() && source.kind != LandscapeKind::Structure {
                return Err(TerrainError::OutsideDomain);
            }
        }
        // Preserve reference lexical key order, independent of incoming server order.
        sources.sort_by_key(|s| {
            format!(
                "{},{},{}",
                s.coordinate[0], s.coordinate[1], s.coordinate[2]
            )
        });
        Ok(Self {
            body,
            projection,
            revision,
            seed,
            relief,
            sources,
        })
    }
    pub fn sources(&self) -> &[LandscapeSource] {
        &self.sources
    }
}
#[derive(Clone)]
struct Source {
    data: LandscapeSource,
    point: BoardPoint,
}
impl Source {
    fn layout(&self) -> Option<&crate::structure::StructureLayout> {
        self.data.layout.as_ref().map(|l| l.layout())
    }
    fn capital(&self) -> bool {
        self.layout()
            .is_some_and(|l| l.style.landmark == StructureLandmark::Hall)
    }
    fn circular(&self) -> bool {
        self.layout().is_some_and(|l| wall_apothem(l).is_none())
    }
}
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct LandscapeSample {
    pub relief: f64,
    pub height: f64,
    pub shore: f64,
    pub coverage: f64,
    pub forest: f64,
    pub foundation: f64,
    pub moisture: f64,
    pub dry: f64,
    pub lush: f64,
    pub litter: f64,
    pub mud: f64,
    pub meadow: f64,
    pub mound: f64,
    pub greenery: f64,
    pub source: Option<[i32; 3]>,
    pub blocked: bool,
}
/// Map-invariant noise and relief. Reusing these retains the exact f64 evaluation,
/// rather than introducing another sampled/interpolated noise field.
#[derive(Clone, Copy, Debug)]
pub(super) struct SampleBasis {
    broad: f64,
    wx: f64,
    wy: f64,
    soil: f64,
    fine: f64,
    ox: f64,
    oy: f64,
    mound: f64,
    pub(super) relief: f64,
}
#[derive(Clone)]
pub struct LandscapeSampler {
    input: Arc<LandscapeInput>,
    sources: Arc<[Source]>,
    source_indices: Vec<usize>,
    water: Vec<usize>,
    water_links: Vec<Connection>,
    water_triangles: Vec<WaterTriangle>,
    green_links: Vec<Connection>,
    shore_links: Vec<Connection>,
    structure_links: Vec<Connection>,
    structure_green: Vec<Connection>,
}
fn adjacent(a: &Source, b: &Source) -> bool {
    (0..3)
        .map(|i| (i64::from(a.data.coordinate[i]) - i64::from(b.data.coordinate[i])).abs())
        .max()
        == Some(1)
}
impl LandscapeSampler {
    pub fn new(input: LandscapeInput) -> Self {
        let sources: Vec<_> = input
            .sources
            .iter()
            .map(|s| Source {
                point: input.projection.hex_center(AxialPoint {
                    q: s.coordinate[0],
                    r: s.coordinate[1],
                }),
                data: s.clone(),
            })
            .collect();
        let mut result = Self {
            input: Arc::new(input),
            source_indices: (0..sources.len()).collect(),
            sources: sources.into(),
            water: vec![],
            water_links: vec![],
            water_triangles: vec![],
            green_links: vec![],
            shore_links: vec![],
            structure_links: vec![],
            structure_green: vec![],
        };
        for a in 0..result.sources.len() {
            let sa = &result.sources[a];
            if sa.data.kind == LandscapeKind::Ocean {
                result.water.push(a)
            }
            for b in a + 1..result.sources.len() {
                let sb = &result.sources[b];
                if !adjacent(sa, sb) {
                    continue;
                }
                let (ak, bk) = (sa.data.kind, sb.data.kind);
                if ak == LandscapeKind::Ocean && bk == LandscapeKind::Ocean {
                    result
                        .water_links
                        .push(Connection::new([a, b], sa.point, sb.point));
                }
                if ak.green() && bk.green() {
                    result
                        .green_links
                        .push(Connection::new([a, b], sa.point, sb.point));
                }
                if (ak.green() && bk == LandscapeKind::Ocean)
                    || (bk.green() && ak == LandscapeKind::Ocean)
                {
                    result
                        .shore_links
                        .push(Connection::new([a, b], sa.point, sb.point));
                }
                if ak == LandscapeKind::Structure
                    && bk == LandscapeKind::Structure
                    && !sa.capital()
                    && !sb.capital()
                {
                    result
                        .structure_links
                        .push(Connection::new([a, b], sa.point, sb.point));
                }
                if ak == LandscapeKind::Structure
                    && matches!(bk, LandscapeKind::Greenery | LandscapeKind::SpecialGreenery)
                {
                    result
                        .structure_green
                        .push(Connection::new([a, b], sa.point, sb.point));
                }
                if bk == LandscapeKind::Structure
                    && matches!(ak, LandscapeKind::Greenery | LandscapeKind::SpecialGreenery)
                {
                    result
                        .structure_green
                        .push(Connection::new([b, a], sb.point, sa.point));
                }
            }
        }
        for link in &result.water_links {
            let [a, b] = link.endpoints;
            for &c in &result.water {
                if c > b
                    && adjacent(&result.sources[a], &result.sources[c])
                    && adjacent(&result.sources[b], &result.sources[c])
                {
                    result.water_triangles.push(WaterTriangle::new(
                        [a, b, c],
                        [
                            result.sources[a].point,
                            result.sources[b].point,
                            result.sources[c].point,
                        ],
                    ));
                }
            }
        }
        result
    }
    /// Keep complete topology, then cull only primitives whose entire influence
    /// lies outside the patch. Endpoint cropping alone loses cross-patch links.
    pub(super) fn for_patch(&self, key: [i32; 2]) -> Self {
        let bounds = Bounds::patch(key);
        let mut local = self.clone();
        local
            .source_indices
            .retain(|&i| bounds.contains(self.sources[i].point, PATCH_SIZE * 1.1));
        // Shore is clamped to .2 after a perturbation of at most .006; warped
        // sample coordinates move by at most .04 on either axis.
        let water_reach = WATER_RADIUS + 0.206 + 0.04;
        local
            .water
            .retain(|&i| bounds.contains(self.sources[i].point, water_reach));
        local
            .water_links
            .retain(|link| link.affects(bounds, water_reach));
        local
            .water_triangles
            .retain(|triangle| triangle.affects(bounds));
        local
            .green_links
            .retain(|link| link.affects(bounds, 0.115 + 0.006));
        local
            .shore_links
            .retain(|link| link.affects(bounds, 0.145 + 0.006));
        local
            .structure_links
            .retain(|link| link.affects(bounds, 0.115 + 0.004));
        // Structure/greenery infill extends at most .14 across the link, with
        // .003 axial noise and .08 extension past the midpoint.
        local
            .structure_green
            .retain(|link| link.affects(bounds, 0.15));
        local
    }
    /// The same support set drives evaluation and cache invalidation.
    pub(super) fn dependencies(&self) -> BTreeSet<usize> {
        let mut indices: BTreeSet<_> = self
            .source_indices
            .iter()
            .chain(&self.water)
            .copied()
            .collect();
        for link in self
            .water_links
            .iter()
            .chain(&self.green_links)
            .chain(&self.shore_links)
            .chain(&self.structure_links)
            .chain(&self.structure_green)
        {
            indices.extend(link.endpoints);
        }
        for triangle in &self.water_triangles {
            indices.extend(triangle.endpoints);
        }
        indices
    }
    pub fn input(&self) -> &LandscapeInput {
        &self.input
    }
    pub fn sample(&self, p: BoardPoint, placement: bool) -> Result<LandscapeSample, TerrainError> {
        let basis = self.basis(p)?;
        self.sample_with_basis(p, placement, basis)
    }
    pub(super) fn basis(&self, p: BoardPoint) -> Result<SampleBasis, TerrainError> {
        let (x, y) = (p.x, p.y);
        if !x.is_finite() || !y.is_finite() {
            return Err(TerrainError::NonFinite);
        }
        if x.abs() > 1e6 || y.abs() > 1e6 {
            return Err(TerrainError::OutsideDomain);
        }
        let seed = self.input.seed;
        let broad = fractal(x * 2.6, y * 2.6, seed);
        let wx = x + (noise(x * 5.0, y * 5.0, seed.wrapping_add(1)) - 0.5) * 0.1;
        let wy = y + (noise(x * 5.0 + 19.0, y * 5.0, seed.wrapping_add(2)) - 0.5) * 0.1;
        let soil = fractal(wx * 11.0, wy * 11.0, seed.wrapping_add(3));
        let fine = noise(x * 73.0, y * 73.0, seed.wrapping_add(4));
        let ox = x + (fractal(x * 18.0, y * 18.0, seed.wrapping_add(5)) - 0.5) * 0.08;
        let oy = y + (fractal(x * 18.0 + 31.7, y * 18.0, seed.wrapping_add(5)) - 0.5) * 0.08;
        let mound = fractal(wx * 9.0, wy * 9.0, seed.wrapping_add(6));
        let relief = if let Some(relief) = &self.input.relief {
            relief.sample_direction(
                self.input
                    .projection
                    .normal(p)
                    .map_err(|_| TerrainError::OutsideDomain)?,
            )?
        } else {
            0.0
        };
        Ok(SampleBasis {
            broad,
            wx,
            wy,
            soil,
            fine,
            ox,
            oy,
            mound,
            relief,
        })
    }
    pub(super) fn sample_with_basis(
        &self,
        p: BoardPoint,
        placement: bool,
        basis: SampleBasis,
    ) -> Result<LandscapeSample, TerrainError> {
        let (x, y) = (p.x, p.y);
        let SampleBasis {
            broad,
            wx,
            wy,
            soil,
            fine,
            ox,
            oy,
            mound,
            relief,
        } = basis;
        let mut shore = 1.0_f64;
        for &a in &self.water {
            let s = &self.sources[a];
            shore = shore.min(planar_length(ox - s.point.x, oy - s.point.y) - WATER_RADIUS)
        }
        for link in &self.water_links {
            shore = shore.min(link.distance(ox, oy) - WATER_RADIUS)
        }
        for triangle in &self.water_triangles {
            let inset = triangle.inset(ox, oy);
            if inset >= 0.0 {
                shore = shore.min(-WATER_RADIUS - inset)
            }
        }
        shore = (shore + (fine - 0.5) * 0.012).min(0.2);
        let moisture =
            (1.0 - smooth(0.005, 0.06 + soil * 0.09, shore.max(0.0))) * (0.55 + broad * 0.45);
        let (mut growth, mut greenery, mut forest, mut strongest) =
            (0.0_f64, 0.0_f64, 0.0_f64, 0.0_f64);
        let (mut blocked, mut structure_pad, mut foundation, mut exclusion) =
            (false, 0.0_f64, 0.0_f64, 0.0_f64);
        let mut source = None;
        for &index in &self.source_indices {
            let s = &self.sources[index];
            let (lx, ly) = (x - s.point.x, y - s.point.y);
            if lx.abs() > PATCH_SIZE * 1.1 || ly.abs() > PATCH_SIZE * 1.1 {
                continue;
            }
            let distance = hex_distance(lx, ly, PATCH_SIZE * 0.5);
            let kind = s.data.kind;
            if kind == LandscapeKind::Ocean {
                continue;
            }
            if kind == LandscapeKind::Excluded {
                let edge =
                    hex_distance(wx - s.point.x, wy - s.point.y, TILE_RADIUS) + (soil - 0.5) * 0.04;
                exclusion = exclusion.max(1.0 - smooth(-0.045, -0.005, edge));
                blocked |= distance < 0.008;
                continue;
            }
            let structure = kind == LandscapeKind::Structure;
            let capital = s.capital();
            let circular = structure && s.circular();
            let footprint = if circular {
                wall(s.layout().expect("circular layout"), lx, ly)
            } else {
                distance
            };
            if structure {
                let edge = footprint
                    + (soil - 0.5) * if circular { 0.006 } else { 0.018 }
                    + (fine - 0.5) * 0.004;
                if !capital {
                    blocked |= footprint < 0.03;
                    foundation = foundation.max(if circular {
                        1.0 - smooth(0.004, 0.024, edge)
                    } else {
                        1.0 - smooth(-0.025, 0.023, edge)
                    });
                }
            }
            let suitability = smooth(0.16, 0.75, soil * 0.7 + broad * 0.2 + moisture * 0.35);
            let warped = hex_distance(wx - s.point.x, wy - s.point.y, PATCH_SIZE * 0.5);
            let mut influence = (1.0 - smooth(-0.06, 0.025, warped))
                * (1.0 - smooth(0.0, PATCH_SIZE * 0.25, distance));
            if circular {
                influence = 1.0 - smooth(0.004, 0.028, footprint)
            }
            if !structure {
                let edge = hex_distance(wx - s.point.x, wy - s.point.y, TILE_RADIUS)
                    + (soil - 0.5) * 0.05
                    + (fine - 0.5) * 0.006;
                influence = 1.0 - smooth(-0.02, 0.05, edge)
            }
            let canopy = (1.0 - smooth(-0.035, 0.006, warped))
                * (0.65 + 0.35 * smooth(0.22, 0.66, soil + moisture * 0.18));
            let mut fertility = if structure { 0.48 } else { 1.0 };
            if structure && let Some(layout) = s.layout() {
                if layout.style.ground == StructureGround::Recessed && planar_length(lx, ly) < 0.095
                {
                    exclusion = 1.0;
                    blocked = true
                }
                if layout.style.landscaping == StructureLandscaping::Sparse {
                    fertility = 0.1
                }
                if capital {
                    fertility = 1.0;
                    influence = 1.0 - smooth(-0.015, 0.018, wall(layout, lx, ly))
                }
                if placement || capital {
                    let (rd, rw) = road(layout, lx, ly);
                    let wd = wall(layout, lx, ly);
                    if capital {
                        let paved = layout
                            .buildings
                            .iter()
                            .any(|b| building(b, lx, ly, 0.0008) || entrance(b, lx, ly, 0.0018));
                        let plaza = lx.abs().max(ly.abs()) < 0.04475;
                        blocked |= plaza;
                        let paving = (if paved || plaza { 1.0_f64 } else { 0.0 })
                            .max(1.0 - smooth(rw, rw + 0.0014, rd))
                            .max(1.0 - smooth(0.002, 0.005, wd.abs()));
                        foundation = foundation.max(paving * influence);
                    }
                    blocked |= inside(layout, lx, ly, 0.003)
                        || rd < rw + 0.003
                        || (layout.style.perimeter != crate::structure::StructurePerimeter::Open
                            && wd.abs() < 0.01);
                }
                structure_pad = structure_pad.max(if circular {
                    1.0 - smooth(0.0, 0.026, footprint)
                } else {
                    1.0 - smooth(-0.028, 0.015, distance)
                });
            }
            if kind == LandscapeKind::Volcano && planar_length(lx, ly) < 0.117 {
                exclusion = 1.0;
                blocked = true
            }
            let strength = influence * fertility;
            if strength > strongest {
                source = Some(s.data.coordinate);
                strongest = strength
            }
            growth = 1.0 - (1.0 - growth) * (1.0 - strength * (0.88 + suitability * 0.12));
            if matches!(
                kind,
                LandscapeKind::Greenery | LandscapeKind::SpecialGreenery
            ) {
                greenery = greenery.max(influence)
            }
            forest = forest.max(canopy * if structure { 0.12 } else { 1.0 });
        }
        for link in &self.green_links {
            let infill = 1.0 - smooth(0.085, 0.115, link.distance(x, y) + (soil - 0.5) * 0.012);
            growth = growth.max(infill);
            greenery = greenery.max(infill)
        }
        let mut shore_growth = 0.0_f64;
        for link in &self.shore_links {
            shore_growth = shore_growth
                .max(1.0 - smooth(0.11, 0.145, link.distance(x, y) + (soil - 0.5) * 0.012))
        }
        shore_growth *= smooth(0.001, 0.006, shore);
        growth = growth.max(shore_growth);
        for link in &self.structure_links {
            let infill = 1.0 - smooth(0.085, 0.115, link.distance(x, y) + (soil - 0.5) * 0.008);
            foundation = foundation.max(infill);
            structure_pad = structure_pad.max(infill)
        }
        for link in &self.structure_green {
            let s = &self.sources[link.endpoints[0]];
            let [dx, dy] = link.direction;
            let midpoint = link.midpoint;
            let (lx, ly) = (x - s.point.x, y - s.point.y);
            let along = lx * dx + ly * dy + (soil - 0.5) * 0.006;
            let across = (lx * dy - ly * dx).abs();
            let wd = s.layout().map_or_else(
                || hex_distance(lx, ly, TILE_RADIUS) + 0.01,
                |l| wall(l, lx, ly),
            );
            if s.circular() {
                let half = midpoint / 3.0_f64.sqrt();
                let edge = (1.0 - smooth(half, half + 0.025, across))
                    * smooth(0.0, 0.025, along)
                    * (1.0 - smooth(midpoint + 0.04, midpoint + 0.08, along));
                growth = growth.max(
                    edge * smooth(
                        0.004,
                        0.024,
                        wd + (soil - 0.5) * 0.006 + (fine - 0.5) * 0.004,
                    ),
                );
                continue;
            }
            let edge = (1.0 - smooth(0.08, 0.115, across))
                * smooth(midpoint - 0.07, midpoint - 0.035, along)
                * smooth(0.0, 0.015, wd)
                * (1.0 - smooth(midpoint + 0.04, midpoint + 0.08, along));
            growth = growth.max(edge * smooth(midpoint - 0.035, midpoint + 0.025, along));
            if !s.capital() {
                let paving = edge * (1.0 - smooth(midpoint - 0.035, midpoint + 0.035, along));
                foundation = foundation.max(paving);
                structure_pad = structure_pad.max(paving)
            }
        }
        let wet_grass = (1.0 - smooth(0.083, 0.125 + soil * 0.02, shore.max(0.0)))
            * (0.72 + soil * 0.2)
            * smooth(0.003, 0.014, shore);
        let coverage = clamp(growth.max(wet_grass) * (1.0 - exclusion));
        forest *= 1.0 - exclusion;
        foundation *= (1.0 - exclusion) * smooth(-0.002, 0.012, shore);
        let wet_soil = moisture
            * (1.0 - smooth(0.002, 0.04, shore.max(0.0)))
            * (0.3 + soil * 0.7)
            * (1.0 - shore_growth * 0.75);
        let fringe =
            coverage * (1.0 - smooth(0.5, 0.88, coverage)) * (1.0 - smooth(0.05, 0.45, forest));
        let mud = wet_soil.max(fringe * 0.4);
        let litter = (coverage * 0.28).min(forest * (0.15 + soil * 0.14));
        let lush = (coverage - litter - mud)
            .max(0.0)
            .min(coverage * (0.8 + moisture * 0.12) * smooth(0.08, 0.65, coverage));
        let dry = (coverage - lush - litter - mud).max(0.0);
        let beach = BEACH_WIDTH_MAX + (BEACH_WIDTH_MIN - BEACH_WIDTH_MAX) * clamp(lush + litter);
        let hummocks = greenery
            * (1.0 - exclusion)
            * 0.027
            * clamp((mound - 0.25) / 0.45)
            * smooth(beach, beach + 0.03, shore);
        let natural =
            relief * (1.0 - exclusion) + 0.0003 + coverage * (0.0012 + broad * 0.0048) + hummocks;
        let mut height = basin_height(shore, beach, natural)?;
        height = height * (1.0 - exclusion) + natural * exclusion;
        height = height * (1.0 - structure_pad) + 0.0003 * structure_pad;
        if blocked {
            forest = 0.0
        }
        Ok(LandscapeSample {
            relief,
            height,
            shore,
            coverage,
            forest,
            foundation,
            moisture,
            dry,
            lush,
            litter,
            mud,
            meadow: clamp(wet_grass * (1.0 - growth) * (1.0 - exclusion)),
            mound,
            greenery: clamp(greenery * (1.0 - exclusion)),
            source,
            blocked: blocked || shore < beach * 0.8 || exclusion > 0.2,
        })
    }
}
pub(super) fn hex_distance(x: f64, y: f64, radius: f64) -> f64 {
    x.abs()
        .max((x * 0.5 + y * 0.866025404).abs())
        .max((-x * 0.5 + y * 0.866025404).abs())
        - radius * 0.866025404
}
fn noise(x: f64, y: f64, seed: u32) -> f64 {
    landscape_noise(x, y, seed).expect("validated bounded landscape coordinates")
}
pub(super) fn fractal(x: f64, y: f64, seed: u32) -> f64 {
    noise(x, y, seed) * 0.57
        + noise(x * 2.1, y * 2.1, seed.wrapping_add(41)) * 0.28
        + noise(x * 4.3, y * 4.3, seed.wrapping_add(79)) * 0.15
}

#[cfg(test)]
mod support_tests {
    use super::*;
    use crate::geometry::terrain::{FIELD_BORDER, FIELD_SAMPLES, FIELD_STEP};

    #[test]
    fn patch_support_matches_complete_topology_at_interiors_borders_and_corners() {
        let layouts: Vec<StructureLayoutRef> = ["grid", "radial", "clustered", "courtyard"]
            .into_iter()
            .map(|plan| {
                crate::structure::layout::generate(
                    17,
                    &crate::structure::StructureStyleRequest {
                        plan: Some(plan.into()),
                        ..Default::default()
                    },
                    "Culling test",
                )
                .unwrap()
            })
            .collect();
        let mut culled = 0;
        for pattern in 0..4 {
            let mut sources = Vec::new();
            for q in -4i32..=4 {
                for r in -4i32..=4 {
                    if q.abs().max(r.abs()).max((q + r).abs()) > 4 {
                        continue;
                    }
                    let index = sources.len();
                    let kind = match (pattern, index % 6) {
                        (0, _) => LandscapeKind::Ocean,
                        (1, _) => LandscapeKind::Greenery,
                        (3, _) => LandscapeKind::Structure,
                        (_, 0) => LandscapeKind::Ocean,
                        (_, 1) => LandscapeKind::Structure,
                        (_, 2) => LandscapeKind::Greenery,
                        (_, 3) => LandscapeKind::SpecialGreenery,
                        (_, 4) => LandscapeKind::Volcano,
                        _ => LandscapeKind::Excluded,
                    };
                    sources.push(LandscapeSource {
                        coordinate: [q, r, -q - r],
                        kind,
                        seed: 17,
                        layout: (kind == LandscapeKind::Structure)
                            .then(|| layouts[index % layouts.len()].clone()),
                    });
                }
            }
            let complete = LandscapeSampler::new(
                LandscapeInput::new(
                    BodyId::Mars,
                    BodyProjection::MARS,
                    0,
                    u32::MAX,
                    None,
                    sources,
                )
                .unwrap(),
            );
            for ix in -4..=4 {
                for iy in -4..=4 {
                    let local = complete.for_patch([ix, iy]);
                    culled += complete.green_links.len() - local.green_links.len();
                    for col in [0, 1, 2, 16, 32, 64, 96, 112, 129, 130, 131] {
                        for row in [0, 1, 2, 16, 32, 64, 96, 112, 129, 130, 131] {
                            let p = BoardPoint {
                                x: f64::from(
                                    ix * (FIELD_SAMPLES - 1) as i32 + col - FIELD_BORDER as i32,
                                ) * FIELD_STEP,
                                y: f64::from(
                                    iy * (FIELD_SAMPLES - 1) as i32 + row - FIELD_BORDER as i32,
                                ) * FIELD_STEP,
                            };
                            for placement in [false, true] {
                                assert_eq!(
                                    local.sample(p, placement).unwrap(),
                                    complete.sample(p, placement).unwrap(),
                                    "pattern={pattern}, patch={ix},{iy}, point={p:?}"
                                );
                            }
                        }
                    }
                }
            }
        }
        assert!(culled > 0, "the comparison must exercise actual culling");
    }
}
