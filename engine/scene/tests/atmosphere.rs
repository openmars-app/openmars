//! Invalid atmosphere inputs must not reach GPU uniforms.
use openmars_scene::atmosphere::{self, Body};

#[test]
fn invalid_profiles_and_views_never_reach_gpu_uniforms() {
    for sky in [f64::NAN, -1., 1.01, f64::INFINITY] {
        assert!(atmosphere::mars_profile(sky, 0.).is_err());
    }
    assert!(atmosphere::mars_profile(0., f64::NAN).is_err());
    assert!(atmosphere::visibility(0., -3., 1., 720.).is_err());
    assert!(atmosphere::visibility(2., -3., 1., f64::INFINITY).is_err());
    let mut p = atmosphere::mars_profile(0., 0.).unwrap();
    p.thickness = 0.;
    assert!(p.validate().is_err());
    let b = Body {
        center: [0.; 3],
        radius: -1.,
        profile: atmosphere::mars_profile(0., 0.).unwrap(),
        follows_mesh: false,
    };
    assert!(b.validate().is_err());
    assert_eq!(atmosphere::MAX_BODIES, 16);
}
