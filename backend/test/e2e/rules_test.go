package e2e_test

import (
	"slices"
	"testing"

	"openmars/internal/delivery/dto"
	"openmars/test/e2e/harness"
)

func handIDs(g dto.GameDto) []string {
	ids := make([]string, len(g.CurrentPlayer.Cards))
	for i, c := range g.CurrentPlayer.Cards {
		ids[i] = c.ID
	}
	return ids
}

func TestRules_SellingTheSamePatentTwiceIsRejected(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	game := srv.Play(t, harness.DevMode, "alice", "bob")
	player := game.Current(t)
	card := cardNobodyHolds(t, game, "Comet", "Big Asteroid", "Giant Ice Asteroid")
	player.Admin(t, dto.AdminCommandTypeGiveCard, map[string]any{"playerId": player.PlayerID(), "cardId": card})

	player.StandardProject(t, "sell-patents")
	before := player.AwaitState(t, "a patent selection is open", func(g dto.GameDto) bool { return g.CurrentPlayer.PendingCardSelection != nil })
	game.SyncAll(t)

	player.Send(t, dto.MessageTypeActionConfirmSellPatents, map[string]any{"selectedCardIds": []string{card, card, card}})
	player.AwaitError(t)
	game.Others(player)[0].ExpectQuiet(t)
	after := player.State(t)
	if after.CurrentPlayer.Resources.Credits != before.CurrentPlayer.Resources.Credits {
		t.Fatalf("selling one card three times paid %d credits", after.CurrentPlayer.Resources.Credits-before.CurrentPlayer.Resources.Credits)
	}

	player.Send(t, dto.MessageTypeActionConfirmSellPatents, map[string]any{"selectedCardIds": []string{card}})
	sold := player.AwaitState(t, "the card is sold", func(g dto.GameDto) bool { return !slices.Contains(handIDs(g), card) })
	if got := sold.CurrentPlayer.Resources.Credits - before.CurrentPlayer.Resources.Credits; got != 1 {
		t.Fatalf("selling one card should pay 1 credit, paid %d", got)
	}
}

func TestRules_StartingChoicesRejectDuplicates(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	game := srv.Lobby(t, nil, "alice")
	alice := game.Host()
	alice.Send(t, dto.MessageTypeActionStartGame, map[string]any{})
	state := alice.AwaitState(t, "starting choices", func(g dto.GameDto) bool { return g.CurrentPlayer.SelectCorporationPhase != nil })
	corp := state.CurrentPlayer.SelectCorporationPhase.AvailableCorporations[0].ID
	card := state.CurrentPlayer.SelectStartingCardsPhase.AvailableCards[0].ID
	preludes := state.CurrentPlayer.SelectPreludeCardsPhase.AvailablePreludes

	for name, choice := range map[string]map[string]any{
		"the same card twice":    {"corporationId": corp, "cardIds": []string{card, card}, "preludeIds": []string{preludes[0].ID, preludes[1].ID}},
		"the same prelude twice": {"corporationId": corp, "cardIds": []string{}, "preludeIds": []string{preludes[0].ID, preludes[0].ID}},
	} {
		alice.Send(t, dto.MessageTypeActionSelectStartingChoices, choice)
		if got := alice.AwaitError(t); got.Message == "" {
			t.Fatalf("choosing %s should be rejected", name)
		}
		if alice.State(t).CurrentPlayer.SelectCorporationPhase == nil {
			t.Fatalf("choosing %s was accepted", name)
		}
	}
}

func TestRules_ProjectFundingNeverChargesMoreThanTheSeatCosts(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	game := srv.Play(t, func(s *dto.GameSetupDto) {
		s.DevelopmentMode = true
		s.Features = append(s.Features, "project-funding")
	}, "alice", "bob")
	player := game.Current(t)
	player.Admin(t, dto.AdminCommandTypeSetResources, map[string]any{"playerId": player.PlayerID(), "resources": map[string]any{"credits": 200}})
	state := player.AwaitState(t, "credits set", func(g dto.GameDto) bool { return g.CurrentPlayer.Resources.Credits == 200 })
	var project *dto.ProjectFundingDto
	for i := range state.ProjectFunding {
		if state.ProjectFunding[i].CanBuySeat {
			project = &state.ProjectFunding[i]
			break
		}
	}
	if project == nil {
		t.Fatalf("no project funding seat can be bought")
	}

	player.Send(t, dto.MessageTypeActionProjectFundingSeat, map[string]any{"projectId": project.ID, "credits": project.NextSeatCost + 50})
	after := player.AwaitState(t, "the seat is bought", func(g dto.GameDto) bool {
		for _, p := range g.ProjectFunding {
			if p.ID == project.ID {
				return p.CurrentPlayerSeats == project.CurrentPlayerSeats+1
			}
		}
		return false
	})
	if paid := 200 - after.CurrentPlayer.Resources.Credits; paid != project.NextSeatCost {
		t.Fatalf("a seat costing %d took %d credits", project.NextSeatCost, paid)
	}
}

func TestRules_ClassicVenusAndColoniesRunTogether(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	game := srv.Play(t, func(s *dto.GameSetupDto) {
		s.GameModeID = "classic"
		s.Features = []string{"venus", "colonies"}
		s.DevelopmentMode = true
	}, "alice", "bob")
	player := game.Ready(t)
	state := player.State(t)
	if !slices.Contains(state.Settings.Systems, "venus") || !slices.Contains(state.Settings.Systems, "colonies") {
		t.Fatal("selected expansions did not activate their systems")
	}
	if len(state.Colonies) == 0 || state.TradeFleets[player.PlayerID()].Total == 0 {
		t.Fatal("colonies or trading fleets missing")
	}
	player.Admin(t, dto.AdminCommandTypeSetResources, map[string]any{"playerId": player.PlayerID(), "resources": map[string]any{"credits": 100}})
	player.AwaitState(t, "credits set", func(g dto.GameDto) bool { return g.CurrentPlayer.Resources.Credits == 100 })
	before := player.State(t).GlobalParameters.Venus
	player.StandardProject(t, "air-scrapping")
	player.AwaitState(t, "Venus terraforming", func(g dto.GameDto) bool { return g.GlobalParameters.Venus > before })
	game.ExpectConverged(t)
}
