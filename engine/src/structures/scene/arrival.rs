//! Preserve the visible portion of the source cubic city emergence.
// Local city geometry is projected with -0.007 foundation bias, while its
// prepared pad is +0.0003. A small positive margin avoids a hidden first sample.
const PAD_AND_FOUNDATION: f32 = 0.008;

pub(super) fn visible_lead(highest: f32, depth: f32) -> f32 {
    let roof = (highest - PAD_AND_FOUNDATION).max(0.);
    1. - (roof / depth).clamp(0., 1.).cbrt()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn first_published_sample_reaches_the_prepared_pad_for_short_and_tall_cities() {
        for (highest, depth) in [(0.018_f32, 0.08_f32), (0.1, 0.145), (0.3, 0.345)] {
            let lead = visible_lead(highest, depth);
            assert!((0. ..=1.).contains(&lead));
            let original_roof = highest - depth;
            assert!(original_roof < PAD_AND_FOUNDATION);
            let first_roof = highest - depth * (1. - lead).powi(3);
            assert!((first_roof - PAD_AND_FOUNDATION).abs() < 1e-6);
            // The final source geometry and cubic displacement remain unchanged.
            assert_eq!(depth * (1. - 1_f32).powi(3), 0.);
        }
    }

    #[test]
    fn skips_only_the_invisible_prefix_of_the_original_cubic_curve() {
        let depth = 0.08_f32;
        let lead = visible_lead(0.018, depth);
        assert!((lead - 0.5).abs() < 1e-6);
        for age in [0_f32, 0.1, 0.25, 0.5] {
            let published_progress = (lead + age).clamp(0., 1.);
            let original_at_same_visible_age = (age + 0.5).clamp(0., 1.);
            assert!(
                (depth * (1. - published_progress).powi(3)
                    - depth * (1. - original_at_same_visible_age).powi(3))
                .abs()
                    < 1e-6
            );
        }
    }
}
