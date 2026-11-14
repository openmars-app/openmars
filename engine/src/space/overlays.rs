//! Project native camera anchors without exposing the diagnostic scene census.
use bevy::prelude::*;
use openmars_scene::{
    geometry::Ray3,
    view::{BodyMarker, OverlayFrame, StationMarker},
};

pub fn frame(world: &mut World) -> OverlayFrame {
    let Some(scene) = world.resource::<crate::scene::SceneState>().0.as_ref() else {
        return OverlayFrame::default();
    };
    let game_id = Some(scene.game_id.clone());
    let details = scene.offworld.station.clone();
    let station_center = world
        .resource::<crate::offworld::scene::OffworldRuntime>()
        .station_center(world);
    let seconds = world
        .resource::<crate::terrain::surface::SurfaceRuntime>()
        .motion_step() as f64
        / 60.;
    let occluders: Vec<_> = super::config::BODIES
        .into_iter()
        .filter_map(|id| {
            let cfg = super::config::config(id);
            let center = if id == openmars_scene::view::BodyId::Mars {
                Vec3::ZERO
            } else {
                super::config::in_mars_frame(super::config::body_pose(id, seconds).ok()?, seconds)
                    .ok()?
                    .center
                    .as_vec3()
            };
            Some((center, cfg.radius))
        })
        .collect();
    let centers = world
        .resource::<super::scene::CelestialRuntime>()
        .marker_centers();
    let view = world.resource::<crate::world::WorldView>();
    let scale = view.size / view.physical.as_vec2();
    let size = view.size;
    let camera = world
        .query_filtered::<(&Camera, &GlobalTransform), With<crate::assets::AssetCamera>>()
        .single(world)
        .ok();
    let bodies = camera.map_or_else(Vec::new, |(camera, pose)| {
        centers
            .into_iter()
            .filter_map(|(body, center)| {
                let point = camera.world_to_viewport(pose, center).ok()? * scale;
                if !point.is_finite()
                    || point.x < 0.
                    || point.y < 0.
                    || point.x > size.x
                    || point.y > size.y
                {
                    return None;
                }
                let edge = camera
                    .world_to_viewport(pose, center + pose.right() * 15.)
                    .ok()?
                    * scale;
                Some(BodyMarker {
                    body,
                    point: point.to_array(),
                    hit_size: point.distance(edge) * 2.,
                })
            })
            .collect()
    });
    let station = camera.and_then(|(camera, pose)| {
        let center = station_center?;
        if occluded(pose.translation(), center, &occluders) {
            return None;
        }
        let point = camera.world_to_viewport(pose, center).ok()? * scale;
        let edge = camera
            .world_to_viewport(pose, center + pose.right() * 0.12)
            .ok()?
            * scale;
        let hit_size = point.distance(edge) * 2.;
        if !point.is_finite()
            || !hit_size.is_finite()
            || hit_size < 1.
            || point.x < 0.
            || point.y < 0.
            || point.x > size.x
            || point.y > size.y
        {
            return None;
        }
        Some(StationMarker {
            point: point.to_array(),
            hit_size,
            details: details?,
        })
    });
    OverlayFrame {
        game_id,
        bodies,
        station,
    }
}

fn occluded(eye: Vec3, target: Vec3, spheres: &[(Vec3, f64)]) -> bool {
    let direction = (target - eye).as_dvec3();
    spheres.iter().any(|(center, radius)| {
        Ray3 {
            origin: (eye - *center).as_dvec3().to_array(),
            dir: direction.to_array(),
        }
        .intersect_sphere(*radius)
        .is_some_and(|hit| hit.distance < direction.length() - 0.12)
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn station_interaction_cannot_reach_through_mars_or_another_body() {
        let eye = Vec3::new(0., 0., 6.);
        let spheres = [(Vec3::ZERO, 2.02), (Vec3::new(3., 0., 0.), 0.415)];
        assert!(occluded(eye, Vec3::new(0., 0., -3.), &spheres));
        assert!(!occluded(eye, Vec3::new(0., 0., 3.), &spheres));
        assert!(occluded(eye, Vec3::new(3.3, 0., -0.6), &spheres));
        assert!(!occluded(eye, Vec3::new(3., 0., 1.), &spheres));
    }
}
