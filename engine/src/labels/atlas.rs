//! Scene-owned, padded distance-field pages shared by all occurrences of a glyph.
use super::font::GlyphMask;
use std::collections::{BTreeMap, BTreeSet};
pub const SIDE: usize = 1024;
pub const RANGE: f32 = 8.;
pub const PAD: usize = 10;
const MAX_PAGES: usize = 8;
const FAR: f64 = 1e12;

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Slot {
    pub page: usize,
    pub x: usize,
    pub y: usize,
    pub width: usize,
    pub height: usize,
}
pub struct Page {
    pub pixels: Vec<u8>,
    x: usize,
    y: usize,
    row: usize,
}
impl Default for Page {
    fn default() -> Self {
        Self {
            pixels: vec![0; SIDE * SIDE * 4],
            x: 0,
            y: 0,
            row: 0,
        }
    }
}
#[derive(Default)]
pub struct Book {
    pub pages: Vec<Page>,
    pub dirty: BTreeSet<usize>,
    slots: BTreeMap<u32, Slot>,
}
impl Book {
    pub fn glyphs(&self) -> usize {
        self.slots.len()
    }
    pub fn insert(&mut self, mask: &GlyphMask) -> Result<Slot, String> {
        if let Some(slot) = self.slots.get(&mask.key) {
            return Ok(*slot);
        }
        let width = mask
            .width
            .checked_add(PAD * 2)
            .ok_or("Label glyph width overflow")?;
        let height = mask
            .height
            .checked_add(PAD * 2)
            .ok_or("Label glyph height overflow")?;
        if mask.width == 0
            || mask.height == 0
            || mask.width.checked_mul(mask.height) != Some(mask.alpha.len())
            || width > SIDE
            || height > SIDE
        {
            return Err("Invalid label glyph mask".into());
        }
        let mut page = self.pages.len().saturating_sub(1);
        if self.pages.is_empty() {
            self.pages.push(Page::default());
        }
        if self.pages[page].x + width > SIDE {
            let p = &mut self.pages[page];
            p.x = 0;
            p.y += p.row;
            p.row = 0;
        }
        if self.pages[page].y + height > SIDE {
            if self.pages.len() >= MAX_PAGES {
                return Err("Label atlas exceeds 32MiB renderer bounds".into());
            }
            page = self.pages.len();
            self.pages.push(Page::default());
        }
        let p = &mut self.pages[page];
        let slot = Slot {
            page,
            x: p.x,
            y: p.y,
            width,
            height,
        };
        let pixels = distance_field(mask)?;
        for y in 0..height {
            let at = ((slot.y + y) * SIDE + slot.x) * 4;
            p.pixels[at..at + width * 4]
                .copy_from_slice(&pixels[y * width * 4..(y + 1) * width * 4]);
        }
        p.x += width;
        p.row = p.row.max(height);
        self.slots.insert(mask.key, slot);
        self.dirty.insert(page);
        Ok(slot)
    }
}
/// Exact squared Euclidean distance by separable lower envelopes of parabolas.
fn line(input: &[f64], output: &mut [f64]) {
    let n = input.len();
    let mut v = vec![0; n];
    let mut z = vec![0.; n + 1];
    let mut k = 0;
    z[0] = f64::NEG_INFINITY;
    z[1] = f64::INFINITY;
    for q in 1..n {
        let intersect = |p: usize| {
            ((input[q] + (q * q) as f64) - (input[p] + (p * p) as f64)) / (2. * (q - p) as f64)
        };
        let mut s = intersect(v[k]);
        while s <= z[k] {
            k -= 1;
            s = intersect(v[k]);
        }
        k += 1;
        v[k] = q;
        z[k] = s;
        z[k + 1] = f64::INFINITY;
    }
    k = 0;
    for (q, d) in output.iter_mut().enumerate() {
        while z[k + 1] < (q as f64) {
            k += 1;
        }
        *d = (q as f64 - v[k] as f64).powi(2) + input[v[k]];
    }
}
fn distances(seeds: &[bool], width: usize, height: usize) -> Vec<f64> {
    let mut intermediate = vec![FAR; width * height];
    let mut row = vec![0.; width];
    for y in 0..height {
        for x in 0..width {
            row[x] = if seeds[y * width + x] { 0. } else { FAR };
        }
        line(&row, &mut intermediate[y * width..(y + 1) * width]);
    }
    let mut column = vec![0.; height];
    let mut result = vec![0.; height];
    let mut output = vec![0.; width * height];
    for x in 0..width {
        for y in 0..height {
            column[y] = intermediate[y * width + x];
        }
        line(&column, &mut result);
        for y in 0..height {
            output[y * width + x] = result[y];
        }
    }
    output
}
pub fn distance_field(mask: &GlyphMask) -> Result<Vec<u8>, String> {
    let width = mask
        .width
        .checked_add(PAD * 2)
        .ok_or("Label mask width overflow")?;
    let height = mask
        .height
        .checked_add(PAD * 2)
        .ok_or("Label mask height overflow")?;
    if mask.width == 0
        || mask.height == 0
        || width > SIDE
        || height > SIDE
        || mask.width.checked_mul(mask.height) != Some(mask.alpha.len())
    {
        return Err("Invalid label mask dimensions".into());
    }
    let mut alpha = vec![0; width * height];
    for y in 0..mask.height {
        let at = (y + PAD) * width + PAD;
        alpha[at..at + mask.width]
            .copy_from_slice(&mask.alpha[y * mask.width..(y + 1) * mask.width]);
    }
    let inside: Vec<_> = alpha.iter().map(|a| *a >= 128).collect();
    let outside: Vec<_> = inside.iter().map(|v| !*v).collect();
    let inner = distances(&inside, width, height);
    let outer = distances(&outside, width, height);
    let mut output = vec![0; width * height * 4];
    for i in 0..alpha.len() {
        let a = alpha[i];
        let signed = if a > 0 && a < 255 {
            f64::from(a) / 255. - 0.5
        } else if inside[i] {
            outer[i].sqrt() - 0.5
        } else {
            0.5 - inner[i].sqrt()
        };
        output[i * 4] = ((0.5 + signed / f64::from(2. * RANGE)).clamp(0., 1.) * 255.).round() as u8;
        output[i * 4 + 3] = 255;
    }
    Ok(output)
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn separable_distance_matches_independent_exhaustive_search() {
        for width in 1..8 {
            for height in 1..7 {
                let seeds: Vec<_> = (0..width * height)
                    .map(|i| i % 5 == 0 || i == width * height - 1)
                    .collect();
                let actual = distances(&seeds, width, height);
                for y in 0..height {
                    for x in 0..width {
                        let expected = seeds
                            .iter()
                            .enumerate()
                            .filter(|(_, v)| **v)
                            .map(|(i, _)| {
                                ((x as f64 - (i % width) as f64).powi(2)
                                    + (y as f64 - (i / width) as f64).powi(2))
                            })
                            .fold(f64::INFINITY, f64::min);
                        assert_eq!(actual[y * width + x], expected);
                    }
                }
            }
        }
    }
    #[test]
    fn outline_padding_alpha_edges_and_shared_custody_are_preserved() {
        let mask = GlyphMask {
            key: 7,
            width: 3,
            height: 3,
            alpha: vec![0, 0, 0, 0, 255, 192, 0, 0, 0],
        };
        let pixels = distance_field(&mask).unwrap();
        let width = 3 + PAD * 2;
        let signed = |x, y| (f32::from(pixels[(y * width + x) * 4]) / 255. - 0.5) * 2. * RANGE;
        assert!((signed(PAD + 1, PAD + 1) - 0.5).abs() < 0.04);
        assert!((signed(PAD, PAD + 1) + 0.5).abs() < 0.04);
        assert!((signed(PAD + 2, PAD + 1) - (192. / 255. - 0.5)).abs() < 0.04);
        assert_eq!(pixels[0], 0);
        let mut book = Book::default();
        let a = book.insert(&mask).unwrap();
        assert_eq!(a.width, width);
        assert_eq!(book.pages.len(), 1);
        book.dirty.clear();
        let b = book.insert(&mask).unwrap();
        assert_eq!(a, b);
        assert!(book.dirty.is_empty());
        assert_eq!(book.glyphs(), 1);
        assert!(
            book.insert(&GlyphMask {
                key: 9,
                width: 1024,
                height: 1,
                alpha: vec![255; 1024]
            })
            .is_err()
        );
    }
}
