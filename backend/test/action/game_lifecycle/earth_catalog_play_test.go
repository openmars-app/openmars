package game_lifecycle_test

import (
	"context"
	"testing"

	"openmars/internal/action"
	cardAction "openmars/internal/action/card"
	gameAction "openmars/internal/action/game"
	"openmars/internal/game"
	"openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func earthPlayableFixture(t *testing.T) (*game.Game, *player.Player, *cardAction.PlayCardAction, cards.CardRegistry) {
	t.Helper()
	ctx := context.Background()
	registry := testutil.CreateTestModeCatalog()
	repo := testutil.NewTestGameRepository(t)
	create := gameAction.NewCreateGameAction(repo, registry, testutil.CreateTestMapRegistry(), testutil.TestLogger())
	g, err := create.Execute(ctx, shared.GameSettings{GameModeID: "earth-withdrawal", MaxPlayers: 1})
	if err != nil {
		t.Fatal(err)
	}
	p, err := g.AddNewPlayer(ctx, "p", "Player")
	if err != nil {
		t.Fatal(err)
	}
	p.SetCorporationID("B01")
	p.Resources().Set(shared.Resources{Credits: 1000, Steel: 20, Titanium: 20, Energy: 20})
	for _, err := range []error{g.UpdateStatus(ctx, shared.GameStatusActive), g.UpdatePhase(ctx, shared.GamePhaseAction), g.SetTurnOrder(ctx, []string{p.ID()}), g.SetCurrentTurn(ctx, p.ID(), -1)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	action.SetupPlayerCardStore(p, g, registry)
	action.SetupModeBehaviors(ctx, g, registry)
	return g, p, cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()), registry
}

func playEarthCard(t *testing.T, g *game.Game, p *player.Player, play *cardAction.PlayCardAction, registry cards.CardRegistry, id string, choice *int, targets []string) error {
	t.Helper()
	if !p.Hand().HasCard(id) {
		p.Hand().AddCard(id)
	}
	c, err := cards.ForGame(registry, g).GetByID(id)
	if err != nil {
		t.Fatal(err)
	}
	return play.Execute(context.Background(), g.ID(), p.ID(), id, shared.NativePayment(shared.ResourceCredit, c.Cost), choice, targets, nil, nil, nil)
}

func TestEarthPublishedSupplyImportAndReconnectionCards(t *testing.T) {
	g, p, play, registry := earthPlayableFixture(t)
	ctx := context.Background()
	for _, id := range []string{"ES001", "ES005", "ES004", "ES008", "ES019", "ES028"} {
		if err := playEarthCard(t, g, p, play, registry, id, nil, nil); err != nil {
			t.Fatalf("play %s: %v", id, err)
		}
	}
	if p.Resources().Production().Steel != 2 || p.Resources().Production().Energy != 3 || p.Resources().Production().Credits != -1 {
		t.Fatalf("signed source production wrong: %+v", p.Resources().Production())
	}
	if p.Resources().PermanentProduction().Energy != 0 {
		t.Fatal("leased modules became permanent without import")
	}
	if err := playEarthCard(t, g, p, play, registry, "ES021", nil, []string{"ES005"}); err != nil {
		t.Fatal(err)
	}
	if got := p.Resources().PermanentProduction(); got.Energy != 3 || got.Credits != -1 {
		t.Fatalf("whole signed package not imported: %+v", got)
	}
	if err := g.SetGeneration(ctx, 6); err != nil {
		t.Fatal(err)
	}
	g.ResolveModeBoundary(ctx, "production-end", false)
	if got := p.Resources().Production(); got.Steel != 0 || got.Energy != 3 || got.Credits != -1 {
		t.Fatalf("withdrawal/import mismatch: %+v", got)
	}
	if got := p.Resources().Get().Plants; got != 8 {
		t.Fatalf("withdrawal reserve payout = %d, want8", got)
	}
	if err := g.SetGeneration(ctx, 7); err != nil {
		t.Fatal(err)
	}
	if err := playEarthCard(t, g, p, play, registry, "ES003", nil, nil); err == nil {
		t.Fatal("pure external infrastructure playable while disconnected")
	}
	if err := playEarthCard(t, g, p, play, registry, "ES043", nil, nil); err != nil {
		t.Fatal(err)
	}
	before := p.Resources().Get()
	if err := playEarthCard(t, g, p, play, registry, "ES044", nil, nil); err == nil {
		t.Fatal("second production pulse accepted")
	}
	if p.Resources().Get() != before || !p.Hand().HasCard("ES044") {
		t.Fatal("rejected pulse charged or consumed card")
	}
	if err := playEarthCard(t, g, p, play, registry, "ES041", nil, nil); err != nil {
		t.Fatalf("nonoverlapping next generation reconnection: %v", err)
	}
	if active, _ := g.ModeStateValue("earth-contact"); active != 0 {
		t.Fatal("pulse created an action window")
	}
	g.ResolveModeBoundary(ctx, "production-start", false)
	if got := p.Resources().Production(); got.Steel != 2 || got.Energy != 3 || got.Credits != -1 {
		t.Fatalf("restored source/import doubled or missing: %+v", got)
	}
	g.ResolveModeBoundary(ctx, "production-end", false)
	if err := g.SetGeneration(ctx, 8); err != nil {
		t.Fatal(err)
	}
	g.ResolveModeBoundary(ctx, "research-start", false)
	if active, _ := g.ModeStateValue("earth-contact"); active != 1 {
		t.Fatal("full reconnection absent at research")
	}
	if err := playEarthCard(t, g, p, play, registry, "ES003", nil, nil); err != nil {
		t.Fatalf("could not install infrastructure during full reconnection: %v", err)
	}
	if p.Resources().Production().Plants != 2 {
		t.Fatal("reconnected new supply inactive")
	}
	g.ResolveModeBoundary(ctx, "production-end", false)
	if p.Resources().Production().Plants != 0 {
		t.Fatal("new supply outlived its connection")
	}
	if got := p.Resources().Get().Plants; got != 10 {
		t.Fatalf("reserve repeated during reconnection: %d", got)
	}
}

