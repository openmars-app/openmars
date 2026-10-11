package game_lifecycle_test

import (
	"context"
	"testing"

	turnAction "openmars/internal/action/turn_management"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func TestStartGameAction_Success(t *testing.T) {
	// Setup
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()

	// Set corporations for all players
	players := testGame.GetAllPlayers()
	for _, p := range players {
		p.SetCorporationID(testutil.PlaceholderCorporationID)
	}

	startAction := turnAction.NewStartGameAction(repo, nil, nil, nil, nil, nil, logger, nil)

	// Execute
	err := startAction.Execute(context.Background(), testGame.ID(), testGame.HostPlayerID())

	// Assert
	testutil.AssertNoError(t, err, "Failed to start game")

	fetchedGame, _ := repo.Get(context.Background(), testGame.ID())
	testutil.AssertEqual(t, shared.GameStatusActive, fetchedGame.Status(), "Game should be active")
	testutil.AssertTrue(t, fetchedGame.CurrentPhase() != "", "Game phase should be set")
}

func TestStartGameAction_GameNotFound(t *testing.T) {
	// Setup
	repo := testutil.NewTestGameRepository(t)
	logger := testutil.TestLogger()

	startAction := turnAction.NewStartGameAction(repo, nil, nil, nil, nil, nil, logger, nil)

	// Execute
	err := startAction.Execute(context.Background(), "non-existent-game", "some-player")

	// Assert
	testutil.AssertErrorContains(t, err, "game not found", "Should fail when game doesn't exist")
}

func TestStartGameAction_NotInLobby(t *testing.T) {
	// Setup
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()

	// Set corporations and start game
	ctx := context.Background()
	players := testGame.GetAllPlayers()
	for _, p := range players {
		p.SetCorporationID(testutil.PlaceholderCorporationID)
	}

	// Start game once using action
	startAction := turnAction.NewStartGameAction(repo, nil, nil, nil, nil, nil, logger, nil)
	testutil.AssertNoError(t, startAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), "start game")

	// Try to start again
	err := startAction.Execute(context.Background(), testGame.ID(), testGame.HostPlayerID())

	// Assert
	testutil.AssertErrorContains(t, err, "game is not in lobby", "Should not allow starting non-lobby game")
}

func TestStartGameAction_NotHost(t *testing.T) {
	// Setup
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()

	// Set corporations
	players := testGame.GetAllPlayers()
	for _, p := range players {
		p.SetCorporationID(testutil.PlaceholderCorporationID)
	}

	startAction := turnAction.NewStartGameAction(repo, nil, nil, nil, nil, nil, logger, nil)

	// Get non-host player
	nonHostPlayer := ""
	for _, p := range players {
		if p.ID() != testGame.HostPlayerID() {
			nonHostPlayer = p.ID()
			break
		}
	}

	// Try to start game as non-host
	err := startAction.Execute(context.Background(), testGame.ID(), nonHostPlayer)

	// Assert
	testutil.AssertErrorContains(t, err, "only host can start the game", "Should not allow non-host to start game")
}

func TestStartGameAction_MinimumPlayers(t *testing.T) {
	// Setup
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()

	// Set corporation for single player
	players := testGame.GetAllPlayers()
	players[0].SetCorporationID(testutil.PlaceholderCorporationID)

	startAction := turnAction.NewStartGameAction(repo, nil, nil, nil, nil, nil, logger, nil)

	// Execute - should allow solo play
	err := startAction.Execute(context.Background(), testGame.ID(), testGame.HostPlayerID())

	// Assert - solo games should be allowed
	testutil.AssertNoError(t, err, "Solo play should be allowed")

	// Verify game state
	fetchedGame, _ := repo.Get(context.Background(), testGame.ID())
	testutil.AssertEqual(t, shared.GameStatusActive, fetchedGame.Status(), "Game should be active after start")
}

func TestStartGameAction_AssignsPlayerColors(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 3)
	logger := testutil.TestLogger()

	players := testGame.GetAllPlayers()
	for _, p := range players {
		p.SetCorporationID(testutil.PlaceholderCorporationID)
	}

	startAction := turnAction.NewStartGameAction(repo, nil, nil, nil, nil, nil, logger, nil)
	err := startAction.Execute(context.Background(), testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Failed to start game")

	fetchedGame, _ := repo.Get(context.Background(), testGame.ID())
	fetchedPlayers := fetchedGame.GetAllPlayers()

	colors := make(map[string]bool)
	for _, p := range fetchedPlayers {
		testutil.AssertTrue(t, p.Color() != "", "Player should have a color assigned")
		testutil.AssertTrue(t, p.Color()[0] == '#', "Color should be a hex string")
		colors[p.Color()] = true
	}
	testutil.AssertEqual(t, 3, len(colors), "Each player should have a unique color")
}

func TestStartGameAction_InitialResourcesSet(t *testing.T) {
	// Setup
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()

	// Set corporations
	players := testGame.GetAllPlayers()
	for _, p := range players {
		p.SetCorporationID(testutil.PlaceholderCorporationID)
	}

	startAction := turnAction.NewStartGameAction(repo, nil, nil, nil, nil, nil, logger, nil)

	// Execute
	err := startAction.Execute(context.Background(), testGame.ID(), testGame.HostPlayerID())

	// Assert
	testutil.AssertNoError(t, err, "Failed to start game")

	fetchedGame, _ := repo.Get(context.Background(), testGame.ID())
	players = fetchedGame.GetAllPlayers()

	// Verify players have initial resources
	for _, p := range players {
		resources := p.Resources()
		testutil.AssertTrue(t, resources != nil, "Player should have resources")
		// Initial terraform rating should be set
		testutil.AssertTrue(t, p.Resources().TerraformRating() >= 20, "Player should have initial TR")
	}
}
