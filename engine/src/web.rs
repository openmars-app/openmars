use bevy::{
    app::PluginsState,
    prelude::*,
    render::{
        RenderApp, RenderPlugin,
        error_handler::{RenderErrorHandler, RenderErrorPolicy},
        render_resource::{CachedPipelineState, PipelineCache},
        renderer::{RenderAdapter, RenderAdapterInfo, RenderDevice, RenderInstance, RenderQueue},
        settings::RenderCreation,
        texture::{ManualTextureView, ManualTextureViews},
    },
    shader::ShaderCacheError,
    window::ExitCondition,
};
use std::sync::{
    Arc,
    atomic::{AtomicBool, Ordering},
};
use wasm_bindgen::prelude::*;

#[derive(Resource)]
struct RenderFailure(String);

/// One independently disposable Bevy world and browser rendering surface.
/// The host calls frame() from requestAnimationFrame and free() on unmount.
#[wasm_bindgen]
pub struct Engine {
    app: App,
    surface: wgpu::Surface<'static>,
    device: RenderDevice,
    device_limits: wgpu::Limits,
    queue: RenderQueue,
    config: wgpu::SurfaceConfiguration,
    submitted: bool,
    frames: super::frame_pacing::FramePacer,
    ready: Arc<AtomicBool>,
    initialized: bool,
    preparation: &'static str,
    profiler: super::profiling::Profiler,
}

impl Drop for Engine {
    fn drop(&mut self) {
        self.device.wgpu_device().destroy();
    }
}

#[wasm_bindgen]
impl Engine {
    pub async fn create(canvas: web_sys::HtmlCanvasElement) -> Result<Engine, JsValue> {
        std::panic::set_hook(Box::new(|error| {
            web_sys::console::error_1(&js_error(error))
        }));
        let instance = wgpu::Instance::new(wgpu::InstanceDescriptor {
            backends: if cfg!(feature = "webgl2") {
                wgpu::Backends::GL
            } else {
                wgpu::Backends::BROWSER_WEBGPU
            },
            ..wgpu::InstanceDescriptor::new_without_display_handle()
        });
        let surface = instance
            .create_surface(wgpu::SurfaceTarget::Canvas(canvas))
            .map_err(js_error)?;
        let adapter = instance
            .request_adapter(&wgpu::RequestAdapterOptions {
                compatible_surface: Some(&surface),
                power_preference: wgpu::PowerPreference::HighPerformance,
                ..Default::default()
            })
            .await
            .map_err(|error| {
                web_sys::console::error_1(&js_error(error));
                JsValue::from_str(
                    "The browser could not access a graphics adapter for this renderer.",
                )
            })?;
        let (device, queue) = adapter
            .request_device(&wgpu::DeviceDescriptor {
                label: Some("Open Mars engine"),
                required_limits: if cfg!(feature = "webgl2") {
                    wgpu::Limits::downlevel_webgl2_defaults().using_resolution(adapter.limits())
                } else {
                    wgpu::Limits::default()
                },
                ..Default::default()
            })
            .await
            .map_err(js_error)?;
        let mut config = surface
            .get_default_config(&adapter, 1, 1)
            .ok_or_else(|| JsValue::from_str("No supported WebGPU surface configuration"))?;
        // Bevy writes linear light. WebGPU canvas formats are usually unorm;
        // use their sRGB view so browser output matches the WebGL2 surface.
        let view_format = config.format.add_srgb_suffix();
        if view_format != config.format {
            config.view_formats.push(view_format);
        }
        let device = RenderDevice::from(device);
        let device_limits = device.limits();
        let queue = RenderQueue::new(queue);
        surface.configure(device.wgpu_device(), &config);
        let renderer = RenderCreation::manual(
            device.clone(),
            queue.clone(),
            RenderAdapterInfo::new(adapter.get_info()),
            RenderAdapter::new(adapter),
            RenderInstance::new(instance),
        );
        let mut app = App::new();
        let profiler = super::profiling::Profiler::default();
        super::assets::configure(&mut app);
        app.add_plugins(
            DefaultPlugins
                .set(super::assets::plugin())
                .disable::<bevy::anti_alias::AntiAliasPlugin>()
                .set(WindowPlugin {
                    primary_window: None,
                    exit_condition: ExitCondition::DontExit,
                    ..default()
                })
                .set(RenderPlugin {
                    render_creation: renderer,
                    ..default()
                }),
        )
        .add_plugins(bevy::anti_alias::fxaa::FxaaPlugin)
        .add_plugins(super::terrain::TerrainMaterialPlugin)
        .add_plugins(super::labels::material::LabelMaterialPlugin)
        .add_plugins(super::terrain::bake::GroundBakePlugin)
        .add_plugins(super::vegetation::materials::FoliageMaterialPlugin)
        .add_plugins(super::structures::materials::StructureMaterialPlugin)
        .add_plugins(super::structures::world_tree::WorldTreePlugin)
        .add_plugins(super::structures::sites::SitePlugin)
        .add_plugins(super::structures::special::SpecialPlugin)
        .add_plugins(super::particles::ParticlePlugin)
        .add_plugins(super::nuclear::NuclearPlugin)
        .add_plugins(super::asteroid::AsteroidPlugin)
        .add_plugins(super::atmosphere::AtmospherePlugin)
        .add_plugins(super::offworld::materials::OffworldMaterialPlugin)
        .add_plugins(super::sky::SkyPlugin)
        .add_plugins(super::space::material::SpaceMaterialPlugin)
        .init_resource::<super::vegetation::FoliageRuntime>()
        .init_resource::<super::fauna::FaunaRuntime>()
        .init_resource::<super::structures::texture_library::TextureLibrary>()
        .init_resource::<super::structures::scene::StructureRuntime>()
        .add_plugins(super::transparent::TransparentOrderPlugin)
        .insert_resource(ClearColor(Color::BLACK))
        .init_resource::<super::scene::SceneState>()
        .init_resource::<super::menu::MenuState>()
        .init_resource::<super::menu::MenuRuntime>()
        .init_resource::<super::space::scene::CelestialRuntime>()
        .init_resource::<super::offworld::scene::OffworldRuntime>()
        .init_resource::<super::labels::scene::LabelRuntime>()
        .init_resource::<super::scene::MotionPreference>()
        .init_resource::<super::scene::WorldEntryState>()
        .init_resource::<super::entry::WorldEntrance>()
        .init_resource::<super::presentation::PresentationRuntime>()
        .init_resource::<super::scene::PresentationState>()
        .init_resource::<super::world::WorldView>()
        .init_resource::<super::graphics::GraphicsState>()
        .add_systems(PreUpdate, super::graphics::antialiasing)
        .init_resource::<super::terrain::TerrainState>()
        .init_resource::<super::terrain::surface::SurfaceRuntime>()
        .init_resource::<super::terrain::grid::GridRuntime>()
        .insert_resource(RenderErrorHandler(|error, world, _| {
            let kind = format!("{:?}", error.ty);
            let message = if error.description.trim().is_empty() {
                kind.clone()
            } else {
                format!("{kind}: {}", error.description)
            };
            let _event = super::profiling::span(world, "renderer.error")
                .arg("kind", kind)
                .arg("message", message.clone());
            world.insert_resource(RenderFailure(message));
            RenderErrorPolicy::StopRendering
        }))
        .add_systems(Startup, (super::setup, super::assets::setup))
        .add_systems(
            Update,
            (
                super::scene::update,
                super::world::camera,
                super::nuclear::blast_compositor::apply_shake.after(super::world::camera),
                super::terrain::surface::cameras
                    .after(super::nuclear::blast_compositor::apply_shake),
                super::world::tiles,
                super::assets::cameras,
                super::assets::synchronize,
            ),
        );
        super::profiling::install(&mut app, profiler.clone());
        Ok(Self {
            app,
            surface,
            device,
            device_limits,
            queue,
            config,
            submitted: false,
            frames: super::frame_pacing::FramePacer::default(),
            ready: Arc::new(AtomicBool::new(false)),
            initialized: false,
            preparation: "Starting Bevy",
            profiler,
        })
    }

