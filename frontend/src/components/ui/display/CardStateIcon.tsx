import { useId, type ReactNode, type SVGProps } from "react";
import { assetUrl } from "@/assets";
import { TAG_ICONS } from "@/utils/iconStore.ts";

const earth = TAG_ICONS.earth;
const cardArt = assetUrl("icons/resources/card", 128);
function Svg({ children, ...props }: SVGProps<SVGSVGElement>) {
  return (
    <svg
      viewBox="0 0 40 40"
      width="100%"
      height="100%"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      {...props}
    >
      {children}
    </svg>
  );
}
function Connected() {
  const id = useId();
  return (
    <svg viewBox="0 0 40 40" width="100%" height="100%">
      <defs>
        <clipPath id={`${id}-half`}>
          <path d="M34 0 5 40h35V0Z" />
        </clipPath>
        <filter id={`${id}-green`} colorInterpolationFilters="sRGB">
          <feColorMatrix type="hueRotate" values="-100" />
        </filter>
      </defs>
      <svg viewBox="110 110 1040 1040" width="40" height="40">
        <image href={assetUrl("icons/earth-connected", 512)} width="1280" height="1280" />
      </svg>
      <g clipPath={`url(#${id}-half)`}>
        <svg viewBox="110 110 1040 1040" width="40" height="40" filter={`url(#${id}-green)`}>
          <image href={assetUrl("icons/earth-connected", 512)} width="1280" height="1280" />
        </svg>
      </g>
    </svg>
  );
}
function Disconnected() {
  const id = useId();
  return (
    <Svg>
      <defs>
        <clipPath id={id}>
          <circle cx="20" cy="20" r="19" />
        </clipPath>
        <linearGradient id={`${id}g`} x1="0" y1="0" x2="1" y2="1">
          <stop stopColor="#f64b50" />
          <stop offset="1" stopColor="#970c26" />
        </linearGradient>
      </defs>
      <g clipPath={`url(#${id})`} stroke="none">
        <image href={earth} width="40" height="40" />
        <path d="M34 0 5 40h35V0Z" fill={`url(#${id}g)`} />
      </g>
      <g stroke="#fff" strokeWidth="3.6">
        <path d="m29 23 4-4a3 3 0 0 0-4-4l-4 4M22 27l-4 4a3 3 0 0 0 4 4l4-4" />
      </g>
    </Svg>
  );
}
function Withdrawal({
  phase = "event",
  direction,
}: {
  phase?: "event" | "before" | "after" | "deadline";
  direction?: "earlier" | "later";
}) {
  if (phase === "before") {
    return (
      <Svg>
        <image href={earth} x="15" y="7" width="27" height="27" />
        <path d="M14 7 1 20l13 13V7Z" fill="#ffb75e" stroke="none" />
      </Svg>
    );
  }
  if (phase === "after") {
    return (
      <Svg>
        <image href={earth} x="-2" y="7" width="27" height="27" />
        <path d="m26 7 13 13-13 13V7Z" fill="#ff6068" stroke="none" />
      </Svg>
    );
  }
  if (phase === "deadline") {
    let clockColor = "#ffb75e";
    let clockFace = "#111a24";
    if (direction === "earlier") {
      clockColor = "#ff6068";
      clockFace = "#47141c";
    } else if (direction === "later") {
      clockColor = "#62e68a";
      clockFace = "#123c24";
    }
    return (
      <Svg>
        <image href={earth} x="0" y="2" width="29" height="29" />
        <circle cx="29" cy="29" r="10" fill={clockFace} stroke={clockColor} strokeWidth="2.8" />
        <path d={direction === "earlier" ? "M29 22v7h-5" : "M29 22v7h5"} stroke={clockColor} />
      </Svg>
    );
  }
  return (
    <Svg>
      <image href={earth} x="1" y="7" width="27" height="27" />
      <path d="M18 21h20m-7-7 7 7-7 7" stroke="#090d12" strokeWidth="7" />
      <path d="M18 21h20m-7-7 7 7-7 7" stroke="#ff6068" strokeWidth="4" />
    </Svg>
  );
}
function Reserve() {
  return (
    <Svg>
      <path d="M4 10 20 3l16 7v22l-16 6-16-6Z" fill="#996337" stroke="#edbd7a" />
      <path d="m4 10 16 6 16-6M20 16v22M12 7l16 6v9l-7-3v-9" stroke="#f2d4a0" strokeWidth="2" />
    </Svg>
  );
}
function StandardResource() {
  return (
    <Svg stroke="#ecf0e8" strokeWidth="1.5">
      <path d="m20 3 16 9-16 9L4 12Z" fill="#e0e6df" />
      <path d="m4 12 16 9v17L4 29Z" fill="#86968f" />
      <path d="m20 21 16-9v17l-16 9Z" fill="#465e59" />
    </Svg>
  );
}

