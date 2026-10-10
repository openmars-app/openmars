package colony_test

import (
	"context"
	"fmt"
	baseaction "openmars/internal/action"
	confirmAction "openmars/internal/action/confirmation"
	"openmars/internal/action/turn_management"
	"openmars/internal/delivery/dto"
	"reflect"
	"testing"

	colonyAction "openmars/internal/action/colony"
	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/colony"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func firstColonyResourceFromQueue(p *player.Player) *shared.PendingColonyResourceSelection {
	queue := p.Selection().GetPendingColonyResourceQueue()
	if len(queue) == 0 {
		return nil
	}
	return &queue[0]
}

func setupColonyGame(t *testing.T) (*game.Game, game.GameRepository, colony.ColonyRegistry, string, string) {
	t.Helper()
	testGame, repo, cardRegistry, player1, player2 := testutil.SetupTwoPlayerGame(t)

	colonyDefs, err := colony.LoadColoniesFromJSON("../../../assets/colonies.json")
	if err != nil {
		t.Fatalf("Failed to load colonies: %v", err)
	}
	colonyRegistry := colony.NewInMemoryColonyRegistry(colonyDefs)

	settings := testGame.Settings()
	settings.Systems = append(settings.Systems, "colonies")
	testGame.UpdateSettings(context.Background(), settings)

	// Give both players energy for trading
	p1, _ := testGame.GetPlayer(player1)
	p2, _ := testGame.GetPlayer(player2)
	p1.Resources().Add(map[shared.ResourceType]int{shared.ResourceEnergy: 10})
	p2.Resources().Add(map[shared.ResourceType]int{shared.ResourceEnergy: 10})

	// Enable trade fleets
	testGame.Colonies().AddTradeFleets(player1, 1)
	testGame.Colonies().AddTradeFleets(player2, 1)

	_ = cardRegistry
	return testGame, repo, colonyRegistry, player1, player2
}

func setupColony(g *game.Game, colonyID string, markerPosition int, playerColonies []string) {
	states := g.Colonies().States()
	states = append(states, &colony.ColonyState{
		DefinitionID:   colonyID,
		MarkerPosition: markerPosition,
		PlayerColonies: playerColonies,
	})
	g.Colonies().SetStates(states)
}

func TestTrade_ImmediateResources_CreditsAdded(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()

	setupColony(testGame, "luna", 3, nil)

	p, _ := testGame.GetPlayer(playerID)
	creditsBefore := p.Resources().Get().Credits

	action := colonyAction.NewTradeAction(repo, colonyRegistry, testutil.CreateTestCardRegistry(), stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), playerID, "luna", colonyAction.TradePaymentEnergy, 0, testutil.TradePayment(colonyAction.TradePaymentEnergy))
	testutil.AssertNoError(t, err, "Trade with Luna should succeed")

	creditsAfter := p.Resources().Get().Credits
	// Luna step 3 gives 7 credits
	testutil.AssertEqual(t, creditsBefore+7, creditsAfter, "Should gain 7 credits from Luna trade at position 3")
}

func TestTrade_ColonyBonusGivenToOwners(t *testing.T) {
	testGame, repo, colonyRegistry, player1, player2 := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()

	// Player 2 has a colony on Luna
	setupColony(testGame, "luna", 3, []string{player2})

	p2, _ := testGame.GetPlayer(player2)
	creditsBefore := p2.Resources().Get().Credits

	action := colonyAction.NewTradeAction(repo, colonyRegistry, testutil.CreateTestCardRegistry(), stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), player1, "luna", colonyAction.TradePaymentEnergy, 0, testutil.TradePayment(colonyAction.TradePaymentEnergy))
	testutil.AssertNoError(t, err, "Trade with Luna should succeed")

	creditsAfter := p2.Resources().Get().Credits
	// Luna colony bonus is 2 credits
	testutil.AssertEqual(t, creditsBefore+2, creditsAfter, "Colony owner should gain 2 credits bonus")
}

