//! Native assets for an admitted immutable field upload. Ownership/publication
//! belongs to the surface batch; creating these assets never changes contact.
use bevy::{
    asset::RenderAssetUsages,
    image::{ImageFilterMode, ImageSampler, ImageSamplerDescriptor},
    mesh::{Indices, PrimitiveTopology},
    prelude::*,
    render::render_resource::{
        Extent3d, TextureDimension, TextureFormat, TextureViewDescriptor, TextureViewDimension,
    },
};
use openmars_scene::{
    geometry::{
        BoardPoint, BodyProjection,
        terrain::{FIELD_SIZE, PATCH_SIZE},
    },
    landscape::{MAX_PATCHES, publication::FieldSlot, upload::FieldUpload},
};

/// No lossy resampling or format conversion occurs at runtime. Taking ownership
/// moves the two prepared byte vectors into Bevy rather than cloning them.
pub fn field_images(upload: FieldUpload) -> Result<[Image; 2], String> {
    let layers = upload.layers();
    let bytes = FIELD_SIZE * FIELD_SIZE * 8 * layers as usize;
    if layers == 0
        || upload.slots.len() != upload.keys.len()
        || upload
            .slots
            .iter()
            .any(|slot| slot.packed_layers().is_none())
        || layers > (MAX_PATCHES * 2) as u32
        || upload.terrain.len() != bytes
        || upload.surface.len() != bytes
    {
        return Err("invalid admitted terrain field upload".into());
    }
    let mut result = Vec::with_capacity(2);
    for (width, format, mut data) in [
        (FIELD_SIZE, TextureFormat::Rgba16Float, upload.terrain),
        (FIELD_SIZE * 2, TextureFormat::Rgba8Unorm, upload.surface),
    ] {
        // wgpu 30's GLES backend infers cube arrays from square extents with
        // a multiple of six layers, even for explicit D2Array views. WebGL2
        // has no cube arrays. One unused storage layer avoids that heuristic
        // while preserving every original sample and the exact sampled view.
        let storage_layers =
            (layers + u32::from(width == FIELD_SIZE && layers.is_multiple_of(6))).max(2);
        let bytes_per_layer = data.len() / layers as usize;
        data.resize(bytes_per_layer * storage_layers as usize, 0);
        let mut image = Image::new(
            Extent3d {
                width: width as u32,
                height: FIELD_SIZE as u32,
                depth_or_array_layers: storage_layers,
            },
            TextureDimension::D2,
            data,
            format,
            RenderAssetUsages::RENDER_WORLD,
        );
        image.texture_view_descriptor = Some(TextureViewDescriptor {
            dimension: Some(TextureViewDimension::D2Array),
            array_layer_count: Some(layers),
            ..default()
        });
        image.sampler = ImageSampler::Descriptor(ImageSamplerDescriptor {
            mag_filter: ImageFilterMode::Linear,
            min_filter: ImageFilterMode::Linear,
            ..default()
        });
        result.push(image);
    }
    result
        .try_into()
        .map_err(|_| "terrain field image count".into())
}

