package tiles_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	tileAction "openmars/internal/action/tile"
	"openmars/internal/delivery/dto"
	"openmars/internal/game"
	"openmars/internal/game/board"
	"openmars/internal/game/cards"
	"openmars/internal/game/datastore"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func TestTileVisualPlacementAndViewerConsistency(t *testing.T) {
	ctx := context.Background()
	g, repo := testutil.CreateTestGameWithPlayers(t, 2)
	testutil.StartTestGame(t, g)
	playerID := g.TurnOrder()[0]
	data, err := os.ReadFile("../../../assets/cards.json")
	if err != nil {
		t.Fatal(err)
	}
	var definitions []cards.Card
	if err = json.Unmarshal(data, &definitions); err != nil {
		t.Fatal(err)
	}
	registry := cards.NewInMemoryCardRegistry(definitions)
	for _, id := range []string{"020", "021", "008", "017", "029", "120", "032", "108"} {
		card, err := registry.GetByID(id)
		if err != nil || card.Style == nil || card.Style.Tile == nil {
			t.Fatalf("Missing card style %s: %v", id, err)
		}
		if dto.ToCardDto(*card).Style.Tile == nil {
			t.Fatal("Card DTO lost style")
		}
		if id == "008" && (card.Style.Tile.Plan != "grid" || card.Style.Tile.Perimeter != "low-wall" || card.Style.Tile.Landmark != "hall" || card.Style.Tile.Landscaping != "lush") {
			t.Fatalf("Capital lost its green civic layout: %+v", card.Style.Tile)
		}
	}
	var coordinate shared.HexPosition
	found := false
	for _, tile := range g.Board().Tiles() {
		if tile.Type == shared.ResourceLandTile && tile.OccupiedBy == nil && len(tile.Bonuses) == 0 {
			coordinate = tile.Coordinates
			found = true
			break
		}
	}
	if !found {
		t.Fatal("No empty land tile")
	}
	if err = g.SetPendingTileSelection(ctx, playerID, &shared.PendingTileSelection{TileType: "city", AvailableHexes: []string{coordinate.String()}, Source: "card", SourceCardID: "032"}); err != nil {
		t.Fatal(err)
	}
	action := tileAction.NewSelectTileAction(repo, registry, game.NewInMemoryGameStateRepository(), testutil.TestLogger())
	if _, err = action.Execute(ctx, g.ID(), playerID, coordinate.String()); err != nil {
		t.Fatal(err)
	}
	placed, err := g.Board().GetTile(coordinate)
	if err != nil {
		t.Fatal(err)
	}
	visual := placed.OccupiedBy.Visual
	if placed.OccupiedBy.DisplayName != "Underground City" {
		t.Fatalf("City lost its card name: %q", placed.OccupiedBy.DisplayName)
	}
	if visual == nil || visual.Seed == 0 || visual.Structure == nil || visual.Structure.Cover != "flat-glass" {
		t.Fatalf("Invalid visual: %+v", visual)
	}
	expected := visual.Clone()
	visual.Structure.Details[0] = "mutated"
	again, _ := g.Board().GetTile(coordinate)
	if !reflect.DeepEqual(again.OccupiedBy.Visual, expected) {
		t.Fatal("Board read mutated saved appearance")
	}
	card, _ := registry.GetByID("032")
	card.Style.Tile.Cover = "dome"
	card.Name = "Renamed card"
	again, _ = g.Board().GetTile(coordinate)
	if again.OccupiedBy.Visual.Structure.Cover != "flat-glass" {
		t.Fatal("Card request aliases placed appearance")
	}
	playerView := dto.ToGameDto(g, registry, playerID)
	spectatorView := dto.ToSpectatorGameDto(g, registry, nil, nil)
	history := dto.ToGameHistoryEntryDtos([]*datastore.GameStateHistoryEntry{{State: &datastore.GameState{Tiles: g.Board().Tiles()}}}, nil)
	var views []*dto.TileVisualDto
	for _, tiles := range [][]dto.TileDto{playerView.Board.Tiles, spectatorView.Board.Tiles, history[0].Board.Tiles} {
		for _, tile := range tiles {
			if tile.Coordinates.Q == coordinate.Q && tile.Coordinates.R == coordinate.R {
				views = append(views, tile.OccupiedBy.Visual)
				if tile.OccupiedBy.DisplayName != "Underground City" {
					t.Fatalf("Viewer lost the placement-time name: %q", tile.OccupiedBy.DisplayName)
				}
			}
		}
	}
	if len(views) != 3 || !reflect.DeepEqual(views[0], views[1]) || !reflect.DeepEqual(views[0], views[2]) {
		t.Fatal("Viewers disagree on visual specification")
	}
	if err = g.Board().ClearTileOccupant(ctx, coordinate); err != nil {
		t.Fatal(err)
	}
	cleared, _ := g.Board().GetTile(coordinate)
	if cleared.OccupiedBy != nil {
		t.Fatal("Cleared tile kept appearance")
	}
	if err = g.Board().UpdateTileOccupancy(ctx, coordinate, board.TileOccupant{Type: shared.ResourceCityTile}, playerID); err != nil {
		t.Fatal(err)
	}
	standard, _ := g.Board().GetTile(coordinate)
	if standard.OccupiedBy.DisplayName != "" {
		t.Fatal("Standard city retained card name")
	}
	if standard.OccupiedBy.Visual == nil || standard.OccupiedBy.Visual.Structure != nil {
		t.Fatal("Standard city retained card request")
	}
	seed := standard.OccupiedBy.Visual.Seed
	if err = g.Board().UpdateTileOccupancy(ctx, coordinate, board.TileOccupant{Type: shared.ResourceCityTile}, playerID); err != nil {
		t.Fatal(err)
	}
	repeated, _ := g.Board().GetTile(coordinate)
	if repeated.OccupiedBy.Visual.Seed != seed {
		t.Fatal("Seed is not deterministic")
	}
}

