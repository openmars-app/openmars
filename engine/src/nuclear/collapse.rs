//! Original NuclearCollapse bounds, measured in its local frame after the
//! source object's world-axis-aligned bounding box has been constructed.
use bevy::math::{DQuat, DVec3};
pub(crate) fn depth(bounds: [DVec3; 2], rotation: DQuat, minimum: f64) -> f32 {
    let corners = |bounds: [DVec3; 2]| {
        std::array::from_fn::<_, 8, _>(|i| {
            DVec3::new(
                bounds[i & 1].x,
                bounds[(i >> 1) & 1].y,
                bounds[(i >> 2) & 1].z,
            )
        })
    };
    let mut world = [DVec3::splat(f64::INFINITY), DVec3::splat(f64::NEG_INFINITY)];
    for p in corners(bounds) {
        let p = rotation * p;
        world[0] = world[0].min(p);
        world[1] = world[1].max(p);
    }
    let inverse = rotation.inverse();
    (corners(world)
        .into_iter()
        .map(|p| {
            let p = inverse * p;
            p.z + p.length() * 0.11
        })
        .fold(minimum, f64::max)
        + 0.065) as f32
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn original_flat_disc_bounds_and_custom_shader_height_set_the_complete_sink() {
        let r = 0.2988;
        let b = [DVec3::new(-r, -r, 0.), DVec3::new(r, r, 0.)];
        assert!(
            (f64::from(depth(b, DQuat::IDENTITY, 0.)) - (r * 2_f64.sqrt() * 0.11 + 0.065)).abs()
                < 1e-8
        );
        assert_eq!(depth(b, DQuat::IDENTITY, 0.15), 0.215);
        assert!(depth(b, DQuat::from_rotation_x(0.7), 0.) > depth(b, DQuat::IDENTITY, 0.));
    }
}

/// A source mesh's bounds and complete authored hierarchy before projection.
/// Cart meshes receive their captured parent pose when the effect starts.
#[derive(Clone, Debug)]
pub(crate) struct PartSource {
    pub seed: u64,
    pub bounds: [DVec3; 2],
    pub body_from_mesh: bevy::math::DMat4,
    pub tile_from_body: bevy::math::DMat4,
    pub cart: bool,
    pub minimum: f64,
}
#[derive(Clone, Copy, Debug)]
pub(crate) struct MotionFacts {
    pub parameters: openmars_scene::geometry::collapse::CollapseParameters,
    pub depth: f64,
}
fn corners(b: [DVec3; 2]) -> [DVec3; 8] {
    std::array::from_fn(|i| DVec3::new(b[i & 1].x, b[(i >> 1) & 1].y, b[(i >> 2) & 1].z))
}
impl PartSource {
    pub fn capture(&self, parent: bevy::math::DMat4) -> MotionFacts {
        let matrix = parent * self.body_from_mesh;
        let mut bounds = [DVec3::splat(f64::INFINITY), DVec3::splat(f64::NEG_INFINITY)];
        for p in corners(self.bounds) {
            let p = matrix.transform_point3(p);
            bounds[0] = bounds[0].min(p);
            bounds[1] = bounds[1].max(p);
        }
        let depth = corners(bounds)
            .into_iter()
            .map(|p| {
                let p = self.tile_from_body.transform_point3(p);
                p.z + p.length() * 0.11
            })
            .fold(self.minimum, f64::max)
            + 0.065;
        MotionFacts {
            parameters: openmars_scene::geometry::collapse::CollapseParameters::new(self.seed),
            depth,
        }
    }
}
pub(crate) fn animated<const N: usize>(facts: &[MotionFacts], age: f64) -> [bevy::math::Vec4; N] {
    std::array::from_fn(|i| {
        facts.get(i).map_or(bevy::math::Vec4::ZERO, |p| {
            let t = p.parameters.progress(age);
            bevy::math::Vec4::new(
                p.parameters.direction.cos() as f32,
                p.parameters.direction.sin() as f32,
                (p.parameters.tilt * t) as f32,
                (p.depth * t) as f32,
            )
        })
    })
}
