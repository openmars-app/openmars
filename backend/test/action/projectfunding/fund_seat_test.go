package projectfunding_test

import (
	"context"
	"testing"

	confirmAction "openmars/internal/action/confirmation"
	pfAction "openmars/internal/action/projectfunding"
	"openmars/internal/game"
	pf "openmars/internal/game/projectfunding"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func setupProjectFundingGame(t *testing.T) (*game.Game, game.GameRepository, pf.ProjectFundingRegistry, string, string) {
	t.Helper()
	testGame, repo, _, player1, player2 := testutil.SetupTwoPlayerGame(t)

	pfDefs, err := pf.LoadProjectsFromJSON("../../../assets/project_funding.json")
	if err != nil {
		t.Fatalf("Failed to load project funding: %v", err)
	}
	pfRegistry := pf.NewInMemoryProjectFundingRegistry(pfDefs)

	settings := testGame.Settings()
	settings.Features = append(settings.Features, "project-funding")
	testGame.UpdateSettings(context.Background(), settings)

	return testGame, repo, pfRegistry, player1, player2
}

func setupProjectState(g *game.Game, projectID string, seatOwners []string) {
	states := g.ProjectFundingStates()
	states = append(states, &pf.ProjectState{
		DefinitionID: projectID,
		SeatOwners:   seatOwners,
		IsCompleted:  false,
	})
	g.SetProjectFundingStates(states)
}

func newAction(repo game.GameRepository, pfReg pf.ProjectFundingRegistry) *pfAction.FundSeatAction {
	stateRepo := game.NewInMemoryGameStateRepository()
	return pfAction.NewFundSeatAction(repo, pfReg, stateRepo)
}

// --- Validation Tests ---

func TestFundSeat_ExpansionNotEnabled_Fails(t *testing.T) {
	testGame, repo, _, player1, _ := testutil.SetupTwoPlayerGame(t)
	ctx := context.Background()

	pfDefs, _ := pf.LoadProjectsFromJSON("../../../assets/project_funding.json")
	pfRegistry := pf.NewInMemoryProjectFundingRegistry(pfDefs)

	setupProjectState(testGame, "pf_orbital_station", nil)

	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: 6})
	testutil.AssertErrorContains(t, err, "project funding expansion is not enabled", "Should fail when project funding expansion is not enabled")
}

func TestFundSeat_WrongPhase_Fails(t *testing.T) {
	testGame, repo, pfRegistry, player1, _ := setupProjectFundingGame(t)
	ctx := context.Background()

	setupProjectState(testGame, "pf_orbital_station", nil)
	if err := testGame.UpdatePhase(ctx, shared.GamePhaseProductionAndCardDraw); err != nil {
		t.Fatalf("Failed to update phase: %v", err)
	}

	p1, _ := testGame.GetPlayer(player1)
	testutil.SetPlayerCredits(ctx, p1, 100)

	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: 6})
	testutil.AssertErrorContains(t, err, "game not in action phase", "Should fail when not in action phase")
	testutil.AssertEqual(t, 100, p1.Resources().Get().Credits, "credits unchanged")
}

func TestFundSeat_NotCurrentTurn_Fails(t *testing.T) {
	testGame, repo, pfRegistry, _, player2 := setupProjectFundingGame(t)
	ctx := context.Background()

	setupProjectState(testGame, "pf_orbital_station", nil)

	p2, _ := testGame.GetPlayer(player2)
	testutil.SetPlayerCredits(ctx, p2, 100)

	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player2, "pf_orbital_station", pfAction.FundSeatPayment{Credits: 6})
	testutil.AssertErrorContains(t, err, "not your turn", "Should fail when not player's turn")
	testutil.AssertEqual(t, 100, p2.Resources().Get().Credits, "credits unchanged")
}

func TestFundSeat_InvalidProjectID_Fails(t *testing.T) {
	testGame, repo, pfRegistry, player1, _ := setupProjectFundingGame(t)
	ctx := context.Background()

	p1, _ := testGame.GetPlayer(player1)
	testutil.SetPlayerCredits(ctx, p1, 100)

	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "nonexistent_project", pfAction.FundSeatPayment{Credits: 6})
	testutil.AssertErrorContains(t, err, "project not found", "Should fail with invalid project ID")
}

