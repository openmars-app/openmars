package tile

import (
	"context"
	"fmt"
	"log/slog"
	baseaction "openmars/internal/action"
	"openmars/internal/action/turn_management"
	"strconv"
	"strings"
	"time"

	"openmars/internal/events"
	"openmars/internal/game"
	"openmars/internal/game/board"
	"openmars/internal/game/cards"
	"openmars/internal/game/shared"
)

// TilePlacementResult contains information about a completed tile placement
type TilePlacementResult struct {
	TileType    string
	Source      string
	Hex         string
	OxygenSteps int
	TRGained    int
	OceanPlaced bool
	OnComplete  *shared.TileCompletionCallback
}

// SelectTileAction handles the business logic for selecting a tile position
type SelectTileAction struct {
	baseaction.BaseAction
	completionRegistry *TileCompletionRegistry
}

// NewSelectTileAction creates a new select tile action
func NewSelectTileAction(
	gameRepo game.GameRepository,
	cardRegistry cards.CardRegistry,
	stateRepo game.GameStateRepository,
	logger *slog.Logger,
) *SelectTileAction {
	return &SelectTileAction{
		BaseAction:         baseaction.NewBaseActionWithStateRepo(gameRepo, cardRegistry, stateRepo),
		completionRegistry: NewTileCompletionRegistry(stateRepo, cardRegistry),
	}
}

