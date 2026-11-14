//! Local menu presentation, independent of authoritative game snapshots.
mod camera;
use crate::atmosphere::standard::{HazeMaterial, SourceMaterials};
use crate::space::phobos as model;
use bevy::{
    camera::{
        RenderTarget,
        visibility::{NoFrustumCulling, RenderLayers},
    },
    core_pipeline::{prepass::DepthPrepass, tonemapping::Tonemapping},
    pbr::PreparedMaterial,
    prelude::*,
    render::{
        RenderApp, erased_render_asset::ErasedRenderAssets, mesh::RenderMesh,
        render_asset::RenderAssets, render_resource::*,
    },
};
use openmars_scene::view::{AssetView, MenuPlacement};
#[derive(Resource, Default)]
pub struct MenuState(pub Option<MenuPlacement>);
#[derive(Resource, Default)]
pub struct MenuRuntime {
    clock: camera::MenuCamera,
    pointer: Vec2,
    root: Option<Entity>,
    planet: Option<Entity>,
    pivot: Option<Entity>,
    moon: Option<Entity>,
    prepared: Option<model::Prepared>,
    materials: SourceMaterials,
    warm: Option<Handle<Image>>,
    camera: Option<Entity>,
    warm_frames: u8,
}
impl MenuRuntime {
    pub fn ready(&self) -> bool {
        self.root.is_some() && self.prepared.is_some() && self.warm_frames >= 2
    }
    pub fn pose(&self, reduced: bool) -> Transform {
        self.clock.camera(reduced)
    }
    pub fn diagnostics(&self) -> serde_json::Value {
        serde_json::json!({"ready":self.ready(),"root":self.root.map(|e|e.to_string()),"position":self.clock.group().to_array(),"rotation":self.clock.planet_rotation().to_array(),"moon":self.prepared.as_ref().map(|p|serde_json::json!({"parts":p.parts.len(),"center":p.center.to_array(),"scale":p.scale})),"warmFrames":self.warm_frames,"warmCamera":self.camera.is_some(),"materials":self.materials.len()})
    }
    fn clear(&mut self, world: &mut World) {
        if let Some(e) = self.root.take() {
            world.despawn(e);
        }
        if let Some(e) = self.camera.take() {
            world.despawn(e);
        }
        *self = Self::default();
    }
    fn update(
        &mut self,
        world: &mut World,
        format: TextureFormat,
        uploaded: bool,
    ) -> Result<(), String> {
        if crate::assets::view(world) != AssetView::Menu {
            if self.root.is_some() {
                self.clear(world)
            };
            return Ok(());
        }
        let reduced = world.resource::<crate::scene::MotionPreference>().0;
        if self.root.is_none() {
            let Some(texture) = crate::assets::texture(world, "textures/planets/mars/surface")
            else {
                return Ok(());
            };
            let Some(sky) = crate::assets::texture(world, "textures/skyboxes/starmap") else {
                return Ok(());
            };
            let Some(source) = model::source(world)? else {
                return Ok(());
            };
            let atmosphere = world
                .resource::<crate::atmosphere::AtmosphereRuntime>()
                .binding()?;
            let mesh = world.resource_mut::<Assets<Mesh>>().add(planet_mesh());
            let material = crate::atmosphere::standard::material(
                world,
                StandardMaterial {
                    base_color_texture: Some(texture),
                    perceptual_roughness: 1.,
                    metallic: 0.,
                    ..default()
                },
            )?;
            world
                .resource_mut::<Assets<HazeMaterial>>()
                .get_mut(&material)
                .unwrap()
                .extension
                .no_environment = true;
            let sky_mesh = world.resource_mut::<Assets<Mesh>>().add(crate::sky::mesh());
            let sky_material = world.resource_mut::<Assets<crate::sky::SkyMaterial>>().add(
                crate::sky::SkyMaterial {
                    textures: crate::sky::SkyTextures { image: sky },
                    atmosphere,
                },
            );
            let root = world
                .spawn((
                    Transform::from_translation(self.clock.group()),
                    Visibility::default(),
                ))
                .id();
            let planet = world
                .spawn((
                    Mesh3d(mesh),
                    MeshMaterial3d(material),
                    Transform::from_rotation(self.clock.planet_rotation()),
                    NoFrustumCulling,
                    RenderLayers::layer(0).with(2),
                    ChildOf(root),
                ))
                .id();
            world.spawn((
                Mesh3d(sky_mesh),
                MeshMaterial3d(sky_material),
                Transform::default(),
                NoFrustumCulling,
                ChildOf(root),
            ));
            let pivot = world
                .spawn((
                    self.clock.moon(reduced),
                    Visibility::default(),
                    ChildOf(root),
                ))
                .id();
            let moon = world
                .spawn((WorldAssetRoot(source), Transform::default(), ChildOf(pivot)))
                .id();
            let warm = world
                .resource_mut::<Assets<Image>>()
                .add(Image::new_target_texture(128, 128, format, None));
            let center = self.clock.group();
            let camera = world
                .spawn((
                    Camera3d::default(),
                    DepthPrepass,
                    Camera {
                        order: -20,
                        ..default()
                    },
                    RenderTarget::Image(warm.clone().into()),
                    Msaa::Off,
                    Tonemapping::None,
                    RenderLayers::layer(2),
                    Transform::from_translation(center + Vec3::Z * 15.).looking_at(center, Vec3::Y),
                ))
                .id();
            self.root = Some(root);
            self.planet = Some(planet);
            self.pivot = Some(pivot);
            self.moon = Some(moon);
            self.warm = Some(warm);
            self.camera = Some(camera);
        }
        if let Some(root) = self.root {
            world
                .entity_mut(root)
                .insert(Transform::from_translation(self.clock.group()));
        }
        if let Some(planet) = self.planet {
            world
                .entity_mut(planet)
                .insert(Transform::from_rotation(self.clock.planet_rotation()));
        }
        if let Some(pivot) = self.pivot {
            world.entity_mut(pivot).insert(self.clock.moon(reduced));
        }
        if let Some(camera) = self.camera {
            let center = self.clock.group();
            world.entity_mut(camera).insert(
                Transform::from_translation(center + Vec3::Z * 15.).looking_at(center, Vec3::Y),
            );
        }
        if self.prepared.is_none()
            && let Some(root) = self.moon
            && let Some(prepared) = model::prepare(world, root, &mut self.materials)?
        {
            world.entity_mut(root).insert(
                Transform::from_translation(-prepared.center * prepared.scale)
                    .with_scale(Vec3::splat(prepared.scale)),
            );
            self.prepared = Some(prepared);
        }
        if uploaded && self.prepared.is_some() {
            self.warm_frames = self.warm_frames.saturating_add(1).min(2);
        }
        if self.warm_frames >= 2
            && let Some(camera) = self.camera.take()
        {
            world.despawn(camera);
            for e in self
                .planet
                .iter()
                .chain(self.prepared.iter().flat_map(|p| p.parts.iter()))
            {
                world.entity_mut(*e).insert(RenderLayers::layer(0));
            }
        }
        Ok(())
    }
}
pub fn reset(world: &mut World) {
    if let Some(mut r) = world.remove_resource::<MenuRuntime>() {
        r.clear(world);
        world.insert_resource(r);
    }
}
pub fn pointer(world: &mut World, x: f32, y: f32) -> Result<(), String> {
    if !x.is_finite() || !y.is_finite() {
        return Err("Invalid menu pointer".into());
    }
    world.resource_mut::<MenuRuntime>().pointer =
        Vec2::new(x, y).clamp(Vec2::splat(-0.2), Vec2::splat(0.2));
    Ok(())
}
pub fn advance(world: &mut World, delta: f64) -> Result<(), String> {
    if crate::assets::view(world) != AssetView::Menu {
        return Ok(());
    }
    let placement = world
        .resource::<MenuState>()
        .0
        .ok_or("Missing menu placement")?;
    let size = world.resource::<crate::world::WorldView>().size;
    let reduced = world.resource::<crate::scene::MotionPreference>().0;
    let mut r = world.resource_mut::<MenuRuntime>();
    let pointer = r.pointer;
    r.clock.advance(placement, size, pointer, delta, reduced)
}
pub fn prepare(app: &mut App, format: TextureFormat) -> Result<(), String> {
    let uploaded = {
        let r = app.world().resource::<MenuRuntime>();
        let render = app.sub_app(RenderApp).world();
        let meshes = render.resource::<RenderAssets<RenderMesh>>();
        let materials = render.resource::<ErasedRenderAssets<PreparedMaterial>>();
        r.prepared.as_ref().is_some_and(|p| {
            p.parts.iter().chain(r.planet.iter()).all(|e| {
                app.world()
                    .get::<Mesh3d>(*e)
                    .is_some_and(|m| meshes.get(m.id()).is_some())
                    && app
                        .world()
                        .get::<MeshMaterial3d<HazeMaterial>>(*e)
                        .is_some_and(|m| materials.get(m.id().untyped()).is_some())
            })
        }) && r
            .camera
            .and_then(|e| app.world().get::<bevy::light::EnvironmentMapLight>(e))
            .is_some()
            && render
                .resource::<PipelineCache>()
                .waiting_pipelines()
                .next()
                .is_none()
    };
    let mut r = app.world_mut().remove_resource::<MenuRuntime>().unwrap();
    let result = r.update(app.world_mut(), format, uploaded);
    app.world_mut().insert_resource(r);
    result
}
pub fn atmosphere_input(app: &App) -> Option<crate::atmosphere::Input> {
    if crate::assets::view(app.world()) != AssetView::Menu {
        return None;
    }
    let r = app.world().resource::<MenuRuntime>();
    let warm = r.warm.clone()?;
    let rgb = |hex: u32| {
        [hex >> 16, hex >> 8, hex]
            .map(|v| openmars_scene::atmosphere::srgb_to_linear(f64::from(v & 255) / 255.))
    };
    Some(crate::atmosphere::Input {
        bodies: vec![crate::atmosphere::BodySpec {
            id: "menu-mars".into(),
            body: openmars_scene::atmosphere::Body {
                center: r.clock.group().as_dvec3().to_array(),
                radius: 3.,
                profile: openmars_scene::atmosphere::Profile {
                    color: rgb(0xefd2a3),
                    shadow: rgb(0x4b8cb5),
                    thickness: 0.055,
                    intensity: 1.1,
                    halo: 1.,
                },
                follows_mesh: false,
            },
            rotation: r.clock.planet_rotation(),
            hull: None,
        }],
        sun: Vec3::new(-7., 4., 10.),
        sun_color: Vec3::new(1., 0.93, 0.85),
        intensity: 0.65,
        ambient: 0.025,
        sky_rotation: 0.,
        sky_brightness: app
            .world()
            .resource::<crate::graphics::GraphicsState>()
            .0
            .background_brightness,
        seconds: 0.,
        camera: r.pose(app.world().resource::<crate::scene::MotionPreference>().0),
        projection_y: 1. / (42f64.to_radians() * 0.5).tan(),
        height: app
            .world()
            .resource::<crate::world::WorldView>()
            .size
            .y
            .into(),
        warm,
    })
}

