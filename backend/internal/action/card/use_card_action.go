package card

import (
	"context"
	"fmt"
	"log/slog"
	baseaction "openmars/internal/action"

	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// UseCardActionAction handles the business logic for using a card's manual action
// Card actions are repeatable blue card abilities with inputs and outputs
type UseCardActionAction struct {
	baseaction.BaseAction
}

// NewUseCardActionAction creates a new use card action action
func NewUseCardActionAction(
	gameRepo game.GameRepository,
	cardRegistry gamecards.CardRegistry,
	stateRepo game.GameStateRepository,
	logger *slog.Logger,
) *UseCardActionAction {
	return &UseCardActionAction{
		BaseAction: baseaction.NewBaseActionWithStateRepo(gameRepo, cardRegistry, stateRepo),
	}
}

// Execute performs the use card action
func (a *UseCardActionAction) Execute(
	ctx context.Context,
	gameID string,
	playerID string,
	cardID string,
	behaviorIndex int,
	choiceIndex *int,
	cardStorageTargets []string,
	targetPlayerID *string,
	stealSourceCardID *string,
	selectedAmount *int,
	actionPayment *shared.Payment,
	reuseSourceCardID *string,
	cardStorageSources []string,
) error {
	log := a.InitLogger(gameID, playerID).With(
		slog.String("card_id", cardID),
		slog.Int("behavior_index", behaviorIndex),
		slog.String("action", "use_card_action"),
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
	if stealSourceCardID != nil {
		log = log.With(slog.String("source_card_for_input", *stealSourceCardID))
	}
	log.Debug("Player attempting to use card action")

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

	p, err := a.GetPlayerFromGame(g, playerID, log)
	if err != nil {
		return err
	}

	cardAction, err := a.findCardAction(p, cardID, behaviorIndex, log)
	if err != nil {
		return err
	}

	completion := shared.CardActionRef{CardID: cardID, BehaviorIndex: behaviorIndex}
	var reuseAction *shared.CardAction
	if reuseSourceCardID != nil {
		reuseAction, err = baseaction.ResolveActionReuse(p, *reuseSourceCardID, *cardAction)
		if err != nil {
			return err
		}
		completion = shared.CardActionRef{CardID: reuseAction.CardID, BehaviorIndex: reuseAction.BehaviorIndex}
	} else if cardAction.TimesUsedThisGeneration >= 1 {
		return fmt.Errorf("action already played this generation")
	}
	if !gamecards.HasManualTrigger(cardAction.Behavior) || baseaction.IsActionReuse(cardAction.Behavior) {
		return fmt.Errorf("select a manual action to execute")
	}
	if err := baseaction.ValidateCardActionChoice(cardAction.Behavior, choiceIndex, p, g, a.CardRegistryForGame(g)); err != nil {
		return err
	}

	log.Debug("Found card action",
		slog.String("card_name", cardAction.CardName),
		slog.Int("times_used_this_generation", cardAction.TimesUsedThisGeneration))

	applier := gamecards.NewBehaviorApplier(p, g, cardAction.CardName, slog.Default()).
		WithSourceCardID(cardID).
		WithSourceBehaviorIndex(behaviorIndex).
		WithBehavior(cardAction.Behavior).
		WithActionCompletion(completion).
		WithCardRegistry(a.CardRegistryForGame(g)).
		WithSourceType(shared.SourceTypeCardAction).WithInputCardIDs(cardStorageSources)
	if len(cardStorageTargets) > 0 {
		applier = applier.WithTargetCardIDs(cardStorageTargets)
	}
	if targetPlayerID != nil {
		applier = applier.WithTargetPlayerID(*targetPlayerID)
	}
	if stealSourceCardID != nil {
		applier = applier.WithStealSourceCardID(*stealSourceCardID)
	}
	if selectedAmount != nil && *selectedAmount < 0 {
		return fmt.Errorf("selected amount must be nonnegative")
	}
	if selectedAmount != nil {
		applier = applier.WithSelectedAmount(*selectedAmount)
	}
	if actionPayment != nil {
		applier = applier.WithActionPayment(actionPayment)
	}

	inputs, outputs := cardAction.Behavior.ExtractInputsOutputs(choiceIndex)

	if choiceIndex != nil {
		log.Debug("Using choice-specific behavior",
			slog.Int("choice_index", *choiceIndex),
			slog.Int("input_count", len(inputs)),
			slog.Int("output_count", len(outputs)))
	}

	if hasVariableAmount(inputs, outputs) && selectedAmount == nil {
		log.Warn("Variable-amount action requires selectedAmount")
		return fmt.Errorf("must select an amount for this action")
	}

	if hasStealFromAnyCard(outputs) && stealSourceCardID == nil {
		log.Warn("Steal action requires a target card")
		return fmt.Errorf("steal action requires a target card; select a card or cancel")
	}

	if err := validateOutputAffordability(p, outputs); err != nil {
		log.Warn("Cannot afford negative resource outputs", slog.Any("error", err))
		return err
	}

	if err := gamecards.ValidateRevealOutputs(outputs, g, a.CardRegistryForGame(g)); err != nil {
		return err
	}
	if err := applier.ValidateModeOutputs(outputs); err != nil {
		return err
	}
	if err := applier.ValidateResourceOutputs(outputs); err != nil {
		return err
	}
	if issues := baseaction.ValidateBehaviorTileOutputs(shared.CardBehavior{Outputs: outputs}, p, g); len(issues) > 0 {
		return fmt.Errorf("cannot place tile: %s", issues[0].Message)
	}
	if err := applier.ApplyInputs(ctx, inputs); err != nil {
		log.Error("Failed to apply inputs", slog.Any("error", err))
		return err
	}

	// Check for card draw outputs (card-peek/take/buy) - these create pending selection
	hasPending, err := applier.ApplyCardDrawOutputs(ctx, outputs)
	if err != nil {
		log.Error("Failed to apply card draw outputs", slog.Any("error", err))
		return err
	}
	if hasPending {
		// Pending selection created - action completion deferred to confirmation
		// Don't increment usage counts or consume action here - that happens in ConfirmCardDraw
		log.Debug("Card draw selection pending, awaiting player choice")
		return nil
	}

	calculatedOutputs, err := applier.ApplyOutputsAndGetCalculated(ctx, outputs)
	if err != nil {
		log.Error("Failed to apply outputs", slog.Any("error", err))
		return err
	}

	a.incrementUsageCounts(p, completion.CardID, completion.BehaviorIndex, log)

	a.ConsumePlayerAction(g, log)

	description := fmt.Sprintf("Used %s action", cardAction.CardName)
	if reuseAction != nil {
		description = fmt.Sprintf("Used %s to reuse %s action", reuseAction.CardName, cardAction.CardName)
	}
	var displayData *game.LogDisplayData
	if cardFromRegistry, err := a.CardRegistryForGame(g).GetByID(cardID); err == nil {
		displayData = baseaction.BuildCardDisplayData(cardFromRegistry, shared.SourceTypeCardAction)
	}
	a.WriteStateLogFull(ctx, g, cardAction.CardName, shared.SourceTypeCardAction, playerID, description, choiceIndex, calculatedOutputs, displayData)

	log.Info("Card action executed")
	return nil
}

// findCardAction finds a card action in the player's available actions
func (a *UseCardActionAction) findCardAction(
	p *player.Player,
	cardID string,
	behaviorIndex int,
	log *slog.Logger,
) (*shared.CardAction, error) {
	actions := p.Actions().List()

	for i := range actions {
		if actions[i].CardID == cardID && actions[i].BehaviorIndex == behaviorIndex {
			return &actions[i], nil
		}
	}

	log.Error("Card action not found in player's available actions",
		slog.String("card_id", cardID),
		slog.Int("behavior_index", behaviorIndex))
	return nil, fmt.Errorf("card action not found: %s[%d]", cardID, behaviorIndex)
}

// incrementUsageCounts increments the usage counts for a card action
func (a *UseCardActionAction) incrementUsageCounts(
	p *player.Player,
	cardID string,
	behaviorIndex int,
	log *slog.Logger,
) {
	actions := p.Actions().List()

	// Find and increment both turn and generation counts
	for i := range actions {
		if actions[i].CardID == cardID && actions[i].BehaviorIndex == behaviorIndex {
			actions[i].TimesUsedThisTurn++
			actions[i].TimesUsedThisGeneration++
			log.Debug("Incremented action usage counts",
				slog.Int("times_used_this_turn", actions[i].TimesUsedThisTurn),
				slog.Int("times_used_this_generation", actions[i].TimesUsedThisGeneration))
			break
		}
	}

	// Update player actions
	p.Actions().SetActions(actions)
}

func hasVariableAmount(inputs, outputs []shared.BehaviorCondition) bool {
	for _, input := range inputs {
		if shared.IsVariableAmount(input) {
			return true
		}
	}
	for _, output := range outputs {
		if shared.IsVariableAmount(output) {
			return true
		}
	}
	return false
}

func hasStealFromAnyCard(outputs []shared.BehaviorCondition) bool {
	for _, output := range outputs {
		if output.GetTarget() == "steal-from-any-card" {
			return true
		}
	}
	return false
}

// validateOutputAffordability checks that the player can afford all negative resource outputs
// before they are applied. This is a defense-in-depth check; card costs should be modeled as
// inputs, but this catches any negative resource outputs that slip through.
func validateOutputAffordability(p *player.Player, outputs []shared.BehaviorCondition) error {
	resources := p.Resources().Get()
	for _, output := range outputs {
		if shared.IsVariableAmount(output) || output.GetAmount() >= 0 {
			continue
		}
		var available int
		switch output.GetResourceType() {
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
		if available < -output.GetAmount() {
			return fmt.Errorf("insufficient %s: need %d, have %d", output.GetResourceType(), -output.GetAmount(), available)
		}
	}
	return nil
}
