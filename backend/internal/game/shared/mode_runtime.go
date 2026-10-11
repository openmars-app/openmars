package shared

// StateRequirement compares a declared game state value.
type StateRequirement struct {
	State  string `json:"state"`
	Equals int    `json:"equals"`
}

// DependencyGroup identifies an ongoing operation and the state that enables it.
type DependencyGroup struct {
	Label         string            `json:"label,omitempty"`
	ID            string            `json:"id"`
	State         string            `json:"state"`
	Equals        int               `json:"equals"`
	Importable    bool              `json:"importable,omitempty"`
	RetireOnState *StateRequirement `json:"retireOnState,omitempty"`
}

// ModeRuntimeDefinition contains executable, declarative mode rules.
type ModeRuntimeDefinition struct {
	GenerationIncome    *GenerationIncomeRule     `json:"generationIncome,omitempty"`
	RequirementMessages map[string]map[int]string `json:"requirementMessages,omitempty"`
	PresentationRules   []StatePresentationRule   `json:"presentationRules,omitempty"`
	State               map[string]int            `json:"state,omitempty"`
	Deadlines           []StateDeadline           `json:"deadlines,omitempty"`
	Behaviors           []CardBehavior            `json:"behaviors,omitempty"`
}

// GenerationIncomeRule pays all active players at the start of consecutive eligible generations.
type GenerationIncomeRule struct {
	State         string       `json:"state"`
	Equals        int          `json:"equals"`
	InitialAmount int          `json:"initialAmount"`
	Increment     int          `json:"increment"`
	Resource      ResourceType `json:"resource"`
	Name          string       `json:"name"`
}

// StateDeadline changes a baseline state after the selected generation's production.
type StateDeadline struct {
	WarningTitle             string `json:"warningTitle,omitempty"`
	WarningDescription       string `json:"warningDescription,omitempty"`
	WarningSoundID           string `json:"warningSoundId,omitempty"`
	State                    string `json:"state"`
	Parameter                string `json:"parameter"`
	ActiveState              string `json:"activeState"`
	OccurredState            string `json:"occurredState"`
	ActiveValue              int    `json:"activeValue"`
	InactiveValue            int    `json:"inactiveValue"`
	MaxParameterAdjustment   int    `json:"maxParameterAdjustment"`
	MinGenerationOffset      int    `json:"minGenerationOffset"`
	PerPlayerGenerationLimit int    `json:"perPlayerGenerationLimit"`
}

// WindowBoundary identifies an offset from the generation in which a window is created.
type WindowBoundary struct {
	GenerationOffset int    `json:"generationOffset"`
	Boundary         string `json:"boundary"`
}

// StateWindow records a reserved interval overriding one state value.
type StateWindow struct {
	ID              string `json:"id"`
	Source          string `json:"source"`
	State           string `json:"state"`
	Value           int    `json:"value"`
	StartGeneration int    `json:"startGeneration"`
	StartBoundary   string `json:"startBoundary"`
	EndGeneration   int    `json:"endGeneration"`
	EndBoundary     string `json:"endBoundary"`
	Status          string `json:"status"`
}

// ModeRuntimeState is serializable authoritative mode state.
type ModeRuntimeState struct {
	LastStartedGeneration int                     `json:"lastStartedGeneration"`
	IncomeStreak          int                     `json:"incomeStreak"`
	Events                []ModePresentationEvent `json:"events"`
	Definition            ModeRuntimeDefinition   `json:"definition"`
	Values                map[string]int          `json:"values"`
	InitialValues         map[string]int          `json:"initialValues"`
	Windows               []StateWindow           `json:"windows"`
	Changes               map[string]int          `json:"changes"`
	Sequence              int                     `json:"sequence"`
}

// DependencyStatus applies to one owned card instance.
type DependencyStatus struct {
	Removed bool `json:"removed"`
	Retired bool `json:"retired"`
}

// ProductionContribution retains the complete signed production from a dependent source.
type ProductionContribution struct {
	CardID     string          `json:"cardId"`
	BehaviorID string          `json:"behaviorId"`
	Dependency DependencyGroup `json:"dependency"`
	Resource   ResourceType    `json:"resource"`
	Amount     int             `json:"amount"`
}

// StateCondition performs a state operation or changes an owned dependency.
type StateCondition struct {
	Description       string               `json:"description,omitempty"`
	MinimumProduction *ProductionThreshold `json:"minimumProduction,omitempty"`
	ConditionBase
	State      string          `json:"state,omitempty"`
	Value      int             `json:"value,omitempty"`
	Dependency string          `json:"dependency,omitempty"`
	Start      *WindowBoundary `json:"start,omitempty"`
	End        *WindowBoundary `json:"end,omitempty"`
}

func (c *StateCondition) isBehaviorCondition() {}
func (c *StateCondition) deepCopyCondition() BehaviorCondition {
	cp := *c
	if c.MinimumProduction != nil {
		t := *c.MinimumProduction
		cp.MinimumProduction = &t
	}
	if c.Start != nil {
		s := *c.Start
		cp.Start = &s
	}
	if c.End != nil {
		e := *c.End
		cp.End = &e
	}
	return &cp
}

const (
	ResourceStateChange         ResourceType = "state-change"
	ResourceRemoveDependency    ResourceType = "remove-dependency"
	ResourceRetireDependency    ResourceType = "retire-dependency"
	ResourceScheduleStateWindow ResourceType = "schedule-state-window"
)

