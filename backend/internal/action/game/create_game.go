package game

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"openmars/internal/game"
	"openmars/internal/game/board"
	"openmars/internal/game/cards"
	"openmars/internal/game/mode"
	"openmars/internal/game/shared"
)

// CreateGameAction handles the business logic for creating new games
type CreateGameAction struct {
	gameRepo     game.GameRepository
	cardRegistry cards.CardRegistry
	mapRegistry  *board.MapRegistry
	logger       *slog.Logger
}

// NewCreateGameAction creates a new create game action
func NewCreateGameAction(
	gameRepo game.GameRepository,
	cardRegistry cards.CardRegistry,
	mapRegistry *board.MapRegistry,
	logger *slog.Logger,
) *CreateGameAction {
	return &CreateGameAction{
		gameRepo:     gameRepo,
		cardRegistry: cardRegistry,
		mapRegistry:  mapRegistry,
		logger:       logger,
	}
}

// Execute performs the create game action
func (a *CreateGameAction) Execute(
	ctx context.Context,
	settings shared.GameSettings,
) (*game.Game, error) {
	log := a.logger.With(
		slog.Int("max_players", settings.MaxPlayers),
		slog.String("game_mode", settings.GameModeID),
	)
	log.Debug("Creating new game")

	gameID := uuid.New().String()
	newGame, err := a.Build(gameID, settings)
	if err != nil {
		return nil, err
	}

	if err := a.gameRepo.Create(ctx, newGame); err != nil {
		log.Error("Failed to create game", slog.Any("error", err))
		return nil, err
	}

	// Log the master RNG seed so a reported game can be reproduced deterministically
	// (seed + action sequence) via the replay harness. Kept out of the client DTO on
	// purpose: exposing it would let players predict the deck shuffle.
	log.Info("Game created", slog.String("game_id", gameID), slog.Uint64("seed", newGame.Seed()))
	return newGame, nil
}

// Build constructs a lobby game with its board and deck under the given ID without
// registering it in the repository. The game state is written to the data store,
// replacing any existing state for that ID.
func (a *CreateGameAction) Build(gameID string, settings shared.GameSettings) (*game.Game, error) {
	if settings.MaxPlayers == 0 {
		settings.MaxPlayers = game.DefaultMaxPlayers
	}
	if settings.MapID == "" {
		settings.MapID = board.DefaultMapID()
	}
	var definition mode.Definition
	var err error
	settings, definition, err = mode.Normalize(cards.ModeDefinitions(a.cardRegistry), settings)
	if err != nil {
		return nil, err
	}
	effective, err := cards.Resolve(a.cardRegistry, settings)
	if err != nil {
		return nil, err
	}

	mapDef, ok := a.mapRegistry.GetMap(settings.MapID)
	if !ok {
		return nil, fmt.Errorf("unknown map: %s", settings.MapID)
	}
	initialTiles := board.GenerateBoardFromMap(mapDef, settings.HasVenus())

	newGame := game.NewGame(a.gameRepo.DataStore(), gameID, "", settings, initialTiles)

	projectCardIDs, corpIDs, preludeIDs := cards.GetCardIDs(effective)
	newGame.InitDeck(projectCardIDs, corpIDs, preludeIDs)
	newGame.SetVPCardLookup(cards.NewVPCardLookupAdapter(effective))
	if err := newGame.ConfigureModeRuntime(definition.Runtime, settings.GameModeParameters); err != nil {
		return nil, err
	}
	a.logger.Debug("Deck initialized",
		slog.String("game_id", gameID),
		slog.Int("project_cards", len(projectCardIDs)),
		slog.Int("corporations", len(corpIDs)),
		slog.Int("preludes", len(preludeIDs)),
		slog.Any("first_5_corps", getFirst5(corpIDs)))

	return newGame, nil
}

// getFirst5 returns up to the first 5 elements of a slice (for logging)
func getFirst5(ids []string) []string {
	if len(ids) <= 5 {
		return ids
	}
	return ids[:5]
}

// GameOptions contains domain settings and boards available when creating a game.
type GameOptions struct {
	Defaults      shared.GameSettings
	AvailableMaps []*board.MapDefinition
	GameModes     []mode.Definition
}

// Options returns the defaults and boards used by game creation.
func (a *CreateGameAction) Options() GameOptions {
	availableMaps := make([]*board.MapDefinition, 0)
	for _, info := range a.mapRegistry.ListMaps() {
		definition, _ := a.mapRegistry.GetMap(info.ID)
		availableMaps = append(availableMaps, definition)
	}
	return GameOptions{Defaults: shared.GameSettings{
		MaxPlayers: game.DefaultMaxPlayers, MapID: board.DefaultMapID(),
		GameModeID: "classic", Features: []string{"prelude"},
		DevelopmentMode: true,
	}, AvailableMaps: availableMaps, GameModes: cards.ModeDefinitions(a.cardRegistry)}
}

// ExecuteSetup validates the complete setup before registering a game.
func (a *CreateGameAction) ExecuteSetup(ctx context.Context, setup *shared.GameSettings) (*game.Game, error) {
	settings := a.Options().Defaults
	if setup != nil {
		settings = *setup
	}
	if err := validateSetup(settings, a.mapRegistry); err != nil {
		return nil, err
	}
	return a.Execute(ctx, settings)
}

func validateSetup(settings shared.GameSettings, registry *board.MapRegistry) error {
	if settings.MaxPlayers < 1 || settings.MaxPlayers > 8 {
		return fmt.Errorf("max players must be between 1 and 8")
	}
	if _, ok := registry.GetMap(settings.MapID); !ok {
		return fmt.Errorf("unknown map: %s", settings.MapID)
	}

	return nil
}
