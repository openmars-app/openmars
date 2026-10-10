package dto

import "time"

// GameHistoryEntryDto represents a single historical game state snapshot.
type GameHistoryEntryDto struct {
	ModeRuntime  *ModeRuntimeDto                 `json:"modeRuntime,omitempty"`
	Sequence     int64                           `json:"sequence"`
	Timestamp    time.Time                       `json:"timestamp"`
	Generation   int                             `json:"generation"`
	Phase        GamePhase                       `json:"phase"`
	Temperature  int                             `json:"temperature"`
	Oxygen       int                             `json:"oxygen"`
	Oceans       int                             `json:"oceans"`
	Venus        int                             `json:"venus"`
	ActionNumber int                             `json:"actionNumber"`
	Board        BoardDto                        `json:"board"`
	Players      map[string]GameHistoryPlayerDto `json:"players"`
	Milestones   []ClaimedMilestoneDto           `json:"milestones"`
	Awards       []FundedAwardDto                `json:"awards"`
	Settings     GameSettingsDto                 `json:"settings"`
}

// GameHistoryPlayerDto contains the player data needed for graphs and replay.
type GameHistoryPlayerDto struct {
	PermanentProduction ProductionDto                  `json:"permanentProduction"`
	DependencyStatuses  map[string]DependencyStatusDto `json:"dependencyStatuses"`
	ID                  string                         `json:"id"`
	Name                string                         `json:"name"`
	Color               string                         `json:"color"`
	TerraformRating     int                            `json:"terraformRating"`
	Credits             int                            `json:"credits"`
	Steel               int                            `json:"steel"`
	Titanium            int                            `json:"titanium"`
	Plants              int                            `json:"plants"`
	Energy              int                            `json:"energy"`
	Heat                int                            `json:"heat"`
	PlayedCardCount     int                            `json:"playedCardCount"`
	Production          ProductionDto                  `json:"production"`
	PlayedCardIDs       []string                       `json:"playedCardIds"`
	HandCardIDs         []string                       `json:"handCardIds"`
	CorporationID       string                         `json:"corporationId"`
	ResourceStorage     map[string]int                 `json:"resourceStorage"`
	TotalVP             int                            `json:"totalVP"`
}

// ClaimedMilestoneDto represents a claimed milestone in history.
type ClaimedMilestoneDto struct {
	Type     string `json:"type"`
	PlayerID string `json:"playerId"`
}

// FundedAwardDto represents a funded award in history.
type FundedAwardDto struct {
	Type     string `json:"type"`
	PlayerID string `json:"playerId"`
}

// GetGameHistoryResponse is the HTTP response for the game history endpoint.
type GetGameHistoryResponse struct {
	Entries []GameHistoryEntryDto `json:"entries"`
}
