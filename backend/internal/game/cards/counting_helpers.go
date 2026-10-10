package cards

import (
	"openmars/internal/game/board"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"slices"
)

// CardRegistryInterface defines the interface for looking up cards
type CardRegistryInterface interface {
	GetByID(cardID string) (*Card, error)
}

// CountPlayerTiles counts tiles owned by a player on the board.
// If tileType is nil, counts all tiles owned by the player.
// If tileType is specified, counts only tiles of that type.
func CountPlayerTiles(playerID string, b *board.Board, tileType *shared.ResourceType) int {
	count := 0
	tiles := b.Tiles()
	for _, tile := range tiles {
		if tile.OwnerID == nil || *tile.OwnerID != playerID {
			continue
		}
		if tile.OccupiedBy == nil {
			continue
		}
		if tileType != nil && tile.OccupiedBy.Type != *tileType {
			continue
		}
		count++
	}
	return count
}

// CountPlayerTagsByType counts actual tags, without interpreting wild tags.
func CountPlayerTagsByType(p *player.Player, cardRegistry CardRegistryInterface, tagType shared.CardTag) int {
	return PlayerTagCounts(p, cardRegistry)[tagType]
}

// HasTag checks if a card has a specific tag.
func HasTag(card *Card, tag shared.CardTag) bool {
	for _, cardTag := range card.Tags {
		if cardTag == tag {
			return true
		}
	}
	return false
}

// CountPlayerNonOceanTiles counts all non-ocean tiles owned by a player.
func CountPlayerNonOceanTiles(playerID string, b *board.Board) int {
	count := 0
	for _, tile := range b.Tiles() {
		if tile.OwnerID == nil || *tile.OwnerID != playerID {
			continue
		}
		if tile.OccupiedBy == nil || tile.OccupiedBy.Type == shared.ResourceOceanTile {
			continue
		}
		count++
	}
	return count
}

// CountAllNonOceanTiles counts all non-ocean tiles on the board.
func CountAllNonOceanTiles(b *board.Board) int {
	count := 0
	for _, tile := range b.Tiles() {
		if tile.OccupiedBy != nil && tile.OccupiedBy.Type != shared.ResourceOceanTile {
			count++
		}
	}
	return count
}

// CountAllTilesOfType counts all tiles of a specific type on the board, regardless of owner.
func CountAllTilesOfType(b *board.Board, tileType shared.ResourceType) int {
	count := 0
	tiles := b.Tiles()
	for _, tile := range tiles {
		if tile.OccupiedBy != nil && tile.OccupiedBy.Type == tileType {
			count++
		}
	}
	return count
}

// CountTilesOfTypeByLocation counts tiles of a specific type, optionally filtered by location and owner.
// If location is "mars", only counts tiles with TileLocationMars.
// If location is nil or "anywhere", counts all tiles of that type.
func CountTilesOfTypeByLocation(b *board.Board, tileType shared.ResourceType, location *string, ownerID *string) int {
	count := 0
	tiles := b.Tiles()
	for _, tile := range tiles {
		if tile.OccupiedBy == nil || tile.OccupiedBy.Type != tileType {
			continue
		}
		if location != nil && *location == "mars" && tile.Location != board.TileLocationMars {
			continue
		}
		if ownerID != nil && (tile.OwnerID == nil || *tile.OwnerID != *ownerID) {
			continue
		}
		count++
	}
	return count
}

// ColonyCounter supplies colony ownership counts without tying counters to game state.
type ColonyCounter interface {
	CountAllColonies() int
	CountPlayerColonies(playerID string) int
}

