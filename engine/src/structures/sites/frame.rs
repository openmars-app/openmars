use bevy::math::{DQuat, DVec3};
use openmars_scene::geometry::{AxialPoint, BodyProjection};
#[derive(Clone, Debug)]
pub(super) struct Frame {
    pub origin: [f64; 3],
    pub rotation: [f64; 4],
}
impl Frame {
    pub fn new(
        projection: BodyProjection,
        coordinate: [i32; 3],
        height: f64,
    ) -> Result<Self, String> {
        let point =
            projection.hex_center(AxialPoint::from_cube(coordinate).map_err(|e| e.to_string())?);
        let normal = projection.normal(point).map_err(|e| e.to_string())?;
        let origin = projection
            .project(point, height + 0.01)
            .map_err(|e| e.to_string())?;
        let rotation = DQuat::from_rotation_arc(DVec3::Z, DVec3::from_array(normal)).to_array();
        Ok(Self { origin, rotation })
    }
}
