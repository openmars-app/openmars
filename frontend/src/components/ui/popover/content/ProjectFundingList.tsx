import { useState } from "react";
import GameButton from "@/components/ui/buttons/GameButton.tsx";
import {
  ColonyOutputDto,
  GameDto,
  PaymentDto,
  PaymentQuoteDto,
  ProjectFundingDto,
  ProjectGlobalOutputDto,
  ProjectSeatDto,
} from "@/types/generated/api-types.ts";
import GameIcon from "../../display/GameIcon.tsx";
import { webSocketService } from "@/services/webSocketService.ts";
import { GamePopoverItem, getThemeStyles } from "../../GamePopover";
import { mapOutputTypeToIcon } from "../ColonySteps.tsx";
import { PaymentPicker } from "../PaymentSelectionPopover.tsx";
import type { ContentDensity } from "./density.ts";
import { canActOnProjects } from "./gamePlayers.ts";

export interface ProjectSeatPurchase {
  buySeat: (project: ProjectFundingDto) => void;
  quote: PaymentQuoteDto | undefined;
  confirm: (payment: PaymentDto) => void;
  cancel: () => void;
}

function allocatedAmount(payment: PaymentDto, resource: string): number {
  return payment.allocations
    .filter((a) => a.source.resource === resource)
    .reduce((n, a) => n + a.amount, 0);
}

export function useProjectSeatPurchase(
  gameState: GameDto | undefined,
  onFlowStart?: () => void,
): ProjectSeatPurchase {
  return useProjectSeatPurchaseState(
    gameState,
    (id, credits, steel, titanium) => {
      void webSocketService.buyProjectSeat(id, credits, steel, titanium);
    },
    onFlowStart,
  );
}

export function useProjectSeatPurchaseState(
  gameState: GameDto | undefined,
  onPurchase: (id: string, credits: number, steel: number, titanium: number) => void,
  onFlowStart?: () => void,
): ProjectSeatPurchase {
  const [pendingProject, setPendingProject] = useState<ProjectFundingDto | null>(null);
  const resources = gameState?.currentPlayer?.resources;

  const quote: PaymentQuoteDto | undefined =
    pendingProject && resources
      ? {
          costs: { credit: pendingProject.nextSeatCost },
          options: [
            {
              source: { target: "self-player", resource: "credit" },
              targetResource: "credit",
              conversionRate: 1,
              available: resources.credits,
            },
            ...pendingProject.paymentSubstitutes.map((s) => ({
              source: { target: "self-player" as const, resource: s.resourceType },
              targetResource: "credit",
              conversionRate: s.conversionRate,
              available: s.resourceType === "steel" ? resources.steel : resources.titanium,
            })),
          ],
        }
      : undefined;

  const confirm = (payment: PaymentDto) => {
    if (!pendingProject) {
      return;
    }
    setPendingProject(null);
    onFlowStart?.();
    onPurchase(
      pendingProject.id,
      allocatedAmount(payment, "credit"),
      allocatedAmount(payment, "steel"),
      allocatedAmount(payment, "titanium"),
    );
  };

  const buySeat = (project: ProjectFundingDto) => {
    if (project.paymentSubstitutes.length > 0) {
      setPendingProject(project);
      return;
    }
    onFlowStart?.();
    onPurchase(project.id, project.nextSeatCost, 0, 0);
  };

  return { buySeat, quote, confirm, cancel: () => setPendingProject(null) };
}

export function ProjectSeatPaymentPicker({ purchase }: { purchase: ProjectSeatPurchase }) {
  if (!purchase.quote) {
    return null;
  }
  return (
    <PaymentPicker quote={purchase.quote} onConfirm={purchase.confirm} onCancel={purchase.cancel} />
  );
}

interface ProjectFundingListProps {
  gameState?: GameDto;
  purchase: ProjectSeatPurchase;
  density: ContentDensity;
}