func TestFundSeat_ProjectAlreadyCompleted_Fails(t *testing.T) {
	testGame, repo, pfRegistry, player1, _ := setupProjectFundingGame(t)
	ctx := context.Background()

	states := testGame.ProjectFundingStates()
	states = append(states, &pf.ProjectState{
		DefinitionID: "pf_orbital_station",
		SeatOwners:   []string{"other1", "other2"},
		IsCompleted:  true,
	})
	testGame.SetProjectFundingStates(states)

	p1, _ := testGame.GetPlayer(player1)
	testutil.SetPlayerCredits(ctx, p1, 100)

	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: 6})
	testutil.AssertErrorContains(t, err, "project is already completed", "Should fail when project is completed")
	testutil.AssertEqual(t, 100, p1.Resources().Get().Credits, "credits unchanged")
}

// --- Basic Purchase Tests ---

func TestFundSeat_BasicPurchase_Success(t *testing.T) {
	testGame, repo, pfRegistry, player1, _ := setupProjectFundingGame(t)
	ctx := context.Background()

	setupProjectState(testGame, "pf_orbital_station", nil)

	p1, _ := testGame.GetPlayer(player1)
	testutil.SetPlayerCredits(ctx, p1, 100)

	cost := pf.ScaledSeatCost(6, 2)
	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: cost})
	testutil.AssertNoError(t, err, "Buy first seat should succeed")

	state := testGame.GetProjectFundingState("pf_orbital_station")
	testutil.AssertEqual(t, 1, len(state.SeatOwners), "Should have 1 seat owner")
	testutil.AssertEqual(t, player1, state.SeatOwners[0], "Owner should be player1")
	testutil.AssertEqual(t, 100-cost, testutil.GetPlayerCredits(p1), "Should deduct the two-player seat cost")
}

func TestFundSeat_Overpayment_ChargesOnlyTheCost(t *testing.T) {
	testGame, repo, pfRegistry, player1, _ := setupProjectFundingGame(t)
	ctx := context.Background()

	setupProjectState(testGame, "pf_orbital_station", nil)

	p1, _ := testGame.GetPlayer(player1)
	testutil.SetPlayerCredits(ctx, p1, 100)

	cost := pf.ScaledSeatCost(6, 2)
	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: cost + 40})
	testutil.AssertNoError(t, err, "Overpaying for a seat should succeed")
	testutil.AssertEqual(t, 100-cost, testutil.GetPlayerCredits(p1), "Only the seat cost should be charged")
}

func TestFundSeat_InsufficientCredits_Fails(t *testing.T) {
	testGame, repo, pfRegistry, player1, _ := setupProjectFundingGame(t)
	ctx := context.Background()

	setupProjectState(testGame, "pf_orbital_station", nil)

	cost := pf.ScaledSeatCost(6, 2)
	p1, _ := testGame.GetPlayer(player1)
	testutil.SetPlayerCredits(ctx, p1, cost-1)

	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: cost})
	testutil.AssertErrorContains(t, err, "insufficient credits", "Should fail with insufficient credits")
	testutil.AssertEqual(t, cost-1, testutil.GetPlayerCredits(p1), "A rejected purchase should charge nothing")
}

func TestFundSeat_SecondSeat_HigherCost(t *testing.T) {
	testGame, repo, pfRegistry, player1, _ := setupProjectFundingGame(t)
	ctx := context.Background()

	setupProjectState(testGame, "pf_orbital_station", []string{"other"})

	p1, _ := testGame.GetPlayer(player1)
	testutil.SetPlayerCredits(ctx, p1, 100)

	cost := pf.ScaledSeatCost(8, 2)
	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: cost})
	testutil.AssertNoError(t, err, "Buy second seat should succeed")

	state := testGame.GetProjectFundingState("pf_orbital_station")
	testutil.AssertEqual(t, 2, len(state.SeatOwners), "Should have 2 seat owners")
	testutil.AssertEqual(t, 100-cost, testutil.GetPlayerCredits(p1), "The second seat costs more than the first")
}

// --- Payment Substitute Tests ---

func TestFundSeat_PayWithSteel_InvalidSeat_Fails(t *testing.T) {
	testGame, repo, pfRegistry, player1, _ := setupProjectFundingGame(t)
	ctx := context.Background()

	// Orbital station first seat has no steel substitution
	setupProjectState(testGame, "pf_orbital_station", nil)

	p1, _ := testGame.GetPlayer(player1)
	testutil.SetPlayerCredits(ctx, p1, 100)
	p1.Resources().Add(map[shared.ResourceType]int{shared.ResourceSteel: 10})

	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: 0, Steel: 3})
	testutil.AssertErrorContains(t, err, "steel cannot be used to pay for this seat", "Should fail when steel not allowed for this seat")
	testutil.AssertEqual(t, 100, p1.Resources().Get().Credits, "credits unchanged")
}

