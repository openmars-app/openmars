//! Live presentation only. Uses the shared nuclear timeline and public snapshot;
//! quiet snapshots and reduced motion allocate no flight/cloud resources.
use super::{
    blast_compositor::{NuclearShake, NuclearView, Probe, Volume},
    blast_geometry as geometry,
    blast_material::{WaveMaterial, WaveUniform},
    timeline::Timeline,
};
use crate::atmosphere::standard::HazeMaterial;
use bevy::{
    camera::{
        RenderTarget,
        visibility::{NoFrustumCulling, RenderLayers},
    },
    core_pipeline::tonemapping::Tonemapping,
    light::{NotShadowCaster, NotShadowReceiver},
    mesh::MeshTag,
    pbr::PreparedMaterial,
    prelude::*,
    render::{
        RenderApp, erased_render_asset::ErasedRenderAssets, mesh::RenderMesh,
        render_asset::RenderAssets, render_resource::*, renderer::RenderAdapter,
    },
};
use openmars_scene::geometry::nuclear_site::BoardFrame;
use std::{
    collections::{BTreeMap, BTreeSet},
    sync::{
        Arc,
        atomic::{AtomicBool, Ordering},
    },
};
type Coordinate = (i32, i32, i32);
const MAX_BYTES: usize = 128 * 1024 * 1024;
struct Entry {
    root: Entity,
    missile: Entity,
    wave: Entity,
    frame: Mat4,
    seed: u32,
    warm: u8,
    age: Option<f32>,
    sound: String,
}
struct Shared {
    meshes: [Handle<Mesh>; 2],
    missile: Handle<HazeMaterial>,
    wave: Handle<WaveMaterial>,
    camera: Entity,
}
struct Target {
    image: Handle<Image>,
    extent: [u32; 2],
    bytes: usize,
    encoded: Arc<AtomicBool>,
}
struct Retired {
    target: Option<Target>,
    shared: Option<Shared>,
    frames: u8,
}
#[derive(Resource, Default)]
pub struct BlastRuntime {
    entries: BTreeMap<Coordinate, Entry>,
    suppressed: BTreeSet<Coordinate>,
    shared: Option<Shared>,
    target: Option<Target>,
    retired: Vec<Retired>,
    probe: Arc<Probe>,
    allocations: u64,
    retirements: u64,
    ready: bool,
}
impl BlastRuntime {
    pub fn ready(&self) -> bool {
        self.ready || self.entries.is_empty()
    }
    pub fn allows(&self, coordinate: Coordinate) -> bool {
        self.suppressed.contains(&coordinate)
            || (self
                .target
                .as_ref()
                .is_some_and(|t| t.encoded.load(Ordering::Acquire))
                && self.entries.get(&coordinate).is_some_and(|e| e.warm >= 2))
    }
    pub fn diagnostics(&self, world: &World) -> serde_json::Value {
        serde_json::json!({"ready":self.ready(),"sites":self.entries.len(),
            "target":self.target.as_ref().map(|t|serde_json::json!({"extent":t.extent,"bytes":t.bytes,"format":world.resource::<Assets<Image>>().get(&t.image).map(|i|format!("{:?}",i.texture_descriptor.format)),"encoded":t.encoded.load(Ordering::Acquire)})),
            "bytes":self.bytes(),"allocations":self.allocations,"retirements":self.retirements,
            "retired":self.retired.len(),"encodedFrames":self.probe.frames.load(Ordering::Acquire),
            "draws":self.probe.draws.load(Ordering::Acquire),"uniformBytes":self.probe.bytes.load(Ordering::Acquire),"uniformAllocations":self.probe.allocations.load(Ordering::Acquire),"uniformRetirements":self.probe.retirements.load(Ordering::Acquire),
            "buffer":self.probe.buffer.lock().unwrap().clone(),
            "shake":world.iter_entities().find_map(|e|e.get::<NuclearShake>().map(|s|s.offset.to_array())),
            "sample":self.entries.iter().map(|(c,e)|serde_json::json!({"coordinate":c,"seed":e.seed,"entity":e.root.to_string(),"warmFrames":e.warm,"age":e.age,
                "missile":world.get::<Transform>(e.missile).map(|t|t.translation.to_array()),"missileVisible":world.get::<Visibility>(e.missile)==Some(&Visibility::Inherited),
                "waveTag":world.get::<MeshTag>(e.wave).map(|t|t.value)})).collect::<Vec<_>>()})
    }
    fn bytes(&self) -> usize {
        self.target
            .iter()
            .chain(self.retired.iter().filter_map(|r| r.target.as_ref()))
            .map(|t| t.bytes)
            .sum::<usize>()
            + (usize::from(self.shared.is_some())
                + self.retired.iter().filter(|r| r.shared.is_some()).count())
                * 4
                * 1024
                * 1024
    }
    fn remove_view(world: &mut World) {
        let cameras: Vec<_> = world
            .query_filtered::<Entity, With<crate::assets::AssetCamera>>()
            .iter(world)
            .collect();
        for camera in cameras {
            world
                .entity_mut(camera)
                .remove::<NuclearView>()
                .remove::<NuclearShake>();
        }
    }
    fn clear(&mut self, world: &mut World) {
        for (_, entry) in std::mem::take(&mut self.entries) {
            world
                .resource_mut::<crate::presentation::SoundRuntime>()
                .stop_sound(&entry.sound);
            world.despawn(entry.root);
        }
        Self::remove_view(world);
        if self.target.is_some() || self.shared.is_some() {
            if let Some(shared) = &self.shared {
                world.despawn(shared.camera);
            }
            self.retired.push(Retired {
                target: self.target.take(),
                shared: self.shared.take(),
                frames: 2,
            });
        }
        self.ready = true;
    }
    fn collect(&mut self) {
        for retired in &mut self.retired {
            retired.frames = retired.frames.saturating_sub(1);
        }
        self.retired.retain(|r| {
            if r.frames == 0 {
                self.retirements += 1;
                false
            } else {
                true
            }
        });
    }
    fn shared(
        &mut self,
        world: &mut World,
        warm: Handle<Image>,
        radius: f32,
    ) -> Result<(), String> {
        if self.shared.is_some() {
            return Ok(());
        }
        if self.bytes() + 4 * 1024 * 1024 > MAX_BYTES {
            return Err("Blast working-set limit exceeded".into());
        }
        let meshes = [geometry::missile(), geometry::wave()]
            .map(|m| world.resource_mut::<Assets<Mesh>>().add(m));
        let missile = crate::atmosphere::standard::material(
            world,
            StandardMaterial {
                base_color: Color::srgb_u8(0x59, 0x63, 0x3d),
                perceptual_roughness: 0.85,
                ..default()
            },
        )?;
        let wave = world
            .resource_mut::<Assets<WaveMaterial>>()
            .add(WaveMaterial {
                parameters: WaveUniform {
                    center_radius: Vec4::new(0., 0., -radius - 0.01, radius),
                },
            });
        let camera = world
            .spawn((
                Camera3d::default(),
                Camera {
                    order: -5,
                    clear_color: ClearColorConfig::Custom(Color::BLACK),
                    ..default()
                },
                RenderTarget::Image(warm.into()),
                Msaa::Off,
                Tonemapping::None,
                RenderLayers::layer(2),
                Transform::from_xyz(0., 0., 6.8).looking_at(Vec3::ZERO, Vec3::Y),
            ))
            .id();
        self.shared = Some(Shared {
            meshes,
            missile,
            wave,
            camera,
        });
        self.allocations += 1;
        Ok(())
    }
    fn target(&mut self, world: &mut World, physical: [u32; 2]) -> Result<(), String> {
        let extent = geometry::cloud_extent(physical).ok_or("Invalid blast viewport")?;
        if self.target.as_ref().is_some_and(|t| t.extent == extent) {
            return Ok(());
        }
        let adapter = world.resource::<RenderAdapter>();
        let format = super::blast_compositor::cloud_format(
            adapter.get_texture_format_features(TextureFormat::Rgba16Float),
        );
        let bytes = extent[0] as usize
            * extent[1] as usize
            * if format == TextureFormat::Rgba16Float {
                8
            } else {
                4
            };
        if self.bytes() + bytes > MAX_BYTES {
            return Err("Blast target limit exceeded".into());
        }
        let mut image = Image::new_target_texture(extent[0], extent[1], format, None);
        image.sampler = bevy::image::ImageSampler::linear();
        let target = Target {
            image: world.resource_mut::<Assets<Image>>().add(image),
            extent,
            bytes,
            encoded: Arc::new(AtomicBool::new(false)),
        };
        if let Some(old) = self.target.replace(target) {
            self.retired.push(Retired {
                target: Some(old),
                shared: None,
                frames: 2,
            });
        }
        self.allocations += 1;
        Ok(())
    }
    fn desired(&mut self, timeline: &Timeline, reduced: bool) -> BTreeSet<Coordinate> {
        let live: BTreeSet<_> = timeline.coordinates().collect();
        self.suppressed.retain(|c| live.contains(c));
        if reduced {
            self.suppressed.extend(live.iter().copied());
        }
        live.difference(&self.suppressed).copied().collect()
    }
    fn update(
        &mut self,
        world: &mut World,
        input: &super::Input,
        uploaded: bool,
    ) -> Result<(), String> {
        self.collect();
        let desired = self.desired(world.resource::<Timeline>(), input.reduced);
        if desired.is_empty() {
            self.clear(world);
            return Ok(());
        }
        if desired.len() > geometry::MAX_BLASTS {
            return Err("Blast count limit exceeded".into());
        }
        self.shared(
            world,
            input.warm.clone(),
            input.snapshot.projection().radius() as f32,
        )?;
        let (physical, css) = {
            let view = world.resource::<crate::world::WorldView>();
            (view.physical.to_array(), view.size)
        };
        self.target(world, physical)?;
        for c in self
            .entries
            .keys()
            .filter(|c| !desired.contains(c))
            .copied()
            .collect::<Vec<_>>()
        {
            let entry = self.entries.remove(&c).unwrap();
            world
                .resource_mut::<crate::presentation::SoundRuntime>()
                .stop_sound(&entry.sound);
            world.despawn(entry.root);
        }
        let game = world
            .resource::<crate::scene::SceneState>()
            .0
            .as_ref()
            .unwrap()
            .game_id
            .clone();
        for c in desired
            .iter()
            .filter(|c| !self.entries.contains_key(c))
            .take(2)
            .copied()
            .collect::<Vec<_>>()
        {
            let frame = BoardFrame::new(input.snapshot.projection(), [c.0, c.1, c.2], true)
                .map_err(|e| e.to_string())?;
            let transform = Transform {
                translation: Vec3::from_array(frame.origin.map(|x| x as f32)),
                rotation: Quat::from_array(frame.rotation.map(|x| x as f32)),
                scale: Vec3::ONE,
            };
            let root = world.spawn((transform, Visibility::Inherited)).id();
            let shared = self.shared.as_ref().unwrap();
            let missile = world
                .spawn((
                    Mesh3d(shared.meshes[0].clone()),
                    MeshMaterial3d(shared.missile.clone()),
                    Transform::IDENTITY,
                    Visibility::Inherited,
                    RenderLayers::layer(2),
                    NotShadowCaster,
                    NotShadowReceiver,
                    ChildOf(root),
                ))
                .id();
            let wave = world
                .spawn((
                    Mesh3d(shared.meshes[1].clone()),
                    MeshMaterial3d(shared.wave.clone()),
                    Transform::IDENTITY,
                    Visibility::Inherited,
                    RenderLayers::layer(2),
                    MeshTag::new(0x80000000),
                    NoFrustumCulling,
                    NotShadowCaster,
                    NotShadowReceiver,
                    crate::transparent::TransparentOrder::BLAST_WAVE,
                    ChildOf(root),
                ))
                .id();
            self.entries.insert(
                c,
                Entry {
                    root,
                    missile,
                    wave,
                    frame: transform.to_matrix(),
                    seed: openmars_scene::landscape::visual_seed(&format!(
                        "{game}:{},{},{}",
                        c.0, c.1, c.2
                    )),
                    warm: 0,
                    age: None,
                    sound: format!("nuclear:{root}"),
                },
            );
        }
        let encoded = self
            .target
            .as_ref()
            .unwrap()
            .encoded
            .load(Ordering::Acquire);
        let mut volumes = Vec::with_capacity(self.entries.len());
        let mut shaking = Vec2::ZERO;
        for (c, entry) in &mut self.entries {
            if uploaded && encoded {
                entry.warm = entry.warm.saturating_add(1).min(2);
            }
            let start = world.resource::<Timeline>().start(*c);
            let sample = start.map(|s| geometry::sample(s.step, input.step, false));
            entry.age = sample.map(|s| s.age);
            let active = entry.warm >= 2 && sample.is_some();
            if active && sample.is_some_and(|s| s.age >= 0.) {
                world
                    .resource_mut::<crate::presentation::SoundRuntime>()
                    .start_sound(&game, &entry.sound)?;
            }
            if let Some(sample) = sample {
                if let Some(pose) = sample.missile {
                    *world.get_mut::<Transform>(entry.missile).unwrap() = pose;
                }
                world.entity_mut(entry.missile).insert((
                    if active && sample.missile.is_some() {
                        Visibility::Inherited
                    } else {
                        Visibility::Hidden
                    },
                    RenderLayers::default(),
                ));
                world.entity_mut(entry.wave).insert((
                    RenderLayers::default(),
                    MeshTag::new(if active && sample.wave {
                        input.step.saturating_sub(start.unwrap().step) as u32
                    } else {
                        0x80000000
                    }),
                ));
            }
            volumes.push(Volume {
                local: entry.frame,
                age: if active {
                    entry.age.unwrap_or(-1.)
                } else {
                    -1.
                },
                seed: entry.seed,
            });
        }
        let camera = world
            .query_filtered::<Entity, With<crate::assets::AssetCamera>>()
            .iter(world)
            .next()
            .ok_or("Missing blast camera")?;
        // Use this frame's unshaken orbit/framing, matching the source
        // compositor's visibility test before its projection perturbation.
        if let (Some((transform, sub, fov)), Some(projection)) = (
            crate::world::framed_camera(world.resource::<crate::world::WorldView>()),
            world.get::<Projection>(camera),
        ) {
            let mut projection = projection.clone();
            if let Projection::Perspective(p) = &mut projection {
                p.fov = fov;
            }
            let clip =
                projection.get_clip_from_view_for_sub(&sub) * transform.to_matrix().inverse();
            for volume in &volumes {
                let p = clip * volume.local.w_axis;
                if geometry::visible(volume.local, clip, transform.translation)
                    && p.w > 0.
                    && p.z >= 0.
                    && p.z <= p.w
                    && p.x.abs() <= p.w
                    && p.y.abs() <= p.w
                {
                    shaking += geometry::shake(volume.age);
                }
            }
        }
        let target = self.target.as_ref().unwrap();
        world.entity_mut(camera).insert((
            NuclearView {
                target: target.image.clone(),
                extent: target.extent,
                world_from_board: Mat4::IDENTITY,
                volumes,
                encoded: target.encoded.clone(),
                probe: self.probe.clone(),
            },
            NuclearShake {
                offset: geometry::clamp_shake(shaking)
                    * Vec2::from_array(physical.map(|x| x as f32))
                    / css,
            },
        ));
        self.ready = desired.iter().all(|c| self.allows(*c));
        Ok(())
    }
}
pub fn prepare_queued(app: &mut App, input: Option<&super::Input>) -> Result<(), String> {
    let uploaded = if let Some(shared) = &app.world().resource::<BlastRuntime>().shared {
        let render = app.sub_app(RenderApp).world();
        let meshes = render.resource::<RenderAssets<RenderMesh>>();
        let materials = render.resource::<ErasedRenderAssets<PreparedMaterial>>();
        shared.meshes.iter().all(|h| meshes.get(h.id()).is_some())
            && materials.get(shared.missile.id().untyped()).is_some()
            && materials.get(shared.wave.id().untyped()).is_some()
            && render
                .resource::<PipelineCache>()
                .waiting_pipelines()
                .next()
                .is_none()
    } else {
        false
    };
    let mut runtime = app.world_mut().remove_resource::<BlastRuntime>().unwrap();
    let result = if let Some(input) = input {
        runtime.update(app.world_mut(), input, uploaded)
    } else {
        runtime.collect();
        runtime.clear(app.world_mut());
        Ok(())
    };
    app.world_mut().insert_resource(runtime);
    result
}
pub fn reset(world: &mut World) {
    let Some(mut runtime) = world.remove_resource::<BlastRuntime>() else {
        return;
    };
    runtime.clear(world);
    runtime.suppressed.clear();
    world.insert_resource(runtime);
}
pub struct BlastPlugin;
impl Plugin for BlastPlugin {
    fn build(&self, app: &mut App) {
        bevy::asset::embedded_asset!(app, "blast_wave.wesl");
        app.add_plugins((
            MaterialPlugin::<WaveMaterial>::default(),
            super::blast_compositor::BlastCompositorPlugin,
        ))
        .init_resource::<BlastRuntime>();
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn reduced_motion_and_quiet_reconnect_cannot_replay_blasts() {
        let c = (0, 0, 0);
        let mut timeline = Timeline::default();
        let mut runtime = BlastRuntime::default();
        timeline.observe([c].into(), false);
        assert_eq!(runtime.desired(&timeline, false), [c].into());
        assert!(runtime.desired(&timeline, true).is_empty());
        assert!(
            runtime.allows(c),
            "suppressed effects cannot block crater readiness"
        );
        assert!(
            runtime.desired(&timeline, false).is_empty(),
            "resuming motion cannot replay"
        );
        timeline.observe([c].into(), true);
        assert!(runtime.desired(&timeline, false).is_empty());
        timeline.observe(BTreeSet::new(), false);
        timeline.observe([c].into(), false);
        assert_eq!(runtime.desired(&timeline, false), [c].into());
    }
    #[test]
    fn clearing_retains_shared_assets_for_two_submitted_turns() {
        let mut world = World::new();
        world.init_resource::<crate::presentation::SoundRuntime>();
        world.insert_resource(Assets::<Mesh>::default());
        world.insert_resource(Assets::<StandardMaterial>::default());
        world.insert_resource(Assets::<WaveMaterial>::default());
        let mut runtime = BlastRuntime::default();
        // This lifetime fixture owns real source meshes; GPU material preparation
        // is exercised by the production browser tests.
        let meshes = [geometry::missile(), geometry::wave()]
            .map(|mesh| world.resource_mut::<Assets<Mesh>>().add(mesh));
        runtime.shared = Some(Shared {
            meshes,
            missile: Handle::default(),
            wave: Handle::default(),
            camera: world.spawn_empty().id(),
        });
        let weak = runtime.shared.as_ref().unwrap().meshes[1].clone();
        assert_eq!(
            world
                .resource::<Assets<Mesh>>()
                .get(&weak)
                .unwrap()
                .count_vertices(),
            5289
        );
        assert_eq!(runtime.bytes(), 4 * 1024 * 1024);
        runtime.clear(&mut world);
        assert!(runtime.shared.is_none() && runtime.entries.is_empty());
        assert_eq!(runtime.retired.len(), 1);
        runtime.collect();
        assert_eq!(runtime.retired.len(), 1);
        runtime.collect();
        assert!(runtime.retired.is_empty());
        assert_eq!(runtime.bytes(), 0);
        assert_eq!(runtime.retirements, 1);
    }
}
