// Adapted from the sibling renderer; graphics-only scalar sampling.
//! CPU reference arithmetic from landscapeFields.ts and volcanoFlowMap.ts.
use super::terrain::TerrainError;

/// Integer-lattice value noise. The explicit seed and JS-compatible wrapping
/// lattice hash do not depend on a generator, thread, or evaluation order.
pub fn landscape_noise(x: f64, y: f64, seed: u32) -> Result<f64, TerrainError> {
    if !x.is_finite() || !y.is_finite() {
        return Err(TerrainError::NonFinite);
    }
    let ix = x.floor();
    let iy = y.floor();
    let curve = |v: f64| v * v * v * (v * (v * 6.0 - 15.0) + 10.0);
    let u = curve(x - ix);
    let v = curve(y - iy);
    // Addition after conversion preserves the adjacent wrapping lattice cell
    // even when ix+1 itself is not representable as an f64.
    let hx = ix.rem_euclid(4294967296.0) as u32;
    let hy = iy.rem_euclid(4294967296.0) as u32;
    let hash = |x: u32, y: u32| {
        let n = x.wrapping_mul(374761393) ^ y.wrapping_mul(668265263) ^ seed;
        let n = (n ^ (n >> 13)).wrapping_mul(1274126177);
        f64::from(n ^ (n >> 16)) / 4294967296.0
    };
    let a = hash(hx, hy);
    let b = hash(hx.wrapping_add(1), hy);
    let c = hash(hx, hy.wrapping_add(1));
    let d = hash(hx.wrapping_add(1), hy.wrapping_add(1));
    Ok((a + (b - a) * u) * (1.0 - v) + (c + (d - c) * u) * v)
}

pub(super) fn smooth(a: f64, b: f64, x: f64) -> f64 {
    let t = ((x - a) / (b - a)).clamp(0.0, 1.0);
    t * t * (3.0 - 2.0 * t)
}

/// The reference 2D simplex kernel. Keeping f64 intermediate arithmetic matches
/// the existing CPU implementation; GPU f32 conformance is a later real-backend
/// check, not certified by these CPU functions.
pub fn simplex(vx: f64, vy: f64) -> f64 {
    const C0: f64 = 0.211324865405187;
    const C1: f64 = 0.366025403784439;
    const C2: f64 = -0.577350269189626;
    const C3: f64 = 0.024390243902439;
    let mod289 = |x: f64| x - (x * (1.0 / 289.0)).floor() * 289.0;
    let permute = |x: f64| mod289((x * 34.0 + 1.0) * x);
    let s = (vx + vy) * C1;
    let ix = (vx + s).floor();
    let iy = (vy + s).floor();
    let t = (ix + iy) * C0;
    let x = vx - ix + t;
    let y = vy - iy + t;
    let (i1x, i1y) = if x > y { (1.0, 0.0) } else { (0.0, 1.0) };
    let mi = mod289(ix);
    let miy = mod289(iy);
    let points = [
        (x, y, permute(permute(miy) + mi)),
        (
            x + C0 - i1x,
            y + C0 - i1y,
            permute(permute(miy + i1y) + mi + i1x),
        ),
        (x + C2, y + C2, permute(permute(miy + 1.0) + mi + 1.0)),
    ];
    let mut result = 0.0;
    for (x, y, p) in points {
        let mut m = (0.5 - (x * x + y * y)).max(0.0);
        m *= m;
        m *= m;
        let gx = 2.0 * ((p * C3) % 1.0) - 1.0;
        let h = gx.abs() - 0.5;
        let a = gx - (gx + 0.5).floor();
        m *= 1.79284291400159 - 0.85373472095314 * (a * a + h * h);
        result += m * (a * x + h * y);
    }
    130.0 * result
}
