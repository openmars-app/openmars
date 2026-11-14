// Adapted from the sibling renderer’s pure geometry module.
//! Exact sign of r²(d·d) - |o×d|² for finite binary64 inputs.
//!
//! Each input is an integer mantissa times a power of two. Expanding the three
//! cross-product squares gives twelve degree-four terms. A term needs at most
//! 212 significant bits; the complete binary64 exponent span needs fewer than
//! 8,448 accumulator bits, including carries. Fixed stack storage avoids both
//! overflow/underflow in the predicate and allocations during pointer picking.
use std::cmp::Ordering;
const LIMBS: usize = 136;

#[derive(Clone, Copy, Default)]
struct Dyadic {
    negative: bool,
    mantissa: u64,
    exponent: i32,
}
impl Dyadic {
    fn new(value: f64) -> Self {
        let bits = value.to_bits();
        let e = ((bits >> 52) & 0x7ff) as i32;
        let fraction = bits & ((1_u64 << 52) - 1);
        Self {
            negative: bits >> 63 != 0,
            mantissa: if e == 0 {
                fraction
            } else {
                fraction | (1_u64 << 52)
            },
            exponent: if e == 0 { -1074 } else { e - 1075 },
        }
    }
}
#[derive(Clone, Copy, Default)]
struct Term {
    negative: bool,
    words: [u64; 4],
    exponent: i32,
}
impl Term {
    fn product(factors: [Dyadic; 4], subtract: bool, double: bool) -> Self {
        let mut result = Self {
            negative: subtract,
            words: [1, 0, 0, 0],
            exponent: i32::from(double),
        };
        for f in factors {
            result.negative ^= f.negative;
            result.exponent += f.exponent;
            let mut carry = 0_u128;
            for word in &mut result.words {
                let product = u128::from(*word) * u128::from(f.mantissa) + carry;
                *word = product as u64;
                carry = product >> 64;
            }
        }
        result
    }
    fn zero(self) -> bool {
        self.words == [0; 4]
    }
}
fn add_word(sum: &mut [u64; LIMBS], mut index: usize, mut word: u64) {
    while word != 0 {
        let (value, carry) = sum[index].overflowing_add(word);
        sum[index] = value;
        word = u64::from(carry);
        index += 1;
    }
}
fn add_term(sum: &mut [u64; LIMBS], term: Term, base: i32) {
    let shift = (term.exponent - base) as usize;
    let offset = shift / 64;
    let bits = shift % 64;
    for (i, word) in term.words.into_iter().enumerate() {
        add_word(sum, offset + i, word << bits);
        if bits != 0 {
            add_word(sum, offset + i + 1, word >> (64 - bits));
        }
    }
}

