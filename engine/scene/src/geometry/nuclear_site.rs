//! Original projected nearest-tile coverage for crater aprons and debris.
use super::{
    AxialPoint, BoardPoint, BodyProjection, GeometryError,
    nuclear::{GroundSample, NuclearProfile},
    nuclear_mesh::TileFrame,
};

#[derive(Clone, Debug)]
pub struct BoardFrame {
    pub coordinate: [i32; 3],
    pub allowed: bool,
    pub normal: [f64; 3],
    pub origin: [f64; 3],
    pub rotation: [f64; 4],
}
fn dot(a: [f64; 3], b: [f64; 3]) -> f64 {
    a.into_iter().zip(b).map(|(a, b)| a * b).sum()
}
fn subtract(a: [f64; 3], b: [f64; 3]) -> [f64; 3] {
    std::array::from_fn(|i| a[i] - b[i])
}
fn cross(a: [f64; 3], b: [f64; 3]) -> [f64; 3] {
    [
        a[1] * b[2] - a[2] * b[1],
        a[2] * b[0] - a[0] * b[2],
        a[0] * b[1] - a[1] * b[0],
    ]
}
fn rotate(q: [f64; 4], v: [f64; 3]) -> [f64; 3] {
    let xyz = [q[0], q[1], q[2]];
    let t = cross(xyz, v).map(|v| v * 2.);
    let c = cross(xyz, t);
    std::array::from_fn(|i| v[i] + q[3] * t[i] + c[i])
}
impl BoardFrame {
    pub fn new(
        projection: BodyProjection,
        coordinate: [i32; 3],
        allowed: bool,
    ) -> Result<Self, GeometryError> {
        let p = projection.hex_center(AxialPoint::from_cube(coordinate)?);
        let normal = projection.normal(p)?;
        let xyz = cross([0., 0., 1.], normal);
        let w = 1. + normal[2];
        // The map lives on the principal front hemisphere; retain an explicit
        // antipode solution for the complete public projection contract.
        let rotation = if w < 1e-12 {
            [1., 0., 0., 0.]
        } else {
            let l = (dot(xyz, xyz) + w * w).sqrt();
            [xyz[0] / l, xyz[1] / l, xyz[2] / l, w / l]
        };
        let origin = normal.map(|n| n * (projection.radius() + 0.01));
        TileFrame::new(origin, rotation)?;
        Ok(Self {
            coordinate,
            allowed,
            normal,
            origin,
            rotation,
        })
    }
    pub fn tile_frame(&self) -> Result<TileFrame, GeometryError> {
        TileFrame::new(self.origin, self.rotation)
    }
    pub fn direction(&self, x: f64, y: f64) -> Result<[f64; 3], GeometryError> {
        self.tile_frame()?.direction(x, y)
    }
    pub fn plane_point(&self, direction: [f64; 3], radius: f64) -> Result<[f64; 3], GeometryError> {
        let facing = dot(direction, self.normal);
        if facing <= 1e-10 {
            return Err(GeometryError::InvalidOrientation);
        }
        let p = subtract(direction.map(|n| n * (radius + 0.01) / facing), self.origin);
        Ok(rotate(
            [
                -self.rotation[0],
                -self.rotation[1],
                -self.rotation[2],
                self.rotation[3],
            ],
            p,
        ))
    }
}
/// Borrowed only: no detached landscape outlives the actual snapshot charge.
/// All frame/nearby vectors are admitted by the scene before constructing sites.
pub struct CraterGround<'a> {
    pub frame: &'a BoardFrame,
    pub nearby: &'a [BoardFrame],
    pub radius: f64,
    pub terrain: &'a dyn Fn(BoardPoint) -> Result<f32, GeometryError>,
}
impl CraterGround<'_> {
    pub fn sample(&self, x: f64, y: f64) -> Result<GroundSample, GeometryError> {
        let direction = self.frame.direction(x, y)?;
        let mut clearance = f64::NEG_INFINITY;
        let mut owner = self.frame;
        for other in self.nearby {
            let local = other.plane_point(direction, self.radius)?;
            let edge = 0.166 * 3_f64.sqrt() / 2.
                - local[0]
                    .abs()
                    .max((local[0] * 0.5 + local[1] * 3_f64.sqrt() / 2.).abs())
                    .max((-local[0] * 0.5 + local[1] * 3_f64.sqrt() / 2.).abs());
            if edge > clearance {
                clearance = edge;
                owner = other;
            }
        }
        if !clearance.is_finite() {
            return Err(GeometryError::InvalidHeight);
        }
        let d = direction[0].hypot(direction[1]);
        let scale = if d > 1e-10 {
            direction[2].clamp(-1., 1.).acos() * self.radius / d
        } else {
            0.
        };
        let point = BoardPoint {
            x: direction[0] * scale,
            y: direction[1] * scale,
        };
        let height = (self.terrain)(point)?;
        if !height.is_finite() {
            return Err(GeometryError::NonFinite);
        }
        let allowed = owner.coordinate == self.frame.coordinate || owner.allowed;
        let t = ((clearance + 0.006) / 0.018).clamp(0., 1.);
        let coverage = if allowed && height > -0.008 {
            t * t * (3. - 2. * t)
        } else {
            0.
        };
        GroundSample {
            height: f64::from(height),
            coverage,
            border_distance: clearance.abs(),
        }
        .validate()
    }
    pub fn projected_height(
        &self,
        profile: &NuclearProfile,
        x: f64,
        y: f64,
    ) -> Result<f64, GeometryError> {
        profile.ground_height(x, y, |x, y| self.sample(x, y))
    }
    pub fn projected_normal(
        &self,
        profile: &NuclearProfile,
        x: f64,
        y: f64,
    ) -> Result<[f64; 3], GeometryError> {
        let point = |x, y| {
            let d = self.frame.direction(x, y)?;
            let h = self.projected_height(profile, x, y)?;
            Ok::<_, GeometryError>(d.map(|v| v * (self.radius + h)))
        };
        let tx = subtract(point(x + 0.0005, y)?, point(x - 0.0005, y)?);
        let ty = subtract(point(x, y + 0.0005)?, point(x, y - 0.0005)?);
        let n = cross(tx, ty);
        let length = dot(n, n).sqrt();
        if !length.is_finite() || length < 1e-12 {
            return Err(GeometryError::InvalidOrientation);
        }
        Ok(n.map(|v| v / length))
    }
}
