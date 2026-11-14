use openmars_scene::structure::{layout::generate, *};
use serde_json::json;

#[test]
fn indexed_storage_preserves_every_triangle_surface_and_independent_animation_group() {
    use super::geometry;
    use bevy::mesh::VertexAttributeValues;
    let request = serde_json::from_value(
        json!({"plan":"grid","density":"dense","details":["pipes","gatehouses"]}),
    )
    .unwrap();
    let layout = generate(28881, &request, "Dense city").unwrap();
    let original = geometry::generate(layout.layout()).unwrap();
    let mut vertices_before = 0;
    let mut vertices_after = 0;
    for batch in &original.batches {
        let indexed = batch.indexed(73).unwrap();
        let mesh = &indexed.mesh;
        let VertexAttributeValues::Float32x3(positions) = mesh
            .attribute(bevy::prelude::Mesh::ATTRIBUTE_POSITION)
            .unwrap()
        else {
            panic!()
        };
        let VertexAttributeValues::Float32x2(animation) =
            mesh.attribute(bevy::prelude::Mesh::ATTRIBUTE_UV_1).unwrap()
        else {
            panic!()
        };
        let VertexAttributeValues::Float32x4(colors) = mesh
            .attribute(bevy::prelude::Mesh::ATTRIBUTE_COLOR)
            .unwrap()
        else {
            panic!()
        };
        let VertexAttributeValues::Float32x4(tangents) = mesh
            .attribute(bevy::prelude::Mesh::ATTRIBUTE_TANGENT)
            .unwrap()
        else {
            panic!()
        };
        let VertexAttributeValues::Float32x3(normals) = mesh
            .attribute(bevy::prelude::Mesh::ATTRIBUTE_NORMAL)
            .unwrap()
        else {
            panic!()
        };
        for (t, n) in tangents.iter().zip(normals) {
            assert!(t.iter().all(|v| v.is_finite()));
            assert!((t[0] * n[0] + t[1] * n[1] + t[2] * n[2]).abs() < 1e-4);
            assert!((t[..3].iter().map(|v| v * v).sum::<f32>() - 1.).abs() < 1e-4);
            assert!(t[3] == 1. || t[3] == -1.);
        }
        let indices = mesh.indices().unwrap().iter().collect::<Vec<_>>();
        assert_eq!(indices.len(), batch.positions.len());
        for (source, &index) in indices.iter().enumerate() {
            assert_eq!(positions[index], batch.positions[source]);
            assert_eq!(colors[index], batch.colors[source]);
            assert_eq!(animation[index][0], batch.frost[source]);
            let group = &indexed.groups[animation[index][1] as usize - 73];
            assert_eq!(group.pivot, batch.pivots[source]);
            assert_eq!(group.motion, batch.collapse[source]);
        }
        vertices_before += batch.positions.len();
        vertices_after += positions.len();
        assert!(indexed.bytes > 0);
        assert!(batch.indexed(u32::MAX).is_err());
    }
    assert!(
        vertices_after < vertices_before,
        "shared triangles should not retain duplicate attribute storage"
    );
}

#[test]
fn geometry_projects_on_each_real_body_and_keeps_deterministic_collapse_groups() {
    use super::geometry;
    use openmars_scene::geometry::{AxialPoint, BodyProjection as Scalar};
    let request = serde_json::from_value(
        json!({"landmark":"factory","details":["smokestacks","tanks","pipes"],"density":"dense"}),
    )
    .unwrap();
    let layout = generate(2907, &request, "Factory").unwrap();
    let a = geometry::generate(layout.layout()).unwrap();
    let b = geometry::generate(layout.layout()).unwrap();
    assert_eq!(a.bounds, b.bounds);
    for (a, b) in a.batches.iter().zip(&b.batches) {
        assert_eq!(a.positions, b.positions);
        assert_eq!(a.collapse, b.collapse);
    }
    assert!(
        a.batches
            .iter()
            .flat_map(|batch| &batch.collapse)
            .any(|m| m[0] > 0.)
    );
    for radius in [2.02, 0.54] {
        let mut mesh = geometry::generate(layout.layout()).unwrap();
        let projection = Scalar::new(radius, radius, 1, AxialPoint { q: 2, r: -1 }).unwrap();
        mesh.project(projection, AxialPoint { q: 2, r: -1 })
            .unwrap();
        for batch in &mesh.batches {
            assert!(batch.positions.iter().all(|p| {
                let r = p.iter().map(|n| f64::from(*n).powi(2)).sum::<f64>().sqrt();
                r >= radius - 0.04 && r < radius + 0.3
            }));
            assert!(
                batch
                    .positions
                    .iter()
                    .flatten()
                    .chain(batch.pivots.iter().flatten())
                    .all(|v| v.is_finite())
            );
        }
    }
}

