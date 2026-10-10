package action

import (
	"fmt"
	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// CalculateResolutionChoiceErrors checks a choice using its retained effect context.
func CalculateResolutionChoiceErrors(choice shared.Choice, resolution *shared.PendingBehaviorResolution, p *player.Player, g *game.Game, registry gamecards.CardRegistry) []player.StateError {
	errors := CalculateChoiceErrors(choice, p, g, registry)
	addError := func(message string) {
		errors = append(errors, player.StateError{Code: player.ErrorCodeInsufficientResources, Category: player.ErrorCategoryInput, Message: message})
	}
	needed := 0
	for _, input := range choice.Inputs {
		if input.GetTarget() == "self-card" && gamecards.IsStorageResourceType(input.GetResourceType()) {
			needed += input.GetAmount()
		}
	}
	if needed > p.Resources().GetCardStorage(resolution.SourceCardID) {
		addError("Not enough resources on " + resolution.Source)
	}
	for _, output := range choice.Outputs {
		if output.GetTarget() == "triggering-card" {
			_, _, err := gamecards.ResolveTriggeringCard(g, registry, resolution.TriggeringCardID, resolution.TriggeringPlayerID, output)
			if err != nil {
				addError(err.Error())
			}
		}
	}
	for _, targets := range ResolutionStorageTargets(choice, p, registry) {
		if len(targets) == 0 {
			addError("No eligible card for resource storage")
		}
	}
	return errors
}

// ValidateResolutionStorageTargets validates every player-selected target before costs are paid.
func ValidateResolutionStorageTargets(choice shared.Choice, targets []string, p *player.Player, registry gamecards.CardRegistry) error {
	index := 0
	for _, output := range choice.Outputs {
		if output.GetTarget() != "any-card" {
			continue
		}
		if index >= len(targets) {
			return fmt.Errorf("missing card storage target")
		}
		id := targets[index]
		index++
		if operation, ok := output.(*shared.StateCondition); ok {
			if err := gamecards.ValidateDependencyTarget(p, registry, id, operation.Dependency, operation.ResourceType == shared.ResourceRemoveDependency, operation.MinimumProduction); err != nil {
				return err
			}
			continue
		}
		if err := gamecards.ValidateStorageDestination(p, registry, output, id, ""); err != nil {
			return err
		}
	}
	if index != len(targets) {
		return fmt.Errorf("unexpected card storage targets")
	}
	return nil
}

// ResolutionStorageTargets lists legal player-selected destinations in output order.
func ResolutionStorageTargets(choice shared.Choice, p *player.Player, registry gamecards.CardRegistry) [][]string {
	var result [][]string
	for _, output := range choice.Outputs {
		if output.GetTarget() != "any-card" {
			continue
		}
		eligible := gamecards.StorageDestinations(p, registry, output, "")
		if operation, ok := output.(*shared.StateCondition); ok {
			eligible = nil
			for _, id := range p.PlayedCards().Cards() {
				if gamecards.ValidateDependencyTarget(p, registry, id, operation.Dependency, operation.ResourceType == shared.ResourceRemoveDependency, operation.MinimumProduction) == nil {
					eligible = append(eligible, id)
				}
			}
		}
		if eligible == nil {
			eligible = []string{}
		}
		result = append(result, eligible)
	}
	return result
}
