package game_lifecycle_test

import (
	"context"
	"testing"

	confirmationAction "openmars/internal/action/confirmation"
	tileAction "openmars/internal/action/tile"
	turnAction "openmars/internal/action/turn_management"
	"openmars/internal/delivery/dto"
	"openmars/internal/game"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

// completeAllSelections makes both players select their starting choices and returns the confirm action.
func completeAllSelections(
	t *testing.T,
	testGame *game.Game,
	selectAction *turnAction.SelectStartingChoicesAction,
	playerID1, playerID2 string,
	hasPrelude bool,
) *turnAction.ConfirmInitAdvanceAction {
	t.Helper()
	if !hasPrelude {
		return completeSelectionsWithPreludes(t, testGame, selectAction, playerID1, playerID2, nil, nil)
	}
	return completeSelectionsWithPreludes(t, testGame, selectAction, playerID1, playerID2,
		[]string{"P01", "P03"}, []string{"P04", "P07"})
}

// completeSelectionsWithPreludes makes both players select their starting choices with the given
// preludes (nil when prelude is disabled) and returns the confirm action.
func completeSelectionsWithPreludes(
	t *testing.T,
	testGame *game.Game,
	selectAction *turnAction.SelectStartingChoicesAction,
	playerID1, playerID2 string,
	preludes1, preludes2 []string,
) *turnAction.ConfirmInitAdvanceAction {
	t.Helper()
	ctx := context.Background()
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()

	if preludes1 != nil {
		err := testGame.SetSelectPreludeCardsPhase(ctx, playerID1, &shared.SelectPreludeCardsPhase{
			AvailablePreludes: preludes1,
			MaxSelectable:     2,
		})
		testutil.AssertNoError(t, err, "set prelude phase for player 1")
		err = testGame.SetSelectPreludeCardsPhase(ctx, playerID2, &shared.SelectPreludeCardsPhase{
			AvailablePreludes: preludes2,
			MaxSelectable:     2,
		})
		testutil.AssertNoError(t, err, "set prelude phase for player 2")
	} else {
		preludes1 = []string{}
		preludes2 = []string{}
	}

	corpID1 := testGame.GetSelectCorporationPhase(playerID1).AvailableCorporations[0]
	err := selectAction.Execute(ctx, testGame.ID(), playerID1, corpID1, preludes1, []string{}, shared.Payment{})
	testutil.AssertNoError(t, err, "Failed to select starting choices for player 1")

	corpID2 := testGame.GetSelectCorporationPhase(playerID2).AvailableCorporations[0]
	err = selectAction.Execute(ctx, testGame.ID(), playerID2, corpID2, preludes2, []string{}, shared.Payment{})
	testutil.AssertNoError(t, err, "Failed to select starting choices for player 2")

	return turnAction.NewConfirmInitAdvanceAction(
		gameRepoWithGame(t, testGame),
		cardRegistry,
		nil,
		nil,
		logger,
	)
}

// gameRepoWithGame creates a repo and inserts the given game.
func gameRepoWithGame(t *testing.T, g *game.Game) game.GameRepository {
	t.Helper()
	repo := testutil.NewTestGameRepository(t)
	err := repo.Create(context.Background(), g)
	testutil.AssertNoError(t, err, "Failed to create game in repo")
	return repo
}

func TestInitPhase_CorpAppliedOneAtATime(t *testing.T) {
	testGame, selectAction, playerID1, playerID2 := setupStartingSelectionGame(t, false)
	ctx := context.Background()

	confirmAction := completeAllSelections(t, testGame, selectAction, playerID1, playerID2, false)

	// After both players select, game should be in init_apply_corp with NO effects applied yet
	testutil.AssertEqual(t, shared.GamePhaseInitApplyCorp, testGame.CurrentPhase(), "Should be in init_apply_corp")
	testutil.AssertTrue(t, testGame.InitPhaseWaitingForConfirm(), "Should be waiting for confirm")

	// First confirm: apply player 1's corp
	err := confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Should apply player 1 corp")

	p1, _ := testGame.GetPlayer(playerID1)
	testutil.AssertTrue(t, p1.HasCorporation(), "Player 1 should have corp after confirm")
	testutil.AssertTrue(t, testGame.InitPhaseWaitingForConfirm(), "Should be waiting after corp applied")

	// Second confirm: advance past player 1 to player 2
	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Should advance to player 2")
	testutil.AssertTrue(t, testGame.InitPhaseWaitingForConfirm(), "Should be waiting for player 2 corp")

	// Third confirm: apply player 2's corp
	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Should apply player 2 corp")

	p2, _ := testGame.GetPlayer(playerID2)
	testutil.AssertTrue(t, p2.HasCorporation(), "Player 2 should have corp after confirm")

	// Fourth confirm: advance past player 2 → roster (no prelude)
	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Should advance to roster")
	testutil.AssertEqual(t, shared.GamePhaseInitApplyCorp, testGame.CurrentPhase(), "Roster stays in init_apply_corp")
	testutil.AssertTrue(t, testGame.InitPhaseRoster(), "Should show roster")
	testutil.AssertTrue(t, testGame.CurrentTurn() == nil, "No turn during roster")

	// Fifth confirm: roster → action phase
	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Should advance to action phase")
	testutil.AssertFalse(t, testGame.InitPhaseRoster(), "Roster cleared")

	testutil.AssertEqual(t, shared.GamePhaseAction, testGame.CurrentPhase(), "Should be in action phase after all corps (no prelude)")
	testutil.AssertTrue(t, testGame.CurrentTurn() != nil, "Current turn should be set")
}

func TestInitPhase_CorpThenPrelude(t *testing.T) {
	testGame, selectAction, playerID1, playerID2 := setupStartingSelectionGame(t, true)
	ctx := context.Background()

	confirmAction := completeAllSelections(t, testGame, selectAction, playerID1, playerID2, true)

	testutil.AssertEqual(t, shared.GamePhaseInitApplyCorp, testGame.CurrentPhase(), "Should be in init_apply_corp")

	// Apply + advance through both corps (4 confirms: apply p1, advance, apply p2, advance)
	err := confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Apply player 1 corp")

	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Advance to player 2")

	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Apply player 2 corp")

	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Advance past corp phase")

	// Should now be in init_apply_prelude
	testutil.AssertEqual(t, shared.GamePhaseInitApplyPrelude, testGame.CurrentPhase(), "Should be in init_apply_prelude")
	testutil.AssertTrue(t, testGame.InitPhaseWaitingForConfirm(), "Should be waiting for confirm in prelude phase")

	// Preludes are applied one confirm at a time, then a confirm advances
	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Apply player 1 first prelude")

	p1, _ := testGame.GetPlayer(playerID1)
	testutil.AssertTrue(t, p1.PlayedCards().Contains("P01"), "P01 should be in played cards after first apply")
	testutil.AssertFalse(t, p1.PlayedCards().Contains("P03"), "P03 waits for its own confirm")

	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Apply player 1 second prelude")
	testutil.AssertTrue(t, p1.PlayedCards().Contains("P03"), "P03 should be in played cards after second apply")

	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Advance to player 2 preludes")

	for i := 0; i < 2; i++ {
		err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
		testutil.AssertNoError(t, err, "Apply player 2 prelude")
	}

	p2, _ := testGame.GetPlayer(playerID2)
	testutil.AssertTrue(t, p2.PlayedCards().Contains("P04"), "P04 should be in played cards")
	testutil.AssertTrue(t, p2.PlayedCards().Contains("P07"), "P07 should be in played cards")

	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Advance to roster")
	testutil.AssertTrue(t, testGame.InitPhaseRoster(), "Should show roster after preludes")

	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Advance to action phase")

	testutil.AssertEqual(t, shared.GamePhaseAction, testGame.CurrentPhase(), "Should be in action phase")
	testutil.AssertTrue(t, testGame.CurrentTurn() != nil, "Current turn should be set")
}

func TestInitPhase_PreludeSkippedWhenDisabled(t *testing.T) {
	testGame, selectAction, playerID1, playerID2 := setupStartingSelectionGame(t, false)
	ctx := context.Background()

	confirmAction := completeAllSelections(t, testGame, selectAction, playerID1, playerID2, false)

	testutil.AssertEqual(t, shared.GamePhaseInitApplyCorp, testGame.CurrentPhase(), "Should be in init_apply_corp")

	// Apply + advance through both corps (4 confirms)
	err := confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Apply player 1 corp")

	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Advance to player 2")

	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Apply player 2 corp")

	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Advance past corp phase")
	testutil.AssertTrue(t, testGame.InitPhaseRoster(), "Should show roster after corps")

	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Finish roster")

	// Should skip prelude phase entirely and go to action
	testutil.AssertEqual(t, shared.GamePhaseAction, testGame.CurrentPhase(), "Should skip to action phase when no prelude")
}

func TestInitPhase_RejectConfirmWrongPhase(t *testing.T) {
	testGame, _, _, _ := setupStartingSelectionGame(t, false)
	ctx := context.Background()
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()

	confirmAction := turnAction.NewConfirmInitAdvanceAction(
		gameRepoWithGame(t, testGame),
		cardRegistry,
		nil,
		nil,
		logger,
	)

	// Game is still in starting_selection
	err := confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertErrorContains(t, err, "game not in init apply phase", "Should reject confirm when not in init phase")
}

func TestInitPhase_RejectConfirmWhenNotWaiting(t *testing.T) {
	testGame, selectAction, playerID1, playerID2 := setupStartingSelectionGame(t, false)
	ctx := context.Background()

	confirmAction := completeAllSelections(t, testGame, selectAction, playerID1, playerID2, false)

	// Game is now in init_apply_corp and waiting for confirm
	testutil.AssertTrue(t, testGame.InitPhaseWaitingForConfirm(), "Should be waiting")

	// Manually clear waiting flag
	testutil.AssertNoError(t, testGame.SetInitPhaseWaitingForConfirm(ctx, false), "clear waiting flag")

	err := confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertErrorContains(t, err, "init phase not waiting for confirmation", "Should reject confirm when not waiting")
}

func TestInitPhase_LastPlayerForcedTilePlacement(t *testing.T) {
	// Set up game where player 2 (last in turn order) has Tharsis Republic (B08)
	// which requires a forced city tile placement
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	cardRegistry := testutil.CreateTestCardRegistry()
	stateRepo := game.NewInMemoryGameStateRepository()
	ctx := context.Background()

	testGame.UpdateSettings(ctx, shared.GameSettings{
		MaxPlayers: 4,
		GameModeID: "classic", Features: []string{},
	})

	err := testGame.UpdateStatus(ctx, shared.GameStatusActive)
	testutil.AssertNoError(t, err, "Failed to set active status")
	err = testGame.UpdatePhase(ctx, shared.GamePhaseStartingSelection)
	testutil.AssertNoError(t, err, "Failed to set starting selection phase")

	players := testGame.GetAllPlayers()
	playerID1, playerID2 := players[0].ID(), players[1].ID()
	err = testGame.SetTurnOrder(ctx, []string{playerID1, playerID2})
	testutil.AssertNoError(t, err, "Failed to set turn order")

	for _, p := range players {
		testutil.SetPlayerCredits(ctx, p, 100)
	}

	// Player 1 gets a corp without forced first action (B01)
	err = testGame.SetSelectCorporationPhase(ctx, playerID1, &shared.SelectCorporationPhase{
		AvailableCorporations: []string{"B01"},
	})
	testutil.AssertNoError(t, err, "Failed to set corp phase for player 1")

	// Player 2 (last) gets Tharsis Republic (B08) which has a forced city placement
	err = testGame.SetSelectCorporationPhase(ctx, playerID2, &shared.SelectCorporationPhase{
		AvailableCorporations: []string{"B08"},
	})
	testutil.AssertNoError(t, err, "Failed to set corp phase for player 2")

	deck := testGame.Deck()
	for _, p := range players {
		projectCards, drawErr := deck.DrawProjectCards(ctx, 10)
		testutil.AssertNoError(t, drawErr, "Failed to draw project cards")
		err = testGame.SetSelectStartingCardsPhase(ctx, p.ID(), &shared.SelectStartingCardsPhase{
			AvailableCards: projectCards,
		})
		testutil.AssertNoError(t, err, "Failed to set starting cards phase")
	}

	selectAction := turnAction.NewSelectStartingChoicesAction(repo, cardRegistry, nil, logger)

	// Both players select their starting choices
	err = selectAction.Execute(ctx, testGame.ID(), playerID1, "B01", []string{}, []string{}, shared.NativePayment(shared.ResourceCredit, len([]string{})*3))
	testutil.AssertNoError(t, err, "Player 1 selection")

	err = selectAction.Execute(ctx, testGame.ID(), playerID2, "B08", []string{}, []string{}, shared.NativePayment(shared.ResourceCredit, len([]string{})*3))
	testutil.AssertNoError(t, err, "Player 2 selection")

	testutil.AssertEqual(t, shared.GamePhaseInitApplyCorp, testGame.CurrentPhase(), "Should be in init_apply_corp")

	confirmAction := turnAction.NewConfirmInitAdvanceAction(
		gameRepoWithGame(t, testGame),
		cardRegistry,
		nil,
		nil,
		logger,
	)

	// Confirm 1: apply player 1's corp (no forced action)
	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Apply player 1 corp")

	// Confirm 2: advance to player 2
	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Advance to player 2")

	// Confirm 3: apply player 2's corp (Tharsis Republic - has forced city placement)
	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Apply player 2 corp (Tharsis Republic)")

	testutil.AssertTrue(t, testGame.GetPendingTileSelection(playerID2) == nil, "first action must wait until player's turn")
	testutil.AssertEqual(t, "queued", testGame.GetForcedFirstAction(playerID2).State, "first action queued during setup")
	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "queued first action must not block setup")
	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "finish roster")
	testutil.AssertEqual(t, shared.GamePhaseAction, testGame.CurrentPhase(), "action phase starts")
	testutil.AssertTrue(t, testGame.GetPendingTileSelection(playerID2) == nil, "second player waits for own turn")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, playerID2, 2), "start second player's turn")
	pendingTile := testGame.GetPendingTileSelection(playerID2)
	testutil.AssertTrue(t, pendingTile != nil, "first turn creates city selection")
	selectTile := tileAction.NewSelectTileAction(gameRepoWithGame(t, testGame), cardRegistry, stateRepo, logger)
	_, err = selectTile.Execute(ctx, testGame.ID(), playerID2, pendingTile.AvailableHexes[0])
	testutil.AssertNoError(t, err, "place first-action city")
	testutil.AssertTrue(t, testGame.GetForcedFirstAction(playerID2) == nil, "first action completed")
	testutil.AssertEqual(t, 1, testGame.CurrentTurn().ActionsRemaining(), "one normal action remains")

}