    pub fn terrain_request(&mut self) -> Result<Vec<u8>, JsValue> {
        super::terrain::surface::worker_request(self.app.world_mut()).map_err(js_error)
    }
    pub fn accept_terrain(&mut self, bytes: &[u8]) -> Result<bool, JsValue> {
        super::terrain::surface::accept_worker(self.app.world_mut(), bytes).map_err(js_error)
    }
    pub fn prepare_cpu(&mut self) -> Result<bool, JsValue> {
        let _span = self.profiler.span("native.prepare_cpu");
        if !self.initialized {
            return Ok(false);
        }
        super::terrain::surface::advance_cpu(self.app.world_mut()).map_err(js_error)
    }

    pub fn frame(&mut self, width: u32, height: u32, delta_seconds: f64) -> Result<bool, JsValue> {
        let mut frame_span = self.profiler.span("native.frame");
        if frame_span.enabled() {
            frame_span.set_arg("submitted_before", self.frames.submitted());
            frame_span.set_arg("width", width);
            frame_span.set_arg("height", height);
        }
        if self.ready.load(Ordering::Relaxed) {
            self.app
                .world_mut()
                .resource_mut::<super::scene::PresentationState>()
                .settling = false;
        }
        if !self.initialized {
            if self.app.plugins_state() == PluginsState::Adding {
                return Ok(false);
            }
            self.app.finish();
            self.app.cleanup();
            super::atmosphere::standard::initialize_highlights(self.app.world_mut());
            super::lighting::configure(&mut self.app);
            if cfg!(feature = "webgl2") {
                // A WebGL resize copy can overwrite newly uploaded indices.
                // Fixed-size pools allocate another slab instead of copying a
                // growing buffer. Preserve packing, geometry and instancing.
                let mut mesh =
                    self.app
                        .sub_app_mut(RenderApp)
                        .world_mut()
                        .resource_mut::<bevy::render::mesh::allocator::MeshAllocatorSettings>();
                mesh.max_slab_size = mesh.min_slab_size;
            }
            self.initialized = true;
            self.preparation = "Preparing scene";
        }
        super::menu::advance(self.app.world_mut(), delta_seconds).map_err(js_error)?;
        super::terrain::surface::advance_clock(self.app.world_mut(), delta_seconds)
            .map_err(js_error)?;
        super::structures::texture_library::advance_cpu(self.app.world_mut()).map_err(js_error)?;
        super::nuclear::advance_cpu(self.app.world_mut()).map_err(js_error)?;
        if width == 0 || height == 0 {
            return Ok(false);
        }
        // Poll WebGL fences even while waiting. WebGPU uses Promise callbacks.
        let admitted = {
            let mut span = self.profiler.span("native.frame.admission");
            self.device
                .wgpu_device()
                .poll(wgpu::PollType::Poll)
                .map_err(js_error)?;
            let admitted = self.frames.can_submit();
            if span.enabled() {
                span.set_arg("admitted", admitted);
                span.set_arg("pending_frames", self.frames.pending());
                span.set_arg("frame_budget", self.frames.budget());
            }
            admitted
        };
        if !admitted {
            return Ok(self.ready.load(Ordering::Relaxed));
        }
        let limit = self.device_limits.max_texture_dimension_2d;
        let width = width.min(limit);
        let height = height.min(limit);
        if self.config.width != width || self.config.height != height {
            let _span = self
                .profiler
                .span("native.surface.resize")
                .arg("width", width)
                .arg("height", height);
            self.config.width = width;
            self.config.height = height;
            self.surface
                .configure(self.device.wgpu_device(), &self.config);
        }
        let output = match self.surface.get_current_texture() {
            wgpu::CurrentSurfaceTexture::Success(output)
            | wgpu::CurrentSurfaceTexture::Suboptimal(output) => output,
            wgpu::CurrentSurfaceTexture::Timeout | wgpu::CurrentSurfaceTexture::Occluded => {
                return Ok(false);
            }
            wgpu::CurrentSurfaceTexture::Outdated | wgpu::CurrentSurfaceTexture::Lost => {
                self.surface
                    .configure(self.device.wgpu_device(), &self.config);
                return Ok(false);
            }
            wgpu::CurrentSurfaceTexture::Validation => {
                return Err(JsValue::from_str("Renderer surface validation failed"));
            }
        };
        self.app
            .world_mut()
            .resource_mut::<ManualTextureViews>()
            .insert(
                super::SURFACE,
                ManualTextureView {
                    texture_view: output
                        .texture
                        .create_view(&wgpu::TextureViewDescriptor {
                            format: Some(self.config.format.add_srgb_suffix()),
                            ..Default::default()
                        })
                        .into(),
                    size: UVec2::new(width, height),
                    view_format: self.config.format.add_srgb_suffix(),
                },
            );
        super::entry::prepare(self.app.world_mut());
        super::terrain::surface::prepare(
            &mut self.app,
            UVec2::new(width, height),
            self.config.format.add_srgb_suffix(),
        )
        .map_err(js_error)?;
        super::lighting::prepare_environment(&mut self.app).map_err(js_error)?;
        super::labels::scene::prepare(&mut self.app).map_err(js_error)?;
        super::terrain::surface::apply_entry(self.app.world_mut());
        super::structures::scene::apply_entry(self.app.world_mut());
        super::structures::world_tree::apply_entry(self.app.world_mut());
        super::structures::sites::apply_entry(self.app.world_mut());
        super::structures::special::apply_entry(self.app.world_mut());
        super::particles::apply_entry(self.app.world_mut());
        super::nuclear::apply_entry(self.app.world_mut());
        super::graphics::prepare_views(self.app.world_mut());
        {
            let _span = self.profiler.span("native.app.update");
            self.app.update();
        }
        if let Some(error) = self.app.world().get_resource::<RenderFailure>() {
            return Err(JsValue::from_str(&error.0));
        }
        self.queue.present(output);
        // Bound the logical frame backlog without serializing each requestAnimationFrame
        // behind a GPU-to-browser acknowledgment. Preparation and draws use one ordered
        // queue; the independent readiness callback below still waits for real completion.
        // Keep this timeline across scene/focus changes: their old frames remain pending.
        let complete = self.frames.submit();
        self.queue
            .on_submitted_work_done(move || complete.complete());

        // New pipelines can fail during live tile changes too. Keep this check
        // active after startup, independently of the initial readiness gate.
        for pipeline in self
            .app
            .sub_app(RenderApp)
            .world()
            .resource::<PipelineCache>()
            .pipelines()
        {
            if let CachedPipelineState::Err(error) = &pipeline.state
                && !matches!(
                    error,
                    ShaderCacheError::ShaderNotLoaded(_)
                        | ShaderCacheError::ShaderImportNotYetAvailable
                )
            {
                return Err(js_error(error));
            }
        }
        // Mesh/material upload checks are a startup gate, not frame work.
        // Scene replacement, resync and destination changes reset `submitted`.
        // The submitted callback still determines when the host may show the world.
        if self.submitted {
            return Ok(self.ready.load(Ordering::Relaxed));
        }
        let _readiness = self.profiler.span("native.readiness");
        let assets_prepared = super::assets::prepared(&mut self.app).map_err(js_error)?;
        let has_scene =
            super::assets::view(self.app.world()) != openmars_scene::view::AssetView::Idle;
        if assets_prepared && has_scene {
            super::assets::publish(self.app.world_mut());
        }
        // Offscreen warm passes run first. Activating the published camera in
        // the next update may queue sky/post-process pipelines, so readiness
        // still requires a real main-view render and its GPU completion.
        let view_active = !has_scene || super::assets::view_active(self.app.world_mut());
        // Authored worlds prepare their own 3D labels through assets::prepared.
        // Only the idle greeting uses the separate Bevy UI camera.
        let text_prepared = has_scene
            || self
                .app
                .world_mut()
                .query::<(&bevy::text::TextLayoutInfo, &ComputedNode)>()
                .iter(self.app.world())
                .any(|(text, node)| !text.glyphs.is_empty() && node.size().x > 0.0);
        let pipelines = self
            .app
            .sub_app(RenderApp)
            .world()
            .resource::<PipelineCache>();
        self.preparation = if !assets_prepared {
            "Decoding and uploading world assets"
        } else if !text_prepared {
            "Preparing text"
        } else if pipelines.waiting_pipelines().next().is_some() {
            "Preparing shaders"
        } else {
            "Waiting for GPU"
        };
        if assets_prepared
            && view_active
            && text_prepared
            && pipelines.waiting_pipelines().next().is_none()
        {
            self.submitted = true;
            let ready = self.ready.clone();
            let event = self.profiler.deferred("scene.ready");
            self.queue.on_submitted_work_done(move || {
                ready.store(true, Ordering::Relaxed);
                event.emit();
            });
        }
        Ok(self.ready.load(Ordering::Relaxed))
    }

