//! Original local particle pools at the canonical 60Hz world clock. No wall
//! clock, frame-sized integration, global RNG or unbounded catch-up allocation.
use bevy::math::DVec3;
use openmars_scene::geometry::placement::random_sequence;
#[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord)]
pub enum Kind {
    Placement,
    Volcano,
    Mining,
    Mohole,
}
impl Kind {
    pub fn count(self) -> u16 {
        match self {
            Self::Placement => 45,
            Self::Volcano => 28,
            Self::Mining => 18,
            Self::Mohole => 25,
        }
    }
    pub fn color(self) -> [f32; 3] {
        match self {
            Self::Placement => [0.58, 0.43, 0.32],
            Self::Volcano => [0.03, 0.025, 0.02],
            Self::Mining => [0.45, 0.35, 0.2],
            Self::Mohole => [0.2, 0.18, 0.15],
        }
    }
    pub fn index(self) -> u32 {
        match self {
            Self::Placement => 0,
            Self::Volcano => 1,
            Self::Mining => 2,
            Self::Mohole => 3,
        }
    }
}
#[derive(Clone, Debug, PartialEq)]
pub struct Particle {
    pub position: DVec3,
    velocity: DVec3,
    pub rotation: f64,
    rotation_speed: f64,
    age: f64,
    max_age: f64,
    initial_scale: f64,
    delay: f64,
    wave_phase: f64,
    pub scale: f64,
    pub opacity: f64,
}
impl Particle {
    fn empty() -> Self {
        Self {
            position: DVec3::ZERO,
            velocity: DVec3::ZERO,
            rotation: 0.,
            rotation_speed: 0.,
            age: 0.,
            max_age: 1.,
            initial_scale: 0.,
            delay: 0.,
            wave_phase: 0.,
            scale: 0.,
            opacity: 0.,
        }
    }
    pub fn tag(&self, kind: Kind, visible: bool) -> u32 {
        let opacity = (self.opacity.clamp(0., 1.) * 4095.).round() as u32;
        let rotation = (self.rotation.rem_euclid(std::f64::consts::TAU) / std::f64::consts::TAU
            * 16383.)
            .round() as u32;
        opacity | (rotation << 12) | (kind.index() << 26) | if visible { 0 } else { 1 << 31 }
    }
}
fn error(s: &str) -> String {
    s.into()
}
pub struct Pool {
    kind: Kind,
    random: Box<dyn FnMut() -> f64 + Send + Sync>,
    particles: Vec<Particle>,
    step: u64,
    born: u64,
    duration: f64,
}
impl Pool {
    /// The owner prepays pool, descriptors and ECS copies before construction.
    pub fn new(
        kind: Kind,
        seed: u32,
        count: u16,
        born: u64,
        newly: bool,
        duration: f64,
    ) -> Result<Self, String> {
        if count == 0
            || count > kind.count()
            || !duration.is_finite()
            || !(0.1..=10.).contains(&duration)
        {
            return Err(error("local particle pool limits"));
        }
        born.checked_add(600)
            .ok_or_else(|| error("local particle deadline"))?;
        let mut random = Box::new(random_sequence(seed)) as Box<dyn FnMut() -> f64 + Send + Sync>;
        let mut particles = Vec::with_capacity(count as usize);
        for i in 0..count {
            let mut p = Particle::empty();
            if kind == Kind::Placement {
                p.rotation = random() * std::f64::consts::TAU;
                let a = random() * std::f64::consts::TAU;
                let radius = random().sqrt() * 0.13;
                p.position = DVec3::new(a.cos() * radius, a.sin() * radius, 0.008);
                p.initial_scale = 0.045 + random() * 0.06;
                p.scale = p.initial_scale;
                let spread = 0.015 + random() * 0.025;
                p.velocity =
                    DVec3::new(a.cos() * spread, a.sin() * spread, 0.025 + random() * 0.035);
                p.rotation_speed = (random() - 0.5) * 0.8;
                p.max_age = duration * (0.65 + random() * 0.35);
                p.opacity = 0.08;
            } else {
                // The original mohole constructor consumes its temporary maxAge
                // before reset and prepopulates old snapshots with varied ages.
                if kind == Kind::Mohole {
                    let _ = 6. + random() * 4.;
                }
                reset(kind, &mut p, 0., &mut random);
                p.delay = match kind {
                    Kind::Volcano => f64::from(i) / f64::from(count) * 12.,
                    Kind::Mining => f64::from(i) / f64::from(count) * 8.,
                    Kind::Mohole if newly => 1. + f64::from(i) / f64::from(count) * 2.,
                    _ => 0.,
                };
                if kind == Kind::Mohole && !newly {
                    p.age = random() * p.max_age;
                    p.wave_phase = -p.age;
                }
            }
            particles.push(p);
        }
        Ok(Self {
            kind,
            random,
            particles,
            step: born,
            born,
            duration,
        })
    }
    pub fn particles(&self) -> &[Particle] {
        &self.particles
    }
    pub fn born(&self) -> u64 {
        self.born
    }
    pub fn done(&self, step: u64) -> bool {
        self.kind == Kind::Placement
            && step >= self.born
            && ((step - self.born) as f64 / 60.) >= self.duration
    }
    /// At most eight complete fixed ticks per call. Deferred work retains the
    /// same pool/RNG; callers never restart it or integrate a large wall delta.
    pub fn advance(&mut self, target: u64) -> Result<bool, String> {
        if target < self.step {
            return Err(error("local particle clock regressed"));
        }
        for _ in 0..8 {
            if self.step == target || self.done(self.step) {
                break;
            }
            self.step += 1;
            let elapsed = (self.step - self.born) as f64 / 60.;
            for p in &mut self.particles {
                if p.delay > 0. {
                    p.delay -= 1. / 60.;
                    p.opacity = 0.;
                    continue;
                }
                if self.kind == Kind::Placement {
                    p.age = elapsed;
                } else {
                    p.age += 1. / 60.;
                    if p.age >= p.max_age {
                        reset(self.kind, p, elapsed, &mut self.random);
                        continue;
                    }
                }
                let life = (p.age / p.max_age).min(1.);
                if self.kind == Kind::Volcano {
                    p.velocity.x += 0.008 / 60.;
                    p.velocity.y += 0.003 / 60.;
                }
                p.position += p.velocity / 60.;
                p.rotation += if self.kind == Kind::Mining {
                    0.2 / 60.
                } else {
                    p.rotation_speed / 60.
                };
                match self.kind {
                    Kind::Placement => {
                        p.opacity = (0.12 + life * 10.).min(1.) * (1. - life * life) * 0.65;
                        p.scale = p.initial_scale * (1. + life * 1.5);
                        p.velocity *= (-1.2_f64 / 60.).exp();
                    }
                    Kind::Volcano => {
                        p.opacity = (life * 6.).min(1.) * (1. - life.powf(1.5)) * 0.55;
                        p.scale = p.initial_scale * (1. + life * 5.);
                    }
                    Kind::Mining => {
                        p.opacity = (life * 5.).min(1.) * (1. - life.powf(1.5)) * 0.35;
                        p.scale = p.initial_scale * (1. + life * 3.);
                    }
                    Kind::Mohole => {
                        let birth = (p.wave_phase * 1.6).sin() * 0.5 + 0.5;
                        let wave = (elapsed * 1.6).sin() * 0.5 + 0.5;
                        p.opacity = (life * 5.).min(1.)
                            * (1. - life.powf(1.5))
                            * 0.35
                            * (0.4 + birth * 0.6)
                            * (0.4 + wave * 0.6);
                        p.scale = p.initial_scale * (1. + life * 2.5);
                    }
                }
                if matches!(self.kind, Kind::Volcano | Kind::Mohole) {
                    p.velocity.x *= 1. - 0.3 / 60.;
                    p.velocity.y *= 1. - 0.3 / 60.;
                }
            }
        }
        Ok(self.step == target || self.done(self.step))
    }
}
fn reset(
    kind: Kind,
    p: &mut Particle,
    time: f64,
    random: &mut Box<dyn FnMut() -> f64 + Send + Sync>,
) {
    use std::f64::consts::TAU;
    let a = random() * TAU;
    let radius = random()
        * match kind {
            Kind::Volcano => 0.012,
            Kind::Mining => 0.08,
            _ => 0.018,
        };
    p.position = DVec3::new(
        a.cos() * radius,
        a.sin() * radius,
        if kind == Kind::Volcano { 0. } else { 0.005 },
    );
    p.age = 0.;
    p.max_age = match kind {
        Kind::Volcano => 9. + random() * 6.,
        Kind::Mining => 4. + random() * 5.,
        _ => 6. + random() * 4.,
    };
    p.initial_scale = match kind {
        Kind::Volcano => 0.035 + random() * 0.025,
        Kind::Mining => 0.02 + random() * 0.015,
        _ => 0.025 + random() * 0.02,
    };
    p.scale = p.initial_scale;
    let spread = if kind == Kind::Mohole {
        0.0002 + random() * 0.0004
    } else {
        0.001 + random() * 0.002
    };
    let angle = random() * TAU;
    let z = match kind {
        Kind::Volcano => 0.012 + random() * 0.008,
        Kind::Mining => 0.003 + random() * 0.004,
        _ => 0.005 + random() * 0.004,
    };
    p.velocity = DVec3::new(angle.cos() * spread, angle.sin() * spread, z);
    p.rotation_speed = if kind == Kind::Mining {
        0.2
    } else {
        (random() - 0.5) * 0.4
    };
    p.rotation = random() * TAU;
    p.opacity = 0.;
    p.wave_phase = time;
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn fixed_tick_chunking_pause_and_pool_bounds_preserve_original_rng() {
        for kind in [Kind::Placement, Kind::Volcano, Kind::Mining, Kind::Mohole] {
            let mut a = Pool::new(kind, 37, kind.count(), 0, true, 3.).unwrap();
            let mut b = Pool::new(kind, 37, kind.count(), 0, true, 3.).unwrap();
            for step in 1..=1200 {
                a.advance(step).unwrap();
            }
            while !b.advance(1200).unwrap() {}
            assert_eq!(a.particles(), b.particles());
            let old = b.particles().to_vec();
            b.advance(1200).unwrap();
            assert_eq!(old, b.particles());
            assert!(b.advance(0).is_err());
            assert!(a.particles().iter().all(|p| p.position.is_finite()
                && p.rotation.is_finite()
                && p.scale.is_finite()
                && (0.0..=1.0).contains(&p.opacity)));
            assert!(Pool::new(kind, 0, kind.count() + 1, 0, true, 3.).is_err());
        }
    }
    #[test]
    fn placement_expires_and_hidden_or_palette_tags_do_not_mix() {
        let mut a = Pool::new(Kind::Placement, 42, 45, 10, true, 2.6).unwrap();
        assert!(!a.done(165));
        assert!(a.done(166));
        while !a.advance(166).unwrap() {}
        let p = &a.particles()[0];
        assert_eq!(
            p.tag(Kind::Mohole, false) ^ p.tag(Kind::Mohole, true),
            1 << 31
        );
        assert_eq!((p.tag(Kind::Mohole, true) >> 26) & 15, 3);
        assert!(Pool::new(Kind::Placement, 42, 45, u64::MAX, true, 2.6).is_err());
    }
}