func initPhaseView(t *testing.T, g *game.Game, viewerID string) *dto.InitPhaseDto {
	t.Helper()
	return dto.ToGameDto(g, testutil.CreateTestCardRegistry(), viewerID).InitPhase
}

func assertInitStage(t *testing.T, g *game.Game, viewerID, playerID string, stage dto.InitPhaseStage) *dto.InitPhaseDto {
	t.Helper()
	view := initPhaseView(t, g, viewerID)
	testutil.AssertTrue(t, view != nil, "init phase view should exist")
	testutil.AssertEqual(t, stage, view.Stage, "init stage")
	if playerID != "" {
		testutil.AssertEqual(t, playerID, view.CurrentPlayerID, "current init player")
	}
	return view
}

func TestInitPhase_ShowcaseStagesWithPreludes(t *testing.T) {
	testGame, selectAction, playerID1, playerID2 := setupStartingSelectionGame(t, true)
	ctx := context.Background()

	confirmAction := completeAllSelections(t, testGame, selectAction, playerID1, playerID2, true)
	confirm := func(label string) {
		t.Helper()
		testutil.AssertNoError(t, confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), label)
	}

	view := assertInitStage(t, testGame, playerID2, playerID1, dto.InitPhaseStageReveal)
	testutil.AssertEqual(t, 0, len(view.Preludes), "no preludes in corp phase")
	testutil.AssertFalse(t, view.HasPendingSelection, "no pending selection")

	confirm("apply p1 corp")
	assertInitStage(t, testGame, playerID2, playerID1, dto.InitPhaseStageApplied)
	confirm("advance to p2 corp")
	assertInitStage(t, testGame, playerID2, playerID2, dto.InitPhaseStageReveal)
	confirm("apply p2 corp")
	assertInitStage(t, testGame, playerID2, playerID2, dto.InitPhaseStageApplied)
	confirm("advance to preludes")

	testutil.AssertEqual(t, shared.GamePhaseInitApplyPrelude, testGame.CurrentPhase(), "prelude phase")
	view = assertInitStage(t, testGame, playerID2, playerID1, dto.InitPhaseStageReveal)
	testutil.AssertEqual(t, 2, len(view.Preludes), "p1 preludes visible before apply")
	testutil.AssertEqual(t, "P01", view.Preludes[0].ID, "first prelude")
	testutil.AssertEqual(t, "P03", view.Preludes[1].ID, "second prelude")

	testutil.AssertEqual(t, 0, view.PreludesPlayed, "nothing played yet")

	confirm("apply p1 first prelude")
	view = assertInitStage(t, testGame, playerID2, playerID1, dto.InitPhaseStageApplied)
	testutil.AssertEqual(t, 1, view.PreludesPlayed, "first prelude played")
	confirm("apply p1 second prelude")
	view = assertInitStage(t, testGame, playerID2, playerID1, dto.InitPhaseStageApplied)
	testutil.AssertEqual(t, 2, view.PreludesPlayed, "both preludes played")
	confirm("advance to p2 preludes")
	view = assertInitStage(t, testGame, playerID1, playerID2, dto.InitPhaseStageReveal)
	testutil.AssertEqual(t, "P04", view.Preludes[0].ID, "p2 first prelude")
	testutil.AssertEqual(t, 0, view.PreludesPlayed, "p2 nothing played yet")
	confirm("apply p2 first prelude")
	confirm("apply p2 second prelude")
	assertInitStage(t, testGame, playerID1, playerID2, dto.InitPhaseStageApplied)
	confirm("advance to roster")

	assertInitStage(t, testGame, playerID1, "", dto.InitPhaseStageRoster)
	testutil.AssertTrue(t, testGame.CurrentTurn() == nil, "no turn during roster")

	confirm("finish roster")
	testutil.AssertEqual(t, shared.GamePhaseAction, testGame.CurrentPhase(), "action phase")
	testutil.AssertTrue(t, initPhaseView(t, testGame, playerID1) == nil, "no init view in action phase")
}

