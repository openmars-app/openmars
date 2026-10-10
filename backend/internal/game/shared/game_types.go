package shared

import (
	"maps"
	"slices"
	"time"
)

// GameStatus represents the current status of a game
type GameStatus string

const (
	GameStatusLobby     GameStatus = "lobby"
	GameStatusActive    GameStatus = "active"
	GameStatusCompleted GameStatus = "completed"
)

// GamePhase represents the current phase of the game
type GamePhase string

const (
	GamePhaseWaitingForGameStart   GamePhase = "waiting_for_game_start"
	GamePhaseStartingSelection     GamePhase = "starting_selection"
	GamePhaseStartGameSelection    GamePhase = "start_game_selection"
	GamePhaseInitApplyCorp         GamePhase = "init_apply_corp"
	GamePhaseInitApplyPrelude      GamePhase = "init_apply_prelude"
	GamePhaseAction                GamePhase = "action"
	GamePhaseProductionAndCardDraw GamePhase = "production_and_card_draw"
	GamePhaseFinalPhase            GamePhase = "final_phase"
	GamePhaseComplete              GamePhase = "complete"
)

var gamePhases = []GamePhase{
	GamePhaseWaitingForGameStart,
	GamePhaseStartingSelection,
	GamePhaseStartGameSelection,
	GamePhaseInitApplyCorp,
	GamePhaseInitApplyPrelude,
	GamePhaseAction,
	GamePhaseProductionAndCardDraw,
	GamePhaseFinalPhase,
	GamePhaseComplete,
}

// IsGamePhase reports whether p is a phase a game can be in.
func IsGamePhase(p GamePhase) bool {
	return slices.Contains(gamePhases, p)
}

// GameSettings contains configurable game parameters
type GameSettings struct {
	MaxPlayers         int
	MapID              string
	Temperature        *int
	Oxygen             *int
	Oceans             *int
	Venus              *int
	DevelopmentMode    bool
	DemoGame           bool
	AllowRandomBuy     bool
	GameModeID         string
	GameModeParameters map[string]int
	Features           []string
	Systems            []string
	Generation         *int
	ClaudeOAuthToken   string `json:"-"`
	BotSpendCapUSD     float64
	SelectedMilestones []string
	SelectedAwards     []string
}

// DefaultBotSpendCapUSD is the per-game bot LLM spend cap used when the host has not set one.
const DefaultBotSpendCapUSD = 5.0

// EffectiveBotSpendCapUSD returns the configured bot spend cap, or the default when unset.
func (s GameSettings) EffectiveBotSpendCapUSD() float64 {
	if s.BotSpendCapUSD <= 0 {
		return DefaultBotSpendCapUSD
	}
	return s.BotSpendCapUSD
}

// HasPrelude reports whether Prelude is enabled for this selection.
func (s GameSettings) HasPrelude() bool        { return slices.Contains(s.Features, "prelude") }
func (s GameSettings) HasColonies() bool       { return slices.Contains(s.Systems, "colonies") }
func (s GameSettings) HasProjectFunding() bool { return slices.Contains(s.Features, "project-funding") }
func (s GameSettings) HasVenus() bool          { return slices.Contains(s.Systems, "venus") }

// AllowsContent applies mode membership and capability requirements consistently.
func (s GameSettings) AllowsContent(gameModes, requiresSystems, requiresFeatures []string) bool {
	if !slices.Contains(gameModes, s.GameModeID) {
		return false
	}
	for _, id := range requiresSystems {
		if !slices.Contains(s.Systems, id) {
			return false
		}
	}
	for _, id := range requiresFeatures {
		if !slices.Contains(s.Features, id) {
			return false
		}
	}
	return true
}

// SourceType identifies the type of action that caused a state change
type SourceType string

const (
	SourceTypeCardPlay                 SourceType = "card_play"
	SourceTypeCardAction               SourceType = "card_action"
	SourceTypeCorporationFirstAction   SourceType = "corporation_first_action"
	SourceTypeStandardProject          SourceType = "standard_project"
	SourceTypePassiveEffect            SourceType = "passive_effect"
	SourceTypeResourceConvert          SourceType = "resource_convert"
	SourceTypeGameEvent                SourceType = "game_event"
	SourceTypeInitial                  SourceType = "initial"
	SourceTypeAward                    SourceType = "award"
	SourceTypeMilestone                SourceType = "milestone"
	SourceTypeActionAdded              SourceType = "action_added"
	SourceTypeEffectAdded              SourceType = "effect_added"
	SourceTypeGlobalBonus              SourceType = "global_bonus"
	SourceTypeColonyTrade              SourceType = "colony_trade"
	SourceTypeColonyBuild              SourceType = "colony_build"
	SourceTypeProjectFundingSeat       SourceType = "project_funding_seat"
	SourceTypeProjectFundingCompletion SourceType = "project_funding_completion"
)

// Chat constants
const (
	MaxChatMessages      = 200
	MaxChatMessageLength = 500
)

// ChatMessageKind distinguishes ordinary chat from bot end-of-game recaps.
type ChatMessageKind string

