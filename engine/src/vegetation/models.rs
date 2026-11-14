//! Named GLTF variants normalized exactly like the original BufferGeometry path.
use bevy::{
    asset::RenderAssetUsages,
    gltf::{Gltf, GltfMaterial, GltfMesh, GltfNode},
    math::{DMat4, DVec3},
    mesh::{Indices, PrimitiveTopology, VertexAttributeValues},
    prelude::*,
};
use std::collections::BTreeSet;
#[derive(Clone, Copy)]
pub struct Request {
    pub source: &'static str,
    pub name: &'static str,
    pub extent: f64,
    pub rock: bool,
}
pub const REQUESTS: [Request; 19] = [
    Request {
        source: "models/trees",
        name: "Tree-01-1",
        extent: 0.08,
        rock: false,
    },
    Request {
        source: "models/trees",
        name: "Tree-01-2",
        extent: 0.08,
        rock: false,
    },
    Request {
        source: "models/trees",
        name: "Tree-01-3",
        extent: 0.08,
        rock: false,
    },
    Request {
        source: "models/trees",
        name: "Tree-01-4",
        extent: 0.08,
        rock: false,
    },
    Request {
        source: "models/trees",
        name: "Bush-01",
        extent: 0.035,
        rock: false,
    },
    Request {
        source: "models/trees",
        name: "Bush-02",
        extent: 0.035,
        rock: false,
    },
    Request {
        source: "models/trees",
        name: "Bush-03",
        extent: 0.035,
        rock: false,
    },
    Request {
        source: "models/trees",
        name: "Bush-04",
        extent: 0.035,
        rock: false,
    },
    Request {
        source: "models/trees",
        name: "Bush-05",
        extent: 0.035,
        rock: false,
    },
    Request {
        source: "models/trees",
        name: "Clover-01",
        extent: 0.012,
        rock: false,
    },
    Request {
        source: "models/trees",
        name: "Clover-02",
        extent: 0.012,
        rock: false,
    },
    Request {
        source: "models/trees",
        name: "Clover-03",
        extent: 0.012,
        rock: false,
    },
    Request {
        source: "models/trees",
        name: "Clover-04",
        extent: 0.012,
        rock: false,
    },
    Request {
        source: "models/trees",
        name: "Clover-05",
        extent: 0.012,
        rock: false,
    },
    Request {
        source: "models/pines",
        name: "Pine4m",
        extent: 0.075,
        rock: false,
    },
    Request {
        source: "models/flowers",
        name: "Circle003",
        extent: 0.025,
        rock: false,
    },
    Request {
        source: "models/flowers",
        name: "Circle008",
        extent: 0.025,
        rock: false,
    },
    Request {
        source: "models/flowers",
        name: "Circle015",
        extent: 0.025,
        rock: false,
    },
    Request {
        source: "models/rock",
        name: "",
        extent: 0.04,
        rock: true,
    },
];
#[derive(Clone)]
pub struct Part {
    pub mesh: Handle<Mesh>,
    pub texture: Option<Handle<Image>>,
    pub double_sided: bool,
}
pub struct Variant {
    pub parts: Vec<Part>,
    pub bounds: [Vec3; 2],
    pub sphere_radius: f64,
    pub vertices: usize,
    pub indices: usize,
}
fn canonical_name(raw: &str) -> String {
    raw.chars()
        .filter(|c| !matches!(c, '[' | ']' | '.' | ':' | '/'))
        .map(|c| if c.is_whitespace() { '_' } else { c })
        .collect()
}
struct Primitive {
    mesh: Handle<Mesh>,
    material: Option<Handle<GltfMaterial>>,
    transform: DMat4,
}
fn hierarchy(world: &World, model: &Gltf, request: Request) -> Result<Vec<Primitive>, String> {
    let nodes = world.resource::<Assets<GltfNode>>();
    if model.nodes.len() > 512 || !model.skins.is_empty() || !model.animations.is_empty() {
        return Err("Static foliage hierarchy/animation bounds".into());
    }
    let mut children = BTreeSet::new();
    for handle in &model.nodes {
        let node = nodes.get(handle).ok_or("Missing GLTF node")?;
        children.extend(node.children.iter().map(|h| h.id()));
    }
    let mut stack: Vec<_> = model
        .nodes
        .iter()
        .filter(|h| !children.contains(&h.id()))
        .rev()
        .map(|h| (h.clone(), DMat4::IDENTITY))
        .collect();
    let mut order = Vec::new();
    let mut seen = BTreeSet::new();
    while let Some((handle, parent)) = stack.pop() {
        if !seen.insert(handle.id()) {
            return Err("Repeated foliage node".into());
        }
        let node = nodes.get(&handle).ok_or("Missing foliage node")?;
        let transform = parent * node.transform.to_matrix().as_dmat4();
        if !transform.is_finite() {
            return Err("Nonfinite model transform".into());
        }
        order.push((handle.clone(), transform));
        for child in node.children.iter().rev() {
            stack.push((child.clone(), transform));
        }
    }
    let selected = if request.rock {
        order
            .iter()
            .rev()
            .find(|(handle, _)| {
                nodes.get(handle).is_some_and(|node| {
                    node.mesh.is_some()
                        && !node.name.to_ascii_lowercase().contains("plane")
                        && !node.name.to_ascii_lowercase().contains("ground")
                })
            })
            .map(|(h, _)| h.clone())
    } else {
        let mut matches = order.iter().filter(|(h, _)| {
            nodes
                .get(h)
                .is_some_and(|n| canonical_name(&n.name) == request.name)
        });
        let selected = matches.next().map(|(h, _)| h.clone());
        if matches.next().is_some() {
            return Err("Ambiguous foliage node name".into());
        }
        selected
    }
    .ok_or_else(|| format!("Missing approved variant {}", request.name))?;
    let mut include = BTreeSet::new();
    let mut pending = vec![selected];
    while let Some(handle) = pending.pop() {
        include.insert(handle.id());
        if !request.rock {
            pending.extend(nodes.get(&handle).unwrap().children.iter().cloned());
        }
    }
    let meshes = world.resource::<Assets<GltfMesh>>();
    let mut out = Vec::new();
    for (handle, transform) in order {
        if include.contains(&handle.id())
            && let Some(mesh) = &nodes.get(&handle).unwrap().mesh
        {
            for primitive in &meshes.get(mesh).ok_or("Missing GLTF primitive")?.primitives {
                out.push(Primitive {
                    mesh: primitive.mesh.clone(),
                    material: primitive.material.clone(),
                    transform,
                });
            }
        }
    }
    if out.is_empty() || out.len() > 64 {
        return Err("Foliage primitive bounds".into());
    }
    Ok(out)
}
struct Built {
    mesh: Mesh,
    texture: Option<Handle<Image>>,
    double_sided: bool,
}
fn source_positions(mesh: &Mesh) -> Result<&[[f32; 3]], String> {
    if mesh.primitive_topology() != PrimitiveTopology::TriangleList {
        return Err("Foliage requires original triangle topology".into());
    }
    match mesh.attribute(Mesh::ATTRIBUTE_POSITION) {
        Some(VertexAttributeValues::Float32x3(p)) if !p.is_empty() => Ok(p),
        _ => Err("Missing foliage positions".into()),
    }
}
fn transformed(point: [f32; 3], matrix: DMat4) -> DVec3 {
    matrix
        .transform_point3(DVec3::from_array(point.map(f64::from)))
        .as_vec3()
        .as_dvec3()
}
fn build(
    world: &World,
    primitives: &[Primitive],
    request: Request,
) -> Result<(Vec<Built>, [Vec3; 2], usize, usize), String> {
    let meshes = world.resource::<Assets<Mesh>>();
    let materials = world.resource::<Assets<GltfMaterial>>();
    let rotate = DMat4::from_rotation_x(std::f64::consts::FRAC_PI_2);
    let (mut low, mut high) = (DVec3::splat(f64::INFINITY), DVec3::splat(f64::NEG_INFINITY));
    let mut vertices = 0;
    let mut index_count = 0;
    for primitive in primitives {
        let mesh = meshes
            .get(&primitive.mesh)
            .ok_or("Missing decoded foliage mesh")?;
        let positions = source_positions(mesh)?;
        vertices += positions.len();
        index_count += mesh.indices().map_or(positions.len(), |i| i.len());
        if vertices > 2_000_000 || index_count > 6_000_000 {
            return Err("Foliage mesh admission exceeded".into());
        }
        for point in positions {
            if !Vec3::from_array(*point).is_finite() {
                return Err("Nonfinite source foliage vertex".into());
            }
            let p = transformed(
                transformed(*point, primitive.transform)
                    .as_vec3()
                    .to_array(),
                rotate,
            );
            low = low.min(p);
            high = high.max(p);
        }
    }
    let extent = (high - low).max_element();
    if !extent.is_finite() || extent <= 0. || !request.extent.is_finite() || request.extent <= 0. {
        return Err("Invalid model normalization extent".into());
    }
    let centre = (low + high) * 0.5;
    let normalize = DMat4::from_scale(DVec3::splat(request.extent / extent))
        * DMat4::from_translation(-DVec3::new(centre.x, centre.y, low.z))
        * rotate;
    let mut out = Vec::new();
    let (mut min, mut max) = (Vec3::splat(f32::INFINITY), Vec3::splat(f32::NEG_INFINITY));
    for primitive in primitives {
        let source = meshes.get(&primitive.mesh).unwrap();
        let positions = source_positions(source)?;
        let material = primitive.material.as_ref().and_then(|h| materials.get(h));
        let color = if request.rock {
            [1.; 4]
        } else {
            let rgb = material
                .map(|m| m.base_color.to_linear().to_f32_array())
                .unwrap_or([1.; 4]);
            [rgb[0], rgb[1], rgb[2], 1.]
        };
        let transform = if request.rock {
            bevy::math::Affine2::IDENTITY
        } else {
            material.map_or(bevy::math::Affine2::IDENTITY, |m| m.uv_transform)
        };
        if !request.rock
            && material.is_some_and(|m| m.base_color_channel != bevy::mesh::UvChannel::Uv0)
        {
            return Err("Approved foliage base texture must use UV0".into());
        }
        let texture = if request.rock {
            None
        } else {
            material.and_then(|m| m.base_color_texture.clone())
        };
        let double_sided = !request.rock && material.is_some_and(|m| m.double_sided);
        let uv = source.attribute(Mesh::ATTRIBUTE_UV_0);
        let mut output = Vec::with_capacity(positions.len());
        let mut uvs = Vec::with_capacity(positions.len());
        for (i, point) in positions.iter().enumerate() {
            let world = transformed(*point, primitive.transform);
            let p = normalize.transform_point3(world).as_vec3();
            if !p.is_finite() {
                return Err("Nonfinite normalized foliage vertex".into());
            }
            min = min.min(p);
            max = max.max(p);
            output.push(p.to_array());
            let input = match uv {
                Some(VertexAttributeValues::Float32x2(uv)) => {
                    *uv.get(i).ok_or("Foliage UV length mismatch")?
                }
                _ => [0.; 2],
            };
            let mut uv = transform.transform_point2(Vec2::from_array(input));
            // The rock overrides its GLTF material with an ordinary THREE
            // texture (flipY=true). Embedded GLTF plant maps use flipY=false.
            if request.rock {
                uv.y = 1. - uv.y;
            }
            if !uv.is_finite() {
                return Err("Nonfinite foliage UV".into());
            }
            uvs.push(uv.to_array());
        }
        let indices: Vec<u32> = match source.indices() {
            Some(indices) => indices.iter().map(|i| i as u32).collect(),
            None => (0..positions.len() as u32).collect(),
        };
        if !indices.len().is_multiple_of(3)
            || indices.iter().any(|i| *i as usize >= positions.len())
        {
            return Err("Foliage indices out of bounds".into());
        }
        let mut mesh = Mesh::new(
            PrimitiveTopology::TriangleList,
            RenderAssetUsages::RENDER_WORLD,
        )
        .with_inserted_attribute(Mesh::ATTRIBUTE_POSITION, output)
        .with_inserted_attribute(Mesh::ATTRIBUTE_UV_0, uvs)
        .with_inserted_attribute(Mesh::ATTRIBUTE_COLOR, vec![color; positions.len()])
        .with_inserted_indices(Indices::U32(indices));
        mesh.compute_area_weighted_normals();
        out.push(Built {
            mesh,
            texture,
            double_sided,
        });
    }
    Ok((out, [min, max], vertices, index_count))
}
pub fn prepare_variant(world: &mut World, request: Request) -> Result<Option<Variant>, String> {
    let Some(handle) = crate::assets::model(world, request.source) else {
        return Ok(None);
    };
    if !world
        .resource::<AssetServer>()
        .is_loaded_with_dependencies(&handle)
    {
        return Ok(None);
    }
    let Some(model) = world.resource::<Assets<Gltf>>().get(&handle) else {
        return Ok(None);
    };
    let primitives = hierarchy(world, model, request)?;
    let (parts, bounds, vertices, indices) = build(world, &primitives, request)?;
    let center = (bounds[0].as_dvec3() + bounds[1].as_dvec3()) * 0.5;
    let sphere_radius = parts
        .iter()
        .flat_map(|p| {
            p.mesh
                .attribute(Mesh::ATTRIBUTE_POSITION)
                .unwrap()
                .as_float3()
                .unwrap()
        })
        .map(|p| (DVec3::from_array(p.map(f64::from)) - center).length_squared())
        .fold(0_f64, f64::max)
        .sqrt();
    let parts = parts
        .into_iter()
        .map(|part| Part {
            mesh: world.resource_mut::<Assets<Mesh>>().add(part.mesh),
            texture: part.texture,
            double_sided: part.double_sided,
        })
        .collect();
    Ok(Some(Variant {
        parts,
        bounds,
        sphere_radius,
        vertices,
        indices,
    }))
}

