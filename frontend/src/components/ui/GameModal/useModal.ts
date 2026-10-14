import { useOverlayEventTarget, isPreviewEvent } from "@/contexts/OverlayEnvironment";
import { useEffect, useRef, type RefObject } from "react";

interface UseModalOptions {
  modalRef: RefObject<HTMLDivElement | null>;
  isVisible: boolean;
  onClose: () => void;
  closeOnEscape?: boolean;
  lockScroll?: boolean;
  preventClose?: boolean;
  onPreventedClose?: () => void;
}

export function useModal({
  modalRef,
  isVisible,
  onClose,
  closeOnEscape = true,
  lockScroll = true,
  preventClose = false,
  onPreventedClose,
}: UseModalOptions) {
  const eventTarget = useOverlayEventTarget();
  const callbacks = useRef({ onClose, onPreventedClose });
  callbacks.current = { onClose, onPreventedClose };
  useEffect(() => {
    if (!isVisible) return;

    const previousFocus = document.activeElement;
    const modal = modalRef.current;
    const focusable = () =>
      Array.from(
        modal?.querySelectorAll<HTMLElement>(
          'button:not(:disabled), a[href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex="0"]',
        ) ?? [],
      ).filter((element) => element.getClientRects().length > 0);
    (focusable()[0] ?? modal)?.focus();
    const handleEscape = (e: KeyboardEvent) => {
      if (eventTarget === document && isPreviewEvent(e)) return;
      // A nested portal owns keyboard input until it is dismissed.
      if (eventTarget !== document && e.target instanceof Node && !modal?.contains(e.target))
        return;
      if (e.key === "Tab" && eventTarget === document) {
        const elements = focusable();
        const first = elements[0];
        const last = elements[elements.length - 1];
        if (!first) {
          e.preventDefault();
          modal?.focus();
        } else if (
          e.shiftKey &&
          (document.activeElement === first || document.activeElement === modal)
        ) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
      if (e.key === "Escape" && !e.defaultPrevented && (preventClose || closeOnEscape)) {
        e.preventDefault();
        if (preventClose) {
          callbacks.current.onPreventedClose?.();
        } else if (closeOnEscape) {
          callbacks.current.onClose();
        }
      }
    };

    eventTarget.addEventListener("keydown", handleEscape);

    const previousOverflow = document.body.style.overflow;
    if (lockScroll && eventTarget === document) {
      document.body.style.overflow = "hidden";
    }

    return () => {
      eventTarget.removeEventListener("keydown", handleEscape);
      if (previousFocus instanceof HTMLElement && previousFocus.isConnected) {
        previousFocus.focus();
      }
      if (lockScroll && eventTarget === document) {
        document.body.style.overflow = previousOverflow;
      }
    };
  }, [eventTarget, isVisible, modalRef, closeOnEscape, lockScroll, preventClose]);
}
