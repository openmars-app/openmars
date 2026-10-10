package action

import (
	"fmt"
	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// CalculateChoiceErrors validates a single choice's requirements against the player/game state.
// Returns a list of errors explaining why the choice is unavailable (empty if available).
func CalculateChoiceErrors(choice shared.Choice, p *player.Player, g *game.Game, cardRegistry gamecards.CardRegistry) []player.StateError {
	var errors []player.StateError
	errors = append(errors, modeBehaviorErrors("", shared.CardBehavior{Outputs: choice.Outputs}, p, g, cardRegistry, nil)...)
	if !g.ModeRequirementsMet(choice.StateRequirements) {
		errors = append(errors, player.StateError{Code: player.ErrorCodeInvalidRequirement, Category: player.ErrorCategoryRequirement, Message: "Mode state requirement not met"})
	}

	if choice.Requirements != nil && len(choice.Requirements.Items) > 0 {
		if err := gamecards.ValidateChoiceTagRequirements(choice.Requirements.Items, gamecards.PlayerTagCounts(p, cardRegistry)); err != nil {
			errors = append(errors, player.StateError{Code: player.ErrorCodeInsufficientTags, Category: player.ErrorCategoryRequirement, Message: "Tag requirements not met"})
		}

		for _, req := range choice.Requirements.Items {
			if err := checkChoiceRequirement(req, p, g, cardRegistry); err != nil {
				errors = append(errors, *err)
			}
		}
	}

	resources := p.Resources().Get()
	for _, outputBC := range choice.Outputs {
		if shared.IsVariableAmount(outputBC) || outputBC.GetAmount() >= 0 {
			continue
		}
		if !isBasicPlayerResource(outputBC.GetResourceType()) {
			continue
		}
		available := resources.GetAmount(outputBC.GetResourceType())
		if available < -outputBC.GetAmount() {
			errors = append(errors, player.StateError{
				Code:     player.ErrorCodeInsufficientResources,
				Category: player.ErrorCategoryInput,
				Message:  fmt.Sprintf("Not enough %s", outputBC.GetResourceType()),
			})
		}
	}

	for _, inputBC := range choice.Inputs {
		if shared.IsVariableAmount(inputBC) || inputBC.GetAmount() <= 0 {
			continue
		}
		if !isBasicPlayerResource(inputBC.GetResourceType()) {
			continue
		}
		available := resources.GetAmount(inputBC.GetResourceType())
		if available < inputBC.GetAmount() {
			errors = append(errors, player.StateError{
				Code:     player.ErrorCodeInsufficientResources,
				Category: player.ErrorCategoryInput,
				Message:  fmt.Sprintf("Not enough %s", inputBC.GetResourceType()),
			})
		}
	}

	return errors
}

// checkChoiceRequirement validates a single choice requirement and returns a StateError if not met.
func checkChoiceRequirement(req shared.ChoiceRequirement, p *player.Player, g *game.Game, cardRegistry gamecards.CardRegistry) *player.StateError {
	switch req.Type {
	case "tags":
		return nil

	case "temperature":
		temp := g.GlobalParameters().Temperature()
		if req.Min != nil && temp < *req.Min {
			return &player.StateError{
				Code:     player.ErrorCodeTemperatureTooLow,
				Category: player.ErrorCategoryRequirement,
				Message:  "Temperature too low",
			}
		}
		if req.Max != nil && temp > *req.Max {
			return &player.StateError{
				Code:     player.ErrorCodeTemperatureTooHigh,
				Category: player.ErrorCategoryRequirement,
				Message:  "Temperature too high",
			}
		}

	case "oxygen":
		oxygen := g.GlobalParameters().Oxygen()
		if req.Min != nil && oxygen < *req.Min {
			return &player.StateError{
				Code:     player.ErrorCodeOxygenTooLow,
				Category: player.ErrorCategoryRequirement,
				Message:  "Oxygen too low",
			}
		}
		if req.Max != nil && oxygen > *req.Max {
			return &player.StateError{
				Code:     player.ErrorCodeOxygenTooHigh,
				Category: player.ErrorCategoryRequirement,
				Message:  "Oxygen too high",
			}
		}

	case "ocean":
		oceans := g.GlobalParameters().Oceans()
		if req.Min != nil && oceans < *req.Min {
			return &player.StateError{
				Code:     player.ErrorCodeOceansTooLow,
				Category: player.ErrorCategoryRequirement,
				Message:  "Too few oceans",
			}
		}
		if req.Max != nil && oceans > *req.Max {
			return &player.StateError{
				Code:     player.ErrorCodeOceansTooHigh,
				Category: player.ErrorCategoryRequirement,
				Message:  "Too many oceans",
			}
		}

	case "tr":
		tr := p.Resources().TerraformRating()
		if req.Min != nil && tr < *req.Min {
			return &player.StateError{
				Code:     player.ErrorCodeTRTooLow,
				Category: player.ErrorCategoryRequirement,
				Message:  "TR too low",
			}
		}
		if req.Max != nil && tr > *req.Max {
			return &player.StateError{
				Code:     player.ErrorCodeTRTooHigh,
				Category: player.ErrorCategoryRequirement,
				Message:  "TR too high",
			}
		}

	case "production":
		if req.Resource == nil {
			return &player.StateError{
				Code:     player.ErrorCodeInvalidRequirement,
				Category: player.ErrorCategoryRequirement,
				Message:  "Invalid production requirement",
			}
		}
		production := p.Resources().Production()
		amount := production.GetAmount(*req.Resource)
		if req.Min != nil && amount < *req.Min {
			return &player.StateError{
				Code:     player.ErrorCodeInsufficientProduction,
				Category: player.ErrorCategoryRequirement,
				Message:  formatInsufficientProductionMessage(*req.Resource),
			}
		}

	case "resource":
		if req.Resource == nil {
			return &player.StateError{
				Code:     player.ErrorCodeInvalidRequirement,
				Category: player.ErrorCategoryRequirement,
				Message:  "Invalid resource requirement",
			}
		}
		resources := p.Resources().Get()
		amount := resources.GetAmount(*req.Resource)
		if req.Min != nil && amount < *req.Min {
			return &player.StateError{
				Code:     player.ErrorCodeInsufficientResources,
				Category: player.ErrorCategoryRequirement,
				Message:  formatInsufficientResourceMessage(*req.Resource),
			}
		}
	}

	return nil
}
