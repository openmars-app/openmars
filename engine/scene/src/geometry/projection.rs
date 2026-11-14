// Adapted from the sibling renderer’s pure geometry module.
use super::Ray3;
use std::{f64::consts::PI, fmt};

pub const HEX_SIZE: f64 = 0.3;
pub const BOARD_SCALE: f64 = 0.4 * PI / 2.0;
pub const MARS_RADIUS: f64 = 2.02;
pub const PICK_SHELL: f64 = 0.02;
pub const HEX_HIT_RADIUS: f64 = 0.17;
const ORIGIN_EPSILON: f64 = 1e-10;

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum GeometryError {
    NonFinite,
    InvalidRadius,
    InvalidOrientation,
    CoordinateOverflow,
    NotCube,
    InvalidHeight,
}
impl fmt::Display for GeometryError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{self:?}")
    }
}
impl std::error::Error for GeometryError {}

/// Normalized axial math input; this is not an authoritative game coordinate.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub struct AxialPoint {
    pub q: i32,
    pub r: i32,
}
impl AxialPoint {
    pub fn cube(self) -> Result<[i32; 3], GeometryError> {
        let s = i32::try_from(-i64::from(self.q) - i64::from(self.r))
            .map_err(|_| GeometryError::CoordinateOverflow)?;
        Ok([self.q, self.r, s])
    }
    pub fn from_cube([q, r, s]: [i32; 3]) -> Result<Self, GeometryError> {
        if i64::from(q) + i64::from(r) + i64::from(s) != 0 {
            return Err(GeometryError::NotCube);
        }
        Ok(Self { q, r })
    }
}

/// Arc lengths on the board plane, in world units, before spherical projection.
#[derive(Clone, Copy, Debug, Default, PartialEq, serde::Serialize, serde::Deserialize)]
pub struct BoardPoint {
    pub x: f64,
    pub y: f64,
}
impl BoardPoint {
    fn validate(self) -> Result<Self, GeometryError> {
        if !self.x.is_finite() || !self.y.is_finite() {
            return Err(GeometryError::NonFinite);
        }
        Ok(self)
    }
}

/// Return the integral cube containing a fractional cube point.
/// Half-integer rounding follows JavaScript Math.round (ties toward +infinity).
/// Equal component errors follow the reference q/r/s correction precedence.
pub fn round_cube(cube: [f64; 3]) -> Result<AxialPoint, GeometryError> {
    if !cube.iter().all(|v| v.is_finite()) {
        return Err(GeometryError::NonFinite);
    }
    let sum = cube[0] + cube[1] + cube[2];
    let scale = cube.iter().map(|v| v.abs()).fold(1.0_f64, f64::max);
    if !sum.is_finite() || sum.abs() > 8.0 * f64::EPSILON * scale {
        return Err(GeometryError::NotCube);
    }
    let mut rounded = cube.map(|v| {
        let floor = v.floor();
        if v - floor >= 0.5 { floor + 1.0 } else { floor }
    });
    let error = std::array::from_fn::<_, 3, _>(|i| (rounded[i] - cube[i]).abs());
    let corrected = if error[0] > error[1] && error[0] > error[2] {
        0
    } else if error[1] > error[2] {
        1
    } else {
        2
    };
    rounded[corrected] = -rounded[(corrected + 1) % 3] - rounded[(corrected + 2) % 3];
    if rounded
        .iter()
        .any(|v| *v < f64::from(i32::MIN) || *v > f64::from(i32::MAX))
    {
        return Err(GeometryError::CoordinateOverflow);
    }
    AxialPoint::from_cube(rounded.map(|v| v as i32))
}

/// A body's independent tile and content spheres. No global Mars-radius fallback.
/// `y_sign` is -1 for the Mars grid and +1 for the reference celestial grid.
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct BodyProjection {
    radius: f64,
    content_radius: f64,
    y_sign: i8,
    coord_offset: AxialPoint,
}
impl BodyProjection {
    pub const MARS: Self = Self {
        radius: MARS_RADIUS,
        content_radius: MARS_RADIUS,
        y_sign: -1,
        coord_offset: AxialPoint { q: 0, r: 0 },
    };
    pub fn new(
        radius: f64,
        content_radius: f64,
        y_sign: i8,
        coord_offset: AxialPoint,
    ) -> Result<Self, GeometryError> {
        if !radius.is_finite() || !content_radius.is_finite() {
            return Err(GeometryError::NonFinite);
        }
        if radius <= 0.0 || content_radius <= 0.0 {
            return Err(GeometryError::InvalidRadius);
        }
        if y_sign != -1 && y_sign != 1 {
            return Err(GeometryError::InvalidOrientation);
        }
        Ok(Self {
            radius,
            content_radius,
            y_sign,
            coord_offset,
        })
    }
    pub fn radius(self) -> f64 {
        self.radius
    }
    pub fn content_radius(self) -> f64 {
        self.content_radius
    }
    pub fn y_sign(self) -> i8 {
        self.y_sign
    }
    pub fn coord_offset(self) -> AxialPoint {
        self.coord_offset
    }

