package game

import (
	"context"
	"fmt"
	"log/slog"
	mathrand "math/rand/v2"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"openmars/internal/events"
	"openmars/internal/game/board"
	"openmars/internal/game/colonies"
	"openmars/internal/game/colony"
	"openmars/internal/game/datastore"
	"openmars/internal/game/deck"
	"openmars/internal/game/global_parameters"
	"openmars/internal/game/player"
	"openmars/internal/game/projectfunding"
	"openmars/internal/game/shared"
)

type Game struct {
	modeBehaviorsRegistered bool
	resumeLobby             *ResumeLobby
	mu                      sync.RWMutex
	ds                      *datastore.DataStore
	id                      string
	globalParameters        *global_parameters.GlobalParameters
	currentTurn             *Turn
	board                   *board.Board
	colonies                *colonies.Colonies
	deck                    *deck.Deck
	players                 map[string]*player.Player
	eventBus                *events.EventBusImpl
	milestones              *Milestones
	awards                  *Awards
	vpCardLookup            VPCardLookup
	firstActionExecutors    map[string]func(context.Context, *Game, string) error
	executingFirstAction    atomic.Bool
}

func (g *Game) update(fn func(s *datastore.GameState)) {
	if err := g.ds.UpdateGame(g.id, fn); err != nil {
		slog.Default().Warn("Failed to update game state", slog.String("game_id", g.id), slog.Any("error", err))
	}
}

func (g *Game) read(fn func(s *datastore.GameState)) {
	if err := g.ds.ReadGame(g.id, fn); err != nil {
		slog.Default().Warn("Failed to read game state", slog.String("game_id", g.id), slog.Any("error", err))
	}
}

// NewGame creates a new game with the given settings and initial board tiles.
// The DataStore is used as the single point of entry for all state reads/writes.
func NewGame(
	ds *datastore.DataStore,
	id string,
	hostPlayerID string,
	settings shared.GameSettings,
	initialTiles []board.Tile,
) *Game {
	now := time.Now()

	eventBus := events.NewEventBus()

	initTemp := DefaultTemperature
	initOxy := DefaultOxygen
	initOcean := DefaultOceans
	initVenus := DefaultVenus
	if settings.Temperature != nil {
		initTemp = *settings.Temperature
	}
	if settings.Oxygen != nil {
		initOxy = *settings.Oxygen
	}
	if settings.Oceans != nil {
		initOcean = *settings.Oceans
	}
	if settings.Venus != nil {
		initVenus = *settings.Venus
	}

	state := &datastore.GameState{
		ID:                         id,
		CreatedAt:                  now,
		UpdatedAt:                  now,
		Status:                     shared.GameStatusLobby,
		Settings:                   settings.Clone(),
		HostPlayerID:               hostPlayerID,
		CurrentPhase:               shared.GamePhaseWaitingForGameStart,
		Generation:                 1,
		Temperature:                initTemp,
		Oxygen:                     initOxy,
		Oceans:                     initOcean,
		MaxOceans:                  global_parameters.MaxOceans,
		Venus:                      initVenus,
		PlayerOrder:                []string{},
		TurnOrder:                  []string{},
		Players:                    make(map[string]*datastore.PlayerState),
		ClaimedMilestones:          []shared.ClaimedMilestone{},
		FundedAwards:               []shared.FundedAward{},
		SelectedMilestones:         []string{},
		SelectedAwards:             []string{},
		Spectators:                 make(map[string]*shared.SpectatorState),
		ChatMessages:               []shared.ChatMessage{},
		PendingTileSelections:      make(map[string]*shared.PendingTileSelection),
		PendingTileSelectionQueues: make(map[string]*shared.PendingTileSelectionQueue),
		ForcedFirstActions:         make(map[string]*shared.ForcedFirstAction),
		ProductionPhases:           make(map[string]*shared.ProductionPhase),
		SelectCorporationPhases:    make(map[string]*shared.SelectCorporationPhase),
		SelectStartingCardsPhases:  make(map[string]*shared.SelectStartingCardsPhase),
		SelectPreludeCardsPhases:   make(map[string]*shared.SelectPreludeCardsPhase),
		DeferredStartingChoices:    make(map[string]*shared.DeferredStartingChoices),
		TradeFleets:                make(map[string]colony.TradeFleet),
		Seed:                       mathrand.Uint64(),
	}

	// Insert state into DataStore so components can read/write through it
	txn := ds.BeginTxn()
	if err := txn.InsertGame(state); err != nil {
		slog.Default().Error("Failed to insert game state", slog.String("game_id", id), slog.Any("error", err))
	}
	txn.Commit()

	ds.RecordInitialHistory(state)

	g := &Game{
		ds:               ds,
		id:               id,
		globalParameters: global_parameters.NewGlobalParameters(ds, id, eventBus),
		board:            board.NewBoardWithTiles(&state.Tiles, id, initialTiles, eventBus),
		colonies:         colonies.NewColonies(ds, id, eventBus),
		players:          make(map[string]*player.Player),
		eventBus:         eventBus,
		milestones:       NewMilestones(ds, id, eventBus),
		awards:           NewAwards(ds, id, eventBus),
	}

	g.subscribeToGenerationalEvents()
	g.subscribeToOceanSpaceEvents()
	g.subscribeToGlobalParameterBonuses()

	return g
}

// ID returns the game ID
func (g *Game) ID() string {
	return g.id
}

// RNG stream identifiers. Each purpose derives an independent deterministic stream
// from the game Seed so that adding a draw in one area does not perturb another.
const (
	rngStreamSetup uint64 = 0x5E7079 // turn order, colony + milestone/award selection
	rngStreamDeck  uint64 = 0xDECC00 // deck shuffles (offset by ShuffleCount per reshuffle)
)

// Seed returns the master RNG seed for this game.
func (g *Game) Seed() uint64 {
	var seed uint64
	g.read(func(s *datastore.GameState) { seed = s.Seed })
	return seed
}

// SetSeed overrides the master RNG seed. Must be called before InitDeck / StartGame
// (e.g. by the replay harness) for the override to take effect on setup randomness.
func (g *Game) SetSeed(seed uint64) {
	g.update(func(s *datastore.GameState) { s.Seed = seed })
}

// SetupRand returns a deterministic RNG for one-time game setup (turn order,
// colony and milestone/award selection), derived from the game Seed.
func (g *Game) SetupRand() *mathrand.Rand {
	return mathrand.New(mathrand.NewPCG(g.Seed(), rngStreamSetup))
}

func (g *Game) CreatedAt() time.Time {
	var v time.Time
	g.read(func(s *datastore.GameState) { v = s.CreatedAt })
	return v
}

func (g *Game) UpdatedAt() time.Time {
	var v time.Time
	g.read(func(s *datastore.GameState) { v = s.UpdatedAt })
	return v
}

func (g *Game) Status() shared.GameStatus {
	var v shared.GameStatus
	g.read(func(s *datastore.GameState) { v = s.Status })
	return v
}

func (g *Game) Settings() shared.GameSettings {
	var v shared.GameSettings
	g.read(func(s *datastore.GameState) { v = s.Settings.Clone() })
	return v
}

func (g *Game) UpdateSettings(ctx context.Context, settings shared.GameSettings) {
	g.update(func(s *datastore.GameState) {
		s.Settings = settings.Clone()
		s.UpdatedAt = time.Now()
	})
}

// ReplaceBoard replaces the board tiles (used when changing maps in lobby)
func (g *Game) ReplaceBoard(ctx context.Context, newTiles []board.Tile) {
	g.update(func(s *datastore.GameState) {
		tilesCopy := make([]board.Tile, len(newTiles))
		copy(tilesCopy, newTiles)
		s.Tiles = tilesCopy
		s.UpdatedAt = time.Now()
	})
}

func (g *Game) HostPlayerID() string {
	var v string
	g.read(func(s *datastore.GameState) { v = s.HostPlayerID })
	return v
}

func (g *Game) State() *datastore.GameState {
	state, _ := g.ds.GetGame(g.id)
	return state
}

// BotSpendUSD returns the total LLM spend of all bots in this game.
func (g *Game) BotSpendUSD() float64 {
	var v float64
	g.read(func(s *datastore.GameState) { v = s.BotSpendUSD })
	return v
}

