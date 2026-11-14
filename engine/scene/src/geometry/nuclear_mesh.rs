//! Cooperative preparation of the original mask/bowl/scorch radial topology.
//! Caller admits `allocation_bytes` before construction; no texture or GPU owner.
use super::{
    GeometryError,
    nuclear::{GroundSample, NuclearProfile, SEGMENTS},
};
use std::num::NonZeroUsize;
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum DiscKind {
    Mask,
    Bowl,
    Scorch,
}
impl DiscKind {
    pub fn rings(self) -> usize {
        match self {
            Self::Mask => 1,
            Self::Bowl => 40,
            Self::Scorch => 20,
        }
    }
    pub fn vertices(self) -> usize {
        (self.rings() + 1) * (SEGMENTS + 1)
    }
    pub fn indices(self) -> usize {
        self.rings() * SEGMENTS * 6
    }
    pub fn allocation_bytes(self) -> u64 {
        (self.vertices() * 40 + self.indices() * 4 + std::mem::size_of::<NuclearProfile>() + 4096)
            as u64
    }
}
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct TileFrame {
    origin: [f64; 3],
    rotation: [f64; 4],
}
impl TileFrame {
    pub fn new(origin: [f64; 3], rotation: [f64; 4]) -> Result<Self, GeometryError> {
        if !origin.into_iter().chain(rotation).all(f64::is_finite) {
            return Err(GeometryError::NonFinite);
        }
        let radius = origin.iter().map(|v| v * v).sum::<f64>().sqrt();
        if !(0.01..=10000.).contains(&radius) {
            return Err(GeometryError::InvalidRadius);
        }
        if (rotation.iter().map(|v| v * v).sum::<f64>() - 1.).abs() > 1e-10 {
            return Err(GeometryError::InvalidOrientation);
        }
        Ok(Self { origin, rotation })
    }
    pub fn direction(self, x: f64, y: f64) -> Result<[f64; 3], GeometryError> {
        if !x.is_finite() || !y.is_finite() {
            return Err(GeometryError::NonFinite);
        }
        let [qx, qy, qz, qw] = self.rotation;
        let t = [-2. * qz * y, 2. * qz * x, 2. * (qx * y - qy * x)];
        let p = [
            x + qw * t[0] + qy * t[2] - qz * t[1] + self.origin[0],
            y + qw * t[1] + qz * t[0] - qx * t[2] + self.origin[1],
            qw * t[2] + qx * t[1] - qy * t[0] + self.origin[2],
        ];
        let length = p[0].hypot(p[1]).hypot(p[2]);
        if !length.is_finite() || length < 1e-10 {
            return Err(GeometryError::InvalidRadius);
        }
        Ok(p.map(|v| v / length))
    }
}
#[derive(Debug, PartialEq)]
pub struct DiscGeometry {
    pub positions: Vec<[f32; 3]>,
    pub uv: Vec<[f32; 2]>,
    pub coverage: Vec<f32>,
    pub apron: Vec<f32>,
    pub planet_directions: Vec<[f32; 3]>,
    pub indices: Vec<u32>,
}
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum DiscError {
    Geometry(GeometryError),
    Capacity,
    Closed,
}
impl From<GeometryError> for DiscError {
    fn from(e: GeometryError) -> Self {
        Self::Geometry(e)
    }
}
impl std::fmt::Display for DiscError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{self:?}")
    }
}
impl std::error::Error for DiscError {}
pub struct DiscWork {
    profile: NuclearProfile,
    kind: DiscKind,
    frame: TileFrame,
    cursor: usize,
    data: Option<DiscGeometry>,
    failed: bool,
}
fn vector<T>(capacity: usize) -> Result<Vec<T>, DiscError> {
    let mut v = Vec::new();
    v.try_reserve_exact(capacity)
        .map_err(|_| DiscError::Capacity)?;
    Ok(v)
}
impl DiscWork {
    pub fn new(
        profile: &NuclearProfile,
        kind: DiscKind,
        frame: TileFrame,
    ) -> Result<Self, DiscError> {
        let n = kind.vertices();
        Ok(Self {
            profile: profile.clone(),
            kind,
            frame,
            cursor: 0,
            failed: false,
            data: Some(DiscGeometry {
                positions: vector(n)?,
                uv: vector(n)?,
                coverage: vector(n)?,
                apron: vector(n)?,
                planet_directions: vector(n)?,
                indices: vector(kind.indices())?,
            }),
        })
    }
    pub fn progress(&self) -> (usize, usize) {
        (self.cursor, self.kind.vertices())
    }
    pub fn step(
        &mut self,
        limit: NonZeroUsize,
        mut ground: impl FnMut(f64, f64) -> Result<GroundSample, GeometryError>,
    ) -> Result<bool, DiscError> {
        if self.failed || self.data.is_none() {
            return Err(DiscError::Closed);
        }
        let result = self.advance(limit.get().min(4096), &mut ground);
        if result.is_err() {
            self.failed = true;
            self.data = None;
        }
        result
    }
    fn advance(
        &mut self,
        limit: usize,
        ground: &mut impl FnMut(f64, f64) -> Result<GroundSample, GeometryError>,
    ) -> Result<bool, DiscError> {
        let p = &self.profile;
        let rings = self.kind.rings();
        let data = self.data.as_mut().ok_or(DiscError::Closed)?;
        for _ in 0..limit {
            if self.cursor == self.kind.vertices() {
                break;
            }
            let ring = self.cursor / (SEGMENTS + 1);
            let segment = self.cursor % (SEGMENTS + 1);
            let t = ring as f64 / rings as f64;
            let angle = segment as f64 / SEGMENTS as f64 * std::f64::consts::TAU;
            let crest = p.radius(angle)?;
            let foot = p.mound_radius(angle)?;
            let lerp = |a: f64, b: f64, t: f64| (1. - t) * a + t * b;
            let radius = match self.kind {
                DiscKind::Mask => crest * t,
                DiscKind::Bowl => {
                    if t <= 0.25 {
                        crest * t * 1.8
                    } else if t <= 0.5 {
                        crest * lerp(0.45, 0.83, (t - 0.25) * 4.)
                    } else if t <= 0.75 {
                        crest * lerp(0.83, 1., (t - 0.5) * 4.)
                    } else {
                        lerp(crest, foot, (t - 0.75) * 4.)
                    }
                }
                DiscKind::Scorch => {
                    let edge = (0.158
                        + p.fan(angle)? * 0.036
                        + (angle * 3. + p.rotation).sin() * 0.007
                        + (angle * 7. - p.rotation).sin() * 0.004)
                        .clamp(0.137, 0.193);
                    lerp(foot, edge, t)
                }
            };
            let (x, y) = (
                p.center[0] + angle.cos() * radius,
                p.center[1] + angle.sin() * radius,
            );
            let height = if self.kind == DiscKind::Mask {
                0.003
            } else {
                p.ground_height(x, y, &mut *ground)?
            };
            let coverage = if self.kind == DiscKind::Scorch {
                ground(x, y)?.validate()?.coverage
            } else {
                1.
            };
            if [x, y, height]
                .iter()
                .any(|v| !v.is_finite() || v.abs() > f64::from(f32::MAX))
            {
                return Err(GeometryError::InvalidHeight.into());
            }
            data.positions.push([x as f32, y as f32, height as f32]);
            data.uv
                .push([segment as f32 / SEGMENTS as f32, (radius / crest) as f32]);
            data.coverage.push(coverage as f32);
            data.apron.push(if self.kind == DiscKind::Scorch {
                t as f32
            } else {
                0.
            });
            data.planet_directions
                .push(self.frame.direction(x, y)?.map(|v| v as f32));
            if ring < rings && segment < SEGMENTS {
                let a = self.cursor as u32;
                let b = a + SEGMENTS as u32 + 1;
                data.indices
                    .extend_from_slice(&[a, b, b + 1, a, b + 1, a + 1]);
            }
            self.cursor += 1;
        }
        Ok(self.cursor == self.kind.vertices())
    }
    pub fn take_ready(&mut self) -> Option<DiscGeometry> {
        if !self.failed && self.cursor == self.kind.vertices() {
            self.data.take()
        } else {
            None
        }
    }
    pub fn cancel(&mut self) {
        self.data = None;
        self.failed = true;
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn exact_radial_topology_and_seams_survive_different_work_quanta() {
        let frame = TileFrame::new([0., 0., 2.03], [0., 0., 0., 1.]).unwrap();
        let p = NuclearProfile::new(37);
        for kind in [DiscKind::Mask, DiscKind::Bowl, DiscKind::Scorch] {
            let build = |quantum| {
                let mut work = DiscWork::new(&p, kind, frame).unwrap();
                assert!(work.take_ready().is_none());
                loop {
                    let before = work.progress().0;
                    let done = work
                        .step(NonZeroUsize::new(quantum).unwrap(), |_, _| {
                            Ok(GroundSample::BARE)
                        })
                        .unwrap();
                    assert!(work.progress().0 - before <= quantum.min(4096));
                    if done {
                        break;
                    }
                }
                let data = work.take_ready().unwrap();
                assert!(work.take_ready().is_none());
                data
            };
            let a = build(17);
            assert_eq!(a, build(4096));
            assert_eq!(a.positions.len(), kind.vertices());
            assert_eq!(a.indices.len(), kind.indices());
            assert!(kind.allocation_bytes() < 512 * 1024);
            assert!(a.indices.iter().all(|i| (*i as usize) < a.positions.len()));
            for ring in 0..=kind.rings() {
                let first = ring * (SEGMENTS + 1);
                let end = first + SEGMENTS;
                for axis in 0..3 {
                    assert!((a.positions[first][axis] - a.positions[end][axis]).abs() < 1e-6);
                }
            }
            assert!(
                a.planet_directions
                    .iter()
                    .all(|p| (p.iter().map(|v| v * v).sum::<f32>() - 1.).abs() < 1e-6)
            );
            if kind == DiscKind::Mask {
                assert!(a.positions.iter().all(|p| p[2] == 0.003));
            }
            if kind == DiscKind::Scorch {
                assert!(a.apron.contains(&1.));
            } else {
                assert!(a.apron.iter().all(|v| *v == 0.));
            }
        }
    }
    #[test]
    fn failed_or_cancelled_preparation_cannot_publish_partial_geometry() {
        assert!(TileFrame::new([0.; 3], [0., 0., 0., 1.]).is_err());
        assert!(TileFrame::new([0., 0., 2.], [0.; 4]).is_err());
        let p = NuclearProfile::new(1);
        let frame = TileFrame::new([0., 0., 2.03], [0., 0., 0., 1.]).unwrap();
        let mut work = DiscWork::new(&p, DiscKind::Bowl, frame).unwrap();
        assert!(
            work.step(NonZeroUsize::new(10).unwrap(), |_, _| Err(
                GeometryError::InvalidHeight
            ))
            .is_err()
        );
        assert!(work.take_ready().is_none());
        assert_eq!(
            work.step(NonZeroUsize::new(1).unwrap(), |_, _| Ok(GroundSample::BARE)),
            Err(DiscError::Closed)
        );
        let mut work = DiscWork::new(&p, DiscKind::Scorch, frame).unwrap();
        work.cancel();
        assert!(work.take_ready().is_none());
    }
}
