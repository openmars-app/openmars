package e2e_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"openmars/internal/delivery/dto"
	"openmars/test/e2e/harness"
)

// garbage is sent as the payload of every message type. None of it is a valid request.
var garbage = []any{nil, []any{}, "x", 42, map[string]any{"unexpected": true}}

// contracts lists every message type a client may send, with payloads specific to that
// type that must be rejected. A type the server handles but this table does not list
// fails TestContract_EveryMessageTypeIsCovered.
var contracts = map[dto.MessageType][]any{
	dto.MessageTypeWatchResumeGame:                {map[string]any{"gameId": 5}, map[string]any{"gameId": "no-such-game"}},
	dto.MessageTypeClaimResumeSeat:                {map[string]any{"seatId": 5}, map[string]any{"gameId": "no-such-game", "seatId": "x"}},
	dto.MessageTypeReleaseResumeSeat:              {map[string]any{"seatId": 5}, map[string]any{"seatId": "x"}},
	dto.MessageTypeResumeGame:                     {map[string]any{"gameId": 5}},
	dto.MessageTypeResumeBotToken:                 {map[string]any{"botToken": 5}},
	dto.MessageTypeQuotePayment:                   {map[string]any{"requestId": 5}, map[string]any{"requestId": "r", "intent": "x"}},
	dto.MessageTypePlayerConnect:                  {map[string]any{"gameId": 5, "playerName": "x"}, map[string]any{"gameId": "no-such-game", "playerName": "x"}},
	dto.MessageTypePlayerTakeover:                 {map[string]any{"gameId": "no-such-game", "targetPlayerId": 5}},
	dto.MessageTypeSpectatorConnect:               {map[string]any{"gameId": 5, "spectatorName": "x"}, map[string]any{"gameId": "no-such-game", "spectatorName": "x"}},
	dto.MessageTypeActionStandardProject:          {map[string]any{"projectId": 5}, map[string]any{"projectId": "no-such-project"}},
	dto.MessageTypeActionConfirmSellPatents:       {map[string]any{"selectedCardIds": []any{1, 2}}, map[string]any{"selectedCardIds": "x"}},
	dto.MessageTypeActionConvertPlantsToGreenery:  {map[string]any{"payment": "x"}, map[string]any{"payment": map[string]any{"credits": -5}}},
	dto.MessageTypeActionConvertHeatToTemperature: {map[string]any{"payment": "x"}, map[string]any{"payment": map[string]any{"credits": -5}}},
	dto.MessageTypeAddBot:                         {map[string]any{}},
	dto.MessageTypeActionStartGame:                {map[string]any{}},
	dto.MessageTypeActionSkipAction:               {map[string]any{}},
	dto.MessageTypeActionSelectDemoChoices:        {map[string]any{"corporationId": 3}},
	dto.MessageTypeActionConfirmInitAdvance:       {map[string]any{}},
	dto.MessageTypeActionClaimMilestone:           {map[string]any{"milestoneType": 1}, map[string]any{"milestoneType": "no-such-milestone"}},
	dto.MessageTypeActionFundAward:                {map[string]any{"awardType": 1}, map[string]any{"awardType": "no-such-award"}},
	dto.MessageTypeActionTileSelected:             {map[string]any{"hex": 5}, map[string]any{"hex": "not-a-hex"}, map[string]any{"hex": "999,0,-999"}},
	dto.MessageTypeActionPlayCard: {
		map[string]any{"cardId": 7},
		map[string]any{"cardId": "no-such-card"},
		map[string]any{"cardId": "no-such-card", "choiceIndex": -1},
		map[string]any{"cardId": "no-such-card", "payment": map[string]any{"credits": -100}},
		map[string]any{"cardId": "no-such-card", "cardStorageTargets": []any{1}},
	},
	dto.MessageTypeActionCardAction: {
		map[string]any{"cardId": "no-such-card", "behaviorIndex": -1},
		map[string]any{"cardId": "no-such-card", "behaviorIndex": 1e9},
		map[string]any{"cardId": "no-such-card", "behaviorIndex": 0.5},
	},
	dto.MessageTypeActionSelectStartingChoices:  {map[string]any{"corporationId": "x", "cardIds": []any{1}}, map[string]any{"corporationId": 5}},
	dto.MessageTypeActionConfirmProductionCards: {map[string]any{"selectedCardIds": []any{1}}, map[string]any{"randomBuy": "yes"}},
	dto.MessageTypeActionCardDrawConfirmed:      {map[string]any{"cardsToTake": []any{1}, "cardsToBuy": "x"}},
	dto.MessageTypeActionCardDiscardConfirmed:   {map[string]any{"resolutionId": "x", "cardIds": []any{nil}}},
	dto.MessageTypeActionConfirmOptionalBehavior: {
		map[string]any{"resolutionId": "x", "accept": "yes"},
		map[string]any{"resolutionId": "x", "accept": true},
	},
	dto.MessageTypeActionBehaviorChoiceConfirmed: {
		map[string]any{"resolutionId": "x", "choiceIndex": -1},
		map[string]any{"resolutionId": "x", "choiceIndex": "first"},
	},
	dto.MessageTypeActionConfirmResourceRemoval: {
		map[string]any{"selectionId": "pending", "targetPlayerId": "target", "amount": "x"},
		map[string]any{"selectionId": "pending", "targetPlayerId": "target", "amount": 1.5},
		map[string]any{"selectionId": "pending", "targetPlayerId": "target", "amount": -1},
	},
	dto.MessageTypeActionColonyTrade: {
		map[string]any{"colonyId": "luna", "paymentType": "energy"},
		map[string]any{"colonyId": "luna", "paymentType": "energy", "trackSteps": "two"},
		map[string]any{"colonyId": "luna", "paymentType": "energy", "trackSteps": -1},
		map[string]any{"colonyId": "luna", "paymentType": "energy", "trackSteps": 1.5},
		map[string]any{"colonyId": "luna", "paymentType": "energy", "trackSteps": nil},
	},
	dto.MessageTypeActionColonyBuild:            {map[string]any{"colonyId": 3}, map[string]any{"colonyId": "no-such-colony"}},
	dto.MessageTypeActionProjectFundingSeat:     {map[string]any{"projectId": "x", "credits": -5}},
	dto.MessageTypeActionConfirmColonyResource:  {map[string]any{"cardId": ""}},
	dto.MessageTypeActionConfirmAwardFund:       {map[string]any{"skip": true}, map[string]any{"skip": "yes"}, map[string]any{"skip": true, "colonyId": "luna"}, map[string]any{"awardType": 3}},
	dto.MessageTypeActionAcknowledgeCardReceipt: {map[string]any{"receiptId": 3}},
	dto.MessageTypeActionConfirmColonyPlacement: {map[string]any{"skip": true}, map[string]any{"skip": "yes"}, map[string]any{"skip": true, "colonyId": "luna"}, map[string]any{"colonyId": ""}},
	dto.MessageTypeActionConfirmCardReveal:      {map[string]any{}},
	dto.MessageTypeActionConfirmEffectSelection: {map[string]any{"skip": true}, map[string]any{"skip": "yes"}, map[string]any{"skip": true, "colonyId": "luna"}, map[string]any{"optionIndex": 1.5}, map[string]any{"optionIndex": -1}},
	dto.MessageTypeActionConfirmFreeTrade:       {map[string]any{"skip": true}, map[string]any{"skip": "yes"}, map[string]any{"skip": true, "colonyId": "luna"}, map[string]any{"trackSteps": "x"}},
	dto.MessageTypeAdminCommand: {
		map[string]any{"commandType": "set-resources", "payload": map[string]any{"playerId": "x", "resources": map[string]any{"credits": "x"}}},
		map[string]any{"commandType": "no-such-command", "payload": map[string]any{}},
	},
	dto.MessageTypeRequestLogs:        {},
	dto.MessageTypeKickPlayer:         {map[string]any{"targetPlayerId": 5}, map[string]any{"targetPlayerId": "no-such-player"}},
	dto.MessageTypeConvertToBot:       {map[string]any{"targetPlayerId": "no-such-player"}},
	dto.MessageTypeEndGame:            {map[string]any{}},
	dto.MessageTypeUpdateGameSettings: {map[string]any{"maxPlayers": -1}},
	dto.MessageTypeSetPlayerColor:     {map[string]any{"color": 5}, map[string]any{"color": "no-such-color"}},
	dto.MessageTypeChatMessage:        {map[string]any{"message": ""}, map[string]any{"message": 5}, map[string]any{"message": strings.Repeat("x", 501)}},
	dto.MessageTypeEmoteSend:          {map[string]any{"emote": "no-such-emote"}},
	dto.MessageTypeBotRetry:           {map[string]any{"playerId": 5}},
	dto.MessageTypeBotInspect:         {map[string]any{"playerId": 5}},
	dto.MessageTypeKickSpectator:      {map[string]any{"targetSpectatorId": 5}},
}

