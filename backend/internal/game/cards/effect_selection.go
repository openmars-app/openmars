package cards

import (
	"fmt"
	"openmars/internal/game"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"slices"
)

// EffectSelectionOptions returns legal complete source/target assignments for deferred outputs.
func EffectSelectionOptions(outputs []shared.BehaviorCondition, p *player.Player, g *game.Game, registry CardRegistryInterface, colonies ColonyBonusLookup) ([]shared.EffectSelectionOption, bool, error) {
	for _, output := range outputs {
		if copy, ok := output.(*shared.CopyCondition); ok {
			if len(outputs) != 1 {
				return nil, true, fmt.Errorf("copy must be a separate behavior")
			}
			options, err := productionCopyOptions(copy, p, g, registry)
			return options, true, err
		}
	}
	var steps []*shared.ColonyCondition
	for _, output := range outputs {
		if output.GetResourceType() == shared.ResourceColonyTrackStep {
			steps = append(steps, output.(*shared.ColonyCondition))
		}
	}
	if len(steps) == 0 {
		return nil, false, nil
	}
	if len(steps) != len(outputs) {
		return nil, true, fmt.Errorf("colony track selection must be a separate behavior")
	}
	if colonies == nil {
		return nil, true, fmt.Errorf("colony definitions are required for track movement")
	}
	var options []shared.EffectSelectionOption
	var visit func([]string)
	visit = func(ids []string) {
		if len(ids) == len(steps) {
			moves := map[string]int{}
			for i, id := range ids {
				moves[id] += steps[i].Amount
			}
			for id, amount := range moves {
				state := g.Colonies().GetState(id)
				definition, err := colonies.GetByID(id)
				if err != nil || state.MarkerPosition+amount < len(state.PlayerColonies) || state.MarkerPosition+amount >= len(definition.Steps) {
					return
				}
			}
			options = append(options, shared.EffectSelectionOption{ColonyIDs: append([]string(nil), ids...)})
			return
		}
		step := steps[len(ids)]
		for _, state := range g.Colonies().States() {
			if state.AwaitingResource != "" {
				continue
			}
			duplicate := false
			for j, id := range ids {
				if id == state.DefinitionID && step.SelectionGroup != "" && steps[j].SelectionGroup == step.SelectionGroup {
					duplicate = true
				}
			}
			if duplicate {
				continue
			}
			definition, err := colonies.GetByID(state.DefinitionID)
			if err != nil {
				continue
			}
			position := state.MarkerPosition + step.Amount
			if position < len(state.PlayerColonies) || position >= len(definition.Steps) {
				continue
			}
			visit(append(ids, state.DefinitionID))
		}
	}
	visit(nil)
	return options, true, nil
}

func productionCopyOptions(copy *shared.CopyCondition, p *player.Player, g *game.Game, registry CardRegistryInterface) ([]shared.EffectSelectionOption, error) {
	if copy.Scope != "production-box" || copy.Zone != "played" || copy.Target != "self-player" || copy.Amount != 1 {
		return nil, fmt.Errorf("unsupported copy specification")
	}
	if registry == nil {
		return nil, fmt.Errorf("copy requires a card registry")
	}
	ids := append([]string(nil), p.PlayedCards().Cards()...)
	if corp := p.CorporationID(); corp != "" && !slices.Contains(ids, corp) {
		ids = append(ids, corp)
	}
	var options []shared.EffectSelectionOption
	for _, id := range ids {
		card, err := registry.GetByID(id)
		if err != nil || (len(copy.Selectors) > 0 && !MatchesAnySelector(card, copy.Selectors)) {
			continue
		}
		var outputs []shared.BehaviorCondition
		needsTarget := false
		for _, behavior := range card.Behaviors {
			if behavior.ProductionBox != "evaluate" {
				continue
			}
			for _, output := range behavior.Outputs {
				production, ok := output.(*shared.ProductionCondition)
				if !ok {
					return nil, fmt.Errorf("production box on %s contains a non-production output", id)
				}
				clone := shared.CloneCondition(production).(*shared.ProductionCondition)
				if behavior.Dependency != "" {
					d, err := DependencyForBehavior(card, behavior)
					if err != nil {
						return nil, err
					}
					clone.Dependency = d
				}
				if clone.Per != nil {
					if clone.Per.Amount <= 0 {
						return nil, fmt.Errorf("production count divisor must be positive")
					}
					clone.Amount *= CountPerCondition(clone.Per, id, p, g.Board(), registry, g.GetAllPlayers(), g.Colonies(), TagCountContext{ActorID: p.ID()}) / clone.Per.Amount
					clone.Per = nil
				}
				outputs = append(outputs, clone)
			}
		}
		for _, resolved := range p.Resources().ResolvedProductionBox(id) {
			outputs = append(outputs, shared.CloneCondition(&resolved))
		}
		if len(outputs) == 0 {
			continue
		}
		for _, output := range outputs {
			if output.GetTarget() == "any-player" {
				needsTarget = true
			}
		}
		targets := []*player.Player{p}
		if needsTarget {
			targets = g.GetAllPlayers()
		}
		for _, target := range targets {
			if !canApplyProductionBox(outputs, p, target) {
				continue
			}
			option := shared.EffectSelectionOption{CardID: id, Outputs: outputs}
			if needsTarget {
				option.TargetPlayerID = target.ID()
			}
			options = append(options, option)
		}
	}
	return options, nil
}

func canApplyProductionBox(outputs []shared.BehaviorCondition, p, target *player.Player) bool {
	totals := map[*player.Player]map[shared.ResourceType]int{}
	for _, output := range outputs {
		owner := p
		if output.GetTarget() == "any-player" {
			owner = target
		}
		if totals[owner] == nil {
			totals[owner] = map[shared.ResourceType]int{}
		}
		totals[owner][output.GetResourceType()] += output.GetAmount()
	}
	for owner, changes := range totals {
		for resource, amount := range changes {
			floor := 0
			if resource == shared.ResourceCreditProduction {
				floor = -5
			}
			if owner.Resources().PermanentProduction().GetAmount(resource)+amount < floor {
				return false
			}
		}
	}
	return true
}

// SameEffectSelection compares the identity of a source/target assignment.
func SameEffectSelection(a, b shared.EffectSelectionOption) bool {
	return a.CardID == b.CardID && a.TargetPlayerID == b.TargetPlayerID && slices.Equal(a.ColonyIDs, b.ColonyIDs)
}
