use super::{Frame, Spec, SurfaceKind, material::SpecialUniform};
use bevy::{
    math::{DQuat, DVec3},
    prelude::*,
};
use openmars_scene::{
    climate::ClimateState,
    geometry::volcano::{VOLCANO_DEFAULT, VolcanoParams, volcano_seed_parameters},
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
    pub climate: ClimateState,
}
fn ease(v: f64) -> f32 {
    (1. - (1. - v.clamp(0., 1.)).powi(3)) as f32
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
    pub fn stages(self, seconds: f64, reduced: bool) -> Vec4 {
        let age = (self.destroyed.map_or(seconds, |(at, _)| {
            seconds.min(at + if reduced { 0. } else { 1. })
        }) - self.born)
            .max(0.);
        let stage = |start, end| {
            if reduced {
                1.
            } else {
                ease((age - start) / (end - start))
            }
        };
        Vec4::new(stage(0., 0.4), stage(0.4, 1.), stage(0.6, 1.2), 0.)
    }
    pub fn parameters(
        self,
        spec: Spec,
        frame: &Frame,
        pose: Pose,
        facts: &[crate::nuclear::collapse::MotionFacts],
    ) -> Result<SpecialUniform, String> {
        let Pose {
            radius,
            seconds,
            motion_seconds,
            reduced,
            climate,
        } = pose;
        let (source, visual) = self.destroyed.map_or((seconds, motion_seconds), |(s, m)| {
            (
                seconds.min(s + if reduced { 0. } else { 1. }),
                motion_seconds.min(m + if reduced { 0. } else { 1. }),
            )
        });
        let age = (source - self.born).max(0.);
        let volcano = spec.kind == SurfaceKind::Volcano;
        let emergence = if reduced {
            1.
        } else if volcano {
            ease(age / 1.2)
        } else {
            ease((age - 0.2) / 0.6)
        };
        let shake_duration = if volcano { 1.2 } else { 1.5 };
        let shake = if reduced {
            0.
        } else {
            (if volcano { 0.008 } else { 0.012 }) * (1. - ease(age / shake_duration))
        };
        let ms = (visual - self.born_motion).max(0.) * 1000.;
        let seed = f64::from(spec.seed);
        let mut alpha = if reduced {
            (age / 0.15).clamp(0., 1.) as f32
        } else {
            1.
        };
        if reduced && let Some((at, _)) = self.destroyed {
            alpha *= (1. - (seconds - at) / 0.15).clamp(0., 1.) as f32;
        }
        let q = DQuat::from_array(frame.rotation);
        Ok(SpecialUniform {
            geometry: Vec4::new(radius, spec.seed as f32, 0., spec.kind.index()),
            seeds: volcano_seed_parameters(&VolcanoParams {
                seed: spec.seed as f32,
                ..VOLCANO_DEFAULT
            })
            .map_err(|e| e.to_string())?
            .map(Vec4::from_array),
            profile: if volcano {
                Vec4::new(0.14, 0.22, 0.08, 0.15)
            } else {
                Vec4::new(0.06, 0.4, 0., 0.14)
            },
            clock: Vec4::new(visual as f32, emergence, alpha, 1.),
            climate: Vec4::new(climate.grass as f32, climate.frost as f32, 0., 0.),
            sun: Vec3::new(0.9, 0., 0.8).normalize().extend(0.65),
            sun_color: Vec4::new(1., 0.93, 0.85, 1.),
            local_x: (q * DVec3::X).as_vec3().extend(0.),
            local_y: (q * DVec3::Y).as_vec3().extend(0.),
            origin: DVec3::from_array(frame.origin).as_vec3().extend(1.),
            motion: Vec4::new(
                (ms * 0.137 + seed).sin() as f32 * shake,
                (ms * 0.173 + seed * 1.3).sin() as f32 * shake,
                if reduced {
                    0.
                } else {
                    self.destroyed
                        .map_or(0., |(at, _)| ((seconds - at) * 0.8 - 0.8) as f32)
                },
                if !reduced && self.destroyed.is_some() {
                    1.
                } else {
                    0.
                },
            ),
            collapse_parts: if reduced {
                [Vec4::ZERO; 5]
            } else {
                crate::nuclear::collapse::animated(
                    facts,
                    self.destroyed
                        .map_or(-1., |(at, _)| (seconds - at) * 0.8 - 0.8),
                )
            },
        })
    }
}
