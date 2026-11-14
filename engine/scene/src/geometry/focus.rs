//! The source compact placement framing policy, in the camera's local frame.
use super::CameraFrame;

#[derive(Clone, Copy, Debug)]
pub struct PlacementFocus {
    pub direction: [f64; 3],
    pub distance: f64,
}

pub fn placement_focus(
    points: &[[f64; 3]],
    frame: CameraFrame,
    height: f64,
    min: f64,
    max: f64,
) -> Option<PlacementFocus> {
    if !height.is_finite()
        || height <= 0.
        || !min.is_finite()
        || !max.is_finite()
        || min < 0.
        || min > max
        || points.iter().flatten().any(|v| !v.is_finite())
    {
        return None;
    }
    let sum: [f64; 3] = std::array::from_fn(|axis| points.iter().map(|p| p[axis]).sum());
    let length_squared = sum.iter().map(|v| v * v).sum::<f64>();
    if length_squared < 1e-8 {
        return None;
    }
    let direction = sum.map(|v| v / length_squared.sqrt());
    let tan_limit =
        (frame.fov_radians / 2.).tan() * frame.available_width.min(frame.available_height) / height
            * 0.8;
    if !tan_limit.is_finite() || tan_limit <= 0. {
        return None;
    }
    let mut distance: f64 = 0.;
    for point in points {
        let depth: f64 = point.iter().zip(direction).map(|(p, d)| p * d).sum();
        let lateral = point
            .iter()
            .zip(direction)
            .map(|(p, d)| (p - d * depth).powi(2))
            .sum::<f64>()
            .sqrt();
        distance = distance.max(depth + (lateral + 0.17) / tan_limit);
    }
    Some(PlacementFocus {
        direction,
        distance: distance.clamp(min, max),
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::geometry::{HudInsets, camera_frame};
    #[test]
    fn source_margin_fits_every_point_and_hex_extent_inside_the_hud() {
        let points = [[-0.7, 0.2, 1.9], [0.9, 0.5, 1.8], [0.2, -0.4, 2.]];
        for (width, height, insets) in [
            (
                750.,
                340.,
                HudInsets {
                    top: 40.,
                    bottom: 48.,
                    left: 88.,
                    right: 88.,
                },
            ),
            (863., 360., HudInsets::default()),
        ] {
            let frame = camera_frame(width, height, insets, true).unwrap();
            let focus = placement_focus(&points, frame, height, 0., 100.).unwrap();
            let tan = (frame.fov_radians / 2.).tan()
                * frame.available_width.min(frame.available_height)
                / height;
            for point in points {
                let depth: f64 = point.iter().zip(focus.direction).map(|(p, d)| p * d).sum();
                let lateral = point
                    .iter()
                    .zip(focus.direction)
                    .map(|(p, d)| (p - d * depth).powi(2))
                    .sum::<f64>()
                    .sqrt();
                assert!((lateral + 0.17) / (focus.distance - depth) <= tan * 0.8 + 1e-12);
            }
        }
    }
    #[test]
    fn source_orbit_limits_and_invalid_directions_are_preserved() {
        let frame = camera_frame(750., 340., HudInsets::default(), true).unwrap();
        assert_eq!(
            placement_focus(&[[0., 0., 2.]], frame, 340., 4., 18.)
                .unwrap()
                .distance,
            4.
        );
        assert_eq!(
            placement_focus(&[[100., 0., 2.]], frame, 340., 4., 18.)
                .unwrap()
                .distance,
            18.
        );
        for points in [
            vec![],
            vec![[0., 0., 1.], [0., 0., -1.]],
            vec![[f64::NAN, 0., 1.]],
        ] {
            assert!(placement_focus(&points, frame, 340., 4., 18.).is_none());
        }
        assert!(placement_focus(&[[0., 0., 2.]], frame, 0., 4., 18.).is_none());
    }
}