    pub fn preparation(&self) -> String {
        if self
            .app
            .world()
            .resource::<super::scene::SceneState>()
            .0
            .is_some()
            && let Some(progress) = self
                .app
                .world()
                .resource::<super::structures::texture_library::TextureLibrary>()
                .progress()
        {
            return progress;
        }
        if self
            .app
            .world()
            .resource::<super::scene::SceneState>()
            .0
            .is_some()
            && let Some(progress) = self
                .app
                .world()
                .resource::<super::vegetation::FoliageRuntime>()
                .progress()
        {
            return progress;
        }
        if self
            .app
            .world()
            .resource::<super::scene::SceneState>()
            .0
            .is_some()
            && let Some(progress) = self
                .app
                .world()
                .resource::<super::structures::scene::StructureRuntime>()
                .progress()
        {
            return progress;
        }
        if self
            .app
            .world()
            .resource::<super::scene::SceneState>()
            .0
            .is_some()
            && let Some(progress) = self
                .app
                .world()
                .resource::<super::structures::world_tree::WorldTreeRuntime>()
                .progress()
        {
            return progress;
        }
        if self
            .app
            .world()
            .resource::<super::scene::SceneState>()
            .0
            .is_some()
            && let Some(progress) = self
                .app
                .world()
                .resource::<super::structures::sites::SiteRuntime>()
                .progress()
        {
            return progress;
        }
        if self
            .app
            .world()
            .resource::<super::scene::SceneState>()
            .0
            .is_some()
            && let Some(progress) = self
                .app
                .world()
                .resource::<super::structures::special::SpecialRuntime>()
                .progress()
        {
            return progress;
        }
        if self
            .app
            .world()
            .resource::<super::scene::SceneState>()
            .0
            .is_some()
            && let Some(progress) = self
                .app
                .world()
                .resource::<super::nuclear::NuclearRuntime>()
                .progress()
        {
            return progress;
        }
        if self
            .app
            .world()
            .resource::<super::scene::SceneState>()
            .0
            .is_some()
            && let Some(progress) = self
                .app
                .world()
                .resource::<super::particles::ParticleRuntime>()
                .progress()
        {
            return progress;
        }
        if let Some(progress) = self
            .app
            .world()
            .resource::<super::atmosphere::AtmosphereRuntime>()
            .progress()
        {
            return progress;
        }
        if self
            .app
            .world()
            .resource::<super::scene::SceneState>()
            .0
            .is_some()
            && let Some(progress) = self
                .app
                .world()
                .resource::<super::fauna::FaunaRuntime>()
                .progress()
        {
            return progress;
        }
        if let Some(progress) = self
            .app
            .world()
            .resource::<super::terrain::surface::SurfaceRuntime>()
            .progress()
        {
            return progress;
        }
        if let Some((done, total)) = self
            .app
            .world()
            .resource::<super::terrain::TerrainState>()
            .progress()
        {
            format!("Building terrain ({done}/{total} rows)")
        } else {
            self.preparation.to_owned()
        }
    }

