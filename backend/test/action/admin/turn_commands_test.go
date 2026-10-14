package admin_test

import (
	"context"
	"testing"

	baseaction "openmars/internal/action"
	adminAction "openmars/internal/action/admin"
	"openmars/internal/game"
	"openmars/test/testutil"
)

func setupTurnGame(t *testing.T) (*game.Game, *adminAction.SetCurrentTurnAction, *adminAction.SetActionsRemainingAction) {
	t.Helper()
	g, repo := testutil.CreateTestGameWithPlayers(t, 3)
	testutil.StartTestGame(t, g)
	logger := testutil.TestLogger()
	return g, adminAction.NewSetCurrentTurnAction(repo, logger), adminAction.NewSetActionsRemainingAction(repo, logger)
}

func TestAdminMoveTurn_GivesNormalTurnAndContinuesInOrder(t *testing.T) {
	ctx := context.Background()
	g, moveTurn, _ := setupTurnGame(t)
	order := g.TurnOrder()

	testutil.AssertNoError(t, moveTurn.Execute(ctx, g.ID(), order[1]), "move turn")
	testutil.AssertEqual(t, order[1], g.CurrentTurn().PlayerID(), "turn moved")
	testutil.AssertEqual(t, 2, g.CurrentTurn().ActionsRemaining(), "normal actions")

	g.CurrentTurn().ConsumeAction()
	g.CurrentTurn().ConsumeAction()
	baseaction.AutoAdvanceTurnIfNeeded(g, order[1], testutil.TestLogger(), testutil.CreateTestCardRegistry())
	testutil.AssertEqual(t, order[2], g.CurrentTurn().PlayerID(), "turn continues to the next player")
}

func TestAdminMoveTurn_RejectsPassedPlayer(t *testing.T) {
	ctx := context.Background()
	g, moveTurn, _ := setupTurnGame(t)
	order := g.TurnOrder()

	passed, _ := g.GetPlayer(order[1])
	passed.SetPassed(true)

	testutil.AssertErrorContains(t, moveTurn.Execute(ctx, g.ID(), order[1]), "has passed or left", "passed player cannot get the turn")
	testutil.AssertEqual(t, order[0], g.CurrentTurn().PlayerID(), "turn unchanged")
}

func TestAdminMoveTurn_LastActivePlayerGetsUnlimited(t *testing.T) {
	ctx := context.Background()
	g, moveTurn, _ := setupTurnGame(t)
	order := g.TurnOrder()

	for _, id := range []string{order[0], order[1]} {
		p, _ := g.GetPlayer(id)
		p.SetPassed(true)
	}

	testutil.AssertNoError(t, moveTurn.Execute(ctx, g.ID(), order[2]), "move turn")
	testutil.AssertEqual(t, -1, g.CurrentTurn().ActionsRemaining(), "unlimited for the last player in")
}

func TestAdminSetActionsRemaining(t *testing.T) {
	ctx := context.Background()
	g, _, setActions := setupTurnGame(t)

	testutil.AssertNoError(t, setActions.Execute(ctx, g.ID(), 5), "set actions")
	testutil.AssertEqual(t, 5, g.CurrentTurn().ActionsRemaining(), "remaining updated")
	testutil.AssertEqual(t, 5, g.CurrentTurn().TotalActions(), "total updated")

	testutil.AssertErrorContains(t, setActions.Execute(ctx, g.ID(), 0), "actions must be at least 1", "zero rejected")
	testutil.AssertEqual(t, 5, g.CurrentTurn().ActionsRemaining(), "unchanged after rejection")
}