// Execute performs the select tile action and returns placement result
func (a *SelectTileAction) Execute(ctx context.Context, gameID string, playerID string, selectedHex string) (*TilePlacementResult, error) {
	log := a.InitLogger(gameID, playerID).With(slog.String("action", "select_tile"))
	log.Debug("Selecting tile", slog.String("hex", selectedHex))

	g, err := baseaction.ValidateActiveGame(ctx, a.GameRepository(), gameID, log)
	if err != nil {
		return nil, err
	}

	phase := g.CurrentPhase()
	if phase != shared.GamePhaseStartingSelection &&
		phase != shared.GamePhaseInitApplyCorp &&
		phase != shared.GamePhaseInitApplyPrelude {
		if err := baseaction.ValidateCurrentTurn(g, playerID, log); err != nil {
			return nil, err
		}
	}

	p, err := a.GetPlayerFromGame(g, playerID, log)
	if err != nil {
		return nil, err
	}

	pendingTileSelection := g.GetPendingTileSelection(playerID)
	if pendingTileSelection == nil {
		log.Warn("No pending tile selection found")
		return nil, fmt.Errorf("no pending tile selection found for player %s", playerID)
	}

	hexIsValid := false
	for _, availableHex := range pendingTileSelection.AvailableHexes {
		if availableHex == selectedHex {
			hexIsValid = true
			break
		}
	}
	if !hexIsValid {
		log.Warn("Invalid hex selection",
			slog.String("selected_hex", selectedHex),
			slog.Any("available_hexes", pendingTileSelection.AvailableHexes))
		return nil, fmt.Errorf("selected hex %s is not valid for placement", selectedHex)
	}

	if pendingTileSelection.SourceCardID != "" || pendingTileSelection.TileRestrictions != nil {
		valid := false
		for _, hex := range g.CalculateAvailableHexesForTile(pendingTileSelection.TileType, playerID, pendingTileSelection.TileRestrictions) {
			if hex == selectedHex {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("selected hex %s is no longer valid for placement", selectedHex)
		}
	}
	coords, err := parseHexPosition(selectedHex)
	if err != nil {
		log.Warn("Failed to parse hex coordinates", slog.String("hex", selectedHex), slog.Any("error", err))
		return nil, fmt.Errorf("invalid hex format: %w", err)
	}

	tileType := pendingTileSelection.TileType

	if tileType != "clear" && tileType != "tile-destruction" && !strings.HasPrefix(tileType, "tile-replacement:") {
		tile, err := g.Board().GetTile(*coords)
		if err != nil {
			return nil, err
		}
		if tile.OccupiedBy != nil || (tile.ReservedBy != nil && *tile.ReservedBy != playerID) {
			return nil, fmt.Errorf("selected hex is occupied or reserved")
		}
	}
	if tileType == "ocean" && g.GlobalParameters().Oceans() >= g.GlobalParameters().GetMaxOceans() {
		return nil, fmt.Errorf("ocean placement is no longer available")
	}
	// Handle clear differently - removes occupant/reservation from a tile (admin debug tool)
	if tileType == "clear" {
		// Check if tile is an ocean before clearing, so we can decrement the ocean count
		clearedTile, tileErr := g.Board().GetTile(*coords)
		if tileErr != nil {
			log.Warn("Failed to get tile for clear", slog.Any("error", tileErr))
			return nil, fmt.Errorf("failed to get tile: %w", tileErr)
		}
		wasOcean := clearedTile.OccupiedBy != nil && clearedTile.OccupiedBy.Type == shared.ResourceOceanTile

		if err := g.Board().ClearTileOccupant(ctx, *coords); err != nil {
			log.Warn("Failed to clear tile occupant", slog.Any("error", err))
			return nil, fmt.Errorf("failed to clear tile: %w", err)
		}

		if wasOcean {
			currentOceans := g.GlobalParameters().Oceans()
			if currentOceans > 0 {
				if err := g.GlobalParameters().SetOceans(ctx, currentOceans-1); err != nil {
					log.Warn("Failed to decrement ocean count", slog.Any("error", err))
				}
			}
		}

		log.Debug("Tile cleared",
			slog.String("position", selectedHex))

		result := &TilePlacementResult{
			TileType:   tileType,
			Source:     pendingTileSelection.Source,
			Hex:        selectedHex,
			OnComplete: pendingTileSelection.OnComplete,
		}

		if err := g.SetPendingTileSelection(ctx, playerID, nil); err != nil {
			return nil, fmt.Errorf("failed to clear pending tile selection: %w", err)
		}

		if err := g.ProcessNextTile(ctx, playerID); err != nil {
			return nil, fmt.Errorf("failed to process next tile: %w", err)
		}

		baseaction.AutoAdvanceTurnIfNeeded(g, playerID, log)

		log.Debug("Tile cleared",
			slog.String("position", selectedHex))
		return result, nil
	}

	// Handle tile replacement - destroys occupant and places a new tile in one step
	if strings.HasPrefix(tileType, "tile-replacement:") {
		replacementTileType := strings.TrimPrefix(tileType, "tile-replacement:")

		if err := g.Board().ClearTileOccupant(ctx, *coords); err != nil {
			log.Warn("Failed to destroy tile for replacement", slog.Any("error", err))
			return nil, fmt.Errorf("failed to destroy tile: %w", err)
		}

		occupant := a.createTileOccupant(g, replacementTileType, pendingTileSelection.SourceCardID)
		if err := g.Board().UpdateTileOccupancy(ctx, *coords, occupant, playerID); err != nil {
			log.Warn("Failed to place replacement tile", slog.Any("error", err))
			return nil, fmt.Errorf("failed to place replacement tile: %w", err)
		}

		result := &TilePlacementResult{
			TileType:   replacementTileType,
			Source:     pendingTileSelection.Source,
			Hex:        selectedHex,
			OnComplete: pendingTileSelection.OnComplete,
		}

		if err := g.SetPendingTileSelection(ctx, playerID, nil); err != nil {
			return nil, fmt.Errorf("failed to clear pending tile selection: %w", err)
		}

		if err := g.ProcessNextTile(ctx, playerID); err != nil {
			return nil, fmt.Errorf("failed to process next tile: %w", err)
		}

		if err := a.completionRegistry.Handle(ctx, g, playerID, result, result.OnComplete); err != nil {
			log.Warn("Failed to handle completion callback", slog.Any("error", err))
		}

		baseaction.AutoAdvanceTurnIfNeeded(g, playerID, log)

		log.Info("Tile replaced",
			slog.String("position", selectedHex),
			slog.String("replacement_type", replacementTileType))
		return result, nil
	}

	// Handle tile destruction - removes any tile (including oceans)
	if tileType == "tile-destruction" {
		destroyedTile, tileErr := g.Board().GetTile(*coords)
		if tileErr != nil {
			log.Warn("Failed to get tile for destruction", slog.Any("error", tileErr))
			return nil, fmt.Errorf("failed to get tile: %w", tileErr)
		}
		wasOcean := destroyedTile.OccupiedBy != nil && destroyedTile.OccupiedBy.Type == shared.ResourceOceanTile

		if err := g.Board().ClearTileOccupant(ctx, *coords); err != nil {
			log.Warn("Failed to destroy tile", slog.Any("error", err))
			return nil, fmt.Errorf("failed to destroy tile: %w", err)
		}

		if wasOcean {
			currentOceans := g.GlobalParameters().Oceans()
			if currentOceans > 0 {
				if err := g.GlobalParameters().SetOceans(ctx, currentOceans-1); err != nil {
					log.Warn("Failed to decrement ocean count", slog.Any("error", err))
				}
			}
		}

		nuclearOccupant := board.TileOccupant{
			Type: shared.ResourceNuclearZoneTile,
		}
		if err := g.Board().UpdateTileOccupancy(ctx, *coords, nuclearOccupant, playerID); err != nil {
			log.Warn("Failed to place nuclear zone tile", slog.Any("error", err))
			return nil, fmt.Errorf("failed to place nuclear zone tile: %w", err)
		}

		result := &TilePlacementResult{
			TileType:   tileType,
			Source:     pendingTileSelection.Source,
			Hex:        selectedHex,
			OnComplete: pendingTileSelection.OnComplete,
		}

		if err := g.SetPendingTileSelection(ctx, playerID, nil); err != nil {
			return nil, fmt.Errorf("failed to clear pending tile selection: %w", err)
		}

		if err := g.ProcessNextTile(ctx, playerID); err != nil {
			return nil, fmt.Errorf("failed to process next tile: %w", err)
		}

		if err := a.completionRegistry.Handle(ctx, g, playerID, result, result.OnComplete); err != nil {
			log.Warn("Failed to handle completion callback", slog.Any("error", err))
		}

		baseaction.AutoAdvanceTurnIfNeeded(g, playerID, log)

		log.Info("Tile destroyed",
			slog.String("position", selectedHex),
			slog.Bool("was_ocean", wasOcean))
		return result, nil
	}

	// Handle land claims differently - they reserve a tile instead of placing an occupant
	if tileType == "land-claim" {
		if err := g.Board().ReserveTile(ctx, *coords, playerID); err != nil {
			log.Warn("Failed to reserve tile", slog.Any("error", err))
			return nil, fmt.Errorf("failed to reserve tile: %w", err)
		}

		log.Debug("Tile reserved for land claim",
			slog.String("position", selectedHex))

		result := &TilePlacementResult{
			TileType:   tileType,
			Source:     pendingTileSelection.Source,
			Hex:        selectedHex,
			OnComplete: pendingTileSelection.OnComplete,
		}

		if err := g.SetPendingTileSelection(ctx, playerID, nil); err != nil {
			return nil, fmt.Errorf("failed to clear pending tile selection: %w", err)
		}

		if err := g.ProcessNextTile(ctx, playerID); err != nil {
			return nil, fmt.Errorf("failed to process next tile: %w", err)
		}

		if err := a.completionRegistry.Handle(ctx, g, playerID, result, result.OnComplete); err != nil {
			log.Warn("Failed to handle completion callback", slog.Any("error", err))
		}

		baseaction.AutoAdvanceTurnIfNeeded(g, playerID, log)

		log.Debug("Land claim reserved",
			slog.String("position", selectedHex))
		return result, nil
	}

	if preTile, err := g.Board().GetTile(*coords); err == nil {
		for _, bonus := range preTile.Bonuses {
			if err := baseaction.CanAffordResourceDeduction(p, bonus.Type, bonus.Amount); err != nil {
				log.Debug("Tile placement blocked by unaffordable bonus", slog.Any("error", err))
				return nil, fmt.Errorf("cannot place tile here: %w", err)
			}
		}
	}

	occupant := a.createTileOccupant(g, tileType, pendingTileSelection.SourceCardID)
	if err := g.Board().UpdateTileOccupancy(ctx, *coords, occupant, playerID); err != nil {
		log.Warn("Failed to place tile", slog.Any("error", err))
		return nil, fmt.Errorf("failed to place tile: %w", err)
	}

	log.Debug("Tile placed on board",
		slog.String("tile_type", tileType),
		slog.String("position", selectedHex))

	var queuedTilePlacements []string

	placedTile, err := g.Board().GetTile(*coords)
	if err != nil {
		log.Warn("Failed to get placed tile for bonus check", slog.Any("error", err))
	} else if len(placedTile.Bonuses) > 0 {
		log.Debug("Tile has bonuses", slog.Int("bonus_count", len(placedTile.Bonuses)))

		resourceBonuses := make(map[string]int)

		for _, bonus := range placedTile.Bonuses {
			switch bonus.Type {
			case shared.ResourceSteel, shared.ResourceTitanium, shared.ResourcePlant,
				shared.ResourceCredit, shared.ResourceEnergy, shared.ResourceHeat:
				p.Resources().Add(map[shared.ResourceType]int{
					bonus.Type: bonus.Amount,
				})
				log.Debug("Awarded resource bonus",
					slog.String("resource", string(bonus.Type)),
					slog.Int("amount", bonus.Amount))

				resourceBonuses[string(bonus.Type)] = bonus.Amount

			case shared.ResourceEnergyProduction, shared.ResourceHeatProduction,
				shared.ResourcePlantProduction, shared.ResourceSteelProduction,
				shared.ResourceTitaniumProduction, shared.ResourceCreditProduction:
				p.Resources().AddProduction(map[shared.ResourceType]int{
					bonus.Type: bonus.Amount,
				})
				log.Debug("Awarded production bonus",
					slog.String("production", string(bonus.Type)),
					slog.Int("amount", bonus.Amount))

				resourceBonuses[string(bonus.Type)] = bonus.Amount

			case shared.ResourceTemperature:
				stepsRaised, err := g.GlobalParameters().IncreaseTemperature(ctx, bonus.Amount, playerID)
				if err != nil {
					log.Warn("Failed to raise temperature for bonus", slog.Any("error", err))
					continue
				}
				if stepsRaised > 0 {
					p.Resources().UpdateTerraformRating(stepsRaised)
				}
				log.Debug("Awarded temperature bonus",
					slog.Int("requested_steps", bonus.Amount),
					slog.Int("actual_steps", stepsRaised))

				resourceBonuses[string(bonus.Type)] = stepsRaised

			case shared.ResourceOxygen:
				stepsRaised, err := g.GlobalParameters().IncreaseOxygen(ctx, bonus.Amount, playerID)
				if err != nil {
					log.Warn("Failed to raise oxygen for bonus", slog.Any("error", err))
					continue
				}
				if stepsRaised > 0 {
					p.Resources().UpdateTerraformRating(stepsRaised)
				}
				log.Debug("Awarded oxygen bonus",
					slog.Int("requested_steps", bonus.Amount),
					slog.Int("actual_steps", stepsRaised))

				resourceBonuses[string(bonus.Type)] = stepsRaised

			case shared.ResourceOceanPlacement:
				for i := 0; i < bonus.Amount; i++ {
					queuedTilePlacements = append(queuedTilePlacements, "ocean")
				}
				log.Debug("Queued ocean placement bonus",
					slog.Int("count", bonus.Amount))

				resourceBonuses[string(bonus.Type)] = bonus.Amount

			case shared.ResourceCardDraw:
				deck := g.Deck()
				cardIDs, err := deck.DrawProjectCards(ctx, bonus.Amount)
				if err != nil {
					log.Warn("Failed to draw cards for bonus", slog.Any("error", err))
					continue
				}

				for _, cardID := range cardIDs {
					p.Hand().AddCard(cardID)
				}
				p.Selection().AddCardReceipt("Tile bonus", pendingTileSelection.SourceCardID, cardIDs)

				g.AddTriggeredEffect(shared.TriggeredEffect{
					CardName:   "Tile Bonus",
					PlayerID:   playerID,
					SourceType: shared.SourceTypeCardPlay,
					CalculatedOutputs: []shared.CalculatedOutput{
						{ResourceType: string(shared.ResourceCardDraw), Amount: len(cardIDs)},
					},
				})

				log.Debug("Awarded card draw bonus",
					slog.Int("cards_drawn", len(cardIDs)),
					slog.Any("card_ids", cardIDs))

			default:
				log.Warn("Unhandled tile bonus type",
					slog.String("type", string(bonus.Type)),
					slog.Int("amount", bonus.Amount))
			}
		}

		if len(resourceBonuses) > 0 {
			events.Publish(g.EventBus(), events.PlacementBonusGainedEvent{
				GameID:       gameID,
				PlayerID:     playerID,
				Resources:    resourceBonuses,
				SourceCardID: pendingTileSelection.SourceCardID,
				Q:            coords.Q,
				R:            coords.R,
				S:            coords.S,
				Timestamp:    time.Now(),
			})
			log.Debug("Published PlacementBonusGainedEvent",
				slog.Any("resources", resourceBonuses))
		}

		// Clear bonuses from tile after claiming
		if err := g.Board().ClearTileBonuses(ctx, *coords); err != nil {
			log.Warn("Failed to clear tile bonuses", slog.Any("error", err))
		}
	}

	// Award 2 M€ per adjacent ocean tile (applies to all tile types, including oceans)
	neighbors := coords.GetNeighbors()
	adjacentOceanCount := 0
	for _, neighbor := range neighbors {
		neighborTile, err := g.Board().GetTile(neighbor)
		if err != nil {
			continue
		}
		if neighborTile.OccupiedBy != nil && neighborTile.OccupiedBy.Type == shared.ResourceOceanTile {
			adjacentOceanCount++
		}
	}
	if adjacentOceanCount > 0 {
		oceanBonus := adjacentOceanCount * 2
		p.Resources().Add(map[shared.ResourceType]int{
			shared.ResourceCredit: oceanBonus,
		})

		g.AddTriggeredEffect(shared.TriggeredEffect{
			CardName:   "Ocean Adjacency",
			PlayerID:   playerID,
			SourceType: shared.SourceTypeGameEvent,
			CalculatedOutputs: []shared.CalculatedOutput{
				{ResourceType: string(shared.ResourceCredit), Amount: oceanBonus},
			},
		})

		log.Debug("Awarded ocean adjacency bonus",
			slog.Int("adjacent_oceans", adjacentOceanCount),
			slog.Int("credits_awarded", oceanBonus))
	}

	result := &TilePlacementResult{
		TileType:   tileType,
		Source:     pendingTileSelection.Source,
		Hex:        selectedHex,
		OnComplete: pendingTileSelection.OnComplete,
	}

	switch tileType {
	case "city":
		log.Debug("City placed (no TR bonus)")

	case "greenery", "world-tree":
		actualSteps, err := g.GlobalParameters().IncreaseOxygen(ctx, 1, playerID)
		if err != nil {
			return nil, fmt.Errorf("failed to increase oxygen: %w", err)
		}
		result.OxygenSteps = actualSteps

		if actualSteps > 0 {
			p.Resources().UpdateTerraformRating(1)
			result.TRGained = 1
			log.Debug("Increased oxygen and TR for forest placement",
				slog.Int("oxygen_steps", actualSteps),
				slog.Int("tr_gained", 1))
		} else {
			log.Debug("Forest placed but oxygen already maxed")
		}

	case "ocean":
		success, err := g.GlobalParameters().PlaceOcean(ctx, playerID)
		if err != nil {
			return nil, fmt.Errorf("failed to place ocean: %w", err)
		}
		result.OceanPlaced = success

		if success {
			p.Resources().UpdateTerraformRating(1)
			result.TRGained = 1
			log.Debug("Placed ocean and increased TR",
				slog.Int("tr_gained", 1))
		} else {
			log.Debug("Ocean placed but ocean count already maxed")
		}

	case "volcano":
		log.Debug("Volcano placed (no TR bonus)")
	}

	if err := g.SetPendingTileSelection(ctx, playerID, nil); err != nil {
		return nil, fmt.Errorf("failed to clear pending tile selection: %w", err)
	}

	if len(queuedTilePlacements) > 0 {
		if err := g.AppendToPendingTileSelectionQueue(ctx, playerID, queuedTilePlacements, "tile-bonus", "", nil); err != nil {
			return nil, fmt.Errorf("failed to queue tile placement from bonus: %w", err)
		}
	}

	if err := g.ProcessNextTile(ctx, playerID); err != nil {
		return nil, fmt.Errorf("failed to process next tile: %w", err)
	}

	// Invoke completion callback to log the action
	if err := a.completionRegistry.Handle(ctx, g, playerID, result, result.OnComplete); err != nil {
		log.Warn("Failed to handle completion callback", slog.Any("error", err))
	}

	switch g.CurrentPhase() {
	case shared.GamePhaseStartingSelection:
		a.checkStartingSelectionCompletion(ctx, g, log)
	case shared.GamePhaseInitApplyCorp, shared.GamePhaseInitApplyPrelude:
		// During init phases, tile placement completes and the frontend sends the next confirm
	default:
		baseaction.AutoAdvanceTurnIfNeeded(g, playerID, log)
	}

	log.Info("Tile placed",
		slog.String("tile_type", tileType),
		slog.String("position", selectedHex))
	return result, nil
}

// checkStartingSelectionCompletion checks if all players finished starting selection and tile placements,
// then advances to action phase
func (a *SelectTileAction) checkStartingSelectionCompletion(ctx context.Context, g *game.Game, log *slog.Logger) {
	allPlayers := g.GetAllPlayers()
	for _, p := range allPlayers {
		if g.GetSelectCorporationPhase(p.ID()) != nil {
			return
		}
		if g.GetSelectPreludeCardsPhase(p.ID()) != nil {
			return
		}
		if g.GetSelectStartingCardsPhase(p.ID()) != nil {
			return
		}
		if g.GetPendingTileSelection(p.ID()) != nil {
			return
		}
		if g.GetPendingTileSelectionQueue(p.ID()) != nil {
			return
		}
	}

	log.Debug("All starting selections and tiles resolved, advancing to action phase")

	turn_management.AdvanceToActionPhase(ctx, g, allPlayers, log)
}

func parseHexPosition(hexStr string) (*shared.HexPosition, error) {
	parts := strings.Split(hexStr, ",")
	if len(parts) != 3 {
		return nil, fmt.Errorf("expected 3 coordinates, got %d", len(parts))
	}

	q, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid q coordinate: %w", err)
	}

	r, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid r coordinate: %w", err)
	}

	s, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid s coordinate: %w", err)
	}

	if q+r+s != 0 {
		return nil, fmt.Errorf("invalid cube coordinates: q+r+s must equal 0")
	}

	return &shared.HexPosition{Q: q, R: r, S: s}, nil
}

