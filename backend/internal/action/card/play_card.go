package card

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	baseaction "openmars/internal/action"

	"openmars/internal/events"
	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// PlayCardAction handles the business logic for playing a project card from hand
// Card playing involves: validating requirements, calculating costs (with discounts),
// moving card to played cards, applying immediate effects, and deducting payment
type PlayCardAction struct {
	baseaction.BaseAction
	colonyBonusLookup gamecards.ColonyBonusLookup
}

// NewPlayCardAction creates a new play card action
func NewPlayCardAction(
	gameRepo game.GameRepository,
	cardRegistry gamecards.CardRegistry,
	stateRepo game.GameStateRepository,
	logger *slog.Logger,
	colonyBonusLookup ...gamecards.ColonyBonusLookup,
) *PlayCardAction {
	var lookup gamecards.ColonyBonusLookup
	if len(colonyBonusLookup) > 0 {
		lookup = colonyBonusLookup[0]
	}
	return &PlayCardAction{
		BaseAction:        baseaction.NewBaseActionWithStateRepo(gameRepo, cardRegistry, stateRepo),
		colonyBonusLookup: lookup,
	}
}

// Execute performs the play card action
func (a *PlayCardAction) Execute(
	ctx context.Context,
	gameID string,
	playerID string,
	cardID string,
	payment shared.Payment,
	choiceIndex *int,
	cardStorageTargets []string,
	targetPlayerID *string,
	selectedAmount *int,
	cardStorageSources []string,
) error {
	log := a.InitLogger(gameID, playerID).With(
		slog.String("card_id", cardID),
		slog.String("action", "play_card"),
	)
	if choiceIndex != nil {
		log = log.With(slog.Int("choice_index", *choiceIndex))
	}
	if len(cardStorageTargets) > 0 {
		log = log.With(slog.Any("card_storage_targets", cardStorageTargets))
	}
	if targetPlayerID != nil {
		log = log.With(slog.String("target_player_id", *targetPlayerID))
	}
	log.Debug("Player attempting to play card")

	g, err := baseaction.ValidateActiveGame(ctx, a.GameRepository(), gameID, log)
	if err != nil {
		return err
	}

	if err := baseaction.ValidateGamePhase(g, shared.GamePhaseAction, log); err != nil {
		return err
	}

	if err := baseaction.ValidateCurrentTurn(g, playerID, log); err != nil {
		return err
	}

	if err := baseaction.ValidateActionsRemaining(g, playerID, log); err != nil {
		return err
	}

	if err := baseaction.ValidateNoPendingSelections(g, playerID, log); err != nil {
		return err
	}

	player, err := a.GetPlayerFromGame(g, playerID, log)
	if err != nil {
		return err
	}

	// Collect temporary "next-card" effect card IDs BEFORE playing
	// so we can clean them up after this card is played (but not any new ones created by this card)
	prePlayTemporaryCardIDs := collectTemporaryEffectCardIDs(player, shared.TemporaryNextCard)

	if !player.Hand().HasCard(cardID) {
		log.Error("Card not in player's hand")
		return fmt.Errorf("card %s not in hand", cardID)
	}

	card, err := a.CardRegistryForGame(g).GetByID(cardID)
	if err != nil {
		log.Error("Card not found in registry", slog.Any("error", err))
		return fmt.Errorf("card not found: %w", err)
	}

	log.Debug("Card data retrieved",
		slog.String("card_name", card.Name),
		slog.Int("base_cost", card.Cost))

	calculator := gamecards.NewRequirementModifierCalculator(a.CardRegistryForGame(g))

	if err := validateCardRequirements(card, g, player, calculator, a.CardRegistryForGame(g)); err != nil {
		log.Error("Card requirements not met", slog.Any("error", err))
		return fmt.Errorf("cannot play card: %w", err)
	}

	for _, behavior := range card.Behaviors {
		if !gamecards.HasAutoTrigger(behavior) || !g.ModeRequirementsMet(behavior.StateRequirements) {
			continue
		}
		effectiveChoice := choiceIndex
		if behavior.ChoicePolicy != nil && behavior.ChoicePolicy.Type == shared.ChoicePolicyTypeAuto {
			index := shared.AutoSelectChoiceIndex(behavior.ChoicePolicy, resolveChoicePolicyCount(behavior.ChoicePolicy, player, a.CardRegistryForGame(g)))
			if index >= 0 {
				effectiveChoice = &index
			}
		}
		if len(behavior.Choices) > 0 {
			if effectiveChoice == nil || *effectiveChoice < 0 || *effectiveChoice >= len(behavior.Choices) {
				return fmt.Errorf("select a valid card choice")
			}
			if !shared.IsChoiceValidForPolicy(*effectiveChoice, behavior.Choices, behavior.ChoicePolicy, player.Resources().Production()) {
				return fmt.Errorf("card choice is not available")
			}
		}
		_, revealOutputs := behavior.ExtractInputsOutputs(effectiveChoice)
		validator := gamecards.NewBehaviorApplier(player, g, card.Name, log).WithSourceCardID(card.ID).WithCardRegistry(a.CardRegistryForGame(g)).WithTargetCardIDs(cardStorageTargets).WithSourceType(shared.SourceTypeCardPlay).WithBehavior(behavior)
		if targetPlayerID != nil {
			validator.WithTargetPlayerID(*targetPlayerID)
		}
		if selectedAmount != nil {
			validator.WithSelectedAmount(*selectedAmount)
		}
		if err := validator.ValidateModeOutputs(revealOutputs); err != nil {
			return err
		}
		if err := validator.ValidateResourceOutputs(revealOutputs); err != nil {
			return err
		}
		if err := gamecards.ValidateRevealOutputs(revealOutputs, g, a.CardRegistryForGame(g)); err != nil {
			return err
		}
		options, deferred, err := gamecards.EffectSelectionOptions(behavior.Outputs, player, g, a.CardRegistryForGame(g), a.colonyBonusLookup)
		if err != nil {
			return err
		}
		if deferred && len(options) == 0 {
			return fmt.Errorf("cannot play card: no legal effect selection")
		}
	}

	log.Debug("Card requirements validated")

	if tileErrors := baseaction.ValidateTileOutputs(card, player, g); len(tileErrors) > 0 {
		log.Error("Tile placement not available", slog.String("error", tileErrors[0].Message))
		return fmt.Errorf("cannot play card: %s", tileErrors[0].Message)
	}

	if err := validateProductionOutputs(card, player, g); err != nil {
		log.Error("Production output validation failed", slog.Any("error", err))
		return fmt.Errorf("cannot play card: %w", err)
	}

	if err := validateNegativeResourceOutputs(card, player, g); err != nil {
		log.Error("Negative resource output validation failed", slog.Any("error", err))
		return fmt.Errorf("cannot play card: %w", err)
	}

	// Validate choice requirements early (before any state mutations)
	if choiceIndex != nil {
		for _, behavior := range card.Behaviors {
			if gamecards.HasAutoTrigger(behavior) && *choiceIndex >= 0 && *choiceIndex < len(behavior.Choices) {
				selectedChoice := behavior.Choices[*choiceIndex]
				if !g.ModeRequirementsMet(selectedChoice.StateRequirements) {
					return fmt.Errorf("choice mode state requirement not met")
				}
				if selectedChoice.Requirements != nil {
					if err := validateChoiceRequirements(selectedChoice.Requirements, player, g, a.CardRegistryForGame(g)); err != nil {
						log.Error("Choice requirements not met", slog.Any("error", err))
						return fmt.Errorf("choice %d requirements not met: %w", *choiceIndex, err)
					}
				}
			}
		}
		log.Debug("Choice requirements validated")
	}

	discountAmount := calculator.CalculateCardDiscounts(player, card)
	effectiveCost := card.Cost - discountAmount
	if effectiveCost < 0 {
		effectiveCost = 0
	}

	if discountAmount > 0 {
		log.Debug("Discount applied",
			slog.Int("base_cost", card.Cost),
			slog.Int("discount", discountAmount),
			slog.Int("effective_cost", effectiveCost))
	}

	quote, err := gamecards.QuotePayment(player, g, a.CardRegistryForGame(g), gamecards.PaymentContext{Costs: map[shared.ResourceType]int{shared.ResourceCredit: effectiveCost}, Action: shared.ActionCardPlaying, Card: card})
	if err != nil {
		return err
	}
	paymentPlan, err := gamecards.ValidatePayment(quote, payment)
	if err != nil {
		return err
	}
	totalValue := effectiveCost

	var immediateInputs []shared.BehaviorCondition
	for _, behavior := range card.Behaviors {
		if !gamecards.HasAutoTrigger(behavior) || gamecards.HasCardDiscardInput(behavior) {
			continue
		}
		effectiveChoice := choiceIndex
		if behavior.ChoicePolicy != nil && len(behavior.Choices) > 0 {
			index := shared.AutoSelectChoiceIndex(behavior.ChoicePolicy, resolveChoicePolicyCount(behavior.ChoicePolicy, player, a.CardRegistryForGame(g)))
			if index >= 0 {
				effectiveChoice = &index
			}
		}
		inputs, outputs := behavior.ExtractInputsOutputs(effectiveChoice)
		if hasVariableAmount(inputs, outputs) && selectedAmount == nil {
			return fmt.Errorf("must select an amount")
		}
		immediateInputs = append(immediateInputs, inputs...)
	}
	inputApplier := gamecards.NewBehaviorApplier(player, g, card.Name, log).WithSourceCardID(card.ID).WithCardRegistry(a.CardRegistryForGame(g)).WithInputCardIDs(cardStorageSources)
	if selectedAmount != nil {
		inputApplier.WithSelectedAmount(*selectedAmount)
	}
	inputApplier.WithReservedInputs(paymentPlan.Resources, paymentPlan.Storage)
	if err := inputApplier.ValidateInputs(immediateInputs); err != nil {
		return err
	}
	inputApplier.WithReservedInputs(nil, nil)

	if !player.Hand().RemoveCard(cardID) {
		log.Error("Failed to remove card from hand - card not found")
		return fmt.Errorf("failed to remove card from hand: card not found")
	}

	log.Debug("Card removed from hand")

	cardTags := make([]string, len(card.Tags))
	for i, tag := range card.Tags {
		cardTags[i] = string(tag)
	}

	player.PlayedCards().AddCard(cardID, card.Name, string(card.Type), cardTags)

	log.Debug("Card added to played cards")

	if card.ResourceStorage != nil {
		g.Colonies().ActivateResource(string(card.ResourceStorage.Type))
		player.Resources().AddToStorage(cardID, card.ResourceStorage.Starting)
		log.Debug("Initialized resource storage",
			slog.String("card_id", cardID),
			slog.String("resource_type", string(card.ResourceStorage.Type)),
			slog.Int("starting_amount", card.ResourceStorage.Starting))
	}

	gamecards.ApplyPayment(player, paymentPlan)

	if err := inputApplier.ApplyInputs(ctx, immediateInputs); err != nil {
		return err
	}

	calculatedOutputs, err := a.applyCardBehaviors(ctx, g, card, player, choiceIndex, cardStorageTargets, targetPlayerID, selectedAmount, log)
	if err != nil {
		log.Error("Failed to apply card behaviors", slog.Any("error", err))
		return fmt.Errorf("failed to apply card behaviors: %w", err)
	}

	baseaction.ActivateSelfTagTriggers(g, player, card, a.CardRegistryForGame(g), log)

	// Clean up temporary "next-card" effects that existed before this card was played
	removePrePlayTemporaryEffects(player, prePlayTemporaryCardIDs, log)

	a.ConsumePlayerAction(g, log)

	description := fmt.Sprintf("Played %s for %d credits", card.Name, totalValue)
	displayData := baseaction.BuildCardDisplayData(card, shared.SourceTypeCardPlay)
	a.WriteStateLogFull(ctx, g, card.Name, shared.SourceTypeCardPlay, playerID, description, choiceIndex, calculatedOutputs, displayData)

	log.Info("Card played",
		slog.String("card_name", card.Name),
		slog.Int("card_cost", card.Cost),
		slog.Int("payment_value", totalValue))

	return nil
}

