package game_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	gameaction "openmars/internal/action/game"
	"openmars/internal/delivery/dto"
	httpdelivery "openmars/internal/delivery/http"
	"openmars/internal/delivery/websocket/core"
	"openmars/internal/game"
	"openmars/internal/game/save"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

type saveBodyReader struct{ remaining, read int64 }

func (r *saveBodyReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > r.remaining {
		n = r.remaining
	}
	for i := int64(0); i < n; i++ {
		p[i] = ' '
	}
	r.remaining -= n
	r.read += n
	return int(n), nil
}
func (r *saveBodyReader) Close() error { return nil }

func TestSaveHTTPBodyLimits(t *testing.T) {
	handler := httpdelivery.NewSaveHandler(nil, nil)
	for _, endpoint := range []struct {
		name   string
		handle http.HandlerFunc
	}{{"validate", handler.Validate}, {"import", handler.Import}} {
		t.Run(endpoint.name, func(t *testing.T) {
			reader := &saveBodyReader{remaining: save.MaxBytes + 1}
			request := httptest.NewRequest(http.MethodPost, "/", nil)
			request.Body = reader
			request.ContentLength = save.MaxBytes + 1
			response := httptest.NewRecorder()
			endpoint.handle(response, request)
			testutil.AssertEqual(t, http.StatusRequestEntityTooLarge, response.Code, "declared oversize")
			testutil.AssertEqual(t, int64(0), reader.read, "must not read declared oversize body")
			// Chunked/unknown length must enforce the same limit while reading.
			request.ContentLength = -1
			response = httptest.NewRecorder()
			endpoint.handle(response, request)
			testutil.AssertEqual(t, http.StatusRequestEntityTooLarge, response.Code, "stream oversize")
			testutil.AssertEqual(t, int64(save.MaxBytes+1), reader.read, "reads only one byte beyond cap")
			var failure dto.GameSaveErrorResponse
			testutil.AssertNoError(t, json.Unmarshal(response.Body.Bytes(), &failure), "error JSON")
			testutil.AssertEqual(t, "save_too_large", failure.Code, "stable size code")
		})
	}
}

func TestSaveValidationBoundaries(t *testing.T) {
	g, _, _, action := saveFixture(t)
	valid := encodeSave(t, g, action)
	cases := map[string][]byte{
		"array": []byte(`[]`), "null": []byte(`null`), "empty": nil, "invalid": []byte(`asd`),
		"unknown":      bytes.Replace(valid, []byte(`"formatVersion":1`), []byte(`"formatVersion":1,"unknown":0`), 1),
		"missing":      bytes.Replace(valid, []byte(`"formatVersion":1,`), nil, 1),
		"duplicate":    bytes.Replace(valid, []byte(`"formatVersion":1`), []byte(`"formatVersion":1,"formatVersion":1`), 1),
		"trailing":     append(bytes.Clone(valid), []byte(`{}`)...),
		"deep":         []byte(strings.Repeat(`[`, 66) + `0` + strings.Repeat(`]`, 66)),
		"long-value":   []byte(`{"x":"` + strings.Repeat("x", save.MaxStringBytes+1) + `"}`),
		"long-key":     []byte(`{"` + strings.Repeat("x", save.MaxStringBytes+1) + `":0}`),
		"large-array":  []byte(`[` + strings.Repeat(`0,`, save.MaxEntries) + `0]`),
		"null-counter": bytes.Replace(valid, []byte(`"currentTurnActions":0`), []byte(`"currentTurnActions":null`), 1),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := action.Validate(data); err == nil {
				t.Fatal("invalid document accepted")
			}
		})
	}
	for _, mutate := range []func(*save.Document){
		func(d *save.Document) { d.State.CurrentTurnActions = -2 }, func(d *save.Document) { d.State.CurrentTurnTotalActions = -2 }, func(d *save.Document) { d.State.DrawnCardCount = -1 }, func(d *save.Document) { d.State.ShuffleCount = -1 }, func(d *save.Document) { d.State.GlobalActionCounter = -1 },
		func(d *save.Document) { d.State.PendingTileSelections["unknown"] = nil }, func(d *save.Document) { d.RulesVersion++ },
	} {
		doc, err := save.Decode(valid)
		testutil.AssertNoError(t, err, "decode")
		mutate(doc)
		data, err := save.Encode(doc)
		testutil.AssertNoError(t, err, "encode")
		if _, err = action.Validate(data); err == nil {
			t.Fatal("invalid state accepted")
		}
	}
	// The wire limit is inclusive; legal trailing whitespace must not invalidate a save.
	atLimit := make([]byte, save.MaxBytes)
	copy(atLimit, valid)
	for i := len(valid); i < len(atLimit); i++ {
		atLimit[i] = ' '
	}
	_, err := action.Validate(atLimit)
	testutil.AssertNoError(t, err, "exact size limit")
	if _, err := action.Validate(append(atLimit, ' ')); err == nil {
		t.Fatal("oversize accepted")
	}
}

