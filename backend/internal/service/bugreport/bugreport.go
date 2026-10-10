package bugreport

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v92/github"
	"github.com/google/uuid"
)

// Config holds configuration for the bug report service.
type Config struct {
	GitHubAppID          int64
	GitHubInstallationID int64
	GitHubPrivateKeyPath string
	GitHubRepoOwner      string
	GitHubRepoName       string
}

// Report represents an in-progress or completed bug report.
type Report struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	StatusMessage string `json:"statusMessage"`
	IssueURL      string `json:"issueUrl,omitempty"`
}

// Capabilities tracks which external services are available.
type Capabilities struct {
	GitHubApp bool
}

// Service handles bug report submission via GitHub Issues.
type Service struct {
	logger       *slog.Logger
	config       Config
	ghClient     *github.Client
	capabilities Capabilities

	mu      sync.Mutex
	reports map[string]*Report
}

// NewService creates and initializes a bug report service.
func NewService(logger *slog.Logger) *Service {
	s := &Service{
		logger:  logger,
		reports: make(map[string]*Report),
	}
	s.initialize()
	return s
}

func (s *Service) initialize() {
	cfg := loadConfig()
	s.config = cfg
	s.capabilities.GitHubApp = s.initGitHub(cfg)
	s.logger.Debug("Bug report service initialized",
		slog.Bool("github_app", s.capabilities.GitHubApp))
}

func (s *Service) initGitHub(cfg Config) bool {
	if cfg.GitHubInstallationID == 0 {
		s.logger.Debug("Bug report: GitHub App disabled (GITHUB_INSTALLATION_ID not set)")
		return false
	}

	if _, err := os.Stat(cfg.GitHubPrivateKeyPath); os.IsNotExist(err) {
		s.logger.Debug("Bug report: GitHub App disabled (private key not found: " + cfg.GitHubPrivateKeyPath + ")")
		return false
	}

	itr, err := ghinstallation.NewKeyFromFile(
		http.DefaultTransport,
		cfg.GitHubAppID,
		cfg.GitHubInstallationID,
		cfg.GitHubPrivateKeyPath,
	)
	if err != nil {
		s.logger.Error("Bug report: GitHub App disabled (failed to create transport)", slog.Any("error", err))
		return false
	}

	s.ghClient, err = github.NewClient(github.WithTransport(itr))
	if err != nil {
		s.logger.Error("Bug report: GitHub App disabled (failed to create client)", slog.Any("error", err))
		return false
	}
	return true
}

// IsAvailable returns whether the bug report service can create issues.
func (s *Service) IsAvailable() bool {
	return s.capabilities.GitHubApp
}

// Capabilities returns which external services are available.
func (s *Service) Capabilities() Capabilities {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.capabilities
}

// GetReport returns a report by ID, or nil if not found.
func (s *Service) GetReport(id string) *Report {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.reports[id]
	if r == nil {
		return nil
	}
	copy := *r
	return &copy
}

func (s *Service) updateReport(id, status, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.reports[id]; r != nil {
		r.Status = status
		r.StatusMessage = message
	}
}

func (s *Service) completeReport(id, issueURL string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.reports[id]; r != nil {
		r.Status = "completed"
		r.StatusMessage = "Issue created"
		r.IssueURL = issueURL
	}
}

func (s *Service) failReport(id, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.reports[id]; r != nil {
		r.Status = "failed"
		r.StatusMessage = message
	}
}

// SubmitBugReport starts async processing of a bug report and returns the report ID.
func (s *Service) SubmitBugReport(title string, description string, tags []string, author string, gameState json.RawMessage) string {
	id := uuid.New().String()

	s.mu.Lock()
	s.reports[id] = &Report{
		ID:            id,
		Status:        "processing",
		StatusMessage: "Submitting feedback...",
	}
	s.mu.Unlock()

	go s.processBugReport(id, title, description, tags, author, gameState)

	return id
}

func (s *Service) processBugReport(id string, title string, description string, tags []string, author string, gameState json.RawMessage) {
	s.updateReport(id, "processing", "Creating GitHub issue...")

	body := buildIssueBody(description, author, gameState)
	labels := buildLabels(tags)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	issue, _, err := s.ghClient.Issues.Create(ctx, s.config.GitHubRepoOwner, s.config.GitHubRepoName, github.CreateIssueRequest{
		Title:  title,
		Body:   github.Ptr(body),
		Labels: labels,
	})
	if err != nil {
		s.failReport(id, "Failed to create GitHub issue")
		s.logger.Error("Failed to create GitHub issue", slog.Any("error", err))
		return
	}

	s.completeReport(id, issue.GetHTMLURL())
	s.logger.Debug("Feedback submitted", slog.String("issue_url", issue.GetHTMLURL()))
}