// validateCardRequirements validates that the player and game state meet all card requirements.
// Uses RequirementModifierCalculator to include global parameter lenience from temporary effects.
func validateCardRequirements(card *gamecards.Card, g *game.Game, player *player.Player, calculator *gamecards.RequirementModifierCalculator, cardRegistry gamecards.CardRegistry) error {
	if card.Requirements == nil || len(card.Requirements.Items) == 0 {
		return nil // No requirements to validate
	}

	if err := gamecards.ValidateTagRequirements(card.Requirements.Items, gamecards.PlayerTagCounts(player, cardRegistry)); err != nil {
		return err
	}

	if calculator.HasIgnoreGlobalRequirements(player) {
		return nil
	}

	for _, req := range card.Requirements.Items {
		switch req.Type {
		case gamecards.RequirementState:
			value, ok := g.ModeStateValue(req.State)
			if !ok || req.Equals == nil || value != *req.Equals {
				return fmt.Errorf("mode state requirement not met")
			}
		case gamecards.RequirementTemperature:
			lenience := calculator.CalculateGlobalParameterRequirementOffset(player, "temperature")
			temp := g.GlobalParameters().Temperature()
			if req.Min != nil && temp < *req.Min-lenience {
				return fmt.Errorf("temperature requirement not met: need %d°C, current %d°C", *req.Min, temp)
			}
			if req.Max != nil && temp > *req.Max+lenience {
				return fmt.Errorf("temperature requirement not met: max %d°C, current %d°C", *req.Max, temp)
			}

		case gamecards.RequirementOxygen:
			lenience := calculator.CalculateGlobalParameterRequirementOffset(player, "oxygen")
			oxygen := g.GlobalParameters().Oxygen()
			if req.Min != nil && oxygen < *req.Min-lenience {
				return fmt.Errorf("oxygen requirement not met: need %d%%, current %d%%", *req.Min, oxygen)
			}
			if req.Max != nil && oxygen > *req.Max+lenience {
				return fmt.Errorf("oxygen requirement not met: max %d%%, current %d%%", *req.Max, oxygen)
			}

		case gamecards.RequirementOceans:
			lenience := calculator.CalculateGlobalParameterRequirementOffset(player, "ocean")
			oceans := g.GlobalParameters().Oceans()
			if req.Min != nil && oceans < *req.Min-lenience {
				return fmt.Errorf("ocean requirement not met: need %d, current %d", *req.Min, oceans)
			}
			if req.Max != nil && oceans > *req.Max+lenience {
				return fmt.Errorf("ocean requirement not met: max %d, current %d", *req.Max, oceans)
			}

		case gamecards.RequirementTR:
			tr := player.Resources().TerraformRating()
			if req.Min != nil && tr < *req.Min {
				return fmt.Errorf("terraform rating requirement not met: need %d, current %d", *req.Min, tr)
			}
			if req.Max != nil && tr > *req.Max {
				return fmt.Errorf("terraform rating requirement not met: max %d, current %d", *req.Max, tr)
			}

		case gamecards.RequirementTags:
			continue

		case gamecards.RequirementProduction:
			if req.Resource == nil {
				return fmt.Errorf("production requirement missing resource specification")
			}
			production := player.Resources().Production()
			if req.Permanent {
				production = player.Resources().PermanentProduction()
			}
			var currentProd int
			switch *req.Resource {
			case shared.ResourceCredit, shared.ResourceCreditProduction:
				currentProd = production.Credits
			case shared.ResourceSteel, shared.ResourceSteelProduction:
				currentProd = production.Steel
			case shared.ResourceTitanium, shared.ResourceTitaniumProduction:
				currentProd = production.Titanium
			case shared.ResourcePlant, shared.ResourcePlantProduction:
				currentProd = production.Plants
			case shared.ResourceEnergy, shared.ResourceEnergyProduction:
				currentProd = production.Energy
			case shared.ResourceHeat, shared.ResourceHeatProduction:
				currentProd = production.Heat
			}
			if req.Min != nil && currentProd < *req.Min {
				return fmt.Errorf("production requirement not met: need %d %s production, have %d", *req.Min, *req.Resource, currentProd)
			}
			if req.Max != nil && currentProd > *req.Max {
				return fmt.Errorf("production requirement not met: max %d %s production, have %d", *req.Max, *req.Resource, currentProd)
			}

		case gamecards.RequirementResource:
			if req.Resource == nil {
				return fmt.Errorf("resource requirement missing resource specification")
			}
			currentAmount := gamecards.CountPlayerResources(player, cardRegistry, *req.Resource)

			if req.Min != nil && currentAmount < *req.Min {
				return fmt.Errorf("resource requirement not met: need %d %s, have %d", *req.Min, *req.Resource, currentAmount)
			}
			if req.Max != nil && currentAmount > *req.Max {
				return fmt.Errorf("resource requirement not met: max %d %s, have %d", *req.Max, *req.Resource, currentAmount)
			}

		case gamecards.RequirementCities, gamecards.RequirementGreeneries:
			if err := gamecards.ValidateTileRequirement(req, player, g.Board()); err != nil {
				return err
			}

		case gamecards.RequirementVenus:
			lenience := calculator.CalculateGlobalParameterRequirementOffset(player, "venus")
			venus := g.GlobalParameters().Venus()
			if req.Min != nil && venus < *req.Min-lenience {
				return fmt.Errorf("venus requirement not met: need %d%%, current %d%%", *req.Min, venus)
			}
			if req.Max != nil && venus > *req.Max+lenience {
				return fmt.Errorf("venus requirement not met: max %d%%, current %d%%", *req.Max, venus)
			}
		}
	}

	return nil
}

