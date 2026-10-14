package confirmation

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	baseaction "openmars/internal/action"
	colonyaction "openmars/internal/action/colony"
	"openmars/internal/events"
	"openmars/internal/game"
	"openmars/internal/game/cards"
	"openmars/internal/game/colony"
	"openmars/internal/game/shared"
)

// ConfirmFreeTradeAction handles confirming a free trade from a card effect
type ConfirmFreeTradeAction struct {
	baseaction.BaseAction
	colonyRegistry colony.ColonyRegistry
}

// NewConfirmFreeTradeAction creates a new confirm free trade action
func NewConfirmFreeTradeAction(
	gameRepo game.GameRepository,
	cardRegistry cards.CardRegistry,
	colonyRegistry colony.ColonyRegistry,
	stateRepo game.GameStateRepository,
) *ConfirmFreeTradeAction {
	return &ConfirmFreeTradeAction{
		BaseAction:     baseaction.NewBaseActionWithStateRepo(gameRepo, cardRegistry, stateRepo),
		colonyRegistry: colonyRegistry,
	}
}

// Execute performs the free trade with the selected colony
func (a *ConfirmFreeTradeAction) Execute(ctx context.Context, gameID string, playerID string, colonyID string, trackSteps int) error {
	log := a.InitLogger(gameID, playerID).With(
		slog.String("action", "confirm_free_trade"),
		slog.String("colony_id", colonyID),
	)
	log.Debug("Confirming free trade")

	g, err := baseaction.ValidateActiveGame(ctx, a.GameRepository(), gameID, log)
	if err != nil {
		return err
	}

	p, err := a.GetPlayerFromGame(g, playerID, log)
	if err != nil {
		return err
	}

	if err := baseaction.ValidateCurrentTurn(g, playerID, log); err != nil {
		return err
	}

	pendingSelection := p.Selection().GetPendingFreeTradeSelection()
	if pendingSelection == nil {
		return fmt.Errorf("no pending free trade selection")
	}

	if !slices.Contains(pendingSelection.AvailableColonyIDs, colonyID) {
		return fmt.Errorf("colony %s is not in the available list", colonyID)
	}

	tileState := g.Colonies().GetState(colonyID)
	if tileState == nil {
		return fmt.Errorf("colony tile not found: %s", colonyID)
	}

	if tileState.TradedThisGen {
		return fmt.Errorf("colony tile already traded this generation")
	}

	if g.Colonies().TradeFleet(playerID).Available() == 0 {
		return fmt.Errorf("trade fleet is not available")
	}

	definition, err := a.colonyRegistry.GetByID(colonyID)
	if err != nil {
		return fmt.Errorf("colony definition not found: %w", err)
	}

	option, err := baseaction.ValidateColonyTradeOption(p, tileState, definition, a.CardRegistryForGame(g), trackSteps)
	if err != nil {
		return err
	}
	if err := g.Colonies().UseTradeFleet(playerID); err != nil {
		return err
	}
	tileState.MarkerPosition = option.MarkerPosition

	// Apply trade income based on marker position (no payment needed - it's free!)
	var pendingResources []*colonyaction.PendingResource
	if tileState.MarkerPosition >= 0 && tileState.MarkerPosition < len(definition.Steps) {
		step := definition.Steps[tileState.MarkerPosition]
		for _, output := range step.Outputs {
			if output.Amount > 0 {
				pending := colonyaction.ApplyTradeOutput(ctx, g, p, output.Type, output.Amount, "Free Trade: "+definition.Name, a.CardRegistryForGame(g), log)
				if pending != nil {
					pendingResources = append(pendingResources, pending)
				}
			}
		}
	}

	// Handle pending card-targeted resources (floaters, microbes, animals)
	if len(pendingResources) > 0 {
		colonyaction.SetPendingColonyResourceFromTrade(p, pendingResources, definition.Name, colonyID, "trade", a.CardRegistryForGame(g), log)
	}

	// Apply colony bonus to all players with colonies
	for _, colonyOwnerID := range tileState.PlayerColonies {
		colonyOwner, ownerErr := g.GetPlayer(colonyOwnerID)
		if ownerErr != nil {
			continue
		}
		var ownerPendings []*colonyaction.PendingResource
		for _, bonus := range definition.ColonyBonus {
			if bonus.Amount > 0 {
				pending := colonyaction.ApplyTradeOutput(ctx, g, colonyOwner, bonus.Type, bonus.Amount, "Colony bonus: "+definition.Name, a.CardRegistryForGame(g), log)
				if pending != nil {
					ownerPendings = append(ownerPendings, pending)
				}
			}
		}
		if len(ownerPendings) > 0 {
			reason := "colony-tax"
			colonyaction.SetPendingColonyResourceFromTrade(colonyOwner, ownerPendings, definition.Name, colonyID, reason, a.CardRegistryForGame(g), log)
		}
	}

	// Reset marker and mark as traded
	tileState.MarkerPosition = len(tileState.PlayerColonies)
	tileState.TradedThisGen = true
	tileState.TraderID = playerID

	events.Publish(g.EventBus(), events.ColonyTradedEvent{
		GameID:    g.ID(),
		PlayerID:  playerID,
		ColonyID:  colonyID,
		Timestamp: time.Now(),
	})

	// Clear the pending selection
	p.Selection().SetPendingFreeTradeSelection(nil)

	a.WriteStateLogFull(ctx, g, "Free Trade: "+definition.Name, shared.SourceTypeColonyTrade,
		playerID, fmt.Sprintf("Free traded with %s", definition.Name), nil, nil, nil)

	log.Info("Free trade confirmed",
		slog.String("colony_id", colonyID))

	return nil
}
