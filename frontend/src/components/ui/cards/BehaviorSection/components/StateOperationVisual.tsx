import type { StateConditionDto, WindowBoundaryDto } from "@/types/generated/api-types.ts";
import GameIcon from "../../../display/GameIcon.tsx";
import { StateSymbol } from "../../CardStateVisuals.tsx";
import { BehaviorArrow } from "./BehaviorIcon.tsx";

function BoundarySymbol({ boundary }: { boundary: WindowBoundaryDto }) {
  const icon = boundary.boundary.startsWith("production") ? "production" : "research";
  return (
    <span
      className="inline-flex items-center gap-0.5"
      role="img"
      aria-label={`Generation +${boundary.generationOffset}, ${boundary.boundary}`}
      data-window-boundary={boundary.boundary}
    >
      <span className="font-orbitron text-[10px]">+{boundary.generationOffset}</span>
      <GameIcon iconType="generation" size="behavior" />
      {boundary.boundary.endsWith("start") && <span className="font-bold">|</span>}
      <GameIcon iconType={icon} size="behavior" />
      {boundary.boundary.endsWith("end") && <span className="font-bold">|</span>}
    </span>
  );
}

export default function StateOperationVisual({ operation }: { operation: StateConditionDto }) {
  if (operation.type === "state-change") {
    if (operation.state === "withdrawal-generation" && operation.amount !== 0) {
      const earlier = operation.amount < 0;
      const amount = Math.abs(operation.amount);
      return (
        <span
          className="inline-flex items-center gap-1"
          data-behavior-atom="state-change"
          data-withdrawal-direction={earlier ? "earlier" : "later"}
          aria-label={`Withdrawal ${amount} generation${amount === 1 ? "" : "s"} ${earlier ? "earlier" : "later"}`}
        >
          <GameIcon
            iconType={earlier ? "withdrawal-earlier" : "withdrawal-later"}
            size="behavior"
            amount={amount > 1 ? amount : undefined}
          />
        </span>
      );
    }
    return (
      <span
        className="inline-flex items-center gap-1"
        data-behavior-atom="state-change"
        aria-label={operation.description}
      >
        <span className="font-orbitron text-[13px] font-bold">
          {operation.amount > 0 ? "+" : "−"}
          {Math.abs(operation.amount) !== 1 && Math.abs(operation.amount)}
        </span>
        <StateSymbol state={operation.state ?? ""} />
      </span>
    );
  }
  if (operation.type === "schedule-state-window") {
    const start = operation.start;
    const end = operation.end;
    let scope: string | undefined;
    if (
      start &&
      end &&
      start.generationOffset <= end.generationOffset &&
      end.boundary === "production-end"
    ) {
      if (start.boundary === "research-start") {
        scope = "contact-window-generation";
      } else if (
        start.boundary === "production-start" &&
        start.generationOffset === end.generationOffset
      ) {
        scope = "contact-window-output";
      }
    }
    if (scope && start && end) {
      const duration = end.generationOffset - start.generationOffset + 1;
      return (
        <span
          className="inline-flex items-center gap-1"
          data-behavior-atom="state-window"
          aria-label={operation.description}
        >
          <span
            className="inline-flex items-center gap-0.5"
            data-window-offset={start.generationOffset}
          >
            <GameIcon
              iconType={scope}
              size="behavior"
              amount={scope === "contact-window-generation" ? duration : undefined}
              amountPlacement="center"
            />
          </span>
        </span>
      );
    }
    return (
      <span
        className="inline-flex flex-col items-center gap-0.5"
        data-behavior-atom="state-window"
        aria-label={operation.description}
      >
        <StateSymbol state={operation.state ?? ""} value={operation.value} />
        {operation.start && operation.end && (
          <span className="inline-flex items-center gap-1 [--behavior-icon-size:15px]">
            <BoundarySymbol boundary={operation.start} />
            <BehaviorArrow />
            <BoundarySymbol boundary={operation.end} />
          </span>
        )}
      </span>
    );
  }
  return (
    <span
      className="inline-flex flex-wrap items-center justify-center gap-1"
      data-behavior-atom={operation.type}
      aria-label={operation.description}
    >
      <GameIcon
        iconType={
          operation.type === "remove-dependency" ? "technology-import" : "operation-retired"
        }
        size="behavior"
      />
      {operation.minimumProduction && (
        <span className="inline-flex items-center gap-0.5 [--behavior-icon-size:20px]">
          <span className="font-orbitron text-[11px]">≥</span>
          <GameIcon
            iconType={operation.minimumProduction.type}
            amount={operation.minimumProduction.amount}
            size="behavior"
          />
        </span>
      )}
    </span>
  );
}
