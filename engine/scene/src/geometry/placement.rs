//! Pure placement math. No card identities, gameplay legality, or graphics state.
use super::{
    BoardPoint,
    terrain::{TerrainError, TerrainField, TerrainSample},
};
use std::{
    collections::{BTreeMap, BTreeSet},
    fmt,
};

pub const MAX_POLYGON_VERTICES: usize = 64;
pub const MAX_FEATURE_SLOTS: usize = 1024;
pub const MAX_PLACEMENT_ATTEMPTS: u32 = 65536;
pub const MAX_PLACEMENT_CHECKS: u32 = 1_048_576;
pub const MAX_OUTPUT_VERTICES: usize = 8192;

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum PlacementError {
    NonFinite,
    InvalidShape,
    SelfIntersection,
    ResourceLimit,
    DuplicateId,
    InvalidRank,
    NumericOverflow,
    PackingFailed,
    Grounding(TerrainError),
}
impl fmt::Display for PlacementError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{self:?}")
    }
}
impl std::error::Error for PlacementError {}
impl From<TerrainError> for PlacementError {
    fn from(e: TerrainError) -> Self {
        Self::Grounding(e)
    }
}

/// FNV-1a over UTF-8, matching landscapeGeometry.ts hashSeed.
pub fn hash_seed(value: &str) -> u32 {
    value.as_bytes().iter().fold(2166136261_u32, |h, b| {
        (h ^ u32::from(*b)).wrapping_mul(16777619)
    })
}
/// Explicit per-request mulberry32 state, never shared globally or by thread.
pub fn random_sequence(mut seed: u32) -> impl FnMut() -> f64 {
    move || {
        seed = seed.wrapping_add(0x6d2b79f5);
        let mut t = (seed ^ (seed >> 15)).wrapping_mul(1 | seed);
        t ^= t.wrapping_add((t ^ (t >> 7)).wrapping_mul(61 | t));
        f64::from(t ^ (t >> 14)) / 4294967296.0
    }
}
pub fn lattice_hash(x: i32, y: i32, seed: u32) -> f64 {
    let n = (x as u32).wrapping_mul(374761393) ^ (y as u32).wrapping_mul(668265263) ^ seed;
    let n = (n ^ (n >> 13)).wrapping_mul(1274126177);
    f64::from(n ^ (n >> 16)) / 4294967296.0
}
pub fn kept_at_foliage_density(seed: u32, density: f64) -> Result<bool, PlacementError> {
    if !density.is_finite() || !(0.0..=1.0).contains(&density) {
        return Err(PlacementError::InvalidRank);
    }
    let mut h = (seed ^ (seed >> 16)).wrapping_mul(0x45d9f3b);
    h = (h ^ (h >> 16)).wrapping_mul(0x45d9f3b);
    h ^= h >> 16;
    Ok(f64::from(h) / 4294967296.0 < density)
}