export default function ProjectFundingList({
  gameState,
  purchase,
  density,
}: ProjectFundingListProps) {
  const isScreen = density === "screen";
  const canAct = canActOnProjects(gameState);
  const projects = gameState?.projectFunding ?? [];

  const list = (
    <div className={`popover-list space-y-2 ${isScreen ? "w-full" : "p-2"}`}>
      {projects.map((project) => (
        <ProjectCard
          key={project.id}
          project={project}
          canAct={canAct}
          onBuySeat={purchase.buySeat}
          isScreen={isScreen}
        />
      ))}
    </div>
  );

  if (!isScreen) {
    return list;
  }

  return (
    <div className="w-full" style={getThemeStyles("colonies")}>
      {list}
      <ProjectSeatPaymentPicker purchase={purchase} />
    </div>
  );
}

interface ProjectCardProps {
  project: ProjectFundingDto;
  canAct: boolean;
  onBuySeat: (project: ProjectFundingDto) => void;
  isScreen: boolean;
}

function ProjectCard({ project, canAct, onBuySeat, isScreen }: ProjectCardProps) {
  const canBuy = canAct && project.canBuySeat;
  const dimmed = canAct && !canBuy && !project.isCompleted;

  const filledSeats = project.seats.filter((s) => s.isFilled).length;
  const totalSeats = project.seats.length;

  const labelText = isScreen ? "text-[11px]" : "text-[10px]";
  const bodyText = isScreen ? "text-[13px]" : "text-xs";

  return (
    <GamePopoverItem
      state={dimmed ? "disabled" : "available"}
      borderColor={project.isCompleted ? "#10b981" : project.style.color}
      className="popover-list-item"
    >
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <h3 className="text-white text-sm font-bold font-orbitron m-0">{project.name}</h3>
          <span
            className={`${isScreen ? "text-[12px]" : "text-[10px]"} font-orbitron text-white/40`}
          >
            {filledSeats}/{totalSeats}
          </span>
          {project.isCompleted && (
            <span
              className={`px-2 py-0.5 rounded-full ${labelText} font-orbitron font-bold text-emerald-400 bg-emerald-400/15`}
            >
              COMPLETED
            </span>
          )}
        </div>

        {canAct && !project.isCompleted && (
          <GameButton
            emphasis="primary"
            shape="cut"
            size="sm"
            className={`${isScreen ? "!min-h-11 min-w-[96px] px-4 text-[13px]" : "px-3 py-1 text-xs"} rounded-none font-semibold font-orbitron transition-all cursor-pointer `}
            onClick={(e) => {
              e.stopPropagation();
              if (canBuy) {
                onBuySeat(project);
              }
            }}
            disabled={!canBuy}
          >
            Buy Seat
          </GameButton>
        )}
      </div>

      <div className={`${bodyText} text-white/50 mb-2`}>{project.description}</div>

      <SeatBar seats={project.seats} styleColor={project.style.color} isScreen={isScreen} />

      {!project.isCompleted && project.nextSeatCost > 0 && (
        <div className={`flex items-center gap-1.5 mt-2 ${bodyText} text-white/60`}>
          <span className={`font-orbitron ${labelText} uppercase tracking-wider`}>Next Seat</span>
          <GameIcon iconType="credit" amount={project.nextSeatCost} size="small" />
          {project.paymentSubstitutes.map((sub) => (
            <span key={sub.resourceType} className={`${labelText} text-white/40`}>
              ({sub.resourceType} {sub.conversionRate}:1)
            </span>
          ))}
        </div>
      )}

      <div style={{ height: "1px", background: "rgba(255,255,255,0.15)", margin: "10px 0" }} />

      <div className={`flex items-center justify-between ${isScreen ? "flex-wrap gap-2" : ""}`}>
        <div className="flex flex-col gap-1">
          <span className={`${labelText} font-orbitron text-white/40 uppercase tracking-wider`}>
            Rewards
          </span>
          <div className="flex items-center gap-2">
            {project.rewardTiers.map((tier) => (
              <div key={tier.seatsOwned} className="flex items-center gap-1">
                <span className={`${labelText} text-white/50 font-orbitron`}>
                  {tier.seatsOwned}x:
                </span>
                <OutputDisplay outputs={tier.rewards} isScreen={isScreen} />
              </div>
            ))}
          </div>
        </div>

        <div className="flex flex-col gap-1 items-end">
          <span className={`${labelText} font-orbitron text-white/40 uppercase tracking-wider`}>
            Completion
          </span>
          <div className="flex flex-col items-end gap-0.5">
            {project.completionEffect.rewards.length > 0 && (
              <OutputDisplay outputs={project.completionEffect.rewards} isScreen={isScreen} />
            )}
            {project.completionEffect.globalEffects &&
              project.completionEffect.globalEffects.length > 0 && (
                <GlobalEffectsDisplay
                  effects={project.completionEffect.globalEffects}
                  labelText={labelText}
                />
              )}
          </div>
        </div>
      </div>

      {project.currentPlayerSeats > 0 && (
        <div className="mt-2 flex items-center gap-2">
          <span className={`${labelText} font-orbitron text-white/40`}>
            Your seats: {project.currentPlayerSeats}
          </span>
          {project.currentPlayerTier && (
            <span className={`${labelText} font-orbitron text-emerald-400/80`}>
              Tier {project.currentPlayerTier.seatsOwned} reward
            </span>
          )}
        </div>
      )}
    </GamePopoverItem>
  );
}

