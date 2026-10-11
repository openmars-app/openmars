package game_lifecycle_test

import (
	"context"
	"slices"
	"testing"

	gameAction "openmars/internal/action/game"
	gamePkg "openmars/internal/game"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

type updateSettingsTestRig struct {
	action    *gameAction.UpdateGameSettingsAction
	repo      gamePkg.GameRepository
	gameID    string
	playerIDs []string
}

func newUpdateSettingsRig(t *testing.T, players int) *updateSettingsTestRig {
	t.Helper()
	repo := testutil.NewTestGameRepository(t)
	cardRegistry := testutil.CreateTestModeCatalog()
	mapRegistry := testutil.CreateTestMapRegistry()
	logger := testutil.TestLogger()

	createAction := gameAction.NewCreateGameAction(repo, cardRegistry, mapRegistry, logger)
	g, err := createAction.Execute(context.Background(), shared.GameSettings{})
	testutil.AssertNoError(t, err, "create game")

	playerIDs := make([]string, 0, players)
	for i := 0; i < players; i++ {
		pid := "player-" + string(rune('a'+i))
		p, err := g.AddNewPlayer(context.Background(), pid, pid)
		testutil.AssertNoError(t, err, "add player")
		playerIDs = append(playerIDs, p.ID())
	}
	if len(playerIDs) > 0 {
		err := g.SetHostPlayerID(context.Background(), playerIDs[0])
		testutil.AssertNoError(t, err, "set host")
	}

	action := gameAction.NewUpdateGameSettingsAction(repo, cardRegistry, mapRegistry, logger)
	return &updateSettingsTestRig{
		action:    action,
		repo:      repo,
		gameID:    g.ID(),
		playerIDs: playerIDs,
	}
}

func (r *updateSettingsTestRig) host() string {
	return r.playerIDs[0]
}

func (r *updateSettingsTestRig) game(t *testing.T) *gamePkg.Game {
	t.Helper()
	g, err := r.repo.Get(context.Background(), r.gameID)
	testutil.AssertNoError(t, err, "get game")
	return g
}

func boolPtr(b bool) *bool             { return &b }
func intPtr(v int) *int                { return &v }
func strPtr(s string) *string          { return &s }
func featuresPtr(p []string) *[]string { return &p }

func TestUpdateGameSettings_HostOnly(t *testing.T) {
	rig := newUpdateSettingsRig(t, 2)
	nonHost := rig.playerIDs[1]

	err := rig.action.Execute(context.Background(), rig.gameID, nonHost, &gameAction.SettingsPatch{
		DevelopmentMode: boolPtr(true),
	})
	testutil.AssertErrorContains(t, err, "only the host can change settings", "non-host should be rejected")
}

func TestUpdateGameSettings_LobbyOnly(t *testing.T) {
	rig := newUpdateSettingsRig(t, 1)
	g := rig.game(t)
	err := g.UpdateStatus(context.Background(), shared.GameStatusActive)
	testutil.AssertNoError(t, err, "update status")

	err = rig.action.Execute(context.Background(), rig.gameID, rig.host(), &gameAction.SettingsPatch{
		DevelopmentMode: boolPtr(true),
	})
	testutil.AssertErrorContains(t, err, "settings can only be changed during lobby phase", "active game should reject settings change")
}

func TestUpdateGameSettings_MaxPlayersBelowJoinedRejected(t *testing.T) {
	rig := newUpdateSettingsRig(t, 3)

	err := rig.action.Execute(context.Background(), rig.gameID, rig.host(), &gameAction.SettingsPatch{
		MaxPlayers: intPtr(2),
	})
	testutil.AssertErrorContains(t, err, "cannot reduce max players below current player count", "lowering max below joined count should reject")
}

func TestUpdateGameSettings_MaxPlayersValidRange(t *testing.T) {
	rig := newUpdateSettingsRig(t, 1)

	err := rig.action.Execute(context.Background(), rig.gameID, rig.host(), &gameAction.SettingsPatch{
		MaxPlayers: intPtr(0),
	})
	testutil.AssertErrorContains(t, err, "max players must be between 1 and 8", "max=0 should reject")

	err = rig.action.Execute(context.Background(), rig.gameID, rig.host(), &gameAction.SettingsPatch{
		MaxPlayers: intPtr(9),
	})
	testutil.AssertErrorContains(t, err, "max players must be between 1 and 8", "max=9 should reject")

	err = rig.action.Execute(context.Background(), rig.gameID, rig.host(), &gameAction.SettingsPatch{
		MaxPlayers: intPtr(8),
	})
	testutil.AssertNoError(t, err, "Classic max=8 should succeed")
	testutil.AssertEqual(t, 8, rig.game(t).Settings().MaxPlayers, "max players should update")
}

func TestUpdateGameSettings_RejectsUnknownModeAndFeatures(t *testing.T) {
	rig := newUpdateSettingsRig(t, 1)

	err := rig.action.Execute(context.Background(), rig.gameID, rig.host(), &gameAction.SettingsPatch{
		GameModeID: strPtr("unknown"),
	})
	testutil.AssertErrorContains(t, err, "unknown game mode", "unknown mode should reject")

	err = rig.action.Execute(context.Background(), rig.gameID, rig.host(), &gameAction.SettingsPatch{
		Features: featuresPtr([]string{"unknown"}),
	})
	testutil.AssertErrorContains(t, err, "feature unknown is unavailable in classic", "unknown feature should reject")
}

func TestUpdateGameSettings_DemoOffClearsChoices(t *testing.T) {
	rig := newUpdateSettingsRig(t, 2)

	err := rig.action.Execute(context.Background(), rig.gameID, rig.host(), &gameAction.SettingsPatch{
		DemoGame: boolPtr(true),
	})
	testutil.AssertNoError(t, err, "demo on")

	g := rig.game(t)
	p, err := g.GetPlayer(rig.playerIDs[1])
	testutil.AssertNoError(t, err, "get player")
	p.SetPendingDemoChoices(&shared.PendingDemoChoices{CorporationID: "credicor"})
	testutil.AssertTrue(t, p.HasPendingDemoChoices(), "should have choices before toggle")

	err = rig.action.Execute(context.Background(), rig.gameID, rig.host(), &gameAction.SettingsPatch{
		DemoGame: boolPtr(false),
	})
	testutil.AssertNoError(t, err, "demo off")

	g = rig.game(t)
	p, err = g.GetPlayer(rig.playerIDs[1])
	testutil.AssertNoError(t, err, "get player after toggle")
	testutil.AssertFalse(t, p.HasPendingDemoChoices(), "demo off should clear pending demo choices")
	testutil.AssertEqual(t, (*int)(nil), g.Settings().Temperature, "Temperature override should be cleared")
}

func TestUpdateGameSettings_AllowRandomBuyToggles(t *testing.T) {
	rig := newUpdateSettingsRig(t, 1)

	err := rig.action.Execute(context.Background(), rig.gameID, rig.host(), &gameAction.SettingsPatch{
		AllowRandomBuy: boolPtr(true),
	})
	testutil.AssertNoError(t, err, "set allow random buy")
	testutil.AssertTrue(t, rig.game(t).Settings().AllowRandomBuy, "allowRandomBuy should be set")

	err = rig.action.Execute(context.Background(), rig.gameID, rig.host(), &gameAction.SettingsPatch{
		AllowRandomBuy: boolPtr(false),
	})
	testutil.AssertNoError(t, err, "unset allow random buy")
	testutil.AssertFalse(t, rig.game(t).Settings().AllowRandomBuy, "allowRandomBuy should be cleared")
}

func TestUpdateGameSettings_UnknownMapRejected(t *testing.T) {
	rig := newUpdateSettingsRig(t, 1)

	err := rig.action.Execute(context.Background(), rig.gameID, rig.host(), &gameAction.SettingsPatch{
		MapID: strPtr("not-a-real-map"),
	})
	testutil.AssertErrorContains(t, err, "unknown map", "unknown map should reject")
}

func TestUpdateGameSettings_DevelopmentModeAndClaudeTokenSet(t *testing.T) {
	rig := newUpdateSettingsRig(t, 1)

	err := rig.action.Execute(context.Background(), rig.gameID, rig.host(), &gameAction.SettingsPatch{
		DevelopmentMode:  boolPtr(false),
		ClaudeOAuthToken: strPtr("sk-ant-oat-test"),
	})
	testutil.AssertNoError(t, err, "update dev + token")

	s := rig.game(t).Settings()
	testutil.AssertFalse(t, s.DevelopmentMode, "dev mode should be off")
	testutil.AssertEqual(t, "sk-ant-oat-test", s.ClaudeOAuthToken, "claude token should be set")
}

func TestUpdateGameSettings_BotSpendCap(t *testing.T) {
	rig := newUpdateSettingsRig(t, 1)
	testutil.AssertEqual(t, shared.DefaultBotSpendCapUSD, rig.game(t).Settings().EffectiveBotSpendCapUSD(), "unset cap uses the default")

	cap := 2.5
	err := rig.action.Execute(context.Background(), rig.gameID, rig.host(), &gameAction.SettingsPatch{BotSpendCapUSD: &cap})
	testutil.AssertNoError(t, err, "cap should be set")
	testutil.AssertEqual(t, 2.5, rig.game(t).Settings().EffectiveBotSpendCapUSD(), "cap should apply")

	zero := 0.0
	err = rig.action.Execute(context.Background(), rig.gameID, rig.host(), &gameAction.SettingsPatch{BotSpendCapUSD: &zero})
	testutil.AssertErrorContains(t, err, "bot spend cap must be positive", "cap must be positive")
}

func TestUpdateGameSettings_ClassicExpansionsResetForEarthWithdrawal(t *testing.T) {
	rig := newUpdateSettingsRig(t, 1)
	err := rig.action.Execute(context.Background(), rig.gameID, rig.host(), &gameAction.SettingsPatch{Features: featuresPtr([]string{"prelude", "venus", "colonies"})})
	testutil.AssertNoError(t, err, "enable both Classic expansions in lobby")
	g := rig.game(t)
	testutil.AssertTrue(t, g.Settings().HasVenus() && g.Settings().HasColonies(), "both systems enabled")
	testutil.AssertTrue(t, slices.Contains(g.Deck().ProjectCards(), "213"), "Venus cards enter deck")
	testutil.AssertTrue(t, slices.Contains(g.Deck().ProjectCards(), "C01"), "Colonies cards enter deck")
	err = rig.action.Execute(context.Background(), rig.gameID, rig.host(), &gameAction.SettingsPatch{GameModeID: strPtr("earth-withdrawal"), Features: featuresPtr([]string{"prelude"})})
	testutil.AssertNoError(t, err, "switch to Earth Withdrawal")
	testutil.AssertFalse(t, g.Settings().HasVenus() || g.Settings().HasColonies(), "expansion systems removed")
	testutil.AssertFalse(t, slices.Contains(g.Deck().ProjectCards(), "213"), "Venus card removed")
	testutil.AssertFalse(t, slices.Contains(g.Deck().ProjectCards(), "C01"), "Colonies card removed")
	testutil.AssertTrue(t, slices.Contains(g.Deck().ProjectCards(), "C14"), "curated Earth card remains")
}
