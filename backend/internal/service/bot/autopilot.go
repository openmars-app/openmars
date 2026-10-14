package bot

import (
	"context"
	"fmt"

	"openmars/internal/action"
	"openmars/internal/delivery/dto"
	"openmars/internal/game/shared"
)

// AcknowledgeReceipts dismisses informational card receipts; the cards are already in hand.
func (ts *ToolServer) AcknowledgeReceipts(ctx context.Context, snap *Snapshot) error {
	for _, receipt := range snap.View.CurrentPlayer.CardReceipts {
		if err := ts.actions.ConfirmCardDraw.AcknowledgeReceipt(ctx, snap.Game.ID(), snap.PlayerID, receipt.ID); err != nil {
			return fmt.Errorf("acknowledge receipt %s: %w", receipt.ID, err)
		}
	}
	return nil
}

// Autopilot takes one legal step without a model: it resolves the first pending selection
// with the simplest legal choice, otherwise it passes. It keeps the game moving when the
// model is failing or the spend cap is reached.
func (ts *ToolServer) Autopilot(ctx context.Context, snap *Snapshot) error {
	a := ts.actions
	gid, pid := snap.Game.ID(), snap.PlayerID
	p := &snap.View.CurrentPlayer

	switch {
	case p.PendingTileSelection != nil:
		if len(p.PendingTileSelection.AvailableHexes) == 0 {
			return fmt.Errorf("tile selection has no available hexes")
		}
		_, err := a.SelectTile.Execute(ctx, gid, pid, p.PendingTileSelection.AvailableHexes[0])
		return err

	case p.PendingCardSelection != nil:
		return a.ConfirmSellPatents.Execute(ctx, gid, pid, firstCardIDs(p.PendingCardSelection.AvailableCards, p.PendingCardSelection.MinCards))

	case p.PendingCardPlaySelection != nil:
		for _, card := range p.PendingCardPlaySelection.AvailableCards {
			if !card.Available {
				continue
			}
			payment, err := ts.quotePayment(snap.Game, snap.Player, action.PaymentIntent{Action: "play-card", CardID: card.ID}, nil)
			if err != nil {
				continue
			}
			if err = a.PlayCard.Execute(ctx, gid, pid, card.ID, payment, nil, nil, nil, nil, nil); err == nil {
				return nil
			}
		}
		return fmt.Errorf("no automatic card play available; resolve the pending card with its choices")
	case p.PendingCardDrawSelection != nil:
		sel := p.PendingCardDrawSelection
		return a.ConfirmCardDraw.Execute(ctx, gid, pid, firstCardIDs(sel.AvailableCards, sel.MinFreeTakeCount), []string{}, emptyPayment())

	case len(p.PendingBehaviorResolutions) > 0:
		res := p.PendingBehaviorResolutions[0]
		if res.Kind == "optional" {
			return a.ConfirmBehaviorChoice.ExecuteOptional(ctx, gid, pid, res.ID, false)
		}
		if res.Kind == "card-discard" {
			return a.ConfirmCardDiscard.Execute(ctx, gid, pid, res.ID, firstCardIDs(p.Cards, res.MinCards))
		}
		for i, choice := range res.Choices {
			if choice.Available {
				return a.ConfirmBehaviorChoice.Execute(ctx, gid, pid, res.ID, i, nil)
			}
		}
		return a.ConfirmBehaviorChoice.Execute(ctx, gid, pid, res.ID, 0, nil)

	case p.PendingCardReveal != nil:
		return a.ConfirmCardReveal.Execute(ctx, gid, pid)

	case p.PendingResourceRemovalSelection != nil:
		sel := p.PendingResourceRemovalSelection
		if err := a.ConfirmResourceRemoval.Execute(ctx, gid, pid, sel.ID, "", 0); err == nil {
			return nil
		}
		for _, target := range sel.EligiblePlayerIDs {
			if amount := min(sel.Amount, sel.MaxAmounts[target]); amount > 0 {
				return a.ConfirmResourceRemoval.Execute(ctx, gid, pid, sel.ID, target, amount)
			}
		}
		return fmt.Errorf("no legal resource removal")

	case p.PendingEffectSelection != nil:
		if p.PendingEffectSelection.CanContinueWithoutSelection {
			return a.ConfirmEffectSelection.Continue(ctx, gid, pid)
		}
		return a.ConfirmEffectSelection.Execute(ctx, gid, pid, 0)

	case p.PendingColonySelection != nil:
		if p.PendingColonySelection.CanContinueWithoutSelection {
			return a.ConfirmColonyPlacement.Continue(ctx, gid, pid)
		}
		if len(p.PendingColonySelection.AvailableColonyIDs) == 0 {
			return fmt.Errorf("colony selection has no options")
		}
		return a.ConfirmColonyPlacement.Execute(ctx, gid, pid, p.PendingColonySelection.AvailableColonyIDs[0])

	case p.PendingColonyResourceSelection != nil:
		return a.ConfirmColonyResource.Execute(ctx, gid, pid, "")

	case p.PendingAwardFundSelection != nil:
		if p.PendingAwardFundSelection.CanContinueWithoutSelection {
			return a.ConfirmAwardFund.Continue(ctx, gid, pid)
		}
		if len(p.PendingAwardFundSelection.AvailableAwards) == 0 {
			return fmt.Errorf("award selection has no options")
		}
		return a.ConfirmAwardFund.Execute(ctx, gid, pid, p.PendingAwardFundSelection.AvailableAwards[0])

	case p.PendingFreeTradeSelection != nil:
		if p.PendingFreeTradeSelection.CanContinueWithoutSelection {
			return a.ConfirmFreeTrade.Continue(ctx, gid, pid)
		}
		if len(p.PendingFreeTradeSelection.AvailableColonyIDs) == 0 {
			return fmt.Errorf("free trade has no colonies")
		}
		colonyID := p.PendingFreeTradeSelection.AvailableColonyIDs[0]
		return a.ConfirmFreeTrade.Execute(ctx, gid, pid, colonyID, firstTradeSteps(snap.View.Colonies, colonyID))

	case p.SelectCorporationPhase != nil || p.SelectStartingCardsPhase != nil || p.SelectPreludeCardsPhase != nil:
		return ts.autopilotStartingChoices(ctx, snap)

	case p.ProductionPhase != nil && !p.ProductionPhase.SelectionComplete:
		return a.ConfirmProductionCards.Execute(ctx, gid, pid, []string{}, false, emptyPayment())

	default:
		return a.SkipAction.Execute(ctx, gid, pid)
	}
}