    pub fn hex_center(self, point: AxialPoint) -> BoardPoint {
        // Widen before subtracting so offsets at opposite i32 extrema cannot wrap.
        let q = (i64::from(point.q) - i64::from(self.coord_offset.q)) as f64;
        let r = (i64::from(point.r) - i64::from(self.coord_offset.r)) as f64;
        BoardPoint {
            x: HEX_SIZE * 3.0_f64.sqrt() * (q + r / 2.0) * BOARD_SCALE,
            y: f64::from(self.y_sign) * HEX_SIZE * 1.5 * r * BOARD_SCALE,
        }
    }
    pub fn project(self, point: BoardPoint, height: f64) -> Result<[f64; 3], GeometryError> {
        project_radius(point, height, self.radius)
    }
    pub fn project_content(
        self,
        point: BoardPoint,
        height: f64,
    ) -> Result<[f64; 3], GeometryError> {
        project_radius(point, height, self.content_radius)
    }
    pub fn normal(self, point: BoardPoint) -> Result<[f64; 3], GeometryError> {
        Ok(self.project(point, 0.0)?.map(|v| v / self.radius))
    }
    /// Principal inverse (angular distance 0..pi). The exact antipode chooses +X
    /// deterministically; a spherical map cannot distinguish antipodal azimuths.
    pub fn unproject(self, direction: [f64; 3]) -> Option<BoardPoint> {
        let n = super::ray::unit(direction)?;
        let planar = n[0].hypot(n[1]);
        let point = if planar == 0.0 {
            BoardPoint {
                x: if n[2] < 0.0 { PI * self.radius } else { 0.0 },
                y: 0.0,
            }
        } else {
            let distance = planar.atan2(n[2]) * self.radius;
            BoardPoint {
                x: n[0] / planar * distance,
                y: n[1] / planar * distance,
            }
        };
        point.validate().ok()
    }
    /// Fractional cube tuple in absolute board coordinates (offset restored).
    pub fn board_to_hex(self, point: BoardPoint) -> Result<[f64; 3], GeometryError> {
        point.validate()?;
        let local_r = point.y * f64::from(self.y_sign) / (HEX_SIZE * 1.5 * BOARD_SCALE);
        let r = local_r + f64::from(self.coord_offset.r);
        let q = point.x / (HEX_SIZE * 3.0_f64.sqrt() * BOARD_SCALE) - local_r / 2.0
            + f64::from(self.coord_offset.q);
        let cube = [q, r, -q - r];
        if !cube.iter().all(|v| v.is_finite()) {
            return Err(GeometryError::NonFinite);
        }
        Ok(cube)
    }
    /// The nearest tile center strictly within the frozen 3-D hit radius.
    /// Exact distance ties choose the smallest (q,r), independent of input order.
    pub fn nearest_tile(self, hit: [f64; 3], tiles: &[AxialPoint]) -> Option<AxialPoint> {
        if !hit.iter().all(|v| v.is_finite()) {
            return None;
        }
        let mut best = None;
        let mut best_distance = HEX_HIT_RADIUS;
        for &tile in tiles {
            let center = self.project(self.hex_center(tile), 0.0).ok()?;
            let distance = (hit[0] - center[0])
                .hypot(hit[1] - center[1])
                .hypot(hit[2] - center[2]);
            if distance < best_distance
                || (distance == best_distance && best.is_some_and(|current| tile < current))
            {
                best = Some(tile);
                best_distance = distance;
            }
        }
        best
    }
    pub fn pick(self, ray: Ray3, tiles: &[AxialPoint]) -> Option<AxialPoint> {
        self.nearest_tile(ray.intersect_sphere(self.radius + PICK_SHELL)?.point, tiles)
    }
}

fn project_radius(
    point: BoardPoint,
    height: f64,
    surface_radius: f64,
) -> Result<[f64; 3], GeometryError> {
    point.validate()?;
    if !height.is_finite() {
        return Err(GeometryError::NonFinite);
    }
    let radius = surface_radius + height;
    if !radius.is_finite() {
        return Err(GeometryError::NonFinite);
    }
    if radius <= 0.0 {
        return Err(GeometryError::InvalidHeight);
    }
    let d = point.x.hypot(point.y);
    if !d.is_finite() {
        return Err(GeometryError::NonFinite);
    }
    if d < ORIGIN_EPSILON {
        return Ok([0.0, 0.0, radius]);
    }
    let angle = d / surface_radius;
    if !angle.is_finite() {
        return Err(GeometryError::NonFinite);
    }
    let (sin, cos) = angle.sin_cos();
    let result = [
        point.x / d * sin * radius,
        point.y / d * sin * radius,
        cos * radius,
    ];
    if !result.iter().all(|v| v.is_finite()) {
        return Err(GeometryError::NonFinite);
    }
    Ok(result)
}
