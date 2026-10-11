package cards

import (
	"openmars/internal/game/shared"
)

// ResourceStorage represents a card's ability to hold resources
type ResourceStorage struct {
	Type        shared.ResourceType `json:"type"`
	Icon        string              `json:"icon,omitempty"`
	Label       string              `json:"label,omitempty"`
	Capacity    *int                `json:"capacity,omitempty"`
	Starting    int                 `json:"starting"`
	Description string              `json:"description,omitempty" ts:"string | undefined"`
}

// VictoryPointCondition represents a VP condition
type VictoryPointCondition struct {
	StateRequirements []shared.StateRequirement `json:"stateRequirements,omitempty"`
	Amount            int                       `json:"amount"`
	Condition         VPConditionType           `json:"condition"`
	MaxTrigger        *int                      `json:"maxTrigger,omitempty"`
	Per               *shared.PerCondition      `json:"per,omitempty"`
	Description       string                    `json:"description,omitempty" ts:"string | undefined"`
}

// VPConditionType represents different types of VP conditions
type VPConditionType string

const (
	VPConditionPer   VPConditionType = "per"
	VPConditionOnce  VPConditionType = "once"
	VPConditionFixed VPConditionType = "fixed"
)
