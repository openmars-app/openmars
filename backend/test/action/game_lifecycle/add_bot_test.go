package game_lifecycle_test

import (
	"context"
	"testing"

	gameAction "openmars/internal/action/game"
	"openmars/internal/game/shared"
	"openmars/internal/service/bot"
	"openmars/test/testutil"
)

type fakeBotLifecycle struct {
	catalog  *bot.PersonaCatalog
	prepared []string
}

func newFakeBotLifecycle(t *testing.T) *fakeBotLifecycle {
	t.Helper()
	catalog, err := bot.NewPersonaCatalog([]bot.Persona{
		{ID: "rival", Label: "Rival", Voice: "competitive", Names: []string{"SHODAN", "Skynet"}},
		{ID: "optimist", Label: "Optimist", Voice: "cheerful", Names: []string{"Wall-E", "R2-D2"}},
	})
	testutil.AssertNoError(t, err, "persona catalog should be valid")
	return &fakeBotLifecycle{catalog: catalog}
}

func (f *fakeBotLifecycle) AssignIdentity(seed uint64, takenNames []string) (string, string) {
	return f.catalog.AssignIdentity(seed, takenNames)
}

func (f *fakeBotLifecycle) PrepareBot(_, playerID string) {
	f.prepared = append(f.prepared, playerID)
}

func lobbyWithToken(t *testing.T, players, maxPlayers int) (*gameAction.AddBotAction, *fakeBotLifecycle, string) {
	t.Helper()
	testGame, repo := testutil.CreateTestGameWithPlayers(t, players)
	testGame.UpdateSettings(context.Background(), shared.GameSettings{
		MaxPlayers:       maxPlayers,
		ClaudeOAuthToken: "test-token",
		GameModeID:       "classic", Features: []string{},
	})
	bots := newFakeBotLifecycle(t)
	return gameAction.NewAddBotAction(repo, testutil.CreateTestCardRegistry(), bots, testutil.TestLogger()), bots, testGame.ID()
}

func TestAddBot_Success(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	testGame.UpdateSettings(context.Background(), shared.GameSettings{
		MaxPlayers:       4,
		ClaudeOAuthToken: "test-token",
		GameModeID:       "classic", Features: []string{},
	})
	bots := newFakeBotLifecycle(t)
	action := gameAction.NewAddBotAction(repo, testutil.CreateTestCardRegistry(), bots, testutil.TestLogger())

	result, err := action.Execute(context.Background(), testGame.ID(), "player-1")
	testutil.AssertNoError(t, err, "Add bot should succeed")

	botPlayer, err := testGame.GetPlayer(result.PlayerID)
	testutil.AssertNoError(t, err, "Should find bot player")
	testutil.AssertTrue(t, botPlayer.IsBot(), "Player should be a bot")
	testutil.AssertTrue(t, botPlayer.Name() != "", "Bot should have a name")
	testutil.AssertTrue(t, botPlayer.BotPersona() == "rival" || botPlayer.BotPersona() == "optimist", "Bot should have a persona from the catalog")
	testutil.AssertEqual(t, 1, len(bots.prepared), "Bot credential check should be started")
	testutil.AssertEqual(t, result.PlayerID, bots.prepared[0], "The new bot should be prepared")
}

func TestAddBot_RejectsNonHost(t *testing.T) {
	action, bots, gameID := lobbyWithToken(t, 2, 4)

	_, err := action.Execute(context.Background(), gameID, "player-2")
	testutil.AssertErrorContains(t, err, "only the host can add bots", "Only the host may add bots")
	testutil.AssertEqual(t, 0, len(bots.prepared), "No bot should be prepared")
}

func TestAddBot_NoToken(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	action := gameAction.NewAddBotAction(repo, testutil.CreateTestCardRegistry(), newFakeBotLifecycle(t), testutil.TestLogger())

	_, err := action.Execute(context.Background(), testGame.ID(), "player-1")
	testutil.AssertErrorContains(t, err, "a Claude token is required", "Add bot should fail without a token")
}

func TestAddBot_GameNotInLobby(t *testing.T) {
	g, repo, cardRegistry, _, _ := testutil.SetupTwoPlayerGame(t)
	g.UpdateSettings(context.Background(), shared.GameSettings{
		MaxPlayers:       4,
		ClaudeOAuthToken: "test-token",
		GameModeID:       "classic", Features: []string{},
	})
	action := gameAction.NewAddBotAction(repo, cardRegistry, newFakeBotLifecycle(t), testutil.TestLogger())

	_, err := action.Execute(context.Background(), g.ID(), g.HostPlayerID())
	testutil.AssertErrorContains(t, err, "game is not in lobby", "Add bot should fail when game is not in lobby")
}

func TestAddBot_GameFull(t *testing.T) {
	action, _, gameID := lobbyWithToken(t, 4, 4)

	_, err := action.Execute(context.Background(), gameID, "player-1")
	testutil.AssertErrorContains(t, err, "game is full", "Add bot should fail when game is full")
}

func TestAddBot_UniqueNames(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	testGame.UpdateSettings(context.Background(), shared.GameSettings{
		MaxPlayers:       5,
		ClaudeOAuthToken: "test-token",
		GameModeID:       "classic", Features: []string{},
	})
	action := gameAction.NewAddBotAction(repo, testutil.CreateTestCardRegistry(), newFakeBotLifecycle(t), testutil.TestLogger())

	names := make(map[string]bool)
	for i := 0; i < 3; i++ {
		result, err := action.Execute(context.Background(), testGame.ID(), "player-1")
		testutil.AssertNoError(t, err, "Bot should be added")
		botPlayer, err := testGame.GetPlayer(result.PlayerID)
		testutil.AssertNoError(t, err, "Should find bot player")
		testutil.AssertFalse(t, names[botPlayer.Name()], "Bot names should be unique")
		names[botPlayer.Name()] = true
	}
}
