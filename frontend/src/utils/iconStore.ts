import { assetUrl } from "@/assets";
/**
 * Centralized icon asset path lookup store.
 * All icon path mappings are defined here to avoid duplication across components.
 */

export const APPROVED_ICONS: { [key: string]: string } = {
  colony: assetUrl("icons/behaviors/colony", 128),
  "colony-count": assetUrl("icons/behaviors/colony", 128),
  "colony-bonus": assetUrl("icons/behaviors/colony", 128),
  "colony-tile-add": assetUrl("icons/behaviors/colony", 128),
  trade: assetUrl("icons/behaviors/trade", 128),
  "trade-fleet": assetUrl("icons/behaviors/fleet", 128),
  "colony-track-step": assetUrl("icons/behaviors/colony-track", 128),
  "colony-track-decrease": assetUrl("icons/behaviors/colony-track-decrease", 128),
  "card-resource": assetUrl("icons/behaviors/stored", 128),
  "card-play": assetUrl("icons/behaviors/project-paper", 128),
  "land-claim": assetUrl("icons/behaviors/claim", 128),
  award: assetUrl("icons/behaviors/award", 128),
  "award-fund": assetUrl("icons/behaviors/award", 128),
  "action-reuse": assetUrl("icons/behaviors/reuse", 128),
  "ignore-global-requirements": assetUrl("icons/behaviors/requirements", 128),
  "project-paper": assetUrl("icons/behaviors/project-paper", 128),
  "global-parameter-lenience": assetUrl("icons/behaviors/requirements", 128),
  resource: assetUrl("icons/behaviors/resources", 128),
  resources: assetUrl("icons/behaviors/resources", 128),
  "standard-project": assetUrl("icons/behaviors/standard-project", 128),
  generation: assetUrl("icons/behaviors/generation", 128),
  protection: assetUrl("icons/behaviors/shield", 128),
  "animal-protection": assetUrl("icons/behaviors/shield-animal", 128),
  "prelude-card": assetUrl("icons/behaviors/prelude-card", 128),
  "corporation-badge": assetUrl("icons/behaviors/corporation-badge", 128),
  copy: assetUrl("icons/behaviors/copy", 128),
  "copy-effect": assetUrl("icons/behaviors/copy-effect", 128),
  actions: assetUrl("icons/behaviors/actions", 128),
  action: assetUrl("icons/behaviors/action", 128),
  "extra-actions": assetUrl("icons/behaviors/actions", 128),
  adjacency: assetUrl("icons/behaviors/adjacency", 128),
  commerce: assetUrl("icons/behaviors/commerce", 128),
  crater: assetUrl("icons/behaviors/crater", 128),
  dam: assetUrl("icons/behaviors/dam", 128),
  ecozone: assetUrl("icons/behaviors/ecozone", 128),
  industry: assetUrl("icons/behaviors/industry", 128),
  magnetic: assetUrl("icons/behaviors/magnetic", 128),
  mine: assetUrl("icons/behaviors/mine", 128),
  mohole: assetUrl("icons/behaviors/mohole", 128),
  nuclear: assetUrl("icons/behaviors/nuclear", 128),
  preserve: assetUrl("icons/behaviors/preserve", 128),
  restricted: assetUrl("icons/behaviors/restricted", 128),
  tree: assetUrl("icons/behaviors/tree", 128),
  "award-banker": assetUrl("icons/awards/banker", 128),
  "award-benefactor": assetUrl("icons/awards/benefactor", 128),
  "award-celebrity": assetUrl("icons/awards/celebrity", 128),
  "award-contractor": assetUrl("icons/awards/contractor", 128),
  "award-cultivator": assetUrl("icons/awards/cultivator", 128),
  "award-desert-settler": assetUrl("icons/awards/desert-settler", 128),
  "award-estate-dealer": assetUrl("icons/awards/estate-dealer", 128),
  "award-excentric": assetUrl("icons/awards/excentric", 128),
  "award-industrialist": assetUrl("icons/awards/industrialist", 128),
  "award-landlord": assetUrl("icons/awards/landlord", 128),
  "award-magnate": assetUrl("icons/awards/magnate", 128),
  "award-miner": assetUrl("icons/awards/miner", 128),
  "award-scientist": assetUrl("icons/awards/scientist", 128),
  "award-space-baron": assetUrl("icons/awards/space-baron", 128),
  "award-thermalist": assetUrl("icons/awards/thermalist", 128),
  "award-venuphile": assetUrl("icons/awards/venuphile", 128),
  "milestone-builder": assetUrl("icons/milestones/builder", 128),
  "milestone-diversifier": assetUrl("icons/milestones/diversifier", 128),
  "milestone-ecologist": assetUrl("icons/milestones/ecologist", 128),
  "milestone-energizer": assetUrl("icons/milestones/energizer", 128),
  "milestone-gardener": assetUrl("icons/milestones/gardener", 128),
  "milestone-generalist": assetUrl("icons/milestones/generalist", 128),
  "milestone-hoverlord": assetUrl("icons/milestones/hoverlord", 128),
  "milestone-legend": assetUrl("icons/milestones/legend", 128),
  "milestone-mayor": assetUrl("icons/milestones/mayor", 128),
  "milestone-planner": assetUrl("icons/milestones/planner", 128),
  "milestone-polar-explorer": assetUrl("icons/milestones/polar-explorer", 128),
  "milestone-rim-settler": assetUrl("icons/milestones/rim-settler", 128),
  "milestone-specialist": assetUrl("icons/milestones/specialist", 128),
  "milestone-tactician": assetUrl("icons/milestones/tactician", 128),
  "milestone-terraformer": assetUrl("icons/milestones/terraformer", 128),
  "milestone-tycoon": assetUrl("icons/milestones/tycoon", 128),
  "colony-callisto": assetUrl("icons/colonies/callisto", 128),
  "colony-ceres": assetUrl("icons/colonies/ceres", 128),
  "colony-enceladus": assetUrl("icons/colonies/enceladus", 128),
  "colony-europa": assetUrl("icons/colonies/europa", 128),
  "colony-ganymede": assetUrl("icons/colonies/ganymede", 128),
  "colony-io": assetUrl("icons/colonies/io", 128),
  "colony-luna": assetUrl("icons/colonies/luna", 128),
  "colony-miranda": assetUrl("icons/colonies/miranda", 128),
  "colony-pluto": assetUrl("icons/colonies/pluto", 128),
  "colony-titan": assetUrl("icons/colonies/titan", 128),
  "colony-triton": assetUrl("icons/colonies/triton", 128),
};

