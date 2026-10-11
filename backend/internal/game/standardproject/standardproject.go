package standardproject

import "openmars/internal/game/shared"

// StandardProjectDefinition is the static template loaded from JSON
type StandardProjectDefinition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	GameModes        []string              `json:"gameModes"`
	RequiresSystems  []string              `json:"requiresSystems,omitempty"`
	RequiresFeatures []string              `json:"requiresFeatures,omitempty"`
	Behaviors        []shared.CardBehavior `json:"behaviors"`
	Style            shared.Style          `json:"style"`
}

// CreditCost extracts the credit input amount from behaviors (the cost to execute this project)
func (d *StandardProjectDefinition) CreditCost() int {
	for _, b := range d.Behaviors {
		for _, input := range b.Inputs {
			if input.GetResourceType() == shared.ResourceCredit {
				return input.GetAmount()
			}
		}
	}
	return 0
}
