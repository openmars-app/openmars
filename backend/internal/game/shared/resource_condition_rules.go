package shared

import "fmt"

// FieldProfile declares which optional fields are valid for a ResourceType category.
// Amount and ResourceType are always valid and omitted from the profile.
type FieldProfile struct {
	AllowCopy                       bool
	AllowSelectionGroup             bool
	AllowTarget                     bool
	AllowSelectors                  bool
	AllowMaxTrigger                 bool
	AllowPer                        bool
	AllowTileRestrictions           bool
	AllowTileType                   bool
	AllowVariableAmount             bool
	AllowTemporary                  bool
	AllowOptional                   bool
	AllowPaymentAllowed             bool
	AllowTargetRestriction          bool
	AllowAllowDuplicatePlayerColony bool
}

// Category-level output profiles

var basicResourceOutputProfile = FieldProfile{
	AllowTarget:            true,
	AllowPer:               true,
	AllowVariableAmount:    true,
	AllowMaxTrigger:        true,
	AllowTargetRestriction: true,
}

var basicResourceInputProfile = FieldProfile{
	AllowTarget:         true,
	AllowVariableAmount: true,
	AllowOptional:       true,
}

var creditInputProfile = FieldProfile{
	AllowTarget:         true,
	AllowPaymentAllowed: true,
}

var cardStorageOutputProfile = FieldProfile{
	AllowTarget:    true,
	AllowSelectors: true,
	AllowPer:       true,
}

var cardStorageInputProfile = FieldProfile{
	AllowSelectors:      true,
	AllowTarget:         true,
	AllowVariableAmount: true,
}

var productionOutputProfile = FieldProfile{
	AllowTarget:         true,
	AllowPer:            true,
	AllowVariableAmount: true,
}

var productionInputProfile = FieldProfile{
	AllowTarget: true,
}

var tilePlacementOutputProfile = FieldProfile{
	AllowTarget:           true,
	AllowTileRestrictions: true,
}

var genericTilePlacementOutputProfile = FieldProfile{
	AllowTarget:           true,
	AllowTileRestrictions: true,
	AllowTileType:         true,
}

var globalParameterOutputProfile = FieldProfile{
	AllowTarget: true,
}

var cardOperationOutputProfile = FieldProfile{
	AllowTarget:    true,
	AllowSelectors: true,
}

var discountOutputProfile = FieldProfile{
	AllowTarget:    true,
	AllowSelectors: true,
	AllowTemporary: true,
}

var paymentSubstituteOutputProfile = FieldProfile{
	AllowTarget:    true,
	AllowSelectors: true,
}

var valueModifierOutputProfile = FieldProfile{
	AllowTarget:    true,
	AllowSelectors: true,
}

var effectOutputProfile = FieldProfile{
	AllowTarget:    true,
	AllowSelectors: true,
	AllowTemporary: true,
}

var colonyTileOutputProfile = FieldProfile{
	AllowTarget:                     true,
	AllowAllowDuplicatePlayerColony: true,
}

var tileReplacementOutputProfile = FieldProfile{
	AllowTarget:   true,
	AllowTileType: true,
}

var bonusTagsOutputProfile = FieldProfile{
	AllowTarget:    true,
	AllowPer:       true,
	AllowSelectors: true,
}

var targetOnlyOutputProfile = FieldProfile{
	AllowTarget: true,
}

var emptyProfile = FieldProfile{}