// applyCardBehaviors processes all card behaviors and applies immediate effects or registers actions/effects
// Returns calculated outputs for logging purposes
func (a *PlayCardAction) applyCardBehaviors(
	ctx context.Context,
	g *game.Game,
	card *gamecards.Card,
	p *player.Player,
	choiceIndex *int,
	cardStorageTargets []string,
	targetPlayerID *string,
	selectedAmount *int,
	log *slog.Logger,
) ([]shared.CalculatedOutput, error) {
	if len(card.Behaviors) == 0 {
		log.Debug("No card behaviors to apply")
		return nil, nil
	}

	log.Debug("Processing card behaviors",
		slog.String("card_id", card.ID),
		slog.Int("behavior_count", len(card.Behaviors)))

	p.Resources().RegisterDependencyGroups(card.ID, card.DependencyGroups)
	var allCalculatedOutputs []shared.CalculatedOutput

	for behaviorIndex, behavior := range card.Behaviors {
		log.Debug("Processing behavior",
			slog.Int("index", behaviorIndex),
			slog.Int("trigger_count", len(behavior.Triggers)))

		// Apply auto-trigger behaviors immediately
		if gamecards.HasAutoTrigger(behavior) && g.ModeRequirementsMet(behavior.StateRequirements) {
			// Auto-select choice if behavior has an auto-selection policy
			effectiveChoiceIndex := choiceIndex
			if behavior.ChoicePolicy != nil && len(behavior.Choices) > 0 {
				count := resolveChoicePolicyCount(behavior.ChoicePolicy, p, a.CardRegistryForGame(g))
				autoIdx := shared.AutoSelectChoiceIndex(behavior.ChoicePolicy, count)
				if autoIdx >= 0 {
					effectiveChoiceIndex = &autoIdx
					log.Debug("Auto-selected choice by policy",
						slog.String("policy_type", string(behavior.ChoicePolicy.Type)),
						slog.Int("choice_index", autoIdx))
				}
			}

			// Extract inputs and outputs, incorporating choice if present
			inputs, outputs := behavior.ExtractInputsOutputs(effectiveChoiceIndex)

			// Check for card-discard inputs — these defer output application
			if gamecards.HasCardDiscardInput(behavior) {
				a.createPendingCardDiscard(p, card, behaviorIndex, inputs, outputs, log)
				continue
			}

			// Check for card-discard outputs — player must choose cards to discard first
			if gamecards.HasCardDiscardOutput(behavior) {
				a.createPendingCardDiscardFromOutputs(p, card, behaviorIndex, outputs, log)
				continue
			}

			log.Debug("Found auto-trigger behavior, applying outputs immediately",
				slog.Int("output_count", len(outputs)))

			// Use BehaviorApplier for consistent output handling
			applier := gamecards.NewBehaviorApplier(p, g, card.Name, slog.Default()).
				WithSourceCardID(card.ID).
				WithCardRegistry(a.CardRegistryForGame(g)).
				WithSourceType(shared.SourceTypeCardPlay).
				WithProductionBox(behavior.ProductionBox).WithBehavior(behavior)
			if a.colonyBonusLookup != nil {
				applier = applier.WithColonyBonusLookup(a.colonyBonusLookup)
			}
			if len(cardStorageTargets) > 0 {
				applier = applier.WithTargetCardIDs(cardStorageTargets)
			}
			if targetPlayerID != nil {
				applier = applier.WithTargetPlayerID(*targetPlayerID)
			}
			if selectedAmount != nil {
				applier = applier.WithSelectedAmount(*selectedAmount)
			}

			calculatedOutputs, err := applier.ApplyOutputsAndGetCalculated(ctx, outputs)
			if err != nil {
				return nil, fmt.Errorf("failed to apply auto behavior %d outputs: %w", behaviorIndex, err)
			}

			allCalculatedOutputs = append(allCalculatedOutputs, calculatedOutputs...)

			if deferred := applier.DeferredRemoval(); deferred != nil {
				callback := &shared.TileCompletionCallback{
					Type:         "adjacent-removal",
					Output:       deferred,
					SourceCardID: card.ID,
					Source:       card.Name,
				}
				g.SetTileQueueOnComplete(ctx, p.ID(), callback)
			}

			// Also register as effect if it has persistent outputs (discount, payment-substitute)
			// These need to show in the effects list for display and for modifier calculations
			if gamecards.HasPersistentEffects(behavior) {
				log.Debug("Registering auto-trigger behavior with persistent effects",
					slog.String("card_name", card.Name))

				effect := shared.CardEffect{
					CardID:        card.ID,
					CardName:      card.Name,
					BehaviorIndex: behaviorIndex,
					Behavior:      behavior,
				}
				p.Effects().AddEffect(effect)

				events.Publish(g.EventBus(), events.PlayerEffectsChangedEvent{
					GameID:    g.ID(),
					PlayerID:  p.ID(),
					Timestamp: time.Now(),
				})

				g.AddTriggeredEffect(shared.TriggeredEffect{
					CardName:   card.Name,
					PlayerID:   p.ID(),
					SourceType: shared.SourceTypeEffectAdded,
					Behaviors:  []shared.CardBehavior{behavior},
				})
			}
		}

		// Register manual-trigger behaviors as player actions
		if gamecards.HasManualTrigger(behavior) {
			log.Debug("Found manual-trigger behavior, registering as player action")

			p.Actions().AddAction(shared.CardAction{
				CardID:                  card.ID,
				CardName:                card.Name,
				BehaviorIndex:           behaviorIndex,
				Behavior:                behavior,
				TimesUsedThisTurn:       0,
				TimesUsedThisGeneration: 0,
			})

			g.AddTriggeredEffect(shared.TriggeredEffect{
				CardName:   card.Name,
				PlayerID:   p.ID(),
				SourceType: shared.SourceTypeActionAdded,
				Behaviors:  []shared.CardBehavior{behavior},
			})
		}

		// Register conditional-trigger behaviors as passive effects
		if gamecards.HasConditionalTrigger(behavior) {
			log.Debug("Found conditional-trigger behavior, registering as passive effect",
				slog.Int("trigger_count", len(behavior.Triggers)))

			effect := shared.CardEffect{
				CardID:        card.ID,
				CardName:      card.Name,
				BehaviorIndex: behaviorIndex,
				Behavior:      behavior,
			}
			p.Effects().AddEffect(effect)

			events.Publish(g.EventBus(), events.PlayerEffectsChangedEvent{
				GameID:    g.ID(),
				PlayerID:  p.ID(),
				Timestamp: time.Now(),
			})

			g.AddTriggeredEffect(shared.TriggeredEffect{
				CardName:   card.Name,
				PlayerID:   p.ID(),
				SourceType: shared.SourceTypeEffectAdded,
				Behaviors:  []shared.CardBehavior{behavior},
			})

			// Subscribe passive effects to relevant events
			baseaction.SubscribePassiveEffectToEvents(ctx, g, p, effect, log, a.CardRegistryForGame(g))
		}
	}

	// Add VP notification if card has VP conditions
	if len(card.VPConditions) > 0 {
		var vpForLog []shared.VPConditionForLog
		for _, vp := range card.VPConditions {
			vpLog := shared.VPConditionForLog{
				Amount:    vp.Amount,
				Condition: string(vp.Condition),
			}
			if vp.MaxTrigger != nil {
				vpLog.MaxTrigger = vp.MaxTrigger
			}
			if vp.Per != nil {
				vpLog.Per = &shared.PerCondition{
					ResourceType: vp.Per.ResourceType,
					Amount:       vp.Per.Amount,
				}
				if vp.Per.Location != nil {
					loc := string(*vp.Per.Location)
					vpLog.Per.Location = &loc
				}
				if vp.Per.Target != nil {
					target := string(*vp.Per.Target)
					vpLog.Per.Target = &target
				}
				if vp.Per.Tag != nil {
					vpLog.Per.Tag = vp.Per.Tag
				}
			}
			vpForLog = append(vpForLog, vpLog)
		}
		g.AddTriggeredEffect(shared.TriggeredEffect{
			CardName:     card.Name,
			PlayerID:     p.ID(),
			SourceType:   shared.SourceTypeCardPlay,
			VPConditions: vpForLog,
		})
	}

	log.Debug("Card behaviors processed")
	return allCalculatedOutputs, nil
}

