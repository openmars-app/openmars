package confirmation

import (
	"context"
	"log/slog"

	confirmaction "openmars/internal/action/confirmation"
	"openmars/internal/delivery/dto"
	"openmars/internal/delivery/websocket/core"
	"openmars/internal/logger"
)

// ConfirmEffectSelectionHandler handles confirm effect selection requests
type ConfirmEffectSelectionHandler struct {
	action      *confirmaction.ConfirmEffectSelectionAction
	broadcaster Broadcaster
	logger      *slog.Logger
}

// NewConfirmEffectSelectionHandler creates a new confirm effect selection handler
func NewConfirmEffectSelectionHandler(action *confirmaction.ConfirmEffectSelectionAction, broadcaster Broadcaster) *ConfirmEffectSelectionHandler {
	return &ConfirmEffectSelectionHandler{
		action:      action,
		broadcaster: broadcaster,
		logger:      logger.Get(),
	}
}

// HandleMessage implements the MessageHandler interface
func (h *ConfirmEffectSelectionHandler) HandleMessage(ctx context.Context, connection *core.Connection, message dto.WebSocketMessage) {
	log := h.logger.With(
		slog.String("connection_id", connection.ID),
		slog.String("message_type", string(message.Type)),
	)

	log.Debug("Processing confirm effect selection request")

	if connection.GameID() == "" || connection.PlayerID() == "" {
		connection.SendError(message.Type, "Not connected to a game")
		return
	}

	payloadMap, ok := message.Payload.(map[string]interface{})
	if !ok {
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

	option, ok := payloadMap["optionIndex"].(float64)
	if !ok || option != float64(int(option)) {
		connection.SendError(message.Type, "Missing or invalid optionIndex in payload")
		return
	}

	err := h.action.Execute(ctx, connection.GameID(), connection.PlayerID(), int(option))
	if err != nil {
		log.Error("Failed to execute confirm effect selection action", slog.Any("error", err))
		connection.SendError(message.Type, err.Error())
		return
	}

	log.Debug("Effect selection confirmed")

	h.broadcaster.BroadcastGameState(connection.GameID(), nil)

}
