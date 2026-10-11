package shared

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Trigger represents when and how an action or effect is activated
type Trigger struct {
	Type      string                    `json:"type"`
	Condition *ResourceTriggerCondition `json:"condition,omitempty"`
}

// TriggerType constants for card behavior triggers
const (
	TriggerTypeAuto   = "auto"   // Automatic trigger (immediate effect when card is played)
	TriggerTypeManual = "manual" // Manual trigger (blue card action, activated by player)
)

// MinMaxValue represents a min/max value constraint
type MinMaxValue struct {
	Min *int `json:"min,omitempty"`
	Max *int `json:"max,omitempty"`
}

// Selector represents matching criteria for cards, resources, or projects.
// Multiple fields within a Selector use AND logic (all must match).
// Multiple Selectors in a slice use OR logic (any match is sufficient).
type Selector struct {
	Dependency           string            `json:"dependency,omitempty"`
	TagCount             *MinMaxValue      `json:"tagCount,omitempty"`
	Tags                 []CardTag         `json:"tags,omitempty"`
	CardTypes            []string          `json:"cardTypes,omitempty"`
	Resources            []string          `json:"resources,omitempty"`
	StandardProjects     []StandardProject `json:"standardProjects,omitempty"`
	RequiredOriginalCost *MinMaxValue      `json:"requiredOriginalCost,omitempty"`
	VP                   *MinMaxValue      `json:"vp,omitempty"`
	GlobalParameters     []string          `json:"globalParameters,omitempty"`
	Actions              []string          `json:"actions,omitempty"`
}

// ResourceTriggerCondition represents what triggers an automatic resource exchange
type ResourceTriggerCondition struct {
	Direction            string         `json:"direction,omitempty"`
	State                string         `json:"state,omitempty"`
	To                   *int           `json:"to,omitempty"`
	Type                 string         `json:"type"`
	ResourceTypes        []ResourceType `json:"resourceTypes,omitempty"`
	Location             *string        `json:"location,omitempty"`
	Selectors            []Selector     `json:"selectors,omitempty"`
	Target               *string        `json:"target,omitempty"`
	RequiredOriginalCost *MinMaxValue   `json:"requiredOriginalCost,omitempty"`
	OnBonusType          []string       `json:"onBonusType,omitempty"`
	Unique               bool           `json:"unique,omitempty"`
}

// TileRestrictions represents restrictions for tile placement
type TileRestrictions struct {
	BoardTags         []string `json:"boardTags,omitempty" ts:"string[]"`
	Adjacency         string   `json:"adjacency,omitempty" ts:"string"`                     // "none" = no adjacent occupied tiles
	Area              string   `json:"area,omitempty"`                                      // Underlying board space: land or ocean; empty uses placement defaults.
	AdjacentToType    string   `json:"adjacentToType,omitempty" ts:"string"`                // "city", "greenery" = must be adjacent to this tile type
	MinAdjacentOfType *int     `json:"minAdjacentOfType,omitempty" ts:"number | undefined"` // min count of adjacent tiles of AdjacentToType
	AdjacentToOwned   bool     `json:"adjacentToOwned,omitempty" ts:"boolean | undefined"`  // must be adjacent to a tile owned by the placing player
	OnBonusType       []string `json:"onBonusType,omitempty" ts:"string[]"`                 // tile must have one of these bonus types (e.g., "steel", "titanium")
}

// Clone returns an independent placement restriction.
func (tr *TileRestrictions) Clone() *TileRestrictions {
	if tr == nil {
		return nil
	}
	result := *tr
	result.BoardTags = append([]string(nil), tr.BoardTags...)
	result.OnBonusType = append([]string(nil), tr.OnBonusType...)
	result.MinAdjacentOfType = clonePointer(tr.MinAdjacentOfType)
	return &result
}

// UnmarshalJSON rejects unknown restrictions instead of silently ignoring placement rules.
func (tr *TileRestrictions) UnmarshalJSON(data []byte) error {
	type plain TileRestrictions
	var value plain
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	if value.Area != "" && value.Area != "land" && value.Area != "ocean" {
		return fmt.Errorf("invalid tile area %q", value.Area)
	}
	*tr = TileRestrictions(value)
	return nil
}

// Temporary effect expiry constants
const (
	TemporaryNextCard      = "next-card"      // Effect expires after the next card is played
	TemporaryGenerationEnd = "generation-end" // Effect expires at end of generation
)

// resourceConditionJSON is the flat JSON deserialization format for card behaviors.
// It is only used internally for JSON unmarshaling, then converted to typed conditions
// via categorizeCondition. All runtime code uses BehaviorCondition and typed category structs.
type resourceConditionJSON struct {
	Description                string               `json:"description,omitempty"`
	MinimumProduction          *ProductionThreshold `json:"minimumProduction,omitempty"`
	DependencyGroup            *DependencyGroup     `json:"dependencyGroup,omitempty"`
	State                      string               `json:"state,omitempty"`
	Value                      int                  `json:"value,omitempty"`
	Dependency                 string               `json:"dependency,omitempty"`
	Start                      *WindowBoundary      `json:"start,omitempty"`
	End                        *WindowBoundary      `json:"end,omitempty"`
	Source                     *PaymentSource       `json:"source,omitempty"`
	TargetResource             ResourceType         `json:"targetResource,omitempty"`
	Destination                string               `json:"destination,omitempty"`
	OnMatch                    *RevealMatch         `json:"onMatch,omitempty"`
	Scope                      string               `json:"scope,omitempty"`
	Zone                       string               `json:"zone,omitempty"`
	SelectionGroup             string               `json:"selectionGroup,omitempty"`
	ResourceType               ResourceType         `json:"type"`
	Amount                     int                  `json:"amount"`
	Target                     string               `json:"target"`
	Selectors                  []Selector           `json:"selectors,omitempty"`
	MaxTrigger                 *int                 `json:"maxTrigger,omitempty"`
	Per                        *PerCondition        `json:"per,omitempty"`
	TileRestrictions           *TileRestrictions    `json:"tileRestrictions,omitempty"`
	TileType                   string               `json:"tileType,omitempty"`
	VariableAmount             bool                 `json:"variableAmount,omitempty"`
	Against                    string               `json:"against,omitempty"`
	Temporary                  string               `json:"temporary,omitempty"`
	Optional                   bool                 `json:"optional,omitempty"`
	PaymentAllowed             []ResourceType       `json:"paymentAllowed,omitempty"`
	TargetRestriction          *TargetRestriction   `json:"targetRestriction,omitempty"`
	AllowDuplicatePlayerColony bool                 `json:"allowDuplicatePlayerColony,omitempty"`
}

