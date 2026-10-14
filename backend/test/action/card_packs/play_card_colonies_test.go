package card_packs_test

import (
	"context"
	"fmt"
	baseaction "openmars/internal/action"
	colonyAction "openmars/internal/action/colony"
	"openmars/internal/delivery/dto"
	"testing"

	cardAction "openmars/internal/action/card"
	confirmAction "openmars/internal/action/confirmation"
	"openmars/internal/action/turn_management"
	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/colony"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

// =============================================================================
// Helper: set up a game with colonies enabled
// =============================================================================

func setupColoniesGame(t *testing.T) (*game.Game, game.GameRepository, colony.ColonyRegistry, string, string) {
	t.Helper()
	testGame, repo, _, player1, player2 := testutil.SetupTwoPlayerGame(t)

	colonyDefs, err := colony.LoadColoniesFromJSON("../../../assets/colonies.json")
	if err != nil {
		t.Fatalf("Failed to load colonies: %v", err)
	}
	colonyRegistry := colony.NewInMemoryColonyRegistry(colonyDefs)

	settings := testGame.Settings()
	settings.Systems = append(settings.Systems, "colonies")
	testGame.UpdateSettings(context.Background(), settings)

	return testGame, repo, colonyRegistry, player1, player2
}

func addColony(g *game.Game, colonyID string, markerPosition int, playerColonies []string) {
	states := g.Colonies().States()
	states = append(states, &colony.ColonyState{
		DefinitionID:   colonyID,
		MarkerPosition: markerPosition,
		PlayerColonies: playerColonies,
	})
	g.Colonies().SetStates(states)
}

// addColonyTestPlayedCards sets up an existing tableau without replaying its effects.
func addColonyTestPlayedCards(p *player.Player, names ...string) {
	for _, name := range names {
		card := testutil.GetCardByName(name)
		tags := make([]string, len(card.Tags))
		for i, tag := range card.Tags {
			tags[i] = string(tag)
		}
		p.PlayedCards().AddCard(card.ID, card.Name, string(card.Type), tags)
	}
}

func playColonyTestCard(t *testing.T, g *game.Game, repo game.GameRepository, playerID, name string, targets ...string) {
	t.Helper()
	p, err := g.GetPlayer(playerID)
	testutil.AssertNoError(t, err, "Get player")
	card := testutil.GetCardByName(name)
	p.Hand().AddCard(card.ID)
	defs, err := colony.LoadColoniesFromJSON("../../../assets/colonies.json")
	testutil.AssertNoError(t, err, "Load colony definitions")
	play := cardAction.NewPlayCardAction(repo, testutil.CreateTestCardRegistry(), nil, testutil.TestLogger(), colony.NewInMemoryColonyRegistry(defs))
	testutil.AssertNoError(t, play.Execute(context.Background(), g.ID(), playerID, card.ID, shared.NativePayment(shared.ResourceCredit, card.Cost), nil, targets, nil, nil, nil), "Play "+name)
}

func TestCommunityServices_CountsOwnTaglessCardsIncludingItself(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cards []string
		want  int
	}{
		{"itself", nil, 1},
		{"own tagless card", []string{"Dust Seals"}, 2},
		{"tagged cards excluded", []string{"Dust Seals", "Mine"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, repo, _, playerID, opponentID := setupColoniesGame(t)
			p, _ := g.GetPlayer(playerID)
			opponent, _ := g.GetPlayer(opponentID)
			addColonyTestPlayedCards(p, tc.cards...)
			addColonyTestPlayedCards(opponent, "Micro-Mills")
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			before := p.Resources().Production().Credits
			playColonyTestCard(t, g, repo, playerID, "Community Services")
			testutil.AssertEqual(t, before+tc.want, p.Resources().Production().Credits, "Credit production counts only own tagless cards, including Community Services")
		})
	}
}

func TestQuantumCommunications_CountsColoniesNotColonyTiles(t *testing.T) {
	for _, populated := range []bool{false, true} {
		name := "empty tiles"
		if populated {
			name = "own and opponent colonies"
		}
		t.Run(name, func(t *testing.T) {
			g, repo, _, playerID, opponentID := setupColoniesGame(t)
			p, _ := g.GetPlayer(playerID)
			addColonyTestPlayedCards(p, "Research", "Search For Life", "Trans-Neptune Probe")
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			want := 0
			if populated {
				addColony(g, "luna", 4, []string{playerID, opponentID})
				addColony(g, "ceres", 4, []string{opponentID})
				want = 3
			} else {
				addColony(g, "luna", 4, nil)
				addColony(g, "ceres", 4, nil)
			}
			addColony(g, "callisto", 4, nil)
			before := p.Resources().Production().Credits
			playColonyTestCard(t, g, repo, playerID, "Quantum Communications")
			testutil.AssertEqual(t, before+want, p.Resources().Production().Credits, "Credit production counts every colony owned by any player")
		})
	}
}

func TestSolarProbe_DrawsPerThreeScienceTagsIncludingItself(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cards []string
		want  int
	}{
		{"own tag alone", nil, 0},
		{"below threshold", []string{"Search For Life"}, 0},
		{"own tag reaches threshold", []string{"Research"}, 1},
		{"two separate science cards plus own tag", []string{"Search For Life", "Trans-Neptune Probe"}, 1},
		{"previous event tag is excluded", []string{"Search For Life", "Invention Contest"}, 0},
		{"round down", []string{"Research", "Search For Life", "Trans-Neptune Probe"}, 1},
		{"two groups", []string{"Research", "Search For Life", "Trans-Neptune Probe", "Inventors' Guild"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, repo, _, playerID, opponentID := setupColoniesGame(t)
			g.InitDeck([]string{testutil.CardID("Dust Seals"), testutil.CardID("Mine"), testutil.CardID("Power Plant")}, nil, nil)
			p, _ := g.GetPlayer(playerID)
			opponent, _ := g.GetPlayer(opponentID)
			addColonyTestPlayedCards(p, tc.cards...)
			addColonyTestPlayedCards(opponent, "Research")
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			before := p.Hand().CardCount()
			drawn := g.Deck().DrawnCardCount()
			playColonyTestCard(t, g, repo, playerID, "Solar Probe")
			testutil.AssertEqual(t, before+tc.want, p.Hand().CardCount(), "Solar Probe draws using science tags, including its own and both Research tags")
			testutil.AssertEqual(t, drawn+tc.want, g.Deck().DrawnCardCount(), "Draws must come from the project deck")
		})
	}
}

func TestEcologyResearch_OwnColoniesAndResourcePlacement(t *testing.T) {
	for _, tc := range []struct {
		name        string
		ownColonies int
	}{
		{"opponent colonies only", 0}, {"two own colonies", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, repo, _, playerID, opponentID := setupColoniesGame(t)
			p, _ := g.GetPlayer(playerID)
			addColony(g, "luna", 4, []string{opponentID})
			if tc.ownColonies == 2 {
				addColony(g, "ceres", 4, []string{playerID})
				addColony(g, "callisto", 4, []string{playerID})
			}
			addColonyTestPlayedCards(p, "Fish", "Tardigrades")
			p.Resources().AddToStorage(testutil.CardID("Fish"), 3)
			p.Resources().AddToStorage(testutil.CardID("Tardigrades"), 4)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			before := p.Resources().Production().Plants
			playColonyTestCard(t, g, repo, playerID, "Ecology Research", testutil.CardID("Fish"), testutil.CardID("Tardigrades"))
			testutil.AssertEqual(t, 4, p.Resources().GetCardStorage(testutil.CardID("Fish")), "Add one animal")
			testutil.AssertEqual(t, 6, p.Resources().GetCardStorage(testutil.CardID("Tardigrades")), "Add two microbes")
			testutil.AssertEqual(t, before+tc.ownColonies, p.Resources().Production().Plants, "Plant production counts only own colonies")
		})
	}
}

