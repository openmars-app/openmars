package action

import (
	"openmars/internal/game"
	"openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// MatchesPendingCardPlay accepts projects in the effect's selection scope.
func MatchesPendingCardPlay(card *cards.Card, pending *shared.PendingCardPlaySelection) bool {
	if pending == nil || card == nil {
		return false
	}
	if card.Type != cards.CardTypeAutomated && card.Type != cards.CardTypeActive && card.Type != cards.CardTypeEvent {
		return false
	}
	return len(pending.Selectors) == 0 || cards.MatchesAnySelector(card, pending.Selectors)
}

// CardPlayCostReduction is derived from authoritative pending state, never the request.
func CardPlayCostReduction(p *player.Player, card *cards.Card) int {
	pending := p.Selection().GetPendingCardPlaySelection()
	if !MatchesPendingCardPlay(card, pending) || !p.Hand().HasCard(card.ID) {
		return 0
	}
	return pending.CostReduction
}

// CanResolveCardPlay checks hand rules without phase blockers or mutating resources.
func CanResolveCardPlay(p *player.Player, g *game.Game, registry cards.CardRegistry, operation *shared.CardOperationCondition) bool {
	pending := &shared.PendingCardPlaySelection{CostReduction: operation.CostReduction, Selectors: operation.Selectors}
	for _, id := range p.Hand().Cards() {
		card, err := registry.GetByID(id)
		if err != nil || !MatchesPendingCardPlay(card, pending) {
			continue
		}
		state := calculatePendingCardPlayability(card, p, g, registry, operation.CostReduction)
		if len(state.Errors) == 0 {
			return true
		}
	}
	return false
}
