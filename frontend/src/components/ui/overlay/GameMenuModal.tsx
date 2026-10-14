import { useOverlayPortalRoot } from "@/contexts/OverlayEnvironment";
import CloseButton from "@/components/ui/buttons/CloseButton.tsx";
import React, { useState, useEffect } from "react";
import { createPortal } from "react-dom";
import BackButton from "../buttons/BackButton.tsx";
import { Z_INDEX } from "@/constants/zIndex.ts";

interface GameMenuModalProps {
  layout?: "dialog" | "lobby" | "wide";
  title: string;
  subtitle?: string;
  children: React.ReactNode;
  onBack?: () => void;
  backLabel?: string;
  visible?: boolean;
  onExited?: () => void;
  showBackdrop?: boolean;
  zIndex?: number;
  onClose?: () => void;
  showCloseButton?: boolean;
  headerStart?: React.ReactNode;
  headerEnd?: React.ReactNode;
  footer?: React.ReactNode;
}

const GameMenuModal: React.FC<GameMenuModalProps> = ({
  layout = "dialog",
  title,
  subtitle,
  children,
  onBack,
  backLabel,
  visible,
  onExited,
  showBackdrop = false,
  zIndex = Z_INDEX.MENU_DROPDOWN,
  onClose,
  showCloseButton = false,
  headerStart,
  headerEnd,
  footer,
}) => {
  const portalRoot = useOverlayPortalRoot();
  const [animState, setAnimState] = useState<"entering" | "visible" | "exiting">("entering");
  const layoutClass = layout === "dialog" ? "" : `menu-modal-${layout}`;

  useEffect(() => {
    if (visible === false) {
      setAnimState("exiting");
    } else {
      setAnimState("entering");
    }
  }, [visible]);

  const animationClass =
    animState === "exiting"
      ? "animate-[lobbyExit_0.2s_ease-out_forwards]"
      : animState === "entering"
        ? "animate-[modalFadeIn_0.3s_ease-out]"
        : "";

  const handleAnimationEnd = (e: React.AnimationEvent) => {
    if (e.target !== e.currentTarget) return;
    if (animState === "entering") setAnimState("visible");
    if (animState === "exiting") onExited?.();
  };

  const content = (
    <>
      <style>{`
        @keyframes modalFadeIn {
          0% { opacity: 0; }
          100% { opacity: 1; }
        }
        @keyframes lobbyExit {
          from {
            transform: scale(1);
            opacity: 1;
          }
          to {
            transform: scale(0.95);
            opacity: 0;
          }
        }
        @keyframes backdropFadeIn {
          from { opacity: 0; }
          to { opacity: 1; }
        }
        @keyframes backdropFadeOut {
          from { opacity: 1; }
          to { opacity: 0; }
        }
      `}</style>

      {showBackdrop && (
        <div
          data-overlay-layer
          className={`fixed inset-0 bg-black/60 backdrop-blur-sm ${
            animState === "exiting"
              ? "animate-[backdropFadeOut_0.2s_ease-out_forwards]"
              : "animate-[backdropFadeIn_0.3s_ease-out]"
          }`}
          style={{ zIndex: zIndex - 1 }}
          onClick={onClose}
        />
      )}

      {onBack && (
        <div className="menu-chrome-top-left menu-chrome-back" style={{ zIndex: Z_INDEX.POPOVER }}>
          <BackButton onClick={onBack}>{backLabel ?? "Back"}</BackButton>
        </div>
      )}
      <div
        data-overlay-layer={showBackdrop || undefined}
        className={`menu-modal-frame ${layoutClass} ${animationClass}`}
        style={{ zIndex }}
        onAnimationEnd={handleAnimationEnd}
      >
        <div className="menu-modal-panel game-panel">
          {showCloseButton && onClose && (
            <div className="absolute top-3 right-3">
              <CloseButton onClick={onClose} />
            </div>
          )}
          <div
            className={`min-h-0 overflow-y-auto overscroll-contain p-6 sm:p-8 compact:p-4 ${footer ? "!pb-0" : ""}`}
          >
            <div className="relative mb-6 text-center compact:mb-3">
              {headerStart && <div className="absolute top-0 left-0">{headerStart}</div>}
              <div>
                <h2 className="font-orbitron text-white text-[24px] m-0 mb-2 text-shadow-glow font-bold tracking-wider compact:text-[20px] compact:mb-1">
                  {title}
                </h2>
                {subtitle && <p className="text-white/60 text-sm m-0">{subtitle}</p>}
              </div>
              {headerEnd && <div className="absolute top-0 right-0">{headerEnd}</div>}
            </div>
            {children}
          </div>
          {footer && <div className="shrink-0 px-6 pb-6 sm:px-8 sm:pb-8">{footer}</div>}
        </div>
      </div>
    </>
  );

  return showBackdrop ? createPortal(content, portalRoot) : content;
};

export default GameMenuModal;
