import { useId, useState, type ReactNode } from "react";
import GameButton from "./buttons/GameButton";

export default function CollapsibleSection({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const id = useId();

  return (
    <div className="text-sm border-t border-white/10 pt-3">
      <GameButton
        emphasis="quiet"
        aria-expanded={open}
        aria-controls={id}
        onClick={() => setOpen((value) => !value)}
        className="w-full !justify-start !px-0 min-h-11 !text-white/70"
      >
        <span className="flex items-center gap-1.5">
          <svg
            width="8"
            height="8"
            viewBox="0 0 8 8"
            aria-hidden="true"
            className={`shrink-0 transition-transform duration-200 ease-in-out motion-reduce:transition-none ${open ? "rotate-90" : ""}`}
          >
            <path d="M1 0 7 4 1 8Z" fill="currentColor" />
          </svg>
          {label}
        </span>
      </GameButton>
      <div
        id={id}
        inert={!open}
        aria-hidden={!open}
        className={`grid transition-[grid-template-rows,opacity] duration-200 ease-in-out motion-reduce:transition-none ${open ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0"}`}
      >
        <div className="min-h-0 overflow-hidden">{children}</div>
      </div>
    </div>
  );
}
