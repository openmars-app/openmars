import { useOverlayPortalRoot } from "@/contexts/OverlayEnvironment";
import { Z_INDEX } from "@/constants/zIndex.ts";
import React, { useRef } from "react";
import { createPortal } from "react-dom";
import { GameModalProps, ModalSize } from "./types";
import { getThemeStyles } from "./themes";
import { useModal } from "./useModal";

const sizeClasses: Record<ModalSize, string> = {
  small: "max-w-[600px]",
  medium: "max-w-[900px]",
  large: "max-w-[1200px]",
  full: "max-w-[1400px]",
};

const GameModal: React.FC<GameModalProps> = ({
  isVisible,
  onClose,
  theme,
  size = "large",
  animation = "slideIn",
  zIndex = Z_INDEX.STANDARD_MODAL,
  closeOnBackdrop = true,
  closeOnEscape = true,
  lockScroll = true,
  preventClose = false,
  onPreventedClose,
  outerContent,
  children,
  className = "",
}) => {
  const portalRoot = useOverlayPortalRoot();
  const modalRef = useRef<HTMLDivElement>(null);
  useModal({
    modalRef,
    isVisible,
    onClose,
    closeOnEscape,
    lockScroll,
    preventClose,
    onPreventedClose,
  });

  if (!isVisible) return null;

  const themeStyles = getThemeStyles(theme);

  const handleBackdropClick = () => {
    if (preventClose) {
      onPreventedClose?.();
    } else if (closeOnBackdrop) {
      onClose();
    }
  };

  let animationClass = "";
  if (animation === "slideIn") {
    animationClass = "animate-[modalSlideIn_0.25s_ease-out]";
  } else if (animation === "fadeIn") {
    animationClass = "menu-enter";
  }

  const modalBox = (
    <div
      ref={modalRef}
      role="dialog"
      aria-modal="true"
      tabIndex={-1}
      className={`relative text-white w-full ${sizeClasses[size]} max-h-[90dvh] game-panel game-panel-clipped overflow-hidden ${animationClass} flex flex-col ${className}`}
      style={themeStyles}
    >
      {children}
    </div>
  );

  return createPortal(
    <div
      className="fixed top-0 left-0 right-0 bottom-0 flex items-center justify-center p-5"
      style={{ zIndex }}
    >
      <div
        className="absolute top-0 left-0 right-0 bottom-0 bg-black/60 cursor-default animate-[fadeIn_0.3s_ease-out]"
        onClick={handleBackdropClick}
      />

      {outerContent ? (
        <div className="relative flex items-center w-fit max-w-full">
          {modalBox}
          {outerContent}
        </div>
      ) : (
        modalBox
      )}
    </div>,
    portalRoot,
  );
};

export default GameModal;
