import { Z_INDEX } from "@/constants/zIndex.ts";
import RevealTrigger from "../display/RevealTrigger.tsx";

interface PlayabilityProps {
  errors: { message: string }[];
  warning?: boolean;
}

export function PlayabilityErrorList({ errors }: PlayabilityProps) {
  return (
    <ul className="m-0 p-0 list-none flex flex-col gap-1">
      {errors.map((error, index) => (
        <li
          key={index}
          className="border-l-[3px] border-l-[#e74c3c] bg-[rgba(231,76,60,0.12)] px-2 py-1 text-[13px] leading-snug text-white/90"
        >
          {error.message}
        </li>
      ))}
    </ul>
  );
}

export default function PlayabilityBadge({ errors, warning = false }: PlayabilityProps) {
  return (
    <RevealTrigger
      as="button"
      aria-label={warning ? "Card warnings" : "Why this card can't be played"}
      placement="below"
      maxWidth={240}
      cornerSize={8}
      className="absolute top-11 right-0 size-11 flex items-center justify-center cursor-pointer"
      style={{ zIndex: Z_INDEX.GAME_BOARD_OVERLAY }}
      content={
        <ul className="m-0 p-0 list-none flex flex-col gap-1 text-[12px] leading-snug">
          {errors.map((error, index) => (
            <li key={index}>{error.message}</li>
          ))}
        </ul>
      }
    >
      <span
        className={`size-8 flex items-center justify-center bg-black/80 border ${warning ? "border-amber-400 text-amber-300" : "border-[#e74c3c] text-[#ff6b6b]"}`}
      >
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <path d="M12 3 2 21h20z" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" />
          <path d="M12 10v5M12 17.5v.5" stroke="currentColor" strokeWidth="2" />
        </svg>
      </span>
    </RevealTrigger>
  );
}
