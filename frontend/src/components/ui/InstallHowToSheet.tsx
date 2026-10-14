import { useOverlayPortalRoot } from "@/contexts/OverlayEnvironment";
import React from "react";
import { createPortal } from "react-dom";
import CloseButton from "./buttons/CloseButton.tsx";
import { Z_INDEX } from "@/constants/zIndex.ts";
import { useBackDismiss } from "@/hooks/useBackDismiss.ts";
import { useReducedMotion } from "@/hooks/useReducedMotion.ts";
import { useInstallAppStore } from "@/stores/installAppStore.ts";
import { installPlatform } from "@/utils/installApp.ts";

const ShareGlyph: React.FC = () => (
  <svg
    width={18}
    height={18}
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth={2}
    strokeLinecap="round"
    strokeLinejoin="round"
    aria-hidden="true"
    className="inline-block align-[-3px] mx-1"
  >
    <path d="M12 3v12" />
    <polyline points="8 7 12 3 16 7" />
    <path d="M8 10H6a1 1 0 0 0-1 1v9a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-9a1 1 0 0 0-1-1h-2" />
  </svg>
);

const MoreGlyph: React.FC = () => (
  <svg
    width={18}
    height={18}
    viewBox="0 0 24 24"
    fill="currentColor"
    aria-hidden="true"
    className="inline-block align-[-3px] mx-1"
  >
    <circle cx="12" cy="5" r="2" />
    <circle cx="12" cy="12" r="2" />
    <circle cx="12" cy="19" r="2" />
  </svg>
);

const IOS_STEPS: React.ReactNode[] = [
  <>
    Tap <strong className="font-semibold text-white">Share</strong>
    <ShareGlyph />
  </>,
  <>
    Choose <strong className="font-semibold text-white">Add to Home Screen</strong>
  </>,
  <>
    Tap <strong className="font-semibold text-white">Add</strong>
  </>,
];

const FIREFOX_ANDROID_STEPS: React.ReactNode[] = [
  <>
    Open the
    <MoreGlyph />
    <strong className="font-semibold text-white">menu</strong>
  </>,
  <>
    Tap <strong className="font-semibold text-white">Install</strong>
  </>,
];

const closeSheet = () => useInstallAppStore.getState().setHowToOpen(false);

export default function InstallHowToSheet() {
  const portalRoot = useOverlayPortalRoot();
  const open = useInstallAppStore((s) => s.howToOpen);
  if (!open) {
    return null;
  }
  return createPortal(<LiveHowToSheet />, portalRoot);
}

function LiveHowToSheet() {
  useBackDismiss(closeSheet);
  return <InstallHowToSheetView platform={installPlatform()} onClose={closeSheet} />;
}

export function InstallHowToSheetView({
  platform,
  onClose,
}: {
  platform: ReturnType<typeof installPlatform>;
  onClose: () => void;
}) {
  const reducedMotion = useReducedMotion();
  let steps = IOS_STEPS;
  if (platform === "firefox-android") {
    steps = FIREFOX_ANDROID_STEPS;
  }

  return (
    <div data-overlay-layer className="fixed inset-0" style={{ zIndex: Z_INDEX.APP_OVERLAY }}>
      <div
        aria-hidden="true"
        className={`absolute inset-0 bg-black/60 ${reducedMotion ? "" : "animate-[fadeIn_180ms_ease-out]"}`}
        onClick={onClose}
      />
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Install the app"
        className={`game-panel absolute inset-x-0 bottom-0 mx-auto w-full max-w-[480px] text-white ${reducedMotion ? "" : "animate-[sheetSlideUp_220ms_ease-out]"}`}
        style={{
          paddingBottom: "calc(16px + var(--safe-bottom))",
          paddingLeft: "calc(16px + var(--safe-left))",
          paddingRight: "calc(4px + var(--safe-right))",
        }}
      >
        <div className="flex items-center justify-between h-12">
          <h2 className="m-0 font-orbitron text-sm font-bold tracking-wider uppercase text-shadow-glow">
            Install the app
          </h2>
          <CloseButton onClick={onClose} label="Close" />
        </div>
        <ol className="m-0 p-0 pr-3 list-none flex flex-col gap-3">
          {steps.map((step, index) => (
            <li key={index} className="flex items-center gap-3 text-sm text-white/80">
              <span className="flex items-center justify-center w-7 h-7 shrink-0 rounded-full border border-white/30 font-orbitron text-xs text-white">
                {index + 1}
              </span>
              <span>{step}</span>
            </li>
          ))}
        </ol>
      </div>
    </div>
  );
}
