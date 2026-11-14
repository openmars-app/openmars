//! Exact Object3D.lookAt followed by the source angular clamp, not a screen billboard.
use crate::space::config::BodyPose;
use bevy::math::{DMat3, DQuat, DVec3};

#[derive(Clone, Copy, Debug)]
pub struct Pose {
    pub position: DVec3,
    pub rotation: DQuat,
    pub scale: f64,
    pub opacity: f64,
}
/// `surface` and `normal` are the tile's original sphere anchors, before relief.
/// The old tile root lifts .01 and its child label lifts .02 in the scaled frame.
pub fn at(
    surface: DVec3,
    normal: DVec3,
    body: BodyPose,
    eye: DVec3,
    entrance: f64,
) -> Option<Pose> {
    if !surface.is_finite()
        || !normal.is_finite()
        || normal.length_squared() < 1e-20
        || !body.center.is_finite()
        || !body.rotation.is_finite()
        || !eye.is_finite()
        || !entrance.is_finite()
        || !(0. ..=1.).contains(&entrance)
    {
        return None;
    }
    let normal = normal.normalize();
    // Match Quaternion.setFromUnitVectors, including its antipodal axis.
    let r = 1. + normal.z;
    let tangent = if r < f64::EPSILON {
        DQuat::from_xyzw(0., -1., 0., 0.)
    } else {
        DQuat::from_xyzw(-normal.y, normal.x, 0., r).normalize()
    };
    let parent = body.rotation * tangent;
    // Preserve the original parent-matrix composition at polar anchors. An
    // algebraically simplified normal*.02 changes lookAt's singular up axis.
    let position = body.center
        + body.rotation * (surface + normal * 0.01)
        + (DMat3::from_quat(body.rotation) * DMat3::from_quat(tangent))
            * (DVec3::Z * (0.02 * entrance));
    let radial = (position - body.center).normalize_or_zero();
    let direction = (eye - position).normalize_or_zero();
    let dot = radial.dot(direction);
    let t = ((dot + 0.15) / 0.3).clamp(0., 1.);
    let opacity = t * t * (3. - 2. * t);
    // THREE.Matrix4.lookAt(target, position, object.up) for a non-camera object.
    let mut z = eye - position;
    if z.length_squared() == 0. {
        z.z = 1.;
    }
    z = z.normalize();
    let mut x = DVec3::Y.cross(z);
    if x.length_squared() == 0. {
        z.z += 0.0001;
        z = z.normalize();
        x = DVec3::Y.cross(z);
    }
    x = x.normalize();
    let y = z.cross(x);
    let billboard = parent.inverse() * DQuat::from_mat3(&DMat3::from_cols(x, y, z));
    let angle = 2. * billboard.w.abs().min(1.).acos();
    let local = if angle > 0.001 {
        DQuat::IDENTITY.slerp(billboard, 0.175 * (angle / 0.175).atan() / angle)
    } else {
        billboard
    };
    Some(Pose {
        position,
        rotation: parent * local,
        scale: entrance,
        opacity,
    })
}
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn bad_inputs_do_not_publish_nonfinite_geometry_and_clamp_is_not_temporal() {
        let b = BodyPose {
            center: DVec3::ZERO,
            rotation: DQuat::IDENTITY,
        };
        assert!(at(DVec3::Z, DVec3::ZERO, b, DVec3::Z * 6., 1.).is_none());
        assert!(at(DVec3::Z, DVec3::Z, b, DVec3::splat(f64::NAN), 1.).is_none());
        assert!(at(DVec3::Z, DVec3::Z, b, DVec3::Z * 6., -1.).is_none());
        let a = at(DVec3::Z * 2.02, DVec3::Z, b, DVec3::new(3., 1., 6.), 1.).unwrap();
        let again = at(DVec3::Z * 2.02, DVec3::Z, b, DVec3::new(3., 1., 6.), 1.).unwrap();
        assert_eq!(a.rotation, again.rotation);
        let tilt = 2. * a.rotation.w.abs().acos();
        assert!(tilt > 0. && tilt < 0.175 * std::f64::consts::FRAC_PI_2);
    }
}