func TestTrade_TraderWithColony_GetsBothIncomeAndBonus(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()

	// Trader has a colony on Luna, marker at position 3 (7 credits)
	setupColony(testGame, "luna", 3, []string{playerID})

	p, _ := testGame.GetPlayer(playerID)
	creditsBefore := p.Resources().Get().Credits

	action := colonyAction.NewTradeAction(repo, colonyRegistry, testutil.CreateTestCardRegistry(), stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), playerID, "luna", colonyAction.TradePaymentEnergy, 0, testutil.TradePayment(colonyAction.TradePaymentEnergy))
	testutil.AssertNoError(t, err, "Trade with Luna should succeed")

	creditsAfter := p.Resources().Get().Credits
	// Luna step 3 = 7 credits trade income + 2 credits colony bonus = 9 total
	testutil.AssertEqual(t, creditsBefore+9, creditsAfter, "Trader with colony should gain trade income + colony bonus")
}

func TestTrade_CardTargetedResources_CombinedWhenTraderHasColony(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()
	cardRegistry := testutil.CreateTestCardRegistry()

	// Trader has a colony on Titan, marker at position 6 (4 floaters)
	setupColony(testGame, "titan", 6, []string{playerID})

	// Give trader a card with floater storage
	p, _ := testGame.GetPlayer(playerID)
	aerialMappersID := testutil.CardID("Aerial Mappers")
	p.PlayedCards().AddCard(aerialMappersID, "Aerial Mappers", "active", []string{"venus"})

	action := colonyAction.NewTradeAction(repo, colonyRegistry, cardRegistry, stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), playerID, "titan", colonyAction.TradePaymentEnergy, 0, testutil.TradePayment(colonyAction.TradePaymentEnergy))
	testutil.AssertNoError(t, err, "Trade with Titan should succeed")

	// Should have a pending selection with combined amount: 4 (trade) + 1 (bonus) = 5
	selection := firstColonyResourceFromQueue(p)
	testutil.AssertTrue(t, selection != nil, "Should have pending colony resource selection")
	testutil.AssertEqual(t, 5, selection.Amount, "Pending floaters should be 4 (trade) + 1 (bonus) = 5")
	testutil.AssertEqual(t, "floater", selection.ResourceType, "Resource type should be floater")
	testutil.AssertEqual(t, "trade", selection.Reason, "Reason should be trade for the trader")
}

func TestTrade_CardTargetedResources_TradeOnlyWithoutColony(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()
	cardRegistry := testutil.CreateTestCardRegistry()

	// No colonies on Titan, marker at position 6 (4 floaters)
	setupColony(testGame, "titan", 6, nil)

	p, _ := testGame.GetPlayer(playerID)
	aerialMappersID := testutil.CardID("Aerial Mappers")
	p.PlayedCards().AddCard(aerialMappersID, "Aerial Mappers", "active", []string{"venus"})

	action := colonyAction.NewTradeAction(repo, colonyRegistry, cardRegistry, stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), playerID, "titan", colonyAction.TradePaymentEnergy, 0, testutil.TradePayment(colonyAction.TradePaymentEnergy))
	testutil.AssertNoError(t, err, "Trade with Titan should succeed")

	selection := firstColonyResourceFromQueue(p)
	testutil.AssertTrue(t, selection != nil, "Should have pending colony resource selection")
	testutil.AssertEqual(t, 4, selection.Amount, "Pending floaters should be 4 (trade only)")
	testutil.AssertEqual(t, "trade", selection.Reason, "Reason should be trade for the trader")
}

func TestTrade_ColonyBonusReason_SetToColonyTax(t *testing.T) {
	testGame, repo, colonyRegistry, player1, player2 := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()
	cardRegistry := testutil.CreateTestCardRegistry()

	// Player 2 has a colony on Titan, player 1 trades
	setupColony(testGame, "titan", 6, []string{player2})

	// Give player 2 a card with floater storage to receive the bonus
	p2, _ := testGame.GetPlayer(player2)
	aerialMappersID := testutil.CardID("Aerial Mappers")
	p2.PlayedCards().AddCard(aerialMappersID, "Aerial Mappers", "active", []string{"venus"})

	action := colonyAction.NewTradeAction(repo, colonyRegistry, cardRegistry, stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), player1, "titan", colonyAction.TradePaymentEnergy, 0, testutil.TradePayment(colonyAction.TradePaymentEnergy))
	testutil.AssertNoError(t, err, "Trade with Titan should succeed")

	selection := firstColonyResourceFromQueue(p2)
	testutil.AssertTrue(t, selection != nil, "Colony owner should have pending colony resource selection")
	testutil.AssertEqual(t, "colony-tax", selection.Reason, "Reason should be colony-tax for non-trader colony owner")
}

