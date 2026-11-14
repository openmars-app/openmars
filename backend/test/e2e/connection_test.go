package e2e_test

import (
	"net/http"
	"testing"

	"openmars/internal/delivery/dto"
	"openmars/internal/game/shared"
	"openmars/test/e2e/harness"
)

func TestConnection_SpectatorRetainsMapIdentity(t *testing.T) {
	t.Parallel()
	for _, mapID := range []string{"tharsis", "hellas"} {
		t.Run(mapID, func(t *testing.T) {
			t.Parallel()
			srv := harness.Start(t)
			gameID := srv.CreateGame(t, func(settings *dto.GameSetupDto) {
				settings.MapID = mapID
			})
			host := srv.Dial(t, "host")
			host.Join(t, gameID, "host")

			var public dto.GetGameResponse
			if status := srv.Request(t, http.MethodGet, "/api/v1/games/"+gameID, nil, &public); status != http.StatusOK {
				t.Fatalf("public game: status %d", status)
			}
			if public.Game.Settings.MapID != mapID {
				t.Fatalf("public game map: want %q, got %q", mapID, public.Game.Settings.MapID)
			}

			watcher := srv.Dial(t, "watcher")
			watcher.Spectate(t, gameID, "watcher")
			if got := watcher.State(t).Settings.MapID; got != mapID {
				t.Fatalf("initial spectator map: want %q, got %q", mapID, got)
			}
			host.Send(t, dto.MessageTypeActionStartGame, map[string]any{})
			active := watcher.AwaitState(t, "game started", func(g dto.GameDto) bool {
				return g.Status == dto.GameStatusActive
			})
			if active.ID != gameID || active.Settings.MapID != mapID {
				t.Fatalf("active spectator identity: want %s/%s, got %s/%s", gameID, mapID, active.ID, active.Settings.MapID)
			}
		})
	}
}

// lobby creates a game and joins the given players, first one hosting.
func lobby(t *testing.T, srv *harness.Server, names ...string) (string, []*harness.Client) {
	t.Helper()
	gameID := srv.CreateGame(t, nil)
	clients := make([]*harness.Client, len(names))
	for i, name := range names {
		clients[i] = srv.Dial(t, name)
		clients[i].Join(t, gameID, name)
	}
	for _, c := range clients {
		c.AwaitState(t, "everyone joined", func(g dto.GameDto) bool { return len(g.OtherPlayers) == len(names)-1 })
	}
	return gameID, clients
}

func TestConnection_DeviceSwitchInLobbyKeepsThePlayer(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	gameID, c := lobby(t, srv, "alice", "bob")
	alice, bob := c[0], c[1]
	aliceID := alice.PlayerID()

	phone := srv.Dial(t, "alice-phone")
	phone.Rejoin(t, gameID, aliceID, "alice")
	srv.CloseAndWait(t, alice)

	bob.Sync(t)
	state := bob.State(t)
	if !harness.Connected(state, aliceID) {
		t.Fatalf("alice is still connected on her phone, but bob sees her as gone or disconnected")
	}
	if state.HostPlayerID != aliceID {
		t.Fatalf("host moved away from alice although she never left")
	}

	phone.Send(t, dto.MessageTypeActionStartGame, map[string]any{})
	bob.AwaitState(t, "game started from the phone", func(g dto.GameDto) bool { return g.Status == dto.GameStatusActive })
}

func TestConnection_DeviceSwitchMidGameKeepsTheSeat(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	gameID, c := lobby(t, srv, "alice", "bob")
	alice, bob := c[0], c[1]
	aliceID := alice.PlayerID()
	alice.Send(t, dto.MessageTypeActionStartGame, map[string]any{})
	bob.AwaitState(t, "game started", func(g dto.GameDto) bool { return g.Status == dto.GameStatusActive })

	phone := srv.Dial(t, "alice-phone")
	phone.Rejoin(t, gameID, aliceID, "alice")
	srv.CloseAndWait(t, alice)

	bob.Sync(t)
	if !harness.Connected(bob.State(t), aliceID) {
		t.Fatalf("alice switched devices and is connected, but shows as disconnected")
	}

	thief := srv.Dial(t, "thief")
	thief.Send(t, dto.MessageTypePlayerTakeover, map[string]any{"gameId": gameID, "targetPlayerId": aliceID})
	thief.AwaitError(t)
}

func TestConnection_JoiningAnotherGameReleasesTheFirst(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	gameA, a := lobby(t, srv, "host-a", "xavier")
	hostA, xavier := a[0], a[1]
	xavierID := xavier.PlayerID()
	gameB, b := lobby(t, srv, "host-b")
	hostB := b[0]

	xavier.Join(t, gameB, "xavier")
	hostA.Sync(t)
	if harness.Connected(hostA.State(t), xavierID) {
		t.Fatalf("xavier moved to another game but still shows as connected in game A")
	}

	hostA.Send(t, dto.MessageTypeEndGame, map[string]any{})
	hostA.Await(t, dto.MessageTypeGameEnded, nil)
	hostA.AwaitClosed(t)

	xavier.Send(t, dto.MessageTypeChatMessage, map[string]any{"message": "still here"})
	hostB.Await(t, dto.MessageTypeChatUpdate, nil)
	for _, msg := range xavier.Sync(t) {
		if msg.Type == dto.MessageTypeGameEnded {
			t.Fatalf("xavier left game A, but was told it ended")
		}
	}
	_ = gameA
}

