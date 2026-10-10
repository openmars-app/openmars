package mode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"openmars/internal/game/shared"
	"os"
	"slices"
)

type Rule struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}
type Parameter struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Default     int    `json:"default"`
	Values      []int  `json:"values"`
}
type Feature struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Experimental bool   `json:"experimental"`
	Default      bool   `json:"default"`
}
type StatusDefinition struct {
	State  string            `json:"state"`
	Label  string            `json:"label"`
	Labels map[string]string `json:"labels,omitempty"`
	Kind   string            `json:"kind"`
	From   string            `json:"from,omitempty"`
	To     string            `json:"to,omitempty"`
}
type Definition struct {
	ID         string                       `json:"id"`
	Name       string                       `json:"name"`
	Summary    string                       `json:"summary"`
	Scenario   string                       `json:"scenario"`
	Rules      []Rule                       `json:"rules"`
	Statuses   []StatusDefinition           `json:"statuses"`
	MinPlayers int                          `json:"minPlayers"`
	MaxPlayers int                          `json:"maxPlayers"`
	Parameters []Parameter                  `json:"parameters"`
	Features   []Feature                    `json:"features"`
	Systems    []string                     `json:"systems"`
	Runtime    shared.ModeRuntimeDefinition `json:"runtime"`
}

func LoadFromJSON(path string) ([]Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read game modes: %w", err)
	}
	var definitions []Definition
	if !json.Valid(data) {
		return nil, fmt.Errorf("invalid JSON document")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&definitions); err != nil {
		return nil, fmt.Errorf("decode game modes: %w", err)
	}
	if len(definitions) == 0 {
		return nil, fmt.Errorf("game mode catalog is empty")
	}
	ids := map[string]bool{}
	for _, d := range definitions {
		if d.ID == "" || ids[d.ID] {
			return nil, fmt.Errorf("duplicate or empty game mode ID %q", d.ID)
		}
		ids[d.ID] = true
		if d.MinPlayers < 1 || d.MaxPlayers < d.MinPlayers {
			return nil, fmt.Errorf("invalid player limits for mode %s", d.ID)
		}
		params := map[string]bool{}
		for _, p := range d.Parameters {
			if p.ID == "" || params[p.ID] || !slices.Contains(p.Values, p.Default) {
				return nil, fmt.Errorf("invalid parameter %q in mode %s", p.ID, d.ID)
			}
			params[p.ID] = true
		}
		states := map[string]bool{}
		for key := range d.Runtime.State {
			if key == "" {
				return nil, fmt.Errorf("empty runtime state in mode %s", d.ID)
			}
			states[key] = true
		}
		for _, deadline := range d.Runtime.Deadlines {
			if deadline.State == "" || states[deadline.State] || !params[deadline.Parameter] || !states[deadline.ActiveState] || !states[deadline.OccurredState] || deadline.ActiveValue == deadline.InactiveValue || deadline.MinGenerationOffset < 1 || deadline.MaxParameterAdjustment < 0 || deadline.PerPlayerGenerationLimit < 1 {
				return nil, fmt.Errorf("invalid deadline %q in mode %s", deadline.State, d.ID)
			}
			states[deadline.State] = true
		}
		for _, status := range d.Statuses {
			if !states[status.State] || (status.Kind != "connection" && status.Kind != "counter") {
				return nil, fmt.Errorf("invalid status %q in mode %s", status.State, d.ID)
			}
		}
		for _, rule := range d.Runtime.PresentationRules {
			if !states[rule.State] {
				return nil, fmt.Errorf("presentation rule references unknown state %s in mode %s", rule.State, d.ID)
			}
		}
		behaviorIDs := map[string]bool{}
		for _, b := range d.Runtime.Behaviors {
			if b.ID == "" || behaviorIDs[b.ID] || (b.Scope != "game" && b.Scope != "each-player") {
				return nil, fmt.Errorf("invalid mode behavior %q in mode %s", b.ID, d.ID)
			}
			behaviorIDs[b.ID] = true
			for _, r := range b.StateRequirements {
				if !states[r.State] {
					return nil, fmt.Errorf("mode behavior %s references unknown state %s", b.ID, r.State)
				}
			}
			for _, trigger := range b.Triggers {
				if trigger.Condition != nil && trigger.Condition.Type == "state-changed" && !states[trigger.Condition.State] {
					return nil, fmt.Errorf("mode behavior %s references unknown trigger state", b.ID)
				}
			}
		}
		features := map[string]bool{}
		for _, f := range d.Features {
			if f.ID == "" || features[f.ID] {
				return nil, fmt.Errorf("invalid feature %q in mode %s", f.ID, d.ID)
			}
			features[f.ID] = true
		}
	}
	return definitions, nil
}

func Find(definitions []Definition, id string) (Definition, error) {
	for _, d := range definitions {
		if d.ID == id {
			return d, nil
		}
	}
	return Definition{}, fmt.Errorf("unknown game mode: %s", id)
}

// Normalize validates a selection and fills the selected mode's declared defaults.
func Normalize(definitions []Definition, settings shared.GameSettings) (shared.GameSettings, Definition, error) {
	if settings.GameModeID == "" {
		settings.GameModeID = "classic"
	}
	d, err := Find(definitions, settings.GameModeID)
	if err != nil {
		return settings, d, err
	}
	if settings.MaxPlayers == 0 {
		settings.MaxPlayers = 5
	}
	if settings.MaxPlayers < d.MinPlayers || settings.MaxPlayers > d.MaxPlayers {
		return settings, d, fmt.Errorf("%s requires between %d and %d players", d.Name, d.MinPlayers, d.MaxPlayers)
	}
	parameters := make(map[string]int, len(d.Parameters))
	for _, p := range d.Parameters {
		value, ok := settings.GameModeParameters[p.ID]
		if !ok {
			value = p.Default
		}
		if !slices.Contains(p.Values, value) {
			return settings, d, fmt.Errorf("invalid value %d for mode parameter %s", value, p.ID)
		}
		parameters[p.ID] = value
	}
	for key := range settings.GameModeParameters {
		if _, ok := parameters[key]; !ok {
			return settings, d, fmt.Errorf("unknown mode parameter: %s", key)
		}
	}
	settings.GameModeParameters = parameters
	if settings.Features == nil {
		settings.Features = []string{}
		for _, f := range d.Features {
			if f.Default {
				settings.Features = append(settings.Features, f.ID)
			}
		}
	} else {
		settings.Features = slices.Clone(settings.Features)
	}
	seen := map[string]bool{}
	for _, id := range settings.Features {
		if seen[id] {
			return settings, d, fmt.Errorf("duplicate feature: %s", id)
		}
		seen[id] = true
		found := false
		for _, f := range d.Features {
			if f.ID == id {
				found = true
				break
			}
		}
		if !found {
			return settings, d, fmt.Errorf("feature %s is unavailable in %s", id, d.ID)
		}
	}
	settings.Systems = slices.Clone(d.Systems)
	return settings, d, nil
}
