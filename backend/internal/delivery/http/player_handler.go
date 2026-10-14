package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"openmars/internal/action/query"
	"openmars/internal/delivery/dto"
	"openmars/internal/game/cards"
	"openmars/internal/logger"

	"github.com/gorilla/mux"
)

// PlayerHandler handles HTTP requests for player queries
type PlayerHandler struct {
	getPlayerAction *query.GetPlayerAction
	getGameAction   *query.GetGameAction
	cardRegistry    cards.CardRegistry
}

// NewPlayerHandler creates a new player handler
func NewPlayerHandler(
	getPlayerAction *query.GetPlayerAction,
	getGameAction *query.GetGameAction,
	cardRegistry cards.CardRegistry,
) *PlayerHandler {
	return &PlayerHandler{
		getPlayerAction: getPlayerAction,
		getGameAction:   getGameAction,
		cardRegistry:    cardRegistry,
	}
}

// GetPlayer handles GET /api/v1/games/{gameId}/players/{playerId}
func (h *PlayerHandler) GetPlayer(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()
	ctx := r.Context()

	// Extract parameters from URL
	vars := mux.Vars(r)
	gameID := vars["gameId"]
	playerID := vars["playerId"]

	log.Debug("HTTP GET /api/v1/games/:gameId/players/:playerId",
		slog.String("game_id", gameID),
		slog.String("player_id", playerID))

	// Execute query actions - need both player and game for DTO mapping
	player, err := h.getPlayerAction.Execute(ctx, gameID, playerID)
	if err != nil {
		log.Error("Failed to get player", slog.Any("error", err))
		http.Error(w, "Player not found", http.StatusNotFound)
		return
	}

	game, err := h.getGameAction.Execute(ctx, gameID)
	if err != nil {
		log.Error("Failed to get game for player DTO mapping", slog.Any("error", err))
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	// Convert to DTO
	playerDto := dto.ToPlayerDto(player, game, h.cardRegistry, nil, nil, nil, nil)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(playerDto); err != nil {
		log.Error("Failed to encode response", slog.Any("error", err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Debug("Player retrieved",
		slog.String("game_id", gameID),
		slog.String("player_id", playerID))
}
