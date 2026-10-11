package bot_test

import (
	"context"
	"net/http"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	gameaction "openmars/internal/action/game"
	milestoneAction "openmars/internal/action/milestone"
	stdprojAction "openmars/internal/action/standard_project"
	tileAction "openmars/internal/action/tile"
	turnAction "openmars/internal/action/turn_management"
	"openmars/internal/delivery/dto"
	"openmars/internal/game"
	playerPkg "openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/internal/game/standardproject"
	"openmars/internal/service/bot"
	"openmars/test/testutil"
)

// serialExecutor stands in for the hub: it runs one function at a time.
type serialExecutor struct {
	mu sync.Mutex
}

func (e *serialExecutor) Do(ctx context.Context, fn func()) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	fn()
	return nil
}

type botFixture struct {
	exec      *serialExecutor
	stateRepo game.GameStateRepository
	game      *game.Game
	repo      game.GameRepository
	botID     string
	humanID   string
	tools     *bot.ToolServer
	registry  bot.Registries
}

func loadStandardProjects(t *testing.T) standardproject.StandardProjectRegistry {
	t.Helper()
	_, currentFile, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "assets", "standard_projects.json")
	data, err := standardproject.LoadStandardProjectsFromJSON(path)
	testutil.AssertNoError(t, err, "standard projects should load")
	return standardproject.NewInMemoryStandardProjectRegistry(data)
}

// newBotFixture starts a two-player game where the first player in turn order is a bot
// that has the turn, and starts a tool server bound to the game's actions.
func newBotFixture(t *testing.T) *botFixture {
	t.Helper()
	g, repo, cardRegistry, botID, humanID := testutil.SetupTwoPlayerGame(t)
	ctx := context.Background()
	p, err := g.GetPlayer(botID)
	testutil.AssertNoError(t, err, "bot player should exist")
	p.SetPlayerType(playerPkg.PlayerTypeBot)
	p.SetBotPersona("rival")
	p.SetBotStatus(playerPkg.BotStatusReady)
	g.UpdateSettings(ctx, shared.GameSettings{MaxPlayers: 4, GameModeID: "classic", Features: []string{}, ClaudeOAuthToken: "test-token", BotSpendCapUSD: 5})

	logger := testutil.TestLogger()
	exec := &serialExecutor{}
	stdProjects := loadStandardProjects(t)
	finalScoring := gameaction.NewFinalScoringAction(repo, cardRegistry, nil, nil, logger)
	registries := bot.Registries{
		Cards:            cardRegistry,
		StandardProjects: stdProjects,
		Milestones:       testutil.CreateTestMilestoneRegistry(),
		Awards:           testutil.CreateTestAwardRegistry(),
	}
	tools := bot.NewToolServer(bot.Actions{
		SkipAction:             turnAction.NewSkipActionAction(repo, finalScoring, logger),
		ExecuteStandardProject: stdprojAction.NewExecuteStandardProjectAction(repo, cardRegistry, stdProjects, nil, logger),
		SelectTile:             tileAction.NewSelectTileAction(repo, cardRegistry, game.NewInMemoryGameStateRepository(), logger),
		ClaimMilestone:         milestoneAction.NewClaimMilestoneAction(repo, cardRegistry, game.NewInMemoryGameStateRepository(), registries.Milestones, logger),
	}, registries, repo, exec, logger)
	serverCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	testutil.AssertNoError(t, tools.Start(serverCtx), "tool server should start")

	return &botFixture{exec: exec, stateRepo: game.NewInMemoryGameStateRepository(), game: g, repo: repo, botID: botID, humanID: humanID, tools: tools, registry: registries}
}

type bearerTransport struct {
	token string
}

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func connectMCP(ctx context.Context, endpoint *bot.MCPEndpoint) (*mcp.ClientSession, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	return client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   endpoint.URL,
		HTTPClient: &http.Client{Transport: bearerTransport{token: endpoint.Bearer}},
	}, nil)
}

func callTool(ctx context.Context, endpoint *bot.MCPEndpoint, name string, args map[string]any) (*mcp.CallToolResult, error) {
	session, err := connectMCP(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	defer func() { _ = session.Close() }()
	return session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
}

func resultText(r *mcp.CallToolResult) string {
	if r == nil || len(r.Content) == 0 {
		return ""
	}
	if text, ok := r.Content[0].(*mcp.TextContent); ok {
		return text.Text
	}
	return ""
}

// fakeHooks is a minimal SessionHooks for tool server tests.
type fakeHooks struct {
	fx      *botFixture
	mu      sync.Mutex
	plan    bot.Plan
	thought []string
	emotes  []string
	said    []string
	tools   []string
	actions int
}

func (h *fakeHooks) Plan() bot.Plan {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.plan
}

func (h *fakeHooks) SetPlan(plan bot.Plan) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.plan = plan
}

func (h *fakeHooks) Think(text string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.thought = append(h.thought, text)
}

func (h *fakeHooks) Emote(name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.emotes = append(h.emotes, name)
	return nil
}

func (h *fakeHooks) Say(_ context.Context, text string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.said = append(h.said, text)
	return nil
}

func (h *fakeHooks) BeforeAction(context.Context) error { return nil }

func (h *fakeHooks) AfterAction() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.actions++
}

func (h *fakeHooks) Snapshot(ctx context.Context) (*bot.Snapshot, error) {
	g, err := h.fx.repo.Get(ctx, h.fx.game.ID())
	if err != nil {
		return nil, err
	}
	p, err := g.GetPlayer(h.fx.botID)
	if err != nil {
		return nil, err
	}
	return &bot.Snapshot{Game: g, Player: p, View: viewFor(g, h.fx), PlayerID: h.fx.botID}, nil
}

func viewFor(g *game.Game, fx *botFixture) dto.GameDto {
	return dto.ToGameDto(g, fx.registry.Cards, fx.botID)
}

// read runs fn on the fixture's executor so test reads never race the bot.
func (fx *botFixture) read(fn func()) {
	_ = fx.exec.Do(context.Background(), fn)
}

func (fx *botFixture) currentTurn() string {
	var id string
	fx.read(func() { id = fx.game.CurrentTurn().PlayerID() })
	return id
}

func (h *fakeHooks) RecordTool(name, _, _ string, _ bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.tools = append(h.tools, name)
}
