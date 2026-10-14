package confirmation

import (
	"context"
	"fmt"
	"log/slog"
	baseaction "openmars/internal/action"
	gameaction "openmars/internal/action/game"
	"openmars/internal/action/resource_conversion"

	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	playerPkg "openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// ConfirmProductionCardsAction handles the business logic for confirming production card selection
type ConfirmProductionCardsAction struct {
	baseaction.BaseAction
	finalScoringAction *gameaction.FinalScoringAction
}

// NewConfirmProductionCardsAction creates a new confirm production cards action
func NewConfirmProductionCardsAction(
	gameRepo game.GameRepository,
	cardRegistry gamecards.CardRegistry,
	finalScoringAction *gameaction.FinalScoringAction,
	logger *slog.Logger,
) *ConfirmProductionCardsAction {
	return &ConfirmProductionCardsAction{
		BaseAction:         baseaction.NewBaseAction(gameRepo, cardRegistry),
		finalScoringAction: finalScoringAction,
	}
}

// Execute performs the confirm production cards action.
// When randomBuy is true and the selection is empty, a single fresh card is
// drawn from the deck and bought (subject to the AllowRandomBuy setting).
func (a *ConfirmProductionCardsAction) Execute(ctx context.Context, gameID string, playerID string, selectedCardIDs []string, randomBuy bool, payment shared.Payment) error {
	log := a.InitLogger(gameID, playerID).With(
		slog.String("action", "confirm_production_cards"),
		slog.Any("selected_card_ids", selectedCardIDs),
		slog.Bool("random_buy", randomBuy),
	)
	log.Debug("Player confirming production card selection")

	g, err := a.GameRepository().Get(ctx, gameID)
	if err != nil {
		log.Error("Failed to get game", slog.Any("error", err))
		return fmt.Errorf("game not found: %s", gameID)
	}

	if g.CurrentPhase() != shared.GamePhaseProductionAndCardDraw {
		log.Warn("Game is not in production phase",
			slog.String("current_phase", string(g.CurrentPhase())),
			slog.String("expected_phase", string(shared.GamePhaseProductionAndCardDraw)))
		return fmt.Errorf("game is not in production phase")
	}

	player, err := g.GetPlayer(playerID)
	if err != nil {
		log.Error("Player not found in game", slog.Any("error", err))
		return fmt.Errorf("player not found: %s", playerID)
	}

	productionPhase := g.GetProductionPhase(playerID)
	if productionPhase == nil {
		log.Error("Player not in production phase")
		return fmt.Errorf("player not in production phase")
	}

	if productionPhase.SelectionComplete {
		log.Error("Production selection already complete")
		return fmt.Errorf("production selection already complete")
	}

	if randomBuy {
		if !g.Settings().AllowRandomBuy {
			return fmt.Errorf("random buy is not enabled for this game")
		}
		if len(selectedCardIDs) > 0 {
			return fmt.Errorf("random buy requires an empty selection")
		}

	} else {
		availableSet := make(map[string]bool)
		for _, id := range productionPhase.AvailableCards {
			availableSet[id] = true
		}
		for _, cardID := range selectedCardIDs {
			if !availableSet[cardID] {
				log.Error("Selected card not available", slog.String("card_id", cardID))
				return fmt.Errorf("card %s not available for selection", cardID)
			}
			delete(availableSet, cardID)
		}
	}

	calc := gamecards.NewRequirementModifierCalculator(a.CardRegistryForGame(g))
	cardBuyDiscounts := calc.CalculateActionDiscounts(player, shared.ActionCardBuying)
	costPerCard := max(3-cardBuyDiscounts[shared.ResourceCredit], 0)
	count := len(selectedCardIDs)
	if randomBuy {
		count = 1
	}
	cost := count * costPerCard

	quote, err := gamecards.QuotePayment(player, g, a.CardRegistryForGame(g), gamecards.PaymentContext{Costs: map[shared.ResourceType]int{shared.ResourceCredit: cost}, Action: "card-buying"})
	if err != nil {
		return err
	}
	paymentPlan, err := gamecards.ValidatePayment(quote, payment)
	if err != nil {
		return err
	}
	if randomBuy {
		drawn, err := g.Deck().DrawProjectCards(ctx, 1)
		if err != nil || len(drawn) == 0 {
			log.Error("Failed to draw random card from deck", slog.Any("error", err))
			return fmt.Errorf("failed to draw random card: %w", err)
		}
		selectedCardIDs = drawn
		log.Debug("Random buy drew card from deck", slog.String("card_id", drawn[0]))
	}
	gamecards.ApplyPayment(player, paymentPlan)

	log.Debug("Adding cards to player hand",
		slog.Any("card_ids", selectedCardIDs),
		slog.Int("count", len(selectedCardIDs)))

	if randomBuy {
		player.Hand().ReceiveCards(selectedCardIDs, "Random research purchase", "")
	} else {
		for _, cardID := range selectedCardIDs {
			player.Hand().AddCard(cardID)
		}
	}

	log.Debug("Cards added to hand",
		slog.Any("card_ids_added", selectedCardIDs),
		slog.Int("card_count", len(selectedCardIDs)))

	productionPhase.SelectionComplete = true
	if err := g.SetProductionPhase(ctx, playerID, productionPhase); err != nil {
		log.Error("Failed to update production phase", slog.Any("error", err))
		return fmt.Errorf("failed to update production phase: %w", err)
	}

	log.Debug("Production selection marked complete")

	allPlayers := g.GetAllPlayers()
	allComplete := true
	for _, p := range allPlayers {
		if p.HasExited() {
			continue
		}
		pPhase := g.GetProductionPhase(p.ID())
		if pPhase == nil || !pPhase.SelectionComplete {
			allComplete = false
			break
		}
	}

	if allComplete {
		for _, p := range allPlayers {
			if err := g.SetProductionPhase(ctx, p.ID(), nil); err != nil {
				log.Warn("Failed to clear production phase",
					slog.String("player_id", p.ID()),
					slog.Any("error", err))
			}
		}

		if g.GlobalParameters().IsMaxed() {
			log.Debug("All global parameters maxed after final production - entering final phase")

			if err := g.UpdatePhase(ctx, shared.GamePhaseFinalPhase); err != nil {
				log.Error("Failed to transition to final phase", slog.Any("error", err))
				return fmt.Errorf("failed to transition to final phase: %w", err)
			}

			autoPassPlayersForFinalPhase(allPlayers, a.CardRegistryForGame(g), log)

			turnOrder := g.TurnOrder()
			firstPlayerID := ""
			activeCount := 0
			for _, id := range turnOrder {
				p, _ := g.GetPlayer(id)
				if p != nil && !p.HasPassed() && !p.HasExited() {
					activeCount++
					if firstPlayerID == "" {
						firstPlayerID = id
					}
				}
			}

			if activeCount == 0 {
				log.Debug("All players auto-passed in final phase - triggering final scoring")
				if err := a.finalScoringAction.Execute(ctx, gameID); err != nil {
					log.Error("Failed to execute final scoring", slog.Any("error", err))
					return fmt.Errorf("failed to execute final scoring: %w", err)
				}
				log.Info("Game ended, final scores calculated")
				return nil
			}

			availableActions := 2
			if activeCount == 1 {
				availableActions = -1
			}
			if err := g.SetCurrentTurn(ctx, firstPlayerID, availableActions); err != nil {
				log.Error("Failed to set current turn for final phase", slog.Any("error", err))
				return fmt.Errorf("failed to set current turn: %w", err)
			}

			log.Info("Final phase started")
			return nil
		}

		log.Debug("All players completed production selection, advancing to action phase")

		if err := g.UpdatePhase(ctx, shared.GamePhaseAction); err != nil {
			log.Error("Failed to transition game phase", slog.Any("error", err))
			return fmt.Errorf("failed to transition game phase: %w", err)
		}

		baseaction.BeginModeGeneration(ctx, g)

		turnOrder := g.TurnOrder()
		activeCount := 0
		firstPlayerID := ""
		for _, id := range turnOrder {
			p, _ := g.GetPlayer(id)
			if p != nil && !p.HasExited() {
				activeCount++
				if firstPlayerID == "" {
					firstPlayerID = id
				}
			}
		}
		if firstPlayerID != "" {
			availableActions := 2
			if activeCount == 1 {
				availableActions = -1
			}
			if err := g.SetCurrentTurn(ctx, firstPlayerID, availableActions); err != nil {
				log.Error("Failed to set current turn", slog.Any("error", err))
				return fmt.Errorf("failed to set current turn: %w", err)
			}
			log.Debug("Set first player turn with actions",
				slog.String("player_id", firstPlayerID),
				slog.Int("available_actions", availableActions))
		}

		for _, p := range allPlayers {
			if !p.HasExited() {
				p.Actions().ResetGenerationCounts()
			}
		}
	}

	log.Info("Production cards selected")
	return nil
}

// autoPassPlayersForFinalPhase marks players as passed if they cannot perform
// any final phase actions. Currently checks if a player can afford at least
// one greenery conversion (plants → greenery).
func autoPassPlayersForFinalPhase(
	players []*playerPkg.Player,
	cardRegistry gamecards.CardRegistry,
	log *slog.Logger,
) {
	calculator := gamecards.NewRequirementModifierCalculator(cardRegistry)

	for _, p := range players {
		if p.HasExited() {
			continue
		}
		if !canAffordGreenery(p, calculator) {
			p.SetPassed(true)
			log.Debug("Auto-passed player in final phase (no available actions)",
				slog.String("player_id", p.ID()))
		}
	}
}

// canAffordGreenery checks whether a player has enough plants to convert to greenery,
// accounting for card discounts (e.g., Ecoline).
func canAffordGreenery(p *playerPkg.Player, calculator *gamecards.RequirementModifierCalculator) bool {
	discounts := calculator.CalculateStandardProjectDiscounts(p, shared.StandardProjectConvertPlantsToGreenery)
	plantDiscount := discounts[shared.ResourcePlant]
	requiredPlants := max(resource_conversion.BasePlantsForGreenery-plantDiscount, 1)
	return p.Resources().Get().Plants >= requiredPlants
}
