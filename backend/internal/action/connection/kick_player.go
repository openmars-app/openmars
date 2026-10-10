package connection

import (
	"context"
	"fmt"
	"log/slog"

	gameaction "openmars/internal/action/game"
	"openmars/internal/action/turn_management"
	"openmars/internal/game"
	playerPkg "openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/internal/service/bot"
)

// KickPlayerAction handles kicking a player from a game.
// In lobby: removes the player entirely (they can rejoin).
// In active game: marks the player as exited (permanently skipped, cannot reconnect).
type KickPlayerAction struct {
	gameRepo           game.GameRepository
	botStopper         bot.BotStopper
	finalScoringAction *gameaction.FinalScoringAction
	logger             *slog.Logger
}

func NewKickPlayerAction(
	gameRepo game.GameRepository,
	botStopper bot.BotStopper,
	finalScoringAction *gameaction.FinalScoringAction,
	logger *slog.Logger,
) *KickPlayerAction {
	return &KickPlayerAction{
		gameRepo:           gameRepo,
		botStopper:         botStopper,
		finalScoringAction: finalScoringAction,
		logger:             logger,
	}
}

func (a *KickPlayerAction) Execute(ctx context.Context, gameID string, requesterID string, targetPlayerID string) error {
	log := a.logger.With(
		slog.String("game_id", gameID),
		slog.String("requester_id", requesterID),
		slog.String("target_player_id", targetPlayerID),
		slog.String("action", "kick_player"),
	)

	g, err := a.gameRepo.Get(ctx, gameID)
	if err != nil {
		log.Error("Failed to get game", slog.Any("error", err))
		return fmt.Errorf("game not found: %s", gameID)
	}

	if g.HostPlayerID() != requesterID {
		return fmt.Errorf("cannot kick player: only host can kick players")
	}

	if requesterID == targetPlayerID {
		return fmt.Errorf("cannot kick player: cannot kick yourself")
	}

	if g.Status() == shared.GameStatusLobby {
		return a.kickFromLobby(ctx, g, targetPlayerID, log)
	}

	return a.kickFromActiveGame(ctx, g, gameID, targetPlayerID, log)
}

func (a *KickPlayerAction) kickFromLobby(ctx context.Context, g *game.Game, targetPlayerID string, log *slog.Logger) error {
	log.Debug("Kicking player from lobby")

	if err := g.RemovePlayer(ctx, targetPlayerID); err != nil {
		log.Error("Failed to remove player from lobby", slog.Any("error", err))
		return fmt.Errorf("failed to kick player: %w", err)
	}

	remaining := g.GetAllPlayers()
	if len(remaining) == 0 {
		if err := a.gameRepo.Delete(ctx, g.ID()); err != nil {
			log.Error("Failed to delete empty game", slog.Any("error", err))
			return fmt.Errorf("failed to delete empty game: %w", err)
		}
		log.Debug("Game deleted (no players remaining)")
		return nil
	}

	log.Info("Player kicked from lobby")
	return nil
}

func (a *KickPlayerAction) kickFromActiveGame(ctx context.Context, g *game.Game, gameID string, targetPlayerID string, log *slog.Logger) error {
	log.Debug("Kicking player from active game")

	target, err := g.GetPlayer(targetPlayerID)
	if err != nil {
		return fmt.Errorf("player not found: %s", targetPlayerID)
	}

	if target.HasExited() {
		return fmt.Errorf("cannot kick player: player already exited")
	}

	target.SetExited(true)
	target.SetConnected(false)
	target.SetPassed(true)

	if target.IsBot() && a.botStopper != nil {
		a.botStopper.StopBot(g.ID(), targetPlayerID)
		target.SetBotStatus(playerPkg.BotStatusNone)
	}

	a.clearPendingState(ctx, g, targetPlayerID, log)
	a.moveToEndOfTurnOrder(ctx, g, targetPlayerID, log)

	switch g.CurrentPhase() {
	case shared.GamePhaseStartingSelection:
		a.handleStartingSelectionKick(ctx, g, log)
	case shared.GamePhaseAction:
		a.handleActionPhaseKick(ctx, g, gameID, targetPlayerID, log)
	case shared.GamePhaseProductionAndCardDraw:
		a.handleProductionPhaseKick(ctx, g, log)
	}

	log.Info("Player kicked from active game",
		slog.String("target_player_id", targetPlayerID))
	return nil
}

