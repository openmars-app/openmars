package tiles_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"openmars/internal/action/confirmation"
	tileAction "openmars/internal/action/tile"
	"openmars/internal/delivery/dto"
	"openmars/internal/game"
	"openmars/internal/game/board"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func TestGreeneryPlacement_OceansDoNotCountAsOwnedTiles(t *testing.T) {
	for _, tc := range []struct {
		name         string
		ownedType    shared.ResourceType
		onOceanSpace bool
		surrounded   bool
	}{
		{name: "only oceans"},
		{name: "city", ownedType: shared.ResourceCityTile},
		{name: "greenery", ownedType: shared.ResourceGreeneryTile},
		{name: "special tile", ownedType: shared.ResourceIndustrialCenterTile},
		{name: "greenery on ocean space", ownedType: shared.ResourceGreeneryTile, onOceanSpace: true},
		{name: "no space beside owned tile", ownedType: shared.ResourceCityTile, surrounded: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ownedType := tc.ownedType
			ctx := context.Background()
			g, repo := testutil.CreateTestGameWithPlayers(t, 2)
			testutil.StartTestGame(t, g)
			id := g.TurnOrder()[0]
			var ocean shared.HexPosition
			for _, tile := range g.Board().Tiles() {
				if tile.Type == shared.ResourceOceanSpace {
					ocean = tile.Coordinates
					break
				}
			}
			testutil.AssertNoError(t, g.Board().UpdateTileOccupancy(ctx, ocean, board.TileOccupant{Type: shared.ResourceOceanTile}, id), "place player's ocean")
			land := g.CalculateAvailableHexesForTile("greenery", id, nil)
			var owned shared.HexPosition
			if ownedType != "" {
				// Choose a separate owned tile with room for greenery.
				found := false
				for _, tile := range g.Board().Tiles() {
					eligible := slices.Contains(land, tile.Coordinates.String()) && !slices.Contains(ocean.GetNeighbors(), tile.Coordinates)
					if tc.onOceanSpace {
						eligible = tile.Type == shared.ResourceOceanSpace && tile.OccupiedBy == nil
					}
					if !eligible {
						continue
					}
					for _, neighbor := range tile.Coordinates.GetNeighbors() {
						if slices.Contains(land, neighbor.String()) {
							owned, found = tile.Coordinates, true
							break
						}
					}
					if found {
						break
					}
				}
				testutil.AssertTrue(t, found, "fixture has land with available neighbors")
				testutil.AssertNoError(t, g.Board().UpdateTileOccupancy(ctx, owned, board.TileOccupant{Type: ownedType}, id), "place owned tile")
			}
			if tc.surrounded {
				for _, tile := range g.Board().Tiles() {
					if tile.OccupiedBy == nil && slices.Contains(owned.GetNeighbors(), tile.Coordinates) {
						testutil.AssertNoError(t, g.Board().UpdateTileOccupancy(ctx, tile.Coordinates, board.TileOccupant{Type: shared.ResourceCityTile}, g.TurnOrder()[1]), "block owned tile with opponent tiles")
					}
				}
			}
			restrictions := &shared.TileRestrictions{AdjacentToOwned: true}
			available := g.CalculateAvailableHexesForTile("greenery", id, restrictions)
			unrestricted := g.CalculateAvailableHexesForTile("greenery", id, nil)
			expected := []string{}
			var oceanOnly string
			for _, tile := range g.Board().Tiles() {
				if !slices.Contains(unrestricted, tile.Coordinates.String()) {
					continue
				}
				adjacentOwned := slices.Contains(owned.GetNeighbors(), tile.Coordinates)
				if ownedType == "" || tc.surrounded || adjacentOwned {
					expected = append(expected, tile.Coordinates.String())
				}
				if !adjacentOwned && slices.Contains(ocean.GetNeighbors(), tile.Coordinates) {
					oceanOnly = tile.Coordinates.String()
				}
			}
			if !slices.Equal(expected, available) {
				t.Fatalf("only non-ocean ownership should restrict greenery placement: want %v, got %v", expected, available)
			}
			testutil.AssertTrue(t, len(available) > 0, "valid greenery placements remain")
			selection := &shared.PendingTileSelection{TileType: "greenery", AvailableHexes: available, TileRestrictions: restrictions, Source: "test"}
			selectTile := tileAction.NewSelectTileAction(repo, testutil.CreateTestCardRegistry(), game.NewInMemoryGameStateRepository(), testutil.TestLogger())
			if ownedType != "" && !tc.surrounded {
				testutil.AssertTrue(t, oceanOnly != "", "fixture includes land next to only the ocean")
				// Even a stale pre-fix offer must be rejected by execution validation.
				selection.AvailableHexes = append(slices.Clone(available), oceanOnly)
				testutil.AssertNoError(t, g.SetPendingTileSelection(ctx, id, selection), "set stale placement offer")
				before := g.GlobalParameters().Oxygen()
				_, err := selectTile.Execute(ctx, g.ID(), id, oceanOnly)
				testutil.AssertErrorContains(t, err, "no longer valid", "reject ocean-only adjacency")
				testutil.AssertEqual(t, before, g.GlobalParameters().Oxygen(), "rejection does not raise oxygen")
			}
			testutil.AssertNoError(t, g.SetPendingTileSelection(ctx, id, selection), "set greenery placement offer")
			_, err := selectTile.Execute(ctx, g.ID(), id, available[0])
			testutil.AssertNoError(t, err, "accept valid greenery placement")
		})
	}
}

