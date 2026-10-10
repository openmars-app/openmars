package integration_test

import (
	"context"
	"testing"

	gameAction "openmars/internal/action/game"
	turnAction "openmars/internal/action/turn_management"
	"openmars/internal/game/shared"
	"openmars/test/testutil"

	"github.com/google/uuid"
)

// TestGameLifecycle_CreateJoinStartPlay tests the complete game lifecycle
func TestGameLifecycle_CreateJoinStartPlay(t *testing.T) {
	// Setup
	repo := testutil.NewTestGameRepository(t)
	cardRegistry := testutil.CreateTestModeCatalog()
	logger := testutil.TestLogger()
	ctx := context.Background()

	// Create actions
	createAction := gameAction.NewCreateGameAction(repo, cardRegistry, testutil.CreateTestMapRegistry(), logger)
	joinAction := gameAction.NewJoinGameAction(repo, cardRegistry, logger)
	startAction := turnAction.NewStartGameAction(repo, nil, nil, nil, nil, nil, logger, nil)

	// Step 1: Create game
	settings := shared.GameSettings{
		MaxPlayers: 4,
		GameModeID: "classic", Features: []string{},
	}

	createdGame, err := createAction.Execute(ctx, settings)
	testutil.AssertNoError(t, err, "Failed to create game")
	gameID := createdGame.ID()

	// Verify game is in lobby
	testutil.AssertEqual(t, shared.GameStatusLobby, createdGame.Status(), "Game should be in lobby")

	// Step 2: Players join
	player1ID := uuid.New().String()
	result1, err := joinAction.Execute(ctx, gameID, "Alice", player1ID)
	testutil.AssertNoError(t, err, "Player 1 failed to join")

	player2ID := uuid.New().String()
	result2, err := joinAction.Execute(ctx, gameID, "Bob", player2ID)
	testutil.AssertNoError(t, err, "Player 2 failed to join")

	// Verify players joined
	testutil.AssertEqual(t, player1ID, result1.PlayerID, "Player 1 ID mismatch")
	testutil.AssertEqual(t, player2ID, result2.PlayerID, "Player 2 ID mismatch")

	// Get game and verify player count
	gameAfterJoin, _ := repo.Get(ctx, gameID)
	players := gameAfterJoin.GetAllPlayers()
	testutil.AssertEqual(t, 2, len(players), "Should have 2 players")

	// Step 3: Select corporations
	for _, p := range players {
		p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	}

	// Step 4: Start game
	hostID := gameAfterJoin.HostPlayerID()
	err = startAction.Execute(ctx, gameID, hostID)
	testutil.AssertNoError(t, err, "Failed to start game")

	// Verify game started
	gameAfterStart, _ := repo.Get(ctx, gameID)
	testutil.AssertEqual(t, shared.GameStatusActive, gameAfterStart.Status(), "Game should be active")
	testutil.AssertTrue(t, gameAfterStart.CurrentPhase() != "", "Game phase should be set")
	testutil.AssertTrue(t, gameAfterStart.Generation() >= 1, "Game generation should be set")

	// Verify current turn is set
	currentTurn := gameAfterStart.CurrentTurn()
	testutil.AssertTrue(t, currentTurn != nil, "Current turn should be set")
	testutil.AssertTrue(t, currentTurn.PlayerID() != "", "Current turn should have player ID")

	// Verify global parameters initialized
	globalParams := gameAfterStart.GlobalParameters()
	testutil.AssertTrue(t, globalParams != nil, "Global parameters should be initialized")
	testutil.AssertTrue(t, globalParams.Temperature() < 8, "Temperature should be below max")
	testutil.AssertTrue(t, globalParams.Oxygen() >= 0, "Oxygen should be initialized")
}

// TestGameLifecycle_MultipleGames tests multiple games running concurrently
func TestGameLifecycle_MultipleGames(t *testing.T) {
	// Setup
	repo := testutil.NewTestGameRepository(t)
	cardRegistry := testutil.CreateTestModeCatalog()
	logger := testutil.TestLogger()
	ctx := context.Background()

	createAction := gameAction.NewCreateGameAction(repo, cardRegistry, testutil.CreateTestMapRegistry(), logger)
	joinAction := gameAction.NewJoinGameAction(repo, cardRegistry, logger)

	// Create 3 games
	game1, err := createAction.Execute(ctx, shared.GameSettings{MaxPlayers: 2, GameModeID: "classic", Features: []string{}})
	testutil.AssertNoError(t, err, "Failed to create game 1")

	game2, err := createAction.Execute(ctx, shared.GameSettings{MaxPlayers: 2, GameModeID: "classic", Features: []string{}})
	testutil.AssertNoError(t, err, "Failed to create game 2")

	game3, err := createAction.Execute(ctx, shared.GameSettings{MaxPlayers: 2, GameModeID: "classic", Features: []string{}})
	testutil.AssertNoError(t, err, "Failed to create game 3")

	// Add players to each game
	_, err = joinAction.Execute(ctx, game1.ID(), "Game1-Player1", uuid.New().String())
	testutil.AssertNoError(t, err, "Failed to join game 1")

	_, err = joinAction.Execute(ctx, game2.ID(), "Game2-Player1", uuid.New().String())
	testutil.AssertNoError(t, err, "Failed to join game 2")

	_, err = joinAction.Execute(ctx, game3.ID(), "Game3-Player1", uuid.New().String())
	testutil.AssertNoError(t, err, "Failed to join game 3")

	// Verify all games exist independently
	allGames, err := repo.List(ctx, nil)
	testutil.AssertNoError(t, err, "Failed to list games")
	testutil.AssertEqual(t, 3, len(allGames), "Should have 3 games")

	// Verify each game has 1 player
	for _, g := range allGames {
		players := g.GetAllPlayers()
		testutil.AssertEqual(t, 1, len(players), "Each game should have 1 player")
	}
}