func TestConscription_DiscountsOnlyTheNextCard(t *testing.T) {
	for _, name := range []string{"Sponsors", "Magnetic Field Generators"} {
		t.Run(name, func(t *testing.T) {
			g, repo, _, playerID, _ := setupColoniesGame(t)
			p, _ := g.GetPlayer(playerID)
			addColonyTestPlayedCards(p, "Acquired Company", "Lunar Mining")
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			p.Resources().AddProduction(map[shared.ResourceType]int{shared.ResourceEnergyProduction: 4})
			testutil.AssertNoError(t, g.SetCurrentTurn(context.Background(), playerID, 10), "Set turn")
			playColonyTestCard(t, g, repo, playerID, "Conscription")
			card := testutil.GetCardByName(name)
			p.Hand().AddCard(card.ID)
			before := p.Resources().Get().Credits
			payment := max(0, card.Cost-16)
			play := cardAction.NewPlayCardAction(repo, testutil.CreateTestCardRegistry(), nil, testutil.TestLogger())
			testutil.AssertNoError(t, play.Execute(context.Background(), g.ID(), playerID, card.ID, shared.NativePayment(shared.ResourceCredit, payment), nil, nil, nil, nil, nil), "Next card should accept the 16 credit discount")
			testutil.AssertEqual(t, before-payment, p.Resources().Get().Credits, "Discount cannot generate credits")
			next := testutil.GetCardByName("Power Plant")
			calculator := gamecards.NewRequirementModifierCalculator(testutil.CreateTestCardRegistry())
			testutil.AssertEqual(t, 0, calculator.CalculateCardDiscounts(p, &next), "Discount is consumed, including any unused portion")
			playColonyTestCard(t, g, repo, playerID, next.Name)
			testutil.AssertEqual(t, before-payment-next.Cost, p.Resources().Get().Credits, "Following card costs its full price")
		})
	}
}

func TestConscription_UnusedDiscountExpiresAfterGeneration(t *testing.T) {
	g, repo, _, playerID, _ := setupColoniesGame(t)
	p, _ := g.GetPlayer(playerID)
	addColonyTestPlayedCards(p, "Sponsors", "Acquired Company")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	playColonyTestCard(t, g, repo, playerID, "Conscription")
	next := testutil.GetCardByName("Power Plant")
	calculator := gamecards.NewRequirementModifierCalculator(testutil.CreateTestCardRegistry())
	if got := calculator.CalculateCardDiscounts(p, &next); got != 16 {
		t.Errorf("Discount must exist before generation ends: got %d, want 16", got)
	}
	testutil.AssertNoError(t, turn_management.ExecuteProductionPhase(context.Background(), g, g.GetAllPlayers(), testutil.TestLogger()), "Advance through production")
	testutil.AssertEqual(t, 0, calculator.CalculateCardDiscounts(p, &next), "Unused Conscription discount expires at generation end")
}

func TestMarketManipulation_RequiresColonyTrackChoices(t *testing.T) {
	g, repo, _, playerID, _ := setupColoniesGame(t)
	p, _ := g.GetPlayer(playerID)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	addColony(g, "luna", 3, nil)
	addColony(g, "ceres", 4, nil)
	addColony(g, "callisto", 5, nil)
	playColonyTestCard(t, g, repo, playerID, "Market Manipulation")
	testutil.AssertTrue(t, g.HasAnyPendingSelection(playerID), "Player must choose a colony track to increase and a different track to decrease")
	for i, state := range g.Colonies().States() {
		testutil.AssertEqual(t, i+3, state.MarkerPosition, "Tracks must not change before the player chooses")
	}
}

func setupMartianZooTest(t *testing.T) (*game.Game, game.GameRepository, string, string) {
	t.Helper()
	g, repo, _, playerID, opponentID := setupColoniesGame(t)
	ctx := context.Background()
	testutil.PlaceTileForPlayer(ctx, t, g, repo, playerID, "city", testutil.FormatHex(shared.HexPosition{Q: 3, R: -1, S: -2}))
	testutil.PlaceTileForPlayer(ctx, t, g, repo, opponentID, "city", testutil.FormatHex(shared.HexPosition{Q: -3, R: 1, S: 2}))
	testutil.AssertNoError(t, g.SetCurrentTurn(ctx, playerID, 10), "Set turn")
	p, _ := g.GetPlayer(playerID)
	addColonyTestPlayedCards(p, "Acquired Company", "Lunar Mining", "Earth Office")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	playColonyTestCard(t, g, repo, playerID, "Martian Zoo")
	return g, repo, playerID, opponentID
}

func TestMartianZoo_RequiresTwoCities(t *testing.T) {
	for _, cities := range []int{0, 1} {
		t.Run(fmt.Sprintf("%d cities", cities), func(t *testing.T) {
			g, repo, _, playerID, _ := setupColoniesGame(t)
			p, _ := g.GetPlayer(playerID)
			if cities == 1 {
				testutil.PlaceTileForPlayer(context.Background(), t, g, repo, playerID, "city", testutil.FormatHex(shared.HexPosition{Q: 3, R: -1, S: -2}))
			}
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			card := testutil.GetCardByName("Martian Zoo")
			p.Hand().AddCard(card.ID)
			before := p.Resources().Get()
			play := cardAction.NewPlayCardAction(repo, testutil.CreateTestCardRegistry(), nil, testutil.TestLogger())
			err := play.Execute(context.Background(), g.ID(), playerID, card.ID, shared.NativePayment(shared.ResourceCredit, card.Cost), nil, nil, nil, nil, nil)
			testutil.AssertErrorContains(t, err, "need at least 2 city tiles", "Martian Zoo requires at least two cities")
			testutil.AssertEqual(t, before, p.Resources().Get(), "Rejected play must not spend resources")
			testutil.AssertFalse(t, p.PlayedCards().Contains(card.ID), "Rejected card must not enter play")
		})
	}
}

func TestMartianZoo_StartsWithoutAnimals(t *testing.T) {
	g, _, playerID, _ := setupMartianZooTest(t)
	p, _ := g.GetPlayer(playerID)
	testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(testutil.CardID("Martian Zoo")), "Playing Martian Zoo does not itself add an animal")
}

func TestMartianZoo_AddsAnimalsPerOwnEarthTag(t *testing.T) {
	for _, tc := range []struct {
		name     string
		card     string
		opponent bool
		want     int
	}{
		{"one earth tag", "Sponsors", false, 1},
		{"two earth tags", "Luna Governor", false, 2},
		{"event earth tag", "Imported GHG", false, 1},
		{"non earth card", "Power Plant", false, 0},
		{"opponent earth tag", "Sponsors", true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, repo, playerID, opponentID := setupMartianZooTest(t)
			p, _ := g.GetPlayer(playerID)
			actorID := playerID
			if tc.opponent {
				actorID = opponentID
			}
			actor, _ := g.GetPlayer(actorID)
			actor.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			testutil.AssertNoError(t, g.SetCurrentTurn(context.Background(), actorID, 10), "Set turn")
			before := p.Resources().GetCardStorage(testutil.CardID("Martian Zoo"))
			playColonyTestCard(t, g, repo, actorID, tc.card)
			testutil.AssertEqual(t, before+tc.want, p.Resources().GetCardStorage(testutil.CardID("Martian Zoo")), "Add one animal per own Earth tag played")
		})
	}
}

