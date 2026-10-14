package game

import (
	"context"
	"log/slog"
	"openmars/internal/events"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

func (g *Game) SetVPCardLookup(lookup VPCardLookup) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.vpCardLookup = lookup
	g.subscribeToVPEvents()
}

// RegisterCorporationVPGranter registers VP conditions from a corporation card.
func (g *Game) RegisterCorporationVPGranter(playerID string, corporationID string) {
	if g.vpCardLookup == nil {
		return
	}
	cardInfo, err := g.vpCardLookup.LookupVPCard(corporationID)
	if err != nil || len(cardInfo.VPConditions) == 0 {
		return
	}
	p, err := g.GetPlayer(playerID)
	if err != nil {
		return
	}
	granter := shared.VPGranter{
		CardID:       cardInfo.CardID,
		CardName:     cardInfo.CardName,
		Description:  cardInfo.Description,
		VPConditions: cardInfo.VPConditions,
	}
	p.VPGranters().Prepend(granter)
	g.recalculatePlayerVP(p)
}

func (g *Game) recalculatePlayerVP(p *player.Player) {
	if g.vpCardLookup == nil {
		return
	}
	ctx := &gameVPRecalculationContext{game: g}
	p.VPGranters().RecalculateAll(ctx)
}

func (g *Game) recalculateAllPlayersVP() {
	for _, p := range g.GetAllPlayers() {
		g.recalculatePlayerVP(p)
	}
}

func (g *Game) subscribeToVPEvents() {
	events.Subscribe(g.eventBus, func(e events.ModeStateChangedEvent) { g.recalculateAllPlayersVP() })
	events.Subscribe(g.eventBus, func(e events.ProductionChangedEvent) {
		p, err := g.GetPlayer(e.PlayerID)
		if err == nil {
			g.recalculatePlayerVP(p)
		}
	})

	events.Subscribe(g.eventBus, func(e events.CardPlayedEvent) {
		if g.vpCardLookup == nil {
			return
		}
		cardInfo, err := g.vpCardLookup.LookupVPCard(e.CardID)
		if err != nil {
			return
		}
		if len(cardInfo.VPConditions) == 0 {
			return
		}

		p, err := g.GetPlayer(e.PlayerID)
		if err != nil {
			return
		}

		granter := shared.VPGranter{
			CardID:       cardInfo.CardID,
			CardName:     cardInfo.CardName,
			Description:  cardInfo.Description,
			VPConditions: cardInfo.VPConditions,
		}
		p.VPGranters().Add(granter)
		g.recalculatePlayerVP(p)
	})

	events.Subscribe(g.eventBus, func(e events.ResourceStorageChangedEvent) {
		p, err := g.GetPlayer(e.PlayerID)
		if err != nil {
			return
		}
		g.recalculatePlayerVP(p)
	})

	events.Subscribe(g.eventBus, func(e events.TagPlayedEvent) {
		p, err := g.GetPlayer(e.PlayerID)
		if err != nil {
			return
		}
		g.recalculatePlayerVP(p)
	})

	events.Subscribe(g.eventBus, func(e events.TilePlacedEvent) {
		g.recalculateAllPlayersVP()
	})

	events.Subscribe(g.eventBus, func(e events.ColonyBuiltEvent) {
		g.recalculateAllPlayersVP()
	})

}

func (g *Game) subscribeToGenerationalEvents() {
	events.Subscribe(g.eventBus, func(e events.TerraformRatingChangedEvent) {
		if e.NewRating > e.OldRating {
			p, err := g.GetPlayer(e.PlayerID)
			if err != nil {
				return
			}
			p.GenerationalEvents().Increment(shared.GenerationalEventTRRaise)
		}
	})

	events.Subscribe(g.eventBus, func(e events.TilePlacedEvent) {
		p, err := g.GetPlayer(e.PlayerID)
		if err != nil {
			return
		}

		switch e.TileType {
		case "ocean":
			p.GenerationalEvents().Increment(shared.GenerationalEventOceanPlacement)
		case "city":
			p.GenerationalEvents().Increment(shared.GenerationalEventCityPlacement)
		case "greenery":
			p.GenerationalEvents().Increment(shared.GenerationalEventGreeneryPlacement)
		}
	})

	events.Subscribe(g.eventBus, func(e events.GenerationAdvancedEvent) {
		for _, p := range g.GetAllPlayers() {
			p.GenerationalEvents().Clear()
			// Clear temporary "generation-end" effects
			p.Effects().RemoveTemporaryEffects(shared.TemporaryGenerationEnd)
			// Also clear any "next-card" effects that weren't consumed
			p.Effects().RemoveTemporaryEffects(shared.TemporaryNextCard)
		}
	})
}

func (g *Game) subscribeToOceanSpaceEvents() {
	events.Subscribe(g.eventBus, func(e events.TilePlacedEvent) {
		if e.TileType == string(shared.ResourceOceanTile) || e.TileType == "ocean" {
			return
		}

		coords := shared.HexPosition{Q: e.Q, R: e.R, S: e.S}
		tile, err := g.board.GetTile(coords)
		if err != nil {
			return
		}
		if tile.Type != shared.ResourceOceanSpace {
			return
		}

		freeOceanSpaces := g.board.FreeOceanSpaces()
		gp := g.globalParameters
		oceansRemaining := gp.GetMaxOceans() - gp.Oceans()
		if freeOceanSpaces < oceansRemaining {
			gp.ReduceMaxOceans(gp.Oceans() + freeOceanSpaces)
		}
	})
}

