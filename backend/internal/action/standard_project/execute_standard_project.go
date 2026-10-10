package standard_project

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	baseaction "openmars/internal/action"

	"openmars/internal/events"
	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/internal/game/standardproject"
)

// ExecuteStandardProjectAction handles all standard projects via a single unified action
type ExecuteStandardProjectAction struct {
	baseaction.BaseAction
	standardProjectRegistry standardproject.StandardProjectRegistry
}

// NewExecuteStandardProjectAction creates a new unified standard project action
func NewExecuteStandardProjectAction(
	gameRepo game.GameRepository,
	cardRegistry gamecards.CardRegistry,
	standardProjectRegistry standardproject.StandardProjectRegistry,
	stateRepo game.GameStateRepository,
	logger *slog.Logger,
) *ExecuteStandardProjectAction {
	return &ExecuteStandardProjectAction{
		BaseAction:              baseaction.NewBaseActionWithStateRepo(gameRepo, cardRegistry, stateRepo),
		standardProjectRegistry: standardProjectRegistry,
	}
}

// Execute performs the standard project action
func (a *ExecuteStandardProjectAction) Execute(
	ctx context.Context,
	gameID string,
	playerID string,
	projectID string,
	payment shared.Payment,
) error {
	log := a.InitLogger(gameID, playerID).With(slog.String("project_id", projectID))
	log.Debug("Executing standard project")

	definition, err := a.standardProjectRegistry.GetByID(projectID)
	if err != nil {
		log.Warn("Unknown standard project", slog.String("project_id", projectID))
		return fmt.Errorf("unknown standard project: %s", projectID)
	}

	g, err := baseaction.ValidateActiveGame(ctx, a.GameRepository(), gameID, log)
	if err != nil {
		return err
	}

	if !g.Settings().AllowsContent(definition.GameModes, definition.RequiresSystems, definition.RequiresFeatures) {
		return fmt.Errorf("standard project is not available in this game mode")
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

	player, err := a.GetPlayerFromGame(g, playerID, log)
	if err != nil {
		return err
	}

	effectiveCost := definition.CreditCost()
	if a.CardRegistryForGame(g) != nil && effectiveCost > 0 {
		calculator := gamecards.NewRequirementModifierCalculator(a.CardRegistryForGame(g))
		discounts := calculator.CalculateStandardProjectDiscounts(player, shared.StandardProject(projectID))
		creditDiscount := discounts[shared.ResourceCredit]
		effectiveCost = definition.CreditCost() - creditDiscount
		if effectiveCost < 0 {
			effectiveCost = 0
		}
		if creditDiscount > 0 {
			log.Debug("Applied standard project discount",
				slog.Int("base_cost", definition.CreditCost()),
				slog.Int("discount", creditDiscount),
				slog.Int("effective_cost", effectiveCost))
		}
	}

	quote, err := gamecards.QuotePayment(player, g, a.CardRegistryForGame(g), gamecards.PaymentContext{Costs: map[shared.ResourceType]int{shared.ResourceCredit: effectiveCost}, Action: "standard-project", StandardProject: shared.StandardProject(projectID)})
	if err != nil {
		return err
	}
	paymentPlan, err := gamecards.ValidatePayment(quote, payment)
	if err != nil {
		return err
	}
	gamecards.ApplyPayment(player, paymentPlan)

	events.Publish(g.EventBus(), events.StandardProjectPlayedEvent{
		GameID:      g.ID(),
		PlayerID:    playerID,
		ProjectType: projectID,
		ProjectCost: definition.CreditCost(),
		Timestamp:   time.Now(),
	})

	// Check for sell-patents pattern: card-discard input with variableAmount
	if hasSellPatentsBehavior(definition.Behaviors) {
		return a.executeSellPatents(g, player, log)
	}

	// Apply behavior outputs via BehaviorApplier
	applier := gamecards.NewBehaviorApplier(player, g, definition.Name, slog.Default()).
		WithSourceType(shared.SourceTypeStandardProject)
	if a.CardRegistryForGame(g) != nil {
		applier = applier.WithCardRegistry(a.CardRegistryForGame(g))
	}

	var allCalculatedOutputs []shared.CalculatedOutput
	for _, behavior := range definition.Behaviors {
		calculatedOutputs, err := applier.ApplyOutputsAndGetCalculated(ctx, behavior.Outputs)
		if err != nil {
			return fmt.Errorf("failed to apply standard project outputs: %w", err)
		}
		allCalculatedOutputs = append(allCalculatedOutputs, calculatedOutputs...)
	}

	a.ConsumePlayerAction(g, log)

	displayData := &game.LogDisplayData{}
	source := "Standard Project: " + definition.Name
	a.WriteStateLogFull(ctx, g, source, shared.SourceTypeStandardProject, playerID, definition.Name, nil, allCalculatedOutputs, displayData)

	log.Info("Standard project executed", slog.String("project", definition.Name))
	return nil
}

// executeSellPatents handles the sell patents special case by creating a pending card selection
func (a *ExecuteStandardProjectAction) executeSellPatents(
	g *game.Game,
	p *player.Player,
	log *slog.Logger,
) error {
	_ = g
	playerCards := p.Hand().Cards()
	if len(playerCards) == 0 {
		log.Warn("No cards to sell")
		return fmt.Errorf("no cards available to sell")
	}

	cardCosts := make(map[string]int)
	cardRewards := make(map[string]int)
	for _, cardID := range playerCards {
		cardCosts[cardID] = 0
		cardRewards[cardID] = 1
	}

	p.Selection().SetPendingCardSelection(&shared.PendingCardSelection{
		Source:         "sell-patents",
		AvailableCards: playerCards,
		CardCosts:      cardCosts,
		CardRewards:    cardRewards,
		MinCards:       0,
		MaxCards:       len(playerCards),
	})

	log.Debug("Created pending card selection for sell patents",
		slog.Int("available_cards", len(playerCards)))

	log.Info("Sell patents initiated")
	return nil
}

// hasSellPatentsBehavior checks if any behavior has a card-discard input with variableAmount
func hasSellPatentsBehavior(behaviors []shared.CardBehavior) bool {
	for _, b := range behaviors {
		for _, input := range b.Inputs {
			if input.GetResourceType() == shared.ResourceCardDiscard {
				if co, ok := input.(*shared.CardOperationCondition); ok && co.VariableAmount {
					return true
				}
			}
		}
	}
	return false
}