    /// Start a new capture on the host performance clock, or stop while retaining
    /// the final events for drain_profile(). Times in returned events are microseconds.
    pub fn set_profiling(&mut self, enabled: bool, host_time_ms: f64) {
        self.profiler.set_enabled(enabled, host_time_ms);
    }

    pub fn set_profile_context(&self, context: u32) {
        self.profiler.set_context(context);
    }

    pub fn drain_profile(&self, max_events: usize) -> String {
        self.profiler.drain(max_events)
    }

    /// Public live log cue. React remains the sole network owner.
    pub fn present(&mut self, json: &str) -> Result<bool, JsValue> {
        if json.len() > 2048 {
            return Err(js_error("Presentation cue is too large"));
        }
        let cue: openmars_scene::presentation::SceneCue =
            serde_json::from_str(json).map_err(js_error)?;
        cue.validate().map_err(js_error)?;
        let world = self.app.world_mut();
        if world
            .resource::<super::scene::SceneState>()
            .0
            .as_ref()
            .is_none_or(|s| s.game_id != cue.game_id)
        {
            return Ok(false);
        }
        let quiet = world.resource::<super::scene::PresentationState>().settling
            || world.resource::<super::scene::MotionPreference>().0;
        world
            .resource_mut::<super::presentation::PresentationRuntime>()
            .accept(cue, quiet)
            .map_err(js_error)
    }
    pub fn take_sounds(&mut self) -> String {
        serde_json::to_string(
            &self
                .app
                .world_mut()
                .resource_mut::<super::presentation::PresentationRuntime>()
                .take_sounds(),
        )
        .expect("Sound events serialize")
    }

    /// Local accessibility control; it does not modify the game snapshot.
    pub fn set_reduced_motion(&mut self, reduced: bool) {
        self.app
            .world_mut()
            .resource_mut::<super::scene::MotionPreference>()
            .0 = reduced;
    }

    /// Explicit local entry phase, independent of game rules and generation.
    pub fn set_world_entry(&mut self, json: &str) -> Result<(), JsValue> {
        if json.len() > 96 {
            return Err(js_error("Invalid world entry phase"));
        }
        let entry: openmars_scene::view::WorldEntry =
            serde_json::from_str(json).map_err(js_error)?;
        self.app
            .world_mut()
            .resource_mut::<super::scene::WorldEntryState>()
            .0 = entry;
        Ok(())
    }

    /// Replace the presentation snapshot atomically, including clearing a departed game.
    pub fn set_scene(&mut self, json: &str, settled: bool) -> Result<String, JsValue> {
        let _span = self
            .profiler
            .span("native.set_scene")
            .arg("bytes", json.len())
            .arg("settled", settled);
        if json.len() > 4 * 1024 * 1024 {
            return Err(js_error("Scene snapshot is too large"));
        }
        let next: Option<openmars_scene::SceneSnapshot> =
            serde_json::from_str(json).map_err(js_error)?;
        if let Some(scene) = &next {
            scene.validate().map_err(js_error)?;
        }
        let previous = &self.app.world().resource::<super::scene::SceneState>().0;
        let identity = |value: &Option<openmars_scene::SceneSnapshot>| {
            value
                .as_ref()
                .map(|s| (s.game_id.clone(), s.map_id.clone()))
        };
        let replaced = identity(previous) != identity(&next);
        let resync = settled && next.is_some();
        if previous != &next {
            self.app
                .world_mut()
                .insert_resource(super::scene::SceneState(next));
        }
        if replaced {
            let view = if self
                .app
                .world()
                .resource::<super::scene::SceneState>()
                .0
                .is_some()
            {
                openmars_scene::view::AssetView::Game
            } else if self
                .app
                .world()
                .resource::<super::menu::MenuState>()
                .0
                .is_some()
            {
                openmars_scene::view::AssetView::Menu
            } else {
                openmars_scene::view::AssetView::Idle
            };
            super::assets::reset(self.app.world_mut(), view).map_err(js_error)?;
            super::world::reset(self.app.world_mut());
        }
        if replaced || resync {
            self.app
                .world_mut()
                .resource_mut::<super::presentation::PresentationRuntime>()
                .clear(replaced);
            self.ready = Arc::new(AtomicBool::new(false));
            self.submitted = false;
            self.preparation = "Preparing scene";
            self.app
                .world_mut()
                .resource_mut::<super::scene::PresentationState>()
                .settling = true;
        }
        let quiet = replaced
            || resync
            || self
                .app
                .world()
                .resource::<super::scene::PresentationState>()
                .settling;
        super::nuclear::timeline::observe(self.app.world_mut(), quiet);
        Ok(if replaced {
            "replace"
        } else if resync {
            "resync"
        } else {
            "live"
        }
        .into())
    }

