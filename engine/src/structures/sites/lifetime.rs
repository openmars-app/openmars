use super::{Frame, Kind, Spec, geometry, material::SiteUniform};
use bevy::{
    math::{DQuat, DVec3},
    prelude::*,
};

#[derive(Clone, Copy, Debug)]
pub(super) struct Life {
    pub born: f64,
    pub born_motion: f64,
    pub destroyed: Option<(f64, f64)>,
}
#[derive(Clone, Copy)]
pub(super) struct Pose {
    pub radius: f32,
    pub seconds: f64,
    pub motion_seconds: f64,
    pub reduced: bool,
}
fn ease(value: f64) -> f32 {
    (1. - (1. - value.clamp(0., 1.)).powi(3)) as f32
}
impl Life {
    pub fn settled() -> Self {
        Self {
            born: -1000.,
            born_motion: -1000.,
            destroyed: None,
        }
    }
    pub fn ended(self, seconds: f64, reduced: bool) -> bool {
        self.destroyed
            .is_some_and(|(at, _)| seconds >= at + if reduced { 0.15 } else { 128. / 60. })
    }
    pub fn parameters(
        self,
        spec: Spec,
        frame: &Frame,
        pose: Pose,
        facts: &[crate::nuclear::collapse::MotionFacts],
    ) -> Result<SiteUniform, String> {
        let Spec { kind, seed, color } = spec;
        let Pose {
            radius,
            seconds,
            motion_seconds,
            reduced,
        } = pose;
        let (source, source_motion) =
            self.destroyed
                .map_or((seconds, motion_seconds), |(at, at_motion)| {
                    (
                        seconds.min(at + if reduced { 0. } else { 1. }),
                        motion_seconds.min(at_motion + if reduced { 0. } else { 1. }),
                    )
                });
        let age = (source - self.born).max(0.);
        let duration = if kind == Kind::Mining { 0.9 } else { 0.8 };
        let emergence = if reduced { 1. } else { ease(age / duration) };
        let shake = if reduced || kind == Kind::Reserved {
            0.
        } else {
            0.01 * (1. - emergence)
        };
        let ms = (source_motion - self.born_motion).max(0.) * 1000.;
        let mut alpha = if reduced {
            (age / 0.15).clamp(0., 1.) as f32
        } else {
            1.
        };
        if reduced && let Some((at, _)) = self.destroyed {
            alpha *= (1. - (seconds - at) / 0.15).clamp(0., 1.) as f32;
        }
        let stage = |start, end| {
            if reduced {
                1.
            } else {
                ease((age - start) / (end - start))
            }
        };
        let cart = geometry::cart_pose(source_motion)?;
        let q = DQuat::from_array(frame.rotation);
        Ok(SiteUniform {
            geometry: Vec4::new(
                radius,
                seed as f32,
                geometry::cave_angle(seed) as f32,
                if kind == Kind::Mining { 2. } else { 3. },
            ),
            clock: Vec4::new(source_motion as f32, emergence, alpha, age as f32),
            local_x: (q * DVec3::X).as_vec3().extend(0.),
            local_y: (q * DVec3::Y).as_vec3().extend(0.),
            origin: DVec3::from_array(frame.origin).as_vec3().extend(1.),
            motion: Vec4::new(
                (ms * 0.137 + f64::from(seed)).sin() as f32 * shake,
                (ms * 0.173 + f64::from(seed) * 1.3).sin() as f32 * shake,
                0.,
                0.,
            ),
            collapse_parts: if reduced {
                [Vec4::ZERO; 32]
            } else {
                crate::nuclear::collapse::animated(
                    facts,
                    self.destroyed
                        .map_or(-1., |(at, _)| (seconds - at) * 0.8 - 0.8),
                )
            },
            stages: Vec4::new(stage(0., 0.6), stage(0.4, 0.8), 1., 0.),
            owner_color: color,
            cart: Vec4::new(
                cart.position.y as f32,
                cart.position.z as f32,
                cart.tilt as f32,
                if cart.ore { 1. } else { 0. },
            ),
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use openmars_scene::geometry::BodyProjection;
    #[test]
    fn staged_posts_and_rails_and_reduced_fade_share_a_frozen_cart_clock() {
        let frame = Frame::new(BodyProjection::MARS, [0, 0, 0], 0.).unwrap();
        let life = Life {
            born: 10.,
            born_motion: 4.,
            destroyed: None,
        };
        let a = life
            .parameters(
                Spec {
                    kind: Kind::Reserved,
                    seed: 37,
                    color: Vec4::ONE,
                },
                &frame,
                Pose {
                    radius: 2.02,
                    seconds: 10.3,
                    motion_seconds: 4.3,
                    reduced: false,
                },
                &[],
            )
            .unwrap();
        assert!((a.stages.x - 0.875).abs() < 1e-6);
        assert_eq!(a.stages.y, 0.);
        let b = life
            .parameters(
                Spec {
                    kind: Kind::Mining,
                    seed: 37,
                    color: Vec4::ONE,
                },
                &frame,
                Pose {
                    radius: 2.02,
                    seconds: 10.075,
                    motion_seconds: 4.3,
                    reduced: true,
                },
                &[],
            )
            .unwrap();
        let c = life
            .parameters(
                Spec {
                    kind: Kind::Mining,
                    seed: 37,
                    color: Vec4::ONE,
                },
                &frame,
                Pose {
                    radius: 2.02,
                    seconds: 12.,
                    motion_seconds: 4.3,
                    reduced: true,
                },
                &[],
            )
            .unwrap();
        assert!((b.clock.z - 0.5).abs() < 1e-6);
        assert_eq!(b.cart, c.cart);
        assert_eq!(b.motion, Vec4::ZERO);
        assert_eq!(b.clock.y, 1.);
        let d = Life::settled()
            .parameters(
                Spec {
                    kind: Kind::Mining,
                    seed: 37,
                    color: Vec4::ONE,
                },
                &frame,
                Pose {
                    radius: 2.02,
                    seconds: 0.,
                    motion_seconds: 0.,
                    reduced: false,
                },
                &[],
            )
            .unwrap();
        assert_eq!(d.motion, Vec4::ZERO);
        assert_eq!(d.clock.y, 1.);
    }
}
