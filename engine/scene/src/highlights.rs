//! Bounded presentation data shared by ground, ocean and celestial grids.
//! Availability is supplied by the authoritative view; no legal-move solver.
use crate::geometry::{AxialPoint, BodyProjection, round_cube};
use std::collections::BTreeMap;

pub const TEXELS_PER_TILE: usize = 5;
const MAX_CELLS: usize = 4096;
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct HighlightState {
    pub hovered: bool,
    pub inspected: bool,
    pub available: bool,
    pub selected: bool,
    pub vp_intensity: f64,
    /// Linear RGB, matching the reference shader's presentation color.
    pub vp_color: [f64; 3],
}
impl Default for HighlightState {
    fn default() -> Self {
        Self {
            hovered: false,
            inspected: false,
            available: false,
            selected: false,
            vp_intensity: 0.0,
            vp_color: [0.0; 3],
        }
    }
}
impl HighlightState {
    fn valid(self) -> bool {
        self.vp_intensity.is_finite()
            && (0.0..=1.0).contains(&self.vp_intensity)
            && self
                .vp_color
                .into_iter()
                .all(|v| v.is_finite() && (0.0..=1.0).contains(&v))
    }
}
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum HighlightError {
    Capacity,
    Duplicate,
    InvalidCoordinate,
    UnknownTile,
    NonFinite,
    ClockRegressed,
    RevisionExhausted,
}
struct Record {
    slot: usize,
    state: HighlightState,
    hover: f64,
    vp: f64,
    visibility: f64,
    entrance: f64,
}
pub struct HighlightBoard {
    projection: BodyProjection,
    grid: [i32; 4],
    records: BTreeMap<AxialPoint, Record>,
    data: Vec<f32>,
    last_step: Option<u64>,
    revision: u64,
    active: bool,
}
impl HighlightBoard {
    pub fn new(
        projection: BodyProjection,
        coordinates: &[AxialPoint],
    ) -> Result<Self, HighlightError> {
        if coordinates.len() > 4096 {
            return Err(HighlightError::Capacity);
        }
        let offset = projection.coord_offset();
        let mut min = [i64::MAX; 2];
        let mut max = [i64::MIN; 2];
        let mut records = BTreeMap::new();
        for &p in coordinates {
            p.cube().map_err(|_| HighlightError::InvalidCoordinate)?;
            if records
                .insert(
                    p,
                    Record {
                        slot: 0,
                        state: HighlightState::default(),
                        hover: 0.0,
                        vp: 0.0,
                        visibility: 1.0,
                        entrance: 1.0,
                    },
                )
                .is_some()
            {
                return Err(HighlightError::Duplicate);
            }
            let q = i64::from(p.q) - i64::from(offset.q);
            let r = i64::from(p.r) - i64::from(offset.r);
            if q.abs() > 4096 || r.abs() > 4096 {
                return Err(HighlightError::InvalidCoordinate);
            }
            for (i, v) in [q, r].into_iter().enumerate() {
                min[i] = min[i].min(v);
                max[i] = max[i].max(v)
            }
        }
        if coordinates.is_empty() {
            min = [0; 2];
            max = [0; 2]
        }
        let width = usize::try_from(max[0] - min[0] + 1).map_err(|_| HighlightError::Capacity)?;
        let height = usize::try_from(max[1] - min[1] + 1).map_err(|_| HighlightError::Capacity)?;
        if width > 256 || height > 256 || width.checked_mul(height).is_none_or(|n| n > MAX_CELLS) {
            return Err(HighlightError::Capacity);
        }
        let mut data = vec![0.0; width * height * TEXELS_PER_TILE * 4];
        for (&coordinate, record) in &mut records {
            let x = (i64::from(coordinate.q) - i64::from(offset.q) - min[0]) as usize;
            let y = (i64::from(coordinate.r) - i64::from(offset.r) - min[1]) as usize;
            let slot = (y * width + x) * TEXELS_PER_TILE * 4;
            record.slot = slot;
            let normal = projection
                .normal(projection.hex_center(coordinate))
                .map_err(|_| HighlightError::InvalidCoordinate)?;
            let (x, y) = tangent_basis(normal);
            data[slot + 7] = 1.0;
            for i in 0..3 {
                data[slot + 8 + i] = normal[i] as f32;
                data[slot + 12 + i] = x[i] as f32;
                data[slot + 16 + i] = y[i] as f32
            }
            data[slot + 11] = (projection.radius() + 0.01) as f32;
            data[slot + 15] = 1.0;
            data[slot + 3] = 1.0;
        }
        Ok(Self {
            projection,
            grid: [min[0] as i32, min[1] as i32, width as i32, height as i32],
            records,
            data,
            last_step: None,
            revision: 1,
            active: false,
        })
    }
    pub fn projection(&self) -> BodyProjection {
        self.projection
    }
    pub fn grid(&self) -> [i32; 4] {
        self.grid
    }
    pub fn texture_extent(&self) -> [u32; 2] {
        [
            (self.grid[2] as usize * TEXELS_PER_TILE) as u32,
            self.grid[3] as u32,
        ]
    }
    pub fn data(&self) -> &[f32] {
        &self.data
    }
    pub fn revision(&self) -> u64 {
        self.revision
    }
    pub fn last_step(&self) -> Option<u64> {
        self.last_step
    }
    pub fn active(&self) -> bool {
        self.active
    }
    pub fn state(&self, p: AxialPoint) -> Option<HighlightState> {
        self.records.get(&p).map(|r| r.state)
    }
    pub fn coordinates(&self) -> impl Iterator<Item = AxialPoint> + '_ {
        self.records.keys().copied()
    }
    pub fn set(&mut self, p: AxialPoint, state: HighlightState) -> Result<(), HighlightError> {
        if !state.valid() {
            return Err(HighlightError::NonFinite);
        }
        self.records
            .get_mut(&p)
            .ok_or(HighlightError::UnknownTile)?
            .state = state;
        Ok(())
    }
    pub fn set_visibility(&mut self, p: AxialPoint, visibility: f64) -> Result<(), HighlightError> {
        if !visibility.is_finite() || !(0.0..=1.0).contains(&visibility) {
            return Err(HighlightError::NonFinite);
        }
        self.records
            .get_mut(&p)
            .ok_or(HighlightError::UnknownTile)?
            .visibility = visibility;
        Ok(())
    }
    pub fn set_entrance_scale(&mut self, p: AxialPoint, scale: f64) -> Result<(), HighlightError> {
        if !scale.is_finite() || !(0.0..=1.0).contains(&scale) {
            return Err(HighlightError::NonFinite);
        }
        self.records
            .get_mut(&p)
            .ok_or(HighlightError::UnknownTile)?
            .entrance = scale;
        Ok(())
    }
    /// Call with the existing canonical world step, including after a paused
    /// frame. Repeated steps do not advance fades or manufacture another clock.
    pub fn tick(&mut self, step: u64, hover_allowed: bool) -> Result<bool, HighlightError> {
        if self.last_step.is_some_and(|old| step < old) {
            return Err(HighlightError::ClockRegressed);
        }
        let delta = self
            .last_step
            .map_or(0.0, |old| (step - old) as f64 / 60.0)
            .min(0.1);
        self.last_step = Some(step);
        let mut dirty = false;
        self.active = false;
        for record in self.records.values_mut() {
            if !hover_allowed {
                record.state.hovered = false
            }
            let hover = if record.state.hovered || record.state.inspected {
                0.3
            } else {
                0.0
            };
            record.hover = fade(record.hover, hover, delta, 9.75);
            record.vp = fade(
                record.vp,
                record.state.vp_intensity,
                delta,
                if record.state.vp_intensity > record.vp {
                    5.0
                } else {
                    2.45
                },
            );
            let placement = u8::from(record.state.available)
                + u8::from(record.state.available && record.state.selected);
            let values = [
                record.hover,
                placement as f64,
                record.vp,
                record.visibility * record.entrance,
                record.state.vp_color[0],
                record.state.vp_color[1],
                record.state.vp_color[2],
            ];
            for (i, v) in values.into_iter().enumerate() {
                let n = v as f32;
                let d = &mut self.data[record.slot + i];
                if d.to_bits() != n.to_bits() {
                    *d = n;
                    dirty = true
                }
            }
            let scale = record.entrance as f32;
            let d = &mut self.data[record.slot + 15];
            if d.to_bits() != scale.to_bits() {
                *d = scale;
                dirty = true
            }
            let hovered = f32::from(u8::from(record.state.hovered));
            let d = &mut self.data[record.slot + 19];
            if d.to_bits() != hovered.to_bits() {
                *d = hovered;
                dirty = true;
            }
            self.active |= record.visibility * record.entrance > 0.002
                && (record.hover > 0.0 || record.state.available || record.vp > 0.0);
        }
        if dirty {
            self.revision = self
                .revision
                .checked_add(1)
                .ok_or(HighlightError::RevisionExhausted)?
        }
        Ok(dirty)
    }
    /// Same projection and cube tie handling as the shared scalar provider.
    pub fn coordinate_at(&self, body_local: [f64; 3]) -> Option<AxialPoint> {
        if !body_local.iter().all(|n| n.is_finite()) || body_local[2] <= 0.0 {
            return None;
        }
        let p = self.projection.unproject(body_local)?;
        let coordinate = round_cube(self.projection.board_to_hex(p).ok()?).ok()?;
        self.records.contains_key(&coordinate).then_some(coordinate)
    }
}
fn fade(current: f64, target: f64, delta: f64, rate: f64) -> f64 {
    let next = current + (target - current) * (1.0 - (-rate * delta).exp());
    if (next - target).abs() < 0.002 {
        target
    } else {
        next
    }
}
/// Shortest-arc rotation from +Z to the actual sphere normal, matching Three's
/// setFromUnitVectors, including its deterministic antipodal branch.
pub fn tangent_basis(n: [f64; 3]) -> ([f64; 3], [f64; 3]) {
    let d = 1.0 + n[2];
    if d < 1e-12 {
        return ([-1.0, 0.0, 0.0], [0.0, 1.0, 0.0]);
    }
    (
        [1.0 - n[0] * n[0] / d, -n[0] * n[1] / d, -n[0]],
        [-n[0] * n[1] / d, 1.0 - n[1] * n[1] / d, -n[1]],
    )
}