// open lists the message types a bound client may send whatever its role, so a
// bystander or spectator gets a reply that is not an error.
var open = map[dto.MessageType]dto.MessageType{
	dto.MessageTypeQuotePayment: dto.MessageTypePaymentQuote,
	dto.MessageTypeRequestLogs:  "",
}

func TestContract_EveryMessageTypeIsCovered(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	for _, messageType := range srv.MessageTypes() {
		if _, ok := contracts[messageType]; !ok {
			t.Errorf("message type %q has no contract cases", messageType)
		}
	}
	for messageType := range contracts {
		if !slices.Contains(srv.MessageTypes(), messageType) {
			t.Errorf("contract lists %q, which the server does not handle", messageType)
		}
	}
}

func TestContract_UnknownTypesAndBrokenFramesAreRejected(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	game := srv.Lobby(t, nil, "alice", "bob")
	alice, bob := game.Players[0], game.Players[1]

	alice.Send(t, "no-such-type", map[string]any{})
	if got := alice.AwaitError(t); got.RequestType != "no-such-type" {
		t.Fatalf("error should name the rejected type, got %q", got.RequestType)
	}
	for _, frame := range []string{"not json", `{"type": 5}`, `[]`, `{"type": "chat-message", "payload": {"message": "unterminated`} {
		alice.SendRaw(t, []byte(frame))
		alice.AwaitError(t)
	}
	bob.ExpectQuiet(t)

	alice.Send(t, dto.MessageTypeChatMessage, map[string]any{"message": "still connected"})
	bob.Await(t, dto.MessageTypeChatUpdate, nil)
}

