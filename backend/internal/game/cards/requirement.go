package cards

import (
	"openmars/internal/game/shared"
)

// RequirementType represents different types of card requirements
type RequirementType string

const (
	RequirementState       RequirementType = "state"
	RequirementTemperature RequirementType = "temperature"
	RequirementOxygen      RequirementType = "oxygen"
	RequirementOceans      RequirementType = "ocean"
	RequirementVenus       RequirementType = "venus"
	RequirementCities      RequirementType = "city"
	RequirementGreeneries  RequirementType = "greenery"
	RequirementTags        RequirementType = "tags"
	RequirementProduction  RequirementType = "production"
	RequirementTR          RequirementType = "tr"
	RequirementResource    RequirementType = "resource"
	RequirementColony      RequirementType = "colony"
)

// CardRequirements wraps a list of requirements with a human-readable description
type CardRequirements struct {
	Description string        `json:"description,omitempty"`
	Items       []Requirement `json:"items"`
}

// Requirement represents a single card requirement
type Requirement struct {
	Permanent bool                 `json:"permanent,omitempty"`
	State     string               `json:"state,omitempty"`
	Equals    *int                 `json:"equals,omitempty"`
	Type      RequirementType      `json:"type"`
	Min       *int                 `json:"min,omitempty"`
	Max       *int                 `json:"max,omitempty"`
	Location  *CardApplyLocation   `json:"location,omitempty"`
	Tag       *shared.CardTag      `json:"tag,omitempty"`
	Resource  *shared.ResourceType `json:"resource,omitempty"`
}

// CardApplyLocation represents different locations
type CardApplyLocation string

const (
	CardApplyLocationAnywhere CardApplyLocation = "anywhere"
	CardApplyLocationMars     CardApplyLocation = "mars"
)