func TestConnection_FailedJoinDoesNotBindTheConnection(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	gameID, c := lobby(t, srv, "alice", "bob")
	c[0].Send(t, dto.MessageTypeActionStartGame, map[string]any{})
	c[0].AwaitState(t, "game started", func(g dto.GameDto) bool { return g.Status == dto.GameStatusActive })

	late := srv.Dial(t, "late")
	late.Send(t, dto.MessageTypePlayerConnect, map[string]any{"gameId": gameID, "playerName": "late"})
	late.AwaitError(t)

	late.Send(t, dto.MessageTypeChatMessage, map[string]any{"message": "I am not in this game"})
	late.AwaitError(t)
	c[0].ExpectQuiet(t)
}

func TestConnection_FailedSpectateDoesNotBindTheConnection(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	gameID, c := lobby(t, srv, "alice")

	watcher := srv.Dial(t, "watcher")
	watcher.Send(t, dto.MessageTypeSpectatorConnect, map[string]any{"gameId": "no-such-game", "spectatorName": "w"})
	watcher.AwaitError(t)
	watcher.Send(t, dto.MessageTypeChatMessage, map[string]any{"message": "hello"})
	watcher.AwaitError(t)
	c[0].ExpectQuiet(t)
	_ = gameID
}

func TestConnection_RejoiningByNameBindsTheExistingPlayer(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	gameID, c := lobby(t, srv, "alice", "bob")
	aliceID := c[0].PlayerID()

	again := srv.Dial(t, "alice-again")
	if got := again.Join(t, gameID, "alice"); got != aliceID {
		t.Fatalf("joining the lobby with an existing name should reconnect as that player: got %s, want %s", got, aliceID)
	}
	color := shared.PlayerColors[len(shared.PlayerColors)-1]
	again.Send(t, dto.MessageTypeSetPlayerColor, map[string]any{"color": color})
	c[1].AwaitState(t, "alice's colour changed", func(g dto.GameDto) bool {
		return harness.HasOther(g, aliceID) && harness.Other(t, g, aliceID).Color == color
	})
}

func TestConnection_KickClosesEveryDeviceOfThePlayer(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	gameID, c := lobby(t, srv, "alice", "bob")
	alice, bob := c[0], c[1]
	bobID := bob.PlayerID()
	bobPhone := srv.Dial(t, "bob-phone")
	bobPhone.Rejoin(t, gameID, bobID, "bob")

	alice.Send(t, dto.MessageTypeKickPlayer, map[string]any{"targetPlayerId": bobID})
	for _, device := range []*harness.Client{bob, bobPhone} {
		device.Await(t, dto.MessageTypePlayerKicked, nil)
		device.AwaitClosed(t)
	}
	alice.AwaitState(t, "bob removed", func(g dto.GameDto) bool { return !harness.HasOther(g, bobID) })
}

func TestConnection_EndGameClosesOnlyThatGame(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	gameA, a := lobby(t, srv, "alice", "bob")
	watcher := srv.Dial(t, "watcher")
	watcher.Spectate(t, gameA, "watcher")
	_, b := lobby(t, srv, "carol")

	a[0].Send(t, dto.MessageTypeEndGame, map[string]any{})
	for _, client := range []*harness.Client{a[0], a[1], watcher} {
		client.Await(t, dto.MessageTypeGameEnded, nil)
		client.AwaitClosed(t)
	}
	b[0].ExpectQuiet(t)
}

func TestConnection_RespectatingDoesNotLeakSpectatorSlots(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	gameID, c := lobby(t, srv, "alice")

	watcher := srv.Dial(t, "watcher")
	for range 6 {
		watcher.Spectate(t, gameID, "watcher")
	}
	state := c[0].AwaitState(t, "watcher spectating", func(g dto.GameDto) bool { return len(g.Spectators) > 0 })
	c[0].Sync(t)
	state = c[0].State(t)
	if len(state.Spectators) != 1 {
		t.Fatalf("one socket spectating repeatedly should be one spectator, got %d", len(state.Spectators))
	}
	other := srv.Dial(t, "other")
	other.Spectate(t, gameID, "other")
}

func TestConnection_ClientsCannotSendServerDisconnectNotices(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	gameID, c := lobby(t, srv, "alice", "bob")
	alice, bob := c[0], c[1]
	watcher := srv.Dial(t, "watcher")
	watcherID := watcher.Spectate(t, gameID, "watcher")
	bob.Drain(t)

	alice.Send(t, dto.MessageType("player-disconnected"), map[string]any{})
	alice.AwaitError(t)

	stranger := srv.Dial(t, "stranger")
	stranger.Send(t, dto.MessageType("spectator-disconnected"), map[string]any{"spectatorId": watcherID, "gameId": gameID})
	stranger.AwaitError(t)

	bob.Sync(t)
	state := bob.State(t)
	if !harness.Connected(state, alice.PlayerID()) {
		t.Fatalf("alice is still connected")
	}
	if len(state.Spectators) != 1 {
		t.Fatalf("a stranger removed the spectator")
	}
}