func (a *KickPlayerAction) clearPendingState(ctx context.Context, g *game.Game, playerID string, log *slog.Logger) {
	if g.GetSelectCorporationPhase(playerID) != nil {
		if err := g.SetSelectCorporationPhase(ctx, playerID, nil); err != nil {
			log.Error("Failed to clear corporation phase", slog.Any("error", err))
		}
	}
	if g.GetSelectStartingCardsPhase(playerID) != nil {
		if err := g.SetSelectStartingCardsPhase(ctx, playerID, nil); err != nil {
			log.Error("Failed to clear starting cards phase", slog.Any("error", err))
		}
	}
	if g.GetSelectPreludeCardsPhase(playerID) != nil {
		if err := g.SetSelectPreludeCardsPhase(ctx, playerID, nil); err != nil {
			log.Error("Failed to clear prelude phase", slog.Any("error", err))
		}
	}
	if g.GetPendingTileSelection(playerID) != nil {
		if err := g.SetPendingTileSelection(ctx, playerID, nil); err != nil {
			log.Error("Failed to clear pending tile selection", slog.Any("error", err))
		}
	}
	if g.GetPendingTileSelectionQueue(playerID) != nil {
		if err := g.SetPendingTileSelectionQueue(ctx, playerID, nil); err != nil {
			log.Error("Failed to clear pending tile queue", slog.Any("error", err))
		}
	}
	if g.GetForcedFirstAction(playerID) != nil {
		if err := g.SetForcedFirstAction(ctx, playerID, nil); err != nil {
			log.Error("Failed to clear forced first action", slog.Any("error", err))
		}
	}
	if g.GetProductionPhase(playerID) != nil {
		if err := g.SetProductionPhase(ctx, playerID, nil); err != nil {
			log.Error("Failed to clear production phase", slog.Any("error", err))
		}
	}

	p, err := g.GetPlayer(playerID)
	if err != nil {
		return
	}
	sel := p.Selection()
	sel.SetPendingCardSelection(nil)
	sel.SetPendingCardDrawSelection(nil)
	sel.ClearPendingBehaviorResolutions()
	sel.SetPendingEffectSelection(nil)
	sel.SetPendingCardReveal(nil)
}

func (a *KickPlayerAction) moveToEndOfTurnOrder(ctx context.Context, g *game.Game, playerID string, log *slog.Logger) {
	turnOrder := g.TurnOrder()
	idx := -1
	for i, id := range turnOrder {
		if id == playerID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return
	}

	newOrder := make([]string, 0, len(turnOrder))
	newOrder = append(newOrder, turnOrder[:idx]...)
	newOrder = append(newOrder, turnOrder[idx+1:]...)
	newOrder = append(newOrder, playerID)

	if err := g.SetTurnOrder(ctx, newOrder); err != nil {
		log.Error("Failed to reorder turn order", slog.Any("error", err))
	}
}

func (a *KickPlayerAction) handleStartingSelectionKick(ctx context.Context, g *game.Game, log *slog.Logger) {
	allPlayers := g.GetAllPlayers()
	for _, p := range allPlayers {
		if p.HasExited() {
			continue
		}
		if g.GetSelectCorporationPhase(p.ID()) != nil ||
			g.GetSelectStartingCardsPhase(p.ID()) != nil ||
			g.GetSelectPreludeCardsPhase(p.ID()) != nil ||
			g.GetPendingTileSelection(p.ID()) != nil ||
			g.GetPendingTileSelectionQueue(p.ID()) != nil {
			log.Debug("Other players still in starting selection")
			return
		}
	}

	log.Debug("All remaining players completed starting selection after kick, advancing to action phase")

	var activePlayers []*playerPkg.Player
	for _, p := range allPlayers {
		if !p.HasExited() {
			activePlayers = append(activePlayers, p)
		}
	}

	if err := g.UpdatePhase(ctx, shared.GamePhaseAction); err != nil {
		log.Error("Failed to transition game phase", slog.Any("error", err))
		return
	}

	turnOrder := g.TurnOrder()
	if len(turnOrder) > 0 {
		firstPlayerID := a.firstActivePlayer(g, turnOrder)
		if firstPlayerID == "" {
			return
		}
		availableActions := 2
		if len(activePlayers) == 1 {
			availableActions = -1
		}
		if err := g.SetCurrentTurn(ctx, firstPlayerID, availableActions); err != nil {
			log.Error("Failed to set current turn", slog.Any("error", err))
		}
	}
}

func (a *KickPlayerAction) handleActionPhaseKick(ctx context.Context, g *game.Game, gameID string, targetPlayerID string, log *slog.Logger) {
	currentTurn := g.CurrentTurn()
	if currentTurn == nil {
		return
	}

	isTheirTurn := currentTurn.PlayerID() == targetPlayerID

	turnOrder := g.TurnOrder()
	activeCount := 0
	for _, id := range turnOrder {
		p, _ := g.GetPlayer(id)
		if p != nil && !p.HasPassed() && !p.HasExited() {
			activeCount++
		}
	}

	if activeCount == 0 {
		a.triggerEndOfGeneration(ctx, g, gameID, log)
		return
	}

	if !isTheirTurn {
		if activeCount == 1 {
			for _, id := range turnOrder {
				p, _ := g.GetPlayer(id)
				if p != nil && !p.HasPassed() && !p.HasExited() {
					if err := g.SetCurrentTurn(ctx, p.ID(), -1); err != nil {
						log.Error("Failed to grant unlimited actions", slog.Any("error", err))
					}
					log.Debug("Last active player granted unlimited actions after kick",
						slog.String("player_id", p.ID()))
					break
				}
			}
		}
		return
	}

	a.advanceToNextActivePlayer(ctx, g, targetPlayerID, activeCount, log)
}

