package colony

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	baseaction "openmars/internal/action"
	"openmars/internal/events"
	"openmars/internal/game"
	"openmars/internal/game/cards"
	gamecolony "openmars/internal/game/colony"
	gameplayer "openmars/internal/game/player"
	"openmars/internal/game/shared"
)

const (
	BuildColonyCost = 17
)

// BuildColonyAction handles the business logic for building a colony on a colony tile
type BuildColonyAction struct {
	baseaction.BaseAction
	colonyRegistry gamecolony.ColonyRegistry
	cardRegistry   cards.CardRegistry
}

// NewBuildColonyAction creates a new build colony action
func NewBuildColonyAction(
	gameRepo game.GameRepository,
	colonyRegistry gamecolony.ColonyRegistry,
	cardRegistry cards.CardRegistry,
	stateRepo game.GameStateRepository,
	logger *slog.Logger,
) *BuildColonyAction {
	return &BuildColonyAction{
		BaseAction:     baseaction.NewBaseActionWithStateRepo(gameRepo, nil, stateRepo),
		colonyRegistry: colonyRegistry,
		cardRegistry:   cardRegistry,
	}
}

// Execute performs the build colony action
func (a *BuildColonyAction) Execute(ctx context.Context, gameID string, playerID string, colonyID string, payment shared.Payment) error {
	log := a.InitLogger(gameID, playerID).With(
		slog.String("action", "build_colony"),
		slog.String("colony_id", colonyID),
	)
	log.Debug("Building colony")

	g, err := baseaction.ValidateActiveGame(ctx, a.GameRepository(), gameID, log)
	if err != nil {
		return err
	}

	if err := baseaction.ValidateGamePhase(g, shared.GamePhaseAction, log); err != nil {
		return err
	}

	if err := baseaction.ValidateCurrentTurn(g, playerID, log); err != nil {
		return err
	}

	if err := baseaction.ValidateActionsRemaining(g, playerID, log); err != nil {
		return err
	}

	if err := baseaction.ValidateNoPendingSelections(g, playerID, log); err != nil {
		return err
	}

	if !g.HasColonies() {
		return fmt.Errorf("colonies expansion is not enabled")
	}

	player, err := a.GetPlayerFromGame(g, playerID, log)
	if err != nil {
		return err
	}

	tileState := g.Colonies().GetState(colonyID)
	if tileState != nil && tileState.AwaitingResource != "" {
		return fmt.Errorf("colony is not active")
	}
	if tileState == nil {
		return fmt.Errorf("colony tile not found: %s", colonyID)
	}

	definition, err := a.colonyRegistry.GetByID(colonyID)
	if err != nil {
		return fmt.Errorf("colony definition not found: %w", err)
	}

	maxColonies := len(definition.Colonies)
	if len(tileState.PlayerColonies) >= maxColonies {
		return fmt.Errorf("colony tile is full: %d/%d colonies", len(tileState.PlayerColonies), maxColonies)
	}

	if slices.Contains(tileState.PlayerColonies, playerID) {
		return fmt.Errorf("player already has a colony on this tile")
	}

	quote, err := cards.QuotePayment(player, g, cards.ForGame(a.cardRegistry, g), cards.PaymentContext{Costs: map[shared.ResourceType]int{shared.ResourceCredit: BuildColonyCost}, Action: "build-colony"})
	if err != nil {
		return err
	}
	paymentPlan, err := cards.ValidatePayment(quote, payment)
	if err != nil {
		return err
	}
	cards.ApplyPayment(player, paymentPlan)

	// Place colony
	slotIndex := len(tileState.PlayerColonies)
	tileState.PlayerColonies = append(tileState.PlayerColonies, playerID)

	if tileState.MarkerPosition < len(tileState.PlayerColonies) {
		tileState.MarkerPosition = len(tileState.PlayerColonies)
	}

	// Apply placement reward
	calculatedOutputs := []shared.CalculatedOutput{
		{ResourceType: "colony", Amount: 1},
	}
	if slotIndex < len(definition.Colonies) {
		slot := definition.Colonies[slotIndex]
		for _, reward := range slot.Reward {
			if reward.Amount > 0 {
				pending := applyOutput(ctx, g, player, reward.Type, reward.Amount, "Build Colony: "+definition.Name, cards.ForGame(a.cardRegistry, g), log)
				if pending != nil {
					setPendingColonyResource(player, pending, definition.Name, colonyID, "build", cards.ForGame(a.cardRegistry, g), log)
				}
				calculatedOutputs = append(calculatedOutputs, shared.CalculatedOutput{
					ResourceType: reward.Type,
					Amount:       reward.Amount,
				})
			}
		}
	}

	g.AddTriggeredEffect(shared.TriggeredEffect{
		CardName:          "Build Colony: " + definition.Name,
		PlayerID:          playerID,
		SourceType:        shared.SourceTypeColonyBuild,
		CalculatedOutputs: calculatedOutputs,
	})

	a.WriteStateLogFull(ctx, g, "Build Colony: "+definition.Name, shared.SourceTypeColonyBuild,
		playerID, fmt.Sprintf("Built colony on %s", definition.Name), nil, calculatedOutputs, nil)

	events.Publish(g.EventBus(), events.ColonyBuiltEvent{
		GameID:    g.ID(),
		PlayerID:  playerID,
		ColonyID:  colonyID,
		Timestamp: time.Now(),
	})

	a.ConsumePlayerAction(g, log)

	log.Info("Colony built",
		slog.String("colony_id", colonyID),
		slog.Int("slot_index", slotIndex))

	return nil
}

