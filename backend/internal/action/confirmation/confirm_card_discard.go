package confirmation

import (
	"context"
	"fmt"
	"log/slog"
	baseaction "openmars/internal/action"
	"slices"

	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// ConfirmCardDiscardAction handles the business logic for confirming card discard selection
type ConfirmCardDiscardAction struct {
	baseaction.BaseAction
}

// NewConfirmCardDiscardAction creates a new confirm card discard action
func NewConfirmCardDiscardAction(
	gameRepo game.GameRepository,
	cardRegistry gamecards.CardRegistry,
	stateRepo game.GameStateRepository,
	logger *slog.Logger,
) *ConfirmCardDiscardAction {
	return &ConfirmCardDiscardAction{
		BaseAction: baseaction.NewBaseActionWithStateRepo(gameRepo, cardRegistry, stateRepo),
	}
}

// Execute performs the confirm card discard action
// cardsToDiscard: card IDs from hand to discard (empty = skip if optional)
func (a *ConfirmCardDiscardAction) Execute(ctx context.Context, gameID string, playerID string, resolutionID string, cardsToDiscard []string) error {
	log := a.InitLogger(gameID, playerID).With(
		slog.String("action", "confirm_card_discard"),
		slog.Int("cards_to_discard", len(cardsToDiscard)),
	)
	log.Debug("Confirming card discard selection")

	g, err := baseaction.ValidateActiveGame(ctx, a.GameRepository(), gameID, log)
	if err != nil {
		return err
	}

	p, err := a.GetPlayerFromGame(g, playerID, log)
	if err != nil {
		return err
	}

	selection := p.Selection().GetPendingBehaviorResolution(resolutionID)
	if selection == nil || selection.Kind != "card-discard" {
		log.Warn("No pending card discard selection found")
		return fmt.Errorf("no pending card discard selection found")
	}

	// Validate discard count
	if len(cardsToDiscard) < selection.MinCards {
		log.Warn("Not enough cards to discard",
			slog.Int("selected", len(cardsToDiscard)),
			slog.Int("min_required", selection.MinCards))
		return fmt.Errorf("must discard at least %d card(s), selected %d", selection.MinCards, len(cardsToDiscard))
	}

	if len(cardsToDiscard) > selection.MaxCards {
		log.Warn("Too many cards to discard",
			slog.Int("selected", len(cardsToDiscard)),
			slog.Int("max_allowed", selection.MaxCards))
		return fmt.Errorf("can discard at most %d card(s), selected %d", selection.MaxCards, len(cardsToDiscard))
	}

	seen := map[string]bool{}
	for _, id := range cardsToDiscard {
		if seen[id] {
			return fmt.Errorf("duplicate discard card %s", id)
		}
		seen[id] = true
	}
	// Validate all cards are in hand
	handCards := p.Hand().Cards()
	for _, cardID := range cardsToDiscard {
		if !slices.Contains(handCards, cardID) {
			log.Warn("Card not in hand", slog.String("card_id", cardID))
			return fmt.Errorf("card %s not in player's hand", cardID)
		}
	}

	// Remove discarded cards from hand
	if len(cardsToDiscard) > 0 {
		validator := gamecards.NewBehaviorApplier(p, g, selection.Source, log).WithSourceCardID(selection.SourceCardID).WithTriggeringCard(selection.TriggeringCardID, selection.TriggeringPlayerID).WithCardRegistry(a.CardRegistryForGame(g))
		if err := validator.ValidateResourceOutputs(selection.PendingOutputs); err != nil {
			return err
		}
	}
	for _, cardID := range cardsToDiscard {
		p.Hand().RemoveCard(cardID)
	}

	if len(cardsToDiscard) > 0 {
		if err := g.Deck().Discard(ctx, cardsToDiscard); err != nil {
			log.Error("Failed to discard cards to discard pile", slog.Any("error", err))
			return fmt.Errorf("failed to discard cards: %w", err)
		}
		log.Debug("Discarded cards from hand to discard pile",
			slog.Int("count", len(cardsToDiscard)),
			slog.Any("card_ids", cardsToDiscard))
	}

	var calculatedOutputs []shared.CalculatedOutput
	// Skipping an optional discard does not grant its reward.
	if len(cardsToDiscard) > 0 && len(selection.PendingOutputs) > 0 {
		selfOutputs, err := a.applyPendingOutputs(ctx, g, p, selection, log)
		if err != nil {
			log.Error("Failed to apply pending outputs after discard", slog.Any("error", err))
			return fmt.Errorf("failed to apply pending outputs: %w", err)
		}

		// Add triggered effect for self-player: discard + draws
		calculatedOutputs = []shared.CalculatedOutput{
			{ResourceType: string(shared.ResourceCardDiscard), Amount: len(cardsToDiscard)},
		}
		calculatedOutputs = append(calculatedOutputs, selfOutputs...)
		g.AddTriggeredEffect(shared.TriggeredEffect{
			CardName:          selection.Source,
			PlayerID:          p.ID(),
			SourceType:        shared.SourceTypeCardPlay,
			CalculatedOutputs: calculatedOutputs,
		})
	}

	// Clear the pending selection
	p.Selection().RemovePendingBehaviorResolution(resolutionID)
	description := "Skipped discard"
	if len(cardsToDiscard) > 0 {
		description = fmt.Sprintf("Discarded %d card(s)", len(cardsToDiscard))
	}
	a.WriteStateLogWithChoiceAndOutputs(ctx, g, selection.Source, shared.SourceTypePassiveEffect, playerID, description, nil, calculatedOutputs)
	baseaction.AutoAdvanceTurnIfNeeded(g, playerID, log, a.CardRegistryForGame(g))

	log.Info("Card discard confirmation completed",
		slog.String("source", selection.Source),
		slog.Int("cards_discarded", len(cardsToDiscard)))

	return nil
}

// applyPendingOutputs applies the outputs after a successful discard.
// Returns calculated outputs for the self-player (for triggered effect notifications).
func (a *ConfirmCardDiscardAction) applyPendingOutputs(
	ctx context.Context,
	g *game.Game,
	p *player.Player,
	selection *shared.PendingBehaviorResolution,
	log *slog.Logger,
) ([]shared.CalculatedOutput, error) {
	var selfOutputs []shared.CalculatedOutput

	for _, outputBC := range selection.PendingOutputs {
		if outputBC.GetResourceType() == shared.ResourceCardDraw {
			if outputBC.GetTarget() == "all-opponents" {
				for _, opponent := range g.GetAllPlayers() {
					if opponent.ID() == p.ID() {
						continue
					}
					drawnCards, err := g.Deck().DrawProjectCards(ctx, outputBC.GetAmount())
					if err != nil {
						log.Warn("Failed to draw cards for opponent",
							slog.String("opponent_id", opponent.ID()),
							slog.Any("error", err))
						continue
					}
					opponent.Hand().ReceiveCards(drawnCards, selection.Source, selection.SourceCardID)
					log.Debug("Opponent drew cards",
						slog.String("opponent_id", opponent.ID()),
						slog.Int("count", len(drawnCards)))

					g.AddTriggeredEffect(shared.TriggeredEffect{
						CardName:   selection.Source,
						PlayerID:   opponent.ID(),
						SourceType: shared.SourceTypeCardPlay,
						CalculatedOutputs: []shared.CalculatedOutput{
							{ResourceType: string(shared.ResourceCardDraw), Amount: len(drawnCards)},
						},
					})
				}
				continue
			}

			drawnCards, err := g.Deck().DrawProjectCards(ctx, outputBC.GetAmount())
			if err != nil {
				return nil, fmt.Errorf("failed to draw cards: %w", err)
			}
			p.Hand().ReceiveCards(drawnCards, selection.Source, selection.SourceCardID)
			selfOutputs = append(selfOutputs, shared.CalculatedOutput{
				ResourceType: string(shared.ResourceCardDraw),
				Amount:       len(drawnCards),
			})
			log.Debug("Drew cards after discard",
				slog.Int("count", len(drawnCards)))
			continue
		}

		// For non-card-draw outputs, use the behavior applier
		applier := gamecards.NewBehaviorApplier(p, g, selection.Source, slog.Default()).
			WithSourceCardID(selection.SourceCardID).
			WithTriggeringCard(selection.TriggeringCardID, selection.TriggeringPlayerID).
			WithCardRegistry(a.CardRegistryForGame(g)).
			WithSourceType(shared.SourceTypePassiveEffect)
		if err := applier.ApplyOutputs(ctx, []shared.BehaviorCondition{outputBC}); err != nil {
			return nil, err
		}
	}
	return selfOutputs, nil
}
