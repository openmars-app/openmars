package action

import (
	"fmt"

	"openmars/internal/game"
	"openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// IsActionReuse reports whether the behavior grants reuse of another card action.
func IsActionReuse(behavior shared.CardBehavior) bool {
	for _, output := range behavior.Outputs {
		if output.GetResourceType() == shared.ResourceActionReuse {
			return true
		}
	}
	return false
}

// ResolveActionReuse validates ownership and usage, returning the action to consume.
func ResolveActionReuse(p *player.Player, sourceID string, target shared.CardAction) (*shared.CardAction, error) {
	if sourceID == target.CardID || IsActionReuse(target.Behavior) {
		return nil, fmt.Errorf("cannot reuse an action-reuse ability")
	}
	if !cards.HasManualTrigger(target.Behavior) || target.TimesUsedThisGeneration < 1 {
		return nil, fmt.Errorf("target must be a manual action already used this generation")
	}
	owned := false
	for _, act := range p.Actions().List() {
		if act.CardID == target.CardID && act.BehaviorIndex == target.BehaviorIndex {
			owned = true
		}
	}
	if !owned {
		return nil, fmt.Errorf("target action is not owned")
	}
	for _, act := range p.Actions().List() {
		if act.CardID != sourceID || !cards.HasManualTrigger(act.Behavior) || !IsActionReuse(act.Behavior) {
			continue
		}
		if act.TimesUsedThisGeneration > 0 {
			return nil, fmt.Errorf("reuse action already played this generation")
		}
		return &act, nil
	}
	return nil, fmt.Errorf("action-reuse action not found on card %s", sourceID)
}

// ValidateCardActionChoice checks action requirements and the selected branch before any costs are paid.
func ValidateCardActionChoice(behavior shared.CardBehavior, choiceIndex *int, p *player.Player, g *game.Game, registry cards.CardRegistry) error {
	if errors := validateGenerationalEventRequirements(behavior, p); len(errors) > 0 {
		return fmt.Errorf("%s", errors[0].Message)
	}
	if len(behavior.Choices) > 0 && choiceIndex == nil {
		return fmt.Errorf("must select an action choice")
	}
	if choiceIndex == nil {
		return nil
	}
	if *choiceIndex < 0 || *choiceIndex >= len(behavior.Choices) {
		return fmt.Errorf("invalid action choice")
	}
	if !shared.IsChoiceValidForPolicy(*choiceIndex, behavior.Choices, behavior.ChoicePolicy, p.Resources().Production()) {
		return fmt.Errorf("choice not valid for policy")
	}
	choice := behavior.Choices[*choiceIndex]
	// Costs are validated together by the payment system, including substitutes.
	if errors := CalculateChoiceErrors(shared.Choice{Requirements: choice.Requirements, StateRequirements: choice.StateRequirements}, p, g, registry); len(errors) > 0 {
		return fmt.Errorf("%s", errors[0].Message)
	}
	return nil
}

// ActionReuseOption contains server-calculated availability for an already-used action.
type ActionReuseOption struct {
	Action shared.CardActionRef
	Errors []player.StateError
}

// CalculateActionReuseOptions evaluates target actions with only their usage limit waived.
func CalculateActionReuseOptions(sourceID string, p *player.Player, g *game.Game, registry cards.CardRegistry) []ActionReuseOption {
	options := []ActionReuseOption{}
	for _, target := range p.Actions().List() {
		if target.CardID == sourceID || IsActionReuse(target.Behavior) || !cards.HasManualTrigger(target.Behavior) || target.TimesUsedThisGeneration < 1 {
			continue
		}
		var errors []player.StateError
		if _, err := ResolveActionReuse(p, sourceID, target); err != nil {
			errors = append(errors, player.StateError{Code: player.ErrorCodeInvalidRequirement, Category: player.ErrorCategoryAvailability, Message: err.Error()})
		}
		var targetErrors []player.StateError
		choices := []*int{nil}
		if len(target.Behavior.Choices) > 0 {
			choices = nil
			for i := range target.Behavior.Choices {
				index := i
				choices = append(choices, &index)
			}
		}
		for _, choice := range choices {
			if err := ValidateCardActionChoice(target.Behavior, choice, p, g, registry); err != nil {
				targetErrors = []player.StateError{{Code: player.ErrorCodeInvalidRequirement, Category: player.ErrorCategoryRequirement, Message: err.Error()}}
				continue
			}
			behavior := target.Behavior
			behavior.Inputs, behavior.Outputs = behavior.ExtractInputsOutputs(choice)
			behavior.Choices = nil
			state := CalculatePlayerCardActionState(target.CardID, behavior, 0, p, g, registry)
			targetErrors = state.Errors
			if state.Available() {
				break
			}
		}
		errors = append(errors, targetErrors...)
		options = append(options, ActionReuseOption{Action: shared.CardActionRef{CardID: target.CardID, BehaviorIndex: target.BehaviorIndex}, Errors: errors})
	}
	return options
}