// PlaceColonyOnTile places a colony on the given tile for free (no cost deduction).
// It applies the placement reward, records the triggered effect, and publishes ColonyBuiltEvent.
// Used by card-triggered colony placements (e.g., Mining Colony, Poseidon first action).
func PlaceColonyOnTile(
	ctx context.Context,
	g *game.Game,
	player *gameplayer.Player,
	definition *gamecolony.ColonyDefinition,
	tileState *gamecolony.ColonyState,
	cardRegistry cards.CardRegistry,
	source string,
	log *slog.Logger,
) error {
	slotIndex := len(tileState.PlayerColonies)
	tileState.PlayerColonies = append(tileState.PlayerColonies, player.ID())

	if tileState.MarkerPosition < len(tileState.PlayerColonies) {
		tileState.MarkerPosition = len(tileState.PlayerColonies)
	}

	calculatedOutputs := []shared.CalculatedOutput{
		{ResourceType: "colony", Amount: 1},
	}
	if slotIndex < len(definition.Colonies) {
		slot := definition.Colonies[slotIndex]
		for _, reward := range slot.Reward {
			if reward.Amount > 0 {
				pending := applyOutput(ctx, g, player, reward.Type, reward.Amount, source+": "+definition.Name, cardRegistry, log)
				if pending != nil {
					setPendingColonyResource(player, pending, definition.Name, tileState.DefinitionID, "build", cardRegistry, log)
				}
				calculatedOutputs = append(calculatedOutputs, shared.CalculatedOutput{
					ResourceType: reward.Type,
					Amount:       reward.Amount,
				})
			}
		}
	}

	g.AddTriggeredEffect(shared.TriggeredEffect{
		CardName:          source + ": " + definition.Name,
		PlayerID:          player.ID(),
		SourceType:        shared.SourceTypeColonyBuild,
		CalculatedOutputs: calculatedOutputs,
	})

	events.Publish(g.EventBus(), events.ColonyBuiltEvent{
		GameID:    g.ID(),
		PlayerID:  player.ID(),
		ColonyID:  tileState.DefinitionID,
		Timestamp: time.Now(),
	})

	log.Debug("Colony placed (free)",
		slog.String("colony_id", tileState.DefinitionID),
		slog.String("player_id", player.ID()),
		slog.Int("slot_index", slotIndex))

	return nil
}
