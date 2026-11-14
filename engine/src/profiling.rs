//! Opt-in, bounded CPU trace events in the browser host's performance time domain.
//! No GPU timestamps are inferred from these CPU spans.
use bevy::{platform::time::Instant, prelude::*};
use serde::Serialize;
use serde_json::Value;
use std::{
    collections::{BTreeMap, VecDeque},
    sync::{
        Arc, Mutex, MutexGuard,
        atomic::{AtomicBool, AtomicU32, Ordering},
    },
};

const CAPACITY: usize = 8192;
const MAX_DRAIN: usize = 128;
const MAX_ARGS: usize = 16;
const MAX_STRING: usize = 512;

#[derive(Serialize)]
struct Event {
    name: &'static str,
    ts: f64,
    dur: f64,
    cat: &'static str,
    #[serde(skip_serializing_if = "BTreeMap::is_empty")]
    args: BTreeMap<&'static str, Value>,
}

#[derive(Serialize)]
struct Drain {
    events: Vec<Event>,
    dropped: u64,
    remaining: usize,
}

struct Capture {
    epoch: u64,
    origin: Instant,
    host_us: f64,
    stop: Option<Instant>,
    events: VecDeque<Event>,
    dropped: u64,
}

impl Default for Capture {
    fn default() -> Self {
        Self {
            epoch: 0,
            origin: Instant::now(),
            host_us: 0.,
            stop: None,
            events: VecDeque::new(),
            dropped: 0,
        }
    }
}

#[derive(Default)]
struct Shared {
    enabled: AtomicBool,
    context: AtomicU32,
    capture: Mutex<Capture>,
}

impl Shared {
    fn lock(&self) -> MutexGuard<'_, Capture> {
        self.capture
            .lock()
            .unwrap_or_else(|error| error.into_inner())
    }
}

#[derive(Clone, Default, Resource)]
pub struct Profiler(Arc<Shared>);

impl Profiler {
    pub fn context(&self) -> u32 {
        self.0.context.load(Ordering::Relaxed)
    }
    pub fn set_context(&self, context: u32) {
        self.0.context.store(context, Ordering::Relaxed);
    }

    pub fn enabled(&self) -> bool {
        self.0.enabled.load(Ordering::Relaxed)
    }

    pub fn set_enabled(&self, enabled: bool, host_time_ms: f64) {
        self.set_at(enabled, host_time_ms, Instant::now());
    }

    fn set_at(&self, enabled: bool, host_time_ms: f64, now: Instant) {
        if enabled {
            let host_us = host_time_ms * 1000.;
            if !host_us.is_finite() || host_time_ms < 0. {
                return;
            }
            let mut capture = self.0.lock();
            capture.epoch = capture.epoch.wrapping_add(1);
            capture.origin = now;
            capture.host_us = host_us;
            capture.stop = None;
            capture.events.clear();
            capture.events.reserve(CAPACITY);
            capture.dropped = 0;
            self.0.enabled.store(true, Ordering::Relaxed);
        } else {
            self.0.enabled.store(false, Ordering::Relaxed);
            let mut capture = self.0.lock();
            capture.stop.get_or_insert(now);
        }
    }

    pub fn span(&self, name: &'static str) -> Span {
        if !self.enabled() {
            return Span::default();
        }
        self.span_at(name, Instant::now())
    }

    pub fn instant(&self, name: &'static str) -> Span {
        let mut span = self.span(name);
        if let Some(active) = &mut span.0 {
            active.instant = true;
        }
        span
    }

    /// Capture the current scene/capture identity now, timestamp only when the
    /// asynchronous callback runs. A stopped or superseded capture drops it.
    pub fn deferred(&self, name: &'static str) -> DeferredEvent {
        if !self.enabled() {
            return DeferredEvent(None);
        }
        let capture = self.0.lock();
        DeferredEvent(Some(Deferred {
            shared: self.0.clone(),
            epoch: capture.epoch,
            context: self.0.context.load(Ordering::Relaxed),
            name,
        }))
    }

