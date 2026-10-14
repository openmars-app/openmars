package action

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"openmars/internal/events"
	"openmars/internal/game"
	"openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/internal/logger"
)

// BaseAction provides common dependencies for all actions.
// Following the new architecture: actions use ONLY GameRepository (+ logger + card registry)
// Broadcasting happens automatically via events published by Game methods
type BaseAction struct {
	gameRepo     game.GameRepository
	cardRegistry cards.CardRegistry
	stateRepo    game.GameStateRepository
	logger       *slog.Logger
}

// NewBaseAction creates a new BaseAction with minimal dependencies
func NewBaseAction(gameRepo game.GameRepository, cardRegistry cards.CardRegistry) BaseAction {
	return BaseAction{
		gameRepo:     gameRepo,
		cardRegistry: cardRegistry,
		logger:       logger.Get(),
	}
}

// NewBaseActionWithStateRepo creates a new BaseAction with state repository for logging
func NewBaseActionWithStateRepo(gameRepo game.GameRepository, cardRegistry cards.CardRegistry, stateRepo game.GameStateRepository) BaseAction {
	return BaseAction{
		gameRepo:     gameRepo,
		cardRegistry: cardRegistry,
		stateRepo:    stateRepo,
		logger:       logger.Get(),
	}
}

// InitLogger creates a logger with game and player context
// This should be called at the start of every Execute method
func (b *BaseAction) InitLogger(gameID, playerID string) *slog.Logger {
	return b.logger.With(
		slog.String("game_id", gameID),
		slog.String("player_id", playerID),
	)
}

// GetLogger returns the base logger for actions that don't have game/player context
func (b *BaseAction) GetLogger() *slog.Logger {
	return b.logger
}

// GameRepository returns the game repository
func (b *BaseAction) GameRepository() game.GameRepository {
	return b.gameRepo
}

// CardRegistry returns the card registry
func (b *BaseAction) CardRegistry() cards.CardRegistry {
	return b.cardRegistry
}

// CardRegistryForGame resolves this action's library for the game's selection.
func (b *BaseAction) CardRegistryForGame(g *game.Game) cards.CardRegistry {
	return cards.ForGame(b.cardRegistry, g)
}

// StateRepository returns the game state repository (may be nil)
func (b *BaseAction) StateRepository() game.GameStateRepository {
	return b.stateRepo
}

// WriteStateLog writes a state diff to the state repository if configured
func (b *BaseAction) WriteStateLog(ctx context.Context, g *game.Game, source string, sourceType shared.SourceType, playerID, description string) {
	b.WriteStateLogWithChoice(ctx, g, source, sourceType, playerID, description, nil)
}

// WriteStateLogWithChoice writes a state diff with an optional choice index
func (b *BaseAction) WriteStateLogWithChoice(ctx context.Context, g *game.Game, source string, sourceType shared.SourceType, playerID, description string, choiceIndex *int) {
	b.WriteStateLogWithChoiceAndOutputs(ctx, g, source, sourceType, playerID, description, choiceIndex, nil)
}

// WriteStateLogWithChoiceAndOutputs writes a state diff with optional choice index and calculated outputs
func (b *BaseAction) WriteStateLogWithChoiceAndOutputs(ctx context.Context, g *game.Game, source string, sourceType shared.SourceType, playerID, description string, choiceIndex *int, calculatedOutputs []shared.CalculatedOutput) {
	b.WriteStateLogFull(ctx, g, source, sourceType, playerID, description, choiceIndex, calculatedOutputs, nil)
}

// WriteStateLogFull writes a state diff with all optional fields including display data
func (b *BaseAction) WriteStateLogFull(ctx context.Context, g *game.Game, source string, sourceType shared.SourceType, playerID, description string, choiceIndex *int, calculatedOutputs []shared.CalculatedOutput, displayData *game.LogDisplayData) {
	if p, err := g.GetPlayer(playerID); err == nil {
		if reveal := p.Selection().GetPendingCardReveal(); reveal != nil && reveal.Source == source {
			if displayData == nil {
				displayData = &game.LogDisplayData{}
			}
			displayData.RevealedCards = append([]shared.RevealedCard(nil), reveal.Cards...)
		}
	}
	if b.stateRepo == nil {
		return
	}
	_, err := b.stateRepo.WriteFull(ctx, g.ID(), g, source, sourceType, playerID, description, choiceIndex, calculatedOutputs, displayData)
	if err != nil {
		b.logger.Warn("Failed to write state log",
			slog.String("game_id", g.ID()),
			slog.String("source", source),
			slog.Any("error", err))
	}
}

