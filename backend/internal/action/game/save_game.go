package game

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"openmars/internal/action"
	"openmars/internal/game"
	"openmars/internal/game/cards"
	"openmars/internal/game/datastore"
	"openmars/internal/game/player"
	"openmars/internal/game/save"
	"openmars/internal/game/shared"
)

// SaveBots provides bot lifecycle operations without coupling saves to the bot service.
type SaveBots interface {
	PrepareBot(gameID, playerID string)
	StartBot(gameID, playerID string) error
	StopBot(gameID, playerID string)
}

// SaveGameAction captures and restores games; callers serialize live access on the hub.
type SaveGameAction struct {
	repo        game.GameRepository
	logs        *game.InMemoryGameStateRepository
	catalog     save.Catalog
	fingerprint string
	version     string
	bots        SaveBots
	logger      *slog.Logger
}

func NewSaveGameAction(repo game.GameRepository, logs *game.InMemoryGameStateRepository, catalog save.Catalog, version string, bots SaveBots, logger *slog.Logger) (*SaveGameAction, error) {
	fingerprint, err := catalog.Fingerprint()
	if err != nil {
		return nil, err
	}
	return &SaveGameAction{repo: repo, logs: logs, catalog: catalog, fingerprint: fingerprint, version: version, bots: bots, logger: logger}, nil
}

// Validate decodes an untrusted document without modifying any live state.
func (a *SaveGameAction) Validate(data []byte) (*save.Document, error) {
	doc, err := save.Decode(data)
	if err != nil {
		return nil, err
	}
	if err := save.Validate(doc, a.catalog, a.fingerprint); err != nil {
		return nil, err
	}
	return doc, nil
}

// Capture returns a detached position and immutable history for host-only export.
func (a *SaveGameAction) Capture(ctx context.Context, gameID, playerID string) (*save.Document, error) {
	g, err := a.repo.Get(ctx, gameID)
	if err != nil {
		return nil, save.Failure("game_not_found", "This game is no longer available on the server.", err)
	}
	if playerID == "" || g.HostPlayerID() != playerID {
		return nil, save.Failure("forbidden", "Only the host can save this game.", nil)
	}
	if g.Status() != shared.GameStatusActive {
		return nil, save.Failure("not_ready", "Only games in progress can be saved.", nil)
	}
	return save.Capture(g, a.repo.DataStore(), a.logs, a.fingerprint, a.version)
}

// Encode completes pending historical score enrichment and writes the portable file.
// Run outside the executor; all inputs are detached or immutable.
func (a *SaveGameAction) Encode(doc *save.Document) ([]byte, error) {
	for i, entry := range doc.History {
		if entry.VPBreakdowns != nil {
			continue
		}
		store, err := datastore.NewDataStore()
		if err != nil {
			return nil, err
		}
		snapshot, err := game.RestoreGame(store, entry.State)
		if err != nil {
			return nil, err
		}
		copy := *entry
		copy.VPBreakdowns = ComputePlayerVPBreakdowns(snapshot, cards.ForGame(a.catalog.Cards, snapshot), a.catalog.Awards, a.catalog.Milestones)
		doc.History[i] = &copy
	}
	return save.Encode(doc)
}

// RestoreRuntime reconstructs derived behavior without performing a game action.
func (a *SaveGameAction) RestoreRuntime(ds *datastore.DataStore, state *datastore.GameState) (*game.Game, error) {
	g, err := game.RestoreGame(ds, state)
	if err != nil {
		return nil, err
	}
	effective, err := cards.Resolve(a.catalog.Cards, g.Settings())
	if err != nil {
		return nil, err
	}
	g.SetVPCardLookup(cards.NewVPCardLookupAdapter(effective))
	g.Colonies().SetDefinitions(a.catalog.Colonies.GetAll())
	processor := cards.NewCorporationProcessor(effective, a.catalog.Awards, a.logger)
	for _, p := range g.GetAllPlayers() {
		action.SetupPlayerCardStore(p, g, effective, a.catalog.Colonies)
		for _, id := range p.Hand().Cards() {
			card, err := effective.GetByID(id)
			if err != nil {
				return nil, err
			}
			p.CardStateStore().SetState(id, action.CalculatePlayerCardState(card, p, g, effective, a.catalog.Colonies))
		}
		for _, effect := range p.Effects().List() {
			action.SubscribePassiveEffectToEvents(context.Background(), g, p, effect, a.logger, effective)
		}
		if g.GetForcedFirstAction(p.ID()) != nil {
			processor.RestoreFirstActionExecutor(g, p.ID())
		}
	}
	action.SetupModeBehaviors(context.Background(), g, effective)
	return g, nil
}

