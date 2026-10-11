package confirmation

import (
	"context"
	"fmt"
	"log/slog"
	baseaction "openmars/internal/action"

	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/shared"
)

// ConfirmBehaviorChoiceAction handles the business logic for confirming a behavior choice selection
type ConfirmBehaviorChoiceAction struct {
	baseaction.BaseAction
}

// NewConfirmBehaviorChoiceAction creates a new confirm behavior choice action
func NewConfirmBehaviorChoiceAction(
	gameRepo game.GameRepository,
	cardRegistry gamecards.CardRegistry,
	stateRepo game.GameStateRepository,
	logger *slog.Logger,
) *ConfirmBehaviorChoiceAction {
	return &ConfirmBehaviorChoiceAction{
		BaseAction: baseaction.NewBaseActionWithStateRepo(gameRepo, cardRegistry, stateRepo),
	}
}

// Execute performs the confirm behavior choice action
func (a *ConfirmBehaviorChoiceAction) Execute(ctx context.Context, gameID string, playerID string, resolutionID string, choiceIndex int, cardStorageTargets []string) error {
	log := a.InitLogger(gameID, playerID).With(
		slog.String("action", "confirm_behavior_choice"),
		slog.Int("choice_index", choiceIndex),
	)
	log.Debug("Confirming behavior choice selection")

	g, err := baseaction.ValidateActiveGame(ctx, a.GameRepository(), gameID, log)
	if err != nil {
		return err
	}

	p, err := a.GetPlayerFromGame(g, playerID, log)
	if err != nil {
		return err
	}

	selection := p.Selection().GetPendingBehaviorResolution(resolutionID)
	if selection == nil || selection.Kind != "choice" {
		log.Warn("No pending behavior choice selection found")
		return fmt.Errorf("no pending behavior choice selection found")
	}

	if choiceIndex < 0 || choiceIndex >= len(selection.Choices) {
		log.Warn("Invalid choice index",
			slog.Int("choice_index", choiceIndex),
			slog.Int("num_choices", len(selection.Choices)))
		return fmt.Errorf("invalid choice index %d, must be 0-%d", choiceIndex, len(selection.Choices)-1)
	}

	selectedChoice := selection.Choices[choiceIndex]

	// Validate choice requirements before applying
	if choiceErrors := baseaction.CalculateResolutionChoiceErrors(selectedChoice, selection, p, g, a.CardRegistryForGame(g)); len(choiceErrors) > 0 {
		log.Warn("Choice requirements not met",
			slog.Int("choice_index", choiceIndex),
			slog.String("error", choiceErrors[0].Message))
		return fmt.Errorf("choice %d requirements not met: %s", choiceIndex, choiceErrors[0].Message)
	}

	if err := baseaction.ValidateResolutionStorageTargets(selectedChoice, cardStorageTargets, p, a.CardRegistryForGame(g)); err != nil {
		return err
	}
	applier := gamecards.NewBehaviorApplier(p, g, selection.Source, slog.Default()).
		WithSourceCardID(selection.SourceCardID).
		WithTriggeringCard(selection.TriggeringCardID, selection.TriggeringPlayerID).
		WithCardRegistry(a.CardRegistryForGame(g)).
		WithSourceType(shared.SourceTypePassiveEffect)

	if len(cardStorageTargets) > 0 {
		applier = applier.WithTargetCardIDs(cardStorageTargets)
	}

	if err := applier.ValidateModeOutputs(selectedChoice.Outputs); err != nil {
		return err
	}
	if err := applier.ValidateResourceOutputs(selectedChoice.Outputs); err != nil {
		return err
	}

	// Apply inputs (deduct resources)
	if len(selectedChoice.Inputs) > 0 {
		if err := applier.ApplyInputs(ctx, selectedChoice.Inputs); err != nil {
			log.Error("Failed to apply choice inputs", slog.Any("error", err))
			return fmt.Errorf("failed to apply choice inputs: %w", err)
		}
	}

	// Apply outputs (add resources)
	var calculatedOutputs []shared.CalculatedOutput
	if len(selectedChoice.Outputs) > 0 {
		var err error
		calculatedOutputs, err = applier.ApplyOutputsAndGetCalculated(ctx, selectedChoice.Outputs)
		if err != nil {
			log.Error("Failed to apply choice outputs", slog.Any("error", err))
			return fmt.Errorf("failed to apply choice outputs: %w", err)
		}
	}

	// Clear the pending selection
	p.Selection().RemovePendingBehaviorResolution(resolutionID)
	a.WriteStateLogWithChoiceAndOutputs(ctx, g, selection.Source, shared.SourceTypePassiveEffect, playerID, "Resolved effect", &choiceIndex, calculatedOutputs)
	baseaction.AutoAdvanceTurnIfNeeded(g, playerID, log)

	log.Info("Behavior choice confirmation completed",
		slog.String("source", selection.Source),
		slog.Int("choice_selected", choiceIndex))

	return nil
}

// ExecuteOptional resolves an optional effect after its card's guaranteed gains.
func (a *ConfirmBehaviorChoiceAction) ExecuteOptional(ctx context.Context, gameID, playerID, resolutionID string, accept bool) error {
	log := a.InitLogger(gameID, playerID)
	g, err := baseaction.ValidateActiveGame(ctx, a.GameRepository(), gameID, log)
	if err != nil {
		return err
	}
	p, err := a.GetPlayerFromGame(g, playerID, log)
	if err != nil {
		return err
	}
	selection := p.Selection().GetPendingBehaviorResolution(resolutionID)
	if selection == nil || selection.Kind != "optional" || selection.OptionalBehavior == nil {
		return fmt.Errorf("no pending optional effect found")
	}
	var outputs []shared.CalculatedOutput
	if accept {
		if errors := baseaction.CalculateOptionalEffectErrors(selection, p, g, a.CardRegistryForGame(g)); len(errors) > 0 {
			return fmt.Errorf("optional effect unavailable: %s", errors[0].Message)
		}
		behavior := selection.OptionalBehavior
		applier := gamecards.NewBehaviorApplier(p, g, selection.Source, log).WithSourceCardID(selection.SourceCardID).WithCardRegistry(a.CardRegistryForGame(g)).WithSourceType(shared.SourceTypeCardPlay)
		if err := applier.ValidateModeOutputs(behavior.Outputs); err != nil {
			return err
		}
		if err := applier.ValidateResourceOutputs(behavior.Outputs); err != nil {
			return err
		}
		if err := applier.ApplyInputs(ctx, behavior.Inputs); err != nil {
			return err
		}
		outputs, err = applier.ApplyOutputsAndGetCalculated(ctx, behavior.Outputs)
		if err != nil {
			return err
		}
	}
	p.Selection().RemovePendingBehaviorResolution(resolutionID)
	description := "Skipped optional effect"
	if accept {
		description = "Applied optional effect"
	}
	a.WriteStateLogWithChoiceAndOutputs(ctx, g, selection.Source, shared.SourceTypeCardPlay, playerID, description, nil, outputs)
	baseaction.AutoAdvanceTurnIfNeeded(g, playerID, log)
	log.Info("Optional effect resolved", slog.String("resolution_id", resolutionID), slog.Bool("accepted", accept))
	return nil
}