// --- Multi-player Seat Ownership Tests ---

func TestFundSeat_SamePlayerMultipleSeats(t *testing.T) {
	testGame, repo, pfRegistry, player1, _ := setupProjectFundingGame(t)
	ctx := context.Background()

	setupProjectState(testGame, "pf_orbital_station", nil)

	p1, _ := testGame.GetPlayer(player1)
	testutil.SetPlayerCredits(ctx, p1, 500)

	action := newAction(repo, pfRegistry)

	err := action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: 6})
	testutil.AssertNoError(t, err, "Buy first seat should succeed")
	if err := testGame.SetCurrentTurn(ctx, player1, 2); err != nil {
		t.Fatalf("Failed to set current turn: %v", err)
	}

	err = action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: 8})
	testutil.AssertNoError(t, err, "Buy second seat should succeed")
	if err := testGame.SetCurrentTurn(ctx, player1, 2); err != nil {
		t.Fatalf("Failed to set current turn: %v", err)
	}

	err = action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: 10})
	testutil.AssertNoError(t, err, "Buy third seat should succeed")

	state := testGame.GetProjectFundingState("pf_orbital_station")
	testutil.AssertEqual(t, 3, len(state.SeatOwners), "Should have 3 seat owners")
	for i := 0; i < 3; i++ {
		testutil.AssertEqual(t, player1, state.SeatOwners[i], "All seats should belong to player1")
	}
}

// --- Completion & Reward Tests ---

func TestFundSeat_Completion_SetsIsCompleted(t *testing.T) {
	testGame, repo, pfRegistry, player1, _ := setupProjectFundingGame(t)
	ctx := context.Background()

	def, _ := pfRegistry.GetByID("pf_orbital_station")
	scaledSeats := pf.ScaledSeats(def.Seats, len(testGame.GetAllPlayers()))

	owners := make([]string, len(scaledSeats)-1)
	for i := range owners {
		owners[i] = "other_player"
	}
	setupProjectState(testGame, "pf_orbital_station", owners)

	p1, _ := testGame.GetPlayer(player1)
	testutil.SetPlayerCredits(ctx, p1, 500)

	lastSeatCost := scaledSeats[len(scaledSeats)-1].Cost

	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: lastSeatCost})
	testutil.AssertNoError(t, err, "Buy last seat should succeed")

	state := testGame.GetProjectFundingState("pf_orbital_station")
	testutil.AssertTrue(t, state.IsCompleted, "Project should be completed")
}

func TestFundSeat_Completion_AllPlayersGetCompletionEffect(t *testing.T) {
	testGame, repo, pfRegistry, player1, player2 := setupProjectFundingGame(t)
	ctx := context.Background()

	def, _ := pfRegistry.GetByID("pf_orbital_station")
	scaledSeats := pf.ScaledSeats(def.Seats, len(testGame.GetAllPlayers()))

	owners := make([]string, len(scaledSeats)-1)
	for i := range owners {
		owners[i] = player1
	}
	setupProjectState(testGame, "pf_orbital_station", owners)

	p1, _ := testGame.GetPlayer(player1)
	p2, _ := testGame.GetPlayer(player2)
	testutil.SetPlayerCredits(ctx, p1, 500)

	p2HandBefore := len(p2.Hand().Cards())
	lastSeatCost := scaledSeats[len(scaledSeats)-1].Cost

	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: lastSeatCost})
	testutil.AssertNoError(t, err, "Buy last seat should succeed")

	// Player2 should receive the card-draw completion effect even though they have no seats
	p2HandAfter := len(p2.Hand().Cards())
	testutil.AssertTrue(t, p2HandAfter > p2HandBefore, "Player2 should receive cards from completion effect even without seats")
}

