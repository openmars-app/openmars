//! Validated resolved layout facts. Generation and drawing belong to later owners.
use super::{board_point_wire, bounded_vec};
use crate::geometry::BoardPoint;
use serde::{Deserialize, Serialize};
use std::sync::Arc;

/// Original scalar placement limits bound resolved collection/vertex work.
pub const MAX_STRUCTURE_ITEMS: usize = crate::geometry::placement::MAX_FEATURE_SLOTS;
pub const MAX_STRUCTURE_POINTS: usize = crate::geometry::placement::MAX_OUTPUT_VERTICES;
pub const MAX_STRUCTURE_BYTES: usize = 8 * 1024 * 1024;
#[derive(Clone, Debug, PartialEq)]
pub struct StructureLayoutRef(Arc<StructureLayout>);
impl StructureLayoutRef {
    pub fn new(layout: StructureLayout) -> Result<Self, StructureLayoutError> {
        layout.validate()?;
        Ok(Self(Arc::new(layout)))
    }
    pub fn layout(&self) -> &StructureLayout {
        &self.0
    }
}
impl Serialize for StructureLayoutRef {
    fn serialize<S: serde::Serializer>(&self, s: S) -> Result<S::Ok, S::Error> {
        self.0.as_ref().serialize(s)
    }
}
impl<'de> Deserialize<'de> for StructureLayoutRef {
    fn deserialize<D: serde::Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        Self::new(StructureLayout::deserialize(d)?).map_err(serde::de::Error::custom)
    }
}
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum StructureLayoutError {
    NonFinite,
    InvalidDimension,
    ResourceLimit,
    InvalidFacade,
}
impl std::fmt::Display for StructureLayoutError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{self:?}")
    }
}
impl std::error::Error for StructureLayoutError {}
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct StructureLayout {
    #[serde(deserialize_with = "bounded_text")]
    pub name: String,
    pub seed: u32,
    #[serde(deserialize_with = "items")]
    pub roads: Vec<RoadPath>,
    #[serde(deserialize_with = "entrances")]
    pub entrances: Vec<f64>,
    pub style: StructureStyle,
    #[serde(deserialize_with = "items")]
    pub buildings: Vec<StructureBuilding>,
    #[serde(deserialize_with = "items")]
    pub fixtures: Vec<StructureFixture>,
    #[serde(deserialize_with = "items")]
    pub pipes: Vec<StructurePipe>,
    #[serde(deserialize_with = "plants")]
    pub plants: Vec<StructurePlant>,
    #[serde(deserialize_with = "items")]
    pub parks: Vec<StructurePark>,
}
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct StructureStyle {
    pub plan: StructurePlan,
    pub density: StructureDensity,
    pub heights: StructureHeights,
    pub ground: StructureGround,
    pub exposure: StructureExposure,
    pub perimeter: StructurePerimeter,
    pub cover: StructureCover,
    pub landmark: StructureLandmark,
    pub connections: StructureConnections,
    pub landscaping: StructureLandscaping,
    pub lighting: StructureLighting,
    pub perimeter_road: StructurePerimeterRoad,
    pub entrance_min: i32,
    pub entrance_max: i32,
    #[serde(deserialize_with = "items")]
    pub details: Vec<StructureDetail>,
}
macro_rules! closed_enum {($name:ident{$($variant:ident),*$(,)?})=>{#[derive(Clone,Copy,Debug,PartialEq,Eq,Serialize,Deserialize)]#[serde(rename_all="kebab-case")]pub enum $name{$($variant),*}}}
closed_enum!(StructurePlan {
    Clustered,
    Radial,
    Grid,
    Courtyard
});
closed_enum!(StructureDensity {
    Sparse,
    Balanced,
    Dense
});
closed_enum!(StructureHeights { Low, Mixed, Tall });
closed_enum!(StructureGround { Level, Recessed });
closed_enum!(StructureExposure {
    Surface,
    MostlyBuried
});
closed_enum!(StructurePerimeter {
    Open,
    LowWall,
    HighWall
});
closed_enum!(StructureCover {
    None,
    Dome,
    FlatGlass
});
closed_enum!(StructureLandmark {
    None,
    Hall,
    Observatory,
    Factory,
    Globe,
    Headquarters
});
closed_enum!(StructureConnections {
    Paths,
    Skybridges,
    Enclosed
});
closed_enum!(StructureLandscaping {
    Sparse,
    Parks,
    Lush
});
closed_enum!(StructureLighting { Normal, Bright });
closed_enum!(StructurePerimeterRoad { None, Ring });
closed_enum!(StructureDetail {
    Antennas,
    Dishes,
    Vents,
    Shafts,
    Pipes,
    Smokestacks,
    Tanks,
    GroundExhausts,
    AccentLights,
    Gatehouses
});
closed_enum!(RoadKind { Road, Path });
closed_enum!(StructureFixtureKind {
    Smokestack,
    Tank,
    GroundExhaust,
    Shaft,
    AccentLight
});
closed_enum!(FixtureAccent { Cyan, Amber });
closed_enum!(FixtureEmitterKind { Smoke, Vapor });
closed_enum!(StructurePlantKind { Tree, Bush });
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct RoadPath {
    #[serde(with = "road_points")]
    pub points: Vec<BoardPoint>,
    pub width: f64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub end_width: Option<f64>,
    pub kind: RoadKind,
}
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct StructureBuilding {
    pub x: f64,
    pub y: f64,
    pub z: f64,
    pub width: f64,
    pub depth: f64,
    pub height: f64,
    pub stepped: bool,
    pub facade: u8,
    #[serde(deserialize_with = "bounded_text")]
    pub tint: String,
    pub landmark: bool,
    pub round: bool,
    pub habitat: bool,
    pub rotation: f64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub entrance_road: Option<EntranceRoad>,
}
#[derive(Clone, Copy, Debug, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct EntranceRoad {
    pub x: f64,
    pub y: f64,
    pub half_width: f64,
}
#[derive(Clone, Copy, Debug, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct StructureFixture {
    pub kind: StructureFixtureKind,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub rotation: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub accent: Option<FixtureAccent>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub emitter: Option<FixtureEmitter>,
    pub x: f64,
    pub y: f64,
    pub z: f64,
    pub radius: f64,
    pub height: f64,
}
#[derive(Clone, Copy, Debug, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct FixtureEmitter {
    pub kind: FixtureEmitterKind,
    pub x: f64,
    pub y: f64,
    pub z: f64,
}
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct StructurePipe {
    #[serde(deserialize_with = "points3")]
    pub points: Vec<StructurePoint3>,
    pub radius: f64,
}
#[derive(Clone, Copy, Debug, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct StructurePoint3 {
    pub x: f64,
    pub y: f64,
    pub z: f64,
}
#[derive(Clone, Copy, Debug, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct StructurePlant {
    pub x: f64,
    pub y: f64,
    pub z: f64,
    pub scale: f64,
    pub kind: StructurePlantKind,
}
#[derive(Clone, Copy, Debug, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct StructurePark {
    pub x: f64,
    pub y: f64,
    pub radius: f64,
    pub z: f64,
}