// createPendingCardDiscard creates a pending discard resolution for behaviors with card-discard inputs.
// The player must select cards to discard before outputs are applied.
func (a *PlayCardAction) createPendingCardDiscard(
	p *player.Player,
	card *gamecards.Card,
	behaviorIndex int,
	inputs []shared.BehaviorCondition,
	outputs []shared.BehaviorCondition,
	log *slog.Logger,
) {
	minCards := 0
	maxCards := 0
	isOptional := false

	for _, input := range inputs {
		if input.GetResourceType() == shared.ResourceCardDiscard {
			maxCards += input.GetAmount()
			if !shared.IsOptional(input) {
				minCards += input.GetAmount()
			} else {
				isOptional = true
			}
		}
	}

	// If optional but player has no cards in hand, skip entirely
	if isOptional && len(p.Hand().Cards()) == 0 {
		log.Debug("Skipping card discard: optional and player has no cards in hand")
		return
	}

	selection := &shared.PendingBehaviorResolution{Kind: "card-discard",
		MinCards:            minCards,
		MaxCards:            maxCards,
		Source:              card.Name,
		SourceCardID:        card.ID,
		SourceBehaviorIndex: behaviorIndex,
		PendingOutputs:      outputs,
	}

	p.Selection().AddPendingBehaviorResolution(selection)

	log.Debug("Created pending card discard selection",
		slog.String("card_name", card.Name),
		slog.Int("min_cards", minCards),
		slog.Int("max_cards", maxCards),
		slog.Bool("optional", isOptional),
		slog.Int("pending_outputs", len(outputs)))
}

