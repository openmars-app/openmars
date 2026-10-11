package action

import (
	"context"
	"log/slog"
	"openmars/internal/events"
	"openmars/internal/game"
	"openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

func subscribeModeLifecycleEffect(ctx context.Context, g *game.Game, p *player.Player, effect shared.CardEffect, trigger shared.Trigger, log *slog.Logger, registry cards.CardRegistry) events.SubscriptionID {
	apply := func() {
		if err := cards.BehaviorAvailable(g, p, effect.CardID, effect.Behavior, registry); err != nil {
			return
		}
		applier := cards.NewBehaviorApplier(p, g, effect.CardName, log).WithCardRegistry(registry).WithSourceCardID(effect.CardID).WithSourceType(shared.SourceTypePassiveEffect).WithBehavior(effect.Behavior)
		if err := applier.ApplyOutputs(ctx, effect.Behavior.Outputs); err != nil {
			log.Warn("Mode lifecycle effect failed", slog.Any("error", err))
		}
	}
	if trigger.Condition.Type == "production" {
		return events.Subscribe(g.EventBus(), func(e events.ProductionStartingEvent) {
			if e.GameID == g.ID() {
				apply()
			}
		})
	}
	return events.Subscribe(g.EventBus(), func(e events.ModeStateChangedEvent) {
		if e.GameID != g.ID() || e.State != trigger.Condition.State {
			return
		}
		if trigger.Condition.Target != nil && *trigger.Condition.Target == "self-player" && e.PlayerID != p.ID() {
			return
		}
		if trigger.Condition.Direction == "increase" && e.New <= e.Old || trigger.Condition.Direction == "decrease" && e.New >= e.Old {
			return
		}
		if trigger.Condition.To != nil && e.New != *trigger.Condition.To {
			return
		}
		apply()
	})
}

// SetupModeBehaviors registers the mode's automatic rules against the shared event stream.
func SetupModeBehaviors(ctx context.Context, g *game.Game, registry cards.CardRegistry) {
	if !g.ClaimModeBehaviorsRegistration() {
		return
	}
	registry = cards.ForGame(registry, g)
	for _, behavior := range g.ModeRuntime().Definition.Behaviors {
		b := behavior
		apply := func() {
			applyFor := func(p *player.Player) {
				applier := cards.NewBehaviorApplier(p, g, b.Description, slog.Default()).WithCardRegistry(registry).WithSourceType(shared.SourceTypePassiveEffect).WithBehavior(b)
				if err := applier.ApplyOutputs(ctx, b.Outputs); err != nil {
					slog.Warn("Mode behavior failed", slog.String("behavior_id", b.ID), slog.Any("error", err))
				}
			}
			if b.Scope == "each-player" {
				for _, p := range g.GetAllPlayers() {
					if !p.HasExited() {
						applyFor(p)
					}
				}
			} else {
				applyFor(nil)
			}
		}
		for _, trigger := range b.Triggers {
			if trigger.Condition == nil {
				continue
			}
			t := trigger.Condition
			switch t.Type {
			case "production":
				events.Subscribe(g.EventBus(), func(e events.ProductionStartingEvent) {
					if e.GameID == g.ID() {
						apply()
					}
				})
			case "state-changed":
				events.Subscribe(g.EventBus(), func(e events.ModeStateChangedEvent) {
					if e.GameID == g.ID() && e.State == t.State && (t.To == nil || *t.To == e.New) && (t.Direction != "increase" || e.New > e.Old) && (t.Direction != "decrease" || e.New < e.Old) {
						apply()
					}
				})
			}
		}
	}
}

// BeginModeGeneration grants the shared income to every active player together.
func BeginModeGeneration(ctx context.Context, g *game.Game) {
	rule, amount := g.BeginModeGeneration()
	if rule == nil || amount == 0 {
		return
	}
	for _, p := range g.GetAllPlayers() {
		if p.HasExited() {
			continue
		}
		applier := cards.NewBehaviorApplier(p, g, rule.Name, slog.Default()).WithSourceType(shared.SourceTypePassiveEffect)
		if err := applier.ApplyOutputs(ctx, []shared.BehaviorCondition{shared.NewBasicResourceCondition(rule.Resource, amount, "self-player")}); err != nil {
			slog.Warn("Generation income failed", slog.Any("error", err))
		}
	}
}