    /// No snapshot is fabricated for a menu. Its local route selects an audience.
    pub fn set_menu(&mut self, json: &str) -> Result<String, JsValue> {
        if json.len() > 32 {
            return Err(js_error("Invalid menu placement"));
        }
        let placement: Option<openmars_scene::view::MenuPlacement> =
            serde_json::from_str(json).map_err(js_error)?;
        self.app
            .world_mut()
            .resource_mut::<super::menu::MenuState>()
            .0 = placement;
        if self
            .app
            .world()
            .resource::<super::scene::SceneState>()
            .0
            .is_some()
        {
            return Ok("live".into());
        }
        let view = if placement.is_some() {
            openmars_scene::view::AssetView::Menu
        } else {
            openmars_scene::view::AssetView::Idle
        };
        if super::assets::view(self.app.world()) == view {
            return Ok("live".into());
        }
        super::assets::reset(self.app.world_mut(), view).map_err(js_error)?;
        super::world::reset(self.app.world_mut());
        self.ready = Arc::new(AtomicBool::new(false));
        self.submitted = false;
        self.preparation = "Preparing scene";
        self.app
            .world_mut()
            .resource_mut::<super::scene::PresentationState>()
            .settling = true;
        Ok("replace".into())
    }
    pub fn set_focus(&mut self, json: &str) -> Result<String, JsValue> {
        if json.len() > 128 {
            return Err(js_error("Invalid camera focus"));
        }
        let focus: openmars_scene::view::FocusTarget =
            serde_json::from_str(json).map_err(js_error)?;
        super::assets::change_focus(self.app.world_mut(), focus).map_err(js_error)?;
        Ok("live".into())
    }
    pub fn set_graphics(&mut self, json: &str) -> Result<(), JsValue> {
        if json.len() > 512 {
            return Err(js_error("Graphics settings require a fresh renderer"));
        }
        let settings: openmars_scene::view::GraphicsSettings =
            serde_json::from_str(json).map_err(js_error)?;
        settings.validate().map_err(js_error)?;
        let previous = self
            .app
            .world()
            .resource::<super::graphics::GraphicsState>()
            .0;
        if self.initialized
            && (previous.texture_quality != settings.texture_quality
                || previous.effects != settings.effects)
        {
            return Err(js_error(
                "Resident quality changes require a fresh renderer",
            ));
        }
        self.app
            .world_mut()
            .resource_mut::<super::graphics::GraphicsState>()
            .0 = settings;
        Ok(())
    }
    pub fn menu_pointer(&mut self, x: f32, y: f32) -> Result<(), JsValue> {
        super::menu::pointer(self.app.world_mut(), x, y).map_err(js_error)
    }
    pub fn asset_demand(&self) -> String {
        serde_json::to_string(&super::assets::demand(self.app.world()))
            .expect("Asset requests serialize")
    }

    pub fn pointer(
        &mut self,
        x: f32,
        y: f32,
        client_x: f32,
        client_y: f32,
        inspect: bool,
    ) -> String {
        let event = super::world::pointer(
            self.app.world_mut(),
            x,
            y,
            Vec2::new(client_x, client_y),
            inspect,
        );
        serde_json::to_string(&event).expect("Scene event is serializable")
    }

    pub fn hover(&mut self, x: f32, y: f32, client_x: f32, client_y: f32) -> String {
        if let Some(body) = super::world::pick_body(self.app.world_mut(), x, y) {
            self.app
                .world_mut()
                .resource_mut::<super::world::WorldView>()
                .hovered = None;
            let event = self
                .app
                .world()
                .resource::<super::scene::SceneState>()
                .0
                .as_ref()
                .map(|scene| openmars_scene::SceneEvent::BodyHovered {
                    game_id: scene.game_id.clone(),
                    body,
                });
            return serde_json::to_string(&event).expect("Scene event is serializable");
        }
        let tile = super::world::pick(self.app.world_mut(), x, y);
        self.app
            .world_mut()
            .resource_mut::<super::world::WorldView>()
            .hovered = tile.clone();
        let event = self
            .app
            .world()
            .resource::<super::scene::SceneState>()
            .0
            .as_ref()
            .map(|scene| openmars_scene::SceneEvent::TileHovered {
                game_id: scene.game_id.clone(),
                tile_id: tile,
                x: client_x,
                y: client_y,
            });
        serde_json::to_string(&event).expect("Scene event is serializable")
    }

