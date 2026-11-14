use openmars_engine_assets::{TransformLimits, environment::*, texture_array};
use serde::Deserialize;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let args: Vec<_> = std::env::args().skip(1).collect();
    match args.first().map(String::as_str) {
        Some("environment") if args.len() == 3 => environment(&args[1..]),
        Some("texture-array") if args.len() == 3 => {
            if std::fs::metadata(&args[1])?.len() > 65536 {
                return Err("Texture array request exceeds byte bound".into());
            }
            let request: texture_array::ArrayRequest = serde_json::from_slice(&std::fs::read(&args[1])?)?;
            std::fs::write(&args[2], texture_array::prepare(&request)?)?;
            Ok(())
        }
        _ => Err("Usage: openmars-engine-assets environment REQUEST.json OUTPUT.ktx2 | texture-array REQUEST.json OUTPUT.ktx2".into()),
    }
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct EnvironmentInput {
    source: std::path::PathBuf,
    binding: SourceBinding,
    cube: CubeRequest,
}
fn environment(args: &[String]) -> Result<(), Box<dyn std::error::Error>> {
    if std::fs::metadata(&args[0])?.len() > 65536 {
        return Err("Environment request exceeds byte bound".into());
    }
    let request: EnvironmentInput = serde_json::from_slice(&std::fs::read(&args[0])?)?;
    let limits = TransformLimits::default();
    preflight_cube(&request.cube, &limits)?;
    if std::fs::metadata(&request.source)?.len() > limits.max_source_bytes {
        return Err("Environment source exceeds input limit".into());
    }
    let bytes = std::fs::read(&request.source)?;
    let source = decode_exr(&bytes, &request.binding, &limits)?;
    let cube = build_environment_cube(&source, &request.cube, &limits)?;
    std::fs::write(&args[1], encode_ktx2(&cube, &limits)?)?;
    Ok(())
}
