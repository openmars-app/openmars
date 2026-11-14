//! Original fixed-capacity ship hover/trail/flame/exhaust distributions and
//! update equations. They consume canonical fixed steps, never render delta.
use super::flight::Pose;
type AssetError = String;
use bevy::math::DVec3;
const TAU: f64 = std::f64::consts::TAU;
const DT: f64 = 1. / 60.;

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Kind {
    Trail,
    Hover,
    Flame,
    Core,
    Exhaust,
}
impl Kind {
    pub fn mode(self) -> u32 {
        match self {
            Self::Trail => 0,
            Self::Hover => 1,
            Self::Flame => 2,
            Self::Core => 3,
            Self::Exhaust => 4,
        }
    }
    pub fn capacity(self, quality: f32) -> Result<usize, AssetError> {
        let count = match self {
            Self::Trail => 600,
            Self::Hover => 100,
            Self::Flame => 60,
            Self::Core => 1,
            Self::Exhaust => 20,
        };
        if !quality.is_finite() || !(0. ..=1.).contains(&quality) {
            return Err(error("ship particle quality exceeds source"));
        }
        Ok((f64::from(count) * f64::from(quality)).ceil().max(1.) as usize)
    }
}
fn error(s: &str) -> AssetError {
    s.into()
}
fn basis(normal: DVec3) -> (DVec3, DVec3) {
    let tangent = normal
        .cross(if normal.y.abs() < 0.9 {
            DVec3::Y
        } else {
            DVec3::X
        })
        .normalize();
    (tangent, normal.cross(tangent).normalize())
}
struct Random(u32);
impl Random {
    fn next(&mut self) -> f64 {
        self.0 = self.0.wrapping_add(0x6d2b79f5);
        let mut t = (self.0 ^ (self.0 >> 15)).wrapping_mul(1 | self.0);
        t ^= t.wrapping_add((t ^ (t >> 7)).wrapping_mul(61 | t));
        f64::from(t ^ (t >> 14)) / 4294967296.
    }
}
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Sprite {
    pub position: DVec3,
    pub scale: f64,
    pub rotation: f64,
    pub opacity: f64,
    pub life: f64,
}
impl Sprite {
    pub fn hidden() -> Self {
        Self {
            position: DVec3::ZERO,
            scale: 0.,
            rotation: 0.,
            opacity: 0.,
            life: 0.,
        }
    }
    /// Existing MeshTag instancing carries bounded opacity/rotation/life.
    /// CPU source values remain f64; only GPU payloads are quantized.
    pub fn tag(self) -> u32 {
        let a = (self.opacity.clamp(0., 1.) * 1023.).round() as u32;
        let r = (self.rotation.rem_euclid(TAU) / TAU * 1023.).round() as u32;
        let life = (self.life.clamp(0., 1.) * 4095.).round() as u32;
        a | (r << 10) | (life << 20)
    }
}
#[derive(Clone, Debug)]
struct Particle {
    position: DVec3,
    velocity: DVec3,
    rotation: f64,
    spin: f64,
    age: u16,
    max_age: f64,
    scale: f64,
    alive: bool,
    jitter: f64,
    identity: u32,
}
impl Particle {
    fn empty() -> Self {
        Self {
            position: DVec3::ZERO,
            velocity: DVec3::ZERO,
            rotation: 0.,
            spin: 0.,
            age: 0,
            max_age: 1.,
            scale: 0.,
            alive: false,
            jitter: 1.,
            identity: 0,
        }
    }
    fn sprite(&self, kind: Kind) -> Sprite {
        if !self.alive {
            return Sprite::hidden();
        }
        let t = f64::from(self.age) * DT / self.max_age;
        let (opacity, scale) = match kind {
            Kind::Trail => (
                (t * 6.).min(1.) * (1. - t.powf(1.2)) * 0.25,
                self.scale * (1. + t * 4.),
            ),
            Kind::Hover => (
                (t * 5.).min(1.) * (1. - t.powf(1.3)) * 0.5,
                self.scale * (1. + t * 3.),
            ),
            Kind::Flame => (
                (t * 10.).min(1.) * (1. - t.powf(1.5)) * self.jitter * 0.8,
                self.scale * (1. + t * 0.3) * (1. - t * 0.3),
            ),
            Kind::Exhaust => (
                (t * 5.).min(1.) * (1. - t * t) * 0.6,
                self.scale * (1. + t * 2.) * 0.1,
            ),
            Kind::Core => return Sprite::hidden(),
        };
        Sprite {
            position: self.position,
            rotation: self.rotation,
            opacity,
            scale,
            life: t,
        }
    }
}
pub struct Pool {
    kind: Kind,
    particles: Vec<Particle>,
    seed: u32,
    ring: usize,
    previous: Option<DVec3>,
    distance: f64,
    last_step: Option<u64>,
}
impl Pool {
    pub fn new(kind: Kind, quality: f32, seed: u32) -> Result<Self, AssetError> {
        let count = kind.capacity(quality)?;
        Ok(Self {
            kind,
            particles: vec![Particle::empty(); count],
            seed,
            ring: 0,
            previous: None,
            distance: 0.,
            last_step: None,
        })
    }
    pub fn capacity(&self) -> usize {
        self.particles.len()
    }
    pub fn sprites(&self) -> impl Iterator<Item = Sprite> + '_ {
        self.particles.iter().map(|p| p.sprite(self.kind))
    }
    pub fn alive(&self) -> bool {
        self.particles.iter().any(|p| p.alive)
    }
    pub fn clear(&mut self) {
        for p in &mut self.particles {
            p.alive = false;
        }
        self.ring = 0;
        self.previous = None;
        self.distance = 0.;
        self.last_step = None;
    }
    fn slot(&mut self, recycle: bool) -> Option<usize> {
        if !recycle {
            return self.particles.iter().position(|p| !p.alive);
        }
        for _ in 0..self.particles.len() {
            let i = self.ring % self.particles.len();
            self.ring = self.ring.wrapping_add(1);
            if !self.particles[i].alive {
                return Some(i);
            }
        }
        let i = self.ring % self.particles.len();
        self.ring = self.ring.wrapping_add(1);
        Some(i)
    }
    fn spawn(
        &mut self,
        random: &mut Random,
        position: DVec3,
        normal: DVec3,
        forward: Option<DVec3>,
        intensity: f64,
    ) {
        let Some(i) = self.slot(self.kind == Kind::Trail) else {
            return;
        };
        let p = &mut self.particles[i];
        p.identity = random.0;
        p.alive = true;
        p.age = 0;
        p.position = position;
        p.jitter = 1.;
        match self.kind {
            Kind::Hover => {
                let (t, b) = basis(normal);
                let angle = random.next() * TAU;
                let radius = 0.04 + random.next() * 0.04;
                p.position += t * (angle.cos() * radius) + b * (angle.sin() * radius);
                let speed = 0.015 + random.next() * 0.015;
                p.velocity = t * (angle.cos() * speed)
                    + b * (angle.sin() * speed)
                    + normal * (0.003 + random.next() * 0.003);
                p.max_age = 0.8 + random.next() * 0.7;
                p.scale = 0.015 + random.next() * 0.015;
                p.rotation = random.next() * TAU;
                p.spin = (random.next() - 0.5) * 0.8;
            }
            Kind::Trail => {
                if let Some(forward) = forward {
                    p.velocity = forward * (-(0.15 + random.next() * 0.08) * intensity);
                    p.velocity += DVec3::new(
                        random.next() - 0.5,
                        random.next() - 0.5,
                        random.next() - 0.5,
                    ) * 0.01;
                } else {
                    p.velocity = DVec3::new(
                        random.next() - 0.5,
                        random.next() - 0.5,
                        random.next() - 0.5,
                    ) * 0.006;
                }
                p.max_age = 0.6 + random.next() * 0.6;
                p.scale = if forward.is_some() {
                    0.025 + random.next() * 0.025
                } else {
                    0.02 + random.next() * 0.02
                };
                p.rotation = random.next() * TAU;
                p.spin = (random.next() - 0.5) * 0.6;
            }
            Kind::Flame => {
                let (t, b) = basis(normal);
                let angle = random.next() * TAU;
                let radius = random.next() * 0.008;
                p.position += t * (angle.cos() * radius) + b * (angle.sin() * radius);
                let speed = (0.15 + random.next() * 0.15) * (0.5 + intensity * 0.5);
                p.velocity = normal * speed
                    + t * ((random.next() - 0.5) * 0.04)
                    + b * ((random.next() - 0.5) * 0.04);
                p.max_age = 0.08 + random.next() * 0.12;
                p.scale = 0.012 + random.next() * 0.008;
                p.rotation = random.next() * TAU;
                p.spin = random.next() - 0.5;
            }
            Kind::Exhaust => {
                let (t, b) = basis(normal);
                let angle = random.next() * TAU;
                let radius = 0.04 + random.next() * 0.06;
                p.position += t * (angle.cos() * radius) + b * (angle.sin() * radius);
                p.rotation = random.next() * TAU;
                p.scale = 0.2 + random.next() * 0.3;
                let spread = random.next() * TAU;
                let speed = 0.02 + random.next() * 0.02;
                p.velocity = t * (spread.cos() * speed) + b * (spread.sin() * speed)
                    - normal * (0.005 + random.next() * 0.005);
                p.spin = (random.next() - 0.5) * 1.4;
                p.max_age = 0.5 + random.next() * 0.8;
            }
            Kind::Core => {
                p.alive = false;
            }
        }
    }
    /// Exactly one canonical source update. Caller bounds/reconstructs a seek
    /// before invoking this; repeated same-step calls cannot emit or advance.
    pub fn step(
        &mut self,
        step: u64,
        pose: Pose,
        origin: DVec3,
        normal: DVec3,
    ) -> Result<(), AssetError> {
        if !pose.position.is_finite()
            || !pose.rotation.is_finite()
            || !origin.is_finite()
            || !normal.is_finite()
            || (normal.length_squared() - 1.).abs() > 1e-6
        {
            return Err(error("ship particle pose"));
        }
        if self.last_step == Some(step) {
            return Ok(());
        }
        if self
            .last_step
            .is_some_and(|last| last.checked_add(1) != Some(step))
        {
            return Err(error("ship particle step requires bounded reconstruction"));
        }
        self.last_step = Some(step);
        let mut random =
            Random(self.seed ^ (step as u32).wrapping_mul(0x9e3779b9) ^ (step >> 32) as u32);
        let rear = -pose.forward();
        match self.kind {
            Kind::Hover => {
                if step < 36 {
                    for _ in 0..2 {
                        self.spawn(&mut random, origin, normal, None, 1.);
                    }
                }
            }
            Kind::Flame => {
                if (36..super::flight::COMPLETE).contains(&step) {
                    let intensity = ((step - 35) as f64 / 30.).min(1.).powi(2);
                    let count = (4. * intensity).round().max(1.) as usize;
                    for _ in 0..count {
                        self.spawn(
                            &mut random,
                            pose.position + rear * 0.035,
                            rear,
                            None,
                            intensity,
                        );
                    }
                }
            }
            Kind::Trail => {
                if step > 72 && step < 162 {
                    let elapsed = step - 72;
                    let backward = (elapsed < 72).then_some(pose.forward());
                    let intensity = 1. - elapsed as f64 / 72.;
                    let per_point = if backward.is_some() { 1 } else { 4 };
                    if self.previous.is_none() {
                        self.previous = Some(pose.position);
                        for _ in 0..per_point {
                            self.spawn(&mut random, pose.position, normal, backward, intensity);
                        }
                    }
                    let previous = self.previous.unwrap();
                    let distance = previous.distance(pose.position);
                    if elapsed < 72 {
                        self.spawn(&mut random, pose.position, normal, backward, intensity);
                    }
                    let total = self.distance + distance;
                    let count = (total / 0.03).floor() as usize;
                    self.distance = total - count as f64 * 0.03;
                    if count > 4096 {
                        return Err(error("ship trail emission distance bound"));
                    }
                    if distance > 0. {
                        for point in 0..count {
                            let along =
                                (point + 1) as f64 * 0.03 - (total - distance - self.distance);
                            let at = previous.lerp(pose.position, (along / distance).clamp(0., 1.));
                            for _ in 0..per_point {
                                self.spawn(&mut random, at, normal, backward, intensity);
                            }
                        }
                    }
                    self.previous = Some(pose.position);
                }
            }
            Kind::Exhaust if step == 0 => {
                for _ in 0..self.particles.len() {
                    self.spawn(&mut random, origin, normal, None, 1.);
                }
            }
            _ => {}
        }
        for p in self.particles.iter_mut() {
            if !p.alive {
                continue;
            }
            p.age = p.age.saturating_add(1);
            if f64::from(p.age) * DT >= p.max_age {
                p.alive = false;
                continue;
            }
            p.position += p.velocity * DT;
            p.rotation += p.spin * DT;
            if self.kind == Kind::Hover {
                p.velocity *= 1. - 0.5 * DT;
            }
            if self.kind == Kind::Exhaust {
                p.velocity *= 0.97;
            }
            if self.kind == Kind::Flame {
                // Source opacity jitter is local to a fixed step/particle.
                let mut jitter = Random(
                    self.seed
                        ^ (step as u32).wrapping_mul(0x85ebca6b)
                        ^ p.identity.wrapping_mul(0xc2b2ae35),
                );
                p.jitter = 0.85 + jitter.next() * 0.15;
            }
        }
        // Original unmounts hover after liftoff and the whole flight's flame,
        // trail and core at completion; it never emits a resumed catch-up burst.
        if self.kind == Kind::Hover && step > 72
            || matches!(self.kind, Kind::Flame | Kind::Trail) && step >= super::flight::COMPLETE
        {
            for p in &mut self.particles {
                p.alive = false;
            }
        }
        Ok(())
    }
    pub fn core(step: u64, world_step: u64, pose: Pose) -> Sprite {
        if !(36..super::flight::COMPLETE).contains(&step) {
            return Sprite::hidden();
        }
        let time = world_step as f64 * DT;
        let intensity = ((step - 35) as f64 / 30.).min(1.).powi(2);
        Sprite {
            position: pose.position - pose.forward() * 0.035,
            rotation: 0.,
            scale: 0.025 * (0.9 + 0.1 * (time * 15.).sin()),
            opacity: 0.9 * intensity * (0.85 + 0.15 * (time * 20.).sin()),
            life: 0.,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn bounded_pools_pause_without_reemission_keep_source_counts_and_retire() {
        let pose = Pose {
            position: DVec3::new(0., 0., 2.035),
            rotation: bevy::math::DQuat::IDENTITY,
        };
        for (kind, count) in [
            (Kind::Hover, 100),
            (Kind::Trail, 600),
            (Kind::Flame, 60),
            (Kind::Exhaust, 20),
        ] {
            let mut pool = Pool::new(kind, 1., 37).unwrap();
            assert_eq!(pool.capacity(), count);
            for step in 0..super::super::flight::COMPLETE + 2 {
                pool.step(step, pose, DVec3::new(0., 0., 2.02), DVec3::Z)
                    .unwrap();
                let first = pool.sprites().collect::<Vec<_>>();
                pool.step(step, pose, DVec3::ZERO, DVec3::Y).unwrap();
                assert_eq!(first, pool.sprites().collect::<Vec<_>>());
                assert!(first.iter().all(|s| s.position.is_finite()
                    && s.scale.is_finite()
                    && s.opacity >= 0.
                    && s.opacity <= 1.));
            }
            assert!(!pool.alive());
            pool.clear();
            assert!(pool.step(u64::MAX, pose, DVec3::ZERO, DVec3::Y).is_ok());
        }
        assert!(Pool::new(Kind::Trail, 1.01, 0).is_err());
    }
}