#[derive(
    Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord, Hash, serde::Serialize, serde::Deserialize,
)]
pub enum GreeneryKind {
    Tree,
    Pine,
    Bush,
    Clover,
    Flower,
    Rock,
}
impl GreeneryKind {
    pub fn token(self) -> &'static str {
        match self {
            Self::Tree => "tree",
            Self::Pine => "pine",
            Self::Bush => "bush",
            Self::Clover => "clover",
            Self::Flower => "flower",
            Self::Rock => "rock",
        }
    }
}
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct GreeneryCell {
    pub point: BoardPoint,
    pub seed: u32,
    pub jitter: f64,
    pub selector: f64,
}
/// Stable world-cell seed/position. Thresholds and exclusion decisions are
/// supplied separately, so changing a climate gate never reshuffles candidates.
pub fn greenery_cell(
    seed: u32,
    kind: GreeneryKind,
    ix: i32,
    iy: i32,
    spacing: f64,
) -> Result<GreeneryCell, PlacementError> {
    if !spacing.is_finite() || spacing <= 0.0 {
        return Err(PlacementError::InvalidShape);
    }
    let name = kind.token();
    let salt = if kind == GreeneryKind::Pine {
        977
    } else {
        name.len() as u32
    };
    let h = lattice_hash(ix, iy, seed.wrapping_add(salt.wrapping_mul(311)));
    let h2 = lattice_hash(ix, iy, seed.wrapping_add(salt.wrapping_mul(829)));
    let point = BoardPoint {
        x: (f64::from(ix) + 0.2 + h * 0.6) * spacing,
        y: (f64::from(iy) + 0.2 + h2 * 0.6) * spacing,
    };
    finite([point.x, point.y])?;
    Ok(GreeneryCell {
        point,
        seed: hash_seed(&format!("{seed}:{name}:{ix}:{iy}")),
        jitter: h,
        selector: h2,
    })
}
/// Preserve h2/threshold ranks, including rank zero for structure/rock plants.
/// The frontend's 0/0 non-structure edge has no finite rank: report it instead
/// of manufacturing NaN. Structure/rock rank zero remains defined at that edge.
pub fn greenery_rank(
    selector: f64,
    threshold: f64,
    structure: bool,
    kind: GreeneryKind,
) -> Result<Option<f64>, PlacementError> {
    if !selector.is_finite()
        || !threshold.is_finite()
        || !(0.0..1.0).contains(&selector)
        || !(0.0..=1.0).contains(&threshold)
    {
        return Err(PlacementError::InvalidRank);
    }
    if selector > threshold {
        return Ok(None);
    }
    if structure || kind == GreeneryKind::Rock {
        return Ok(Some(0.0));
    }
    if threshold == 0.0 {
        return Err(PlacementError::InvalidRank);
    }
    Ok(Some(selector / threshold))
}

fn finite(p: [f64; 2]) -> Result<(), PlacementError> {
    if p.into_iter().all(f64::is_finite) {
        Ok(())
    } else {
        Err(PlacementError::NonFinite)
    }
}
fn cross(a: [f64; 2], b: [f64; 2], c: [f64; 2]) -> Result<f64, PlacementError> {
    for p in [a, b, c] {
        finite(p)?;
    }
    Ok(match super::placement_predicate::orientation(a, b, c) {
        std::cmp::Ordering::Less => -1.0,
        std::cmp::Ordering::Equal => 0.0,
        std::cmp::Ordering::Greater => 1.0,
    })
}
fn on_segment(p: [f64; 2], a: [f64; 2], b: [f64; 2]) -> Result<bool, PlacementError> {
    Ok(cross(a, b, p)? == 0.0
        && p[0] >= a[0].min(b[0])
        && p[0] <= a[0].max(b[0])
        && p[1] >= a[1].min(b[1])
        && p[1] <= a[1].max(b[1]))
}
fn intersect(a: [f64; 2], b: [f64; 2], c: [f64; 2], d: [f64; 2]) -> Result<bool, PlacementError> {
    let ac = cross(a, b, c)?;
    let ad = cross(a, b, d)?;
    let ca = cross(c, d, a)?;
    let cb = cross(c, d, b)?;
    Ok((ac > 0.0 && ad < 0.0 || ac < 0.0 && ad > 0.0)
        && (ca > 0.0 && cb < 0.0 || ca < 0.0 && cb > 0.0)
        || on_segment(c, a, b)?
        || on_segment(d, a, b)?
        || on_segment(a, c, d)?
        || on_segment(b, c, d)?)
}
pub fn segment_distance(p: [f64; 2], a: [f64; 2], b: [f64; 2]) -> Result<f64, PlacementError> {
    for p in [p, a, b] {
        finite(p)?;
    }
    let d = [b[0] - a[0], b[1] - a[1]];
    let scale = d[0].abs().max(d[1].abs());
    if !scale.is_finite() {
        return Err(PlacementError::NumericOverflow);
    }
    let t = if scale > 0.0 {
        let u = [d[0] / scale, d[1] / scale];
        let relative = [(p[0] - a[0]) / scale, (p[1] - a[1]) / scale];
        let projection = (relative[0] * u[0] + relative[1] * u[1]) / (u[0] * u[0] + u[1] * u[1]);
        if projection.is_nan() {
            return Err(PlacementError::NumericOverflow);
        }
        projection.clamp(0.0, 1.0)
    } else {
        0.0
    };
    let nearest = [a[0] * (1.0 - t) + b[0] * t, a[1] * (1.0 - t) + b[1] * t];
    let distance = (p[0] - nearest[0]).hypot(p[1] - nearest[1]);
    if !distance.is_finite() {
        return Err(PlacementError::NumericOverflow);
    }
    Ok(distance)
}

