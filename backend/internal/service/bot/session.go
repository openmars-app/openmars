package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	rand "math/rand/v2"
	"strings"
	"sync"
	"time"

	"openmars/internal/delivery/dto"
	"openmars/internal/events"
	"openmars/internal/game"
	"openmars/internal/game/cards"
	playerPkg "openmars/internal/game/player"
	"openmars/internal/game/shared"
)

const maxChatsPerTurn = 2

// botSession plays for one bot player. One goroutine owns the turn loop; the planner and
// reactions run beside it, and tool calls arrive on the tool server's goroutines.
type botSession struct {
	bc       *BotController
	gameID   string
	playerID string
	name     string
	persona  Persona
	log      *slog.Logger

	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	wakeCh  chan struct{}
	retryCh chan struct{}
	endedCh chan struct{}
	happen  chan Happening

	bus  *events.EventBusImpl
	subs []events.SubscriptionID

	mu          sync.Mutex
	plan        Plan
	lastPlanRun time.Time
	// pendingTrigger is a replan reason that arrived while the planner was busy.
	pendingTrigger string
	lastThought    string
	thoughtAt      time.Time
	lastActionAt   time.Time
	actionCount    int
	chatsLeft      int
	failed         bool
	failures       int
	planCancel     context.CancelFunc
	reacting       bool
	gate           ReactionGate
	trace          *trace
	grudges        grudgeLedger
	// seenLogSeq is the last game log entry already checked for losses; touched only by the run loop.
	seenLogSeq int64
	// leftAfterGame marks a session that ended because the game ended; it announces leaving.
	leftAfterGame bool
}

func newBotSession(bc *BotController, g *game.Game, bot *playerPkg.Player) *botSession {
	ctx, cancel := context.WithCancel(context.Background())
	return &botSession{
		bc:       bc,
		gameID:   g.ID(),
		playerID: bot.ID(),
		name:     bot.Name(),
		persona:  bc.personas.Get(bot.BotPersona()),
		log:      bc.logger.With(slog.String("game_id", g.ID()), slog.String("player_id", bot.ID()), slog.String("bot_name", bot.Name())),
		ctx:      ctx,
		cancel:   cancel,
		done:     make(chan struct{}),
		wakeCh:   make(chan struct{}, 1),
		retryCh:  make(chan struct{}, 1),
		endedCh:  make(chan struct{}, 1),
		happen:   make(chan Happening, 64),
		bus:      g.EventBus(),
		trace:    newTrace(bot.ID()),
		grudges:  grudgeLedger{},
		gate:     ReactionGate{Cooldown: bc.cfg.ReactionCooldown, BigChance: bc.cfg.BigEventChance},
	}
}

func (s *botSession) start() {
	s.subscribe()
	go s.run()
	s.wake()
}

func (s *botSession) stop() {
	s.cancel()
	<-s.done
}

func (s *botSession) wake() {
	select {
	case s.wakeCh <- struct{}{}:
	default:
	}
}

func (s *botSession) requestRetry() {
	select {
	case s.retryCh <- struct{}{}:
	default:
	}
}

// observe queues a happening without blocking; event handlers run inside game actions.
func (s *botSession) observe(h Happening) {
	if h.At.IsZero() {
		h.At = time.Now()
	}
	select {
	case s.happen <- h:
	default:
	}
}

