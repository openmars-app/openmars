//! Original seeded orbits and raw PNG-byte noise sampling.
use bevy::{math::DVec3, prelude::*};
use openmars_scene::geometry::placement::random_sequence;
#[derive(Clone, Copy, Debug, PartialEq)]
pub(super) struct Flight {
    radius: f64,
    radius_y: f64,
    speed: f64,
    phase: f64,
    height: f64,
    noise_u: f64,
    noise_v: f64,
    bob_phase: f64,
    bob_speed: f64,
    pub(super) animation_speed: f64,
    pub(super) animation_start: f64,
}
pub(super) fn flights(key: (i32, i32, i32), duration: f64) -> Vec<Flight> {
    // JS bitwise operators coerce the coordinate products to32 bits. The
    // board's validated small coordinates also make the original abs exact.
    let seed = (i64::from(key.0) * 73856093).unsigned_abs() as u32
        ^ key.1.wrapping_mul(19349663) as u32
        ^ key.2.wrapping_mul(83492791) as u32;
    let mut rng = random_sequence(seed.wrapping_add(55555));
    let count = 5 + (rng() * 2.).floor() as usize;
    (0..count)
        .map(|_| Flight {
            radius: 0.04 + rng() * 0.06,
            radius_y: 0.03 + rng() * 0.05,
            speed: 0.6 + rng() * 0.8,
            phase: rng() * std::f64::consts::TAU,
            height: 0.04 + rng() * 0.06,
            noise_u: rng(),
            noise_v: rng(),
            bob_phase: rng() * std::f64::consts::TAU,
            bob_speed: 0.4 + rng() * 0.6,
            animation_speed: 3.2 + rng() * 1.6,
            animation_start: rng() * duration,
        })
        .collect()
}
pub(super) fn sample_noise(p: &[u8], w: u32, h: u32, u: f64, v: f64) -> f64 {
    let (x, y) = (
        u.rem_euclid(1.) * f64::from(w - 1),
        v.rem_euclid(1.) * f64::from(h - 1),
    );
    let (x0, y0) = (x.floor() as u32, y.floor() as u32);
    let at = |x: u32, y: u32| f64::from(p[(y * w + x) as usize * 4]);
    let (x1, y1) = ((x0 + 1) % w, (y0 + 1) % h);
    let (dx, dy) = (x - f64::from(x0), y - f64::from(y0));
    let top = at(x0, y0) + (at(x1, y0) - at(x0, y0)) * dx;
    let bottom = at(x0, y1) + (at(x1, y1) - at(x0, y1)) * dx;
    (top + (bottom - top) * dy) / 255.
}
pub(super) fn flight_transform(
    f: Flight,
    center: DVec3,
    t: f64,
    noise: impl Fn(f64, f64) -> f64,
) -> Transform {
    let normal = center.normalize();
    let axis = if normal.y.abs() > 0.9 {
        DVec3::X
    } else {
        DVec3::Y
    };
    let tx = normal.cross(axis).normalize();
    let ty = normal.cross(tx).normalize();
    let angle = t * f.speed + f.phase;
    let nx = (noise(f.noise_u + t * 0.03, f.noise_v) - 0.5) * 2.;
    let ny = (noise(f.noise_u, f.noise_v + t * 0.021) - 0.5) * 2.;
    let nh = (noise(f.noise_u + 0.5, f.noise_v + t * 0.015) - 0.5) * 2.;
    let p = center
        + normal * (f.height + nh * 0.008 + (t * f.bob_speed + f.bob_phase).sin() * 0.008)
        + tx * (angle.cos() * f.radius + nx * 0.025)
        + ty * (angle.sin() * f.radius_y + ny * 0.025);
    let forward = (tx * (-angle.sin() * f.speed * f.radius)
        + ty * (angle.cos() * f.speed * f.radius_y))
        .normalize();
    let right = normal.cross(forward).normalize();
    let up = forward.cross(right).normalize();
    Transform {
        translation: p.as_vec3(),
        rotation: Quat::from_mat3(&Mat3::from_cols(
            right.as_vec3(),
            up.as_vec3(),
            forward.as_vec3(),
        )),
        scale: Vec3::splat(0.0015),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn noise_uses_top_first_red_bytes_with_periodic_wrapping() {
        let rgba = [
            0, 255, 255, 255, 64, 0, 0, 255, 128, 0, 0, 255, 255, 0, 0, 255,
        ];
        assert_eq!(sample_noise(&rgba, 2, 2, 0., 0.), 0.);
        let mid = sample_noise(&rgba, 2, 2, 0.5, 0.5);
        assert!((mid - 111.75 / 255.).abs() < 1e-14);
        assert_eq!(mid, sample_noise(&rgba, 2, 2, -0.5, 1.5));
        assert!(sample_noise(&rgba, 2, 2, 0., 0.75) > sample_noise(&rgba, 2, 2, 0., 0.25));
    }
    #[test]
    fn orbit_has_finite_orientation_at_poles_and_same_clock_freezes() {
        let f = flights((0, 0, 0), 5.)[0];
        for center in [DVec3::Z * 2.02, DVec3::Y * 2.02, DVec3::NEG_Y * 2.02] {
            let pose = flight_transform(f, center, 10., |_, _| 0.5);
            assert!(pose.to_matrix().is_finite());
            assert!((pose.rotation.length() - 1.).abs() < 1e-6);
            assert_eq!(pose.scale, Vec3::splat(0.0015));
            assert!(pose.translation.as_dvec3().distance(center) < 0.2);
            assert_eq!(pose, flight_transform(f, center, 10., |_, _| 0.5));
            assert_ne!(pose, flight_transform(f, center, 11., |_, _| 0.5));
        }
    }
}