func TestTrade_MarkerAtZero_NoTradeIncome(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()

	// Ganymede at position 0 gives 0 plants
	setupColony(testGame, "ganymede", 0, nil)

	p, _ := testGame.GetPlayer(playerID)
	plantsBefore := p.Resources().Get().Plants

	action := colonyAction.NewTradeAction(repo, colonyRegistry, testutil.CreateTestCardRegistry(), stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), playerID, "ganymede", colonyAction.TradePaymentEnergy, 0, testutil.TradePayment(colonyAction.TradePaymentEnergy))
	testutil.AssertNoError(t, err, "Trade with Ganymede at position 0 should succeed")

	plantsAfter := p.Resources().Get().Plants
	testutil.AssertEqual(t, plantsBefore, plantsAfter, "Should gain 0 plants at position 0")
}

func TestTrade_ResetsMarkerPosition(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()

	// 2 colonies on Luna, marker at position 5
	setupColony(testGame, "luna", 5, []string{"other-1", "other-2"})

	action := colonyAction.NewTradeAction(repo, colonyRegistry, testutil.CreateTestCardRegistry(), stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), playerID, "luna", colonyAction.TradePaymentEnergy, 0, testutil.TradePayment(colonyAction.TradePaymentEnergy))
	testutil.AssertNoError(t, err, "Trade should succeed")

	tileState := testGame.Colonies().GetState("luna")
	// Marker resets to number of colonies
	testutil.AssertEqual(t, 2, tileState.MarkerPosition, "Marker should reset to number of colonies (2)")
	testutil.AssertTrue(t, tileState.TradedThisGen, "Colony should be marked as traded")
	testutil.AssertEqual(t, playerID, tileState.TraderID, "Trader ID should be set")
}

func TestTrade_AlreadyTraded_Fails(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()

	setupColony(testGame, "luna", 3, nil)
	tileState := testGame.Colonies().GetState("luna")
	tileState.TradedThisGen = true

	action := colonyAction.NewTradeAction(repo, colonyRegistry, testutil.CreateTestCardRegistry(), stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), playerID, "luna", colonyAction.TradePaymentEnergy, 0, testutil.TradePayment(colonyAction.TradePaymentEnergy))
	testutil.AssertErrorContains(t, err, "colony tile already traded this generation", "Should fail when colony already traded")
}

func TestTrade_InsufficientEnergy_Fails(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()

	setupColony(testGame, "luna", 3, nil)

	// Remove energy
	p, _ := testGame.GetPlayer(playerID)
	r := p.Resources().Get()
	r.Energy = 0
	p.Resources().Set(r)

	action := colonyAction.NewTradeAction(repo, colonyRegistry, testutil.CreateTestCardRegistry(), stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), playerID, "luna", colonyAction.TradePaymentEnergy, 0, testutil.TradePayment(colonyAction.TradePaymentEnergy))
	testutil.AssertErrorContains(t, err, "payment exceeds available source pool", "Should fail with insufficient energy")
	testutil.AssertTrue(t, !testGame.Colonies().GetState("luna").TradedThisGen, "colony not traded")
}

func TestTrade_NoTradeFleet_Fails(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()

	setupColony(testGame, "luna", 3, nil)
	if testGame.Colonies().TradeFleet(playerID).Available() > 0 {
		testutil.AssertNoError(t, testGame.Colonies().UseTradeFleet(playerID), "Use fleet")
	}

	action := colonyAction.NewTradeAction(repo, colonyRegistry, testutil.CreateTestCardRegistry(), stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), playerID, "luna", colonyAction.TradePaymentEnergy, 0, testutil.TradePayment(colonyAction.TradePaymentEnergy))
	testutil.AssertErrorContains(t, err, "trade fleet is not available", "Should fail without trade fleet")
}