// CountPerCondition is the unified counter for PerCondition evaluation.
// Used by card behaviors (scaled outputs), VP calculations, and award scoring.
// Parameters:
//   - per: the condition to evaluate
//   - sourceCardID: card ID for self-card storage or adjacency (empty string if N/A)
//   - p: the player to evaluate for
//   - b: the game board (nil-safe for non-tile conditions)
//   - cardRegistry: card registry for tag lookups (nil-safe for non-tag conditions)
//   - allPlayers: all players in game (only needed for any-player/other-players targeting, nil otherwise)
func CountPerCondition(
	per *shared.PerCondition,
	sourceCardID string,
	p *player.Player,
	b *board.Board,
	cardRegistry CardRegistryInterface,
	allPlayers []*player.Player,
	colonies ColonyCounter,
	tagContext TagCountContext,
) int {
	if per == nil {
		return 0
	}

	if per.ResourceType == shared.ResourcePermanentProductionTypes {
		if p == nil {
			return 0
		}
		return p.Resources().PermanentProduction().PositiveTypes()
	}

	if per.ResourceType == shared.ResourceColonyCount || per.ResourceType == shared.ResourceColony {
		if colonies == nil {
			return 0
		}
		if per.Target != nil && *per.Target == "self-player" {
			return colonies.CountPlayerColonies(p.ID())
		}
		if per.Target != nil && *per.Target == "other-players" {
			return colonies.CountAllColonies() - colonies.CountPlayerColonies(p.ID())
		}
		return colonies.CountAllColonies()
	}

	if per.ResourceType == shared.ResourceCardCount && per.Zone == "played" && cardRegistry != nil {
		count := 0
		players := []*player.Player{p}
		if per.Target != nil && (*per.Target == "any-player" || *per.Target == "other-players") {
			players = allPlayers
		}
		for _, owner := range players {
			if per.Target != nil && *per.Target == "other-players" && owner.ID() == p.ID() {
				continue
			}
			ids := append([]string(nil), owner.PlayedCards().Cards()...)
			if corp := owner.CorporationID(); corp != "" && !slices.Contains(ids, corp) {
				ids = append(ids, corp)
			}
			if owner.ID() == p.ID() && per.IncludeSource && sourceCardID != "" && !slices.Contains(ids, sourceCardID) {
				ids = append(ids, sourceCardID)
			}
			for _, id := range ids {
				card, err := cardRegistry.GetByID(id)
				if err == nil && (len(per.Selectors) == 0 || MatchesAnySelector(card, per.Selectors)) {
					count++
				}
			}
		}
		return count
	}

	// Card storage (e.g., animals on this card)
	if per.Target != nil && *per.Target == "self-card" {
		if sourceCardID != "" {
			return p.Resources().GetCardStorage(sourceCardID)
		}
		return 0
	}

	// Adjacent to tile placed by this card (e.g., Capital: 1 VP per adjacent ocean)
	if per.AdjacentToSelfTile && b != nil {
		return countAdjacentTilesOfTypeForCard(b, sourceCardID, per.ResourceType)
	}

	// Adjacent to tile type (e.g., World Tree: 1 VP per adjacent forest)
	// Skip for land-tile — handled in the tile counting switch with dedicated logic
	if per.AdjacentToTileType != nil && b != nil && per.ResourceType != shared.ResourceNonOceanTile {
		return countAdjacentTilesOfType(p.ID(), b, per.ResourceType, *per.AdjacentToTileType)
	}

	if (len(per.Tags) > 0 || per.Tag != nil) && cardRegistry != nil {
		tags := append([]shared.CardTag(nil), per.Tags...)
		if per.Tag != nil {
			tags = append(tags, *per.Tag)
		}
		owners := []*player.Player{p}
		if per.Target != nil && (*per.Target == "any-player" || *per.Target == "other-players") {
			owners = allPlayers
		}
		count := 0
		for _, owner := range owners {
			if per.Target != nil && *per.Target == "other-players" && owner.ID() == p.ID() {
				continue
			}
			count += CountPlayerTags(owner, cardRegistry, tags, tagContext)
		}
		if per.IncludeSource && sourceCardID != "" && (per.Target == nil || *per.Target == "self-player" || *per.Target == "any-player") {
			source, err := cardRegistry.GetByID(sourceCardID)
			if err == nil && (source.Type == CardTypeEvent || (!slices.Contains(p.PlayedCards().Cards(), sourceCardID) && p.CorporationID() != sourceCardID)) {
				for _, tag := range source.Tags {
					if tag != shared.TagWild && slices.Contains(tags, tag) {
						count++
					}
				}
			}
		}
		return count
	}

	// Tile counting
	if b != nil {
		switch per.ResourceType {
		case shared.ResourceOceanTile:
			return CountAllTilesOfType(b, shared.ResourceOceanTile)
		case shared.ResourceNonOceanTile:
			if per.MinRow != nil && per.Target != nil && *per.Target == "self-player" {
				return countPlayerTilesOnRows(p.ID(), b, *per.MinRow)
			}
			if per.AdjacentToTileType != nil && per.Target != nil && *per.Target == "self-player" {
				return countPlayerTilesAdjacentToOcean(p.ID(), b)
			}
			if per.Target != nil && *per.Target == "self-player" {
				return CountPlayerNonOceanTiles(p.ID(), b)
			}
			return CountAllNonOceanTiles(b)
		case shared.ResourceCityTile:
			var ownerID *string
			if per.Target != nil && *per.Target == "self-player" {
				id := p.ID()
				ownerID = &id
			}
			return CountTilesOfTypeByLocation(b, shared.ResourceCityTile, per.Location, ownerID)
		case shared.ResourceGreeneryTile:
			if per.Target != nil && *per.Target == "self-player" {
				rt := shared.ResourceGreeneryTile
				return CountPlayerTiles(p.ID(), b, &rt)
			}
			return CountAllTilesOfType(b, shared.ResourceGreeneryTile)
		}
	}

	// Terraform rating
	if per.ResourceType == shared.ResourceTR {
		return p.Resources().TerraformRating()
	}

	// Cards in hand
	if per.ResourceType == shared.ResourceCardCount {
		return p.Hand().CardCount()
	}

	// Card storage resources (floater, animal, microbe, science, etc.)
	if isCardStorageType(per.ResourceType) && cardRegistry != nil {
		return CountPlayerCardStorageByType(p, cardRegistry, per.ResourceType)
	}

	// Production counting (e.g., credit-production)
	if isProductionType(per.ResourceType) {
		return p.Resources().Production().GetAmount(per.ResourceType)
	}

	// Resource counting (e.g., heat, steel, titanium)
	if isBasicResourceType(per.ResourceType) {
		return p.Resources().Get().GetAmount(per.ResourceType)
	}

	// Distinct tag count (Diversifier: 8 different tags)
	if per.ResourceType == shared.ResourceDistinctTagCount && cardRegistry != nil {
		return countDistinctTags(p, cardRegistry, tagContext)
	}

	// Cards with requirements (Tactician: 5 cards with requirements)
	if per.ResourceType == shared.ResourceCardsWithRequirements && cardRegistry != nil {
		return countCardsWithRequirements(p, cardRegistry)
	}

	// Max single production (Specialist: 10 of any single production)
	if per.ResourceType == shared.ResourceMaxSingleProduction {
		return maxSingleProduction(p)
	}

	// Played card type count (Tycoon, Legend, Magnate, Celebrity)
	if per.ResourceType == shared.ResourcePlayedCardTypeCount && cardRegistry != nil {
		if per.MinCost != nil {
			return countCardsWithMinCost(p, cardRegistry, *per.MinCost)
		}
		if per.CardTypeFilter != nil {
			return countPlayedCardsByType(p, cardRegistry, *per.CardTypeFilter)
		}
	}

	// Total card storage (Excentric: most resources on cards)
	if per.ResourceType == shared.ResourceTotalCardStorage {
		return countTotalCardStorage(p)
	}

	// Fallback: try to count as a tag type
	if cardRegistry != nil {
		return CountPlayerTags(p, cardRegistry, []shared.CardTag{shared.CardTag(per.ResourceType)}, tagContext)
	}

	return 0
}

