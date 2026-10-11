package game

import (
	"context"
	"fmt"
	"maps"
	"openmars/internal/events"
	"openmars/internal/game/datastore"
	"openmars/internal/game/shared"
	"slices"
)

// ConfigureModeRuntime initializes mode state before play or after a lobby settings change.
func (g *Game) ConfigureModeRuntime(def shared.ModeRuntimeDefinition, parameters map[string]int) error {
	def = def.Clone()
	values := maps.Clone(def.State)
	if values == nil {
		values = map[string]int{}
	}
	for _, d := range def.Deadlines {
		v, ok := parameters[d.Parameter]
		if !ok {
			return fmt.Errorf("missing mode parameter %s", d.Parameter)
		}
		values[d.State] = v
	}
	g.update(func(s *datastore.GameState) {
		s.ModeRuntime = shared.ModeRuntimeState{Definition: def, Values: values, InitialValues: maps.Clone(values), Changes: map[string]int{}}
	})
	return nil
}

// ModeRuntime returns an independent snapshot of the runtime state.
func (g *Game) ModeRuntime() shared.ModeRuntimeState {
	var out shared.ModeRuntimeState
	g.read(func(s *datastore.GameState) {
		out = s.ModeRuntime
		out.Definition = out.Definition.Clone()
		out.Values = maps.Clone(out.Values)
		out.InitialValues = maps.Clone(out.InitialValues)
		out.Changes = maps.Clone(out.Changes)
		out.Windows = slices.Clone(out.Windows)
		out.Events = slices.Clone(out.Events)
		for i := range out.Events {
			out.Events[i].Effects = shared.ClonePresentationEffects(out.Events[i].Effects)
		}
	})
	return out
}

// ModeStateValue returns the currently effective state value.
func (g *Game) ModeStateValue(key string) (int, bool) { return g.ModeRuntime().EffectiveValue(key) }

// ModeRequirementsMet checks declared state predicates.
func (g *Game) ModeRequirementsMet(requirements []shared.StateRequirement) bool {
	return g.ValidateModeRequirements(requirements) == nil
}

// ValidateModeRequirements explains the first unmet state requirement using the mode's wording.
func (g *Game) ValidateModeRequirements(requirements []shared.StateRequirement) error {
	if len(requirements) == 0 {
		return nil
	}
	runtime := g.ModeRuntime()
	for _, requirement := range requirements {
		value, ok := runtime.EffectiveValue(requirement.State)
		if ok && value == requirement.Equals {
			continue
		}
		message := runtime.Definition.RequirementMessages[requirement.State][requirement.Equals]
		if message == "" {
			message = "Requirement not met"
		}
		return fmt.Errorf("%s", message)
	}
	return nil
}

// ValidateModeStateChange checks all deadline bounds without mutating the game.
func (g *Game) ValidateModeStateChange(playerID, key string, amount int) error {
	r := g.ModeRuntime()
	v, ok := r.Values[key]
	if !ok {
		return fmt.Errorf("unknown mode state %s", key)
	}
	for _, d := range r.Definition.Deadlines {
		if d.State != key {
			continue
		}
		if r.Values[d.OccurredState] != 0 {
			return fmt.Errorf("deadline has already occurred")
		}
		if amount == 0 {
			return fmt.Errorf("state change must be nonzero")
		}
		next := v + amount
		initial := r.InitialValues[key]
		if next < initial-d.MaxParameterAdjustment || next > initial+d.MaxParameterAdjustment {
			return fmt.Errorf("deadline adjustment exceeds configured bounds")
		}
		if next < g.Generation()+d.MinGenerationOffset {
			return fmt.Errorf("deadline must preserve the notice period")
		}
		usage := fmt.Sprintf("%s:%s:%d", playerID, key, g.Generation())
		if d.PerPlayerGenerationLimit > 0 && r.Changes[usage] >= d.PerPlayerGenerationLimit {
			return fmt.Errorf("deadline already changed this generation")
		}
	}
	return nil
}