func TestSelectTileAction_BonusesRemovedAfterClaim(t *testing.T) {
	ctx := context.Background()
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	cardRegistry := testutil.CreateTestCardRegistry()
	stateRepo := game.NewInMemoryGameStateRepository()

	// Start the game to get to action phase
	testutil.StartTestGame(t, testGame)

	playerID := testGame.TurnOrder()[0]
	p, _ := testGame.GetPlayer(playerID)

	// Give player enough credits
	testutil.SetPlayerCredits(ctx, p, 100)

	// Find a tile with bonuses on the board
	tiles := testGame.Board().Tiles()
	var bonusTileCoords *shared.HexPosition
	var expectedBonus board.TileBonus
	for _, tile := range tiles {
		if len(tile.Bonuses) > 0 && tile.OccupiedBy == nil && tile.Type == shared.ResourceLandTile {
			bonusTileCoords = &tile.Coordinates
			expectedBonus = tile.Bonuses[0]
			break
		}
	}

	if bonusTileCoords == nil {
		t.Fatal("No tile with bonuses found on board")
	}

	// Record initial resources
	initialResources := getResourceAmount(p, expectedBonus.Type)

	// Set up pending tile selection for the player (simulating city placement)
	hexStr := formatHexCoords(*bonusTileCoords)
	pendingSelection := &shared.PendingTileSelection{
		TileType:       "city",
		AvailableHexes: []string{hexStr},
		Source:         "test",
	}
	testutil.AssertNoError(t, testGame.SetPendingTileSelection(ctx, playerID, pendingSelection), "set pending tile selection")

	// Execute tile placement
	selectTileAction := tileAction.NewSelectTileAction(repo, cardRegistry, stateRepo, logger)
	_, err := selectTileAction.Execute(ctx, testGame.ID(), playerID, hexStr)
	testutil.AssertNoError(t, err, "Failed to place tile")

	// Verify bonus was awarded to player
	finalResources := getResourceAmount(p, expectedBonus.Type)
	expectedFinal := initialResources + expectedBonus.Amount
	testutil.AssertEqual(t, expectedFinal, finalResources,
		"Player should have received bonus resources")

	// Verify bonus was removed from the tile
	placedTile, err := testGame.Board().GetTile(*bonusTileCoords)
	testutil.AssertNoError(t, err, "Failed to get placed tile")
	testutil.AssertEqual(t, 0, len(placedTile.Bonuses),
		"Tile bonuses should be cleared after placement")
}

