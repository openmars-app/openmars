//! Offline terrain arrays. Order, periodic filtering, straight alpha and linear
//! sRGB mip generation follow the sibling terrain/textures.rs implementation.
use crate::error::PipelineError;
use flate2::{Compression, write::ZlibEncoder};
use serde::Deserialize;
use sha2::{Digest, Sha256};
use std::{
    io::{Cursor, Write},
    path::PathBuf,
};

const MAX_SOURCE: u64 = 64 * 1024 * 1024;
const MAX_DECODED: usize = 128 * 1024 * 1024;
const MAX_OUTPUT: usize = 64 * 1024 * 1024;
fn invalid(message: impl Into<String>) -> PipelineError {
    PipelineError::invalid("texture array", message)
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ArrayRequest {
    pub size: u32,
    pub srgb: bool,
    pub sources: Vec<ArraySource>,
}
#[derive(Clone, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ArraySource {
    pub path: PathBuf,
    pub sha256: String,
}
impl ArrayRequest {
    fn validate(&self) -> Result<usize, PipelineError> {
        if !(16..=1024).contains(&self.size)
            || !self.size.is_power_of_two()
            || self.sources.is_empty()
            || self.sources.len() > 32
        {
            return Err(invalid("invalid size or layer count"));
        }
        let bytes = (0..=self.size.ilog2())
            .map(|m| (self.size >> m).pow(2) as usize * 4 * self.sources.len())
            .sum();
        if bytes > MAX_DECODED {
            return Err(invalid("decoded array exceeds byte bound"));
        }
        Ok(bytes)
    }
}

pub fn prepare(request: &ArrayRequest) -> Result<Vec<u8>, PipelineError> {
    request.validate()?;
    let mut levels: Vec<Vec<u8>> = (0..=request.size.ilog2())
        .map(|m| {
            Vec::with_capacity((request.size >> m).pow(2) as usize * 4 * request.sources.len())
        })
        .collect();
    // Decode one source at a time. The largest source (4096² RGBA8) and two
    // profile-sized staging layers coexist with at most 128 MiB of final mips.
    for source in &request.sources {
        if std::fs::metadata(&source.path)
            .map_err(|e| invalid(e.to_string()))?
            .len()
            > MAX_SOURCE
        {
            return Err(invalid("source exceeds byte bound"));
        }
        let bytes = std::fs::read(&source.path).map_err(|e| invalid(e.to_string()))?;
        if bytes.len() as u64 > MAX_SOURCE
            || format!("{:x}", Sha256::digest(&bytes)) != source.sha256
        {
            return Err(invalid("source changed after catalog hashing"));
        }
        let mut reader = image::ImageReader::new(Cursor::new(bytes))
            .with_guessed_format()
            .map_err(|e| invalid(e.to_string()))?;
        if !matches!(
            reader.format(),
            Some(image::ImageFormat::Png | image::ImageFormat::Jpeg | image::ImageFormat::WebP)
        ) {
            return Err(invalid("array sources must be PNG, JPEG or WebP"));
        }
        let mut limits = image::Limits::default();
        limits.max_image_width = Some(4096);
        limits.max_image_height = Some(4096);
        limits.max_alloc = Some(256 * 1024 * 1024);
        reader.limits(limits);
        let source = reader
            .decode()
            .map_err(|e| invalid(e.to_string()))?
            .into_rgba8();
        append_layer(
            &mut levels,
            source.as_raw(),
            source.width(),
            source.height(),
            request.size,
            request.srgb,
        )?;
    }
    encode(
        &levels,
        request.size,
        request.sources.len() as u32,
        request.srgb,
    )
}

