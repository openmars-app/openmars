//! Port of the former Three.js CameraFraming policy, independent of Three.js.
#[derive(Clone, Copy, Debug, Default)]
pub struct HudInsets {
    pub top: f64,
    pub bottom: f64,
    pub left: f64,
    pub right: f64,
}

#[derive(Clone, Copy, Debug)]
pub struct CameraFrame {
    pub fov_radians: f64,
    pub offset_x: f64,
    pub offset_y: f64,
    pub available_width: f64,
    pub available_height: f64,
}

pub fn camera_frame(
    width: f64,
    height: f64,
    insets: HudInsets,
    compact: bool,
) -> Option<CameraFrame> {
    if !width.is_finite()
        || !height.is_finite()
        || width <= 0.0
        || height <= 0.0
        || [insets.top, insets.bottom, insets.left, insets.right]
            .iter()
            .any(|value| !value.is_finite() || *value < 0.0)
    {
        return None;
    }
    let available_width = (width - insets.left - insets.right).max(1.0);
    let available_height = (height - insets.top - insets.bottom).max(1.0);
    let required_tan = 25.0_f64.to_radians().tan() / if compact { 1.22 } else { 1.0 };
    let half_fov_tan =
        (required_tan * height / available_height).max(required_tan * height / available_width);
    Some(CameraFrame {
        fov_radians: (half_fov_tan.atan() * 2.0).min(100.0_f64.to_radians()),
        offset_x: (insets.left - insets.right) / 2.0,
        offset_y: (insets.top - insets.bottom) / 2.0,
        available_width,
        available_height,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn wide_desktop_preserves_reference_fov() {
        let frame = camera_frame(1600.0, 900.0, HudInsets::default(), false).unwrap();
        assert!((frame.fov_radians - 50.0_f64.to_radians()).abs() < 1e-12);
        assert_eq!((frame.offset_x, frame.offset_y), (0.0, 0.0));
    }
    #[test]
    fn asymmetric_hud_and_compact_scale_keep_visible_board_space() {
        let insets = HudInsets {
            left: 200.0,
            right: 100.0,
            top: 40.0,
            bottom: 80.0,
        };
        let desktop = camera_frame(1000.0, 600.0, insets, false).unwrap();
        let compact = camera_frame(1000.0, 600.0, insets, true).unwrap();
        assert_eq!((desktop.offset_x, desktop.offset_y), (50.0, -20.0));
        assert_eq!(
            (desktop.available_width, desktop.available_height),
            (700.0, 480.0)
        );
        assert!(desktop.fov_radians > 50.0_f64.to_radians());
        assert!(compact.fov_radians < desktop.fov_radians);
    }
    #[test]
    fn invalid_or_fully_covered_views_never_produce_nan_projection() {
        assert!(camera_frame(0.0, 600.0, HudInsets::default(), false).is_none());
        assert!(camera_frame(f64::NAN, 600.0, HudInsets::default(), false).is_none());
        let frame = camera_frame(
            200.0,
            100.0,
            HudInsets {
                left: 400.0,
                bottom: 200.0,
                ..Default::default()
            },
            false,
        )
        .unwrap();
        assert_eq!(frame.fov_radians, 100.0_f64.to_radians());
        assert_eq!((frame.available_width, frame.available_height), (1.0, 1.0));
    }
}
