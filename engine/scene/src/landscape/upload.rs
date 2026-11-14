//! Immutable GPU field payloads. Packing yields between rows; no graphics API,
//! live scene mutation, or partly prepared image escapes this module.
use super::LandscapeSnapshot;
use super::{
    MAX_PATCHES,
    preparation::half,
    publication::{FieldSlot, FieldTimeline},
};
use crate::geometry::{
    BoardPoint,
    terrain::{FIELD_BORDER, FIELD_SIZE, FIELD_STEP, PATCH_SIZE, TerrainError},
};
use std::{num::NonZeroUsize, sync::Arc};

const ROW_BYTES: usize = FIELD_SIZE * 8;
const LAYER_BYTES: usize = ROW_BYTES * FIELD_SIZE;
const MAX_ROWS_PER_STEP: usize = FIELD_SIZE;
/// Two images with at most 256 field-history layers in total.
pub const MAX_UPLOAD_BYTES: usize = LAYER_BYTES * 2 * 2 * MAX_PATCHES;

/// Layers are grouped by patch: one base followed by up to two live targets.
/// Height/shore/coverage/foundation is RGBA16F, 132x132. Surface is RGBA8,
/// 264x132, with material and detail planes adjacent (never downsampled).
pub struct PackedField {
    pub terrain: Vec<u8>,
    pub surface: Vec<u8>,
}
impl PackedField {
    pub fn from_patch(p: &super::LandscapePatch) -> Self {
        let terrain = p.terrain().iter().flat_map(|v| v.to_le_bytes()).collect();
        let mut surface = Vec::with_capacity(LAYER_BYTES);
        for y in 0..FIELD_SIZE {
            let start = y * FIELD_SIZE * 4;
            surface.extend_from_slice(&p.materials()[start..start + FIELD_SIZE * 4]);
            surface.extend_from_slice(&p.detail()[start..start + FIELD_SIZE * 4]);
        }
        Self { terrain, surface }
    }
}
pub struct FieldUpload {
    pub keys: Vec<[i32; 2]>,
    pub slots: Vec<FieldSlot>,
    pub terrain: Vec<u8>,
    pub surface: Vec<u8>,
}
impl FieldUpload {
    pub fn layers(&self) -> u32 {
        self.slots
            .iter()
            .flat_map(|slot| slot.layers)
            .max()
            .map_or(0, |layer| layer + 1)
    }
}

