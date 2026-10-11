package game_lifecycle_test

import (
	"context"
	"openmars/internal/delivery/dto"
	"slices"
	"testing"

	gameAction "openmars/internal/action/game"
	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func TestCreateGameAction_Success(t *testing.T) {
	// Setup
	repo := testutil.NewTestGameRepository(t)
	cardRegistry := testutil.CreateTestModeCatalog()
	logger := testutil.TestLogger()

	createAction := gameAction.NewCreateGameAction(repo, cardRegistry, testutil.CreateTestMapRegistry(), logger)

	// Execute
	settings := shared.GameSettings{
		MaxPlayers: 4,
		GameModeID: "classic", Features: []string{},
	}

	createdGame, err := createAction.Execute(context.Background(), settings)

	// Assert
	testutil.AssertNoError(t, err, "Failed to create game")
	testutil.AssertNotEqual(t, "", createdGame.ID(), "Game ID should not be empty")
	testutil.AssertEqual(t, shared.GameStatusLobby, createdGame.Status(), "Game should start in lobby status")
	testutil.AssertEqual(t, 4, createdGame.Settings().MaxPlayers, "Max players should be 4")

	// Verify game exists in repository
	fetchedGame, err := repo.Get(context.Background(), createdGame.ID())
	testutil.AssertNoError(t, err, "Failed to fetch created game")
	testutil.AssertEqual(t, createdGame.ID(), fetchedGame.ID(), "Fetched game ID should match")
}

func TestCreateGameAction_DefaultSettings(t *testing.T) {
	// Setup
	repo := testutil.NewTestGameRepository(t)
	cardRegistry := testutil.CreateTestModeCatalog()
	logger := testutil.TestLogger()

	createAction := gameAction.NewCreateGameAction(repo, cardRegistry, testutil.CreateTestMapRegistry(), logger)

	// Execute with empty settings
	settings := shared.GameSettings{}
	createdGame, err := createAction.Execute(context.Background(), settings)

	// Assert defaults are applied
	testutil.AssertNoError(t, err, "Failed to create game with empty settings")
	testutil.AssertEqual(t, game.DefaultMaxPlayers, createdGame.Settings().MaxPlayers, "Should use default max players")
	testutil.AssertTrue(t, createdGame.Settings().GameModeID == "classic", "Should select Classic by default")
}

func TestCreateGameAction_DeckInitialization(t *testing.T) {
	// Setup
	repo := testutil.NewTestGameRepository(t)
	cardRegistry := testutil.CreateTestModeCatalog()
	logger := testutil.TestLogger()

	createAction := gameAction.NewCreateGameAction(repo, cardRegistry, testutil.CreateTestMapRegistry(), logger)

	// Execute
	settings := shared.GameSettings{
		MaxPlayers: 4,
		GameModeID: "classic", Features: []string{},
	}

	createdGame, err := createAction.Execute(context.Background(), settings)

	// Assert
	testutil.AssertNoError(t, err, "Failed to create game")

	deck := createdGame.Deck()
	testutil.AssertTrue(t, deck != nil, "Deck should be initialized")

	// Verify deck has cards from Classic
	// (The deck should contain project cards from the registry)
}

func TestCreateGameAction_PreludeFeature(t *testing.T) {
	// Setup
	repo := testutil.NewTestGameRepository(t)
	cardRegistry := testutil.CreateTestModeCatalog()
	logger := testutil.TestLogger()

	createAction := gameAction.NewCreateGameAction(repo, cardRegistry, testutil.CreateTestMapRegistry(), logger)

	// Execute with the Prelude feature
	settings := shared.GameSettings{
		MaxPlayers: 4,
		GameModeID: "classic", Features: []string{"prelude"},
	}

	createdGame, err := createAction.Execute(context.Background(), settings)

	// Assert
	testutil.AssertNoError(t, err, "Failed to create game with Preludes")

	effective, err := gamecards.Resolve(cardRegistry, createdGame.Settings())
	testutil.AssertNoError(t, err, "resolve selected mode")
	projects, corps, preludes := gamecards.GetCardIDs(effective)
	testutil.AssertEqual(t, len(projects), len(createdGame.Deck().ProjectCards()), "every selected project card is in the deck")
	testutil.AssertEqual(t, len(corps), len(createdGame.Deck().Corporations()), "every selected corporation is in the deck")
	testutil.AssertEqual(t, len(preludes), len(createdGame.Deck().PreludeCards()), "every prelude is in the deck")
	testutil.AssertTrue(t, len(preludes) > 0, "the enabled Prelude feature supplies preludes")
}

func TestCreateGameAction_BoardInitialization(t *testing.T) {
	// Setup
	repo := testutil.NewTestGameRepository(t)
	cardRegistry := testutil.CreateTestModeCatalog()
	logger := testutil.TestLogger()

	createAction := gameAction.NewCreateGameAction(repo, cardRegistry, testutil.CreateTestMapRegistry(), logger)

	// Execute
	settings := shared.GameSettings{
		MaxPlayers: 2,
		GameModeID: "classic", Features: []string{},
	}

	createdGame, err := createAction.Execute(context.Background(), settings)

	// Assert
	testutil.AssertNoError(t, err, "Failed to create game")

	board := createdGame.Board()
	testutil.AssertTrue(t, board != nil, "Board should be initialized")
}

func TestCreateGameRequestOptions(t *testing.T) {
	repo := testutil.NewTestGameRepository(t)
	registry := testutil.CreateTestModeCatalog()
	action := gameAction.NewCreateGameAction(repo, registry, testutil.CreateTestMapRegistry(), testutil.TestLogger())
	options := dto.MapGameOptions(action.Options())
	testutil.AssertTrue(t, options.Defaults.DevelopmentMode, "new games default to development mode")
	if len(options.AvailableMaps) == 0 || len(options.AvailableMaps[0].Tiles) == 0 {
		t.Fatal("setup must include map previews")
	}
	created, err := action.ExecuteSetup(context.Background(), (*dto.GameSetupDto)(nil).ToSettings())
	testutil.AssertNoError(t, err, "create with defaults")
	testutil.AssertEqual(t, options.Defaults.MapID, created.Settings().MapID, "map defaults match")
	testutil.AssertEqual(t, options.Defaults.MaxPlayers, created.Settings().MaxPlayers, "player defaults match")
	testutil.AssertEqual(t, options.Defaults.DevelopmentMode, created.Settings().DevelopmentMode, "development default matches")
	if !slices.Equal(options.Defaults.Features, created.Settings().Features) {
		t.Fatal("feature defaults differ")
	}

	setup := options.Defaults
	setup.MapID = options.AvailableMaps[len(options.AvailableMaps)-1].ID
	setup.MaxPlayers = 3
	setup.Features = []string{}
	setup.GameModeID = "classic"
	setup.DevelopmentMode = false
	customized, err := action.ExecuteSetup(context.Background(), setup.ToSettings())
	testutil.AssertNoError(t, err, "create configured game")
	testutil.AssertEqual(t, setup.MapID, customized.Settings().MapID, "chosen map retained")
	testutil.AssertEqual(t, 3, customized.Settings().MaxPlayers, "chosen player count retained")
	testutil.AssertFalse(t, customized.Settings().DevelopmentMode, "explicit false retained")
	testutil.AssertFalse(t, customized.Settings().HasVenus(), "classic excludes Venus")
	testutil.AssertFalse(t, customized.Settings().HasPrelude(), "prelude can be omitted")
}

func TestCreateGameRequestRejectsInvalidSetup(t *testing.T) {
	tests := []struct {
		name   string
		change func(*dto.GameSetupDto)
	}{
		{"map", func(s *dto.GameSetupDto) { s.MapID = "unknown" }},
		{"zero players", func(s *dto.GameSetupDto) { s.MaxPlayers = 0 }},
		{"too many players", func(s *dto.GameSetupDto) { s.MaxPlayers = 11 }},
		{"unknown mode", func(s *dto.GameSetupDto) { s.GameModeID = "unknown" }},
		{"unknown feature", func(s *dto.GameSetupDto) { s.Features = []string{"unknown"} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := testutil.NewTestGameRepository(t)
			action := gameAction.NewCreateGameAction(repo, testutil.CreateTestModeCatalog(), testutil.CreateTestMapRegistry(), testutil.TestLogger())
			setup := dto.MapGameOptions(action.Options()).Defaults
			tt.change(&setup)
			created, err := action.ExecuteSetup(context.Background(), setup.ToSettings())
			if err == nil || created != nil {
				t.Fatal("invalid setup created a game")
			}
			games, err := repo.List(context.Background(), nil)
			testutil.AssertNoError(t, err, "list games")
			if len(games) != 0 {
				t.Fatal("invalid setup registered a game")
			}
		})
	}
}
