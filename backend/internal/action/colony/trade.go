package colony

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	baseaction "openmars/internal/action"
	"openmars/internal/events"
	"openmars/internal/game"
	"openmars/internal/game/cards"
	"openmars/internal/game/colony"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// TradePaymentType represents the resource used to pay for a colony trade
type TradePaymentType string

const (
	TradePaymentCredits  TradePaymentType = "credits"
	TradePaymentEnergy   TradePaymentType = "energy"
	TradePaymentTitanium TradePaymentType = "titanium"
)

// tradePaymentResource maps a payment type to the resource it spends.
func tradePaymentResource(paymentType TradePaymentType) (shared.ResourceType, bool) {
	switch paymentType {
	case TradePaymentCredits:
		return shared.ResourceCredit, true
	case TradePaymentEnergy:
		return shared.ResourceEnergy, true
	case TradePaymentTitanium:
		return shared.ResourceTitanium, true
	default:
		return "", false
	}
}

// TradeAction handles the business logic for trading with a colony tile
type TradeAction struct {
	baseaction.BaseAction
	colonyRegistry colony.ColonyRegistry
	cardRegistry   cards.CardRegistry
}

// NewTradeAction creates a new trade action
func NewTradeAction(
	gameRepo game.GameRepository,
	colonyRegistry colony.ColonyRegistry,
	cardRegistry cards.CardRegistry,
	stateRepo game.GameStateRepository,
	logger *slog.Logger,
) *TradeAction {
	return &TradeAction{
		BaseAction:     baseaction.NewBaseActionWithStateRepo(gameRepo, nil, stateRepo),
		colonyRegistry: colonyRegistry,
		cardRegistry:   cardRegistry,
	}
}

// Execute performs the trade action
func (a *TradeAction) Execute(ctx context.Context, gameID string, playerID string, colonyID string, paymentType TradePaymentType, trackSteps int, payment shared.Payment) error {
	log := a.InitLogger(gameID, playerID).With(
		slog.String("action", "colony_trade"),
		slog.String("colony_id", colonyID),
	)
	log.Debug("Trading with colony")

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

	traderPlayer, err := a.GetPlayerFromGame(g, playerID, log)
	if err != nil {
		return err
	}

	if g.Colonies().TradeFleet(playerID).Available() == 0 {
		return fmt.Errorf("trade fleet is not available")
	}

	tileState := g.Colonies().GetState(colonyID)
	if tileState != nil && tileState.AwaitingResource != "" {
		return fmt.Errorf("colony is not active")
	}
	if tileState == nil {
		return fmt.Errorf("colony tile not found: %s", colonyID)
	}

	if tileState.TradedThisGen {
		return fmt.Errorf("colony tile already traded this generation")
	}

	// Derive the effective cost for the chosen payment type from the single
	// source of truth (applies discounts like Rim Freighters).
	paymentResource, ok := tradePaymentResource(paymentType)
	if !ok {
		return fmt.Errorf("invalid trade payment type: %s", paymentType)
	}
	effectiveCosts, _ := baseaction.CalculateEffectiveTradeCosts(traderPlayer, cards.ForGame(a.cardRegistry, g))
	effectiveCost := effectiveCosts[string(paymentResource)]

	quote, err := cards.QuotePayment(traderPlayer, g, cards.ForGame(a.cardRegistry, g), cards.PaymentContext{Costs: map[shared.ResourceType]int{paymentResource: effectiveCost}, Action: "colony-trade"})
	if err != nil {
		return err
	}
	paymentPlan, err := cards.ValidatePayment(quote, payment)
	if err != nil {
		return err
	}

	definition, err := a.colonyRegistry.GetByID(colonyID)
	if err != nil {
		return fmt.Errorf("colony definition not found: %w", err)
	}

	option, err := baseaction.ValidateColonyTradeOption(traderPlayer, tileState, definition, cards.ForGame(a.cardRegistry, g), trackSteps)
	if err != nil {
		return err
	}
	if err := g.Colonies().UseTradeFleet(playerID); err != nil {
		return err
	}

	cards.ApplyPayment(traderPlayer, paymentPlan)

	tileState.MarkerPosition = option.MarkerPosition

	// Collect pending card-targeted resources per player, so same-type resources
	// from trade income + colony bonus are combined into a single selection.
	pendingByPlayer := map[string][]*PendingResource{}
	outputsByPlayer := map[string][]shared.CalculatedOutput{}

	// Give trade income based on marker position
	if tileState.MarkerPosition >= 0 && tileState.MarkerPosition < len(definition.Steps) {
		step := definition.Steps[tileState.MarkerPosition]
		for _, output := range step.Outputs {
			if output.Amount > 0 {
				pending := applyOutput(ctx, g, traderPlayer, output.Type, output.Amount, "Trade: "+definition.Name, cards.ForGame(a.cardRegistry, g), log)
				if pending != nil {
					pendingByPlayer[playerID] = append(pendingByPlayer[playerID], pending)
				}
				outputsByPlayer[playerID] = append(outputsByPlayer[playerID], shared.CalculatedOutput{
					ResourceType: output.Type,
					Amount:       output.Amount,
				})
			}
		}
	}

	// Give colony bonus to all players with colonies on this tile
	for _, colonyOwnerID := range tileState.PlayerColonies {
		colonyOwner, ownerErr := g.GetPlayer(colonyOwnerID)
		if ownerErr != nil {
			continue
		}
		for _, bonus := range definition.ColonyBonus {
			if bonus.Amount > 0 {
				pending := applyOutput(ctx, g, colonyOwner, bonus.Type, bonus.Amount, "Colony bonus: "+definition.Name, cards.ForGame(a.cardRegistry, g), log)
				if pending != nil {
					pendingByPlayer[colonyOwnerID] = append(pendingByPlayer[colonyOwnerID], pending)
				}
				outputsByPlayer[colonyOwnerID] = append(outputsByPlayer[colonyOwnerID], shared.CalculatedOutput{
					ResourceType: bonus.Type,
					Amount:       bonus.Amount,
				})
			}
		}
	}

	// Resolve pending resources — combine same-type for each player
	for pid, pendings := range pendingByPlayer {
		p, pErr := g.GetPlayer(pid)
		if pErr != nil {
			continue
		}
		reason := "colony-tax"
		if pid == playerID {
			reason = "trade"
		}
		for _, combined := range combinePendingResources(pendings) {
			setPendingColonyResource(p, combined, definition.Name, colonyID, reason, cards.ForGame(a.cardRegistry, g), log)
		}
	}

	// Add triggered effects for trader and colony bonus recipients
	for pid, outputs := range outputsByPlayer {
		g.AddTriggeredEffect(shared.TriggeredEffect{
			CardName:          "Trade: " + definition.Name,
			PlayerID:          pid,
			SourceType:        shared.SourceTypeColonyTrade,
			CalculatedOutputs: combineCalculatedOutputs(outputs),
		})
	}

	a.WriteStateLogFull(ctx, g, "Trade: "+definition.Name, shared.SourceTypeColonyTrade,
		playerID, fmt.Sprintf("Traded with %s", definition.Name), nil, combineCalculatedOutputs(outputsByPlayer[playerID]), nil)

	// Reset marker to position after last colony
	tileState.MarkerPosition = len(tileState.PlayerColonies)
	tileState.TradedThisGen = true
	tileState.TraderID = playerID

	events.Publish(g.EventBus(), events.ColonyTradedEvent{
		GameID:    g.ID(),
		PlayerID:  playerID,
		ColonyID:  colonyID,
		Timestamp: time.Now(),
	})

	a.ConsumePlayerAction(g, log)

	log.Info("Colony traded",
		slog.String("colony_id", colonyID),
		slog.Int("marker_position", tileState.MarkerPosition))

	return nil
}