func TestFundSeat_NotCompleted_WhenSeatsRemain(t *testing.T) {
	testGame, repo, pfRegistry, player1, _ := setupProjectFundingGame(t)
	ctx := context.Background()

	setupProjectState(testGame, "pf_orbital_station", nil)

	p1, _ := testGame.GetPlayer(player1)
	testutil.SetPlayerCredits(ctx, p1, 100)

	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: 6})
	testutil.AssertNoError(t, err, "Buy first seat should succeed")

	state := testGame.GetProjectFundingState("pf_orbital_station")
	testutil.AssertFalse(t, state.IsCompleted, "Project should not be completed with seats remaining")
}

// --- Completion Effect Tests ---

func TestFundSeat_Completion_TerraformingHub_TR(t *testing.T) {
	testGame, repo, pfRegistry, player1, player2 := setupProjectFundingGame(t)
	ctx := context.Background()

	// pf_terraforming_hub: completion gives +2 TR to all players
	def, _ := pfRegistry.GetByID("pf_terraforming_hub")
	scaledSeats := pf.ScaledSeats(def.Seats, len(testGame.GetAllPlayers()))

	owners := make([]string, len(scaledSeats)-1)
	for i := range owners {
		if i%2 == 0 {
			owners[i] = player1
		} else {
			owners[i] = player2
		}
	}
	setupProjectState(testGame, "pf_terraforming_hub", owners)

	p1, _ := testGame.GetPlayer(player1)
	p2, _ := testGame.GetPlayer(player2)
	testutil.SetPlayerCredits(ctx, p1, 500)

	p1TRBefore := p1.Resources().TerraformRating()
	p2TRBefore := p2.Resources().TerraformRating()
	lastSeatCost := scaledSeats[len(scaledSeats)-1].Cost

	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_terraforming_hub", pfAction.FundSeatPayment{Credits: lastSeatCost})
	testutil.AssertNoError(t, err, "Buy last seat should succeed")

	testutil.AssertTrue(t, p1.Resources().TerraformRating() > p1TRBefore, "P1 should gain TR from completion")
	testutil.AssertTrue(t, p2.Resources().TerraformRating() > p2TRBefore, "P2 should gain TR from completion")
}

func TestFundSeat_Completion_ProductionChoice_SetForAllPlayers(t *testing.T) {
	testGame, repo, pfRegistry, player1, player2 := setupProjectFundingGame(t)
	ctx := context.Background()

	// pf_solar_forge: completion gives each player production choice
	def, _ := pfRegistry.GetByID("pf_solar_forge")
	scaledSeats := pf.ScaledSeats(def.Seats, len(testGame.GetAllPlayers()))

	owners := make([]string, len(scaledSeats)-1)
	for i := range owners {
		owners[i] = player1
	}
	setupProjectState(testGame, "pf_solar_forge", owners)

	p1, _ := testGame.GetPlayer(player1)
	p2, _ := testGame.GetPlayer(player2)
	testutil.SetPlayerCredits(ctx, p1, 500)

	lastSeatCost := scaledSeats[len(scaledSeats)-1].Cost

	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_solar_forge", pfAction.FundSeatPayment{Credits: lastSeatCost})
	testutil.AssertNoError(t, err, "Buy last seat should succeed")

	p1Choice := p1.Selection().GetPendingBehaviorResolutions()[0]
	p2Choice := p2.Selection().GetPendingBehaviorResolutions()[0]

	testutil.AssertTrue(t, p1Choice != nil, "P1 should have pending production choice")
	testutil.AssertTrue(t, p2Choice != nil, "P2 should have pending production choice")
	testutil.AssertEqual(t, 6, len(p1Choice.Choices), "Should have 6 production type choices")
	testutil.AssertEqual(t, "project-funding-completion", p1Choice.Source, "Source should be project-funding-completion")
	current, actions := testGame.CurrentTurn().PlayerID(), testGame.CurrentTurn().ActionsRemaining()
	confirm := confirmAction.NewConfirmBehaviorChoiceAction(repo, testutil.GetCardDB(), nil, testutil.TestLogger())
	testutil.AssertNoError(t, confirm.Execute(ctx, testGame.ID(), player2, p2Choice.ID, 0, nil), "resolve off-turn completion reward")
	testutil.AssertEqual(t, 0, len(p2.Selection().GetPendingBehaviorResolutions()), "off-turn decision removed")
	testutil.AssertEqual(t, 1, len(p1.Selection().GetPendingBehaviorResolutions()), "other player's decision retained")
	testutil.AssertEqual(t, current, testGame.CurrentTurn().PlayerID(), "off-turn reward does not change turn")
	testutil.AssertEqual(t, actions, testGame.CurrentTurn().ActionsRemaining(), "off-turn reward does not consume action")
	testutil.AssertErrorContains(t, confirm.Execute(ctx, testGame.ID(), player2, p2Choice.ID, 0, nil), "no pending behavior choice selection", "cannot replay completion reward")

}