    pub fn camera_input(&mut self, yaw: f32, pitch: f32, zoom: f32, reset: bool) {
        if ![yaw, pitch, zoom].iter().all(|v| v.is_finite()) {
            return;
        }
        let view = &mut *self
            .app
            .world_mut()
            .resource_mut::<super::world::WorldView>();
        let limits = super::space::config::orbit(view.focus);
        view.placement.cancel();
        if reset {
            let home = super::space::config::home(view.focus);
            view.yaw = home.theta as f32;
            view.pitch = (std::f64::consts::FRAC_PI_2 - home.phi) as f32;
            view.distance = if view.focus == openmars_scene::view::FocusTarget::default() {
                6.8
            } else {
                home.radius as f32
            };
        } else {
            view.yaw = (view.yaw + yaw).rem_euclid(std::f32::consts::TAU);
            view.pitch = (view.pitch + pitch).clamp(-1.4835298, 1.4835298);
            view.distance = (view.distance * zoom.clamp(-1., 1.).exp())
                .clamp(limits.min as f32, limits.max as f32);
        }
        view.hovered = None;
    }

    pub fn viewport(
        &mut self,
        metrics: &[f32],
        physical_width: u32,
        physical_height: u32,
        compact: bool,
    ) {
        let [width, height, top, bottom, left, right] = metrics else {
            return;
        };
        if !metrics.iter().all(|v| v.is_finite() && *v >= 0.0)
            || *width == 0.0
            || *height == 0.0
            || physical_width == 0
            || physical_height == 0
        {
            return;
        }
        let view = &mut *self
            .app
            .world_mut()
            .resource_mut::<super::world::WorldView>();
        view.size = Vec2::new(*width, *height);
        view.physical = UVec2::new(physical_width, physical_height);
        view.insets = openmars_scene::geometry::HudInsets {
            top: (*top).into(),
            bottom: (*bottom).into(),
            left: (*left).into(),
            right: (*right).into(),
        };
        view.compact = compact;
    }

    pub fn overlay_frame(&mut self) -> String {
        let frame = if self.ready.load(Ordering::Acquire) {
            super::space::overlays::frame(self.app.world_mut())
        } else {
            openmars_scene::view::OverlayFrame::default()
        };
        serde_json::to_string(&frame).expect("Overlay projections serialize")
    }

    /// Projected tile identities and camera state for scene inspection tools.
    pub fn scene_stats(&mut self) -> String {
        // The browser may inspect between async device creation and the first
        // frame. Lighting and startup-owned scene resources do not exist yet.
        if !self.initialized {
            return "null".to_owned();
        }
        let lighting = super::lighting::environment_stats(&self.app);
        let world = self.app.world_mut();
        let camera = world
            .query_filtered::<(&Camera, &GlobalTransform), With<super::assets::AssetCamera>>()
            .single(world)
            .ok()
            .map(|(c, t)| (c.clone(), *t));
        let view = world.resource::<super::world::WorldView>();
        let scale = view.size / view.physical.as_vec2();
        let relief = world
            .resource::<super::terrain::TerrainState>()
            .relief
            .clone();
        let camera_state = serde_json::json!({"yaw":view.yaw,"pitch":view.pitch,"distance":view.distance,"hovered":view.hovered,"framing":view.placement.animating()});
        let tiles:Vec<_>=world.query::<(Entity,&super::world::BoardTile)>().iter(world).map(|(entity,tile)|{
            let board=openmars_scene::geometry::BodyProjection::MARS.hex_center(openmars_scene::geometry::AxialPoint{q:tile.0.q,r:tile.0.r});
            let direction=openmars_scene::geometry::BodyProjection::MARS.project(board,0.0).unwrap_or([0.0;3]);
            let height=relief.as_ref().and_then(|map|map.sample_direction(direction).ok()).unwrap_or(0.0);
            let position=openmars_scene::geometry::BodyProjection::MARS.project(board,height+0.02).unwrap_or([0.0;3]);
            let point=camera.as_ref().and_then(|(camera,transform)|camera.world_to_viewport(transform,Vec3::from_array(position.map(|v|v as f32))).ok()).map(|p|(p*scale).to_array());
            serde_json::json!({"id":tile.0.id,"entity":entity.to_string(),"occupant":tile.0.occupant,"point":point})
        }).collect();
        let terrain = world.resource::<super::terrain::surface::SurfaceRuntime>();
        serde_json::json!({"entry":world.resource::<super::entry::WorldEntrance>().diagnostics(),"entryActors":super::entry::actors(world),"boardReady":super::assets::board_ready(world),"labels":world.resource::<super::labels::scene::LabelRuntime>().diagnostics(world),"offworld":world.resource::<super::offworld::scene::OffworldRuntime>().diagnostics(world),"celestialGrids":world.resource::<super::terrain::grid::GridRuntime>().diagnostics(world,camera.as_ref().map(|(c,t)|(c,t)),scale),"space":world.resource::<super::space::scene::CelestialRuntime>().diagnostics(camera.as_ref().map(|(c,t)|(c,t)),scale),"graphics":world.resource::<super::graphics::GraphicsState>().0,"focus":super::assets::focus(world),"menu":world.resource::<super::menu::MenuRuntime>().diagnostics(),"lighting":lighting,"atmosphere":world.resource::<super::atmosphere::AtmosphereRuntime>().diagnostics(),"asteroids":world.resource::<super::asteroid::AsteroidRuntime>().diagnostics(world),"presentation":world.resource::<super::presentation::PresentationRuntime>().diagnostics(),"nuclearDebris":world.resource::<super::nuclear::NuclearRuntime>().debris_poses(world),"blast":world.resource::<super::nuclear::blast::BlastRuntime>().diagnostics(world),"vegetationPoses":world.resource::<super::vegetation::FoliageRuntime>().poses(world),"nuclearTimeline":world.resource::<super::nuclear::timeline::Timeline>().diagnostics(),"craters":world.resource::<super::nuclear::NuclearRuntime>().diagnostics(),"specialSurfaces":world.resource::<super::structures::special::SpecialRuntime>().diagnostics(),"worksites":world.resource::<super::structures::sites::SiteRuntime>().diagnostics(),"step":terrain.step(),"settling":world.resource::<super::scene::PresentationState>().settling,"worldTrees":world.resource::<super::structures::world_tree::WorldTreeRuntime>().diagnostics(),"vegetation":world.resource::<super::vegetation::FoliageRuntime>().diagnostics(),"structures":world.resource::<super::structures::scene::StructureRuntime>().diagnostics(),"camera":camera_state,"tiles":tiles,"terrain":{"revision":terrain.revision(),"transitioning":terrain.transitioning()}}).to_string()
    }

