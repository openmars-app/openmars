package confirmation

import (
	"context"
	"fmt"
	baseaction "openmars/internal/action"
	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/shared"
)

// ConfirmEffectSelectionAction resolves an effect's complete source/target assignment.
type ConfirmEffectSelectionAction struct {
	baseaction.BaseAction
	colonies gamecards.ColonyBonusLookup
}

func NewConfirmEffectSelectionAction(repo game.GameRepository, registry gamecards.CardRegistry, colonies gamecards.ColonyBonusLookup, stateRepo game.GameStateRepository) *ConfirmEffectSelectionAction {
	return &ConfirmEffectSelectionAction{BaseAction: baseaction.NewBaseActionWithStateRepo(repo, registry, stateRepo), colonies: colonies}
}

func (a *ConfirmEffectSelectionAction) Execute(ctx context.Context, gameID, playerID string, optionIndex int) error {
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
	pending := p.Selection().GetPendingEffectSelection()
	if pending == nil {
		return fmt.Errorf("no pending effect selection")
	}
	if optionIndex < 0 || optionIndex >= len(pending.Options) {
		return fmt.Errorf("invalid effect option")
	}
	options, _, err := gamecards.EffectSelectionOptions(pending.Outputs, p, g, a.CardRegistryForGame(g), a.colonies)
	if err != nil {
		return err
	}
	var selected *shared.EffectSelectionOption
	for i := range options {
		if gamecards.SameEffectSelection(options[i], pending.Options[optionIndex]) {
			selected = &options[i]
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("effect selection is no longer legal")
	}
	description := "Moved colony tracks"
	if selected.CardID != "" {
		source, err := a.CardRegistryForGame(g).GetByID(selected.CardID)
		if err != nil {
			return err
		}
		description = "Copied production from " + source.Name
		applier := gamecards.NewBehaviorApplier(p, g, pending.Source, log).WithSourceCardID(pending.SourceCardID).WithCardRegistry(a.CardRegistryForGame(g)).WithTargetPlayerID(selected.TargetPlayerID).WithSourceType(shared.SourceTypeCardPlay)
		if err := applier.ApplyOutputs(ctx, selected.Outputs); err != nil {
			return err
		}
	} else {
		moves := map[string]int{}
		i := 0
		for _, output := range pending.Outputs {
			if output.GetResourceType() != shared.ResourceColonyTrackStep {
				continue
			}
			moves[selected.ColonyIDs[i]] += output.GetAmount()
			i++
		}
		g.Colonies().MoveTradeMarkers(moves)
	}
	p.Selection().SetPendingEffectSelection(nil)
	a.WriteStateLog(ctx, g, pending.Source, shared.SourceTypeCardPlay, playerID, description)
	baseaction.AutoAdvanceTurnIfNeeded(g, playerID, log, a.CardRegistryForGame(g))
	return nil
}