/// The field shader displaces these canonical 32x32 quads. Slot/origin data
/// stays full float in COLOR; no per-tile or per-vertex material allocation.
pub fn patch_mesh(
    keys: &[[i32; 2]],
    slots: &[FieldSlot],
    projection: BodyProjection,
) -> Result<Mesh, String> {
    if keys.is_empty()
        || keys.len() != slots.len()
        || slots.iter().any(|v| v.packed_layers().is_none())
        || keys.len() > MAX_PATCHES
        || keys.windows(2).any(|pair| pair[0] >= pair[1])
    {
        return Err("terrain patch mesh requires bounded sorted unique keys".into());
    }
    let mut positions = Vec::with_capacity(keys.len() * 33 * 33);
    let mut normals = Vec::with_capacity(positions.capacity());
    let mut uvs = Vec::with_capacity(positions.capacity());
    let mut colors = Vec::with_capacity(positions.capacity());
    let mut clocks = Vec::with_capacity(positions.capacity());
    let mut indices = Vec::with_capacity(keys.len() * 32 * 32 * 6);
    for (slot, key) in keys.iter().enumerate() {
        let start = positions.len() as u32;
        let origin = [
            f64::from(key[0]) * PATCH_SIZE,
            f64::from(key[1]) * PATCH_SIZE,
        ];
        for y in 0..=32 {
            for x in 0..=32 {
                let uv = [x as f64 / 32., y as f64 / 32.];
                let point = BoardPoint {
                    x: origin[0] + uv[0] * PATCH_SIZE,
                    y: origin[1] + uv[1] * PATCH_SIZE,
                };
                positions.push(
                    projection
                        .project(point, 0.)
                        .map_err(|e| e.to_string())?
                        .map(|v| v as f32),
                );
                normals.push(
                    projection
                        .normal(point)
                        .map_err(|e| e.to_string())?
                        .map(|v| v as f32),
                );
                uvs.push(uv.map(|v| v as f32));
                clocks.push([slots[slot].births[1], 0.]);
                colors.push([
                    origin[0] as f32,
                    origin[1] as f32,
                    slots[slot].packed_layers().unwrap(),
                    slots[slot].births[0],
                ]);
            }
        }
        for y in 0..32 {
            for x in 0..32 {
                let a = start + y * 33 + x;
                let b = a + 1;
                let c = a + 33;
                let d = c + 1;
                indices.extend_from_slice(&[a, b, d, a, d, c]);
            }
        }
    }
    Ok(Mesh::new(
        PrimitiveTopology::TriangleList,
        RenderAssetUsages::RENDER_WORLD,
    )
    .with_inserted_attribute(Mesh::ATTRIBUTE_POSITION, positions)
    .with_inserted_attribute(Mesh::ATTRIBUTE_NORMAL, normals)
    .with_inserted_attribute(Mesh::ATTRIBUTE_UV_0, uvs)
    .with_inserted_attribute(Mesh::ATTRIBUTE_UV_1, clocks)
    .with_inserted_attribute(Mesh::ATTRIBUTE_COLOR, colors)
    .with_inserted_indices(Indices::U32(indices)))
}

