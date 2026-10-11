package award

import (
	"sort"

	"openmars/internal/game/shared"
)

// AwardDefinition is the static template loaded from JSON
type AwardDefinition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	GameModes        []string              `json:"gameModes"`
	RequiresSystems  []string              `json:"requiresSystems,omitempty"`
	RequiresFeatures []string              `json:"requiresFeatures,omitempty"`
	Costs            []AwardCost           `json:"costs"`
	Rewards          []AwardReward         `json:"rewards"`
	Quantifier       []shared.PerCondition `json:"quantifier"`
	Style            shared.Style          `json:"style"`
}

// AwardCost defines the funding cost at a given number of already-funded awards
type AwardCost struct {
	AwardsBought int `json:"awardsBought"`
	Cost         int `json:"cost"`
}

// AwardReward defines VP output for a placement
type AwardReward struct {
	Place   int            `json:"place"`
	Outputs []RewardOutput `json:"outputs"`
}

// RewardOutput represents a single reward (e.g., VP)
type RewardOutput struct {
	Type   string `json:"type"`
	Amount int    `json:"amount"`
}

// GetCostForFundedCount returns the funding cost given the number of currently funded awards.
// Finds the cost entry with the highest awardsBought <= fundedCount.
func (d *AwardDefinition) GetCostForFundedCount(fundedCount int) int {
	if len(d.Costs) == 0 {
		return 0
	}

	sorted := make([]AwardCost, len(d.Costs))
	copy(sorted, d.Costs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].AwardsBought < sorted[j].AwardsBought
	})

	result := sorted[0].Cost
	for _, c := range sorted {
		if c.AwardsBought <= fundedCount {
			result = c.Cost
		} else {
			break
		}
	}
	return result
}

// GetRewardVP returns the total VP for a given placement from the rewards array
func (d *AwardDefinition) GetRewardVP(place int) int {
	for _, r := range d.Rewards {
		if r.Place == place {
			vp := 0
			for _, o := range r.Outputs {
				if o.Type == "vp" {
					vp += o.Amount
				}
			}
			return vp
		}
	}
	return 0
}
