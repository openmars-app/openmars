package confirmation

import (
	"context"
	"fmt"
	baseaction "openmars/internal/action"
	"openmars/internal/game"
)

// ConfirmCardRevealAction acknowledges an already resolved public reveal.
type ConfirmCardRevealAction struct{ baseaction.BaseAction }

func NewConfirmCardRevealAction(repo game.GameRepository) *ConfirmCardRevealAction {
	return &ConfirmCardRevealAction{BaseAction: baseaction.NewBaseAction(repo, nil)}
}
func (a *ConfirmCardRevealAction) Execute(ctx context.Context, gameID, playerID string) error {
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
	if p.Selection().GetPendingCardReveal() == nil {
		return fmt.Errorf("no pending card reveal")
	}
	p.Selection().SetPendingCardReveal(nil)
	baseaction.AutoAdvanceTurnIfNeeded(g, playerID, log, a.CardRegistryForGame(g))
	return nil
}
