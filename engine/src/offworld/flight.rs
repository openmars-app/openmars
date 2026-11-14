//! Original SpaceshipRenderer parking and complete flight path in body-local
//! units. The canonical60Hz clock replaces wall time; no rule state is inferred.
use bevy::math::{DMat3, DQuat, DVec3};

pub const MAX_SHIPS: usize = 1024;
pub const LIFTOFF: u64 = 72;
pub const ARC_END: u64 = 792;
pub const CRUISE_END: u64 = 1152;
pub const COMPLETE: u64 = 1332;
pub const LANDING_EXHAUST: u64 = 1135;
pub const HOVER_HEIGHT: f64 = 0.15;
fn error(text: &str) -> String {
    text.into()
}
/// A caller supplies the actual predicted destination in the departure frame.
#[derive(Clone, Copy, Debug)]
pub struct Destination {
    pub position: DVec3,
    pub normal: DVec3,
    pub center: DVec3,
}
impl Destination {
    pub fn landing(self) -> Result<(DVec3, DVec3), String> {
        if !self.position.is_finite()
            || !self.normal.is_finite()
            || !self.center.is_finite()
            || (self.normal.length_squared() - 1.).abs() > 1e-8
        {
            return Err(error("Invalid flight destination"));
        }
        Ok((self.position, self.normal))
    }
}
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Pose {
    pub position: DVec3,
    pub rotation: DQuat,
}
impl Pose {
    pub fn transform(self) -> bevy::prelude::Transform {
        bevy::prelude::Transform::from_translation(self.position.as_vec3())
            .with_rotation(self.rotation.as_quat())
    }
    pub fn forward(self) -> DVec3 {
        self.rotation * -DVec3::Z
    }
}
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Parking {
    pub ship: Pose,
    pub pad: Pose,
    pub normal: DVec3,
}
fn sphere(theta: f64, phi: f64, radius: f64) -> DVec3 {
    DVec3::new(
        radius * phi.sin() * theta.cos(),
        radius * phi.sin() * theta.sin(),
        radius * phi.cos(),
    )
}
fn orientation(forward: DVec3, up: DVec3) -> Result<DQuat, String> {
    if !forward.is_finite() || !up.is_finite() || forward.length_squared() < 1e-20 {
        return Err(error("ship orientation inputs"));
    }
    let z = -forward.normalize();
    let x = up.cross(z).normalize_or_zero();
    let y = z.cross(x).normalize_or_zero();
    if x.length_squared() < 0.5 || y.length_squared() < 0.5 {
        return Err(error("ship orientation singular"));
    }
    Ok(DQuat::from_mat3(&DMat3::from_cols(x, y, z)).normalize())
}
pub fn parking(count: usize, radius: f64) -> Result<Vec<Parking>, String> {
    if count > MAX_SHIPS || !radius.is_finite() || radius <= 0. {
        return Err(error("ship parking bounds"));
    }
    let center = sphere(-0.55, 0.82, radius);
    let to_board = (sphere(0., std::f64::consts::FRAC_PI_2, radius) - center).normalize();
    let offset = count.saturating_sub(1) as f64 * 0.17 / 2.;
    (0..count)
        .map(|index| {
            let t = index as f64 * 0.17 - offset;
            let position = sphere(
                -0.55 + t * 0.15_f64.cos(),
                0.82 + t * 0.15_f64.sin(),
                radius,
            );
            let normal = position.normalize();
            let projected = (to_board - normal * to_board.dot(normal)).normalize();
            Ok(Parking {
                ship: Pose {
                    position: position + normal * 0.015,
                    rotation: orientation(-projected, normal)?,
                },
                pad: Pose {
                    position,
                    rotation: DQuat::from_rotation_arc(DVec3::Z, normal),
                },
                normal,
            })
        })
        .collect()
}
#[derive(Clone, Debug)]
pub struct FlightPath {
    pub parked: Pose,
    pub start_normal: DVec3,
    pub liftoff_end: DVec3,
    pub arc: [DVec3; 4],
    pub cruise: [DVec3; 2],
    pub landed: Pose,
    pub landing_normal: DVec3,
}
impl FlightPath {
    pub fn new(parked: Pose, destination: Destination) -> Result<Self, String> {
        if !parked.position.is_finite()
            || parked.position.length_squared() < 1e-20
            || !parked.rotation.is_finite()
            || (parked.rotation.length_squared() - 1.).abs() > 1e-8
        {
            return Err(error("ship flight origin"));
        }
        let start_normal = parked.position.normalize();
        let liftoff_end = parked.position + start_normal * HOVER_HEIGHT;
        let (landing, landing_normal) = destination.landing()?;
        let center = destination.center;
        let approach = landing + landing_normal;
        let p1 = liftoff_end + parked.forward().normalize() * 17. + start_normal * 8.;
        let to_venus = (center - liftoff_end).normalize();
        let midpoint = liftoff_end.lerp(approach, 0.5);
        let p2 = midpoint + midpoint.normalize() * 6. + to_venus * 10.;
        let p3 = liftoff_end.lerp(approach, 0.6);
        Ok(Self {
            parked,
            start_normal,
            liftoff_end,
            arc: [liftoff_end, p1, p2, p3],
            cruise: [p3, approach],
            landed: Pose {
                position: landing,
                rotation: orientation(-landing_normal, DVec3::Y)?,
            },
            landing_normal,
        })
    }
    pub fn sample(&self, step: u64) -> Result<Pose, String> {
        if step < LIFTOFF {
            let t = (step as f64 / LIFTOFF as f64).powi(2);
            return Ok(Pose {
                position: self.parked.position + self.start_normal * (HOVER_HEIGHT * t),
                rotation: self.parked.rotation,
            });
        }
        if step < ARC_END {
            let raw = (step - LIFTOFF) as f64 / (ARC_END - LIFTOFF) as f64;
            let t = raw.powf(1.5);
            let u = 1. - t;
            let [p0, p1, p2, p3] = self.arc;
            let position =
                p0 * u.powi(3) + p1 * (3. * u * u * t) + p2 * (3. * u * t * t) + p3 * t.powi(3);
            let tangent =
                ((p1 - p0) * (3. * u * u) + (p2 - p1) * (6. * u * t) + (p3 - p2) * (3. * t * t))
                    .normalize();
            let outward = position.normalize();
            let bank_up = (outward + tangent.cross(outward).normalize() * 0.6).normalize();
            let blend = (raw / 0.4).min(1.);
            let blend = blend * blend * (3. - 2. * blend);
            return Ok(Pose {
                position,
                rotation: self
                    .parked
                    .rotation
                    .slerp(orientation(tangent, bank_up)?, blend),
            });
        }
        let cruise_direction = (self.cruise[1] - self.cruise[0]).normalize();
        let cruise_rotation = orientation(cruise_direction, DVec3::Y)?;
        if step < CRUISE_END {
            let t = (step - ARC_END) as f64 / (CRUISE_END - ARC_END) as f64;
            let t = if t < 0.5 {
                2. * t * t
            } else {
                1. - (-2. * t + 2.).powi(2) / 2.
            };
            return Ok(Pose {
                position: self.cruise[0].lerp(self.cruise[1], t),
                rotation: cruise_rotation,
            });
        }
        if step < COMPLETE {
            let t = ((step - CRUISE_END) as f64 / (COMPLETE - CRUISE_END) as f64).powi(3);
            return Ok(Pose {
                position: self.cruise[1].lerp(self.landed.position, t),
                rotation: cruise_rotation.slerp(self.landed.rotation, t),
            });
        }
        Ok(self.landed)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn complete_paths_keep_all_phase_boundaries_and_source_parking() {
        assert!(parking(MAX_SHIPS + 1, 2.02).is_err());
        assert_eq!(parking(MAX_SHIPS, 2.02).unwrap().len(), MAX_SHIPS);
        assert!(parking(1, f64::NAN).is_err());
        assert!(parking(0, 2.02).unwrap().is_empty());
        for count in [1, 2, 5, 8] {
            for park in parking(count, 2.02).unwrap() {
                assert!((park.pad.position.length() - 2.02).abs() < 1e-12);
                assert!((park.ship.position.length() - 2.035).abs() < 1e-12);
                assert!((park.pad.rotation * DVec3::Z).abs_diff_eq(park.normal, 1e-12));
                for destination in [
                    Destination {
                        position: DVec3::new(-45., 25., -148.)
                            - DVec3::new(-45., 25., -148.).normalize() * 2.02,
                        normal: -DVec3::new(-45., 25., -148.).normalize(),
                        center: DVec3::new(-45., 25., -148.),
                    },
                    Destination {
                        position: DVec3::new(50., 80., -200.),
                        normal: DVec3::new(50., 80., -200.).normalize(),
                        center: DVec3::new(50., 80., -200.)
                            - DVec3::new(50., 80., -200.).normalize() * 2.02,
                    },
                ] {
                    let path = FlightPath::new(park.ship, destination).unwrap();
                    assert_eq!(path.sample(0).unwrap(), park.ship);
                    assert!(
                        path.sample(LIFTOFF)
                            .unwrap()
                            .position
                            .abs_diff_eq(path.liftoff_end, 1e-12)
                    );
                    assert!(
                        path.sample(ARC_END)
                            .unwrap()
                            .position
                            .abs_diff_eq(path.arc[3], 1e-12)
                    );
                    assert!(
                        path.sample(CRUISE_END)
                            .unwrap()
                            .position
                            .abs_diff_eq(path.cruise[1], 1e-12)
                    );
                    assert_eq!(path.sample(u64::MAX).unwrap(), path.landed);
                    for step in 0..=COMPLETE {
                        let pose = path.sample(step).unwrap();
                        assert!(pose.position.is_finite());
                        assert!((pose.rotation.length_squared() - 1.).abs() < 1e-12);
                        assert_eq!(
                            pose,
                            path.sample(step).unwrap(),
                            "seek/pause has no integration state"
                        );
                    }
                }
            }
        }
    }
}
