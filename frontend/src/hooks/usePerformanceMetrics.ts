import { useState, useEffect } from "react";
import { performanceStore, PerformanceSnapshot } from "@/services/performanceStore.ts";

export function usePerformanceMetrics(enabled = true) {
  const [snapshots, setSnapshots] = useState<PerformanceSnapshot[]>([]);

  useEffect(() => {
    if (!enabled) {
      setSnapshots([]);
      return;
    }
    performanceStore.start();
    const unsubscribe = performanceStore.subscribe(setSnapshots);
    return () => {
      unsubscribe();
      performanceStore.stop();
    };
  }, [enabled]);

  const latest = snapshots.length > 0 ? snapshots[snapshots.length - 1] : null;

  return { snapshots, latest };
}