func TestPhobosCityUsesDomeAppearance(t *testing.T) {
	ctx := context.Background()
	g, repo := testutil.CreateTestGameWithPlayers(t, 1)
	testutil.StartTestGame(t, g)
	playerID := g.TurnOrder()[0]
	registry := testutil.CreateTestCardRegistry()
	for _, tile := range g.Board().Tiles() {
		if tile.Location != board.TileLocationPhobos {
			continue
		}
		selection := &shared.PendingTileSelection{TileType: "city", AvailableHexes: []string{tile.Coordinates.String()}, Source: "card", SourceCardID: "021", TileRestrictions: &shared.TileRestrictions{BoardTags: []string{"phobos-space-haven"}}}
		if err := g.SetPendingTileSelection(ctx, playerID, selection); err != nil {
			t.Fatal(err)
		}
		action := tileAction.NewSelectTileAction(repo, registry, game.NewInMemoryGameStateRepository(), testutil.TestLogger())
		if _, err := action.Execute(ctx, g.ID(), playerID, tile.Coordinates.String()); err != nil {
			t.Fatal(err)
		}
		placed, err := g.Board().GetTile(tile.Coordinates)
		if err != nil {
			t.Fatal(err)
		}
		visual := placed.OccupiedBy.Visual
		if placed.OccupiedBy.DisplayName != "Phobos Space Haven" {
			t.Fatalf("Phobos lost its card name: %q", placed.OccupiedBy.DisplayName)
		}
		if visual == nil || visual.Structure == nil || visual.Structure.Cover != "dome" || visual.Structure.Landscaping != "sparse" {
			t.Fatalf("Phobos lost its sealed habitat appearance: %+v", visual)
		}
		return
	}
	t.Fatal("Phobos tile is missing")
}