/// Closed simple polygon (last vertex implicitly joins the first), either
/// winding, including concave footprints. No holes or self-intersections.
#[derive(Clone, Debug, PartialEq)]
pub struct PolygonFootprint {
    vertices: Vec<[f64; 2]>,
}
impl PolygonFootprint {
    pub fn new(vertices: Vec<[f64; 2]>) -> Result<Self, PlacementError> {
        let n = vertices.len();
        if n > MAX_POLYGON_VERTICES {
            return Err(PlacementError::ResourceLimit);
        }
        if n < 3 {
            return Err(PlacementError::InvalidShape);
        }
        for &v in &vertices {
            finite(v)?;
        }
        for i in 0..n {
            let a = vertices[i];
            let b = vertices[(i + 1) % n];
            if a == b {
                return Err(PlacementError::InvalidShape);
            }
            // Adjacent collinear edges may continue, but may not double back.
            let c = vertices[(i + 2) % n];
            if cross(a, b, c)? == 0.0 && (on_segment(a, b, c)? || on_segment(c, a, b)?) {
                return Err(PlacementError::InvalidShape);
            }
            for j in i + 1..n {
                if j == i + 1 || i == 0 && j == n - 1 {
                    continue;
                }
                if intersect(a, b, vertices[j], vertices[(j + 1) % n])? {
                    return Err(PlacementError::SelfIntersection);
                }
            }
        }
        if super::placement_predicate::area(&vertices) == std::cmp::Ordering::Equal {
            return Err(PlacementError::InvalidShape);
        }
        Ok(Self { vertices })
    }
    pub fn vertices(&self) -> &[[f64; 2]] {
        &self.vertices
    }
    pub fn contains(&self, p: [f64; 2]) -> Result<bool, PlacementError> {
        finite(p)?;
        let mut inside = false;
        for i in 0..self.vertices.len() {
            let a = self.vertices[i];
            let b = self.vertices[(i + 1) % self.vertices.len()];
            if on_segment(p, a, b)? {
                return Ok(true);
            }
            if (a[1] > p[1]) != (b[1] > p[1]) {
                let side = cross(a, b, p)?;
                if (side > 0.0) == (b[1] > a[1]) {
                    inside = !inside;
                }
            }
        }
        Ok(inside)
    }
    pub fn distance(&self, p: [f64; 2]) -> Result<f64, PlacementError> {
        if self.contains(p)? {
            return Ok(0.0);
        }
        let mut distance = f64::INFINITY;
        for i in 0..self.vertices.len() {
            distance = distance.min(segment_distance(
                p,
                self.vertices[i],
                self.vertices[(i + 1) % self.vertices.len()],
            )?);
        }
        Ok(distance)
    }
    /// Whole-footprint containment, including concave boundaries. An edge
    /// through a reentrant boundary vertex is split before midpoint tests.
    pub fn contains_polygon(&self, other: &Self, clearance: f64) -> Result<bool, PlacementError> {
        if !clearance.is_finite() || clearance < 0.0 {
            return Err(PlacementError::InvalidShape);
        }
        for &p in &other.vertices {
            if !self.contains(p)? {
                return Ok(false);
            }
        }
        for i in 0..other.vertices.len() {
            let a = other.vertices[i];
            let b = other.vertices[(i + 1) % other.vertices.len()];
            let axis = if (b[0] - a[0]).abs() >= (b[1] - a[1]).abs() {
                0
            } else {
                1
            };
            let mut cuts = vec![0.0, 1.0];
            for j in 0..self.vertices.len() {
                let c = self.vertices[j];
                let d = self.vertices[(j + 1) % self.vertices.len()];
                let x = cross(a, b, c)?;
                let y = cross(a, b, d)?;
                let u = cross(c, d, a)?;
                let v = cross(c, d, b)?;
                if x * y < 0.0 && u * v < 0.0 {
                    return Ok(false);
                }
                if on_segment(c, a, b)? {
                    let t = (c[axis] - a[axis]) / (b[axis] - a[axis]);
                    if !t.is_finite() {
                        return Err(PlacementError::NumericOverflow);
                    }
                    cuts.push(t);
                }
                if clearance > 0.0
                    && (segment_distance(a, c, d)? < clearance
                        || segment_distance(b, c, d)? < clearance
                        || segment_distance(c, a, b)? < clearance
                        || segment_distance(d, a, b)? < clearance)
                {
                    return Ok(false);
                }
            }
            cuts.sort_by(f64::total_cmp);
            cuts.dedup();
            for pair in cuts.windows(2) {
                let t = pair[0] + (pair[1] - pair[0]) * 0.5;
                let p = [a[0] * (1.0 - t) + b[0] * t, a[1] * (1.0 - t) + b[1] * t];
                if !self.contains(p)? {
                    return Ok(false);
                }
            }
        }
        Ok(true)
    }
    pub fn overlaps(&self, other: &Self, clearance: f64) -> Result<bool, PlacementError> {
        if !clearance.is_finite() || clearance < 0.0 {
            return Err(PlacementError::InvalidShape);
        }
        if self.contains(other.vertices[0])? || other.contains(self.vertices[0])? {
            return Ok(true);
        }
        for i in 0..self.vertices.len() {
            for j in 0..other.vertices.len() {
                let a = self.vertices[i];
                let b = self.vertices[(i + 1) % self.vertices.len()];
                let c = other.vertices[j];
                let d = other.vertices[(j + 1) % other.vertices.len()];
                if intersect(a, b, c, d)?
                    || segment_distance(a, c, d)? <= clearance
                    || segment_distance(b, c, d)? <= clearance
                    || segment_distance(c, a, b)? <= clearance
                    || segment_distance(d, a, b)? <= clearance
                {
                    return Ok(true);
                }
            }
        }
        Ok(false)
    }
    pub fn bounds(&self) -> [[f64; 2]; 2] {
        self.vertices
            .iter()
            .fold([self.vertices[0]; 2], |mut b, p| {
                for c in 0..2 {
                    b[0][c] = b[0][c].min(p[c]);
                    b[1][c] = b[1][c].max(p[c]);
                }
                b
            })
    }
    pub fn transformed(&self, t: PlacementTransform) -> Result<Self, PlacementError> {
        Self::new(
            self.vertices
                .iter()
                .map(|&p| t.to_board(p).map(|p| [p.x, p.y]))
                .collect::<Result<_, _>>()?,
        )
    }
}