func TestContract_RejectedRequestsReachOnlyTheSender(t *testing.T) {
	t.Parallel()
	for messageType, specific := range contracts {
		t.Run(string(messageType), func(t *testing.T) {
			t.Parallel()
			srv := harness.Start(t)
			game := srv.Play(t, nil, "host", "second", "third")
			bystander := bystanderOf(t, game)
			spectator := srv.Dial(t, "spectator")
			spectator.Spectate(t, game.ID, "spectator")
			unbound := srv.Dial(t, "unbound")
			game.SyncAll(t)

			payloads := append(slices.Clone(garbage), specific...)
			for role, sender := range map[role]*harness.Client{unboundRole: unbound, spectatorRole: spectator, playerRole: bystander} {
				for _, payload := range payloads {
					sender.Send(t, messageType, payload)
					expectRejected(t, sender, role, messageType, payload)
				}
			}
			for _, c := range game.Players {
				c.ExpectQuiet(t)
			}
			spectator.ExpectQuiet(t)
		})
	}
}

func TestContract_PrivilegedPlayersCannotCrashTheServer(t *testing.T) {
	t.Parallel()
	for messageType, specific := range contracts {
		t.Run(string(messageType), func(t *testing.T) {
			t.Parallel()
			srv := harness.Start(t)
			game := srv.Play(t, func(s *dto.GameSetupDto) { s.DevelopmentMode = true }, "host", "second", "third")
			senders := []*harness.Client{game.Host(), game.Current(t)}
			for _, sender := range senders {
				for _, payload := range append(slices.Clone(garbage), specific...) {
					if !sender.TrySend(t, messageType, payload) {
						break
					}
					replies, ok := sender.TrySync(t)
					if !ok {
						break
					}
					for _, msg := range replies {
						if msg.Type != dto.MessageTypeError {
							continue
						}
						var e dto.ErrorPayload
						msg.Decode(t, &e)
						if strings.Contains(e.Message, "Internal server error") {
							t.Fatalf("%s with payload %s crashed its handler", messageType, mustJSON(payload))
						}
					}
				}
			}
			watcher := srv.Dial(t, "watcher")
			watcher.Sync(t)
		})
	}
}