func TestSaveHTTPPublicErrors(t *testing.T) {
	g, _, _, action := saveFixture(t)
	hub := core.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	h := httpdelivery.NewSaveHandler(action, hub)
	for _, tc := range []struct {
		name string
		body []byte
		code string
	}{{"wrong-file", []byte(`[]`), "invalid_save"}, {"paste", []byte(`asd`), "invalid_json"}, {"missing-fields", []byte(`{}`), "invalid_save"}} {
		t.Run(tc.name, func(t *testing.T) {
			for _, handle := range []http.HandlerFunc{h.Validate, h.Import} {
				out := httptest.NewRecorder()
				handle(out, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(tc.body)))
				testutil.AssertEqual(t, 400, out.Code, "bad request")
				var response dto.GameSaveErrorResponse
				testutil.AssertNoError(t, json.Unmarshal(out.Body.Bytes(), &response), "response")
				testutil.AssertEqual(t, tc.code, response.Code, "code")
				for _, private := range []string{"unmarshal", "save.Document", "invalid character", "$:"} {
					if strings.Contains(response.Message, private) {
						t.Fatalf("private error exposed: %s", response.Message)
					}
				}
			}
		})
	}
	for _, tc := range []struct {
		id, player string
		status     int
	}{{g.ID(), "bob", 403}, {"missing", "alice", 404}} {
		out := httptest.NewRecorder()
		h.Export(out, mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/?playerId="+tc.player, nil), map[string]string{"gameId": tc.id}))
		testutil.AssertEqual(t, tc.status, out.Code, "export status")
	}
	// A live invariant failure is a server error, never exposed as an invalid user file.
	testutil.AssertNoError(t, g.SetCurrentTurnActions(ctx, -2), "broken live state")
	out := httptest.NewRecorder()
	h.Export(out, mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/?playerId=alice", nil), map[string]string{"gameId": g.ID()}))
	testutil.AssertEqual(t, 500, out.Code, "export validation failure")
	if strings.Contains(out.Body.String(), "counter") {
		t.Fatal("internal invariant exposed")
	}
}

type failingSaveRepository struct {
	game.GameRepository
	attempted string
}

func (r *failingSaveRepository) Create(_ context.Context, g *game.Game) error {
	r.attempted = g.ID()
	return errors.New("injected registration failure")
}

func TestSaveImportRollbackAndCancellation(t *testing.T) {
	g, repo, logs, action := saveFixture(t)
	doc, err := action.Validate(encodeSave(t, g, action))
	testutil.AssertNoError(t, err, "valid save")
	// Use the fixture catalog so only the final repository registration fails.
	catalog := fullSaveCatalog(t)
	fresh := testutil.NewTestGameRepository(t)
	broken := &failingSaveRepository{GameRepository: fresh}
	a, err := gameaction.NewSaveGameAction(broken, logs, catalog, "test", nil, testutil.TestLogger())
	testutil.AssertNoError(t, err, "service")
	fingerprint, err := catalog.Fingerprint()
	testutil.AssertNoError(t, err, "fingerprint")
	originalFingerprint := doc.ContentFingerprint
	doc.ContentFingerprint = fingerprint
	prepared, err := a.PrepareImport(doc, "alice", "Alice", "")
	testutil.AssertNoError(t, err, "prepare")
	if _, err = a.Import(context.Background(), prepared); err == nil {
		t.Fatal("registration should fail")
	}
	if _, err = fresh.DataStore().GetGame(broken.attempted); err == nil {
		t.Fatal("orphaned state")
	}
	history, err := fresh.DataStore().GetGameHistory(broken.attempted)
	testutil.AssertNoError(t, err, "history")
	if len(history) != 0 {
		t.Fatal("orphaned history")
	}
	entries, baseline := logs.CaptureLog(broken.attempted)
	if len(entries) > 0 || baseline != nil {
		t.Fatal("orphaned log")
	}
	if _, err = a.Import(context.Background(), prepared); err == nil {
		t.Fatal("reused prepared import")
	}
	// Restore the fixture fingerprint for an independent cancellation test.
	doc.ContentFingerprint = originalFingerprint
	prepared, err = action.PrepareImport(doc, "alice", "Alice", "")
	testutil.AssertNoError(t, err, "prepare")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = action.Import(ctx, prepared); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost")
	}
	games, err := repo.List(context.Background(), nil)
	testutil.AssertNoError(t, err, "list")
	testutil.AssertEqual(t, 1, len(games), "source repository unchanged")
}