#[derive(Clone, Copy, Debug, PartialEq)]
pub struct PlacementTransform {
    origin: BoardPoint,
    rotation: f64,
    scale: f64,
}
impl PlacementTransform {
    pub fn new(origin: BoardPoint, rotation: f64, scale: f64) -> Result<Self, PlacementError> {
        finite([origin.x, origin.y])?;
        if !rotation.is_finite() || !scale.is_finite() || scale <= 0.0 {
            return Err(PlacementError::InvalidShape);
        }
        Ok(Self {
            origin,
            rotation,
            scale,
        })
    }
    pub fn to_board(self, point: [f64; 2]) -> Result<BoardPoint, PlacementError> {
        finite(point)?;
        let (s, c) = self.rotation.sin_cos();
        let p = BoardPoint {
            x: self.origin.x + (c * point[0] - s * point[1]) * self.scale,
            y: self.origin.y + (s * point[0] + c * point[1]) * self.scale,
        };
        finite([p.x, p.y]).map_err(|_| PlacementError::NumericOverflow)?;
        Ok(p)
    }
    pub fn to_local(self, point: BoardPoint) -> Result<[f64; 2], PlacementError> {
        finite([point.x, point.y])?;
        let (s, c) = self.rotation.sin_cos();
        let x = (point.x - self.origin.x) / self.scale;
        let y = (point.y - self.origin.y) / self.scale;
        let p = [c * x + s * y, -s * x + c * y];
        finite(p).map_err(|_| PlacementError::NumericOverflow)?;
        Ok(p)
    }
    pub fn grounded(
        self,
        point: [f64; 2],
        terrain: &TerrainField,
    ) -> Result<GroundedPoint, PlacementError> {
        let board = self.to_board(point)?;
        let sample = terrain.sample(board)?;
        Ok(GroundedPoint {
            board,
            position: terrain.contact(board)?,
            sample,
        })
    }
}
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct GroundedPoint {
    pub board: BoardPoint,
    pub position: [f64; 3],
    pub sample: TerrainSample,
}

