package confirmation

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	baseaction "openmars/internal/action"
	"openmars/internal/action/turn_management"
	"openmars/internal/game"
	"openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// ConfirmCardDrawAction handles the business logic for confirming card draw selection
type ConfirmCardDrawAction struct {
	baseaction.BaseAction
}

// NewConfirmCardDrawAction creates a new confirm card draw action
func NewConfirmCardDrawAction(
	gameRepo game.GameRepository,
	cardRegistry cards.CardRegistry,
	logger *slog.Logger,
) *ConfirmCardDrawAction {
	return &ConfirmCardDrawAction{
		BaseAction: baseaction.NewBaseAction(gameRepo, cardRegistry),
	}
}

// Execute performs the confirm card draw action
func (a *ConfirmCardDrawAction) Execute(ctx context.Context, gameID string, playerID string, cardsToTake []string, cardsToBuy []string, payment shared.Payment) error {
	log := a.InitLogger(gameID, playerID).With(
		slog.String("action", "confirm_card_draw"),
		slog.Int("cards_to_take", len(cardsToTake)),
		slog.Int("cards_to_buy", len(cardsToBuy)),
	)
	log.Debug("Confirming card draw selection")

	g, err := baseaction.ValidateActiveGame(ctx, a.GameRepository(), gameID, log)
	if err != nil {
		return err
	}

	p, err := a.GetPlayerFromGame(g, playerID, log)
	if err != nil {
		return err
	}

	selection := p.Selection().GetPendingCardDrawSelection()
	if selection == nil {
		log.Warn("No pending card draw selection found")
		return fmt.Errorf("no pending card draw selection found")
	}

	if err := baseaction.ValidateCurrentTurnOrInitPlayer(g, playerID, log); err != nil {
		return err
	}
	if len(cardsToTake) < selection.MinFreeTakeCount {
		return fmt.Errorf("must take at least %d cards", selection.MinFreeTakeCount)
	}
	seen := map[string]bool{}
	for _, group := range [][]string{cardsToTake, cardsToBuy} {
		for _, id := range group {
			if seen[id] {
				return fmt.Errorf("duplicate selected card %s", id)
			}
			seen[id] = true
		}
	}
	totalSelected := len(cardsToTake) + len(cardsToBuy)
	maxAllowed := selection.FreeTakeCount + selection.MaxBuyCount

	if totalSelected > maxAllowed {
		log.Warn("Too many cards selected",
			slog.Int("selected", totalSelected),
			slog.Int("max_allowed", maxAllowed))
		return fmt.Errorf("too many cards selected: selected %d, max allowed %d", totalSelected, maxAllowed)
	}

	if len(cardsToTake) > selection.FreeTakeCount {
		log.Warn("Too many free cards selected",
			slog.Int("selected", len(cardsToTake)),
			slog.Int("max", selection.FreeTakeCount))
		return fmt.Errorf("too many free cards selected: selected %d, max %d", len(cardsToTake), selection.FreeTakeCount)
	}

	if len(cardsToBuy) > selection.MaxBuyCount {
		log.Warn("Too many cards to buy",
			slog.Int("selected", len(cardsToBuy)),
			slog.Int("max", selection.MaxBuyCount))
		return fmt.Errorf("too many cards to buy: selected %d, max %d", len(cardsToBuy), selection.MaxBuyCount)
	}

	allSelectedCards := append(append([]string(nil), cardsToTake...), cardsToBuy...)
	for _, cardID := range allSelectedCards {
		if !slices.Contains(selection.AvailableCards, cardID) {
			log.Warn("Card not in available cards", slog.String("card_id", cardID))
			return fmt.Errorf("card %s not in available cards", cardID)
		}
	}

	totalCost := len(cardsToBuy) * selection.CardBuyCost

	quote, err := cards.QuotePayment(p, g, a.CardRegistryForGame(g), cards.PaymentContext{Costs: map[shared.ResourceType]int{shared.ResourceCredit: totalCost}, Action: "card-buying"})
	if err != nil {
		return err
	}
	paymentPlan, err := cards.ValidatePayment(quote, payment)
	if err != nil {
		return err
	}
	cards.ApplyPayment(p, paymentPlan)

	p.Selection().SetPendingCardDrawSelection(nil)

	if selection.PlayAsPrelude {
		// Play selected prelude cards instead of adding to hand
		for _, preludeID := range allSelectedCards {
			if err := turn_management.ApplyPreludeCard(ctx, g, p, preludeID, a.CardRegistryForGame(g), a.StateRepository(), log); err != nil {
				return fmt.Errorf("failed to play prelude card %s: %w", preludeID, err)
			}
		}
		log.Debug("Played selected prelude cards",
			slog.Any("prelude_ids", allSelectedCards))
	} else {
		for _, cardID := range allSelectedCards {
			p.Hand().AddCard(cardID)
		}
		log.Debug("Added selected cards to hand",
			slog.Int("cards_taken", len(cardsToTake)),
			slog.Int("cards_bought", len(cardsToBuy)),
			slog.Int("total_cards", len(allSelectedCards)))
	}

	unselectedCards := []string{}
	for _, cardID := range selection.AvailableCards {
		if !slices.Contains(allSelectedCards, cardID) {
			unselectedCards = append(unselectedCards, cardID)
		}
	}

	if len(unselectedCards) > 0 {
		if selection.PlayAsPrelude {
			// Prelude cards are removed permanently, never discarded
			if err := g.Deck().Remove(ctx, unselectedCards); err != nil {
				log.Error("Failed to remove unselected prelude cards", slog.Any("error", err))
				return fmt.Errorf("failed to remove unselected prelude cards: %w", err)
			}
			log.Debug("Removed unselected prelude cards permanently",
				slog.Int("count", len(unselectedCards)),
				slog.Any("card_ids", unselectedCards))
		} else {
			if err := g.Deck().Discard(ctx, unselectedCards); err != nil {
				log.Error("Failed to discard unselected cards", slog.Any("error", err))
				return fmt.Errorf("failed to discard unselected cards: %w", err)
			}
			log.Debug("Discarded unselected cards to discard pile",
				slog.Int("count", len(unselectedCards)),
				slog.Any("card_ids", unselectedCards))
		}
	}

	// If this selection was triggered by a card action, complete the action now
	if selection.CompleteAction != nil && !selection.PlayAsPrelude {
		a.completeCardAction(g, p, selection, log)
	}

	if phase := g.CurrentPhase(); phase != shared.GamePhaseInitApplyCorp && phase != shared.GamePhaseInitApplyPrelude {
		baseaction.AutoAdvanceTurnIfNeeded(g, playerID, log, a.CardRegistryForGame(g))
	}
	log.Info("Card draw confirmation completed",
		slog.String("source", selection.Source),
		slog.Int("cards_taken", len(cardsToTake)),
		slog.Int("cards_bought", len(cardsToBuy)),
		slog.Int("total_cost", totalCost),
		slog.Bool("play_as_prelude", selection.PlayAsPrelude))

	return nil
}

