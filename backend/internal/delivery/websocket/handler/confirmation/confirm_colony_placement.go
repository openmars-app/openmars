package confirmation

import (
	"context"
	"log/slog"

	confirmaction "openmars/internal/action/confirmation"
	"openmars/internal/delivery/dto"
	"openmars/internal/delivery/websocket/core"
	"openmars/internal/logger"
)

// ConfirmColonyPlacementHandler handles confirm colony placement requests
type ConfirmColonyPlacementHandler struct {
	action      *confirmaction.ConfirmColonyPlacementAction
	broadcaster Broadcaster
	logger      *slog.Logger
}

// NewConfirmColonyPlacementHandler creates a new confirm colony placement handler
func NewConfirmColonyPlacementHandler(action *confirmaction.ConfirmColonyPlacementAction, broadcaster Broadcaster) *ConfirmColonyPlacementHandler {
	return &ConfirmColonyPlacementHandler{
		action:      action,
		broadcaster: broadcaster,
		logger:      logger.Get(),
	}
}

// HandleMessage implements the MessageHandler interface
func (h *ConfirmColonyPlacementHandler) HandleMessage(ctx context.Context, connection *core.Connection, message dto.WebSocketMessage) {
	log := h.logger.With(
		slog.String("connection_id", connection.ID),
		slog.String("message_type", string(message.Type)),
	)

	log.Debug("Processing confirm colony placement request")

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

	colonyID, _ := payloadMap["colonyId"].(string)
	if colonyID == "" {
		connection.SendError(message.Type, "Missing colonyId in payload")
		return
	}

	log.Debug("Parsed confirm colony placement request",
		slog.String("colony_id", colonyID))

	err := h.action.Execute(ctx, connection.GameID(), connection.PlayerID(), colonyID)
	if err != nil {
		log.Error("Failed to execute confirm colony placement action", slog.Any("error", err))
		connection.SendError(message.Type, err.Error())
		return
	}

	log.Debug("Colony placement confirmed")

	h.broadcaster.BroadcastGameState(connection.GameID(), nil)

}