// bystanderOf returns a player who is neither the host nor the player whose turn it is.
func bystanderOf(t *testing.T, game *harness.Game) *harness.Client {
	t.Helper()
	current := game.Current(t)
	for _, c := range game.Players[1:] {
		if c != current {
			return c
		}
	}
	t.Fatalf("no bystander in a game of %d", len(game.Players))
	return nil
}

type role int

const (
	unboundRole role = iota
	spectatorRole
	playerRole
)

func expectRejected(t *testing.T, sender *harness.Client, as role, messageType dto.MessageType, payload any) {
	t.Helper()
	replies := sender.Sync(t)
	if messageType == dto.MessageTypeBotInspect && as == playerRole && stopsInspection(payload) {
		for _, msg := range replies {
			if msg.Type != dto.MessageTypeLogUpdate {
				t.Fatalf("%s: stopping bot inspection should not reply, got %s", sender.Name, msg.Type)
			}
		}
		return
	}
	if reply, ok := open[messageType]; ok && as != unboundRole {
		for _, msg := range replies {
			if msg.Type == dto.MessageTypeError {
				t.Fatalf("%s: %s with %s should be allowed, got error %s", sender.Name, messageType, mustJSON(payload), short(msg.Payload))
			}
		}
		if reply != "" && !slices.ContainsFunc(replies, func(m harness.Message) bool { return m.Type == reply }) {
			t.Fatalf("%s: %s with %s should reply %s", sender.Name, messageType, mustJSON(payload), reply)
		}
		return
	}
	if messageType == dto.MessageTypeQuotePayment {
		if !slices.ContainsFunc(replies, func(m harness.Message) bool { return m.Type == dto.MessageTypePaymentQuote }) {
			t.Fatalf("%s: quote-payment should always be answered", sender.Name)
		}
		return
	}
	var errors []dto.ErrorPayload
	for _, msg := range replies {
		switch msg.Type {
		case dto.MessageTypeError:
			var e dto.ErrorPayload
			msg.Decode(t, &e)
			errors = append(errors, e)
		case dto.MessageTypeLogUpdate:
		default:
			t.Fatalf("%s: %s with %s was not rejected: got %s %s", sender.Name, messageType, mustJSON(payload), msg.Type, short(msg.Payload))
		}
	}
	if len(errors) != 1 {
		t.Fatalf("%s: %s with %s should be rejected with one error, got %d", sender.Name, messageType, mustJSON(payload), len(errors))
	}
	if errors[0].RequestType != messageType {
		t.Fatalf("%s: error for %s names request type %q", sender.Name, messageType, errors[0].RequestType)
	}
	if errors[0].Message == "" || strings.Contains(errors[0].Message, "Internal server error") {
		t.Fatalf("%s: %s with %s failed with %q", sender.Name, messageType, mustJSON(payload), errors[0].Message)
	}
}

// stopsInspection reports whether a bot-inspect payload names no bot, which is how a
// client stops inspecting.
func stopsInspection(payload any) bool {
	switch p := payload.(type) {
	case nil:
		return true
	case map[string]any:
		switch id := p["playerId"].(type) {
		case nil:
			return true
		case string:
			return id == ""
		}
	}
	return false
}

func short(raw []byte) string {
	if len(raw) > 200 {
		return string(raw[:200]) + "..."
	}
	return string(raw)
}

func mustJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}
