//! Renderer-independent preparation protocol. Source metadata is bounded JSON;
//! canonical field planes and relief samples transfer as exact binary bytes.
use super::*;
use crate::geometry::{BodyProjection, terrain::ReliefMap};
use serde::{Deserialize, Serialize};

pub const MAX_PACKET_BYTES: usize = 96 * 1024 * 1024;
#[derive(Serialize, Deserialize)]
struct Relief {
    dimensions: [usize; 2],
    depths: Vec<f32>,
}
#[derive(Serialize, Deserialize)]
struct Job {
    ticket: u64,
    owner: u32,
    revision: u64,
    body: BodyId,
    projection: (f64, f64, i8, [i32; 2]),
    seed: u32,
    relief_hash: Option<[u8; 32]>,
    relief: Option<Relief>,
    sources: Vec<LandscapeSource>,
    known: Vec<([i32; 2], [u8; 32])>,
}
#[derive(Serialize, Deserialize)]
struct Patch {
    key: [i32; 2],
    signature: [u8; 32],
    fresh: bool,
    water: bool,
    plants: Vec<vegetation::LandscapePlant>,
}
#[derive(Serialize, Deserialize)]
struct ResultHeader {
    ticket: u64,
    revision: u64,
    identity: [u8; 32],
    context: [u8; 32],
    patches: Vec<Patch>,
}
#[derive(Default)]
pub struct TerrainWorker {
    builder: LandscapeBuilder,
    owner: u32,
    revision: u64,
    transferred: usize,
    reused: usize,
    bytes: usize,
    relief: Option<Arc<ReliefMap>>,
    relief_hash: Option<[u8; 32]>,
}
impl TerrainWorker {
    pub fn metadata(&self) -> String {
        format!(
            "{{\"context\":{},\"landscapeRevision\":{},\"patchesTransferred\":{},\"patchesReused\":{},\"payloadBytes\":{}}}",
            self.owner, self.revision, self.transferred, self.reused, self.bytes
        )
    }

