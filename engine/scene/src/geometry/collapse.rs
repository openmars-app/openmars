//! Original nuclearTransitions.ts arithmetic. Part seeds are JavaScript numbers:
//! adding an index must not wrap at the u32 boundary of the source tile seed.
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct CollapseParameters {
    pub delay: f64,
    pub tilt: f64,
    pub direction: f64,
}
impl CollapseParameters {
    pub fn new(seed: u64) -> Self {
        let random = |salt: f64| {
            let value = (seed as f64 * 12.9898 + salt * 78.233).sin() * 43758.5453;
            value - value.floor()
        };
        Self {
            delay: random(1.) * 0.12,
            tilt: (2. + random(2.) * 4.) * std::f64::consts::PI / 180.,
            direction: random(3.) * std::f64::consts::TAU,
        }
    }
    /// `age` is visual time since impact: real seconds * .8 - .8.
    pub fn progress(self, age: f64) -> f64 {
        ((age - self.delay) / 0.75).clamp(0., 1.).powi(2)
    }
}
