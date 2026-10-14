package action

import (
	"context"
	"fmt"
	"log/slog"
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

func CalculateOptionalEffectErrors(resolution *shared.PendingBehaviorResolution, p *player.Player, g *game.Game, registry gamecards.CardRegistry) []player.StateError {
	behavior := resolution.OptionalBehavior
	if behavior == nil {
		return []player.StateError{{Code: player.ErrorCodeInvalidRequirement, Category: player.ErrorCategoryRequirement, Message: "No optional effect to resolve"}}
	}
	errors := CalculateChoiceErrors(shared.Choice{StateRequirements: behavior.StateRequirements, Outputs: behavior.Outputs}, p, g, registry)
	applier := gamecards.NewBehaviorApplier(p, g, resolution.Source, nil).WithSourceCardID(resolution.SourceCardID).WithCardRegistry(registry)
	if err := applier.ValidateInputs(behavior.Inputs); err != nil {
		errors = append(errors, player.StateError{Code: player.ErrorCodeInsufficientResources, Category: player.ErrorCategoryInput, Message: err.Error()})
	}
	return errors
}

// ResolveForcedPassiveChoices runs after an action's synchronous effects have settled.
// Keep multiple pending effects interactive: their resolution order can change eligibility.
func ResolveForcedPassiveChoices(g *game.Game, log *slog.Logger, registry gamecards.CardRegistry) {
	for _, p := range g.GetAllPlayers() {
		selection := p.Selection()
		pending := selection.GetPendingBehaviorResolutions()
		if len(pending) != 1 || pending[0].Kind != "choice" {
			continue
		}
		if g.GetPendingTileSelection(p.ID()) != nil ||
			selection.GetPendingCardSelection() != nil || selection.GetPendingCardReveal() != nil ||
			selection.GetPendingCardDrawSelection() != nil || selection.GetPendingCardPlaySelection() != nil || selection.GetPendingResourceRemovalSelection() != nil ||
			len(selection.GetPendingColonyResourceQueue()) > 0 || selection.GetPendingAwardFundSelection() != nil ||
			selection.GetPendingColonySelection() != nil || selection.GetPendingFreeTradeSelection() != nil ||
			selection.GetPendingEffectSelection() != nil {
			continue
		}
		resolution := pending[0]
		if registry != nil {
			if card, err := registry.GetByID(resolution.SourceCardID); err == nil &&
				resolution.SourceBehaviorIndex >= 0 && resolution.SourceBehaviorIndex < len(card.Behaviors) &&
				card.Behaviors[resolution.SourceBehaviorIndex].Optional {
				continue
			}
		}
		if resolveForcedPassiveChoice(g, p, resolution, log, registry) {
			selection.RemovePendingBehaviorResolution(resolution.ID)
		}
	}
}

func resolveForcedPassiveChoice(g *game.Game, p *player.Player, resolution *shared.PendingBehaviorResolution, log *slog.Logger, cr gamecards.CardRegistry) bool {
	selected := -1
	var applier *gamecards.BehaviorApplier
	for i, choice := range resolution.Choices {
		if len(CalculateResolutionChoiceErrors(choice, resolution, p, g, cr)) > 0 {
			continue
		}
		if !automaticChoiceOperations(choice) {
			return false
		}
		candidate := gamecards.NewBehaviorApplier(p, g, resolution.Source, log).
			WithSourceCardID(resolution.SourceCardID).
			WithTriggeringCard(resolution.TriggeringCardID, resolution.TriggeringPlayerID).
			WithCardRegistry(cr).
			WithSourceType(shared.SourceTypePassiveEffect)
		if err := candidate.ValidateInputs(choice.Inputs); err != nil {
			continue
		}
		if err := candidate.ValidateModeOutputs(choice.Outputs); err != nil {
			continue
		}
		if err := candidate.ValidateResourceOutputs(choice.Outputs); err != nil {
			continue
		}
		if selected >= 0 {
			return false
		}
		selected, applier = i, candidate
	}
	if selected < 0 {
		return true
	}
	choice := resolution.Choices[selected]
	if err := applier.ApplyInputs(context.Background(), choice.Inputs); err != nil {
		log.Error("Failed to apply forced passive choice inputs", slog.String("card_name", resolution.Source), slog.Any("error", err))
		return false
	}
	if err := applier.ApplyOutputs(context.Background(), choice.Outputs); err != nil {
		log.Error("Failed to apply forced passive choice outputs", slog.String("card_name", resolution.Source), slog.Any("error", err))
		return false
	}
	return true
}

// Automatic resolution only executes operations whose application cannot fail after
// input validation with the retained player, game, and storage contexts. Interactive
// and multi-step operations stay explicit because the applier is not transactional.
func automaticChoiceOperations(choice shared.Choice) bool {
	for _, input := range choice.Inputs {
		if shared.IsVariableAmount(input) || shared.IsOptional(input) || shared.GetPerCondition(input) != nil || input.GetAmount() < 0 {
			return false
		}
		switch input.(type) {
		case *shared.BasicResourceCondition:
			if input.GetTarget() != "self-player" || !isBasicPlayerResource(input.GetResourceType()) {
				return false
			}
		case *shared.CardStorageCondition:
			if input.GetTarget() != "self-card" || !gamecards.IsStorageResourceType(input.GetResourceType()) {
				return false
			}
		default:
			return false
		}
	}
	for _, output := range choice.Outputs {
		if shared.IsVariableAmount(output) || shared.IsOptional(output) || shared.GetPerCondition(output) != nil || output.GetAmount() < 0 {
			return false
		}
		switch output.(type) {
		case *shared.BasicResourceCondition:
			if output.GetTarget() != "self-player" || !isBasicPlayerResource(output.GetResourceType()) {
				return false
			}
		case *shared.CardStorageCondition:
			if (output.GetTarget() != "self-card" && output.GetTarget() != "triggering-card") || !gamecards.IsStorageResourceType(output.GetResourceType()) {
				return false
			}
		case *shared.CardOperationCondition:
			if output.GetTarget() != "self-player" || output.GetResourceType() != shared.ResourceCardDraw {
				return false
			}
		default:
			return false
		}
	}
	return true
}
