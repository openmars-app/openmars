//! Bound queued canvas frames without waiting for every frame's GPU acknowledgment.
use bevy::platform::time::Instant;
use std::sync::{
    Arc, Mutex,
    atomic::{AtomicU64, Ordering},
};
use std::time::Duration;

/// Four slots can overlap roughly 16 ms acknowledgments at 240 Hz. This is a
/// backlog bound, not a frame-rate target or a measurement of GPU execution time.
pub const MAX_IN_FLIGHT: u64 = 4;

// Acknowledgments include GPU work and browser scheduling/IPC, not GPU time.
// Preserve four slots for ordinary high-refresh latency, including a controlled
// 60 ms acknowledgment delay. Very slow acknowledgments drain to one slot so a
// slow/software GPU does not consume the browser's whole rendering budget.
const SLOW_ACK: Duration = Duration::from_millis(120);
const FAST_ACK: Duration = Duration::from_millis(40);

struct Policy {
    budget: u64,
    acknowledgment_ms: Option<f64>,
}

impl Default for Policy {
    fn default() -> Self {
        Self {
            budget: MAX_IN_FLIGHT,
            acknowledgment_ms: None,
        }
    }
}

#[derive(Default)]
struct Completed {
    serial: AtomicU64,
    // Serialize watermark/policy publication so an older callback cannot undo a
    // newer decision, even when the helper is exercised from native threads.
    policy: Mutex<Policy>,
}

/// One engine-lifetime timeline, independent of scene/readiness generations.
#[derive(Default)]
pub struct FramePacer {
    submitted: u64,
    completed: Arc<Completed>,
}

impl FramePacer {
    pub fn submitted(&self) -> u64 {
        self.submitted
    }

    pub fn completed(&self) -> u64 {
        self.completed.serial.load(Ordering::Acquire)
    }

    pub fn pending(&self) -> u64 {
        self.submitted - self.completed()
    }

    pub fn can_submit(&self) -> bool {
        self.pending() < self.budget()
    }

    pub fn budget(&self) -> u64 {
        self.completed.policy.lock().unwrap().budget
    }

    pub fn acknowledgment_ms(&self) -> Option<f64> {
        self.completed.policy.lock().unwrap().acknowledgment_ms
    }

    /// Call once after presenting an admitted logical frame. A frame may contain
    /// several ordered queue submissions, all covered by its completion callback.
    pub fn submit(&mut self) -> Completion {
        // Admission was checked before rendering. A callback may reduce the soft
        // budget in between; the presented frame still consumes its admitted slot.
        assert!(
            self.pending() < MAX_IN_FLIGHT,
            "Canvas frame backlog exceeded its bound"
        );
        self.submitted = self
            .submitted
            .checked_add(1)
            .expect("Canvas frame serial exhausted");
        Completion {
            serial: self.submitted,
            completed: self.completed.clone(),
            started: Instant::now(),
        }
    }
}

/// The callback owns only its original timeline, so late callbacks cannot release
/// slots belonging to a replacement engine. A later serial completes all earlier
/// work on the same queue; delayed earlier callbacks must never regress it.
pub struct Completion {
    serial: u64,
    completed: Arc<Completed>,
    started: Instant,
}

impl Completion {
    pub fn complete(self) {
        let latency = self.started.elapsed();
        self.complete_with_latency(latency);
    }

