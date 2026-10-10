package game

import (
	"context"
	"encoding/json"
	"log/slog"

	gameaction "openmars/internal/action/game"
	"openmars/internal/delivery/dto"
	"openmars/internal/delivery/websocket/core"
	"openmars/internal/logger"
)

// UpdateGameSettingsHandler handles incoming update-game-settings messages.
type UpdateGameSettingsHandler struct {
	action      *gameaction.UpdateGameSettingsAction
	broadcaster Broadcaster
	logger      *slog.Logger
}

// NewUpdateGameSettingsHandler creates a new update game settings handler.
func NewUpdateGameSettingsHandler(action *gameaction.UpdateGameSettingsAction, broadcaster Broadcaster) *UpdateGameSettingsHandler {
	return &UpdateGameSettingsHandler{
		action:      action,
		broadcaster: broadcaster,
		logger:      logger.Get(),
	}
}

// HandleMessage implements the MessageHandler interface.
func (h *UpdateGameSettingsHandler) HandleMessage(ctx context.Context, connection *core.Connection, message dto.WebSocketMessage) {
	log := h.logger.With(
		slog.String("connection_id", connection.ID),
		slog.String("message_type", string(message.Type)),
	)

	gameID := connection.GameID()
	playerID := connection.PlayerID()
	if gameID == "" || playerID == "" {
		connection.SendError(message.Type, "Not connected to a game")
		return
	}

	// Round-trip through JSON so we get pointer fields populated only for keys
	// that the client actually sent.
	raw, err := json.Marshal(message.Payload)
	if err != nil {
		connection.SendError(message.Type, "Invalid payload")
		return
	}
	var patch dto.UpdateGameSettingsRequest
	if err := json.Unmarshal(raw, &patch); err != nil {
		connection.SendError(message.Type, "Invalid payload")
		return
	}

	domainPatch := gameaction.SettingsPatch{
		MaxPlayers:      patch.MaxPlayers,
		MapID:           patch.MapID,
		DevelopmentMode: patch.DevelopmentMode,
		DemoGame:        patch.DemoGame,
		AllowRandomBuy:  patch.AllowRandomBuy,
		GameModeID:      patch.GameModeID, GameModeParameters: patch.GameModeParameters, Features: patch.Features,
		ClaudeOAuthToken: patch.ClaudeOAuthToken,
		BotSpendCapUSD:   patch.BotSpendCapUSD,
	}
	if err := h.action.Execute(ctx, gameID, playerID, &domainPatch); err != nil {
		log.Debug("Failed to update game settings", slog.Any("error", err))
		connection.SendError(message.Type, err.Error())
		return
	}

	h.broadcaster.BroadcastGameState(gameID, nil)
	log.Debug("Game settings updated and broadcast")
}