// outputFieldProfiles maps each ResourceType to its valid field profile for outputs.
var outputFieldProfiles = map[ResourceType]FieldProfile{
	ResourceStateChange: {AllowTarget: true}, ResourceRemoveDependency: {AllowTarget: true}, ResourceRetireDependency: {AllowTarget: true}, ResourceScheduleStateWindow: {AllowTarget: true},
	ResourceCopy: {AllowTarget: true, AllowSelectors: true, AllowCopy: true},
	// Basic resources
	ResourceCredit:   basicResourceOutputProfile,
	ResourceSteel:    basicResourceOutputProfile,
	ResourceTitanium: basicResourceOutputProfile,
	ResourcePlant:    basicResourceOutputProfile,
	ResourceEnergy:   basicResourceOutputProfile,
	ResourceHeat:     basicResourceOutputProfile,

	// Card storage resources
	ResourceMicrobe:      cardStorageOutputProfile,
	ResourceAnimal:       cardStorageOutputProfile,
	ResourceFloater:      cardStorageOutputProfile,
	ResourceScience:      cardStorageOutputProfile,
	ResourceAsteroid:     cardStorageOutputProfile,
	ResourceFighter:      cardStorageOutputProfile,
	ResourceDisease:      cardStorageOutputProfile,
	ResourceCamp:         cardStorageOutputProfile,
	ResourceCardResource: cardStorageOutputProfile,

	// Card operations
	ResourceCardPlay:    {AllowTarget: true, AllowSelectors: true},
	ResourceCardDraw:    {AllowTarget: true, AllowSelectors: true, AllowPer: true},
	ResourceCardReveal:  {AllowTarget: true},
	ResourceCardTake:    {AllowTarget: true, AllowSelectors: true, AllowOptional: true, AllowVariableAmount: true},
	ResourceCardPeek:    cardOperationOutputProfile,
	ResourceCardBuy:     cardOperationOutputProfile,
	ResourceCardDiscard: cardOperationOutputProfile,

	// Tile placements
	ResourceCityPlacement:     tilePlacementOutputProfile,
	ResourceOceanPlacement:    tilePlacementOutputProfile,
	ResourceGreeneryPlacement: tilePlacementOutputProfile,
	ResourceVolcanoPlacement:  tilePlacementOutputProfile,
	ResourceTilePlacement:     genericTilePlacementOutputProfile,

	// Tile count types (used in PerCondition, not directly as conditions)
	ResourceCityTile:               emptyProfile,
	ResourceOceanTile:              emptyProfile,
	ResourceGreeneryTile:           emptyProfile,
	ResourceVolcanoTile:            emptyProfile,
	ResourceNaturalPreserveTile:    emptyProfile,
	ResourceMiningTile:             emptyProfile,
	ResourceNuclearZoneTile:        emptyProfile,
	ResourceEcologicalZoneTile:     emptyProfile,
	ResourceMoholeTile:             emptyProfile,
	ResourceRestrictedTile:         emptyProfile,
	ResourceCommercialDistrictTile: emptyProfile,
	ResourceIndustrialCenterTile:   emptyProfile,
	ResourceLandTile:               emptyProfile,
	ResourceNonOceanTile:           emptyProfile,
	ResourceOceanSpace:             emptyProfile,

	// Colony
	ResourceColony:        colonyTileOutputProfile,
	ResourceColonyTileAdd: targetOnlyOutputProfile,
	ResourceColonyCount:   targetOnlyOutputProfile,
	ResourceColonyBonus:   targetOnlyOutputProfile,

	// Global parameters
	ResourceTemperature:     globalParameterOutputProfile,
	ResourceOxygen:          globalParameterOutputProfile,
	ResourceOcean:           globalParameterOutputProfile,
	ResourceVenus:           globalParameterOutputProfile,
	ResourceTR:              {AllowTarget: true, AllowPer: true},
	ResourceGlobalParameter: globalParameterOutputProfile,

	// Production
	ResourceCreditProduction:   productionOutputProfile,
	ResourceSteelProduction:    productionOutputProfile,
	ResourceTitaniumProduction: productionOutputProfile,
	ResourcePlantProduction:    productionOutputProfile,
	ResourceEnergyProduction:   productionOutputProfile,
	ResourceHeatProduction:     productionOutputProfile,
	ResourceAnyProduction:      productionOutputProfile,

	// Effects
	ResourceEffect:            effectOutputProfile,
	ResourceTag:               effectOutputProfile,
	ResourceDiscount:          discountOutputProfile,
	ResourcePaymentSubstitute: paymentSubstituteOutputProfile,

	ResourceValueModifier:            valueModifierOutputProfile,
	ResourceGlobalParameterLenience:  effectOutputProfile,
	ResourceIgnoreGlobalRequirements: effectOutputProfile,
	ResourceOceanAdjacencyBonus:      effectOutputProfile,
	ResourceDefense:                  effectOutputProfile,
	ResourceActionReuse:              targetOnlyOutputProfile,

	// Special
	ResourceLandClaim:       targetOnlyOutputProfile,
	ResourceExtraActions:    targetOnlyOutputProfile,
	ResourceTileDestruction: targetOnlyOutputProfile,
	ResourceTileReplacement: tileReplacementOutputProfile,
	ResourceBonusTags:       bonusTagsOutputProfile,
	ResourceWorldTreeTile:   targetOnlyOutputProfile,
	ResourceAwardFund:       targetOnlyOutputProfile,
	ResourceFreeTrade:       targetOnlyOutputProfile,
	ResourceCardCount:       emptyProfile,
	ResourceColonyTrackStep: {AllowTarget: true, AllowSelectionGroup: true, AllowOptional: true},
	ResourceTradeFleet:      targetOnlyOutputProfile,
}

