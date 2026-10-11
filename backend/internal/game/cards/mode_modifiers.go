package cards

import (
	"encoding/json"
	"fmt"
	"openmars/internal/game"
	"openmars/internal/game/mode"
	"openmars/internal/game/shared"
	"slices"
)

type ModeModifier struct {
	When    ModifierWhen `json:"when"`
	Changes []CardChange `json:"changes"`
}
type ModifierWhen struct {
	GameModes []string `json:"gameModes"`
}
type CardChange struct {
	Systems          []string                 `json:"systems,omitempty"`
	Type             string                   `json:"type"`
	Amount           *int                     `json:"amount,omitempty"`
	Description      CardDescription          `json:"description,omitempty"`
	BehaviorID       string                   `json:"behaviorId,omitempty"`
	Behavior         *shared.CardBehavior     `json:"behavior,omitempty"`
	Requirements     *CardRequirements        `json:"requirements,omitempty"`
	DependencyGroups []shared.DependencyGroup `json:"dependencyGroups,omitempty"`
}

func cloneJSON[T any](value T) T {
	data, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("copy card data: %v", err))
	}
	var result T
	if err = json.Unmarshal(data, &result); err != nil {
		panic(fmt.Sprintf("copy card data: %v", err))
	}
	return result
}
func cloneModifiers(value []ModeModifier) []ModeModifier { return cloneJSON(value) }

func ApplyModeModifiers(original Card, modeID string) (Card, error) {
	c := original.DeepCopy()
	changed := map[string]bool{}
	for _, modifier := range c.Modifiers {
		if !slices.Contains(modifier.When.GameModes, modeID) {
			continue
		}
		for _, change := range modifier.Changes {
			key := change.Type
			switch change.Type {
			case "set-description", "append-description":
				key = "description"
			case "replace-behavior", "remove-behavior":
				key = "behavior:" + change.BehaviorID
			case "add-behavior":
				if change.Behavior != nil {
					key = "behavior:" + change.Behavior.ID
				}
			}
			if changed[key] {
				return Card{}, fmt.Errorf("card %s: conflicting mode changes for %s", c.ID, key)
			}
			changed[key] = true
			switch change.Type {
			case "set-cost":
				if change.Amount == nil || *change.Amount < 0 {
					return Card{}, fmt.Errorf("card %s: invalid set-cost", c.ID)
				}
				c.Cost = *change.Amount
			case "set-description":
				c.Description = change.Description.DeepCopy()
			case "append-description":
				c.Description = append(c.Description, change.Description.DeepCopy()...)
			case "set-requirements":
				c.Requirements = cloneJSON(change.Requirements)
			case "set-required-systems":
				c.RequiresSystems = slices.Clone(change.Systems)
			case "set-dependency-groups":
				c.DependencyGroups = cloneJSON(change.DependencyGroups)
			case "add-behavior":
				if change.Behavior == nil || change.Behavior.ID == "" {
					return Card{}, fmt.Errorf("card %s: added behavior needs an ID", c.ID)
				}
				for _, b := range c.Behaviors {
					if b.ID == change.Behavior.ID {
						return Card{}, fmt.Errorf("card %s: duplicate behavior %s", c.ID, b.ID)
					}
				}
				c.Behaviors = append(c.Behaviors, change.Behavior.DeepCopy())
			case "replace-behavior", "remove-behavior":
				if change.BehaviorID == "" {
					return Card{}, fmt.Errorf("card %s: behavior target ID is empty", c.ID)
				}
				index := -1
				for i, b := range c.Behaviors {
					if b.ID == change.BehaviorID {
						index = i
						break
					}
				}
				if index < 0 {
					return Card{}, fmt.Errorf("card %s: unknown behavior %s", c.ID, change.BehaviorID)
				}
				if change.Type == "remove-behavior" {
					c.Behaviors = append(c.Behaviors[:index], c.Behaviors[index+1:]...)
				} else {
					if change.Behavior == nil {
						return Card{}, fmt.Errorf("card %s: replacement behavior missing", c.ID)
					}
					b := change.Behavior.DeepCopy()
					if b.ID != "" && b.ID != change.BehaviorID {
						return Card{}, fmt.Errorf("card %s: replacement cannot change behavior ID", c.ID)
					}
					b.ID = change.BehaviorID
					c.Behaviors[index] = b
				}
			default:
				return Card{}, fmt.Errorf("card %s: unknown modifier operation %s", c.ID, change.Type)
			}
		}
	}
	c.Modifiers = nil
	return c, nil
}

type modeCatalogProvider interface{ ModeDefinitions() []mode.Definition }

func ModeDefinitions(registry CardRegistry) []mode.Definition {
	if p, ok := registry.(modeCatalogProvider); ok {
		return p.ModeDefinitions()
	}
	return nil
}

// Resolve builds an isolated effective catalog, applying selection before any lookups.
func Resolve(registry CardRegistry, settings shared.GameSettings) (CardRegistry, error) {
	if r, ok := registry.(*InMemoryCardRegistry); ok {
		return r.resolve(settings)
	}
	return resolveUncached(registry, settings)
}
func resolveUncached(registry CardRegistry, settings shared.GameSettings) (CardRegistry, error) {
	result := make([]Card, 0)
	for _, card := range registry.GetAll() {
		if !slices.Contains(card.GameModes, settings.GameModeID) {
			continue
		}
		if card.Type == CardTypePrelude && !settings.HasPrelude() {
			continue
		}
		effective, err := ApplyModeModifiers(card, settings.GameModeID)
		if err != nil {
			return nil, err
		}
		if settings.AllowsContent(effective.GameModes, effective.RequiresSystems, effective.RequiresFeatures) {
			result = append(result, effective)
		}
	}
	resolved := NewInMemoryCardRegistry(result, ModeDefinitions(registry)...)
	resolved.effective = true
	return resolved, nil
}

// ForGame returns the immutable catalog for a validated game selection. Standalone
// registries without a mode catalog are already-scoped domain fixtures.
func ForGame(registry CardRegistry, g *game.Game) CardRegistry {
	if registry == nil || g == nil {
		return registry
	}
	if r, ok := registry.(*InMemoryCardRegistry); ok {
		if r.effective || len(r.modes) == 0 {
			return r
		}
	} else if len(ModeDefinitions(registry)) == 0 {
		return registry
	}
	resolved, err := Resolve(registry, g.Settings())
	if err != nil {
		return failedRegistry{err}
	}
	return resolved
}

type failedRegistry struct{ err error }

func (r failedRegistry) GetByID(string) (*Card, error) { return nil, r.err }
func (r failedRegistry) GetAll() []Card                { return nil }

func GetCardIDs(registry CardRegistry) (projects, corps, preludes []string) {
	for _, card := range registry.GetAll() {
		switch card.Type {
		case CardTypeCorporation:
			corps = append(corps, card.ID)
		case CardTypePrelude:
			preludes = append(preludes, card.ID)
		default:
			projects = append(projects, card.ID)
		}
	}
	return
}
