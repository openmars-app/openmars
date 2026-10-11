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
	State    string `json:"state"`
	Equals   int    `json:"equals"`
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
		e := ModePresentationEventDto{ID: event.ID, Kind: event.Kind, State: event.State, OldValue: event.OldValue, NewValue: event.NewValue, Title: event.Title, SoundID: event.SoundID, Description: event.Description}
		for _, effect := range event.Effects {
			mapped := ModePresentationEffectDto{Kind: effect.Kind}
			if effect.Link != nil {
				mapped.Link = &LinkTransitionDto{From: effect.Link.From, To: effect.Link.To, Connected: effect.Link.Connected}
			}
			e.Effects = append(e.Effects, mapped)
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
			status := DependencyStatusDto{ID: d.ID, State: d.State, Equals: d.Equals, Label: d.Label, Active: true}
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

// ModePresentationEffectDto requests an optional presentation capability.
type ModePresentationEffectDto struct {
	Kind string             `json:"kind"`
	Link *LinkTransitionDto `json:"link,omitempty"`
}

// LinkTransitionDto describes the endpoints and outcome of a link transition.
type LinkTransitionDto struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Connected bool   `json:"connected"`
}

// ModePresentationEventDto is an ordered state transition with an accessible fallback.
type ModePresentationEventDto struct {
	ID          string                      `json:"id"`
	Kind        string                      `json:"kind"`
	State       string                      `json:"state"`
	OldValue    int                         `json:"oldValue"`
	NewValue    int                         `json:"newValue"`
	Title       string                      `json:"title"`
	SoundID     string                      `json:"soundId,omitempty"`
	Description string                      `json:"description,omitempty"`
	Effects     []ModePresentationEffectDto `json:"effects,omitempty"`
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
		direction := "later"
		amount := c.Amount
		if amount < 0 {
			direction = "earlier"
			amount = -amount
		}
		unit := "generation"
		if amount != 1 {
			unit = "generations"
		}
		return fmt.Sprintf("Move the scenario deadline %d %s %s", amount, unit, direction)
	case shared.ResourceRemoveDependency:
		return "Make the Earth-dependent effects of one of your projects permanent"
	case shared.ResourceRetireDependency:
		return "Shut down support on one of your projects"
	case shared.ResourceScheduleStateWindow:
		if c.Start != nil && c.End != nil {
			if c.Start.GenerationOffset == c.End.GenerationOffset && c.End.Boundary == "production-end" {
				if c.Start.GenerationOffset == 1 && c.Start.Boundary == "research-start" {
					return "Restore support for the next generation"
				}
				if c.Start.GenerationOffset == 0 && c.Start.Boundary == "production-start" {
					return "Restore support for this generation's production"
				}
			}
			return fmt.Sprintf("Restore support from generation +%d %s through generation +%d %s", c.Start.GenerationOffset, strings.ReplaceAll(c.Start.Boundary, "-", " "), c.End.GenerationOffset, strings.ReplaceAll(c.End.Boundary, "-", " "))
		}
		return "Schedule temporary restored support"
	}
	return "Change scenario state"
}