// ChangeModeState applies a validated baseline change.
func (g *Game) ChangeModeState(ctx context.Context, playerID, key string, amount int) error {
	if err := g.ValidateModeStateChange(playerID, key, amount); err != nil {
		return err
	}
	old := g.ModeRuntime().Values[key]
	g.update(func(s *datastore.GameState) {
		s.ModeRuntime.Values[key] += amount
		appendModePresentation(&s.ModeRuntime, key, old, old+amount)
		if s.ModeRuntime.Changes == nil {
			s.ModeRuntime.Changes = map[string]int{}
		}
		s.ModeRuntime.Changes[fmt.Sprintf("%s:%s:%d", playerID, key, s.Generation)]++
	})
	g.publishModeStateChange(ctx, playerID, key, old, old+amount)
	return nil
}

func (g *Game) publishModeStateChange(_ context.Context, playerID, key string, old, value int) {
	events.Publish(g.eventBus, events.ModeStateChangedEvent{PlayerID: playerID, GameID: g.id, State: key, Old: old, New: value})
	events.Publish(g.eventBus, events.GameStateChangedEvent{GameID: g.id})
}

func boundaryOrder(name string) int {
	switch name {
	case "research-start":
		return 0
	case "production-start":
		return 1
	case "production-end":
		return 2
	}
	return -1
}
func boundaryTime(generation int, boundary string) int { return generation*3 + boundaryOrder(boundary) }

// ValidateStateWindow rejects overlapping reservations and invalid boundaries.
func (g *Game) ValidateStateWindow(output *shared.StateCondition) error {
	if output.Start == nil || output.End == nil {
		return fmt.Errorf("state window needs start and end")
	}
	if boundaryOrder(output.Start.Boundary) < 0 || boundaryOrder(output.End.Boundary) < 0 {
		return fmt.Errorf("invalid window boundary")
	}
	if output.Start.GenerationOffset < 0 || output.End.GenerationOffset < 0 {
		return fmt.Errorf("cannot schedule past window")
	}
	if output.Start.GenerationOffset == 0 && output.Start.Boundary == "research-start" {
		return fmt.Errorf("research has already started")
	}
	if output.Start.GenerationOffset > 0 && g.GlobalParameters().IsMaxed() {
		return fmt.Errorf("game has no next generation")
	}
	r := g.ModeRuntime()
	if _, ok := r.Values[output.State]; !ok {
		return fmt.Errorf("unknown window state %s", output.State)
	}
	start := boundaryTime(g.Generation()+output.Start.GenerationOffset, output.Start.Boundary)
	end := boundaryTime(g.Generation()+output.End.GenerationOffset, output.End.Boundary)
	if end <= start {
		return fmt.Errorf("window must end after it starts")
	}
	for _, w := range r.Windows {
		if w.State != output.State || w.Status == "cancelled" {
			continue
		}
		a := boundaryTime(w.StartGeneration, w.StartBoundary)
		b := boundaryTime(w.EndGeneration, w.EndBoundary)
		if start < b && end > a {
			return fmt.Errorf("state window overlaps a reserved interval")
		}
	}
	return nil
}

// ScheduleStateWindow reserves a deterministic interval.
func (g *Game) ScheduleStateWindow(output *shared.StateCondition, source string) error {
	if err := g.ValidateStateWindow(output); err != nil {
		return err
	}
	g.update(func(s *datastore.GameState) {
		s.ModeRuntime.Sequence++
		s.ModeRuntime.Windows = append(s.ModeRuntime.Windows, shared.StateWindow{ID: fmt.Sprintf("window-%d", s.ModeRuntime.Sequence), Source: source, State: output.State, Value: output.Value, StartGeneration: s.Generation + output.Start.GenerationOffset, StartBoundary: output.Start.Boundary, EndGeneration: s.Generation + output.End.GenerationOffset, EndBoundary: output.End.Boundary, Status: "pending"})
	})
	events.Publish(g.eventBus, events.GameStateChangedEvent{GameID: g.id})
	return nil
}