func TestTrade_DeductsEnergyCost(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()

	setupColony(testGame, "luna", 3, nil)

	p, _ := testGame.GetPlayer(playerID)
	energyBefore := p.Resources().Get().Energy

	action := colonyAction.NewTradeAction(repo, colonyRegistry, testutil.CreateTestCardRegistry(), stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), playerID, "luna", colonyAction.TradePaymentEnergy, 0, testutil.TradePayment(colonyAction.TradePaymentEnergy))
	testutil.AssertNoError(t, err, "Trade should succeed")

	energyAfter := p.Resources().Get().Energy
	testutil.AssertEqual(t, energyBefore-3, energyAfter, "Should deduct 3 energy")
}

func TestTrade_ConsumesTradeFleet(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()

	setupColony(testGame, "luna", 3, nil)

	action := colonyAction.NewTradeAction(repo, colonyRegistry, testutil.CreateTestCardRegistry(), stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), playerID, "luna", colonyAction.TradePaymentEnergy, 0, testutil.TradePayment(colonyAction.TradePaymentEnergy))
	testutil.AssertNoError(t, err, "Trade should succeed")

	testutil.AssertFalse(t, testGame.Colonies().TradeFleet(playerID).Available() > 0, "Trade fleet should be consumed")
}

func TestTrade_MultipleColonyOwners_AllGetBonus(t *testing.T) {
	testGame, repo, colonyRegistry, player1, player2 := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()

	// Both players have colonies on Io (bonus: 2 heat each)
	setupColony(testGame, "io", 4, []string{player1, player2})

	p1, _ := testGame.GetPlayer(player1)
	p2, _ := testGame.GetPlayer(player2)
	p1Heat := p1.Resources().Get().Heat
	p2Heat := p2.Resources().Get().Heat

	action := colonyAction.NewTradeAction(repo, colonyRegistry, testutil.CreateTestCardRegistry(), stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), player1, "io", colonyAction.TradePaymentEnergy, 0, testutil.TradePayment(colonyAction.TradePaymentEnergy))
	testutil.AssertNoError(t, err, "Trade should succeed")

	// Io step 4 = 8 heat trade income, bonus = 2 heat per colony
	p1HeatAfter := p1.Resources().Get().Heat
	p2HeatAfter := p2.Resources().Get().Heat

	// Player 1 (trader + colony owner): 8 heat trade + 2 heat bonus = 10
	testutil.AssertEqual(t, p1Heat+10, p1HeatAfter, "Trader with colony should gain trade income + bonus")
	// Player 2 (colony owner only): 2 heat bonus
	testutil.AssertEqual(t, p2Heat+2, p2HeatAfter, "Non-trader colony owner should gain bonus only")
}

func TestTrade_PayWithCredits(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()

	setupColony(testGame, "luna", 3, nil)

	p, _ := testGame.GetPlayer(playerID)
	testutil.SetPlayerCredits(ctx, p, 20)
	creditsBefore := p.Resources().Get().Credits
	energyBefore := p.Resources().Get().Energy

	action := colonyAction.NewTradeAction(repo, colonyRegistry, testutil.CreateTestCardRegistry(), stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), playerID, "luna", colonyAction.TradePaymentCredits, 0, testutil.TradePayment(colonyAction.TradePaymentCredits))
	testutil.AssertNoError(t, err, "Trade with credits should succeed")

	testutil.AssertEqual(t, creditsBefore-9+7, p.Resources().Get().Credits, "Should deduct 9 credits cost and gain 7 from Luna")
	testutil.AssertEqual(t, energyBefore, p.Resources().Get().Energy, "Energy should be unchanged")
}

func TestTrade_PayWithTitanium(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()

	setupColony(testGame, "luna", 3, nil)

	p, _ := testGame.GetPlayer(playerID)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceTitanium: 5})
	titaniumBefore := p.Resources().Get().Titanium
	energyBefore := p.Resources().Get().Energy

	action := colonyAction.NewTradeAction(repo, colonyRegistry, testutil.CreateTestCardRegistry(), stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), playerID, "luna", colonyAction.TradePaymentTitanium, 0, testutil.TradePayment(colonyAction.TradePaymentTitanium))
	testutil.AssertNoError(t, err, "Trade with titanium should succeed")

	testutil.AssertEqual(t, titaniumBefore-3, p.Resources().Get().Titanium, "Should deduct 3 titanium")
	testutil.AssertEqual(t, energyBefore, p.Resources().Get().Energy, "Energy should be unchanged")
}

