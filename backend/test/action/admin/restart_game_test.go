package admin_test

import (
	"context"
	"testing"

	adminAction "openmars/internal/action/admin"
	gameAction "openmars/internal/action/game"
	turnAction "openmars/internal/action/turn_management"
	"openmars/internal/game"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

type stopRecorder struct {
	stopped []string
}

func (s *stopRecorder) StopAllBotsForGame(gameID string) {
	s.stopped = append(s.stopped, gameID)
}

func setupRestartableGame(t *testing.T, developmentMode bool) (*game.Game, game.GameRepository, *adminAction.RestartGameAction, *stopRecorder) {
	t.Helper()
	ctx := context.Background()
	logger := testutil.TestLogger()
	cardRegistry := testutil.CreateTestModeCatalog()
	repo := testutil.NewTestGameRepository(t)

	createAction := gameAction.NewCreateGameAction(repo, cardRegistry, testutil.CreateTestMapRegistry(), logger)
	startAction := turnAction.NewStartGameAction(repo, nil, nil, nil, nil, nil, logger, nil)

	g, err := createAction.Execute(ctx, shared.GameSettings{
		MaxPlayers: 4,
		GameModeID: "classic", Features: []string{"prelude"},
		DevelopmentMode: developmentMode,
	})
	testutil.AssertNoError(t, err, "create game")

	_, err = g.AddNewPlayer(ctx, "host", "Host")
	testutil.AssertNoError(t, err, "add host")
	_, err = g.AddNewPlayer(ctx, "guest", "Guest")
	testutil.AssertNoError(t, err, "add guest")
	testutil.AssertNoError(t, g.SetHostPlayerID(ctx, "host"), "set host")

	testutil.AssertNoError(t, startAction.Execute(ctx, g.ID(), "host"), "start game")

	stopper := &stopRecorder{}
	restart := adminAction.NewRestartGameAction(repo, createAction, startAction, cardRegistry, nil, stopper, logger)
	return g, repo, restart, stopper
}

func TestRestartGame_DealsFreshStartWithSamePlayers(t *testing.T) {
	ctx := context.Background()
	oldGame, repo, restart, stopper := setupRestartableGame(t, true)

	oldHost, _ := oldGame.GetPlayer("host")
	oldColors := map[string]string{}
	for _, p := range oldGame.GetAllPlayers() {
		oldColors[p.ID()] = p.Color()
	}
	testutil.AssertNoError(t, oldGame.UpdatePhase(ctx, shared.GamePhaseAction), "move to action phase")
	testutil.SetPlayerCredits(ctx, oldHost, 99)
	oldHost.SetCorporationID("B01")

	testutil.AssertNoError(t, restart.Execute(ctx, oldGame.ID()), "restart")

	g, err := repo.Get(ctx, oldGame.ID())
	testutil.AssertNoError(t, err, "get restarted game")
	testutil.AssertTrue(t, g != oldGame, "game instance is rebuilt")
	testutil.AssertEqual(t, 1, len(stopper.stopped), "bots stopped once")
	testutil.AssertEqual(t, oldGame.ID(), stopper.stopped[0], "bots stopped for the game")

	testutil.AssertEqual(t, shared.GameStatusActive, g.Status(), "active")
	testutil.AssertEqual(t, shared.GamePhaseStartingSelection, g.CurrentPhase(), "starting selection")
	testutil.AssertEqual(t, 1, g.Generation(), "generation reset")
	testutil.AssertEqual(t, "host", g.HostPlayerID(), "host kept")

	players := g.GetAllPlayers()
	testutil.AssertEqual(t, 2, len(players), "both players kept")
	for _, p := range players {
		testutil.AssertEqual(t, oldColors[p.ID()], p.Color(), "color kept for "+p.ID())
		testutil.AssertTrue(t, g.GetSelectCorporationPhase(p.ID()) != nil, "corps dealt to "+p.ID())
		testutil.AssertTrue(t, g.GetSelectPreludeCardsPhase(p.ID()) != nil, "preludes dealt to "+p.ID())
		testutil.AssertEqual(t, 20, p.Resources().TerraformRating(), "TR reset for "+p.ID())
	}

	host, _ := g.GetPlayer("host")
	testutil.AssertEqual(t, "Host", host.Name(), "name kept")
	testutil.AssertEqual(t, 0, host.Resources().Get().Credits, "credits reset")
	testutil.AssertFalse(t, host.HasCorporation(), "corporation cleared")

	history, err := repo.DataStore().GetGameHistory(g.ID())
	testutil.AssertNoError(t, err, "history")
	for _, entry := range history {
		testutil.AssertTrue(t, entry.State.CurrentPhase != shared.GamePhaseAction, "old history cleared")
	}
}

func TestRestartGame_RejectedOutsideDevelopmentMode(t *testing.T) {
	g, _, restart, stopper := setupRestartableGame(t, false)

	err := restart.Execute(context.Background(), g.ID())
	testutil.AssertErrorContains(t, err, "only available in development mode", "restart outside development mode")
	testutil.AssertEqual(t, 0, len(stopper.stopped), "bots untouched")
}
