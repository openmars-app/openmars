package core_test

import (
	"context"
	"fmt"
	"testing"

	"path/filepath"
	"runtime"

	"openmars/internal/action"
	baseaction "openmars/internal/action"
	cardAction "openmars/internal/action/card"
	gameaction "openmars/internal/action/game"
	resconvAction "openmars/internal/action/resource_conversion"
	spAction "openmars/internal/action/standard_project"
	turnmgmt "openmars/internal/action/turn_management"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/internal/game/standardproject"
	"openmars/test/testutil"
)

func TestPlayCardConsumesAction(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()

	p, _ := testGame.GetPlayer(playerID)
	testutil.SetPlayerCredits(context.Background(), p, 100)
	p.Hand().AddCard(testutil.CardID("Power Plant"))

	playAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 4)

	err := playAction.Execute(context.Background(), testGame.ID(), playerID, testutil.CardID("Power Plant"), payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Playing card should succeed")

	turn := testGame.CurrentTurn()
	testutil.AssertEqual(t, 1, turn.ActionsRemaining(), "Should have 1 action remaining after playing card")
}

func TestZeroActionsBlocksCardPlay(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()

	// Set actions to 0
	err := testGame.SetCurrentTurn(context.Background(), playerID, 0)
	testutil.AssertNoError(t, err, "Setting turn should succeed")

	p, _ := testGame.GetPlayer(playerID)
	testutil.SetPlayerCredits(context.Background(), p, 100)
	p.Hand().AddCard(testutil.CardID("Power Plant"))

	playAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 4)

	err = playAction.Execute(context.Background(), testGame.ID(), playerID, testutil.CardID("Power Plant"), payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "no actions remaining", "Should fail with 0 actions remaining")
	testutil.AssertEqual(t, 100, p.Resources().Get().Credits, "credits unchanged")
	testutil.AssertTrue(t, p.Hand().HasCard(testutil.CardID("Power Plant")), "card stays in hand")
}

func TestZeroActionsBlocksStandardProject(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()

	err := testGame.SetCurrentTurn(context.Background(), playerID, 0)
	testutil.AssertNoError(t, err, "Setting turn should succeed")

	p, _ := testGame.GetPlayer(playerID)
	testutil.SetPlayerCredits(context.Background(), p, 100)

	_, currentFile, _, _ := runtime.Caller(0)
	stdProjPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "assets", "standard_projects.json")
	stdProjData, _ := standardproject.LoadStandardProjectsFromJSON(stdProjPath)
	stdProjRegistry := standardproject.NewInMemoryStandardProjectRegistry(stdProjData)

	buildAction := spAction.NewExecuteStandardProjectAction(repo, cardRegistry, stdProjRegistry, nil, logger)

	err = buildAction.Execute(context.Background(), testGame.ID(), playerID, "power-plant", shared.NativePayment(shared.ResourceCredit, shared.StandardProjectCost[shared.StandardProject("power-plant")]))
	testutil.AssertErrorContains(t, err, "no actions remaining", "Should fail with 0 actions remaining")
	testutil.AssertEqual(t, 100, p.Resources().Get().Credits, "credits unchanged")
}

func TestZeroActionsBlocksConvertHeat(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()

	err := testGame.SetCurrentTurn(context.Background(), playerID, 0)
	testutil.AssertNoError(t, err, "Setting turn should succeed")

	p, _ := testGame.GetPlayer(playerID)
	testutil.SetPlayerHeat(context.Background(), p, 20)

	convertAction := resconvAction.NewConvertHeatToTemperatureAction(repo, cardRegistry, nil, logger)

	err = convertAction.Execute(context.Background(), testGame.ID(), playerID, shared.NativePayment(shared.ResourceHeat, 8))
	testutil.AssertErrorContains(t, err, "no actions remaining", "Should fail with 0 actions remaining")
	testutil.AssertEqual(t, 20, p.Resources().Get().Heat, "heat unchanged")
}

