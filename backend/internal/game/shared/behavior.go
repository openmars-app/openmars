package shared

import "encoding/json"

// ChoicePolicyType identifies the kind of choice policy.
type ChoicePolicyType string

const (
	ChoicePolicyTypeLowest ChoicePolicyType = "lowest"
	ChoicePolicyTypeAuto   ChoicePolicyType = "auto"
)

// ChoicePolicySelect describes an auto-selection rule: pick Option when count satisfies MinMax.
type ChoicePolicySelect struct {
	Option       int      `json:"option"`
	MinMax       MinMax   `json:"minMax"`
	ResourceType string   `json:"resourceType"`
	Tag          *CardTag `json:"tag,omitempty"`
}

// ChoicePolicy governs how choices are filtered or auto-selected for a behavior.
type ChoicePolicy struct {
	Type    ChoicePolicyType    `json:"type"`
	Default *int                `json:"default,omitempty"`
	Select  *ChoicePolicySelect `json:"select,omitempty"`
}

// CardBehavior represents card behaviors (immediate and repeatable)
type CardBehavior struct {
	ID                            string                         `json:"id,omitempty"`
	Optional                      bool                           `json:"optional,omitempty"`
	Dependency                    string                         `json:"dependency,omitempty"`
	Scope                         string                         `json:"scope,omitempty"`
	StateRequirements             []StateRequirement             `json:"stateRequirements,omitempty"`
	MaxUsesPerGame                int                            `json:"maxUsesPerGame,omitempty"`
	UsageGroup                    string                         `json:"usageGroup,omitempty"`
	ResourceRequirements          []BehaviorResourceRequirement  `json:"resourceRequirements,omitempty"`
	MaxUsesPerGeneration          int                            `json:"maxUsesPerGeneration,omitempty"`
	ProductionBox                 string                         `json:"productionBox,omitempty"`
	Description                   string                         `json:"description,omitempty" ts:"string | undefined"`
	Triggers                      []Trigger                      `json:"triggers,omitempty"`
	Inputs                        []BehaviorCondition            `json:"inputs,omitempty"`
	Outputs                       []BehaviorCondition            `json:"outputs,omitempty"`
	Choices                       []Choice                       `json:"choices,omitempty"`
	ChoicePolicy                  *ChoicePolicy                  `json:"choicePolicy,omitempty"`
	GenerationalEventRequirements []GenerationalEventRequirement `json:"generationalEventRequirements,omitempty" ts:"GenerationalEventRequirement[] | undefined"`
	Group                         string                         `json:"group,omitempty" ts:"string | undefined"`
}

