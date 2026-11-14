//! Scene-owned Bevy font shaping. These CPU glyph atlases never enter the
//! application's UI font resources or render-world image collection.
use bevy::{
    asset::AssetId,
    prelude::*,
    text::{
        ComputedTextBlock, FontAtlasSet, FontCx, FontHinting, LayoutCx, LetterSpacing, LineHeight,
        RemSize, ScaleCx, TextBounds, TextElement, TextLayoutInfo, TextPipeline,
    },
};
use std::{collections::HashMap, sync::Arc};

pub const RASTER_SIZE: f32 = 64.;
const MAX_GLYPHS: usize = 4096;
const MAX_MASK_BYTES: usize = 16 * 1024 * 1024;
#[derive(Clone, Debug)]
pub struct GlyphMask {
    pub key: u32,
    pub width: usize,
    pub height: usize,
    pub alpha: Vec<u8>,
}
#[derive(Clone, Debug)]
pub struct Glyph {
    /// Pixel coordinates centered on the whole block; positive Y points up.
    pub center: Vec2,
    pub mask: Arc<GlyphMask>,
}
#[derive(Clone, Debug)]
pub struct Layout {
    pub size: Vec2,
    pub glyphs: Vec<Glyph>,
}
#[derive(Clone, Copy, Debug)]
pub struct Metrics {
    pub units: f32,
    pub ascender: f32,
    pub descender: f32,
    pub gap: f32,
}
impl Metrics {
    fn from_ttf(bytes: &[u8]) -> Result<Self, String> {
        let u16_at = |data: &[u8], at: usize| {
            data.get(at..at + 2)
                .and_then(|b| b.try_into().ok())
                .map(u16::from_be_bytes)
        };
        let u32_at = |data: &[u8], at: usize| {
            data.get(at..at + 4)
                .and_then(|b| b.try_into().ok())
                .map(u32::from_be_bytes)
        };
        let table = |tag: &[u8; 4]| -> Option<&[u8]> {
            let count = usize::from(u16_at(bytes, 4)?);
            for i in 0..count {
                let offset = 12 + i * 16;
                if bytes.get(offset..offset + 4)? == tag {
                    let start = usize::try_from(u32_at(bytes, offset + 8)?).ok()?;
                    let len = usize::try_from(u32_at(bytes, offset + 12)?).ok()?;
                    return bytes.get(start..start.checked_add(len)?);
                }
            }
            None
        };
        let head = table(b"head").ok_or("Missing label font head table")?;
        let units = f32::from(u16_at(head, 18).ok_or("Missing label font units")?);
        let (metrics, at) = if let Some(os2) = table(b"OS/2").filter(|t| t.len() >= 74) {
            (os2, 68)
        } else {
            (table(b"hhea").ok_or("Missing label font metrics")?, 4)
        };
        let signed = |at| {
            u16_at(metrics, at)
                .map(|v| f32::from(v as i16))
                .ok_or_else(|| "Incomplete label font metrics".to_owned())
        };
        let result = Self {
            units,
            ascender: signed(at)?,
            descender: signed(at + 2)?,
            gap: signed(at + 4)?,
        };
        if result.units <= 0. || result.ascender - result.descender + result.gap <= 0. {
            return Err("Degenerate label font metrics".into());
        }
        Ok(result)
    }
    pub fn line_height(self) -> f32 {
        (self.ascender - self.descender + self.gap) / self.units * RASTER_SIZE
    }
}