// createPendingCardDiscardFromOutputs creates a pending discard resolution for behaviors with card-discard outputs.
// The player must select cards to discard before remaining outputs (draws, etc.) are applied.
func (a *PlayCardAction) createPendingCardDiscardFromOutputs(
	p *player.Player,
	card *gamecards.Card,
	behaviorIndex int,
	outputs []shared.BehaviorCondition,
	log *slog.Logger,
) {
	minCards := 0
	maxCards := 0
	var pendingOutputs []shared.BehaviorCondition

	for _, output := range outputs {
		if output.GetResourceType() == shared.ResourceCardDiscard {
			minCards += output.GetAmount()
			maxCards += output.GetAmount()
		} else {
			pendingOutputs = append(pendingOutputs, output)
		}
	}

	selection := &shared.PendingBehaviorResolution{Kind: "card-discard",
		MinCards:            minCards,
		MaxCards:            maxCards,
		Source:              card.Name,
		SourceCardID:        card.ID,
		SourceBehaviorIndex: behaviorIndex,
		PendingOutputs:      pendingOutputs,
	}

	p.Selection().AddPendingBehaviorResolution(selection)

	log.Debug("Created pending card discard selection from outputs",
		slog.String("card_name", card.Name),
		slog.Int("min_cards", minCards),
		slog.Int("max_cards", maxCards),
		slog.Int("pending_outputs", len(pendingOutputs)))
}