func TestSelectTileAction_CardDrawBonusRemovedAfterClaim(t *testing.T) {
	ctx := context.Background()
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	cardRegistry := testutil.CreateTestCardRegistry()
	stateRepo := game.NewInMemoryGameStateRepository()

	// Start the game
	testutil.StartTestGame(t, testGame)

	playerID := testGame.TurnOrder()[0]
	p, _ := testGame.GetPlayer(playerID)

	// Find a tile with card draw bonus
	tiles := testGame.Board().Tiles()
	var cardDrawTileCoords *shared.HexPosition
	for _, tile := range tiles {
		for _, bonus := range tile.Bonuses {
			if bonus.Type == shared.ResourceCardDraw && tile.OccupiedBy == nil && tile.Type == shared.ResourceLandTile {
				cardDrawTileCoords = &tile.Coordinates
				break
			}
		}
		if cardDrawTileCoords != nil {
			break
		}
	}

	if cardDrawTileCoords == nil {
		t.Fatal("No tile with card draw bonus found on board")
	}

	// Record initial hand size
	initialHand := slices.Clone(p.Hand().Cards())
	initialHandSize := len(initialHand)

	// Set up pending tile selection
	hexStr := formatHexCoords(*cardDrawTileCoords)
	pendingSelection := &shared.PendingTileSelection{
		TileType:       "city",
		AvailableHexes: []string{hexStr},
		Source:         "test",
	}
	testutil.AssertNoError(t, testGame.SetPendingTileSelection(ctx, playerID, pendingSelection), "set pending tile selection")

	// Execute tile placement
	selectTileAction := tileAction.NewSelectTileAction(repo, cardRegistry, stateRepo, logger)
	_, err := selectTileAction.Execute(ctx, testGame.ID(), playerID, hexStr)
	testutil.AssertNoError(t, err, "Failed to place tile")

	// Verify cards were drawn
	finalHandSize := len(p.Hand().Cards())
	testutil.AssertTrue(t, finalHandSize > initialHandSize,
		"Player should have received card draw bonus")
	receipts := p.Selection().CardReceipts()
	testutil.AssertEqual(t, 1, len(receipts), "tile reward opens one received-cards receipt")
	testutil.AssertEqual(t, "Tile bonus", receipts[0].Source, "receipt identifies the placement bonus")
	testutil.AssertEqual(t, finalHandSize-initialHandSize, len(receipts[0].Cards), "receipt includes every awarded card")
	for _, cardID := range receipts[0].Cards {
		testutil.AssertTrue(t, slices.Contains(p.Hand().Cards(), cardID), "received card is already in hand")
		testutil.AssertTrue(t, !slices.Contains(initialHand, cardID), "receipt contains only newly drawn cards")
	}
	testutil.AssertEqual(t, 1, len(dto.ToGameDto(testGame, cardRegistry, playerID).CurrentPlayer.CardReceipts), "owner receives the modal data")
	otherID := testGame.TurnOrder()[1]
	testutil.AssertEqual(t, 0, len(dto.ToGameDto(testGame, cardRegistry, otherID).CurrentPlayer.CardReceipts), "opponent cannot see the received cards")
	testutil.AssertEqual(t, 0, len(dto.ToGameDto(testGame, cardRegistry, "").CurrentPlayer.CardReceipts), "spectator cannot see the received cards")
	ack := confirmation.NewConfirmCardDrawAction(repo, cardRegistry, logger)
	testutil.AssertNoError(t, ack.AcknowledgeReceipt(ctx, testGame.ID(), otherID, receipts[0].ID), "other player dismissal")
	testutil.AssertEqual(t, 1, len(p.Selection().CardReceipts()), "other player cannot dismiss receipt")
	actionsRemaining := testGame.CurrentTurn().ActionsRemaining()
	for range 2 {
		testutil.AssertNoError(t, ack.AcknowledgeReceipt(ctx, testGame.ID(), playerID, receipts[0].ID), "dismiss received cards")
	}
	testutil.AssertEqual(t, 0, len(p.Selection().CardReceipts()), "receipt dismissed")
	testutil.AssertEqual(t, finalHandSize, len(p.Hand().Cards()), "dismissal never grants duplicate cards")
	testutil.AssertEqual(t, actionsRemaining, testGame.CurrentTurn().ActionsRemaining(), "dismissal consumes no action")

	// Verify bonus was removed from the tile
	placedTile, err := testGame.Board().GetTile(*cardDrawTileCoords)
	testutil.AssertNoError(t, err, "Failed to get placed tile")
	testutil.AssertEqual(t, 0, len(placedTile.Bonuses),
		"Card draw bonus should be cleared after placement")
}