/// A nonnegative magnitude with a separate binary exponent. All intermediate
/// sums/ratios stay near unity, even when squared raw coordinates exceed f64.
#[derive(Clone, Copy, Debug)]
pub(super) struct Magnitude {
    mantissa: f64,
    exponent: i32,
}
impl Magnitude {
    const ZERO: Self = Self {
        mantissa: 0.0,
        exponent: 0,
    };
    fn new(mut mantissa: f64, mut exponent: i32) -> Self {
        if mantissa == 0.0 {
            return Self::ZERO;
        }
        while mantissa >= 2.0 {
            mantissa *= 0.5;
            exponent += 1;
        }
        while mantissa < 1.0 {
            mantissa *= 2.0;
            exponent -= 1;
        }
        Self { mantissa, exponent }
    }
    pub(super) fn add(self, other: Self) -> Self {
        if self.mantissa == 0.0 {
            return other;
        }
        if other.mantissa == 0.0 {
            return self;
        }
        let (a, b) = if self.exponent >= other.exponent {
            (self, other)
        } else {
            (other, self)
        };
        Self::new(
            a.mantissa + b.mantissa * 2.0_f64.powi(b.exponent - a.exponent),
            a.exponent,
        )
    }
    pub(super) fn divide(self, other: Self) -> Self {
        Self::new(
            self.mantissa / other.mantissa,
            self.exponent - other.exponent,
        )
    }
    fn sqrt(self) -> Self {
        Self::new(
            (self.mantissa
                * if self.exponent.rem_euclid(2) == 0 {
                    1.0
                } else {
                    2.0
                })
            .sqrt(),
            self.exponent.div_euclid(2),
        )
    }
    pub(super) fn value(self) -> f64 {
        scale_binary(self.mantissa, self.exponent)
    }
}
#[derive(Clone, Copy)]
pub(super) struct Exact {
    pub(super) sign: Ordering,
    pub(super) magnitude: Magnitude,
}
fn sum(terms: &[Term]) -> Exact {
    let Some(base) = terms.iter().filter(|t| !t.zero()).map(|t| t.exponent).min() else {
        return Exact {
            sign: Ordering::Equal,
            magnitude: Magnitude::ZERO,
        };
    };
    let mut positive = [0; LIMBS];
    let mut negative = [0; LIMBS];
    for term in terms.iter().copied().filter(|t| !t.zero()) {
        add_term(
            if term.negative {
                &mut negative
            } else {
                &mut positive
            },
            term,
            base,
        );
    }
    let sign = positive.iter().rev().cmp(negative.iter().rev());
    if sign == Ordering::Equal {
        return Exact {
            sign,
            magnitude: Magnitude::ZERO,
        };
    }
    if sign == Ordering::Less {
        std::mem::swap(&mut positive, &mut negative);
    }
    let mut borrow = false;
    for i in 0..LIMBS {
        let (v, b1) = positive[i].overflowing_sub(negative[i]);
        let (v, b2) = v.overflowing_sub(u64::from(borrow));
        positive[i] = v;
        borrow = b1 || b2;
    }
    let top = positive.iter().rposition(|word| *word != 0).unwrap();
    let high = 63 - positive[top].leading_zeros() as i32;
    let mut leading = positive[top] as f64;
    if top > 0 {
        leading += positive[top - 1] as f64 / 18446744073709551616.0;
    }
    Exact {
        sign,
        magnitude: Magnitude::new(
            leading / (1_u64 << high) as f64,
            base + (top * 64) as i32 + high,
        ),
    }
}
fn two(a: f64, b: f64, subtract: bool) -> Term {
    Term::product(
        [
            Dyadic::new(a),
            Dyadic::new(b),
            Dyadic::new(1.0),
            Dyadic::new(1.0),
        ],
        subtract,
        false,
    )
}
/// Exact C=|origin|²-radius²: equality identifies the t=0 surface boundary.
pub(super) fn surface(origin: [f64; 3], radius: f64) -> Exact {
    sum(&[
        two(origin[0], origin[0], false),
        two(origin[1], origin[1], false),
        two(origin[2], origin[2], false),
        two(radius, radius, true),
    ])
}
/// Exact B=origin·direction sign classifies outside forward/behind rays.
pub(super) fn along(origin: [f64; 3], direction: [f64; 3]) -> (Exact, Magnitude) {
    let b = sum(&std::array::from_fn::<_, 3, _>(|i| {
        two(origin[i], direction[i], false)
    }));
    let a = sum(&std::array::from_fn::<_, 3, _>(|i| {
        two(direction[i], direction[i], false)
    }))
    .magnitude;
    (b, a.sqrt())
}
/// Perpendicular closest point: (d×(o×d))/|d|². Each signed cubic
/// numerator is cancelled exactly before division, so no origin-size residual
/// contaminates a small sphere hit on a distant center-directed line.
pub(super) fn closest(origin: [f64; 3], direction: [f64; 3]) -> [f64; 3] {
    let o = origin.map(Dyadic::new);
    let d = direction.map(Dyadic::new);
    let one = Dyadic::new(1.0);
    let a = sum(&std::array::from_fn::<_, 3, _>(|i| {
        two(direction[i], direction[i], false)
    }))
    .magnitude;
    std::array::from_fn(|i| {
        let mut terms = [Term::default(); 4];
        let mut n = 0;
        for j in 0..3 {
            if i != j {
                terms[n] = Term::product([o[i], d[j], d[j], one], false, false);
                terms[n + 1] = Term::product([o[j], d[i], d[j], one], true, false);
                n += 2;
            }
        }
        let result = sum(&terms);
        let value = result.magnitude.divide(a).value();
        if result.sign == Ordering::Less {
            -value
        } else {
            value
        }
    })
}
/// None is a strict miss; zero is an exact tangent. The chord remains scaled
/// until final reconstruction so even tiny positive chords keep their sign.
pub(super) fn half_chord(origin: [f64; 3], direction: [f64; 3], radius: f64) -> Option<Magnitude> {
    let o = origin.map(Dyadic::new);
    let d = direction.map(Dyadic::new);
    let r = Dyadic::new(radius);
    let mut terms = [Term::default(); 12];
    for i in 0..3 {
        terms[i] = Term::product([r, r, d[i], d[i]], false, false);
    }
    let mut index = 3;
    for (i, j) in [(0, 1), (0, 2), (1, 2)] {
        terms[index] = Term::product([o[i], o[i], d[j], d[j]], true, false);
        terms[index + 1] = Term::product([o[j], o[j], d[i], d[i]], true, false);
        terms[index + 2] = Term::product([o[i], o[j], d[i], d[j]], false, true);
        index += 3;
    }
    let discriminant = sum(&terms);
    if discriminant.sign == Ordering::Less {
        return None;
    }
    let a = sum(&std::array::from_fn::<_, 3, _>(|i| {
        two(direction[i], direction[i], false)
    }))
    .magnitude;
    Some(discriminant.magnitude.sqrt().divide(a.sqrt()))
}
fn scale_binary(value: f64, power: i32) -> f64 {
    if value == 0.0 {
        return 0.0;
    }
    if power > 1023 {
        (value * 2.0_f64.powi(power - 512)) * 2.0_f64.powi(512)
    } else if power < -1022 {
        (value * 2.0_f64.powi(power + 512)) * 2.0_f64.powi(-512)
    } else {
        value * 2.0_f64.powi(power)
    }
}