func TestCityPlacementNamesWithoutCustomModels(t *testing.T) {
	for _, tt := range []struct {
		name        string
		sourceID    string
		cardType    cards.CardType
		replacement string
		wantName    string
	}{
		{name: "project", sourceID: "named-city", cardType: cards.CardTypeAutomated, wantName: "Named City"},
		{name: "prelude", sourceID: "named-city", cardType: cards.CardTypePrelude, wantName: "Named City"},
		{name: "standard project"},
		{name: "unknown source", sourceID: "missing"},
		{name: "named replacement", sourceID: "named-city", replacement: "city", wantName: "Named City"},
		{name: "unnamed replacement", replacement: "city"},
		{name: "non-city replacement", sourceID: "named-city", replacement: "greenery"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			g, repo := testutil.CreateTestGameWithPlayers(t, 1)
			testutil.StartTestGame(t, g)
			playerID := g.TurnOrder()[0]
			registry := cards.NewInMemoryCardRegistry([]cards.Card{{ID: "named-city", Name: "Named City", Type: tt.cardType}})
			var coordinate shared.HexPosition
			found := false
			for _, tile := range g.Board().Tiles() {
				if tile.Type == shared.ResourceLandTile && tile.OccupiedBy == nil && len(tile.Bonuses) == 0 && len(tile.Tags) == 0 {
					coordinate = tile.Coordinates
					found = true
					break
				}
			}
			if !found {
				t.Fatal("No empty land tile")
			}
			var output shared.BehaviorCondition = shared.NewTilePlacementCondition(shared.ResourceCityPlacement, 1, "none")
			if tt.replacement != "" {
				if err := g.Board().UpdateTileOccupancy(ctx, coordinate, board.TileOccupant{Type: shared.ResourceCityTile, DisplayName: "Previous City"}, playerID); err != nil {
					t.Fatal(err)
				}
				output = &shared.TileModificationCondition{
					ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceTileReplacement, Amount: 1, Target: "none"},
					TileType:      tt.replacement,
				}
			}
			p, err := g.GetPlayer(playerID)
			if err != nil {
				t.Fatal(err)
			}
			applier := cards.NewBehaviorApplier(p, g, "card", testutil.TestLogger()).WithSourceCardID(tt.sourceID).WithCardRegistry(registry)
			if err := applier.ApplyOutputs(ctx, []shared.BehaviorCondition{output}); err != nil {
				t.Fatal(err)
			}
			selection := g.GetPendingTileSelection(playerID)
			if selection == nil || selection.SourceCardID != tt.sourceID {
				t.Fatalf("Placement lost its source card: %+v", selection)
			}
			action := tileAction.NewSelectTileAction(repo, registry, game.NewInMemoryGameStateRepository(), testutil.TestLogger())
			if _, err := action.Execute(ctx, g.ID(), playerID, coordinate.String()); err != nil {
				t.Fatal(err)
			}
			placed, err := g.Board().GetTile(coordinate)
			if err != nil {
				t.Fatal(err)
			}
			if placed.OccupiedBy.DisplayName != tt.wantName {
				t.Fatalf("Placed name = %q, want %q", placed.OccupiedBy.DisplayName, tt.wantName)
			}
			if placed.DisplayName != nil {
				t.Fatal("Placement overwrote the board-space name")
			}
		})
	}
}