// completeCardAction increments usage counts and consumes an action
// for the action responsible for this selection (which may differ from its effect source).
func (a *ConfirmCardDrawAction) completeCardAction(
	g *game.Game,
	p *player.Player,
	selection *shared.PendingCardDrawSelection,
	log *slog.Logger,
) {
	// Complete the stored action reference, not the effect source.
	actions := p.Actions().List()
	for i := range actions {
		if actions[i].CardID == selection.CompleteAction.CardID && actions[i].BehaviorIndex == selection.CompleteAction.BehaviorIndex {
			actions[i].TimesUsedThisTurn++
			actions[i].TimesUsedThisGeneration++
			log.Debug("Incremented action usage counts from card draw confirmation",
				slog.String("card_id", selection.CompleteAction.CardID),
				slog.Int("behavior_index", selection.CompleteAction.BehaviorIndex),
				slog.Int("times_used_this_turn", actions[i].TimesUsedThisTurn),
				slog.Int("times_used_this_generation", actions[i].TimesUsedThisGeneration))
			break
		}
	}
	p.Actions().SetActions(actions)

	// Consume the player action
	a.ConsumePlayerAction(g, log)
}

// AcknowledgeReceipt dismisses a private receipt without granting cards or spending an action.
func (a *ConfirmCardDrawAction) AcknowledgeReceipt(ctx context.Context, gameID, playerID, receiptID string) error {
	if receiptID == "" {
		return fmt.Errorf("receipt ID is required")
	}
	g, err := a.GameRepository().Get(ctx, gameID)
	if err != nil {
		return err
	}
	p, err := g.GetPlayer(playerID)
	if err != nil {
		return err
	}
	p.Selection().AcknowledgeCardReceipt(receiptID)
	return nil
}
