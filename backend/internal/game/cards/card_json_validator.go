package cards

import (
	"fmt"
	"strings"

	"openmars/internal/game/shared"
)

// ValidateCardJSON validates the JSON structure of a card at load time
// This ensures all enum values are valid and structure is correct
func ValidateCardJSON(card *Card) []error {
	for _, b := range card.Behaviors {
		for _, o := range b.Outputs {
			if c, ok := o.(*shared.PaymentSubstituteCondition); ok && c.Source.Target == "self-card" && (card.ResourceStorage == nil || card.ResourceStorage.Type != c.Source.Resource) {
				return []error{fmt.Errorf("card %s: payment source does not match card storage", card.ID)}
			}
		}
	}
	var errors []error

	if !isValidCardType(card.Type) {
		errors = append(errors, fmt.Errorf("card %s: invalid card type: %s", card.ID, card.Type))
	}

	for i, section := range card.Description {
		if !section.Type.IsValid() {
			errors = append(errors, fmt.Errorf("card %s: description section %d: invalid type: %s", card.ID, i, section.Type))
		}
		if strings.TrimSpace(section.Text) == "" {
			errors = append(errors, fmt.Errorf("card %s: description section %d: empty text", card.ID, i))
		}
	}

	for _, tag := range card.Tags {
		if !isValidCardTag(tag) {
			errors = append(errors, fmt.Errorf("card %s: invalid tag: %s", card.ID, tag))
		}
	}

	if card.Requirements != nil {
		for i, req := range card.Requirements.Items {
			if reqErr := validateRequirement(card.ID, i, req); reqErr != nil {
				errors = append(errors, reqErr)
			}
		}
	}

	for i, behavior := range card.Behaviors {
		if behavior.Optional && (i != len(card.Behaviors)-1 || !HasAutoTrigger(behavior) || len(behavior.Triggers) != 1 || behavior.Triggers[0].Condition != nil || len(behavior.Choices) > 0 || behavior.ChoicePolicy != nil || behavior.ID == "") {
			errors = append(errors, fmt.Errorf("card %s: optional effects must be terminal auto behaviors without choices", card.ID))
		}
		behaviorErrors := validateBehavior(card.ID, i, behavior)
		errors = append(errors, behaviorErrors...)
	}

	if card.ResourceStorage != nil {
		if !isValidResourceType(card.ResourceStorage.Type) {
			errors = append(errors, fmt.Errorf("card %s: invalid resource storage type: %s", card.ID, card.ResourceStorage.Type))
		}
	}

	for i, vp := range card.VPConditions {
		if vpErr := validateVictoryPointCondition(card.ID, i, vp); vpErr != nil {
			errors = append(errors, vpErr)
		}
	}

	if card.StartingResources != nil {
		if rsErr := validateResourceSet(card.ID, "starting resources", *card.StartingResources); rsErr != nil {
			errors = append(errors, rsErr)
		}
	}

	if card.StartingProduction != nil {
		if prodErr := validateResourceSet(card.ID, "starting production", *card.StartingProduction); prodErr != nil {
			errors = append(errors, prodErr)
		}
	}

	return errors
}

func validateRequirement(cardID string, index int, req Requirement) error {
	if !isValidRequirementType(req.Type) {
		return fmt.Errorf("card %s: requirement[%d] has invalid type: %s", cardID, index, req.Type)
	}

	if req.Target != "" && !((req.Type == RequirementCities && req.Target == "self-player") || (req.Type == RequirementColony && (req.Target == "self-player" || req.Target == "any-player"))) {
		return fmt.Errorf("card %s: requirement[%d] has invalid target: %s", cardID, index, req.Target)
	}

	if req.Type == RequirementTags && req.Tag != nil {
		if !isValidCardTag(*req.Tag) {
			return fmt.Errorf("card %s: requirement[%d] has invalid tag: %s", cardID, index, *req.Tag)
		}
	}

	return nil
}