func TestInitPhase_ShowcaseRosterFollowsCorpsWithoutPrelude(t *testing.T) {
	testGame, selectAction, playerID1, playerID2 := setupStartingSelectionGame(t, false)
	ctx := context.Background()

	confirmAction := completeAllSelections(t, testGame, selectAction, playerID1, playerID2, false)
	for i := 0; i < 4; i++ {
		testutil.AssertNoError(t, confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), "corp confirm")
	}

	assertInitStage(t, testGame, playerID1, "", dto.InitPhaseStageRoster)
	testutil.AssertEqual(t, shared.GamePhaseInitApplyCorp, testGame.CurrentPhase(), "still in corp phase")

	testutil.AssertNoError(t, confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), "finish roster")
	testutil.AssertEqual(t, shared.GamePhaseAction, testGame.CurrentPhase(), "action phase")
}

func TestInitPhase_ShowcasePreludeTileDocksAndBlocksAdvance(t *testing.T) {
	testGame, selectAction, playerID1, playerID2 := setupStartingSelectionGame(t, true)
	ctx := context.Background()
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()
	stateRepo := game.NewInMemoryGameStateRepository()

	// Great Aquifer (P14) places two oceans.
	confirmAction := completeSelectionsWithPreludes(t, testGame, selectAction, playerID1, playerID2,
		[]string{"P14", "P01"}, []string{"P04", "P07"})
	for i := 0; i < 4; i++ {
		testutil.AssertNoError(t, confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), "corp confirm")
	}
	testutil.AssertEqual(t, shared.GamePhaseInitApplyPrelude, testGame.CurrentPhase(), "prelude phase")

	testutil.AssertNoError(t, confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), "apply Great Aquifer")

	p1, _ := testGame.GetPlayer(playerID1)
	view := assertInitStage(t, testGame, playerID2, playerID1, dto.InitPhaseStageApplied)
	testutil.AssertEqual(t, 1, view.PreludesPlayed, "only Great Aquifer played")
	testutil.AssertFalse(t, p1.PlayedCards().Contains("P01"), "second prelude waits")
	testutil.AssertTrue(t, view.HasPendingSelection, "ocean placement is pending")
	testutil.AssertEqual(t, "P14", view.PendingSourceCardID, "pending choice comes from Great Aquifer")

	testutil.AssertErrorContains(t, confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), "current player has pending selection", "confirm rejected while placing")

	selectTile := tileAction.NewSelectTileAction(gameRepoWithGame(t, testGame), cardRegistry, stateRepo, logger)
	for i := 0; i < 2; i++ {
		pending := testGame.GetPendingTileSelection(playerID1)
		testutil.AssertTrue(t, pending != nil, "ocean selection pending")
		_, err := selectTile.Execute(ctx, testGame.ID(), playerID1, pending.AvailableHexes[0])
		testutil.AssertNoError(t, err, "place ocean")
		if i == 0 {
			view = assertInitStage(t, testGame, playerID2, playerID1, dto.InitPhaseStageApplied)
			testutil.AssertTrue(t, view.HasPendingSelection, "still pending between the two oceans")
		}
	}

	view = assertInitStage(t, testGame, playerID2, playerID1, dto.InitPhaseStageApplied)
	testutil.AssertFalse(t, view.HasPendingSelection, "placement resolved")
	testutil.AssertEqual(t, "", view.PendingSourceCardID, "no pending source")

	testutil.AssertNoError(t, confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), "apply second prelude")
	view = assertInitStage(t, testGame, playerID2, playerID1, dto.InitPhaseStageApplied)
	testutil.AssertEqual(t, 2, view.PreludesPlayed, "second prelude played")
	testutil.AssertTrue(t, p1.PlayedCards().Contains("P01"), "P01 played")

	testutil.AssertNoError(t, confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), "advance to p2 preludes")
	assertInitStage(t, testGame, playerID2, playerID2, dto.InitPhaseStageReveal)
}