    fn span_at(&self, name: &'static str, now: Instant) -> Span {
        let capture = self.0.lock();
        if !self.enabled() || capture.stop.is_some() {
            return Span::default();
        }
        Span(Some(Active {
            shared: self.0.clone(),
            epoch: capture.epoch,
            started: now,
            ts: capture.host_us
                + now.saturating_duration_since(capture.origin).as_secs_f64() * 1_000_000.,
            name,
            instant: false,
            args: BTreeMap::from([("context", self.0.context.load(Ordering::Relaxed).into())]),
        }))
    }

    /// Drain only accumulated events. Disable retains the final events until this call.
    pub fn drain(&self, max_events: usize) -> String {
        let data = {
            let mut capture = self.0.lock();
            let count = max_events.clamp(1, MAX_DRAIN).min(capture.events.len());
            Drain {
                events: capture.events.drain(..count).collect(),
                dropped: std::mem::take(&mut capture.dropped),
                remaining: capture.events.len(),
            }
        };
        serde_json::to_string(&data).expect("finite native profiler events")
    }
}

struct Active {
    shared: Arc<Shared>,
    epoch: u64,
    started: Instant,
    ts: f64,
    name: &'static str,
    instant: bool,
    args: BTreeMap<&'static str, Value>,
}

#[derive(Default)]
pub struct Span(Option<Active>);

struct Deferred {
    shared: Arc<Shared>,
    epoch: u64,
    context: u32,
    name: &'static str,
}

pub struct DeferredEvent(Option<Deferred>);

impl DeferredEvent {
    pub fn emit(self) {
        if self.0.is_some() {
            self.emit_at(Instant::now());
        }
    }

    fn emit_at(self, now: Instant) {
        let Some(event) = self.0 else { return };
        let mut capture = event.shared.lock();
        if capture.epoch != event.epoch
            || capture.stop.is_some()
            || !event.shared.enabled.load(Ordering::Relaxed)
        {
            return;
        }
        if capture.events.len() >= CAPACITY {
            capture.dropped = capture.dropped.saturating_add(1);
            return;
        }
        let ts = capture.host_us
            + now.saturating_duration_since(capture.origin).as_secs_f64() * 1_000_000.;
        capture.events.push_back(Event {
            name: event.name,
            ts,
            dur: 0.,
            cat: "native.cpu",
            args: BTreeMap::from([
                ("context", event.context.into()),
                ("source", "queue_completion_callback".into()),
            ]),
        });
    }
}

impl Span {
    pub fn enabled(&self) -> bool {
        self.0.is_some()
    }

    pub fn arg(mut self, key: &'static str, value: impl Into<Value>) -> Self {
        self.set_arg(key, value);
        self
    }

    pub fn set_arg(&mut self, key: &'static str, value: impl Into<Value>) {
        let Some(active) = &mut self.0 else { return };
        if active.args.len() >= MAX_ARGS {
            return;
        }
        let mut value = value.into();
        if let Value::String(text) = &mut value {
            if text.len() > MAX_STRING {
                let mut end = MAX_STRING;
                while !text.is_char_boundary(end) {
                    end -= 1;
                }
                text.truncate(end);
            }
        } else if !matches!(value, Value::Number(_) | Value::Bool(_) | Value::Null) {
            return;
        }
        active.args.insert(key, value);
    }

    fn finish_at(&mut self, now: Instant) {
        let Some(active) = self.0.take() else { return };
        let mut capture = active.shared.lock();
        if capture.epoch != active.epoch {
            return;
        }
        if capture.events.len() >= CAPACITY {
            capture.dropped = capture.dropped.saturating_add(1);
            return;
        }
        let end = capture.stop.map_or(now, |stop| stop.min(now));
        capture.events.push_back(Event {
            name: active.name,
            cat: "native.cpu",
            ts: active.ts,
            dur: if active.instant {
                0.
            } else {
                end.saturating_duration_since(active.started).as_secs_f64() * 1_000_000.
            },
            args: active.args,
        });
    }
}

impl Drop for Span {
    fn drop(&mut self) {
        if self.0.is_some() {
            self.finish_at(Instant::now());
        }
    }
}

pub fn span(world: &World, name: &'static str) -> Span {
    world
        .get_resource::<Profiler>()
        .map_or_else(Span::default, |profiler| profiler.span(name))
}

pub fn instant(world: &World, name: &'static str) -> Span {
    world
        .get_resource::<Profiler>()
        .map_or_else(Span::default, |profiler| profiler.instant(name))
}

