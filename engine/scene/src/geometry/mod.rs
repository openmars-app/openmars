//! Body-local math shared by mesh construction and picking. No game rules or transport.
mod discriminant;
pub mod focus;
mod framing;
pub mod noise;
pub mod nuclear;
pub mod nuclear_mesh;
pub mod nuclear_site;
mod projection;
mod ray;
pub mod terrain;
pub mod volcano;
pub use framing::{CameraFrame, HudInsets, camera_frame};
pub use noise::landscape_noise;
pub use projection::{
    AxialPoint, BOARD_SCALE, BoardPoint, BodyProjection, GeometryError, HEX_HIT_RADIUS, HEX_SIZE,
    MARS_RADIUS, PICK_SHELL, round_cube,
};
pub use ray::{Ray3, SphereHit};

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn mars_grid_centers_the_origin_and_orients_east_and_north() {
        let projection = BodyProjection::MARS;
        assert_eq!(
            projection.hex_center(AxialPoint::default()),
            BoardPoint::default()
        );
        assert_eq!(
            projection.project(BoardPoint::default(), 0.0).unwrap(),
            [0.0, 0.0, MARS_RADIUS]
        );
        let east = projection.hex_center(AxialPoint { q: 1, r: 0 });
        assert!(east.x > 0.0);
        assert!(projection.hex_center(AxialPoint { q: 0, r: -1 }).y > 0.0);
    }
    #[test]
    fn projected_board_centers_round_trip_and_pick_their_own_tiles() {
        let projection = BodyProjection::MARS;
        let tiles: Vec<_> = (-4_i32..=4)
            .flat_map(|q| {
                (-4_i32..=4)
                    .filter_map(move |r| ((q + r).abs() <= 4).then_some(AxialPoint { q, r }))
            })
            .collect();
        assert_eq!(tiles.len(), 61);
        for tile in &tiles {
            let board = projection.hex_center(*tile);
            let surface = projection.project(board, 0.0).unwrap();
            let restored = projection.unproject(surface).unwrap();
            assert!((restored.x - board.x).abs() < 1e-12 && (restored.y - board.y).abs() < 1e-12);
            assert_eq!(
                round_cube(projection.board_to_hex(restored).unwrap()).unwrap(),
                *tile
            );
            let ray = Ray3 {
                origin: surface.map(|v| v * 3.0),
                dir: surface.map(|v| -v),
            };
            assert_eq!(projection.pick(ray, &tiles), Some(*tile));
        }
    }
    #[test]
    fn picking_rejects_misses_behind_rays_and_nonfinite_values() {
        assert!(
            Ray3 {
                origin: [0.0, 0.0, 5.0],
                dir: [0.0, 0.0, 1.0]
            }
            .intersect_sphere(2.02)
            .is_none()
        );
        assert!(
            Ray3 {
                origin: [0.0, 0.0, 5.0],
                dir: [1.0, 0.0, 0.0]
            }
            .intersect_sphere(2.02)
            .is_none()
        );
        assert!(
            Ray3 {
                origin: [f64::NAN, 0.0, 5.0],
                dir: [0.0, 0.0, -1.0]
            }
            .intersect_sphere(2.02)
            .is_none()
        );
        assert!(
            BodyProjection::MARS
                .project(
                    BoardPoint {
                        x: f64::INFINITY,
                        y: 0.0
                    },
                    0.0
                )
                .is_err()
        );
        let hit = Ray3 {
            origin: [0.0, 0.0, 5.0],
            dir: [0.0, 0.0, -1000.0],
        }
        .intersect_sphere(2.02)
        .unwrap();
        assert!((hit.distance - 2.98).abs() < 1e-12);
    }
}

pub mod placement;
mod placement_predicate;

pub mod collapse;
