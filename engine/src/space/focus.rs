//! Local camera framing from public placement choices. No gameplay decisions.
use bevy::prelude::*;
use openmars_scene::{
    geometry::{AxialPoint, BodyProjection, camera_frame, focus::placement_focus},
    view::{BodyId, FocusTarget},
};
use std::collections::{BTreeMap, BTreeSet};

#[derive(Default)]
pub struct PlacementCamera {
    framed: BTreeSet<BodyId>,
    animation: Option<Animation>,
}
struct Animation {
    focus: FocusTarget,
    start: f64,
    from: Vec3,
    to: Vec3,
}
impl PlacementCamera {
    pub fn animating(&self) -> bool {
        self.animation.is_some()
    }
    pub fn cancel(&mut self) {
        self.animation = None;
    }
}

pub fn prepare(world: &mut World) {
    let Some(scene) = world.resource::<crate::scene::SceneState>().0.as_ref() else {
        return;
    };
    let space = world.resource::<super::scene::CelestialRuntime>();
    if !space.ready() {
        return;
    }
    let mut points: BTreeMap<BodyId, Vec<Vec3>> = BTreeMap::new();
    let available: BTreeSet<_> = scene.selectable_tiles.iter().map(String::as_str).collect();
    for tile in &scene.tiles {
        if !available.contains(tile.id.as_str()) {
            continue;
        }
        let Some(body) = BodyId::from_location(&tile.location) else {
            continue;
        };
        let frame = if body == BodyId::Mars {
            Some((BodyProjection::MARS, Transform::IDENTITY))
        } else {
            space.grid_frame(body)
        };
        let Some((projection, transform)) = frame else {
            continue;
        };
        let Ok(local) = projection.project(
            projection.hex_center(AxialPoint {
                q: tile.q,
                r: tile.r,
            }),
            0.,
        ) else {
            continue;
        };
        points
            .entry(body)
            .or_default()
            .push(transform.transform_point(Vec3::from_array(local.map(|v| v as f32))));
    }
    let step = world
        .resource::<crate::terrain::surface::SurfaceRuntime>()
        .step() as f64
        / 60.;
    let mut view = world.resource_mut::<crate::world::WorldView>();
    view.placement
        .framed
        .retain(|body| points.contains_key(body));
    if view
        .placement
        .animation
        .as_ref()
        .is_some_and(|a| a.focus != view.focus)
    {
        view.placement.cancel();
    }
    if view.compact
        && let FocusTarget::Body(target) = view.focus
    {
        let bodies: Vec<_> = points
            .keys()
            .copied()
            .filter(|b| b.focus_group() == target.focus_group())
            .collect();
        if bodies.iter().any(|b| !view.placement.framed.contains(b)) {
            let inverse = view.binding.to_matrix().inverse();
            let local: Vec<_> = bodies
                .iter()
                .flat_map(|b| &points[b])
                .map(|p| inverse.transform_point3(*p).as_dvec3().to_array())
                .collect();
            if let Some(frame) =
                camera_frame(view.size.x.into(), view.size.y.into(), view.insets, true)
            {
                let limits = super::config::orbit(view.focus);
                if let Some(request) =
                    placement_focus(&local, frame, view.size.y.into(), limits.min, limits.max)
                {
                    let direction = Vec3::from_array(request.direction.map(|v| v as f32));
                    let theta = direction.x.atan2(direction.z);
                    let delta = (theta - view.yaw + std::f32::consts::PI)
                        .rem_euclid(std::f32::consts::TAU)
                        - std::f32::consts::PI;
                    view.placement.animation = Some(Animation {
                        focus: view.focus,
                        start: step,
                        from: Vec3::new(view.yaw, view.pitch, view.distance),
                        to: Vec3::new(
                            view.yaw + delta,
                            direction
                                .y
                                .clamp(-1., 1.)
                                .asin()
                                .clamp(-1.4835298, 1.4835298),
                            request.distance as f32,
                        ),
                    });
                    view.placement.framed.extend(bodies);
                }
            }
        }
    }
    if let Some(animation) = view.placement.animation.take() {
        let t = ((step - animation.start) / 0.4).clamp(0., 1.) as f32;
        let value = animation.from.lerp(animation.to, 1. - (1. - t).powi(3));
        view.yaw = value.x;
        view.pitch = value.y;
        view.distance = value.z;
        view.hovered = None;
        if t < 1. {
            view.placement.animation = Some(animation);
        }
    }
}
