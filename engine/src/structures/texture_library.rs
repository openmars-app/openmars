//! Scene-owned facade preparation. CPU work advances outside the GPU fence;
//! generated images are admitted/uploaded through its bounded rendering queue.
use super::textures::{CpuJob, GENERATED_IMAGES, SOURCES, Slot, Spec, plans};
use crate::terrain::mips::{MipRenderer, MipWork, target_image};
use bevy::{
    platform::time::Instant,
    prelude::*,
    render::{
        RenderApp,
        render_asset::RenderAssets,
        renderer::{RenderDevice, RenderQueue},
        texture::GpuImage,
    },
};
use std::collections::{BTreeMap, VecDeque};
struct Resident {
    base: Option<Handle<Image>>,
    sampled: Handle<Image>,
    work: Option<MipWork>,
    ready: bool,
}
#[derive(Resource, Default)]
pub struct TextureLibrary {
    pending: VecDeque<Spec>,
    cpu: Option<CpuJob>,
    completed: VecDeque<(Spec, Image)>,
    resident: BTreeMap<Slot, Resident>,
    started: bool,
    pub cpu_steps: u64,
    pub mip_draws: u64,
}
impl TextureLibrary {
    pub fn ready(&self) -> bool {
        self.started
            && self.pending.is_empty()
            && self.cpu.is_none()
            && self.completed.is_empty()
            && self.resident.len() == GENERATED_IMAGES
            && self.resident.values().all(|r| r.ready)
    }
    pub fn prepared(&self) -> usize {
        self.resident.values().filter(|r| r.ready).count()
    }
    pub fn images(&self) -> usize {
        self.resident
            .values()
            .map(|r| 1 + usize::from(r.base.is_some()))
            .sum()
    }
    pub fn pending_cpu_bytes(&self) -> usize {
        self.completed
            .iter()
            .map(|(_, v)| v.data.as_ref().map_or(0, Vec::len))
            .sum::<usize>()
            + self
                .cpu
                .as_ref()
                .map_or(0, |j| j.spec.width as usize * j.spec.height as usize * 4)
    }
    pub fn progress(&self) -> Option<String> {
        (!self.ready()).then(|| {
            let detail = self
                .cpu
                .as_ref()
                .map(|v| {
                    let (done, total) = v.progress();
                    format!(", {done}/{total} rows")
                })
                .unwrap_or_default();
            format!(
                "Preparing structure textures ({}/{} images{detail})",
                self.prepared(),
                GENERATED_IMAGES
            )
        })
    }
    pub fn get(&self, mut slot: Slot) -> Option<Handle<Image>> {
        if let Slot::Facade(family, role) = slot
            && family >= 3
            && (role == 1 || role == 2)
        {
            slot = Slot::Facade(family - 3, role);
        }
        self.resident
            .get(&slot)
            .filter(|v| v.ready)
            .map(|v| v.sampled.clone())
    }
    pub fn advance_cpu(&mut self, world: &World) -> Result<(), String> {
        if world.resource::<crate::scene::SceneState>().0.is_none() {
            return Ok(());
        }
        if !self.started {
            self.pending = plans().into();
            self.started = true;
        }
        let started = Instant::now();
        // At most2 pending decoded outputs and16 bounded row chunks; stop
        // after6ms between chunks so the loading UI remains responsive.
        for _ in 0..16 {
            if self.completed.len() >= 2 {
                return Ok(());
            }
            if self.cpu.is_none() {
                let Some(spec) = self.pending.pop_front() else {
                    return Ok(());
                };
                self.cpu = Some(CpuJob::new(spec)?);
            }
            let job = self.cpu.as_mut().unwrap();
            let handle = job
                .spec
                .source
                .and_then(|i| crate::assets::texture(world, SOURCES[i].0));
            let source = handle
                .as_ref()
                .and_then(|h| world.resource::<Assets<Image>>().get(h));
            if job.spec.source.is_some() && source.is_none() {
                return Ok(());
            }
            let complete = job.advance(source)?;
            self.cpu_steps += 1;
            if complete {
                let job = self.cpu.take().unwrap();
                let spec = job.spec;
                self.completed.push_back((spec, job.finish()?));
            }
            if started.elapsed().as_secs_f64() >= 0.006 {
                return Ok(());
            }
        }
        Ok(())
    }
    pub fn prepare_queued(
        &mut self,
        app: &mut App,
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<(), String> {
        // Prepare these maps before foliage is admitted: filling a mip chain
        // must not repeatedly draw thousands of live plants behind loading UI.
        // Foliage and terrain bake await this library, keeping one mip job/frame.
        let pending = self.resident.values().filter(|r| !r.ready).count();
        for _ in pending..2 {
            let Some((spec, image)) = self.completed.pop_front() else {
                break;
            };
            let sampler = image.sampler.clone();
            let base = app.world_mut().resource_mut::<Assets<Image>>().add(image);
            let mut target = target_image(
                spec.width,
                spec.height,
                1,
                spec.width.max(spec.height).ilog2() + 1,
                spec.srgb,
            );
            target.sampler = sampler;
            let sampled = app.world_mut().resource_mut::<Assets<Image>>().add(target);
            let work = MipWork::for_images(app.world(), &[(base.clone(), sampled.clone())])?;
            if self
                .resident
                .insert(
                    spec.slot,
                    Resident {
                        base: Some(base),
                        sampled,
                        work: Some(work),
                        ready: false,
                    },
                )
                .is_some()
            {
                return Err("Duplicate structure texture slot".into());
            }
        }
        if let Some(resident) = self.resident.values_mut().find(|r| !r.ready) {
            let mut renderer = app.world_mut().remove_resource::<MipRenderer>().unwrap();
            let before = renderer.draws;
            let result = renderer.advance_queued(
                resident.work.as_mut().unwrap(),
                app.sub_app(RenderApp)
                    .world()
                    .resource::<RenderAssets<GpuImage>>(),
                device,
                queue,
            );
            self.mip_draws += renderer.draws - before;
            app.world_mut().insert_resource(renderer);
            if result? {
                resident.ready = true;
                resident.work = None;
                resident.base = None;
            }
        }
        Ok(())
    }
}
pub fn advance_cpu(world: &mut World) -> Result<(), String> {
    let mut profile = crate::profiling::span(world, "structures.textures.cpu");
    let mut library = world.remove_resource::<TextureLibrary>().unwrap();
    if profile.enabled() {
        profile.set_arg("steps_before", library.cpu_steps);
        profile.set_arg("images_ready", library.prepared());
    }
    let result = library.advance_cpu(world);
    profile.set_arg("steps_after", library.cpu_steps);
    world.insert_resource(library);
    result
}
pub fn prepare_queued(
    app: &mut App,
    device: &RenderDevice,
    queue: &RenderQueue,
) -> Result<(), String> {
    let _profile = crate::profiling::span(app.world(), "structures.textures.upload");
    let mut library = app.world_mut().remove_resource::<TextureLibrary>().unwrap();
    let result = library.prepare_queued(app, device, queue);
    app.world_mut().insert_resource(library);
    result
}