func (a *KickPlayerAction) advanceToNextActivePlayer(ctx context.Context, g *game.Game, currentPlayerID string, activeCount int, log *slog.Logger) {
	turnOrder := g.TurnOrder()
	currentIdx := -1
	for i, id := range turnOrder {
		if id == currentPlayerID {
			currentIdx = i
			break
		}
	}
	if currentIdx == -1 {
		return
	}

	nextActions := 2
	if activeCount == 1 {
		nextActions = -1
	}

	for i := 1; i < len(turnOrder); i++ {
		nextIdx := (currentIdx + i) % len(turnOrder)
		nextPlayer, _ := g.GetPlayer(turnOrder[nextIdx])
		if nextPlayer != nil && !nextPlayer.HasPassed() && !nextPlayer.HasExited() {
			if err := g.SetCurrentTurn(ctx, nextPlayer.ID(), nextActions); err != nil {
				log.Error("Failed to advance turn", slog.Any("error", err))
				return
			}
			log.Debug("Advanced turn after kick",
				slog.String("next_player_id", nextPlayer.ID()))
			return
		}
	}
}

func (a *KickPlayerAction) handleProductionPhaseKick(ctx context.Context, g *game.Game, log *slog.Logger) {
	allPlayers := g.GetAllPlayers()
	allComplete := true
	for _, p := range allPlayers {
		if p.HasExited() {
			continue
		}
		pPhase := g.GetProductionPhase(p.ID())
		if pPhase == nil || !pPhase.SelectionComplete {
			allComplete = false
			break
		}
	}

	if !allComplete {
		log.Debug("Other players still in production phase")
		return
	}

	log.Debug("All remaining players completed production after kick, advancing to action phase")

	if err := g.UpdatePhase(ctx, shared.GamePhaseAction); err != nil {
		log.Error("Failed to transition game phase", slog.Any("error", err))
		return
	}

	turnOrder := g.TurnOrder()
	if len(turnOrder) > 0 {
		firstPlayerID := a.firstActivePlayer(g, turnOrder)
		if firstPlayerID == "" {
			return
		}

		activeCount := 0
		for _, id := range turnOrder {
			p, _ := g.GetPlayer(id)
			if p != nil && !p.HasExited() {
				activeCount++
			}
		}

		availableActions := 2
		if activeCount == 1 {
			availableActions = -1
		}
		if err := g.SetCurrentTurn(ctx, firstPlayerID, availableActions); err != nil {
			log.Error("Failed to set current turn", slog.Any("error", err))
		}
	}

	for _, p := range allPlayers {
		if !p.HasExited() {
			p.Actions().ResetGenerationCounts()
		}
		if err := g.SetProductionPhase(ctx, p.ID(), nil); err != nil {
			log.Warn("Failed to clear production phase",
				slog.String("player_id", p.ID()),
				slog.Any("error", err))
		}
	}
}

func (a *KickPlayerAction) triggerEndOfGeneration(ctx context.Context, g *game.Game, gameID string, log *slog.Logger) {
	if g.CurrentPhase() == shared.GamePhaseFinalPhase {
		if err := a.finalScoringAction.Execute(ctx, gameID); err != nil {
			log.Error("Failed to execute final scoring", slog.Any("error", err))
		}
		return
	}

	log.Debug("All players finished after kick - triggering production phase")

	activePlayers := []*playerPkg.Player{}
	for _, p := range g.GetAllPlayers() {
		if !p.HasExited() {
			activePlayers = append(activePlayers, p)
		}
	}

	if g.GlobalParameters().IsMaxed() {
		if err := turn_management.ExecuteFinalProductionPhase(ctx, g, activePlayers, log); err != nil {
			log.Error("Failed final production", slog.Any("error", err))
		}
		return
	}
	if err := turn_management.ExecuteProductionPhase(ctx, g, activePlayers, log); err != nil {
		log.Error("Failed to execute production phase after kick", slog.Any("error", err))
	}
}

func (a *KickPlayerAction) firstActivePlayer(g *game.Game, turnOrder []string) string {
	for _, id := range turnOrder {
		p, _ := g.GetPlayer(id)
		if p != nil && !p.HasExited() {
			return p.ID()
		}
	}
	return ""
}
