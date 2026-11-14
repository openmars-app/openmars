//! Source MenuMars positioning, without game-state or browser dependencies.
use bevy::prelude::*;
use openmars_scene::view::MenuPlacement;
#[derive(Default)]
pub struct MenuCamera {
    seconds: f64,
    initialized: bool,
    position: Vec3,
    pointer: Vec2,
    spin: Option<f32>,
}
impl MenuCamera {
    pub fn advance(
        &mut self,
        placement: MenuPlacement,
        size: Vec2,
        pointer: Vec2,
        delta: f64,
        reduced: bool,
    ) -> Result<(), String> {
        if !delta.is_finite() || delta < 0. || !size.is_finite() || !pointer.is_finite() {
            return Err("Invalid menu frame".into());
        }
        if !reduced {
            self.seconds += delta.min(0.1);
        }
        let target = position(placement, size);
        let blend = if reduced || !self.initialized {
            1.
        } else {
            1. - (-delta.min(0.05) as f32 * 6.).exp()
        };
        self.position = self.position.lerp(target, blend);
        self.pointer = self.pointer.lerp(
            if reduced {
                Vec2::ZERO
            } else {
                pointer.clamp(Vec2::splat(-0.2), Vec2::splat(0.2))
            },
            blend,
        );
        self.spin = Some(if reduced {
            self.spin.unwrap_or(0.6)
        } else {
            (self.seconds * std::f64::consts::TAU / 300.) as f32
        });
        self.initialized = true;
        Ok(())
    }
    pub fn group(&self) -> Vec3 {
        self.position
    }
    pub fn planet_rotation(&self) -> Quat {
        Quat::from_euler(EulerRot::XYZ, 0.2, self.spin.unwrap_or(0.6), 0.1)
    }
    pub fn moon(&self, reduced: bool) -> Transform {
        let angle = 2.5
            + if reduced {
                0.
            } else {
                (self.seconds * std::f64::consts::TAU / 240.) as f32
            };
        Transform::from_translation(Vec3::new(
            angle.cos() * 4.2,
            angle.sin() * 0.9,
            angle.sin() * 4.2,
        ))
        .with_rotation(Quat::from_euler(EulerRot::XYZ, 0.2, -angle, 0.4))
    }
    pub fn camera(&self, reduced: bool) -> Transform {
        let time = if reduced { 0. } else { self.seconds };
        Transform::from_xyz(
            self.pointer.x + (time / 80.).sin() as f32 * 0.16,
            -self.pointer.y + (time / 110.).sin() as f32 * 0.08,
            10.,
        )
        .looking_at(Vec3::ZERO, Vec3::Y)
    }
}
fn position(placement: MenuPlacement, size: Vec2) -> Vec3 {
    if size.x < 1024. {
        if size.x > size.y {
            Vec3::new(4.6, -1.4, -1.5)
        } else {
            Vec3::new(2.6, -3.2, -1.5)
        }
    } else {
        match placement {
            MenuPlacement::Landing => Vec3::new(3.9, 0., 0.),
            MenuPlacement::Panel => Vec3::new(5.4, -0.8, -0.5),
            MenuPlacement::Lobby => Vec3::new(3.9, -3.7, 1.4),
        }
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn route_and_device_poses_follow_the_source_and_reduced_motion_freezes_orbits() {
        let mut c = MenuCamera::default();
        let desktop = Vec2::new(1280., 720.);
        c.advance(
            MenuPlacement::Landing,
            desktop,
            Vec2::new(0.06, 0.04),
            0.,
            false,
        )
        .unwrap();
        assert_eq!(c.group(), Vec3::new(3.9, 0., 0.));
        c.advance(MenuPlacement::Lobby, desktop, Vec2::ZERO, 0.05, false)
            .unwrap();
        assert!(c.group().y < 0. && c.group().y > -3.7);
        c.advance(
            MenuPlacement::Panel,
            Vec2::new(844., 390.),
            Vec2::ZERO,
            0.05,
            true,
        )
        .unwrap();
        assert_eq!(c.group(), Vec3::new(4.6, -1.4, -1.5));
        let moon = c.moon(true);
        let planet = c.planet_rotation();
        for _ in 0..120 {
            c.advance(
                MenuPlacement::Panel,
                Vec2::new(844., 390.),
                Vec2::ONE,
                1. / 60.,
                true,
            )
            .unwrap();
        }
        assert_eq!(c.moon(true), moon);
        assert_eq!(c.planet_rotation(), planet);
        assert_eq!(c.camera(true).translation, Vec3::new(0., 0., 10.));
        c.advance(
            MenuPlacement::Panel,
            Vec2::new(390., 844.),
            Vec2::ZERO,
            0.,
            true,
        )
        .unwrap();
        assert_eq!(c.group(), Vec3::new(2.6, -3.2, -1.5));
    }
}