export const RESOURCE_ICONS: { [key: string]: string } = {
  credit: assetUrl("icons/resources/megacredit", 128),
  steel: assetUrl("icons/resources/steel", 128),
  titanium: assetUrl("icons/resources/titanium", 128),
  plant: assetUrl("icons/resources/plant", 128),
  energy: assetUrl("icons/resources/power", 128),
  power: assetUrl("icons/resources/power", 128),
  heat: assetUrl("icons/resources/heat", 128),
  microbe: assetUrl("icons/resources/microbe", 128),
  animal: assetUrl("icons/resources/animal", 128),
  floater: assetUrl("icons/resources/floater", 128),
  science: assetUrl("icons/resources/science", 128),
  asteroid: assetUrl("icons/resources/asteroid", 128),
  disease: assetUrl("icons/resources/disease", 128),
  tr: assetUrl("icons/resources/tr", 128),
  "terraform-rating": assetUrl("icons/resources/tr", 128),
  fighter: assetUrl("icons/resources/fighter", 128),
  camp: assetUrl("icons/resources/camp", 128),
  preservation: assetUrl("icons/resources/preservation", 128),
  data: assetUrl("icons/resources/data", 128),
  specialized: assetUrl("icons/resources/specialized-robot", 128),
  "specialized-robot": assetUrl("icons/resources/specialized-robot", 128),
  delegate: assetUrl("icons/resources/director", 128),
  director: assetUrl("icons/resources/director", 128),
  influence: assetUrl("icons/ui/influence", 128),
  // Production variants
  "credit-production": assetUrl("icons/resources/megacredit", 128),
  "steel-production": assetUrl("icons/resources/steel", 128),
  "titanium-production": assetUrl("icons/resources/titanium", 128),
  "plant-production": assetUrl("icons/resources/plant", 128),
  "energy-production": assetUrl("icons/resources/power", 128),
  "heat-production": assetUrl("icons/resources/heat", 128),
};

export const TAG_ICONS: { [key: string]: string } = {
  earth: assetUrl("icons/tags/earth", 128),
  science: assetUrl("icons/tags/science", 128),
  plant: assetUrl("icons/tags/plant", 128),
  microbe: assetUrl("icons/tags/microbe", 128),
  animal: assetUrl("icons/tags/animal", 128),
  power: assetUrl("icons/tags/power", 128),
  space: assetUrl("icons/tags/space", 128),
  building: assetUrl("icons/tags/building", 128),
  city: assetUrl("icons/tags/city", 128),
  jovian: assetUrl("icons/tags/jovian", 128),
  venus: assetUrl("icons/tags/venus", 128),
  event: assetUrl("icons/tags/event", 128),
  "mars-tag": assetUrl("icons/tags/mars", 128),
  moon: assetUrl("icons/tags/moon", 128),
  wild: assetUrl("icons/tags/wild", 128),
  wildlife: assetUrl("icons/tags/wild", 128), // Use wild.png as fallback
  clone: assetUrl("icons/tags/clone", 128),
  crime: assetUrl("icons/tags/crime", 128),
};