const (
	ChatMessageKindChat  ChatMessageKind = "chat"
	ChatMessageKindRecap ChatMessageKind = "recap"
	// ChatMessageKindSystem is a notice such as a bot leaving, shown without a speaker.
	ChatMessageKindSystem ChatMessageKind = "system"
)

// Emotes are the reactions players and bots can show over their player card.
var Emotes = []string{"angry", "celebrate", "thinking", "applause", "shock", "laugh", "sad", "cool"}

// IsEmote reports whether name is a known emote.
func IsEmote(name string) bool {
	return slices.Contains(Emotes, name)
}

// Spectator constants and colors
const MaxSpectators = 4

// PlayerColors is the palette of 10 visually distinct colors available to players.
var PlayerColors = []string{
	"#e53935", "#1e88e5", "#43a047", "#ffb300", "#8e24aa",
	"#00acc1", "#f4511e", "#3949ab", "#c0ca33", "#d81b60",
}

// SpectatorColors is the palette of colors assigned to spectators.
var SpectatorColors = []string{"#9b9b9b", "#7eb8da", "#c4a6e8", "#e8c4a6"}

// SpectatorState represents a spectator's data
type SpectatorState struct {
	ID    string
	Name  string
	Color string
}

// ChatMessage represents a single chat message
type ChatMessage struct {
	SenderID    string
	SenderName  string
	SenderColor string
	Message     string
	Timestamp   time.Time
	IsSpectator bool
	Kind        ChatMessageKind
}

// ClaimedMilestone represents a milestone that has been claimed
type ClaimedMilestone struct {
	Type       MilestoneType
	PlayerID   string
	Generation int
	ClaimedAt  time.Time
}

// FundedAward represents an award that has been funded
type FundedAward struct {
	Type           AwardType
	FundedByPlayer string
	FundingOrder   int
	FundingCost    int
	FundedAt       time.Time
}

// FinalScore holds the final scoring breakdown for a player
type FinalScore struct {
	PlayerID   string
	PlayerName string
	Breakdown  VPBreakdown
	Credits    int
	Placement  int
	IsWinner   bool
}

// VPBreakdown contains the victory point breakdown
type VPBreakdown struct {
	TerraformRating   int
	CardVP            int
	CardVPDetails     []CardVPDetail
	MilestoneVP       int
	AwardVP           int
	GreeneryVP        int
	GreeneryVPDetails []GreeneryVPDetail
	CityVP            int
	CityVPDetails     []CityVPDetail
	TotalVP           int
}

// CardVPDetail contains VP details for a single card
type CardVPDetail struct {
	CardID     string
	CardName   string
	Conditions []CardVPConditionDetail
	TotalVP    int
}

// CardVPConditionDetail contains details of a single VP condition evaluation
type CardVPConditionDetail struct {
	ConditionType  string
	Amount         int
	Count          int
	MaxTrigger     *int
	ActualTriggers int
	TotalVP        int
	Explanation    string
}

// GreeneryVPDetail contains VP details for a greenery tile
type GreeneryVPDetail struct {
	Coordinate string
	VP         int
}

// CityVPDetail contains VP details for a city tile
type CityVPDetail struct {
	CityCoordinate     string
	AdjacentGreeneries []string
	VP                 int
}

// TriggeredEffect records a triggered passive effect for notification
type TriggeredEffect struct {
	CardName          string
	PlayerID          string
	SourceType        SourceType
	Outputs           []BehaviorCondition
	CalculatedOutputs []CalculatedOutput
	Behaviors         []CardBehavior
	VPConditions      []VPConditionForLog
}

// CalculatedOutput represents an actual output value that was applied
type CalculatedOutput struct {
	ResourceType string
	Amount       int
	IsScaled     bool
}

// VPConditionForLog is a simplified VP condition for log display
type VPConditionForLog struct {
	Amount     int
	Condition  string
	MaxTrigger *int
	Per        *PerCondition
}

// PendingDemoChoices holds a player's card selections made during the demo lobby phase
type PendingDemoChoices struct {
	CorporationID   string
	PreludeIDs      []string
	CardIDs         []string
	Resources       Resources
	Production      Production
	TerraformRating int
}

// DeferredStartingChoices holds choices that are applied after init
type DeferredStartingChoices struct {
	Payment              Payment
	CorporationID        string
	PreludeIDs           []string
	CardIDs              []string
	CorpApplied          bool
	PreludesAppliedCount int
}

// PreludesDone reports whether every chosen prelude has been applied.
func (c *DeferredStartingChoices) PreludesDone() bool {
	return c.PreludesAppliedCount >= len(c.PreludeIDs)
}

// Clone returns an independent settings snapshot.
func (s GameSettings) Clone() GameSettings {
	s.GameModeParameters = maps.Clone(s.GameModeParameters)
	s.Features = slices.Clone(s.Features)
	s.Systems = slices.Clone(s.Systems)
	s.SelectedMilestones = slices.Clone(s.SelectedMilestones)
	s.SelectedAwards = slices.Clone(s.SelectedAwards)
	s.Temperature = clonePointer(s.Temperature)
	s.Oxygen = clonePointer(s.Oxygen)
	s.Oceans = clonePointer(s.Oceans)
	s.Venus = clonePointer(s.Venus)
	s.Generation = clonePointer(s.Generation)
	return s
}