struct UploadLayer {
    key: [i32; 2],
    snapshot: Arc<LandscapeSnapshot>,
    bare: Option<Arc<LandscapeSnapshot>>,
    packed: Option<Arc<PackedField>>,
}
pub struct FieldUploadBuilder {
    layers: Vec<UploadLayer>,
    payload: FieldUpload,
    completed_rows: usize,
}
impl FieldUploadBuilder {
    /// Admission precedes allocation. Empty fields need no GPU batch. On an
    /// initial scene one complete field is stored, with no growth from bare relief.
    pub fn new(
        timeline: FieldTimeline,
        max_array_layers: u32,
        byte_limit: usize,
    ) -> Result<Option<Self>, TerrainError> {
        let keys: Vec<_> = timeline.keys().collect();
        if keys.is_empty() {
            return Ok(None);
        }
        let layers: Vec<_> = timeline
            .layers()
            .map(|(key, snapshot)| (key, snapshot.clone()))
            .collect();
        let slots = timeline.slots();
        let layers: Vec<_> = layers
            .iter()
            .map(|(key, snapshot)| {
                let bare = layers
                    .iter()
                    .find(|(other, candidate)| {
                        other == key
                            && candidate.base_identity() == snapshot.base_identity()
                            && candidate.bare_row(*key, 0).is_some()
                    })
                    .map(|(_, s)| s.clone());
                UploadLayer {
                    key: *key,
                    snapshot: snapshot.clone(),
                    bare,
                    packed: snapshot.patch(*key).map(|p| p.packed()),
                }
            })
            .collect();
        let layer_count = layers.len();
        let image_bytes = LAYER_BYTES * layer_count;
        if keys.len() > MAX_PATCHES
            || layer_count > max_array_layers as usize
            || byte_limit > MAX_UPLOAD_BYTES
            || image_bytes * 2 > byte_limit
        {
            return Err(TerrainError::ResourceLimit);
        }
        Ok(Some(Self {
            layers,
            payload: FieldUpload {
                keys,
                slots,
                terrain: vec![0; image_bytes],
                surface: vec![0; image_bytes],
            },
            completed_rows: 0,
        }))
    }
    pub fn progress(&self) -> (usize, usize) {
        (self.completed_rows, self.layers.len() * FIELD_SIZE)
    }
    /// Returns actual completed rows. Clamps caller budgets to bound fallback
    /// relief sampling and copying even when a caller supplies usize::MAX.
    pub fn step(&mut self, budget: NonZeroUsize) -> Result<usize, TerrainError> {
        let (_, total) = self.progress();
        let count = budget
            .get()
            .min(MAX_ROWS_PER_STEP)
            .min(total - self.completed_rows);
        for row in self.completed_rows..self.completed_rows + count {
            let layer = row / FIELD_SIZE;
            let y = row % FIELD_SIZE;
            let layer = &self.layers[layer];
            let key = layer.key;
            let snapshot = &layer.snapshot;
            let target = row * ROW_BYTES;
            let height = &mut self.payload.terrain[target..target + ROW_BYTES];
            let surface = &mut self.payload.surface[target..target + ROW_BYTES];
            if let Some(packed) = &layer.packed {
                height.copy_from_slice(
                    &packed.terrain[target % LAYER_BYTES..target % LAYER_BYTES + ROW_BYTES],
                );
                surface.copy_from_slice(
                    &packed.surface[target % LAYER_BYTES..target % LAYER_BYTES + ROW_BYTES],
                );
            } else {
                let mut cached = layer.bare.as_ref().and_then(|s| s.bare_row(key, y));
                for x in 0..FIELD_SIZE {
                    let h = if let Some(value) = cached.as_mut().and_then(Iterator::next) {
                        value
                    } else {
                        snapshot.bare_height(BoardPoint {
                            x: f64::from(key[0]) * PATCH_SIZE
                                + (x as f64 - FIELD_BORDER as f64) * FIELD_STEP,
                            y: f64::from(key[1]) * PATCH_SIZE
                                + (y as f64 - FIELD_BORDER as f64) * FIELD_STEP,
                        })?
                    };
                    height[x * 8..x * 8 + 2].copy_from_slice(&half(h).to_le_bytes());
                    height[x * 8 + 2..x * 8 + 4].copy_from_slice(&half(0.2).to_le_bytes());
                    surface[FIELD_SIZE * 4 + x * 4 + 3] =
                        ((-(h - 0.0003) / 0.036).clamp(0.0, 1.0) * 255.0).round() as u8;
                }
            }
        }
        self.completed_rows += count;
        Ok(count)
    }
    /// Consuming an unfinished builder cancels it and releases its snapshots
    /// and staging buffers. There is no partially valid texture return value.
    pub fn finish(self) -> Result<FieldUpload, TerrainError> {
        let (completed, total) = self.progress();
        if completed != total {
            return Err(TerrainError::OutsideDomain);
        }
        Ok(self.payload)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{geometry::BodyProjection, landscape::*};
    #[test]
    fn cached_bare_layers_match_scalar_relief_and_material_bytes() {
        let mut depths = vec![0.012f32; 32];
        for y in 1..3 {
            for x in 0..8 {
                depths[y * 8 + x] = 0.012 + (x + y) as f32 * 0.001;
            }
        }
        let relief = Arc::new(crate::geometry::terrain::ReliefMap::new(8, 4, depths).unwrap());
        let make = |revision, sources| {
            let mut builder = LandscapeBuilder::new();
            builder
                .begin(
                    LandscapeInput::new(
                        BodyId::Mars,
                        BodyProjection::MARS,
                        revision,
                        73,
                        Some(relief.clone()),
                        sources,
                    )
                    .unwrap(),
                )
                .unwrap();
            while matches!(builder.state(), PreparationState::Preparing { .. }) {
                builder.step(NonZeroUsize::new(1024).unwrap()).unwrap();
            }
            builder.take_ready().unwrap()
        };
        let bare = make(1, vec![]);
        let next = make(2, vec![ocean()]);
        let base = FieldTimeline::prepare(None, bare.clone(), 0).unwrap();
        let timeline = FieldTimeline::prepare(Some(&base), next, 0).unwrap();
        let job = FieldUploadBuilder::new(timeline, 256, MAX_UPLOAD_BYTES)
            .unwrap()
            .unwrap();
        assert!(job.layers.iter().all(|layer| layer.bare.is_some()));
        let upload = complete(job, 32);
        for (slot, key) in upload.keys.iter().enumerate() {
            let layer = upload.slots[slot].layers[0] as usize;
            for y in 0..FIELD_SIZE {
                for x in 0..FIELD_SIZE {
                    let h = bare
                        .bare_height(BoardPoint {
                            x: f64::from(key[0]) * PATCH_SIZE
                                + (x as f64 - FIELD_BORDER as f64) * FIELD_STEP,
                            y: f64::from(key[1]) * PATCH_SIZE
                                + (y as f64 - FIELD_BORDER as f64) * FIELD_STEP,
                        })
                        .unwrap();
                    let offset = layer * LAYER_BYTES + y * ROW_BYTES + x * 8;
                    assert_eq!(&upload.terrain[offset..offset + 2], &half(h).to_le_bytes());
                    let alpha = layer * LAYER_BYTES + y * ROW_BYTES + FIELD_SIZE * 4 + x * 4 + 3;
                    assert_eq!(
                        upload.surface[alpha],
                        ((-(h - 0.0003) / 0.036).clamp(0., 1.) * 255.).round() as u8
                    );
                }
            }
        }
    }

    fn upload_job(
        previous: Option<Arc<LandscapeSnapshot>>,
        next: Arc<LandscapeSnapshot>,
        layers: u32,
        bytes: usize,
    ) -> Result<Option<FieldUploadBuilder>, TerrainError> {
        let previous = previous.map(|snapshot| FieldTimeline::prepare(None, snapshot, 0).unwrap());
        FieldUploadBuilder::new(
            FieldTimeline::prepare(previous.as_ref(), next, 0)?,
            layers,
            bytes,
        )
    }
    fn snapshot(revision: u64, sources: Vec<LandscapeSource>) -> Arc<LandscapeSnapshot> {
        let mut builder = LandscapeBuilder::new();
        builder
            .begin(
                LandscapeInput::new(
                    BodyId::Mars,
                    BodyProjection::MARS,
                    revision,
                    73,
                    None,
                    sources,
                )
                .unwrap(),
            )
            .unwrap();
        while matches!(builder.state(), PreparationState::Preparing { .. }) {
            builder.step(NonZeroUsize::new(1024).unwrap()).unwrap();
        }
        builder.take_ready().unwrap()
    }
    fn ocean() -> LandscapeSource {
        LandscapeSource {
            coordinate: [0, 0, 0],
            kind: LandscapeKind::Ocean,
            seed: 3,
            layout: None,
        }
    }
    fn complete(mut job: FieldUploadBuilder, budget: usize) -> FieldUpload {
        while job.progress().0 < job.progress().1 {
            let before = job.progress().0;
            let rows = job.step(NonZeroUsize::new(budget).unwrap()).unwrap();
            assert!(rows <= budget.min(MAX_ROWS_PER_STEP));
            assert_eq!(job.progress().0, before + rows);
        }
        assert_eq!(job.step(NonZeroUsize::new(1).unwrap()).unwrap(), 0);
        job.finish().unwrap()
    }
    #[test]
    fn upload_preserves_half_bits_full_planes_layer_order_and_bounded_work() {
        let old = snapshot(1, vec![ocean()]);
        let next = snapshot(2, vec![]);
        let upload = complete(
            upload_job(Some(old.clone()), next, 256, MAX_UPLOAD_BYTES)
                .unwrap()
                .unwrap(),
            7,
        );
        assert_eq!(upload.layers(), (old.patches().count() * 2) as u32);
        for (slot, key) in upload.keys.iter().enumerate() {
            let patch = old.patch(*key).unwrap();
            for y in 0..FIELD_SIZE {
                for x in 0..FIELD_SIZE {
                    let source = (y * FIELD_SIZE + x) * 4;
                    let target =
                        upload.slots[slot].layers[0] as usize * LAYER_BYTES + y * ROW_BYTES + x * 8;
                    for c in 0..4 {
                        assert_eq!(
                            &upload.terrain[target + c * 2..target + c * 2 + 2],
                            &patch.terrain()[source + c].to_le_bytes()
                        );
                    }
                    let surface =
                        upload.slots[slot].layers[0] as usize * LAYER_BYTES + y * ROW_BYTES + x * 4;
                    assert_eq!(
                        &upload.surface[surface..surface + 4],
                        &patch.materials()[source..source + 4]
                    );
                    assert_eq!(
                        &upload.surface[surface + FIELD_SIZE * 4..surface + FIELD_SIZE * 4 + 4],
                        &patch.detail()[source..source + 4]
                    );
                    let next_target =
                        upload.slots[slot].layers[1] as usize * LAYER_BYTES + y * ROW_BYTES + x * 8;
                    let bare = [0xea, 0x0c, 0x66, 0x32, 0, 0, 0, 0]; // half(0.0003), half(0.2), zero coverage/foundation
                    assert_eq!(&upload.terrain[next_target..next_target + 8], &bare);
                }
            }
        }
    }
    #[test]
    fn initial_upload_has_one_complete_side_and_is_independent_of_poll_budget() {
        let next = snapshot(1, vec![ocean()]);
        let make = || {
            upload_job(None, next.clone(), 256, MAX_UPLOAD_BYTES)
                .unwrap()
                .unwrap()
        };
        let a = complete(make(), 1);
        let b = complete(make(), usize::MAX);
        assert_eq!(a.keys, b.keys);
        assert_eq!(a.terrain, b.terrain);
        assert_eq!(a.surface, b.surface);
        assert_eq!(a.layers(), a.keys.len() as u32);
        assert!(
            a.slots
                .iter()
                .all(|slot| slot.layers[0] == slot.layers[1] && slot.layers[1] == slot.layers[2])
        );
    }
    #[test]
    fn upload_admission_and_cancellation_do_not_publish_partial_images() {
        let next = snapshot(1, vec![ocean()]);
        assert!(matches!(
            upload_job(None, next.clone(), 1, MAX_UPLOAD_BYTES),
            Err(TerrainError::ResourceLimit)
        ));
        assert!(matches!(
            upload_job(None, next.clone(), 256, 1),
            Err(TerrainError::ResourceLimit)
        ));
        assert!(matches!(
            upload_job(None, next.clone(), 256, MAX_UPLOAD_BYTES + 1),
            Err(TerrainError::ResourceLimit)
        ));
        let references = Arc::strong_count(&next);
        let mut job = upload_job(None, next.clone(), 256, MAX_UPLOAD_BYTES)
            .unwrap()
            .unwrap();
        job.step(NonZeroUsize::new(1).unwrap()).unwrap();
        assert!(job.finish().is_err());
        assert_eq!(Arc::strong_count(&next), references);
        assert!(
            upload_job(None, snapshot(2, vec![]), 1, 0)
                .unwrap()
                .is_none()
        );
    }
}