func TestMartianZoo_ActionPaysPerAnimalWithoutSpendingAnimals(t *testing.T) {
	g, repo, playerID, _ := setupMartianZooTest(t)
	p, _ := g.GetPlayer(playerID)
	cardID := testutil.CardID("Martian Zoo")
	p.Resources().AddToStorage(cardID, 3)
	animals := p.Resources().GetCardStorage(cardID)
	for _, action := range p.Actions().List() {
		if action.CardID != cardID {
			continue
		}
		before := p.Resources().Get().Credits
		use := cardAction.NewUseCardActionAction(repo, testutil.CreateTestCardRegistry(), nil, testutil.TestLogger())
		err := use.Execute(context.Background(), g.ID(), playerID, cardID, action.BehaviorIndex, nil, nil, nil, nil, nil, nil, nil, nil)
		testutil.AssertNoError(t, err, "Use Martian Zoo income action")
		testutil.AssertEqual(t, before+animals, p.Resources().Get().Credits, "Gain one credit per animal")
		testutil.AssertEqual(t, animals, p.Resources().GetCardStorage(cardID), "Income action does not spend animals")
		err = use.Execute(context.Background(), g.ID(), playerID, cardID, action.BehaviorIndex, nil, nil, nil, nil, nil, nil, nil, nil)
		testutil.AssertErrorContains(t, err, "action already played this generation", "Income action can only be used once per generation")
		return
	}
	t.Fatal("Playing Martian Zoo must register its income action")
}

func TestSpinOffDepartment_InitialProduction(t *testing.T) {
	g, repo, _, playerID, _ := setupColoniesGame(t)
	p, _ := g.GetPlayer(playerID)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	before := p.Resources().Production().Credits
	playColonyTestCard(t, g, repo, playerID, "Spin-Off Department")
	testutil.AssertEqual(t, before+2, p.Resources().Production().Credits, "Initial credit production is retained")
}

func TestSpinOffDepartment_DrawsForOwnCardsWithBasicCostAtLeastTwenty(t *testing.T) {
	for _, tc := range []struct {
		name       string
		card       string
		discounted bool
		opponent   bool
		want       int
	}{
		{"below threshold", "Sponsors", false, false, 0},
		{"exact threshold", "Magnetic Field Generators", false, false, 1},
		{"above threshold", "Earth Catapult", false, false, 1},
		{"discounted below threshold", "Magnetic Field Generators", true, false, 1},
		{"opponent card", "Magnetic Field Generators", false, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, repo, _, playerID, opponentID := setupColoniesGame(t)
			g.InitDeck([]string{testutil.CardID("Dust Seals"), testutil.CardID("Mine"), testutil.CardID("Power Plant")}, nil, nil)
			p, _ := g.GetPlayer(playerID)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			testutil.AssertNoError(t, g.SetCurrentTurn(context.Background(), playerID, 10), "Set turn")
			playColonyTestCard(t, g, repo, playerID, "Spin-Off Department")
			if tc.discounted {
				playColonyTestCard(t, g, repo, playerID, "Earth Catapult")
			}
			actorID := playerID
			if tc.opponent {
				actorID = opponentID
			}
			actor, _ := g.GetPlayer(actorID)
			actor.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			actor.Resources().AddProduction(map[shared.ResourceType]int{shared.ResourceEnergyProduction: 4})
			testutil.AssertNoError(t, g.SetCurrentTurn(context.Background(), actorID, 10), "Set turn")
			card := testutil.GetCardByName(tc.card)
			actor.Hand().AddCard(card.ID)
			before := p.Hand().CardCount()
			payment := card.Cost
			if tc.discounted {
				payment -= 2
			}
			play := cardAction.NewPlayCardAction(repo, testutil.CreateTestCardRegistry(), nil, testutil.TestLogger())
			testutil.AssertNoError(t, play.Execute(context.Background(), g.ID(), actorID, card.ID, shared.NativePayment(shared.ResourceCredit, payment), nil, nil, nil, nil, nil), "Play card after Spin-Off Department")
			want := before + tc.want
			if !tc.opponent {
				want--
			}
			testutil.AssertEqual(t, want, p.Hand().CardCount(), "Spin-Off Department uses basic cost, regardless of the price paid")
		})
	}
}

// =============================================================================
// Titan Floating Launch-Pad (C44, active, colonies)
// Auto: Add 2 floaters to any jovian card.
// Manual choice A: Add 1 floater to any jovian card
// Manual choice B: Spend 1 floater here to trade for free
// =============================================================================

func TestTitanFloatingLaunchPad_AutoBehavior_PlacesFloatersOnSelf(t *testing.T) {
	testGame, repo, _, playerID, _ := setupColoniesGame(t)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()
	ctx := context.Background()

	p, _ := testGame.GetPlayer(playerID)
	card := testutil.GetCardByName("Titan Floating Launch-Pad")

	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 18)
	// cardStorageTargets: target self (the card being played) for the 2 floaters
	err := playCardAction.Execute(ctx, testGame.ID(), playerID, card.ID, payment, nil, []string{card.ID}, nil, nil, nil)
	testutil.AssertNoError(t, err, "Titan Floating Launch-Pad should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Card should be in played cards")

	storage := p.Resources().GetCardStorage(card.ID)
	testutil.AssertEqual(t, 2, storage, "Card should have 2 floaters from auto behavior")
}

func TestTitanFloatingLaunchPad_ManualAction_AddFloater(t *testing.T) {
	testGame, repo, _, playerID, _ := setupColoniesGame(t)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()
	ctx := context.Background()

	p, _ := testGame.GetPlayer(playerID)
	card := testutil.GetCardByName("Titan Floating Launch-Pad")

	// Set up card as already played with 1 floater
	p.PlayedCards().AddCard(card.ID, card.Name, string(card.Type), []string{"jovian"})
	p.Resources().AddToStorage(card.ID, 1)

	// Register manual action (behavior index 1 is the manual action)
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        card.ID,
			CardName:      card.Name,
			BehaviorIndex: 1,
			Behavior:      card.Behaviors[1],
		},
	})

	// Use choice 0: add 1 floater to a jovian card (target self)
	choiceIndex := 0
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, card.ID, 1, &choiceIndex, []string{card.ID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Manual action choice A should succeed")

	storage := p.Resources().GetCardStorage(card.ID)
	testutil.AssertEqual(t, 2, storage, "Card should have 2 floaters (1 original + 1 from action)")
}

func TestTitanFloatingLaunchPad_ManualAction_SpendFloaterForFreeTrade(t *testing.T) {
	testGame, repo, _, playerID, _ := setupColoniesGame(t)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()
	ctx := context.Background()

	p, _ := testGame.GetPlayer(playerID)
	card := testutil.GetCardByName("Titan Floating Launch-Pad")

	// Set up card as already played with 2 floaters
	p.PlayedCards().AddCard(card.ID, card.Name, string(card.Type), []string{"jovian"})
	p.Resources().AddToStorage(card.ID, 2)

	// Set up colony tiles and trade fleet for free trade
	addColony(testGame, "luna", 3, nil)
	testGame.Colonies().AddTradeFleets(playerID, 1)

	// Register manual action
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        card.ID,
			CardName:      card.Name,
			BehaviorIndex: 1,
			Behavior:      card.Behaviors[1],
		},
	})

	// Use choice 1: spend 1 floater to trade for free
	choiceIndex := 1
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, card.ID, 1, &choiceIndex, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Manual action choice B (free trade) should succeed")

	storage := p.Resources().GetCardStorage(card.ID)
	testutil.AssertEqual(t, 1, storage, "Card should have 1 floater (2 - 1 spent)")

	// Should have a pending free trade selection
	pendingTrade := p.Selection().GetPendingFreeTradeSelection()
	testutil.AssertTrue(t, pendingTrade != nil, "Should have pending free trade selection")
}