func (s *botSession) subscribe() {
	pid := s.playerID
	s.subs = append(s.subs,
		events.Subscribe(s.bus, func(e events.TilePlacedEvent) {
			s.observe(Happening{Kind: HappeningTile, ActorID: e.PlayerID, Target: fmt.Sprintf("%d,%d,%d", e.Q, e.R, e.S), Detail: e.TileType})
		}),
		events.Subscribe(s.bus, func(e events.MilestoneClaimedEvent) {
			s.observe(Happening{Kind: HappeningMilestone, ActorID: e.PlayerID, Target: e.MilestoneType})
		}),
		events.Subscribe(s.bus, func(e events.AwardFundedEvent) {
			s.observe(Happening{Kind: HappeningAward, ActorID: e.PlayerID, Target: e.AwardType})
		}),
		events.Subscribe(s.bus, func(e events.ColonyBuiltEvent) {
			s.observe(Happening{Kind: HappeningColony, ActorID: e.PlayerID, Target: e.ColonyID})
		}),
		events.Subscribe(s.bus, func(e events.CardPlayedEvent) {
			if e.PlayerID != pid {
				s.observe(Happening{Kind: HappeningCard, ActorID: e.PlayerID, Target: e.CardName, Detail: e.CardID})
			}
		}),
		events.Subscribe(s.bus, func(e events.ResourcesChangedEvent) {
			if e.PlayerID != pid {
				return
			}
			for _, amount := range e.Changes {
				if amount < 0 {
					s.observe(Happening{Kind: HappeningResourceLoss})
					return
				}
			}
		}),
		events.Subscribe(s.bus, func(events.GameEndedEvent) {
			select {
			case s.endedCh <- struct{}{}:
			default:
			}
		}),
	)
}

func (s *botSession) run() {
	defer s.finish()

	var batch []Happening
	var batchC <-chan time.Time
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.endedCh:
			s.recap()
			s.leftAfterGame = true
			return
		case <-s.retryCh:
			s.clearFailure()
			s.takeTurnIfMine()
		case <-s.wakeCh:
			s.takeTurnIfMine()
		case h := <-s.happen:
			batch = append(batch, h)
			if batchC == nil {
				batchC = time.After(s.bc.cfg.ReactionBatch)
			}
		case <-batchC:
			s.considerReaction(batch)
			batch, batchC = nil, nil
		}
	}
}

func (s *botSession) finish() {
	for _, id := range s.subs {
		s.bus.Unsubscribe(id)
	}
	s.cancelPlanner()
	s.cancel()
	s.bc.forget(s)
	if s.leftAfterGame {
		s.bc.announceLeft(s.gameID, s.playerID)
	}
	close(s.done)
	s.log.Debug("Bot session stopped")
}

// onGame runs fn on the executor, where game state may be touched, and reports whether it ran.
func (s *botSession) onGame(fn func()) bool {
	return s.bc.onGame(s.ctx, fn) == nil
}

func (s *botSession) snapshot(ctx context.Context) (*Snapshot, error) {
	return s.bc.snapshot(ctx, s.gameID, s.playerID)
}

func (s *botSession) takeTurnIfMine() {
	mine := false
	s.onGame(func() {
		snap, err := s.snapshot(s.ctx)
		if err != nil {
			return
		}
		if len(snap.View.CurrentPlayer.CardReceipts) > 0 {
			if err := s.bc.tools.AcknowledgeReceipts(s.ctx, snap); err != nil {
				s.log.Warn("Failed to acknowledge card receipts", slog.Any("error", err))
			}
		}
		mine = snap.Game.Status() == shared.GameStatusActive && IsMyTurn(&snap.View, s.playerID)
	})
	if !mine {
		return
	}
	s.cancelPlanner()
	remark := s.playTurn()
	if s.ctx.Err() != nil {
		return
	}
	s.setStatus(playerPkg.BotStatusReady)
	if remark != "" && rand.Float64() < s.bc.cfg.RemarkThoughtChance {
		s.onGame(func() { s.Think(remark) })
	}
	s.startPlanner(true, "")
}

// turnState is what the turn loop needs from one look at the game.
type turnState struct {
	mine       bool
	reason     string
	invocation Invocation
	intent     string
}

func (s *botSession) readTurn() (turnState, bool) {
	var st turnState
	ran := s.onGame(func() {
		snap, err := s.snapshot(s.ctx)
		if err != nil || snap.Game.Status() != shared.GameStatusActive || !IsMyTurn(&snap.View, s.playerID) {
			return
		}
		st.mine = true
		if st.reason = s.autopilotReason(snap.Game); st.reason != "" {
			return
		}
		plan := s.Plan()
		st.intent = plan.Intent()
		cfg := s.bc.cfg
		st.invocation = Invocation{
			Model:        cfg.ExecutorModel,
			SystemPrompt: executorSystemPrompt(s.name, s.persona, cfg.Strategy),
			Prompt:       executorPrompt(snap, plan, s.grudgeText()),
			Token:        snap.Game.Settings().ClaudeOAuthToken,
			MaxBudgetUSD: s.budget(snap.Game, cfg.ExecutorBudgetUSD),
		}
	})
	return st, ran
}