func TestInitPhase_PreludeTilesDoNotOverwriteEachOther(t *testing.T) {
	testGame, selectAction, playerID1, playerID2 := setupStartingSelectionGame(t, true)
	ctx := context.Background()
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()
	stateRepo := game.NewInMemoryGameStateRepository()

	// Early Settlement (P09) builds a city, Great Aquifer (P14) places two oceans.
	confirmAction := completeSelectionsWithPreludes(t, testGame, selectAction, playerID1, playerID2,
		[]string{"P09", "P14"}, []string{"P04", "P07"})
	for i := 0; i < 4; i++ {
		testutil.AssertNoError(t, confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), "corp confirm")
	}

	testutil.AssertNoError(t, confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), "apply Early Settlement")
	city := testGame.GetPendingTileSelection(playerID1)
	testutil.AssertTrue(t, city != nil, "city placement pending")
	testutil.AssertEqual(t, "city", city.TileType, "city tile first")
	testutil.AssertErrorContains(t, confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), "current player has pending selection", "Great Aquifer waits for the city")

	selectTile := tileAction.NewSelectTileAction(gameRepoWithGame(t, testGame), cardRegistry, stateRepo, logger)
	_, err := selectTile.Execute(ctx, testGame.ID(), playerID1, city.AvailableHexes[0])
	testutil.AssertNoError(t, err, "place city")

	testutil.AssertNoError(t, confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), "apply Great Aquifer")
	ocean := testGame.GetPendingTileSelection(playerID1)
	testutil.AssertTrue(t, ocean != nil, "ocean placement pending")
	testutil.AssertEqual(t, "P14", ocean.SourceCardID, "ocean comes from Great Aquifer")

	cities := 0
	for _, tile := range testGame.Board().Tiles() {
		if tile.OccupiedBy != nil && tile.OccupiedBy.Type == shared.ResourceCityTile && tile.OwnerID != nil && *tile.OwnerID == playerID1 {
			cities++
		}
	}
	testutil.AssertEqual(t, 1, cities, "Early Settlement city was placed")
}