func TestSaveCompetingClaimsUseHub(t *testing.T) {
	g, _, _, a := saveFixture(t)
	doc, err := a.Validate(encodeSave(t, g, a))
	testutil.AssertNoError(t, err, "validate")
	restored, err := importSave(t, a, doc, "alice", "alice")
	testutil.AssertNoError(t, err, "import")
	hub := core.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	results := make(chan error, 2)
	for _, name := range []string{"First", "Second"} {
		go func(name string) {
			var claim error
			err := hub.Do(ctx, func() { claim = a.ClaimSeat(ctx, restored.ID(), "", "bob", name) })
			if err != nil {
				results <- err
			} else {
				results <- claim
			}
		}(name)
	}
	successes := 0
	for i := 0; i < 2; i++ {
		if claimErr := <-results; claimErr == nil {
			successes++
		} else {
			testutil.AssertEqual(t, "seat_taken", save.PublicError(claimErr, "").Code, "loser receives a seat conflict")
		}
	}
	testutil.AssertEqual(t, 1, successes, "exactly one claimant wins")
	var reconnectErr, resumeErr error
	testutil.AssertNoError(t, hub.Do(ctx, func() {
		if a.ReleaseSeat(ctx, restored.ID(), "bob", "alice") == nil {
			t.Error("guest released host")
		}
		if a.Resume(ctx, restored.ID(), "bob") == nil {
			t.Error("guest resumed game")
		}
		if a.Resume(ctx, restored.ID(), "alice") == nil {
			t.Error("disconnected host resumed")
		}
		reconnectErr = a.ClaimSeat(ctx, restored.ID(), "alice", "alice", "alice")
		resumeErr = a.Resume(ctx, restored.ID(), "alice")
	}), "hub")
	testutil.AssertNoError(t, reconnectErr, "reconnect host")
	testutil.AssertNoError(t, resumeErr, "ready resume")
}

type partialSaveBots struct {
	started, stopped []string
	fail             bool
}

func (b *partialSaveBots) PrepareBot(_, _ string) {}
func (b *partialSaveBots) StartBot(_, id string) error {
	if b.fail && len(b.started) == 1 {
		return errors.New("provider secret must stay private")
	}
	b.started = append(b.started, id)
	return nil
}
func (b *partialSaveBots) StopBot(_, id string) { b.stopped = append(b.stopped, id) }

func TestSavePartialBotStartupRollback(t *testing.T) {
	w := newSaveWorld(t, 3, shared.GameSettings{Features: []string{}})
	w.active(t)
	for _, id := range []string{"p1", "p2"} {
		w.g.State().Players[id].PlayerType = "bot"
	}
	doc, err := w.action.Capture(context.Background(), w.g.ID(), "p0")
	testutil.AssertNoError(t, err, "capture")
	repo := testutil.NewTestGameRepository(t)
	bots := &partialSaveBots{fail: true}
	a, err := gameaction.NewSaveGameAction(repo, game.NewInMemoryGameStateRepository(), w.catalog, "test", bots, testutil.TestLogger())
	testutil.AssertNoError(t, err, "service")
	prepared, err := a.PrepareImport(doc, "p0", "p0", "fresh credential")
	testutil.AssertNoError(t, err, "prepare")
	restored, err := a.Import(context.Background(), prepared)
	testutil.AssertNoError(t, err, "import")
	testutil.AssertNoError(t, a.ClaimSeat(context.Background(), restored.ID(), "p0", "p0", "p0"), "host joins")
	for _, id := range []string{"p1", "p2"} {
		p, _ := restored.GetPlayer(id)
		p.SetBotStatus("ready")
	}
	err = a.Resume(context.Background(), restored.ID(), "p0")
	if err == nil {
		t.Fatal("bot startup should fail")
	}
	if restored.ResumeLobby() == nil || len(bots.started) != 1 || len(bots.stopped) != 1 || bots.started[0] != bots.stopped[0] {
		t.Fatal("partial startup was not rolled back")
	}
	public := save.PublicError(err, "fallback")
	if strings.Contains(public.Message, "secret") {
		t.Fatal("provider error leaked")
	}
	botPlayer, err := restored.GetPlayer("p1")
	testutil.AssertNoError(t, err, "bot player")
	botPlayer.SetBotError("provider secret diagnostic")
	view := dto.ToResumeGameDto(restored, "p0", w.catalog.Cards)
	for _, seat := range view.ResumeLobby.Seats {
		if strings.Contains(seat.BotError, "secret") {
			t.Fatal("provider error leaked through resume lobby")
		}
	}
	bots.fail = false
	bots.started = nil
	testutil.AssertNoError(t, a.Resume(context.Background(), restored.ID(), "p0"), "retry")
	testutil.AssertEqual(t, 2, len(bots.started), "both bots started once on retry")
}