// DeepCopy creates a deep copy of the CardBehavior
func (cb CardBehavior) DeepCopy() CardBehavior {
	var result CardBehavior
	result.ID = cb.ID
	result.Optional = cb.Optional
	result.Dependency = cb.Dependency
	result.Scope = cb.Scope
	result.StateRequirements = append([]StateRequirement(nil), cb.StateRequirements...)
	result.MaxUsesPerGeneration = cb.MaxUsesPerGeneration
	result.MaxUsesPerGame = cb.MaxUsesPerGame
	result.UsageGroup = cb.UsageGroup
	result.ResourceRequirements = append([]BehaviorResourceRequirement(nil), cb.ResourceRequirements...)
	for i := range result.ResourceRequirements {
		result.ResourceRequirements[i].Min = clonePointer(result.ResourceRequirements[i].Min)
		result.ResourceRequirements[i].Max = clonePointer(result.ResourceRequirements[i].Max)
	}

	result.Description = cb.Description
	result.Group = cb.Group
	result.ProductionBox = cb.ProductionBox

	if cb.ChoicePolicy != nil {
		cp := *cb.ChoicePolicy
		if cp.Default != nil {
			d := *cp.Default
			cp.Default = &d
		}
		if cp.Select != nil {
			s := *cp.Select
			if s.MinMax.Min != nil {
				v := *s.MinMax.Min
				s.MinMax.Min = &v
			}
			if s.MinMax.Max != nil {
				v := *s.MinMax.Max
				s.MinMax.Max = &v
			}
			if s.Tag != nil {
				t := *s.Tag
				s.Tag = &t
			}
			cp.Select = &s
		}
		result.ChoicePolicy = &cp
	}

	if cb.Triggers != nil {
		result.Triggers = make([]Trigger, len(cb.Triggers))
		for i, trigger := range cb.Triggers {
			result.Triggers[i] = trigger
			if trigger.Condition != nil {
				c := *trigger.Condition
				c.To = clonePointer(c.To)
				c.Target = clonePointer(c.Target)
				c.Location = clonePointer(c.Location)
				c.ResourceTypes = append([]ResourceType(nil), c.ResourceTypes...)
				c.Selectors = CloneSelectors(c.Selectors)
				c.OnBonusType = append([]string(nil), c.OnBonusType...)
				if c.RequiredOriginalCost != nil {
					v := *c.RequiredOriginalCost
					v.Min = clonePointer(v.Min)
					v.Max = clonePointer(v.Max)
					c.RequiredOriginalCost = &v
				}
				result.Triggers[i].Condition = &c
			}
		}
	}

	if cb.Inputs != nil {
		result.Inputs = make([]BehaviorCondition, len(cb.Inputs))
		for i, input := range cb.Inputs {
			result.Inputs[i] = input.deepCopyCondition()
		}
	}

	if cb.Outputs != nil {
		result.Outputs = make([]BehaviorCondition, len(cb.Outputs))
		for i, output := range cb.Outputs {
			result.Outputs[i] = output.deepCopyCondition()
		}
	}

	if cb.Choices != nil {
		result.Choices = make([]Choice, len(cb.Choices))
		for i, choice := range cb.Choices {
			choiceCopy := Choice{StateRequirements: append([]StateRequirement(nil), choice.StateRequirements...)}

			if choice.Inputs != nil {
				choiceCopy.Inputs = make([]BehaviorCondition, len(choice.Inputs))
				for j, input := range choice.Inputs {
					choiceCopy.Inputs[j] = input.deepCopyCondition()
				}
			}

			if choice.Outputs != nil {
				choiceCopy.Outputs = make([]BehaviorCondition, len(choice.Outputs))
				for j, output := range choice.Outputs {
					choiceCopy.Outputs[j] = output.deepCopyCondition()
				}
			}

			if choice.Requirements != nil {
				items := make([]ChoiceRequirement, len(choice.Requirements.Items))
				copy(items, choice.Requirements.Items)
				choiceCopy.Requirements = &ChoiceRequirements{Items: items}
			}

			result.Choices[i] = choiceCopy
		}
	}

	if cb.GenerationalEventRequirements != nil {
		result.GenerationalEventRequirements = make([]GenerationalEventRequirement, len(cb.GenerationalEventRequirements))
		for i, req := range cb.GenerationalEventRequirements {
			result.GenerationalEventRequirements[i] = deepCopyGenerationalEventRequirement(req)
		}
	}

	return result
}

// ExtractInputsOutputs extracts the combined inputs and outputs for a behavior,
// optionally incorporating a selected choice. Returns base + choice inputs/outputs.
// If choiceIndex is nil or out of range, only base inputs/outputs are returned.
func (cb CardBehavior) ExtractInputsOutputs(choiceIndex *int) (inputs []BehaviorCondition, outputs []BehaviorCondition) {
	if len(cb.Inputs) > 0 {
		inputs = make([]BehaviorCondition, len(cb.Inputs))
		copy(inputs, cb.Inputs)
	}
	if len(cb.Outputs) > 0 {
		outputs = make([]BehaviorCondition, len(cb.Outputs))
		copy(outputs, cb.Outputs)
	}

	if choiceIndex != nil && *choiceIndex >= 0 && *choiceIndex < len(cb.Choices) {
		selectedChoice := cb.Choices[*choiceIndex]

		// Append choice inputs to base inputs
		if len(selectedChoice.Inputs) > 0 {
			inputs = append(inputs, selectedChoice.Inputs...)
		}

		// Append choice outputs to base outputs
		if len(selectedChoice.Outputs) > 0 {
			outputs = append(outputs, selectedChoice.Outputs...)
		}
	}

	return inputs, outputs
}

