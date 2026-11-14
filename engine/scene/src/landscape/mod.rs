//! Continuous ground and shoreline fields, independent of graphics and game rules.
mod fields;
mod footprint;
mod input;
mod support;
pub use fields::{
    LandscapeInput, LandscapeKind, LandscapeSample, LandscapeSampler, LandscapeSource,
};
pub use input::{LandscapeInputError, visual_seed};
#[derive(Clone, Copy, Debug, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum BodyId {
    Mars,
    Phobos,
    Venus,
    Mercury,
    Earth,
    Luna,
    Ceres,
    Jupiter,
    Ganymede,
    Saturn,
    Uranus,
    Neptune,
}
pub const TILE_RADIUS: f64 = 0.166;
pub const WATER_RADIUS: f64 = 0.17 * super::geometry::BOARD_SCALE;
pub const WATER_LEVEL: f64 = -0.008;
pub const BEACH_WIDTH_MIN: f64 = 0.05;
pub const BEACH_WIDTH_MAX: f64 = 0.075;
pub const LAKE_MASK_REACH: f64 = 0.09;
pub const LAKE_GROUND_REACH: f64 = 0.115;
pub const TRANSITION_SECONDS: f64 = 0.6;
pub const MAX_SOURCES: usize = 512;
pub const MAX_PATCHES: usize = 128;
pub(super) fn clamp(x: f64) -> f64 {
    x.clamp(0.0, 1.0)
}
pub(super) fn smooth(a: f64, b: f64, x: f64) -> f64 {
    let t = clamp((x - a) / (b - a));
    t * t * (3.0 - 2.0 * t)
}

/// Ordinary board distances map directly to WASM's square-root instruction.
/// Retain hypot's scaling for extreme finite inputs and subnormal distances.
#[inline]
pub(super) fn planar_length(x: f64, y: f64) -> f64 {
    let squared = x * x + y * y;
    if squared.is_normal() {
        squared.sqrt()
    } else if x == 0.0 && y == 0.0 {
        0.0
    } else {
        x.hypot(y)
    }
}

mod preparation;
pub mod vegetation;
pub use preparation::{
    LandscapeBuilder, LandscapePatch, LandscapeSnapshot, PreparationState, PreparationWork,
};
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct TerrainRevision(pub u64);
pub trait TerrainSampler: Send + Sync {
    fn revision(&self) -> TerrainRevision;
    fn body(&self) -> BodyId;
    fn sample(
        &self,
        p: crate::geometry::BoardPoint,
    ) -> Result<crate::geometry::terrain::TerrainSample, crate::geometry::terrain::TerrainError>;
    fn height(
        &self,
        p: crate::geometry::BoardPoint,
    ) -> Result<f32, crate::geometry::terrain::TerrainError>;
    fn bounds(&self) -> Option<[crate::geometry::BoardPoint; 2]>;
}

pub use preparation::transfer::{MAX_PACKET_BYTES, TerrainWorker};
pub mod publication;
pub mod transition;
pub mod upload;

#[cfg(test)]
mod distance_tests {
    use super::planar_length;

    #[test]
    fn board_distance_matches_hypot_precision_and_preserves_extreme_inputs() {
        let mut seed = 17u32;
        for _ in 0..20_000 {
            seed = seed.wrapping_mul(1664525).wrapping_add(1013904223);
            let x = f64::from(seed as i32) / 2147.0;
            seed = seed.wrapping_mul(1664525).wrapping_add(1013904223);
            let y = f64::from(seed as i32) / 2147.0;
            let expected = x.hypot(y);
            assert!((planar_length(x, y) - expected).abs() <= expected * 2.0 * f64::EPSILON);
        }
        for (x, y) in [
            (0.0, -0.0),
            (1e-200, 2e-200),
            (f64::from_bits(1), f64::from_bits(2)),
            (1e308, 1e308),
            (f64::MAX, f64::MAX),
            (f64::INFINITY, f64::NAN),
        ] {
            assert_eq!(planar_length(x, y), x.hypot(y));
        }
        assert!(planar_length(f64::NAN, 1.0).is_nan());
    }
}