func (ts *ToolServer) autopilotStartingChoices(ctx context.Context, snap *Snapshot) error {
	p := &snap.View.CurrentPlayer
	corporationID := ""
	if p.SelectCorporationPhase != nil && len(p.SelectCorporationPhase.AvailableCorporations) > 0 {
		corporationID = p.SelectCorporationPhase.AvailableCorporations[0].ID
	}
	preludeIDs := []string{}
	if sel := p.SelectPreludeCardsPhase; sel != nil {
		for i := 0; i < sel.MaxSelectable && i < len(sel.AvailablePreludes); i++ {
			preludeIDs = append(preludeIDs, sel.AvailablePreludes[i].ID)
		}
	}
	payment, err := ts.quotePayment(snap.Game, snap.Player, action.PaymentIntent{Action: "select-starting-choices", CorporationID: corporationID}, nil)
	if err != nil {
		return err
	}
	return ts.actions.SelectStartingChoices.Execute(ctx, snap.Game.ID(), snap.PlayerID, corporationID, preludeIDs, []string{}, payment)
}

func firstCardIDs(cards []dto.PlayerCardDto, n int) []string {
	ids := []string{}
	for i := 0; i < n && i < len(cards); i++ {
		ids = append(ids, cards[i].ID)
	}
	return ids
}

func emptyPayment() shared.Payment {
	return shared.Payment{Allocations: []shared.PaymentAllocation{}}
}

func firstTradeSteps(colonies []dto.ColonyDto, colonyID string) int {
	for _, c := range colonies {
		if c.ID == colonyID && len(c.TradeOptions) > 0 {
			return c.TradeOptions[0].TrackSteps
		}
	}
	return 0
}
