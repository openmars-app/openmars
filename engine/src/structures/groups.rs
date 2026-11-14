//! One bounded RGBA32F animation table with direct dirty-range queue writes.
use bevy::{
    asset::RenderAssetUsages,
    prelude::*,
    render::{render_resource::*, renderer::RenderQueue, texture::GpuImage},
};
use std::collections::BTreeSet;
pub const GROUPS: usize = 16384;
pub const WIDTH: u32 = 1024;
pub const HEIGHT: u32 = (GROUPS as u32 * 6).div_ceil(WIDTH);
pub struct GroupPool {
    pub free: Vec<u32>,
    pub rows: Vec<[f32; 4]>,
    pub dirty: BTreeSet<u32>,
    pub writes: u64,
}
impl Default for GroupPool {
    fn default() -> Self {
        Self::new()
    }
}
impl GroupPool {
    pub fn new() -> Self {
        Self {
            free: (0..GROUPS as u32).rev().collect(),
            rows: vec![[0.; 4]; GROUPS * 6],
            dirty: BTreeSet::new(),
            writes: 0,
        }
    }
    pub fn allocate(&mut self) -> Result<u32, String> {
        self.free.pop().ok_or("Structure group capacity".into())
    }
    pub fn set(&mut self, id: u32, rows: [[f32; 4]; 6]) {
        self.rows[id as usize * 6..id as usize * 6 + 6].copy_from_slice(&rows);
        self.dirty.insert(id);
    }
    pub fn release(&mut self, id: u32) {
        self.set(id, [[0.; 4]; 6]);
        self.free.push(id);
    }
    pub fn publish(&mut self, gpu: &GpuImage, queue: &RenderQueue) -> Result<(), String> {
        if gpu.texture_descriptor.size.width != WIDTH
            || gpu.texture_descriptor.size.height != HEIGHT
        {
            return Err("Structure group texture extent".into());
        }
        for id in &self.dirty {
            let mut at = *id * 6;
            let end = at + 6;
            while at < end {
                let x = at % WIDTH;
                let count = (end - at).min(WIDTH - x);
                let mut bytes = [0u8; 96];
                for (dst, value) in bytes.as_chunks_mut::<4>().0.iter_mut().zip(
                    self.rows[at as usize..(at + count) as usize]
                        .iter()
                        .flatten(),
                ) {
                    dst.copy_from_slice(&value.to_le_bytes());
                }
                let mut target = gpu.texture.as_image_copy();
                target.origin = Origin3d {
                    x,
                    y: at / WIDTH,
                    z: 0,
                };
                queue.write_texture(
                    target,
                    &bytes[..count as usize * 16],
                    TexelCopyBufferLayout {
                        offset: 0,
                        bytes_per_row: Some(count * 16),
                        rows_per_image: Some(1),
                    },
                    Extent3d {
                        width: count,
                        height: 1,
                        depth_or_array_layers: 1,
                    },
                );
                at += count;
                self.writes += 1;
            }
        }
        self.dirty.clear();
        Ok(())
    }
}
pub fn image() -> Image {
    Image::new(
        Extent3d {
            width: WIDTH,
            height: HEIGHT,
            depth_or_array_layers: 1,
        },
        TextureDimension::D2,
        vec![0; WIDTH as usize * HEIGHT as usize * 16],
        TextureFormat::Rgba32Float,
        RenderAssetUsages::RENDER_WORLD,
    )
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn animation_table_is_bounded_and_retirement_clears_complete_groups() {
        let mut pool = GroupPool::new();
        let ids = (0..GROUPS)
            .map(|_| pool.allocate().unwrap())
            .collect::<BTreeSet<_>>();
        assert_eq!(ids.len(), GROUPS);
        assert!(pool.allocate().is_err());
        // Group170 crosses a1024-texel row: its six texels must stay intact.
        pool.set(170, [[7.; 4]; 6]);
        assert_eq!(&pool.rows[1020..1026], &[[7.; 4]; 6]);
        pool.release(170);
        assert_eq!(&pool.rows[1020..1026], &[[0.; 4]; 6]);
        assert_eq!(pool.allocate().unwrap(), 170);
        assert!(pool.dirty.contains(&170));
    }
}
