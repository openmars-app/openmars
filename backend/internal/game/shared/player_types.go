package shared

// CardAction represents a repeatable card action available to a player
type CardAction struct {
	CardID                  string
	CardName                string
	BehaviorIndex           int
	Behavior                CardBehavior
	TimesUsedThisTurn       int
	TimesUsedThisGeneration int
}

// CardEffect represents a passive card effect
type CardEffect struct {
	CardID        string
	CardName      string
	BehaviorIndex int
	Behavior      CardBehavior
}

// VPGranter represents a card that grants victory points
type VPGranter struct {
	CardID        string
	CardName      string
	Description   string
	VPConditions  []VPCondition
	ComputedValue int
}

// VPCondition defines how victory points are calculated
type VPCondition struct {
	StateRequirements []StateRequirement
	Amount            int
	Condition         string // "fixed", "per", "once"
	MaxTrigger        *int
	Per               *PerCondition
}

// VP condition type constants
const (
	VPConditionFixed = "fixed"
	VPConditionPer   = "per"
	VPConditionOnce  = "once"
)
