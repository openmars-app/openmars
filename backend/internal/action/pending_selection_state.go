package action

import (
	"openmars/internal/game"
	"openmars/internal/game/award"
	"openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"slices"
)

// PendingColonyChoices revalidates the options of an already committed effect.
func PendingColonyChoices(g *game.Game, p *player.Player, pending *shared.PendingColonySelection) []string {
	ids := []string{}
	if pending == nil || !g.HasColonies() {
		return ids
	}
	legal := g.Colonies().GetPlaceableIDs(p.ID(), pending.AllowDuplicatePlayerColony)
	if pending.AddTile {
		legal = nil
		for _, def := range g.Colonies().UnusedDefinitions() {
			legal = append(legal, def.ID)
		}
	}
	for _, id := range pending.AvailableColonyIDs {
		if slices.Contains(legal, id) {
			ids = append(ids, id)
		}
	}
	return ids
}

func PendingFreeTradeChoices(g *game.Game, p *player.Player, pending *shared.PendingFreeTradeSelection) []string {
	ids := []string{}
	if pending == nil || !g.HasColonies() || g.Colonies().TradeFleet(p.ID()).Available() == 0 {
		return ids
	}
	legal := g.Colonies().GetTradeableIDs()
	for _, id := range pending.AvailableColonyIDs {
		if slices.Contains(legal, id) {
			ids = append(ids, id)
		}
	}
	return ids
}

func PendingAwardChoices(g *game.Game, pending *shared.PendingAwardFundSelection, registry award.AwardRegistry) []string {
	ids := []string{}
	if pending == nil || !g.Awards().CanFundMore() {
		return ids
	}
	for _, id := range pending.AvailableAwards {
		if g.Awards().IsFunded(shared.AwardType(id)) {
			continue
		}
		if selected := g.SelectedAwards(); len(selected) > 0 && !slices.Contains(selected, id) {
			continue
		}
		if registry != nil {
			def, err := registry.GetByID(id)
			if err != nil || !g.Settings().AllowsContent(def.GameModes, def.RequiresSystems, def.RequiresFeatures) {
				continue
			}
		}
		ids = append(ids, id)
	}
	return ids
}

func CanContinuePendingEffect(g *game.Game, p *player.Player, pending *shared.PendingEffectSelection, registry cards.CardRegistry, colonies cards.ColonyBonusLookup) bool {
	if pending == nil {
		return false
	}
	options, _, err := cards.EffectSelectionOptions(pending.Outputs, p, g, registry, colonies)
	if err != nil {
		return false
	}
	for _, previous := range pending.Options {
		for _, current := range options {
			if cards.SameEffectSelection(previous, current) {
				return false
			}
		}
	}
	return true
}