/// Numeric footprint expressed in request-local units; rotation is radians
/// about the local origin. Clearance is required from the enclosing boundary,
/// exclusions, and other features (pairwise maximum, never an implicit sum).
#[derive(Clone, Debug, PartialEq)]
pub struct FeatureRequest {
    id: u64,
    footprint: PolygonFootprint,
    count: u16,
    rotation: f64,
    clearance: f64,
}
impl FeatureRequest {
    pub fn new(
        id: u64,
        footprint: PolygonFootprint,
        count: u16,
        rotation: f64,
        clearance: f64,
    ) -> Result<Self, PlacementError> {
        if count == 0 || usize::from(count) > MAX_FEATURE_SLOTS {
            return Err(PlacementError::ResourceLimit);
        }
        if !rotation.is_finite() || !clearance.is_finite() || clearance < 0.0 {
            return Err(PlacementError::InvalidShape);
        }
        Ok(Self {
            id,
            footprint,
            count,
            rotation,
            clearance,
        })
    }
}
/// Total candidate attempts and conservative polygon-edge work are both capped.
/// The caller chooses smaller budgets freely; exhaustion is an explicit error.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct PlacementBudget {
    pub max_slots: u16,
    pub max_attempts: u32,
    pub max_checks: u32,
}
impl PlacementBudget {
    fn validate(self) -> Result<(), PlacementError> {
        if self.max_slots == 0
            || usize::from(self.max_slots) > MAX_FEATURE_SLOTS
            || self.max_attempts == 0
            || self.max_attempts > MAX_PLACEMENT_ATTEMPTS
            || self.max_checks == 0
            || self.max_checks > MAX_PLACEMENT_CHECKS
        {
            return Err(PlacementError::ResourceLimit);
        }
        Ok(())
    }
}
/// Immutable numeric request. Boundary/exclusions share local units; transform
/// maps them into board units with explicit positive scale and radian rotation.
/// This does not interpret authored styles or architectural renderer recipes.
#[derive(Clone, Debug)]
pub struct StructureRequest {
    seed: u32,
    transform: PlacementTransform,
    boundary: PolygonFootprint,
    features: Vec<FeatureRequest>,
    exclusions: Vec<PolygonFootprint>,
    budget: PlacementBudget,
}
#[derive(Clone, Debug, PartialEq)]
pub struct PlacementSlot {
    pub feature_id: u64,
    pub ordinal: u16,
    pub seed: u32,
    /// Stable normalized rank; consumers apply gates without reordering slots.
    pub rank: f64,
    pub local: [f64; 2],
    pub grounded: GroundedPoint,
    pub local_footprint: PolygonFootprint,
    pub board_footprint: PolygonFootprint,
}
#[derive(Clone, Debug, PartialEq)]
pub struct PlacementLayout {
    seed: u32,
    slots: Vec<PlacementSlot>,
    boundary: PolygonFootprint,
    exclusions: Vec<PolygonFootprint>,
}
impl PlacementLayout {
    pub fn seed(&self) -> u32 {
        self.seed
    }
    /// Canonical ascending feature ID, then ascending ordinal.
    pub fn slots(&self) -> &[PlacementSlot] {
        &self.slots
    }
    pub fn board_boundary(&self) -> &PolygonFootprint {
        &self.boundary
    }
    pub fn board_exclusions(&self) -> &[PolygonFootprint] {
        &self.exclusions
    }
    pub fn excluded(&self, board: [f64; 2], padding: f64) -> Result<bool, PlacementError> {
        finite(board)?;
        if !padding.is_finite() || padding < 0.0 {
            return Err(PlacementError::InvalidShape);
        }
        for p in self
            .exclusions
            .iter()
            .chain(self.slots.iter().map(|s| &s.board_footprint))
        {
            if p.distance(board)? <= padding {
                return Ok(true);
            }
        }
        Ok(false)
    }
}
struct Work {
    attempts: u32,
    checks: u32,
}
impl Work {
    fn check(&mut self, a: usize, b: usize) -> Result<(), PlacementError> {
        // Containment also tests all vertices and split edge intervals. Bound
        // its cubic worst case conservatively, before temporary allocation.
        let cost = a
            .checked_mul(b)
            .and_then(|v| v.checked_mul(b + 2))
            .and_then(|v| u32::try_from(v).ok())
            .ok_or(PlacementError::ResourceLimit)?;
        self.checks = self
            .checks
            .checked_sub(cost)
            .ok_or(PlacementError::ResourceLimit)?;
        Ok(())
    }
}
impl StructureRequest {
    pub fn new(
        seed: u32,
        transform: PlacementTransform,
        boundary: PolygonFootprint,
        mut features: Vec<FeatureRequest>,
        mut exclusions: Vec<PolygonFootprint>,
        budget: PlacementBudget,
    ) -> Result<Self, PlacementError> {
        budget.validate()?;
        if features.len() > MAX_FEATURE_SLOTS || exclusions.len() > 128 {
            return Err(PlacementError::ResourceLimit);
        }
        features.sort_by_key(|f| f.id);
        exclusions.sort_by(|a, b| {
            a.vertices
                .iter()
                .flatten()
                .zip(b.vertices.iter().flatten())
                .map(|(x, y)| x.total_cmp(y))
                .find(|o| !o.is_eq())
                .unwrap_or(a.vertices.len().cmp(&b.vertices.len()))
        });
        if features.windows(2).any(|w| w[0].id == w[1].id) {
            return Err(PlacementError::DuplicateId);
        }
        let total = features
            .iter()
            .try_fold(0usize, |n, f| n.checked_add(usize::from(f.count)))
            .ok_or(PlacementError::ResourceLimit)?;
        let vertices = features
            .iter()
            .try_fold(0usize, |n, f| {
                f.footprint
                    .vertices
                    .len()
                    .checked_mul(usize::from(f.count))
                    .and_then(|m| n.checked_add(m))
            })
            .ok_or(PlacementError::ResourceLimit)?;
        if total > usize::from(budget.max_slots)
            || total > budget.max_attempts as usize
            || vertices > MAX_OUTPUT_VERTICES
        {
            return Err(PlacementError::ResourceLimit);
        }
        Ok(Self {
            seed,
            transform,
            boundary,
            features,
            exclusions,
            budget,
        })
    }
    pub fn layout(&self, terrain: &TerrainField) -> Result<PlacementLayout, PlacementError> {
        let mut work = Work {
            attempts: self.budget.max_attempts,
            checks: self.budget.max_checks,
        };
        let mut accepted: Vec<(PlacementSlot, f64)> = Vec::new();
        let bounds = self.boundary.bounds();
        for feature in &self.features {
            let rotation = PlacementTransform::new(BoardPoint::default(), feature.rotation, 1.0)?;
            let rotated = feature.footprint.transformed(rotation)?;
            let fb = rotated.bounds();
            let low = [
                bounds[0][0] - fb[0][0] + feature.clearance,
                bounds[0][1] - fb[0][1] + feature.clearance,
            ];
            let high = [
                bounds[1][0] - fb[1][0] - feature.clearance,
                bounds[1][1] - fb[1][1] - feature.clearance,
            ];
            finite(low)?;
            finite(high)?;
            if low[0] > high[0] || low[1] > high[1] {
                return Err(PlacementError::PackingFailed);
            }
            for ordinal in 0..feature.count {
                let seed = hash_seed(&format!("{}:{}:{ordinal}", self.seed, feature.id));
                let mut random = random_sequence(seed);
                let rank = random();
                let mut first = true;
                let slot = loop {
                    if work.attempts == 0 {
                        return Err(PlacementError::PackingFailed);
                    }
                    work.attempts -= 1;
                    let t = if first {
                        first = false;
                        [0.5, 0.5]
                    } else {
                        [random(), random()]
                    };
                    let local = [
                        low[0] * (1.0 - t[0]) + high[0] * t[0],
                        low[1] * (1.0 - t[1]) + high[1] * t[1],
                    ];
                    let translated = rotated.transformed(PlacementTransform::new(
                        BoardPoint {
                            x: local[0],
                            y: local[1],
                        },
                        0.0,
                        1.0,
                    )?)?;
                    work.check(translated.vertices.len(), self.boundary.vertices.len())?;
                    if !self
                        .boundary
                        .contains_polygon(&translated, feature.clearance)?
                    {
                        continue;
                    }
                    let mut blocked = false;
                    for exclusion in &self.exclusions {
                        work.check(translated.vertices.len(), exclusion.vertices.len())?;
                        if translated.overlaps(exclusion, feature.clearance)? {
                            blocked = true;
                            break;
                        }
                    }
                    if blocked {
                        continue;
                    }
                    for (other, clearance) in &accepted {
                        work.check(
                            translated.vertices.len(),
                            other.local_footprint.vertices.len(),
                        )?;
                        if translated
                            .overlaps(&other.local_footprint, feature.clearance.max(*clearance))?
                        {
                            blocked = true;
                            break;
                        }
                    }
                    if blocked {
                        continue;
                    }
                    break PlacementSlot {
                        feature_id: feature.id,
                        ordinal,
                        seed,
                        rank,
                        local,
                        grounded: self.transform.grounded(local, terrain)?,
                        board_footprint: translated.transformed(self.transform)?,
                        local_footprint: translated,
                    };
                };
                accepted.push((slot, feature.clearance));
            }
        }
        Ok(PlacementLayout {
            seed: self.seed,
            slots: accepted.into_iter().map(|(s, _)| s).collect(),
            boundary: self.boundary.transformed(self.transform)?,
            exclusions: self
                .exclusions
                .iter()
                .map(|p| p.transformed(self.transform))
                .collect::<Result<_, _>>()?,
        })
    }
}