// setPendingColonyResource sets a pending colony resource selection on a player
// if they have at least one card that can store the resource type.
func setPendingColonyResource(p *player.Player, pending *PendingResource, colonyName string, colonyID string, reason string, cardRegistry cards.CardRegistry, log *slog.Logger) {
	if !hasEligibleStorageCard(p, pending.ResourceType, cardRegistry) {
		log.Debug("No eligible storage card, resources lost",
			slog.String("player_id", p.ID()),
			slog.String("resource_type", pending.ResourceType),
			slog.Int("amount", pending.Amount))
		return
	}

	p.Selection().AppendPendingColonyResource(shared.PendingColonyResourceSelection{
		ResourceType: pending.ResourceType,
		Amount:       pending.Amount,
		Source:       colonyName,
		ColonyID:     colonyID,
		Reason:       reason,
	})

	log.Debug("Set pending colony resource selection",
		slog.String("player_id", p.ID()),
		slog.String("resource_type", pending.ResourceType),
		slog.Int("amount", pending.Amount))
}

// hasEligibleStorageCard checks if a player has any played card that can store the given resource type.
func hasEligibleStorageCard(p *player.Player, resourceType string, cardRegistry cards.CardRegistry) bool {
	if cardRegistry == nil {
		return false
	}
	rt := shared.ResourceType(resourceType)

	// Check played cards
	for _, cardID := range p.PlayedCards().Cards() {
		card, err := cardRegistry.GetByID(cardID)
		if err != nil {
			continue
		}
		if card.ResourceStorage != nil && card.ResourceStorage.Type == rt {
			return true
		}
	}

	// Check corporation
	if corpID := p.CorporationID(); corpID != "" {
		corp, err := cardRegistry.GetByID(corpID)
		if err == nil {
			if corp.ResourceStorage != nil && corp.ResourceStorage.Type == rt {
				return true
			}
		}
	}

	return false
}

// SetPendingColonyResourceFromTrade handles pending card-targeted resources from trade/colony operations.
func SetPendingColonyResourceFromTrade(p *player.Player, pendings []*PendingResource, colonyName string, colonyID string, reason string, cardRegistry cards.CardRegistry, log *slog.Logger) {
	for _, combined := range combinePendingResources(pendings) {
		setPendingColonyResource(p, combined, colonyName, colonyID, reason, cardRegistry, log)
	}
}
