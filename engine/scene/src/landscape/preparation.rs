pub mod transfer;
use super::*;
use crate::geometry::{
    AxialPoint, BoardPoint,
    terrain::{
        FIELD_BORDER, FIELD_SAMPLES, FIELD_SIZE, FIELD_STEP, PATCH_SIZE, SurfaceClass,
        TerrainError, TerrainField, TerrainPatch, TerrainSample,
    },
};
use sha2::{Digest, Sha256};
use std::{
    collections::{BTreeMap, BTreeSet},
    io::Write,
    num::NonZeroUsize,
    sync::{
        Arc,
        atomic::{AtomicU64, Ordering},
    },
};

const POOL_BYTES: u64 = 256 * 1024 * 1024;
const MAX_POLL: usize = 1024;
const CELLS: usize = FIELD_SIZE * FIELD_SIZE;
const PATCH_BASE_BYTES: u64 = (CELLS * 40 + 512) as u64;
// One admitted scatter output plus its shrink-to-fit replacement can overlap.
// Published patches retain only their actual vector capacity in the same pool.
const PATCH_BYTES: u64 = PATCH_BASE_BYTES
    + (2 * vegetation::MAX_PLANTS_PER_PATCH * std::mem::size_of::<vegetation::LandscapePlant>()
        + std::mem::size_of::<vegetation::PlantScatter>()
        + 4096) as u64;
