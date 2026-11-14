//! CPU form of the field transition shared by the surface shader. Upload and
//! render-retirement observations belong to the actual renderer, not these values.
use super::{TRANSITION_SECONDS, WATER_RADIUS, clamp, smooth};
use crate::geometry::terrain::TerrainError;

pub fn lake_blend_weight(previous: f64, next: f64, elapsed: f64) -> Result<f64, TerrainError> {
    if ![previous, next, elapsed].into_iter().all(f64::is_finite) {
        return Err(TerrainError::NonFinite);
    }
    let elapsed = clamp(elapsed);
    if next >= previous {
        return Ok(elapsed * elapsed * (3.0 - 2.0 * elapsed));
    }
    let front = -WATER_RADIUS + elapsed * (0.2 + WATER_RADIUS);
    let settle = next.max(-WATER_RADIUS);
    Ok(smooth(settle, settle + 0.1, front).max(smooth(0.85, 1.0, elapsed)))
}
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct BlendedField {
    pub terrain: [f64; 4],
    pub materials: [f64; 4],
    pub detail: [f64; 4],
    pub basin_shore: f64,
    pub done: bool,
}
pub fn blend(
    previous: [f64; 4],
    next: [f64; 4],
    previous_materials: [f64; 4],
    next_materials: [f64; 4],
    previous_detail: [f64; 4],
    next_detail: [f64; 4],
    seconds: f64,
) -> Result<BlendedField, TerrainError> {
    if !previous
        .into_iter()
        .chain(next)
        .chain(previous_materials)
        .chain(next_materials)
        .chain(previous_detail)
        .chain(next_detail)
        .chain([seconds])
        .all(f64::is_finite)
    {
        return Err(TerrainError::NonFinite);
    }
    if seconds < 0.0 {
        return Err(TerrainError::OutsideDomain);
    }
    let elapsed = clamp(seconds / TRANSITION_SECONDS);
    let weight = lake_blend_weight(previous[1], next[1], elapsed)?;
    let mix = |a: [f64; 4], b: [f64; 4]| {
        if elapsed <= 0.0 {
            a
        } else if elapsed >= 1.0 {
            b
        } else {
            std::array::from_fn(|i| a[i] + (b[i] - a[i]) * weight)
        }
    };
    let basin_shore = if elapsed <= 0.0 {
        previous[1]
    } else if elapsed >= 1.0 {
        next[1]
    } else if next[1] < previous[1] && weight < 0.02 {
        previous[1]
    } else {
        previous[1].min(next[1])
    };
    Ok(BlendedField {
        terrain: mix(previous, next),
        materials: mix(previous_materials, next_materials),
        detail: mix(previous_detail, next_detail),
        basin_shore,
        done: elapsed >= 1.0,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn shoreline_front_keeps_ground_and_all_material_channels_together() {
        let previous = [0.0003, 0.2, 0., 0.];
        let next = [-0.024, -0.1, 0.6, 0.2];
        let at =
            |seconds| blend(previous, next, [0.; 4], [1.; 4], [0.; 4], [0.5; 4], seconds).unwrap();
        let start = at(0.);
        assert_eq!(start.terrain, previous);
        assert_eq!(start.materials, [0.; 4]);
        assert_eq!(start.basin_shore, 0.2);
        let middle = at(0.3);
        assert_eq!(middle.basin_shore, -0.1);
        assert!(!middle.done);
        assert!(middle.materials[0] > 0.);
        for i in 0..4 {
            assert!((middle.detail[i] - middle.materials[i] * 0.5).abs() < 1e-12);
            assert!(
                (middle.terrain[i] - (previous[i] + (next[i] - previous[i]) * middle.materials[i]))
                    .abs()
                    < 1e-12
            );
        }
        let end = at(0.6);
        assert_eq!(end.terrain, next);
        assert_eq!(end.materials, [1.; 4]);
        assert_eq!(end.detail, [0.5; 4]);
        assert!(end.done);
        assert_eq!(at(5.), end);
        let removing = blend(next, previous, [1.; 4], [0.; 4], [0.5; 4], [0.; 4], 0.3).unwrap();
        assert_eq!(
            removing.basin_shore, -0.1,
            "hold the old basin until removal settles"
        );
        assert_eq!(lake_blend_weight(-0.1, 0.2, 0.5).unwrap(), 0.5);
    }
    #[test]
    fn invalid_transition_inputs_are_rejected_before_sampling() {
        for value in [f64::NAN, f64::INFINITY, f64::NEG_INFINITY] {
            assert_eq!(
                lake_blend_weight(value, 0., 0.),
                Err(TerrainError::NonFinite)
            );
            assert_eq!(
                lake_blend_weight(0., value, 0.),
                Err(TerrainError::NonFinite)
            );
            assert_eq!(
                lake_blend_weight(0., 0., value),
                Err(TerrainError::NonFinite)
            );
            for channel in 0..6 {
                let mut channels = [[0.; 4]; 6];
                channels[channel][2] = value;
                assert_eq!(
                    blend(
                        channels[0],
                        channels[1],
                        channels[2],
                        channels[3],
                        channels[4],
                        channels[5],
                        0.
                    ),
                    Err(TerrainError::NonFinite)
                );
            }
        }
        assert_eq!(
            blend([0.; 4], [0.; 4], [0.; 4], [0.; 4], [0.; 4], [0.; 4], -0.01),
            Err(TerrainError::OutsideDomain)
        );
    }
}