// productionResourceTypes lists all production resource types for policy evaluation.
var productionResourceTypes = []ResourceType{
	ResourceCreditProduction,
	ResourceSteelProduction,
	ResourceTitaniumProduction,
	ResourcePlantProduction,
	ResourceEnergyProduction,
	ResourceHeatProduction,
}

// FilterChoiceIndicesByPolicy returns the indices of choices that are valid under the given policy.
// For nil policy, all indices are returned.
// For "lowest", only choices whose production output is at the player's minimum production level.
func FilterChoiceIndicesByPolicy(choices []Choice, policy *ChoicePolicy, production Production) []int {
	if policy == nil {
		return allChoiceIndices(choices)
	}

	switch policy.Type {
	case ChoicePolicyTypeLowest:
		return filterLowestProductionChoices(choices, production)
	case ChoicePolicyTypeAuto:
		return allChoiceIndices(choices)
	default:
		return allChoiceIndices(choices)
	}
}

// IsChoiceValidForPolicy checks whether a specific choice index is valid under the given policy.
func IsChoiceValidForPolicy(choiceIndex int, choices []Choice, policy *ChoicePolicy, production Production) bool {
	if policy == nil || choiceIndex < 0 || choiceIndex >= len(choices) {
		return choiceIndex >= 0 && choiceIndex < len(choices)
	}

	validIndices := FilterChoiceIndicesByPolicy(choices, policy, production)
	for _, idx := range validIndices {
		if idx == choiceIndex {
			return true
		}
	}
	return false
}

func allChoiceIndices(choices []Choice) []int {
	indices := make([]int, len(choices))
	for i := range choices {
		indices[i] = i
	}
	return indices
}

// AutoSelectChoiceIndex returns an auto-selected choice index for "auto" policies.
// The count parameter is the resolved count for the policy's resource/tag type.
// Returns -1 if the policy is nil or not an auto policy.
func AutoSelectChoiceIndex(policy *ChoicePolicy, count int) int {
	if policy == nil || policy.Type != ChoicePolicyTypeAuto || policy.Select == nil {
		return -1
	}
	sel := policy.Select
	if sel.MinMax.Min != nil && count >= *sel.MinMax.Min {
		return sel.Option
	}
	if sel.MinMax.Max != nil && count <= *sel.MinMax.Max {
		return sel.Option
	}
	if policy.Default != nil {
		return *policy.Default
	}
	return -1
}

func filterLowestProductionChoices(choices []Choice, production Production) []int {
	// Find the minimum production value across all 6 types
	minValue := production.Credits
	for _, rt := range productionResourceTypes {
		val := production.GetAmount(rt)
		if val < minValue {
			minValue = val
		}
	}

	// Return indices of choices whose production output type is at the minimum level
	var valid []int
	for i, choice := range choices {
		for _, output := range choice.Outputs {
			rt := output.GetResourceType()
			if IsProductionResourceType(rt) && production.GetAmount(rt) == minValue {
				valid = append(valid, i)
				break
			}
		}
	}
	return valid
}

// IsProductionResourceType returns true if the resource type is a production type.
func IsProductionResourceType(rt ResourceType) bool {
	switch rt {
	case ResourceCreditProduction, ResourceSteelProduction, ResourceTitaniumProduction,
		ResourcePlantProduction, ResourceEnergyProduction, ResourceHeatProduction:
		return true
	default:
		return false
	}
}