func mapTileTypeToResourceType(tileType string) shared.ResourceType {
	switch tileType {
	case "city":
		return shared.ResourceCityTile
	case "greenery":
		return shared.ResourceGreeneryTile
	case "ocean":
		return shared.ResourceOceanTile
	case "volcano":
		return shared.ResourceVolcanoTile
	case "world-tree":
		return shared.ResourceWorldTreeTile
	default:
		return shared.ResourceType(tileType + "-tile")
	}
}

func (a *SelectTileAction) createTileOccupant(g *game.Game, tileType, sourceCardID string) board.TileOccupant {
	occupant := board.TileOccupant{
		Type: mapTileTypeToResourceType(tileType),
		Tags: []string{},
	}
	if sourceCardID != "" {
		occupant.Tags = append(occupant.Tags, "source:"+sourceCardID)
	}
	registry := a.CardRegistryForGame(g)
	if registry == nil {
		return occupant
	}
	if sourceCardID == "" {
		if occupant.Type != shared.ResourceCityTile {
			if card := cards.UniqueStructureCard(registry, tileType); card != nil {
				occupant.DisplayName = card.Name
				occupant.Visual = &shared.TileVisual{Structure: card.Style.Tile.Clone()}
			}
		}
		return occupant
	}
	card, err := registry.GetByID(sourceCardID)
	if err != nil {
		return occupant
	}
	if occupant.Type == shared.ResourceCityTile || (card.Style != nil && card.Style.Tile != nil) {
		occupant.DisplayName = card.Name
	}
	if card.Style != nil && card.Style.Tile != nil {
		occupant.Visual = &shared.TileVisual{Structure: card.Style.Tile.Clone()}
	}
	return occupant
}
