//! Exact topology predicates for finite binary64 polygon coordinates. Products
//! have at most 106 bits; separate positive/negative fixed accumulators cover
//! the full exponent span plus the sum of 128 polygon wedges.
use std::cmp::Ordering;
const LIMBS: usize = 68;
fn parts(x: f64) -> (bool, u64, i32) {
    let b = x.to_bits();
    let e = ((b >> 52) & 2047) as i32;
    (
        b >> 63 != 0,
        (b & ((1 << 52) - 1)) | if e == 0 { 0 } else { 1 << 52 },
        if e == 0 { -1074 } else { e - 1075 },
    )
}
fn add(sum: &mut [u64; LIMBS], mut i: usize, mut word: u64) {
    while word != 0 {
        let (v, carry) = sum[i].overflowing_add(word);
        sum[i] = v;
        word = u64::from(carry);
        i += 1;
    }
}
fn sign(terms: impl IntoIterator<Item = (f64, f64, bool)>) -> Ordering {
    let mut positive = [0; LIMBS];
    let mut negative = [0; LIMBS];
    for (a, b, subtract) in terms {
        let (sa, ma, ea) = parts(a);
        let (sb, mb, eb) = parts(b);
        let product = u128::from(ma) * u128::from(mb);
        let shift = (ea + eb + 2148) as usize;
        let target = if sa ^ sb ^ subtract {
            &mut negative
        } else {
            &mut positive
        };
        for (j, word) in [product as u64, (product >> 64) as u64]
            .into_iter()
            .enumerate()
        {
            add(target, shift / 64 + j, word << (shift % 64));
            if !shift.is_multiple_of(64) {
                add(target, shift / 64 + j + 1, word >> (64 - shift % 64));
            }
        }
    }
    positive.iter().rev().cmp(negative.iter().rev())
}
pub(super) fn orientation(a: [f64; 2], b: [f64; 2], c: [f64; 2]) -> Ordering {
    sign([
        (b[0], c[1], false),
        (b[1], c[0], true),
        (a[0], c[1], true),
        (a[1], c[0], false),
        (b[0], a[1], true),
        (b[1], a[0], false),
    ])
}
pub(super) fn area(vertices: &[[f64; 2]]) -> Ordering {
    sign((0..vertices.len()).flat_map(|i| {
        let a = vertices[i];
        let b = vertices[(i + 1) % vertices.len()];
        [(a[0], b[1], false), (a[1], b[0], true)]
    }))
}
