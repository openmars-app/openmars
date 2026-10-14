import type { RefObject, ReactNode } from "react";
import { GamePopover } from "../GamePopover";
import type { PopoverPosition } from "../GamePopover/types";
import ServerList from "@/components/ui/servers/ServerList.tsx";

/** Bottom-left, above the menu footer the trigger lives in */
const FOOTER_POSITION: PopoverPosition = { type: "fixed", left: 16, bottom: 64 };

interface ServerSwitchPopoverProps {
  isVisible: boolean;
  onClose: () => void;
  anchorRef: RefObject<HTMLElement | null>;
}

/** Desktop server switcher; phones get ServerSelectorScreen from the menu drawer. */
export default function ServerSwitchPopover({
  isVisible,
  onClose,
  anchorRef,
}: ServerSwitchPopoverProps) {
  return (
    <ServerSwitchPopoverView isVisible={isVisible} onClose={onClose} anchorRef={anchorRef}>
      <ServerList active={isVisible} rowClassName="px-4 py-3 text-sm" />
    </ServerSwitchPopoverView>
  );
}

export function ServerSwitchPopoverView({
  isVisible,
  onClose,
  anchorRef,
  children,
}: ServerSwitchPopoverProps & { children: ReactNode }) {
  return (
    <GamePopover
      className="game-popover-list"
      isVisible={isVisible}
      onClose={onClose}
      position={FOOTER_POSITION}
      theme="menu"
      header={{ title: "Server" }}
      width={300}
      maxHeight="auto"
      animation="slideUp"
      excludeRef={anchorRef}
    >
      <div className="py-1">{children}</div>
    </GamePopover>
  );
}
