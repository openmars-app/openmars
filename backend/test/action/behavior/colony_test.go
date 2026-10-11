package behavior_test

import (
	"context"
	"openmars/internal/action/confirmation"
	gamecards "openmars/internal/game/cards"
	"testing"

	"openmars/internal/game/colony"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func TestColony_PlacementCreatesPendingSelection(t *testing.T) {
	testGame, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)

	p, _ := testGame.GetPlayer(playerID)

	// Enable colonies expansion
	settings := testGame.Settings()
	settings.Systems = append(settings.Systems, "colonies")
	testGame.UpdateSettings(context.Background(), settings)

	// Set up colony tiles on the game
	testGame.Colonies().SetStates([]*colony.ColonyState{
		{DefinitionID: "luna", MarkerPosition: 1, PlayerColonies: nil},
		{DefinitionID: "europa", MarkerPosition: 1, PlayerColonies: nil},
	})

	output := shared.NewColonyCondition(shared.ResourceColony, 1, "none")
	applyOutputs(t, p, testGame, cardRegistry, output)

	selection := p.Selection().GetPendingColonySelection()
	testutil.AssertTrue(t, selection != nil, "Should have a pending colony selection")
	testutil.AssertTrue(t, len(selection.AvailableColonyIDs) >= 2, "Should have at least 2 available colonies")
}

func TestColony_AllowDuplicatePlayerColony(t *testing.T) {
	testGame, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)

	p, _ := testGame.GetPlayer(playerID)

	// Enable colonies expansion
	settings := testGame.Settings()
	settings.Systems = append(settings.Systems, "colonies")
	testGame.UpdateSettings(context.Background(), settings)

	// Player already has a colony on luna
	testGame.Colonies().SetStates([]*colony.ColonyState{
		{DefinitionID: "luna", MarkerPosition: 1, PlayerColonies: []string{playerID}},
		{DefinitionID: "europa", MarkerPosition: 1, PlayerColonies: nil},
	})

	output := &shared.ColonyCondition{
		ConditionBase:              shared.ConditionBase{ResourceType: shared.ResourceColony, Amount: 1, Target: "none"},
		AllowDuplicatePlayerColony: true,
	}
	applyOutputs(t, p, testGame, cardRegistry, output)

	selection := p.Selection().GetPendingColonySelection()
	testutil.AssertTrue(t, selection != nil, "Should have a pending colony selection")

	// Verify luna is still in the available list despite player already having a colony there
	foundLuna := false
	for _, id := range selection.AvailableColonyIDs {
		if id == "luna" {
			foundLuna = true
			break
		}
	}
	testutil.AssertTrue(t, foundLuna, "Luna should be available even though player already has a colony there")
}

func TestColony_PlacementNoColoniesAvailable(t *testing.T) {
	testGame, _, cardRegistry, playerID, otherPlayerID := testutil.SetupTwoPlayerGame(t)

	p, _ := testGame.GetPlayer(playerID)

	// Enable colonies expansion
	settings := testGame.Settings()
	settings.Systems = append(settings.Systems, "colonies")
	testGame.UpdateSettings(context.Background(), settings)

	// Set up all colony tiles as fully colonized (3 colonies each = max)
	testGame.Colonies().SetStates([]*colony.ColonyState{
		{DefinitionID: "luna", MarkerPosition: 1, PlayerColonies: []string{playerID, otherPlayerID, playerID}},
		{DefinitionID: "europa", MarkerPosition: 1, PlayerColonies: []string{playerID, otherPlayerID, otherPlayerID}},
	})

	output := shared.NewColonyCondition(shared.ResourceColony, 1, "none")
	applyOutputs(t, p, testGame, cardRegistry, output)

	// No colonies available, so no pending selection should be created
	selection := p.Selection().GetPendingColonySelection()
	testutil.AssertTrue(t, selection == nil, "Should have no pending colony selection when all colonies are full")
}

func TestColonyTileAddResolvesMultipleTilesWithoutBuildingColonies(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	settings := g.Settings()
	settings.Systems = append(settings.Systems, "colonies")
	g.UpdateSettings(ctx, settings)
	definitions := []colony.ColonyDefinition{{ID: "first", Name: "First"}, {ID: "second", Name: "Second"}}
	g.Colonies().SetDefinitions(definitions)
	p, _ := g.GetPlayer(id)
	testutil.AssertNoError(t, g.SetForcedFirstAction(ctx, id, &shared.ForcedFirstAction{State: "resolving"}), "start effect")
	applyOutputs(t, p, g, registry, shared.NewColonyCondition(shared.ResourceColonyTileAdd, 2, "none"))
	confirm := confirmation.NewConfirmColonyPlacementAction(repo, registry, colony.NewInMemoryColonyRegistry(definitions), testutil.TestLogger())
	testutil.AssertNoError(t, confirm.Execute(ctx, g.ID(), id, "first"), "add first tile")
	testutil.AssertEqual(t, 2, g.CurrentTurn().ActionsRemaining(), "second tile still pending")
	testutil.AssertTrue(t, p.Selection().GetPendingColonySelection() != nil, "next selection queued")
	testutil.AssertNoError(t, confirm.Execute(ctx, g.ID(), id, "second"), "add second tile")
	testutil.AssertEqual(t, 2, len(g.Colonies().States()), "both tiles added")
	testutil.AssertEqual(t, 0, g.Colonies().CountAllColonies(), "no player colonies")
	testutil.AssertEqual(t, 1, g.CurrentTurn().ActionsRemaining(), "only one action consumed")
}

func TestResourceDependentColonyInitializationAndActivation(t *testing.T) {
	for _, resource := range []shared.ResourceType{shared.ResourceFloater, shared.ResourceAnimal, shared.ResourceMicrobe} {
		t.Run(string(resource), func(t *testing.T) {
			g, _, _, id, _ := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			definition := colony.ColonyDefinition{ID: "resource-tile", ActivationResource: string(resource)}
			holder := gamecards.Card{ID: "holder", ResourceStorage: &gamecards.ResourceStorage{Type: resource}}
			registry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{holder})
			state := gamecards.InitializeColonyTile(g, definition, registry)
			g.Colonies().SetStates([]*colony.ColonyState{state})
			testutil.AssertEqual(t, -1, state.MarkerPosition, "inactive marker")
			testutil.AssertEqual(t, 0, len(g.Colonies().GetPlaceableIDs(id, false)), "cannot build on inactive tile")
			testutil.AssertEqual(t, 0, len(g.Colonies().GetTradeableIDs()), "cannot trade on inactive tile")
			g.Colonies().ActivateResource("heat")
			testutil.AssertEqual(t, string(resource), state.AwaitingResource, "unrelated resource cannot activate")
			g.Colonies().ActivateResource(string(resource))
			testutil.AssertEqual(t, 1, state.MarkerPosition, "activation starts trade track")
			testutil.AssertEqual(t, 1, len(g.Colonies().GetPlaceableIDs(id, false)), "active tile accepts colonies")
			testutil.AssertEqual(t, 1, len(g.Colonies().GetTradeableIDs()), "active tile accepts trades")
			p.PlayedCards().SetCards([]string{holder.ID})
			ready := gamecards.InitializeColonyTile(g, definition, registry)
			testutil.AssertEqual(t, "", ready.AwaitingResource, "holder already in play activates new tile immediately")
		})
	}
}