func collectTemporaryEffectCardIDs(p *player.Player, temporaryType string) []string {
	var cardIDs []string
	for _, effect := range p.Effects().List() {
		for _, output := range effect.Behavior.Outputs {
			if shared.GetTemporary(output) == temporaryType {
				cardIDs = append(cardIDs, effect.CardID)
				break
			}
		}
	}
	return cardIDs
}

// removePrePlayTemporaryEffects removes temporary effects by their card IDs (collected before card play).
func removePrePlayTemporaryEffects(p *player.Player, cardIDs []string, log *slog.Logger) {
	for _, cardID := range cardIDs {
		p.Effects().RemoveEffectsByCardID(cardID)
		log.Debug("Removed temporary next-card effect",
			slog.String("effect_card_id", cardID))
	}
}

// validateChoiceRequirements checks if a choice's requirements are met by the player.
func validateChoiceRequirements(reqs *shared.ChoiceRequirements, p *player.Player, g *game.Game, cardRegistry gamecards.CardRegistry) error {
	if reqs == nil || len(reqs.Items) == 0 {
		return nil
	}

	if err := gamecards.ValidateChoiceTagRequirements(reqs.Items, gamecards.PlayerTagCounts(p, cardRegistry)); err != nil {
		return err
	}

	for _, req := range reqs.Items {
		if err := checkChoiceRequirement(req, p, g, cardRegistry); err != nil {
			return err
		}
	}
	return nil
}