func TestTitanFloatingLaunchPad_UsableOnSameGeneration(t *testing.T) {
	testGame, repo, _, playerID, _ := setupColoniesGame(t)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()
	ctx := context.Background()

	p, _ := testGame.GetPlayer(playerID)
	card := testutil.GetCardByName("Titan Floating Launch-Pad")

	// Give player enough credits and extra actions
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, playerID, 3), "set current turn")

	// Play the card (auto behavior places 2 floaters on self)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 18)
	err := playCardAction.Execute(ctx, testGame.ID(), playerID, card.ID, payment, nil, []string{card.ID}, nil, nil, nil)
	testutil.AssertNoError(t, err, "Card should play successfully")

	storage := p.Resources().GetCardStorage(card.ID)
	testutil.AssertEqual(t, 2, storage, "Card should have 2 floaters from auto behavior")

	// Now use manual action choice 0: add 1 floater
	choiceIndex := 0
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err = useAction.Execute(ctx, testGame.ID(), playerID, card.ID, 1, &choiceIndex, []string{card.ID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Manual action should work on same generation as play")

	storage = p.Resources().GetCardStorage(card.ID)
	testutil.AssertEqual(t, 3, storage, "Card should have 3 floaters (2 + 1 from manual action)")
}

// =============================================================================
// Productive Outpost (C30, automated, colonies)
// "Gain all your colony bonuses"
// =============================================================================

func TestProductiveOutpost_NoColonies(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColoniesGame(t)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()
	ctx := context.Background()

	p, _ := testGame.GetPlayer(playerID)
	card := testutil.GetCardByName("Productive Outpost")

	// Set up colony tiles but player has no colonies
	addColony(testGame, "luna", 3, nil)
	addColony(testGame, "io", 2, nil)

	creditsBefore := p.Resources().Get().Credits
	p.Hand().AddCard(card.ID)

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger, colonyRegistry)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), playerID, card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Productive Outpost should play successfully")

	creditsAfter := p.Resources().Get().Credits
	testutil.AssertEqual(t, creditsBefore, creditsAfter, "Credits should not change when player has no colonies")
}

func TestProductiveOutpost_SingleColony_Luna(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColoniesGame(t)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()
	ctx := context.Background()

	p, _ := testGame.GetPlayer(playerID)
	card := testutil.GetCardByName("Productive Outpost")

	// Player has a colony on Luna (bonus: 2 credits)
	addColony(testGame, "luna", 3, []string{playerID})

	testutil.SetPlayerCredits(ctx, p, 0)
	p.Hand().AddCard(card.ID)

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger, colonyRegistry)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), playerID, card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Productive Outpost should play successfully")

	resources := p.Resources().Get()
	testutil.AssertEqual(t, 2, resources.Credits, "Should gain 2 credits from Luna colony bonus")
}

func TestProductiveOutpost_MultipleColonies(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColoniesGame(t)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()
	ctx := context.Background()

	p, _ := testGame.GetPlayer(playerID)
	card := testutil.GetCardByName("Productive Outpost")

	// Player has colonies on Luna (bonus: 2 credits) and Io (bonus: 2 heat)
	addColony(testGame, "luna", 3, []string{playerID})
	addColony(testGame, "io", 2, []string{playerID})

	testutil.SetPlayerCredits(ctx, p, 0)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceHeat: 0})
	p.Hand().AddCard(card.ID)

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger, colonyRegistry)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), playerID, card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Productive Outpost should play successfully")

	resources := p.Resources().Get()
	testutil.AssertEqual(t, 2, resources.Credits, "Should gain 2 credits from Luna colony bonus")
	testutil.AssertEqual(t, 2, resources.Heat, "Should gain 2 heat from Io colony bonus")
}

func TestProductiveOutpost_OnlyOwnColonies(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, otherPlayerID := setupColoniesGame(t)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()
	ctx := context.Background()

	p, _ := testGame.GetPlayer(playerID)
	card := testutil.GetCardByName("Productive Outpost")

	// Player has colony on Luna, other player has colony on Io
	addColony(testGame, "luna", 3, []string{playerID})
	addColony(testGame, "io", 2, []string{otherPlayerID})

	testutil.SetPlayerCredits(ctx, p, 0)
	p.Hand().AddCard(card.ID)

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger, colonyRegistry)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), playerID, card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Productive Outpost should play successfully")

	resources := p.Resources().Get()
	testutil.AssertEqual(t, 2, resources.Credits, "Should gain 2 credits from own Luna colony only")
	testutil.AssertEqual(t, 0, resources.Heat, "Should not gain heat from other player's Io colony")
}

func TestProductiveOutpost_MultipleColoniesOnDifferentTiles(t *testing.T) {
	testGame, repo, colonyRegistry, playerID, _ := setupColoniesGame(t)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()
	ctx := context.Background()

	p, _ := testGame.GetPlayer(playerID)
	card := testutil.GetCardByName("Productive Outpost")

	// Player has colonies on Luna (2 credits), Ceres (2 steel), and Ganymede (1 plant)
	addColony(testGame, "luna", 3, []string{playerID})
	addColony(testGame, "ceres", 2, []string{playerID})
	addColony(testGame, "ganymede", 1, []string{playerID})

	testutil.SetPlayerCredits(ctx, p, 0)
	p.Hand().AddCard(card.ID)

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger, colonyRegistry)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), playerID, card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Productive Outpost should play successfully")

	resources := p.Resources().Get()
	testutil.AssertEqual(t, 2, resources.Credits, "Should gain 2 credits from Luna")
	testutil.AssertEqual(t, 2, resources.Steel, "Should gain 2 steel from Ceres")
	testutil.AssertEqual(t, 1, resources.Plants, "Should gain 1 plant from Ganymede")
}

func TestMarketManipulation_ConfirmsBothTracksAtomically(t *testing.T) {
	g, repo, colonies, id, _ := setupColoniesGame(t)
	p, _ := g.GetPlayer(id)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	addColony(g, "luna", 3, []string{id})
	addColony(g, "ceres", 4, nil)
	playColonyTestCard(t, g, repo, id, "Market Manipulation")
	pending := p.Selection().GetPendingEffectSelection()
	if pending == nil {
		t.Fatal("Expected track selection")
	}
	confirm := confirmAction.NewConfirmEffectSelectionAction(repo, testutil.CreateTestCardRegistry(), colonies, nil)
	testutil.AssertErrorContains(t, confirm.Execute(context.Background(), g.ID(), id, -1), "invalid effect option", "Reject invalid option")
	testutil.AssertEqual(t, 3, g.Colonies().GetState("luna").MarkerPosition, "Invalid selection cannot move tracks")
	before := p.Resources().Get()
	for i, o := range pending.Options {
		testutil.AssertFalse(t, o.ColonyIDs[0] == o.ColonyIDs[1], "Track targets must differ")
		if o.ColonyIDs[0] != "luna" {
			continue
		}
		testutil.AssertNoError(t, confirm.Execute(context.Background(), g.ID(), id, i), "Confirm both tracks")
		testutil.AssertEqual(t, 4, g.Colonies().GetState("luna").MarkerPosition, "Raise first track")
		testutil.AssertEqual(t, 3, g.Colonies().GetState("ceres").MarkerPosition, "Lower second track")
		testutil.AssertEqual(t, before, p.Resources().Get(), "No trading income or colony bonus")
		testutil.AssertFalse(t, g.Colonies().GetState("luna").TradedThisGen, "Movement is not a trade")
		testutil.AssertFalse(t, g.HasAnyPendingSelection(id), "Selection completed")
		return
	}
	t.Fatal("Expected legal pair")
}

