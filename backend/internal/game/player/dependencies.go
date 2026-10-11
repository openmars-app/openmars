package player

import (
	"fmt"
	"openmars/internal/events"
	"openmars/internal/game/datastore"
	"openmars/internal/game/shared"
	"time"
)

// DependencyStatus returns the status of an owned ongoing operation.
func (r *PlayerResources) DependencyStatus(cardID, dependency string) shared.DependencyStatus {
	var status shared.DependencyStatus
	r.read(func(s *datastore.PlayerState) {
		status = s.DependencyStatuses[shared.DependencyKey(cardID, dependency)]
	})
	return status
}

// DependencyActive evaluates an ongoing operation against the current game state.
func (r *PlayerResources) DependencyActive(cardID string, d shared.DependencyGroup) bool {
	var runtime shared.ModeRuntimeState
	_ = r.ds.ReadGame(r.gameID, func(s *datastore.GameState) { runtime = s.ModeRuntime })
	return shared.DependencyEnabled(runtime, d, r.DependencyStatus(cardID, d.ID))
}

// AddDependentProduction records a signed contribution without modifying permanent production.
func (r *PlayerResources) AddDependentProduction(c shared.ProductionContribution) {
	old := r.Production()
	r.update(func(s *datastore.PlayerState) { s.ProductionContributions = append(s.ProductionContributions, c) })
	r.publishProductionDifference(old, r.Production())
	events.Publish(r.eventBus, events.GameStateChangedEvent{GameID: r.gameID})
}

// SetDependencyStatus permanently imports or retires a complete ongoing operation.
func (r *PlayerResources) SetDependencyStatus(cardID, dependency string, remove bool) error {
	old := r.PermanentProduction()
	status := r.DependencyStatus(cardID, dependency)
	if status.Removed || status.Retired {
		return fmt.Errorf("dependency is already removed or retired")
	}
	r.update(func(s *datastore.PlayerState) {
		if s.DependencyStatuses == nil {
			s.DependencyStatuses = map[string]shared.DependencyStatus{}
		}
		s.DependencyStatuses[shared.DependencyKey(cardID, dependency)] = shared.DependencyStatus{Removed: remove, Retired: !remove}
		if remove {
			for _, c := range s.ProductionContributions {
				if c.CardID == cardID && c.Dependency.ID == dependency {
					addProductionValue(&s.Production, c.Resource, c.Amount)
				}
			}
		}
	})
	r.publishProductionDifference(old, r.PermanentProduction())
	events.Publish(r.eventBus, events.GameStateChangedEvent{GameID: r.gameID})
	return nil
}

// PermanentProduction excludes supply that can later cease.
func (r *PlayerResources) PermanentProduction() shared.Production {
	var p shared.Production
	r.read(func(s *datastore.PlayerState) { p = s.Production })
	return p
}

func addProductionValue(p *shared.Production, resource shared.ResourceType, amount int) {
	switch resource {
	case shared.ResourceCreditProduction:
		p.Credits += amount
	case shared.ResourceSteelProduction:
		p.Steel += amount
	case shared.ResourceTitaniumProduction:
		p.Titanium += amount
	case shared.ResourcePlantProduction:
		p.Plants += amount
	case shared.ResourceEnergyProduction:
		p.Energy += amount
	case shared.ResourceHeatProduction:
		p.Heat += amount
	}
}

// BehaviorUses returns successful capped executions during a generation.
func (r *PlayerResources) BehaviorUses(cardID, behaviorID string, generation int) int {
	var n int
	r.read(func(s *datastore.PlayerState) {
		n = s.BehaviorUses[fmt.Sprintf("%s:%s:%d", cardID, behaviorID, generation)]
	})
	return n
}

// RecordBehaviorUse records one successful capped execution.
func (r *PlayerResources) RecordBehaviorUse(cardID, behaviorID string, generation int) {
	r.update(func(s *datastore.PlayerState) {
		if s.BehaviorUses == nil {
			s.BehaviorUses = map[string]int{}
		}
		s.BehaviorUses[fmt.Sprintf("%s:%s:%d", cardID, behaviorID, generation)]++
	})
	events.Publish(r.eventBus, events.GameStateChangedEvent{GameID: r.gameID})
}

// RegisterDependencyGroups records immutable source definitions for passive evaluation.
func (r *PlayerResources) RegisterDependencyGroups(cardID string, groups []shared.DependencyGroup) {
	if len(groups) == 0 {
		return
	}
	r.update(func(s *datastore.PlayerState) {
		if s.DependencyGroups == nil {
			s.DependencyGroups = map[string]shared.DependencyGroup{}
		}
		for _, d := range groups {
			s.DependencyGroups[shared.DependencyKey(cardID, d.ID)] = d
		}
	})
}

// ProductionContributions returns source records for presentation and copying.
func (r *PlayerResources) ProductionContributions() []shared.ProductionContribution {
	var out []shared.ProductionContribution
	r.read(func(s *datastore.PlayerState) { out = append(out, s.ProductionContributions...) })
	return out
}

func (r *PlayerResources) publishProductionDifference(old, current shared.Production) {
	for _, pair := range []struct {
		resource shared.ResourceType
		name     string
	}{{shared.ResourceCreditProduction, "credits"}, {shared.ResourceSteelProduction, "steel"}, {shared.ResourceTitaniumProduction, "titanium"}, {shared.ResourcePlantProduction, "plants"}, {shared.ResourceEnergyProduction, "energy"}, {shared.ResourceHeatProduction, "heat"}} {
		before, after := old.GetAmount(pair.resource), current.GetAmount(pair.resource)
		if before != after {
			events.Publish(r.eventBus, events.ProductionChangedEvent{GameID: r.gameID, PlayerID: r.playerID, ResourceType: pair.name, OldProduction: before, NewProduction: after, Timestamp: time.Now()})
		}
	}
}

// EffectiveDependencyStatus includes an irreversible retirement declared by the source.
func (r *PlayerResources) EffectiveDependencyStatus(cardID string, d shared.DependencyGroup) shared.DependencyStatus {
	status := r.DependencyStatus(cardID, d.ID)
	if !status.Removed && d.RetireOnState != nil {
		_ = r.ds.ReadGame(r.gameID, func(s *datastore.GameState) {
			value, ok := s.ModeRuntime.Values[d.RetireOnState.State]
			status.Retired = status.Retired || (ok && value == d.RetireOnState.Equals)
		})
	}
	return status
}