func TestAuthoredStructurePlacement(t *testing.T) {
	for _, tt := range []struct {
		cardID, tileType string
		admin            bool
	}{
		{"123", "industrial-center", false}, {"123", "industrial-center", true},
		{"085", "commercial-district", false}, {"085", "commercial-district", true},
		{"P37", "city", false},
		{"182", "city", false},
	} {
		source := tt.cardID
		if tt.admin {
			source = ""
		}
		t.Run(tt.cardID+"/source="+source, func(t *testing.T) {
			ctx := context.Background()
			g, repo := testutil.CreateTestGameWithPlayers(t, 2)
			testutil.StartTestGame(t, g)
			playerID := g.TurnOrder()[0]
			registry := testutil.CreateTestCardRegistry()
			card, err := registry.GetByID(tt.cardID)
			testutil.AssertNoError(t, err, "load authored card")
			var coordinate shared.HexPosition
			for _, tile := range g.Board().Tiles() {
				if tile.Type == shared.ResourceLandTile && tile.OccupiedBy == nil && len(tile.Bonuses) == 0 {
					coordinate = tile.Coordinates
					break
				}
			}
			selection := &shared.PendingTileSelection{TileType: tt.tileType, AvailableHexes: []string{coordinate.String()}, Source: "admin-tile-selection", SourceCardID: source}
			testutil.AssertNoError(t, g.SetPendingTileSelection(ctx, playerID, selection), "set placement")
			action := tileAction.NewSelectTileAction(repo, registry, game.NewInMemoryGameStateRepository(), testutil.TestLogger())
			_, err = action.Execute(ctx, g.ID(), playerID, coordinate.String())
			testutil.AssertNoError(t, err, "place structure")
			tile, err := g.Board().GetTile(coordinate)
			testutil.AssertNoError(t, err, "read tile")
			if tile.OccupiedBy.Type != shared.ResourceType(tt.tileType+"-tile") || tile.OccupiedBy.DisplayName != card.Name {
				t.Fatalf("Structure changed gameplay identity: %+v", tile.OccupiedBy)
			}
			visual := tile.OccupiedBy.Visual
			if visual == nil || visual.Seed == 0 || visual.Structure == nil || !reflect.DeepEqual(visual.Structure, card.Style.Tile) {
				t.Fatalf("Missing authored request: %+v", visual)
			}
			if source == "" && len(tile.OccupiedBy.Tags) != 0 {
				t.Fatal("Admin appearance must not fabricate a source-card tag")
			}
			expected := visual.Clone()
			visual.Structure.Details[0] = "mutated"
			again, _ := g.Board().GetTile(coordinate)
			if !reflect.DeepEqual(again.OccupiedBy.Visual, expected) {
				t.Fatal("Structure appearance aliases board state")
			}
			player := dto.ToGameDto(g, registry, playerID)
			spectator := dto.ToSpectatorGameDto(g, registry, nil, nil)
			history := dto.ToGameHistoryEntryDtos([]*datastore.GameStateHistoryEntry{{State: &datastore.GameState{Tiles: g.Board().Tiles()}}}, nil)
			for _, tiles := range [][]dto.TileDto{player.Board.Tiles, spectator.Board.Tiles, history[0].Board.Tiles} {
				for _, tile := range tiles {
					if tile.Coordinates.Q == coordinate.Q && tile.Coordinates.R == coordinate.R {
						if tile.OccupiedBy.Visual.Seed != expected.Seed || !reflect.DeepEqual(tile.OccupiedBy.Visual.Structure, dto.ToCardDto(*card).Style.Tile) {
							t.Fatal("Viewer lost structure appearance")
						}
						data, err := json.Marshal(tile.OccupiedBy.Visual)
						testutil.AssertNoError(t, err, "encode visual")
						var raw map[string]any
						testutil.AssertNoError(t, json.Unmarshal(data, &raw), "decode visual")
						if raw["city"] != nil || raw["structure"] == nil {
							t.Fatal("Visual wire contract must use only structure")
						}
					}
				}
			}
			testutil.AssertNoError(t, g.Board().ClearTileOccupant(ctx, coordinate), "clear structure")
			testutil.AssertNoError(t, g.Board().UpdateTileOccupancy(ctx, coordinate, board.TileOccupant{Type: shared.ResourceCityTile}, playerID), "replace with city")
			replaced, _ := g.Board().GetTile(coordinate)
			if replaced.OccupiedBy.Visual.Structure != nil {
				t.Fatal("Replacement retained authored request")
			}
		})
	}
}

func TestStructureDefaultRequiresUniqueAuthoredCard(t *testing.T) {
	registry := testutil.CreateTestCardRegistry()
	original := cards.UniqueStructureCard(registry, "industrial-center")
	if original == nil || original.ID != "123" {
		t.Fatal("Factory style not resolved from catalog")
	}
	duplicate := original.DeepCopy()
	duplicate.ID = "another-factory"
	ambiguous := cards.NewInMemoryCardRegistry([]cards.Card{*original, duplicate})
	if cards.UniqueStructureCard(ambiguous, "industrial-center") != nil {
		t.Fatal("Ambiguous styles must require a card source")
	}
}
