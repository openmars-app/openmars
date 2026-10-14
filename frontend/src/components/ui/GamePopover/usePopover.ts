import { useOverlayEventTarget, isPreviewEvent } from "@/contexts/OverlayEnvironment";
import { useEffect, RefObject } from "react";

export function usePopover({
  isVisible,
  onClose,
  popoverRef,
  anchorRef,
}: {
  isVisible: boolean;
  onClose: () => void;
  popoverRef: RefObject<HTMLElement | null>;
  anchorRef?: RefObject<HTMLElement | null>;
}) {
  const eventTarget = useOverlayEventTarget();
  useEffect(() => {
    if (!isVisible) {
      return;
    }

    const handleClickOutside = (e: PointerEvent) => {
      if (eventTarget === document && isPreviewEvent(e)) return;
      const target = e.target as HTMLElement;
      if (target.closest?.("[data-overlay-layer]")) {
        return;
      }
      const outsidePopover = popoverRef.current && !popoverRef.current.contains(target);
      const outsideAnchor = !anchorRef?.current || !anchorRef.current.contains(target);
      if (outsidePopover && outsideAnchor) {
        onClose();
      }
    };

    const handleKeyDown = (e: KeyboardEvent) => {
      if (
        eventTarget !== document &&
        e.target instanceof Node &&
        !popoverRef.current?.contains(e.target)
      )
        return;
      if (eventTarget === document && isPreviewEvent(e)) return;
      if (e.key !== "Escape" || e.defaultPrevented) {
        return;
      }
      e.preventDefault();
      onClose();
    };

    eventTarget.addEventListener("pointerdown", handleClickOutside);
    eventTarget.addEventListener("keydown", handleKeyDown);
    return () => {
      eventTarget.removeEventListener("pointerdown", handleClickOutside);
      eventTarget.removeEventListener("keydown", handleKeyDown);
    };
  }, [eventTarget, isVisible, onClose, popoverRef, anchorRef]);
}
