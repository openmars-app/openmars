package dto

import (
	"fmt"
	"openmars/internal/game"
	"openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"strings"
)

// StateRequirementDto describes a mode state prerequisite.
type StateRequirementDto struct {
	State  string `json:"state"`
	Equals int    `json:"equals"`
}

// DependencyStatusDto describes the current state of an ongoing operation.
type DependencyStatusDto struct {
	ID       string `json:"id"`
	Label    string `json:"label,omitempty"`
	Active   bool   `json:"active"`
	Imported bool   `json:"imported"`
	Retired  bool   `json:"retired"`
}

// WindowBoundaryDto describes a relative phase boundary.
type WindowBoundaryDto struct {
	GenerationOffset int    `json:"generationOffset"`
	Boundary         string `json:"boundary"`
}

// StateConditionDto describes a declarative state operation.
type StateConditionDto struct {
	Description       string                  `json:"description"`
	MinimumProduction *ProductionThresholdDto `json:"minimumProduction,omitempty"`
	Type              string                  `json:"type" tstype:"'state-change' | 'remove-dependency' | 'retire-dependency' | 'schedule-state-window'"`
	Amount            int                     `json:"amount"`
	Target            TargetType              `json:"target"`
	State             string                  `json:"state,omitempty"`
	Value             int                     `json:"value,omitempty"`
	Dependency        string                  `json:"dependency,omitempty"`
	Start             *WindowBoundaryDto      `json:"start,omitempty"`
	End               *WindowBoundaryDto      `json:"end,omitempty"`
}

// StateWindowDto is a scheduled state override.
type StateWindowDto struct {
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

// ModeRuntimeDto exposes authoritative baseline and effective state.
type ModeRuntimeDto struct {
	Events          []ModePresentationEventDto `json:"events"`
	Values          map[string]int             `json:"values"`
	EffectiveValues map[string]int             `json:"effectiveValues"`
	InitialValues   map[string]int             `json:"initialValues"`
	Windows         []StateWindowDto           `json:"windows"`
	Sequence        int                        `json:"sequence"`
}

// ToModeRuntimeDto builds the current mode-state view.
func ToModeRuntimeDto(g *game.Game) *ModeRuntimeDto {
	return ModeRuntimeStateToDto(g.ModeRuntime())
}

// ModeRuntimeStateToDto converts a captured runtime snapshot.
func ModeRuntimeStateToDto(r shared.ModeRuntimeState) *ModeRuntimeDto {
	if len(r.Values) == 0 {
		return nil
	}
	out := &ModeRuntimeDto{Events: []ModePresentationEventDto{}, Values: r.Values, InitialValues: r.InitialValues, EffectiveValues: map[string]int{}, Windows: []StateWindowDto{}, Sequence: r.Sequence}
	for k := range r.Values {
		v, _ := r.EffectiveValue(k)
		out.EffectiveValues[k] = v
	}
	for _, w := range r.Windows {
		out.Windows = append(out.Windows, StateWindowDto{ID: w.ID, Source: w.Source, State: w.State, Value: w.Value, StartGeneration: w.StartGeneration, StartBoundary: w.StartBoundary, EndGeneration: w.EndGeneration, EndBoundary: w.EndBoundary, Status: w.Status})
	}
	for _, event := range r.Events {
		e := ModePresentationEventDto{ID: event.ID, Kind: event.Kind, State: event.State, OldValue: event.OldValue, NewValue: event.NewValue, Title: event.Title, Description: event.Description}
		if event.Presentation != nil {
			h := event.Presentation
			e.Presentation = &ModePresentationHintDto{Kind: h.Kind, From: h.From, To: h.To, Connected: h.Connected}
		}
		out.Events = append(out.Events, e)
	}
	return out
}

// AnnotateCardDependencies adds instance status without rewriting authored behaviors.
func AnnotateCardDependencies(out *CardDto, card cards.Card, p *player.Player) {
	for i, b := range card.Behaviors {
		if b.Dependency == "" {
			continue
		}
		for _, d := range card.DependencyGroups {
			if d.ID != b.Dependency {
				continue
			}
			status := DependencyStatusDto{ID: d.ID, Label: d.Label, Active: true}
			if p != nil {
				s := p.Resources().EffectiveDependencyStatus(card.ID, d)
				status.Active = p.Resources().DependencyActive(card.ID, d)
				status.Imported = s.Removed
				status.Retired = s.Retired
			}
			out.Behaviors[i].DependencyStatus = &status
		}
	}
}
func toStateRequirementDto(s shared.StateRequirement) StateRequirementDto {
	return StateRequirementDto{State: s.State, Equals: s.Equals}
}

// ModePresentationHintDto supplies an optional visual hint.
type ModePresentationHintDto struct {
	Kind      string `json:"kind"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Connected *bool  `json:"connected,omitempty"`
}

// ModePresentationEventDto is an ordered state transition with an accessible fallback.
type ModePresentationEventDto struct {
	ID           string                   `json:"id"`
	Kind         string                   `json:"kind"`
	State        string                   `json:"state"`
	OldValue     int                      `json:"oldValue"`
	NewValue     int                      `json:"newValue"`
	Title        string                   `json:"title"`
	Description  string                   `json:"description,omitempty"`
	Presentation *ModePresentationHintDto `json:"presentation,omitempty"`
}

// AnnotateCardDependencyPreview evaluates a catalog card against preview state.
func AnnotateCardDependencyPreview(out *CardDto, card cards.Card, state map[string]int) {
	AnnotateCardDependencies(out, card, nil)
	runtime := shared.ModeRuntimeState{Values: state}
	for i, b := range card.Behaviors {
		for _, d := range card.DependencyGroups {
			if d.ID == b.Dependency && out.Behaviors[i].DependencyStatus != nil {
				out.Behaviors[i].DependencyStatus.Active = shared.DependencyEnabled(runtime, d, shared.DependencyStatus{})
			}
		}
	}
}

// BehaviorResourceRequirementDto describes an availability constraint.
type BehaviorResourceRequirementDto struct {
	Location  string       `json:"location,omitempty"`
	Type      ResourceType `json:"type"`
	Min       *int         `json:"min,omitempty"`
	Max       *int         `json:"max,omitempty"`
	Permanent bool         `json:"permanent,omitempty"`
	Target    string       `json:"target,omitempty"`
}

// ProductionThresholdDto limits dependency target selection.
type ProductionThresholdDto struct {
	Type   ResourceType `json:"type"`
	Amount int          `json:"amount"`
}

func stateOperationDescription(c *shared.StateCondition) string {
	if c.Description != "" {
		return c.Description
	}
	switch c.ResourceType {
	case shared.ResourceStateChange:
		direction := "Delay"
		amount := c.Amount
		if amount < 0 {
			direction = "Advance"
			amount = -amount
		}
		unit := "generation"
		if amount != 1 {
			unit = "generations"
		}
		return fmt.Sprintf("%s the scenario deadline by %d %s", direction, amount, unit)
	case shared.ResourceRemoveDependency:
		return "Import one eligible project's ongoing operation"
	case shared.ResourceRetireDependency:
		return "Retire one eligible project's ongoing operation"
	case shared.ResourceScheduleStateWindow:
		if c.Start != nil && c.End != nil {
			return fmt.Sprintf("Restore support from generation +%d %s through generation +%d %s", c.Start.GenerationOffset, strings.ReplaceAll(c.Start.Boundary, "-", " "), c.End.GenerationOffset, strings.ReplaceAll(c.End.Boundary, "-", " "))
		}
		return "Schedule temporary restored support"
	}
	return "Change scenario state"
}
