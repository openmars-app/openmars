//! Reference solarSystemConfig/orbitFrame values in the common world units.
//! The lunar/model transforms are shared with picking; no alternate tile sphere.
use bevy::math::{DMat3, DQuat, DVec3};
use openmars_scene::view::{BodyId, FocusTarget};
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct OrbitLimits {
    pub min: f64,
    pub max: f64,
    pub default: f64,
}
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Spherical {
    pub radius: f64,
    pub phi: f64,
    pub theta: f64,
}
impl Spherical {
    pub fn vector(self) -> DVec3 {
        DVec3::new(
            self.radius * self.phi.sin() * self.theta.sin(),
            self.radius * self.phi.cos(),
            self.radius * self.phi.sin() * self.theta.cos(),
        )
    }
    pub fn from_vector(v: DVec3) -> Option<Self> {
        let radius = v.length();
        (v.is_finite() && radius > 0.).then(|| Self {
            radius,
            phi: (v.y / radius).clamp(-1., 1.).acos(),
            theta: v.x.atan2(v.z),
        })
    }
}
#[derive(Clone, Copy, Debug)]
pub struct Atmosphere {
    pub color: [u8; 3],
    pub shadow: [u8; 3],
    pub thickness: f32,
    pub intensity: f32,
    pub halo: f32,
}
#[derive(Clone, Copy, Debug)]
pub struct BodyConfig {
    pub body: BodyId,
    pub radius: f64,
    pub orbit_radius: f64,
    pub angle_deg: f64,
    pub period: f64,
    pub limits: OrbitLimits,
    pub camera_offset: DVec3,
    pub camera: Option<Spherical>,
    pub atmosphere: Atmosphere,
    pub texture: &'static str,
    pub location_offset: i32,
}
const ROCK: Atmosphere = Atmosphere {
    color: [206, 195, 173],
    shadow: [101, 131, 165],
    thickness: 0.055,
    intensity: 1.1,
    halo: 0.9,
};
const fn atmo(color: [u8; 3], shadow: [u8; 3], thickness: f32, intensity: f32) -> Atmosphere {
    Atmosphere {
        color,
        shadow,
        thickness,
        intensity,
        halo: 1.,
    }
}
const fn limits(min: f64, max: f64, default: f64) -> OrbitLimits {
    OrbitLimits { min, max, default }
}
pub const BODIES: [BodyId; 12] = [
    BodyId::Mars,
    BodyId::Phobos,
    BodyId::Mercury,
    BodyId::Venus,
    BodyId::Earth,
    BodyId::Luna,
    BodyId::Ceres,
    BodyId::Jupiter,
    BodyId::Ganymede,
    BodyId::Saturn,
    BodyId::Uranus,
    BodyId::Neptune,
];
pub fn config(body: BodyId) -> BodyConfig {
    use BodyId::*;
    let (
        radius,
        orbit_radius,
        angle_deg,
        period,
        lim,
        offset,
        camera,
        atmosphere,
        texture,
        location_offset,
    ) = match body {
        Mars => (
            2.02,
            250.,
            270.,
            960.,
            limits(2.4, 20., 7.),
            DVec3::ZERO,
            None,
            atmo([239, 210, 163], [75, 140, 181], 0.055, 1.1),
            "mars",
            0,
        ),
        Mercury => (
            1.2,
            80.,
            45.,
            340.,
            limits(2., 15., 6.),
            DVec3::ZERO,
            None,
            ROCK,
            "mercury",
            400,
        ),
        Venus => (
            2.02,
            140.,
            120.,
            550.,
            limits(3., 20., 8.),
            DVec3::ZERO,
            None,
            atmo([244, 215, 160], [133, 123, 160], 0.06, 1.25),
            "venus",
            100,
        ),
        Earth => (
            2.02,
            200.,
            200.,
            700.,
            limits(1.5, 10., 3.5),
            DVec3::new(2.5, 0., 0.),
            Some(Spherical {
                radius: 4.7,
                phi: 1.4184,
                theta: 0.3586,
            }),
            atmo([177, 217, 255], [40, 112, 179], 0.055, 1.1),
            "earth",
            0,
        ),
        Ceres => (
            0.28,
            320.,
            0.,
            1500.,
            limits(0.5, 5., 1.5),
            DVec3::ZERO,
            None,
            ROCK,
            "ceres",
            0,
        ),
        Jupiter => (
            11.3,
            400.,
            330.,
            2400.,
            limits(5., 40., 14.),
            DVec3::new(12., 0., 0.),
            Some(Spherical {
                radius: 8.6,
                phi: 1.5714,
                theta: 0.689,
            }),
            atmo([217, 201, 170], [98, 125, 150], 0.065, 1.5),
            "jupiter",
            0,
        ),
        Saturn => (
            9.5,
            550.,
            15.,
            3800.,
            limits(12., 70., 30.),
            DVec3::ZERO,
            None,
            atmo([230, 213, 172], [113, 135, 149], 0.065, 1.5),
            "saturn",
            0,
        ),
        Uranus => (
            8.1,
            700.,
            160.,
            6400.,
            limits(10., 60., 25.),
            DVec3::ZERO,
            None,
            atmo([144, 213, 220], [52, 120, 148], 0.06, 1.35),
            "uranus",
            0,
        ),
        Neptune => (
            7.9,
            850.,
            80.,
            9000.,
            limits(10., 60., 25.),
            DVec3::ZERO,
            None,
            atmo([113, 158, 232], [35, 79, 154], 0.065, 1.5),
            "neptune",
            0,
        ),
        Luna => (
            0.27,
            200.,
            200.,
            700.,
            limits(0.4, 5., 1.5),
            DVec3::ZERO,
            None,
            ROCK,
            "moon",
            300,
        ),
        Ganymede => (
            0.5,
            400.,
            330.,
            2400.,
            limits(0.7, 8., 2.),
            DVec3::ZERO,
            None,
            ROCK,
            "ganymede",
            200,
        ),
        Phobos => (
            0.35,
            250.,
            270.,
            960.,
            limits(0.5, 5., 1.5),
            DVec3::ZERO,
            None,
            Atmosphere {
                thickness: 0.075,
                halo: 1.2,
                ..ROCK
            },
            "phobos",
            500,
        ),
    };
    BodyConfig {
        body,
        radius,
        orbit_radius,
        angle_deg,
        period,
        limits: lim,
        camera_offset: offset,
        camera,
        atmosphere,
        texture,
        location_offset,
    }
}
pub fn parent(body: BodyId) -> BodyId {
    match body {
        BodyId::Luna => BodyId::Earth,
        BodyId::Ganymede => BodyId::Jupiter,
        BodyId::Phobos => BodyId::Mars,
        _ => body,
    }
}
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct BodyPose {
    pub center: DVec3,
    pub rotation: DQuat,
}
impl BodyPose {
    pub fn local_to_world(self, p: DVec3) -> DVec3 {
        self.center + self.rotation * p
    }
    pub fn world_to_local(self, p: DVec3) -> DVec3 {
        self.rotation.inverse() * (p - self.center)
    }
}
pub fn body_pose(body: BodyId, seconds: f64) -> Result<BodyPose, String> {
    if !seconds.is_finite() || seconds < 0. {
        return Err("Invalid orbital time".into());
    }
    let cfg = config(parent(body));
    let angle = (cfg.angle_deg + seconds.rem_euclid(cfg.period) / cfg.period * 360.).to_radians();
    let center = DVec3::new(
        angle.cos() * cfg.orbit_radius,
        0.,
        angle.sin() * cfg.orbit_radius,
    );
    let rotation = source_rotation(center);
    let offset = match body {
        BodyId::Luna => DVec3::new(3.5, 0.4, 0.8),
        BodyId::Ganymede => DVec3::new(16., 0.5, 1.),
        BodyId::Phobos => DVec3::new(3.2, 0.3, 0.5),
        _ => DVec3::ZERO,
    };
    let center = center + rotation * offset;
    let rotation = if body == BodyId::Phobos {
        let forward = -center.normalize();
        let right = DVec3::Y.cross(forward).normalize();
        let up = forward.cross(right);
        DQuat::from_mat3(&DMat3::from_cols(right, up, forward))
    } else {
        rotation
    };
    Ok(BodyPose { center, rotation })
}
pub fn focus_pose(target: FocusTarget, seconds: f64) -> Result<BodyPose, String> {
    match target {
        FocusTarget::Body(body) => {
            let pose = body_pose(body, seconds)?;
            let center = pose.local_to_world(config(body).camera_offset);
            Ok(BodyPose {
                center,
                rotation: if center.length_squared() > 0. {
                    source_rotation(center)
                } else {
                    DQuat::IDENTITY
                },
            })
        }
        FocusTarget::SolarSystem => {
            if seconds.is_finite() && seconds >= 0. {
                Ok(BodyPose {
                    center: DVec3::ZERO,
                    rotation: DQuat::IDENTITY,
                })
            } else {
                Err("Invalid orbital time".into())
            }
        }
        FocusTarget::OrbitalStation => {
            let center = station_pose(seconds)?.center;
            Ok(BodyPose {
                center,
                rotation: source_rotation(center),
            })
        }
    }
}
/// Actual orbit/tangent shared by the satellite entity and camera focus.
pub fn station_pose(seconds: f64) -> Result<BodyPose, String> {
    let mars = body_pose(BodyId::Mars, seconds)?;
    let angle = seconds * 0.08;
    let offset = |angle: f64| {
        DVec3::new(
            angle.cos() * 3.,
            angle.sin() * 0.3_f64.sin() * 3. * 0.3,
            angle.sin() * 3.,
        )
    };
    let center = mars.center + offset(angle);
    let forward = (offset(angle + 0.1) - offset(angle)).normalize();
    let right = forward.cross(DVec3::Y).normalize();
    let up = right.cross(forward);
    Ok(BodyPose {
        center,
        rotation: DQuat::from_mat3(&DMat3::from_cols(right, up, -forward)),
    })
}
pub fn orbit(target: FocusTarget) -> OrbitLimits {
    match target {
        FocusTarget::Body(body) => config(body).limits,
        FocusTarget::SolarSystem => limits(300., 2500., 1588.),
        FocusTarget::OrbitalStation => limits(0.3, 5., 0.8),
    }
}
pub fn home(target: FocusTarget) -> Spherical {
    match target {
        FocusTarget::Body(body) => config(body).camera.unwrap_or(Spherical {
            radius: orbit(target).default,
            phi: std::f64::consts::FRAC_PI_2,
            theta: 0.,
        }),
        _ => Spherical {
            radius: orbit(target).default,
            phi: if target == FocusTarget::SolarSystem {
                1.281
            } else {
                std::f64::consts::FRAC_PI_2
            },
            theta: 0.,
        },
    }
}