// AddBotSpend adds to the game's bot LLM spend and returns the new total.
func (g *Game) AddBotSpend(amountUSD float64) float64 {
	var total float64
	g.update(func(s *datastore.GameState) {
		s.BotSpendUSD += amountUSD
		total = s.BotSpendUSD
	})
	return total
}

// EventBus returns the event bus for publishing domain events
func (g *Game) EventBus() *events.EventBusImpl {
	return g.eventBus
}

func (g *Game) CurrentPhase() shared.GamePhase {
	var v shared.GamePhase
	g.read(func(s *datastore.GameState) { v = s.CurrentPhase })
	return v
}

func (g *Game) Generation() int {
	var v int
	g.read(func(s *datastore.GameState) { v = s.Generation })
	return v
}

func (g *Game) PlayerOrder() []string {
	var order []string
	g.read(func(s *datastore.GameState) {
		order = make([]string, len(s.PlayerOrder))
		copy(order, s.PlayerOrder)
	})
	return order
}

func (g *Game) TurnOrder() []string {
	var order []string
	g.read(func(s *datastore.GameState) {
		order = make([]string, len(s.TurnOrder))
		copy(order, s.TurnOrder)
	})
	return order
}

func (g *Game) CurrentTurn() *Turn {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.currentTurn
}

func (g *Game) GlobalParameters() *global_parameters.GlobalParameters {
	return g.globalParameters
}

func (g *Game) Board() *board.Board {
	return g.board
}

func (g *Game) Colonies() *colonies.Colonies {
	return g.colonies
}

func (g *Game) Deck() *deck.Deck {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.deck
}

func (g *Game) SetDeck(d *deck.Deck) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.deck = d
	g.update(func(s *datastore.GameState) { s.UpdatedAt = time.Now() })
}

func (g *Game) InitDeck(projectCardIDs, corpIDs, preludeIDs []string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.deck = deck.NewDeck(g.ds, g.id, g.Seed(), projectCardIDs, corpIDs, preludeIDs)
	g.update(func(s *datastore.GameState) { s.UpdatedAt = time.Now() })
}

func (g *Game) Milestones() *Milestones {
	return g.milestones
}

func (g *Game) Awards() *Awards {
	return g.awards
}

// SelectedMilestones returns the milestone IDs selected for this game.
func (g *Game) SelectedMilestones() []string {
	var result []string
	g.read(func(s *datastore.GameState) {
		result = make([]string, len(s.SelectedMilestones))
		copy(result, s.SelectedMilestones)
	})
	return result
}

// SelectedAwards returns the award IDs selected for this game.
func (g *Game) SelectedAwards() []string {
	var result []string
	g.read(func(s *datastore.GameState) {
		result = make([]string, len(s.SelectedAwards))
		copy(result, s.SelectedAwards)
	})
	return result
}

// SetSelectedMilestones sets the milestone IDs available for this game.
func (g *Game) SetSelectedMilestones(milestoneIDs []string) {
	g.update(func(s *datastore.GameState) {
		s.SelectedMilestones = make([]string, len(milestoneIDs))
		copy(s.SelectedMilestones, milestoneIDs)
	})
}

// SetSelectedAwards sets the award IDs available for this game.
func (g *Game) SetSelectedAwards(awardIDs []string) {
	g.update(func(s *datastore.GameState) {
		s.SelectedAwards = make([]string, len(awardIDs))
		copy(s.SelectedAwards, awardIDs)
	})
}

func (g *Game) GetFinalScores() []shared.FinalScore {
	var result []shared.FinalScore
	g.read(func(s *datastore.GameState) {
		if s.FinalScores == nil {
			return
		}
		result = make([]shared.FinalScore, len(s.FinalScores))
		copy(result, s.FinalScores)
	})
	return result
}

func (g *Game) GetWinnerID() string {
	var v string
	g.read(func(s *datastore.GameState) { v = s.WinnerID })
	return v
}

func (g *Game) IsTie() bool {
	var v bool
	g.read(func(s *datastore.GameState) { v = s.IsTie })
	return v
}

func (g *Game) GetPlayer(playerID string) (*player.Player, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	p, exists := g.players[playerID]
	if !exists {
		return nil, fmt.Errorf("player %s not found in game %s", playerID, g.id)
	}
	return p, nil
}

func (g *Game) GetAllPlayers() []*player.Player {
	g.mu.RLock()
	defer g.mu.RUnlock()

	playerOrder := g.PlayerOrder()
	players := make([]*player.Player, 0, len(playerOrder))
	for _, id := range playerOrder {
		if p, exists := g.players[id]; exists {
			players = append(players, p)
		}
	}
	return players
}

