package harness

import (
	"slices"
	"testing"

	"openmars/internal/delivery/dto"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

// Game is a game in progress with one connected client per player. Players[0] hosts.
type Game struct {
	ID      string
	Players []*Client
}

// Host returns the host's client.
func (g *Game) Host() *Client {
	return g.Players[0]
}

// Lobby creates a game and joins one client per name. The first player hosts.
func (s *Server) Lobby(t testing.TB, edit func(*dto.GameSetupDto), names ...string) *Game {
	t.Helper()
	game := &Game{ID: s.CreateGame(t, edit)}
	for _, name := range names {
		c := s.Dial(t, name)
		c.Join(t, game.ID, name)
		game.Players = append(game.Players, c)
	}
	for _, c := range game.Players {
		c.AwaitState(t, "every player joined", func(g dto.GameDto) bool { return len(g.OtherPlayers) == len(names)-1 })
	}
	return game
}

// Play creates a game, starts it and plays through corporation and prelude selection
// and the opening showcase, choosing the first option offered each time, until the
// first generation's action phase.
func (s *Server) Play(t testing.TB, edit func(*dto.GameSetupDto), names ...string) *Game {
	t.Helper()
	game := s.Lobby(t, edit, names...)
	game.Host().Send(t, dto.MessageTypeActionStartGame, map[string]any{})

	for _, c := range game.Players {
		state := c.AwaitState(t, "starting choices offered", func(g dto.GameDto) bool {
			return g.CurrentPlayer.SelectCorporationPhase != nil
		})
		choice := map[string]any{
			"corporationId": plainCorporation(state.CurrentPlayer.SelectCorporationPhase.AvailableCorporations),
			"cardIds":       []string{},
			"preludeIds":    []string{},
		}
		if preludes := state.CurrentPlayer.SelectPreludeCardsPhase; preludes != nil {
			ids := make([]string, 0, preludes.MaxSelectable)
			for _, p := range preludes.AvailablePreludes[:preludes.MaxSelectable] {
				ids = append(ids, p.ID)
			}
			choice["preludeIds"] = ids
		}
		c.Send(t, dto.MessageTypeActionSelectStartingChoices, choice)
		c.AwaitState(t, "starting choices accepted", func(g dto.GameDto) bool {
			return g.CurrentPlayer.SelectCorporationPhase == nil
		})
	}

	game.AdvanceToAction(t)
	game.Ready(t)
	return game
}

// plainCorporation picks an offered corporation without a forced first action, so the
// player can take any action on their first turn.
func plainCorporation(offered []dto.CardDto) string {
	for _, corp := range offered {
		card, err := testutil.GetCardDB().GetByID(corp.ID)
		if err != nil {
			continue
		}
		if !slices.ContainsFunc(card.Behaviors, gamecards.HasCorporationFirstActionTrigger) {
			return corp.ID
		}
	}
	return offered[0].ID
}

// AdvanceToAction resolves the opening showcase: players place any tiles they are asked
// to, and the host confirms each step, until the action phase begins.
func (g *Game) AdvanceToAction(t testing.TB) {
	t.Helper()
	host := g.Host()
	for range 100 {
		host.Sync(t)
		state := host.State(t)
		if state.CurrentPhase == dto.GamePhaseAction {
			for _, c := range g.Players {
				c.AwaitState(t, "action phase", func(g dto.GameDto) bool { return g.CurrentPhase == dto.GamePhaseAction })
			}
			return
		}
		if state.CurrentPhase != dto.GamePhaseInitApplyCorp && state.CurrentPhase != dto.GamePhaseInitApplyPrelude {
			host.AwaitState(t, "the showcase begins", func(g dto.GameDto) bool {
				return g.CurrentPhase == dto.GamePhaseInitApplyCorp || g.CurrentPhase == dto.GamePhaseAction
			})
			continue
		}
		if g.placePendingTiles(t) {
			continue
		}
		if state.InitPhase != nil && state.InitPhase.WaitingForConfirm {
			version := state.InitPhase.ConfirmVersion
			phase := state.CurrentPhase
			host.Send(t, dto.MessageTypeActionConfirmInitAdvance, map[string]any{})
			host.AwaitState(t, "the showcase advanced", func(g dto.GameDto) bool {
				return g.CurrentPhase != phase || g.InitPhase == nil || g.InitPhase.ConfirmVersion != version
			})
			continue
		}
		host.AwaitState(t, "the showcase waits for confirmation", func(g dto.GameDto) bool {
			return g.CurrentPhase == dto.GamePhaseAction || (g.InitPhase != nil && g.InitPhase.WaitingForConfirm)
		})
	}
	t.Fatalf("the opening showcase did not reach the action phase")
}

// placePendingTiles resolves whatever selection any player is waiting on.
func (g *Game) placePendingTiles(t testing.TB) bool {
	t.Helper()
	resolved := false
	for _, c := range g.Players {
		resolved = c.ResolvePending(t) || resolved
	}
	return resolved
}

// ResolvePending answers one selection the player is waiting on, choosing the first
// option, and reports whether there was one. Supported: tile placement, card draws,
// award funding and card receipts.
func (c *Client) ResolvePending(t testing.TB) bool {
	t.Helper()
	c.Sync(t)
	p := c.State(t).CurrentPlayer
	switch {
	case p.PendingTileSelection != nil && len(p.PendingTileSelection.AvailableHexes) > 0:
		pending := p.PendingTileSelection
		c.expectAccepted(t, dto.MessageTypeActionTileSelected, map[string]any{"hex": pending.AvailableHexes[0]})
	case p.PendingCardDrawSelection != nil:
		pending := p.PendingCardDrawSelection
		take := []string{}
		for _, card := range pending.AvailableCards[:min(pending.FreeTakeCount, len(pending.AvailableCards))] {
			take = append(take, card.ID)
		}
		c.expectAccepted(t, dto.MessageTypeActionCardDrawConfirmed, map[string]any{"cardsToTake": take, "cardsToBuy": []string{}})
	case p.PendingAwardFundSelection != nil && len(p.PendingAwardFundSelection.AvailableAwards) > 0:
		c.expectAccepted(t, dto.MessageTypeActionConfirmAwardFund, map[string]any{"awardType": p.PendingAwardFundSelection.AvailableAwards[0]})
	case p.PendingColonySelection != nil && len(p.PendingColonySelection.AvailableColonyIDs) > 0:
		c.expectAccepted(t, dto.MessageTypeActionConfirmColonyPlacement, map[string]any{"colonyId": p.PendingColonySelection.AvailableColonyIDs[0]})
	case len(p.CardReceipts) > 0:
		c.expectAccepted(t, dto.MessageTypeActionAcknowledgeCardReceipt, map[string]any{"receiptId": p.CardReceipts[0].ID})
	default:
		return false
	}
	return true
}

// expectAccepted sends a request and fails the test if the server rejects it.
func (c *Client) expectAccepted(t testing.TB, messageType dto.MessageType, payload any) {
	t.Helper()
	c.Send(t, messageType, payload)
	for _, msg := range c.Sync(t) {
		if msg.Type == dto.MessageTypeError {
			t.Fatalf("%s: %s rejected: %s", c.Name, messageType, msg.Payload)
		}
	}
}

// Ready resolves the corporation first action and any selection the player whose turn
// it is must finish, so they are free to take any action.
func (g *Game) Ready(t testing.TB) *Client {
	t.Helper()
	for range 20 {
		g.SyncAll(t)
		current := g.Current(t)
		state := current.State(t)
		if state.CurrentPlayer.ForcedFirstAction == nil && !current.ResolvePending(t) {
			return current
		}
		if state.CurrentPlayer.ForcedFirstAction != nil && !current.ResolvePending(t) {
			t.Fatalf("%s: corporation first action %q needs a selection the harness cannot make", current.Name, state.CurrentPlayer.ForcedFirstAction.Description)
		}
	}
	t.Fatalf("the current player never became free to act")
	return nil
}

// Current returns the client whose turn it is.
func (g *Game) Current(t testing.TB) *Client {
	t.Helper()
	state := g.Host().State(t)
	if state.CurrentTurn == nil {
		t.Fatalf("nobody has the turn in phase %s", state.CurrentPhase)
	}
	return g.Client(t, *state.CurrentTurn)
}

// Client returns the client of a player.
func (g *Game) Client(t testing.TB, playerID string) *Client {
	t.Helper()
	for _, c := range g.Players {
		if c.PlayerID() == playerID {
			return c
		}
	}
	t.Fatalf("no client plays %s", playerID)
	return nil
}

// Others returns every client except c.
func (g *Game) Others(c *Client) []*Client {
	var others []*Client
	for _, p := range g.Players {
		if p != c {
			others = append(others, p)
		}
	}
	return others
}

// SyncAll waits until every client has received everything sent so far.
func (g *Game) SyncAll(t testing.TB) {
	t.Helper()
	for _, c := range g.Players {
		c.Sync(t)
	}
}

// Pay builds a payment of exactly the given costs from the player's own resources.
func Pay(costs map[string]int) shared.Payment {
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	for resource, amount := range costs {
		if amount > 0 {
			payment.Allocations = append(payment.Allocations, shared.NativePayment(shared.ResourceType(resource), amount).Allocations...)
		}
	}
	return payment
}

// StandardProject runs a standard project, paying its effective cost from the client's
// current state, and fails the test if the server rejects it. Everything the client
// received meanwhile is consumed.
func (c *Client) StandardProject(t testing.TB, projectID string) {
	t.Helper()
	for _, p := range c.State(t).CurrentPlayer.StandardProjects {
		if p.ProjectType == projectID {
			c.Send(t, dto.MessageTypeActionStandardProject, map[string]any{"projectId": projectID, "payment": Pay(p.EffectiveCost)})
			for _, msg := range c.Sync(t) {
				if msg.Type == dto.MessageTypeError {
					t.Fatalf("%s: standard project %s rejected: %s", c.Name, projectID, msg.Payload)
				}
			}
			return
		}
	}
	t.Fatalf("%s: no standard project %s", c.Name, projectID)
}

// PassAll has every player pass, in turn order, until the action phase ends.
func (g *Game) PassAll(t testing.TB) {
	t.Helper()
	for range 50 {
		g.SyncAll(t)
		phase := g.Host().State(t).CurrentPhase
		if phase != dto.GamePhaseAction && phase != dto.GamePhaseFinalPhase {
			return
		}
		current := g.Ready(t)
		current.expectAccepted(t, dto.MessageTypeActionSkipAction, map[string]any{})
	}
	t.Fatalf("the players never finished passing")
}

// ConfirmProduction has every player finish the production phase without buying cards.
func (g *Game) ConfirmProduction(t testing.TB) {
	t.Helper()
	g.SyncAll(t)
	for _, c := range g.Players {
		production := c.State(t).CurrentPlayer.ProductionPhase
		if production == nil || production.SelectionComplete {
			continue
		}
		c.expectAccepted(t, dto.MessageTypeActionConfirmProductionCards, map[string]any{"cardIds": []string{}})
	}
	g.SyncAll(t)
}

// ExpectConverged fails unless every client sees the same public game state.
func (g *Game) ExpectConverged(t testing.TB) {
	t.Helper()
	g.SyncAll(t)
	want := g.Host().State(t)
	for _, c := range g.Players[1:] {
		got := c.State(t)
		if got.Generation != want.Generation || got.CurrentPhase != want.CurrentPhase || got.Status != want.Status {
			t.Fatalf("%s sees generation %d %s %s, %s sees generation %d %s %s", c.Name, got.Generation, got.CurrentPhase, got.Status, g.Host().Name, want.Generation, want.CurrentPhase, want.Status)
		}
		if got.GlobalParameters.Temperature != want.GlobalParameters.Temperature || got.GlobalParameters.Oxygen != want.GlobalParameters.Oxygen || got.GlobalParameters.Oceans != want.GlobalParameters.Oceans {
			t.Fatalf("%s and %s disagree on the global parameters", c.Name, g.Host().Name)
		}
		if (got.CurrentTurn == nil) != (want.CurrentTurn == nil) || (got.CurrentTurn != nil && *got.CurrentTurn != *want.CurrentTurn) {
			t.Fatalf("%s and %s disagree on whose turn it is", c.Name, g.Host().Name)
		}
	}
	for _, c := range g.Players {
		self := c.State(t).CurrentPlayer
		for _, other := range g.Others(c) {
			seen := Other(t, other.State(t), self.ID)
			if seen.TerraformRating != self.TerraformRating || seen.Resources != self.Resources || seen.Production != self.Production {
				t.Fatalf("%s sees %s with TR %d %+v, but %s has TR %d %+v", other.Name, c.Name, seen.TerraformRating, seen.Resources, c.Name, self.TerraformRating, self.Resources)
			}
		}
	}
}