// checkChoiceRequirement validates a single choice requirement.
func checkChoiceRequirement(req shared.ChoiceRequirement, p *player.Player, g *game.Game, cardRegistry gamecards.CardRegistry) error {
	switch req.Type {
	case "tags":
		return nil

	case "temperature":
		temp := g.GlobalParameters().Temperature()
		if req.Min != nil && temp < *req.Min {
			return fmt.Errorf("temperature too low: need %d, have %d", *req.Min, temp)
		}
		if req.Max != nil && temp > *req.Max {
			return fmt.Errorf("temperature too high: max %d, have %d", *req.Max, temp)
		}

	case "oxygen":
		oxygen := g.GlobalParameters().Oxygen()
		if req.Min != nil && oxygen < *req.Min {
			return fmt.Errorf("oxygen too low: need %d, have %d", *req.Min, oxygen)
		}
		if req.Max != nil && oxygen > *req.Max {
			return fmt.Errorf("oxygen too high: max %d, have %d", *req.Max, oxygen)
		}

	case "ocean":
		oceans := g.GlobalParameters().Oceans()
		if req.Min != nil && oceans < *req.Min {
			return fmt.Errorf("too few oceans: need %d, have %d", *req.Min, oceans)
		}
		if req.Max != nil && oceans > *req.Max {
			return fmt.Errorf("too many oceans: max %d, have %d", *req.Max, oceans)
		}

	case "venus":
		venus := g.GlobalParameters().Venus()
		if req.Min != nil && venus < *req.Min {
			return fmt.Errorf("venus too low: need %d, have %d", *req.Min, venus)
		}
		if req.Max != nil && venus > *req.Max {
			return fmt.Errorf("venus too high: max %d, have %d", *req.Max, venus)
		}

	case "tr":
		tr := p.Resources().TerraformRating()
		if req.Min != nil && tr < *req.Min {
			return fmt.Errorf("TR too low: need %d, have %d", *req.Min, tr)
		}
		if req.Max != nil && tr > *req.Max {
			return fmt.Errorf("TR too high: max %d, have %d", *req.Max, tr)
		}

	case "production":
		if req.Resource == nil {
			return fmt.Errorf("production requirement missing resource type")
		}
		production := p.Resources().PermanentProduction()
		amount := production.GetAmount(*req.Resource)
		if req.Min != nil && amount < *req.Min {
			return fmt.Errorf("%s production too low: need %d, have %d", *req.Resource, *req.Min, amount)
		}

	case "resource":
		if req.Resource == nil {
			return fmt.Errorf("resource requirement missing resource type")
		}
		resources := p.Resources().Get()
		amount := resources.GetAmount(*req.Resource)
		if req.Min != nil && amount < *req.Min {
			return fmt.Errorf("%s too low: need %d, have %d", *req.Resource, *req.Min, amount)
		}
	}

	return nil
}