/// Reconcile stable instance slots. Retiring IDs keep their old slots until the
/// owner drops them; new IDs take lowest free slots in stable numeric-ID order.
pub fn stable_slots(
    previous: &[(u64, u16)],
    desired: &[u64],
    retiring: &[u64],
    capacity: u16,
) -> Result<Vec<(u64, u16)>, PlacementError> {
    if usize::from(capacity) > MAX_FEATURE_SLOTS
        || previous.len() > MAX_FEATURE_SLOTS
        || desired.len() > MAX_FEATURE_SLOTS
        || retiring.len() > MAX_FEATURE_SLOTS
    {
        return Err(PlacementError::ResourceLimit);
    }
    let wanted: BTreeSet<_> = desired.iter().copied().collect();
    let retired: BTreeSet<_> = retiring.iter().copied().collect();
    if wanted.len() != desired.len()
        || retired.len() != retiring.len()
        || !wanted.is_disjoint(&retired)
    {
        return Err(PlacementError::DuplicateId);
    }
    let mut old = BTreeMap::new();
    let mut used = BTreeSet::new();
    for &(id, slot) in previous {
        if slot >= capacity || old.insert(id, slot).is_some() || !used.insert(slot) {
            return Err(PlacementError::InvalidShape);
        }
    }
    if retired.iter().any(|id| !old.contains_key(id)) {
        return Err(PlacementError::InvalidShape);
    }
    let mut result: BTreeMap<_, _> = old
        .into_iter()
        .filter(|(id, _)| wanted.contains(id) || retired.contains(id))
        .collect();
    let mut used: BTreeSet<_> = result.values().copied().collect();
    for id in wanted {
        if let std::collections::btree_map::Entry::Vacant(entry) = result.entry(id) {
            let slot = (0..capacity)
                .find(|s| !used.contains(s))
                .ok_or(PlacementError::ResourceLimit)?;
            used.insert(slot);
            entry.insert(slot);
        }
    }
    Ok(result.into_iter().collect())
}
