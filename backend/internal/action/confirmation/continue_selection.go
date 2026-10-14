package confirmation

import (
	"context"
	"fmt"
	baseaction "openmars/internal/action"
	"openmars/internal/game"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// continueSelection only completes an effect whose remaining choices are impossible.
func continueSelection(ctx context.Context, base *baseaction.BaseAction, gameID, playerID string,
	inspect func(*game.Game, *player.Player) (string, bool, error), clear func(*player.Player)) error {
	log := base.InitLogger(gameID, playerID)
	g, err := baseaction.ValidateActiveGame(ctx, base.GameRepository(), gameID, log)
	if err != nil {
		return err
	}
	if err := baseaction.ValidateCurrentTurn(g, playerID, log); err != nil {
		return err
	}
	p, err := base.GetPlayerFromGame(g, playerID, log)
	if err != nil {
		return err
	}
	source, allowed, err := inspect(g, p)
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("selection still has legal choices")
	}
	clear(p)
	base.WriteStateLog(ctx, g, source, shared.SourceTypeCardPlay, playerID, "Continued without a selection: no legal choices")
	baseaction.AutoAdvanceTurnIfNeeded(g, playerID, log, base.CardRegistryForGame(g))
	log.Info("Continued impossible pending effect")
	return nil
}

func (a *ConfirmColonyPlacementAction) Continue(ctx context.Context, gameID, playerID string) error {
	return continueSelection(ctx, &a.BaseAction, gameID, playerID, func(g *game.Game, p *player.Player) (string, bool, error) {
		pending := p.Selection().GetPendingColonySelection()
		if pending == nil {
			return "", false, fmt.Errorf("no pending colony selection")
		}
		return pending.Source, len(baseaction.PendingColonyChoices(g, p, pending)) == 0, nil
	}, func(p *player.Player) { p.Selection().SetPendingColonySelection(nil) })
}

func (a *ConfirmFreeTradeAction) Continue(ctx context.Context, gameID, playerID string) error {
	return continueSelection(ctx, &a.BaseAction, gameID, playerID, func(g *game.Game, p *player.Player) (string, bool, error) {
		pending := p.Selection().GetPendingFreeTradeSelection()
		if pending == nil {
			return "", false, fmt.Errorf("no pending free trade selection")
		}
		return pending.Source, len(baseaction.PendingFreeTradeChoices(g, p, pending)) == 0, nil
	}, func(p *player.Player) { p.Selection().SetPendingFreeTradeSelection(nil) })
}

func (a *ConfirmAwardFundAction) Continue(ctx context.Context, gameID, playerID string) error {
	return continueSelection(ctx, &a.BaseAction, gameID, playerID, func(g *game.Game, p *player.Player) (string, bool, error) {
		pending := p.Selection().GetPendingAwardFundSelection()
		if pending == nil {
			return "", false, fmt.Errorf("no pending award fund selection")
		}
		return pending.Source, len(baseaction.PendingAwardChoices(g, pending, a.awardRegistry)) == 0, nil
	}, func(p *player.Player) { p.Selection().SetPendingAwardFundSelection(nil) })
}

func (a *ConfirmEffectSelectionAction) Continue(ctx context.Context, gameID, playerID string) error {
	return continueSelection(ctx, &a.BaseAction, gameID, playerID, func(g *game.Game, p *player.Player) (string, bool, error) {
		pending := p.Selection().GetPendingEffectSelection()
		if pending == nil {
			return "", false, fmt.Errorf("no pending effect selection")
		}
		return pending.Source, baseaction.CanContinuePendingEffect(g, p, pending, a.CardRegistryForGame(g), a.colonies), nil
	}, func(p *player.Player) { p.Selection().SetPendingEffectSelection(nil) })
}
