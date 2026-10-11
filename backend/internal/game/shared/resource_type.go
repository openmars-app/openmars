package shared

// ResourceType represents different types of resources in the game
type ResourceType string

const (
	ResourceCopy     ResourceType = "copy"
	ResourceCredit   ResourceType = "credit"
	ResourceSteel    ResourceType = "steel"
	ResourceTitanium ResourceType = "titanium"
	ResourcePlant    ResourceType = "plant"
	ResourceEnergy   ResourceType = "energy"
	ResourceHeat     ResourceType = "heat"
	ResourceMicrobe  ResourceType = "microbe"
	ResourceAnimal   ResourceType = "animal"
	ResourceFloater  ResourceType = "floater"
	ResourceScience  ResourceType = "science"
	ResourceAsteroid ResourceType = "asteroid"
	ResourceFighter  ResourceType = "fighter"
	ResourceDisease  ResourceType = "disease"
	ResourceCamp     ResourceType = "camp"

	ResourceCardReveal  ResourceType = "card-reveal"
	ResourceCardDraw    ResourceType = "card-draw"
	ResourceCardTake    ResourceType = "card-take"
	ResourceCardPeek    ResourceType = "card-peek"
	ResourceCardBuy     ResourceType = "card-buy"
	ResourceCardDiscard ResourceType = "card-discard"

	ResourceCityPlacement     ResourceType = "city-placement"
	ResourceOceanPlacement    ResourceType = "ocean-placement"
	ResourceGreeneryPlacement ResourceType = "greenery-placement"
	ResourceVolcanoPlacement  ResourceType = "volcano-placement"
	ResourceTilePlacement     ResourceType = "tile-placement"

	ResourceCityTile               ResourceType = "city-tile"
	ResourceOceanTile              ResourceType = "ocean-tile"
	ResourceGreeneryTile           ResourceType = "greenery-tile"
	ResourceVolcanoTile            ResourceType = "volcano-tile"
	ResourceColony                 ResourceType = "colony"
	ResourceColonyTileAdd          ResourceType = "colony-tile-add"
	ResourceColonyCount            ResourceType = "colony-count"
	ResourceColonyBonus            ResourceType = "colony-bonus"
	ResourceNaturalPreserveTile    ResourceType = "natural-preserve-tile"
	ResourceMiningTile             ResourceType = "mining-tile"
	ResourceNuclearZoneTile        ResourceType = "nuclear-zone-tile"
	ResourceEcologicalZoneTile     ResourceType = "ecological-zone-tile"
	ResourceMoholeTile             ResourceType = "mohole-tile"
	ResourceRestrictedTile         ResourceType = "restricted-tile"
	ResourceCommercialDistrictTile ResourceType = "commercial-district-tile"
	ResourceIndustrialCenterTile   ResourceType = "industrial-center-tile"

	ResourceLandTile     ResourceType = "land"
	ResourceNonOceanTile ResourceType = "land-tile"
	ResourceOceanSpace   ResourceType = "ocean-space"

	ResourceTemperature     ResourceType = "temperature"
	ResourceOxygen          ResourceType = "oxygen"
	ResourceOcean           ResourceType = "ocean"
	ResourceVenus           ResourceType = "venus"
	ResourceTR              ResourceType = "tr"
	ResourceGlobalParameter ResourceType = "global-parameter"

	ResourceCreditProduction   ResourceType = "credit-production"
	ResourceSteelProduction    ResourceType = "steel-production"
	ResourceTitaniumProduction ResourceType = "titanium-production"
	ResourcePlantProduction    ResourceType = "plant-production"
	ResourceEnergyProduction   ResourceType = "energy-production"
	ResourceHeatProduction     ResourceType = "heat-production"
	ResourceAnyProduction      ResourceType = "any-production"

	ResourceEffect ResourceType = "effect"

	ResourceTag ResourceType = "tag"

	ResourceGlobalParameterLenience  ResourceType = "global-parameter-lenience"
	ResourceIgnoreGlobalRequirements ResourceType = "ignore-global-requirements"
	ResourceDefense                  ResourceType = "defense"
	ResourceDiscount                 ResourceType = "discount"
	ResourceValueModifier            ResourceType = "value-modifier"
	ResourcePaymentSubstitute        ResourceType = "payment-substitute"
	ResourceOceanAdjacencyBonus      ResourceType = "ocean-adjacency-bonus"

	ResourceLandClaim    ResourceType = "land-claim"
	ResourceCardResource ResourceType = "card-resource"
	ResourceActionReuse  ResourceType = "action-reuse"

	ResourceExtraActions    ResourceType = "extra-actions"
	ResourceTileDestruction ResourceType = "tile-destruction"
	ResourceTileReplacement ResourceType = "tile-replacement"
	ResourceBonusTags       ResourceType = "bonus-tags"

	ResourceWorldTreeTile ResourceType = "world-tree-tile"

	ResourceAwardFund       ResourceType = "award-fund"
	ResourceFreeTrade       ResourceType = "trade"
	ResourceCardCount       ResourceType = "card-count"
	ResourceColonyTrackStep ResourceType = "colony-track-step"
	ResourceTradeFleet      ResourceType = "trade-fleet"

	ResourceDistinctTagCount      ResourceType = "distinct-tag-count"
	ResourceCardsWithRequirements ResourceType = "cards-with-requirements"
	ResourceMaxSingleProduction   ResourceType = "max-single-production"
	ResourcePlayedCardTypeCount   ResourceType = "played-card-type-count"
	ResourceTotalCardStorage      ResourceType = "total-card-storage"
)

// IsForestTile returns true if the tile type counts as a forest (greenery or world-tree).
func IsForestTile(tileType ResourceType) bool {
	return tileType == ResourceGreeneryTile || tileType == ResourceWorldTreeTile
}

// ActionType constants for action-level selectors (discounts targeting specific game actions)
const (
	ActionCardBuying  = "card-buying"
	ActionCardPlaying = "card-playing"
	ActionColonyTrade = "colony-trade"
)

// ResourcePermanentProductionTypes counts distinct resources with positive permanent production.
const ResourcePermanentProductionTypes ResourceType = "permanent-production-types"
