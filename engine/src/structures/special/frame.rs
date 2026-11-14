use bevy::math::{DQuat, DVec3};
use openmars_scene::geometry::{AxialPoint, BodyProjection};
#[derive(Clone, Debug)]
pub(crate) struct Frame {
    pub origin: [f64; 3],
    pub normal: [f64; 3],
    pub rotation: [f64; 4],
}
impl Frame {
    pub fn new(projection: BodyProjection, coordinate: [i32; 3]) -> Result<Self, String> {
        let point =
            projection.hex_center(AxialPoint::from_cube(coordinate).map_err(|e| e.to_string())?);
        let normal = projection.normal(point).map_err(|e| e.to_string())?;
        let origin = normal.map(|v| v * (projection.radius() + 0.01));
        let rotation = DQuat::from_rotation_arc(DVec3::Z, DVec3::from_array(normal)).to_array();
        Ok(Self {
            origin,
            normal,
            rotation,
        })
    }
    pub fn direction(&self, x: f64, y: f64) -> [f64; 3] {
        (DVec3::from_array(self.origin) + DQuat::from_array(self.rotation) * DVec3::new(x, y, 0.))
            .normalize()
            .to_array()
    }
}
