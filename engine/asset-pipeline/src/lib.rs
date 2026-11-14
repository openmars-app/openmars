//! Offline environment preparation, extracted from the sibling renderer pipeline.
pub mod environment;
pub mod texture_array;

pub mod error {
    #[derive(Debug)]
    pub struct PipelineError(String);
    impl PipelineError {
        pub fn invalid(subject: impl Into<String>, message: impl Into<String>) -> Self {
            Self(format!("{}: {}", subject.into(), message.into()))
        }
    }
    impl std::fmt::Display for PipelineError {
        fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
            f.write_str(&self.0)
        }
    }
    impl std::error::Error for PipelineError {}
}

#[derive(Clone, Debug)]
pub struct TransformLimits {
    pub max_source_bytes: u64,
    pub max_output_bytes: u64,
    pub max_working_bytes: u64,
    pub max_dimension: u32,
}
impl Default for TransformLimits {
    fn default() -> Self {
        Self {
            max_source_bytes: 256 * 1024 * 1024,
            max_output_bytes: 136 * 1024 * 1024,
            max_working_bytes: 2 * 1024 * 1024 * 1024,
            max_dimension: 8192,
        }
    }
}
impl TransformLimits {
    pub fn validate(&self) -> Result<(), error::PipelineError> {
        if self.max_source_bytes == 0
            || self.max_source_bytes > 256 * 1024 * 1024
            || self.max_output_bytes == 0
            || self.max_output_bytes > 136 * 1024 * 1024
            || self.max_working_bytes < 768 * 1024 * 1024
            || self.max_working_bytes > 2 * 1024 * 1024 * 1024
            || self.max_dimension == 0
            || self.max_dimension > 8192
        {
            return Err(error::PipelineError::invalid(
                "environment limits",
                "invalid byte or dimension bounds",
            ));
        }
        Ok(())
    }
    pub fn job_bytes(&self) -> u64 {
        self.max_working_bytes
    }
}

#[cfg(test)]
mod tests {
    use super::{environment::*, *};
    #[test]
    fn cube_preserves_linear_radiance_faces_and_mips() {
        let limits = TransformLimits::default();
        let request = CubeRequest {
            encoding: CubeEncoding::Rgba16Float,
            id: "stars".into(),
            face_size: 4,
            levels: 3,
            recipe: Recipe::OrdinaryLinear,
            samples: 16,
            diffuse_grid: None,
        };
        let cube = build_cube(&|_| [0.125, 0.25, 2.0], &request, &limits).unwrap();
        let bytes = encode_ktx2(&cube, &limits).unwrap();
        let reader = ktx2::Reader::new(&bytes).unwrap();
        assert_eq!(reader.header().face_count, 6);
        assert_eq!(reader.header().level_count, 3);
        assert_eq!(
            reader.header().format,
            Some(ktx2::Format::R16G16B16A16_SFLOAT)
        );
        for level in &cube.levels {
            for rgb in level {
                assert_eq!(*rgb, [0.125, 0.25, 2.0]);
            }
        }
    }
    #[test]
    fn bounded_and_deterministic_environment_processing() {
        let limits = TransformLimits::default();
        let mut request = CubeRequest {
            encoding: CubeEncoding::Rgba16Float,
            id: "stars".into(),
            face_size: 4,
            levels: 3,
            recipe: Recipe::OrdinaryLinear,
            samples: 16,
            diffuse_grid: None,
        };
        let make = || {
            encode_ktx2(
                &build_cube(&|d| d.map(|x| x + 2.0), &request, &limits).unwrap(),
                &limits,
            )
            .unwrap()
        };
        assert_eq!(make(), make());
        request.face_size = 2048;
        assert!(preflight_cube(&request, &limits).is_err());
        assert!(half_bits(f32::INFINITY).is_err());
        assert!(inspect_exr(&[0; 32], &limits).is_err());
    }

    #[test]
    fn ggx_roughness_filters_directional_energy_without_clamping_hdr() {
        let sample = |d: [f32; 3]| [0.5 + 0.25 * d[0], 1. + 0.5 * d[1], 2. + 0.75 * d[2]];
        for axis in 0..3 {
            for sign in [-1., 1.] {
                let mut n = [0.; 3];
                n[axis] = sign;
                assert_eq!(
                    convolve(&sample, n, Recipe::Ggx, 0., 8192).unwrap(),
                    sample(n)
                );
                let filtered = convolve(&sample, n, Recipe::Ggx, 1., 8192).unwrap();
                // At alpha1 the reflected directions are uniform on the hemisphere;
                // cosine weighting has analytic first moment (2/3)N.
                let expected = sample(n.map(|v| v * 2. / 3.));
                for (a, b) in filtered.into_iter().zip(expected) {
                    assert!((a - b).abs() < 0.001, "{a} != {b}");
                }
            }
        }
    }
    #[test]
    fn global_diffuse_grid_matches_analytic_cosine_weighted_directional_field() {
        let (width, height) = (128, 64);
        let sample = |n: [f64; 3]| [0.5 + 0.25 * n[0], 1. + 0.5 * n[1], 2. + 0.75 * n[2]];
        let pixels = (0..height)
            .flat_map(|y| {
                (0..width).map(move |x| {
                    let theta = std::f64::consts::PI * (f64::from(y) + 0.5) / f64::from(height);
                    let phi = std::f64::consts::TAU * (f64::from(x) + 0.5) / f64::from(width)
                        - std::f64::consts::PI;
                    sample([
                        theta.sin() * phi.cos(),
                        theta.cos(),
                        theta.sin() * phi.sin(),
                    ])
                    .map(|v| v as f32)
                })
            })
            .collect();
        let image = LinearImage {
            width,
            height,
            pixels,
        };
        let grid = DiffuseGrid::from_image(
            &image,
            GridSpec { width, height },
            &TransformLimits::default(),
        )
        .unwrap();
        for axis in 0..3 {
            for sign in [-1., 1.] {
                let mut n = [0.; 3];
                n[axis] = sign;
                let result = grid.irradiance(n).unwrap();
                let expected = sample(n.map(|v| v * 2. / 3.));
                for (a, b) in result.into_iter().zip(expected) {
                    assert!((f64::from(a) - b).abs() < 0.001, "{a} != {b}");
                }
            }
        }
    }
}
