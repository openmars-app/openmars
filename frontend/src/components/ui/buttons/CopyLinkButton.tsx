import React, { useState, useRef, useEffect } from "react";
import GameButton from "./GameButton.tsx";

interface CopyLinkButtonProps {
  textToCopy: string;
  defaultText: string;
  copiedText?: string;
  className?: string;
  emphasis?: "primary" | "secondary" | "quiet";
  size?: "sm" | "md" | "lg";
  icon?: React.ReactNode;
  onCopySuccess?: () => void;
  onCopyError?: (error: Error) => void;
}

const CopyLinkButton: React.FC<CopyLinkButtonProps> = (props) => (
  <CopyLinkButtonView {...props} onCopy={(text) => navigator.clipboard.writeText(text)} />
);

export const CopyLinkButtonView: React.FC<
  CopyLinkButtonProps & { onCopy: (text: string) => Promise<void> }
> = ({
  textToCopy,
  defaultText,
  copiedText = "Copied!",
  className = "",
  emphasis = "secondary",
  size = "md",
  icon,
  onCopySuccess,
  onCopyError,
  onCopy,
}) => {
  const resetTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(
    () => () => {
      if (resetTimer.current) {
        clearTimeout(resetTimer.current);
      }
    },
    [],
  );
  const [isCopied, setIsCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await onCopy(textToCopy);
      setIsCopied(true);
      onCopySuccess?.();

      // Fade back to default text after 1 second
      resetTimer.current = setTimeout(() => {
        setIsCopied(false);
      }, 1000);
    } catch (error) {
      console.error("Failed to copy to clipboard:", error);
      onCopyError?.(error as Error);
    }
  };

  return (
    <GameButton
      emphasis={emphasis}
      size={size}
      onClick={handleCopy}
      disabled={isCopied}
      className={`min-w-[120px] ${className}`}
    >
      <span
        className={`inline-flex items-center gap-2 transition-opacity duration-300 ${isCopied ? "opacity-70" : "opacity-100"}`}
      >
        {isCopied ? copiedText : defaultText}
        {icon && !isCopied && icon}
      </span>
    </GameButton>
  );
};

export default CopyLinkButton;