// TargetRestriction restricts which players can be targeted by an output.
type TargetRestriction struct {
	Selectors []Selector `json:"selectors,omitempty"`
	Adjacent  string     `json:"adjacent,omitempty" ts:"string"` // "self-card" = only players with tiles adjacent to this card's tile placement
}

// PerCondition represents what to count for conditional resource gains.
// Used by card behaviors, VP conditions, and award quantifiers.
type PerCondition struct {
	Zone               string        `json:"zone,omitempty"`
	Selectors          []Selector    `json:"selectors,omitempty"`
	IncludeSource      bool          `json:"includeSource,omitempty"`
	ResourceType       ResourceType  `json:"type"`
	Amount             int           `json:"amount"`
	Location           *string       `json:"location,omitempty"`
	Target             *string       `json:"target,omitempty"`
	Tag                *CardTag      `json:"tag,omitempty"`
	Tags               []CardTag     `json:"tags,omitempty"`
	AdjacentToTileType *ResourceType `json:"adjacentToTileType,omitempty"`
	AdjacentToSelfTile bool          `json:"adjacentToSelfTile,omitempty"`
	MinRow             *int          `json:"minRow,omitempty"`
	CardTypeFilter     *string       `json:"cardTypeFilter,omitempty"`
	MinCost            *int          `json:"minCost,omitempty"`
}

// ChoiceRequirement represents a requirement that gates whether a choice is available to a player.
// Uses raw string types to avoid circular dependency with cards/ package.
type ChoiceRequirement struct {
	Type     string        `json:"type"`                                             // Same values as cards.RequirementType (e.g. "tags", "temperature")
	Min      *int          `json:"min,omitempty"`                                    // Minimum value
	Max      *int          `json:"max,omitempty"`                                    // Maximum value
	Location *string       `json:"location,omitempty"`                               // Location constraint (e.g. "mars", "anywhere")
	Tag      *CardTag      `json:"tag,omitempty"`                                    // Tag to count (for type "tags")
	Resource *ResourceType `json:"resource,omitempty" ts:"ResourceType | undefined"` // Resource type (for type "production" or "resource")
}

// ChoiceRequirements wraps a list of requirements for a choice option
type ChoiceRequirements struct {
	Items []ChoiceRequirement `json:"items"`
}

// Choice represents a player choice option
type Choice struct {
	StateRequirements []StateRequirement  `json:"stateRequirements,omitempty"`
	Inputs            []BehaviorCondition `json:"inputs,omitempty"`
	Outputs           []BehaviorCondition `json:"outputs,omitempty"`
	Requirements      *ChoiceRequirements `json:"requirements,omitempty"` // If set, choice is only available when requirements are met
}

// Clone returns an independent copy of a count specification.
func (p *PerCondition) Clone() *PerCondition {
	if p == nil {
		return nil
	}
	cp := *p
	cp.Tags = append([]CardTag(nil), p.Tags...)
	cp.Selectors = CloneSelectors(p.Selectors)
	cp.Location = clonePointer(p.Location)
	cp.Target = clonePointer(p.Target)
	cp.Tag = clonePointer(p.Tag)
	cp.AdjacentToTileType = clonePointer(p.AdjacentToTileType)
	cp.MinRow = clonePointer(p.MinRow)
	cp.CardTypeFilter = clonePointer(p.CardTypeFilter)
	cp.MinCost = clonePointer(p.MinCost)
	return &cp
}

func clonePointer[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// CloneSelectors copies selectors and their nested constraints.
func CloneSelectors(selectors []Selector) []Selector {
	if selectors == nil {
		return nil
	}
	result := make([]Selector, len(selectors))
	for i, s := range selectors {
		result[i] = s
		result[i].Tags = append([]CardTag(nil), s.Tags...)
		result[i].CardTypes = append([]string(nil), s.CardTypes...)
		result[i].Resources = append([]string(nil), s.Resources...)
		result[i].StandardProjects = append([]StandardProject(nil), s.StandardProjects...)
		result[i].GlobalParameters = append([]string(nil), s.GlobalParameters...)
		result[i].Actions = append([]string(nil), s.Actions...)
		result[i].TagCount = cloneMinMax(s.TagCount)
		result[i].RequiredOriginalCost = cloneMinMax(s.RequiredOriginalCost)
		result[i].VP = cloneMinMax(s.VP)
	}
	return result
}
func cloneMinMax(v *MinMaxValue) *MinMaxValue {
	if v == nil {
		return nil
	}
	return &MinMaxValue{Min: clonePointer(v.Min), Max: clonePointer(v.Max)}
}