func TestTrade_InsufficientCredits_Fails(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()

	setupColony(testGame, "luna", 3, nil)

	p, _ := testGame.GetPlayer(playerID)
	testutil.SetPlayerCredits(ctx, p, 5)

	action := colonyAction.NewTradeAction(repo, colonyRegistry, testutil.CreateTestCardRegistry(), stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), playerID, "luna", colonyAction.TradePaymentCredits, 0, testutil.TradePayment(colonyAction.TradePaymentCredits))
	testutil.AssertErrorContains(t, err, "payment exceeds available source pool", "Should fail with insufficient credits")
	testutil.AssertEqual(t, 5, p.Resources().Get().Credits, "credits unchanged")
	testutil.AssertTrue(t, !testGame.Colonies().GetState("luna").TradedThisGen, "colony not traded")
}

func TestTrade_InsufficientTitanium_Fails(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()

	setupColony(testGame, "luna", 3, nil)

	action := colonyAction.NewTradeAction(repo, colonyRegistry, testutil.CreateTestCardRegistry(), stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), playerID, "luna", colonyAction.TradePaymentTitanium, 0, testutil.TradePayment(colonyAction.TradePaymentTitanium))
	testutil.AssertErrorContains(t, err, "payment exceeds available source pool", "Should fail with insufficient titanium")
}

func TestTrade_InvalidPaymentType_Fails(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColonyGame(t)
	ctx := context.Background()
	stateRepo := game.NewInMemoryGameStateRepository()
	logger := testutil.TestLogger()

	setupColony(testGame, "luna", 3, nil)

	action := colonyAction.NewTradeAction(repo, colonyRegistry, testutil.CreateTestCardRegistry(), stateRepo, logger)
	err := action.Execute(ctx, testGame.ID(), playerID, "luna", colonyAction.TradePaymentType("invalid"), 0, shared.NativePayment(map[string]shared.ResourceType{"credits": shared.ResourceCredit, "energy": shared.ResourceEnergy, "titanium": shared.ResourceTitanium}[string(colonyAction.TradePaymentType("invalid"))], map[string]int{"credits": 9, "energy": 3, "titanium": 3}[string(colonyAction.TradePaymentType("invalid"))]))
	testutil.AssertErrorContains(t, err, "invalid trade payment type", "Should fail with invalid payment type")
}

func addTradeModifierCards(p *player.Player, names ...string) {
	for _, name := range names {
		card := testutil.GetCardByName(name)
		p.PlayedCards().AddCard(card.ID, card.Name, string(card.Type), nil)
	}
}

