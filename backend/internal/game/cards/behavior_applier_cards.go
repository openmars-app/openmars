package cards

import (
	"context"
	"fmt"
	"log/slog"

	"openmars/internal/game/shared"
)

func (a *BehaviorApplier) applyCardStorageOutput(ctx context.Context, o *shared.CardStorageCondition, amount int, log *slog.Logger) error {
	if a.player == nil {
		return fmt.Errorf("cannot apply card resource: no player context")
	}
	rt := o.ResourceType
	if o.Target == "triggering-card" {
		owner, target, err := ResolveTriggeringCard(a.game, a.cardRegistry, a.triggeringCardID, a.triggeringPlayerID, o)
		if err != nil {
			return err
		}
		owner.Resources().AddToStorage(target.ID, amount)
		return nil
	}

	// Generic card-resource: add resources of whatever type the target card stores
	if rt == shared.ResourceCardResource && o.Target != "self-card" {
		targetID := a.nextTargetCardID()
		if amount == 0 || (targetID == "" && amount > 0) {
			return nil
		}
		if targetID == "" {
			return fmt.Errorf("no target card specified for card-resource output")
		}
		if a.cardRegistry == nil {
			return fmt.Errorf("cannot apply card-resource: no card registry")
		}
		targetCard, err := a.cardRegistry.GetByID(targetID)
		if err != nil {
			return fmt.Errorf("target card not found in registry: %w", err)
		}
		if targetCard.ResourceStorage == nil {
			return fmt.Errorf("target card %s has no resource storage", targetID)
		}
		if amount < 0 {
			owner, err := a.removalStorageOwner(targetID, rt, -amount, false)
			if err != nil {
				return err
			}
			owner.Resources().AddToStorage(targetID, -min(-amount, owner.Resources().GetCardStorage(targetID)))
		} else {
			a.player.Resources().AddToStorage(targetID, amount)
		}
		log.Debug("Added card-resource to target card storage",
			slog.String("card_id", targetID), slog.String("storage_type", string(targetCard.ResourceStorage.Type)), slog.Int("amount", amount))
		return nil
	}

	// Specific storage types (animal, microbe, floater, etc.)
	switch o.Target {
	case "self-card":
		if a.sourceCardID == "" {
			log.Warn("Cannot place resource on self-card: no source card ID", slog.String("resource_type", string(rt)))
			return nil
		}
		a.player.Resources().AddToStorage(a.sourceCardID, amount)
		log.Debug("Added resource to card storage",
			slog.String("card_id", a.sourceCardID), slog.String("resource_type", string(rt)), slog.Int("amount", amount))

	case "steal-from-any-card":
		owner, err := a.removalStorageOwner(a.stealSourceCardID, rt, amount, true)
		if err != nil {
			return err
		}
		if a.sourceCardID == "" {
			return fmt.Errorf("missing transfer destination")
		}
		owner.Resources().AddToStorage(a.stealSourceCardID, -amount)
		a.player.Resources().AddToStorage(a.sourceCardID, amount)

	case "any-card":
		targetID := a.nextTargetCardID()
		if amount == 0 {
			return nil
		}
		if targetID == "" {
			log.Warn("No target card for any-card resource placement — resources lost",
				slog.String("resource_type", string(rt)), slog.Int("amount", amount))
			return nil
		}
		if a.cardRegistry != nil {
			targetCard, err := a.cardRegistry.GetByID(targetID)
			if err != nil {
				return fmt.Errorf("target card %s not found in registry: %w", targetID, err)
			}
			if targetCard.ResourceStorage == nil {
				return fmt.Errorf("target card %s has no resource storage", targetID)
			}
			if targetCard.ResourceStorage.Type != rt {
				return fmt.Errorf("target card %s stores %s, cannot add %s", targetID, targetCard.ResourceStorage.Type, rt)
			}
		}
		if amount < 0 {
			owner, err := a.removalStorageOwner(targetID, rt, -amount, false)
			if err != nil {
				return err
			}
			owner.Resources().AddToStorage(targetID, -min(-amount, owner.Resources().GetCardStorage(targetID)))
		} else {
			a.player.Resources().AddToStorage(targetID, amount)
		}
		log.Debug("Added resource to target card storage",
			slog.String("card_id", targetID), slog.String("resource_type", string(rt)), slog.Int("amount", amount))

	default:
		if a.sourceCardID != "" {
			a.player.Resources().AddToStorage(a.sourceCardID, amount)
			log.Debug("Added resource to card storage (default to self)",
				slog.String("card_id", a.sourceCardID), slog.String("resource_type", string(rt)), slog.Int("amount", amount))
		} else {
			log.Warn("Unhandled target for card resource", slog.String("target", o.Target), slog.String("resource_type", string(rt)))
		}
	}
	return nil
}