export const TILE_ICONS: { [key: string]: string } = {
  "city-placement": assetUrl("icons/placements/city", 128),
  "ocean-placement": assetUrl("icons/placements/ocean", 128),
  "greenery-placement": assetUrl("icons/placements/greenery-no-oxygen", 128),
  "land-claim": assetUrl("icons/placements/special", 128),
  "tile-placement": assetUrl("icons/placements/special", 128),
  tile: assetUrl("icons/placements/special", 128),
  "city-tile": assetUrl("icons/placements/city", 128),
  "ocean-tile": assetUrl("icons/placements/ocean", 128),
  "greenery-tile": assetUrl("icons/placements/greenery-no-oxygen", 128),
  "greenery-placed": assetUrl("icons/placements/greenery-no-oxygen", 128), // For triggered effects
  "ocean-placed": assetUrl("icons/placements/ocean", 128), // For triggered effects
  "city-placed": assetUrl("icons/placements/city", 128), // For triggered effects
  "volcano-tile": assetUrl("icons/placements/volcano", 128),
  "volcano-placement": assetUrl("icons/placements/volcano", 128),
  "colony-tile-add": assetUrl("icons/placements/colony", 128),
  colony: assetUrl("icons/placements/colony", 128),
  "colony-count": assetUrl("icons/placements/colony", 128),
  "colony-bonus": assetUrl("icons/placements/colony", 128),
  greenery: assetUrl("icons/placements/greenery-no-oxygen", 128),
  trade: assetUrl("icons/placements/trade", 128),
  "trade-fleet": assetUrl("icons/placements/trade", 128),
  "colony-track-step": assetUrl("icons/placements/colony", 128),
};

export const GLOBAL_PARAMETER_ICONS: { [key: string]: string } = {
  temperature: assetUrl("icons/parameters/temperature", 128),
  oxygen: assetUrl("icons/parameters/oxygen", 128),
  ocean: assetUrl("icons/placements/ocean", 128),
  venus: assetUrl("icons/parameters/venus", 128),
};

export const SPECIAL_ICONS: { [key: string]: string } = {
  "card-draw": assetUrl("icons/resources/card", 128),
  "card-take": assetUrl("icons/resources/card", 128),
  "card-peek": assetUrl("icons/resources/card", 128),
  "card-discard": assetUrl("icons/resources/card", 128),
  "card-buy": assetUrl("icons/resources/card", 128),
  card: assetUrl("icons/resources/card", 128),
  tag: assetUrl("icons/tags/wild", 128),
  discount: assetUrl("icons/resources/megacredit", 128),
  milestone: assetUrl("icons/ui/checkmark", 128),
  award: assetUrl("icons/ui/first-player", 128),
  asterisk: assetUrl("icons/ui/asterisc", 128),
  asterisc: assetUrl("icons/ui/asterisc", 128), // Support both spellings
  mars: assetUrl("icons/ui/mars", 128),
  arrow: assetUrl("icons/ui/arrow", 128),
  "card-resource": assetUrl("icons/resources/wild", 128),
};

/**
 * Get the icon path for a given icon type.
 * Searches across all icon categories.
 * Resources are checked first since card behaviors use resource types,
 * and tags are checked after for tag displays.
 */
export function getIconPath(iconType: string): string | null {
  const cleanType = iconType?.toLowerCase().replace(/[_\s]/g, "-");

  if (APPROVED_ICONS[cleanType]) {
    return APPROVED_ICONS[cleanType];
  }

  if (RESOURCE_ICONS[cleanType]) {
    return RESOURCE_ICONS[cleanType];
  }

  if (TILE_ICONS[cleanType]) {
    return TILE_ICONS[cleanType];
  }

  if (GLOBAL_PARAMETER_ICONS[cleanType]) {
    return GLOBAL_PARAMETER_ICONS[cleanType];
  }

  if (TAG_ICONS[cleanType]) {
    return TAG_ICONS[cleanType];
  }

  if (SPECIAL_ICONS[cleanType]) {
    return SPECIAL_ICONS[cleanType];
  }

  // Handle production suffix
  if (cleanType.includes("-production")) {
    const baseResourceType = cleanType.replace("-production", "");
    if (RESOURCE_ICONS[baseResourceType]) {
      return RESOURCE_ICONS[baseResourceType];
    }
  }

  // Handle -tag suffix to force tag icon lookup (e.g., "plant-tag" -> tag icon for plant)
  if (cleanType.endsWith("-tag")) {
    const baseTagType = cleanType.replace("-tag", "");
    if (TAG_ICONS[baseTagType]) {
      return TAG_ICONS[baseTagType];
    }
  }

  return null;
}

/**
 * Check if an icon type is a tag.
 */
export function isTagIcon(iconType: string): boolean {
  const cleanType = iconType?.toLowerCase().replace(/[_\s]/g, "-");
  return TAG_ICONS[cleanType] !== undefined;
}

/**
 * Get the tag icon path specifically.
 * Use this when you know you need the tag icon (e.g., for card tag displays).
 */
export function getTagIconPath(iconType: string): string | null {
  const cleanType = iconType?.toLowerCase().replace(/[_\s]/g, "-");
  return TAG_ICONS[cleanType] || null;
}

/**
 * Check if an icon type is a tile.
 */
export function isTileIcon(iconType: string): boolean {
  const cleanType = iconType?.toLowerCase().replace(/[_\s]/g, "-");
  return TILE_ICONS[cleanType] !== undefined;
}

/**
 * Check if an icon type is a global parameter.
 */
export function isGlobalParameterIcon(iconType: string): boolean {
  const cleanType = iconType?.toLowerCase().replace(/[_\s]/g, "-");
  return GLOBAL_PARAMETER_ICONS[cleanType] !== undefined;
}
