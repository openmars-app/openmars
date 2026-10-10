package game

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"

	"openmars/internal/game"
	"openmars/internal/game/board"
	"openmars/internal/game/cards"
	"openmars/internal/game/mode"
	"openmars/internal/game/shared"
)

// SettingsPatch is the domain-level, transport-agnostic patch of editable lobby
// settings. The delivery layer maps its request DTO onto this so the action does not
// depend on delivery/dto.
type SettingsPatch struct {
	MaxPlayers         *int
	MapID              *string
	DevelopmentMode    *bool
	DemoGame           *bool
	AllowRandomBuy     *bool
	GameModeID         *string
	GameModeParameters *map[string]int
	Features           *[]string
	ClaudeOAuthToken   *string
	BotSpendCapUSD     *float64
}

// UpdateGameSettingsAction handles editing game settings during the lobby phase.
type UpdateGameSettingsAction struct {
	gameRepo     game.GameRepository
	cardRegistry cards.CardRegistry
	mapRegistry  *board.MapRegistry
	logger       *slog.Logger
}

// NewUpdateGameSettingsAction creates a new update game settings action.
func NewUpdateGameSettingsAction(
	gameRepo game.GameRepository,
	cardRegistry cards.CardRegistry,
	mapRegistry *board.MapRegistry,
	logger *slog.Logger,
) *UpdateGameSettingsAction {
	return &UpdateGameSettingsAction{
		gameRepo:     gameRepo,
		cardRegistry: cardRegistry,
		mapRegistry:  mapRegistry,
		logger:       logger,
	}
}

// Execute applies the patch to the game's settings. Host-only, lobby-only.
func (a *UpdateGameSettingsAction) Execute(
	ctx context.Context,
	gameID string,
	playerID string,
	patch *SettingsPatch,
) error {
	log := a.logger.With(
		slog.String("game_id", gameID),
		slog.String("player_id", playerID),
	)

	g, err := a.gameRepo.Get(ctx, gameID)
	if err != nil {
		return fmt.Errorf("game not found: %w", err)
	}

	if g.Status() != shared.GameStatusLobby {
		return fmt.Errorf("settings can only be changed during lobby phase")
	}

	if g.HostPlayerID() != playerID {
		return fmt.Errorf("only the host can change settings")
	}

	settings := g.Settings()
	selectionChanged := false
	mapChanged := false
	demoTurnedOff := false

	if patch.MaxPlayers != nil {
		newMax := *patch.MaxPlayers
		if newMax < 1 || newMax > 8 {
			return fmt.Errorf("max players must be between 1 and 8")
		}
		joined := len(g.GetAllPlayers())
		if newMax < joined {
			return fmt.Errorf("cannot reduce max players below current player count (%d)", joined)
		}
		settings.MaxPlayers = newMax
	}

	if patch.MapID != nil {
		if _, ok := a.mapRegistry.GetMap(*patch.MapID); !ok {
			return fmt.Errorf("unknown map: %s", *patch.MapID)
		}
		if settings.MapID != *patch.MapID {
			settings.MapID = *patch.MapID
			mapChanged = true
		}
	}

	if patch.GameModeID != nil && settings.GameModeID != *patch.GameModeID {
		settings.GameModeID = *patch.GameModeID
		settings.GameModeParameters = nil
		selectionChanged = true
	}
	if patch.GameModeParameters != nil && !reflect.DeepEqual(settings.GameModeParameters, *patch.GameModeParameters) {
		settings.GameModeParameters = *patch.GameModeParameters
		selectionChanged = true
	}
	if patch.Features != nil && !reflect.DeepEqual(settings.Features, *patch.Features) {
		settings.Features = *patch.Features
		selectionChanged = true
	}

	if patch.DevelopmentMode != nil {
		settings.DevelopmentMode = *patch.DevelopmentMode
	}

	if patch.DemoGame != nil {
		if settings.DemoGame && !*patch.DemoGame {
			demoTurnedOff = true
		}
		settings.DemoGame = *patch.DemoGame
	}

	if patch.AllowRandomBuy != nil {
		settings.AllowRandomBuy = *patch.AllowRandomBuy
	}

	if patch.ClaudeOAuthToken != nil {
		settings.ClaudeOAuthToken = *patch.ClaudeOAuthToken
	}
	if patch.BotSpendCapUSD != nil {
		if *patch.BotSpendCapUSD <= 0 {
			return fmt.Errorf("bot spend cap must be positive")
		}
		settings.BotSpendCapUSD = *patch.BotSpendCapUSD
	}

	if demoTurnedOff {
		settings.Temperature = nil
		settings.Oxygen = nil
		settings.Oceans = nil
		settings.Venus = nil
		settings.Generation = nil
	}

	settings, definition, err := mode.Normalize(cards.ModeDefinitions(a.cardRegistry), settings)
	if err != nil {
		return err
	}
	if err := validateSetup(settings, a.mapRegistry); err != nil {
		return err
	}
	if settings.MaxPlayers < len(g.GetAllPlayers()) {
		return fmt.Errorf("cannot reduce max players below current player count (%d)", len(g.GetAllPlayers()))
	}
	effective, err := cards.Resolve(a.cardRegistry, settings)
	if err != nil {
		return err
	}
	g.UpdateSettings(ctx, settings)
	if demoTurnedOff {
		for _, p := range g.GetAllPlayers() {
			p.SetPendingDemoChoices(nil)
		}
	}
	if selectionChanged {
		projects, corps, preludes := cards.GetCardIDs(effective)
		g.InitDeck(projects, corps, preludes)
		g.SetVPCardLookup(cards.NewVPCardLookupAdapter(effective))
		if err := g.ConfigureModeRuntime(definition.Runtime, settings.GameModeParameters); err != nil {
			return err
		}
		for _, p := range g.GetAllPlayers() {
			p.SetPendingDemoChoices(nil)
		}
	}
	if mapChanged || selectionChanged {
		mapDef, _ := a.mapRegistry.GetMap(settings.MapID)
		g.ReplaceBoard(ctx, board.GenerateBoardFromMap(mapDef, settings.HasVenus()))
	}

	log.Info("Game settings updated")
	return nil
}