func TestMarketManipulation_NoLegalPairDoesNotSpend(t *testing.T) {
	g, repo, colonies, id, _ := setupColoniesGame(t)
	p, _ := g.GetPlayer(id)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	addColony(g, "luna", 1, []string{id})
	card := testutil.GetCardByName("Market Manipulation")
	p.Hand().AddCard(card.ID)
	before := p.Resources().Get()
	err := cardAction.NewPlayCardAction(repo, testutil.CreateTestCardRegistry(), nil, testutil.TestLogger(), colonies).Execute(context.Background(), g.ID(), id, card.ID, shared.NativePayment(shared.ResourceCredit, 1), nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "no legal effect selection", "Cannot move same track twice or below occupied slot")
	testutil.AssertEqual(t, before, p.Resources().Get(), "No payment for impossible pair")
}

func TestMarketManipulation_StalePairDoesNotPartiallyMove(t *testing.T) {
	g, repo, colonies, id, _ := setupColoniesGame(t)
	p, _ := g.GetPlayer(id)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	addColony(g, "luna", 3, nil)
	addColony(g, "ceres", 1, []string{id})
	// Ceres cannot decrease below its occupied colony slot, but Luna can decrease below the initial position.
	playColonyTestCard(t, g, repo, id, "Market Manipulation")
	pending := p.Selection().GetPendingEffectSelection()
	if pending == nil || len(pending.Options) != 1 {
		t.Fatal("Expected exactly one legal pair")
	}
	def, _ := colonies.GetByID("ceres")
	g.Colonies().MoveTradeMarkers(map[string]int{"ceres": len(def.Steps) - 2})
	before := g.Colonies().GetState("luna").MarkerPosition
	err := confirmAction.NewConfirmEffectSelectionAction(repo, testutil.CreateTestCardRegistry(), colonies, nil).Execute(context.Background(), g.ID(), id, 0)
	testutil.AssertErrorContains(t, err, "effect selection is no longer legal", "Revalidate both tracks at confirmation")
	testutil.AssertEqual(t, before, g.Colonies().GetState("luna").MarkerPosition, "No partial decrease when increase is invalid")
	testutil.AssertTrue(t, g.HasAnyPendingSelection(id), "Invalid confirmation keeps selection")
}

func TestCryoSleep_PersistentTradeDiscount(t *testing.T) {
	g, repo, _, id, _ := setupColoniesGame(t)
	p, _ := g.GetPlayer(id)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	before := p.Resources().Get()
	playColonyTestCard(t, g, repo, id, "Cryo-Sleep")
	after := p.Resources().Get()
	testutil.AssertEqual(t, before.Credits-testutil.GetCardByName("Cryo-Sleep").Cost, after.Credits, "Only card cost is spent")
	testutil.AssertEqual(t, before.Energy, after.Energy, "No immediate energy")
	testutil.AssertEqual(t, before.Titanium, after.Titanium, "No immediate titanium")
	discounts := gamecards.NewRequirementModifierCalculator(testutil.CreateTestCardRegistry()).CalculateActionDiscounts(p, shared.ActionColonyTrade)
	for _, resource := range []shared.ResourceType{shared.ResourceCredit, shared.ResourceEnergy, shared.ResourceTitanium} {
		testutil.AssertEqual(t, 1, discounts[resource], "Persistent trade discount")
	}
}

func TestFleetCards_DeclareFleetGain(t *testing.T) {
	for _, name := range []string{"Sky Docks", "Space Port", "Space Port Colony"} {
		t.Run(name, func(t *testing.T) {
			card := testutil.GetCardByName(name)
			amount := 0
			for _, behavior := range gamecards.GetImmediateBehaviors(&card) {
				for _, output := range behavior.Outputs {
					if output.GetResourceType() == "trade-fleet" && output.GetTarget() == "self-player" {
						amount += output.GetAmount()
					}
				}
			}
			testutil.AssertEqual(t, 1, amount, "Gain one fleet")
		})
	}
}

func TestTradeTrackCards_DeclareOptionalContextualIncrease(t *testing.T) {
	for _, name := range []string{"Trade Envoys", "Trading Colony"} {
		t.Run(name, func(t *testing.T) {
			card := testutil.GetCardByName(name)
			found := false
			for _, behavior := range gamecards.GetPassiveBehaviors(&card) {
				for _, output := range behavior.Outputs {
					if output.GetResourceType() == shared.ResourceColonyTrackStep {
						found = true
						testutil.AssertEqual(t, "trigger-colony", output.GetTarget(), "Increase traded colony")
						testutil.AssertTrue(t, shared.IsOptional(output), "Increase is optional")
					}
				}
			}
			testutil.AssertTrue(t, found, "Has trade modifier")
		})
	}
}

func TestFleetCards_GainImmediatelyAvailableFleet(t *testing.T) {
	for _, name := range []string{"Sky Docks", "Space Port", "Space Port Colony"} {
		t.Run(name, func(t *testing.T) {
			g, repo, _, id, _ := setupColoniesGame(t)
			g.InitializeTradeFleets([]string{id})
			testutil.AssertNoError(t, g.Colonies().UseTradeFleet(id), "Use original fleet")
			p, _ := g.GetPlayer(id)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			p.Resources().AddProduction(map[shared.ResourceType]int{shared.ResourceEnergyProduction: 1})
			addColonyTestPlayedCards(p, "Earth Catapult", "Earth Office")
			addColony(g, "luna", 3, []string{id})
			before := p.Resources().Production()
			playColonyTestCard(t, g, repo, id, name)
			fleet := g.Colonies().TradeFleet(id)
			testutil.AssertEqual(t, 2, fleet.Capacity, "Permanent capacity increases")
			testutil.AssertEqual(t, 1, fleet.Used, "Existing use is retained")
			testutil.AssertEqual(t, 1, fleet.Available(), "New fleet is immediately available")
			switch name {
			case "Sky Docks":
				discounts := gamecards.NewRequirementModifierCalculator(testutil.CreateTestCardRegistry()).CalculateActionDiscounts(p, shared.ActionCardPlaying)
				// Earth Catapult and Earth Office are tableau fixtures, not registered discounts.
				testutil.AssertEqual(t, 1, discounts[shared.ResourceCredit], "Card discount is retained")
			case "Space Port":
				testutil.AssertTrue(t, g.GetPendingTileSelection(id) != nil, "City placement is retained")
				testutil.AssertEqual(t, before.Credits+4, p.Resources().Production().Credits, "Credit production is retained")
				testutil.AssertEqual(t, before.Energy-1, p.Resources().Production().Energy, "Energy decrease is retained")
			case "Space Port Colony":
				testutil.AssertTrue(t, p.Selection().GetPendingColonySelection() != nil, "Colony placement is retained")
			}
		})
	}
}