// inputFieldProfiles maps each ResourceType to its valid field profile for inputs.
// Most resource types never appear as inputs. Types not in this map are invalid as inputs.
var inputFieldProfiles = map[ResourceType]FieldProfile{
	// Basic resources
	ResourceCredit:   creditInputProfile,
	ResourceSteel:    basicResourceInputProfile,
	ResourceTitanium: basicResourceInputProfile,
	ResourcePlant:    basicResourceInputProfile,
	ResourceEnergy:   basicResourceInputProfile,
	ResourceHeat:     basicResourceInputProfile,

	// Card storage resources (can be spent from cards)
	ResourceMicrobe:  cardStorageInputProfile,
	ResourceAnimal:   cardStorageInputProfile,
	ResourceFloater:  cardStorageInputProfile,
	ResourceScience:  cardStorageInputProfile,
	ResourceAsteroid: cardStorageInputProfile,
	ResourceFighter:  cardStorageInputProfile,
	ResourceCamp:     cardStorageInputProfile,

	// Card operations
	ResourceCardDiscard: {AllowTarget: true, AllowOptional: true, AllowVariableAmount: true},

	// Production (can be reduced)
	ResourceCreditProduction: productionInputProfile,
	ResourceEnergyProduction: productionInputProfile,
}

// ValidTargets is the set of known valid target values.
var validTargets = map[string]bool{
	"game":                true,
	"self-player":         true,
	"self-card":           true,
	"triggering-card":     true,
	"trigger-colony":      true,
	"any-card":            true,
	"any-player":          true,
	"all-opponents":       true,
	"none":                true,
	"steal-any-player":    true,
	"steal-from-any-card": true,
	"":                    true,
}

