package cards

import (
	"context"
	"fmt"
	"openmars/internal/game"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// DependencyForBehavior resolves a behavior's authored operation.
func DependencyForBehavior(card *Card, b shared.CardBehavior) (*shared.DependencyGroup, error) {
	if b.Dependency == "" {
		return nil, nil
	}
	for _, d := range card.DependencyGroups {
		if d.ID == b.Dependency {
			return &d, nil
		}
	}
	return nil, fmt.Errorf("unknown dependency %s on %s", b.Dependency, card.ID)
}

// BehaviorAvailable checks dynamic conditions and per-generation execution caps.
func BehaviorAvailable(g *game.Game, p *player.Player, cardID string, b shared.CardBehavior, registry CardRegistryInterface) error {
	if g == nil {
		if len(b.StateRequirements) > 0 || b.Dependency != "" {
			return fmt.Errorf("behavior requires game state")
		}
		return nil
	}
	if err := g.ValidateModeRequirements(b.StateRequirements); err != nil {
		return err
	}
	if b.Dependency != "" {
		if p == nil || registry == nil {
			return fmt.Errorf("dependency requires an owned source")
		}
		c, err := registry.GetByID(cardID)
		if err != nil {
			return err
		}
		d, err := DependencyForBehavior(c, b)
		if err != nil {
			return err
		}
		installation := false
		for _, trigger := range b.Triggers {
			if trigger.Condition == nil && (trigger.Type == string(ResourceTriggerAuto) || trigger.Type == string(ResourceTriggerAutoCorporationStart)) {
				installation = true
			}
		}
		if !installation && !p.Resources().DependencyActive(cardID, *d) {
			if p.Resources().DependencyStatus(cardID, d.ID).Retired {
				return fmt.Errorf("This operation has been shut down")
			}
			if err := g.ValidateModeRequirements([]shared.StateRequirement{{State: d.State, Equals: d.Equals}}); err != nil {
				return err
			}
			return fmt.Errorf("This operation is no longer available")
		}
	}
	if p != nil {
		if err := ValidateBehaviorResources(p, cardID, b.ResourceRequirements, g); err != nil {
			return err
		}
	}
	usageID := b.ID
	if b.UsageGroup != "" {
		usageID = b.UsageGroup
	}
	if p != nil && b.MaxUsesPerGame > 0 && p.Resources().BehaviorUses(cardID, b.ID, 0) >= b.MaxUsesPerGame {
		return fmt.Errorf("behavior game limit reached")
	}
	if p != nil && b.MaxUsesPerGeneration > 0 && p.Resources().BehaviorUses(cardID, usageID, g.Generation()) >= b.MaxUsesPerGeneration {
		return fmt.Errorf("behavior generation limit reached")
	}
	return nil
}

// WithBehavior retains the source behavior for dependency and cap execution.
func (a *BehaviorApplier) WithBehavior(b shared.CardBehavior) *BehaviorApplier {
	a.behavior = &b
	a.productionBox = b.ProductionBox
	a.behaviorValidated = false
	return a
}

// ValidateModeOutputs validates every state operation before paying or applying anything.
func (a *BehaviorApplier) ValidateModeOutputs(outputs []shared.BehaviorCondition) error {
	if a.behavior != nil && !a.behaviorValidated {
		if err := BehaviorAvailable(a.game, a.player, a.sourceCardID, *a.behavior, a.cardRegistry); err != nil {
			return err
		}
	}
	targetIndex := 0
	for _, output := range outputs {
		o, ok := output.(*shared.StateCondition)
		if !ok {
			if output.GetTarget() == "any-card" {
				targetIndex++
			}
			continue
		}
		if a.game == nil {
			return fmt.Errorf("state operation requires game")
		}
		switch o.ResourceType {
		case shared.ResourceStateChange:
			if a.player == nil {
				return fmt.Errorf("state change needs player")
			}
			if err := a.game.ValidateModeStateChange(a.player.ID(), o.State, o.Amount); err != nil {
				return err
			}
		case shared.ResourceScheduleStateWindow:
			if err := a.game.ValidateStateWindow(o); err != nil {
				return err
			}
		case shared.ResourceRemoveDependency, shared.ResourceRetireDependency:
			id := a.sourceCardID
			if o.Target == "any-card" {
				if targetIndex >= len(a.targetCardIDs) {
					return fmt.Errorf("select an eligible project")
				}
				id = a.targetCardIDs[targetIndex]
				targetIndex++
			}
			if err := ValidateDependencyTarget(a.player, a.cardRegistry, id, o.Dependency, o.ResourceType == shared.ResourceRemoveDependency, o.MinimumProduction); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown state operation %s", o.ResourceType)
		}
	}
	return nil
}

// ValidateDependencyTarget checks explicit eligibility, ownership and instance state.
func ValidateDependencyTarget(p *player.Player, registry CardRegistryInterface, id, dependency string, importing bool, thresholds ...*shared.ProductionThreshold) error {
	if p == nil || registry == nil || !p.PlayedCards().Contains(id) {
		return fmt.Errorf("select an owned played project")
	}
	c, err := registry.GetByID(id)
	if err != nil {
		return err
	}
	if c.Type == CardTypeCorporation || c.Type == CardTypePrelude || c.Type == CardTypeEvent {
		return fmt.Errorf("target must be an ongoing project")
	}
	for _, d := range c.DependencyGroups {
		if d.ID != dependency {
			continue
		}
		status := p.Resources().EffectiveDependencyStatus(id, d)
		if status.Removed || status.Retired {
			return fmt.Errorf("operation is already imported or retired")
		}
		if importing && !d.Importable {
			return fmt.Errorf("operation cannot be imported")
		}
		if importing && !p.Resources().DependencyActive(id, d) {
			return fmt.Errorf("operation must be connected to import")
		}
		for _, threshold := range thresholds {
			if threshold != nil {
				total := 0
				for _, c := range p.Resources().ProductionContributions() {
					if c.CardID == id && c.Dependency.ID == dependency && c.Resource == threshold.Type {
						total += c.Amount
					}
				}
				if total < threshold.Amount {
					return fmt.Errorf("target dependency has insufficient production")
				}
			}
		}
		return nil
	}
	return fmt.Errorf("target does not have the required dependency")
}

func (a *BehaviorApplier) applyStateOutput(ctx context.Context, o *shared.StateCondition) error {
	switch o.ResourceType {
	case shared.ResourceStateChange:
		return a.game.ChangeModeState(ctx, a.player.ID(), o.State, o.Amount)
	case shared.ResourceScheduleStateWindow:
		return a.game.ScheduleStateWindow(o, a.sourceCardID)
	case shared.ResourceRemoveDependency, shared.ResourceRetireDependency:
		id := a.sourceCardID
		if o.Target == "any-card" {
			id = a.nextTargetCardID()
		}
		if err := ValidateDependencyTarget(a.player, a.cardRegistry, id, o.Dependency, o.ResourceType == shared.ResourceRemoveDependency, o.MinimumProduction); err != nil {
			return err
		}
		return a.player.Resources().SetDependencyStatus(id, o.Dependency, o.ResourceType == shared.ResourceRemoveDependency)
	}
	return fmt.Errorf("unknown mode output %s", o.ResourceType)
}

// ValidateBehaviorResources evaluates authored availability constraints.
func ValidateBehaviorResources(p *player.Player, cardID string, reqs []shared.BehaviorResourceRequirement, g *game.Game) error {
	for _, r := range reqs {
		amount := p.Resources().Get().GetAmount(r.Type)
		if r.Type == shared.ResourceCityPlacement || r.Type == shared.ResourceCityTile {
			if g == nil {
				return fmt.Errorf("city requirement requires game")
			}
			var owner *string
			if r.Target == "self-player" {
				id := p.ID()
				owner = &id
			}
			var location *string
			if r.Location != "" {
				location = &r.Location
			}
			amount = CountTilesOfTypeByLocation(g.Board(), shared.ResourceCityTile, location, owner)
		}
		if r.Target == "self-card" {
			amount = p.Resources().GetCardStorage(cardID)
		} else if shared.IsProductionResourceType(r.Type) {
			if r.Permanent {
				amount = p.Resources().PermanentProduction().GetAmount(r.Type)
			} else {
				amount = p.Resources().Production().GetAmount(r.Type)
			}
		}
		if r.Min != nil && amount < *r.Min || r.Max != nil && amount > *r.Max {
			return fmt.Errorf("%s requirement not met", r.Type)
		}
	}
	return nil
}
