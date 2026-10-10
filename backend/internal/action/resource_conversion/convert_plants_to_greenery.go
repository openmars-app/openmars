package resource_conversion

import (
	"context"
	"fmt"
	"log/slog"
	baseaction "openmars/internal/action"

	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/shared"
)

const (
	// BasePlantsForGreenery is the base cost in plants to convert to greenery (before card discounts)
	BasePlantsForGreenery = 8
)

// ConvertPlantsToGreeneryAction handles the business logic for converting plants to greenery tile
// Uses RequirementModifierCalculator to apply card discounts (e.g., Ecoline: 7 plants instead of 8)
type ConvertPlantsToGreeneryAction struct {
	baseaction.BaseAction
	cardRegistry gamecards.CardRegistry
}

// NewConvertPlantsToGreeneryAction creates a new convert plants to greenery action
func NewConvertPlantsToGreeneryAction(
	gameRepo game.GameRepository,
	cardRegistry gamecards.CardRegistry,
	stateRepo game.GameStateRepository,
	logger *slog.Logger,
) *ConvertPlantsToGreeneryAction {
	return &ConvertPlantsToGreeneryAction{
		BaseAction:   baseaction.NewBaseActionWithStateRepo(gameRepo, nil, stateRepo),
		cardRegistry: cardRegistry,
	}
}

// Execute performs the convert plants to greenery action
func (a *ConvertPlantsToGreeneryAction) Execute(ctx context.Context, gameID string, playerID string, payment shared.Payment) error {
	log := a.InitLogger(gameID, playerID).With(slog.String("action", "convert_plants_to_greenery"))
	log.Debug("Converting plants to greenery")

	g, err := baseaction.ValidateActiveGame(ctx, a.GameRepository(), gameID, log)
	if err != nil {
		return err
	}

	phase := g.CurrentPhase()
	if phase != shared.GamePhaseAction && phase != shared.GamePhaseFinalPhase {
		log.Error("Game not in valid phase for greenery conversion",
			slog.String("actual", string(phase)))
		return fmt.Errorf("game not in action or final phase")
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

	player, err := a.GetPlayerFromGame(g, playerID, log)
	if err != nil {
		return err
	}

	calculator := gamecards.NewRequirementModifierCalculator(gamecards.ForGame(a.cardRegistry, g))
	discounts := calculator.CalculateStandardProjectDiscounts(player, shared.StandardProjectConvertPlantsToGreenery)
	plantDiscount := discounts[shared.ResourcePlant]
	requiredPlants := BasePlantsForGreenery - plantDiscount
	if requiredPlants < 1 {
		requiredPlants = 1
	}
	log.Debug("Calculated plants cost",
		slog.Int("base_cost", BasePlantsForGreenery),
		slog.Int("discount", plantDiscount),
		slog.Int("final_cost", requiredPlants))

	quote, err := gamecards.QuotePayment(player, g, gamecards.ForGame(a.cardRegistry, g), gamecards.PaymentContext{Costs: map[shared.ResourceType]int{shared.ResourcePlant: requiredPlants}, Action: "standard-project", StandardProject: shared.StandardProjectConvertPlantsToGreenery})
	if err != nil {
		return err
	}
	plan, err := gamecards.ValidatePayment(quote, payment)
	if err != nil {
		return err
	}
	gamecards.ApplyPayment(player, plan)

	queue := &shared.PendingTileSelectionQueue{
		Items:  []string{"greenery"},
		Source: "convert-plants-to-greenery",
		OnComplete: &shared.TileCompletionCallback{
			Type: "convert-plants-to-greenery",
		},
		TileRestrictions: &shared.TileRestrictions{
			AdjacentToOwned: true,
		},
	}
	if err := g.SetPendingTileSelectionQueue(ctx, playerID, queue); err != nil {
		return fmt.Errorf("failed to queue tile placement: %w", err)
	}

	log.Debug("Created tile queue for greenery placement (auto-processed by SetPendingTileSelectionQueue)")

	a.ConsumePlayerAction(g, log)

	log.Info("Plants converted, greenery queued",
		slog.Int("plants_spent", requiredPlants))
	return nil
}
