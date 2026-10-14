import { type ReactNode, useEffect, useRef } from "react";
import { createPortal } from "react-dom";
import { useOverlayPortalRoot } from "@/contexts/OverlayEnvironment";
import CardBrowser from "../CardBrowser.tsx";
import { Z_INDEX } from "@/constants/zIndex.ts";

interface CardBrowserOverlayProps {
  isVisible: boolean;
  onClose: () => void;
}

export default function CardBrowserOverlay({ isVisible, onClose }: CardBrowserOverlayProps) {
  return (
    <CardBrowserOverlayView isVisible={isVisible} onClose={onClose}>
      <CardBrowser onBack={onClose} backLabel="Back to Game" />
    </CardBrowserOverlayView>
  );
}

export function CardBrowserOverlayView({
  isVisible,
  onClose,
  children,
}: CardBrowserOverlayProps & { children: ReactNode }) {
  const portalRoot = useOverlayPortalRoot();
  const panel = useRef<HTMLDivElement>(null);
  const close = useRef(onClose);
  useEffect(() => {
    close.current = onClose;
  }, [onClose]);
  useEffect(() => {
    if (!isVisible) {
      return;
    }
    const previous = document.activeElement as HTMLElement | null;
    const focusable = () =>
      Array.from(
        panel.current?.querySelectorAll<HTMLElement>(
          'button:not(:disabled), input, select, summary, [tabindex="0"]',
        ) ?? [],
      ).filter(
        (element) =>
          element.getClientRects().length > 0 &&
          !element.closest("[inert]") &&
          getComputedStyle(element).visibility !== "hidden",
      );
    focusable()[0]?.focus();
    const handleKey = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !event.defaultPrevented) {
        event.preventDefault();
        event.stopPropagation();
        close.current();
      } else if (event.key === "Tab") {
        const elements = focusable();
        const first = elements[0];
        const last = elements[elements.length - 1];
        if (event.shiftKey && document.activeElement === first) {
          event.preventDefault();
          last?.focus();
        } else if (!event.shiftKey && document.activeElement === last) {
          event.preventDefault();
          first?.focus();
        }
      }
    };
    panel.current?.addEventListener("keydown", handleKey);
    const element = panel.current;
    return () => {
      element?.removeEventListener("keydown", handleKey);
      previous?.focus();
    };
  }, [isVisible]);

  if (!isVisible) {
    return null;
  }
  return createPortal(
    <div
      ref={panel}
      data-overlay-layer
      role="dialog"
      aria-modal="true"
      aria-label="Card browser"
      className="fixed inset-0 overflow-hidden bg-black"
      style={{ zIndex: Z_INDEX.APP_OVERLAY }}
    >
      {children}
    </div>,
    portalRoot,
  );
}