func TestEarthPublishedLateReserveAndTimelineFallback(t *testing.T) {
	g, p, play, registry := earthPlayableFixture(t)
	ctx := context.Background()
	if err := g.SetGeneration(ctx, 6); err != nil {
		t.Fatal(err)
	}
	g.ResolveModeBoundary(ctx, "production-end", false)
	if err := g.SetGeneration(ctx, 7); err != nil {
		t.Fatal(err)
	}
	if err := playEarthCard(t, g, p, play, registry, "ES019", nil, nil); err != nil {
		t.Fatal(err)
	}
	if p.Resources().Get().Plants != 8 {
		t.Fatal("late reserve did not pay immediate residual")
	}
	fallback := 1
	if err := playEarthCard(t, g, p, play, registry, "ES010", &fallback, nil); err != nil {
		t.Fatal(err)
	}
	first := 0
	before := p.Resources().Get()
	if err := playEarthCard(t, g, p, play, registry, "ES015", &first, nil); err == nil {
		t.Fatal("deadline changed after withdrawal")
	}
	if p.Resources().Get() != before {
		t.Fatal("invalid deadline choice spent resources")
	}
}

func TestEarthPublishedImportedManualServiceRetainsItsActionCost(t *testing.T) {
	for _, imported := range []bool{false, true} {
		name := "dependent"
		if imported {
			name = "imported"
		}
		t.Run(name, func(t *testing.T) {
			g, p, play, registry := earthPlayableFixture(t)
			ctx := context.Background()
			if err := playEarthCard(t, g, p, play, registry, "ES004", nil, nil); err != nil {
				t.Fatal(err)
			}
			if imported {
				if err := playEarthCard(t, g, p, play, registry, "ES021", nil, []string{"ES004"}); err != nil {
					t.Fatal(err)
				}
			}
			if err := g.SetGeneration(ctx, 6); err != nil {
				t.Fatal(err)
			}
			g.ResolveModeBoundary(ctx, "production-end", false)
			use := cardAction.NewUseCardActionAction(play.GameRepository(), registry, nil, testutil.TestLogger())
			payment := shared.NativePayment(shared.ResourceCredit, 3)
			before := p.Resources().Get().Credits
			err := use.Execute(ctx, g.ID(), p.ID(), "ES004", 0, nil, nil, nil, nil, nil, &payment, nil, nil)
			if !imported {
				if err == nil {
					t.Fatal("disconnected service action remained enabled")
				}
				if p.Resources().Get().Credits != before {
					t.Fatal("inactive action charged payment")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if p.Resources().Get().Credits != before-3 {
				t.Fatal("import erased or duplicated action cost")
			}
			selection := p.Selection().GetPendingCardDrawSelection()
			if selection == nil || len(selection.AvailableCards) != 3 || selection.FreeTakeCount != 1 {
				t.Fatalf("imported action changed its outputs: %+v", selection)
			}
		})
	}
}