// AddNewPlayer creates a new human player backed by this game's state and adds them to the game
func (g *Game) AddNewPlayer(ctx context.Context, playerID, playerName string) (*player.Player, error) {
	if err := g.ds.UpdateGame(g.id, func(s *datastore.GameState) {
		s.Players[playerID] = &datastore.PlayerState{
			ID:                 playerID,
			Name:               playerName,
			Connected:          true,
			PlayerType:         "human",
			TerraformRating:    20,
			HandCardIDs:        []string{},
			PlayedCardIDs:      []string{},
			ResourceStorage:    make(map[string]int),
			BonusTags:          make(map[shared.CardTag]int),
			GenerationalEvents: make(map[shared.GenerationalEvent]int),
		}
	}); err != nil {
		slog.Default().Error("Failed to add player to game state", slog.String("game_id", g.id), slog.String("player_id", playerID), slog.Any("error", err))
	}
	p := player.NewPlayer(g.ds, g.id, playerID, g.eventBus)
	if err := g.AddPlayer(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// AddNewBotPlayer creates a new bot player backed by this game's state and adds them to the game
func (g *Game) AddNewBotPlayer(ctx context.Context, botID, botName, persona string) (*player.Player, error) {
	if err := g.ds.UpdateGame(g.id, func(s *datastore.GameState) {
		s.Players[botID] = &datastore.PlayerState{
			ID:                 botID,
			Name:               botName,
			Connected:          false,
			PlayerType:         "bot",
			BotStatus:          string(player.BotStatusLoading),
			BotPersona:         persona,
			TerraformRating:    20,
			HandCardIDs:        []string{},
			PlayedCardIDs:      []string{},
			ResourceStorage:    make(map[string]int),
			BonusTags:          make(map[shared.CardTag]int),
			GenerationalEvents: make(map[shared.GenerationalEvent]int),
		}
	}); err != nil {
		slog.Default().Error("Failed to add bot player to game state", slog.String("game_id", g.id), slog.String("bot_id", botID), slog.Any("error", err))
	}
	p := player.NewPlayer(g.ds, g.id, botID, g.eventBus)
	if err := g.AddPlayer(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// PlayerIdentity is the part of a player that is kept when a game is rebuilt.
type PlayerIdentity struct {
	ID                 string
	Name               string
	Color              string
	PlayerType         string
	BotStatus          string
	BotPersona         string
	Connected          bool
	PendingDemoChoices *shared.PendingDemoChoices
}

// IdentityOf returns the identity of a player state.
func IdentityOf(s *datastore.PlayerState) PlayerIdentity {
	return PlayerIdentity{
		ID:                 s.ID,
		Name:               s.Name,
		Color:              s.Color,
		PlayerType:         s.PlayerType,
		BotStatus:          s.BotStatus,
		BotPersona:         s.BotPersona,
		Connected:          s.Connected,
		PendingDemoChoices: s.PendingDemoChoices,
	}
}

// AddRestoredPlayer adds a fresh player that keeps the given identity.
func (g *Game) AddRestoredPlayer(ctx context.Context, identity PlayerIdentity) (*player.Player, error) {
	if err := g.ds.UpdateGame(g.id, func(s *datastore.GameState) {
		s.Players[identity.ID] = &datastore.PlayerState{
			ID:                 identity.ID,
			Name:               identity.Name,
			Color:              identity.Color,
			Connected:          identity.Connected,
			PlayerType:         identity.PlayerType,
			BotStatus:          identity.BotStatus,
			BotPersona:         identity.BotPersona,
			PendingDemoChoices: identity.PendingDemoChoices,
			TerraformRating:    20,
			HandCardIDs:        []string{},
			PlayedCardIDs:      []string{},
			ResourceStorage:    make(map[string]int),
			BonusTags:          make(map[shared.CardTag]int),
			GenerationalEvents: make(map[shared.GenerationalEvent]int),
		}
	}); err != nil {
		return nil, fmt.Errorf("failed to restore player %s: %w", identity.ID, err)
	}
	p := player.NewPlayer(g.ds, g.id, identity.ID, g.eventBus)
	if err := g.AddPlayer(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// RestoreSocial copies spectators and chat from a previous state of this game.
func (g *Game) RestoreSocial(spectators map[string]*shared.SpectatorState, chat []shared.ChatMessage) {
	g.update(func(s *datastore.GameState) {
		s.Spectators = spectators
		s.ChatMessages = chat
	})
}

func (g *Game) AddPlayer(ctx context.Context, p *player.Player) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	g.mu.Lock()
	if _, exists := g.players[p.ID()]; exists {
		g.mu.Unlock()
		return fmt.Errorf("player %s already exists in game %s", p.ID(), g.id)
	}

	if p.Color() == "" {
		taken := make(map[string]bool, len(g.players))
		for _, existing := range g.players {
			if existing.Color() != "" {
				taken[existing.Color()] = true
			}
		}
		for _, c := range shared.PlayerColors {
			if !taken[c] {
				p.SetColor(c)
				break
			}
		}
	}

	g.players[p.ID()] = p
	g.update(func(s *datastore.GameState) {
		s.PlayerOrder = append(s.PlayerOrder, p.ID())
		s.UpdatedAt = time.Now()
	})
	g.mu.Unlock()

	if g.eventBus != nil {
		events.Publish(g.eventBus, events.PlayerJoinedEvent{
			GameID:   g.id,
			PlayerID: p.ID(),
		})
	}

	return nil
}

func (g *Game) RemovePlayer(ctx context.Context, playerID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	g.mu.Lock()
	if _, exists := g.players[playerID]; !exists {
		g.mu.Unlock()
		return fmt.Errorf("player %s not found in game %s", playerID, g.id)
	}

	delete(g.players, playerID)
	g.update(func(s *datastore.GameState) {
		for i, id := range s.PlayerOrder {
			if id == playerID {
				s.PlayerOrder = append(s.PlayerOrder[:i], s.PlayerOrder[i+1:]...)
				break
			}
		}
		s.UpdatedAt = time.Now()
	})
	g.mu.Unlock()

	return nil
}

func (g *Game) AddSpectator(ctx context.Context, s *Spectator) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var addErr error
	g.update(func(state *datastore.GameState) {
		if len(state.Spectators) >= shared.MaxSpectators {
			addErr = fmt.Errorf("game %s already has the maximum number of spectators (%d)", g.id, shared.MaxSpectators)
			return
		}
		if _, exists := state.Spectators[s.ID()]; exists {
			addErr = fmt.Errorf("spectator %s already exists in game %s", s.ID(), g.id)
			return
		}
		state.Spectators[s.ID()] = &shared.SpectatorState{
			ID:    s.ID(),
			Name:  s.Name(),
			Color: s.Color(),
		}
		state.UpdatedAt = time.Now()
	})
	return addErr
}

func (g *Game) RemoveSpectator(ctx context.Context, spectatorID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var removeErr error
	g.update(func(state *datastore.GameState) {
		if _, exists := state.Spectators[spectatorID]; !exists {
			removeErr = fmt.Errorf("spectator %s not found in game %s", spectatorID, g.id)
			return
		}
		delete(state.Spectators, spectatorID)
		state.UpdatedAt = time.Now()
	})
	return removeErr
}

func (g *Game) GetSpectator(spectatorID string) (*Spectator, error) {
	var result *Spectator
	var getErr error
	g.read(func(state *datastore.GameState) {
		ss, exists := state.Spectators[spectatorID]
		if !exists {
			getErr = fmt.Errorf("spectator %s not found in game %s", spectatorID, g.id)
			return
		}
		result = NewSpectator(ss.ID, ss.Name, ss.Color)
	})
	return result, getErr
}

func (g *Game) GetAllSpectators() []*Spectator {
	var spectators []*Spectator
	g.read(func(state *datastore.GameState) {
		spectators = make([]*Spectator, 0, len(state.Spectators))
		for _, ss := range state.Spectators {
			spectators = append(spectators, NewSpectator(ss.ID, ss.Name, ss.Color))
		}
	})
	return spectators
}

func (g *Game) SpectatorCount() int {
	var count int
	g.read(func(state *datastore.GameState) { count = len(state.Spectators) })
	return count
}

func (g *Game) NextSpectatorColor() string {
	var color string
	g.read(func(state *datastore.GameState) {
		idx := len(state.Spectators) % len(shared.SpectatorColors)
		color = shared.SpectatorColors[idx]
	})
	return color
}

// IsPlayerColorAvailable returns true if the given color is in the palette and
// not taken by another player (excluding the specified player).
func (g *Game) IsPlayerColorAvailable(color string, excludePlayerID string) bool {
	if !slices.Contains(shared.PlayerColors, color) {
		return false
	}

	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, p := range g.players {
		if p.ID() != excludePlayerID && p.Color() == color {
			return false
		}
	}
	return true
}

func (g *Game) AddChatMessage(ctx context.Context, msg shared.ChatMessage) {
	if ctx.Err() != nil {
		return
	}

	g.update(func(s *datastore.GameState) {
		s.ChatMessages = append(s.ChatMessages, msg)
		if len(s.ChatMessages) > shared.MaxChatMessages {
			s.ChatMessages = s.ChatMessages[len(s.ChatMessages)-shared.MaxChatMessages:]
		}
		s.UpdatedAt = time.Now()
	})
}

func (g *Game) GetChatMessages() []shared.ChatMessage {
	var msgs []shared.ChatMessage
	g.read(func(s *datastore.GameState) {
		msgs = make([]shared.ChatMessage, len(s.ChatMessages))
		copy(msgs, s.ChatMessages)
	})
	return msgs
}

func (g *Game) UpdateStatus(ctx context.Context, newStatus shared.GameStatus) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var oldStatus shared.GameStatus
	g.update(func(s *datastore.GameState) {
		oldStatus = s.Status
		s.Status = newStatus
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil && oldStatus != newStatus {
		events.Publish(g.eventBus, events.GameStatusChangedEvent{
			GameID:    g.id,
			OldStatus: string(oldStatus),
			NewStatus: string(newStatus),
		})
	}

	return nil
}

func (g *Game) SetFinalScores(ctx context.Context, scores []shared.FinalScore, winnerID string, isTie bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	g.update(func(s *datastore.GameState) {
		s.FinalScores = make([]shared.FinalScore, len(scores))
		copy(s.FinalScores, scores)
		s.WinnerID = winnerID
		s.IsTie = isTie
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil {
		events.Publish(g.eventBus, events.GameStateChangedEvent{
			GameID:    g.id,
			Timestamp: time.Now(),
		})
	}

	return nil
}

func (g *Game) UpdatePhase(ctx context.Context, newPhase shared.GamePhase) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var oldPhase shared.GamePhase
	g.update(func(s *datastore.GameState) {
		oldPhase = s.CurrentPhase
		s.CurrentPhase = newPhase
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil && oldPhase != newPhase {
		events.Publish(g.eventBus, events.GamePhaseChangedEvent{
			GameID:   g.id,
			OldPhase: string(oldPhase),
			NewPhase: string(newPhase),
		})
	}

	return nil
}

func (g *Game) AdvanceGeneration(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var oldGeneration, newGeneration int
	g.update(func(s *datastore.GameState) {
		oldGeneration = s.Generation
		s.Generation++
		newGeneration = s.Generation
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil {
		events.Publish(g.eventBus, events.GenerationAdvancedEvent{
			GameID:        g.id,
			OldGeneration: oldGeneration,
			NewGeneration: newGeneration,
		})
	}

	return nil
}

func (g *Game) SetGeneration(ctx context.Context, generation int) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var oldGeneration, newGeneration int
	g.update(func(s *datastore.GameState) {
		oldGeneration = s.Generation
		s.Generation = generation
		newGeneration = s.Generation
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil && oldGeneration != newGeneration {
		events.Publish(g.eventBus, events.GenerationAdvancedEvent{
			GameID:        g.id,
			OldGeneration: oldGeneration,
			NewGeneration: newGeneration,
		})
	}

	return nil
}

func (g *Game) SetCurrentTurn(ctx context.Context, playerID string, actionsRemaining int) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	g.mu.Lock()
	g.update(func(s *datastore.GameState) {
		s.CurrentTurnPlayerID = playerID
		s.CurrentTurnActions = actionsRemaining
		s.CurrentTurnTotalActions = actionsRemaining
		s.UpdatedAt = time.Now()
	})
	g.currentTurn = NewTurn(g.ds, g.id)
	g.mu.Unlock()
	if err := g.ExecuteFirstActionIfNeeded(ctx, playerID); err != nil {
		return err
	}

	if g.eventBus != nil {
		events.Publish(g.eventBus, events.GameStateChangedEvent{
			GameID:    g.id,
			Timestamp: time.Now(),
		})
	}

	return nil
}

// SetCurrentTurnActions replaces the current turn's remaining and total actions.
func (g *Game) SetCurrentTurnActions(ctx context.Context, actions int) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	g.update(func(s *datastore.GameState) {
		s.CurrentTurnActions = actions
		s.CurrentTurnTotalActions = actions
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil {
		events.Publish(g.eventBus, events.GameStateChangedEvent{
			GameID:    g.id,
			Timestamp: time.Now(),
		})
	}

	return nil
}

func (g *Game) SetTurnOrder(ctx context.Context, turnOrder []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	g.update(func(s *datastore.GameState) {
		s.TurnOrder = make([]string, len(turnOrder))
		copy(s.TurnOrder, turnOrder)
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil {
		events.Publish(g.eventBus, events.GameStateChangedEvent{
			GameID:    g.id,
			Timestamp: time.Now(),
		})
	}

	return nil
}

func (g *Game) SetHostPlayerID(ctx context.Context, playerID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	g.update(func(s *datastore.GameState) {
		s.HostPlayerID = playerID
		s.UpdatedAt = time.Now()
	})

	return nil
}

func (g *Game) NextPlayer() *string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	turnOrder := g.TurnOrder()
	if g.currentTurn == nil || len(turnOrder) == 0 {
		return nil
	}

	currentPlayerID := g.currentTurn.PlayerID()

	currentIndex := -1
	for i, playerID := range turnOrder {
		if playerID == currentPlayerID {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		return &turnOrder[0]
	}

	nextIndex := (currentIndex + 1) % len(turnOrder)
	return &turnOrder[nextIndex]
}

// HasAnyPendingSelection returns true if the player has any pending selection
// (tile placement, card storage, steal target, etc.) that blocks other actions.
func (g *Game) HasAnyPendingSelection(playerID string) bool {
	if g.GetPendingTileSelection(playerID) != nil {
		return true
	}
	p, err := g.GetPlayer(playerID)
	if err != nil {
		return false
	}
	return p.Selection().HasPendingSelection()
}

func (g *Game) GetPendingTileSelection(playerID string) *shared.PendingTileSelection {
	var result *shared.PendingTileSelection
	g.read(func(s *datastore.GameState) {
		selection, exists := s.PendingTileSelections[playerID]
		if !exists || selection == nil {
			return
		}
		selectionCopy := *selection
		selectionCopy.TileRestrictions = selection.TileRestrictions.Clone()
		result = &selectionCopy
	})
	return result
}

func (g *Game) SetPendingTileSelection(ctx context.Context, playerID string, selection *shared.PendingTileSelection) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	g.update(func(s *datastore.GameState) {
		if selection == nil {
			delete(s.PendingTileSelections, playerID)
		} else {
			selectionCopy := *selection
			selectionCopy.TileRestrictions = selection.TileRestrictions.Clone()
			s.PendingTileSelections[playerID] = &selectionCopy
		}
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil {
		events.Publish(g.eventBus, events.GameStateChangedEvent{
			GameID:    g.id,
			Timestamp: time.Now(),
		})
	}

	return nil
}

func (g *Game) GetPendingTileSelectionQueue(playerID string) *shared.PendingTileSelectionQueue {
	var result *shared.PendingTileSelectionQueue
	g.read(func(s *datastore.GameState) {
		queue, exists := s.PendingTileSelectionQueues[playerID]
		if !exists || queue == nil {
			return
		}
		queueCopy := *queue
		result = &queueCopy
	})
	return result
}

func (g *Game) SetPendingTileSelectionQueue(ctx context.Context, playerID string, queue *shared.PendingTileSelectionQueue) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	g.update(func(s *datastore.GameState) {
		if queue == nil {
			delete(s.PendingTileSelectionQueues, playerID)
		} else {
			queueCopy := *queue
			s.PendingTileSelectionQueues[playerID] = &queueCopy
		}
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil {
		events.Publish(g.eventBus, events.GameStateChangedEvent{
			GameID:    g.id,
			Timestamp: time.Now(),
		})
	}

	if queue != nil && len(queue.Items) > 0 {
		if err := g.ProcessNextTile(ctx, playerID); err != nil {
			return fmt.Errorf("failed to auto-process first queued tile: %w", err)
		}
	}

	return nil
}

func (g *Game) SetTileQueueOnComplete(_ context.Context, playerID string, callback *shared.TileCompletionCallback) {
	g.update(func(s *datastore.GameState) {
		if queue, exists := s.PendingTileSelectionQueues[playerID]; exists && queue != nil {
			queue.OnComplete = callback
		}
		if sel, exists := s.PendingTileSelections[playerID]; exists && sel != nil {
			sel.OnComplete = callback
		}
	})
}

func (g *Game) AppendToPendingTileSelectionQueue(ctx context.Context, playerID string, tileTypes []string, source string, sourceCardID string, tileRestrictions *shared.TileRestrictions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if len(tileTypes) == 0 {
		return nil
	}

	wasEmpty := false
	g.update(func(s *datastore.GameState) {
		existingQueue, exists := s.PendingTileSelectionQueues[playerID]
		var items []string
		var queueSource string
		var queueSourceCardID string
		var queueTileRestrictions *shared.TileRestrictions

		if exists && existingQueue != nil {
			items = existingQueue.Items
			queueSource = existingQueue.Source
			queueSourceCardID = existingQueue.SourceCardID
			queueTileRestrictions = existingQueue.TileRestrictions
		} else {
			items = []string{}
			queueSource = source
			queueSourceCardID = sourceCardID
			queueTileRestrictions = tileRestrictions
			wasEmpty = true
		}

		if exists && existingQueue != nil && len(existingQueue.Items) == 0 {
			wasEmpty = true
		}

		items = append(items, tileTypes...)

		s.PendingTileSelectionQueues[playerID] = &shared.PendingTileSelectionQueue{
			Items:            items,
			TileRestrictions: queueTileRestrictions,
			Source:           queueSource,
			SourceCardID:     queueSourceCardID,
		}
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil {
		events.Publish(g.eventBus, events.GameStateChangedEvent{
			GameID:    g.id,
			Timestamp: time.Now(),
		})
	}

	if wasEmpty {
		if err := g.ProcessNextTile(ctx, playerID); err != nil {
			return fmt.Errorf("failed to auto-process first queued tile: %w", err)
		}
	}

	return nil
}

func (g *Game) GetForcedFirstAction(playerID string) *shared.ForcedFirstAction {
	var result *shared.ForcedFirstAction
	g.read(func(s *datastore.GameState) {
		action, exists := s.ForcedFirstActions[playerID]
		if !exists || action == nil {
			return
		}
		actionCopy := *action
		actionCopy.BehaviorIndices = append([]int{}, action.BehaviorIndices...)
		result = &actionCopy
	})
	return result
}

func (g *Game) SetForcedFirstAction(ctx context.Context, playerID string, action *shared.ForcedFirstAction) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	g.update(func(s *datastore.GameState) {
		if action == nil {
			delete(s.ForcedFirstActions, playerID)
		} else {
			actionCopy := *action
			actionCopy.BehaviorIndices = append([]int{}, action.BehaviorIndices...)
			s.ForcedFirstActions[playerID] = &actionCopy
		}
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil {
		events.Publish(g.eventBus, events.GameStateChangedEvent{
			GameID:    g.id,
			Timestamp: time.Now(),
		})
	}

	return nil
}

func (g *Game) GetProductionPhase(playerID string) *shared.ProductionPhase {
	var result *shared.ProductionPhase
	g.read(func(s *datastore.GameState) {
		phase, exists := s.ProductionPhases[playerID]
		if !exists || phase == nil {
			return
		}
		phaseCopy := *phase
		result = &phaseCopy
	})
	return result
}

func (g *Game) SetProductionPhase(ctx context.Context, playerID string, phase *shared.ProductionPhase) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	g.update(func(s *datastore.GameState) {
		if phase == nil {
			delete(s.ProductionPhases, playerID)
		} else {
			phaseCopy := *phase
			s.ProductionPhases[playerID] = &phaseCopy
		}
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil {
		events.Publish(g.eventBus, events.GameStateChangedEvent{
			GameID:    g.id,
			Timestamp: time.Now(),
		})
	}

	return nil
}

func (g *Game) GetSelectCorporationPhase(playerID string) *shared.SelectCorporationPhase {
	var result *shared.SelectCorporationPhase
	g.read(func(s *datastore.GameState) {
		phase, exists := s.SelectCorporationPhases[playerID]
		if !exists || phase == nil {
			return
		}
		phaseCopy := *phase
		result = &phaseCopy
	})
	return result
}

func (g *Game) SetSelectCorporationPhase(ctx context.Context, playerID string, phase *shared.SelectCorporationPhase) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	g.update(func(s *datastore.GameState) {
		if phase == nil {
			delete(s.SelectCorporationPhases, playerID)
		} else {
			phaseCopy := *phase
			s.SelectCorporationPhases[playerID] = &phaseCopy
		}
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil {
		events.Publish(g.eventBus, events.GameStateChangedEvent{
			GameID:    g.id,
			Timestamp: time.Now(),
		})
	}

	return nil
}

func (g *Game) GetSelectStartingCardsPhase(playerID string) *shared.SelectStartingCardsPhase {
	var result *shared.SelectStartingCardsPhase
	g.read(func(s *datastore.GameState) {
		phase, exists := s.SelectStartingCardsPhases[playerID]
		if !exists || phase == nil {
			return
		}
		phaseCopy := *phase
		result = &phaseCopy
	})
	return result
}

func (g *Game) SetSelectStartingCardsPhase(ctx context.Context, playerID string, phase *shared.SelectStartingCardsPhase) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	g.update(func(s *datastore.GameState) {
		if phase == nil {
			delete(s.SelectStartingCardsPhases, playerID)
		} else {
			phaseCopy := *phase
			s.SelectStartingCardsPhases[playerID] = &phaseCopy
		}
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil {
		events.Publish(g.eventBus, events.GameStateChangedEvent{
			GameID:    g.id,
			Timestamp: time.Now(),
		})
	}

	return nil
}

func (g *Game) GetSelectPreludeCardsPhase(playerID string) *shared.SelectPreludeCardsPhase {
	var result *shared.SelectPreludeCardsPhase
	g.read(func(s *datastore.GameState) {
		phase, exists := s.SelectPreludeCardsPhases[playerID]
		if !exists || phase == nil {
			return
		}
		phaseCopy := *phase
		result = &phaseCopy
	})
	return result
}

func (g *Game) SetSelectPreludeCardsPhase(ctx context.Context, playerID string, phase *shared.SelectPreludeCardsPhase) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	g.update(func(s *datastore.GameState) {
		if phase == nil {
			delete(s.SelectPreludeCardsPhases, playerID)
		} else {
			phaseCopy := *phase
			s.SelectPreludeCardsPhases[playerID] = &phaseCopy
		}
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil {
		events.Publish(g.eventBus, events.GameStateChangedEvent{
			GameID:    g.id,
			Timestamp: time.Now(),
		})
	}

	return nil
}

func (g *Game) ProcessNextTile(ctx context.Context, playerID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var nextTileType string
	var source string
	var sourceCardID string
	var onComplete *shared.TileCompletionCallback
	var tileRestrictions *shared.TileRestrictions
	var found bool

	g.update(func(s *datastore.GameState) {
		queue, exists := s.PendingTileSelectionQueues[playerID]
		if !exists || queue == nil || len(queue.Items) == 0 {
			return
		}
		found = true

		nextTileType = queue.Items[0]
		remainingItems := queue.Items[1:]
		source = queue.Source
		sourceCardID = queue.SourceCardID
		onComplete = queue.OnComplete
		tileRestrictions = queue.TileRestrictions

		if len(remainingItems) > 0 {
			s.PendingTileSelectionQueues[playerID] = &shared.PendingTileSelectionQueue{
				Items:            remainingItems,
				TileRestrictions: tileRestrictions,
				Source:           source,
				SourceCardID:     sourceCardID,
				OnComplete:       onComplete,
			}
		} else {
			delete(s.PendingTileSelectionQueues, playerID)
		}
	})

	if !found {
		return nil
	}

	if nextTileType == "ocean" && g.GlobalParameters().Oceans() >= g.GlobalParameters().GetMaxOceans() {
		return g.ProcessNextTile(ctx, playerID)
	}
	availableHexes := g.calculateAvailableHexesForTile(nextTileType, playerID, tileRestrictions)

	if len(availableHexes) == 0 {
		return g.ProcessNextTile(ctx, playerID)
	}

	err := g.SetPendingTileSelection(ctx, playerID, &shared.PendingTileSelection{
		TileType:         nextTileType,
		TileRestrictions: tileRestrictions,
		AvailableHexes:   availableHexes,
		Source:           source,
		SourceCardID:     sourceCardID,
		OnComplete:       onComplete,
	})

	return err
}

// calculateAvailableHexesForTile returns a list of valid hex positions for placing a tile
// tileRestrictions controls placement rules:
//   - BoardTags: restricts to tiles with matching tags (e.g., Noctis City)
//   - Adjacency: "none" means no adjacent occupied tiles allowed (Research Outpost)
//
// For cities: if BoardTags is set, only matching tiles are valid (ignoring adjacency);
// if Adjacency is "none", tiles must have no adjacent occupied tiles;
// otherwise, tagged tiles (reserved areas) are excluded and normal adjacency rules apply
func (g *Game) calculateAvailableHexesForTile(tileType string, playerID string, tileRestrictions *shared.TileRestrictions) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.calculateAvailableHexesForTileLocked(tileType, playerID, tileRestrictions)
}

// calculateAvailableHexesForTileLocked preserves restrictions during fallback without reacquiring the game lock.
func (g *Game) calculateAvailableHexesForTileLocked(tileType string, playerID string, tileRestrictions *shared.TileRestrictions) []string {

	if g.board == nil {
		return []string{}
	}

	tiles := g.board.Tiles()
	availableHexes := []string{}

	// Extract restrictions
	var boardTags []string
	var adjacency string
	if tileRestrictions != nil {
		boardTags = tileRestrictions.BoardTags
		adjacency = tileRestrictions.Adjacency
	}

	if tileRestrictions != nil && tileRestrictions.Area != "" && tileRestrictions.Area != "land" && tileRestrictions.Area != "ocean" {
		return availableHexes
	}
	matchesArea := func(tile board.Tile, defaultArea string) bool {
		area := defaultArea
		if tileRestrictions != nil && tileRestrictions.Area != "" {
			area = tileRestrictions.Area
		}
		if area == "ocean" {
			return tile.Type == shared.ResourceOceanSpace
		}
		return tile.Type == shared.ResourceLandTile
	}

	// Helper to check if tile has any of the required board tags
	tileHasRequiredTag := func(tile board.Tile, requiredTags []string) bool {
		return slices.ContainsFunc(requiredTags, func(reqTag string) bool {
			return slices.Contains(tile.Tags, reqTag)
		})
	}

	// Helper to check if tile has any tags (is a reserved area)
	tileHasAnyTag := func(tile board.Tile) bool {
		return len(tile.Tags) > 0
	}

	// Helper to check if a tile has any adjacent occupied tiles
	hasAnyAdjacentOccupied := func(tile board.Tile) bool {
		for _, neighborPos := range tile.Coordinates.GetNeighbors() {
			for _, neighborTile := range tiles {
				if neighborTile.Coordinates.Equals(neighborPos) && neighborTile.OccupiedBy != nil {
					return true
				}
			}
		}
		return false
	}

	// Helper to check if tile is reserved by another player
	isReservedByOther := func(tile board.Tile) bool {
		return tile.ReservedBy != nil && *tile.ReservedBy != playerID
	}

	// Helper to count adjacent occupied tiles of a specific type
	countAdjacentOfType := func(tile board.Tile, tileOccupantType string) int {
		count := 0
		var targetType shared.ResourceType
		switch tileOccupantType {
		case "city":
			targetType = shared.ResourceCityTile
		case "greenery":
			targetType = shared.ResourceGreeneryTile
		case "ocean":
			targetType = shared.ResourceOceanTile
		default:
			targetType = shared.ResourceType(tileOccupantType + "-tile")
		}
		for _, neighborPos := range tile.Coordinates.GetNeighbors() {
			for _, neighborTile := range tiles {
				if neighborTile.Coordinates.Equals(neighborPos) && neighborTile.OccupiedBy != nil && neighborTile.OccupiedBy.Type == targetType {
					count++
					break
				}
			}
		}
		return count
	}

	// Helper to check if tile has an adjacent tile of a specific type owned by the player
	hasAdjacentOwnedOfType := func(tile board.Tile, tileOccupantType string) bool {
		var targetType shared.ResourceType
		switch tileOccupantType {
		case "city":
			targetType = shared.ResourceCityTile
		case "greenery":
			targetType = shared.ResourceGreeneryTile
		case "ocean":
			targetType = shared.ResourceOceanTile
		default:
			targetType = shared.ResourceType(tileOccupantType + "-tile")
		}
		for _, neighborPos := range tile.Coordinates.GetNeighbors() {
			for _, neighborTile := range tiles {
				if neighborTile.Coordinates.Equals(neighborPos) &&
					neighborTile.OccupiedBy != nil &&
					neighborTile.OccupiedBy.Type == targetType &&
					neighborTile.OwnerID != nil &&
					*neighborTile.OwnerID == playerID {
					return true
				}
			}
		}
		return false
	}

	// Helper to check if tile has any adjacent tile owned by the player (regardless of type)
	hasAnyAdjacentOwned := func(tile board.Tile) bool {
		for _, neighborPos := range tile.Coordinates.GetNeighbors() {
			for _, neighborTile := range tiles {
				if neighborTile.Coordinates.Equals(neighborPos) &&
					neighborTile.OccupiedBy != nil &&
					neighborTile.OwnerID != nil &&
					*neighborTile.OwnerID == playerID {
					return true
				}
			}
		}
		return false
	}

	// Helper to check if a tile has one of the specified bonus types
	hasBonusOfType := func(tile board.Tile, bonusTypes []string) bool {
		for _, bonus := range tile.Bonuses {
			if slices.Contains(bonusTypes, string(bonus.Type)) {
				return true
			}
		}
		return false
	}

	// Helper to check if a tile passes adjacentToType/minAdjacentOfType/adjacentToOwned restrictions
	passesAdjacentRestrictions := func(tile board.Tile) bool {
		if tileRestrictions == nil {
			return true
		}
		// Check adjacentToType + minAdjacentOfType
		if tileRestrictions.AdjacentToType != "" {
			minRequired := 1
			if tileRestrictions.MinAdjacentOfType != nil {
				minRequired = *tileRestrictions.MinAdjacentOfType
			}
			if countAdjacentOfType(tile, tileRestrictions.AdjacentToType) < minRequired {
				return false
			}
			// If both adjacentToType and adjacentToOwned are set, check owned of that type
			if tileRestrictions.AdjacentToOwned && !hasAdjacentOwnedOfType(tile, tileRestrictions.AdjacentToType) {
				return false
			}
		} else if tileRestrictions.AdjacentToOwned {
			// AdjacentToOwned without AdjacentToType: adjacent to any owned tile
			if !hasAnyAdjacentOwned(tile) {
				return false
			}
		}
		return true
	}

	for _, tile := range tiles {
		// Clear targets occupied or reserved tiles (inverse of normal placement)
		if tileType == "clear" {
			if tile.OccupiedBy != nil || tile.ReservedBy != nil {
				availableHexes = append(availableHexes, tile.Coordinates.String())
			}
			continue
		}

		// Tile replacement targets any occupied non-ocean tile
		if strings.HasPrefix(tileType, "tile-replacement:") {
			if tile.OccupiedBy != nil && tile.OccupiedBy.Type != shared.ResourceOceanTile {
				availableHexes = append(availableHexes, tile.Coordinates.String())
			}
			continue
		}

		// Tile destruction targets any occupied tile on the board
		if tileType == "tile-destruction" {
			if tile.OccupiedBy != nil {
				availableHexes = append(availableHexes, tile.Coordinates.String())
			}
			continue
		}

		// Skip tiles that are already occupied
		if tile.OccupiedBy != nil {
			continue
		}
		// Area exceptions do not waive ownership, reservation or adjacency restrictions.
		if isReservedByOther(tile) {
			continue
		}
		if !passesAdjacentRestrictions(tile) {
			continue
		}
		if adjacency == "none" && hasAnyAdjacentOccupied(tile) {
			continue
		}
		if tileRestrictions != nil && len(tileRestrictions.OnBonusType) > 0 && !hasBonusOfType(tile, tileRestrictions.OnBonusType) {
			continue
		}
		if len(boardTags) > 0 && !tileHasRequiredTag(tile, boardTags) {
			continue
		}
		if len(boardTags) == 0 && tileHasAnyTag(tile) && tileType != "volcano" {
			continue
		}

		switch tileType {
		case "land-claim":
			// Land claim can only be placed on unoccupied, unreserved land tiles
			if !matchesArea(tile, "land") {
				continue
			}
			// Exclude reserved areas (tagged tiles like Noctis City)
			if tileHasAnyTag(tile) {
				continue
			}
			// Exclude tiles already reserved by anyone
			if tile.ReservedBy != nil {
				continue
			}
			availableHexes = append(availableHexes, tile.Coordinates.String())

		case "city":
			if !matchesArea(tile, "land") {
				continue
			}

			// If boardTags specified, only allow tiles with matching tags (Noctis City case)
			if len(boardTags) > 0 {
				if tileHasRequiredTag(tile, boardTags) {
					availableHexes = append(availableHexes, tile.Coordinates.String())
					slog.Default().Debug("Tile available for city (board tag match)",
						slog.String("tile", tile.Coordinates.String()),
						slog.Any("board_tags", boardTags))
				}
				continue
			}

			// Skip tiles reserved by other players (current player can use their own reserved tiles)
			if isReservedByOther(tile) {
				continue
			}

			// Normal city placement: exclude reserved areas (tagged tiles)
			if tileHasAnyTag(tile) {
				slog.Default().Debug("Skipping reserved tile for normal city placement",
					slog.String("tile", tile.Coordinates.String()),
					slog.Any("tile_tags", tile.Tags))
				continue
			}

			// Handle "no adjacent tiles" restriction (Research Outpost)
			if adjacency == "none" {
				if !hasAnyAdjacentOccupied(tile) {
					availableHexes = append(availableHexes, tile.Coordinates.String())
					slog.Default().Debug("Tile available for city (no adjacent tiles)",
						slog.String("tile", tile.Coordinates.String()))
				}
				continue // Skip normal city adjacency rules
			}

			// Handle adjacentToType restriction (e.g., Urbanized Area: adjacent to 2+ cities)
			// This overrides the normal "no adjacent cities" rule
			if tileRestrictions != nil && tileRestrictions.AdjacentToType != "" {
				if passesAdjacentRestrictions(tile) {
					availableHexes = append(availableHexes, tile.Coordinates.String())
				}
				continue
			}

			// Check city adjacency rule (no adjacent cities)
			hasAdjacentCity := false
			neighbors := tile.Coordinates.GetNeighbors()

			slog.Default().Debug("Checking city placement",
				slog.String("tile", tile.Coordinates.String()),
				slog.Int("neighbor_count", len(neighbors)))

			for _, neighborPos := range neighbors {
				for _, neighborTile := range tiles {
					if neighborTile.Coordinates.Equals(neighborPos) {
						occupantType := ""
						if neighborTile.OccupiedBy != nil {
							occupantType = string(neighborTile.OccupiedBy.Type)
						}

						slog.Default().Debug("Checking neighbor",
							slog.String("neighbor_pos", neighborPos.String()),
							slog.String("neighbor_tile", neighborTile.Coordinates.String()),
							slog.Bool("occupied", neighborTile.OccupiedBy != nil),
							slog.String("occupant_type", occupantType))

						if neighborTile.OccupiedBy != nil && neighborTile.OccupiedBy.Type == shared.ResourceCityTile {
							hasAdjacentCity = true
							break
						}
					}
				}
				if hasAdjacentCity {
					break
				}
			}

			if !hasAdjacentCity {
				availableHexes = append(availableHexes, tile.Coordinates.String())
				slog.Default().Debug("Tile available for city",
					slog.String("tile", tile.Coordinates.String()))
			} else {
				slog.Default().Debug("Tile unavailable for city (adjacent city)",
					slog.String("tile", tile.Coordinates.String()))
			}

		case "greenery", "world-tree":
			// Check if restricted to ocean tiles (Mangrove card)
			if tileRestrictions != nil && tileRestrictions.Area == "ocean" {
				if matchesArea(tile, "ocean") {
					availableHexes = append(availableHexes, tile.Coordinates.String())
				}
				continue
			}

			// Skip tiles reserved by other players (current player can use their own reserved tiles)
			if isReservedByOther(tile) {
				continue
			}

			// Exclude reserved areas from normal greenery placement
			if len(boardTags) == 0 && tileHasAnyTag(tile) {
				continue
			}
			if matchesArea(tile, "land") {
				// Apply adjacency restrictions if set (e.g., Ecological Zone: adjacent to greenery)
				if !passesAdjacentRestrictions(tile) {
					continue
				}
				availableHexes = append(availableHexes, tile.Coordinates.String())
			}

		case "ocean":
			if matchesArea(tile, "ocean") {
				availableHexes = append(availableHexes, tile.Coordinates.String())
			}

		case "volcano":
			if !tileHasRequiredTag(tile, []string{board.BoardTagVolcanic}) {
				continue
			}
			if matchesArea(tile, "land") {
				availableHexes = append(availableHexes, tile.Coordinates.String())
			}

		case "mohole":
			if matchesArea(tile, "ocean") {
				availableHexes = append(availableHexes, tile.Coordinates.String())
			}

		default:
			// Handle ocean-space placement (e.g., Mohole Area)
			if tileRestrictions != nil && tileRestrictions.Area == "ocean" {
				if matchesArea(tile, "ocean") {
					availableHexes = append(availableHexes, tile.Coordinates.String())
				}
				continue
			}

			// Skip tiles reserved by other players (current player can use their own reserved tiles)
			if isReservedByOther(tile) {
				continue
			}

			// If boardTags specified, only allow tiles with matching tags
			if len(boardTags) > 0 {
				if tileHasRequiredTag(tile, boardTags) {
					availableHexes = append(availableHexes, tile.Coordinates.String())
				}
				continue
			}

			// Exclude reserved areas from normal placement
			if tileHasAnyTag(tile) {
				continue
			}

			// Must be on land
			if !matchesArea(tile, "land") {
				continue
			}

			// Check bonus type restriction (e.g., Mining Area/Mining Rights)
			if tileRestrictions != nil && len(tileRestrictions.OnBonusType) > 0 {
				if !hasBonusOfType(tile, tileRestrictions.OnBonusType) {
					continue
				}
			}

			// Handle "no adjacent tiles" restriction (e.g., Natural Preserve)
			if adjacency == "none" {
				if !hasAnyAdjacentOccupied(tile) {
					availableHexes = append(availableHexes, tile.Coordinates.String())
				}
				continue
			}

			// Apply adjacency restrictions if set
			if !passesAdjacentRestrictions(tile) {
				continue
			}

			availableHexes = append(availableHexes, tile.Coordinates.String())
		}
	}

	if len(availableHexes) == 0 && tileRestrictions != nil {
		fallback := *tileRestrictions
		canFallback := false
		if len(boardTags) > 0 {
			hasReservedSpace := false
			for _, tile := range tiles {
				if tileHasRequiredTag(tile, boardTags) {
					hasReservedSpace = true
					break
				}
			}
			if !hasReservedSpace {
				fallback.BoardTags = nil
				canFallback = true
			}
		}
		// Only greenery's standard ownership adjacency is a preference, not a requirement.
		if (tileType == "greenery" || tileType == "world-tree") && tileRestrictions.AdjacentToOwned && tileRestrictions.AdjacentToType == "" && len(tileRestrictions.OnBonusType) == 0 {
			fallback.AdjacentToOwned = false
			canFallback = true
		}
		if canFallback {
			return g.calculateAvailableHexesForTileLocked(tileType, playerID, &fallback)
		}
	}
	return availableHexes
}

// CountAvailableHexesForTile returns the number of valid hex positions for placing a tile
// This is used by state calculators to determine if tile-placing actions are available
func (g *Game) CountAvailableHexesForTile(tileType string, playerID string, tileRestrictions *shared.TileRestrictions) int {
	return len(g.calculateAvailableHexesForTile(tileType, playerID, tileRestrictions))
}

// CalculateAvailableHexesForTile returns the list of valid hex coordinate strings for placing a tile
func (g *Game) CalculateAvailableHexesForTile(tileType string, playerID string, tileRestrictions *shared.TileRestrictions) []string {
	return g.calculateAvailableHexesForTile(tileType, playerID, tileRestrictions)
}

func (g *Game) AddTriggeredEffect(effect shared.TriggeredEffect) {
	g.update(func(s *datastore.GameState) {
		s.TriggeredEffects = append(s.TriggeredEffects, effect)
	})
}

// AddOrMergeTriggeredEffect adds a triggered effect, merging calculated outputs into the last
// effect if it has the same CardName, PlayerID, and SourceType.
func (g *Game) AddOrMergeTriggeredEffect(effect shared.TriggeredEffect) {
	g.update(func(s *datastore.GameState) {
		if len(s.TriggeredEffects) > 0 {
			last := &s.TriggeredEffects[len(s.TriggeredEffects)-1]
			if last.CardName == effect.CardName && last.PlayerID == effect.PlayerID && last.SourceType == effect.SourceType {
				last.CalculatedOutputs = append(last.CalculatedOutputs, effect.CalculatedOutputs...)
				return
			}
		}
		s.TriggeredEffects = append(s.TriggeredEffects, effect)
	})
}

func (g *Game) AppendToLastTriggeredEffect(playerID string, outputs []shared.CalculatedOutput) {
	g.update(func(s *datastore.GameState) {
		for i := len(s.TriggeredEffects) - 1; i >= 0; i-- {
			if s.TriggeredEffects[i].PlayerID == playerID {
				s.TriggeredEffects[i].CalculatedOutputs = append(s.TriggeredEffects[i].CalculatedOutputs, outputs...)
				return
			}
		}
	})
}

func (g *Game) GetTriggeredEffects() []shared.TriggeredEffect {
	var result []shared.TriggeredEffect
	g.read(func(s *datastore.GameState) {
		result = make([]shared.TriggeredEffect, len(s.TriggeredEffects))
		copy(result, s.TriggeredEffects)
	})
	return result
}

func (g *Game) ClearTriggeredEffects() {
	g.update(func(s *datastore.GameState) {
		s.TriggeredEffects = nil
	})
}

func (g *Game) GetDeferredStartingChoices(playerID string) *shared.DeferredStartingChoices {
	var result *shared.DeferredStartingChoices
	g.read(func(s *datastore.GameState) {
		choices, exists := s.DeferredStartingChoices[playerID]
		if !exists || choices == nil {
			return
		}
		choicesCopy := *choices
		choicesCopy.Payment.Allocations = append([]shared.PaymentAllocation(nil), choices.Payment.Allocations...)
		result = &choicesCopy
	})
	return result
}

func (g *Game) SetDeferredStartingChoices(ctx context.Context, playerID string, choices *shared.DeferredStartingChoices) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	g.update(func(s *datastore.GameState) {
		if choices == nil {
			delete(s.DeferredStartingChoices, playerID)
		} else {
			choicesCopy := *choices
			choicesCopy.Payment.Allocations = append([]shared.PaymentAllocation(nil), choices.Payment.Allocations...)
			s.DeferredStartingChoices[playerID] = &choicesCopy
		}
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil {
		events.Publish(g.eventBus, events.GameStateChangedEvent{
			GameID:    g.id,
			Timestamp: time.Now(),
		})
	}

	return nil
}

func (g *Game) MarkCorpApplied(playerID string) {
	g.update(func(s *datastore.GameState) {
		if choices, ok := s.DeferredStartingChoices[playerID]; ok && choices != nil {
			choices.CorpApplied = true
		}
	})
}

func (g *Game) MarkPreludeApplied(playerID string) {
	g.update(func(s *datastore.GameState) {
		if choices, ok := s.DeferredStartingChoices[playerID]; ok && choices != nil {
			choices.PreludesAppliedCount++
		}
	})
}

func (g *Game) InitPhasePlayerIndex() int {
	var v int
	g.read(func(s *datastore.GameState) { v = s.InitPhasePlayerIndex })
	return v
}

func (g *Game) SetInitPhasePlayerIndex(ctx context.Context, index int) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	g.update(func(s *datastore.GameState) {
		s.InitPhasePlayerIndex = index
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil {
		events.Publish(g.eventBus, events.GameStateChangedEvent{
			GameID:    g.id,
			Timestamp: time.Now(),
		})
	}

	return nil
}

func (g *Game) InitPhaseWaitingForConfirm() bool {
	var v bool
	g.read(func(s *datastore.GameState) { v = s.InitPhaseWaitingForConfirm })
	return v
}

func (g *Game) InitPhaseConfirmVersion() int {
	var v int
	g.read(func(s *datastore.GameState) { v = s.InitPhaseConfirmVersion })
	return v
}

// InitPhaseRoster reports whether the init phase is showing the final roster of all players
// before the action phase begins.
func (g *Game) InitPhaseRoster() bool {
	var v bool
	g.read(func(s *datastore.GameState) { v = s.InitPhaseRoster })
	return v
}

func (g *Game) SetInitPhaseRoster(ctx context.Context, roster bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	g.update(func(s *datastore.GameState) {
		s.InitPhaseRoster = roster
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil {
		events.Publish(g.eventBus, events.GameStateChangedEvent{
			GameID:    g.id,
			Timestamp: time.Now(),
		})
	}

	return nil
}

func (g *Game) SetInitPhaseWaitingForConfirm(ctx context.Context, waiting bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	g.update(func(s *datastore.GameState) {
		s.InitPhaseWaitingForConfirm = waiting
		if waiting {
			s.InitPhaseConfirmVersion++
		}
		s.UpdatedAt = time.Now()
	})

	if g.eventBus != nil {
		events.Publish(g.eventBus, events.GameStateChangedEvent{
			GameID:    g.id,
			Timestamp: time.Now(),
		})
	}

	return nil
}

func (g *Game) HasColonies() bool {
	return g.Settings().HasColonies()
}

// CountAllColonies delegates to the Colonies component.
// This allows Game to satisfy the BoardContext interface for VP calculation.
func (g *Game) CountAllColonies() int {
	return g.colonies.CountAllColonies()
}

func (g *Game) InitializeTradeFleets(playerIDs []string) {
	g.update(func(s *datastore.GameState) {
		s.TradeFleets = make(map[string]colony.TradeFleet, len(playerIDs))
		for _, id := range playerIDs {
			s.TradeFleets[id] = colony.TradeFleet{Capacity: 1}
		}
		s.UpdatedAt = time.Now()
	})
}

// HasProjectFunding returns true if the project funding expansion is enabled
func (g *Game) HasProjectFunding() bool {
	return g.Settings().HasProjectFunding()
}

// ProjectFundingStates returns the project funding states
func (g *Game) ProjectFundingStates() []*projectfunding.ProjectState {
	var result []*projectfunding.ProjectState
	g.read(func(s *datastore.GameState) { result = s.ProjectFundingStates })
	return result
}

// SetProjectFundingStates sets the project funding states
func (g *Game) SetProjectFundingStates(states []*projectfunding.ProjectState) {
	g.update(func(s *datastore.GameState) {
		s.ProjectFundingStates = states
		s.UpdatedAt = time.Now()
	})
}

// IsNextGenTurnOrderFrozen returns true if turn order rotation is skipped next generation.
func (g *Game) IsNextGenTurnOrderFrozen() bool {
	var v bool
	g.read(func(s *datastore.GameState) { v = s.NextGenTurnOrderFrozen })
	return v
}

// SetNextGenTurnOrderFrozen sets whether turn order rotation is skipped next generation.
func (g *Game) SetNextGenTurnOrderFrozen(frozen bool) {
	g.update(func(s *datastore.GameState) {
		s.NextGenTurnOrderFrozen = frozen
		s.UpdatedAt = time.Now()
	})
}

// GetProjectFundingState returns the state for a specific project
func (g *Game) GetProjectFundingState(projectID string) *projectfunding.ProjectState {
	var result *projectfunding.ProjectState
	g.read(func(s *datastore.GameState) {
		for _, state := range s.ProjectFundingStates {
			if state.DefinitionID == projectID {
				result = state
				return
			}
		}
	})
	return result
}

// SetFirstActionExecutor installs the owner's behavior executor used at turn entry.
func (g *Game) SetFirstActionExecutor(playerID string, execute func(context.Context, *Game, string) error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.firstActionExecutors == nil {
		g.firstActionExecutors = make(map[string]func(context.Context, *Game, string) error)
	}
	g.firstActionExecutors[playerID] = execute
}

// ExecuteFirstActionIfNeeded executes a queued first action only on its owner's action-phase turn.
func (g *Game) ExecuteFirstActionIfNeeded(ctx context.Context, playerID string) error {
	if !g.executingFirstAction.CompareAndSwap(false, true) {
		return nil
	}
	defer g.executingFirstAction.Store(false)
	turn := g.CurrentTurn()
	pending := g.GetForcedFirstAction(playerID)
	if g.CurrentPhase() != shared.GamePhaseAction || turn == nil || turn.PlayerID() != playerID || turn.ActionsRemaining() == 0 || pending == nil || pending.State != "queued" {
		return nil
	}
	g.mu.RLock()
	execute := g.firstActionExecutors[playerID]
	g.mu.RUnlock()
	if execute == nil {
		return fmt.Errorf("missing first action executor")
	}
	pending.State = "resolving"
	if err := g.SetForcedFirstAction(ctx, playerID, pending); err != nil {
		return err
	}
	if err := execute(ctx, g, playerID); err != nil {
		return err
	}
	g.executingFirstAction.Store(false)
	g.CompleteFirstActionIfReady(playerID)
	return nil
}

// CompleteFirstActionIfReady finishes a first action after all of its selections resolve.
func (g *Game) CompleteFirstActionIfReady(playerID string) bool {
	if g.executingFirstAction.Load() || g.HasAnyPendingSelection(playerID) || g.GetPendingTileSelectionQueue(playerID) != nil {
		return false
	}
	completed := false
	g.update(func(s *datastore.GameState) {
		pending := s.ForcedFirstActions[playerID]
		if s.CurrentPhase != shared.GamePhaseAction || pending == nil || pending.State != "resolving" || s.CurrentTurnPlayerID != playerID {
			return
		}
		delete(s.ForcedFirstActions, playerID)
		s.GlobalActionCounter++
		if s.CurrentTurnActions > 0 {
			s.CurrentTurnActions--
		}
		s.UpdatedAt = time.Now()
		completed = true
	})
	if completed && g.eventBus != nil {
		events.Publish(g.eventBus, events.GameStateChangedEvent{GameID: g.id, Timestamp: time.Now()})
	}
	return completed
}
