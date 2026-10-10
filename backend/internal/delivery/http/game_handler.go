package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	gameaction "openmars/internal/action/game"
	"openmars/internal/action/query"
	"openmars/internal/delivery/dto"
	"openmars/internal/game/award"
	"openmars/internal/game/cards"
	"openmars/internal/game/milestone"
	"openmars/internal/game/mode"
	"openmars/internal/game/shared"
	"openmars/internal/logger"

	"github.com/gorilla/mux"
)

// GameHandler handles HTTP requests for games
type GameHandler struct {
	createGameAction     *gameaction.CreateGameAction
	getGameAction        *query.GetGameAction
	getGameLogsAction    *query.GetGameLogsAction
	getGameHistoryAction *query.GetGameHistoryAction
	listGamesAction      *query.ListGamesAction
	listCardsAction      *query.ListCardsAction
	cardRegistry         cards.CardRegistry
	milestoneRegistry    milestone.MilestoneRegistry
	awardRegistry        award.AwardRegistry
}

// NewGameHandler creates a new game handler
func NewGameHandler(
	createGameAction *gameaction.CreateGameAction,
	getGameAction *query.GetGameAction,
	getGameLogsAction *query.GetGameLogsAction,
	getGameHistoryAction *query.GetGameHistoryAction,
	listGamesAction *query.ListGamesAction,
	listCardsAction *query.ListCardsAction,
	cardRegistry cards.CardRegistry,
	milestoneRegistry milestone.MilestoneRegistry,
	awardRegistry award.AwardRegistry,
) *GameHandler {
	return &GameHandler{
		createGameAction:     createGameAction,
		getGameAction:        getGameAction,
		getGameLogsAction:    getGameLogsAction,
		getGameHistoryAction: getGameHistoryAction,
		listGamesAction:      listGamesAction,
		listCardsAction:      listCardsAction,
		cardRegistry:         cardRegistry,
		milestoneRegistry:    milestoneRegistry,
		awardRegistry:        awardRegistry,
	}
}

