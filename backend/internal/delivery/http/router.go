package http

import (
	"net/http"

	gameaction "openmars/internal/action/game"
	"openmars/internal/action/query"
	"openmars/internal/delivery/dto"
	"openmars/internal/game/award"
	"openmars/internal/game/cards"
	"openmars/internal/game/milestone"
	httpmiddleware "openmars/internal/middleware/http"
	"openmars/internal/service/bugreport"

	"github.com/gorilla/mux"
)

// SetupRouter creates HTTP router
// Includes both query (GET) and mutation (POST) endpoints
func SetupRouter(
	createGameAction *gameaction.CreateGameAction,
	getGameAction *query.GetGameAction,
	getGameLogsAction *query.GetGameLogsAction,
	getGameHistoryAction *query.GetGameHistoryAction,
	listGamesAction *query.ListGamesAction,
	listCardsAction *query.ListCardsAction,
	getPlayerAction *query.GetPlayerAction,
	cardRegistry cards.CardRegistry,
	milestoneRegistry milestone.MilestoneRegistry,
	awardRegistry award.AwardRegistry,
	bugReportService *bugreport.Service,
	meta dto.MetaResponse,
	changelogDir string,
	changelog dto.ChangelogResponse,
) *mux.Router {
	gameHandler := NewGameHandler(createGameAction, getGameAction, getGameLogsAction, getGameHistoryAction, listGamesAction, listCardsAction, cardRegistry, milestoneRegistry, awardRegistry)
	playerHandler := NewPlayerHandler(getPlayerAction, getGameAction, cardRegistry)
	healthHandler := NewHealthHandler()
	metaHandler := NewMetaHandler(meta)
	changelogHandler := NewChangelogHandler(changelogDir, changelog)
	bugReportHandler := NewBugReportHandler(bugReportService)

	router := mux.NewRouter()
	router.Use(httpmiddleware.Recovery)
	router.Use(httpmiddleware.CORS)
	router.Use(httpmiddleware.LoggingMiddleware)
	router.Methods(http.MethodOptions).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	api := router.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/game-options", gameHandler.GameOptions).Methods(http.MethodGet)
	api.HandleFunc("/game-modes", gameHandler.GameModes).Methods(http.MethodGet)
	api.HandleFunc("/health", healthHandler.HealthCheck).Methods(http.MethodGet)
	api.HandleFunc("/meta", metaHandler.Meta).Methods(http.MethodGet)
	api.HandleFunc("/changelog", changelogHandler.Changelog).Methods(http.MethodGet)
	api.HandleFunc("/changelog/{version}/{file}", changelogHandler.Image).Methods(http.MethodGet)

	gameRoutes := api.PathPrefix("/games").Subrouter()
	gameRoutes.HandleFunc("", gameHandler.CreateGame).Methods(http.MethodPost)
	gameRoutes.HandleFunc("", gameHandler.ListGames).Methods(http.MethodGet)
	gameRoutes.HandleFunc("/{gameId}", gameHandler.GetGame).Methods(http.MethodGet)
	gameRoutes.HandleFunc("/{gameId}/logs", gameHandler.GetGameLogs).Methods(http.MethodGet)
	gameRoutes.HandleFunc("/{gameId}/history", gameHandler.GetGameHistory).Methods(http.MethodGet)

	playerRoutes := api.PathPrefix("/games/{gameId}/players").Subrouter()
	playerRoutes.HandleFunc("/{playerId}", playerHandler.GetPlayer).Methods(http.MethodGet)

	api.HandleFunc("/cards", gameHandler.ListCards).Methods(http.MethodGet)
	api.HandleFunc("/milestones-awards", gameHandler.ListMilestonesAndAwards).Methods(http.MethodGet)

	api.HandleFunc("/bugs", bugReportHandler.SubmitBugReport).Methods(http.MethodPost)
	api.HandleFunc("/bugs/status", bugReportHandler.GetStatus).Methods(http.MethodGet)
	api.HandleFunc("/bugs/{id}", bugReportHandler.GetReport).Methods(http.MethodGet)

	return router
}