    pub fn run(&mut self, bytes: &[u8]) -> Result<Vec<u8>, String> {
        if bytes.len() > MAX_PACKET_BYTES {
            return Err("Terrain request too large".into());
        }
        let job: Job = serde_json::from_slice(bytes).map_err(|e| e.to_string())?;
        self.owner = job.owner;
        self.revision = job.revision;
        if let Some(relief) = job.relief {
            let map = ReliefMap::new(relief.dimensions[0], relief.dimensions[1], relief.depths)
                .map_err(|e| e.to_string())?;
            if Some(map.content_hash()) != job.relief_hash {
                return Err("Terrain relief identity mismatch".into());
            }
            self.relief = Some(Arc::new(map));
            self.relief_hash = job.relief_hash;
        } else if job.relief_hash.is_none() {
            self.relief = None;
            self.relief_hash = None;
        }
        if self.relief_hash != job.relief_hash {
            return Err("Missing terrain relief".into());
        }
        let (radius, content, sign, [q, r]) = job.projection;
        let projection = BodyProjection::new(radius, content, sign, AxialPoint { q, r })
            .map_err(|e| e.to_string())?;
        let input = LandscapeInput::new(
            job.body,
            projection,
            job.revision,
            job.seed,
            self.relief.clone(),
            job.sources,
        )
        .map_err(|e| e.to_string())?;
        self.builder.begin(input).map_err(|e| e.to_string())?;
        while matches!(self.builder.state(), PreparationState::Preparing { .. }) {
            self.builder
                .step(NonZeroUsize::new(MAX_POLL).unwrap())
                .map_err(|e| e.to_string())?;
        }
        let snapshot = self
            .builder
            .take_ready()
            .ok_or("Missing prepared terrain")?;
        let known: BTreeMap<_, _> = job.known.into_iter().collect();
        let header = ResultHeader {
            ticket: job.ticket,
            revision: snapshot.revision(),
            identity: snapshot.source_identity,
            context: snapshot.base_identity,
            patches: snapshot
                .patches()
                .map(|p| {
                    let fresh = known.get(&p.key) != Some(&p.signature);
                    Patch {
                        key: p.key,
                        signature: p.signature,
                        fresh,
                        water: p.water,
                        plants: if fresh { p.plants.clone() } else { vec![] },
                    }
                })
                .collect(),
        };
        self.transferred = header.patches.iter().filter(|p| p.fresh).count();
        self.reused = header.patches.len() - self.transferred;
        let json = serde_json::to_vec(&header).map_err(|e| e.to_string())?;
        let mut out = Vec::with_capacity(4 + json.len());
        out.extend((json.len() as u32).to_le_bytes());
        out.extend(json);
        for p in header.patches.iter().filter(|p| p.fresh) {
            let patch = snapshot.patch(p.key).unwrap();
            let packed = patch.packed();
            out.extend_from_slice(&packed.terrain);
            out.extend_from_slice(&packed.surface);
            for row in 0..FIELD_SIZE {
                for value in snapshot
                    .bare_row(p.key, row)
                    .ok_or("Missing prepared relief")?
                {
                    out.extend(value.to_le_bytes());
                }
            }
        }
        if out.len() > MAX_PACKET_BYTES {
            return Err("Terrain result too large".into());
        }
        self.bytes = out.len();
        Ok(out)
    }
}
impl LandscapeBuilder {
    pub fn worker_request(
        &self,
        ticket: u64,
        owner: u32,
        with_relief: bool,
    ) -> Result<Vec<u8>, String> {
        let request = self.pending.as_ref().ok_or("Missing terrain request")?;
        let input = &request.input;
        let offset = input.projection.coord_offset();
        let job = Job {
            ticket,
            owner,
            revision: input.revision,
            body: input.body,
            projection: (
                input.projection.radius(),
                input.projection.content_radius(),
                input.projection.y_sign(),
                [offset.q, offset.r],
            ),
            seed: input.seed,
            relief_hash: input.relief.as_ref().map(|r| r.content_hash()),
            relief: with_relief
                .then(|| {
                    input.relief.as_ref().map(|r| Relief {
                        dimensions: r.dimensions(),
                        depths: r.depths().to_vec(),
                    })
                })
                .flatten(),
            sources: input.sources.clone(),
            known: self.cache.iter().map(|(k, p)| (*k, p.signature)).collect(),
        };
        let bytes = serde_json::to_vec(&job).map_err(|e| e.to_string())?;
        if bytes.len() > MAX_PACKET_BYTES {
            return Err("Terrain request too large".into());
        }
        Ok(bytes)
    }
    /// A cancelled/superseded result is discarded before any allocation or mutation.
    pub fn accept_worker(&mut self, ticket: u64, bytes: &[u8]) -> Result<bool, String> {
        if bytes.len() < 4 || bytes.len() > MAX_PACKET_BYTES {
            return Err("Invalid terrain packet bounds".into());
        }
        let size = u32::from_le_bytes(bytes[..4].try_into().unwrap()) as usize;
        let mut cursor = 4usize
            .checked_add(size)
            .filter(|n| *n <= bytes.len())
            .ok_or("Invalid terrain header")?;
        let header: ResultHeader =
            serde_json::from_slice(&bytes[4..cursor]).map_err(|e| e.to_string())?;
        if header.ticket != ticket
            || self
                .pending
                .as_ref()
                .is_none_or(|r| r.input.revision != header.revision)
        {
            return Ok(false);
        }
        let request = self.pending.as_ref().unwrap();
        if header.identity != request.identity
            || header.context != request.context
            || header.patches.len() > MAX_PATCHES
        {
            return Err("Terrain result identity mismatch".into());
        }
        let mut patches = BTreeMap::new();
        for p in header.patches {
            if !request.keys.contains(&p.key) || patches.contains_key(&p.key) {
                return Err("Invalid terrain patch key".into());
            }
            let patch = if p.fresh {
                if p.plants.len() > vegetation::MAX_PLANTS_PER_PATCH
                    || p.plants.iter().any(|p| {
                        ![p.point.x, p.point.y, p.height, p.scale, p.rank]
                            .iter()
                            .all(|n| n.is_finite())
                            || !(0.0..=1.0).contains(&p.rank)
                            || p.scale < 0.
                    })
                {
                    return Err("Invalid terrain plants".into());
                }
                let length = CELLS * 24;
                let end = cursor
                    .checked_add(length)
                    .filter(|end| *end <= bytes.len())
                    .ok_or("Truncated terrain patch")?;
                let data = &bytes[cursor..end];
                cursor = end;
                let packed = Arc::new(super::super::upload::PackedField {
                    terrain: data[..CELLS * 8].to_vec(),
                    surface: data[CELLS * 8..CELLS * 16].to_vec(),
                });
                let terrain = packed
                    .terrain
                    .as_chunks::<2>()
                    .0
                    .iter()
                    .map(|v| u16::from_le_bytes(*v))
                    .collect();
                let mut materials = Vec::with_capacity(CELLS * 4);
                let mut detail = Vec::with_capacity(CELLS * 4);
                for row in packed.surface.as_chunks::<{ FIELD_SIZE * 8 }>().0 {
                    materials.extend_from_slice(&row[..FIELD_SIZE * 4]);
                    detail.extend_from_slice(&row[FIELD_SIZE * 4..]);
                }
                let bare: Vec<_> = data[CELLS * 16..]
                    .as_chunks::<8>()
                    .0
                    .iter()
                    .map(|v| f64::from_le_bytes(*v))
                    .collect();
                if bare.iter().any(|n| !n.is_finite()) {
                    return Err("Invalid terrain relief samples".into());
                }
                let allocation = self
                    .pool
                    .allocate(
                        PATCH_BASE_BYTES
                            + (CELLS * 8
                                + p.plants.len()
                                    * std::mem::size_of::<vegetation::LandscapePlant>())
                                as u64,
                    )
                    .map_err(|e| e.to_string())?;
                let cell = std::sync::OnceLock::new();
                let _ = cell.set(packed);
                Arc::new(LandscapePatch {
                    key: p.key,
                    signature: p.signature,
                    terrain,
                    materials,
                    detail,
                    water: p.water,
                    plants: p.plants,
                    bare: Some(bare.into()),
                    contact: std::sync::OnceLock::new(),
                    packed: cell,
                    _allocation: allocation,
                })
            } else {
                self.cache
                    .get(&p.key)
                    .filter(|cached| cached.signature == p.signature)
                    .ok_or("Missing cached terrain patch")?
                    .clone()
            };
            patches.insert(p.key, patch);
        }
        if cursor != bytes.len() {
            return Err("Trailing terrain packet bytes".into());
        }
        let allocation = self
            .pool
            .allocate(
                (patches.len() * 512
                    + CELLS * 8
                    + crate::geometry::terrain::MAX_RELIEF_PIXELS * 4
                    + 4096) as u64,
            )
            .map_err(|e| e.to_string())?;
        let input = &request.input;
        let contact = patches
            .values()
            .map(|p| p.contact())
            .collect::<Result<Vec<_>, _>>()
            .map_err(|e| e.to_string())?;
        let field = TerrainField::new(
            input.projection,
            input.revision,
            input.seed,
            input.relief.as_deref().cloned(),
            contact,
        )
        .map_err(|e| e.to_string())?;
        let request = self.pending.take().unwrap();
        let count = patches.len();
        self.cache = patches.clone();
        self.ready = Some(Arc::new(LandscapeSnapshot {
            sources: request.input.sources,
            _sources_allocation: request._allocation,
            source_identity: request.identity,
            base_identity: request.context,
            basis: BTreeMap::new(),
            patches,
            sampler: SampleView {
                body: request.input.body,
                field,
            },
            _allocation: allocation,
        }));
        self.state = PreparationState::Ready { patches: count };
        Ok(true)
    }
    pub fn pending_relief_hash(&self) -> Option<[u8; 32]> {
        self.pending
            .as_ref()
            .and_then(|r| r.input.relief.as_ref().map(|r| r.content_hash()))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    fn input(revision: u64, cells: &[(i32, i32, LandscapeKind)]) -> LandscapeInput {
        let relief = ReliefMap::new(
            8,
            4,
            (0..32)
                .map(|i| {
                    if !(8..24).contains(&i) {
                        0.01
                    } else {
                        0.01 + i as f32 * 0.0003
                    }
                })
                .collect(),
        )
        .unwrap();
        LandscapeInput::new(
            BodyId::Mars,
            BodyProjection::MARS,
            revision,
            73,
            Some(Arc::new(relief)),
            cells
                .iter()
                .map(|&(q, r, kind)| LandscapeSource {
                    coordinate: [q, r, -q - r],
                    kind,
                    seed: 17,
                    layout: None,
                })
                .collect(),
        )
        .unwrap()
    }
    fn complete(builder: &mut LandscapeBuilder, input: LandscapeInput) -> Arc<LandscapeSnapshot> {
        builder.begin(input).unwrap();
        while matches!(builder.state(), PreparationState::Preparing { .. }) {
            builder.step(NonZeroUsize::new(MAX_POLL).unwrap()).unwrap();
        }
        builder.take_ready().unwrap()
    }
    #[test]
    fn worker_round_trips_exact_fields_plants_contact_and_reuses_unchanged_patch_bytes() {
        let mut renderer = LandscapeBuilder::new();
        let mut worker = TerrainWorker::default();
        let mut reference = LandscapeBuilder::new();
        let mut cells = vec![(0, 0, LandscapeKind::Ocean)];
        let mut previous: Option<Arc<LandscapeSnapshot>> = None;
        for revision in 1..=4 {
            let data = input(revision, &cells);
            renderer.begin(data.clone()).unwrap();
            let request = renderer.worker_request(revision, 7, revision == 1).unwrap();
            let result = worker.run(&request).unwrap();
            assert!(renderer.accept_worker(revision, &result).unwrap());
            let actual = renderer.take_ready().unwrap();
            let expected = complete(&mut reference, data);
            assert_eq!(actual.source_identity(), expected.source_identity());
            for (a, b) in actual.patches().zip(expected.patches()) {
                assert_eq!(a.key(), b.key());
                assert_eq!(a.terrain(), b.terrain());
                assert_eq!(a.materials(), b.materials());
                assert_eq!(a.detail(), b.detail());
                assert_eq!(a.plants(), b.plants());
                assert_eq!(a.packed().terrain, b.packed().terrain);
                assert_eq!(a.packed().surface, b.packed().surface);
                for row in [0, 65, 131] {
                    assert_eq!(
                        actual.bare_row(a.key(), row).unwrap().collect::<Vec<_>>(),
                        expected.bare_row(a.key(), row).unwrap().collect::<Vec<_>>()
                    );
                }
                if let Some(old) = &previous
                    && let Some(old_patch) = old.patches.get(&a.key())
                    && old_patch.signature == a.signature
                {
                    assert!(Arc::ptr_eq(
                        old_patch,
                        actual.patches.get(&a.key()).unwrap()
                    ));
                    assert!(Arc::ptr_eq(&old_patch.packed(), &a.packed()));
                }
            }
            for x in -10..10 {
                let p = BoardPoint {
                    x: f64::from(x) * 0.03,
                    y: 0.,
                };
                assert_eq!(actual.sampler().height(p), expected.sampler().height(p));
            }
            previous = Some(actual);
            cells.push((revision as i32, 0, LandscapeKind::Greenery));
        }
        renderer.begin(input(5, &cells[..1])).unwrap();
        let full = worker
            .run(&renderer.worker_request(5, 8, false).unwrap())
            .unwrap();
        renderer.accept_worker(5, &full).unwrap();
        renderer.take_ready().unwrap();
        renderer.begin(input(6, &cells[..1])).unwrap();
        let cached = worker
            .run(&renderer.worker_request(6, 9, false).unwrap())
            .unwrap();
        assert!(cached.len() < full.len() / 10);
        renderer.accept_worker(6, &cached).unwrap();
    }
    #[test]
    fn cancelled_unknown_and_malformed_results_cannot_publish_or_corrupt_the_cache() {
        let mut renderer = LandscapeBuilder::new();
        let mut worker = TerrainWorker::default();
        renderer
            .begin(input(1, &[(0, 0, LandscapeKind::Ocean)]))
            .unwrap();
        let packet = worker
            .run(&renderer.worker_request(11, 3, true).unwrap())
            .unwrap();
        assert!(!renderer.accept_worker(12, &packet).unwrap());
        assert!(renderer.prepared().is_none());
        renderer.cancel();
        assert!(!renderer.accept_worker(11, &packet).unwrap());
        renderer
            .begin(input(1, &[(0, 0, LandscapeKind::Ocean)]))
            .unwrap();
        for bytes in [&packet[..3], &packet[..packet.len() - 1]] {
            assert!(renderer.accept_worker(11, bytes).is_err());
            assert!(renderer.prepared().is_none());
        }
        let mut trailing = packet.clone();
        trailing.push(0);
        assert!(renderer.accept_worker(11, &trailing).is_err());
        assert!(renderer.accept_worker(11, &packet).unwrap());
        renderer.take_ready().unwrap();
        renderer
            .begin(input(2, &[(0, 0, LandscapeKind::Ocean)]))
            .unwrap();
        let cached = worker
            .run(&renderer.worker_request(22, 3, false).unwrap())
            .unwrap();
        renderer.cache.clear();
        assert!(renderer.accept_worker(22, &cached).is_err());
    }
}