// PreparedGameImport owns a validated, detached game ready for atomic registration.
// Its fields are private so transport code cannot change validated state.
type PreparedGameImport struct {
	doc       *save.Document
	lobby     *game.ResumeLobby
	installed bool
}

// PrepareImport validates and hydrates in isolation, outside the live game executor.
func (a *SaveGameAction) PrepareImport(doc *save.Document, seatID, name, botToken string) (*PreparedGameImport, error) {
	if err := save.Validate(doc, a.catalog, a.fingerprint); err != nil {
		return nil, err
	}
	data, copyErr := save.Encode(doc)
	if copyErr != nil {
		return nil, copyErr
	}
	doc, copyErr = save.Decode(data)
	if copyErr != nil {
		return nil, copyErr
	}
	if err := validateResumeName(name); err != nil {
		return nil, err
	}
	host := doc.State.Players[seatID]
	if host == nil || host.PlayerType != "human" || host.HasExited {
		return nil, save.Failure("seat_unavailable", "Choose an available human seat.", nil)
	}
	newID := uuid.NewString()
	doc.State.ID = newID
	doc.State.HostPlayerID = seatID
	doc.State.Settings.ClaudeOAuthToken = strings.TrimSpace(botToken)
	lobby := &game.ResumeLobby{SavedNames: map[string]string{}, Claimed: map[string]bool{seatID: true}}
	for id, p := range doc.State.Players {
		lobby.SavedNames[id] = p.Name
		p.Connected = false
		p.BotStatus = ""
		p.BotError = ""
	}
	host.Name = strings.TrimSpace(name)
	for _, h := range doc.History {
		h.GameID = newID
		h.State.ID = newID
	}
	for i := range doc.Log {
		doc.Log[i].GameID = newID
	}
	staged, err := datastore.NewDataStore()
	if err != nil {
		return nil, err
	}
	if _, err := a.RestoreRuntime(staged, doc.State); err != nil {
		return nil, err
	}
	return &PreparedGameImport{doc: doc, lobby: lobby}, nil
}

// Import registers a prepared game on the executor, then starts bot preparation.
func (a *SaveGameAction) Import(ctx context.Context, prepared *PreparedGameImport) (_ *game.Game, err error) {
	if prepared == nil || prepared.installed {
		return nil, save.Failure("not_ready", "This import has already been used. Select the save again.", nil)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	prepared.installed = true
	doc := prepared.doc
	newID := doc.State.ID
	registered := false
	defer func() {
		if !registered {
			_ = a.repo.Delete(context.Background(), newID)
			_ = a.repo.DataStore().DeleteGameHistory(newID)
			a.logs.DeleteLog(newID)
		}
	}()
	g, err := a.RestoreRuntime(a.repo.DataStore(), doc.State)
	if err != nil {
		return nil, err
	}
	g.SetResumeLobby(prepared.lobby)
	if err := a.repo.DataStore().RestoreHistory(newID, doc.History); err != nil {
		return nil, err
	}
	a.logs.RestoreLog(newID, doc.Log, doc.LogBaseline)
	if err := a.repo.Create(ctx, g); err != nil {
		return nil, err
	}
	registered = true
	for _, p := range g.GetAllPlayers() {
		if p.IsBot() && !p.HasExited() {
			if a.bots == nil || doc.State.Settings.ClaudeOAuthToken == "" {
				p.SetBotStatus(player.BotStatusFailed)
				p.SetBotError("Enter a Claude OAuth token to reconnect this bot.")
			} else {
				p.SetBotStatus(player.BotStatusLoading)
				a.bots.PrepareBot(newID, p.ID())
			}
		}
	}
	return g, nil
}

func validateResumeName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > game.MaxPlayerNameLength || strings.ContainsFunc(name, unicode.IsControl) {
		return save.Failure("invalid_name", fmt.Sprintf("Enter a name of 1–%d characters.", game.MaxPlayerNameLength), nil)
	}
	return nil
}

