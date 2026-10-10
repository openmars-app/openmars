import type { OtherPlayerDto, PlayerDto, ResourceType } from "@/types/generated/api-types.ts";

export interface StorageSelection {
  resourceType: ResourceType;
  amount: number;
  target: string;
  selectorTags?: string[];
  eligibleCardIds?: string[];
}

export interface StorageNeed {
  resourceType: ResourceType;
  amount: number;
  selectorTags?: string[];
  eligibleCardIds?: string[];
}

const STORAGE_RESOURCES: ResourceType[] = [
  "animal",
  "microbe",
  "floater",
  "science",
  "asteroid",
  "card-resource",
];

const TARGETABLE_RESOURCES: ResourceType[] = [
  "credit",
  "steel",
  "titanium",
  "plant",
  "energy",
  "heat",
  "credit-production",
  "steel-production",
  "titanium-production",
  "plant-production",
  "energy-production",
  "heat-production",
];

export function getAllAnyCardStorageSelections(
  outputs: any[] | undefined,
  eligibleCardIds?: string[],
): StorageSelection[] {
  if (!outputs) return [];

  const results: StorageSelection[] = [];

  for (const output of outputs) {
    if (
      output.target === "any-card" &&
      ["remove-dependency", "retire-dependency"].includes(output.type)
    ) {
      results.push({
        resourceType: output.type,
        amount: 1,
        target: output.target,
        eligibleCardIds: eligibleCardIds ?? [],
      });
      continue;
    }
    if (
      output.amount > 0 &&
      (output.target === "any-card" || output.target === "self-card") &&
      STORAGE_RESOURCES.includes(output.type as ResourceType)
    ) {
      let selectorTags: string[] | undefined;
      if (output.selectors) {
        selectorTags = output.selectors.flatMap((s: any) => s.tags || []);
      }
      if (output.target === "any-card") {
        results.push({
          resourceType: output.type as ResourceType,
          amount: output.amount || 1,
          target: output.target as string,
          selectorTags,
        });
      }
    }
  }

  return results;
}

export function needsCardStorageSelection(outputs: any[] | undefined): StorageSelection | null {
  if (!outputs) return null;

  for (const output of outputs) {
    if (
      output.amount > 0 &&
      (output.target === "any-card" || output.target === "self-card") &&
      STORAGE_RESOURCES.includes(output.type as ResourceType)
    ) {
      let selectorTags: string[] | undefined;
      if (output.selectors) {
        selectorTags = output.selectors.flatMap((s: any) => s.tags || []);
      }
      return {
        resourceType: output.type as ResourceType,
        amount: output.amount || 1,
        target: output.target as string,
        selectorTags,
      };
    }
  }

  return null;
}

export function needsTargetPlayerSelection(
  outputs: any[] | undefined,
  otherPlayers: OtherPlayerDto[] | undefined,
): { resourceType: ResourceType; amount: number; isSteal: boolean } | null {
  if (!outputs) return null;
  if (!otherPlayers || otherPlayers.length === 0) return null;

  for (const output of outputs) {
    if (output.targetRestriction) continue;
    if (
      (output.target === "any-player" || output.target === "steal-any-player") &&
      TARGETABLE_RESOURCES.includes(output.type as ResourceType)
    ) {
      return {
        resourceType: output.type as ResourceType,
        amount: output.amount || 1,
        isSteal: output.target === "steal-any-player",
      };
    }
  }

  return null;
}

export function needsCardResourceInput(
  inputs: any[] | undefined,
  outputs?: any[] | undefined,
): { resourceType: ResourceType; amount: number } | null {
  if (inputs) {
    for (const input of inputs) {
      if (input.target === "steal-from-any-card") {
        return {
          resourceType: input.type as ResourceType,
          amount: input.amount || 1,
        };
      }
    }
  }

  if (outputs) {
    for (const output of outputs) {
      if (output.target === "steal-from-any-card") {
        return {
          resourceType: output.type as ResourceType,
          amount: output.amount || 1,
        };
      }
    }
  }

  return null;
}

export function getVariableAmountInfo(
  inputs: any[] | undefined,
  outputs: any[] | undefined,
  currentPlayer: PlayerDto | null,
  inputOptions?: import("@/types/generated/api-types.ts").BehaviorInputOptionsDto,
): { resourceType: ResourceType; maxAmount: number } | null {
  if (!currentPlayer) return null;
  if (inputOptions?.variableAmount) {
    return {
      resourceType: inputOptions.variableAmount.resourceType,
      maxAmount: inputOptions.variableAmount.max,
    };
  }

  if (inputs) {
    for (const input of inputs) {
      if (!input.variableAmount) continue;
      const resType = input.type as string;
      let max = 0;
      const resources = currentPlayer.resources;
      if (resType === "energy") {
        max = resources.energy;
      } else if (resType === "heat") {
        max = resources.heat;
      } else if (resType === "credit") {
        max = resources.credits;
      } else if (resType === "steel") {
        max = resources.steel;
      } else if (resType === "titanium") {
        max = resources.titanium;
      } else if (resType === "plant") {
        max = resources.plants;
      }
      if (max > 0) return { resourceType: resType, maxAmount: max };
    }
  }

  if (outputs) {
    for (const output of outputs) {
      if (!output.variableAmount || output.amount >= 0) continue;
      const resType = output.type as string;
      let max = 0;
      const production = currentPlayer.production;
      if (resType === "heat-production") {
        max = production.heat;
      } else if (resType === "energy-production") {
        max = production.energy;
      } else if (resType === "credit-production") {
        max = production.credits;
      } else if (resType === "steel-production") {
        max = production.steel;
      } else if (resType === "titanium-production") {
        max = production.titanium;
      } else if (resType === "plant-production") {
        max = production.plants;
      }
      if (max > 0) return { resourceType: resType, maxAmount: max };
    }
  }

  return null;
}

export function collectStorageNeeds(behaviors: any[] | undefined): StorageNeed[] {
  const allStorageNeeds: StorageNeed[] = [];
  for (const behavior of behaviors || []) {
    const selections = getAllAnyCardStorageSelections(behavior.outputs);
    for (const sel of selections) {
      allStorageNeeds.push({
        resourceType: sel.resourceType,
        amount: sel.amount,
        selectorTags: sel.selectorTags,
      });
    }
  }
  return allStorageNeeds;
}