fn append_layer(
    levels: &mut [Vec<u8>],
    source: &[u8],
    width: u32,
    height: u32,
    size: u32,
    srgb: bool,
) -> Result<(), PipelineError> {
    if width == 0 || height == 0 || source.len() != width as usize * height as usize * 4 {
        return Err(invalid("invalid source pixels"));
    }
    let mut previous = vec![0; size as usize * size as usize * 4];
    for y in 0..size {
        for x in 0..size {
            let pixel = sample(
                source,
                width,
                height,
                (f64::from(x) + 0.5) * f64::from(width) / f64::from(size) - 0.5,
                (f64::from(y) + 0.5) * f64::from(height) / f64::from(size) - 0.5,
                srgb,
            );
            previous[((y * size + x) * 4) as usize..][..4].copy_from_slice(&pixel);
        }
    }
    levels[0].extend_from_slice(&previous);
    let mut n = size;
    for level in &mut levels[1..] {
        let side = n / 2;
        let mut next = vec![0; side as usize * side as usize * 4];
        for y in 0..side {
            for x in 0..side {
                let pixel = sample(
                    &previous,
                    n,
                    n,
                    f64::from(x) * 2.0 + 0.5,
                    f64::from(y) * 2.0 + 0.5,
                    srgb,
                );
                next[((y * side + x) * 4) as usize..][..4].copy_from_slice(&pixel);
            }
        }
        level.extend_from_slice(&next);
        previous = next;
        n = side;
    }
    Ok(())
}
fn linear(v: f64) -> f64 {
    if v <= 0.04045 {
        v / 12.92
    } else {
        ((v + 0.055) / 1.055).powf(2.4)
    }
}
fn srgb(v: f64) -> f64 {
    if v <= 0.0031308 {
        v * 12.92
    } else {
        1.055 * v.powf(1.0 / 2.4) - 0.055
    }
}
fn sample(data: &[u8], width: u32, height: u32, x: f64, y: f64, color: bool) -> [u8; 4] {
    let (ix, iy) = (x.floor() as i64, y.floor() as i64);
    let (tx, ty) = (x - x.floor(), y - y.floor());
    std::array::from_fn(|channel| {
        let read = |dx: i64, dy: i64| {
            let x = (ix + dx).rem_euclid(i64::from(width)) as usize;
            let y = (iy + dy).rem_euclid(i64::from(height)) as usize;
            let value = f64::from(data[(y * width as usize + x) * 4 + channel]) / 255.0;
            if color && channel < 3 {
                linear(value)
            } else {
                value
            }
        };
        let value = (read(0, 0) * (1.0 - tx) + read(1, 0) * tx) * (1.0 - ty)
            + (read(0, 1) * (1.0 - tx) + read(1, 1) * tx) * ty;
        let value = if color && channel < 3 {
            srgb(value)
        } else {
            value
        };
        (value.clamp(0.0, 1.0) * 255.0).round() as u8
    })
}

