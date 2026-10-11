package e2e_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"openmars/internal/delivery/dto"
	"openmars/test/e2e/harness"
	"openmars/test/testutil"
)

// expectNoMention fails when any message a client received mentions one of the IDs.
func expectNoMention(t *testing.T, who string, received []harness.Message, private map[string]string) {
	t.Helper()
	for _, msg := range received {
		for id, owner := range private {
			if strings.Contains(string(msg.Payload), `"`+id+`"`) {
				t.Fatalf("%s received %s, which reveals %s's private card %s at %s", who, msg.Type, owner, id, jsonPathOf(msg.Payload, id))
			}
		}
	}
}

// jsonPathOf returns the JSON paths at which a string value occurs.
func jsonPathOf(raw []byte, value string) []string {
	var doc any
	if json.Unmarshal(raw, &doc) != nil {
		return nil
	}
	var paths []string
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				walk(path+"."+k, child)
			}
		case []any:
			for i, child := range x {
				walk(fmt.Sprintf("%s[%d]", path, i), child)
			}
		case string:
			if x == value {
				paths = append(paths, path)
			}
		}
	}
	walk("", doc)
	return paths
}

func TestPrivacy_StartingOptionsAreSeenOnlyByTheirPlayer(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	game := srv.Lobby(t, nil, "alice", "bob")
	watcher := srv.Dial(t, "watcher")
	watcher.Spectate(t, game.ID, "watcher")
	game.Host().Send(t, dto.MessageTypeActionStartGame, map[string]any{})

	private := map[string]string{}
	for _, c := range game.Players {
		state := c.AwaitState(t, "starting options", func(g dto.GameDto) bool { return g.CurrentPlayer.SelectCorporationPhase != nil })
		for _, corp := range state.CurrentPlayer.SelectCorporationPhase.AvailableCorporations {
			private[corp.ID] = c.Name
		}
		for _, card := range state.CurrentPlayer.SelectStartingCardsPhase.AvailableCards {
			private[card.ID] = c.Name
		}
	}
	for _, c := range game.Players {
		mine := map[string]string{}
		for id, owner := range private {
			if owner != c.Name {
				mine[id] = owner
			}
		}
		expectNoMention(t, c.Name, c.Sync(t), mine)
	}
	expectNoMention(t, "the spectator", watcher.Sync(t), private)
}

func TestPrivacy_HandsAreSeenOnlyByTheirPlayer(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	game := srv.Play(t, harness.DevMode, "alice", "bob")
	alice, bob := game.Players[0], game.Players[1]
	watcher := srv.Dial(t, "watcher")
	watcher.Spectate(t, game.ID, "watcher")
	game.SyncAll(t)
	watcher.Sync(t)

	secret := cardNobodyHolds(t, game, "Giant Ice Asteroid", "Comet", "Big Asteroid", "Deimos Down")
	alice.Admin(t, dto.AdminCommandTypeGiveCard, map[string]any{"playerId": alice.PlayerID(), "cardId": secret})
	alice.AwaitState(t, "the card is in alice's hand", func(g dto.GameDto) bool {
		for _, c := range g.CurrentPlayer.Cards {
			if c.ID == secret {
				return true
			}
		}
		return false
	})

	expectNoMention(t, "bob", bob.Sync(t), map[string]string{secret: "alice"})
	expectNoMention(t, "the spectator", watcher.Sync(t), map[string]string{secret: "alice"})
	if got := harness.Other(t, bob.State(t), alice.PlayerID()).HandCardCount; got == 0 {
		t.Fatalf("bob should still see how many cards alice holds")
	}
}

// cardNobodyHolds returns the first named card that is in no player's hand. The admin
// give-card command copies a card without taking it from the deck, so a player may
// already hold it.
func cardNobodyHolds(t *testing.T, game *harness.Game, names ...string) string {
	t.Helper()
	game.SyncAll(t)
	for _, name := range names {
		id := testutil.CardID(name)
		held := false
		for _, c := range game.Players {
			for _, card := range c.State(t).CurrentPlayer.Cards {
				held = held || card.ID == id
			}
		}
		if !held {
			return id
		}
	}
	t.Fatalf("every candidate card is already held")
	return ""
}

func TestPrivacy_AdminCommandsNeedDevelopmentModeAndAPlayer(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	setTo := func(c *harness.Client, credits int) {
		c.Send(t, dto.MessageTypeAdminCommand, map[string]any{
			"commandType": dto.AdminCommandTypeSetResources,
			"payload":     map[string]any{"playerId": c.PlayerID(), "resources": map[string]any{"credits": credits}},
		})
	}

	normal := srv.Play(t, func(s *dto.GameSetupDto) { s.DevelopmentMode = false }, "alice", "bob")
	setTo(normal.Players[1], 999)
	normal.Players[1].AwaitError(t)
	normal.Host().ExpectQuiet(t)

	dev := srv.Play(t, harness.DevMode, "alice", "bob")
	watcher := srv.Dial(t, "watcher")
	watcher.Spectate(t, dev.ID, "watcher")
	dev.Host().Drain(t)
	watcher.Send(t, dto.MessageTypeAdminCommand, map[string]any{
		"commandType": dto.AdminCommandTypeSetResources,
		"payload":     map[string]any{"playerId": dev.Host().PlayerID(), "resources": map[string]any{"credits": 999}},
	})
	watcher.AwaitError(t)
	dev.Host().ExpectQuiet(t)

	setTo(dev.Players[1], 777)
	dev.Players[1].AwaitState(t, "credits set", func(g dto.GameDto) bool { return g.CurrentPlayer.Resources.Credits == 777 })
}