func TestAutoAdvanceAfterSecondAction(t *testing.T) {
	testGame, repo, cardRegistry, player1ID, player2ID := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()

	p1, _ := testGame.GetPlayer(player1ID)
	testutil.SetPlayerCredits(context.Background(), p1, 200)

	// Play first card
	p1.Hand().AddCard(testutil.CardID("Power Plant"))
	playAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 4)

	err := playAction.Execute(context.Background(), testGame.ID(), player1ID, testutil.CardID("Power Plant"), payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "First card play should succeed")
	testutil.AssertEqual(t, 1, testGame.CurrentTurn().ActionsRemaining(), "Should have 1 action after first play")
	testutil.AssertEqual(t, player1ID, testGame.CurrentTurn().PlayerID(), "Should still be player 1's turn")

	// Play second card
	p1.Hand().AddCard(testutil.CardID("Asteroid"))
	payment2 := shared.NativePayment(shared.ResourceCredit, 14)
	err = playAction.Execute(context.Background(), testGame.ID(), player1ID, testutil.CardID("Asteroid"), payment2, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Second card play should succeed")

	// Should auto-advance to player 2
	testutil.AssertEqual(t, player2ID, testGame.CurrentTurn().PlayerID(), "Turn should auto-advance to player 2")
	testutil.AssertEqual(t, 2, testGame.CurrentTurn().ActionsRemaining(), "Player 2 should have 2 actions")
}

func TestSoloUnlimitedActionsNotBlocked(t *testing.T) {
	testGame, repo, cardRegistry, playerID := testutil.SetupSoloGame(t)
	logger := testutil.TestLogger()

	p, _ := testGame.GetPlayer(playerID)
	testutil.SetPlayerCredits(context.Background(), p, 200)

	// Verify unlimited actions
	testutil.AssertEqual(t, -1, testGame.CurrentTurn().ActionsRemaining(), "Solo should have unlimited actions")

	// Play card - should succeed and remain unlimited
	p.Hand().AddCard(testutil.CardID("Power Plant"))
	playAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 4)

	err := playAction.Execute(context.Background(), testGame.ID(), playerID, testutil.CardID("Power Plant"), payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Solo card play should succeed")

	testutil.AssertEqual(t, -1, testGame.CurrentTurn().ActionsRemaining(), "Solo should still have unlimited actions")
	testutil.AssertEqual(t, playerID, testGame.CurrentTurn().PlayerID(), "Solo player should still have the turn")
}

func TestStateCalculatorReportsNoActionsRemaining(t *testing.T) {
	testGame, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)

	err := testGame.SetCurrentTurn(context.Background(), playerID, 0)
	testutil.AssertNoError(t, err, "Setting turn should succeed")

	p, _ := testGame.GetPlayer(playerID)
	testutil.SetPlayerCredits(context.Background(), p, 200)

	// Check state calculator for standard project
	state := action.CalculatePlayerStandardProjectState(
		"power-plant",
		p,
		testGame,
		cardRegistry,
	)

	hasNoActionsError := false
	for _, stateErr := range state.Errors {
		if stateErr.Code == player.ErrorCodeNoActionsRemaining {
			hasNoActionsError = true
			break
		}
	}
	testutil.AssertTrue(t, hasNoActionsError, "State calculator should report no-actions-remaining error")
}

func TestAutoAdvanceWaitsForPendingTileSelection(t *testing.T) {
	testGame, _, _, player1ID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()

	// Set pending tile selection for player 1
	err := testGame.SetPendingTileSelection(context.Background(), player1ID, &shared.PendingTileSelection{
		TileType:       "greenery",
		AvailableHexes: []string{"0,1,-1"},
		Source:         "test",
	})
	testutil.AssertNoError(t, err, "Setting pending tile selection should succeed")

	// Set actions to 0 (simulating consumed action)
	err = testGame.SetCurrentTurn(context.Background(), player1ID, 0)
	testutil.AssertNoError(t, err, "Setting turn should succeed")

	// Call AutoAdvanceTurnIfNeeded - should NOT advance because of pending tile
	baseaction.AutoAdvanceTurnIfNeeded(testGame, player1ID, logger, testutil.CreateTestCardRegistry())

	testutil.AssertEqual(t, player1ID, testGame.CurrentTurn().PlayerID(), "Should NOT advance due to pending tile selection")
}