// AllResourceTypes contains every ResourceType constant for exhaustiveness testing.
var AllResourceTypes = []ResourceType{
	ResourceStateChange, ResourceRemoveDependency, ResourceRetireDependency, ResourceScheduleStateWindow,
	ResourceCopy,
	ResourceCredit, ResourceSteel, ResourceTitanium, ResourcePlant, ResourceEnergy, ResourceHeat,
	ResourceMicrobe, ResourceAnimal, ResourceFloater, ResourceScience, ResourceAsteroid, ResourceFighter, ResourceDisease, ResourceCamp,
	ResourceCardPlay, ResourceCardReveal, ResourceCardDraw, ResourceCardTake, ResourceCardPeek, ResourceCardBuy, ResourceCardDiscard,
	ResourceCityPlacement, ResourceOceanPlacement, ResourceGreeneryPlacement, ResourceVolcanoPlacement, ResourceTilePlacement,
	ResourceCityTile, ResourceOceanTile, ResourceGreeneryTile, ResourceVolcanoTile,
	ResourceColonyTileAdd, ResourceColony, ResourceColonyCount, ResourceColonyBonus,
	ResourceNaturalPreserveTile, ResourceMiningTile, ResourceNuclearZoneTile, ResourceEcologicalZoneTile, ResourceMoholeTile, ResourceRestrictedTile, ResourceCommercialDistrictTile, ResourceIndustrialCenterTile,
	ResourceLandTile, ResourceNonOceanTile, ResourceOceanSpace,
	ResourceTemperature, ResourceOxygen, ResourceOcean, ResourceVenus, ResourceTR, ResourceGlobalParameter,
	ResourceCreditProduction, ResourceSteelProduction, ResourceTitaniumProduction, ResourcePlantProduction, ResourceEnergyProduction, ResourceHeatProduction, ResourceAnyProduction,
	ResourceEffect, ResourceTag,
	ResourceGlobalParameterLenience, ResourceIgnoreGlobalRequirements, ResourceDefense, ResourceDiscount, ResourceValueModifier, ResourcePaymentSubstitute, ResourceOceanAdjacencyBonus,
	ResourceLandClaim, ResourceCardResource, ResourceActionReuse,
	ResourceExtraActions, ResourceTileDestruction, ResourceTileReplacement, ResourceBonusTags,
	ResourceWorldTreeTile, ResourceAwardFund, ResourceFreeTrade, ResourceCardCount,
	ResourceColonyTrackStep, ResourceTradeFleet,
}

