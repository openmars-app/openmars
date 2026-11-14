//! Deterministic presentation layouts ported from the sibling renderer.
use crate::geometry::BoardPoint;
pub mod layout;
mod types;
pub use types::*;
// Placement-time visual requests, independent of placement/game rules.
use serde::{Deserialize, Serialize};

/// Best-effort authored style requests retain their original tokens and order.
#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
#[cfg_attr(feature = "export-types", derive(ts_rs::TS))]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct StructureStyleRequest {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    #[cfg_attr(feature = "export-types", ts(optional))]
    pub plan: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    #[cfg_attr(feature = "export-types", ts(optional))]
    pub density: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    #[cfg_attr(feature = "export-types", ts(optional))]
    pub heights: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    #[cfg_attr(feature = "export-types", ts(optional))]
    pub ground: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    #[cfg_attr(feature = "export-types", ts(optional))]
    pub exposure: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    #[cfg_attr(feature = "export-types", ts(optional))]
    pub perimeter: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    #[cfg_attr(feature = "export-types", ts(optional))]
    pub cover: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    #[cfg_attr(feature = "export-types", ts(optional))]
    pub landmark: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    #[cfg_attr(feature = "export-types", ts(optional))]
    pub connections: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    #[cfg_attr(feature = "export-types", ts(optional))]
    pub landscaping: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    #[cfg_attr(feature = "export-types", ts(optional))]
    pub lighting: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    #[cfg_attr(feature = "export-types", ts(optional))]
    pub perimeter_road: Option<String>,
    #[serde(default)]
    pub entrance_min: i32,
    #[serde(default)]
    pub entrance_max: i32,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub details: Vec<String>,
}

impl StructureStyleRequest {
    pub fn is_bounded(&self) -> bool {
        [
            &self.plan,
            &self.density,
            &self.heights,
            &self.ground,
            &self.exposure,
            &self.perimeter,
            &self.cover,
            &self.landmark,
            &self.connections,
            &self.landscaping,
            &self.lighting,
            &self.perimeter_road,
        ]
        .into_iter()
        .flatten()
        .all(|s| s.len() <= 64)
            && self.details.len() <= 32
            && self.details.iter().all(|s| s.len() <= 64)
            && self.entrance_min.unsigned_abs() <= 4096
            && self.entrance_max.unsigned_abs() <= 4096
    }
}

mod board_point_wire {
    use super::*;
    #[derive(Serialize, Deserialize)]
    #[serde(remote = "BoardPoint", deny_unknown_fields)]
    struct Wire {
        #[serde(deserialize_with = "finite_f64")]
        x: f64,
        #[serde(deserialize_with = "finite_f64")]
        y: f64,
    }
    fn finite_f64<'de, D: serde::Deserializer<'de>>(d: D) -> Result<f64, D::Error> {
        let n = f64::deserialize(d)?;
        if n.is_finite() {
            Ok(n)
        } else {
            Err(serde::de::Error::custom("nonfinite-board-point"))
        }
    }
    pub fn serialize<S: serde::Serializer>(p: &BoardPoint, s: S) -> Result<S::Ok, S::Error> {
        if !p.x.is_finite() || !p.y.is_finite() {
            return Err(serde::ser::Error::custom("nonfinite-board-point"));
        }
        Wire::serialize(p, s)
    }
    pub fn deserialize<'de, D: serde::Deserializer<'de>>(d: D) -> Result<BoardPoint, D::Error> {
        Wire::deserialize(d)
    }
}
fn bounded_vec<'de, D: serde::Deserializer<'de>, T: Deserialize<'de>, const MAX: usize>(
    d: D,
) -> Result<Vec<T>, D::Error> {
    struct Visitor<T, const MAX: usize>(std::marker::PhantomData<T>);
    impl<'de, T: Deserialize<'de>, const MAX: usize> serde::de::Visitor<'de> for Visitor<T, MAX> {
        type Value = Vec<T>;
        fn expecting(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
            write!(f, "at most{MAX} elements")
        }
        fn visit_seq<A: serde::de::SeqAccess<'de>>(
            self,
            mut a: A,
        ) -> Result<Self::Value, A::Error> {
            let mut out = Vec::new();
            while let Some(v) = a.next_element()? {
                if out.len() == MAX {
                    return Err(serde::de::Error::custom("layout-resource-limit"));
                }
                out.push(v)
            }
            Ok(out)
        }
    }
    d.deserialize_seq(Visitor::<T, MAX>(std::marker::PhantomData))
}