// ResolveModeBoundary activates or ends windows and applies original deadlines once.
func (g *Game) ResolveModeBoundary(ctx context.Context, boundary string, final bool) {
	var changes []events.ModeStateChangedEvent
	generation := g.Generation()
	g.update(func(s *datastore.GameState) {
		r := &s.ModeRuntime
		before := make(map[string]int, len(r.Values))
		for key := range r.Values {
			before[key], _ = r.EffectiveValue(key)
		}
		defer func() {
			for _, key := range sortedStateKeys(r.Values) {
				value, _ := r.EffectiveValue(key)
				appendModePresentation(r, key, before[key], value)
			}
		}()
		for i := range r.Windows {
			w := &r.Windows[i]
			if final && w.Status == "pending" && w.StartGeneration > generation {
				w.Status = "cancelled"
				continue
			}
			if w.Status == "active" && w.EndGeneration == generation && w.EndBoundary == boundary {
				w.Status = "completed"
			}
			if w.Status == "pending" && w.StartGeneration == generation && w.StartBoundary == boundary {
				w.Status = "active"
			}
		}
		if boundary == "production-end" {
			for _, d := range r.Definition.Deadlines {
				if r.Values[d.OccurredState] == 0 && generation >= r.Values[d.State] {
					old := r.Values[d.ActiveState]
					r.Values[d.ActiveState] = d.InactiveValue
					r.Values[d.OccurredState] = 1
					changes = append(changes, events.ModeStateChangedEvent{GameID: g.id, State: d.ActiveState, Old: old, New: d.InactiveValue}, events.ModeStateChangedEvent{GameID: g.id, State: d.OccurredState, Old: 0, New: 1})
				}
			}
		}
	})
	for _, c := range changes {
		events.Publish(g.eventBus, c)
	}
	events.Publish(g.eventBus, events.GameStateChangedEvent{GameID: g.id})
}

// ClaimModeBehaviorsRegistration prevents duplicate subscriptions for one runtime.
func (g *Game) ClaimModeBehaviorsRegistration() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.modeBehaviorsRegistered {
		return false
	}
	g.modeBehaviorsRegistered = true
	return true
}

func sortedStateKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
func appendModePresentation(r *shared.ModeRuntimeState, state string, old, value int) {
	if old == value {
		return
	}
	for _, rule := range r.Definition.PresentationRules {
		if rule.State != state || (rule.To != nil && *rule.To != value) {
			continue
		}
		r.Sequence++
		r.Events = append(r.Events, shared.ModePresentationEvent{ID: fmt.Sprintf("mode-event-%d", r.Sequence), Kind: "state-change", State: state, OldValue: old, NewValue: value, Title: rule.Title, SoundID: rule.SoundID, Description: rule.Description, Effects: shared.ClonePresentationEffects(rule.Effects)})
	}
}

// BeginModeGeneration claims one generation's income and deadline warnings exactly once.
func (g *Game) BeginModeGeneration() (*shared.GenerationIncomeRule, int) {
	var income *shared.GenerationIncomeRule
	amount := 0
	changed := false
	g.update(func(s *datastore.GameState) {
		r := &s.ModeRuntime
		if s.Generation <= r.LastStartedGeneration || (r.Definition.GenerationIncome == nil && len(r.Definition.Deadlines) == 0) {
			return
		}
		changed = true
		if s.Generation != r.LastStartedGeneration+1 {
			r.IncomeStreak = 0
		}
		r.LastStartedGeneration = s.Generation
		if rule := r.Definition.GenerationIncome; rule != nil {
			value, ok := r.EffectiveValue(rule.State)
			if ok && value == rule.Equals {
				r.IncomeStreak++
				copy := *rule
				income = &copy
				amount = rule.InitialAmount + (r.IncomeStreak-1)*rule.Increment
			} else {
				r.IncomeStreak = 0
			}
		}
		for _, deadline := range r.Definition.Deadlines {
			if deadline.WarningTitle == "" || r.Values[deadline.OccurredState] != 0 || r.Values[deadline.State] != s.Generation {
				continue
			}
			r.Sequence++
			r.Events = append(r.Events, shared.ModePresentationEvent{ID: fmt.Sprintf("mode-event-%d", r.Sequence), Kind: "deadline-warning", State: deadline.State, Title: deadline.WarningTitle, SoundID: deadline.WarningSoundID, Description: deadline.WarningDescription})
		}
	})
	if changed {
		events.Publish(g.eventBus, events.GameStateChangedEvent{GameID: g.id})
	}
	return income, amount
}
