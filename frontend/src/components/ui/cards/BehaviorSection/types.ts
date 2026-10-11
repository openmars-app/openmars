import {
  CardBehaviorDto,
  ComputedBehaviorValueDto,
  ResourcesDto,
} from "@/types/generated/api-types.ts";

export type BehaviorPresentation = "compact" | "inspection";

export interface BehaviorSectionProps {
  presentation?: BehaviorPresentation;
  layout?: "project" | "corporation";
  showTooltips?: boolean;
  behaviors?: CardBehaviorDto[];
  computedValues?: ComputedBehaviorValueDto[];
  playerResources?: ResourcesDto;
  resourceStorage?: { [cardId: string]: number };
  cardId?: string;
  cardName?: string;
  sourceBehaviorIndex?: number;
  greyOutAll?: boolean;
  hideActionChip?: boolean;
  noContainer?: boolean;
}

export interface ClassifiedBehavior {
  behavior: CardBehaviorDto;
  type:
    | "manual-action"
    | "immediate-production"
    | "immediate-effect"
    | "triggered-effect"
    | "auto-no-background"
    | "discount"
    | "payment-substitute"
    | "value-modifier"
    | "defense";
  description?: string;
  mergedBehaviors?: CardBehaviorDto[];
  presentationGroup?: "dependency" | "shared-action" | "shared-effect";
  members?: ClassifiedBehavior[];
  originalIndex?: number;
  originalIndices?: number[];
  alternativeOriginalIndices?: number[];
}

export interface LayoutRequirement {
  totalIcons: number;
  separatorCount: number;
  separatorPositions: number[];
  behaviorType: string;
  needsMultipleRows: boolean;
  maxHorizontalIcons: number;
}

export interface IconDisplayInfo {
  resourceType: string;
  amount: number;
  displayMode: "individual" | "number";
  iconCount: number;
  variableAmount?: boolean;
}

export interface LayoutPlan {
  rows: IconDisplayInfo[][];
  separators: Array<{ position: number; type: string }>;
  totalRows: number;
}

export interface TileScaleInfo {
  scale: 1 | 1.25 | 1.5 | 2;
  tileType: string | null;
  tileCount?: number;
}

export interface CardLayoutPlan {
  behaviors: Array<{
    behaviorIndex: number;
    layoutPlan: LayoutPlan;
    estimatedRows: number;
  }>;
  totalEstimatedRows: number;
  needsOverflowHandling: boolean;
  maxRows: number;
}

export type IconContext = "standalone" | "action" | "production" | "default";
