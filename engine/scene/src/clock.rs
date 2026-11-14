//! Canonical presentation time. The host omits paused/hidden wall time.
#[derive(Default)]
pub struct SceneClock {
    seconds: f64,
}
impl SceneClock {
    pub fn advance(&mut self, delta: f64) -> Result<(), &'static str> {
        if !delta.is_finite() || !(0.0..=0.25).contains(&delta) {
            return Err("Invalid presentation delta");
        }
        self.seconds += delta;
        Ok(())
    }
    pub fn step(&self) -> u64 {
        (self.seconds * 60.0 + 1e-9).floor() as u64
    }
    pub fn seconds(&self) -> f64 {
        self.step() as f64 / 60.0
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn clock_partition_pause_and_invalid_deltas_preserve_fixed_steps() {
        let mut a = SceneClock::default();
        let mut b = SceneClock::default();
        for _ in 0..600 {
            a.advance(1.0 / 60.0).unwrap();
        }
        for _ in 0..1200 {
            b.advance(1.0 / 120.0).unwrap();
        }
        assert_eq!(a.step(), 600);
        assert_eq!(a.step(), b.step());
        for _ in 0..60 {
            a.advance(0.0).unwrap();
        }
        assert_eq!(a.seconds(), 10.0);
        for delta in [f64::NAN, f64::INFINITY, -0.1, 0.251] {
            assert!(a.advance(delta).is_err());
        }
        assert_eq!(a.step(), 600);
    }
}