// EffectiveValue includes an active scheduled override.
func (s ModeRuntimeState) EffectiveValue(key string) (int, bool) {
	value, ok := s.Values[key]
	for _, w := range s.Windows {
		if w.State == key && w.Status == "active" {
			return w.Value, true
		}
	}
	return value, ok
}

// DependencyEnabled evaluates an operation without changing its definition.
func DependencyEnabled(s ModeRuntimeState, d DependencyGroup, status DependencyStatus) bool {
	if status.Retired {
		return false
	}
	if status.Removed {
		return true
	}
	if d.RetireOnState != nil {
		if v, ok := s.Values[d.RetireOnState.State]; ok && v == d.RetireOnState.Equals {
			return false
		}
	}
	v, ok := s.EffectiveValue(d.State)
	return ok && v == d.Equals
}

// DependencyKey identifies a dependency on one owned card.
func DependencyKey(cardID, dependencyID string) string { return cardID + ":" + dependencyID }

// StatePresentationRule supplies presentation without coupling the engine to a scenario.
type StatePresentationRule struct {
	State       string                   `json:"state"`
	To          *int                     `json:"to,omitempty"`
	Title       string                   `json:"title"`
	SoundID     string                   `json:"soundId,omitempty"`
	Description string                   `json:"description,omitempty"`
	Effects     []ModePresentationEffect `json:"effects,omitempty"`
}

// ModePresentationEffect requests an optional renderer capability.
type ModePresentationEffect struct {
	Kind string          `json:"kind"`
	Link *LinkTransition `json:"link,omitempty"`
}

// LinkTransition describes a connection between named scene entities.
type LinkTransition struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Connected bool   `json:"connected"`
}

// ClonePresentationEffects keeps presentation snapshots independent.
func ClonePresentationEffects(effects []ModePresentationEffect) []ModePresentationEffect {
	result := append([]ModePresentationEffect(nil), effects...)
	for i := range result {
		result[i].Link = clonePointer(result[i].Link)
	}
	return result
}

// ModePresentationEvent is an authoritative ordered state transition.
type ModePresentationEvent struct {
	ID          string                   `json:"id"`
	Kind        string                   `json:"kind"`
	State       string                   `json:"state"`
	OldValue    int                      `json:"oldValue"`
	NewValue    int                      `json:"newValue"`
	Title       string                   `json:"title"`
	SoundID     string                   `json:"soundId,omitempty"`
	Description string                   `json:"description,omitempty"`
	Effects     []ModePresentationEffect `json:"effects,omitempty"`
}

// BehaviorResourceRequirement gates an operation without spending the resource.
type BehaviorResourceRequirement struct {
	Location  string       `json:"location,omitempty"`
	Type      ResourceType `json:"type"`
	Min       *int         `json:"min,omitempty"`
	Max       *int         `json:"max,omitempty"`
	Permanent bool         `json:"permanent,omitempty"`
	Target    string       `json:"target,omitempty"`
}

// ProductionThreshold limits a dependency target to a meaningful source contribution.
type ProductionThreshold struct {
	Type   ResourceType `json:"type"`
	Amount int          `json:"amount"`
}

// EffectiveProduction combines permanent production and active signed source contributions.
func EffectiveProduction(permanent Production, contributions []ProductionContribution, statuses map[string]DependencyStatus, runtime ModeRuntimeState) Production {
	result := permanent
	for _, c := range contributions {
		status := statuses[DependencyKey(c.CardID, c.Dependency.ID)]
		if status.Removed || !DependencyEnabled(runtime, c.Dependency, status) {
			continue
		}
		switch c.Resource {
		case ResourceCreditProduction:
			result.Credits += c.Amount
		case ResourceSteelProduction:
			result.Steel += c.Amount
		case ResourceTitaniumProduction:
			result.Titanium += c.Amount
		case ResourcePlantProduction:
			result.Plants += c.Amount
		case ResourceEnergyProduction:
			result.Energy += c.Amount
		case ResourceHeatProduction:
			result.Heat += c.Amount
		}
	}
	result.Credits = max(result.Credits, MinCreditProduction)
	result.Steel = max(result.Steel, 0)
	result.Titanium = max(result.Titanium, 0)
	result.Plants = max(result.Plants, 0)
	result.Energy = max(result.Energy, 0)
	result.Heat = max(result.Heat, 0)
	return result
}

// Clone returns an independent runtime definition.
func (d ModeRuntimeDefinition) Clone() ModeRuntimeDefinition {
	copy := d
	copy.GenerationIncome = clonePointer(d.GenerationIncome)
	copy.RequirementMessages = make(map[string]map[int]string, len(d.RequirementMessages))
	for state, messages := range d.RequirementMessages {
		copy.RequirementMessages[state] = make(map[int]string, len(messages))
		for value, message := range messages {
			copy.RequirementMessages[state][value] = message
		}
	}
	copy.State = make(map[string]int, len(d.State))
	for k, v := range d.State {
		copy.State[k] = v
	}
	copy.Deadlines = append([]StateDeadline(nil), d.Deadlines...)
	copy.Behaviors = make([]CardBehavior, len(d.Behaviors))
	for i, b := range d.Behaviors {
		copy.Behaviors[i] = b.DeepCopy()
	}
	copy.PresentationRules = append([]StatePresentationRule(nil), d.PresentationRules...)
	for i := range copy.PresentationRules {
		r := &copy.PresentationRules[i]
		r.To = clonePointer(r.To)
		r.Effects = ClonePresentationEffects(r.Effects)
	}
	return copy
}