func TestCryoSleepAndRimFreighters_DiscountedTradeAcrossAllPayments(t *testing.T) {
	for _, count := range []int{1, 2} {
		for _, resource := range []shared.ResourceType{shared.ResourceCredit, shared.ResourceEnergy, shared.ResourceTitanium} {
			t.Run(fmt.Sprintf("%d discounts/%s", count, resource), func(t *testing.T) {
				g, repo, colonies, id, _ := setupColoniesGame(t)
				g.InitializeTradeFleets([]string{id})
				p, _ := g.GetPlayer(id)
				p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
				playColonyTestCard(t, g, repo, id, "Cryo-Sleep")
				if count == 2 {
					playColonyTestCard(t, g, repo, id, "Rim Freighters")
				}
				testutil.AssertNoError(t, g.SetCurrentTurn(context.Background(), id, 2), "Reset turn")
				addColony(g, "luna", 3, nil)
				base := 3
				payment := colonyAction.TradePaymentType(resource)
				if resource == shared.ResourceCredit {
					base = 9
					payment = colonyAction.TradePaymentCredits
				}
				p.Resources().Set(shared.Resources{})
				p.Resources().Add(map[shared.ResourceType]int{resource: base - count})
				registry := testutil.CreateTestCardRegistry()
				state := baseaction.CalculateColonyTradeState(p, g, registry)
				testutil.AssertTrue(t, state.Available(), "Discounted trade is available")
				mapped := dto.ToGameDto(g, registry, id, colonies)
				testutil.AssertTrue(t, mapped.Colonies[0].TradeAvailable, "Colony DTO agrees with discounted affordability")
				trade := colonyAction.NewTradeAction(repo, colonies, registry, game.NewInMemoryGameStateRepository(), testutil.TestLogger())
				testutil.AssertNoError(t, trade.Execute(context.Background(), g.ID(), id, "luna", payment, 0, testutil.TradePayment(payment)), "Pay discounted amount")
				expected := 0
				if resource == shared.ResourceCredit {
					expected = 7
				}
				testutil.AssertEqual(t, expected, p.Resources().Get().GetAmount(resource), "Exact discounted cost charged")
				testutil.AssertEqual(t, 0, g.Colonies().TradeFleet(id).Available(), "One fleet consumed")
			})
		}
	}
}

func TestTradeTrackCards_RealPlayKeepsImmediateEffects(t *testing.T) {
	for _, name := range []string{"Trade Envoys", "Trading Colony"} {
		t.Run(name, func(t *testing.T) {
			g, repo, colonies, id, _ := setupColoniesGame(t)
			p, _ := g.GetPlayer(id)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			addColony(g, "luna", 3, nil)
			playColonyTestCard(t, g, repo, id, name)
			testutil.AssertEqual(t, 3, g.Colonies().GetState("luna").MarkerPosition, "Track is not advanced when playing the card")
			testutil.AssertTrue(t, p.Selection().GetPendingEffectSelection() == nil, "No independent track selection")
			if name == "Trading Colony" {
				testutil.AssertTrue(t, p.Selection().GetPendingColonySelection() != nil, "Still places a colony")
				confirm := confirmAction.NewConfirmColonyPlacementAction(repo, testutil.CreateTestCardRegistry(), colonies, testutil.TestLogger())
				testutil.AssertNoError(t, confirm.Execute(context.Background(), g.ID(), id, "luna"), "Place colony")
			}
			mapped := dto.ToGameDto(g, testutil.CreateTestCardRegistry(), id, colonies)
			testutil.AssertEqual(t, 2, len(mapped.Colonies[0].TradeOptions), "Real card provides optional increase")
		})
	}
}

func TestAirRaid_RequiresOwnedFloaterPayment(t *testing.T) {
	for _, tc := range []struct {
		name        string
		source      string
		owner       bool
		stored      int
		valid       bool
		corporation bool
		wantErr     string
	}{
		{"owned card", "Titan Shuttles", true, 2, true, false, ""},
		{"owned corporation", "Stormcraft Incorporated", true, 1, true, true, ""},
		{"missing selection", "", true, 0, false, false, "select an eligible owned card"},
		{"empty source", "Titan Shuttles", true, 0, false, false, "insufficient resources on card"},
		{"opponent source", "Titan Shuttles", false, 2, false, false, "select an eligible owned card"},
		{"wrong storage type", "Pets", true, 2, false, false, "select an eligible owned card"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, repo, registry, id, opponentID := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			opponent, _ := g.GetPlayer(opponentID)
			testutil.AssertNoError(t, g.SetCurrentTurn(context.Background(), id, 2), "Set turn")
			raid := testutil.GetCardByName("Air Raid")
			p.Hand().AddCard(raid.ID)
			var sources []string
			owner := p
			if !tc.owner {
				owner = opponent
			}
			if tc.source != "" {
				source := testutil.GetCardByName(tc.source)
				if tc.corporation {
					owner.SetCorporationID(source.ID)
				} else {
					addColonyTestPlayedCards(owner, tc.source)
				}
				owner.Resources().AddToStorage(source.ID, tc.stored)
				sources = []string{source.ID}
			}

			state := baseaction.CalculatePlayerCardState(&raid, p, g, registry)
			testutil.AssertEqual(t, tc.valid, state.Available(), "Card availability requires owned floaters")
			p.CardStateStore().SetState(raid.ID, state)
			playerDto := dto.ToPlayerDto(p, g, registry, nil, nil, nil, nil)
			for _, c := range playerDto.Cards {
				if c.ID == raid.ID {
					if c.Behaviors[0].InputOptions == nil {
						t.Fatal("Missing storage source options")
					}
					eligible := c.Behaviors[0].InputOptions.StorageSources[0]
					if tc.valid && (len(eligible) != 1 || eligible[0] != sources[0]) {
						t.Fatalf("Unexpected source choices: %v", eligible)
					}
					if !tc.valid && len(eligible) != 0 {
						t.Fatalf("Invalid sources advertised: %v", eligible)
					}
				}
			}
			opponent.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 10})
			before, victimBefore := p.Resources().Get().Credits, opponent.Resources().Get().Credits
			turnBefore := g.CurrentTurn().ActionsRemaining()
			play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
			err := play.Execute(context.Background(), g.ID(), id, raid.ID, shared.Payment{Allocations: []shared.PaymentAllocation{}}, nil, nil, &opponentID, nil, sources)
			if !tc.valid {
				testutil.AssertErrorContains(t, err, tc.wantErr, "Invalid source must reject card")
				testutil.AssertTrue(t, p.Hand().HasCard(raid.ID), "Failed play keeps card in hand")
				testutil.AssertEqual(t, before, p.Resources().Get().Credits, "No theft on invalid payment")
				testutil.AssertEqual(t, victimBefore, opponent.Resources().Get().Credits, "Victim unchanged")
				testutil.AssertEqual(t, turnBefore, g.CurrentTurn().ActionsRemaining(), "Action count unchanged")
			} else {
				testutil.AssertNoError(t, err, "Pay own floater")
				testutil.AssertEqual(t, before+5, p.Resources().Get().Credits, "Steal five credits")
				testutil.AssertEqual(t, victimBefore-5, opponent.Resources().Get().Credits, "Chosen victim loses five credits")
			}
			if len(sources) > 0 {
				want := tc.stored
				if tc.valid {
					want--
				}
				testutil.AssertEqual(t, want, owner.Resources().GetCardStorage(sources[0]), "Floater payment exactly once")
			}
		})
	}
}