func TestInitPhase_PreludeCardSelectionBlocksNextPrelude(t *testing.T) {
	testGame, selectAction, playerID1, playerID2 := setupStartingSelectionGame(t, true)
	ctx := context.Background()
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()

	// Corporate Archives (X39) opens a card selection before P01 may be played.
	confirmAction := completeSelectionsWithPreludes(t, testGame, selectAction, playerID1, playerID2,
		[]string{"X39", "P01"}, []string{"P04", "P07"})
	for i := 0; i < 4; i++ {
		testutil.AssertNoError(t, confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), "corp confirm")
	}

	testutil.AssertNoError(t, confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), "apply Corporate Archives")
	p1, _ := testGame.GetPlayer(playerID1)
	selection := p1.Selection().GetPendingCardDrawSelection()
	testutil.AssertTrue(t, selection != nil, "card selection pending")

	view := assertInitStage(t, testGame, playerID2, playerID1, dto.InitPhaseStageApplied)
	testutil.AssertTrue(t, view.HasPendingSelection, "selection reported as pending")
	testutil.AssertErrorContains(t, confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), "current player has pending selection", "next prelude waits")

	confirmDraw := confirmationAction.NewConfirmCardDrawAction(gameRepoWithGame(t, testGame), cardRegistry, logger)
	err := confirmDraw.Execute(ctx, testGame.ID(), playerID1, selection.AvailableCards[:selection.FreeTakeCount], []string{}, shared.Payment{})
	testutil.AssertNoError(t, err, "take cards")

	testutil.AssertNoError(t, confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), "apply P01")
	testutil.AssertTrue(t, p1.PlayedCards().Contains("P01"), "P01 played after selection")
}

func TestInitPhase_OnlyHostAdvancesUnlessHostDisconnected(t *testing.T) {
	testGame, selectAction, playerID1, playerID2 := setupStartingSelectionGame(t, false)
	ctx := context.Background()

	confirmAction := completeAllSelections(t, testGame, selectAction, playerID1, playerID2, false)
	hostID := testGame.HostPlayerID()
	guestID := playerID2
	if guestID == hostID {
		guestID = playerID1
	}

	testutil.AssertErrorContains(t, confirmAction.Execute(ctx, testGame.ID(), guestID), "only the host can advance the showcase", "guest cannot advance while host is connected")
	testutil.AssertFalse(t, testGame.GetDeferredStartingChoices(testGame.TurnOrder()[0]).CorpApplied, "nothing applied")

	host, err := testGame.GetPlayer(hostID)
	testutil.AssertNoError(t, err, "get host")
	host.SetConnected(false)

	testutil.AssertNoError(t, confirmAction.Execute(ctx, testGame.ID(), guestID), "guest advances while host is disconnected")
	testutil.AssertTrue(t, testGame.GetDeferredStartingChoices(testGame.TurnOrder()[0]).CorpApplied, "first corp applied")
}
