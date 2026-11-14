//! Source geometry, including Three's arc-length mapping and parallel frames.
use bevy::{
    asset::RenderAssetUsages,
    math::{DQuat, DVec3},
    mesh::{Indices, PrimitiveTopology, VertexAttributeValues},
    prelude::*,
};
pub fn sphere(radius: f32) -> Mesh {
    let mut mesh = Sphere::new(radius).mesh().uv(96, 64);
    for attr in [Mesh::ATTRIBUTE_POSITION, Mesh::ATTRIBUTE_NORMAL] {
        if let Some(VertexAttributeValues::Float32x3(v)) = mesh.attribute_mut(attr) {
            for p in v {
                *p = [-p[0], p[2], p[1]];
            }
        }
    }
    mesh
}
pub fn orbit(radius: f32) -> Mesh {
    let angles: Vec<_> = (0..=1024)
        .map(|i| i as f32 / 1024. * std::f32::consts::TAU)
        .collect();
    Mesh::new(
        PrimitiveTopology::LineStrip,
        RenderAssetUsages::RENDER_WORLD,
    )
    .with_inserted_attribute(
        Mesh::ATTRIBUTE_POSITION,
        angles
            .iter()
            .map(|a| [a.cos() * radius, 0., a.sin() * radius])
            .collect::<Vec<_>>(),
    )
    .with_inserted_attribute(Mesh::ATTRIBUTE_NORMAL, vec![[0., 1., 0.]; 1025])
    .with_inserted_attribute(
        Mesh::ATTRIBUTE_UV_0,
        angles.into_iter().map(|a| [a, 0.]).collect::<Vec<_>>(),
    )
}
fn control(i: i32) -> DVec3 {
    let at = |n: i32| {
        let t = f64::from(n) / 12.;
        DVec3::new(
            (t - 0.5) * 5.6,
            (t * std::f64::consts::PI).sin() * (2.8 + 0.6 * (t * 5.).sin()),
            (t * std::f64::consts::TAU).sin() * 0.65,
        )
    };
    if i < 0 {
        at(0) * 2. - at(1)
    } else if i > 12 {
        at(12) * 2. - at(11)
    } else {
        at(i)
    }
}
fn curve(t: f64) -> DVec3 {
    let x = t.clamp(0., 1.) * 12.;
    let i = (x.floor() as i32).min(11);
    let u = x - f64::from(i);
    let [p0, p1, p2, p3] = [control(i - 1), control(i), control(i + 1), control(i + 2)];
    let t1 = p0.distance(p1).sqrt();
    let t2 = t1 + p1.distance(p2).sqrt();
    let t3 = t2 + p2.distance(p3).sqrt();
    let t = t1 + (t2 - t1) * u;
    let a1 = p0 * ((t1 - t) / t1) + p1 * (t / t1);
    let a2 = p1 * ((t2 - t) / (t2 - t1)) + p2 * ((t - t1) / (t2 - t1));
    let a3 = p2 * ((t3 - t) / (t3 - t2)) + p3 * ((t - t2) / (t3 - t2));
    let b1 = a1 * ((t2 - t) / t2) + a2 * (t / t2);
    let b2 = a2 * ((t3 - t) / (t3 - t1)) + a3 * ((t - t1) / (t3 - t1));
    b1 * ((t2 - t) / (t2 - t1)) + b2 * ((t - t1) / (t2 - t1))
}
pub fn prominence() -> Mesh {
    let mut lengths = vec![0.];
    let mut previous = curve(0.);
    for i in 1..=200 {
        let p = curve(f64::from(i) / 200.);
        lengths.push(lengths.last().unwrap() + p.distance(previous));
        previous = p;
    }
    let mapping = |u: f64| {
        let target = u * lengths[200];
        let i = lengths.partition_point(|l| *l <= target).saturating_sub(1);
        if i >= 200 || lengths[i] == target {
            i as f64 / 200.
        } else {
            (i as f64 + (target - lengths[i]) / (lengths[i + 1] - lengths[i])) / 200.
        }
    };
    let parameters: Vec<_> = (0..=64).map(|i| mapping(f64::from(i) / 64.)).collect();
    let tangents: Vec<_> = parameters
        .iter()
        .map(|t| (curve((t + 0.0001).min(1.)) - curve((t - 0.0001).max(0.))).normalize())
        .collect();
    let first = tangents[0];
    let mut axis = DVec3::X;
    let mut min = first.x.abs();
    if first.y.abs() <= min {
        min = first.y.abs();
        axis = DVec3::Y;
    }
    if first.z.abs() <= min {
        axis = DVec3::Z;
    }
    let mut normal = first.cross(first.cross(axis).normalize());
    let (mut positions, mut normals, mut uvs, mut indices) = (
        Vec::with_capacity(585),
        Vec::with_capacity(585),
        Vec::with_capacity(585),
        Vec::with_capacity(3072),
    );
    for i in 0..=64 {
        if i > 0 {
            let cross = tangents[i - 1].cross(tangents[i]);
            if cross.length() > f64::EPSILON {
                let angle = tangents[i - 1].dot(tangents[i]).clamp(-1., 1.).acos();
                normal = DQuat::from_axis_angle(cross.normalize(), angle) * normal;
            }
        }
        let binormal = tangents[i].cross(normal);
        let center = curve(parameters[i]);
        for j in 0..=8 {
            let angle = f64::from(j) / 8. * std::f64::consts::TAU;
            let n = (-normal * angle.cos() + binormal * angle.sin()).normalize();
            positions.push((center + n * 0.12).as_vec3().to_array());
            normals.push(n.as_vec3().to_array());
            uvs.push([i as f32 / 64., j as f32 / 8.]);
        }
    }
    for i in 1..=64 {
        for j in 1..=8 {
            let a = 9 * (i - 1) + j - 1;
            let b = 9 * i + j - 1;
            let c = 9 * i + j;
            let d = 9 * (i - 1) + j;
            indices.extend_from_slice(&[a, b, d, b, c, d]);
        }
    }
    Mesh::new(
        PrimitiveTopology::TriangleList,
        RenderAssetUsages::RENDER_WORLD,
    )
    .with_inserted_attribute(Mesh::ATTRIBUTE_POSITION, positions)
    .with_inserted_attribute(Mesh::ATTRIBUTE_NORMAL, normals)
    .with_inserted_attribute(Mesh::ATTRIBUTE_UV_0, uvs)
    .with_inserted_indices(Indices::U32(indices))
}