func TestTitanShuttles_ConvertsSelectedFloaters(t *testing.T) {
	for _, amount := range []int{0, 2, 5, 6, -1, -2} {
		t.Run(fmt.Sprint(amount), func(t *testing.T) {
			g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			testutil.AssertNoError(t, g.SetCurrentTurn(context.Background(), id, 10), "Set turn")
			playColonyTestCard(t, g, repo, id, "Titan Shuttles")
			card := testutil.GetCardByName("Titan Shuttles")
			stored := 5
			if amount == 0 {
				stored = 0
			}
			p.Resources().AddToStorage(card.ID, stored)
			before := p.Resources().Get().Titanium
			choice := 1
			playerDto := dto.ToPlayerDto(p, g, registry, nil, nil, nil, nil)
			found := false
			for _, a := range playerDto.Actions {
				if a.CardID == card.ID {
					found = true
					opts := a.Behavior.Choices[1].InputOptions
					if opts == nil || opts.VariableAmount == nil {
						t.Fatal("Missing storage amount bounds")
					}
					testutil.AssertEqual(t, stored, opts.VariableAmount.Max, "Amount picker uses this card's floaters")
				}
			}
			if !found {
				t.Fatal("Titan Shuttles action not registered")
			}
			use := cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger())
			selected := &amount
			if amount == -2 {
				selected = nil
			}
			err := use.Execute(context.Background(), g.ID(), id, card.ID, 0, &choice, nil, nil, nil, selected, nil, nil, nil)
			if amount < 0 || amount > 5 {
				wantErr := map[int]string{-2: "must select an amount", -1: "selected amount must be nonnegative", 6: "insufficient resources on card"}[amount]
				testutil.AssertErrorContains(t, err, wantErr, "Reject invalid conversion amount")
				testutil.AssertEqual(t, stored, p.Resources().GetCardStorage(card.ID), "Invalid conversion keeps floaters")
				testutil.AssertEqual(t, before, p.Resources().Get().Titanium, "Invalid conversion keeps titanium")
			} else {
				testutil.AssertNoError(t, err, "Convert selected floaters")
				testutil.AssertEqual(t, stored-amount, p.Resources().GetCardStorage(card.ID), "Spend chosen floaters")
				testutil.AssertEqual(t, before+amount, p.Resources().Get().Titanium, "Gain equal titanium")
				testutil.AssertErrorContains(t, use.Execute(context.Background(), g.ID(), id, card.ID, 0, &choice, nil, nil, nil, &amount, nil, nil, nil), "action already played this generation", "Action cannot repeat this generation")
			}
			vp := gamecards.CalculatePlayerVP(p, g, nil, nil, g.GetAllPlayers(), registry, nil, nil)
			testutil.AssertEqual(t, 1, vp.CardVP, "One fixed VP regardless of stored floaters")
		})
	}
}

func TestTitanShuttles_AddsOnlyToJovianStorage(t *testing.T) {
	for _, name := range []string{"Titan Shuttles", "Dirigibles"} {
		t.Run(name, func(t *testing.T) {
			g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			testutil.AssertNoError(t, g.SetCurrentTurn(context.Background(), id, 10), "Set turn")
			playColonyTestCard(t, g, repo, id, "Titan Shuttles")
			if name != "Titan Shuttles" {
				addColonyTestPlayedCards(p, name)
			}
			target := testutil.CardID(name)
			choice := 0
			err := cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger()).Execute(context.Background(), g.ID(), id, testutil.CardID("Titan Shuttles"), 0, &choice, []string{target}, nil, nil, nil, nil, nil, nil)
			if name == "Titan Shuttles" {
				testutil.AssertNoError(t, err, "Jovian source accepts floaters")
				testutil.AssertEqual(t, 2, p.Resources().GetCardStorage(target), "Adds two floaters")
			} else {
				testutil.AssertErrorContains(t, err, "target card does not match selectors", "Non-Jovian target rejected")
				testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(target), "Rejected action leaves storage unchanged")
			}
		})
	}
}

func TestRefugeeCamps_StoresCampsAndScoresWithProductionFloor(t *testing.T) {
	for _, production := range []int{2, 0, -4, -5} {
		t.Run(fmt.Sprint(production), func(t *testing.T) {
			g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			testutil.AssertNoError(t, g.SetCurrentTurn(context.Background(), id, 10), "Set turn")
			playColonyTestCard(t, g, repo, id, "Refugee Camps")
			card := testutil.GetCardByName("Refugee Camps")
			p.Resources().AddProduction(map[shared.ResourceType]int{shared.ResourceCreditProduction: production - p.Resources().Production().Credits})
			p.Resources().AddToStorage(card.ID, 3)
			before := p.Resources().Get().Credits
			state := baseaction.CalculatePlayerCardActionState(card.ID, card.Behaviors[0], 0, p, g, registry)
			testutil.AssertEqual(t, production > -5, state.Available(), "Availability follows production floor")
			err := cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger()).Execute(context.Background(), g.ID(), id, card.ID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
			want := 3
			if production > -5 {
				testutil.AssertNoError(t, err, "Pay production for camp")
				want++
				testutil.AssertEqual(t, production-1, p.Resources().Production().Credits, "Production reduced")
			} else {
				testutil.AssertErrorContains(t, err, "insufficient credit-production", "Cannot reduce below floor")
				testutil.AssertEqual(t, -5, p.Resources().Production().Credits, "Production unchanged")
			}
			testutil.AssertEqual(t, before, p.Resources().Get().Credits, "Camp does not grant spendable credits")
			testutil.AssertEqual(t, want, p.Resources().GetCardStorage(card.ID), "Camp storage")
			vp := gamecards.CalculatePlayerVP(p, g, nil, nil, g.GetAllPlayers(), registry, nil, nil)
			testutil.AssertEqual(t, want, vp.CardVP, "One VP per camp")
		})
	}
}

func assertColonyScaledPreview(t *testing.T, state player.EntityState, resource string, want int) {
	t.Helper()
	for _, value := range state.ComputedValues {
		for _, output := range value.Outputs {
			if output.ResourceType == resource {
				testutil.AssertEqual(t, want, output.Amount, "Preview matches payout")
				testutil.AssertTrue(t, output.IsScaled, "Preview is scaled")
				return
			}
		}
	}
	t.Fatalf("Missing computed preview for %s", resource)
}

func TestLunarMining_ScalesEarthTagsIncludingItself(t *testing.T) {
	for _, tc := range []struct {
		name        string
		existing    int
		corporation bool
		want        int
	}{
		{"source alone", 0, false, 0}, {"source reaches threshold", 1, false, 1},
		{"round down", 2, false, 1}, {"two groups", 3, false, 2}, {"five total tags", 4, false, 2},
		{"corporation tag", 0, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, repo, registry, id, opponentID := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			opponent, _ := g.GetPlayer(opponentID)
			names := []string{"Earth Catapult", "Earth Office", "Sponsors", "Lunar Exports"}
			addColonyTestPlayedCards(p, names[:tc.existing]...)
			addColonyTestPlayedCards(p, "Business Contacts")
			addColonyTestPlayedCards(opponent, "Earth Catapult", "Sponsors")
			if tc.corporation {
				p.SetCorporationID(testutil.CardID("Point Luna"))
			}
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			card := testutil.GetCardByName("Lunar Mining")
			assertColonyScaledPreview(t, baseaction.CalculatePlayerCardState(&card, p, g, registry), "titanium-production", tc.want)
			before := p.Resources().Production().Titanium
			playColonyTestCard(t, g, repo, id, card.Name)
			testutil.AssertEqual(t, before+tc.want, p.Resources().Production().Titanium, "Production counts own Earth tags including source once")
			assertColonyScaledPreview(t, baseaction.CalculatePlayerCardState(&card, p, g, registry), "titanium-production", tc.want)
		})
	}
}