// validateProductionOutputs checks that playing the card won't bring production below the minimum.
func validateProductionOutputs(card *gamecards.Card, p *player.Player, g *game.Game) error {
	if len(card.Behaviors) == 0 {
		return nil
	}

	production := p.Resources().PermanentProduction()

	for _, behavior := range card.Behaviors {
		if !gamecards.HasAutoTrigger(behavior) || !g.ModeRequirementsMet(behavior.StateRequirements) {
			continue
		}
		for _, outputBC := range behavior.Outputs {
			if shared.IsVariableAmount(outputBC) || outputBC.GetAmount() >= 0 {
				continue
			}
			if outputBC.GetTarget() != "self-player" {
				continue
			}
			var current, minProd int
			switch outputBC.GetResourceType() {
			case shared.ResourceCreditProduction:
				current, minProd = production.Credits, shared.MinCreditProduction
			case shared.ResourceSteelProduction:
				current, minProd = production.Steel, shared.MinOtherProduction
			case shared.ResourceTitaniumProduction:
				current, minProd = production.Titanium, shared.MinOtherProduction
			case shared.ResourcePlantProduction:
				current, minProd = production.Plants, shared.MinOtherProduction
			case shared.ResourceEnergyProduction:
				current, minProd = production.Energy, shared.MinOtherProduction
			case shared.ResourceHeatProduction:
				current, minProd = production.Heat, shared.MinOtherProduction
			default:
				continue
			}
			if current+outputBC.GetAmount() < minProd {
				return fmt.Errorf("insufficient %s: have %d, need at least %d to decrease by %d", outputBC.GetResourceType(), current, -outputBC.GetAmount(), -outputBC.GetAmount())
			}
		}
	}
	return nil
}

func validateNegativeResourceOutputs(card *gamecards.Card, p *player.Player, g *game.Game) error {
	if len(card.Behaviors) == 0 {
		return nil
	}

	resources := p.Resources().Get()

	for _, behavior := range card.Behaviors {
		if !gamecards.HasAutoTrigger(behavior) || !g.ModeRequirementsMet(behavior.StateRequirements) {
			continue
		}
		for _, outputBC := range behavior.Outputs {
			if shared.IsVariableAmount(outputBC) || outputBC.GetAmount() >= 0 {
				continue
			}
			if outputBC.GetTarget() != "self-player" {
				continue
			}
			var available int
			switch outputBC.GetResourceType() {
			case shared.ResourceCredit:
				available = resources.Credits
			case shared.ResourceSteel:
				available = resources.Steel
			case shared.ResourceTitanium:
				available = resources.Titanium
			case shared.ResourcePlant:
				available = resources.Plants
			case shared.ResourceEnergy:
				available = resources.Energy
			case shared.ResourceHeat:
				available = resources.Heat
			default:
				continue
			}
			required := -outputBC.GetAmount()
			if available < required {
				return fmt.Errorf("not enough %s: have %d, need %d", outputBC.GetResourceType(), available, required)
			}
		}
	}
	return nil
}

func resolveChoicePolicyCount(policy *shared.ChoicePolicy, p *player.Player, registry gamecards.CardRegistryInterface) int {
	if policy == nil || policy.Select == nil {
		return 0
	}
	sel := policy.Select
	if sel.ResourceType == "tag" && sel.Tag != nil {
		return gamecards.CountPlayerTags(p, registry, []shared.CardTag{*sel.Tag}, gamecards.TagCountContext{ActorID: p.ID()})
	}
	return 0
}