// playTurn plays until the turn is over and returns the model's closing remark, if any.
func (s *botSession) playTurn() string {
	cfg := s.bc.cfg
	remark := ""
	introduced := false
	for step := 0; s.ctx.Err() == nil; step++ {
		st, ran := s.readTurn()
		if !ran || !st.mine {
			return remark
		}
		if step >= cfg.MaxStepsPerTurn {
			s.log.Error("Bot exceeded its step limit for one turn")
			s.markFailed("the bot got stuck and stopped acting this turn")
			return remark
		}

		if st.reason != "" {
			s.markFailed(st.reason)
			if err := s.autopilotStep(); err != nil {
				s.log.Error("Autopilot could not act", slog.Any("error", err))
				s.markFailed("autopilot could not act: " + truncate(err.Error(), 150))
				return remark
			}
			continue
		}

		s.setStatus(playerPkg.BotStatusThinking)
		before := s.actions()
		showIntent := !introduced && rand.Float64() < cfg.IntentThoughtChance
		introduced = true
		text, err := s.runExecutor(st, showIntent)
		if err == nil {
			remark = text
		}
		if err == nil && s.actions() == before {
			err = errors.New("the model ended its turn without acting")
		}
		if err == nil {
			s.mu.Lock()
			s.failures = 0
			s.mu.Unlock()
			continue
		}

		s.log.Warn("Bot turn attempt failed", slog.Any("error", err))
		s.mu.Lock()
		s.failures++
		failures := s.failures
		s.mu.Unlock()
		if failures > len(cfg.RetryBackoff) {
			s.markFailed(truncate(err.Error(), 200))
			continue
		}
		select {
		case <-s.ctx.Done():
			return remark
		case <-time.After(cfg.RetryBackoff[failures-1]):
		}
	}
	return remark
}

func (s *botSession) autopilotStep() error {
	if err := s.BeforeAction(s.ctx); err != nil {
		return err
	}
	var err error
	if !s.onGame(func() {
		var snap *Snapshot
		if snap, err = s.snapshot(s.ctx); err != nil {
			return
		}
		if err = s.bc.tools.Autopilot(s.ctx, snap); err == nil {
			s.AfterAction()
		}
	}) {
		return s.ctx.Err()
	}
	return err
}

// autopilotReason returns why the bot must play without the model, or "" when it may use it.
// It runs on the executor.
func (s *botSession) autopilotReason(g *game.Game) string {
	if s.overBudget(g) {
		return "spend cap reached"
	}
	s.mu.Lock()
	failed := s.failed
	s.mu.Unlock()
	if !failed {
		return ""
	}
	if p, err := g.GetPlayer(s.playerID); err == nil && p.BotError() != "" {
		return p.BotError()
	}
	return "the bot failed"
}

func (s *botSession) overBudget(g *game.Game) bool {
	return g.BotSpendUSD() >= g.Settings().EffectiveBotSpendCapUSD()
}

func (s *botSession) budget(g *game.Game, perCall float64) float64 {
	return min(perCall, g.Settings().EffectiveBotSpendCapUSD()-g.BotSpendUSD())
}

func (s *botSession) addSpend(cost float64) {
	if cost <= 0 {
		return
	}
	s.onGame(func() {
		if g, err := s.bc.gameRepo.Get(s.ctx, s.gameID); err == nil {
			g.AddBotSpend(cost)
		}
	})
}

// runExecutor plays one model call of the turn and returns its closing remark.
func (s *botSession) runExecutor(st turnState, showIntent bool) (string, error) {
	if showIntent && st.intent != "" {
		s.onGame(func() { s.Think(st.intent) })
	}
	s.mu.Lock()
	s.chatsLeft = maxChatsPerTurn
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(s.ctx, s.bc.cfg.TurnTimeout)
	defer cancel()
	result, err := s.call(ctx, "executor", st.invocation, RoleExecutor)
	return result.Text, err
}