// GetGame handles GET /api/v1/games/{gameId}
func (h *GameHandler) GetGame(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()
	ctx := r.Context()

	vars := mux.Vars(r)
	gameID := vars["gameId"]
	playerID := r.URL.Query().Get("playerId")

	log.Debug("HTTP GET /api/v1/games/:gameId", slog.String("game_id", gameID))

	game, err := h.getGameAction.Execute(ctx, gameID)
	if err != nil {
		log.Debug("Failed to get game", slog.Any("error", err))
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	// If playerId provided, verify player is in the game
	if playerID != "" {
		if _, err := game.GetPlayer(playerID); err != nil {
			log.Debug("Player not in game", slog.String("player_id", playerID))
			http.Error(w, "Player not in game", http.StatusNotFound)
			return
		}
	}

	gameDto := dto.ToGameDto(game, h.cardRegistry, playerID)

	response := dto.GetGameResponse{
		Game: gameDto,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error("Failed to encode response", slog.Any("error", err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Debug("Game retrieved", slog.String("game_id", gameID))
}

// ListGames handles GET /api/v1/games
func (h *GameHandler) ListGames(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()
	ctx := r.Context()

	log.Debug("HTTP GET /api/v1/games")

	var statusFilter *shared.GameStatus
	if statusParam := r.URL.Query().Get("status"); statusParam != "" {
		status := shared.GameStatus(statusParam)
		statusFilter = &status
	}

	games, err := h.listGamesAction.Execute(ctx, statusFilter)
	if err != nil {
		log.Error("Failed to list games", slog.Any("error", err))
		http.Error(w, "Failed to list games", http.StatusInternalServerError)
		return
	}

	gameDtos := make([]dto.GameDto, len(games))
	for i, game := range games {
		gameDtos[i] = dto.ToGameDto(game, h.cardRegistry, "")
	}

	response := dto.ListGamesResponse{Games: gameDtos}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error("Failed to encode response", slog.Any("error", err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Debug("Games listed", slog.Int("count", len(games)))
}

// CreateGame handles POST /api/v1/games
func (h *GameHandler) CreateGame(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()
	ctx := r.Context()

	log.Debug("HTTP POST /api/v1/games")

	var req dto.CreateGameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error("Failed to decode request", slog.Any("error", err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	game, err := h.createGameAction.ExecuteSetup(ctx, req.Settings.ToSettings())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(dto.ErrorResponse{Error: err.Error()})
		return
	}

	gameDto := dto.ToGameDto(game, h.cardRegistry, "")

	response := dto.CreateGameResponse{
		Game: gameDto,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error("Failed to encode response", slog.Any("error", err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Debug("Game created", slog.String("game_id", game.ID()))
}

// ListCards handles GET /api/v1/cards
func (h *GameHandler) ListCards(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()
	ctx := r.Context()

	log.Debug("HTTP GET /api/v1/cards")

	queryParams := r.URL.Query()
	offset := 0
	limit := 100 // Default limit

	if offsetParam := queryParams.Get("offset"); offsetParam != "" {
		var parsedOffset int
		if _, err := fmt.Sscanf(offsetParam, "%d", &parsedOffset); err == nil {
			offset = parsedOffset
		}
	}

	if limitParam := queryParams.Get("limit"); limitParam != "" {
		var parsedLimit int
		if _, err := fmt.Sscanf(limitParam, "%d", &parsedLimit); err == nil {
			limit = parsedLimit
		}
	}

	preview := queryParams.Get("preview")
	if preview != "" && preview != "connected" && preview != "disconnected" && preview != "restored" {
		http.Error(w, "unknown card preview", http.StatusBadRequest)
		return
	}
	if preview != "" && queryParams.Get("gameModeId") == "" {
		http.Error(w, "card preview requires gameModeId", http.StatusBadRequest)
		return
	}
	// Execute query action
	var selection *shared.GameSettings
	if modeID := queryParams.Get("gameModeId"); modeID != "" {
		selection = &shared.GameSettings{GameModeID: modeID}
	}
	result, err := h.listCardsAction.Execute(ctx, offset, limit, selection)
	if err != nil {
		log.Error("Failed to list cards", slog.Any("error", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	previewState := map[string]int{}
	if selection != nil {
		_, definition, err := mode.Normalize(cards.ModeDefinitions(h.cardRegistry), *selection)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for k, v := range definition.Runtime.State {
			previewState[k] = v
		}
		for _, d := range definition.Runtime.Deadlines {
			previewState[d.ActiveState] = d.ActiveValue
			if preview == "disconnected" {
				previewState[d.ActiveState] = d.InactiveValue
			}
			if preview == "disconnected" || preview == "restored" {
				previewState[d.OccurredState] = 1
			}
		}
	}
	// Convert to DTOs
	cardDtos := make([]dto.CardDto, len(result.Cards))
	for i, card := range result.Cards {
		cardDtos[i] = dto.ToCardDto(card)
		if selection != nil {
			dto.AnnotateCardDependencyPreview(&cardDtos[i], card, previewState)
		}
	}

	response := dto.ListCardsResponse{
		Cards:      cardDtos,
		TotalCount: result.TotalCount,
		Offset:     result.Offset,
		Limit:      result.Limit,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error("Failed to encode response", slog.Any("error", err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Debug("Cards listed", slog.Int("count", len(cardDtos)))
}

// GetGameLogs handles GET /api/v1/games/{gameId}/logs
func (h *GameHandler) GetGameLogs(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()
	ctx := r.Context()

	vars := mux.Vars(r)
	gameID := vars["gameId"]

	queryParams := r.URL.Query()
	var since int64 = 0
	if sinceParam := queryParams.Get("since"); sinceParam != "" {
		var parsedSince int64
		if _, err := fmt.Sscanf(sinceParam, "%d", &parsedSince); err == nil {
			since = parsedSince
		}
	}

	log.Debug("HTTP GET /api/v1/games/:gameId/logs", slog.String("game_id", gameID), slog.Int64("since", since))

	diffs, err := h.getGameLogsAction.Execute(ctx, gameID, since)
	if err != nil {
		log.Error("Failed to get game logs", slog.Any("error", err))
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	diffsDto := dto.ToStateDiffDtos(diffs)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(diffsDto); err != nil {
		log.Error("Failed to encode response", slog.Any("error", err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Debug("Game logs retrieved", slog.String("game_id", gameID), slog.Int("count", len(diffs)))
}

// GetGameHistory handles GET /api/v1/games/{gameId}/history
func (h *GameHandler) GetGameHistory(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()
	ctx := r.Context()

	vars := mux.Vars(r)
	gameID := vars["gameId"]

	log.Debug("HTTP GET /api/v1/games/:gameId/history", slog.String("game_id", gameID))

	var filter *query.HistoryFilter
	queryParams := r.URL.Query()
	phasesParam := queryParams.Get("phases")
	policyParam := queryParams.Get("policy")

	if phasesParam != "" || policyParam != "" {
		filter = &query.HistoryFilter{}
		if phasesParam != "" {
			for _, p := range strings.Split(phasesParam, ",") {
				phase := shared.GamePhase(strings.TrimSpace(p))
				filter.Phases = append(filter.Phases, phase)
			}
		}
		if policyParam != "" {
			filter.Policy = query.HistoryPolicy(policyParam)
		}
	}

	entries, err := h.getGameHistoryAction.Execute(ctx, gameID, filter)
	switch {
	case errors.Is(err, query.ErrGameNotFound):
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	case errors.Is(err, query.ErrInvalidFilter):
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	case err != nil:
		log.Error("Failed to get game history", slog.Any("error", err))
		http.Error(w, "Failed to get game history", http.StatusInternalServerError)
		return
	}

	response := dto.GetGameHistoryResponse{
		Entries: dto.ToGameHistoryEntryDtos(entries, h.cardRegistry),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error("Failed to encode response", slog.Any("error", err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Debug("Game history retrieved", slog.String("game_id", gameID), slog.Int("count", len(entries)))
}

// ListMilestonesAndAwards handles GET /api/v1/milestones-awards
func (h *GameHandler) ListMilestonesAndAwards(w http.ResponseWriter, r *http.Request) {
	var milestoneDtos []dto.MilestoneAwardItemDto
	if h.milestoneRegistry != nil {
		for _, def := range h.milestoneRegistry.GetAll() {
			milestoneDtos = append(milestoneDtos, dto.MilestoneAwardItemDto{
				ID:          def.ID,
				Name:        def.Name,
				Description: def.Description,
			})
		}
	}

	var awardDtos []dto.MilestoneAwardItemDto
	if h.awardRegistry != nil {
		for _, def := range h.awardRegistry.GetAll() {
			awardDtos = append(awardDtos, dto.MilestoneAwardItemDto{
				ID:          def.ID,
				Name:        def.Name,
				Description: def.Description,
			})
		}
	}

	response := dto.ListMilestonesAwardsResponse{
		Milestones: milestoneDtos,
		Awards:     awardDtos,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// GameOptions handles GET /api/v1/game-options.
func (h *GameHandler) GameOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(dto.MapGameOptions(h.createGameAction.Options())); err != nil {
		logger.Get().Error("Failed to encode game options", slog.Any("error", err))
	}
}

// GameModes exposes the data-driven mode catalog.
func (h *GameHandler) GameModes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(dto.GameModesResponse{GameModes: dto.MapGameModes(cards.ModeDefinitions(h.cardRegistry))}); err != nil {
		logger.Get().Error("Failed to encode game modes", slog.Any("error", err))
	}
}