func validateBehavior(cardID string, index int, behavior shared.CardBehavior) []error {
	var errors []error
	for _, r := range behavior.ResourceRequirements {
		if r.Type == "" || r.Min == nil && r.Max == nil || r.Min != nil && r.Max != nil && *r.Min > *r.Max || r.Permanent && !shared.IsProductionResourceType(r.Type) {
			errors = append(errors, fmt.Errorf("card %s: invalid resource requirement", cardID))
		}
	}
	contextualTrade := false
	for _, trigger := range behavior.Triggers {
		if trigger.Type == shared.TriggerTypeAuto && trigger.Condition != nil && trigger.Condition.Type == "before-colony-trade" {
			contextualTrade = true
			if trigger.Condition.Target == nil || *trigger.Condition.Target != "self-player" || len(trigger.Condition.Selectors) > 0 || len(trigger.Condition.ResourceTypes) > 0 || trigger.Condition.Location != nil || trigger.Condition.RequiredOriginalCost != nil || len(trigger.Condition.OnBonusType) > 0 || trigger.Condition.Unique {
				errors = append(errors, fmt.Errorf("card %s: unsupported before-colony-trade trigger", cardID))
			}
		}
	}
	conditions := append([]shared.BehaviorCondition(nil), behavior.Outputs...)
	conditions = append(conditions, behavior.Inputs...)
	for _, choice := range behavior.Choices {
		conditions = append(conditions, choice.Outputs...)
		conditions = append(conditions, choice.Inputs...)
	}
	for _, condition := range conditions {
		if condition.GetTarget() == "trigger-colony" && !contextualTrade {
			errors = append(errors, fmt.Errorf("card %s: trigger-colony requires before-colony-trade", cardID))
		}
		if contextualTrade {
			if condition.GetResourceType() != shared.ResourceColonyTrackStep || condition.GetTarget() != "trigger-colony" || condition.GetAmount() <= 0 {
				errors = append(errors, fmt.Errorf("card %s: before-colony-trade requires positive contextual track outputs", cardID))
			}
		}
	}
	if contextualTrade && (len(behavior.Triggers) != 1 || len(behavior.Inputs) > 0 || len(behavior.Choices) > 0 || len(behavior.Outputs) == 0) {
		errors = append(errors, fmt.Errorf("card %s: before-colony-trade must be a separate output behavior", cardID))
	}

	if behavior.ProductionBox != "" {
		if behavior.ProductionBox != "evaluate" && behavior.ProductionBox != "resolved" {
			errors = append(errors, fmt.Errorf("card %s: unknown productionBox %q", cardID, behavior.ProductionBox))
		}
		if behavior.ProductionBox == "evaluate" && (len(behavior.Inputs) > 0 || len(behavior.Choices) > 0 || len(behavior.Outputs) == 0) {
			errors = append(errors, fmt.Errorf("card %s: production box must contain only production outputs", cardID))
		}
		for _, output := range behavior.Outputs {
			if behavior.ProductionBox == "evaluate" && !shared.IsProductionResourceType(output.GetResourceType()) {
				errors = append(errors, fmt.Errorf("card %s: production box contains non-production output", cardID))
			}
		}
		for _, trigger := range behavior.Triggers {
			if trigger.Type == shared.TriggerTypeManual || (behavior.ProductionBox == "evaluate" && trigger.Condition != nil) {
				errors = append(errors, fmt.Errorf("card %s: cannot copy recurring production as a production box", cardID))
			}
		}
	}

	for i, trigger := range behavior.Triggers {
		if trigger.Type == "" {
			errors = append(errors, fmt.Errorf("card %s: behavior[%d].trigger[%d] has empty type", cardID, index, i))
		}

		if trigger.Condition != nil {
			for _, rt := range trigger.Condition.ResourceTypes {
				if !isValidResourceType(rt) {
					errors = append(errors, fmt.Errorf("card %s: behavior[%d].trigger[%d] has invalid resource type: %s", cardID, index, i, rt))
				}
			}
		}
	}

	for i, input := range behavior.Inputs {
		if inputErr := validateBehaviorCondition(cardID, index, "input", i, input); inputErr != nil {
			errors = append(errors, inputErr)
		}
	}

	for i, output := range behavior.Outputs {
		if outputErr := validateBehaviorCondition(cardID, index, "output", i, output); outputErr != nil {
			errors = append(errors, outputErr)
		}
	}

	for i, choice := range behavior.Choices {
		for k, input := range choice.Inputs {
			if inputErr := validateBehaviorCondition(cardID, index, fmt.Sprintf("choice[%d].input", i), k, input); inputErr != nil {
				errors = append(errors, inputErr)
			}
		}

		for k, output := range choice.Outputs {
			if outputErr := validateBehaviorCondition(cardID, index, fmt.Sprintf("choice[%d].output", i), k, output); outputErr != nil {
				errors = append(errors, outputErr)
			}
		}
	}

	return errors
}

