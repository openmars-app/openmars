//! Prepared connection geometry and conservative influence bounds.
use super::planar_length;
use crate::geometry::{
    BoardPoint,
    terrain::{FIELD_BORDER, FIELD_STEP, PATCH_SIZE},
};

#[derive(Clone, Copy)]
pub(super) struct Bounds {
    min: BoardPoint,
    max: BoardPoint,
}
impl Bounds {
    pub fn patch(key: [i32; 2]) -> Self {
        // Include both overlap texels and rounding of the shared integer lattice.
        let border = FIELD_STEP * FIELD_BORDER as f64 + 1e-9;
        Self {
            min: BoardPoint {
                x: f64::from(key[0]) * PATCH_SIZE - border,
                y: f64::from(key[1]) * PATCH_SIZE - border,
            },
            max: BoardPoint {
                x: f64::from(key[0] + 1) * PATCH_SIZE + border,
                y: f64::from(key[1] + 1) * PATCH_SIZE + border,
            },
        }
    }
    fn points(points: &[BoardPoint]) -> Self {
        Self {
            min: BoardPoint {
                x: points.iter().map(|p| p.x).fold(f64::INFINITY, f64::min),
                y: points.iter().map(|p| p.y).fold(f64::INFINITY, f64::min),
            },
            max: BoardPoint {
                x: points.iter().map(|p| p.x).fold(f64::NEG_INFINITY, f64::max),
                y: points.iter().map(|p| p.y).fold(f64::NEG_INFINITY, f64::max),
            },
        }
    }
    pub fn contains(self, point: BoardPoint, reach: f64) -> bool {
        self.overlaps(
            Self {
                min: point,
                max: point,
            },
            reach,
        )
    }
    fn overlaps(self, other: Self, reach: f64) -> bool {
        other.max.x >= self.min.x - reach
            && other.min.x <= self.max.x + reach
            && other.max.y >= self.min.y - reach
            && other.min.y <= self.max.y + reach
    }
}

#[derive(Clone, Copy)]
pub(super) struct Connection {
    pub endpoints: [usize; 2],
    start: BoardPoint,
    delta: BoardPoint,
    denominator: f64,
    bounds: Bounds,
    pub direction: [f64; 2],
    pub midpoint: f64,
}
impl Connection {
    pub fn new(endpoints: [usize; 2], a: BoardPoint, b: BoardPoint) -> Self {
        let delta = BoardPoint {
            x: b.x - a.x,
            y: b.y - a.y,
        };
        let length = planar_length(delta.x, delta.y);
        Self {
            endpoints,
            start: a,
            delta,
            denominator: (delta.x * delta.x + delta.y * delta.y).max(1e-10),
            bounds: Bounds::points(&[a, b]),
            direction: [delta.x / length, delta.y / length],
            midpoint: length / 2.0,
        }
    }
    pub fn affects(&self, bounds: Bounds, reach: f64) -> bool {
        bounds.overlaps(self.bounds, reach)
    }
    #[inline]
    pub fn distance(&self, x: f64, y: f64) -> f64 {
        let t = super::clamp(
            ((x - self.start.x) * self.delta.x + (y - self.start.y) * self.delta.y)
                / self.denominator,
        );
        planar_length(
            x - self.start.x - t * self.delta.x,
            y - self.start.y - t * self.delta.y,
        )
    }
}

#[derive(Clone, Copy)]
pub(super) struct WaterTriangle {
    pub endpoints: [usize; 3],
    points: [BoardPoint; 3],
    edges: [BoardPoint; 3],
    scale: f64,
    bounds: Bounds,
}
impl WaterTriangle {
    pub fn new(endpoints: [usize; 3], points: [BoardPoint; 3]) -> Self {
        let [a, b, c] = points;
        let edges = [
            BoardPoint {
                x: b.x - a.x,
                y: b.y - a.y,
            },
            BoardPoint {
                x: c.x - b.x,
                y: c.y - b.y,
            },
            BoardPoint {
                x: a.x - c.x,
                y: a.y - c.y,
            },
        ];
        let orientation = (edges[0].x * (c.y - a.y) - edges[0].y * (c.x - a.x)).signum();
        Self {
            endpoints,
            points,
            edges,
            scale: orientation / planar_length(edges[0].x, edges[0].y),
            bounds: Bounds::points(&points),
        }
    }
    pub fn affects(&self, bounds: Bounds) -> bool {
        // The ocean sampling position is warped by at most .04 on each axis.
        bounds.overlaps(self.bounds, 0.04)
    }
    #[inline]
    pub fn inset(&self, x: f64, y: f64) -> f64 {
        let [a, b, c] = self.points;
        let [ab, bc, ca] = self.edges;
        ((ab.x * (y - a.y) - ab.y * (x - a.x)) * self.scale)
            .min((bc.x * (y - b.y) - bc.y * (x - b.x)) * self.scale)
            .min((ca.x * (y - c.y) - ca.y * (x - c.x)) * self.scale)
    }
}