func (a *BehaviorApplier) applyCardOperationOutput(ctx context.Context, o *shared.CardOperationCondition, amount int, log *slog.Logger) error {
	switch o.ResourceType {
	case shared.ResourceCardDraw:
		if a.game == nil || a.player == nil {
			return fmt.Errorf("cannot apply card-draw: missing game or player context")
		}
		if o.Target == "all-opponents" {
			for _, opponent := range a.game.GetAllPlayers() {
				if opponent.ID() == a.player.ID() {
					continue
				}
				drawnCards, err := a.game.Deck().DrawProjectCards(ctx, amount)
				if err != nil {
					log.Warn("Failed to draw cards for opponent", slog.String("opponent_id", opponent.ID()), slog.Any("error", err))
					continue
				}
				for _, cardID := range drawnCards {
					opponent.Hand().AddCard(cardID)
				}
				log.Debug("Opponent drew cards", slog.String("opponent_id", opponent.ID()), slog.Int("amount", len(drawnCards)))
			}
		} else if HasCardSelectors(o.Selectors) && a.cardRegistry != nil {
			matcher := func(cardID string) bool {
				card, err := a.cardRegistry.GetByID(cardID)
				if err != nil || card == nil {
					return false
				}
				return MatchesAnySelector(card, o.Selectors)
			}
			matched, discarded, err := a.game.Deck().DrawProjectCardsUntilMatching(ctx, amount, matcher)
			if err != nil {
				log.Warn("Failed to draw matching cards", slog.Any("error", err))
				return nil
			}
			for _, cardID := range matched {
				a.player.Hand().AddCard(cardID)
			}
			if a.sourceType == shared.SourceTypeCorporationFirstAction || a.sourceType == shared.SourceTypeCardAction || (a.sourceType == shared.SourceTypeCardPlay && a.game.CurrentPhase() == shared.GamePhaseAction) {
				a.player.Selection().AddCardReceipt(a.source, a.sourceCardID, matched)
			}
			if len(discarded) > 0 {
				_ = a.game.Deck().Discard(ctx, discarded)
			}
			log.Debug("Drew matching cards (draw-until)", slog.Int("matched", len(matched)), slog.Int("discarded", len(discarded)))
		} else {
			drawnCards, err := a.game.Deck().DrawProjectCards(ctx, amount)
			if err != nil {
				log.Warn("Failed to draw cards", slog.Any("error", err))
				return nil
			}
			for _, cardID := range drawnCards {
				a.player.Hand().AddCard(cardID)
			}
			if a.sourceType == shared.SourceTypeCorporationFirstAction || a.sourceType == shared.SourceTypeCardAction || (a.sourceType == shared.SourceTypeCardPlay && a.game.CurrentPhase() == shared.GamePhaseAction) {
				a.player.Selection().AddCardReceipt(a.source, a.sourceCardID, drawnCards)
			}
			log.Debug("Drew cards and added to hand", slog.Int("amount", len(drawnCards)))
		}

	case shared.ResourceCardDiscard:
		log.Debug("Skipping card-discard output (handled at action layer)")

	case shared.ResourceCardPeek, shared.ResourceCardTake, shared.ResourceCardBuy:
		log.Debug("Skipping card draw output (handled by ApplyCardDrawOutputs)",
			slog.String("type", string(o.ResourceType)), slog.Int("amount", amount))

	default:
		log.Warn("Unhandled card operation type", slog.String("type", string(o.ResourceType)))
	}
	return nil
}
