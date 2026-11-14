//! Shared-page text quads and aspect-correct source bonus planes.
use super::{
    atlas::{Book, PAD, SIDE},
    content::{CREDIT_SIZE, Content, FONT_SIZE, ICON_SIZE},
    font::{RASTER_SIZE, Shaper},
};
use bevy::{
    asset::RenderAssetUsages,
    mesh::{Indices, PrimitiveTopology},
    prelude::*,
};
use std::collections::BTreeMap;
#[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord)]
pub enum Key {
    Name(usize),
    Credit(usize),
    Icon(&'static str),
}
#[derive(Default)]
struct Data {
    positions: Vec<[f32; 3]>,
    uvs: Vec<[f32; 2]>,
    indices: Vec<u32>,
}
impl Data {
    fn quad(&mut self, center: Vec3, size: Vec2, uv: [Vec2; 2]) -> Result<(), String> {
        if self.positions.len() + 4 > 262144 {
            return Err("Label geometry exceeds renderer bounds".into());
        }
        let i = self.positions.len() as u32;
        let half = size / 2.;
        self.positions.extend(
            [
                [-half.x, -half.y],
                [half.x, -half.y],
                [half.x, half.y],
                [-half.x, half.y],
            ]
            .map(|p| (center + Vec3::new(p[0], p[1], 0.)).to_array()),
        );
        self.uvs.extend([
            [uv[0].x, uv[1].y],
            [uv[1].x, uv[1].y],
            [uv[1].x, uv[0].y],
            [uv[0].x, uv[0].y],
        ]);
        self.indices.extend([i, i + 1, i + 2, i, i + 2, i + 3]);
        Ok(())
    }
    fn mesh(self) -> Mesh {
        Mesh::new(
            PrimitiveTopology::TriangleList,
            RenderAssetUsages::MAIN_WORLD | RenderAssetUsages::RENDER_WORLD,
        )
        .with_inserted_attribute(Mesh::ATTRIBUTE_POSITION, self.positions)
        .with_inserted_attribute(Mesh::ATTRIBUTE_UV_0, self.uvs)
        .with_inserted_indices(Indices::U32(self.indices))
    }
}
fn text(
    data: &mut BTreeMap<Key, Data>,
    shaper: &mut Shaper,
    book: &mut Book,
    value: &str,
    credit: bool,
    offset: Vec3,
) -> Result<(), String> {
    let layout = shaper.shape(value, !credit)?;
    let scale = (if credit { CREDIT_SIZE } else { FONT_SIZE }) as f32 / RASTER_SIZE;
    for glyph in layout.glyphs {
        let slot = book.insert(&glyph.mask)?;
        let key = if credit {
            Key::Credit(slot.page)
        } else {
            Key::Name(slot.page)
        };
        let center = offset + glyph.center.extend(0.) * scale;
        let size = Vec2::new(
            (glyph.mask.width + PAD * 2) as f32,
            (glyph.mask.height + PAD * 2) as f32,
        ) * scale;
        let uv = [
            Vec2::new(slot.x as f32, slot.y as f32) / SIDE as f32,
            Vec2::new((slot.x + slot.width) as f32, (slot.y + slot.height) as f32) / SIDE as f32,
        ];
        data.entry(key).or_default().quad(center, size, uv)?;
    }
    Ok(())
}
pub fn build(
    content: &Content,
    shaper: &mut Shaper,
    book: &mut Book,
    dimensions: impl Fn(&str) -> Option<UVec2>,
) -> Result<BTreeMap<Key, Mesh>, String> {
    let mut data = BTreeMap::new();
    if let Some(name) = &content.name {
        text(&mut data, shaper, book, name, false, Vec3::ZERO)?;
    }
    for icon in &content.icons {
        let d = dimensions(icon.asset)
            .ok_or_else(|| format!("Bonus icon {} is not decoded", icon.asset))?;
        if d.min_element() == 0 {
            return Err("Bonus icon has zero dimensions".into());
        }
        let size = d.as_vec2() / d.max_element() as f32 * ICON_SIZE as f32;
        let center = Vec3::new(icon.x as f32, content.icon_y as f32, 0.);
        data.entry(Key::Icon(icon.asset))
            .or_insert_with(Data::default)
            .quad(center, size, [Vec2::ZERO, Vec2::ONE])?;
        if let Some(amount) = icon.credit {
            text(
                &mut data,
                shaper,
                book,
                &amount.to_string(),
                true,
                center + Vec3::Z * 0.002,
            )?;
        }
    }
    Ok(data
        .into_iter()
        .map(|(key, data)| (key, data.mesh()))
        .collect())
}
#[cfg(test)]
mod tests {
    use super::*;
    use crate::labels::content::Icon;
    #[test]
    fn actual_font_geometry_shares_pages_and_credit_numbers_have_separate_source_depth() {
        let mut shaper =
            Shaper::new(include_bytes!("../../../assets/original/fonts/prototype.ttf").to_vec())
                .unwrap();
        let mut book = Book::default();
        let content = Content {
            name: Some("Arsia Mons".into()),
            icon_y: -0.08,
            icons: vec![Icon {
                asset: "icons/resources/megacredit",
                x: 0.,
                credit: Some(12),
            }],
        };
        let meshes = build(&content, &mut shaper, &mut book, |_| {
            Some(UVec2::new(256, 128))
        })
        .unwrap();
        assert_eq!(book.pages.len(), 1);
        assert_eq!(meshes.len(), 3);
        assert_eq!(meshes[&Key::Name(0)].count_vertices(), 9 * 4);
        assert_eq!(meshes[&Key::Credit(0)].count_vertices(), 2 * 4);
        let icon = &meshes[&Key::Icon("icons/resources/megacredit")];
        let bevy::mesh::VertexAttributeValues::Float32x3(p) =
            icon.attribute(Mesh::ATTRIBUTE_POSITION).unwrap()
        else {
            panic!()
        };
        assert!((p[1][0] - p[0][0] - 0.05).abs() < 1e-7);
        assert!((p[3][1] - p[0][1] - 0.025).abs() < 1e-7);
        let bevy::mesh::VertexAttributeValues::Float32x3(p) = meshes[&Key::Credit(0)]
            .attribute(Mesh::ATTRIBUTE_POSITION)
            .unwrap()
        else {
            panic!()
        };
        assert!(p.iter().all(|p| (p[2] - 0.002).abs() < 1e-7));
        let before = book.glyphs();
        book.dirty.clear();
        build(&content, &mut shaper, &mut book, |_| {
            Some(UVec2::new(256, 128))
        })
        .unwrap();
        assert_eq!(before, book.glyphs());
        assert!(book.dirty.is_empty());
    }
}