func TestFundSeat_Completion_MassCardDraw(t *testing.T) {
	testGame, repo, pfRegistry, player1, player2 := setupProjectFundingGame(t)
	ctx := context.Background()

	// pf_orbital_station: completion deals 3 cards to each player
	def, _ := pfRegistry.GetByID("pf_orbital_station")
	scaledSeats := pf.ScaledSeats(def.Seats, len(testGame.GetAllPlayers()))

	owners := make([]string, len(scaledSeats)-1)
	for i := range owners {
		owners[i] = player1
	}
	setupProjectState(testGame, "pf_orbital_station", owners)

	p1, _ := testGame.GetPlayer(player1)
	p2, _ := testGame.GetPlayer(player2)
	testutil.SetPlayerCredits(ctx, p1, 500)

	p1HandBefore := len(p1.Hand().Cards())
	p2HandBefore := len(p2.Hand().Cards())
	lastSeatCost := scaledSeats[len(scaledSeats)-1].Cost

	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: lastSeatCost})
	testutil.AssertNoError(t, err, "Buy last seat should succeed")

	p1HandAfter := len(p1.Hand().Cards())
	p2HandAfter := len(p2.Hand().Cards())

	testutil.AssertEqual(t, p1HandBefore+3, p1HandAfter, "P1 should receive 3 cards")
	testutil.AssertEqual(t, p2HandBefore+3, p2HandAfter, "P2 should receive 3 cards")
}

// --- Player-Count Scaling Tests ---

func setupProjectFundingGameN(t *testing.T, numPlayers int) (*game.Game, game.GameRepository, pf.ProjectFundingRegistry, []string) {
	t.Helper()
	testGame, repo, _, players := testutil.SetupMultiPlayerGame(t, numPlayers)

	pfDefs, err := pf.LoadProjectsFromJSON("../../../assets/project_funding.json")
	if err != nil {
		t.Fatalf("Failed to load project funding: %v", err)
	}
	pfRegistry := pf.NewInMemoryProjectFundingRegistry(pfDefs)

	settings := testGame.Settings()
	settings.Features = append(settings.Features, "project-funding")
	testGame.UpdateSettings(context.Background(), settings)

	return testGame, repo, pfRegistry, players
}

func TestFundSeat_ScaledCost_TwoPlayers_DeductsScaledAmount(t *testing.T) {
	testGame, repo, pfRegistry, players := setupProjectFundingGameN(t, 2)
	ctx := context.Background()
	player1 := players[0]

	setupProjectState(testGame, "pf_orbital_station", nil)

	def, _ := pfRegistry.GetByID("pf_orbital_station")
	scaledCost := pf.ScaledSeatCost(def.Seats[0].Cost, 2) // 6 * 2 / 4 = 3

	p1, _ := testGame.GetPlayer(player1)
	testutil.SetPlayerCredits(ctx, p1, 100)

	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: scaledCost})
	testutil.AssertNoError(t, err, "Paying exactly the scaled cost should succeed")
	testutil.AssertEqual(t, 100-scaledCost, testutil.GetPlayerCredits(p1), "Should deduct only the scaled cost")
}

func TestFundSeat_ScaledCost_FivePlayers_RequiresMore(t *testing.T) {
	testGame, repo, pfRegistry, players := setupProjectFundingGameN(t, 5)
	ctx := context.Background()
	player1 := players[0]

	setupProjectState(testGame, "pf_orbital_station", nil)

	def, _ := pfRegistry.GetByID("pf_orbital_station")
	baseCost := def.Seats[0].Cost                // 6
	scaledCost := pf.ScaledSeatCost(baseCost, 5) // 6 * 5 / 4 = 7
	testutil.AssertTrue(t, scaledCost > baseCost, "5-player scaled cost should exceed JSON base")

	p1, _ := testGame.GetPlayer(player1)
	testutil.SetPlayerCredits(ctx, p1, baseCost) // exactly the JSON base, not enough

	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: scaledCost})
	testutil.AssertErrorContains(t, err, "insufficient credits", "Player with only base-cost credits should fail in a 5-player game")

	testutil.SetPlayerCredits(ctx, p1, scaledCost)
	err = action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: scaledCost})
	testutil.AssertNoError(t, err, "With scaled-cost credits the seat should be affordable")
}