func (g *Game) subscribeToGlobalParameterBonuses() {
	log := slog.Default()

	events.Subscribe(g.eventBus, func(e events.TemperatureChangedEvent) {
		if e.ChangedBy == "" {
			return
		}
		p, err := g.GetPlayer(e.ChangedBy)
		if err != nil {
			return
		}

		if e.OldValue < -24 && e.NewValue >= -24 {
			p.Resources().AddProduction(map[shared.ResourceType]int{
				shared.ResourceHeatProduction: 1,
			})
			g.AddTriggeredEffect(shared.TriggeredEffect{
				CardName:   "Temperature Bonus",
				PlayerID:   e.ChangedBy,
				SourceType: shared.SourceTypeGlobalBonus,
				CalculatedOutputs: []shared.CalculatedOutput{
					{ResourceType: string(shared.ResourceHeatProduction), Amount: 1},
				},
			})
			log.Debug("Temperature bonus: +1 heat production at -24C", slog.String("player_id", e.ChangedBy))
		}

		if e.OldValue < -20 && e.NewValue >= -20 {
			p.Resources().AddProduction(map[shared.ResourceType]int{
				shared.ResourceHeatProduction: 1,
			})
			g.AddTriggeredEffect(shared.TriggeredEffect{
				CardName:   "Temperature Bonus",
				PlayerID:   e.ChangedBy,
				SourceType: shared.SourceTypeGlobalBonus,
				CalculatedOutputs: []shared.CalculatedOutput{
					{ResourceType: string(shared.ResourceHeatProduction), Amount: 1},
				},
			})
			log.Debug("Temperature bonus: +1 heat production at -20C", slog.String("player_id", e.ChangedBy))
		}

		if e.OldValue < 0 && e.NewValue >= 0 {
			ctx := context.Background()
			if err := g.AppendToPendingTileSelectionQueue(ctx, e.ChangedBy, []string{"ocean"}, "Temperature Bonus", "", nil); err != nil {
				log.Warn("Failed to queue ocean tile for temperature bonus", slog.Any("error", err))
			}
			g.AddTriggeredEffect(shared.TriggeredEffect{
				CardName:   "Temperature Bonus",
				PlayerID:   e.ChangedBy,
				SourceType: shared.SourceTypeGlobalBonus,
				CalculatedOutputs: []shared.CalculatedOutput{
					{ResourceType: string(shared.ResourceOceanTile), Amount: 1},
				},
			})
			log.Debug("Temperature bonus: place ocean at 0C", slog.String("player_id", e.ChangedBy))
		}
	})

	events.Subscribe(g.eventBus, func(e events.OxygenChangedEvent) {
		if e.ChangedBy == "" {
			return
		}

		if e.OldValue < 8 && e.NewValue >= 8 {
			ctx := context.Background()
			actualSteps, err := g.globalParameters.IncreaseTemperature(ctx, 1, e.ChangedBy)
			if err != nil {
				slog.Default().Warn("Failed to increase temperature from oxygen bonus", slog.Any("error", err))
			}
			if actualSteps > 0 {
				p, pErr := g.GetPlayer(e.ChangedBy)
				if pErr == nil {
					p.Resources().UpdateTerraformRating(actualSteps)
				}
			}
			g.AddTriggeredEffect(shared.TriggeredEffect{
				CardName:   "Oxygen Bonus",
				PlayerID:   e.ChangedBy,
				SourceType: shared.SourceTypeGlobalBonus,
				CalculatedOutputs: []shared.CalculatedOutput{
					{ResourceType: string(shared.ResourceTemperature), Amount: 1},
				},
			})
			log.Debug("Oxygen bonus: +1 temperature step at 8%", slog.String("player_id", e.ChangedBy))
		}
	})

	events.Subscribe(g.eventBus, func(e events.VenusChangedEvent) {
		if e.ChangedBy == "" {
			return
		}
		p, err := g.GetPlayer(e.ChangedBy)
		if err != nil {
			return
		}

		if e.OldValue < 8 && e.NewValue >= 8 {
			ctx := context.Background()
			cardIDs, drawErr := g.deck.DrawProjectCards(ctx, 1)
			if drawErr == nil && len(cardIDs) > 0 {
				p.Hand().ReceiveCards(cardIDs, "Venus Bonus", "")
			}
			g.AddTriggeredEffect(shared.TriggeredEffect{
				CardName:   "Venus Bonus",
				PlayerID:   e.ChangedBy,
				SourceType: shared.SourceTypeGlobalBonus,
				CalculatedOutputs: []shared.CalculatedOutput{
					{ResourceType: "card-draw", Amount: 1},
				},
			})
			log.Debug("Venus bonus: draw 1 card at 8%", slog.String("player_id", e.ChangedBy))
		}

		if e.OldValue < 16 && e.NewValue >= 16 {
			p.Resources().UpdateTerraformRating(1)
			g.AddTriggeredEffect(shared.TriggeredEffect{
				CardName:   "Venus Bonus",
				PlayerID:   e.ChangedBy,
				SourceType: shared.SourceTypeGlobalBonus,
				CalculatedOutputs: []shared.CalculatedOutput{
					{ResourceType: string(shared.ResourceTR), Amount: 1},
				},
			})
			log.Debug("Venus bonus: +1 TR at 16%", slog.String("player_id", e.ChangedBy))
		}
	})
}