struct Pool {
    used: AtomicU64,
    limit: u64,
}
struct Allocation {
    pool: Arc<Pool>,
    bytes: u64,
}
impl Allocation {
    fn resize(&mut self, bytes: u64) -> Result<(), TerrainError> {
        self.pool
            .used
            .try_update(Ordering::Relaxed, Ordering::Relaxed, |used| {
                used.checked_sub(self.bytes)
                    .and_then(|used| used.checked_add(bytes))
                    .filter(|used| *used <= self.pool.limit)
            })
            .map_err(|_| TerrainError::ResourceLimit)?;
        self.bytes = bytes;
        Ok(())
    }
}
impl Drop for Allocation {
    fn drop(&mut self) {
        let previous = self.pool.used.fetch_sub(self.bytes, Ordering::Relaxed);
        assert!(previous >= self.bytes, "owned landscape allocation");
    }
}
impl Pool {
    fn allocate(self: &Arc<Self>, bytes: u64) -> Result<Allocation, TerrainError> {
        self.used
            .try_update(Ordering::Relaxed, Ordering::Relaxed, |used| {
                used.checked_add(bytes).filter(|used| *used <= self.limit)
            })
            .map_err(|_| TerrainError::ResourceLimit)?;
        Ok(Allocation {
            pool: self.clone(),
            bytes,
        })
    }
}
/// The lease is retained by every cached/current/previous patch and snapshot.
/// Evicting a cache entry cannot free a buffer still used by a transition.
pub struct LandscapePatch {
    key: [i32; 2],
    signature: [u8; 32],
    terrain: Vec<u16>,
    materials: Vec<u8>,
    detail: Vec<u8>,
    water: bool,
    plants: Vec<vegetation::LandscapePlant>,
    bare: Option<Arc<[f64]>>,
    contact: std::sync::OnceLock<TerrainPatch>,
    packed: std::sync::OnceLock<Arc<super::upload::PackedField>>,
    _allocation: Allocation,
}
impl LandscapePatch {
    pub fn key(&self) -> [i32; 2] {
        self.key
    }
    pub fn signature(&self) -> [u8; 32] {
        self.signature
    }
    pub fn terrain(&self) -> &[u16] {
        &self.terrain
    }
    pub fn materials(&self) -> &[u8] {
        &self.materials
    }
    pub fn detail(&self) -> &[u8] {
        &self.detail
    }
    pub fn water(&self) -> bool {
        self.water
    }
    fn contact(&self) -> Result<TerrainPatch, TerrainError> {
        if let Some(contact) = self.contact.get() {
            return Ok(contact.clone());
        }
        let contact = TerrainPatch::from_half_rgba(self.key, &self.terrain, SurfaceClass::Ground)?;
        let _ = self.contact.set(contact.clone());
        Ok(contact)
    }
    pub(super) fn packed(&self) -> Arc<super::upload::PackedField> {
        self.packed
            .get_or_init(|| Arc::new(super::upload::PackedField::from_patch(self)))
            .clone()
    }
    pub fn plants(&self) -> &[vegetation::LandscapePlant] {
        &self.plants
    }
}
struct SampleView {
    body: BodyId,
    field: TerrainField,
}
impl TerrainSampler for SampleView {
    fn revision(&self) -> TerrainRevision {
        TerrainRevision(self.field.revision())
    }
    fn body(&self) -> BodyId {
        self.body
    }
    fn sample(&self, p: BoardPoint) -> Result<TerrainSample, TerrainError> {
        self.field.sample(p)
    }
    fn height(&self, p: BoardPoint) -> Result<f32, TerrainError> {
        self.field.height(p)
    }
    fn bounds(&self) -> Option<[BoardPoint; 2]> {
        self.field.bounds()
    }
}
pub struct LandscapeSnapshot {
    sources: Vec<LandscapeSource>,
    _sources_allocation: Allocation,
    source_identity: [u8; 32],
    base_identity: [u8; 32],
    basis: BTreeMap<[i32; 2], Arc<BasisGrid>>,
    patches: BTreeMap<[i32; 2], Arc<LandscapePatch>>,
    sampler: SampleView,
    _allocation: Allocation,
}
impl LandscapeSnapshot {
    /// The exact immutable layouts used by the published terrain and scatter.
    pub fn sources(&self) -> &[LandscapeSource] {
        &self.sources
    }
    pub fn source_identity(&self) -> [u8; 32] {
        self.source_identity
    }
    pub fn patches(&self) -> impl Iterator<Item = &LandscapePatch> {
        self.patches.values().map(Arc::as_ref)
    }
    pub fn patch(&self, key: [i32; 2]) -> Option<&LandscapePatch> {
        self.patches.get(&key).map(Arc::as_ref)
    }
    // Borrowed only: a detached scalar field cannot outlive its memory charge.
    pub fn sampler(&self) -> &dyn TerrainSampler {
        &self.sampler
    }
    pub fn revision(&self) -> u64 {
        self.sampler.field.revision()
    }
    pub fn projection(&self) -> crate::geometry::BodyProjection {
        self.sampler.field.projection()
    }
    pub fn base_identity(&self) -> [u8; 32] {
        self.base_identity
    }
    pub fn height_shore(&self, p: BoardPoint) -> Result<[f32; 2], TerrainError> {
        self.sampler.field.height_shore(p)
    }
    /// Prepared canonical lattice values; no projection or relief lookup occurs
    /// while packing an old bare side of a live update.
    pub fn bare_row(
        &self,
        key: [i32; 2],
        row: usize,
    ) -> Option<impl ExactSizeIterator<Item = f64> + '_> {
        let start = row.checked_mul(FIELD_SIZE)?;
        let end = start.checked_add(FIELD_SIZE)?;
        let grid = self.basis.get(&key).filter(|g| end <= g.samples.len());
        let bare = self
            .patches
            .get(&key)
            .and_then(|p| p.bare.as_ref())
            .filter(|g| end <= g.len());
        if grid.is_none_or(|g| end > g.samples.len()) && bare.is_none_or(|g| end > g.len()) {
            return None;
        }
        Some(
            (start..end).map(move |i| {
                grid.map_or_else(|| bare.unwrap()[i], |g| g.samples[i].relief + 0.0003)
            }),
        )
    }
    pub fn bare_height(&self, p: BoardPoint) -> Result<f64, TerrainError> {
        self.sampler.field.fallback_height(p)
    }
}
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum PreparationState {
    Idle,
    Preparing { completed: usize, total: usize },
    Ready { patches: usize },
    Failed(TerrainError),
    Cancelled,
}
struct BasisGrid {
    samples: Vec<fields::SampleBasis>,
    _allocation: Allocation,
}
struct ActivePatch {
    basis: Option<Arc<BasisGrid>>,
    basis_work: Option<BasisGrid>,
    scatter: Option<vegetation::PlantScatter>,
    sampler: Arc<LandscapeSampler>,
    visible: bool,
    cells: usize,
    patch: LandscapePatch,
}
struct Request {
    input: LandscapeInput,
    keys: Vec<[i32; 2]>,
    sampler: Arc<LandscapeSampler>,
    context: [u8; 32],
    identity: [u8; 32],
    source_hashes: Vec<[u8; 32]>,
    next: usize,
    done: usize,
    patches: BTreeMap<[i32; 2], Arc<LandscapePatch>>,
    current: Option<ActivePatch>,
    _allocation: Allocation,
}
#[derive(Clone, Copy, Debug, Default)]
pub struct PreparationWork {
    pub field_samples: usize,
    pub noise_samples: usize,
    pub scatter_checks: usize,
    pub reused_patches: usize,
}
pub struct LandscapeBuilder {
    work: PreparationWork,
    basis: BTreeMap<[i32; 2], Arc<BasisGrid>>,
    basis_context: Option<[u8; 32]>,
    pool: Arc<Pool>,
    cache: BTreeMap<[i32; 2], Arc<LandscapePatch>>,
    empty: BTreeMap<[i32; 2], [u8; 32]>,
    pending: Option<Request>,
    ready: Option<Arc<LandscapeSnapshot>>,
    state: PreparationState,
}
impl Default for LandscapeBuilder {
    fn default() -> Self {
        Self::new()
    }
}
impl LandscapeBuilder {
    pub fn new() -> Self {
        Self::with_pool(Arc::new(Pool {
            used: AtomicU64::new(0),
            limit: POOL_BYTES,
        }))
    }
    pub fn with_byte_limit(limit: u64) -> Result<Self, TerrainError> {
        if limit == 0 || limit > POOL_BYTES {
            return Err(TerrainError::ResourceLimit);
        }
        Ok(Self::with_pool(Arc::new(Pool {
            used: AtomicU64::new(0),
            limit,
        })))
    }
    fn with_pool(pool: Arc<Pool>) -> Self {
        Self {
            work: PreparationWork::default(),
            basis: BTreeMap::new(),
            basis_context: None,
            pool,
            cache: BTreeMap::new(),
            empty: BTreeMap::new(),
            pending: None,
            ready: None,
            state: PreparationState::Idle,
        }
    }
    pub fn work(&self) -> PreparationWork {
        self.work
    }
    pub fn reserved_bytes(&self) -> u64 {
        self.pool.used.load(Ordering::Relaxed)
    }
    pub fn state(&self) -> PreparationState {
        self.state
    }
    pub fn cancel(&mut self) {
        self.pending = None;
        self.ready = None;
        self.state = PreparationState::Cancelled;
    }
    pub fn clear_cache(&mut self) {
        self.cache.clear();
        self.empty.clear();
        self.basis.clear();
    }
    pub fn begin(&mut self, input: LandscapeInput) -> Result<(), TerrainError> {
        self.work = PreparationWork::default();
        let mut keys = BTreeSet::new();
        for s in input
            .sources()
            .iter()
            .filter(|s| s.kind != LandscapeKind::Excluded)
        {
            let p = input.projection.hex_center(AxialPoint {
                q: s.coordinate[0],
                r: s.coordinate[1],
            });
            let reach = PATCH_SIZE * 1.1;
            for x in ((p.x - reach) / PATCH_SIZE).floor() as i32
                ..=((p.x + reach) / PATCH_SIZE).floor() as i32
            {
                for y in ((p.y - reach) / PATCH_SIZE).floor() as i32
                    ..=((p.y + reach) / PATCH_SIZE).floor() as i32
                {
                    keys.insert([x, y]);
                    if keys.len() > MAX_PATCHES {
                        return Err(TerrainError::ResourceLimit);
                    }
                }
            }
        }
        // Validate and hash before replacing current work. Stream the resolved
        // layouts with one aggregate bound; never allocate their JSON bodies.
        let mut encoded = 0usize;
        let mut source_hashes: Vec<[u8; 32]> = Vec::with_capacity(input.sources.len());
        for source in &input.sources {
            let mut w = HashWriter {
                hash: Sha256::new(),
                remaining: 8 * 1024 * 1024 - encoded,
                written: 0,
            };
            serde_json::to_writer(&mut w, source).map_err(|_| TerrainError::ResourceLimit)?;
            encoded += w.written;
            source_hashes.push(w.hash.finalize().into());
        }
        let mut h = Sha256::new();
        h.update(b"landscape-fields-v4/primitive-support/132/128/32");
        h.update(input.seed.to_le_bytes());
        h.update(format!("{:?}/{:?}", input.body, input.projection).as_bytes());
        if let Some(relief) = &input.relief {
            h.update([1]);
            h.update(relief.content_hash());
        } else {
            h.update([0]);
        }
        // Topology is resolved once from the complete input. Patch samplers and
        // cache signatures use the same conservative primitive support sets.

        let context: [u8; 32] = h.finalize().into();
        // Patch signatures use local dependencies, but scene consumers must
        // observe EVERY source change, including additions outside a patch.
        let mut identity = Sha256::new();
        identity.update(context);
        for hash in &source_hashes {
            identity.update(hash);
        }
        let identity = identity.finalize().into();
        let source_bytes = input
            .sources
            .iter()
            .filter_map(|s| s.layout.as_ref())
            .try_fold(0usize, |bytes, layout| {
                bytes.checked_add(layout.layout().storage_bytes())
            })
            .ok_or(TerrainError::ResourceLimit)?;
        let allocation = self.pool.allocate(
            (source_bytes
                + input.sources.capacity() * std::mem::size_of::<LandscapeSource>()
                + input.sources.len() * 2048
                + 65536) as u64,
        )?;
        self.pending = None;
        self.ready = None;
        self.cache.retain(|k, _| keys.contains(k));
        self.empty.retain(|k, _| keys.contains(k));
        if self.basis_context != Some(context) {
            self.basis.clear();
            self.basis_context = Some(context);
        }
        self.basis.retain(|k, _| keys.contains(k));
        let total = keys.len() * CELLS;
        let sampler = Arc::new(LandscapeSampler::new(input.clone()));
        self.pending = Some(Request {
            input,
            keys: keys.into_iter().collect(),
            sampler,
            context,
            identity,
            source_hashes,
            next: 0,
            done: 0,
            patches: BTreeMap::new(),
            current: None,
            _allocation: allocation,
        });
        self.state = PreparationState::Preparing {
            completed: 0,
            total,
        };
        Ok(())
    }
    /// Browser fallback cooperatively prepares at most 1024 texels per poll.
    /// Ready is published only after all patches and exact seam validation.
    pub fn step(&mut self, budget: NonZeroUsize) -> Result<usize, TerrainError> {
        if budget.get() > MAX_POLL {
            return Err(TerrainError::ResourceLimit);
        }
        let result = self.step_inner(budget.get());
        if let Err(e) = result {
            self.pending = None;
            self.state = PreparationState::Failed(e);
        }
        result
    }
    fn step_inner(&mut self, budget: usize) -> Result<usize, TerrainError> {
        let Some(request) = self.pending.as_mut() else {
            return Ok(0);
        };
        let mut spent = 0;
        while request.next < request.keys.len() && spent < budget {
            let key = request.keys[request.next];
            if request.current.is_none() {
                let mut h = Sha256::new();
                h.update(request.context);
                h.update(key[0].to_le_bytes());
                h.update(key[1].to_le_bytes());
                let sampler = request.sampler.for_patch(key);
                for index in sampler.dependencies() {
                    h.update(request.source_hashes[index]);
                }
                let signature: [u8; 32] = h.finalize().into();
                if self.empty.get(&key) == Some(&signature) {
                    self.work.reused_patches += 1;
                    request.done += CELLS;
                    request.next += 1;
                    continue;
                }
                if let Some(old) = self.cache.get(&key).filter(|p| p.signature == signature) {
                    request.patches.insert(key, old.clone());
                    self.work.reused_patches += 1;
                    request.done += CELLS;
                    request.next += 1;
                    continue;
                }
                // Free the builder's obsolete cache reference before admitting
                // replacement. Actual current scene references retain their charge.
                self.cache.remove(&key);
                self.empty.remove(&key);
                let allocation = self.pool.allocate(PATCH_BYTES)?;
                let patch = LandscapePatch {
                    key,
                    signature,
                    terrain: vec![0; CELLS * 4],
                    materials: vec![0; CELLS * 4],
                    detail: vec![0; CELLS * 4],
                    water: false,
                    plants: Vec::new(),
                    bare: None,
                    contact: std::sync::OnceLock::new(),
                    packed: std::sync::OnceLock::new(),
                    _allocation: allocation,
                };
                request.current = Some(ActivePatch {
                    basis: self.basis.get(&key).cloned(),
                    basis_work: if self.basis.contains_key(&key) {
                        None
                    } else {
                        let allocation = self.pool.allocate(
                            (CELLS * std::mem::size_of::<fields::SampleBasis>()) as u64,
                        )?;
                        Some(BasisGrid {
                            samples: Vec::with_capacity(CELLS),
                            _allocation: allocation,
                        })
                    },
                    scatter: None,
                    patch,
                    sampler: Arc::new(sampler),
                    visible: false,
                    cells: 0,
                });
            }
            let current = request.current.as_mut().expect("admitted patch");
            while current.cells < CELLS && spent < budget {
                let index = current.cells;
                let (col, row) = (index % FIELD_SIZE, index / FIELD_SIZE);
                // Shared integer lattice gives bit-identical overlap texels even
                // across negative keys and corner-only adjacent patches.
                let p = BoardPoint {
                    x: f64::from(
                        key[0] * (FIELD_SAMPLES - 1) as i32 + col as i32 - FIELD_BORDER as i32,
                    ) * FIELD_STEP,
                    y: f64::from(
                        key[1] * (FIELD_SAMPLES - 1) as i32 + row as i32 - FIELD_BORDER as i32,
                    ) * FIELD_STEP,
                };
                let basis = if let Some(grid) = &current.basis {
                    grid.samples[index]
                } else {
                    self.work.noise_samples += 1;
                    let basis = current.sampler.basis(p)?;
                    current
                        .basis_work
                        .as_mut()
                        .expect("building basis")
                        .samples
                        .push(basis);
                    basis
                };
                self.work.field_samples += 1;
                let s = current.sampler.sample_with_basis(p, false, basis)?;
                let offset = index * 4;
                for (n, v) in [s.height, s.shore, s.coverage, s.foundation]
                    .into_iter()
                    .enumerate()
                {
                    current.patch.terrain[offset + n] = half(v);
                }
                for (n, v) in [s.dry, s.lush, s.litter, s.mud].into_iter().enumerate() {
                    current.patch.materials[offset + n] = (clamp(v) * 255.0).round() as u8;
                }
                for (n, v) in [s.meadow, s.mound, s.greenery, -s.relief / 0.036]
                    .into_iter()
                    .enumerate()
                {
                    current.patch.detail[offset + n] = (clamp(v) * 255.0).round() as u8;
                }
                current.visible |=
                    s.coverage > 0.001 || s.foundation > 0.001 || s.shore < LAKE_GROUND_REACH;
                current.patch.water |= s.shore < 0.005;
                current.cells += 1;
                request.done += 1;
                spent += 1;
            }
            if current.cells == CELLS && current.visible {
                if current.scatter.is_none() {
                    current.patch.plants = Vec::with_capacity(vegetation::MAX_PLANTS_PER_PATCH);
                    current.scatter =
                        Some(vegetation::PlantScatter::new(current.sampler.clone(), key)?);
                }
                let scatter = current.scatter.as_mut().expect("admitted scatter");
                if spent < budget {
                    let checks = scatter.step(&mut current.patch.plants, budget - spent)?;
                    self.work.scatter_checks += checks;
                    spent += checks;
                }
                if !scatter.complete() {
                    continue;
                }
            }
            if current.cells == CELLS {
                let mut current = request.current.take().expect("complete current patch");
                if let Some(basis) = current.basis_work.take() {
                    self.basis.insert(key, Arc::new(basis));
                }
                if current.visible {
                    current.scatter = None;
                    current.patch.plants.shrink_to_fit();
                    current.patch._allocation.resize(
                        PATCH_BASE_BYTES
                            + (current.patch.plants.capacity()
                                * std::mem::size_of::<vegetation::LandscapePlant>())
                                as u64,
                    )?;
                    let patch = Arc::new(current.patch);
                    request.patches.insert(key, patch.clone());
                    self.cache.insert(key, patch);
                } else {
                    self.empty.insert(key, current.patch.signature);
                }
                request.next += 1;
            }
        }
        self.state = PreparationState::Preparing {
            completed: request.done,
            total: request.keys.len() * CELLS,
        };
        if request.next == request.keys.len() {
            // Include one conversion scratch patch and a full maximum-size
            // relief clone; neither can create uncharged finalization storage.
            let allocation = self.pool.allocate(
                (request.patches.len() * 512
                    + CELLS * 8
                    + super::super::geometry::terrain::MAX_RELIEF_PIXELS * 4
                    + 4096) as u64,
            )?;
            let patches = request
                .patches
                .values()
                .map(|p| p.contact())
                .collect::<Result<Vec<_>, _>>()?;
            let relief = request.input.relief.as_deref().cloned();
            let field = TerrainField::new(
                request.input.projection,
                request.input.revision,
                request.input.seed,
                relief,
                patches,
            )?;
            let request = self.pending.take().expect("completed request");
            let count = request.patches.len();
            self.ready = Some(Arc::new(LandscapeSnapshot {
                sources: request.input.sources,
                _sources_allocation: request._allocation,
                source_identity: request.identity,
                base_identity: request.context,
                basis: self.basis.clone(),
                patches: request.patches,
                sampler: SampleView {
                    body: request.input.body,
                    field,
                },
                _allocation: allocation,
            }));
            self.state = PreparationState::Ready { patches: count };
        }
        Ok(spent)
    }
    pub fn take_ready(&mut self) -> Option<Arc<LandscapeSnapshot>> {
        let ready = self.ready.take();
        if ready.is_some() {
            self.state = PreparationState::Idle
        }
        ready
    }
    pub fn prepared(&self) -> Option<&Arc<LandscapeSnapshot>> {
        self.ready.as_ref()
    }
}
struct HashWriter {
    hash: Sha256,
    remaining: usize,
    written: usize,
}
impl Write for HashWriter {
    fn write(&mut self, b: &[u8]) -> std::io::Result<usize> {
        if b.len() > self.remaining {
            return Err(std::io::ErrorKind::FileTooLarge.into());
        }
        self.hash.update(b);
        self.remaining -= b.len();
        self.written += b.len();
        Ok(b.len())
    }
    fn flush(&mut self) -> std::io::Result<()> {
        Ok(())
    }
}
// Match Three DataUtils' truncating binary16 conversion, including subnormals
// and negative zero. Rounding-to-nearest would create different source fields.
pub(super) fn half(value: f64) -> u16 {
    let bits = (value.clamp(-65504.0, 65504.0) as f32).to_bits();
    let sign = ((bits >> 16) & 0x8000) as u16;
    let e = ((bits >> 23) & 255) as i32 - 127;
    let mantissa = bits & 0x7fffff;
    if e < -27 {
        sign
    } else if e < -14 {
        sign + ((0x400u32 >> (-e - 14)) + (mantissa >> (-e - 1))) as u16
    } else {
        sign + ((((e + 15) as u32) << 10) + (mantissa >> 13)) as u16
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    fn builder() -> LandscapeBuilder {
        LandscapeBuilder::new()
    }
    fn input(revision: u64, kind: LandscapeKind) -> LandscapeInput {
        LandscapeInput::new(
            BodyId::Mars,
            crate::geometry::BodyProjection::MARS,
            revision,
            73,
            None,
            vec![LandscapeSource {
                coordinate: [0, 0, 0],
                kind,
                seed: 3,
                layout: None,
            }],
        )
        .unwrap()
    }
    fn complete(b: &mut LandscapeBuilder) -> Arc<LandscapeSnapshot> {
        while matches!(b.state(), PreparationState::Preparing { .. }) {
            assert!(b.step(NonZeroUsize::new(1024).unwrap()).unwrap() <= 1024)
        }
        b.take_ready().unwrap()
    }
    #[test]
    fn preparation_has_exact_seams_atomic_readiness_cancellation_and_retained_budget() {
        let mut b = builder();
        let pool = b.pool.clone();
        b.begin(input(1, LandscapeKind::Ocean)).unwrap();
        assert!(b.take_ready().is_none());
        assert_eq!(b.step(NonZeroUsize::new(7).unwrap()).unwrap(), 7);
        assert!(b.take_ready().is_none());
        b.cancel();
        assert_eq!(b.reserved_bytes(), 0);
        b.begin(input(2, LandscapeKind::Ocean)).unwrap();
        let snapshot = complete(&mut b);
        assert!(snapshot.patches().count() > 1);
        assert_eq!(snapshot.revision(), 2);
        let center = snapshot
            .sampler()
            .sample(BoardPoint { x: 0.0, y: 0.0 })
            .unwrap();
        assert!(center.height < super::super::WATER_LEVEL as f32);
        assert!(center.shore < 0.0);
        let before = b.reserved_bytes();
        let noise_bytes = b
            .basis
            .values()
            .map(|grid| grid._allocation.bytes)
            .sum::<u64>();
        b.clear_cache();
        assert!(noise_bytes > 0);
        assert_eq!(
            b.reserved_bytes(),
            before,
            "the published snapshot retains its prepared base rows"
        );
        drop(b);
        assert!(
            snapshot
                .sampler()
                .height(BoardPoint { x: 0.0, y: 0.0 })
                .unwrap()
                .is_finite()
        );
        drop(snapshot);
        assert_eq!(pool.used.load(Ordering::Relaxed), 0);
    }
    #[test]
    fn published_layouts_keep_their_pool_charge_after_builder_and_cache_drop() {
        let layout =
            crate::structure::layout::generate(19, &Default::default(), "Published city").unwrap();
        let bytes = layout.layout().storage_bytes() as u64;
        let mut source = input(1, LandscapeKind::Structure);
        source.sources[0].layout = Some(layout.clone());
        let mut b = builder();
        let pool = b.pool.clone();
        b.begin(source).unwrap();
        assert!(b.reserved_bytes() >= bytes);
        let snapshot = complete(&mut b);
        assert_eq!(snapshot.sources()[0].layout.as_ref().unwrap(), &layout);
        assert!(snapshot._sources_allocation.bytes >= bytes);
        drop(layout);
        b.clear_cache();
        drop(b);
        assert!(pool.used.load(Ordering::Relaxed) >= bytes);
        assert_eq!(
            snapshot.sources()[0].layout.as_ref().unwrap().layout().name,
            "Published city"
        );
        drop(snapshot);
        assert_eq!(pool.used.load(Ordering::Relaxed), 0);
    }
    #[test]
    fn admission_failure_releases_work_and_never_publishes_partial_fields() {
        let mut b = LandscapeBuilder::with_byte_limit(128 * 1024).unwrap();
        b.begin(input(1, LandscapeKind::Ocean)).unwrap();
        assert_eq!(
            b.step(NonZeroUsize::new(1024).unwrap()),
            Err(TerrainError::ResourceLimit)
        );
        assert_eq!(
            b.state(),
            PreparationState::Failed(TerrainError::ResourceLimit)
        );
        assert!(b.take_ready().is_none());
        assert_eq!(b.reserved_bytes(), 0);
        assert!(LandscapeBuilder::with_byte_limit(0).is_err());
        assert!(LandscapeBuilder::with_byte_limit(POOL_BYTES + 1).is_err());
    }
    #[test]
    fn joined_oceans_and_greenery_at_board_edge_share_complete_topology_and_seams() {
        let sources = [
            ([0, -4, 4], LandscapeKind::Greenery),
            ([1, -4, 3], LandscapeKind::Ocean),
            ([2, -4, 2], LandscapeKind::Greenery),
            ([3, -4, 1], LandscapeKind::Ocean),
            ([4, -4, 0], LandscapeKind::Ocean),
            ([-1, -3, 4], LandscapeKind::Greenery),
            ([0, -3, 3], LandscapeKind::Greenery),
            ([1, -3, 2], LandscapeKind::Greenery),
            ([2, -3, 1], LandscapeKind::Greenery),
            ([3, -3, 0], LandscapeKind::Greenery),
            ([4, -3, -1], LandscapeKind::Ocean),
        ]
        .into_iter()
        .enumerate()
        .map(|(index, (coordinate, kind))| LandscapeSource {
            coordinate,
            kind,
            seed: index as u32 + 37,
            layout: None,
        })
        .collect();
        let input = LandscapeInput::new(
            BodyId::Mars,
            crate::geometry::BodyProjection::MARS,
            1,
            42,
            None,
            sources,
        )
        .unwrap();
        let mut preparation = builder();
        preparation.begin(input).unwrap();
        let snapshot = complete(&mut preparation);
        assert!(snapshot.patches().count() > 1);
        assert!(snapshot.patches().any(|p| p.water));
        assert!(snapshot.patches().any(|p| !p.plants().is_empty()));
        let mut plant_ids = BTreeSet::new();
        for plant in snapshot.patches().flat_map(|p| p.plants()) {
            assert!(plant_ids.insert(plant.id));
        }
        // Completion constructs the real immutable TerrainField, including its
        // exact horizontal, vertical and corner seam checks. No tolerance or
        // duplicate-border filtering can hide mismatched neighbouring texels.
        assert!(
            snapshot
                .sampler()
                .height(BoardPoint { x: 0., y: 0. })
                .unwrap()
                .is_finite()
        );
    }
    #[test]
    fn cached_unaffected_patches_keep_identity_changed_sources_invalidate_locally() {
        let mut b = builder();
        b.begin(input(1, LandscapeKind::Greenery)).unwrap();
        let a = complete(&mut b);
        b.begin(input(2, LandscapeKind::Greenery)).unwrap();
        let c = complete(&mut b);
        assert_eq!(a.source_identity(), c.source_identity());
        assert_eq!(b.work().noise_samples, 0);
        assert_eq!(b.work().field_samples, 0);
        assert!(b.work().reused_patches > 0);
        assert!(
            a.patches
                .keys()
                .all(|k| Arc::ptr_eq(&a.patches[k], &c.patches[k]))
        );
        b.begin(input(3, LandscapeKind::Ocean)).unwrap();
        let d = complete(&mut b);
        assert_ne!(a.source_identity(), d.source_identity());
        assert!(
            a.patches
                .keys()
                .filter(|k| d.patches.contains_key(*k))
                .all(|k| !Arc::ptr_eq(&a.patches[k], &d.patches[k]))
        );
    }
    #[test]
    fn prepared_relief_is_reused_only_for_the_same_map_content() {
        let mut b = builder();
        let mut first = input(1, LandscapeKind::Ocean);
        first.relief = Some(Arc::new(
            crate::geometry::terrain::ReliefMap::new(4, 2, vec![0.012; 8]).unwrap(),
        ));
        b.begin(first.clone()).unwrap();
        let old = complete(&mut b);
        let key = old.patches().next().unwrap().key();
        let mut next = first;
        next.revision = 2;
        next.relief = Some(Arc::new(
            crate::geometry::terrain::ReliefMap::new(4, 2, vec![0.024; 8]).unwrap(),
        ));
        b.begin(next.clone()).unwrap();
        let actual = complete(&mut b);
        assert!(b.work().noise_samples > 0);
        let mut fresh = builder();
        fresh.begin(next).unwrap();
        let expected = complete(&mut fresh);
        for patch in actual.patches() {
            assert_eq!(
                patch.terrain(),
                expected.patch(patch.key()).unwrap().terrain()
            );
        }
        assert_ne!(old.base_identity(), actual.base_identity());
        assert!(
            (actual.bare_row(key, 0).unwrap().next().unwrap() - (-f64::from(0.024f32) + 0.0003))
                .abs()
                < 1e-12
        );
        assert!(actual.bare_row(key, usize::MAX).is_none());
    }
    #[test]
    fn local_edits_match_full_rebuild_including_lake_triangles_and_removals() {
        let source = |q, r, kind| LandscapeSource {
            coordinate: [q, r, -q - r],
            kind,
            seed: 17,
            layout: None,
        };
        let mut sources = vec![
            source(-4, 0, LandscapeKind::Greenery),
            source(4, 0, LandscapeKind::Greenery),
        ];
        let make = |sources: Vec<LandscapeSource>, revision| {
            LandscapeInput::new(
                BodyId::Mars,
                crate::geometry::BodyProjection::MARS,
                revision,
                73,
                None,
                sources,
            )
            .unwrap()
        };
        let mut cached = builder();
        cached.begin(make(sources.clone(), 0)).unwrap();
        let initial = complete(&mut cached);
        let mut reused = 0;
        let mut previous_identity = initial.source_identity();
        for (index, next) in [
            source(0, 0, LandscapeKind::Ocean),
            source(1, 0, LandscapeKind::Ocean),
            source(0, 1, LandscapeKind::Ocean),
            source(-1, 0, LandscapeKind::Greenery),
            source(1, -1, LandscapeKind::Excluded),
            source(0, 0, LandscapeKind::Greenery),
            source(1, -1, LandscapeKind::SpecialGreenery),
            source(-1, 0, LandscapeKind::Volcano),
            LandscapeSource {
                layout: Some(
                    crate::structure::layout::generate(17, &Default::default(), "City").unwrap(),
                ),
                ..source(0, 0, LandscapeKind::Structure)
            },
            LandscapeSource {
                layout: Some(
                    crate::structure::layout::generate(31, &Default::default(), "City").unwrap(),
                ),
                ..source(1, 0, LandscapeKind::Structure)
            },
            LandscapeSource {
                layout: Some(
                    crate::structure::layout::generate(71, &Default::default(), "Capital").unwrap(),
                ),
                ..source(0, 0, LandscapeKind::Structure)
            },
        ]
        .into_iter()
        .enumerate()
        {
            sources.retain(|old| old.coordinate != next.coordinate);
            sources.push(next);
            let request = make(sources.clone(), index as u64 + 1);
            cached.begin(request.clone()).unwrap();
            let actual = complete(&mut cached);
            assert_ne!(actual.source_identity(), previous_identity);
            previous_identity = actual.source_identity();
            let mut reference = builder();
            reference.begin(request).unwrap();
            let expected = complete(&mut reference);
            assert_eq!(actual.source_identity(), expected.source_identity());
            assert_eq!(actual.patches.len(), expected.patches.len());
            for (key, patch) in &actual.patches {
                let full = &expected.patches[key];
                assert_eq!(patch.terrain, full.terrain, "height/shore {key:?}");
                assert_eq!(patch.materials, full.materials, "materials {key:?}");
                assert_eq!(patch.detail, full.detail, "detail {key:?}");
                assert_eq!(patch.plants, full.plants, "scatter {key:?}");
                reused += usize::from(
                    initial
                        .patches
                        .get(key)
                        .is_some_and(|old| Arc::ptr_eq(old, patch)),
                );
            }
        }
        sources.retain(|s| s.coordinate != [0, 1, -1]);
        let request = make(sources, 7);
        cached.begin(request.clone()).unwrap();
        let actual = complete(&mut cached);
        assert_ne!(actual.source_identity(), previous_identity);
        let mut reference = builder();
        reference.begin(request).unwrap();
        let expected = complete(&mut reference);
        for (key, patch) in &actual.patches {
            assert_eq!(patch.terrain, expected.patches[key].terrain);
            assert_eq!(patch.plants, expected.patches[key].plants);
        }
        assert!(reused > 0, "distant unchanged patches must retain identity");
    }
    #[test]
    fn half_matches_reference_truncation_and_subnormal_edges() {
        assert_eq!(half(1.0), 0x3c00);
        assert_eq!(half(-0.0), 0x8000);
        assert_eq!(half(2.0_f64.powi(-24)), 1);
        assert_eq!(half(65504.0), 0x7bff);
        assert_eq!(half(0.0003), 0x0cea);
    }
}
