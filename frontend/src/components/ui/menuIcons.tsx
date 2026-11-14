import React from "react";

const svgProps = {
  width: 16,
  height: 16,
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 2,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
};

export const SettingsIcon: React.FC = () => (
  <svg {...svgProps} aria-hidden="true">
    <path d="m9 3-.5 3-2 1-2.8-1-2 3.5L4 11v2l-2.3 1.5 2 3.5 2.8-1 2 1 .5 3h4l.5-3 2-1 2.8 1 2-3.5L18 13v-2l2.3-1.5-2-3.5-2.8 1-2-1-.5-3Z" />
    <circle cx="11" cy="12" r="3" />
  </svg>
);

export const CopyIcon: React.FC = () => (
  <svg {...svgProps}>
    <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
    <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
  </svg>
);

export const FullscreenIcon: React.FC = () => (
  <svg {...svgProps}>
    <polyline points="15 3 21 3 21 9" />
    <polyline points="9 21 3 21 3 15" />
    <line x1="21" y1="3" x2="14" y2="10" />
    <line x1="3" y1="21" x2="10" y2="14" />
  </svg>
);

export const ExitFullscreenIcon: React.FC = () => (
  <svg {...svgProps}>
    <polyline points="4 14 10 14 10 20" />
    <polyline points="20 10 14 10 14 4" />
    <line x1="14" y1="10" x2="21" y2="3" />
    <line x1="3" y1="21" x2="10" y2="14" />
  </svg>
);

export const InstallIcon: React.FC = () => (
  <svg {...svgProps}>
    <rect x="6" y="2" width="12" height="20" rx="2" ry="2" />
    <polyline points="9 10 12 13 15 10" />
    <line x1="12" y1="6" x2="12" y2="13" />
    <line x1="10" y1="18" x2="14" y2="18" />
  </svg>
);

export const PerformanceIcon: React.FC = () => (
  <svg {...svgProps}>
    <polyline points="22,12 18,12 15,21 9,3 6,12 2,12" />
  </svg>
);

export const BugIcon: React.FC = () => (
  <svg {...svgProps}>
    <path d="M8 2l1.88 1.88" />
    <path d="M14.12 3.88L16 2" />
    <path d="M9 7.13v-1a3.003 3.003 0 1 1 6 0v1" />
    <path d="M12 20c-3.3 0-6-2.7-6-6v-3a4 4 0 0 1 4-4h4a4 4 0 0 1 4 4v3c0 3.3-2.7 6-6 6" />
    <path d="M12 20v-9" />
    <path d="M6.53 9C4.6 8.8 3 7.1 3 5" />
    <path d="M6 13H2" />
    <path d="M3 21c0-2.1 1.7-3.9 3.8-4" />
    <path d="M20.97 5c0 2.1-1.6 3.8-3.5 4" />
    <path d="M22 13h-4" />
    <path d="M17.2 17c2.1.1 3.8 1.9 3.8 4" />
  </svg>
);

export const FeedbackIcon: React.FC = () => (
  <svg {...svgProps}>
    <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
  </svg>
);

export const ServerIcon: React.FC = () => (
  <svg {...svgProps}>
    <rect x="2" y="3" width="20" height="8" rx="1" />
    <rect x="2" y="13" width="20" height="8" rx="1" />
    <path d="M6 7h.01" />
    <path d="M6 17h.01" />
  </svg>
);

export const HomeIcon: React.FC = () => (
  <svg {...svgProps}>
    <path d="M3 11 12 4l9 7" />
    <path d="M5 10v10h14V10" />
  </svg>
);

export const LeaveIcon: React.FC = () => (
  <svg {...svgProps}>
    <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
    <polyline points="16 17 21 12 16 7" />
    <line x1="21" y1="12" x2="9" y2="12" />
  </svg>
);

export const EndGameIcon: React.FC = () => (
  <svg {...svgProps}>
    <rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
    <line x1="9" y1="9" x2="15" y2="15" />
    <line x1="15" y1="9" x2="9" y2="15" />
  </svg>
);

export const EyeIcon: React.FC = () => (
  <svg
    width={18}
    height={18}
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth={2}
    strokeLinecap="round"
    strokeLinejoin="round"
  >
    <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
    <circle cx="12" cy="12" r="3" />
  </svg>
);

export const CardsIcon: React.FC = () => (
  <svg
    width={16}
    height={16}
    viewBox="0 0 14 14"
    fill="none"
    stroke="currentColor"
    strokeWidth={1}
    strokeLinecap="round"
    strokeLinejoin="round"
  >
    <path d="M6.546.857a.475.475 0 0 1 .581-.335l6.02 1.612a.475.475 0 0 1 .337.581l-2.31 8.618a.475.475 0 0 1-.582.335l-6.02-1.612a.475.475 0 0 1-.336-.581z" />
    <path d="M6.108 2.535L.852 3.944a.475.475 0 0 0-.336.581l2.308 8.618a.475.475 0 0 0 .582.335l3.01-.806" />
  </svg>
);

export const HamburgerIcon: React.FC = () => (
  <svg
    width={20}
    height={20}
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth={2}
    strokeLinecap="round"
  >
    <line x1="3" y1="6" x2="21" y2="6" />
    <line x1="3" y1="12" x2="21" y2="12" />
    <line x1="3" y1="18" x2="21" y2="18" />
  </svg>
);

export const ThoughtIcon: React.FC = () => (
  <svg {...svgProps}>
    <path d="M21 11.5a8.38 8.38 0 0 1-9 8 8.5 8.5 0 0 1-3.8-.9L3 20l1.4-4.2A8.38 8.38 0 0 1 3.5 12a8.5 8.5 0 0 1 8.5-8.5 8.38 8.38 0 0 1 9 8z" />
  </svg>
);

export const ChangelogIcon: React.FC = () => (
  <svg {...svgProps}>
    <path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z" />
    <path d="M14 2v4a2 2 0 0 0 2 2h4" />
    <path d="M16 13H8" />
    <path d="M16 17H8" />
    <path d="M10 9H8" />
  </svg>
);

export const UpdateIcon: React.FC = () => (
  <svg {...svgProps}>
    <path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8" />
    <path d="M21 3v5h-5" />
    <path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16" />
    <path d="M8 16H3v5" />
  </svg>
);

export const DownloadIcon: React.FC = () => (
  <svg {...svgProps}>
    <path d="M12 3v12m-5-5 5 5 5-5M5 16v5h14v-5" />
  </svg>
);
