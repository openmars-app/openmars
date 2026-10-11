import type { ReactNode } from "react";

const glyphs: Record<string, ReactNode> = {
  connection: (
    <>
      <path d="M9 8H7a4 4 0 0 0 0 8h3m4-8h3a4 4 0 0 1 0 8h-2M8 12h8" />
    </>
  ),
  "connection-open": (
    <>
      <path d="M9 8H7a4 4 0 0 0 0 8h3m4-8h3a4 4 0 0 1 0 8h-2M8 12h8" />
      <path d="m16 3 2 2 3-3" />
    </>
  ),
  "connection-closed": (
    <>
      <path d="M8 8H7a4 4 0 0 0 0 8h2m7-8h1a4 4 0 0 1 0 8h-1M3 3l18 18" />
    </>
  ),
  transition: (
    <>
      <path d="M4 5h16M4 19h16M7 5v3l10 8v3M17 5v3L7 16v3" />
    </>
  ),
  "transition-pending": (
    <>
      <path d="M5 4v16m4-8h11m-7-4-4 4 4 4" />
      <circle cx="5" cy="4" r="1" />
    </>
  ),
  "transition-complete": (
    <>
      <path d="M19 4v16M4 12h11m-4-4 4 4-4 4" />
      <circle cx="19" cy="4" r="1" />
    </>
  ),
  "generation-deadline": (
    <>
      <path d="M4 3v18M4 4h15l-3 4 3 4H4" />
      <path d="M10 15v4h4" />
    </>
  ),
  generation: (
    <>
      <path d="M18 6a8 8 0 1 0 2 10M18 2v5h-5" />
      <path d="M12 7v5l3 2" />
    </>
  ),
  "game-limit": (
    <>
      <path d="M6 3h12v16l-6 3-6-3z" />
      <path d="m9 11 2 2 4-5" />
    </>
  ),
  "shared-use": (
    <>
      <path d="M3 7h7l4 10h7M3 17h7l4-10h7m-3-3 3 3-3 3m0 4 3 3-3 3" />
    </>
  ),
  "technology-import": (
    <>
      <path d="M4 3h10l4 4v14H4zM14 3v5h5" />
      <path d="M1 12h11m-4-4 4 4-4 4" />
    </>
  ),
  "operation-paused": (
    <>
      <rect x="3" y="3" width="18" height="18" />
      <path d="M9 7v10m6-10v10" />
    </>
  ),
  "operation-retired": (
    <>
      <path d="M5 3h11l4 4v14H5zM16 3v5h4M3 3l18 18" />
    </>
  ),
  independent: (
    <>
      <path d="M4 3h16v18H4z" />
      <path d="m7 12 4 4 6-8" />
    </>
  ),
  defense: (
    <>
      <path d="m12 2 9 4v6c0 5-9 10-9 10S3 17 3 12V6z" />
      <path d="m7 11 4 4 6-7" />
    </>
  ),
  production: (
    <>
      <path d="M3 21V10l6 3V7l6 4V3h5v18zM6 17h2m4 0h2m3 0h2" />
    </>
  ),
  research: (
    <>
      <path d="M4 3h7v17H4zm9 0h7v17h-7M7 7h1m8 0h1M7 11h1m8 0h1" />
    </>
  ),
  action: (
    <>
      <path d="m13 2-8 12h6l-1 8 9-13h-7z" />
    </>
  ),
  "award-fund": (
    <>
      <path d="M7 3h10v7a5 5 0 0 1-10 0zM7 5H3v4a4 4 0 0 0 4 4m10-8h4v4a4 4 0 0 1-4 4M12 15v5m-4 1h8" />
    </>
  ),
  "production-types": (
    <>
      <rect x="2" y="4" width="8" height="7" />
      <rect x="14" y="4" width="8" height="7" />
      <rect x="8" y="14" width="8" height="7" />
      <path d="M6 11v6h2m10-6v6h-2" />
    </>
  ),
  "ignore-global-requirements": (
    <>
      <path d="M3 3l18 18M7 3v10a4 4 0 1 0 5 0V3zM16 4h5m-5 4h5" />
    </>
  ),
  "action-reuse": (
    <>
      <path d="M18 6a8 8 0 1 0 2 10M18 2v5h-5" />
      <path d="m12 8-3 5h3v4l4-6h-4z" />
    </>
  ),
  "extra-actions": (
    <>
      <path d="m8 2-6 11h6l-1 9 8-13H9zM17 15h6m-3-3v6" />
    </>
  ),
  "tile-replacement": (
    <>
      <path d="m8 2 6 3v7l-6 3-6-3V5zm8 7 6 3v7l-6 3-6-3v-4M12 3h8v4m-3-3 3 3 3-3" />
    </>
  ),
  effect: (
    <>
      <path d="m12 2 2 6 6-2-3 6 5 4-7 1-3 5-3-5-7-1 5-4-3-6 6 2z" />
    </>
  ),
};

export function hasSemanticIcon(icon: string): boolean {
  return Object.hasOwn(glyphs, icon);
}

export default function SemanticIcon({
  icon,
  className = "",
  label = icon,
}: {
  icon: string;
  className?: string;
  label?: string;
}) {
  const glyph = glyphs[icon] ?? glyphs.effect;
  return (
    <svg
      role="img"
      aria-label={label}
      data-semantic-icon={icon}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinejoin="round"
      strokeLinecap="round"
      className={`shrink-0 text-sky-100 ${className}`}
    >
      {glyph}
    </svg>
  );
}