// UnmarshalJSON implements custom JSON unmarshaling for CardBehavior.
// This is needed because []BehaviorCondition (interface slices) cannot be
// directly unmarshaled. We unmarshal into []resourceConditionJSON first, then
// convert to []BehaviorCondition with pointer elements.
func (cb *CardBehavior) UnmarshalJSON(data []byte) error {
	type cardBehaviorJSON struct {
		ID                            string                         `json:"id,omitempty"`
		Optional                      bool                           `json:"optional,omitempty"`
		Dependency                    string                         `json:"dependency,omitempty"`
		Scope                         string                         `json:"scope,omitempty"`
		StateRequirements             []StateRequirement             `json:"stateRequirements,omitempty"`
		MaxUsesPerGame                int                            `json:"maxUsesPerGame,omitempty"`
		UsageGroup                    string                         `json:"usageGroup,omitempty"`
		ResourceRequirements          []BehaviorResourceRequirement  `json:"resourceRequirements,omitempty"`
		MaxUsesPerGeneration          int                            `json:"maxUsesPerGeneration,omitempty"`
		ProductionBox                 string                         `json:"productionBox,omitempty"`
		Description                   string                         `json:"description,omitempty"`
		Triggers                      []Trigger                      `json:"triggers,omitempty"`
		Inputs                        []resourceConditionJSON        `json:"inputs,omitempty"`
		Outputs                       []resourceConditionJSON        `json:"outputs,omitempty"`
		Choices                       []choiceJSON                   `json:"choices,omitempty"`
		ChoicePolicy                  *ChoicePolicy                  `json:"choicePolicy,omitempty"`
		GenerationalEventRequirements []GenerationalEventRequirement `json:"generationalEventRequirements,omitempty"`
		Group                         string                         `json:"group,omitempty"`
	}
	var raw cardBehaviorJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	cb.ID = raw.ID
	cb.Optional = raw.Optional
	cb.Dependency = raw.Dependency
	cb.Scope = raw.Scope
	cb.StateRequirements = raw.StateRequirements
	cb.MaxUsesPerGeneration = raw.MaxUsesPerGeneration
	cb.MaxUsesPerGame = raw.MaxUsesPerGame
	cb.UsageGroup = raw.UsageGroup
	cb.ResourceRequirements = raw.ResourceRequirements
	cb.Description = raw.Description
	cb.Triggers = raw.Triggers
	cb.ChoicePolicy = raw.ChoicePolicy
	cb.GenerationalEventRequirements = raw.GenerationalEventRequirements
	cb.Group = raw.Group
	cb.ProductionBox = raw.ProductionBox
	cb.Inputs = resourceConditionsToInterface(raw.Inputs)
	cb.Outputs = resourceConditionsToInterface(raw.Outputs)
	cb.Choices = make([]Choice, len(raw.Choices))
	for i, c := range raw.Choices {
		cb.Choices[i] = Choice{
			Inputs:            resourceConditionsToInterface(c.Inputs),
			Outputs:           resourceConditionsToInterface(c.Outputs),
			Requirements:      c.Requirements,
			StateRequirements: c.StateRequirements,
		}
	}
	return nil
}

// choiceJSON is the JSON deserialization format for Choice (uses flat resourceConditionJSON).
type choiceJSON struct {
	StateRequirements []StateRequirement      `json:"stateRequirements,omitempty"`
	Inputs            []resourceConditionJSON `json:"inputs,omitempty"`
	Outputs           []resourceConditionJSON `json:"outputs,omitempty"`
	Requirements      *ChoiceRequirements     `json:"requirements,omitempty"`
}

// resourceConditionsToInterface converts a flat []resourceConditionJSON to []BehaviorCondition.
func resourceConditionsToInterface(rcs []resourceConditionJSON) []BehaviorCondition {
	if len(rcs) == 0 {
		return nil
	}
	result := make([]BehaviorCondition, len(rcs))
	for i := range rcs {
		result[i] = categorizeCondition(rcs[i])
	}
	return result
}

func deepCopyGenerationalEventRequirement(req GenerationalEventRequirement) GenerationalEventRequirement {
	result := GenerationalEventRequirement{
		Event: req.Event,
	}

	if req.Count != nil {
		countCopy := MinMax{}
		if req.Count.Min != nil {
			minCopy := *req.Count.Min
			countCopy.Min = &minCopy
		}
		if req.Count.Max != nil {
			maxCopy := *req.Count.Max
			countCopy.Max = &maxCopy
		}
		result.Count = &countCopy
	}

	if req.Target != nil {
		targetCopy := *req.Target
		result.Target = &targetCopy
	}

	return result
}