func TestSelectTileAction_OceanAdjacencyBonus(t *testing.T) {
	ctx := context.Background()
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	cardRegistry := testutil.CreateTestCardRegistry()
	stateRepo := game.NewInMemoryGameStateRepository()

	testutil.StartTestGame(t, testGame)

	playerID := testGame.TurnOrder()[0]
	p, _ := testGame.GetPlayer(playerID)
	testutil.SetPlayerCredits(ctx, p, 100)

	// Place an ocean tile at (1,1,-2) directly on the board
	oceanCoords := shared.HexPosition{Q: 1, R: 1, S: -2}
	err := testGame.Board().UpdateTileOccupancy(ctx, oceanCoords,
		board.TileOccupant{Type: shared.ResourceOceanTile}, playerID)
	testutil.AssertNoError(t, err, "Should place ocean tile")

	// Record credits before placing the adjacent city
	initialCredits := p.Resources().Get().Credits

	// Place a city adjacent to the ocean at (2,0,-2) — neighbor of (1,1,-2)
	cityCoords := shared.HexPosition{Q: 2, R: 0, S: -2}
	hexStr := formatHexCoords(cityCoords)
	pendingSelection := &shared.PendingTileSelection{
		TileType:       "city",
		AvailableHexes: []string{hexStr},
		Source:         "test",
	}
	testutil.AssertNoError(t, testGame.SetPendingTileSelection(ctx, playerID, pendingSelection), "set pending tile selection")

	selectTileAction := tileAction.NewSelectTileAction(repo, cardRegistry, stateRepo, logger)
	_, err = selectTileAction.Execute(ctx, testGame.ID(), playerID, hexStr)
	testutil.AssertNoError(t, err, "Failed to place city tile")

	// Verify player received +2 M€ for one adjacent ocean
	finalCredits := p.Resources().Get().Credits
	testutil.AssertEqual(t, initialCredits+2, finalCredits,
		"Player should receive +2 M€ for one adjacent ocean tile")
}

func TestSelectTileAction_OceanAdjacencyBonus_MultipleOceans(t *testing.T) {
	ctx := context.Background()
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	cardRegistry := testutil.CreateTestCardRegistry()
	stateRepo := game.NewInMemoryGameStateRepository()

	testutil.StartTestGame(t, testGame)

	playerID := testGame.TurnOrder()[0]
	p, _ := testGame.GetPlayer(playerID)
	testutil.SetPlayerCredits(ctx, p, 100)

	// Place two ocean tiles adjacent to the same land tile (1,2,-3)
	// Ocean at (1,1,-2) — neighbor via East direction
	ocean1 := shared.HexPosition{Q: 1, R: 1, S: -2}
	err := testGame.Board().UpdateTileOccupancy(ctx, ocean1,
		board.TileOccupant{Type: shared.ResourceOceanTile}, playerID)
	testutil.AssertNoError(t, err, "Should place first ocean tile")

	// Ocean at (0,3,-3) — neighbor via West direction
	ocean2 := shared.HexPosition{Q: 0, R: 3, S: -3}
	err = testGame.Board().UpdateTileOccupancy(ctx, ocean2,
		board.TileOccupant{Type: shared.ResourceOceanTile}, playerID)
	testutil.AssertNoError(t, err, "Should place second ocean tile")

	initialCredits := p.Resources().Get().Credits

	// Place a greenery at (1,2,-3) — adjacent to both oceans
	greeneryCoords := shared.HexPosition{Q: 1, R: 2, S: -3}
	hexStr := formatHexCoords(greeneryCoords)
	pendingSelection := &shared.PendingTileSelection{
		TileType:       "greenery",
		AvailableHexes: []string{hexStr},
		Source:         "test",
	}
	testutil.AssertNoError(t, testGame.SetPendingTileSelection(ctx, playerID, pendingSelection), "set pending tile selection")

	selectTileAction := tileAction.NewSelectTileAction(repo, cardRegistry, stateRepo, logger)
	_, err = selectTileAction.Execute(ctx, testGame.ID(), playerID, hexStr)
	testutil.AssertNoError(t, err, "Failed to place greenery tile")

	// Verify player received +4 M€ for two adjacent oceans
	finalCredits := p.Resources().Get().Credits
	testutil.AssertEqual(t, initialCredits+4, finalCredits,
		"Player should receive +4 M€ for two adjacent ocean tiles")
}

