//! Analytic source interaction shells in each body's shared local frame.
use super::config::{self, BodyPose};
use bevy::math::DVec3;
use openmars_scene::{
    geometry::{AxialPoint, BodyProjection, PICK_SHELL, Ray3},
    view::{BodyId, FocusTarget},
};

pub fn projection(body: BodyId, phobos_content: Option<f64>) -> Option<BodyProjection> {
    if body == BodyId::Mars {
        return Some(BodyProjection::MARS);
    }
    let cfg = config::config(body);
    if cfg.location_offset == 0 {
        return None;
    }
    BodyProjection::new(
        cfg.radius,
        if body == BodyId::Phobos {
            phobos_content?
        } else {
            cfg.radius
        },
        1,
        AxialPoint {
            q: cfg.location_offset,
            r: 0,
        },
    )
    .ok()
}
pub fn active(body: BodyId, focus: FocusTarget) -> bool {
    matches!(focus,FocusTarget::Body(target) if target.focus_group()==body.focus_group())
}

/// The nearest visible sphere wins, including the current body and Sun as
/// occluders. Travel selects the same parent group as the existing menu.
pub fn body(ray: Ray3, seconds: f64, focus: FocusTarget) -> Option<BodyId> {
    let origin = DVec3::from_array(ray.origin);
    let hit = |center: DVec3, radius: f64| {
        Ray3 {
            origin: (origin - center).to_array(),
            dir: ray.dir,
        }
        .intersect_sphere(radius)
        .map(|hit| hit.distance)
    };
    let (id, distance) = config::BODIES
        .into_iter()
        .filter_map(|id| {
            let center = if id == BodyId::Mars {
                DVec3::ZERO
            } else {
                config::in_mars_frame(config::body_pose(id, seconds).ok()?, seconds)
                    .ok()?
                    .center
            };
            Some((id, hit(center, config::config(id).radius)?))
        })
        .min_by(|a, b| a.1.total_cmp(&b.1))?;
    let sun = config::in_mars_frame(
        BodyPose {
            center: DVec3::ZERO,
            rotation: bevy::math::DQuat::IDENTITY,
        },
        seconds,
    )
    .ok()?
    .center;
    if hit(sun, 22.).is_some_and(|sun_distance| sun_distance < distance) || active(id, focus) {
        return None;
    }
    Some(id.focus_group())
}
/// Distances are unchanged by the rigid floating-frame transform. The source
/// picks radius+.02 (also for Phobos), then finds a tile center strictly <.17.
pub fn tile(
    ray: Ray3,
    projection: BodyProjection,
    pose: BodyPose,
    coordinates: &[AxialPoint],
) -> Option<(AxialPoint, f64)> {
    let origin = pose.world_to_local(DVec3::from_array(ray.origin));
    let direction = pose.rotation.inverse() * DVec3::from_array(ray.dir);
    let local = Ray3 {
        origin: origin.to_array(),
        dir: direction.to_array(),
    };
    let hit = local.intersect_sphere(projection.radius() + PICK_SHELL)?;
    Some((
        projection.nearest_tile(hit.point, coordinates)?,
        hit.distance,
    ))
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn visible_body_surfaces_are_travel_targets_outside_the_current_group() {
        for seconds in [0., 100., 18000.] {
            for id in config::BODIES {
                if config::parent(id) != id {
                    continue;
                }
                let center = if id == BodyId::Mars {
                    DVec3::ZERO
                } else {
                    config::in_mars_frame(config::body_pose(id, seconds).unwrap(), seconds)
                        .unwrap()
                        .center
                };
                let ray = Ray3 {
                    origin: (center + DVec3::Z * (config::config(id).radius + 1.)).to_array(),
                    dir: (-DVec3::Z).to_array(),
                };
                assert_eq!(body(ray, seconds, FocusTarget::SolarSystem), Some(id));
                assert_eq!(body(ray, seconds, FocusTarget::Body(id)), None);
            }
        }
    }
    #[test]
    fn every_authored_tile_body_picks_the_absolute_offset_in_its_moving_frame() {
        for seconds in [0., 100., 18000.] {
            for body in config::BODIES {
                let Some(projection) = projection(body, Some(0.3276999)) else {
                    continue;
                };
                let pose = if body == BodyId::Mars {
                    BodyPose {
                        center: DVec3::ZERO,
                        rotation: bevy::math::DQuat::IDENTITY,
                    }
                } else {
                    config::in_mars_frame(config::body_pose(body, seconds).unwrap(), seconds)
                        .unwrap()
                };
                let coordinate = AxialPoint {
                    q: projection.coord_offset().q + 1,
                    r: 0,
                };
                let center = DVec3::from_array(
                    projection
                        .project(projection.hex_center(coordinate), 0.)
                        .unwrap(),
                );
                let normal = pose.rotation * center.normalize();
                let ray = Ray3 {
                    origin: (pose.center + normal * (projection.radius() + 8.)).to_array(),
                    dir: (-normal).to_array(),
                };
                assert_eq!(
                    tile(ray, projection, pose, &[coordinate]).unwrap().0,
                    coordinate,
                    "{body:?} {seconds}"
                );
            }
        }
    }
    #[test]
    fn only_the_focused_parent_group_accepts_tiles() {
        for (parent, child) in [
            (BodyId::Mars, BodyId::Phobos),
            (BodyId::Earth, BodyId::Luna),
            (BodyId::Jupiter, BodyId::Ganymede),
        ] {
            assert!(active(child, FocusTarget::Body(parent)));
            assert!(!active(child, FocusTarget::SolarSystem));
            assert!(!active(child, FocusTarget::Body(BodyId::Venus)));
        }
        assert!(projection(BodyId::Earth, None).is_none());
        assert!(projection(BodyId::Phobos, None).is_none());
    }
}
