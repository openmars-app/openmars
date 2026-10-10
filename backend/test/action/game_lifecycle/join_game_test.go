package game_lifecycle_test

import (
	"context"
	"strings"
	"testing"

	gameAction "openmars/internal/action/game"
	"openmars/internal/game"
	"openmars/internal/game/board"
	"openmars/internal/game/datastore"
	"openmars/internal/game/shared"
	"openmars/test/testutil"

	"github.com/google/uuid"
)

func TestJoinGameAction_Success(t *testing.T) {
	// Setup
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 0)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()

	joinAction := gameAction.NewJoinGameAction(repo, cardRegistry, logger)

	// Execute
	playerID := uuid.New().String()
	result, err := joinAction.Execute(context.Background(), testGame.ID(), "Alice", playerID)

	// Assert
	testutil.AssertNoError(t, err, "Failed to join game")
	testutil.AssertEqual(t, playerID, result.PlayerID, "Player ID should match")

	// Verify player was added to game
	fetchedGame, _ := repo.Get(context.Background(), testGame.ID())
	players := fetchedGame.GetAllPlayers()
	testutil.AssertEqual(t, 1, len(players), "Should have 1 player")
	testutil.AssertEqual(t, "Alice", players[0].Name(), "Player name should be Alice")
}

func TestJoinGameAction_IdempotentJoin(t *testing.T) {
	// Setup
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 0)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()

	joinAction := gameAction.NewJoinGameAction(repo, cardRegistry, logger)

	// Join first time
	playerID1 := uuid.New().String()
	result1, err1 := joinAction.Execute(context.Background(), testGame.ID(), "Bob", playerID1)
	testutil.AssertNoError(t, err1, "First join failed")

	// Join second time with same name but different ID
	playerID2 := uuid.New().String()
	result2, err2 := joinAction.Execute(context.Background(), testGame.ID(), "Bob", playerID2)
	testutil.AssertNoError(t, err2, "Idempotent join failed")

	// Assert - should return the original player ID
	testutil.AssertEqual(t, result1.PlayerID, result2.PlayerID, "Should return existing player ID")

	// Verify only one player exists
	fetchedGame, _ := repo.Get(context.Background(), testGame.ID())
	players := fetchedGame.GetAllPlayers()
	testutil.AssertEqual(t, 1, len(players), "Should still have only 1 player")
}

func TestJoinGameAction_GameNotFound(t *testing.T) {
	// Setup
	repo := testutil.NewTestGameRepository(t)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()

	joinAction := gameAction.NewJoinGameAction(repo, cardRegistry, logger)

	// Execute with non-existent game ID
	playerID := uuid.New().String()
	_, err := joinAction.Execute(context.Background(), "non-existent-game", "Charlie", playerID)

	// Assert
	testutil.AssertErrorContains(t, err, "game not found", "Should fail when game doesn't exist")
}

func TestJoinGameAction_GameNotInLobby(t *testing.T) {
	// Setup
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()

	// Start the game (move it out of lobby status)
	players := testGame.GetAllPlayers()
	if len(players) >= 2 {
		// Set corporations for players
		for _, p := range players {
			p.SetCorporationID(testutil.PlaceholderCorporationID)
		}
	}

	testutil.StartTestGame(t, testGame)

	joinAction := gameAction.NewJoinGameAction(repo, cardRegistry, logger)

	// Try to join an active game
	playerID := uuid.New().String()
	_, err := joinAction.Execute(context.Background(), testGame.ID(), "Late Joiner", playerID)

	// Assert
	testutil.AssertErrorContains(t, err, "game is not in lobby", "Should not allow joining non-lobby game")
}

func TestJoinGameAction_MaxPlayersReached(t *testing.T) {
	// Setup
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()
	repo := testutil.NewTestGameRepository(t)

	// Create game with max 2 players
	settings := shared.GameSettings{
		MaxPlayers: 2,
		GameModeID: "classic", Features: []string{},
	}

	ds, _ := datastore.NewDataStore()
	testGame := game.NewGame(ds, "test-game", "", settings, board.GenerateMarsBoard(false))
	testutil.AssertNoError(t, repo.Create(context.Background(), testGame), "create game in repo")

	// Add 2 players
	ctx := context.Background()
	_, err := testGame.AddNewPlayer(ctx, "player-1", "Player1")
	testutil.AssertNoError(t, err, "add player 1")
	_, err = testGame.AddNewPlayer(ctx, "player-2", "Player2")
	testutil.AssertNoError(t, err, "add player 2")

	joinAction := gameAction.NewJoinGameAction(repo, cardRegistry, logger)

	// Try to add 3rd player
	playerID := uuid.New().String()
	_, err = joinAction.Execute(context.Background(), testGame.ID(), "Player3", playerID)

	// Assert
	testutil.AssertErrorContains(t, err, "game is full", "Should not allow joining when max players reached")
}

func TestJoinGameAction_MaxPlayerNameLengthConst(t *testing.T) {
	testutil.AssertEqual(t, 45, game.MaxPlayerNameLength, "MaxPlayerNameLength should be 45")
}

func TestJoinGameAction_RejectsOverLengthName(t *testing.T) {
	// Setup
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 0)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()

	joinAction := gameAction.NewJoinGameAction(repo, cardRegistry, logger)

	// Execute with a 46-rune name (one over the cap)
	name := strings.Repeat("a", 46)
	playerID := uuid.New().String()
	_, err := joinAction.Execute(context.Background(), testGame.ID(), name, playerID)

	// Assert - rejected loudly, no player added
	testutil.AssertErrorContains(t, err, "player name exceeds maximum length", "Should reject name longer than the cap")

	fetchedGame, _ := repo.Get(context.Background(), testGame.ID())
	players := fetchedGame.GetAllPlayers()
	testutil.AssertEqual(t, 0, len(players), "Should not add a player with an over-length name")
}

func TestJoinGameAction_AcceptsMaxLengthName(t *testing.T) {
	// Setup
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 0)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()

	joinAction := gameAction.NewJoinGameAction(repo, cardRegistry, logger)

	// Execute with a name of exactly the maximum length
	name := strings.Repeat("a", 45)
	playerID := uuid.New().String()
	_, err := joinAction.Execute(context.Background(), testGame.ID(), name, playerID)

	// Assert - accepted, player added
	testutil.AssertNoError(t, err, "Should accept name of exactly the maximum length")

	fetchedGame, _ := repo.Get(context.Background(), testGame.ID())
	players := fetchedGame.GetAllPlayers()
	testutil.AssertEqual(t, 1, len(players), "Should add a player with a max-length name")
	testutil.AssertEqual(t, name, players[0].Name(), "Player name should match")
}

func TestJoinGameAction_SetHostForFirstPlayer(t *testing.T) {
	// Setup
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 0)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()

	joinAction := gameAction.NewJoinGameAction(repo, cardRegistry, logger)

	// Verify no host initially
	testutil.AssertEqual(t, "", testGame.HostPlayerID(), "Host should be empty initially")

	// First player joins
	playerID := uuid.New().String()
	result, err := joinAction.Execute(context.Background(), testGame.ID(), "Host Player", playerID)

	// Assert
	testutil.AssertNoError(t, err, "Failed to join as first player")

	fetchedGame, _ := repo.Get(context.Background(), testGame.ID())
	testutil.AssertEqual(t, result.PlayerID, fetchedGame.HostPlayerID(), "First player should be host")
}