interface SeatBarProps {
  seats: ProjectSeatDto[];
  styleColor: string;
  isScreen: boolean;
}

function SeatBar({ seats, styleColor, isScreen }: SeatBarProps) {
  return (
    <div className="flex gap-1">
      {seats.map((seat, i) => (
        <div
          key={i}
          className={`flex-1 rounded-sm flex items-center justify-center font-orbitron font-bold transition-colors ${isScreen ? "h-7 text-[11px]" : "h-5 text-[9px]"}`}
          style={{
            backgroundColor: seat.isFilled
              ? (seat.ownerColor || styleColor) + "80"
              : "rgba(255,255,255,0.06)",
            border: `1px solid ${seat.isFilled ? (seat.ownerColor || styleColor) + "60" : "rgba(255,255,255,0.1)"}`,
          }}
        >
          {seat.isFilled ? (
            <span className="text-white/80 truncate px-0.5">{seat.ownerName}</span>
          ) : (
            <span className="text-white/20">{seat.cost}</span>
          )}
        </div>
      ))}
    </div>
  );
}

function OutputDisplay({ outputs, isScreen }: { outputs: ColonyOutputDto[]; isScreen: boolean }) {
  return (
    <span className="inline-flex items-center gap-0.5">
      {outputs.map((output, i) => {
        const icon = mapOutputTypeToIcon(output.type);
        const useAmountProp = output.type === "credit" || output.type === "credit-production";
        return (
          <span key={i} className="inline-flex items-center gap-0.5">
            {!useAmountProp && output.amount > 1 && (
              <span
                className={`${isScreen ? "text-[13px]" : "text-xs"} text-white/70 font-orbitron font-bold`}
              >
                {output.amount}
              </span>
            )}
            <GameIcon
              iconType={icon}
              amount={useAmountProp ? output.amount : undefined}
              size="small"
            />
          </span>
        );
      })}
    </span>
  );
}

const globalEffectLabel = (effect: ProjectGlobalOutputDto): string => {
  switch (effect.type) {
    case "freeze-turn-order":
      return "Freeze turn order";
    case "production-choice":
      return `Each player: +${effect.amount} any production`;
    case "card-draw":
      return `Each player: +${effect.amount} cards`;
    case "temperature":
      return `+${effect.amount} temperature`;
    case "oxygen":
      return `+${effect.amount} oxygen`;
    default:
      return effect.type;
  }
};

function GlobalEffectsDisplay({
  effects,
  labelText,
}: {
  effects: ProjectGlobalOutputDto[];
  labelText: string;
}) {
  return (
    <div className="flex flex-col items-end gap-0.5">
      {effects.map((effect, i) => (
        <span key={i} className={`${labelText} font-orbitron text-purple-300/80`}>
          {globalEffectLabel(effect)}
        </span>
      ))}
    </div>
  );
}
