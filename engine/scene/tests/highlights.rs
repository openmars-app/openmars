#[test]
fn highlight_lookup_uses_shared_seams_orientation_and_celestial_offset() {
    use openmars_scene::{
        geometry::{AxialPoint, BoardPoint, BodyProjection},
        highlights::HighlightBoard,
    };
    for projection in [
        BodyProjection::MARS,
        BodyProjection::new(0.55, 0.55, 1, AxialPoint { q: 12, r: -7 }).unwrap(),
    ] {
        let offset = projection.coord_offset();
        let a = offset;
        let b = AxialPoint {
            q: offset.q + 1,
            r: offset.r,
        };
        let board = HighlightBoard::new(projection, &[a, b]).unwrap();
        for p in [a, b] {
            let point = projection.project(projection.hex_center(p), 0.02).unwrap();
            assert_eq!(board.coordinate_at(point), Some(p));
        }
        let ca = projection.hex_center(a);
        let cb = projection.hex_center(b);
        let middle = (ca.x + cb.x) / 2.0;
        for (epsilon, expected) in [(-1e-6, a), (1e-6, b)] {
            let point = projection
                .project(
                    BoardPoint {
                        x: middle + epsilon,
                        y: 0.0,
                    },
                    0.0,
                )
                .unwrap();
            assert_eq!(board.coordinate_at(point), Some(expected));
        }
        assert_eq!(board.coordinate_at([0.0, 0.0, -projection.radius()]), None);
    }
}
#[test]
fn highlights_never_infer_available_moves_and_pause_uses_one_clock() {
    use openmars_scene::{
        geometry::{AxialPoint, BodyProjection},
        highlights::{HighlightBoard, HighlightState},
    };
    let p = AxialPoint { q: 0, r: 0 };
    let mut board = HighlightBoard::new(BodyProjection::MARS, &[p]).unwrap();
    board
        .set(
            p,
            HighlightState {
                hovered: true,
                selected: true,
                ..Default::default()
            },
        )
        .unwrap();
    board.tick(0, true).unwrap();
    board.tick(6, true).unwrap();
    let hover = board.data()[0];
    assert!(hover > 0.0 && hover < 0.3);
    assert_eq!(board.data()[1], 0.0);
    let revision = board.revision();
    assert!(!board.tick(6, true).unwrap());
    assert_eq!(board.revision(), revision);
    assert_eq!(board.data()[0], hover);
    board
        .set(
            p,
            HighlightState {
                available: true,
                selected: true,
                ..Default::default()
            },
        )
        .unwrap();
    board.tick(7, true).unwrap();
    assert_eq!(board.data()[1], 2.0);
    board.set_visibility(p, 0.0).unwrap();
    board.tick(8, false).unwrap();
    assert!(!board.active());
    assert_eq!(board.data()[3], 0.0);
    assert!(board.tick(7, true).is_err());
}
#[test]
fn highlight_layout_rejects_duplicate_sparse_and_unbounded_inputs() {
    use openmars_scene::{
        geometry::{AxialPoint, BodyProjection},
        highlights::{HighlightBoard, HighlightState},
    };
    let p = AxialPoint { q: 0, r: 0 };
    assert!(HighlightBoard::new(BodyProjection::MARS, &[p, p]).is_err());
    assert!(HighlightBoard::new(BodyProjection::MARS, &[p, AxialPoint { q: 500, r: 0 }]).is_err());
    let mut board = HighlightBoard::new(BodyProjection::MARS, &[p]).unwrap();
    assert!(
        board
            .set(
                p,
                HighlightState {
                    vp_intensity: f64::NAN,
                    ..Default::default()
                }
            )
            .is_err()
    );
    assert!(
        board
            .set(AxialPoint { q: 1, r: 0 }, HighlightState::default())
            .is_err()
    );
    assert!(board.data().iter().all(|v| v.is_finite()));
    assert_eq!(board.texture_extent(), [5, 1]);
}

#[test]
fn raw_border_hover_uses_spare_channel_and_does_not_overwrite_tangent() {
    use openmars_scene::{
        geometry::{AxialPoint, BodyProjection},
        highlights::{HighlightBoard, HighlightState},
    };
    let p = AxialPoint { q: 0, r: 0 };
    let mut board = HighlightBoard::new(BodyProjection::MARS, &[p]).unwrap();
    let tangent = board.data()[16..19].to_vec();
    board
        .set(
            p,
            HighlightState {
                hovered: true,
                ..Default::default()
            },
        )
        .unwrap();
    board.tick(0, true).unwrap();
    assert_eq!(board.data()[19], 1.);
    assert_eq!(&board.data()[16..19], tangent);
    board.tick(1, false).unwrap();
    assert_eq!(board.data()[19], 0.);
    assert_eq!(&board.data()[16..19], tangent);
}