func TestSkipWithOneActionAdvancesToNextPlayer(t *testing.T) {
	testGame, repo, cardRegistry, player1ID, player2ID := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()

	// Set player 1 to have 1 action remaining
	err := testGame.SetCurrentTurn(context.Background(), player1ID, 1)
	testutil.AssertNoError(t, err, "Setting turn should succeed")

	// Create skip action
	finalScoringAction := gameaction.NewFinalScoringAction(repo, cardRegistry, nil, nil, logger)
	skipAction := turnmgmt.NewSkipActionAction(repo, finalScoringAction, logger)

	// Player 1 SKIPs with 1 action
	err = skipAction.Execute(context.Background(), testGame.ID(), player1ID)
	testutil.AssertNoError(t, err, "SKIP should succeed")

	// Turn should advance to player 2 with 2 actions
	testutil.AssertEqual(t, player2ID, testGame.CurrentTurn().PlayerID(), "Turn should advance to player 2")
	testutil.AssertEqual(t, 2, testGame.CurrentTurn().ActionsRemaining(), "Player 2 should have 2 actions")
}

func TestAutoAdvanceGrantsUnlimitedActionsToLastNonPassedPlayer(t *testing.T) {
	testGame, repo, cardRegistry, player1ID, player2ID := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()

	// Player 2 passes
	p2, _ := testGame.GetPlayer(player2ID)
	p2.SetPassed(true)

	// Player 1 plays two cards to consume both actions
	p1, _ := testGame.GetPlayer(player1ID)
	testutil.SetPlayerCredits(context.Background(), p1, 200)

	// Play first card
	p1.Hand().AddCard(testutil.CardID("Power Plant"))
	playAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 4)

	err := playAction.Execute(context.Background(), testGame.ID(), player1ID, testutil.CardID("Power Plant"), payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "First card play should succeed")
	testutil.AssertEqual(t, 1, testGame.CurrentTurn().ActionsRemaining(), "Should have 1 action after first play")

	// Play second card - this should auto-advance and grant unlimited actions to player 1
	p1.Hand().AddCard(testutil.CardID("Asteroid"))
	payment2 := shared.NativePayment(shared.ResourceCredit, 14)
	err = playAction.Execute(context.Background(), testGame.ID(), player1ID, testutil.CardID("Asteroid"), payment2, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Second card play should succeed")

	// Player 1 should now have unlimited actions since they're the last non-passed player
	testutil.AssertEqual(t, player1ID, testGame.CurrentTurn().PlayerID(), "Player 1 should still have the turn")
	testutil.AssertEqual(t, -1, testGame.CurrentTurn().ActionsRemaining(), "Player 1 should have unlimited actions as last non-passed player")
}

func TestSkipGrantsUnlimitedActionsToLastNonPassedPlayer(t *testing.T) {
	testGame, repo, cardRegistry, player1ID, player2ID := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()

	// Player 2 passes
	p2, _ := testGame.GetPlayer(player2ID)
	p2.SetPassed(true)

	// Player 1 has 1 action and SKIPs
	err := testGame.SetCurrentTurn(context.Background(), player1ID, 1)
	testutil.AssertNoError(t, err, "Setting turn should succeed")

	// Create skip action
	finalScoringAction := gameaction.NewFinalScoringAction(repo, cardRegistry, nil, nil, logger)
	skipAction := turnmgmt.NewSkipActionAction(repo, finalScoringAction, logger)

	// Player 1 SKIPs
	err = skipAction.Execute(context.Background(), testGame.ID(), player1ID)
	testutil.AssertNoError(t, err, "SKIP should succeed")

	// Player 1 should get unlimited actions since they're the last non-passed player
	testutil.AssertEqual(t, player1ID, testGame.CurrentTurn().PlayerID(), "Player 1 should still have the turn")
	testutil.AssertEqual(t, -1, testGame.CurrentTurn().ActionsRemaining(), "Player 1 should have unlimited actions as last non-passed player")
}