#[cfg(test)]
mod tests {
    use super::*;
    fn fixture() -> (World, Vec<Primitive>) {
        let mut world = World::new();
        world.init_resource::<Assets<Mesh>>();
        world.init_resource::<Assets<GltfMaterial>>();
        let mesh = Mesh::new(
            PrimitiveTopology::TriangleList,
            RenderAssetUsages::MAIN_WORLD,
        )
        .with_inserted_attribute(
            Mesh::ATTRIBUTE_POSITION,
            vec![[0., 0., 0.], [2., 0., 0.], [0., 1., 0.]],
        )
        .with_inserted_attribute(Mesh::ATTRIBUTE_UV_0, vec![[0., 0.], [1., 0.], [0., 1.]])
        .with_inserted_indices(Indices::U32(vec![0, 1, 2]));
        let mesh = world.resource_mut::<Assets<Mesh>>().add(mesh);
        let material = world
            .resource_mut::<Assets<GltfMaterial>>()
            .add(GltfMaterial {
                base_color: Color::linear_rgba(0.2, 0.4, 0.6, 0.1),
                double_sided: true,
                uv_transform: bevy::math::Affine2::from_scale_angle_translation(
                    Vec2::splat(2.),
                    0.,
                    Vec2::new(0.1, 0.2),
                ),
                ..default()
            });
        (
            world,
            vec![Primitive {
                mesh,
                material: Some(material),
                transform: DMat4::from_scale_rotation_translation(
                    DVec3::new(2., 3., 1.),
                    bevy::math::DQuat::IDENTITY,
                    DVec3::new(7., 13., -11.),
                ),
            }],
        )
    }
    #[test]
    fn preserves_authored_triangles_colors_uvs_and_ground_pivot() {
        let (world, primitives) = fixture();
        let (parts, bounds, vertices, indices) = build(&world, &primitives, REQUESTS[0]).unwrap();
        assert_eq!((vertices, indices), (3, 3));
        assert_eq!(parts.len(), 1);
        let [low, high] = bounds;
        assert!((low.z).abs() < 1e-6);
        assert!((low.x + high.x).abs() < 1e-6);
        assert!((low.y + high.y).abs() < 1e-6);
        assert!(((high - low).max_element() - 0.08).abs() < 1e-6);
        assert!(parts[0].double_sided);
        assert_eq!(
            parts[0].mesh.indices().unwrap().iter().collect::<Vec<_>>(),
            [0, 1, 2]
        );
        match parts[0].mesh.attribute(Mesh::ATTRIBUTE_COLOR).unwrap() {
            VertexAttributeValues::Float32x4(colors) => {
                assert_eq!(colors, &vec![[0.2, 0.4, 0.6, 1.]; 3])
            }
            _ => panic!("wrong colors"),
        }
        match parts[0].mesh.attribute(Mesh::ATTRIBUTE_UV_0).unwrap() {
            VertexAttributeValues::Float32x2(uvs) => {
                assert_eq!(uvs, &[[0.1, 0.2], [2.1, 0.2], [0.1, 2.2]])
            }
            _ => panic!("wrong UVs"),
        }
        match parts[0].mesh.attribute(Mesh::ATTRIBUTE_NORMAL).unwrap() {
            VertexAttributeValues::Float32x3(normals) => assert!(
                normals
                    .iter()
                    .all(|n| (Vec3::from_array(*n) + Vec3::Y).length() < 1e-5)
            ),
            _ => panic!("missing computed normals"),
        }
    }
    #[test]
    fn rejects_invalid_vertices_indices_and_topology_before_publication() {
        let (mut world, primitives) = fixture();
        let h = &primitives[0].mesh;
        world
            .resource_mut::<Assets<Mesh>>()
            .get_mut(h)
            .unwrap()
            .insert_indices(Indices::U32(vec![0, 1, 8]));
        assert!(build(&world, &primitives, REQUESTS[0]).is_err());
        world
            .resource_mut::<Assets<Mesh>>()
            .get_mut(h)
            .unwrap()
            .insert_attribute(Mesh::ATTRIBUTE_POSITION, vec![[f32::NAN, 0., 0.]; 3]);
        assert!(
            build(&world, &primitives, REQUESTS[0])
                .err()
                .unwrap()
                .contains("Nonfinite source")
        );
        let lines = Mesh::new(PrimitiveTopology::LineList, RenderAssetUsages::MAIN_WORLD);
        assert!(source_positions(&lines).is_err());
    }
    #[test]
    fn rock_override_uses_raw_uvs_and_the_original_external_texture_orientation() {
        let (world, primitives) = fixture();
        let (parts, _, _, _) = build(&world, &primitives, REQUESTS[18]).unwrap();
        assert!(!parts[0].double_sided);
        assert!(parts[0].texture.is_none());
        match parts[0].mesh.attribute(Mesh::ATTRIBUTE_UV_0).unwrap() {
            VertexAttributeValues::Float32x2(uvs) => {
                assert_eq!(uvs, &[[0., 1.], [1., 1.], [0., 0.]])
            }
            _ => panic!("wrong rock UVs"),
        }
        match parts[0].mesh.attribute(Mesh::ATTRIBUTE_COLOR).unwrap() {
            VertexAttributeValues::Float32x4(colors) => {
                assert!(colors.iter().all(|v| *v == [1.; 4]))
            }
            _ => panic!("wrong rock override color"),
        }
    }
    #[test]
    fn matches_gltf_loader_node_names() {
        assert_eq!(canonical_name("Circle.003"), "Circle003");
        assert_eq!(canonical_name("Tree [01]:/ One"), "Tree_01_One");
    }
}