func TestSelectTileAction_OceanAdjacencyBonus_OceanTileGetsBonus(t *testing.T) {
	ctx := context.Background()
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	cardRegistry := testutil.CreateTestCardRegistry()
	stateRepo := game.NewInMemoryGameStateRepository()

	testutil.StartTestGame(t, testGame)

	playerID := testGame.TurnOrder()[0]
	p, _ := testGame.GetPlayer(playerID)
	testutil.SetPlayerCredits(ctx, p, 100)

	// Place an ocean tile at (2,-1,-1)
	ocean1 := shared.HexPosition{Q: 2, R: -1, S: -1}
	err := testGame.Board().UpdateTileOccupancy(ctx, ocean1,
		board.TileOccupant{Type: shared.ResourceOceanTile}, playerID)
	testutil.AssertNoError(t, err, "Should place first ocean tile")

	initialCredits := p.Resources().Get().Credits

	// Place another ocean adjacent to the first at (3,-2,-1) — also an ocean space, neighbor via East
	ocean2Coords := shared.HexPosition{Q: 3, R: -2, S: -1}
	hexStr := formatHexCoords(ocean2Coords)
	pendingSelection := &shared.PendingTileSelection{
		TileType:       "ocean",
		AvailableHexes: []string{hexStr},
		Source:         "test",
	}
	testutil.AssertNoError(t, testGame.SetPendingTileSelection(ctx, playerID, pendingSelection), "set pending tile selection")

	selectTileAction := tileAction.NewSelectTileAction(repo, cardRegistry, stateRepo, logger)
	_, err = selectTileAction.Execute(ctx, testGame.ID(), playerID, hexStr)
	testutil.AssertNoError(t, err, "Failed to place ocean tile")

	// Ocean tiles SHOULD receive adjacency bonus (2 MC per adjacent ocean)
	finalCredits := p.Resources().Get().Credits
	testutil.AssertEqual(t, initialCredits+2, finalCredits,
		"Ocean tile placement should receive +2 M€ for one adjacent ocean tile")
}