fn encode(
    levels: &[Vec<u8>],
    size: u32,
    layers: u32,
    srgb: bool,
) -> Result<Vec<u8>, PipelineError> {
    let format = if srgb {
        ktx2::Format::R8G8B8A8_SRGB
    } else {
        ktx2::Format::R8G8B8A8_UNORM
    };
    let (basic, _) = ktx2::dfd::Basic::from_format(format).map_err(|e| invalid(e.to_string()))?;
    let dfd = ktx2::dfd::Block::Basic(basic).to_vec();
    let dfdoff = 80 + levels.len() * 24;
    let dfdlen = dfd.len() + 4;
    let mut output = vec![0; dfdoff + dfdlen];
    let mut indices = vec![[0u8; 24]; levels.len()];
    for (mip, level) in levels.iter().enumerate().rev() {
        let n = size >> mip;
        if level.len() != n as usize * n as usize * layers as usize * 4 {
            return Err(invalid("invalid array mip"));
        }
        let mut encoder = ZlibEncoder::new(Vec::new(), Compression::default());
        encoder
            .write_all(level)
            .map_err(|e| invalid(e.to_string()))?;
        let compressed = encoder.finish().map_err(|e| invalid(e.to_string()))?;
        let index = ktx2::LevelIndex {
            byte_offset: output.len() as u64,
            byte_length: compressed.len() as u64,
            uncompressed_byte_length: level.len() as u64,
        };
        indices[mip] = index.as_bytes();
        if output.len() + compressed.len() > MAX_OUTPUT {
            return Err(invalid("compressed array exceeds byte bound"));
        }
        output.extend_from_slice(&compressed);
    }
    let header = ktx2::Header {
        format: Some(format),
        type_size: 1,
        pixel_width: size,
        pixel_height: size,
        pixel_depth: 0,
        layer_count: layers,
        face_count: 1,
        level_count: levels.len() as u32,
        supercompression_scheme: Some(ktx2::SupercompressionScheme::ZLIB),
        index: ktx2::Index {
            dfd_byte_offset: dfdoff as u32,
            dfd_byte_length: dfdlen as u32,
            kvd_byte_offset: 0,
            kvd_byte_length: 0,
            sgd_byte_offset: 0,
            sgd_byte_length: 0,
        },
    };
    output[..80].copy_from_slice(&header.as_bytes());
    output[dfdoff..dfdoff + 4].copy_from_slice(&(dfdlen as u32).to_le_bytes());
    output[dfdoff + 4..dfdoff + dfdlen].copy_from_slice(&dfd);
    for (mip, index) in indices.iter().enumerate() {
        output[80 + mip * 24..104 + mip * 24].copy_from_slice(index);
    }
    ktx2::Reader::new(&output).map_err(|e| invalid(format!("KTX2 validation: {e:?}")))?;
    Ok(output)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Read;
    #[test]
    fn every_mip_preserves_layer_order_and_straight_alpha() {
        let mut levels: Vec<Vec<u8>> = (0..5).map(|_| vec![]).collect();
        for pixel in [[17, 63, 129, 43], [201, 111, 29, 211]] {
            append_layer(&mut levels, &pixel, 1, 1, 16, true).unwrap();
        }
        let bytes = encode(&levels, 16, 2, true).unwrap();
        assert_eq!(bytes, encode(&levels, 16, 2, true).unwrap());
        let reader = ktx2::Reader::new(&bytes).unwrap();
        assert_eq!(reader.header().layer_count, 2);
        assert_eq!(reader.header().format, Some(ktx2::Format::R8G8B8A8_SRGB));
        for (mip, level) in reader.levels().enumerate() {
            let mut data = vec![];
            flate2::read::ZlibDecoder::new(level.data)
                .read_to_end(&mut data)
                .unwrap();
            assert_eq!(data, levels[mip]);
            let stride = (16_u32 >> mip).pow(2) as usize * 4;
            assert!(
                data[..stride]
                    .as_chunks::<4>()
                    .0
                    .iter()
                    .all(|p| *p == [17, 63, 129, 43])
            );
            assert!(
                data[stride..]
                    .as_chunks::<4>()
                    .0
                    .iter()
                    .all(|p| *p == [201, 111, 29, 211])
            );
        }
    }
    #[test]
    fn color_mips_filter_in_linear_light_and_detail_bytes_remain_linear() {
        let pixels = [0, 0, 0, 0, 255, 255, 255, 255];
        assert_eq!(sample(&pixels, 2, 1, 0.5, 0.0, true), [188, 188, 188, 128]);
        assert_eq!(sample(&pixels, 2, 1, 0.5, 0.0, false), [128, 128, 128, 128]);
        assert_eq!(sample(&pixels, 2, 1, -0.5, 0.0, true), [188, 188, 188, 128]);
        let request = |size, layers| ArrayRequest {
            size,
            srgb: false,
            sources: vec![
                ArraySource {
                    path: PathBuf::from("test.png"),
                    sha256: "0".repeat(64)
                };
                layers
            ],
        };
        assert!(request(1024, 11).validate().is_ok());
        assert!(request(1024, 32).validate().is_err());
        assert!(request(4096, 1).validate().is_err());
        assert!(request(16, 0).validate().is_err());
    }
}
