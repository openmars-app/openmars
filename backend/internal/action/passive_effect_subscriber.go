package action

import (
	"context"
	"log/slog"
	"slices"

	"openmars/internal/events"
	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// SubscribePassiveEffectToEvents subscribes passive effects to relevant domain events
// This function is called when cards with passive effects are played or corporations are selected
func SubscribePassiveEffectToEvents(
	ctx context.Context,
	g *game.Game,
	p *player.Player,
	effect shared.CardEffect,
	log *slog.Logger,
	cardRegistry ...gamecards.CardRegistry,
) {
	var cr gamecards.CardRegistry
	if len(cardRegistry) > 0 {
		cr = gamecards.ForGame(cardRegistry[0], g)
	}
	for _, trigger := range effect.Behavior.Triggers {
		if trigger.Condition == nil || (trigger.Type != string(gamecards.ResourceTriggerAuto) && trigger.Type != string(gamecards.ResourceTriggerAutoCorporationStart)) {
			continue
		}

		var subID events.SubscriptionID

		// Handle placement-bonus-gained trigger
		if trigger.Condition.Type == "placement-bonus-gained" {
			subID = subscribePlacementBonusEffect(ctx, g, p, effect, trigger, log, cr)
		}

		switch trigger.Condition.Type {
		case "production", "state-changed":
			subID = subscribeModeLifecycleEffect(ctx, g, p, effect, trigger, log, cr)
		case "city-placed":
			subID = subscribeTileTypePlacedEffect(ctx, g, p, effect, trigger, log, cr, shared.ResourceCityTile)
		case "ocean-placed":
			subID = subscribeTileTypePlacedEffect(ctx, g, p, effect, trigger, log, cr, shared.ResourceOceanTile)
		case "greenery-placed":
			subID = subscribeTileTypePlacedEffect(ctx, g, p, effect, trigger, log, cr, shared.ResourceGreeneryTile)
		}

		// Handle tag-played trigger
		if trigger.Condition.Type == "tag-played" {
			subID = subscribeTagPlayedEffect(ctx, g, p, effect, trigger, log, cr)
		}

		// Handle card-played trigger with selectors
		if trigger.Condition.Type == "card-played" {
			subID = subscribeCardPlayedEffect(ctx, g, p, effect, trigger, log, cr)
		}

		// Handle standard-project-played trigger
		if trigger.Condition.Type == "standard-project-played" {
			subID = subscribeStandardProjectPlayedEffect(ctx, g, p, effect, trigger, log, cr)
		}

		// Handle tile-placed trigger (production based on placement bonus type)
		if trigger.Condition.Type == "tile-placed" {
			subID = subscribeTilePlacedEffect(ctx, g, p, effect, trigger, log, cr)
		}

		// Handle global-parameter-raised trigger
		if trigger.Condition.Type == "global-parameter-raised" {
			subID = subscribeGlobalParameterRaisedEffect(ctx, g, p, effect, trigger, log, cr)
		}

		// Handle production-increased trigger
		if trigger.Condition.Type == "production-increased" {
			subID = subscribeProductionIncreasedEffect(ctx, g, p, effect, trigger, log, cr)
		}

		// Handle colony-placed trigger
		if trigger.Condition.Type == "colony-placed" {
			subID = subscribeColonyPlacedEffect(ctx, g, p, effect, trigger, log, cr)
		}

		// Register subscription for cleanup when effect is removed
		if subID != "" {
			p.Effects().RegisterSubscription(effect.CardID, subID)
		}
	}
}

// subscribePlacementBonusEffect subscribes to PlacementBonusGainedEvent
func subscribePlacementBonusEffect(
	_ context.Context,
	g *game.Game,
	p *player.Player,
	effect shared.CardEffect,
	trigger shared.Trigger,
	log *slog.Logger,
	cr gamecards.CardRegistry,
) events.SubscriptionID {
	subID := events.Subscribe(g.EventBus(), func(event events.PlacementBonusGainedEvent) {
		if err := gamecards.BehaviorAvailable(g, p, effect.CardID, effect.Behavior, cr); err != nil {
			return
		}
		// Only process if event is for this game and player
		if event.GameID != g.ID() {
			return
		}

		// Check target condition (self-player, any-player, etc.)
		target := "self-player" // Default
		if trigger.Condition.Target != nil {
			target = *trigger.Condition.Target
		}

		if target == "self-player" && event.PlayerID != p.ID() {
			return // Effect only applies to self
		}

		// Check if selectors match the bonus resources
		if len(trigger.Condition.Selectors) > 0 {
			matchFound := false
			for _, sel := range trigger.Condition.Selectors {
				for _, resource := range sel.Resources {
					if _, exists := event.Resources[resource]; exists {
						matchFound = true
						break
					}
				}
				if matchFound {
					break
				}
			}
			if !matchFound {
				return // No matching resources in the bonus
			}
		}

		// Condition matched! Apply the effect outputs using BehaviorApplier
		log.Debug("Passive effect triggered",
			slog.String("card_name", effect.CardName),
			slog.String("trigger_type", trigger.Condition.Type),
			slog.Any("resources_gained", event.Resources))

		applier := gamecards.NewBehaviorApplier(p, g, effect.CardName, slog.Default()).
			WithSourceCardID(effect.CardID).
			WithCardRegistry(cr).
			WithSourceType(shared.SourceTypePassiveEffect).
			WithProductionBox(effect.Behavior.ProductionBox).WithBehavior(effect.Behavior)
		if err := applier.ApplyOutputs(context.Background(), effect.Behavior.Outputs); err != nil {
			log.Error("Failed to apply passive effect outputs",
				slog.String("card_name", effect.CardName),
				slog.Any("error", err))
		}
	})

	log.Debug("Subscribed passive effect to PlacementBonusGainedEvent",
		slog.String("card_name", effect.CardName))

	return subID
}

// subscribeTileTypePlacedEffect handles placement effects for a specific tile type.
func subscribeTileTypePlacedEffect(
	_ context.Context,
	g *game.Game,
	p *player.Player,
	effect shared.CardEffect,
	trigger shared.Trigger,
	log *slog.Logger,
	cr gamecards.CardRegistry,
	tileType shared.ResourceType,
) events.SubscriptionID {
	subID := events.Subscribe(g.EventBus(), func(event events.TilePlacedEvent) {
		if err := gamecards.BehaviorAvailable(g, p, effect.CardID, effect.Behavior, cr); err != nil {
			return
		}
		// Only process if event is for this game
		if event.GameID != g.ID() {
			return
		}

		// Only process the requested tile type
		if event.TileType != string(tileType) {
			return
		}

		// Check target condition (self-player, any-player, etc.)
		target := "self-player" // Default
		if trigger.Condition.Target != nil {
			target = *trigger.Condition.Target
		}

		if target == "self-player" && event.PlayerID != p.ID() {
			return // Effect only applies to self
		}

		// Check location condition
		location := "anywhere" // Default
		if trigger.Condition.Location != nil {
			location = *trigger.Condition.Location
		}

		// For now, we treat all tile placements as "mars" or "anywhere"
		// Future: implement Phobos/colony distinction if needed
		if location != "anywhere" && location != "mars" {
			return // Location doesn't match
		}

		// Condition matched! Apply the effect outputs using BehaviorApplier
		log.Debug("Passive effect triggered (tile placement)",
			slog.String("card_name", effect.CardName),
			slog.String("player_id", p.ID()),
			slog.String("placed_by", event.PlayerID),
			slog.String("tile_type", event.TileType))

		applier := gamecards.NewBehaviorApplier(p, g, effect.CardName, log).
			WithSourceCardID(effect.CardID).
			WithCardRegistry(cr).
			WithSourceType(shared.SourceTypePassiveEffect).
			WithProductionBox(effect.Behavior.ProductionBox).WithBehavior(effect.Behavior)
		if err := applier.ApplyOutputs(context.Background(), effect.Behavior.Outputs); err != nil {
			log.Error("Failed to apply passive effect outputs",
				slog.String("card_name", effect.CardName),
				slog.Any("error", err))
		}
	})

	log.Debug("Subscribed passive effect to TilePlacedEvent",
		slog.String("card_name", effect.CardName))

	return subID
}

func subscribeTagPlayedEffect(
	_ context.Context,
	g *game.Game,
	p *player.Player,
	effect shared.CardEffect,
	trigger shared.Trigger,
	log *slog.Logger,
	cr gamecards.CardRegistry,
) events.SubscriptionID {
	return events.Subscribe(g.EventBus(), tagPlayedEffectHandler(g, p, effect, trigger, log, cr))
}

func tagPlayedEffectHandler(g *game.Game, p *player.Player, effect shared.CardEffect, trigger shared.Trigger, log *slog.Logger, cr gamecards.CardRegistry) func(events.TagPlayedEvent) {
	return func(event events.TagPlayedEvent) {
		if err := gamecards.BehaviorAvailable(g, p, effect.CardID, effect.Behavior, cr); err != nil {
			return
		}
		if event.GameID != g.ID() {
			return
		}

		target := "self-player"
		if trigger.Condition.Target != nil {
			target = *trigger.Condition.Target
		}

		if target == "self-player" && event.PlayerID != p.ID() {
			return
		}

		// Check if selectors match the played tag
		if len(trigger.Condition.Selectors) > 0 {
			matchFound := false
			for _, sel := range trigger.Condition.Selectors {
				for _, tag := range sel.Tags {
					if string(tag) == event.Tag {
						matchFound = true
						break
					}
				}
				if matchFound {
					break
				}
			}
			if !matchFound {
				return
			}
		}

		if trigger.Condition.Unique && cr != nil {
			tag := shared.CardTag(event.Tag)
			if tag == shared.TagWild {
				return
			}
			count := gamecards.CountPlayerTagsByType(p, cr, tag)
			if count != 1 {
				return
			}
		}

		log.Debug("Passive effect triggered (tag played)",
			slog.String("card_name", effect.CardName),
			slog.String("effect_owner", p.ID()),
			slog.String("tag_played_by", event.PlayerID),
			slog.String("tag", event.Tag))

		// Check if this effect requires card-discard input (e.g., Mars University)
		if gamecards.HasCardDiscardInput(effect.Behavior) {
			createPassiveCardDiscard(p, effect, event.CardID, event.PlayerID, log)
			return
		}

		// Check if this effect has choices requiring player selection (e.g., Olympus Conference, Viral Enhancers)
		if gamecards.HasChoices(effect.Behavior) {
			createPassiveBehaviorChoice(p, effect, event.CardID, event.PlayerID, log)
			return
		}

		applier := gamecards.NewBehaviorApplier(p, g, effect.CardName, slog.Default()).
			WithSourceCardID(effect.CardID).
			WithTriggeringCard(event.CardID, event.PlayerID).
			WithCardRegistry(cr).
			WithSourceType(shared.SourceTypePassiveEffect).
			WithProductionBox(effect.Behavior.ProductionBox).WithBehavior(effect.Behavior)
		if err := applier.ApplyOutputs(context.Background(), effect.Behavior.Outputs); err != nil {
			log.Error("Failed to apply passive effect outputs",
				slog.String("card_name", effect.CardName),
				slog.Any("error", err))
		}
	}
}

// ActivateSelfTagTriggers dispatches initial tags only to the newly registered card effects.
// Existing subscribers already received these tags from AddCard and must not receive them again.
func ActivateSelfTagTriggers(g *game.Game, p *player.Player, card *gamecards.Card, registry gamecards.CardRegistry, log *slog.Logger) {
	for _, effect := range p.Effects().List() {
		if effect.CardID != card.ID {
			continue
		}
		for _, trigger := range effect.Behavior.Triggers {
			if trigger.Condition == nil || trigger.Condition.Type != "tag-played" {
				continue
			}
			handler := tagPlayedEffectHandler(g, p, effect, trigger, log, registry)
			for _, tag := range card.Tags {
				handler(events.TagPlayedEvent{GameID: g.ID(), PlayerID: p.ID(), CardID: card.ID, CardName: card.Name, Tag: string(tag)})
			}
		}
	}
}

func subscribeCardPlayedEffect(
	_ context.Context,
	g *game.Game,
	p *player.Player,
	effect shared.CardEffect,
	trigger shared.Trigger,
	log *slog.Logger,
	cr gamecards.CardRegistry,
) events.SubscriptionID {
	subID := events.Subscribe(g.EventBus(), func(event events.CardPlayedEvent) {
		if err := gamecards.BehaviorAvailable(g, p, effect.CardID, effect.Behavior, cr); err != nil {
			return
		}
		if event.GameID != g.ID() {
			return
		}

		target := "self-player"
		if trigger.Condition.Target != nil {
			target = *trigger.Condition.Target
		}

		if target == "self-player" && event.PlayerID != p.ID() {
			return
		}

		if cr == nil {
			return
		}

		card, err := cr.GetByID(event.CardID)
		if err != nil {
			return
		}

		// Check if the card matches any selector
		if len(trigger.Condition.Selectors) > 0 {
			if !gamecards.MatchesAnySelector(card, trigger.Condition.Selectors) {
				return
			}
		}

		log.Debug("Passive effect triggered (card played)",
			slog.String("card_name", effect.CardName),
			slog.String("effect_owner", p.ID()),
			slog.String("card_played_by", event.PlayerID),
			slog.String("card_played", event.CardName))

		if gamecards.HasChoices(effect.Behavior) {
			createPassiveBehaviorChoice(p, effect, event.CardID, event.PlayerID, log)
			return
		}

		applier := gamecards.NewBehaviorApplier(p, g, effect.CardName, slog.Default()).
			WithSourceCardID(effect.CardID).
			WithTriggeringCard(event.CardID, event.PlayerID).
			WithCardRegistry(cr).
			WithSourceType(shared.SourceTypePassiveEffect).
			WithProductionBox(effect.Behavior.ProductionBox).WithBehavior(effect.Behavior)
		if err := applier.ApplyOutputs(context.Background(), effect.Behavior.Outputs); err != nil {
			log.Error("Failed to apply passive effect outputs",
				slog.String("card_name", effect.CardName),
				slog.Any("error", err))
		}
	})

	log.Debug("Subscribed passive effect to CardPlayedEvent",
		slog.String("card_name", effect.CardName))

	return subID
}

func subscribeStandardProjectPlayedEffect(
	_ context.Context,
	g *game.Game,
	p *player.Player,
	effect shared.CardEffect,
	trigger shared.Trigger,
	log *slog.Logger,
	cr gamecards.CardRegistry,
) events.SubscriptionID {
	subID := events.Subscribe(g.EventBus(), func(event events.StandardProjectPlayedEvent) {
		if err := gamecards.BehaviorAvailable(g, p, effect.CardID, effect.Behavior, cr); err != nil {
			return
		}
		if event.GameID != g.ID() {
			return
		}

		target := "self-player"
		if trigger.Condition.Target != nil {
			target = *trigger.Condition.Target
		}

		if target == "self-player" && event.PlayerID != p.ID() {
			return
		}

		// Check selectors for cost and project type matching
		if len(trigger.Condition.Selectors) > 0 {
			matched := false
			for _, sel := range trigger.Condition.Selectors {
				// Check cost requirement
				if sel.RequiredOriginalCost != nil {
					if sel.RequiredOriginalCost.Min != nil && event.ProjectCost < *sel.RequiredOriginalCost.Min {
						continue
					}
					if sel.RequiredOriginalCost.Max != nil && event.ProjectCost > *sel.RequiredOriginalCost.Max {
						continue
					}
				}
				// Check project type match (if specified)
				if len(sel.StandardProjects) > 0 {
					if !gamecards.MatchesStandardProjectSelector(shared.StandardProject(event.ProjectType), sel) {
						continue
					}
				}
				matched = true
				break
			}
			if !matched {
				return
			}
		}

		log.Debug("Passive effect triggered (standard project played)",
			slog.String("card_name", effect.CardName),
			slog.String("effect_owner", p.ID()),
			slog.String("project_type", event.ProjectType),
			slog.Int("project_cost", event.ProjectCost))

		applier := gamecards.NewBehaviorApplier(p, g, effect.CardName, slog.Default()).
			WithSourceCardID(effect.CardID).
			WithCardRegistry(cr).
			WithSourceType(shared.SourceTypePassiveEffect).
			WithProductionBox(effect.Behavior.ProductionBox).WithBehavior(effect.Behavior)
		if err := applier.ApplyOutputs(context.Background(), effect.Behavior.Outputs); err != nil {
			log.Error("Failed to apply passive effect outputs",
				slog.String("card_name", effect.CardName),
				slog.Any("error", err))
		}
	})

	log.Debug("Subscribed passive effect to StandardProjectPlayedEvent",
		slog.String("card_name", effect.CardName))

	return subID
}

func subscribeTilePlacedEffect(
	_ context.Context,
	g *game.Game,
	p *player.Player,
	effect shared.CardEffect,
	trigger shared.Trigger,
	log *slog.Logger,
	cr gamecards.CardRegistry,
) events.SubscriptionID {
	subID := events.Subscribe(g.EventBus(), func(event events.PlacementBonusGainedEvent) {
		if err := gamecards.BehaviorAvailable(g, p, effect.CardID, effect.Behavior, cr); err != nil {
			return
		}
		if event.GameID != g.ID() {
			return
		}

		target := "self-player"
		if trigger.Condition.Target != nil {
			target = *trigger.Condition.Target
		}

		if target == "self-card" {
			if event.SourceCardID != effect.CardID {
				return
			}
		} else if target == "self-player" && event.PlayerID != p.ID() {
			return
		}

		if len(trigger.Condition.OnBonusType) > 0 {
			matchFound := false
			for _, requiredBonus := range trigger.Condition.OnBonusType {
				if _, exists := event.Resources[requiredBonus]; exists {
					matchFound = true
					break
				}
			}
			if !matchFound {
				return
			}
		}

		log.Debug("Passive effect triggered (tile placed on bonus)",
			slog.String("card_name", effect.CardName),
			slog.String("trigger_type", trigger.Condition.Type),
			slog.Any("bonus_resources", event.Resources))

		applier := gamecards.NewBehaviorApplier(p, g, effect.CardName, slog.Default()).
			WithSourceCardID(effect.CardID).
			WithCardRegistry(cr).
			WithSourceType(shared.SourceTypePassiveEffect).
			WithProductionBox(effect.Behavior.ProductionBox).WithBehavior(effect.Behavior)
		if err := applier.ApplyOutputs(context.Background(), effect.Behavior.Outputs); err != nil {
			log.Error("Failed to apply passive effect outputs",
				slog.String("card_name", effect.CardName),
				slog.Any("error", err))
		}
	})

	log.Debug("Subscribed passive effect to PlacementBonusGainedEvent (tile-placed)",
		slog.String("card_name", effect.CardName))

	return subID
}

func subscribeGlobalParameterRaisedEffect(
	_ context.Context,
	g *game.Game,
	p *player.Player,
	effect shared.CardEffect,
	_ shared.Trigger,
	log *slog.Logger,
	cr gamecards.CardRegistry,
) events.SubscriptionID {
	globalParams := getGlobalParametersFromSelectors(effect.Behavior.Triggers)

	applyPerStep := func(steps int, paramName string) {
		if steps <= 0 {
			return
		}
		log.Debug("Passive effect triggered (global parameter raised)",
			slog.String("card_name", effect.CardName),
			slog.String("player_id", p.ID()),
			slog.String("parameter", paramName),
			slog.Int("steps", steps))

		for i := 0; i < steps; i++ {
			applier := gamecards.NewBehaviorApplier(p, g, effect.CardName, slog.Default()).
				WithSourceCardID(effect.CardID).
				WithCardRegistry(cr).
				WithSourceType(shared.SourceTypePassiveEffect).
				WithProductionBox(effect.Behavior.ProductionBox).WithBehavior(effect.Behavior)
			if err := applier.ApplyOutputs(context.Background(), effect.Behavior.Outputs); err != nil {
				log.Error("Failed to apply passive effect outputs",
					slog.String("card_name", effect.CardName),
					slog.Any("error", err))
			}
		}
	}

	if slices.Contains(globalParams, "venus") {
		subID := events.Subscribe(g.EventBus(), func(event events.VenusChangedEvent) {
			if err := gamecards.BehaviorAvailable(g, p, effect.CardID, effect.Behavior, cr); err != nil {
				return
			}
			if event.GameID != g.ID() {
				return
			}
			steps := (event.NewValue - event.OldValue) / 2
			applyPerStep(steps, "venus")
		})
		p.Effects().RegisterSubscription(effect.CardID, subID)
	}

	if slices.Contains(globalParams, "temperature") {
		subID := events.Subscribe(g.EventBus(), func(event events.TemperatureChangedEvent) {
			if err := gamecards.BehaviorAvailable(g, p, effect.CardID, effect.Behavior, cr); err != nil {
				return
			}
			if event.GameID != g.ID() {
				return
			}
			steps := (event.NewValue - event.OldValue) / 2
			applyPerStep(steps, "temperature")
		})
		p.Effects().RegisterSubscription(effect.CardID, subID)
	}

	if slices.Contains(globalParams, "oxygen") {
		subID := events.Subscribe(g.EventBus(), func(event events.OxygenChangedEvent) {
			if err := gamecards.BehaviorAvailable(g, p, effect.CardID, effect.Behavior, cr); err != nil {
				return
			}
			if event.GameID != g.ID() {
				return
			}
			steps := event.NewValue - event.OldValue
			applyPerStep(steps, "oxygen")
		})
		p.Effects().RegisterSubscription(effect.CardID, subID)
	}

	log.Debug("Subscribed passive effect to global parameter raised events",
		slog.String("card_name", effect.CardName),
		slog.Any("parameters", globalParams))

	// Subscriptions are registered internally per parameter, return empty to avoid duplicate registration by caller
	return ""
}

func getGlobalParametersFromSelectors(triggers []shared.Trigger) []string {
	for _, trigger := range triggers {
		if trigger.Condition == nil {
			continue
		}
		for _, sel := range trigger.Condition.Selectors {
			if len(sel.GlobalParameters) > 0 {
				return sel.GlobalParameters
			}
		}
	}
	return nil
}

// createPassiveCardDiscard creates a pending card discard selection from a passive effect
// Used for effects like Mars University that require player to optionally discard before gaining outputs
func createPassiveCardDiscard(p *player.Player, effect shared.CardEffect, triggeringCardID, triggeringPlayerID string, log *slog.Logger) {
	// Find card-discard inputs to determine min/max
	minCards := 0
	maxCards := 0
	for _, input := range effect.Behavior.Inputs {
		if input.GetResourceType() == shared.ResourceCardDiscard {
			if !shared.IsOptional(input) {
				minCards = input.GetAmount()
			}
			maxCards = input.GetAmount()
			break
		}
	}

	p.Selection().AddPendingBehaviorResolution(&shared.PendingBehaviorResolution{Kind: "card-discard", TriggeringCardID: triggeringCardID, TriggeringPlayerID: triggeringPlayerID, SourceBehaviorIndex: effect.BehaviorIndex,
		MinCards:       minCards,
		MaxCards:       maxCards,
		Source:         effect.CardName,
		SourceCardID:   effect.CardID,
		PendingOutputs: effect.Behavior.Outputs,
	})

	log.Debug("Created pending card discard selection from passive effect",
		slog.String("card_name", effect.CardName),
		slog.Int("min_cards", minCards),
		slog.Int("max_cards", maxCards))
}

// resourceNameToProductionType maps event resource names to production resource types
var resourceNameToProductionType = map[string]shared.ResourceType{
	"credits":  shared.ResourceCreditProduction,
	"steel":    shared.ResourceSteelProduction,
	"titanium": shared.ResourceTitaniumProduction,
	"plants":   shared.ResourcePlantProduction,
	"energy":   shared.ResourceEnergyProduction,
	"heat":     shared.ResourceHeatProduction,
}

// subscribeProductionIncreasedEffect subscribes to ProductionChangedEvent for production increase triggers
func subscribeProductionIncreasedEffect(
	_ context.Context,
	g *game.Game,
	p *player.Player,
	effect shared.CardEffect,
	trigger shared.Trigger,
	log *slog.Logger,
	cr gamecards.CardRegistry,
) events.SubscriptionID {
	subID := events.Subscribe(g.EventBus(), func(event events.ProductionChangedEvent) {
		if err := gamecards.BehaviorAvailable(g, p, effect.CardID, effect.Behavior, cr); err != nil {
			return
		}
		if event.GameID != g.ID() {
			return
		}

		target := "self-player"
		if trigger.Condition.Target != nil {
			target = *trigger.Condition.Target
		}

		if target == "self-player" && event.PlayerID != p.ID() {
			return
		}

		increase := event.NewProduction - event.OldProduction
		if increase <= 0 {
			return
		}

		// Check if this production type matches the trigger's resource type filter
		if len(trigger.Condition.ResourceTypes) > 0 {
			productionType, exists := resourceNameToProductionType[event.ResourceType]
			if !exists {
				return
			}
			if !slices.Contains(trigger.Condition.ResourceTypes, productionType) {
				return
			}
		}

		// Scale outputs by the production increase amount
		scaledOutputs := make([]shared.BehaviorCondition, len(effect.Behavior.Outputs))
		for i, output := range effect.Behavior.Outputs {
			scaled := shared.CloneCondition(output)
			scaled.SetAmount(output.GetAmount() * increase)
			scaledOutputs[i] = scaled
		}

		log.Debug("Passive effect triggered",
			slog.String("card_name", effect.CardName),
			slog.String("trigger_type", trigger.Condition.Type),
			slog.String("resource_type", event.ResourceType),
			slog.Int("increase", increase))

		applier := gamecards.NewBehaviorApplier(p, g, effect.CardName, slog.Default()).
			WithSourceCardID(effect.CardID).
			WithCardRegistry(cr).
			WithSourceType(shared.SourceTypePassiveEffect).
			WithProductionBox(effect.Behavior.ProductionBox).WithBehavior(effect.Behavior)
		if err := applier.ApplyOutputs(context.Background(), scaledOutputs); err != nil {
			log.Error("Failed to apply passive effect outputs",
				slog.String("card_name", effect.CardName),
				slog.Any("error", err))
		}
	})

	log.Debug("Subscribed passive effect to ProductionChangedEvent",
		slog.String("card_name", effect.CardName))

	return subID
}

// createPassiveBehaviorChoice creates a pending behavior choice selection from a passive effect
// Used for effects like Viral Enhancers and Olympus Conference that require player to choose between options
func createPassiveBehaviorChoice(p *player.Player, effect shared.CardEffect, triggeringCardID, triggeringPlayerID string, log *slog.Logger) {
	p.Selection().AddPendingBehaviorResolution(&shared.PendingBehaviorResolution{Kind: "choice", TriggeringCardID: triggeringCardID, TriggeringPlayerID: triggeringPlayerID, SourceBehaviorIndex: effect.BehaviorIndex,
		Choices:      effect.Behavior.Choices,
		Source:       effect.CardName,
		SourceCardID: effect.CardID,
	})

	log.Debug("Created pending behavior choice selection from passive effect",
		slog.String("card_name", effect.CardName),
		slog.Int("num_choices", len(effect.Behavior.Choices)))
}

// subscribeColonyPlacedEffect subscribes to ColonyBuiltEvent for colony-placed triggers
func subscribeColonyPlacedEffect(
	_ context.Context,
	g *game.Game,
	p *player.Player,
	effect shared.CardEffect,
	trigger shared.Trigger,
	log *slog.Logger,
	cr gamecards.CardRegistry,
) events.SubscriptionID {
	subID := events.Subscribe(g.EventBus(), func(event events.ColonyBuiltEvent) {
		if err := gamecards.BehaviorAvailable(g, p, effect.CardID, effect.Behavior, cr); err != nil {
			return
		}
		if event.GameID != g.ID() {
			return
		}

		target := "self-player"
		if trigger.Condition.Target != nil {
			target = *trigger.Condition.Target
		}

		if target == "self-player" && event.PlayerID != p.ID() {
			return
		}

		log.Debug("Passive effect triggered (colony placed)",
			slog.String("card_name", effect.CardName),
			slog.String("player_id", p.ID()),
			slog.String("placed_by", event.PlayerID),
			slog.String("colony_id", event.ColonyID))

		applier := gamecards.NewBehaviorApplier(p, g, effect.CardName, slog.Default()).
			WithSourceCardID(effect.CardID).
			WithCardRegistry(cr).
			WithSourceType(shared.SourceTypePassiveEffect).
			WithProductionBox(effect.Behavior.ProductionBox).WithBehavior(effect.Behavior)
		if err := applier.ApplyOutputs(context.Background(), effect.Behavior.Outputs); err != nil {
			log.Error("Failed to apply passive effect outputs",
				slog.String("card_name", effect.CardName),
				slog.Any("error", err))
		}
	})

	log.Debug("Subscribed passive effect to ColonyBuiltEvent",
		slog.String("card_name", effect.CardName))

	return subID
}