func TestTurnOrderRotatesAfterGeneration(t *testing.T) {
	testGame, repo, cardRegistry, player1ID, player2ID := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()

	// Verify initial turn order
	initialTurnOrder := testGame.TurnOrder()
	testutil.AssertEqual(t, player1ID, initialTurnOrder[0], "Player 1 should be first in initial turn order")
	testutil.AssertEqual(t, player2ID, initialTurnOrder[1], "Player 2 should be second in initial turn order")

	// Both players pass to trigger production phase
	finalScoringAction := gameaction.NewFinalScoringAction(repo, cardRegistry, nil, nil, logger)
	skipAction := turnmgmt.NewSkipActionAction(repo, finalScoringAction, logger)

	// Player 1 passes (2 actions = pass)
	err := skipAction.Execute(context.Background(), testGame.ID(), player1ID)
	testutil.AssertNoError(t, err, "Player 1 PASS should succeed")

	// Player 2 passes (2 actions = pass)
	err = skipAction.Execute(context.Background(), testGame.ID(), player2ID)
	testutil.AssertNoError(t, err, "Player 2 PASS should succeed")

	// After both pass, production phase triggers and turn order should rotate
	// Player 2 should now be first in turn order
	newTurnOrder := testGame.TurnOrder()
	testutil.AssertEqual(t, player2ID, newTurnOrder[0], "Player 2 should be first after turn order rotation")
	testutil.AssertEqual(t, player1ID, newTurnOrder[1], "Player 1 should be second after turn order rotation")
}

func TestForcedFirstActionConsumesExactlyOnePlayerAction(t *testing.T) {
	g, _, _, id, _ := testutil.SetupTwoPlayerGame(t)
	ctx := context.Background()
	testutil.AssertNoError(t, g.SetForcedFirstAction(ctx, id, &shared.ForcedFirstAction{CorporationID: testutil.CardID("Tharsis Republic"), State: "resolving"}), "queue first action")
	testutil.AssertTrue(t, g.CompleteFirstActionIfReady(id), "complete first action")
	testutil.AssertEqual(t, 1, g.CurrentTurn().ActionsRemaining(), "first action consumes one action")
	testutil.AssertFalse(t, g.CompleteFirstActionIfReady(id), "completion cannot replay")
	testutil.AssertEqual(t, 1, g.CurrentTurn().ActionsRemaining(), "duplicate completion consumes nothing")
}

func TestCardAction_RequirementsCheckedBeforePayment(t *testing.T) {
	for _, reuse := range []bool{false, true} {
		t.Run(fmt.Sprint(reuse), func(t *testing.T) {
			g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			testutil.SetPlayerCredits(context.Background(), p, 10)
			minimum := 5
			scienceTag := shared.TagScience
			behavior := shared.CardBehavior{
				Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
				Inputs:   []shared.BehaviorCondition{shared.NewBasicResourceCondition(shared.ResourceCredit, 2, "self-player")},
				Choices:  []shared.Choice{{Requirements: &shared.ChoiceRequirements{Items: []shared.ChoiceRequirement{{Type: "tags", Tag: &scienceTag, Min: &minimum}}}, Outputs: []shared.BehaviorCondition{shared.NewBasicResourceCondition(shared.ResourceHeat, 2, "self-player")}}},
			}
			target := shared.CardAction{CardID: "conditional-action", Behavior: behavior}
			var source *string
			if reuse {
				target.TimesUsedThisGeneration = 1
				value := "reuse"
				source = &value
			}
			p.Actions().SetActions([]shared.CardAction{target, {CardID: "reuse", Behavior: shared.CardBehavior{Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}}, Outputs: []shared.BehaviorCondition{shared.NewEffectCondition(shared.ResourceActionReuse, 1, "self-player")}}}})
			choice := 0
			err := cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger()).Execute(context.Background(), g.ID(), id, target.CardID, 0, &choice, nil, nil, nil, nil, nil, source, nil)
			testutil.AssertErrorContains(t, err, "Tag requirements not met", "unmet requirement")
			testutil.AssertEqual(t, 10, p.Resources().Get().Credits, "no payment")
			testutil.AssertEqual(t, 0, p.Resources().Get().Heat, "no output")
			testutil.AssertEqual(t, 2, g.CurrentTurn().ActionsRemaining(), "no action consumed")
			if reuse {
				options := baseaction.CalculateActionReuseOptions("reuse", p, g, registry)
				testutil.AssertEqual(t, 1, len(options), "target listed")
				testutil.AssertTrue(t, len(options[0].Errors) > 0, "UI also blocks unmet requirement")
			}
		})
	}
}
