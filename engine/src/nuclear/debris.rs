//! Seeded rock trajectories prepared cooperatively against the same paid crater
//! ground as its mesh. Descriptors remain stable when the presentation is sought.
use bevy::{
    math::{DQuat, DVec3, EulerRot},
    prelude::*,
};
use openmars_scene::geometry::{
    nuclear::NuclearProfile, nuclear_site::CraterGround, placement::random_sequence,
};
use std::f64::consts::TAU;
fn error() -> String {
    "nuclear fragment ground/trajectory".into()
}
#[derive(Clone, Debug, PartialEq)]
pub struct Fragment {
    pub x: f64,
    pub y: f64,
    pub size: f64,
    pub spill: bool,
    pub start: DVec3,
    pub end: DVec3,
    pub up: DVec3,
    pub rotation: DQuat,
    pub launch_rotation: DQuat,
    pub scale: DVec3,
    pub color: [f64; 3],
    pub delay: f64,
    pub duration: f64,
    pub arc: f64,
}
impl Fragment {
    /// Canonical fixed steps run at 60Hz. The original scene clock advances .8
    /// seconds per playback second, including its .8-second missile flight.
    pub fn pose(&self, start: Option<u64>, step: u64, reduced: bool) -> (Transform, bool) {
        let age = match start {
            None => f64::INFINITY,
            Some(born) if step < born => f64::NEG_INFINITY,
            Some(born) if reduced => {
                if step - born < 9 {
                    f64::NEG_INFINITY
                } else {
                    f64::INFINITY
                }
            }
            Some(born) => (step - born) as f64 / 60. * 0.8 - 0.8,
        };
        let visible = age >= self.delay;
        let t = ((age - self.delay) / self.duration).clamp(0., 1.);
        let position = self.start.lerp(self.end, t) + self.up * (self.arc * 4. * t * (1. - t));
        let rotation = self.launch_rotation.slerp(self.rotation, t);
        (
            Transform {
                translation: position.as_vec3(),
                rotation: rotation.as_quat(),
                scale: self.scale.as_vec3(),
            },
            visible,
        )
    }
    /// Original linear shade as f32 bits, not a quantized RGB palette. The top bit is the upload/timeline
    /// visibility bit; stock mesh tags carry the same tint through instancing.
    pub fn tag(&self, visible: bool) -> u32 {
        ((self.color[0] / 0.75) as f32).to_bits() | if visible { 0 } else { 1 << 31 }
    }
}
pub struct FragmentWork {
    random: Box<dyn FnMut() -> f64 + Send + Sync>,
    next: usize,
    fragments: Vec<Fragment>,
    failed: bool,
}
impl FragmentWork {
    /// Caller admits the complete bounded descriptor/instance allocation first.
    pub fn new(seed: u32) -> Self {
        Self {
            random: Box::new(random_sequence(seed ^ 0x5482)),
            next: 0,
            fragments: Vec::with_capacity(144),
            failed: false,
        }
    }
    pub fn advance(
        &mut self,
        profile: &NuclearProfile,
        ground: &CraterGround<'_>,
    ) -> Result<bool, String> {
        if self.failed {
            return Err(error());
        }
        let result = self.advance_inner(profile, ground);
        if result.is_err() {
            self.failed = true;
        }
        result
    }
    fn advance_inner(&mut self, p: &NuclearProfile, g: &CraterGround<'_>) -> Result<bool, String> {
        // Eight authored candidates, at most 96 attempts in one scene turn.
        for i in self.next..(self.next + 8).min(144) {
            self.next = i + 1;
            let spill = i >= 120;
            let r = &mut self.random;
            let mut accepted = None;
            for _ in 0..if spill { 1 } else { 12 } {
                let mut angle = r() * TAU;
                if spill {
                    angle = p.fans[i % 3] + (r() - 0.5) * 0.55;
                } else if r() < 0.3 {
                    let b = p.breaches()[(r() * p.breaches().len() as f64).floor() as usize];
                    angle = b[0] + (r() - 0.5) * (b[1] * 5.);
                }
                let mut radius = p.radius(angle).map_err(|_| error())? * (0.99 + r() * 0.17);
                if spill {
                    radius = 0.17 + r() * 0.025;
                }
                let roll = r();
                let mut size = 0.0015 + r() * 0.002;
                if roll > 0.97 {
                    size = 0.006 + r() * 0.003;
                } else if roll > 0.75 {
                    size = 0.0035 + r() * 0.0025;
                }
                if spill {
                    size = size.min(0.004);
                }
                let x = p.center[0] + angle.cos() * radius;
                let y = p.center[1] + angle.sin() * radius;
                let ground = g.sample(x, y).map_err(|_| error())?;
                if ground.coverage < 0.99 || ground.border_distance < 0.006 + size * 0.5 {
                    continue;
                }
                accepted = Some((x, y, size));
                break;
            }
            let Some((x, y, size)) = accepted else {
                continue;
            };
            let up = DVec3::from_array(g.frame.direction(x, y).map_err(|_| error())?);
            let h = g.projected_height(p, x, y).map_err(|_| error())?;
            let end = up * (g.radius + h - size * (0.15 + r() * 0.2));
            let start =
                DVec3::from_array(g.frame.direction(x * 0.3, y * 0.3).map_err(|_| error())?)
                    * (g.radius + 0.004);
            let normal = DVec3::from_array(g.projected_normal(p, x, y).map_err(|_| error())?);
            let rotation = DQuat::from_rotation_arc(DVec3::Z, normal)
                * DQuat::from_euler(
                    EulerRot::XYZ,
                    (r() - 0.5) * 0.28,
                    (r() - 0.5) * 0.28,
                    r() * TAU,
                );
            let launch_rotation =
                rotation * DQuat::from_euler(EulerRot::XYZ, r() * 3., r() * 3., r() * 3.);
            let shade = 0.65 + r() * 0.4;
            let delay = 0.8 * r() * 0.12;
            let scale = DVec3::new(1., 0.65 + r() * 0.45, 0.6 + r() * 0.45) * (size / 0.04);
            let duration = 0.8 * (1. + r() * 0.38);
            let arc = 0.025 + r() * 0.04;
            self.fragments.push(Fragment {
                x,
                y,
                size,
                spill,
                start,
                end,
                up,
                rotation,
                launch_rotation,
                scale,
                color: [0.75 * shade, 0.52 * shade, 0.35 * shade],
                delay,
                duration,
                arc,
            });
        }
        Ok(self.next == 144)
    }
    pub(super) fn progress(&self) -> usize {
        self.next
    }
    pub fn take_ready(&mut self) -> Option<Vec<Fragment>> {
        if self.next != 144 || self.failed {
            return None;
        }
        self.failed = true;
        Some(std::mem::take(&mut self.fragments))
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    use openmars_scene::geometry::{BodyProjection, nuclear_site::BoardFrame};
    #[test]
    fn water_and_invalid_ground_cannot_publish_fragments() {
        let frame = BoardFrame::new(BodyProjection::MARS, [0, 0, 0], false).unwrap();
        let nearby = [frame.clone()];
        let terrain = |_| Ok(-0.024);
        let ground = CraterGround {
            frame: &frame,
            nearby: &nearby,
            radius: 2.02,
            terrain: &terrain,
        };
        let profile = NuclearProfile::new(37);
        let mut work = FragmentWork::new(37);
        for _ in 0..18 {
            work.advance(&profile, &ground).unwrap();
        }
        assert!(work.take_ready().unwrap().is_empty());
        let terrain = |_| Ok(f32::NAN);
        let invalid = CraterGround {
            frame: &frame,
            nearby: &nearby,
            radius: 2.02,
            terrain: &terrain,
        };
        let mut work = FragmentWork::new(37);
        assert!(work.advance(&profile, &invalid).is_err());
        assert!(work.take_ready().is_none());
    }
}
