use serde_json::json;
use sha2::{Digest, Sha256};
use std::{fs, path::PathBuf, process::Command};
struct Fixture(PathBuf);
impl Fixture {
    fn new() -> Self {
        let root = std::env::temp_dir().join(format!(
            "openmars-environment-{}-{}",
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_nanos()
        ));
        fs::create_dir_all(&root).unwrap();
        Self(root)
    }
}
impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}
#[test]
fn real_cli_preserves_hdr_radiance_and_validates_recipe_and_source_before_publish() {
    let f = Fixture::new();
    let source = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("../../frontend/scripts/assets/fixtures/environment.exr");
    let source_bytes = fs::read(&source).unwrap();
    let request_path = f.0.join("request.json");
    let output_path = f.0.join("cube.ktx2");
    let mut request = json!({"source":source,"binding":{"sha256":format!("{:x}",Sha256::digest(source_bytes)),"absent_chromaticities_are_linear_bt709":true},"cube":{"id":"test/environment","encoding":"rgba16-float","face_size":4,"levels":3,"recipe":"ordinary-linear","samples":16,"diffuse_grid":null}});
    let run = |request: &serde_json::Value| {
        fs::write(&request_path, serde_json::to_vec(request).unwrap()).unwrap();
        Command::new(env!("CARGO_BIN_EXE_openmars-engine-assets"))
            .arg("environment")
            .arg(&request_path)
            .arg(&output_path)
            .output()
            .unwrap()
    };
    for recipe in ["ordinary-linear", "normalized-irradiance", "ggx"] {
        request["cube"]["recipe"] = json!(recipe);
        let diffuse = recipe == "normalized-irradiance";
        request["cube"]["levels"] = json!(if diffuse { 1 } else { 3 });
        request["cube"]["samples"] = json!(if diffuse { 0 } else { 16 });
        request["cube"]["diffuse_grid"] = if diffuse {
            json!({"width":4,"height":2})
        } else {
            json!(null)
        };
        let result = run(&request);
        assert!(
            result.status.success(),
            "{}",
            String::from_utf8_lossy(&result.stderr)
        );
        let bytes = fs::read(&output_path).unwrap();
        let reader = ktx2::Reader::new(&bytes).unwrap();
        assert_eq!(reader.header().face_count, 6);
        assert_eq!(reader.header().level_count, if diffuse { 1 } else { 3 });
        assert_eq!(
            reader.header().format,
            Some(ktx2::Format::R16G16B16A16_SFLOAT)
        );
        for level in reader.levels() {
            let (pixels, remainder) = level.data.as_chunks::<8>();
            assert!(remainder.is_empty());
            for pixel in pixels {
                let rgba: [f32; 4] = std::array::from_fn(|i| {
                    half::f16::from_bits(u16::from_le_bytes([pixel[i * 2], pixel[i * 2 + 1]]))
                        .to_f32()
                });
                assert_eq!(
                    rgba,
                    [0.125, 0.25, 2., 1.],
                    "{recipe} must preserve HDR and linear energy"
                );
            }
        }
    }
    request["cube"]["encoding"] = json!("rgb9e5-ufloat");
    request["cube"]["recipe"] = json!("ordinary-linear");
    assert!(run(&request).status.success());
    let bytes = fs::read(&output_path).unwrap();
    let reader = ktx2::Reader::new(&bytes).unwrap();
    assert_eq!(
        reader.header().format,
        Some(ktx2::Format::E5B9G9R9_UFLOAT_PACK32)
    );
    for level in reader.levels() {
        use std::io::Read;
        let mut raw = Vec::new();
        flate2::read::ZlibDecoder::new(level.data)
            .read_to_end(&mut raw)
            .unwrap();
        for pixel in raw.as_chunks::<4>().0 {
            let word = u32::from_le_bytes(*pixel);
            let scale = 2f64.powi((word >> 27) as i32 - 24);
            assert_eq!(
                [word & 511, (word >> 9) & 511, (word >> 18) & 511].map(|m| f64::from(m) * scale),
                [0.125, 0.25, 2.]
            );
        }
    }
    fs::write(&output_path, b"keep existing output").unwrap();
    request["binding"]["sha256"] = json!("0".repeat(64));
    assert!(!run(&request).status.success());
    assert_eq!(fs::read(&output_path).unwrap(), b"keep existing output");
    request["cube"]["face_size"] = json!(4096);
    request["source"] = json!("source-must-not-be-opened.exr");
    let result = run(&request);
    assert!(!result.status.success());
    assert!(String::from_utf8_lossy(&result.stderr).contains("bounds"));
}