func validateBehaviorCondition(cardID string, behaviorIndex int, condType string, index int, cond shared.BehaviorCondition) error {
	if c, ok := cond.(*shared.StateCondition); ok {
		if problems := shared.ValidateResourceCondition(c, strings.HasSuffix(condType, "input")); len(problems) > 0 {
			return fmt.Errorf("card %s: %v", cardID, problems)
		}
	}
	if c, ok := cond.(*shared.PaymentSubstituteCondition); ok {
		if problems := shared.ValidateResourceCondition(c, strings.HasSuffix(condType, "input")); len(problems) > 0 {
			return fmt.Errorf("card %s: %v", cardID, problems)
		}
	}
	if reveal, ok := cond.(*shared.CardRevealCondition); ok {
		if problems := shared.ValidateResourceCondition(reveal, strings.HasSuffix(condType, "input")); len(problems) > 0 {
			return fmt.Errorf("card %s: %s", cardID, strings.Join(problems, "; "))
		}
	}
	if cond.GetTarget() == "" {
		return fmt.Errorf("card %s: behavior[%d].%s[%d] has empty target", cardID, behaviorIndex, condType, index)
	}

	if !isValidResourceType(cond.GetResourceType()) {
		return fmt.Errorf("card %s: behavior[%d].%s[%d] has invalid resource type: %s", cardID, behaviorIndex, condType, index, cond.GetResourceType())
	}

	if per := shared.GetPerCondition(cond); per != nil {
		if per.Amount <= 0 {
			return fmt.Errorf("card %s: per amount must be positive", cardID)
		}
		if per.Zone != "" && per.Zone != "played" {
			return fmt.Errorf("card %s: unsupported count zone %q", cardID, per.Zone)
		}
		if (per.Zone != "" || len(per.Selectors) > 0) && per.ResourceType != shared.ResourceCardCount {
			return fmt.Errorf("card %s: zone/selectors require card-count", cardID)
		}
		if !isValidResourceType(per.ResourceType) {
			return fmt.Errorf("card %s: behavior[%d].%s[%d].per has invalid resource type: %s", cardID, behaviorIndex, condType, index, per.ResourceType)
		}
	}

	return nil
}

func validateVictoryPointCondition(cardID string, index int, vp VictoryPointCondition) error {
	if vp.Per != nil {
		if !isValidResourceType(vp.Per.ResourceType) {
			return fmt.Errorf("card %s: victory_point_condition[%d].per has invalid resource type: %s", cardID, index, vp.Per.ResourceType)
		}
	}

	return nil
}

func validateResourceSet(cardID, fieldName string, rs shared.ResourceSet) error {
	return nil
}

func isValidCardType(ct CardType) bool {
	switch ct {
	case CardTypeCorporation, CardTypeAutomated, CardTypeActive, CardTypeEvent, CardTypePrelude:
		return true
	default:
		return false
	}
}

func isValidCardTag(tag shared.CardTag) bool {
	switch tag {
	case shared.TagBuilding, shared.TagSpace, shared.TagScience,
		shared.TagPower, shared.TagEarth, shared.TagJovian,
		shared.TagVenus, shared.TagPlant, shared.TagMicrobe,
		shared.TagAnimal, shared.TagCity, shared.TagEvent,
		shared.TagWildlife, shared.TagWild:
		return true
	default:
		return false
	}
}

func isValidRequirementType(rt RequirementType) bool {
	switch rt {
	case RequirementState, RequirementTemperature, RequirementOxygen, RequirementOceans,
		RequirementTags, RequirementProduction, RequirementTR,
		RequirementResource, RequirementVenus, RequirementCities,
		RequirementGreeneries, RequirementColony:
		return true
	default:
		return false
	}
}

func isValidResourceType(rt shared.ResourceType) bool {
	return rt != ""
}
