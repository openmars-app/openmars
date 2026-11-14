import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Z_INDEX } from "@/constants/zIndex";
import OpenMarsLogo from "@/components/ui/OpenMarsLogo.tsx";

interface LoadingOverlayProps {
  message?: string;
  subtitle?: string;
  isLoaded: boolean;
  onTransitionEnd?: () => void;
  /**
   * Wait this many milliseconds before showing the overlay. If the load
   * finishes before this elapses, the overlay is skipped entirely so fast
   * transitions don't briefly flash a black panel.
   */
  showDelayMs?: number;
  /**
   * Once shown, keep the overlay visible at least this long even if the
   * load finishes quickly. Prevents a jarring pop-out on medium-speed loads.
   */
  minDurationMs?: number;
  fadeDurationMs?: number;
  /** Optional measured fraction in [0, 1]; omitted work is indeterminate. */
  progress?: number;
  showProgress?: boolean;
}

type Phase = "waiting" | "showing" | "fading" | "done";

export default function LoadingOverlay({
  message,
  subtitle,
  isLoaded,
  onTransitionEnd,
  showDelayMs = 500,
  minDurationMs = 200,
  fadeDurationMs = 800,
  showProgress = false,
  progress,
}: LoadingOverlayProps) {
  const [phase, setPhase] = useState<Phase>(() => (showDelayMs === 0 ? "showing" : "waiting"));
  const shownAtRef = useRef<number | null>(showDelayMs === 0 ? Date.now() : null);
  const ref = useRef<HTMLDivElement>(null);
  const detail = subtitle || message || "Loading";

  useEffect(() => {
    if (phase !== "waiting") {
      return;
    }
    const id = window.setTimeout(() => {
      shownAtRef.current = Date.now();
      setPhase("showing");
    }, showDelayMs);
    return () => window.clearTimeout(id);
  }, [phase, showDelayMs]);

  useEffect(() => {
    if (!isLoaded) {
      return;
    }
    if (phase === "waiting") {
      setPhase("done");
      onTransitionEnd?.();
      return;
    }
    if (phase !== "showing") {
      return;
    }
    const shownAt = shownAtRef.current ?? Date.now();
    const elapsed = Date.now() - shownAt;
    const remaining = Math.max(0, minDurationMs - elapsed);
    if (remaining === 0) {
      setPhase("fading");
      return;
    }
    const id = window.setTimeout(() => setPhase("fading"), remaining);
    return () => window.clearTimeout(id);
  }, [isLoaded, phase, minDurationMs, onTransitionEnd]);

  useEffect(() => {
    if (phase !== "fading" || !ref.current) {
      return;
    }
    const animation = ref.current.animate([{ opacity: 1 }, { opacity: 0 }], {
      duration: fadeDurationMs,
      easing: "ease-out",
      fill: "forwards",
    });
    void animation.finished.then(
      () => {
        setPhase("done");
        onTransitionEnd?.();
      },
      () => {},
    );
    return () => animation.cancel();
  }, [phase, onTransitionEnd, fadeDurationMs]);

  useLayoutEffect(() => {
    if (!isLoaded && (phase === "fading" || phase === "done")) {
      shownAtRef.current = Date.now();
      setPhase("showing");
    }
  }, [isLoaded, phase]);

  if (phase === "waiting" || phase === "done") {
    return null;
  }

  return (
    <div
      ref={ref}
      role="status"
      aria-live="polite"
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        width: "100vw",
        height: "100dvh",
        backgroundColor: "#000000",
        zIndex: Z_INDEX.LOADING_OVERLAY,
        opacity: 1,
        pointerEvents: phase === "fading" ? "none" : "auto",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        color: "white",
        fontSize: "18px",
        fontFamily: "Orbitron, sans-serif",
      }}
    >
      <div className="mb-12 w-[min(520px,calc(100%-48px))] text-center compact:mb-6 compact:w-[340px] compact:max-w-[calc(100%-48px)]">
        <OpenMarsLogo surface="loading" />
      </div>
      {showProgress && progress !== undefined ? (
        <div
          role="progressbar"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={Math.round(progress * 100)}
          className="mb-4 w-[220px] text-center"
        >
          <div className="mb-2 text-3xl font-bold">{Math.round(progress * 100)}%</div>
          <div className="h-[3px] bg-white/10">
            <div
              className="h-full bg-white"
              style={{ width: `${Math.max(0, Math.min(1, progress)) * 100}%` }}
            />
          </div>
        </div>
      ) : (
        <>
          <div
            style={{
              width: "40px",
              height: "40px",
              border: "4px solid rgba(255, 255, 255, 0.1)",
              borderTop: "4px solid white",
              borderRadius: "50%",
              animation: "spin 1s linear infinite",
              marginBottom: "16px",
            }}
          />
          <style>
            {`
          @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
          }
        `}
          </style>
        </>
      )}
      {!showProgress && (message || "Loading")}
      {(subtitle || showProgress) && (
        <div
          style={{
            marginTop: "8px",
            fontSize: "13px",
            color: "rgba(255, 255, 255, 0.4)",
            fontFamily: "Orbitron, sans-serif",
          }}
        >
          {showProgress ? detail : subtitle}
        </div>
      )}
    </div>
  );
}
