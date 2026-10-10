package action

import (
	"openmars/internal/game"
	"openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

func modeBehaviorErrors(cardID string, b shared.CardBehavior, p *player.Player, g *game.Game, registry cards.CardRegistry, metadata map[string]interface{}) []player.StateError {
	var errors []player.StateError
	add := func(message string) {
		errors = append(errors, player.StateError{Code: player.ErrorCodeInvalidRequirement, Category: player.ErrorCategoryRequirement, Message: message})
	}
	if err := cards.BehaviorAvailable(g, p, cardID, b, registry); err != nil {
		add(err.Error())
	}
	for _, output := range b.Outputs {
		o, ok := output.(*shared.StateCondition)
		if !ok {
			continue
		}
		switch o.ResourceType {
		case shared.ResourceStateChange:
			if err := g.ValidateModeStateChange(p.ID(), o.State, o.Amount); err != nil {
				add(err.Error())
			}
		case shared.ResourceScheduleStateWindow:
			if err := g.ValidateStateWindow(o); err != nil {
				add(err.Error())
			}
		case shared.ResourceRemoveDependency, shared.ResourceRetireDependency:
			var ids []string
			for _, id := range p.PlayedCards().Cards() {
				if cards.ValidateDependencyTarget(p, registry, id, o.Dependency, o.ResourceType == shared.ResourceRemoveDependency, o.MinimumProduction) == nil {
					ids = append(ids, id)
				}
			}
			if o.Target == "self-card" {
				ids = nil
				if cards.ValidateDependencyTarget(p, registry, cardID, o.Dependency, false) == nil {
					ids = []string{cardID}
				}
			}
			if len(ids) == 0 {
				add("No eligible ongoing project")
			}
			if metadata != nil {
				metadata["dependencyTargets"] = ids
			}
		}
	}
	return errors
}
