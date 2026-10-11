package dto

import "encoding/json"

// GameSetupDto contains settings selected before creating a lobby.
type GameSetupDto struct {
	GameModeID         string         `json:"gameModeId"`
	GameModeParameters map[string]int `json:"gameModeParameters"`
	Features           []string       `json:"features"`
	MaxPlayers         int            `json:"maxPlayers"`
	MapID              string         `json:"mapId"`
	DevelopmentMode    bool           `json:"developmentMode"`
	DemoGame           bool           `json:"demoGame"`
	AllowRandomBuy     bool           `json:"allowRandomBuy"`
}

// GameOptionsDto provides authoritative defaults and map previews for setup.
type GameOptionsDto struct {
	GameModes     []GameModeDto `json:"gameModes"`
	Defaults      GameSetupDto  `json:"defaults"`
	AvailableMaps []MapInfoDto  `json:"availableMaps"`
}

// CreateGameRequest selects initial settings; omission uses server defaults.
type CreateGameRequest struct {
	Settings *GameSetupDto `json:"settings,omitempty"`
}

// UpdateGameSettingsRequest represents a partial settings update sent from the
// lobby. Pointer fields distinguish "leave alone" from "set to zero value".
type UpdateGameSettingsRequest struct {
	GameModeID         *string         `json:"gameModeId,omitempty"`
	GameModeParameters *map[string]int `json:"gameModeParameters,omitempty"`
	Features           *[]string       `json:"features,omitempty"`
	MaxPlayers         *int            `json:"maxPlayers,omitempty"`
	MapID              *string         `json:"mapId,omitempty"`
	DevelopmentMode    *bool           `json:"developmentMode,omitempty"`
	DemoGame           *bool           `json:"demoGame,omitempty"`
	AllowRandomBuy     *bool           `json:"allowRandomBuy,omitempty"`
	ClaudeOAuthToken   *string         `json:"claudeOAuthToken,omitempty"`
	BotSpendCapUSD     *float64        `json:"botSpendCapUsd,omitempty"`
}

// CreateGameResponse represents the response for creating a game
type CreateGameResponse struct {
	Game GameDto `json:"game"`
}

// JoinGameRequest represents the request body for joining a game
type JoinGameRequest struct {
	PlayerName string `json:"playerName"`
}

// JoinGameResponse represents the response for joining a game
type JoinGameResponse struct {
	Game     GameDto `json:"game"`
	PlayerID string  `json:"playerId"`
}

// GetGameResponse represents the response for getting a game
type GetGameResponse struct {
	Game GameDto `json:"game"`
}

// ListGamesResponse represents the response for listing games
type ListGamesResponse struct {
	Games []GameDto `json:"games"`
}

// GetPlayerResponse represents the response for getting a player
type GetPlayerResponse struct {
	Player PlayerDto `json:"player"`
}

// ListCardsResponse represents the response for listing cards with pagination
type ListCardsResponse struct {
	Cards      []CardDto `json:"cards"`
	TotalCount int       `json:"totalCount"`
	Offset     int       `json:"offset"`
	Limit      int       `json:"limit"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// MilestoneAwardItemDto represents a single milestone or award for selection
type MilestoneAwardItemDto struct {
	ID          string `json:"id" ts:"string"`
	Name        string `json:"name" ts:"string"`
	Description string `json:"description" ts:"string"`
}

// ListMilestonesAwardsResponse represents the response for listing milestones and awards
type ListMilestonesAwardsResponse struct {
	Milestones []MilestoneAwardItemDto `json:"milestones" ts:"MilestoneAwardItemDto[]"`
	Awards     []MilestoneAwardItemDto `json:"awards" ts:"MilestoneAwardItemDto[]"`
}

// FeedbackRequest represents the request body for submitting feedback
type FeedbackRequest struct {
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Tags        []string        `json:"tags"`
	Author      string          `json:"author,omitempty"`
	GameState   json.RawMessage `json:"gameState,omitempty"`
}

// FeedbackDto represents a feedback submission's current state
type FeedbackDto struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	StatusMessage string `json:"statusMessage"`
	IssueURL      string `json:"issueUrl,omitempty"`
}

// FeedbackResponse represents the response for feedback operations
type FeedbackResponse struct {
	Report FeedbackDto `json:"report"`
}

// FeedbackStatusResponse represents the response for feedback service availability
type FeedbackStatusResponse struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// MetaResponse identifies this server to a gateway that fronts several servers.
type MetaResponse struct {
	Alias   string `json:"alias"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ChangelogResponse lists the player release notes of every published version, newest first
type ChangelogResponse struct {
	Entries []ChangelogEntry `json:"entries"`
}

// ChangelogEntry is the release notes for one version
type ChangelogEntry struct {
	Version  string             `json:"version"`
	Intro    string             `json:"intro"`
	Sections []ChangelogSection `json:"sections"`
}

// ChangelogSection is one heading of a version's release notes and its bullets.
// Major update sections also carry an intro paragraph and may have an image.
type ChangelogSection struct {
	Title string          `json:"title"`
	Major bool            `json:"major"`
	Intro string          `json:"intro"`
	Image *ChangelogImage `json:"image,omitempty"`
	Items []string        `json:"items"`
}

// ChangelogImage is served from GET /api/v1/changelog/{version}/{file}
type ChangelogImage struct {
	File string `json:"file"`
	Alt  string `json:"alt"`
}
