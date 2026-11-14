//! Resolved layout predicates used by ground preparation, not structure generation.
use super::{fields::hex_distance, planar_length};
use crate::structure::*;

pub(super) fn wall_apothem(l: &StructureLayout) -> Option<f64> {
    if l.style.landmark == StructureLandmark::Hall {
        Some(0.127)
    } else if l.style.plan == StructurePlan::Grid
        && l.style.ground == StructureGround::Level
        && l.style.cover != StructureCover::Dome
    {
        Some(0.132)
    } else {
        None
    }
}
pub(super) fn wall(l: &StructureLayout, x: f64, y: f64) -> f64 {
    wall_apothem(l).map_or_else(
        || planar_length(x, y) - 0.121,
        |a| hex_distance(x, y, 0.0) - a,
    )
}
pub(super) fn building(b: &StructureBuilding, x: f64, y: f64, pad: f64) -> bool {
    let (dx, dy) = (x - b.x, y - b.y);
    if b.landmark && b.round {
        return planar_length(dx, dy) < b.width.max(b.depth) / 2.0 + pad;
    }
    let (s, c) = b.rotation.sin_cos();
    (dx * c + dy * s).abs() < b.width / 2.0 + pad && (-dx * s + dy * c).abs() < b.depth / 2.0 + pad
}
pub(super) fn entrance(b: &StructureBuilding, x: f64, y: f64, pad: f64) -> bool {
    let Some(e) = b.entrance_road else {
        return false;
    };
    let (dx, dy) = (e.x - b.x, e.y - b.y);
    let length = dx * dx + dy * dy;
    if length < 0.000001 {
        return false;
    }
    let t = super::clamp(((x - b.x) * dx + (y - b.y) * dy) / length);
    planar_length(x - b.x - t * dx, y - b.y - t * dy) < pad
}
// Resolved roads/pipes use their true segment length, unlike the deliberately
// softened landscape bridge distance above. Tiny authored segments stay exact.
fn resolved_segment(x: f64, y: f64, ax: f64, ay: f64, bx: f64, by: f64) -> f64 {
    let (dx, dy) = (bx - ax, by - ay);
    let length = dx * dx + dy * dy;
    let t = if length > 0.0 {
        super::clamp(((x - ax) * dx + (y - ay) * dy) / length)
    } else {
        0.0
    };
    planar_length(x - ax - t * dx, y - ay - t * dy)
}
pub(super) fn road(l: &StructureLayout, x: f64, y: f64) -> (f64, f64) {
    let mut nearest = (f64::INFINITY, 0.0);
    for r in &l.roads {
        for s in r.points.windows(2) {
            let distance = resolved_segment(x, y, s[0].x, s[0].y, s[1].x, s[1].y);
            if distance - r.width / 2.0 < nearest.0 - nearest.1 {
                nearest = (distance, r.width / 2.0)
            }
        }
    }
    nearest
}
pub(super) fn inside(l: &StructureLayout, x: f64, y: f64, pad: f64) -> bool {
    if let Some(a) = wall_apothem(l)
        && l.style.details.contains(&StructureDetail::Gatehouses)
        && l.entrances.iter().any(|angle| {
            let (s, c) = angle.sin_cos();
            (x * c + y * s - (a - 0.007)).abs() < 0.012 + pad
                && (-x * s + y * c).abs() < 0.028 + pad
        })
    {
        return true;
    }
    l.buildings
        .iter()
        .any(|b| building(b, x, y, pad) || entrance(b, x, y, pad))
        || l.fixtures
            .iter()
            .any(|f| planar_length(f.x - x, f.y - y) < f.radius + pad)
        || l.pipes.iter().any(|p| {
            p.points
                .windows(2)
                .any(|s| resolved_segment(x, y, s[0].x, s[0].y, s[1].x, s[1].y) < p.radius + pad)
        })
}

#[cfg(test)]
mod tests {
    #[test]
    fn tiny_resolved_segments_preserve_midpoint_contact_and_zero_length_is_finite() {
        assert_eq!(
            super::resolved_segment(0.0000005, 0.0, 0.0, 0.0, 0.000001, 0.0),
            0.0
        );
        assert_eq!(super::resolved_segment(3.0, 4.0, 0.0, 0.0, 0.0, 0.0), 5.0);
    }
}
