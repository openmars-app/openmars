//! Original seeded crater profile and ground blending. Fixed arrays retain
//! every authored lobe/fracture/gouge without a global profile cache or renderer.
use super::{GeometryError, placement::random_sequence};
use std::f64::consts::TAU;
pub const SEGMENTS: usize = 128;
#[derive(Clone, Debug, PartialEq)]
pub struct NuclearProfile {
    pub center: [f64; 2],
    pub floor_offset: [f64; 2],
    pub depth: f64,
    pub rotation: f64,
    pub aspect: f64,
    radii: [f64; SEGMENTS],
    rim_heights: [f64; SEGMENTS],
    breaches: [[f64; 3]; 5],
    breach_count: usize,
    pub fans: [f64; 3],
    pub gouges: [[f64; 4]; 5],
}
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct GroundSample {
    pub height: f64,
    pub coverage: f64,
    pub border_distance: f64,
}
impl GroundSample {
    pub const BARE: Self = Self {
        height: 0.0003,
        coverage: 1.,
        border_distance: 1.,
    };
    pub fn validate(self) -> Result<Self, GeometryError> {
        if ![self.height, self.coverage, self.border_distance]
            .into_iter()
            .all(f64::is_finite)
        {
            return Err(GeometryError::NonFinite);
        }
        if !(0.0..=1.0).contains(&self.coverage) || self.border_distance < 0. {
            return Err(GeometryError::InvalidHeight);
        }
        Ok(self)
    }
}
fn smooth(a: f64, b: f64, x: f64) -> f64 {
    let t = ((x - a) / (b - a)).clamp(0., 1.);
    t * t * (3. - 2. * t)
}
fn angle_distance(a: f64, b: f64) -> f64 {
    (a - b).sin().atan2((a - b).cos())
}
fn angle_sample(values: &[f64; SEGMENTS], angle: f64) -> Result<f64, GeometryError> {
    if !angle.is_finite() {
        return Err(GeometryError::NonFinite);
    }
    // Keep the original two modulo operations, including negative angles.
    let sample = (((angle % TAU) + TAU) % TAU) / TAU * SEGMENTS as f64;
    let index = sample.floor() as usize;
    Ok(values[index] + (values[(index + 1) % SEGMENTS] - values[index]) * (sample - index as f64))
}
impl NuclearProfile {
    pub fn new(seed: u32) -> Self {
        let mut random = random_sequence(seed);
        let rotation = random() * TAU;
        let aspect = 1.08 + random() * 0.12;
        let center = [(random() - 0.5) * 0.009, (random() - 0.5) * 0.009];
        let floor_offset = [(random() - 0.5) * 0.026, (random() - 0.5) * 0.026];
        let depth = 0.014 + random() * 0.004;
        let count = 5 + (random() * 3.).floor() as usize;
        let mut lobes = [[0.; 3]; 7];
        for (i, lobe) in lobes[..count].iter_mut().enumerate() {
            *lobe = [
                rotation + (i as f64 + random() * 0.65) * TAU / count as f64,
                0.36 + random() * 0.3,
                if i % 2 == 0 { -1. } else { 1. } * (0.005 + random() * 0.007),
            ];
        }
        let fans = std::array::from_fn(|i| rotation + (i as f64 + random() * 0.6) * TAU / 3.);
        let radii = std::array::from_fn(|i| {
            let angle = i as f64 * TAU / SEGMENTS as f64;
            let c = (angle - rotation).cos();
            let s = (angle - rotation).sin();
            let mut radius = 0.11 / (c * c + aspect * aspect * s * s).sqrt();
            for lobe in &lobes[..count] {
                radius += lobe[2] * (-(angle_distance(angle, lobe[0]) / lobe[1]).powi(2)).exp();
            }
            radius.clamp(0.079, 0.121)
        });
        let gouges = std::array::from_fn(|i| {
            let angle = fans[i % 3] + (random() - 0.5) * 0.45;
            let start = 0.052 + random() * 0.024;
            let end = start + 0.025 + random() * 0.025;
            [
                center[0] + angle.cos() * start,
                center[1] + angle.sin() * start,
                center[0] + (angle + (random() - 0.5) * 0.2).cos() * end,
                center[1] + (angle + (random() - 0.5) * 0.2).sin() * end,
            ]
        });
        let count = 20 + (random() * 11.).floor() as usize;
        let mut fractures = [[0.; 3]; 30];
        for (i, f) in fractures[..count].iter_mut().enumerate() {
            *f = [
                rotation + (i as f64 + random() * 0.8) * TAU / count as f64,
                0.035 + random() * 0.075,
                (random() - 0.35) * 0.0024,
            ];
        }
        let breach_count = 3 + (random() * 3.).floor() as usize;
        let mut breaches = [[0.; 3]; 5];
        for (i, b) in breaches[..breach_count].iter_mut().enumerate() {
            *b = [
                fans[i % 3] + (random() - 0.5) * 0.8,
                0.1 + random() * 0.13,
                0.4 + random() * 0.25,
            ];
        }
        let rim_heights = std::array::from_fn(|i| {
            let angle = i as f64 * TAU / SEGMENTS as f64;
            let mut h = 0.0075 + (angle * 3. + rotation).sin() * 0.001;
            for f in &fractures[..count] {
                h += f[2] * (1. - angle_distance(angle, f[0]).abs() / f[1]).max(0.);
            }
            for b in &breaches[..breach_count] {
                h *= 1. - b[2] * (-(angle_distance(angle, b[0]) / b[1]).powi(2)).exp();
            }
            h.clamp(0.0015, 0.0095) * 1.65
        });
        Self {
            center,
            floor_offset,
            depth,
            rotation,
            aspect,
            radii,
            rim_heights,
            breaches,
            breach_count,
            fans,
            gouges,
        }
    }
    pub fn radii(&self) -> &[f64; SEGMENTS] {
        &self.radii
    }
    pub fn rim_heights(&self) -> &[f64; SEGMENTS] {
        &self.rim_heights
    }
    pub fn breaches(&self) -> &[[f64; 3]] {
        &self.breaches[..self.breach_count]
    }
    pub fn radius(&self, angle: f64) -> Result<f64, GeometryError> {
        angle_sample(&self.radii, angle)
    }
    pub fn mound_radius(&self, angle: f64) -> Result<f64, GeometryError> {
        Ok(
            (self.radius(angle)? + 0.023 + 0.004 * (angle * 5. + self.rotation).sin())
                .min(0.134 - self.center[0].hypot(self.center[1])),
        )
    }
    pub fn fan(&self, angle: f64) -> Result<f64, GeometryError> {
        if !angle.is_finite() {
            return Err(GeometryError::NonFinite);
        }
        Ok(self
            .fans
            .iter()
            .map(|f| (-(angle_distance(angle, *f) / 0.25).powi(2)).exp())
            .fold(0., f64::max))
    }
    fn position(&self, x: f64, y: f64) -> Result<(f64, f64, f64), GeometryError> {
        if !x.is_finite() || !y.is_finite() {
            return Err(GeometryError::NonFinite);
        }
        if x.abs() > 10000. || y.abs() > 10000. {
            return Err(GeometryError::InvalidHeight);
        }
        let (dx, dy) = (x - self.center[0], y - self.center[1]);
        let angle = dy.atan2(dx);
        let radius = self.radius(angle)?;
        Ok((
            angle,
            dx.hypot(dy) / radius,
            self.mound_radius(angle)? / radius,
        ))
    }
    pub fn surface_height(&self, x: f64, y: f64) -> Result<f64, GeometryError> {
        let (angle, r, foot) = self.position(x, y)?;
        let (dx, dy) = (x - self.center[0], y - self.center[1]);
        let bowl = -self.depth * (1. - smooth(0.46, 1., r));
        let floor_noise = ((x * 120. + self.rotation).sin() * (y * 93. - self.rotation).sin()
            + (x * 61. - y * 87.).sin())
            * 0.0006;
        let hollow = -0.0012
            * (-((dx - self.floor_offset[0]).powi(2) + (dy - self.floor_offset[1]).powi(2))
                / 0.00025)
                .exp();
        let inner = ((r - 0.62) / 0.38).clamp(0., 1.);
        let outer = 1. - ((r - 1.) / (foot - 1.)).clamp(0., 1.);
        let rim = angle_sample(&self.rim_heights, angle)? * inner * outer * outer;
        Ok(0.0008 + bowl + (floor_noise + hollow) * (1. - smooth(0.45, 0.9, r)) + rim)
    }
    pub fn ground_height(
        &self,
        x: f64,
        y: f64,
        mut ground: impl FnMut(f64, f64) -> Result<GroundSample, GeometryError>,
    ) -> Result<f64, GeometryError> {
        let (_, r, foot) = self.position(x, y)?;
        let center = ground(self.center[0], self.center[1])?.validate()?;
        let here = ground(x, y)?.validate()?;
        let height = self.surface_height(x, y)?
            + (center.height - 0.0003) * (1. - smooth(0.7, foot, r))
            + (here.height + 0.0006 - 0.0008) * smooth(0.7, foot, r);
        if !height.is_finite() {
            return Err(GeometryError::NonFinite);
        }
        Ok(height)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn seeded_profiles_retain_bounded_complete_relief_without_global_state() {
        for seed in [0, 1, 37, u32::MAX, 0x5482, 0x88351] {
            let p = NuclearProfile::new(seed);
            assert_eq!(p, NuclearProfile::new(seed));
            assert_eq!(p.radii().len(), 128);
            assert_eq!(p.rim_heights().len(), 128);
            assert!((3..=5).contains(&p.breaches().len()));
            assert!(p.radii().iter().all(|v| (0.079..=0.121).contains(v)));
            assert!(
                p.rim_heights()
                    .iter()
                    .all(|v| (0.0015 * 1.65..=0.0095 * 1.65).contains(v))
            );
            let floor = p.surface_height(p.center[0], p.center[1]).unwrap();
            assert!(floor < -0.01);
            for i in -256..256 {
                let angle = i as f64 * TAU / 128.;
                assert!((p.radius(angle).unwrap() - p.radius(angle + TAU).unwrap()).abs() < 1e-14);
            }
            assert!(
                (p.ground_height(0.3, 0.3, |_, _| Ok(GroundSample {
                    height: 0.07,
                    ..GroundSample::BARE
                }))
                .unwrap()
                    - 0.0706)
                    .abs()
                    < 1e-12
            );
        }
    }
    #[test]
    fn invalid_ground_and_nonfinite_geometry_never_become_a_surface() {
        let p = NuclearProfile::new(37);
        assert!(p.radius(f64::NAN).is_err());
        assert!(p.surface_height(f64::INFINITY, 0.).is_err());
        assert!(
            p.ground_height(0., 0., |_, _| Ok(GroundSample {
                height: f64::NAN,
                ..GroundSample::BARE
            }))
            .is_err()
        );
        assert!(
            p.ground_height(0., 0., |_, _| Ok(GroundSample {
                coverage: 2.,
                ..GroundSample::BARE
            }))
            .is_err()
        );
        assert!(
            p.ground_height(0., 0., |_, _| Err(GeometryError::InvalidHeight))
                .is_err()
        );
    }
}