// call runs one traced model call. With a tool role it gets its own tool grant whose
// tool calls are recorded on this call.
func (s *botSession) call(ctx context.Context, role string, inv Invocation, toolRole Role) (Result, error) {
	id, started := s.trace.startCall(role, inv.Model, inv.Prompt)
	s.bc.publishTrace(s.gameID, started)
	if toolRole != "" {
		endpoint, revoke := s.bc.tools.Issue(s.gameID, s.playerID, toolRole, callHooks{botSession: s, callID: id})
		defer revoke()
		inv.MCP = endpoint
	}
	inv.OnText = func(text string) {
		s.recordStep(id, dto.BotCallStepDto{Kind: "text", Text: text})
	}
	result, err := s.bc.runner.Run(ctx, inv)
	if ended, ok := s.trace.endCall(id, result, err); ok {
		s.bc.publishTrace(s.gameID, ended)
	}
	s.addSpend(result.CostUSD)
	return result, err
}

func (s *botSession) recordStep(callID string, step dto.BotCallStepDto) {
	if event, ok := s.trace.addStep(callID, step); ok {
		s.bc.publishTrace(s.gameID, event)
	}
}

func (s *botSession) recordReaction(a Assessment, decision, output string) {
	s.bc.publishTrace(s.gameID, s.trace.addReaction(dto.BotReactionDto{
		Lines:    append([]string{}, a.Lines...),
		PlanHit:  a.PlanHit,
		Personal: a.Personal,
		Directed: a.Directed,
		Big:      a.Big,
		Decision: decision,
		Output:   output,
	}))
}

// callHooks are the session hooks for one traced call, so its tool calls land on that call.
type callHooks struct {
	*botSession
	callID string
}

// RecordTool records a finished tool call on the owning model call.
func (h callHooks) RecordTool(name, input, result string, isError bool) {
	h.recordStep(h.callID, dto.BotCallStepDto{Kind: "tool", Tool: name, Input: input, Result: result, IsError: isError})
}

// lossesFromLog finds resources and production the bot lost to other players' actions in
// log entries after seenSeq. The log names the acting player, so blame is exact.
func lossesFromLog(diffs []game.StateDiff, botID string, seenSeq int64) ([]Happening, int64) {
	var losses []Happening
	last := seenSeq
	for _, d := range diffs {
		if d.SequenceNumber <= seenSeq {
			continue
		}
		last = max(last, d.SequenceNumber)
		if d.PlayerID == "" || d.PlayerID == botID || d.Changes == nil {
			continue
		}
		pc := d.Changes.PlayerChanges[botID]
		if pc == nil {
			continue
		}
		for _, r := range []struct {
			name  string
			value *game.DiffValueInt
		}{
			{"credits", pc.Credits}, {"steel", pc.Steel}, {"titanium", pc.Titanium},
			{"plants", pc.Plants}, {"energy", pc.Energy}, {"heat", pc.Heat},
			{"credit production", pc.CreditsProduction}, {"steel production", pc.SteelProduction},
			{"titanium production", pc.TitaniumProduction}, {"plant production", pc.PlantsProduction},
			{"energy production", pc.EnergyProduction}, {"heat production", pc.HeatProduction},
		} {
			if r.value != nil && r.value.New < r.value.Old {
				losses = append(losses, Happening{Kind: HappeningResourceLoss, ActorID: d.PlayerID, Target: r.name, Amount: r.value.Old - r.value.New, At: d.Timestamp})
			}
		}
	}
	return losses, last
}

// hostileTrigger says why a hostile batch triggers a replan, or "" when nothing hostile happened.
func hostileTrigger(a Assessment) string {
	if len(a.Hostile) == 0 {
		return ""
	}
	return strings.Join(a.Lines, "\n")
}

// grudgeText renders who has targeted the bot, for prompts.
func (s *botSession) grudgeText() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.grudges.describe()
}