#[derive(Resource, Default)]
struct ScheduleSpans {
    current: Option<Span>,
}

fn begin_main(profiler: Res<Profiler>, mut spans: ResMut<ScheduleSpans>) {
    if profiler.enabled() {
        spans.current = Some(profiler.span("bevy.main.First-to-Last"));
    }
}
fn begin_render(profiler: Res<Profiler>, mut spans: ResMut<ScheduleSpans>) {
    if profiler.enabled() {
        spans.current = Some(profiler.span("bevy.render"));
    }
}
fn end_schedule(mut spans: ResMut<ScheduleSpans>) {
    spans.current = None;
}

pub fn install(app: &mut App, profiler: Profiler) {
    use bevy::render::{Render, RenderApp, RenderSystems};
    app.insert_resource(profiler.clone())
        .init_resource::<ScheduleSpans>()
        .add_systems(First, begin_main)
        .add_systems(Last, end_schedule);
    if let Some(render) = app.get_sub_app_mut(RenderApp) {
        render
            .insert_resource(profiler)
            .init_resource::<ScheduleSpans>()
            .add_systems(Render, begin_render.before(RenderSystems::ExtractCommands))
            .add_systems(Render, end_schedule.after(RenderSystems::PostCleanup));
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::Duration;

    fn events(profiler: &Profiler) -> Value {
        serde_json::from_str(&profiler.drain(MAX_DRAIN)).unwrap()
    }

    #[test]
    fn host_microseconds_and_nested_spans_share_one_clock() {
        let profiler = Profiler::default();
        let origin = Instant::now();
        profiler.set_at(true, 1250., origin);
        let mut outer = profiler.span_at("frame", origin + Duration::from_millis(2));
        let mut inner = profiler
            .span_at("terrain", origin + Duration::from_millis(3))
            .arg("rows", 32);
        inner.finish_at(origin + Duration::from_millis(4));
        outer.finish_at(origin + Duration::from_millis(8));
        let result = events(&profiler);
        assert_eq!(result["events"][0]["ts"], 1_253_000.);
        assert_eq!(result["events"][0]["dur"], 1000.);
        assert_eq!(result["events"][0]["args"]["rows"], 32);
        assert_eq!(result["events"][1]["ts"], 1_252_000.);
        assert_eq!(result["events"][1]["dur"], 6000.);
        assert_eq!(events(&profiler)["events"], serde_json::json!([]));
    }

    #[test]
    fn disabled_is_empty_and_stop_preserves_and_clips_in_flight_events() {
        let profiler = Profiler::default();
        assert!(!profiler.span("disabled").enabled());
        let origin = Instant::now();
        profiler.set_at(true, 10., origin);
        let mut span = profiler.span_at("work", origin + Duration::from_millis(1));
        profiler.set_at(false, 0., origin + Duration::from_millis(5));
        span.finish_at(origin + Duration::from_millis(9));
        assert!(!profiler.span("after stop").enabled());
        let result = events(&profiler);
        assert_eq!(result["events"][0]["dur"], 4000.);
    }

    #[test]
    fn start_clears_previous_capture_and_late_spans_cannot_cross_epochs() {
        let profiler = Profiler::default();
        let origin = Instant::now();
        profiler.set_at(true, 0., origin);
        let mut completed = profiler.span_at("old completed", origin);
        completed.finish_at(origin);
        let mut old = profiler.span_at("old", origin);
        profiler.set_at(true, 20., origin + Duration::from_millis(1));
        old.finish_at(origin + Duration::from_millis(2));
        let mut fresh = profiler.span_at("new", origin + Duration::from_millis(3));
        fresh.finish_at(origin + Duration::from_millis(4));
        let result = events(&profiler);
        assert_eq!(result["events"].as_array().unwrap().len(), 1);
        assert_eq!(result["events"][0]["name"], "new");
        assert_eq!(result["events"][0]["ts"], 22_000.);
    }

    #[test]
    fn event_buffer_is_bounded_and_drain_accounts_for_loss() {
        let profiler = Profiler::default();
        let origin = Instant::now();
        profiler.set_at(true, 0., origin);
        for _ in 0..CAPACITY + 3 {
            let mut span = profiler.span_at("work", origin);
            span.finish_at(origin);
        }
        let result = events(&profiler);
        assert_eq!(result["events"].as_array().unwrap().len(), MAX_DRAIN);
        assert_eq!(result["dropped"], 3);
        assert_eq!(result["remaining"], CAPACITY - MAX_DRAIN);
        let mut drained = MAX_DRAIN;
        while drained < CAPACITY {
            let batch = events(&profiler);
            drained += batch["events"].as_array().unwrap().len();
            assert_eq!(batch["dropped"], 0);
        }
        assert_eq!(drained, CAPACITY);
        assert_eq!(events(&profiler)["dropped"], 0);
    }

    #[test]
    fn harvest_caps_requests_and_preserves_fifo_events_after_stop() {
        let profiler = Profiler::default();
        let origin = Instant::now();
        profiler.set_at(true, 0., origin);
        for index in 0..300 {
            let mut span = profiler.span_at("ordered", origin).arg("index", index);
            span.finish_at(origin);
        }
        profiler.set_at(false, 0., origin);
        let first: Value = serde_json::from_str(&profiler.drain(usize::MAX)).unwrap();
        assert_eq!(first["events"].as_array().unwrap().len(), 128);
        assert_eq!(first["events"][0]["args"]["index"], 0);
        assert_eq!(first["events"][127]["args"]["index"], 127);
        assert_eq!(first["events"][0]["cat"], "native.cpu");
        assert_eq!(first["remaining"], 172);
        let second: Value = serde_json::from_str(&profiler.drain(17)).unwrap();
        assert_eq!(second["events"].as_array().unwrap().len(), 17);
        assert_eq!(second["events"][0]["args"]["index"], 128);
        assert_eq!(second["remaining"], 155);
    }

    #[test]
    fn contexts_are_captured_when_spans_begin_and_instant_events_have_zero_duration() {
        let profiler = Profiler::default();
        let origin = Instant::now();
        profiler.set_at(true, 0., origin);
        profiler.set_context(7);
        let mut first = profiler.span_at("old revision", origin);
        profiler.set_context(8);
        let mut second = profiler.span_at("new revision", origin);
        first.finish_at(origin + Duration::from_millis(1));
        second.finish_at(origin + Duration::from_millis(2));
        drop(profiler.instant("publication").arg("landscapeRevision", 12));
        let result = events(&profiler);
        assert_eq!(result["events"][0]["args"]["context"], 7);
        assert_eq!(result["events"][1]["args"]["context"], 8);
        assert_eq!(result["events"][2]["args"]["context"], 8);
        assert_eq!(result["events"][2]["args"]["landscapeRevision"], 12);
        assert_eq!(result["events"][2]["dur"], 0.);
    }

    #[test]
    fn metadata_stays_flat_bounded_and_valid_utf8() {
        let profiler = Profiler::default();
        let origin = Instant::now();
        profiler.set_at(true, 0., origin);
        let mut span = profiler
            .span_at("bounded", origin)
            .arg("long", "é".repeat(400))
            .arg("nested", serde_json::json!({"not": "flat"}));
        for key in [
            "a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p",
        ] {
            span.set_arg(key, 1);
        }
        span.finish_at(origin);
        let result = events(&profiler);
        let args = result["events"][0]["args"].as_object().unwrap();
        assert_eq!(args.len(), MAX_ARGS);
        assert!(!args.contains_key("nested"));
        assert_eq!(args["long"].as_str().unwrap().len(), MAX_STRING);
    }

    #[test]
    fn readiness_callback_uses_its_scene_context_and_completion_timestamp() {
        let profiler = Profiler::default();
        let origin = Instant::now();
        profiler.set_at(true, 100., origin);
        profiler.set_context(3);
        let ready = profiler.deferred("scene.ready");
        profiler.set_context(4);
        ready.emit_at(origin + Duration::from_millis(25));
        let result = events(&profiler);
        assert_eq!(result["events"][0]["ts"], 125_000.);
        assert_eq!(result["events"][0]["dur"], 0.);
        assert_eq!(result["events"][0]["args"]["context"], 3);
        let stopped = profiler.deferred("stopped");
        profiler.set_at(false, 0., origin + Duration::from_millis(30));
        stopped.emit_at(origin + Duration::from_millis(40));
        assert_eq!(events(&profiler)["events"], serde_json::json!([]));
    }
}