// ValidateResourceCondition checks that only valid fields are set for the given ResourceType.
// isInput indicates whether this is an input (true) or output (false).
// Returns a list of violation descriptions (empty = valid).
func ValidateResourceCondition(bc BehaviorCondition, isInput bool) []string {
	if o, ok := bc.(*StateCondition); ok {
		if isInput {
			return []string{"state operations cannot be inputs"}
		}
		switch o.ResourceType {
		case ResourceStateChange:
			if o.State == "" || o.Amount == 0 || o.Target != "game" {
				return []string{"state-change requires state, nonzero amount and game target"}
			}
		case ResourceScheduleStateWindow:
			if o.State == "" || o.Start == nil || o.End == nil || o.Target != "game" {
				return []string{"state window requires state, boundaries and game target"}
			}
		case ResourceRemoveDependency, ResourceRetireDependency:
			if o.Dependency == "" || (o.Target != "any-card" && o.Target != "self-card") {
				return []string{"dependency operation requires dependency and card target"}
			}
		default:
			return []string{"unknown state operation"}
		}
		return nil
	}
	if payment, ok := bc.(*PaymentSubstituteCondition); ok {
		var problems []string
		if isInput || payment.Target != "self-player" || payment.Amount <= 0 {
			problems = append(problems, "payment-substitute requires a positive self-player output")
		}
		if payment.Source.Target != "self-player" && payment.Source.Target != "self-card" {
			problems = append(problems, "invalid payment source target")
		}
		if payment.Source.CardID != "" {
			problems = append(problems, "payment source cardId is resolved at runtime")
		}
		if !IsBasicPaymentResource(payment.TargetResource) {
			problems = append(problems, "invalid payment destination")
		}
		if payment.Source.Target == "self-player" && !IsBasicPaymentResource(payment.Source.Resource) {
			problems = append(problems, "player payment source must be a basic resource")
		}
		if payment.Source.Resource == "" {
			problems = append(problems, "payment source resource is required")
		}
		if payment.Source.Target == "self-card" {
			switch payment.Source.Resource {
			case ResourceMicrobe, ResourceAnimal, ResourceFloater, ResourceScience, ResourceAsteroid, ResourceFighter, ResourceDisease, ResourceCamp:
			default:
				problems = append(problems, "card payment source must be a storage resource")
			}
		}
		for _, s := range payment.Selectors {
			if len(s.Resources) > 0 {
				problems = append(problems, "payment source and destination cannot be encoded in selectors")
			}
		}
		return problems
	}
	rc := flattenCondition(bc)
	var violations []string

	profiles := outputFieldProfiles
	direction := "output"
	if isInput {
		profiles = inputFieldProfiles
		direction = "input"
	}

	profile, ok := profiles[rc.ResourceType]
	if !ok {
		return []string{fmt.Sprintf("resource type %q is not valid as %s", rc.ResourceType, direction)}
	}

	if rc.ResourceType == ResourceCardPlay {
		if rc.Amount != 1 || rc.Target != "self-player" || rc.CostReduction < 0 {
			violations = append(violations, "card-play requires amount 1, self-player target and nonnegative costReduction")
		}
	} else if rc.CostReduction != 0 {
		violations = append(violations, "costReduction is only valid for card-play")
	}
	if rc.ResourceType == ResourceCardReveal {
		if rc.Amount <= 0 || rc.Target != "self-player" || rc.Destination != "discard" {
			violations = append(violations, "card-reveal requires positive amount, self-player target and discard destination")
		}
		if rc.OnMatch != nil {
			if len(rc.OnMatch.Selectors) == 0 || len(rc.OnMatch.Outputs) == 0 {
				violations = append(violations, "onMatch requires selectors and outputs")
			}
			for _, output := range rc.OnMatch.Outputs {
				violations = append(violations, ValidateResourceCondition(output, false)...)
				if !IsImmediateRevealReward(output) {
					violations = append(violations, "onMatch output must be an immediate nonnegative self resource or production reward")
				}
			}
		}
	} else if rc.Destination != "" || rc.OnMatch != nil {
		violations = append(violations, "destination and onMatch are only valid for card-reveal")
	}
	if rc.Target != "" && rc.Target != "none" && !profile.AllowTarget {
		violations = append(violations, fmt.Sprintf("field 'target' (%q) not allowed for %s %q", rc.Target, direction, rc.ResourceType))
	}

	if rc.Target != "" && !validTargets[rc.Target] {
		violations = append(violations, fmt.Sprintf("unknown target value %q", rc.Target))
	}

	if rc.ResourceType == ResourceTradeFleet && (isInput || rc.Amount <= 0 || rc.Target != "self-player") {
		violations = append(violations, "trade-fleet requires a positive self-player output")
	}
	if rc.Target == "trigger-colony" && (isInput || rc.ResourceType != ResourceColonyTrackStep || rc.Amount <= 0 || rc.SelectionGroup != "") {
		violations = append(violations, "trigger-colony requires a positive colony-track-step output without a selection group")
	}
	if rc.ResourceType == ResourceColonyTrackStep && rc.Optional && rc.Target != "trigger-colony" {
		violations = append(violations, "optional colony track movement requires trigger-colony")
	}
	if rc.Target == "triggering-card" {
		_, storage := bc.(*CardStorageCondition)
		if !storage || isInput {
			violations = append(violations, "triggering-card is only supported for card-storage outputs")
		}
	}
	if len(rc.Selectors) > 0 && !profile.AllowSelectors {
		violations = append(violations, fmt.Sprintf("field 'selectors' not allowed for %s %q", direction, rc.ResourceType))
	}

	if (rc.Scope != "" || rc.Zone != "") && !profile.AllowCopy {
		violations = append(violations, "copy fields are only valid for copy outputs")
	}
	if rc.ResourceType == ResourceCopy && (rc.Scope != "production-box" || rc.Zone != "played" || rc.Target != "self-player" || rc.Amount != 1) {
		violations = append(violations, "copy requires scope production-box, zone played, target self-player and amount 1")
	}
	if rc.SelectionGroup != "" && !profile.AllowSelectionGroup {
		violations = append(violations, "selectionGroup is only valid for colony-track-step outputs")
	}
	if rc.MaxTrigger != nil && !profile.AllowMaxTrigger {
		violations = append(violations, fmt.Sprintf("field 'maxTrigger' not allowed for %s %q", direction, rc.ResourceType))
	}
	if rc.MaxTrigger != nil {
		if *rc.MaxTrigger < 0 {
			violations = append(violations, "maxTrigger must be nonnegative")
		}
		if rc.Per == nil || rc.Per.Amount <= 0 {
			violations = append(violations, "maxTrigger requires a positive per amount")
		}
		if rc.VariableAmount {
			violations = append(violations, "maxTrigger cannot be combined with variableAmount")
		}
	}

	if rc.Per != nil && !profile.AllowPer {
		violations = append(violations, fmt.Sprintf("field 'per' not allowed for %s %q", direction, rc.ResourceType))
	}

	if rc.TileRestrictions != nil && rc.TileRestrictions.Area != "" && rc.TileRestrictions.Area != "land" && rc.TileRestrictions.Area != "ocean" {
		violations = append(violations, "area must be land or ocean")
	}
	if rc.TileRestrictions != nil && !profile.AllowTileRestrictions {
		violations = append(violations, fmt.Sprintf("field 'tileRestrictions' not allowed for %s %q", direction, rc.ResourceType))
	}

	if rc.TileType != "" && !profile.AllowTileType {
		violations = append(violations, fmt.Sprintf("field 'tileType' not allowed for %s %q", direction, rc.ResourceType))
	}

	if rc.VariableAmount && !profile.AllowVariableAmount {
		violations = append(violations, fmt.Sprintf("field 'variableAmount' not allowed for %s %q", direction, rc.ResourceType))
	}

	if rc.ResourceType == ResourceDefense {
		if rc.Against != "any-player" && rc.Against != "opponents" {
			violations = append(violations, "defense requires against: any-player or opponents")
		}
		if rc.Target != "self-card" && rc.Target != "self-player" {
			violations = append(violations, "defense requires self-card or self-player target")
		}
		if len(rc.Selectors) == 0 {
			violations = append(violations, "defense requires resource selectors")
		}
		for _, selector := range rc.Selectors {
			if len(selector.Resources) == 0 || len(selector.Tags) > 0 || len(selector.CardTypes) > 0 || selector.TagCount != nil || selector.RequiredOriginalCost != nil || selector.VP != nil || len(selector.StandardProjects) > 0 || len(selector.GlobalParameters) > 0 || len(selector.Actions) > 0 {
				violations = append(violations, "defense selectors must select resources only")
			}
		}
	} else if rc.Against != "" {
		violations = append(violations, "against is only allowed for defense")
	}
	if rc.Temporary != "" && !profile.AllowTemporary {
		violations = append(violations, fmt.Sprintf("field 'temporary' not allowed for %s %q", direction, rc.ResourceType))
	}

	if rc.Optional && !profile.AllowOptional {
		violations = append(violations, fmt.Sprintf("field 'optional' not allowed for %s %q", direction, rc.ResourceType))
	}

	if len(rc.PaymentAllowed) > 0 && !profile.AllowPaymentAllowed {
		violations = append(violations, fmt.Sprintf("field 'paymentAllowed' not allowed for %s %q", direction, rc.ResourceType))
	}

	if rc.TargetRestriction != nil && !profile.AllowTargetRestriction {
		violations = append(violations, fmt.Sprintf("field 'targetRestriction' not allowed for %s %q", direction, rc.ResourceType))
	}

	if rc.AllowDuplicatePlayerColony && !profile.AllowAllowDuplicatePlayerColony {
		violations = append(violations, fmt.Sprintf("field 'allowDuplicatePlayerColony' not allowed for %s %q", direction, rc.ResourceType))
	}

	return violations
}

// GetOutputProfile returns the output FieldProfile for a ResourceType and whether it exists.
func GetOutputProfile(rt ResourceType) (FieldProfile, bool) {
	p, ok := outputFieldProfiles[rt]
	return p, ok
}

// GetInputProfile returns the input FieldProfile for a ResourceType and whether it exists.
func GetInputProfile(rt ResourceType) (FieldProfile, bool) {
	p, ok := inputFieldProfiles[rt]
	return p, ok
}
