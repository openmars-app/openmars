package turn_management

import (
	"context"
	"fmt"
	"log/slog"

	"openmars/internal/game"
	"openmars/internal/game/award"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// ConfirmInitAdvanceAction advances the init phase to the next player's corp/prelude application.
// The frontend sends this after displaying effects and completing any required tile placements.
type ConfirmInitAdvanceAction struct {
	gameRepo      game.GameRepository
	cardRegistry  gamecards.CardRegistry
	awardRegistry award.AwardRegistry
	stateRepo     game.GameStateRepository
	corpProc      *gamecards.CorporationProcessor
	logger        *slog.Logger
}

// NewConfirmInitAdvanceAction creates a new confirm init advance action
func NewConfirmInitAdvanceAction(
	gameRepo game.GameRepository,
	cardRegistry gamecards.CardRegistry,
	awardRegistry award.AwardRegistry,
	stateRepo game.GameStateRepository,
	logger *slog.Logger,
) *ConfirmInitAdvanceAction {
	return &ConfirmInitAdvanceAction{
		gameRepo:      gameRepo,
		cardRegistry:  cardRegistry,
		awardRegistry: awardRegistry,
		stateRepo:     stateRepo,
		corpProc:      gamecards.NewCorporationProcessor(cardRegistry, awardRegistry, slog.Default()),
		logger:        logger,
	}
}

// Execute applies the current player's effects and advances to the next player.
// The confirm flow is: enter phase (no effects applied) → confirm → apply current → wait →
// confirm → apply next → wait → ... → confirm → apply last → wait → confirm → roster →
// confirm → action phase. The roster step only follows the final init phase.
func (a *ConfirmInitAdvanceAction) Execute(ctx context.Context, gameID string, playerID string) error {
	log := a.logger.With(
		slog.String("game_id", gameID),
		slog.String("player_id", playerID),
		slog.String("action", "confirm_init_advance"),
	)

	g, err := a.gameRepo.Get(ctx, gameID)
	if err != nil {
		return fmt.Errorf("game not found: %s", gameID)
	}

	phase := g.CurrentPhase()
	if phase != shared.GamePhaseInitApplyCorp && phase != shared.GamePhaseInitApplyPrelude {
		return fmt.Errorf("game not in init apply phase, current: %s", phase)
	}

	if !mayAdvanceShowcase(g, playerID) {
		return fmt.Errorf("only the host can advance the showcase")
	}

	if !g.InitPhaseWaitingForConfirm() {
		return fmt.Errorf("init phase not waiting for confirmation")
	}

	if g.InitPhaseRoster() {
		return finishInitRoster(ctx, g, log)
	}

	turnOrder := g.TurnOrder()
	currentIndex := g.InitPhasePlayerIndex()
	if currentIndex >= len(turnOrder) {
		return fmt.Errorf("init phase player index out of range")
	}

	currentPlayerID := turnOrder[currentIndex]

	if g.HasAnyPendingSelection(currentPlayerID) {
		return fmt.Errorf("current player has pending selection")
	}
	if g.GetPendingTileSelectionQueue(currentPlayerID) != nil {
		return fmt.Errorf("current player has pending tile queue")
	}

	if needsInitApply(g, phase, currentPlayerID) {
		if err := g.SetInitPhaseWaitingForConfirm(ctx, false); err != nil {
			return fmt.Errorf("failed to clear waiting for confirm: %w", err)
		}
		return a.applyCurrentPlayer(ctx, g, phase, currentPlayerID, log)
	}

	if _, err := AdvanceInitPhaseAfterForcedAction(ctx, g, log); err != nil {
		return err
	}
	return nil
}

// AdvanceInitPhaseAfterForcedAction advances the init phase to the next player when the
// current init player's effects have already been applied and all per-player gates are
// satisfied. It is the single source of truth for the already-applied advance path,
// shared between the confirm flow and backend auto-advance after a forced action.
//
// It returns (true, nil) when it cleared the waiting-for-confirm flag and advanced.
// It returns (false, nil) without mutating state when advancement is not applicable:
// when the phase is not an init-apply phase, when not waiting for confirm, when the
// roster is showing, when the init player index is out of range, or when the current init
// player still has a pending selection or a pending tile queue.
func AdvanceInitPhaseAfterForcedAction(ctx context.Context, g *game.Game, log *slog.Logger) (bool, error) {
	phase := g.CurrentPhase()
	if phase != shared.GamePhaseInitApplyCorp && phase != shared.GamePhaseInitApplyPrelude {
		return false, nil
	}

	if !g.InitPhaseWaitingForConfirm() || g.InitPhaseRoster() {
		return false, nil
	}

	turnOrder := g.TurnOrder()
	currentIndex := g.InitPhasePlayerIndex()
	if currentIndex >= len(turnOrder) {
		return false, nil
	}

	currentPlayerID := turnOrder[currentIndex]

	if g.HasAnyPendingSelection(currentPlayerID) {
		return false, nil
	}
	if g.GetPendingTileSelectionQueue(currentPlayerID) != nil {
		return false, nil
	}
	if needsInitApply(g, phase, currentPlayerID) {
		return false, nil
	}

	if err := g.SetInitPhaseWaitingForConfirm(ctx, false); err != nil {
		return false, fmt.Errorf("failed to clear waiting for confirm: %w", err)
	}

	if err := advanceToNextPlayer(ctx, g, phase, currentIndex, turnOrder, g.GetAllPlayers(), log); err != nil {
		return false, err
	}
	return true, nil
}

func (a *ConfirmInitAdvanceAction) applyCurrentPlayer(ctx context.Context, g *game.Game, phase shared.GamePhase, currentPlayerID string, log *slog.Logger) error {
	switch phase {
	case shared.GamePhaseInitApplyCorp:
		if err := ApplyCorpForPlayer(ctx, g, currentPlayerID, gamecards.ForGame(a.cardRegistry, g), a.corpProc, log); err != nil {
			return fmt.Errorf("failed to apply corp for player %s: %w", currentPlayerID, err)
		}
		log.Debug("Applied corp effects", slog.String("player_id", currentPlayerID))

	case shared.GamePhaseInitApplyPrelude:
		if err := ApplyNextPreludeForPlayer(ctx, g, currentPlayerID, gamecards.ForGame(a.cardRegistry, g), a.stateRepo, log); err != nil {
			return fmt.Errorf("failed to apply prelude for player %s: %w", currentPlayerID, err)
		}
	}

	// After applying, wait for the frontend to display the effects
	if err := g.SetInitPhaseWaitingForConfirm(ctx, true); err != nil {
		return fmt.Errorf("failed to set waiting for confirm: %w", err)
	}
	return nil
}

func advanceToNextPlayer(ctx context.Context, g *game.Game, phase shared.GamePhase, currentIndex int, turnOrder []string, allPlayers []*player.Player, log *slog.Logger) error {
	nextPlayerID := findNextActivePlayer(g, turnOrder, currentIndex+1)

	if nextPlayerID != "" {
		actualIndex := findPlayerIndex(turnOrder, nextPlayerID)
		if err := g.SetInitPhasePlayerIndex(ctx, actualIndex); err != nil {
			return fmt.Errorf("failed to set init phase player index: %w", err)
		}
		if err := g.SetInitPhaseWaitingForConfirm(ctx, true); err != nil {
			return fmt.Errorf("failed to set waiting for confirm: %w", err)
		}
		return nil
	}

	// No more players in current phase
	switch phase {
	case shared.GamePhaseInitApplyCorp:
		if g.Settings().HasPrelude() {
			log.Debug("All corps applied, advancing to init_apply_prelude phase")
			if err := g.UpdatePhase(ctx, shared.GamePhaseInitApplyPrelude); err != nil {
				return fmt.Errorf("failed to transition to prelude phase: %w", err)
			}

			firstPlayerID := findFirstActivePlayer(g, turnOrder)
			if firstPlayerID == "" {
				AdvanceToActionPhase(ctx, g, allPlayers, log)
				return nil
			}

			firstIndex := findPlayerIndex(turnOrder, firstPlayerID)
			if err := g.SetInitPhasePlayerIndex(ctx, firstIndex); err != nil {
				return fmt.Errorf("failed to reset init phase player index: %w", err)
			}
			if err := g.SetInitPhaseWaitingForConfirm(ctx, true); err != nil {
				return fmt.Errorf("failed to set waiting for confirm: %w", err)
			}
			return nil
		}

		log.Debug("All corps applied (no prelude), showing roster")
		return startInitRoster(ctx, g)

	case shared.GamePhaseInitApplyPrelude:
		log.Debug("All preludes applied, showing roster")
		return startInitRoster(ctx, g)
	}

	return nil
}

// mayAdvanceShowcase reports whether the player may advance the init phase: the host,
// or anyone while the host is disconnected or gone so setup cannot stall.
func mayAdvanceShowcase(g *game.Game, playerID string) bool {
	hostID := g.HostPlayerID()
	if playerID == hostID {
		return true
	}
	host, err := g.GetPlayer(hostID)
	if err != nil {
		return true
	}
	return host.HasExited() || !host.IsConnected()
}

// needsInitApply reports whether the player still has a corp or prelude to apply in
// the current init phase.
func needsInitApply(g *game.Game, phase shared.GamePhase, playerID string) bool {
	choices := g.GetDeferredStartingChoices(playerID)
	if choices == nil {
		return false
	}
	switch phase {
	case shared.GamePhaseInitApplyCorp:
		return !choices.CorpApplied
	case shared.GamePhaseInitApplyPrelude:
		return !choices.PreludesDone()
	}
	return false
}

func startInitRoster(ctx context.Context, g *game.Game) error {
	if err := g.SetInitPhaseRoster(ctx, true); err != nil {
		return fmt.Errorf("failed to set init roster: %w", err)
	}
	if err := g.SetInitPhaseWaitingForConfirm(ctx, true); err != nil {
		return fmt.Errorf("failed to set waiting for confirm: %w", err)
	}
	return nil
}

func finishInitRoster(ctx context.Context, g *game.Game, log *slog.Logger) error {
	if err := g.SetInitPhaseRoster(ctx, false); err != nil {
		return fmt.Errorf("failed to clear init roster: %w", err)
	}
	if err := g.SetInitPhaseWaitingForConfirm(ctx, false); err != nil {
		return fmt.Errorf("failed to clear waiting for confirm: %w", err)
	}
	log.Info("Init roster shown, advancing to action phase")
	AdvanceToActionPhase(ctx, g, g.GetAllPlayers(), log)
	return nil
}

// findNextActivePlayer finds the next non-exited player in turn order starting from the given index
func findNextActivePlayer(g *game.Game, turnOrder []string, fromIndex int) string {
	for i := fromIndex; i < len(turnOrder); i++ {
		p, err := g.GetPlayer(turnOrder[i])
		if err == nil && !p.HasExited() {
			return p.ID()
		}
	}
	return ""
}

func findPlayerIndex(turnOrder []string, playerID string) int {
	for i, id := range turnOrder {
		if id == playerID {
			return i
		}
	}
	return 0
}
