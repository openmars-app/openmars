package cards

import (
	"context"
	"fmt"
	"log/slog"

	"openmars/internal/game"
	"openmars/internal/game/award"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// CorporationProcessor handles applying corporation card effects
type CorporationProcessor struct {
	cardRegistry  CardRegistryInterface
	awardRegistry award.AwardRegistry
	logger        *slog.Logger
}

// NewCorporationProcessor creates a new corporation processor
func NewCorporationProcessor(cardRegistry CardRegistryInterface, awardRegistry award.AwardRegistry, logger *slog.Logger) *CorporationProcessor {
	return &CorporationProcessor{
		cardRegistry:  cardRegistry,
		awardRegistry: awardRegistry,
		logger:        logger,
	}
}

// ApplyStartingEffects processes ONLY auto-corporation-start behaviors
// and applies starting resources/production
func (p *CorporationProcessor) ApplyStartingEffects(
	ctx context.Context,
	card *Card,
	pl *player.Player,
	g *game.Game,
) error {
	log := p.logger.With(
		slog.String("corporation_id", card.ID),
		slog.String("corporation_name", card.Name),
		slog.String("player_id", pl.ID()),
	)

	pl.Resources().RegisterDependencyGroups(card.ID, card.DependencyGroups)
	log.Debug("Applying corporation starting effects")

	applier := NewBehaviorApplier(pl, g, card.Name, p.logger).
		WithSourceCardID(card.ID).
		WithCardRegistry(p.registryForGame(g))

	// Process ONLY behaviors with auto-corporation-start trigger
	for _, behavior := range card.Behaviors {
		if !g.ModeRequirementsMet(behavior.StateRequirements) {
			continue
		}
		for _, trigger := range behavior.Triggers {
			if trigger.Type == string(ResourceTriggerAutoCorporationStart) && trigger.Condition == nil {
				log.Debug("Found auto-corporation-start behavior",
					slog.Int("outputs", len(behavior.Outputs)))

				if err := applier.WithBehavior(behavior).ApplyOutputs(ctx, behavior.Outputs); err != nil {
					return fmt.Errorf("failed to apply starting effects: %w", err)
				}
			}
		}
	}

	log.Debug("Corporation starting effects applied")
	return nil
}

// ApplyAutoEffects processes auto triggers WITHOUT conditions
// (e.g., payment-substitute for Helion)
func (p *CorporationProcessor) ApplyAutoEffects(
	ctx context.Context,
	card *Card,
	pl *player.Player,
	g *game.Game,
) error {
	log := p.logger.With(
		slog.String("corporation_id", card.ID),
		slog.String("corporation_name", card.Name),
		slog.String("player_id", pl.ID()),
	)

	log.Debug("Applying corporation auto effects")

	applier := NewBehaviorApplier(pl, g, card.Name, p.logger).
		WithSourceCardID(card.ID).
		WithCardRegistry(p.registryForGame(g))

	// Process behaviors with auto trigger WITHOUT conditions
	for _, behavior := range card.Behaviors {
		if !g.ModeRequirementsMet(behavior.StateRequirements) {
			continue
		}
		for _, trigger := range behavior.Triggers {
			// Handle auto trigger WITHOUT conditions (immediate effects like payment-substitute)
			// Auto triggers WITH conditions are passive effects handled separately
			if trigger.Type == string(ResourceTriggerAuto) && trigger.Condition == nil {
				log.Debug("Found auto behavior (no condition)",
					slog.Int("outputs", len(behavior.Outputs)))

				if err := applier.WithBehavior(behavior).ApplyOutputs(ctx, behavior.Outputs); err != nil {
					return fmt.Errorf("failed to apply auto effects: %w", err)
				}
			}
		}
	}

	log.Debug("Corporation auto effects applied")
	return nil
}

// SetupForcedFirstAction queues corporation behaviors until the owner's first turn.
func (p *CorporationProcessor) SetupForcedFirstAction(ctx context.Context, card *Card, g *game.Game, playerID string) error {
	var indices []int
	for i, behavior := range card.Behaviors {
		for _, trigger := range behavior.Triggers {
			if trigger.Type == string(ResourceTriggerAutoCorporationFirstAction) {
				indices = append(indices, i)
				break
			}
		}
	}
	if len(indices) == 0 {
		return nil
	}
	g.SetFirstActionExecutor(playerID, p.executeFirstAction)
	if err := g.SetForcedFirstAction(ctx, playerID, &shared.ForcedFirstAction{
		CorporationID: card.ID, BehaviorIndices: indices, State: "queued", Description: card.Name + " first action",
	}); err != nil {
		return err
	}
	return g.ExecuteFirstActionIfNeeded(ctx, playerID)
}

func (p *CorporationProcessor) executeFirstAction(ctx context.Context, g *game.Game, playerID string) error {
	pending := g.GetForcedFirstAction(playerID)
	card, err := p.registryForGame(g).GetByID(pending.CorporationID)
	if err != nil {
		return err
	}
	pl, err := g.GetPlayer(playerID)
	if err != nil {
		return err
	}
	var outputs []shared.BehaviorCondition
	for _, index := range pending.BehaviorIndices {
		outputs = append(outputs, card.Behaviors[index].Outputs...)
	}
	applier := NewBehaviorApplier(pl, g, card.Name, p.logger).
		WithSourceCardID(card.ID).WithCardRegistry(p.registryForGame(g)).
		WithSourceType(shared.SourceTypeCorporationFirstAction).WithAwardRegistry(p.awardRegistry)
	return applier.ApplyOutputs(ctx, outputs)
}

// GetAutoEffects returns all auto effects (without conditions) from a corporation card
// These are behaviors with auto triggers without conditions (e.g., payment-substitute for Helion)
// They are applied immediately AND registered in effects list for display purposes
// This is a READ-ONLY helper that parses the card behaviors and returns CardEffect structs
// The action layer is responsible for adding these effects to the player
func (p *CorporationProcessor) GetAutoEffects(card *Card) []shared.CardEffect {
	var effects []shared.CardEffect

	// Iterate through all behaviors and find auto triggers without conditions
	for behaviorIndex, behavior := range card.Behaviors {
		for _, trigger := range behavior.Triggers {
			// Auto triggers WITHOUT conditions are immediate/permanent effects
			if trigger.Type == string(ResourceTriggerAuto) && trigger.Condition == nil {
				effect := shared.CardEffect{
					CardID:        card.ID,
					CardName:      card.Name,
					BehaviorIndex: behaviorIndex,
					Behavior:      behavior,
				}
				effects = append(effects, effect)
			}
		}
	}

	return effects
}

// GetTriggerEffects returns all trigger effects (conditional triggers) from a corporation card
// These are behaviors with auto triggers that have conditions, for event subscription
// This is a READ-ONLY helper that parses the card behaviors and returns CardEffect structs
// The action layer is responsible for adding these effects to the player
func (p *CorporationProcessor) GetTriggerEffects(card *Card) []shared.CardEffect {
	var effects []shared.CardEffect

	// Iterate through all behaviors and find conditional triggers
	for behaviorIndex, behavior := range card.Behaviors {
		if HasConditionalTrigger(behavior) {
			effect := shared.CardEffect{
				CardID:        card.ID,
				CardName:      card.Name,
				BehaviorIndex: behaviorIndex,
				Behavior:      behavior,
			}
			effects = append(effects, effect)
		}
	}

	return effects
}

// GetManualActions returns all manual actions (manual triggers) from a corporation card
// This is a READ-ONLY helper that parses the card behaviors and returns CardAction structs
// The action layer is responsible for adding these actions to the player
func (p *CorporationProcessor) GetManualActions(card *Card) []shared.CardAction {
	var actions []shared.CardAction

	// Iterate through all behaviors and find manual triggers
	for behaviorIndex, behavior := range card.Behaviors {
		if HasManualTrigger(behavior) {
			action := shared.CardAction{
				CardID:                  card.ID,
				CardName:                card.Name,
				BehaviorIndex:           behaviorIndex,
				Behavior:                behavior,
				TimesUsedThisTurn:       0,
				TimesUsedThisGeneration: 0,
			}
			actions = append(actions, action)
		}
	}

	return actions
}

// RestoreFirstActionExecutor binds an existing continuation without scheduling or executing it.
func (p *CorporationProcessor) RestoreFirstActionExecutor(g *game.Game, playerID string) {
	g.SetFirstActionExecutor(playerID, p.executeFirstAction)
}

func (p *CorporationProcessor) registryForGame(g *game.Game) CardRegistryInterface {
	if registry, ok := p.cardRegistry.(CardRegistry); ok {
		return ForGame(registry, g)
	}
	return p.cardRegistry
}
