package game

import (
	"context"
	"fmt"
	"log/slog"

	internalgame "openmars/internal/game"
	"openmars/internal/game/cards"
	"openmars/internal/game/shared"
)

// DemoGlobalParameters is the host-only global-parameter override for a demo game.
type DemoGlobalParameters struct {
	Temperature int
	Oxygen      int
	Oceans      int
}

// DemoChoices is the domain-level, transport-agnostic demo lobby selection. The
// delivery layer maps its request DTO onto this so the action does not depend on
// delivery/dto.
type DemoChoices struct {
	CorporationID    string
	PreludeIDs       []string
	CardIDs          []string
	Resources        shared.Resources
	Production       shared.Production
	TerraformRating  int
	GlobalParameters *DemoGlobalParameters // host only
	Generation       *int                  // host only
}

// SelectDemoChoicesAction handles a player selecting cards during the demo lobby phase
type SelectDemoChoicesAction struct {
	gameRepo     internalgame.GameRepository
	cardRegistry cards.CardRegistry
	logger       *slog.Logger
}

// NewSelectDemoChoicesAction creates a new select demo choices action
func NewSelectDemoChoicesAction(
	gameRepo internalgame.GameRepository,
	cardRegistry cards.CardRegistry,
	logger *slog.Logger,
) *SelectDemoChoicesAction {
	return &SelectDemoChoicesAction{
		gameRepo:     gameRepo,
		cardRegistry: cardRegistry,
		logger:       logger,
	}
}

// Execute validates and stores a player's demo lobby card selections
func (a *SelectDemoChoicesAction) Execute(
	ctx context.Context,
	gameID string,
	playerID string,
	request *DemoChoices,
) error {
	log := a.logger.With(
		slog.String("game_id", gameID),
		slog.String("player_id", playerID),
		slog.String("action", "select_demo_choices"),
	)
	log.Debug("Player selecting demo choices")

	g, err := a.gameRepo.Get(ctx, gameID)
	if err != nil {
		return fmt.Errorf("game not found: %s", gameID)
	}

	if g.Status() != shared.GameStatusLobby {
		return fmt.Errorf("game is not in lobby: %s", g.Status())
	}

	if !g.Settings().DemoGame {
		return fmt.Errorf("game is not a demo game")
	}

	p, err := g.GetPlayer(playerID)
	if err != nil {
		return fmt.Errorf("player not found: %s", playerID)
	}

	if request.CorporationID == "" {
		return fmt.Errorf("corporation ID is required")
	}
	corpCard, err := cards.ForGame(a.cardRegistry, g).GetByID(request.CorporationID)
	if err != nil {
		return fmt.Errorf("corporation not found: %s", request.CorporationID)
	}
	if corpCard.Type != "corporation" {
		return fmt.Errorf("card %s is not a corporation", request.CorporationID)
	}

	settings := g.Settings()
	if settings.HasPrelude() {
		if len(request.PreludeIDs) != 2 {
			return fmt.Errorf("must select exactly 2 prelude cards, got %d", len(request.PreludeIDs))
		}
		for _, id := range request.PreludeIDs {
			card, err := cards.ForGame(a.cardRegistry, g).GetByID(id)
			if err != nil {
				return fmt.Errorf("prelude card not found: %s", id)
			}
			if card.Type != "prelude" {
				return fmt.Errorf("card %s is not a prelude", id)
			}
		}
	} else if len(request.PreludeIDs) > 0 {
		return fmt.Errorf("prelude cards not enabled for this game")
	}

	for _, id := range request.CardIDs {
		card, err := cards.ForGame(a.cardRegistry, g).GetByID(id)
		if err != nil {
			return fmt.Errorf("card not found: %s", id)
		}
		if card.Type == "corporation" || card.Type == "prelude" {
			return fmt.Errorf("card %s is a %s, not a project card", id, card.Type)
		}
	}

	p.SetPendingDemoChoices(&shared.PendingDemoChoices{
		CorporationID:   request.CorporationID,
		PreludeIDs:      request.PreludeIDs,
		CardIDs:         request.CardIDs,
		Resources:       request.Resources,
		Production:      request.Production,
		TerraformRating: request.TerraformRating,
	})

	isHost := g.HostPlayerID() == playerID
	if isHost && request.GlobalParameters != nil {
		settings.Temperature = &request.GlobalParameters.Temperature
		settings.Oxygen = &request.GlobalParameters.Oxygen
		settings.Oceans = &request.GlobalParameters.Oceans
	}
	if isHost && request.Generation != nil {
		settings.Generation = request.Generation
	}
	if isHost && (request.GlobalParameters != nil || request.Generation != nil) {
		g.UpdateSettings(ctx, settings)
	}

	log.Info("Demo choices selected",
		slog.String("corporation", corpCard.Name),
		slog.Int("prelude_count", len(request.PreludeIDs)),
		slog.Int("card_count", len(request.CardIDs)))

	return nil
}
