package admin

import (
	"context"
	"fmt"
	"log/slog"

	"openmars/internal/game"
	"openmars/internal/game/cards"
)

// GiveCardAction handles the admin action to give a card to a player
type GiveCardAction struct {
	gameRepo     game.GameRepository
	cardRegistry cards.CardRegistry
	logger       *slog.Logger
}

// NewGiveCardAction creates a new give card admin action
func NewGiveCardAction(
	gameRepo game.GameRepository,
	cardRegistry cards.CardRegistry,
	logger *slog.Logger,
) *GiveCardAction {
	return &GiveCardAction{
		gameRepo:     gameRepo,
		cardRegistry: cardRegistry,
		logger:       logger,
	}
}

// Execute performs the give card admin action
func (a *GiveCardAction) Execute(ctx context.Context, gameID string, playerID string, cardID string) error {
	log := a.logger.With(
		slog.String("game_id", gameID),
		slog.String("player_id", playerID),
		slog.String("action", "admin_give_card"),
		slog.String("card_id", cardID),
	)
	log.Debug("Admin: Giving card to player")

	game, err := a.gameRepo.Get(ctx, gameID)
	if err != nil {
		log.Error("Failed to get game", slog.Any("error", err))
		return fmt.Errorf("game not found: %s", gameID)
	}

	player, err := game.GetPlayer(playerID)
	if err != nil {
		log.Error("Player not found in game", slog.Any("error", err))
		return fmt.Errorf("player not found: %s", playerID)
	}

	if _, err := cards.ForGame(a.cardRegistry, game).GetByID(cardID); err != nil {
		return fmt.Errorf("card unavailable in selected mode: %w", err)
	}
	player.Hand().AddCard(cardID)

	log.Info("Admin give card completed")
	return nil
}