    pub fn install_asset(
        &mut self,
        request: u32,
        id: String,
        filename: String,
        bytes: Vec<u8>,
    ) -> Result<(), JsValue> {
        super::assets::install(self.app.world_mut(), request, id, filename, bytes).map_err(js_error)
    }

    /// Diagnostic ownership counters for repeated scene loads and performance tools.
    pub fn asset_stats(&mut self) -> String {
        let visible = self
            .app
            .world_mut()
            .query::<&ViewVisibility>()
            .iter(self.app.world())
            .filter(|visibility| visibility.get())
            .count();
        let active_cameras = self
            .app
            .world_mut()
            .query::<&Camera>()
            .iter(self.app.world())
            .filter(|camera| camera.is_active)
            .count();
        let world = self.app.world();
        let render = self.app.sub_app(RenderApp).world();
        let arrays = super::assets::array_stats(world, render);
        let terrain = world.resource::<super::terrain::surface::SurfaceRuntime>();
        let (shade_buffers, shade_revision, shade_shapes, shade_draws) = world
            .resource::<super::vegetation::FoliageRuntime>()
            .shade_stats();
        let structures = world.resource::<super::structures::texture_library::TextureLibrary>();
        let foliage = serde_json::json!({
            "structureTextureImages": structures.images(),
            "structureTexturesPrepared": structures.prepared(),
            "structureTextureCpuBytes": structures.pending_cpu_bytes(),
            "structureTextureCpuSteps": structures.cpu_steps,
            "structureTextureMipDraws": structures.mip_draws,
            "shadeBuffers":shade_buffers,"shadeRevision":shade_revision,"shadeShapes":shade_shapes,"shadeDraws":shade_draws,
            "foliageModels": world.resource::<super::vegetation::FoliageRuntime>().variants(),
            "foliagePlants": world.resource::<super::vegetation::FoliageRuntime>().plants(),
            "foliageTextures": world.resource::<super::vegetation::FoliageRuntime>().textures(),
            "foliageVertices": world.resource::<super::vegetation::FoliageRuntime>().mesh_vertices(),
            "foliageIndices": world.resource::<super::vegetation::FoliageRuntime>().mesh_indices(),
            "foliageUniforms": world.resource::<super::vegetation::FoliageRuntime>().uniforms(),
            "foliageUniformWrites": world.resource::<super::vegetation::FoliageRuntime>().uniform_writes,
            "foliageRetiredUniforms": world.resource::<super::vegetation::FoliageRuntime>().retired_uniforms,
            "foliageMipDraws": world.resource::<super::vegetation::FoliageRuntime>().mip_draws,
            "foliageSpawns": world.resource::<super::vegetation::FoliageRuntime>().spawns,
        });
        let mut stats = serde_json::json!({
            "fauna":world.resource::<super::fauna::FaunaRuntime>().diagnostics(world,render),
            "structures":world.resource::<super::structures::scene::StructureRuntime>().diagnostics(),
            "bakeUniforms": world.resource::<super::terrain::bake::BakeRuntime>().uniforms(),
            "bakeValid": u32::from(world.resource::<super::terrain::bake::BakeRuntime>().valid()),
            "bakeRevision": world.resource::<super::terrain::bake::BakeRuntime>().revision(),
            "bakeStrip": world.resource::<super::terrain::bake::BakeRuntime>().pending_strip(),
            "bakeWidth": world.resource::<super::terrain::bake::BakeRuntime>().extent()[0],
            "bakeHeight": world.resource::<super::terrain::bake::BakeRuntime>().extent()[1],
            "bakeStripes": world.resource::<super::terrain::bake::BakeRuntime>().stripes,
            "bakePublications": world.resource::<super::terrain::bake::BakeRuntime>().publications,
            "bakeCancellations": world.resource::<super::terrain::bake::BakeRuntime>().cancellations,
            "bakeUniformWrites": world.resource::<super::terrain::bake::BakeRuntime>().uniform_writes,
            "bakeMipDraws": world.resource::<super::terrain::bake::BakeRuntime>().mip_draws,
            "gridUniforms": world.resource::<super::terrain::grid::GridRuntime>().uniforms(),
            "gridUniformWrites": world.resource::<super::terrain::grid::GridRuntime>().uniform_writes,
            "gridTextureWrites": world.resource::<super::terrain::grid::GridRuntime>().texture_writes,
            "gridMeshBuilds": world.resource::<super::terrain::grid::GridRuntime>().mesh_builds,
            "terrainBatches": terrain.batches(),
            "terrainUniforms": terrain.uniforms(),
            "terrainRevision": terrain.revision(),
            "terrainRequestedRevision": terrain.requested_revision(),
            "terrainTransitioning": u32::from(terrain.transitioning()),
            "terrainUniformWrites": terrain.uniform_writes,
            "terrainRetiredUniforms": terrain.retired_uniforms,
            "terrainPatches": terrain.patches(),
            "terrainStep": terrain.step(),
            "sources": super::assets::source_count(world),
            "gpuArrays": arrays.0,
            "cpuArrays": super::assets::cpu_array_count(world),
            "gpuArrayLayers": arrays.1,
            "gpuArrayMips": arrays.2,
            "visible": visible,
            "activeCameras": active_cameras,
            "models": world.resource::<Assets<bevy::gltf::Gltf>>().len(),
            "meshes": world.resource::<Assets<Mesh>>().len(),
            "images": world.resource::<Assets<Image>>().len(),
            "materials": super::assets::material_count(world),
            "gpuMeshes": render.get_resource::<bevy::render::render_asset::RenderAssets<bevy::render::mesh::RenderMesh>>().map_or(0, |assets| assets.iter().count()),
            "gpuImages": render.get_resource::<bevy::render::render_asset::RenderAssets<bevy::render::texture::GpuImage>>().map_or(0, |assets| assets.iter().count()),
            "gpuMaterials": render.get_resource::<bevy::render::erased_render_asset::ErasedRenderAssets<bevy::pbr::PreparedMaterial>>().map_or(0, |assets| assets.iter().count()),
        });
        stats["bakeViewMeasurements"] = serde_json::json!(
            world
                .resource::<super::terrain::bake::BakeRuntime>()
                .view_measurements
        );
        stats["bakeViewCacheHits"] = serde_json::json!(
            world
                .resource::<super::terrain::bake::BakeRuntime>()
                .view_cache_hits
        );
        stats["bakeDrawCalls"] = serde_json::json!(
            world
                .resource::<super::terrain::bake::BakeRuntime>()
                .draw_calls
        );
        stats["gridReady"] = serde_json::json!(
            world
                .resource::<super::terrain::grid::GridRuntime>()
                .ready()
        );
        stats["blast"] = world
            .resource::<super::nuclear::blast::BlastRuntime>()
            .diagnostics(world);
        stats["nuclearTimeline"] = world
            .resource::<super::nuclear::timeline::Timeline>()
            .diagnostics();
        stats["craters"] = world
            .resource::<super::nuclear::NuclearRuntime>()
            .diagnostics();
        stats["particles"] = world
            .resource::<super::particles::ParticleRuntime>()
            .diagnostics(world);
        stats["specialSurfaces"] = world
            .resource::<super::structures::special::SpecialRuntime>()
            .diagnostics();
        stats["worksites"] = world
            .resource::<super::structures::sites::SiteRuntime>()
            .diagnostics();
        stats["worldTrees"] = world
            .resource::<super::structures::world_tree::WorldTreeRuntime>()
            .diagnostics();
        stats["reducedMotion"] =
            serde_json::json!(world.resource::<super::scene::MotionPreference>().0);
        stats["motionStep"] = serde_json::json!(terrain.motion_step());
        stats["settling"] =
            serde_json::json!(world.resource::<super::scene::PresentationState>().settling);
        stats
            .as_object_mut()
            .unwrap()
            .extend(foliage.as_object().unwrap().clone());
        stats.to_string()
    }

