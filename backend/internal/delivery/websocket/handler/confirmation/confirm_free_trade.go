package confirmation

import (
	"context"
	"encoding/json"
	"log/slog"

	confirmaction "openmars/internal/action/confirmation"
	"openmars/internal/delivery/dto"
	"openmars/internal/delivery/websocket/core"
	"openmars/internal/logger"
)

// ConfirmFreeTradeHandler handles confirm free trade requests
type ConfirmFreeTradeHandler struct {
	action      *confirmaction.ConfirmFreeTradeAction
	broadcaster Broadcaster
	logger      *slog.Logger
}

// NewConfirmFreeTradeHandler creates a new confirm free trade handler
func NewConfirmFreeTradeHandler(action *confirmaction.ConfirmFreeTradeAction, broadcaster Broadcaster) *ConfirmFreeTradeHandler {
	return &ConfirmFreeTradeHandler{
		action:      action,
		broadcaster: broadcaster,
		logger:      logger.Get(),
	}
}

// HandleMessage implements the MessageHandler interface
func (h *ConfirmFreeTradeHandler) HandleMessage(ctx context.Context, connection *core.Connection, message dto.WebSocketMessage) {
	log := h.logger.With(
		slog.String("connection_id", connection.ID),
		slog.String("message_type", string(message.Type)),
	)

	log.Debug("Processing confirm free trade request")

	if connection.GameID() == "" || connection.PlayerID() == "" {
		connection.SendError(message.Type, "Not connected to a game")
		return
	}

	payloadBytes, err := json.Marshal(message.Payload)
	if err != nil {
		connection.SendError(message.Type, "Invalid payload format")
		return
	}
	var payload dto.FreeTradeRequest
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		connection.SendError(message.Type, "Invalid payload format")
		return
	}
	if payload.Skip {
		if payload.ColonyID != "" || payload.TrackSteps != nil {
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
	colonyID := payload.ColonyID
	if colonyID == "" {
		connection.SendError(message.Type, "Missing colonyId in payload")
		return
	}
	if payload.TrackSteps == nil {
		connection.SendError(message.Type, "trackSteps is required")
		return
	}

	err = h.action.Execute(ctx, connection.GameID(), connection.PlayerID(), colonyID, *payload.TrackSteps)
	if err != nil {
		log.Error("Failed to execute confirm free trade action", slog.Any("error", err))
		connection.SendError(message.Type, err.Error())
		return
	}

	log.Debug("Free trade confirmed")

	h.broadcaster.BroadcastGameState(connection.GameID(), nil)

}