func TestJupiterFloatingStation_CappedPayout(t *testing.T) {
	for _, stored := range []int{0, 3, 4, 5, 10} {
		t.Run(fmt.Sprint(stored), func(t *testing.T) {
			g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			testutil.AssertNoError(t, g.SetCurrentTurn(context.Background(), id, 10), "Set turn")
			addColonyTestPlayedCards(p, "Research", "Search For Life")
			beforeVP := gamecards.CalculatePlayerVP(p, g, nil, nil, g.GetAllPlayers(), registry, nil, nil).CardVP
			playColonyTestCard(t, g, repo, id, "Jupiter Floating Station")
			card := testutil.GetCardByName("Jupiter Floating Station")
			p.Resources().AddToStorage(card.ID, stored)
			want := min(stored, 4)
			assertColonyScaledPreview(t, baseaction.CalculatePlayerCardActionState(card.ID, card.Behaviors[0], 0, p, g, registry), "credit", want)
			before := p.Resources().Get().Credits
			choice := 1
			use := cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger())
			err := use.Execute(context.Background(), g.ID(), id, card.ID, 0, &choice, nil, nil, nil, nil, nil, nil, nil)
			testutil.AssertNoError(t, err, "Gain capped credits")
			testutil.AssertEqual(t, before+want, p.Resources().Get().Credits, "Payout is capped at four")
			testutil.AssertEqual(t, stored, p.Resources().GetCardStorage(card.ID), "Floaters are not spent")
			testutil.AssertErrorContains(t, use.Execute(context.Background(), g.ID(), id, card.ID, 0, &choice, nil, nil, nil, nil, nil, nil, nil), "action already played this generation", "Cannot repeat action this generation")
			vp := gamecards.CalculatePlayerVP(p, g, nil, nil, g.GetAllPlayers(), registry, nil, nil)
			testutil.AssertEqual(t, beforeVP+1, vp.CardVP, "Fixed VP is independent of floaters")
		})
	}
}

func TestJupiterFloatingStation_AddsOnlyToJovianStorage(t *testing.T) {
	for _, name := range []string{"Jupiter Floating Station", "Titan Shuttles", "Dirigibles"} {
		t.Run(name, func(t *testing.T) {
			g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			testutil.AssertNoError(t, g.SetCurrentTurn(context.Background(), id, 10), "Set turn")
			addColonyTestPlayedCards(p, "Research", "Search For Life")
			playColonyTestCard(t, g, repo, id, "Jupiter Floating Station")
			if name != "Jupiter Floating Station" {
				addColonyTestPlayedCards(p, name)
			}
			target := testutil.CardID(name)
			choice := 0
			err := cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger()).Execute(context.Background(), g.ID(), id, testutil.CardID("Jupiter Floating Station"), 0, &choice, []string{target}, nil, nil, nil, nil, nil, nil)
			if name == "Dirigibles" {
				testutil.AssertErrorContains(t, err, "target card does not match selectors", "Reject non-Jovian storage")
				testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(target), "Rejected action leaves storage unchanged")
			} else {
				testutil.AssertNoError(t, err, "Add floater to Jovian storage")
				testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(target), "Add exactly one floater")
			}
		})
	}
}

func TestMarketManipulation_ExcludesInactiveColoniesAndTrackBounds(t *testing.T) {
	g, repo, colonies, id, _ := setupColoniesGame(t)
	p, _ := g.GetPlayer(id)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	miranda, err := colonies.GetByID("miranda")
	testutil.AssertNoError(t, err, "Load Miranda")
	luna, err := colonies.GetByID("luna")
	testutil.AssertNoError(t, err, "Load Luna")
	g.Colonies().SetStates([]*colony.ColonyState{
		colony.NewTileState(*miranda, false),
		{DefinitionID: "luna", MarkerPosition: len(luna.Steps) - 1},
		{DefinitionID: "ceres", MarkerPosition: 1, PlayerColonies: []string{id}},
	})
	playColonyTestCard(t, g, repo, id, "Market Manipulation")
	pending := p.Selection().GetPendingEffectSelection()
	if pending == nil {
		t.Fatal("Expected track selection")
	}
	testutil.AssertEqual(t, 1, len(pending.Options), "Only increase Ceres and decrease Luna is legal")
	testutil.AssertEqual(t, "ceres", pending.Options[0].ColonyIDs[0], "Cannot raise an inactive or maximum track")
	testutil.AssertEqual(t, "luna", pending.Options[0].ColonyIDs[1], "Cannot lower an inactive track or cross the colony floor")
	confirm := confirmAction.NewConfirmEffectSelectionAction(repo, testutil.CreateTestCardRegistry(), colonies, nil)
	testutil.AssertNoError(t, confirm.Execute(context.Background(), g.ID(), id, 0), "Confirm legal pair")
	testutil.AssertEqual(t, -1, g.Colonies().GetState("miranda").MarkerPosition, "Inactive colony marker stays absent")
	testutil.AssertEqual(t, "animal", g.Colonies().GetState("miranda").AwaitingResource, "Card does not activate a colony")
	testutil.AssertEqual(t, 2, g.Colonies().GetState("ceres").MarkerPosition, "Increase applies")
	testutil.AssertEqual(t, len(luna.Steps)-2, g.Colonies().GetState("luna").MarkerPosition, "Decrease applies")
}

func TestUrbanDecomposers_RequiresCityAndAnyPlayersColony(t *testing.T) {
	for _, tc := range []struct {
		name         string
		city, colony bool
	}{
		{"both absent", false, false}, {"no city", false, true}, {"no colony", true, false}, {"opponent owns both", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, repo, _, id, opponent := setupColoniesGame(t)
			ctx := context.Background()
			registry := testutil.CreateTestCardRegistry()
			p, _ := g.GetPlayer(id)
			if tc.city {
				testutil.PlaceTileForPlayer(ctx, t, g, repo, opponent, "city", testutil.FindUnoccupiedLandHex(t, g))
			}
			if tc.colony {
				g.Colonies().SetStates([]*colony.ColonyState{{DefinitionID: "luna", MarkerPosition: 1, PlayerColonies: []string{opponent}}})
			}
			testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 2), "restore player turn")
			p.Resources().Set(shared.Resources{Credits: 6})
			// A legal destination for the two microbes.
			p.PlayedCards().AddCard(testutil.CardID("Regolith Eaters"), "Regolith Eaters", "active", []string{"microbe", "science"})
			idCard := testutil.CardID("Urban Decomposers")
			card, _ := registry.GetByID(idCard)
			p.Hand().AddCard(idCard)
			state := baseaction.CalculatePlayerCardState(card, p, g, registry)
			testutil.AssertEqual(t, tc.city && tc.colony, state.Available(), "playability agrees with requirements")
			play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
			err := play.Execute(ctx, g.ID(), id, idCard, shared.NativePayment(shared.ResourceCredit, 6), nil, []string{testutil.CardID("Regolith Eaters")}, nil, nil, nil)
			if tc.city && tc.colony {
				testutil.AssertNoError(t, err, "opponent's city and colony satisfy requirements")
				testutil.AssertEqual(t, 1, p.Resources().Production().Plants, "plant production reward")
			} else {
				testutil.AssertTrue(t, err != nil, "execution rejects missing requirement")
				testutil.AssertEqual(t, 6, p.Resources().Get().Credits, "no spending on rejection")
			}
		})
	}
}