func buildLabels(tags []string) []string {
	labels := []string{"in-game"}
	for _, tag := range tags {
		switch tag {
		case "bug":
			labels = append(labels, "bug")
		case "feature-request":
			labels = append(labels, "enhancement")
		}
	}
	return labels
}

func loadConfig() Config {
	cfg := Config{
		GitHubPrivateKeyPath: "./private-key.pem",
		GitHubRepoOwner:      "openmars-app",
		GitHubRepoName:       "openmars",
	}

	if v := os.Getenv("GITHUB_APP_ID"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.GitHubAppID = n
		}
	}
	if v := os.Getenv("GITHUB_INSTALLATION_ID"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.GitHubInstallationID = n
		}
	}
	if v := os.Getenv("GITHUB_PRIVATE_KEY_PATH"); v != "" {
		cfg.GitHubPrivateKeyPath = v
	}
	if v := os.Getenv("GITHUB_REPO_OWNER"); v != "" {
		cfg.GitHubRepoOwner = v
	}
	if v := os.Getenv("GITHUB_REPO_NAME"); v != "" {
		cfg.GitHubRepoName = v
	}

	return cfg
}

func buildIssueBody(description, author string, gameState json.RawMessage) string {
	var b strings.Builder

	b.WriteString("## Description\n\n")
	b.WriteString(description)
	b.WriteString("\n\n")

	b.WriteString("## Submitted by\n\n")
	if author != "" {
		b.WriteString(author)
	} else {
		b.WriteString("_Unknown_")
	}
	b.WriteString("\n\n")

	b.WriteString("## Game Context\n\n")
	b.WriteString(buildGameMeta(gameState))
	b.WriteString("\n")

	return b.String()
}