type Key = (AssetId<Image>, [u32; 4]);
pub struct Shaper {
    font: TextFont,
    metrics: Metrics,
    cx: FontCx,
    layout: LayoutCx,
    scale: ScaleCx,
    pipeline: TextPipeline,
    atlases: FontAtlasSet,
    images: Assets<Image>,
    masks: HashMap<Key, Arc<GlyphMask>>,
    mask_bytes: usize,
}
impl Shaper {
    pub fn new(bytes: Vec<u8>) -> Result<Self, String> {
        let metrics = Metrics::from_ttf(&bytes)?;
        let font = Font::from_bytes(bytes);
        let mut cx = FontCx::default();
        let registered = cx.collection.register_fonts(font.data, None);
        let id = registered
            .first()
            .ok_or("Label font contains no usable face")?
            .0;
        let family = cx
            .collection
            .family_name(id)
            .ok_or("Label font family is missing")?
            .to_owned();
        let font = TextFont {
            font: FontSource::family(family),
            font_size: FontSize::Px(RASTER_SIZE),
            ..default()
        };
        Ok(Self {
            font,
            metrics,
            cx,
            layout: LayoutCx::default(),
            scale: ScaleCx::default(),
            pipeline: TextPipeline::default(),
            atlases: FontAtlasSet::default(),
            images: Assets::default(),
            masks: HashMap::new(),
            mask_bytes: 0,
        })
    }
    pub fn glyphs(&self) -> usize {
        self.masks.len()
    }
    pub fn bytes(&self) -> usize {
        self.mask_bytes
    }
    /// Source maxWidth .18 at fontSize .045 is four ems, or256pixels.
    /// Credit amounts use the same glyph cache with unbounded single-line width.
    pub fn shape(&mut self, text: &str, wrap: bool) -> Result<Layout, String> {
        if text.len() > 128 {
            return Err("Label text exceeds public presentation bounds".into());
        }
        let bounds = TextBounds {
            width: wrap.then_some(RASTER_SIZE * 4.),
            height: None,
        };
        let mut computed = ComputedTextBlock::default();
        self.pipeline
            .update_buffer(
                &Assets::<Font>::default(),
                std::iter::once((
                    Entity::PLACEHOLDER,
                    0,
                    TextElement::Text {
                        text,
                        font: &self.font,
                        color: Color::WHITE,
                        line_height: LineHeight::Px(self.metrics.line_height()),
                        letter_spacing: LetterSpacing::default(),
                    },
                )),
                LineBreak::WordBoundary,
                Justify::Center,
                bounds,
                1.,
                &mut computed,
                &mut self.cx,
                &mut self.layout,
                Vec2::ONE,
                RemSize(16.),
            )
            .map_err(|e| e.to_string())?;
        let mut info = TextLayoutInfo::default();
        self.pipeline
            .update_text_layout_info(
                &mut info,
                &mut self.atlases,
                &mut self.images,
                &mut computed,
                &mut self.scale,
                bounds,
                Justify::Center,
                FontHinting::Disabled,
            )
            .map_err(|e| e.to_string())?;
        let lines: Vec<_> = computed
            .buffer()
            .lines()
            .map(|line| *line.metrics())
            .collect();
        let line_height = self.metrics.line_height();
        let soft_wrap = lines.len() > text.split('\n').count();
        let width = if soft_wrap {
            computed.buffer().width().max(RASTER_SIZE * 4.)
        } else {
            computed.buffer().width()
        };
        let size = Vec2::new(width, lines.len() as f32 * line_height);
        let ascender = self.metrics.ascender / self.metrics.units * RASTER_SIZE;
        let leading = self.metrics.gap / self.metrics.units * RASTER_SIZE * 0.5;
        let glyphs = info
            .glyphs
            .into_iter()
            .map(|g| {
                let rect = g.atlas_info.rect;
                let key = (
                    g.atlas_info.texture,
                    [
                        rect.min.x.to_bits(),
                        rect.min.y.to_bits(),
                        rect.max.x.to_bits(),
                        rect.max.y.to_bits(),
                    ],
                );
                let mask = if let Some(mask) = self.masks.get(&key) {
                    Arc::clone(mask)
                } else {
                    let image = self
                        .images
                        .get(g.atlas_info.texture)
                        .ok_or("Label glyph atlas is missing")?;
                    let pixels = image
                        .data
                        .as_ref()
                        .ok_or("Label glyph atlas has no pixels")?;
                    let width = rect.width() as usize;
                    let height = rect.height() as usize;
                    let count = width
                        .checked_mul(height)
                        .ok_or("Label glyph size overflow")?;
                    if width > 1024
                        || height > 1024
                        || self.masks.len() >= MAX_GLYPHS
                        || self
                            .mask_bytes
                            .checked_add(count)
                            .is_none_or(|n| n > MAX_MASK_BYTES)
                    {
                        return Err("Label glyph cache exceeds renderer bounds".to_owned());
                    }
                    let mut alpha = Vec::with_capacity(count);
                    for y in 0..height {
                        for x in 0..width {
                            let offset = (((rect.min.y as usize + y) * image.width() as usize)
                                + rect.min.x as usize
                                + x)
                                * 4
                                + 3;
                            alpha.push(
                                *pixels
                                    .get(offset)
                                    .ok_or("Label glyph lies outside its atlas")?,
                            );
                        }
                    }
                    let mask = Arc::new(GlyphMask {
                        key: self.masks.len() as u32,
                        width,
                        height,
                        alpha,
                    });
                    self.mask_bytes += count;
                    self.masks.insert(key, Arc::clone(&mask));
                    mask
                };
                if mask.alpha.iter().all(|alpha| *alpha == 0) {
                    return Ok(None);
                }
                let line = lines
                    .get(g.line_index as usize)
                    .ok_or("Label glyph line is missing")?;
                let source_baseline =
                    size.y / 2. - g.line_index as f32 * line_height - ascender - leading;
                Ok(Some(Glyph {
                    center: Vec2::new(
                        g.position.x - line.offset - (line.advance - line.trailing_whitespace) / 2.,
                        source_baseline + line.baseline - g.position.y,
                    ),
                    mask,
                }))
            })
            .collect::<Result<Vec<_>, String>>()?;
        Ok(Layout {
            size,
            glyphs: glyphs.into_iter().flatten().collect(),
        })
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    const FONT: &[u8] = include_bytes!("../../../assets/original/fonts/prototype.ttf");
    #[test]
    fn actual_prototype_font_shapes_wraps_and_reuses_owned_glyphs() {
        let mut shaper = Shaper::new(FONT.to_vec()).unwrap();
        let a = shaper.shape("Arsia Mons", true).unwrap();
        assert_eq!(a.glyphs.len(), 9);
        assert!(a.size.x <= 256. && a.size.y > 0.);
        assert!(
            a.glyphs
                .iter()
                .all(|g| g.center.is_finite() && g.mask.alpha.iter().any(|a| *a > 0))
        );
        let count = shaper.glyphs();
        let bytes = shaper.bytes();
        let b = shaper.shape("Arsia Mons", true).unwrap();
        assert_eq!(a.size, b.size);
        assert_eq!(shaper.glyphs(), count);
        assert_eq!(shaper.bytes(), bytes);
        for (a, b) in a.glyphs.iter().zip(&b.glyphs) {
            assert!(Arc::ptr_eq(&a.mask, &b.mask));
            assert_eq!(a.center, b.center);
        }
        let long = shaper.shape("Olympus Mons Long Name", true).unwrap();
        assert!(long.size.y > a.size.y);
        let digits = shaper.shape("4096", false).unwrap();
        assert_eq!(digits.glyphs.len(), 4);
        assert!(shaper.shape(&"M".repeat(129), true).is_err());
        assert!(Shaper::new(Vec::new()).is_err());
    }
}