// grudgeTextWith renders the grudges as they will be once the given acts are recorded, so a
// reaction already knows "that's the second time".
func (s *botSession) grudgeTextWith(hostile []Hostility, nameOf func(string) string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	preview := grudgeLedger{}
	for id, g := range s.grudges {
		copied := *g
		copied.recent = append([]string{}, g.recent...)
		preview[id] = &copied
	}
	for _, h := range hostile {
		preview.record(h, nameOf(h.ActorID), time.Now())
	}
	return preview.describe()
}

func (s *botSession) recordGrudges(hostile []Hostility, names map[string]string) {
	s.mu.Lock()
	for _, h := range hostile {
		s.grudges.record(h, names[h.ActorID], time.Now())
	}
	grudges := s.grudges.dtos()
	s.mu.Unlock()
	s.bc.publishTrace(s.gameID, s.trace.setGrudges(grudges))
}

// reactionMaxAge drops events too old to react to naturally.
const reactionMaxAge = time.Minute

// minReplanInterval limits replanning triggered by opponents' moves.
const minReplanInterval = 30 * time.Second

// startPlanner plans ahead in the background. Without force it only replans when the
// last planning run is old enough, so a busy table does not trigger a stream of calls.
// A trigger says why the bot is replanning (it was just targeted); a triggered replan that
// arrives while another is running is queued and runs right after it.
func (s *botSession) startPlanner(force bool, trigger string) {
	s.mu.Lock()
	if s.failed {
		s.mu.Unlock()
		return
	}
	if s.planCancel != nil {
		if trigger != "" {
			s.pendingTrigger = strings.TrimSpace(s.pendingTrigger + "\n" + trigger)
		}
		s.mu.Unlock()
		return
	}
	if trigger == "" && !force && time.Since(s.lastPlanRun) < minReplanInterval {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	cfg := s.bc.cfg
	var inv Invocation
	ok := false
	s.onGame(func() {
		snap, err := s.snapshot(s.ctx)
		if err != nil || snap.Game.Status() != shared.GameStatusActive || s.overBudget(snap.Game) || IsMyTurn(&snap.View, s.playerID) {
			return
		}
		inv = Invocation{
			Model:        cfg.PlannerModel,
			SystemPrompt: plannerSystemPrompt(s.name, s.persona, cfg.Strategy),
			Prompt:       plannerPrompt(snap, s.Plan(), s.grudgeText(), trigger),
			Token:        snap.Game.Settings().ClaudeOAuthToken,
			MaxBudgetUSD: s.budget(snap.Game, cfg.PlannerBudgetUSD),
		}
		ok = true
	})
	if !ok {
		return
	}

	s.mu.Lock()
	if s.planCancel != nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithTimeout(s.ctx, cfg.PlanTimeout)
	s.planCancel = cancel
	s.lastPlanRun = time.Now()
	s.mu.Unlock()

	go func() {
		defer func() {
			cancel()
			s.mu.Lock()
			s.planCancel = nil
			pending := s.pendingTrigger
			s.pendingTrigger = ""
			s.mu.Unlock()
			if pending != "" && s.ctx.Err() == nil {
				s.startPlanner(true, pending)
			}
		}()
		_, err := s.call(ctx, "planner", inv, RolePlanner)
		if err != nil && ctx.Err() == nil {
			s.log.Warn("Bot planning failed", slog.Any("error", err))
		}
	}()
}

func (s *botSession) cancelPlanner() {
	s.mu.Lock()
	cancel := s.planCancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// reactionInput is what a reaction needs, read from the game in one go.
type reactionInput struct {
	assessment Assessment
	names      map[string]string
	invocation Invocation
	myTurn     bool
	blocked    bool
	// quiet means the phase forbids reactions: setup and the opening showcase.
	quiet bool
}

func (s *botSession) readReaction(batch []Happening) (reactionInput, bool) {
	var in reactionInput
	ran := s.onGame(func() {
		snap, err := s.snapshot(s.ctx)
		if err != nil || snap.Game.Status() != shared.GameStatusActive {
			in.blocked = true
			return
		}
		in.myTurn = IsMyTurn(&snap.View, s.playerID)
		cutoff := time.Now().Add(-reactionMaxAge)
		happenings := make([]Happening, 0, len(batch))
		checkLosses := false
		for _, h := range batch {
			if h.At.Before(cutoff) {
				continue
			}
			switch h.Kind {
			case HappeningResourceLoss:
				checkLosses = true
				continue
			case HappeningCard:
				if card, err := cards.ForGame(s.bc.cardRegistry, snap.Game).GetByID(h.Detail); err == nil {
					h.Big = card.Cost >= s.bc.cfg.BigCardCost
				}
			}
			happenings = append(happenings, h)
		}
		losses, lastSeq := lossesFromLog(snap.Log, s.playerID, s.seenLogSeq)
		s.seenLogSeq = lastSeq
		if checkLosses {
			happenings = append(happenings, losses...)
		}

		soleOpponent := len(snap.View.OtherPlayers) == 1
		nameOf := func(id string) string { return findPlayerName(&snap.View, id) }
		plan := s.Plan()
		in.assessment = AssessHappenings(plan, s.playerID, s.name, soleOpponent, happenings, nameOf)
		in.names = map[string]string{}
		for _, h := range in.assessment.Hostile {
			in.names[h.ActorID] = nameOf(h.ActorID)
		}
		in.quiet = !presencePhase(snap.Game.CurrentPhase())
		in.blocked = s.overBudget(snap.Game) || in.quiet
		cfg := s.bc.cfg
		in.invocation = Invocation{
			Model:        cfg.ReactorModel,
			SystemPrompt: reactorSystemPrompt(s.name, s.persona),
			Prompt:       reactorPrompt(in.assessment.Lines, plan, s.grudgeTextWith(in.assessment.Hostile, nameOf), snap.Chat),
			Token:        snap.Game.Settings().ClaudeOAuthToken,
			MaxBudgetUSD: s.budget(snap.Game, cfg.ReactorBudgetUSD),
		}
	})
	return in, ran
}

func (s *botSession) considerReaction(batch []Happening) {
	in, ran := s.readReaction(batch)
	if ran && len(in.assessment.Hostile) > 0 {
		s.recordGrudges(in.assessment.Hostile, in.names)
	}
	s.mu.Lock()
	failed := s.failed
	s.mu.Unlock()
	if ran && in.quiet && in.assessment.Relevant() {
		s.recordReaction(in.assessment, "quiet", "")
	}
	if !ran || in.blocked || failed {
		return
	}
	if (in.assessment.PlanHit || len(in.assessment.Hostile) > 0) && !in.myTurn {
		s.startPlanner(false, hostileTrigger(in.assessment))
	}
	if !in.assessment.Relevant() {
		return
	}
	if in.myTurn && !in.assessment.Personal && !in.assessment.PlanHit {
		s.recordReaction(in.assessment, "own-turn", "")
		return
	}

	s.mu.Lock()
	busy := s.reacting
	allowed := !busy && s.gate.Allow(in.assessment, time.Now(), rand.Float64())
	if allowed {
		s.reacting = true
	}
	s.mu.Unlock()
	switch {
	case allowed:
		go s.react(in)
	case busy:
		s.recordReaction(in.assessment, "busy", "")
	default:
		s.recordReaction(in.assessment, "throttled", "")
	}
}

func (s *botSession) react(in reactionInput) {
	defer func() {
		s.mu.Lock()
		s.reacting = false
		s.mu.Unlock()
	}()
	if in.assessment.Directed {
		s.onGame(func() { s.bc.broadcaster.BroadcastBotThought(s.gameID, s.playerID, "", true) })
		defer s.onGame(func() { s.bc.broadcaster.BroadcastBotThought(s.gameID, s.playerID, "", false) })
	}

	ctx, cancel := context.WithTimeout(s.ctx, s.bc.cfg.ReactTimeout)
	defer cancel()
	result, err := s.call(ctx, "reactor", in.invocation, "")
	if err != nil {
		s.log.Debug("Bot reaction failed", slog.Any("error", err))
		s.recordReaction(in.assessment, "reacted", "failed: "+truncate(err.Error(), 120))
		return
	}
	reaction := ParseReaction(result.Text)
	s.recordReaction(in.assessment, "reacted", strings.TrimSpace(reaction.Kind+" "+reaction.Emote+reaction.Text))
	s.onGame(func() {
		switch reaction.Kind {
		case "chat":
			if snap, err := s.snapshot(s.ctx); err == nil {
				s.post(s.ctx, snap, reaction.Text, shared.ChatMessageKindChat)
			}
		case "emote":
			if err := s.Emote(reaction.Emote); err != nil {
				s.log.Debug("Bot reaction emote rejected", slog.Any("error", err))
			}
		}
	})
}

func (s *botSession) recap() {
	var inv Invocation
	ok := false
	s.onGame(func() {
		snap, err := s.snapshot(s.ctx)
		if err != nil || s.overBudget(snap.Game) {
			return
		}
		cfg := s.bc.cfg
		inv = Invocation{
			Model:        cfg.ReactorModel,
			SystemPrompt: identityPreamble(s.name, s.persona) + "\n\n" + untrustedTextRule,
			Prompt:       recapPrompt(snap, s.Plan()),
			Token:        snap.Game.Settings().ClaudeOAuthToken,
			MaxBudgetUSD: s.budget(snap.Game, cfg.ReactorBudgetUSD),
		}
		ok = true
	})
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(s.ctx, s.bc.cfg.ReactTimeout)
	defer cancel()
	result, err := s.call(ctx, "recap", inv, "")
	if err != nil {
		s.log.Debug("Bot recap failed", slog.Any("error", err))
		return
	}
	text := cleanLine(result.Text, shared.MaxChatMessageLength)
	if text == "" {
		return
	}
	s.onGame(func() {
		if snap, err := s.snapshot(s.ctx); err == nil {
			s.bc.say(s.ctx, snap.Game, snap.Player, text, shared.ChatMessageKindRecap)
		}
	})
}

func (s *botSession) setStatus(status playerPkg.BotStatus) {
	s.onGame(func() {
		g, err := s.bc.gameRepo.Get(s.ctx, s.gameID)
		if err != nil {
			return
		}
		p, err := g.GetPlayer(s.playerID)
		if err != nil {
			return
		}
		s.mu.Lock()
		failed := s.failed
		s.mu.Unlock()
		if failed {
			status = playerPkg.BotStatusFailed
		}
		if p.BotStatus() == status {
			return
		}
		p.SetBotStatus(status)
		s.bc.broadcaster.BroadcastGameState(s.gameID, nil)
	})
}

func (s *botSession) markFailed(reason string) {
	s.mu.Lock()
	s.failed = true
	s.mu.Unlock()

	s.onGame(func() {
		g, err := s.bc.gameRepo.Get(s.ctx, s.gameID)
		if err != nil {
			return
		}
		p, err := g.GetPlayer(s.playerID)
		if err != nil {
			return
		}
		if p.BotStatus() == playerPkg.BotStatusFailed && p.BotError() == reason {
			return
		}
		s.log.Warn("Bot switched to autopilot", slog.String("reason", reason))
		p.SetBotStatus(playerPkg.BotStatusFailed)
		p.SetBotError(reason)
		s.bc.broadcaster.BroadcastGameState(s.gameID, nil)
	})
}

func (s *botSession) clearFailure() {
	s.mu.Lock()
	s.failed = false
	s.failures = 0
	s.mu.Unlock()

	s.onGame(func() {
		g, err := s.bc.gameRepo.Get(s.ctx, s.gameID)
		if err != nil {
			return
		}
		p, err := g.GetPlayer(s.playerID)
		if err != nil {
			return
		}
		p.SetBotStatus(playerPkg.BotStatusReady)
		p.SetBotError("")
		s.bc.broadcaster.BroadcastGameState(s.gameID, nil)
	})
	s.log.Info("Bot retried by host")
}

func (s *botSession) actions() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.actionCount
}

// Plan returns the bot's current plan.
func (s *botSession) Plan() Plan {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.plan
}

// SetPlan replaces the bot's plan.
func (s *botSession) SetPlan(plan Plan) {
	s.mu.Lock()
	s.plan = plan
	s.mu.Unlock()
	s.bc.publishTrace(s.gameID, s.trace.setPlan(plan))
}

// Think shows a thought bubble above the bot's card. It runs on the executor.
func (s *botSession) Think(text string) {
	text = cleanLine(text, 80)
	if text == "" || !s.presenceAllowed() {
		return
	}
	s.mu.Lock()
	if strings.EqualFold(text, s.lastThought) || time.Since(s.thoughtAt) < s.bc.cfg.ThoughtInterval {
		s.mu.Unlock()
		return
	}
	s.lastThought, s.thoughtAt = text, time.Now()
	s.mu.Unlock()
	s.bc.broadcaster.BroadcastBotThought(s.gameID, s.playerID, text, false)
}

// presenceAllowed reports whether the bot may chat, think or emote now. Bots stay quiet
// during setup and the corporation and prelude showcase. It runs on the executor.
func (s *botSession) presenceAllowed() bool {
	g, err := s.bc.gameRepo.Get(s.ctx, s.gameID)
	if err != nil {
		return false
	}
	return presencePhase(g.CurrentPhase())
}

func presencePhase(phase shared.GamePhase) bool {
	switch phase {
	case shared.GamePhaseAction, shared.GamePhaseProductionAndCardDraw, shared.GamePhaseFinalPhase:
		return true
	default:
		return false
	}
}

// post sends a chat message unless the bot said the same thing recently. It runs on the executor.
func (s *botSession) post(ctx context.Context, snap *Snapshot, text string, kind shared.ChatMessageKind) {
	if repeatsRecent(snap.Chat, s.playerID, text) {
		return
	}
	s.bc.say(ctx, snap.Game, snap.Player, text, kind)
}

// repeatsRecent reports whether the sender's last few messages already contain text.
func repeatsRecent(chat []shared.ChatMessage, senderID, text string) bool {
	seen := 0
	for i := len(chat) - 1; i >= 0 && seen < 3; i-- {
		if chat[i].SenderID != senderID {
			continue
		}
		seen++
		if strings.EqualFold(chat[i].Message, text) {
			return true
		}
	}
	return false
}

// Emote shows an emote over the bot's card. It runs on the executor.
func (s *botSession) Emote(name string) error {
	if !shared.IsEmote(name) {
		return fmt.Errorf("unknown emote %q", name)
	}
	if !s.presenceAllowed() {
		return nil
	}
	s.bc.broadcaster.BroadcastEmote(s.gameID, s.playerID, name)
	return nil
}

// Say posts a chat message from the bot, limited per turn. It runs on the executor.
func (s *botSession) Say(ctx context.Context, text string) error {
	text = cleanLine(text, shared.MaxChatMessageLength)
	if text == "" {
		return errors.New("empty message")
	}
	if !s.presenceAllowed() {
		return nil
	}
	s.mu.Lock()
	if s.chatsLeft <= 0 {
		s.mu.Unlock()
		return errors.New("chat limit for this turn reached")
	}
	s.chatsLeft--
	s.mu.Unlock()

	snap, err := s.snapshot(ctx)
	if err != nil {
		return err
	}
	s.post(ctx, snap, text, shared.ChatMessageKindChat)
	return nil
}

// BeforeAction waits until the presentation gap since the bot's last action has passed.
func (s *botSession) BeforeAction(ctx context.Context) error {
	s.mu.Lock()
	wait := time.Until(s.lastActionAt.Add(s.bc.cfg.ActionGap))
	s.mu.Unlock()
	if wait <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
		return nil
	}
}

// AfterAction records a game action and publishes the new state. It runs on the executor.
func (s *botSession) AfterAction() {
	s.mu.Lock()
	s.lastActionAt = time.Now()
	s.actionCount++
	s.mu.Unlock()
	s.bc.broadcaster.BroadcastGameState(s.gameID, nil)
}

// Snapshot returns the game as the bot sees it now. It runs on the executor.
func (s *botSession) Snapshot(ctx context.Context) (*Snapshot, error) {
	return s.snapshot(ctx)
}