func buildGameMeta(gameState json.RawMessage) string {
	if len(gameState) == 0 {
		return "_No game state available_"
	}

	var game struct {
		ID               string  `json:"id"`
		Status           string  `json:"status"`
		CurrentPhase     string  `json:"currentPhase"`
		Generation       int     `json:"generation"`
		CurrentTurn      *string `json:"currentTurn"`
		GlobalParameters struct {
			Temperature int `json:"temperature"`
			Oxygen      int `json:"oxygen"`
			Oceans      int `json:"oceans"`
			MaxOceans   int `json:"maxOceans"`
			Venus       int `json:"venus"`
		} `json:"globalParameters"`
		Settings struct {
			MaxPlayers int      `json:"maxPlayers"`
			GameModeID string   `json:"gameModeId"`
			Features   []string `json:"features"`
		} `json:"settings"`
		CurrentPlayer *struct {
			ID              string `json:"id"`
			Name            string `json:"name"`
			TerraformRating int    `json:"terraformRating"`
			Corporation     *struct {
				Name string `json:"name"`
			} `json:"corporation"`
			Resources struct {
				Credits  int `json:"credits"`
				Steel    int `json:"steel"`
				Titanium int `json:"titanium"`
				Plants   int `json:"plants"`
				Energy   int `json:"energy"`
				Heat     int `json:"heat"`
			} `json:"resources"`
			Production struct {
				Credits  int `json:"credits"`
				Steel    int `json:"steel"`
				Titanium int `json:"titanium"`
				Plants   int `json:"plants"`
				Energy   int `json:"energy"`
				Heat     int `json:"heat"`
			} `json:"production"`
			PlayedCards []struct {
				Name string `json:"name"`
			} `json:"playedCards"`
			Cards []struct{} `json:"cards"`
		} `json:"currentPlayer"`
		OtherPlayers []struct {
			ID              string `json:"id"`
			Name            string `json:"name"`
			TerraformRating int    `json:"terraformRating"`
			Corporation     *struct {
				Name string `json:"name"`
			} `json:"corporation"`
			Resources struct {
				Credits  int `json:"credits"`
				Steel    int `json:"steel"`
				Titanium int `json:"titanium"`
				Plants   int `json:"plants"`
				Energy   int `json:"energy"`
				Heat     int `json:"heat"`
			} `json:"resources"`
			PlayedCards []struct {
				Name string `json:"name"`
			} `json:"playedCards"`
			HandCardCount int `json:"handCardCount"`
		} `json:"otherPlayers"`
		Milestones []struct {
			Name      string  `json:"name"`
			ClaimedBy *string `json:"claimedBy"`
		} `json:"milestones"`
		Awards []struct {
			Name     string  `json:"name"`
			FundedBy *string `json:"fundedBy"`
		} `json:"awards"`
	}

	if err := json.Unmarshal(gameState, &game); err != nil {
		return "_Failed to parse game state_"
	}

	var b strings.Builder

	b.WriteString("| Property | Value |\n")
	b.WriteString("|----------|-------|\n")
	b.WriteString("| Game ID | `" + game.ID + "` |\n")
	b.WriteString("| Status | " + game.Status + " |\n")
	b.WriteString("| Phase | " + game.CurrentPhase + " |\n")
	b.WriteString("| Generation | " + strconv.Itoa(game.Generation) + " |\n")
	b.WriteString("| Temperature | " + strconv.Itoa(game.GlobalParameters.Temperature) + "°C |\n")
	b.WriteString("| Oxygen | " + strconv.Itoa(game.GlobalParameters.Oxygen) + "% |\n")
	b.WriteString("| Oceans | " + strconv.Itoa(game.GlobalParameters.Oceans) + "/" + strconv.Itoa(game.GlobalParameters.MaxOceans) + " |\n")
	b.WriteString("| Venus | " + strconv.Itoa(game.GlobalParameters.Venus) + "% |\n")

	if game.Settings.GameModeID != "" {
		b.WriteString("| Game mode | " + game.Settings.GameModeID + " (" + strings.Join(game.Settings.Features, ", ") + ")" + " |\n")
	}

	var claimedMilestones []string
	for _, m := range game.Milestones {
		if m.ClaimedBy != nil {
			claimedMilestones = append(claimedMilestones, m.Name+" ("+*m.ClaimedBy+")")
		}
	}
	if len(claimedMilestones) > 0 {
		b.WriteString("| Milestones | " + strings.Join(claimedMilestones, ", ") + " |\n")
	}

	var fundedAwards []string
	for _, a := range game.Awards {
		if a.FundedBy != nil {
			fundedAwards = append(fundedAwards, a.Name+" ("+*a.FundedBy+")")
		}
	}
	if len(fundedAwards) > 0 {
		b.WriteString("| Awards | " + strings.Join(fundedAwards, ", ") + " |\n")
	}

	b.WriteString("\n#### Players\n\n")

	if game.CurrentPlayer != nil {
		p := game.CurrentPlayer
		corp := "None"
		if p.Corporation != nil {
			corp = p.Corporation.Name
		}
		b.WriteString("**" + p.Name + "** (reporting player)\n")
		b.WriteString("- Corporation: " + corp + "\n")
		b.WriteString("- TR: " + strconv.Itoa(p.TerraformRating) + "\n")
		b.WriteString("- Credits: " + strconv.Itoa(p.Resources.Credits) +
			", Steel: " + strconv.Itoa(p.Resources.Steel) +
			", Titanium: " + strconv.Itoa(p.Resources.Titanium) +
			", Plants: " + strconv.Itoa(p.Resources.Plants) +
			", Energy: " + strconv.Itoa(p.Resources.Energy) +
			", Heat: " + strconv.Itoa(p.Resources.Heat) + "\n")
		b.WriteString("- Production: " +
			strconv.Itoa(p.Production.Credits) + "M€, " +
			strconv.Itoa(p.Production.Steel) + " steel, " +
			strconv.Itoa(p.Production.Titanium) + " titanium, " +
			strconv.Itoa(p.Production.Plants) + " plants, " +
			strconv.Itoa(p.Production.Energy) + " energy, " +
			strconv.Itoa(p.Production.Heat) + " heat\n")
		b.WriteString("- Cards in hand: " + strconv.Itoa(len(p.Cards)) + "\n")
		b.WriteString("- Cards played: " + strconv.Itoa(len(p.PlayedCards)) + "\n")
		if len(p.PlayedCards) > 0 {
			var cardNames []string
			for _, c := range p.PlayedCards {
				cardNames = append(cardNames, c.Name)
			}
			b.WriteString("- Played: " + strings.Join(cardNames, ", ") + "\n")
		}
		b.WriteString("\n")
	}

	for _, p := range game.OtherPlayers {
		corp := "None"
		if p.Corporation != nil {
			corp = p.Corporation.Name
		}
		b.WriteString("**" + p.Name + "**\n")
		b.WriteString("- Corporation: " + corp + "\n")
		b.WriteString("- TR: " + strconv.Itoa(p.TerraformRating) + "\n")
		b.WriteString("- Credits: " + strconv.Itoa(p.Resources.Credits) +
			", Steel: " + strconv.Itoa(p.Resources.Steel) +
			", Titanium: " + strconv.Itoa(p.Resources.Titanium) +
			", Plants: " + strconv.Itoa(p.Resources.Plants) +
			", Energy: " + strconv.Itoa(p.Resources.Energy) +
			", Heat: " + strconv.Itoa(p.Resources.Heat) + "\n")
		b.WriteString("- Cards in hand: " + strconv.Itoa(p.HandCardCount) + "\n")
		b.WriteString("- Cards played: " + strconv.Itoa(len(p.PlayedCards)) + "\n")
		if len(p.PlayedCards) > 0 {
			var cardNames []string
			for _, c := range p.PlayedCards {
				cardNames = append(cardNames, c.Name)
			}
			b.WriteString("- Played: " + strings.Join(cardNames, ", ") + "\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}