// GetPlayerFromGame fetches a player from the game with consistent error handling
func (b *BaseAction) GetPlayerFromGame(g *game.Game, playerID string, log *slog.Logger) (*player.Player, error) {
	p, err := g.GetPlayer(playerID)
	if err != nil {
		log.Error("Player not found in game", slog.Any("error", err))
		return nil, fmt.Errorf("player not found: %s", playerID)
	}
	return p, nil
}

// ConsumePlayerAction consumes an action from the game's current turn
// Returns true if an action was consumed, false if unlimited (-1) or no actions remaining (0)
// This properly handles unlimited actions by not consuming them
// When the last action is consumed and no tile placement is pending, auto-advances to the next player
func (b *BaseAction) ConsumePlayerAction(g *game.Game, log *slog.Logger) bool {
	ResolveForcedPassiveChoices(g, log, b.CardRegistryForGame(g))
	currentTurn := g.CurrentTurn()
	if currentTurn == nil {
		log.Warn("No current turn set, cannot consume action")
		return false
	}

	currentTurn.IncrementGlobalActionCounter()
	playerID := currentTurn.PlayerID()
	consumed := currentTurn.ConsumeAction()
	if consumed {
		log.Debug("Action consumed", slog.Int("remaining_actions", currentTurn.ActionsRemaining()))

		if currentTurn.ActionsRemaining() == 0 {
			AutoAdvanceTurnIfNeeded(g, playerID, log, b.CardRegistryForGame(g))
		}

		if eventBus := g.EventBus(); eventBus != nil {
			events.Publish(eventBus, events.GameStateChangedEvent{
				GameID:    g.ID(),
				Timestamp: time.Now(),
			})
		}
	}

	return consumed
}

// AutoAdvanceTurnIfNeeded advances the turn to the next non-passed player
// if the current player has 0 actions remaining and no pending tile selection.
// This is called after consuming an action or after completing a tile placement.
func AutoAdvanceTurnIfNeeded(g *game.Game, playerID string, log *slog.Logger, registry cards.CardRegistry) {
	ResolveForcedPassiveChoices(g, log, registry)
	g.CompleteFirstActionIfReady(playerID)
	currentTurn := g.CurrentTurn()
	if currentTurn == nil {
		return
	}
	if currentTurn.PlayerID() != playerID {
		return
	}
	if currentTurn.ActionsRemaining() != 0 {
		return
	}
	if g.HasAnyPendingSelection(playerID) {
		return
	}

	turnOrder := g.TurnOrder()
	if len(turnOrder) == 0 {
		return
	}

	currentIndex := -1
	for i, id := range turnOrder {
		if id == playerID {
			currentIndex = i
			break
		}
	}
	if currentIndex == -1 {
		return
	}

	// Count non-passed players to determine if next player should get unlimited actions
	nonPassedCount := 0
	for _, id := range turnOrder {
		p, err := g.GetPlayer(id)
		if err == nil && !p.HasPassed() {
			nonPassedCount++
		}
	}

	// If current player is the only non-passed player, give them unlimited actions
	if nonPassedCount == 1 {
		currentPlayer, err := g.GetPlayer(playerID)
		if err == nil && !currentPlayer.HasPassed() {
			if err := g.SetCurrentTurn(context.Background(), playerID, -1); err != nil {
				log.Error("Failed to grant unlimited actions to last player", slog.Any("error", err))
			} else {
				log.Debug("Last non-passed player granted unlimited actions",
					slog.String("player_id", playerID))
			}
			return
		}
	}

	for i := 1; i < len(turnOrder); i++ {
		nextIndex := (currentIndex + i) % len(turnOrder)
		nextID := turnOrder[nextIndex]
		nextPlayer, err := g.GetPlayer(nextID)
		if err != nil {
			continue
		}
		if !nextPlayer.HasPassed() {
			// If next player will be the only non-passed player, give unlimited actions
			nextActions := 2
			if nonPassedCount == 1 {
				nextActions = -1
				log.Debug("Last non-passed player granted unlimited actions",
					slog.String("player_id", nextID))
			}
			if err := g.SetCurrentTurn(context.Background(), nextID, nextActions); err != nil {
				log.Error("Failed to auto-advance turn", slog.Any("error", err))
			} else {
				log.Debug("Auto-advanced turn to next player",
					slog.String("from", playerID),
					slog.String("to", nextID),
					slog.Int("actions", nextActions))
			}
			return
		}
	}
}