func isProductionType(rt shared.ResourceType) bool {
	switch rt {
	case shared.ResourceCreditProduction, shared.ResourceSteelProduction,
		shared.ResourceTitaniumProduction, shared.ResourcePlantProduction,
		shared.ResourceEnergyProduction, shared.ResourceHeatProduction:
		return true
	}
	return false
}

func isBasicResourceType(rt shared.ResourceType) bool {
	switch rt {
	case shared.ResourceCredit, shared.ResourceSteel, shared.ResourceTitanium,
		shared.ResourcePlant, shared.ResourceEnergy, shared.ResourceHeat:
		return true
	}
	return false
}

func isCardStorageType(rt shared.ResourceType) bool {
	switch rt {
	case shared.ResourceFloater, shared.ResourceAnimal, shared.ResourceMicrobe,
		shared.ResourceScience, shared.ResourceAsteroid, shared.ResourceFighter,
		shared.ResourceDisease, shared.ResourceCamp:
		return true
	}
	return false
}

// CountPlayerResources counts a resource in the player pool or on owned cards.
func CountPlayerResources(p *player.Player, cardRegistry CardRegistryInterface, resourceType shared.ResourceType) int {
	if isCardStorageType(resourceType) {
		if cardRegistry == nil {
			return 0
		}
		return CountPlayerCardStorageByType(p, cardRegistry, resourceType)
	}
	return p.Resources().Get().GetAmount(resourceType)
}