impl StructureLayout {
    /// Account for retained capacities, including nested geometry and strings.
    pub fn storage_bytes(&self) -> usize {
        use std::mem::size_of;
        size_of::<Self>()
            + self.name.capacity()
            + self.roads.capacity() * size_of::<RoadPath>()
            + self
                .roads
                .iter()
                .map(|r| r.points.capacity() * size_of::<BoardPoint>())
                .sum::<usize>()
            + self.entrances.capacity() * size_of::<f64>()
            + self.style.details.capacity() * size_of::<StructureDetail>()
            + self.buildings.capacity() * size_of::<StructureBuilding>()
            + self
                .buildings
                .iter()
                .map(|b| b.tint.capacity())
                .sum::<usize>()
            + self.fixtures.capacity() * size_of::<StructureFixture>()
            + self.pipes.capacity() * size_of::<StructurePipe>()
            + self
                .pipes
                .iter()
                .map(|p| p.points.capacity() * size_of::<StructurePoint3>())
                .sum::<usize>()
            + self.plants.capacity() * size_of::<StructurePlant>()
            + self.parks.capacity() * size_of::<StructurePark>()
    }
    pub fn validate(&self) -> Result<(), StructureLayoutError> {
        use StructureLayoutError::*;
        let finite = |v: &[f64]| {
            if v.iter().all(|v| v.is_finite()) {
                Ok(())
            } else {
                Err(NonFinite)
            }
        };
        let positive = |v: &[f64]| {
            finite(v)?;
            if v.iter().all(|v| *v > 0.) {
                Ok(())
            } else {
                Err(InvalidDimension)
            }
        };
        if self.name.len() > 128
            || self.roads.len() > MAX_STRUCTURE_ITEMS
            || self.buildings.len() > MAX_STRUCTURE_ITEMS
            || self.fixtures.len() > MAX_STRUCTURE_ITEMS
            || self.pipes.len() > MAX_STRUCTURE_ITEMS
            || self.parks.len() > MAX_STRUCTURE_ITEMS
            || self.style.details.len() > MAX_STRUCTURE_ITEMS
            || self.plants.len() > 140
            || self.entrances.is_empty()
            || self.entrances.len() > 4
        {
            return Err(ResourceLimit);
        }
        finite(&self.entrances)?;
        let mut points = self.entrances.len();
        for road in &self.roads {
            if road.points.len() < 2 || road.points.len() > MAX_STRUCTURE_POINTS {
                return Err(ResourceLimit);
            }
            positive(&[road.width])?;
            if let Some(width) = road.end_width {
                positive(&[width])?
            }
            for p in &road.points {
                finite(&[p.x, p.y])?
            }
            points = points.checked_add(road.points.len()).ok_or(ResourceLimit)?;
        }
        for b in &self.buildings {
            finite(&[b.x, b.y, b.z, b.rotation])?;
            positive(&[b.width, b.depth, b.height])?;
            if b.tint.len() > 128 {
                return Err(ResourceLimit);
            }
            if b.facade > 2 {
                return Err(InvalidFacade);
            }
            if let Some(r) = b.entrance_road {
                finite(&[r.x, r.y, r.half_width])?;
                if r.half_width < 0. {
                    return Err(InvalidDimension);
                }
            }
        }
        for f in &self.fixtures {
            finite(&[f.x, f.y, f.z])?;
            positive(&[f.radius, f.height])?;
            if let Some(rotation) = f.rotation {
                finite(&[rotation])?
            }
            if let Some(e) = f.emitter {
                finite(&[e.x, e.y, e.z])?
            }
        }
        for pipe in &self.pipes {
            positive(&[pipe.radius])?;
            if pipe.points.len() < 2 || pipe.points.len() > MAX_STRUCTURE_POINTS {
                return Err(ResourceLimit);
            }
            for p in &pipe.points {
                finite(&[p.x, p.y, p.z])?
            }
            points = points.checked_add(pipe.points.len()).ok_or(ResourceLimit)?;
        }
        for p in &self.plants {
            finite(&[p.x, p.y, p.z])?;
            positive(&[p.scale])?
        }
        for p in &self.parks {
            finite(&[p.x, p.y, p.z])?;
            positive(&[p.radius])?
        }
        if points > MAX_STRUCTURE_POINTS
            || serde_json::to_vec(self).map_err(|_| NonFinite)?.len() > MAX_STRUCTURE_BYTES
        {
            return Err(ResourceLimit);
        }
        Ok(())
    }
}
fn bounded_text<'de, D: serde::Deserializer<'de>>(d: D) -> Result<String, D::Error> {
    let s = String::deserialize(d)?;
    if s.len() <= 128 {
        Ok(s)
    } else {
        Err(serde::de::Error::custom("structure-text-limit"))
    }
}
fn items<'de, D: serde::Deserializer<'de>, T: Deserialize<'de>>(d: D) -> Result<Vec<T>, D::Error> {
    bounded_vec::<D, T, MAX_STRUCTURE_ITEMS>(d)
}
fn entrances<'de, D: serde::Deserializer<'de>>(d: D) -> Result<Vec<f64>, D::Error> {
    bounded_vec::<D, f64, 4>(d)
}
fn plants<'de, D: serde::Deserializer<'de>>(d: D) -> Result<Vec<StructurePlant>, D::Error> {
    bounded_vec::<D, StructurePlant, 140>(d)
}
fn points3<'de, D: serde::Deserializer<'de>>(d: D) -> Result<Vec<StructurePoint3>, D::Error> {
    bounded_vec::<D, StructurePoint3, MAX_STRUCTURE_POINTS>(d)
}
mod road_points {
    use super::*;
    #[derive(Serialize, Deserialize)]
    struct Point(#[serde(with = "board_point_wire")] BoardPoint);
    pub fn serialize<S: serde::Serializer>(points: &[BoardPoint], s: S) -> Result<S::Ok, S::Error> {
        use serde::ser::SerializeSeq;
        let mut seq = s.serialize_seq(Some(points.len()))?;
        for p in points {
            seq.serialize_element(&Point(*p))?
        }
        seq.end()
    }
    pub fn deserialize<'de, D: serde::Deserializer<'de>>(
        d: D,
    ) -> Result<Vec<BoardPoint>, D::Error> {
        Ok(bounded_vec::<D, Point, MAX_STRUCTURE_POINTS>(d)?
            .into_iter()
            .map(|p| p.0)
            .collect())
    }
}
