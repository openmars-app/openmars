package confirmation

import (
	"context"
	"log/slog"

	confirmaction "openmars/internal/action/confirmation"
	"openmars/internal/delivery/dto"
	"openmars/internal/delivery/websocket/core"
	"openmars/internal/logger"
)

// ConfirmAwardFundHandler handles confirm award fund requests
type ConfirmAwardFundHandler struct {
	action      *confirmaction.ConfirmAwardFundAction
	broadcaster Broadcaster
	logger      *slog.Logger
}

// NewConfirmAwardFundHandler creates a new confirm award fund handler
func NewConfirmAwardFundHandler(action *confirmaction.ConfirmAwardFundAction, broadcaster Broadcaster) *ConfirmAwardFundHandler {
	return &ConfirmAwardFundHandler{
		action:      action,
		broadcaster: broadcaster,
		logger:      logger.Get(),
	}
}

// HandleMessage implements the MessageHandler interface
func (h *ConfirmAwardFundHandler) HandleMessage(ctx context.Context, connection *core.Connection, message dto.WebSocketMessage) {
	log := h.logger.With(
		slog.String("connection_id", connection.ID),
		slog.String("message_type", string(message.Type)),
	)

	log.Debug("Processing confirm award fund request")

	if connection.GameID() == "" || connection.PlayerID() == "" {
		log.Error("Missing connection context")
		connection.SendError(message.Type, "Not connected to a game")
		return
	}

	payloadMap, ok := message.Payload.(map[string]interface{})
	if !ok {
		log.Error("Invalid payload format")
		connection.SendError(message.Type, "Invalid payload format")
		return
	}

	if value, exists := payloadMap["skip"]; exists {
		skip, ok := value.(bool)
		if !ok {
			connection.SendError(message.Type, "skip must be a boolean")
			return
		}
		if skip {
			if len(payloadMap) != 1 {
				connection.SendError(message.Type, "skip cannot include a selection")
				return
			}
			if err := h.action.Continue(ctx, connection.GameID(), connection.PlayerID()); err != nil {
				connection.SendError(message.Type, err.Error())
				return
			}
			h.broadcaster.BroadcastGameState(connection.GameID(), nil)
			return
		}
	}

	awardType, ok := payloadMap["awardType"].(string)
	if !ok || awardType == "" {
		log.Error("Missing or invalid awardType in payload")
		connection.SendError(message.Type, "Missing awardType")
		return
	}

	err := h.action.Execute(ctx, connection.GameID(), connection.PlayerID(), awardType)
	if err != nil {
		log.Error("Failed to execute confirm award fund action", slog.Any("error", err))
		connection.SendError(message.Type, err.Error())
		return
	}

	log.Debug("Award fund confirmed")

	h.broadcaster.BroadcastGameState(connection.GameID(), nil)

}