func TestTrade_OptionalTrackChoicesAndPreview(t *testing.T) {
	for _, names := range [][]string{nil, {"Trade Envoys"}, {"Trading Colony"}, {"Trade Envoys", "Trading Colony"}} {
		for _, position := range []int{3, 5, 6} {
			for _, free := range []bool{false, true} {
				for steps := 0; steps <= min(len(names), 6-position); steps++ {
					t.Run(fmt.Sprintf("%v/position%d/free%v/steps%d", names, position, free, steps), func(t *testing.T) {
						g, repo, colonies, id, opponent := setupColonyGame(t)
						p, _ := g.GetPlayer(id)
						addTradeModifierCards(p, names...)
						setupColony(g, "luna", position, []string{id, id, opponent})
						registry := testutil.CreateTestCardRegistry()
						mapped := dto.ToGameDto(g, registry, id, colonies)
						options := mapped.Colonies[0].TradeOptions
						testutil.AssertEqual(t, min(len(names), 6-position)+1, len(options), "Legal options, capped and deduplicated")
						chosen := options[steps]
						testutil.AssertEqual(t, steps, chosen.TrackSteps, "Choices ascending")
						testutil.AssertEqual(t, position+steps, chosen.MarkerPosition, "Preview marker")
						before := p.Resources().Get()
						other, _ := g.GetPlayer(opponent)
						otherBefore := other.Resources().Get().Credits
						ctx := context.Background()
						if free {
							p.Selection().SetPendingFreeTradeSelection(&shared.PendingFreeTradeSelection{AvailableColonyIDs: []string{"luna"}, Source: "Test trade"})
							action := confirmAction.NewConfirmFreeTradeAction(repo, registry, colonies, game.NewInMemoryGameStateRepository())
							testutil.AssertNoError(t, action.Execute(ctx, g.ID(), id, "luna", steps), "Free trade")
							testutil.AssertTrue(t, p.Selection().GetPendingFreeTradeSelection() == nil, "Pending cleared")
							testutil.AssertEqual(t, before.Energy, p.Resources().Get().Energy, "Free trade has no resource cost")
						} else {
							action := colonyAction.NewTradeAction(repo, colonies, registry, game.NewInMemoryGameStateRepository(), testutil.TestLogger())
							testutil.AssertNoError(t, action.Execute(ctx, g.ID(), id, "luna", colonyAction.TradePaymentEnergy, steps, testutil.TradePayment(colonyAction.TradePaymentEnergy)), "Paid trade")
							testutil.AssertEqual(t, before.Energy-3, p.Resources().Get().Energy, "Trade cost paid once")
						}
						testutil.AssertEqual(t, before.Credits+chosen.Outputs[0].Amount, p.Resources().Get().Credits, "Actual gains equal preview including both owned colonies")
						testutil.AssertEqual(t, otherBefore+2, other.Resources().Get().Credits, "Opponent receives colony bonus")
						testutil.AssertEqual(t, 1, g.Colonies().TradeFleet(id).Used, "One fleet used")
						testutil.AssertEqual(t, 3, g.Colonies().GetState("luna").MarkerPosition, "Marker resets beside colonies")
						testutil.AssertTrue(t, g.Colonies().GetState("luna").TradedThisGen, "Colony unavailable for another fleet")
					})
				}
			}
		}
	}
}

func TestTrade_InvalidOrStaleChoiceHasNoSideEffects(t *testing.T) {
	for _, failure := range []string{"negative", "too large", "removed effect", "track moved", "colony traded", "fleet used", "unaffordable"} {
		for _, free := range []bool{false, true} {
			if free && failure == "unaffordable" {
				continue
			}
			t.Run(fmt.Sprintf("%s/free%v", failure, free), func(t *testing.T) {
				g, repo, colonies, id, _ := setupColonyGame(t)
				p, _ := g.GetPlayer(id)
				addTradeModifierCards(p, "Trade Envoys")
				setupColony(g, "luna", 3, nil)
				steps := 1
				switch failure {
				case "negative":
					steps = -1
				case "too large":
					steps = 2
				case "removed effect":
					p.PlayedCards().RemoveCard(testutil.CardID("Trade Envoys"))
				case "track moved":
					g.Colonies().GetState("luna").MarkerPosition = 6
				case "colony traded":
					g.Colonies().GetState("luna").TradedThisGen = true
				case "fleet used":
					testutil.AssertNoError(t, g.Colonies().UseTradeFleet(id), "Use fleet")
				case "unaffordable":
					p.Resources().Set(shared.Resources{})
				}
				before := p.Resources().Get()
				fleet := g.Colonies().TradeFleet(id)
				marker := g.Colonies().GetState("luna").MarkerPosition
				actions := g.CurrentTurn().ActionsRemaining()
				registry := testutil.CreateTestCardRegistry()
				var err error
				if free {
					p.Selection().SetPendingFreeTradeSelection(&shared.PendingFreeTradeSelection{AvailableColonyIDs: []string{"luna"}})
					action := confirmAction.NewConfirmFreeTradeAction(repo, registry, colonies, game.NewInMemoryGameStateRepository())
					err = action.Execute(context.Background(), g.ID(), id, "luna", steps)
					testutil.AssertTrue(t, p.Selection().GetPendingFreeTradeSelection() != nil, "Pending retained after rejection")
				} else {
					action := colonyAction.NewTradeAction(repo, colonies, registry, game.NewInMemoryGameStateRepository(), testutil.TestLogger())
					err = action.Execute(context.Background(), g.ID(), id, "luna", colonyAction.TradePaymentEnergy, steps, testutil.TradePayment(colonyAction.TradePaymentEnergy))
				}
				testutil.AssertTrue(t, err != nil, "Invalid request rejected")
				testutil.AssertTrue(t, reflect.DeepEqual(before, p.Resources().Get()), "Resources unchanged")
				testutil.AssertTrue(t, fleet == g.Colonies().TradeFleet(id), "Fleets unchanged")
				testutil.AssertEqual(t, marker, g.Colonies().GetState("luna").MarkerPosition, "Marker unchanged")
				testutil.AssertEqual(t, actions, g.CurrentTurn().ActionsRemaining(), "Actions unchanged")
			})
		}
	}
}