    fn complete_with_latency(self, latency: Duration) {
        let mut policy = self.completed.policy.lock().unwrap();
        if self.serial <= self.completed.serial.load(Ordering::Acquire) {
            return;
        }
        policy.acknowledgment_ms = Some(latency.as_secs_f64() * 1000.0);
        if latency > SLOW_ACK {
            policy.budget = 1;
        } else if latency < FAST_ACK {
            policy.budget = MAX_IN_FLIGHT;
        }
        self.completed.serial.store(self.serial, Ordering::Release);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn caps_pending_frames_and_reopens_only_acknowledged_slots() {
        let mut frames = FramePacer::default();
        let mut callbacks: Vec<_> = (0..MAX_IN_FLIGHT).map(|_| frames.submit()).collect();
        assert_eq!(frames.submitted(), MAX_IN_FLIGHT);
        assert_eq!(frames.pending(), MAX_IN_FLIGHT);
        assert!(!frames.can_submit());
        callbacks
            .remove(0)
            .complete_with_latency(Duration::from_millis(16));
        assert_eq!(frames.pending(), MAX_IN_FLIGHT - 1);
        assert!(frames.can_submit());
        let _next = frames.submit();
        assert_eq!(frames.pending(), MAX_IN_FLIGHT);
        assert!(!frames.can_submit());
    }

    #[test]
    fn completion_watermark_cannot_regress_when_callbacks_arrive_out_of_order() {
        let mut frames = FramePacer::default();
        let first = frames.submit();
        let second = frames.submit();
        let third = frames.submit();
        third.complete();
        assert_eq!(frames.completed(), 3);
        assert_eq!(frames.pending(), 0);
        first.complete();
        second.complete();
        assert_eq!(frames.completed(), 3);
        let _fourth = frames.submit();
        assert_eq!(frames.pending(), 1);
    }

    #[test]
    fn a_late_callback_never_releases_another_engines_slots() {
        let mut old = FramePacer::default();
        let late = old.submit();
        drop(old);
        let mut next = FramePacer::default();
        let _next_callbacks: Vec<_> = (0..MAX_IN_FLIGHT).map(|_| next.submit()).collect();
        late.complete();
        assert_eq!(next.completed(), 0);
        assert_eq!(next.pending(), MAX_IN_FLIGHT);
        assert!(!next.can_submit());
    }

    #[test]
    fn slow_acknowledgments_drain_old_frames_before_fast_recovery() {
        let mut frames = FramePacer::default();
        let first = frames.submit();
        let second = frames.submit();
        let third = frames.submit();
        let fourth = frames.submit();
        assert_eq!(frames.acknowledgment_ms(), None);
        first.complete_with_latency(Duration::from_millis(121));
        assert_eq!(frames.budget(), 1);
        assert_eq!(frames.acknowledgment_ms(), Some(121.0));
        assert_eq!(frames.pending(), 3);
        assert!(!frames.can_submit());
        second.complete_with_latency(Duration::from_millis(200));
        third.complete_with_latency(Duration::from_millis(200));
        assert_eq!(frames.pending(), 1);
        assert!(!frames.can_submit());
        fourth.complete_with_latency(Duration::from_millis(200));
        assert_eq!(frames.pending(), 0);
        assert!(frames.can_submit());
        let fifth = frames.submit();
        assert!(!frames.can_submit());
        fifth.complete_with_latency(Duration::from_millis(16));
        assert_eq!(frames.budget(), MAX_IN_FLIGHT);
        assert_eq!(frames.acknowledgment_ms(), Some(16.0));
        assert!(frames.can_submit());
    }

    #[test]
    fn middle_latency_preserves_the_previous_budget_with_strict_thresholds() {
        let mut frames = FramePacer::default();
        for milliseconds in [60, 120] {
            frames
                .submit()
                .complete_with_latency(Duration::from_millis(milliseconds));
            assert_eq!(frames.budget(), MAX_IN_FLIGHT);
        }
        frames
            .submit()
            .complete_with_latency(Duration::from_millis(121));
        assert_eq!(frames.budget(), 1);
        for milliseconds in [60, 40] {
            frames
                .submit()
                .complete_with_latency(Duration::from_millis(milliseconds));
            assert_eq!(frames.budget(), 1);
        }
        frames
            .submit()
            .complete_with_latency(Duration::from_millis(39));
        assert_eq!(frames.budget(), MAX_IN_FLIGHT);
    }

    #[test]
    fn older_callbacks_cannot_restore_stale_policy_or_latency() {
        let mut frames = FramePacer::default();
        let old_fast = frames.submit();
        let new_slow = frames.submit();
        new_slow.complete_with_latency(Duration::from_millis(300));
        old_fast.complete_with_latency(Duration::from_millis(1));
        assert_eq!(frames.completed(), 2);
        assert_eq!(frames.budget(), 1);
        assert_eq!(frames.acknowledgment_ms(), Some(300.0));
        let old_slow = frames.submit();
        // Simulate a frame admitted immediately before a callback reduced the
        // budget: submit enforces the hard bound, not the current soft budget.
        let new_fast = frames.submit();
        new_fast.complete_with_latency(Duration::from_millis(16));
        old_slow.complete_with_latency(Duration::from_millis(400));
        assert_eq!(frames.completed(), 4);
        assert_eq!(frames.pending(), 0);
        assert_eq!(frames.budget(), MAX_IN_FLIGHT);
        assert_eq!(frames.acknowledgment_ms(), Some(16.0));
    }

    #[test]
    #[should_panic(expected = "Canvas frame backlog exceeded its bound")]
    fn rejects_a_submission_without_capacity() {
        let mut frames = FramePacer::default();
        let _callbacks: Vec<_> = (0..MAX_IN_FLIGHT).map(|_| frames.submit()).collect();
        frames.submit();
    }
}