/// Bevy's UV sphere is Z-polar. The authored globe is Y-polar with negative X
/// at its seam. This proper rotation preserves triangle winding and maps the
/// image's top row to the north pole in the browser texture convention.
fn planet_mesh() -> Mesh {
    let mut mesh = Sphere::new(3.).mesh().uv(96, 64);
    for attribute in [Mesh::ATTRIBUTE_POSITION, Mesh::ATTRIBUTE_NORMAL] {
        if let Some(bevy::mesh::VertexAttributeValues::Float32x3(values)) =
            mesh.attribute_mut(attribute)
        {
            for p in values {
                *p = [-p[0], p[2], p[1]];
            }
        }
    }
    mesh
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn menu_globe_keeps_authored_poles_seam_and_outward_normals() {
        let mesh = planet_mesh();
        let bevy::mesh::VertexAttributeValues::Float32x3(positions) =
            mesh.attribute(Mesh::ATTRIBUTE_POSITION).unwrap()
        else {
            panic!()
        };
        let bevy::mesh::VertexAttributeValues::Float32x3(normals) =
            mesh.attribute(Mesh::ATTRIBUTE_NORMAL).unwrap()
        else {
            panic!()
        };
        let bevy::mesh::VertexAttributeValues::Float32x2(uvs) =
            mesh.attribute(Mesh::ATTRIBUTE_UV_0).unwrap()
        else {
            panic!()
        };
        assert_eq!(positions.len(), 97 * 65);
        for i in [0, 32 * 97, 32 * 97 + 24, 64 * 97] {
            let p = Vec3::from_array(positions[i]);
            let n = Vec3::from_array(normals[i]);
            let [u, v] = uvs[i];
            let theta = v * std::f32::consts::PI;
            let phi = u * std::f32::consts::TAU;
            let source = Vec3::new(
                -3. * phi.cos() * theta.sin(),
                3. * theta.cos(),
                3. * phi.sin() * theta.sin(),
            );
            assert!(p.distance(source) < 1e-5);
            assert!((p.normalize() - n).length() < 1e-5);
        }
    }
}
