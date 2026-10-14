import type { ReactNode } from "react";
import CloseButton from "@/components/ui/buttons/CloseButton.tsx";
import { Z_INDEX } from "@/constants/zIndex.ts";
import { useRenderPause } from "@/stores/renderPauseStore.ts";
import { useUIOverlayStore } from "@/stores/uiOverlayStore.ts";
import ServerList from "./ServerList.tsx";

/**
 * Phone server switcher, opened from the menu drawer. Same layout as the
 * gateway's own server selector on phones: a top bar, then a divided list.
 */
export default function ServerSelectorScreen() {
  const open = useUIOverlayStore((s) => s.showServerSelector);
  const setOpen = useUIOverlayStore((s) => s.setShowServerSelector);
  useRenderPause("server-selector", open);
  if (!open) {
    return null;
  }
  return (
    <ServerSelectorScreenView onClose={() => setOpen(false)}>
      <ServerList active={open} rowClassName="min-h-12 !px-1 border-b border-white/10" />
    </ServerSelectorScreenView>
  );
}

export function ServerSelectorScreenView({
  onClose,
  children,
}: {
  onClose: () => void;
  children: ReactNode;
}) {
  return (
    <div
      className="fixed inset-0 overflow-y-auto bg-black text-white font-orbitron"
      style={{ zIndex: Z_INDEX.MOBILE_MENU_DRAWER }}
    >
      <header className="sticky top-0 flex items-center gap-3 border-b border-white/10 bg-black pt-[calc(14px+var(--safe-top))] pb-[14px] pr-[calc(8px+var(--safe-right))] pl-[calc(16px+var(--safe-left))]">
        <h1 className="m-0 text-base font-bold text-shadow-glow-strong">OPEN MARS</h1>
        <span className="flex-1 text-[0.7rem] font-medium uppercase tracking-[0.15em] text-white/45">
          Server selector
        </span>
        <CloseButton onClick={onClose} label="Close server selector" />
      </header>
      <div className="flex flex-col pr-[calc(16px+var(--safe-right))] pb-[var(--safe-bottom)] pl-[calc(16px+var(--safe-left))]">
        {children}
      </div>
    </div>
  );
}