func TestFundSeat_ScaledSeatCount_TwoPlayers_TruncatesProject(t *testing.T) {
	testGame, repo, pfRegistry, players := setupProjectFundingGameN(t, 2)
	ctx := context.Background()
	player1 := players[0]

	def, _ := pfRegistry.GetByID("pf_orbital_station")
	scaledCount := pf.ScaledSeatCount(len(def.Seats), 2) // 6 * 2 / 4 = 3
	testutil.AssertTrue(t, scaledCount < len(def.Seats), "2-player game should have fewer seats than JSON")

	owners := make([]string, scaledCount)
	for i := range owners {
		owners[i] = "other_player"
	}
	setupProjectState(testGame, "pf_orbital_station", owners)

	p1, _ := testGame.GetPlayer(player1)
	testutil.SetPlayerCredits(ctx, p1, 500)

	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: 50})
	testutil.AssertErrorContains(t, err, "all seats are filled", "Funding past the scaled seat count should fail with all-seats-filled")
}

func TestFundSeat_ScaledSeatCount_FivePlayers_ExtendsProject(t *testing.T) {
	testGame, repo, pfRegistry, players := setupProjectFundingGameN(t, 5)
	ctx := context.Background()
	player1 := players[0]

	def, _ := pfRegistry.GetByID("pf_orbital_station")
	scaledCount := pf.ScaledSeatCount(len(def.Seats), 5) // 6 * 5 / 4 = 8 (rounded)
	testutil.AssertTrue(t, scaledCount > len(def.Seats), "5-player game should have more seats than JSON")

	// Pre-fill exactly len(def.Seats) seats; the action should still succeed
	// because ScaledSeats grows by repeating the last entry.
	owners := make([]string, len(def.Seats))
	for i := range owners {
		owners[i] = "other_player"
	}
	setupProjectState(testGame, "pf_orbital_station", owners)

	p1, _ := testGame.GetPlayer(player1)
	testutil.SetPlayerCredits(ctx, p1, 500)

	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: 50})
	testutil.AssertNoError(t, err, "5-player game allows extra seats beyond JSON count")

	state := testGame.GetProjectFundingState("pf_orbital_station")
	testutil.AssertEqual(t, len(def.Seats)+1, len(state.SeatOwners), "Extra seat should be recorded")
}

func TestFundSeat_ScaledTiers_TwoPlayers_LowerThresholds(t *testing.T) {
	testGame, repo, pfRegistry, players := setupProjectFundingGameN(t, 2)
	ctx := context.Background()
	player1 := players[0]

	// pf_orbital_station tier-1 reward (baseline at 2 seats) gives 5 credits + 2 titanium.
	// At 2 players the threshold scales to 1 seat owned, so a single-seat funder qualifies.
	def, _ := pfRegistry.GetByID("pf_orbital_station")
	scaledSeats := pf.ScaledSeats(def.Seats, 2)

	owners := make([]string, len(scaledSeats)-1)
	for i := range owners {
		owners[i] = "other_player"
	}
	setupProjectState(testGame, "pf_orbital_station", owners)

	p1, _ := testGame.GetPlayer(player1)
	testutil.SetPlayerCredits(ctx, p1, 500)

	creditsBefore := testutil.GetPlayerCredits(p1)
	titaniumBefore := p1.Resources().Get().Titanium
	lastSeatCost := scaledSeats[len(scaledSeats)-1].Cost

	action := newAction(repo, pfRegistry)
	err := action.Execute(ctx, testGame.ID(), player1, "pf_orbital_station", pfAction.FundSeatPayment{Credits: lastSeatCost})
	testutil.AssertNoError(t, err, "Funding the last seat should succeed")

	// Player1 owns 1 seat. With baseline tiers (min 2) they'd get nothing.
	// With scaled tiers (threshold 1) they should get the first tier's rewards.
	expectedCredits := creditsBefore - lastSeatCost + 5
	testutil.AssertEqual(t, expectedCredits, testutil.GetPlayerCredits(p1),
		"Single-seat funder should receive scaled tier-1 credit reward")
	testutil.AssertEqual(t, titaniumBefore+2, p1.Resources().Get().Titanium,
		"Single-seat funder should receive scaled tier-1 titanium reward")
}