    pub fn max_texture_dimension_2d(&self) -> u32 {
        self.device_limits.max_texture_dimension_2d
    }

    /// Frames actually submitted to the canvas, excluding readiness/queue polls.
    pub fn submitted_frames(&self) -> f64 {
        self.frames.submitted() as f64
    }

    /// The exact browser device owned by this renderer; never a global device probe.
    pub fn browser_device(&self) -> JsValue {
        #[cfg(feature = "webgpu")]
        if let Some(device) = self.device.wgpu_device().as_webgpu() {
            return JsValue::from(device.clone());
        }
        JsValue::NULL
    }

    pub fn linear_memory(&self) -> JsValue {
        wasm_bindgen::memory()
    }

    /// Resident resources and visible mesh entities, not draw-call estimates.
    /// Collected on demand by the performance panel, never every frame.
    pub fn performance_stats(&mut self) -> String {
        use bevy::render::{
            erased_render_asset::ErasedRenderAssets,
            mesh::{RenderMesh, RenderMeshBufferInfo},
            render_asset::RenderAssets,
            texture::GpuImage,
        };
        let world = self.app.world_mut();
        let entities = world.entities().count_spawned();
        let visible_meshes = world
            .query::<(&Mesh3d, &ViewVisibility)>()
            .iter(world)
            .filter(|(_, visible)| visible.get())
            .count();
        let render = self.app.sub_app(RenderApp).world();
        let (mut meshes, mut vertices, mut triangles) = (0_u64, 0_u64, 0_u64);
        if let Some(assets) = render.get_resource::<RenderAssets<RenderMesh>>() {
            for (_, mesh) in assets.iter() {
                meshes += 1;
                vertices += u64::from(mesh.vertex_count);
                let count = match mesh.buffer_info {
                    RenderMeshBufferInfo::Indexed { count, .. } => count,
                    RenderMeshBufferInfo::NonIndexed => mesh.vertex_count,
                };
                triangles += match mesh.primitive_topology() {
                    bevy::mesh::PrimitiveTopology::TriangleList => u64::from(count / 3),
                    bevy::mesh::PrimitiveTopology::TriangleStrip => {
                        u64::from(count.saturating_sub(2))
                    }
                    _ => 0,
                };
            }
        }
        serde_json::json!({
            "submittedFrames": self.frames.submitted(),
            "completedFrames": self.frames.completed(),
            "pendingFrames": self.frames.pending(),
            "frameLimit": super::frame_pacing::MAX_IN_FLIGHT,
            "frameBudget": self.frames.budget(),
            "completionAckMs": self.frames.acknowledgment_ms(),
            "entities": entities,
            "visibleMeshes": visible_meshes,
            "residentMeshes": meshes,
            "residentVertices": vertices,
            "residentTriangles": triangles,
            "residentImages": render.get_resource::<RenderAssets<GpuImage>>().map_or(0, |assets| assets.iter().count()),
            "residentMaterials": render.get_resource::<ErasedRenderAssets<bevy::pbr::PreparedMaterial>>().map_or(0, |assets| assets.iter().count()),
        }).to_string()
    }

    pub fn supports_compute(&self) -> bool {
        self.device_limits.max_compute_workgroups_per_dimension > 0
    }

    pub fn supports_storage_buffers(&self) -> bool {
        self.device_limits.max_storage_buffers_per_shader_stage > 0
    }
}

fn js_error(error: impl std::fmt::Display) -> JsValue {
    JsValue::from_str(&error.to_string())
}