#[test]
fn repeated_corners_and_pivots_keep_exact_public_spherical_coordinates() {
    use openmars_scene::geometry::{AxialPoint, BoardPoint, BodyProjection};
    let request = serde_json::from_value(
        json!({"plan":"grid","density":"dense","details":["pipes","gatehouses"]}),
    )
    .unwrap();
    let layout = generate(28881, &request, "Projection contract").unwrap();
    let original = super::geometry::generate(layout.layout()).unwrap();
    for (radius, sign, coordinate) in [
        (2.02, 1, AxialPoint { q: 3, r: -2 }),
        (0.54, -1, AxialPoint { q: 1, r: 0 }),
    ] {
        let projection =
            BodyProjection::new(radius, radius, sign, AxialPoint { q: 0, r: 0 }).unwrap();
        let center = projection.hex_center(coordinate);
        let mut projected = super::geometry::generate(layout.layout()).unwrap();
        projected.project(projection, coordinate).unwrap();
        for (before, after) in original.batches.iter().zip(&projected.batches) {
            for (source, actual) in before
                .positions
                .iter()
                .chain(&before.pivots)
                .zip(after.positions.iter().chain(&after.pivots))
            {
                let expected = projection
                    .project_content(
                        BoardPoint {
                            x: center.x + f64::from(source[0]),
                            y: center.y + f64::from(source[1]),
                        },
                        f64::from(source[2]) - 0.007,
                    )
                    .unwrap()
                    .map(|n| n as f32);
                assert_eq!(*actual, expected);
            }
            assert_eq!(before.collapse, after.collapse);
            assert_eq!(before.uv, after.uv);
            assert_eq!(before.colors, after.colors);
        }
    }
}

#[test]
fn authored_styles_keep_their_surfaces_scale_and_bounded_geometry() {
    let cards: serde_json::Value =
        serde_json::from_slice(include_bytes!("../../../backend/assets/cards.json")).unwrap();
    let mut count = 0;
    for card in cards.as_array().unwrap() {
        let Some(request) = card.get("style").and_then(|s| s.get("tile")) else {
            continue;
        };
        let request = serde_json::from_value(request.clone()).unwrap();
        let layout = generate(0x91b751, &request, card["name"].as_str().unwrap()).unwrap();
        let bound = super::geometry::working_bound(layout.layout()).unwrap();
        let geometry = super::geometry::generate(layout.layout()).unwrap();
        assert!(bound >= geometry.bytes() as u64);
        assert!(bound < 256 * 1024 * 1024);
        assert!(geometry.bounds[0][2] < geometry.bounds[1][2]);
        assert!(geometry.emergence_depth > geometry.bounds[1][2]);
        assert!(geometry.batches.iter().any(|b| b.material == 4));
        if layout.layout().style.cover != StructureCover::None {
            assert!(geometry.batches.iter().any(|b| b.material == 10));
        }
        if layout.layout().style.landmark == StructureLandmark::Hall {
            assert!(geometry.bounds[1][2] > 0.17);
        }
        for batch in geometry.batches {
            let n = batch.positions.len();
            assert!(n.is_multiple_of(3));
            assert_eq!(batch.normals.len(), n);
            assert_eq!(batch.uv.len(), n);
            assert_eq!(batch.frost.len(), n);
            assert!(
                batch
                    .positions
                    .iter()
                    .flatten()
                    .chain(batch.normals.iter().flatten())
                    .chain(batch.uv.iter().flatten())
                    .all(|v| v.is_finite())
            );
            assert!(
                batch
                    .normals
                    .iter()
                    .all(|n| (n.iter().map(|v| v * v).sum::<f32>() - 1.).abs() < 1e-4)
            );
            assert!(batch.frost.iter().all(|v| *v == 1. || *v == 0.1));
        }
        count += 1;
    }
    assert!(count >= 12);
}