// CountPlayerCardStorageByType sums card storage across all played cards and corporation
// where the card's storage type matches the given type.
func CountPlayerCardStorageByType(p *player.Player, cardRegistry CardRegistryInterface, storageType shared.ResourceType) int {
	total := 0
	for _, cardID := range p.PlayedCards().Cards() {
		card, err := cardRegistry.GetByID(cardID)
		if err != nil || card.ResourceStorage == nil || card.ResourceStorage.Type != storageType {
			continue
		}
		total += p.Resources().GetCardStorage(cardID)
	}

	if corpID := p.CorporationID(); corpID != "" {
		if corp, err := cardRegistry.GetByID(corpID); err == nil && corp.ResourceStorage != nil && corp.ResourceStorage.Type == storageType {
			total += p.Resources().GetCardStorage(corpID)
		}
	}

	return total
}

func countDistinctTags(p *player.Player, cardRegistry CardRegistryInterface, tagContext TagCountContext) int {
	counts := PlayerTagCounts(p, cardRegistry)
	distinct, missing := 0, 0
	for tag, count := range counts {
		if count > 0 && tag != shared.TagWild && tag != shared.TagEvent {
			distinct++
		}
	}
	for _, tag := range []shared.CardTag{shared.TagSpace, shared.TagEarth, shared.TagScience, shared.TagPower, shared.TagBuilding, shared.TagMicrobe, shared.TagAnimal, shared.TagPlant, shared.TagCity, shared.TagVenus, shared.TagJovian} {
		if counts[tag] == 0 {
			missing++
		}
	}
	return distinct + min(missing, eligibleWildTags(p, cardRegistry, tagContext, counts))
}

func countCardsWithRequirements(p *player.Player, cardRegistry CardRegistryInterface) int {
	count := 0
	for _, cardID := range p.PlayedCards().Cards() {
		card, err := cardRegistry.GetByID(cardID)
		if err != nil {
			continue
		}
		if card.Requirements != nil && len(card.Requirements.Items) > 0 {
			count++
		}
	}
	return count
}

func countPlayerTilesOnRows(playerID string, b *board.Board, minRow int) int {
	count := 0
	for _, tile := range b.Tiles() {
		if tile.OwnerID == nil || *tile.OwnerID != playerID {
			continue
		}
		if tile.OccupiedBy == nil {
			continue
		}
		if tile.Coordinates.R >= minRow {
			count++
		}
	}
	return count
}

func maxSingleProduction(p *player.Player) int {
	prod := p.Resources().Production()
	best := 0
	for _, rt := range []shared.ResourceType{
		shared.ResourceCreditProduction,
		shared.ResourceSteelProduction,
		shared.ResourceTitaniumProduction,
		shared.ResourcePlantProduction,
		shared.ResourceEnergyProduction,
		shared.ResourceHeatProduction,
	} {
		if v := prod.GetAmount(rt); v > best {
			best = v
		}
	}
	return best
}

func countPlayedCardsByType(p *player.Player, cardRegistry CardRegistryInterface, filter string) int {
	count := 0
	for _, cardID := range p.PlayedCards().Cards() {
		card, err := cardRegistry.GetByID(cardID)
		if err != nil {
			continue
		}
		switch filter {
		case "automated":
			if card.Type == CardTypeAutomated {
				count++
			}
		case "event":
			if card.Type == CardTypeEvent {
				count++
			}
		case "automated+event":
			if card.Type == CardTypeAutomated || card.Type == CardTypeEvent {
				count++
			}
		}
	}
	return count
}

func countCardsWithMinCost(p *player.Player, cardRegistry CardRegistryInterface, minCost int) int {
	count := 0
	for _, cardID := range p.PlayedCards().Cards() {
		card, err := cardRegistry.GetByID(cardID)
		if err != nil {
			continue
		}
		if card.Cost >= minCost {
			count++
		}
	}
	return count
}

func countPlayerTilesAdjacentToOcean(playerID string, b *board.Board) int {
	tiles := b.Tiles()
	oceanPositions := make(map[shared.HexPosition]bool)
	for _, tile := range tiles {
		if tile.OccupiedBy != nil && tile.OccupiedBy.Type == shared.ResourceOceanTile {
			oceanPositions[tile.Coordinates] = true
		}
	}

	count := 0
	for _, tile := range tiles {
		if tile.OwnerID == nil || *tile.OwnerID != playerID {
			continue
		}
		if tile.OccupiedBy == nil || tile.OccupiedBy.Type == shared.ResourceOceanTile {
			continue
		}
		for _, neighbor := range tile.Coordinates.GetNeighbors() {
			if oceanPositions[neighbor] {
				count++
				break
			}
		}
	}
	return count
}

func countTotalCardStorage(p *player.Player) int {
	total := 0
	for _, cardID := range p.PlayedCards().Cards() {
		stored := p.Resources().GetCardStorage(cardID)
		if stored > 0 {
			total += stored
		}
	}
	return total
}
