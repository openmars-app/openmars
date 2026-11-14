//! Terrain bounds, contact, and climate behavior.
use openmars_scene::{
    climate::*,
    geometry::{
        BoardPoint, BodyProjection,
        terrain::{FIELD_SIZE, ReliefMap, SurfaceClass, TerrainError, TerrainField, TerrainPatch},
    },
};
fn close(a: f64, b: f64, tolerance: f64) {
    assert!((a - b).abs() <= tolerance, "{a} != {b}");
}
fn relief() -> ReliefMap {
    let rgba: Vec<u8> = (0..16).flat_map(|i| [i * 13, i * 7, i * 3, 255]).collect();
    ReliefMap::from_rgba(4, 4, &rgba).unwrap()
}

#[test]
fn relief_is_bounded_continuous_at_the_seam_and_equal_at_each_pole() {
    let map = relief();
    for pole in [0.0, 1.0] {
        assert_eq!(map.sample_uv(0.1, pole), map.sample_uv(0.9, pole));
    }
    close(
        map.sample_uv(-1e-9, 0.5).unwrap(),
        map.sample_uv(1e-9, 0.5).unwrap(),
        1e-9,
    );
    assert_eq!(map.content_hash(), relief().content_hash());
    assert!(map.sample_direction([0.0; 3]).is_err());
    assert!(ReliefMap::from_rgba(usize::MAX, 2, &[]).is_err());
    assert!(ReliefMap::new(2, 2, vec![0.0, 0.01, 0.0, 0.0]).is_err());
}
fn flat_patch() -> TerrainPatch {
    let rgba: Vec<_> = (0..FIELD_SIZE * FIELD_SIZE)
        .flat_map(|_| [0x2400, 0x3000, 0, 0])
        .collect();
    TerrainPatch::from_half_rgba([0, 0], &rgba, SurfaceClass::Ground).unwrap()
}
#[test]
fn prepared_field_contact_uses_the_actual_body_radius_and_half_float_height() {
    for radius in [0.27, 2.02, 7.0] {
        let projection = BodyProjection::new(radius, radius, -1, Default::default()).unwrap();
        let field = TerrainField::new(projection, 7, 42, None, vec![flat_patch()]).unwrap();
        let point = BoardPoint { x: 0.05, y: 0.08 };
        let sample = field.sample(point).unwrap();
        close(sample.height.into(), 1.0 / 64.0, 1e-9);
        close(sample.shore.into(), 0.125, 1e-9);
        let contact = field.contact(point).unwrap();
        close(
            contact[0].hypot(contact[1]).hypot(contact[2]),
            radius + f64::from(sample.height),
            2e-15,
        );
        assert_eq!((field.revision(), field.seed()), (7, 42));
    }
}
#[test]
fn prepared_fields_reject_duplicate_patches_and_nonfinite_binary16_inputs() {
    assert!(
        TerrainField::new(
            BodyProjection::MARS,
            0,
            0,
            None,
            vec![flat_patch(), flat_patch()]
        )
        .is_err()
    );
    let mut rgba = vec![0; FIELD_SIZE * FIELD_SIZE * 4];
    rgba[0] = 0x7c00;
    assert!(TerrainPatch::from_half_rgba([0, 0], &rgba, SurfaceClass::Ground).is_err());
    rgba[0] = 0x7e00;
    assert!(TerrainPatch::from_half_rgba([0, 0], &rgba, SurfaceClass::Ground).is_err());
}
#[test]
fn water_changes_keep_ground_cache_channels_while_frost_changes_invalidate_them() {
    let eight = ClimateParameters {
        oceans: 8.,
        ..ClimateParameters::TERRAFORMED
    };
    let mut climate = ClimateRuntime::new(eight, 0).unwrap();
    climate.retarget(ClimateParameters::TERRAFORMED, 0).unwrap();
    assert_ne!(climate.current().waves, climate.target().waves);
    assert_ne!(climate.current(), climate.target());
    assert_eq!(
        climate.current().ground_channels(),
        climate.target().ground_channels()
    );
    climate
        .retarget(
            ClimateParameters {
                temperature: -2.,
                ..ClimateParameters::TERRAFORMED
            },
            0,
        )
        .unwrap();
    assert_ne!(
        climate.current().ground_channels(),
        climate.target().ground_channels()
    );
}
#[test]
fn climate_normalizes_custom_ocean_counts_and_matches_visual_endpoints() {
    let barren = from_parameters(ClimateParameters::BARREN).unwrap();
    assert_eq!(
        (barren.frost, barren.ice, barren.tree, barren.grass),
        (1.0, 1.0, 0.0, 0.0)
    );
    let full = from_parameters(ClimateParameters::TERRAFORMED).unwrap();
    assert_eq!(
        (full.frost, full.ice, full.tree, full.foam, full.flower),
        (0.0, 0.0, 1.0, 1.0, 1.0)
    );
    let mut parameters = ClimateParameters::TERRAFORMED;
    parameters.oceans = 12.0;
    parameters.max_oceans = 12.0;
    assert_eq!(from_parameters(parameters).unwrap(), full);
    parameters.temperature = f64::NAN;
    assert_eq!(from_parameters(parameters), Err(TerrainError::NonFinite));
}
#[test]
fn climate_fixed_steps_pause_and_reconnect_snap_without_replaying_milestones() {
    let mut a = ClimateRuntime::new(ClimateParameters::BARREN, 0).unwrap();
    let mut b = ClimateRuntime::new(ClimateParameters::BARREN, 0).unwrap();
    a.retarget(ClimateParameters::TERRAFORMED, 0).unwrap();
    b.retarget(ClimateParameters::TERRAFORMED, 0).unwrap();
    a.advance(60).unwrap();
    for step in 1..=60 {
        b.advance(step).unwrap();
    }
    assert_eq!(a.current(), b.current());
    let paused = a.current();
    a.advance(60).unwrap();
    assert_eq!(a.current(), paused);
    assert!(a.pulses().warm > 0.0 && a.pulses().sweep > 0.0);
    assert!(a.advance(59).is_err());
    assert_eq!(a.current(), paused);
    a.snap(ClimateParameters::TERRAFORMED, 600).unwrap();
    assert_eq!(a.pulses(), ClimatePulses::default());
    assert_eq!(a.current(), a.target());
}