#[cfg(test)]
mod tests {
    use super::*;
    use bevy::mesh::VertexAttributeValues;
    #[test]
    fn field_images_move_exact_bytes_into_linear_array_views() {
        let count = FIELD_SIZE * FIELD_SIZE * 8 * 2;
        let height: Vec<_> = (0..count).map(|n| (n % 251) as u8).collect();
        let surface: Vec<_> = (0..count).map(|n| (n % 239) as u8).collect();
        let pointers = [height.as_ptr(), surface.as_ptr()];
        let [height, surface] = field_images(FieldUpload {
            keys: vec![[0, 0]],
            slots: vec![FieldSlot {
                layers: [0, 1, 1],
                births: [-1., -3.],
            }],
            terrain: height,
            surface,
        })
        .unwrap();
        for (i, image) in [&height, &surface].into_iter().enumerate() {
            assert_eq!(
                image.data.as_ref().unwrap().as_ptr(),
                pointers[i],
                "no staging clone"
            );
            assert_eq!(image.texture_descriptor.size.depth_or_array_layers, 2);
            assert_eq!(image.texture_descriptor.size.height, FIELD_SIZE as u32);
            assert_eq!(
                image.texture_descriptor.size.width,
                (FIELD_SIZE * (i + 1)) as u32
            );
            assert_eq!(image.texture_descriptor.mip_level_count, 1);
            assert_eq!(
                image.texture_view_descriptor.as_ref().unwrap().dimension,
                Some(TextureViewDimension::D2Array)
            );
            assert_eq!(image.asset_usage, RenderAssetUsages::RENDER_WORLD);
        }
        assert_eq!(height.texture_descriptor.format, TextureFormat::Rgba16Float);
        assert_eq!(surface.texture_descriptor.format, TextureFormat::Rgba8Unorm);
        assert!(
            field_images(FieldUpload {
                keys: vec![[0, 0]],
                slots: vec![FieldSlot {
                    layers: [0, 1, 1],
                    births: [-1., -3.]
                }],
                terrain: vec![],
                surface: vec![]
            })
            .is_err()
        );
    }
    #[test]
    fn square_field_arrays_avoid_cube_array_inference_without_changing_samples() {
        for patches in [1usize, 3, 4, 15] {
            let layers = patches * 2;
            let bytes = FIELD_SIZE * FIELD_SIZE * 8 * layers;
            let original: Vec<u8> = (0..bytes).map(|i| (i % 251) as u8).collect();
            let [terrain, surface] = field_images(FieldUpload {
                keys: (0..patches).map(|i| [i as i32, 0]).collect(),
                slots: (0..patches)
                    .map(|i| FieldSlot {
                        layers: [i as u32 * 2, i as u32 * 2 + 1, i as u32 * 2 + 1],
                        births: [-1., -3.],
                    })
                    .collect(),
                terrain: original.clone(),
                surface: original.clone(),
            })
            .unwrap();
            let padded = layers + usize::from(layers.is_multiple_of(6));
            assert_eq!(
                terrain.texture_descriptor.size.depth_or_array_layers,
                padded as u32
            );
            assert!(
                !terrain
                    .texture_descriptor
                    .size
                    .depth_or_array_layers
                    .is_multiple_of(6)
            );
            assert_eq!(
                surface.texture_descriptor.size.depth_or_array_layers,
                layers as u32
            );
            for image in [&terrain, &surface] {
                let data = image.data.as_ref().unwrap();
                assert_eq!(&data[..bytes], &original);
                assert!(data[bytes..].iter().all(|v| *v == 0));
                assert_eq!(
                    image
                        .texture_view_descriptor
                        .as_ref()
                        .unwrap()
                        .array_layer_count,
                    Some(layers as u32)
                );
            }
        }
    }
    #[test]
    fn patch_mesh_keeps_projection_outward_winding_and_exact_shader_slots() {
        let mesh = patch_mesh(
            &[[-1, 0], [0, 0]],
            &[FieldSlot::settled(0), FieldSlot::settled(1)],
            BodyProjection::MARS,
        )
        .unwrap();
        let Some(VertexAttributeValues::Float32x3(p)) = mesh.attribute(Mesh::ATTRIBUTE_POSITION)
        else {
            panic!()
        };
        let Some(VertexAttributeValues::Float32x3(n)) = mesh.attribute(Mesh::ATTRIBUTE_NORMAL)
        else {
            panic!()
        };
        let Some(VertexAttributeValues::Float32x4(c)) = mesh.attribute(Mesh::ATTRIBUTE_COLOR)
        else {
            panic!()
        };
        let Some(VertexAttributeValues::Float32x2(uv)) = mesh.attribute(Mesh::ATTRIBUTE_UV_0)
        else {
            panic!()
        };
        assert_eq!(p.len(), 2 * 33 * 33);
        assert_eq!(c[0], [-PATCH_SIZE as f32, 0., 0., -2.]);
        assert_eq!(c[33 * 33], [0., 0., 65793., -2.]);
        assert_eq!(uv[0], [0., 0.]);
        assert_eq!(uv[33 * 33 - 1], [1., 1.]);
        assert_eq!(p[32], p[33 * 33]);
        for (position, normal) in p.iter().zip(n) {
            assert!((Vec3::from_array(*position).length() - 2.02).abs() < 1e-5);
            assert!((Vec3::from_array(*normal).length() - 1.).abs() < 1e-5);
        }
        let Some(Indices::U32(indices)) = mesh.indices() else {
            panic!()
        };
        assert_eq!(indices.len(), 2 * 32 * 32 * 6);
        for triangle in indices.as_chunks::<3>().0 {
            let [a, b, c] = triangle.map(|index| Vec3::from_array(p[index as usize]));
            assert!((b - a).cross(c - a).dot(a) > 0.);
        }
        for keys in [&[][..], &[[0, 0], [0, 0]], &[[0, 0], [-1, 0]]] {
            assert!(
                patch_mesh(
                    keys,
                    &vec![FieldSlot::settled(0); keys.len()],
                    BodyProjection::MARS
                )
                .is_err()
            );
        }
    }
}