// TestSelectTileAction_MultipleBonusesOnTile verifies that placing on a tile
// with multiple bonuses (e.g. temperature + credit, as on the Vastitas Borealis
// Nova north-pole tile) awards each bonus independently: the temperature step
// raises the global parameter and grants 1 TR, and the credit bonus is added
// to the player's resources.
func TestSelectTileAction_MultipleBonusesOnTile(t *testing.T) {
	ctx := context.Background()
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	cardRegistry := testutil.CreateTestCardRegistry()
	stateRepo := game.NewInMemoryGameStateRepository()

	testutil.StartTestGame(t, testGame)

	playerID := testGame.TurnOrder()[0]
	p, _ := testGame.GetPlayer(playerID)
	testutil.SetPlayerCredits(ctx, p, 100)

	// Find an unoccupied land tile and overwrite its bonuses to [temperature 1, credit 4]
	tiles := testGame.Board().Tiles()
	var targetIdx = -1
	for i, tile := range tiles {
		if tile.OccupiedBy == nil && tile.Type == shared.ResourceLandTile {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		t.Fatal("No unoccupied land tile found")
	}
	tiles[targetIdx].Bonuses = []board.TileBonus{
		{Type: shared.ResourceTemperature, Amount: 1},
		{Type: shared.ResourceCredit, Amount: 4},
	}
	targetCoords := tiles[targetIdx].Coordinates
	testutil.AssertNoError(t, testGame.Board().SetTiles(ctx, tiles), "set tiles")

	initialCredits := p.Resources().Get().Credits
	initialTR := p.Resources().TerraformRating()
	initialTemp := testGame.GlobalParameters().Temperature()

	hexStr := formatHexCoords(targetCoords)
	pendingSelection := &shared.PendingTileSelection{
		TileType:       "city",
		AvailableHexes: []string{hexStr},
		Source:         "test",
	}
	testutil.AssertNoError(t, testGame.SetPendingTileSelection(ctx, playerID, pendingSelection), "set pending tile selection")

	selectTileAction := tileAction.NewSelectTileAction(repo, cardRegistry, stateRepo, logger)
	_, err := selectTileAction.Execute(ctx, testGame.ID(), playerID, hexStr)
	testutil.AssertNoError(t, err, "Failed to place tile")

	testutil.AssertEqual(t, initialTemp+2, testGame.GlobalParameters().Temperature(),
		"Temperature should rise by 2°C (1 step)")
	testutil.AssertEqual(t, initialTR+1, p.Resources().TerraformRating(),
		"Player should gain 1 TR for the temperature step")
	testutil.AssertEqual(t, initialCredits+4, p.Resources().Get().Credits,
		"Player should gain 4 credits from the credit bonus")

	placedTile, err := testGame.Board().GetTile(targetCoords)
	testutil.AssertNoError(t, err, "Failed to get placed tile")
	testutil.AssertEqual(t, 0, len(placedTile.Bonuses),
		"Bonuses should be cleared after placement")
}

// TestSelectTileAction_OceanPlacementBonus verifies that placing on a tile
// with an `ocean-placement` bonus queues a follow-up ocean placement: after the
// initial tile is placed, the player has a new pending tile selection of type
// "ocean" so they can pick where to place it.
func TestSelectTileAction_OceanPlacementBonus(t *testing.T) {
	ctx := context.Background()
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	cardRegistry := testutil.CreateTestCardRegistry()
	stateRepo := game.NewInMemoryGameStateRepository()

	testutil.StartTestGame(t, testGame)

	playerID := testGame.TurnOrder()[0]
	p, _ := testGame.GetPlayer(playerID)
	testutil.SetPlayerCredits(ctx, p, 100)

	tiles := testGame.Board().Tiles()
	var targetIdx = -1
	for i, tile := range tiles {
		if tile.OccupiedBy == nil && tile.Type == shared.ResourceLandTile {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		t.Fatal("No unoccupied land tile found")
	}
	tiles[targetIdx].Bonuses = []board.TileBonus{
		{Type: shared.ResourceOceanPlacement, Amount: 1},
	}
	targetCoords := tiles[targetIdx].Coordinates
	testutil.AssertNoError(t, testGame.Board().SetTiles(ctx, tiles), "set tiles")

	hexStr := formatHexCoords(targetCoords)
	pendingSelection := &shared.PendingTileSelection{
		TileType:       "city",
		AvailableHexes: []string{hexStr},
		Source:         "test",
	}
	testutil.AssertNoError(t, testGame.SetPendingTileSelection(ctx, playerID, pendingSelection), "set pending tile selection")

	selectTileAction := tileAction.NewSelectTileAction(repo, cardRegistry, stateRepo, logger)
	_, err := selectTileAction.Execute(ctx, testGame.ID(), playerID, hexStr)
	testutil.AssertNoError(t, err, "Failed to place tile")

	pending := testGame.GetPendingTileSelection(playerID)
	if pending == nil {
		t.Fatal("expected a follow-up pending tile selection for ocean placement, got nil")
	}
	testutil.AssertEqual(t, "ocean", pending.TileType,
		"Follow-up pending selection should be an ocean placement")

	placedTile, err := testGame.Board().GetTile(targetCoords)
	testutil.AssertNoError(t, err, "Failed to get placed tile")
	testutil.AssertEqual(t, 0, len(placedTile.Bonuses),
		"Bonuses should be cleared after placement")
}

// TestSelectTileAction_NegativeBonusBlocksUnaffordablePlacement verifies that
// placing on a tile whose bonuses would push a basic resource below zero is
// rejected before any board mutation. Mirrors the negative-output gate used
// for card play.
func TestSelectTileAction_NegativeBonusBlocksUnaffordablePlacement(t *testing.T) {
	ctx := context.Background()
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	cardRegistry := testutil.CreateTestCardRegistry()
	stateRepo := game.NewInMemoryGameStateRepository()

	testutil.StartTestGame(t, testGame)

	playerID := testGame.TurnOrder()[0]
	p, _ := testGame.GetPlayer(playerID)
	testutil.SetPlayerCredits(ctx, p, 2)

	tiles := testGame.Board().Tiles()
	var targetIdx = -1
	for i, tile := range tiles {
		if tile.OccupiedBy == nil && tile.Type == shared.ResourceLandTile {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		t.Fatal("No unoccupied land tile found")
	}
	tiles[targetIdx].Bonuses = []board.TileBonus{
		{Type: shared.ResourceCredit, Amount: -6},
	}
	targetCoords := tiles[targetIdx].Coordinates
	testutil.AssertNoError(t, testGame.Board().SetTiles(ctx, tiles), "set tiles")

	hexStr := formatHexCoords(targetCoords)
	pendingSelection := &shared.PendingTileSelection{
		TileType:       "city",
		AvailableHexes: []string{hexStr},
		Source:         "test",
	}
	testutil.AssertNoError(t, testGame.SetPendingTileSelection(ctx, playerID, pendingSelection), "set pending tile selection")

	selectTileAction := tileAction.NewSelectTileAction(repo, cardRegistry, stateRepo, logger)
	_, err := selectTileAction.Execute(ctx, testGame.ID(), playerID, hexStr)
	if err == nil {
		t.Fatal("expected placement to be rejected when player cannot afford negative bonus")
	}

	placedTile, getErr := testGame.Board().GetTile(targetCoords)
	testutil.AssertNoError(t, getErr, "Failed to get tile")
	if placedTile.OccupiedBy != nil {
		t.Fatal("Tile should not have been placed when affordability check fails")
	}
	testutil.AssertEqual(t, 1, len(placedTile.Bonuses),
		"Bonus should still be on the tile (placement rejected)")
	testutil.AssertEqual(t, 2, p.Resources().Get().Credits,
		"Player credits should be unchanged")
}

// TestSelectTileAction_NegativeBonusAppliedWhenAffordable verifies that the
// affordability gate lets the placement proceed when the player has enough.
func TestSelectTileAction_NegativeBonusAppliedWhenAffordable(t *testing.T) {
	ctx := context.Background()
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	cardRegistry := testutil.CreateTestCardRegistry()
	stateRepo := game.NewInMemoryGameStateRepository()

	testutil.StartTestGame(t, testGame)

	playerID := testGame.TurnOrder()[0]
	p, _ := testGame.GetPlayer(playerID)
	testutil.SetPlayerCredits(ctx, p, 10)

	tiles := testGame.Board().Tiles()
	var targetIdx = -1
	for i, tile := range tiles {
		if tile.OccupiedBy == nil && tile.Type == shared.ResourceLandTile {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		t.Fatal("No unoccupied land tile found")
	}
	tiles[targetIdx].Bonuses = []board.TileBonus{
		{Type: shared.ResourceCredit, Amount: -6},
	}
	targetCoords := tiles[targetIdx].Coordinates
	testutil.AssertNoError(t, testGame.Board().SetTiles(ctx, tiles), "set tiles")

	hexStr := formatHexCoords(targetCoords)
	pendingSelection := &shared.PendingTileSelection{
		TileType:       "city",
		AvailableHexes: []string{hexStr},
		Source:         "test",
	}
	testutil.AssertNoError(t, testGame.SetPendingTileSelection(ctx, playerID, pendingSelection), "set pending tile selection")

	selectTileAction := tileAction.NewSelectTileAction(repo, cardRegistry, stateRepo, logger)
	_, err := selectTileAction.Execute(ctx, testGame.ID(), playerID, hexStr)
	testutil.AssertNoError(t, err, "Affordable negative bonus should not block placement")

	testutil.AssertEqual(t, 4, p.Resources().Get().Credits,
		"Player credits should drop by 6 (10 - 6 = 4)")
}

func getResourceAmount(p *player.Player, resourceType shared.ResourceType) int {
	resources := p.Resources().Get()
	switch resourceType {
	case shared.ResourceSteel:
		return resources.Steel
	case shared.ResourceTitanium:
		return resources.Titanium
	case shared.ResourcePlant:
		return resources.Plants
	case shared.ResourceCredit:
		return resources.Credits
	default:
		return 0
	}
}

func formatHexCoords(pos shared.HexPosition) string {
	return fmt.Sprintf("%d,%d,%d", pos.Q, pos.R, pos.S)
}