// TestGameLifecycle_PlayerReconnection tests player reconnection scenario
func TestGameLifecycle_PlayerReconnection(t *testing.T) {
	// Setup
	repo := testutil.NewTestGameRepository(t)
	cardRegistry := testutil.CreateTestModeCatalog()
	logger := testutil.TestLogger()
	ctx := context.Background()

	createAction := gameAction.NewCreateGameAction(repo, cardRegistry, testutil.CreateTestMapRegistry(), logger)
	joinAction := gameAction.NewJoinGameAction(repo, cardRegistry, logger)

	// Create game
	createdGame, err := createAction.Execute(ctx, shared.GameSettings{MaxPlayers: 2, GameModeID: "classic", Features: []string{}})
	testutil.AssertNoError(t, err, "Failed to create game")

	// Player joins
	result, err := joinAction.Execute(ctx, createdGame.ID(), "Alice", uuid.New().String())
	testutil.AssertNoError(t, err, "Failed to join game")

	// Player "reconnects" (joins with same name)
	reconnectResult, err := joinAction.Execute(ctx, createdGame.ID(), "Alice", uuid.New().String())
	testutil.AssertNoError(t, err, "Failed to reconnect")

	// Should get same player ID
	testutil.AssertEqual(t, result.PlayerID, reconnectResult.PlayerID, "Reconnection should return same player ID")

	// Verify still only 1 player in game
	gameState, _ := repo.Get(ctx, createdGame.ID())
	players := gameState.GetAllPlayers()
	testutil.AssertEqual(t, 1, len(players), "Should still have only 1 player")
}

// TestGameLifecycle_SoloMode tests solo game mode
func TestGameLifecycle_SoloMode(t *testing.T) {
	// Setup
	repo := testutil.NewTestGameRepository(t)
	cardRegistry := testutil.CreateTestModeCatalog()
	logger := testutil.TestLogger()
	ctx := context.Background()

	createAction := gameAction.NewCreateGameAction(repo, cardRegistry, testutil.CreateTestMapRegistry(), logger)
	joinAction := gameAction.NewJoinGameAction(repo, cardRegistry, logger)
	startAction := turnAction.NewStartGameAction(repo, nil, nil, nil, nil, nil, logger, nil)

	// Create game
	createdGame, err := createAction.Execute(ctx, shared.GameSettings{MaxPlayers: 1, GameModeID: "classic", Features: []string{}})
	testutil.AssertNoError(t, err, "Failed to create game")

	// Single player joins
	playerID := uuid.New().String()
	_, err = joinAction.Execute(ctx, createdGame.ID(), "Solo Player", playerID)
	testutil.AssertNoError(t, err, "Failed to join solo game")

	// Set corporation
	gameState, _ := repo.Get(ctx, createdGame.ID())
	players := gameState.GetAllPlayers()
	players[0].SetCorporationID(testutil.CardID("Tharsis Republic"))

	// Start game
	err = startAction.Execute(ctx, createdGame.ID(), gameState.HostPlayerID())

	// Solo mode should work
	testutil.AssertNoError(t, err, "Solo mode should be allowed")

	gameAfterStart, _ := repo.Get(ctx, createdGame.ID())
	testutil.AssertEqual(t, shared.GameStatusActive, gameAfterStart.Status(), "Game should be active in solo mode")
}

// TestGameLifecycle_GameStateConsistency tests that game state remains consistent
func TestGameLifecycle_GameStateConsistency(t *testing.T) {
	// Setup
	repo := testutil.NewTestGameRepository(t)
	cardRegistry := testutil.CreateTestModeCatalog()
	logger := testutil.TestLogger()
	ctx := context.Background()

	createAction := gameAction.NewCreateGameAction(repo, cardRegistry, testutil.CreateTestMapRegistry(), logger)
	joinAction := gameAction.NewJoinGameAction(repo, cardRegistry, logger)

	// Create game
	createdGame, err := createAction.Execute(ctx, shared.GameSettings{MaxPlayers: 4, GameModeID: "classic", Features: []string{}})
	testutil.AssertNoError(t, err, "Failed to create game")
	gameID := createdGame.ID()

	// Multiple reads should return consistent state
	game1, _ := repo.Get(ctx, gameID)
	game2, _ := repo.Get(ctx, gameID)

	testutil.AssertEqual(t, game1.ID(), game2.ID(), "Game IDs should match")
	testutil.AssertEqual(t, game1.Status(), game2.Status(), "Game status should match")

	// Add player
	playerID := uuid.New().String()
	_, err = joinAction.Execute(ctx, gameID, "Test Player", playerID)
	testutil.AssertNoError(t, err, "Failed to join game")

	// Read again
	game3, _ := repo.Get(ctx, gameID)
	players := game3.GetAllPlayers()
	testutil.AssertEqual(t, 1, len(players), "Should have 1 player")

	// Verify player state persists
	player := players[0]
	testutil.AssertEqual(t, "Test Player", player.Name(), "Player name should persist")
	testutil.AssertEqual(t, playerID, player.ID(), "Player ID should persist")
}