// Exact Three.setFromUnitVectors(Z,toSun), including the chosen antipodal axis.
fn source_rotation(center: DVec3) -> DQuat {
    let n = -center.normalize();
    let r = 1. + n.z;
    if r < f64::EPSILON {
        DQuat::from_xyzw(0., -1., 0., 0.)
    } else {
        DQuat::from_xyzw(-n.y, n.x, 0., r).normalize()
    }
}
/// Express the solar scene in the existing board's Mars-local coordinate frame.
pub fn in_mars_frame(pose: BodyPose, seconds: f64) -> Result<BodyPose, String> {
    let mars = body_pose(BodyId::Mars, seconds)?;
    let inverse = mars.rotation.inverse();
    Ok(BodyPose {
        center: inverse * (pose.center - mars.center),
        rotation: inverse * pose.rotation,
    })
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn the_floating_mars_frame_keeps_the_board_and_sun_fixed_through_the_halfturn() {
        for seconds in [0., 240., 479.999, 480., 480.001, 720., 960., 1e9] {
            let mars = body_pose(BodyId::Mars, seconds).unwrap();
            assert!((mars.rotation * DVec3::Z + mars.center.normalize()).length() < 1e-9);
            assert!((mars.rotation * DVec3::Y - DVec3::Y).length() < 1e-9);
            let local = in_mars_frame(mars, seconds).unwrap();
            assert!(local.center.length() < 1e-9);
            let sun = in_mars_frame(
                BodyPose {
                    center: DVec3::ZERO,
                    rotation: DQuat::IDENTITY,
                },
                seconds,
            )
            .unwrap();
            assert!((sun.center - DVec3::Z * 250.).length() < 1e-8);
            for body in BODIES {
                let pose = body_pose(body, seconds).unwrap();
                let local = in_mars_frame(pose, seconds).unwrap();
                assert!((mars.local_to_world(local.center) - pose.center).length() < 1e-8);
                let point = DVec3::new(0.01, 0.02, 0.03);
                assert!((pose.world_to_local(pose.local_to_world(point)) - point).length() < 1e-8);
            }
        }
    }
    #[test]
    fn source_antipode_and_tilted_focus_preserve_the_original_quaternion_choice() {
        let q = source_rotation(DVec3::Z);
        assert_eq!(q, DQuat::from_xyzw(0., -1., 0., 0.));
        for center in [
            DVec3::new(1., 0.3, 0.5),
            DVec3::new(-1., -0.2, 0.5),
            DVec3::new(0., 0., -1.),
        ] {
            assert!((source_rotation(center) * DVec3::Z + center.normalize()).length() < 1e-9);
        }
        assert!(body_pose(BodyId::Mars, f64::NAN).is_err());
        assert!(body_pose(BodyId::Mars, -1.).is_err());
    }
}
