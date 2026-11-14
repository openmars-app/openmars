//! Original asteroid flight and impact dust sampled at canonical60Hz. A seeded
//! event stream replaces ambient Math.random; no frame-size integration drifts.
use bevy::{
    math::{DQuat, DVec3},
    prelude::*,
};
use openmars_scene::geometry::placement::random_sequence;
pub const FLIGHT_STEPS: u64 = 108;
pub const DUST_STEPS: u64 = 180;
pub const TOTAL_STEPS: u64 = FLIGHT_STEPS + DUST_STEPS;
fn error(s: &str) -> String {
    s.into()
}
#[derive(Clone, Copy, Debug)]
pub struct Dust {
    position: DVec3,
    velocity: DVec3,
    rotation: f64,
    rotation_speed: f64,
    lifetime: f64,
    scale: f64,
}
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct DustPose {
    pub position: DVec3,
    pub scale: f64,
    pub rotation: f64,
    pub opacity: f64,
}
impl Dust {
    pub fn sample(&self, steps: u64) -> DustPose {
        let age = steps as f64 / 60.;
        let ratio = age / self.lifetime;
        // The last source frame at/after lifetime makes the sprite invisible
        // without advancing its position. Closed-form sums preserve that rule.
        let moves = steps.min((self.lifetime * 60.).ceil() as u64 - 1);
        let travel = (1. - 0.97_f64.powf(moves as f64)) / (1. - 0.97) / 60.;
        let last_ratio = if ratio < 1. {
            ratio
        } else {
            moves as f64 / 60. / self.lifetime
        };
        DustPose {
            position: self.position + self.velocity * travel,
            rotation: self.rotation + self.rotation_speed * moves as f64 / 60.,
            scale: self.scale * (1. + last_ratio * 2.),
            opacity: if ratio < 1. {
                (ratio * 5.).min(1.) * (1. - ratio * ratio) * 0.85
            } else {
                0.
            },
        }
    }
}
pub struct Flight {
    pub impact: DVec3,
    start: DVec3,
    axis: DVec3,
    pub dust: Vec<Dust>,
}
impl Flight {
    /// Owner admits descriptors/entity work before allocation. Counts never
    /// exceed the actual original60-particle pool or change global limits.
    pub fn new(seed: u32, radius: f64, count: u16) -> Result<Self, String> {
        if !radius.is_finite() || radius <= 0. || count == 0 || count > 60 {
            return Err(error("asteroid radius/particle bound"));
        }
        let mut random = random_sequence(seed);
        let theta = random() * std::f64::consts::PI * 0.6 - std::f64::consts::PI * 0.3;
        let phi = random() * std::f64::consts::PI * 0.4;
        let impact = DVec3::new(
            radius * phi.sin() * theta.cos(),
            radius * phi.sin() * theta.sin(),
            radius * phi.cos(),
        );
        let start = impact + DVec3::new(3. + random() * 2., 4. + random() * 2., 6.);
        let axis = DVec3::new(random() - 0.5, random() - 0.5, random() - 0.5).normalize_or_zero();
        let axis = if axis.length_squared() == 0. {
            DVec3::X
        } else {
            axis
        };
        let normal = impact.normalize();
        let tangent = normal
            .cross(if normal.y.abs() < 0.9 {
                DVec3::Y
            } else {
                DVec3::X
            })
            .normalize();
        let bitangent = normal.cross(tangent).normalize();
        let mut dust = Vec::with_capacity(count as usize);
        for _ in 0..count {
            let angle = random() * std::f64::consts::TAU;
            let r = random() * 0.08;
            let position = tangent * (angle.cos() * r) + bitangent * (angle.sin() * r);
            let rotation = random() * std::f64::consts::TAU;
            let scale = 0.4 + random() * 0.6;
            let spread = random() * std::f64::consts::TAU;
            let velocity_radius = 0.04 + random() * 0.06;
            let velocity = normal * (0.03 + random() * 0.025)
                + tangent * (spread.cos() * velocity_radius)
                + bitangent * (spread.sin() * velocity_radius);
            let rotation_speed = (random() - 0.5) * 1.8;
            let lifetime = 1.5 + random() * 1.5;
            dust.push(Dust {
                position,
                velocity,
                rotation,
                rotation_speed,
                lifetime,
                scale,
            });
        }
        Ok(Self {
            impact,
            start,
            axis,
            dust,
        })
    }
    pub fn pose(&self, steps: u64) -> Transform {
        let n = steps.min(FLIGHT_STEPS) as f64;
        let t = n / FLIGHT_STEPS as f64;
        let eased = 1. - (1. - t).powi(3);
        // Source rotates once per60Hz frame, including the initial frame.
        let angle = 0.016 * (12. * (n + 1.) - 8. / FLIGHT_STEPS as f64 * n * (n + 1.) * 0.5);
        Transform {
            translation: self.start.lerp(self.impact, eased).as_vec3(),
            rotation: DQuat::from_axis_angle(self.axis, angle).as_quat(),
            scale: Vec3::splat((1. - eased * 0.3) as f32),
        }
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn dust_seeks_repeatably_and_expires_without_catchup() {
        let flight = Flight::new(37, 2.02, 60).unwrap();
        assert!((flight.impact.length() - 2.02).abs() < 1e-12);
        for particle in &flight.dust {
            let before = particle.sample(30);
            let after = particle.sample(90);
            assert!(before.position.is_finite() && after.position.is_finite());
            assert_eq!(before, particle.sample(30));
            assert_eq!(after, particle.sample(90));
            assert_eq!(particle.sample(DUST_STEPS).opacity, 0.);
        }
        assert!(
            flight
                .pose(108)
                .translation
                .abs_diff_eq(flight.impact.as_vec3(), 1e-6)
        );
        assert_eq!(flight.pose(108).scale, Vec3::splat(0.7));
        assert!(Flight::new(0, 0., 1).is_err());
        assert!(Flight::new(0, 2.02, 61).is_err());
    }
}
