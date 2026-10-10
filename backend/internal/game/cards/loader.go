package cards

import (
	"bytes"
	"encoding/json"
	"fmt"
	"openmars/internal/game/mode"
	"openmars/internal/game/shared"
	"os"
	"path/filepath"
	"slices"
)

// LoadCardsFromJSON loads cards from a JSON file
func LoadCardsFromJSON(filepath string) ([]Card, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read card file: %w", err)
	}

	var cards []Card
	if !json.Valid(data) {
		return nil, fmt.Errorf("invalid JSON document")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cards); err != nil {
		return nil, fmt.Errorf("failed to parse card JSON: %w", err)
	}

	if len(cards) == 0 {
		return nil, fmt.Errorf("no cards found in file: %s", filepath)
	}

	return cards, nil
}

// LoadCatalog loads and validates the card library together with its mode definitions.
func LoadCatalog(path string) (*InMemoryCardRegistry, error) {
	list, err := LoadCardsFromJSON(path)
	if err != nil {
		return nil, err
	}
	modes, err := mode.LoadFromJSON(filepath.Join(filepath.Dir(path), "gamemodes.json"))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, card := range list {
		if card.ID == "" || seen[card.ID] {
			return nil, fmt.Errorf("duplicate or empty card ID %q", card.ID)
		}
		seen[card.ID] = true
	}
	registry := NewInMemoryCardRegistry(list, modes...)
	if err := ValidateCatalog(registry); err != nil {
		return nil, err
	}
	return registry, nil
}
func ValidateCatalog(registry CardRegistry) error {
	definitions := ModeDefinitions(registry)
	ids := map[string]bool{}
	for _, d := range definitions {
		ids[d.ID] = true
	}
	seen := map[string]bool{}
	for _, c := range registry.GetAll() {
		if c.ID == "" || seen[c.ID] {
			return fmt.Errorf("duplicate or empty card ID %q", c.ID)
		}
		seen[c.ID] = true
		for _, id := range c.GameModes {
			if !ids[id] {
				return fmt.Errorf("card %s references unknown mode %s", c.ID, id)
			}
		}
		for _, id := range c.GameModes {
			definition, _ := mode.Find(definitions, id)
			effective, err := ApplyModeModifiers(c, id)
			if err != nil {
				return err
			}
			if err := validateModeCard(effective, definition); err != nil {
				return err
			}
		}
		for _, modifier := range c.Modifiers {
			if len(modifier.When.GameModes) == 0 {
				return fmt.Errorf("card %s has modifier without mode selector", c.ID)
			}
			for _, id := range modifier.When.GameModes {
				if !ids[id] || !slices.Contains(c.GameModes, id) {
					return fmt.Errorf("card %s modifier references unavailable mode %s", c.ID, id)
				}
				if _, err := ApplyModeModifiers(c, id); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// validateModeCard validates references against the effective, selected mode.
func validateModeCard(c Card, d mode.Definition) error {
	if errs := ValidateCardJSON(&c); len(errs) > 0 {
		return fmt.Errorf("card %s in mode %s: %v", c.ID, d.ID, errs)
	}
	stateExists := func(state string) bool {
		_, ok := d.Runtime.State[state]
		if ok {
			return true
		}
		for _, deadline := range d.Runtime.Deadlines {
			if deadline.State == state {
				return true
			}
		}
		return false
	}
	requireState := func(state string) error {
		if !stateExists(state) {
			return fmt.Errorf("card %s in mode %s references unknown state %q", c.ID, d.ID, state)
		}
		return nil
	}
	groups := map[string]bool{}
	for _, group := range c.DependencyGroups {
		if group.ID == "" || groups[group.ID] {
			return fmt.Errorf("card %s has empty or duplicate dependency ID %q", c.ID, group.ID)
		}
		groups[group.ID] = true
		if err := requireState(group.State); err != nil {
			return err
		}
		if group.RetireOnState != nil {
			if err := requireState(group.RetireOnState.State); err != nil {
				return err
			}
		}
	}
	if c.Requirements != nil {
		for _, r := range c.Requirements.Items {
			if r.Type == RequirementState {
				if r.Equals == nil {
					return fmt.Errorf("card %s has state requirement without equals", c.ID)
				}
				if err := requireState(r.State); err != nil {
					return err
				}
			}
		}
	}
	for _, feature := range c.RequiresFeatures {
		found := false
		for _, f := range d.Features {
			if f.ID == feature {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("card %s requires undeclared feature %s in %s", c.ID, feature, d.ID)
		}
	}
	for _, system := range c.RequiresSystems {
		if !slices.Contains(d.Systems, system) {
			return fmt.Errorf("card %s requires unsupported system %s in %s", c.ID, system, d.ID)
		}
	}
	for _, vp := range c.VPConditions {
		for _, r := range vp.StateRequirements {
			if err := requireState(r.State); err != nil {
				return err
			}
		}
	}
	seen := map[string]bool{}
	for _, b := range c.Behaviors {
		if b.MaxUsesPerGeneration < 0 || b.MaxUsesPerGame < 0 {
			return fmt.Errorf("card %s has negative behavior use limit", c.ID)
		}
		if b.ID == "" && (b.Dependency != "" || b.MaxUsesPerGeneration > 0 || b.MaxUsesPerGame > 0) {
			return fmt.Errorf("card %s tracked behavior needs stable ID", c.ID)
		}
		if b.ID != "" {
			if seen[b.ID] {
				return fmt.Errorf("card %s has duplicate behavior %s", c.ID, b.ID)
			}
			seen[b.ID] = true
		}
		if b.Dependency != "" && !groups[b.Dependency] {
			return fmt.Errorf("card %s behavior %s references unknown dependency %s", c.ID, b.ID, b.Dependency)
		}
		for _, r := range b.StateRequirements {
			if err := requireState(r.State); err != nil {
				return err
			}
		}
		for _, trigger := range b.Triggers {
			if trigger.Condition != nil && trigger.Condition.Type == "state-changed" {
				if err := requireState(trigger.Condition.State); err != nil {
					return err
				}
			}
		}
		outputs := append([]shared.BehaviorCondition{}, b.Outputs...)
		for _, choice := range b.Choices {
			outputs = append(outputs, choice.Outputs...)
			for _, r := range choice.StateRequirements {
				if err := requireState(r.State); err != nil {
					return err
				}
			}
		}
		for _, output := range outputs {
			if state, ok := output.(*shared.StateCondition); ok && state.State != "" {
				if err := requireState(state.State); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