// ClaimSeat reserves one human position and fixes the player's identity until released.
func (a *SaveGameAction) ClaimSeat(ctx context.Context, gameID, currentID, seatID, name string) error {
	g, err := a.repo.Get(ctx, gameID)
	if err != nil {
		return save.Failure("game_not_found", "This game is no longer available on the server.", err)
	}
	lobby := g.ResumeLobby()
	if lobby == nil {
		return save.Failure("not_ready", "This game is no longer waiting to resume.", nil)
	}
	if currentID != "" && currentID != seatID {
		return save.Failure("invalid_request", "You have already joined a seat.", nil)
	}
	if err := validateResumeName(name); err != nil {
		return err
	}
	p, err := g.GetPlayer(seatID)
	if err != nil {
		return save.Failure("seat_unavailable", "This seat is unavailable. Choose another seat.", err)
	}
	if p.IsBot() || p.HasExited() {
		return save.Failure("seat_unavailable", "This seat is unavailable. Choose another seat.", nil)
	}
	if lobby.Claimed[seatID] && seatID != currentID {
		return save.Failure("seat_taken", "Seat already taken", nil)
	}
	if lobby.Claimed[seatID] && p.Name() != strings.TrimSpace(name) {
		return save.Failure("invalid_request", "Your name is fixed after joining.", nil)
	}
	lobby.Claimed[seatID] = true
	g.SetResumeLobby(lobby)
	return a.repo.DataStore().UpdatePlayer(gameID, seatID, func(s *datastore.PlayerState) { s.Name = strings.TrimSpace(name); s.Connected = true })
}

// ReleaseSeat releases a claim, never the saved position itself.
func (a *SaveGameAction) ReleaseSeat(ctx context.Context, gameID, requesterID, seatID string) error {
	g, err := a.repo.Get(ctx, gameID)
	if err != nil {
		return save.Failure("game_not_found", "This game is no longer available on the server.", err)
	}
	if g.ResumeLobby() == nil || requesterID != g.HostPlayerID() {
		return save.Failure("forbidden", "Only the host can release a seat.", nil)
	}
	if seatID == requesterID {
		return save.Failure("seat_unavailable", "You cannot release your own seat.", nil)
	}
	p, err := g.GetPlayer(seatID)
	if err != nil {
		return save.Failure("seat_unavailable", "This seat is unavailable. Choose another seat.", err)
	}
	if p.IsBot() || p.HasExited() {
		return save.Failure("seat_unavailable", "This seat cannot be released.", nil)
	}
	lobby := g.ResumeLobby()
	delete(lobby.Claimed, seatID)
	g.SetResumeLobby(lobby)
	p.SetConnected(false)
	return nil
}

// Resume releases the session gate without advancing the saved gameplay position.
func (a *SaveGameAction) Resume(ctx context.Context, gameID, requesterID string) error {
	g, err := a.repo.Get(ctx, gameID)
	if err != nil {
		return save.Failure("game_not_found", "This game is no longer available on the server.", err)
	}
	lobby := g.ResumeLobby()
	if lobby == nil || requesterID != g.HostPlayerID() {
		return save.Failure("forbidden", "Only the host can resume this game.", nil)
	}
	for _, p := range g.GetAllPlayers() {
		if p.HasExited() {
			continue
		}
		if p.IsBot() {
			if a.bots == nil || p.BotStatus() != player.BotStatusReady {
				return save.Failure("not_ready", "Wait for every bot to be ready before resuming.", nil)
			}
			continue
		}
		if !lobby.Claimed[p.ID()] || !p.IsConnected() {
			return save.Failure("not_ready", "Wait for every player to join before resuming.", nil)
		}
	}
	started := []string{}
	for _, p := range g.GetAllPlayers() {
		if p.IsBot() && !p.HasExited() {
			if err := a.bots.StartBot(gameID, p.ID()); err != nil {
				for _, id := range started {
					a.bots.StopBot(gameID, id)
				}
				return save.Failure("internal_error", "A bot couldn’t start. Reconnect the bots and try again.", err)
			}
			started = append(started, p.ID())
		}
	}
	g.SetResumeLobby(nil)
	return nil
}

// SetBotToken lets the host repair credentials without changing saved game settings.
func (a *SaveGameAction) SetBotToken(ctx context.Context, gameID, requesterID, token string) error {
	g, err := a.repo.Get(ctx, gameID)
	if err != nil {
		return save.Failure("game_not_found", "This game is no longer available on the server.", err)
	}
	if g.ResumeLobby() == nil || requesterID != g.HostPlayerID() {
		return save.Failure("forbidden", "Only the host can reconnect bots.", nil)
	}
	if a.bots == nil || strings.TrimSpace(token) == "" {
		return save.Failure("invalid_token", "Enter a Claude OAuth token to reconnect the bots.", nil)
	}
	settings := g.Settings()
	settings.ClaudeOAuthToken = strings.TrimSpace(token)
	g.UpdateSettings(ctx, settings)
	for _, p := range g.GetAllPlayers() {
		if p.IsBot() && !p.HasExited() {
			p.SetBotStatus(player.BotStatusLoading)
			a.bots.PrepareBot(gameID, p.ID())
		}
	}
	return nil
}
