import { useLayoutEffect, useRef, useState, type ReactNode, type RefObject } from "react";

interface FittedCardProps {
  /** Width the card is laid out at before scaling (200 compact card, 360 inspection, 320 corporation). */
  naturalWidth: number;
  /** Fit the card to this height. */
  height?: number;
  /** Fit the card inside this element's content box instead of a fixed height. */
  boundsRef?: RefObject<HTMLElement | null>;
  /** Width inside the bounds kept free for other content (box fit only). */
  reservedWidth?: number;
  maxScale?: number;
  className?: string;
  children: ReactNode;
}

interface Fit {
  scale: number;
  width: number;
  height: number;
}

function contentBox(element: HTMLElement) {
  const style = getComputedStyle(element);
  return {
    width: element.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight),
    height: element.clientHeight - parseFloat(style.paddingTop) - parseFloat(style.paddingBottom),
  };
}

/**
 * Renders a card at its design size and scales it as one unit, like an image, so every part of
 * the card keeps its proportions at any size. The wrapper reserves the scaled size in layout.
 */
export default function FittedCard({
  naturalWidth,
  height,
  boundsRef,
  reservedWidth = 0,
  maxScale = 1,
  className = "",
  children,
}: FittedCardProps) {
  const innerRef = useRef<HTMLDivElement>(null);
  const [fit, setFit] = useState<Fit | null>(null);

  useLayoutEffect(() => {
    const inner = innerRef.current;
    if (!inner) {
      return;
    }
    let observedBounds: HTMLElement | null = null;
    let frame = 0;
    const observer = new ResizeObserver(() => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(update);
    });
    const update = () => {
      // An ancestor's ref is attached after this effect first runs; the observer retries.
      const bounds = boundsRef?.current ?? null;
      if (boundsRef && !bounds) {
        return;
      }
      if (bounds && bounds !== observedBounds) {
        observer.observe(bounds);
        observedBounds = bounds;
      }
      const naturalHeight = inner.offsetHeight;
      if (naturalHeight === 0) {
        return;
      }
      let scale = maxScale;
      if (height !== undefined) {
        scale = Math.min(scale, height / naturalHeight);
      }
      if (bounds) {
        const box = contentBox(bounds);
        scale = Math.min(
          scale,
          box.height / naturalHeight,
          (box.width - reservedWidth) / naturalWidth,
        );
      }
      scale = Math.max(0, scale);
      setFit((previous) => {
        if (previous && Math.abs(previous.scale - scale) < 0.001) {
          return previous;
        }
        return { scale, width: naturalWidth * scale, height: naturalHeight * scale };
      });
    };
    update();
    observer.observe(inner);
    return () => {
      observer.disconnect();
      cancelAnimationFrame(frame);
    };
  }, [boundsRef, height, maxScale, naturalWidth, reservedWidth]);

  return (
    <div
      className={`relative shrink-0 ${className}`}
      style={{ width: fit?.width ?? naturalWidth, height: fit?.height ?? 0 }}
    >
      <div
        ref={innerRef}
        className="absolute top-0 left-0 origin-top-left"
        style={{
          width: naturalWidth,
          transform: `scale(${fit?.scale ?? 1})`,
          visibility: fit ? "visible" : "hidden",
        }}
      >
        {children}
      </div>
    </div>
  );
}
