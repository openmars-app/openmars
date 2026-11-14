//! Preserve every original surface and animation attribute when plot slices retire.
use super::{Coordinate, Slice};
#[cfg(test)]
use super::{GROUPS, GroupPool};
use bevy::{
    asset::RenderAssetUsages,
    mesh::{Indices, VertexAttributeValues},
    prelude::*,
};
use std::collections::BTreeMap;
#[cfg(test)]
use std::collections::BTreeSet;
fn err(s: &str) -> String {
    s.into()
}
pub(super) fn compact_mesh(
    source: &Mesh,
    members: &BTreeMap<Coordinate, Slice>,
) -> Result<(Mesh, BTreeMap<Coordinate, Slice>), String> {
    let mut ranges = BTreeMap::new();
    let (mut vertices, mut index_count) = (0, 0);
    let mut indices = Vec::new();
    let Some(Indices::U32(original)) = source.indices() else {
        return Err(err("structure chunk requires U32 indices"));
    };
    for (&coordinate, slice) in members {
        let end = vertices + slice.vertices.len();
        let index_end = index_count + slice.indices.len();
        for index in original
            .get(slice.indices.clone())
            .ok_or_else(|| err("structure index custody"))?
        {
            let local = (*index as usize)
                .checked_sub(slice.vertices.start)
                .filter(|i| *i < slice.vertices.len())
                .ok_or_else(|| err("structure index crosses plot"))?;
            indices.push(
                u32::try_from(vertices + local).map_err(|_| err("structure index overflow"))?,
            );
        }
        ranges.insert(
            coordinate,
            Slice {
                vertices: vertices..end,
                indices: index_count..index_end,
            },
        );
        vertices = end;
        index_count = index_end;
    }
    let mut mesh = Mesh::new(
        bevy::mesh::PrimitiveTopology::TriangleList,
        RenderAssetUsages::MAIN_WORLD | RenderAssetUsages::RENDER_WORLD,
    );
    for (attribute, values) in source.attributes() {
        macro_rules! copy {
            ($values:ident,$variant:ident) => {{
                let mut result = Vec::with_capacity(vertices);
                for slice in members.values() {
                    result.extend_from_slice(
                        $values
                            .get(slice.vertices.clone())
                            .ok_or_else(|| err("structure attribute custody"))?,
                    );
                }
                VertexAttributeValues::$variant(result)
            }};
        }
        let values = match values {
            VertexAttributeValues::Float32x2(v) => copy!(v, Float32x2),
            VertexAttributeValues::Float32x3(v) => copy!(v, Float32x3),
            VertexAttributeValues::Float32x4(v) => copy!(v, Float32x4),
            _ => return Err(err("unexpected structure attribute")),
        };
        mesh.insert_attribute(*attribute, values);
    }
    mesh.insert_indices(Indices::U32(indices));
    Ok((mesh, ranges))
}

#[cfg(test)]
mod tests {
    use super::*;
    fn triangle(offset: f32, group: f32) -> Mesh {
        Mesh::new(
            bevy::mesh::PrimitiveTopology::TriangleList,
            RenderAssetUsages::MAIN_WORLD,
        )
        .with_inserted_attribute(
            Mesh::ATTRIBUTE_POSITION,
            vec![[offset, 0., 0.], [offset + 1., 0., 0.], [offset, 1., 0.]],
        )
        .with_inserted_attribute(Mesh::ATTRIBUTE_NORMAL, vec![[0., 0., 1.]; 3])
        .with_inserted_attribute(Mesh::ATTRIBUTE_UV_0, vec![[0., 0.], [1., 0.], [0., 1.]])
        .with_inserted_attribute(Mesh::ATTRIBUTE_UV_1, vec![[0.1, group]; 3])
        .with_inserted_attribute(Mesh::ATTRIBUTE_COLOR, vec![[0.2, 0.3, 0.4, 1.]; 3])
        .with_inserted_attribute(Mesh::ATTRIBUTE_TANGENT, vec![[1., 0., 0., -1.]; 3])
        .with_inserted_indices(Indices::U32(vec![0, 1, 2]))
    }
    #[test]
    fn removing_and_reordering_plots_preserves_all_surface_and_animation_attributes() {
        let mut source = triangle(10., 17.);
        source.merge(&triangle(20., 31.)).unwrap();
        source.merge(&triangle(30., 93.)).unwrap();
        let members = BTreeMap::from([
            (
                (-1, 0, 1),
                Slice {
                    vertices: 6..9,
                    indices: 6..9,
                },
            ),
            (
                (1, 0, -1),
                Slice {
                    vertices: 0..3,
                    indices: 0..3,
                },
            ),
        ]);
        let (next, ranges) = compact_mesh(&source, &members).unwrap();
        assert_eq!(next.count_vertices(), 6);
        assert_eq!(
            next.indices().unwrap().iter().collect::<Vec<_>>(),
            vec![0, 1, 2, 3, 4, 5]
        );
        assert_eq!(ranges[&(-1, 0, 1)].vertices, 0..3);
        assert_eq!(ranges[&(1, 0, -1)].vertices, 3..6);
        for (attribute, original) in source.attributes() {
            let retained = next.attribute(attribute.id).unwrap();
            macro_rules! check {
                ($variant:ident) => {
                    if let (
                        VertexAttributeValues::$variant(old),
                        VertexAttributeValues::$variant(new),
                    ) = (original, retained)
                    {
                        assert_eq!(new, &[&old[6..9], &old[..3]].concat());
                    } else {
                        panic!("attribute type changed");
                    }
                };
            }
            match original {
                VertexAttributeValues::Float32x2(_) => check!(Float32x2),
                VertexAttributeValues::Float32x3(_) => check!(Float32x3),
                VertexAttributeValues::Float32x4(_) => check!(Float32x4),
                _ => panic!(),
            }
        }
    }
    #[test]
    fn compaction_rejects_indices_that_escape_their_plot_or_missing_ranges() {
        let mut source = triangle(0., 0.);
        source.merge(&triangle(2., 1.)).unwrap();
        let mut members = BTreeMap::from([(
            (0, 0, 0),
            Slice {
                vertices: 0..3,
                indices: 3..6,
            },
        )]);
        assert!(compact_mesh(&source, &members).is_err());
        members.get_mut(&(0, 0, 0)).unwrap().indices = 99..102;
        assert!(compact_mesh(&source, &members).is_err());
    }
    #[test]
    fn animation_groups_fail_at_capacity_and_clear_all_rows_on_retirement() {
        let mut pool = GroupPool::new();
        let ids = (0..GROUPS)
            .map(|_| pool.allocate().unwrap())
            .collect::<BTreeSet<_>>();
        assert_eq!(ids.len(), GROUPS);
        assert!(pool.allocate().is_err());
        pool.set(81, [[7.; 4]; 6]);
        pool.release(81);
        assert_eq!(pool.allocate().unwrap(), 81);
        assert_eq!(&pool.rows[81 * 6..82 * 6], &[[0.; 4]; 6]);
    }
}
