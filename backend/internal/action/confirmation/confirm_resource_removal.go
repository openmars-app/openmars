package confirmation

import (
	"context"
	"fmt"
	"log/slog"
	baseaction "openmars/internal/action"
	"openmars/internal/game"
	"openmars/internal/game/cards"
	"openmars/internal/game/shared"
)

// ConfirmResourceRemovalAction resolves an optional, restricted resource removal.
type ConfirmResourceRemovalAction struct{ baseaction.BaseAction }

// NewConfirmResourceRemovalAction creates a resource removal confirmation action.
func NewConfirmResourceRemovalAction(repo game.GameRepository, registry cards.CardRegistry, stateRepo game.GameStateRepository, _ *slog.Logger) *ConfirmResourceRemovalAction {
	return &ConfirmResourceRemovalAction{BaseAction: baseaction.NewBaseActionWithStateRepo(repo, registry, stateRepo)}
}

// Execute validates the complete selection before mutating any state.
func (a *ConfirmResourceRemovalAction) Execute(ctx context.Context, gameID, playerID, selectionID, targetPlayerID string, amount int) error {
	log := a.InitLogger(gameID, playerID)
	g, err := baseaction.ValidateActiveGame(ctx, a.GameRepository(), gameID, log)
	if err != nil {
		return err
	}
	if err := baseaction.ValidateCurrentTurn(g, playerID, log); err != nil {
		return err
	}
	p, err := a.GetPlayerFromGame(g, playerID, log)
	if err != nil {
		return err
	}
	selection := p.Selection().GetPendingResourceRemovalSelection()
	if selection == nil || selection.ID != selectionID {
		return fmt.Errorf("resource removal selection is no longer pending")
	}
	if amount < 0 || (amount == 0 && targetPlayerID != "") || (amount > 0 && targetPlayerID == "") {
		return fmt.Errorf("invalid resource removal amount or target")
	}
	description := "Skipped resource removal from " + selection.Source
	if amount > 0 {
		targets, err := cards.ResourceRemovalTargets(g, p, selection.Output, selection.Placement, a.CardRegistryForGame(g))
		if err != nil {
			return err
		}
		if amount > targets[targetPlayerID] {
			return fmt.Errorf("target or amount is not eligible for resource removal")
		}
		target, err := g.GetPlayer(targetPlayerID)
		if err != nil {
			return err
		}
		target.Resources().Add(map[shared.ResourceType]int{selection.ResourceType: -amount})
		description = fmt.Sprintf("Removed %d %s from %s", amount, selection.ResourceType, target.Name())
	}
	p.Selection().SetPendingResourceRemovalSelection(nil)
	a.WriteStateLogFull(ctx, g, selection.Source, shared.SourceTypeCardPlay, playerID, description, nil, []shared.CalculatedOutput{{ResourceType: string(selection.ResourceType), Amount: -amount}}, nil)
	baseaction.AutoAdvanceTurnIfNeeded(g, playerID, log, a.CardRegistryForGame(g))
	return nil
}