function CardOperation({ kind }: { kind: "import" | "shutdown" }) {
  const marker =
    kind === "import" ? (
      <path d="M8 8c-3-5-7-5-7 0s4 5 7 0 7-5 7 0-4 5-7 0Z" />
    ) : (
      <path d="M8 1v7m-5-5a7 7 0 1 0 10 0" />
    );
  return (
    <span className="relative inline-flex h-full w-full">
      <img
        src={cardArt}
        alt=""
        className="h-full w-full object-contain [filter:drop-shadow(0_1px_3px_rgba(0,0,0,0.6))_drop-shadow(0_0_1px_rgba(255,248,220,0.6))_drop-shadow(0_0_2px_rgba(255,248,220,0.4))]"
      />
      <svg
        data-card-operation-marker={kind === "import" ? "permanent" : "shutdown"}
        className="absolute -bottom-0.5 -right-1 w-4 h-4 overflow-visible"
        viewBox="0 0 16 16"
        fill="none"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <g stroke="#0b1017" strokeWidth="4.5">
          {marker}
        </g>
        <g stroke={kind === "import" ? "#c8ebff" : "#ffae73"} strokeWidth="1.75">
          {marker}
        </g>
      </svg>
    </span>
  );
}
function WindowIcon({ production = false }) {
  return (
    <Svg>
      <path d="M31 9A16 16 0 1 0 35 25M31 2v8H23" stroke="#c8ebff" />
      {production && (
        <path d="M10 29V18l7 4v-8l7 5V12h5v17Z" fill="#805531" stroke="#f1c08d" strokeWidth="1.8" />
      )}
    </Svg>
  );
}
const icons: Record<string, ReactNode> = {
  connection: <Connected />,
  "connection-open": <Connected />,
  "connection-closed": <Disconnected />,
  transition: <Withdrawal />,
  "transition-pending": <Withdrawal phase="before" />,
  "transition-complete": <Withdrawal phase="after" />,
  "generation-deadline": <Withdrawal phase="deadline" />,
  "withdrawal-earlier": <Withdrawal phase="deadline" direction="earlier" />,
  "withdrawal-later": <Withdrawal phase="deadline" direction="later" />,
  reserve: <Reserve />,
  "standard-resource": <StandardResource />,
  permanent: (
    <Svg>
      <path
        d="M20 20c-5-8-9-9-12-6-4 4-1 12 4 11 6-1 10-13 16-12 7 0 8 12 1 13-4 1-6-2-9-6Z"
        stroke="#fff"
        strokeWidth="3.5"
      />
    </Svg>
  ),
  "technology-import": <CardOperation kind="import" />,
  "operation-retired": <CardOperation kind="shutdown" />,
  "game-limit": (
    <Svg>
      <text
        x="20"
        y="28"
        textAnchor="middle"
        fill="#fff"
        stroke="none"
        fontSize="24"
        fontFamily="Orbitron"
      >
        1×
      </text>
    </Svg>
  ),
  "contact-window-generation": <WindowIcon />,
  "contact-window-output": <WindowIcon production />,
};

export function getCardStateIcon(icon: string): ReactNode | undefined {
  return icons[icon];
}