func TestTrade_MultipleFleetsAndGenerationReset(t *testing.T) {
	g, repo, colonies, id, _ := setupColonyGame(t)
	setupColony(g, "luna", 3, nil)
	setupColony(g, "io", 3, nil)
	setupColony(g, "callisto", 3, nil)
	g.Colonies().AddTradeFleets(id, 1)
	trade := colonyAction.NewTradeAction(repo, colonies, testutil.CreateTestCardRegistry(), game.NewInMemoryGameStateRepository(), testutil.TestLogger())
	for _, colonyID := range []string{"luna", "io"} {
		testutil.AssertNoError(t, trade.Execute(context.Background(), g.ID(), id, colonyID, colonyAction.TradePaymentEnergy, 0, testutil.TradePayment(colonyAction.TradePaymentEnergy)), "Trade with another fleet")
	}
	testutil.AssertEqual(t, 0, g.Colonies().TradeFleet(id).Available(), "Fleets exhausted")
	testutil.AssertNoError(t, g.SetCurrentTurn(context.Background(), id, 2), "Reset turn")
	testutil.AssertTrue(t, trade.Execute(context.Background(), g.ID(), id, "callisto", colonyAction.TradePaymentEnergy, 0, testutil.TradePayment(colonyAction.TradePaymentEnergy)) != nil, "Cannot trade beyond capacity")
	g.Colonies().AddTradeFleets(id, 1)
	testutil.AssertEqual(t, 1, g.Colonies().TradeFleet(id).Available(), "Gaining a fleet after exhaustion")
	testutil.AssertTrue(t, trade.Execute(context.Background(), g.ID(), id, "luna", colonyAction.TradePaymentEnergy, 0, testutil.TradePayment(colonyAction.TradePaymentEnergy)) != nil, "Extra fleet cannot revisit traded colony")
	testutil.AssertNoError(t, trade.Execute(context.Background(), g.ID(), id, "callisto", colonyAction.TradePaymentEnergy, 0, testutil.TradePayment(colonyAction.TradePaymentEnergy)), "New fleet can trade immediately")
	testutil.AssertNoError(t, turn_management.ExecuteProductionPhase(context.Background(), g, g.GetAllPlayers(), testutil.TestLogger()), "Generation rollover")
	testutil.AssertEqual(t, 3, g.Colonies().TradeFleet(id).Available(), "All fleets return")
	testutil.AssertEqual(t, 3, g.Colonies().TradeFleet(id).Capacity, "Capacity retained")
	testutil.AssertFalse(t, g.Colonies().GetState("luna").TradedThisGen, "Colonies become available")
}

func TestTradeOptions_SupportWholeNonUnitEffects(t *testing.T) {
	g, _, colonies, id, _ := setupColonyGame(t)
	p, _ := g.GetPlayer(id)
	setupColony(g, "luna", 0, nil)
	card := testutil.GetCardByName("Trade Envoys")
	card.ID = "test-non-unit"
	card.Behaviors[0].Outputs[0].(*shared.ColonyCondition).Amount = 2
	registry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{card})
	p.PlayedCards().AddCard(card.ID, card.Name, string(card.Type), nil)
	definition, _ := colonies.GetByID("luna")
	options := baseaction.CalculateColonyTradeOptions(p, g.Colonies().GetState("luna"), definition, registry)
	testutil.AssertEqual(t, 2, len(options), "Whole optional effect, not arbitrary partial amount")
	testutil.AssertEqual(t, 2, options[1].TrackSteps, "Non-unit output is respected")
}
