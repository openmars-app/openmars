use openmars_scene::{
    geometry::{BoardPoint, BodyProjection, terrain::ReliefMap},
    landscape::*,
};
use std::sync::Arc;
fn source(coordinate: [i32; 3], kind: LandscapeKind) -> LandscapeSource {
    LandscapeSource {
        coordinate,
        kind,
        seed: 17,
        layout: None,
    }
}
fn input(seed: u32, sources: Vec<LandscapeSource>) -> LandscapeInput {
    LandscapeInput::new(BodyId::Mars, BodyProjection::MARS, 3, seed, None, sources).unwrap()
}
#[test]
fn source_order_is_deterministic_seed_wraps_and_inputs_are_bounded() {
    let sources = vec![
        source([0, 0, 0], LandscapeKind::Greenery),
        source([1, 0, -1], LandscapeKind::Ocean),
        source([0, 1, -1], LandscapeKind::SpecialGreenery),
    ];
    let a = LandscapeSampler::new(input(u32::MAX, sources.clone()));
    let mut reversed = sources;
    reversed.reverse();
    let b = LandscapeSampler::new(input(u32::MAX, reversed));
    for x in -10..=10 {
        for y in -10..=10 {
            let p = BoardPoint {
                x: f64::from(x) * 0.025,
                y: f64::from(y) * 0.025,
            };
            let (a, b) = (a.sample(p, true).unwrap(), b.sample(p, true).unwrap());
            assert_eq!(a.height.to_bits(), b.height.to_bits());
            assert_eq!(a.coverage.to_bits(), b.coverage.to_bits());
            assert_eq!(a.source, b.source);
            for v in [
                a.coverage,
                a.foundation,
                a.moisture,
                a.dry,
                a.lush,
                a.litter,
                a.mud,
                a.meadow,
                a.mound,
                a.greenery,
            ] {
                assert!((0.0..=1.0).contains(&v), "{v}")
            }
        }
    }
    assert!(
        LandscapeInput::new(
            BodyId::Mars,
            BodyProjection::MARS,
            0,
            0,
            None,
            vec![source([0, 0, 1], LandscapeKind::Ocean)]
        )
        .is_err()
    );
    assert!(
        LandscapeInput::new(
            BodyId::Mars,
            BodyProjection::MARS,
            0,
            0,
            None,
            vec![source([0, 0, 0], LandscapeKind::Ocean); 2]
        )
        .is_err()
    );
    assert!(
        LandscapeInput::new(
            BodyId::Mars,
            BodyProjection::MARS,
            0,
            0,
            None,
            vec![source([0, 0, 0], LandscapeKind::Ocean); 513]
        )
        .is_err()
    );
    assert!(
        a.sample(
            BoardPoint {
                x: f64::NAN,
                y: 0.0
            },
            false
        )
        .is_err()
    );
}
#[test]
fn lake_neighbor_triangle_has_no_dry_center_and_stays_below_water() {
    let coords = [[0, 0, 0], [1, 0, -1], [0, 1, -1]];
    let sampler = LandscapeSampler::new(input(
        71,
        coords.map(|c| source(c, LandscapeKind::Ocean)).to_vec(),
    ));
    let points = coords.map(|c| {
        BodyProjection::MARS.hex_center(openmars_scene::geometry::AxialPoint { q: c[0], r: c[1] })
    });
    let center = BoardPoint {
        x: points.iter().map(|p| p.x).sum::<f64>() / 3.0,
        y: points.iter().map(|p| p.y).sum::<f64>() / 3.0,
    };
    let sample = sampler.sample(center, false).unwrap();
    assert!(sample.shore < -WATER_RADIUS);
    assert!(sample.height < WATER_LEVEL);
    assert!(sample.blocked);
}
#[test]
fn relief_identity_is_content_based_and_exclusions_keep_their_own_ground() {
    let a = ReliefMap::new(4, 2, vec![0.012; 8]).unwrap();
    let b = ReliefMap::new(4, 2, vec![0.012; 8]).unwrap();
    let c = ReliefMap::new(4, 2, vec![0.013; 8]).unwrap();
    assert_eq!(a.content_hash(), b.content_hash());
    assert_ne!(a.content_hash(), c.content_hash());
    let mut i = input(31, vec![source([0, 0, 0], LandscapeKind::Excluded)]);
    i.relief = Some(Arc::new(a));
    let sampler = LandscapeSampler::new(i);
    let s = sampler.sample(BoardPoint { x: 0.0, y: 0.0 }, true).unwrap();
    assert!(s.blocked);
    assert_eq!(s.coverage, 0.0);
    assert_eq!(s.greenery, 0.0);
    assert_eq!(s.forest, 0.0);
    assert!((s.height - 0.0003).abs() < 1e-12);
}
