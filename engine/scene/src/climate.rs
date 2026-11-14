// Adapted from the sibling visual climate runtime, independent of transport/rules.
//! Visual climate only. Parameters come from the authoritative snapshot; these
//! thresholds affect appearance and never card eligibility or legal moves.
fn clamp(x: f64) -> f64 {
    x.clamp(0.0, 1.0)
}
fn smooth(a: f64, b: f64, x: f64) -> f64 {
    let t = clamp((x - a) / (b - a));
    t * t * (3.0 - 2.0 * t)
}
use crate::geometry::terrain::TerrainError;

#[derive(Clone, Copy, Debug, PartialEq)]
pub struct ClimateParameters {
    pub temperature: f64,
    pub oxygen: f64,
    pub oceans: f64,
    pub max_oceans: f64,
}
impl ClimateParameters {
    pub const BARREN: Self = Self {
        temperature: -30.0,
        oxygen: 0.0,
        oceans: 0.0,
        max_oceans: 9.0,
    };
    pub const TERRAFORMED: Self = Self {
        temperature: 8.0,
        oxygen: 14.0,
        oceans: 9.0,
        max_oceans: 9.0,
    };
    fn validate(self) -> Result<Self, TerrainError> {
        if ![self.temperature, self.oxygen, self.oceans, self.max_oceans]
            .into_iter()
            .all(f64::is_finite)
        {
            return Err(TerrainError::NonFinite);
        }
        if self.max_oceans <= 0.0 || self.oceans < 0.0 {
            return Err(TerrainError::OutsideDomain);
        }
        Ok(self)
    }
    fn terraformed(self) -> bool {
        self.temperature >= 8.0 && self.oxygen >= 14.0 && self.oceans >= self.max_oceans
    }
}
macro_rules! climate_state {($($field:ident),+)=> {
    #[derive(Clone,Copy,Debug,PartialEq)]
    pub struct ClimateState {$(pub $field:f64),+}
    impl ClimateState {
        fn ease(&mut self,target:Self,dt:f64) {
            $(let delta=target.$field-self.$field;
              let seconds=if matches!(stringify!($field),"frost"|"ice"){0.6}else{1.0};
              self.$field=if delta.abs()<1e-4 {target.$field}else{self.$field+delta*(1.0-(-dt/seconds).exp())};)+
        }
        pub fn channels(self)->[f64;18] {[$(self.$field),+]}
    }
}}
climate_state!(
    frost,
    plant_frost,
    ice,
    ice_age,
    moss,
    grass,
    bush,
    tree,
    tree_scale,
    greening,
    meadow,
    flower,
    birds,
    clarity,
    algae,
    waves,
    foam,
    sky
);
impl ClimateState {
    /// Channels used by the static ground bake. Water motion, clarity, fauna
    /// and atmosphere continue independently and do not invalidate this cache.
    pub fn ground_channels(self) -> [f64; 4] {
        [self.frost, self.ice, self.greening, self.meadow]
    }
}
pub fn from_parameters(p: ClimateParameters) -> Result<ClimateState, TerrainError> {
    let p = p.validate()?;
    let (t, o) = (p.temperature, p.oxygen);
    let w = p.oceans * 9.0 / p.max_oceans.max(1.0);
    let air = clamp(o / 14.0);
    let ice = 1.0 - smooth(-4.0, 0.0, t);
    Ok(ClimateState {
        frost: 1.0 - smooth(-30.0, 0.0, t),
        plant_frost: 1.0 - smooth(-32.0, -20.0, t),
        ice,
        ice_age: smooth(-30.0, -10.0, t),
        moss: smooth(1.0, 3.0, w),
        grass: smooth(-17.0, -12.0, t),
        bush: smooth(-11.0, -6.0, t),
        tree: smooth(-4.0, 2.0, t),
        tree_scale: 0.6 + 0.4 * air,
        greening: smooth(-28.0, 6.0, t) * (0.75 + 0.25 * air),
        meadow: smooth(-6.0, 2.0, t) * smooth(1.0, 3.0, w),
        flower: smooth(9.0, 11.0, o) * smooth(-1.0, 1.0, t),
        birds: smooth(5.5, 10.0, o),
        clarity: air,
        algae: smooth(3.0, 6.0, o) * (1.0 - ice),
        waves: smooth(2.0, 9.0, w),
        foam: smooth(6.0, 8.0, w),
        sky: smooth(-30.0, 8.0, t),
    })
}
#[derive(Clone, Copy, Debug, Default, PartialEq)]
pub struct ClimatePulses {
    pub warm: f64,
    pub shimmer: f64,
    pub sweep: f64,
}
pub struct ClimateRuntime {
    previous: ClimateParameters,
    current: ClimateState,
    target: ClimateState,
    world_step: u64,
    milestones: [Option<u64>; 3],
}
impl ClimateRuntime {
    /// First/reconnect snapshot snaps without replaying earlier milestones.
    pub fn new(parameters: ClimateParameters, world_step: u64) -> Result<Self, TerrainError> {
        let state = from_parameters(parameters)?;
        Ok(Self {
            previous: parameters,
            current: state,
            target: state,
            world_step,
            milestones: [None; 3],
        })
    }
    pub fn parameters(&self) -> ClimateParameters {
        self.previous
    }
    pub fn current(&self) -> ClimateState {
        self.current
    }
    pub fn target(&self) -> ClimateState {
        self.target
    }
    pub fn retarget(&mut self, p: ClimateParameters, world_step: u64) -> Result<(), TerrainError> {
        let target = from_parameters(p)?;
        if world_step < self.world_step {
            return Err(TerrainError::OutsideDomain);
        }
        self.advance(world_step)?;
        let old = self.previous;
        if old.temperature < 8.0 && p.temperature >= 8.0 {
            self.milestones[0] = Some(world_step)
        }
        if old.oceans < p.max_oceans && p.oceans >= p.max_oceans {
            self.milestones[1] = Some(world_step)
        }
        if !old.terraformed() && p.terraformed() {
            self.milestones[2] = Some(world_step)
        }
        self.previous = p;
        self.target = target;
        Ok(())
    }
    /// Consume the canonical fixed world step; repeated/paused frames do nothing.
    pub fn advance(&mut self, world_step: u64) -> Result<(), TerrainError> {
        let elapsed = world_step
            .checked_sub(self.world_step)
            .ok_or(TerrainError::OutsideDomain)?;
        // Advance fixed steps to preserve identical easing across frame partition.
        // At most the first 1024 steps matter; all channels then snap below epsilon.
        for _ in 0..elapsed.min(1024) {
            self.current.ease(self.target, 1.0 / 60.0)
        }
        if elapsed > 1024 {
            self.current = self.target
        }
        self.world_step = world_step;
        Ok(())
    }
    pub fn pulses(&self) -> ClimatePulses {
        let envelope = |start: Option<u64>, duration: f64| {
            let Some(start) = start else { return 0.0 };
            let t = (self.world_step.saturating_sub(start) as f64 / 60.0) / duration;
            if !(0.0..1.0).contains(&t) {
                0.0
            } else {
                (t * 8.0).min(1.0) * (1.0 - t) * (1.0 - t)
            }
        };
        let sweep =
            self.milestones[2].map_or(0.0, |s| self.world_step.saturating_sub(s) as f64 / 150.0);
        ClimatePulses {
            warm: envelope(self.milestones[0], 1.5),
            shimmer: envelope(self.milestones[1], 3.0),
            sweep: if sweep > 0.0 && sweep < 1.0 {
                sweep
            } else {
                0.0
            },
        }
    }
    pub fn snap(&mut self, p: ClimateParameters, world_step: u64) -> Result<(), TerrainError> {
        *self = Self::new(p, world_step)?;
        Ok(())
    }
}
