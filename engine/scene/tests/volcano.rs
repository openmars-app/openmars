use openmars_scene::geometry::volcano::{
    VOLCANO_DEFAULT, VOLCANO_FLOW_GRID, VolcanoFlowWork, VolcanoParams, volcano_flow_map,
    volcano_height,
};
#[test]
fn invalid_volcano_parameters_are_rejected() {
    for bad in [f32::NAN, f32::INFINITY, -1.] {
        assert!(
            volcano_flow_map(&VolcanoParams {
                height: bad,
                ..VOLCANO_DEFAULT
            })
            .is_err()
        );
    }
    assert!(volcano_height(1.01, 0., &VOLCANO_DEFAULT).is_err());
}
#[test]
fn incremental_flow_is_bounded_and_publishes_once() {
    for seed in [17., 42., 8191.] {
        let p = VolcanoParams {
            seed,
            ..VOLCANO_DEFAULT
        };
        let actual = volcano_flow_map(&p).unwrap();
        assert_eq!(actual.len(), VOLCANO_FLOW_GRID * VOLCANO_FLOW_GRID);
        assert!(
            actual
                .iter()
                .all(|v| v.is_finite() && (0. ..=1.).contains(v))
        );
        assert_eq!(actual.iter().copied().fold(0.0_f32, f32::max), 1.);
        let mut work = VolcanoFlowWork::new(p).unwrap();
        assert!(work.take_ready().is_none());
        let mut completed = false;
        for _ in 0..256 {
            if work.advance().unwrap() {
                completed = true;
                break;
            }
        }
        assert!(
            completed,
            "incremental preparation must finish within its work budget"
        );
        assert_eq!(actual, work.take_ready().unwrap());
        assert!(work.take_ready().is_none());
        assert!(work.advance().is_err());
    }
}
