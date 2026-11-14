//! Source-authored fixture phases and tangent motion; no game rules or transport.
use crate::structures::special::frame::Frame;
use bevy::math::{DQuat, DVec3};
use openmars_scene::{
    geometry::{AxialPoint, BoardPoint, BodyProjection, placement::random_sequence},
    structure::{FixtureEmitterKind, StructureLayoutRef},
};
pub(super) struct Outlet {
    pub vapor: bool,
    pub local: DVec3,
    pub phases: Vec<f64>,
}
#[derive(Clone, Copy, Debug)]
pub(super) struct Pose {
    pub position: DVec3,
    pub scale: f64,
    pub opacity: f64,
    pub rotation: f64,
}
pub(super) fn prepare(
    layout: &StructureLayoutRef,
    coordinate: [i32; 3],
    frame: &Frame,
    projection: BodyProjection,
) -> Result<Vec<Outlet>, String> {
    let center =
        projection.hex_center(AxialPoint::from_cube(coordinate).map_err(|e| e.to_string())?);
    let inverse = DQuat::from_array(frame.rotation).conjugate();
    let origin = DVec3::from_array(frame.origin);
    let mut random = random_sequence(layout.layout().seed ^ 0x88351);
    let mut count = 0;
    let mut out = Vec::new();
    for fixture in &layout.layout().fixtures {
        let Some(e) = fixture.emitter else { continue };
        let vapor = e.kind == FixtureEmitterKind::Vapor;
        let n = if vapor { 5 } else { 8 };
        count += n;
        if count > 8192 {
            return Err("Fixture particle working-set bound".into());
        }
        let position = projection
            .project_content(
                BoardPoint {
                    x: center.x + e.x,
                    y: center.y + e.y,
                },
                e.z - 0.007,
            )
            .map_err(|e| e.to_string())?;
        out.push(Outlet {
            vapor,
            local: inverse * (DVec3::from_array(position) - origin),
            phases: (0..n).map(|_| random()).collect(),
        });
    }
    Ok(out)
}
pub(super) fn sample(
    outlet: &Outlet,
    index: usize,
    seconds: f64,
    fade: f64,
) -> Result<Pose, String> {
    if !seconds.is_finite() || seconds < 0. || !fade.is_finite() || !(0. ..=1.).contains(&fade) {
        return Err("Invalid fixture particle clock or fade".into());
    }
    let phase = *outlet.phases.get(index).ok_or("Fixture particle index")?;
    let vapor = outlet.vapor;
    let life = (seconds / if vapor { 3.5 } else { 6. } + phase).fract();
    Ok(Pose {
        position: outlet.local
            + DVec3::new(
                if vapor { 0.01 } else { 0.017 } * life * life,
                0.006 * life,
                if vapor { 0.025 } else { 0.055 } * life,
            ),
        scale: if vapor {
            0.005 + 0.013 * life
        } else {
            0.007 + 0.021 * life
        },
        opacity: (std::f64::consts::PI * life).sin() * if vapor { 0.13 } else { 0.19 } * fade,
        rotation: phase * std::f64::consts::PI + life * 0.2,
    })
}
pub(super) fn tag(pose: Pose, vapor: bool) -> u32 {
    let alpha = (pose.opacity.clamp(0., 1.) * 4095.).round() as u32;
    let rotation = (pose.rotation.rem_euclid(std::f64::consts::TAU) / std::f64::consts::TAU
        * 16383.)
        .round() as u32;
    alpha | (rotation << 12) | ((if vapor { 9 } else { 10 }) << 26)
}
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn actual_factory_outlets_are_bounded_and_repeated_seeks_do_not_change_phases() {
        let p = BodyProjection::MARS;
        let frame = Frame::new(p, [1, 0, -1]).unwrap();
        let request=serde_json::from_value(serde_json::json!({"plan":"grid","landmark":"factory","cover":"none","details":["smokestacks","ground-exhausts","tanks","pipes"],"density":"dense"})).unwrap();
        let layout =
            openmars_scene::structure::layout::generate(37, &request, "Fixture control").unwrap();
        let out = prepare(&layout, [1, 0, -1], &frame, p).unwrap();
        assert!(out.iter().any(|o| o.vapor));
        assert!(out.iter().any(|o| !o.vapor));
        for o in out {
            assert_eq!(o.phases.len(), if o.vapor { 5 } else { 8 });
            let before = sample(&o, 0, 12345.5, 1.).unwrap();
            sample(&o, 0, 0., 1.).unwrap();
            let after = sample(&o, 0, 12345.5, 1.).unwrap();
            assert_eq!(before.position, after.position);
            assert_eq!(before.opacity, after.opacity);
            assert!(sample(&o, usize::MAX, 0., 1.).is_err());
            assert!(sample(&o, 0, f64::NAN, 1.).is_err());
        }
    }
}
