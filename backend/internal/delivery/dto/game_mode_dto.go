package dto

import "openmars/internal/game/mode"

type GameModeRuleDto struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}
type GameModeParameterDto struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Default     int    `json:"default"`
	Values      []int  `json:"values"`
}
type GameModeFeatureDto struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Experimental bool   `json:"experimental"`
	Default      bool   `json:"default"`
}
type GameModeStatusDto struct {
	State  string            `json:"state"`
	Label  string            `json:"label"`
	Labels map[string]string `json:"labels,omitempty"`
	Kind   string            `json:"kind" tstype:"'connection' | 'counter'"`
	From   string            `json:"from,omitempty"`
	To     string            `json:"to,omitempty"`
}
type GameModeDto struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Summary    string                 `json:"summary"`
	Scenario   string                 `json:"scenario"`
	Rules      []GameModeRuleDto      `json:"rules"`
	MinPlayers int                    `json:"minPlayers"`
	MaxPlayers int                    `json:"maxPlayers"`
	Parameters []GameModeParameterDto `json:"parameters"`
	Features   []GameModeFeatureDto   `json:"features"`
	Systems    []string               `json:"systems"`
	Statuses   []GameModeStatusDto    `json:"statuses"`
}
type GameModesResponse struct {
	GameModes []GameModeDto `json:"gameModes"`
}

func MapGameModes(definitions []mode.Definition) []GameModeDto {
	result := make([]GameModeDto, 0, len(definitions))
	for _, d := range definitions {
		out := GameModeDto{ID: d.ID, Name: d.Name, Summary: d.Summary, Scenario: d.Scenario, MinPlayers: d.MinPlayers, MaxPlayers: d.MaxPlayers, Systems: append([]string{}, d.Systems...), Rules: []GameModeRuleDto{}, Parameters: []GameModeParameterDto{}, Features: []GameModeFeatureDto{}, Statuses: []GameModeStatusDto{}}
		for _, r := range d.Rules {
			out.Rules = append(out.Rules, GameModeRuleDto{Title: r.Title, Text: r.Text})
		}
		for _, p := range d.Parameters {
			out.Parameters = append(out.Parameters, GameModeParameterDto{ID: p.ID, Label: p.Label, Description: p.Description, Default: p.Default, Values: p.Values})
		}
		for _, f := range d.Features {
			out.Features = append(out.Features, GameModeFeatureDto{ID: f.ID, Name: f.Name, Description: f.Description, Experimental: f.Experimental, Default: f.Default})
		}
		for _, s := range d.Statuses {
			out.Statuses = append(out.Statuses, GameModeStatusDto{State: s.State, Label: s.Label, Labels: s.Labels, Kind: s.Kind, From: s.From, To: s.To})
		}
		result = append(result, out)
	}
	return result
}
