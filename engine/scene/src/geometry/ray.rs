// Adapted from the sibling renderer’s pure geometry module.
/// Ray in body-local coordinates; direction need not be normalized.
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Ray3 {
    pub origin: [f64; 3],
    pub dir: [f64; 3],
}
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct SphereHit {
    pub point: [f64; 3],
    /// World-unit distance along the normalized forward ray, not its raw parameter.
    pub distance: f64,
}

pub(super) fn unit(value: [f64; 3]) -> Option<[f64; 3]> {
    if !value.iter().all(|v| v.is_finite()) {
        return None;
    }
    let scale = value.iter().map(|v| v.abs()).fold(0.0_f64, f64::max);
    if scale == 0.0 {
        return None;
    }
    let scaled = value.map(|v| v / scale);
    let norm = scaled[0].hypot(scaled[1]).hypot(scaled[2]);
    Some(scaled.map(|v| v / norm))
}
impl Ray3 {
    /// Analytic origin-centered intersection. Exact dyadic boundary predicates
    /// reject real misses/behind rays without an epsilon. A surface origin is
    /// the first hit at distance zero for every valid direction.
    pub fn intersect_sphere(self, radius: f64) -> Option<SphereHit> {
        use super::discriminant;
        use std::cmp::Ordering;
        if !radius.is_finite() || radius <= 0.0 || !self.origin.iter().all(|v| v.is_finite()) {
            return None;
        }
        let dir = unit(self.dir)?;
        let surface = discriminant::surface(self.origin, radius);
        if surface.sign == Ordering::Equal {
            return Some(SphereHit {
                point: self.origin,
                distance: 0.0,
            });
        }
        let (dot, direction_norm) = discriminant::along(self.origin, self.dir);
        if surface.sign == Ordering::Greater && dot.sign != Ordering::Less {
            return None;
        }
        let chord = discriminant::half_chord(self.origin, self.dir, radius)?;
        let along = dot.magnitude.divide(direction_norm);
        // C/(|along|+half_chord) is the cancellation-free small forward
        // root. An inside ray pointing inward instead uses the large exit root.
        let distance = if surface.sign == Ordering::Less && dot.sign == Ordering::Less {
            along.add(chord).value()
        } else {
            surface.magnitude.divide(along.add(chord)).value()
        };
        let side = if surface.sign == Ordering::Greater {
            -1.0
        } else {
            1.0
        };
        let closest = discriminant::closest(self.origin, self.dir);
        // The exact chord never exceeds radius; cap numerical overshoot at that
        // proved geometric bound only after the exact hit/miss classification.
        let half_chord = chord.value().min(radius);
        let point = std::array::from_fn(|i| (side * half_chord).mul_add(dir[i], closest[i]));
        if !distance.is_finite() || !point.iter().all(|v| v.is_finite()) {
            return None;
        }
        Some(SphereHit {
            point,
            distance: distance + 0.0,
        })
    }
}
