package bot

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"openmars/internal/delivery/dto"
	"openmars/internal/game"
	"openmars/internal/game/shared"
)

// SummarizeGameState produces a human-readable text summary of the game state.
func SummarizeGameState(game *dto.GameDto, myPlayerID string) string {
	if game == nil {
		return "No game state available."
	}

	var lines []string

	lines = append(lines, formatGameInfo(game, myPlayerID))
	lines = append(lines, formatGlobalParams(&game.GlobalParameters))

	p := &game.CurrentPlayer
	lines = append(lines, formatPendingActions(p))
	lines = append(lines, formatPlayerStatus(p))
	lines = append(lines, formatHand(p.Cards))
	lines = append(lines, formatCardActions(p.Actions))
	lines = append(lines, formatStandardProjects(p.StandardProjects))
	lines = append(lines, formatMilestones(game, p.Milestones))
	lines = append(lines, formatAwards(game, p.Awards))
	lines = append(lines, formatColonies(game, myPlayerID))
	lines = append(lines, formatProjectFunding(game.ProjectFunding))
	lines = append(lines, formatOpponents(game.OtherPlayers))
	lines = append(lines, formatBoard(game))

	if len(game.FinalScores) > 0 {
		lines = append(lines, formatFinalScores(game))
	}

	var filtered []string
	for _, l := range lines {
		if l != "" {
			filtered = append(filtered, l)
		}
	}
	return strings.Join(filtered, "\n\n")
}

func formatGameInfo(game *dto.GameDto, myPlayerID string) string {
	turnInfo := "N/A"
	if game.CurrentTurn != nil {
		if *game.CurrentTurn == myPlayerID {
			turnInfo = "YOUR TURN"
		} else {
			turnInfo = fmt.Sprintf("Waiting for %s", findPlayerName(game, *game.CurrentTurn))
		}
	}

	return strings.Join([]string{
		"=== GAME INFO ===",
		fmt.Sprintf("Game ID: %s", game.ID),
		fmt.Sprintf("Phase: %s", string(game.CurrentPhase)),
		fmt.Sprintf("Status: %s", string(game.Status)),
		fmt.Sprintf("Generation: %d", game.Generation),
		fmt.Sprintf("Turn: %s", turnInfo),
		fmt.Sprintf("Players: %d", len(game.TurnOrder)),
		fmt.Sprintf("Turn order: %s", formatTurnOrder(game)),
	}, "\n")
}

func formatTurnOrder(game *dto.GameDto) string {
	var names []string
	for _, id := range game.TurnOrder {
		names = append(names, findPlayerName(game, id))
	}
	return strings.Join(names, " -> ")
}

func formatGlobalParams(gp *dto.GlobalParametersDto) string {
	return strings.Join([]string{
		"=== GLOBAL PARAMETERS ===",
		fmt.Sprintf("Temperature: %d°C (target: 8°C)", gp.Temperature),
		fmt.Sprintf("Oxygen: %d%% (target: 14%%)", gp.Oxygen),
		fmt.Sprintf("Oceans: %d/%d", gp.Oceans, gp.MaxOceans),
		fmt.Sprintf("Venus: %d%% (target: 30%%)", gp.Venus),
	}, "\n")
}

func formatPendingActions(player *dto.PlayerDto) string {
	var parts []string

	if player.PendingTileSelection != nil {
		parts = append(parts, formatPendingTile(player.PendingTileSelection))
	}
	if player.PendingCardSelection != nil {
		parts = append(parts, formatPendingCardSelection(player.PendingCardSelection))
	}
	if player.PendingCardPlaySelection != nil {
		parts = append(parts, fmt.Sprintf("Play one card from hand now with %d M€ cost reduction using play_card.", player.PendingCardPlaySelection.CostReduction))
	}
	if player.PendingCardDrawSelection != nil {
		parts = append(parts, formatPendingCardDraw(player.PendingCardDrawSelection))
	}
	for i := range player.PendingBehaviorResolutions {
		resolution := &player.PendingBehaviorResolutions[i]
		if resolution.Kind == "optional" {
			data, _ := json.Marshal(resolution.OptionalEffect)
			parts = append(parts, fmt.Sprintf("Optional effect from %s (resolutionId=%s): %s. Use confirm_optional_behavior with accept=true to pay or accept=false to skip.", resolution.Source, resolution.ID, data))
		} else if resolution.Kind == "card-discard" {
			parts = append(parts, formatPendingCardDiscard(resolution))
		} else {
			parts = append(parts, formatPendingBehaviorChoice(resolution))
		}
	}
	if reveal := player.PendingCardReveal; reveal != nil {
		parts = append(parts, fmt.Sprintf("Public reveal from %s: %v. Rewards: %v. Acknowledge with confirm_card_reveal.", reveal.Source, reveal.Results, reveal.Rewards))
	}
	if selection := player.PendingResourceRemovalSelection; selection != nil {
		data, _ := json.Marshal(selection)
		parts = append(parts, "Pending resource removal: "+string(data)+". Use confirm_resource_removal with selectionId, targetPlayerId and amount; skip with empty target and zero amount.")
	}
	if selection := player.PendingEffectSelection; selection != nil {
		parts = append(parts, "Pending effect: "+selection.Source+". Confirm with confirm_effect_selection and optionIndex.")
		for i, option := range selection.Options {
			parts = append(parts, fmt.Sprintf("Option %d: card=%s target=%s colonies=%v outputs=%v", i, option.CardID, option.TargetPlayerID, option.ColonyIDs, option.Outputs))
		}
	}

	if pending := player.PendingColonySelection; pending != nil {
		purpose := "Build colony"
		if pending.AddTile {
			purpose = "Add an unused colony tile"
		}
		parts = append(parts, fmt.Sprintf("%s: choose %v with confirm_colony_placement and colonyId.", purpose, pending.AvailableColonyIDs))
	}
	if pending := player.PendingAwardFundSelection; pending != nil {
		parts = append(parts, fmt.Sprintf("Fund an award for free: choose %v with confirm_award_fund and awardType.", pending.AvailableAwards))
	}
	if pending := player.PendingColonyResourceSelection; pending != nil {
		parts = append(parts, fmt.Sprintf("Place %d %s on an eligible owned card using confirm_colony_resource and cardId (empty to skip).", pending.Amount, pending.ResourceType))
	}
	if pending := player.PendingFreeTradeSelection; pending != nil {
		parts = append(parts, fmt.Sprintf("FREE TRADE from %s: choose a colony from %v and a track option with confirm_free_trade (colonyId, trackSteps). See COLONIES for each colony's trade options.", pending.Source, pending.AvailableColonyIDs))
	}
	if player.ForcedFirstAction != nil && player.ForcedFirstAction.State == "resolving" {
		parts = append(parts, formatForcedAction(player.ForcedFirstAction))
	}
	if player.SelectCorporationPhase != nil || player.SelectStartingCardsPhase != nil || player.SelectPreludeCardsPhase != nil {
		parts = append(parts, formatStartingSelection(player))
	}
	if player.ProductionPhase != nil && !player.ProductionPhase.SelectionComplete {
		parts = append(parts, formatProductionPhase(player))
	}

	if len(parts) == 0 {
		return ""
	}

	return ">>> PENDING ACTIONS (must resolve) <<<\n" + strings.Join(parts, "\n\n")
}

func formatPendingTile(sel *dto.PendingTileSelectionDto) string {
	return strings.Join([]string{
		fmt.Sprintf("TILE PLACEMENT REQUIRED: Place a %s tile", sel.TileType),
		fmt.Sprintf("Source: %s", sel.Source),
		fmt.Sprintf("Available hexes: %s", strings.Join(sel.AvailableHexes, ", ")),
		"Use select_tile with one of these hexes.",
	}, "\n")
}

func formatPendingCardSelection(sel *dto.PendingCardSelectionDto) string {
	var cardLines []string
	for _, c := range sel.AvailableCards {
		cost := sel.CardCosts[c.ID]
		reward := sel.CardRewards[c.ID]
		info := ""
		if cost > 0 {
			info = fmt.Sprintf(" (cost: %dM€)", cost)
		} else if reward > 0 {
			info = fmt.Sprintf(" (reward: %dM€)", reward)
		}
		cardLines = append(cardLines, fmt.Sprintf("  - %s [%s]%s", c.Name, c.ID, info))
	}

	return strings.Join([]string{
		fmt.Sprintf("CARD SELECTION REQUIRED (source: %s)", sel.Source),
		fmt.Sprintf("Select %d-%d cards:", sel.MinCards, sel.MaxCards),
		strings.Join(cardLines, "\n"),
		`Use confirm_sell_patents with cardIds.`,
	}, "\n")
}

// formatCardDescription flattens a card description for a bullet line, indenting continuation lines
func formatCardDescription(sections []dto.CardDescriptionSectionDto) string {
	return strings.ReplaceAll(dto.CardDescriptionPlainText(sections), "\n", "\n    ")
}

func formatPendingCardDraw(sel *dto.PendingCardDrawSelectionDto) string {
	var cardLines []string
	for _, c := range sel.AvailableCards {
		cardLines = append(cardLines, fmt.Sprintf("  - %s [%s]: %s", c.Name, c.ID, formatCardDescription(c.Description)))
	}

	buyCostStr := ""
	if sel.CardBuyCost > 0 {
		buyCostStr = fmt.Sprintf(" (%dM€ each)", sel.CardBuyCost)
	}

	return strings.Join([]string{
		fmt.Sprintf("CARD DRAW SELECTION (source: %s)", sel.Source),
		fmt.Sprintf("Free takes: %d to %d, Max buy: %d%s", sel.MinFreeTakeCount, sel.FreeTakeCount, sel.MaxBuyCount, buyCostStr),
		strings.Join(cardLines, "\n"),
		`Use confirm_card_draw with cardsToTake and cardsToBuy.`,
	}, "\n")
}

func formatPendingCardDiscard(sel *dto.PendingBehaviorResolutionDto) string {
	return strings.Join([]string{
		fmt.Sprintf("CARD DISCARD REQUIRED (source: %s)", sel.Source),
		fmt.Sprintf("Discard %d-%d cards from hand.", sel.MinCards, sel.MaxCards),
		fmt.Sprintf("Use confirm_card_discard with resolutionId %q and cardsToDiscard.", sel.ID),
	}, "\n")
}

func formatPendingBehaviorChoice(sel *dto.PendingBehaviorResolutionDto) string {
	var choiceLines []string
	for i, c := range sel.Choices {
		desc := formatBehaviorBrief(c.Inputs, c.Outputs)
		avail := ""
		if !c.Available {
			avail = " [UNAVAILABLE]"
		}
		choiceLines = append(choiceLines, fmt.Sprintf("  %d: %s%s", i, desc, avail))
	}

	return strings.Join([]string{
		fmt.Sprintf("BEHAVIOR CHOICE REQUIRED (source: %s)", sel.Source),
		strings.Join(choiceLines, "\n"),
		fmt.Sprintf("Use confirm_behavior_choice with resolutionId %q and choiceIndex. Triggering card: %s. Fixed triggering-card destinations cannot be overridden.", sel.ID, sel.TriggeringCardName),
	}, "\n")
}

func formatForcedAction(fa *dto.ForcedFirstActionDto) string {
	return strings.Join([]string{
		fmt.Sprintf("FORCED FIRST ACTION: %s", fa.Description),
		fmt.Sprintf("State: %s", fa.State),
		fmt.Sprintf("Corporation: %s", fa.CorporationID),
	}, "\n")
}

func formatStartingSelection(player *dto.PlayerDto) string {
	parts := []string{"STARTING SELECTION REQUIRED"}

	if player.SelectCorporationPhase != nil {
		var corps []string
		for _, c := range player.SelectCorporationPhase.AvailableCorporations {
			corps = append(corps, fmt.Sprintf("  - %s [%s]: %s", c.Name, c.ID, formatCardDescription(c.Description)))
		}
		parts = append(parts, "Corporations:\n"+strings.Join(corps, "\n"))
	}

	if player.SelectPreludeCardsPhase != nil {
		var preludes []string
		for _, c := range player.SelectPreludeCardsPhase.AvailablePreludes {
			preludes = append(preludes, fmt.Sprintf("  - %s [%s]: %s", c.Name, c.ID, formatCardDescription(c.Description)))
		}
		parts = append(parts, fmt.Sprintf("Preludes (pick %d):\n%s",
			player.SelectPreludeCardsPhase.MaxSelectable,
			strings.Join(preludes, "\n")))
	}

	if player.SelectStartingCardsPhase != nil {
		var cards []string
		for _, c := range player.SelectStartingCardsPhase.AvailableCards {
			tags := ""
			if len(c.Tags) > 0 {
				tagStrs := make([]string, len(c.Tags))
				for i, t := range c.Tags {
					tagStrs[i] = string(t)
				}
				tags = " " + strings.Join(tagStrs, ", ")
			}
			cards = append(cards, fmt.Sprintf("  - %s [%s] (%dM€) [%s]%s: %s",
				c.Name, c.ID, c.Cost, string(c.Type), tags, formatCardDescription(c.Description)))
		}
		parts = append(parts, "Starting cards (pick any to buy at 3M€ each):\n"+strings.Join(cards, "\n"))
	}

	parts = append(parts, "Use select_starting_choices with corporationId, preludeIds, and cardIds.")

	return strings.Join(parts, "\n")
}

func formatProductionPhase(player *dto.PlayerDto) string {
	pp := player.ProductionPhase
	var cards []string
	for _, c := range pp.AvailableCards {
		cards = append(cards, fmt.Sprintf("  - %s [%s] (%dM€): %s", c.Name, c.ID, c.Cost, formatCardDescription(c.Description)))
	}

	cardList := "  (no cards available)"
	if len(cards) > 0 {
		cardList = strings.Join(cards, "\n")
	}

	return strings.Join([]string{
		"PRODUCTION PHASE - Select cards to buy:",
		cardList,
		`Use confirm_production_cards with cardIds.`,
	}, "\n")
}

func formatPlayerStatus(player *dto.PlayerDto) string {
	r := &player.Resources
	p := &player.Production

	corpName := "None"
	if player.Corporation != nil {
		corpName = player.Corporation.Name
	}

	lines := []string{
		"=== YOUR STATUS ===",
		fmt.Sprintf("Name: %s | Corporation: %s | TR: %d", player.Name, corpName, player.TerraformRating),
		fmt.Sprintf("Status: %s | Actions remaining: %d | Passed: %v", string(player.Status), player.AvailableActions, player.Passed),
		"",
		"Resources (amount / production):",
		fmt.Sprintf("  Credits:  %d / %s", r.Credits, formatProd(p.Credits)),
		fmt.Sprintf("  Steel:    %d / %s", r.Steel, formatProd(p.Steel)),
		fmt.Sprintf("  Titanium: %d / %s", r.Titanium, formatProd(p.Titanium)),
		fmt.Sprintf("  Plants:   %d / %s", r.Plants, formatProd(p.Plants)),
		fmt.Sprintf("  Energy:   %d / %s", r.Energy, formatProd(p.Energy)),
		fmt.Sprintf("  Heat:     %d / %s", r.Heat, formatProd(p.Heat)),
	}

	lines = append(lines, "", "Legal resource removal targets (subject to the card's target restrictions):")
	for _, target := range player.ResourceRemovalTargets {
		lines = append(lines, fmt.Sprintf("  player=%s card=%s resource=%s amount=%d", target.PlayerID, target.CardID, target.ResourceType, target.Amount))
	}
	if len(player.PlayedCards) > 0 {
		var names []string
		for _, c := range player.PlayedCards {
			names = append(names, c.Name)
		}
		lines = append(lines, "", fmt.Sprintf("Played cards (%d): %s", len(player.PlayedCards), strings.Join(names, ", ")))
	}

	if len(player.ResourceStorage) > 0 {
		var storage []string
		for k, v := range player.ResourceStorage {
			if v > 0 {
				storage = append(storage, fmt.Sprintf("%s: %d", k, v))
			}
		}
		if len(storage) > 0 {
			lines = append(lines, fmt.Sprintf("Resource storage: %s", strings.Join(storage, ", ")))
		}
	}

	if len(player.PaymentSubstitutes) > 0 {
		var subs []string
		for _, s := range player.PaymentSubstitutes {
			subs = append(subs, fmt.Sprintf("%s (%d:1)", string(s.Source.Resource), s.ConversionRate))
		}
		lines = append(lines, fmt.Sprintf("Payment substitutes: %s", strings.Join(subs, ", ")))
	}

	if len(player.Effects) > 0 {
		var effects []string
		for _, e := range player.Effects {
			effects = append(effects, e.CardName)
		}
		lines = append(lines, fmt.Sprintf("Active effects: %s", strings.Join(effects, ", ")))
	}

	return strings.Join(lines, "\n")
}

func formatHand(cards []dto.PlayerCardDto) string {
	if len(cards) == 0 {
		return "=== HAND (0 cards) ===\n(empty)"
	}

	header := fmt.Sprintf("=== HAND (%d cards) ===", len(cards))
	var cardLines []string
	for _, c := range cards {
		avail := "PLAYABLE"
		if !c.Available {
			avail = "BLOCKED"
		}

		errInfo := ""
		if !c.Available && len(c.Errors) > 0 {
			var msgs []string
			for _, e := range c.Errors {
				msgs = append(msgs, e.Message)
			}
			errInfo = fmt.Sprintf(" (%s)", strings.Join(msgs, "; "))
		}

		tags := ""
		if len(c.Tags) > 0 {
			tagStrs := make([]string, len(c.Tags))
			for i, t := range c.Tags {
				tagStrs[i] = string(t)
			}
			tags = fmt.Sprintf(" [%s]", strings.Join(tagStrs, ", "))
		}

		discount := ""
		if c.EffectiveCost < c.Cost {
			discount = fmt.Sprintf(" (discounted from %d)", c.Cost)
		}

		line := fmt.Sprintf("  - %s [%s] | %dM€%s | %s%s | %s%s",
			c.Name, c.ID, c.EffectiveCost, discount, string(c.Type), tags, avail, errInfo)
		if desc := formatCardDescription(c.Description); desc != "" {
			line += "\n    " + desc
		}

		cardLines = append(cardLines, line)
	}

	return header + "\n" + strings.Join(cardLines, "\n")
}

func formatCardActions(actions []dto.PlayerActionDto) string {
	if len(actions) == 0 {
		return ""
	}

	header := "=== CARD ACTIONS ==="
	var actionLines []string
	for _, a := range actions {
		avail := "AVAILABLE"
		if !a.Available {
			avail = "BLOCKED"
		}

		errInfo := ""
		if !a.Available && len(a.Errors) > 0 {
			var msgs []string
			for _, e := range a.Errors {
				msgs = append(msgs, e.Message)
			}
			errInfo = fmt.Sprintf(" (%s)", strings.Join(msgs, "; "))
		}

		usedInfo := ""
		if a.TimesUsedThisTurn > 0 {
			usedInfo = fmt.Sprintf(" [used %dx this turn]", a.TimesUsedThisTurn)
		}

		desc := formatBehaviorBrief(a.Behavior.Inputs, a.Behavior.Outputs)

		line := fmt.Sprintf("  - %s [%s] behavior#%d | %s%s%s",
			a.CardName, a.CardID, a.BehaviorIndex, avail, usedInfo, errInfo)
		if desc != "" {
			line += fmt.Sprintf("\n    %s", desc)
		}

		for _, option := range a.ReuseOptions {
			if option.Available {
				line += fmt.Sprintf("\n    Reuse target cardId=%s behaviorIndex=%d with reuseSourceCardId=%s", option.CardID, option.BehaviorIndex, a.CardID)
			}
		}

		actionLines = append(actionLines, line)
	}

	return header + "\n" + strings.Join(actionLines, "\n")
}

func formatStandardProjects(projects []dto.PlayerStandardProjectDto) string {
	if len(projects) == 0 {
		return ""
	}

	header := "=== STANDARD PROJECTS ==="
	var lines []string
	for _, p := range projects {
		avail := "AVAILABLE"
		if !p.Available {
			avail = "BLOCKED"
		}

		errInfo := ""
		if !p.Available && len(p.Errors) > 0 {
			errInfo = fmt.Sprintf(" (%s)", p.Errors[0].Message)
		}

		var costParts []string
		for k, v := range p.EffectiveCost {
			costParts = append(costParts, fmt.Sprintf("%d %s", v, k))
		}
		costStr := strings.Join(costParts, ", ")

		lines = append(lines, fmt.Sprintf("  - %s | %s | %s%s", p.ProjectType, costStr, avail, errInfo))
	}

	return header + "\n" + strings.Join(lines, "\n")
}

func blockedReason(errors []dto.StateErrorDto) string {
	if len(errors) == 0 {
		return ""
	}
	msgs := make([]string, 0, len(errors))
	for _, e := range errors {
		msgs = append(msgs, e.Message)
	}
	return strings.Join(msgs, "; ")
}

func formatMilestones(game *dto.GameDto, milestones []dto.PlayerMilestoneDto) string {
	if len(milestones) == 0 {
		return ""
	}

	header := "=== MILESTONES ==="
	var lines []string
	for _, m := range milestones {
		var status string
		switch {
		case m.IsClaimed:
			by := ""
			if m.ClaimedBy != nil {
				by = findPlayerName(game, *m.ClaimedBy)
			}
			status = "CLAIMED by " + by
		case m.Available:
			status = fmt.Sprintf("CLAIMABLE NOW (%dM€)", m.ClaimCost)
		case m.Progress >= m.Required:
			status = fmt.Sprintf("QUALIFIED but blocked (%s); costs %dM€", blockedReason(m.Errors), m.ClaimCost)
		default:
			status = "not yet qualified"
		}
		lines = append(lines, fmt.Sprintf("  - %s [%s]: %s | Progress: %d/%d | %s",
			m.Name, m.Type, m.Description, m.Progress, m.Required, status))
	}

	return header + "\n" + strings.Join(lines, "\n")
}

func formatAwards(game *dto.GameDto, awards []dto.PlayerAwardDto) string {
	if len(awards) == 0 {
		return ""
	}

	header := "=== AWARDS ==="
	var lines []string
	for _, a := range awards {
		var status string
		switch {
		case a.IsFunded:
			by := ""
			if a.FundedBy != nil {
				by = findPlayerName(game, *a.FundedBy)
			}
			status = "FUNDED by " + by
		case a.Available:
			status = fmt.Sprintf("FUNDABLE NOW (%dM€)", a.FundingCost)
		default:
			status = fmt.Sprintf("not fundable (%s)", blockedReason(a.Errors))
		}
		lines = append(lines, fmt.Sprintf("  - %s [%s]: %s | %s", a.Name, a.Type, a.Description, status))
	}

	return header + "\n" + strings.Join(lines, "\n")
}

func formatOpponents(others []dto.OtherPlayerDto) string {
	if len(others) == 0 {
		return ""
	}

	header := "=== OPPONENTS ==="
	var lines []string
	for _, o := range others {
		r := &o.Resources
		corpName := "None"
		if o.Corporation != nil {
			corpName = o.Corporation.Name
		}
		lines = append(lines, strings.Join([]string{
			fmt.Sprintf("  %s (%s) | TR: %d | Status: %s | Passed: %v",
				o.Name, corpName, o.TerraformRating, string(o.Status), o.Passed),
			fmt.Sprintf("    Resources: %dM€, %d steel, %d ti, %d plants, %d energy, %d heat",
				r.Credits, r.Steel, r.Titanium, r.Plants, r.Energy, r.Heat),
			fmt.Sprintf("    Cards in hand: %d | Played: %d cards",
				o.HandCardCount, len(o.PlayedCards)),
		}, "\n"))
	}

	return header + "\n" + strings.Join(lines, "\n")
}

func formatBoard(game *dto.GameDto) string {
	var occupied []string
	freeByType := map[string][]string{}
	var freeTypes []string
	for _, t := range game.Board.Tiles {
		coord := fmt.Sprintf("%d,%d,%d", t.Coordinates.Q, t.Coordinates.R, t.Coordinates.S)
		if t.OccupiedBy == nil {
			if t.ReservedBy != nil || t.Location != "mars" {
				continue
			}
			if _, seen := freeByType[t.Type]; !seen {
				freeTypes = append(freeTypes, t.Type)
			}
			freeByType[t.Type] = append(freeByType[t.Type], coord+formatTileBonuses(t.Bonuses))
			continue
		}
		owner := ""
		if t.OwnerID != nil {
			owner = " (owner: " + findPlayerName(game, *t.OwnerID) + ")"
		}
		name := ""
		if t.DisplayName != nil {
			name = " " + *t.DisplayName
		}
		occupied = append(occupied, fmt.Sprintf("  %s%s | %s%s", coord, name, t.OccupiedBy.Type, owner))
	}

	lines := []string{"=== BOARD ==="}
	if len(occupied) > 0 {
		lines = append(lines, "Occupied:", strings.Join(occupied, "\n"))
	}
	for _, tileType := range freeTypes {
		lines = append(lines, fmt.Sprintf("Free %s spaces: %s", tileType, strings.Join(freeByType[tileType], "; ")))
	}
	return strings.Join(lines, "\n")
}

func formatTileBonuses(bonuses []dto.TileBonusDto) string {
	if len(bonuses) == 0 {
		return ""
	}
	parts := make([]string, 0, len(bonuses))
	for _, b := range bonuses {
		parts = append(parts, fmt.Sprintf("%d %s", b.Amount, b.Type))
	}
	return " [" + strings.Join(parts, ", ") + "]"
}

func formatColonyOutputs(outputs []dto.ColonyOutputDto) string {
	parts := make([]string, 0, len(outputs))
	for _, o := range outputs {
		parts = append(parts, fmt.Sprintf("%d %s", o.Amount, o.Type))
	}
	return strings.Join(parts, ", ")
}

func formatColonies(game *dto.GameDto, myPlayerID string) string {
	if len(game.Colonies) == 0 {
		return ""
	}
	lines := []string{"=== COLONIES ==="}
	if fleet, ok := game.TradeFleets[myPlayerID]; ok {
		lines = append(lines, fmt.Sprintf("Your trade fleets: %d available of %d", fleet.Available, fleet.Total))
	}
	for _, c := range game.Colonies {
		if !c.Active {
			continue
		}
		owners := make([]string, 0, len(c.PlayerColonies))
		for _, id := range c.PlayerColonies {
			owners = append(owners, findPlayerName(game, id))
		}
		status := []string{}
		if c.TradeAvailable {
			status = append(status, "TRADE: colony_trade")
		} else if len(c.TradeErrors) > 0 {
			status = append(status, "trade blocked ("+c.TradeErrors[0].Message+")")
		}
		if c.BuildAvailable {
			status = append(status, "BUILD: colony_build")
		} else if len(c.BuildErrors) > 0 {
			status = append(status, "build blocked ("+c.BuildErrors[0].Message+")")
		}
		if c.TradedThisGen {
			status = append(status, "traded this generation")
		}
		lines = append(lines, fmt.Sprintf("  - %s [%s] | marker %d | colonies: %s | colony bonus: %s | %s",
			c.Name, c.ID, c.MarkerPosition, strings.Join(owners, ", "), formatColonyOutputs(c.ColonyBonus), strings.Join(status, " | ")))
		for _, option := range c.TradeOptions {
			lines = append(lines, fmt.Sprintf("    trackSteps=%d -> marker %d, you gain: %s", option.TrackSteps, option.MarkerPosition, formatColonyOutputs(option.Outputs)))
		}
	}
	return strings.Join(lines, "\n")
}

func formatProjectFunding(projects []dto.ProjectFundingDto) string {
	if len(projects) == 0 {
		return ""
	}
	lines := []string{"=== PROJECT FUNDING ==="}
	for _, p := range projects {
		owners := make([]string, 0, len(p.SeatOwners))
		for _, o := range p.SeatOwners {
			owners = append(owners, o.Name)
		}
		status := "COMPLETED"
		if !p.IsCompleted {
			if p.CanBuySeat {
				status = fmt.Sprintf("next seat %dM€: project_fund_seat", p.NextSeatCost)
			} else if len(p.BuyErrors) > 0 {
				status = fmt.Sprintf("next seat %dM€, blocked (%s)", p.NextSeatCost, p.BuyErrors[0].Message)
			}
		}
		lines = append(lines, fmt.Sprintf("  - %s [%s]: %s | seats: %s | your seats: %d | %s",
			p.Name, p.ID, p.Description, strings.Join(owners, ", "), p.CurrentPlayerSeats, status))
	}
	return strings.Join(lines, "\n")
}

func formatFinalScores(game *dto.GameDto) string {
	if len(game.FinalScores) == 0 {
		return ""
	}

	header := "=== FINAL SCORES ==="
	var lines []string
	for _, s := range game.FinalScores {
		vp := s.VPBreakdown
		winner := ""
		if s.IsWinner {
			winner = " (WINNER)"
		}
		lines = append(lines, strings.Join([]string{
			fmt.Sprintf("  #%d %s%s: %d VP", s.Placement, s.PlayerName, winner, vp.TotalVP),
			fmt.Sprintf("    TR: %d, Cards: %d, Greenery: %d, City: %d, Milestones: %d, Awards: %d",
				vp.TerraformRating, vp.CardVP, vp.GreeneryVP, vp.CityVP, vp.MilestoneVP, vp.AwardVP),
		}, "\n"))
	}

	return header + "\n" + strings.Join(lines, "\n")
}

func formatProd(val int) string {
	if val >= 0 {
		return fmt.Sprintf("+%d", val)
	}
	return fmt.Sprintf("%d", val)
}

type conditionSummary interface {
	GetConditionType() string
	GetConditionAmount() int
}

func extractTypeAmount(item any) (string, int) {
	if c, ok := item.(conditionSummary); ok {
		return c.GetConditionType(), c.GetConditionAmount()
	}
	return "unknown", 0
}

func formatBehaviorBrief(inputs, outputs []any) string {
	formatItems := func(items []any) string {
		var parts []string
		for _, item := range items {
			t, a := extractTypeAmount(item)
			parts = append(parts, fmt.Sprintf("%d %s", a, t))
		}
		return strings.Join(parts, ", ")
	}

	var result []string
	if len(inputs) > 0 {
		result = append(result, "Costs: "+formatItems(inputs))
	}
	if len(outputs) > 0 {
		result = append(result, "Gives: "+formatItems(outputs))
	}
	return strings.Join(result, " -> ")
}

func findPlayerName(game *dto.GameDto, playerID string) string {
	if game.CurrentPlayer.ID == playerID {
		return game.CurrentPlayer.Name
	}
	for _, o := range game.OtherPlayers {
		if o.ID == playerID {
			return o.Name
		}
	}
	return playerID
}

// formatRecentLog lists recent game events with who did them and what changed for whom,
// so the bot can tell which player hurt or helped it.
func formatRecentLog(diffs []game.StateDiff, maxEntries int, nameOf func(playerID string) string) string {
	if len(diffs) == 0 {
		return ""
	}
	start := max(0, len(diffs)-maxEntries)

	var lines []string
	for _, d := range diffs[start:] {
		if d.Description == "" {
			continue
		}
		actor := "Game"
		if d.PlayerID != "" {
			actor = nameOf(d.PlayerID)
		}
		line := fmt.Sprintf("  - %s: %s", actor, d.Description)
		if effects := formatDiffEffects(d.Changes, nameOf); effects != "" {
			line += " [" + effects + "]"
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}
	return "=== RECENT GAME LOG (oldest first; who acted: what happened [effects]) ===\n" + strings.Join(lines, "\n")
}

func formatDiffEffects(changes *game.GameChanges, nameOf func(playerID string) string) string {
	if changes == nil {
		return ""
	}
	var parts []string
	addInt := func(label string, v *game.DiffValueInt) {
		if v != nil && v.New != v.Old {
			parts = append(parts, fmt.Sprintf("%s %+d", label, v.New-v.Old))
		}
	}
	addInt("temperature", changes.Temperature)
	addInt("oxygen", changes.Oxygen)
	addInt("oceans", changes.Oceans)

	playerIDs := make([]string, 0, len(changes.PlayerChanges))
	for id := range changes.PlayerChanges {
		playerIDs = append(playerIDs, id)
	}
	sort.Strings(playerIDs)
	for _, id := range playerIDs {
		pc := changes.PlayerChanges[id]
		var deltas []string
		add := func(label string, v *game.DiffValueInt) {
			if v != nil && v.New != v.Old {
				deltas = append(deltas, fmt.Sprintf("%s %+d", label, v.New-v.Old))
			}
		}
		add("credits", pc.Credits)
		add("steel", pc.Steel)
		add("titanium", pc.Titanium)
		add("plants", pc.Plants)
		add("energy", pc.Energy)
		add("heat", pc.Heat)
		add("TR", pc.TerraformRating)
		add("credit prod", pc.CreditsProduction)
		add("steel prod", pc.SteelProduction)
		add("titanium prod", pc.TitaniumProduction)
		add("plant prod", pc.PlantsProduction)
		add("energy prod", pc.EnergyProduction)
		add("heat prod", pc.HeatProduction)
		if len(deltas) > 0 {
			parts = append(parts, nameOf(id)+" "+strings.Join(deltas, ", "))
		}
	}
	if changes.BoardChanges != nil {
		for _, t := range changes.BoardChanges.TilesPlaced {
			owner := ""
			if t.OwnerID != "" {
				owner = " by " + nameOf(t.OwnerID)
			}
			parts = append(parts, fmt.Sprintf("%s placed at %s%s", t.TileType, t.HexID, owner))
		}
	}
	return strings.Join(parts, "; ")
}

func formatRecentChat(messages []shared.ChatMessage, maxEntries int) string {
	if len(messages) == 0 {
		return ""
	}

	start := 0
	if len(messages) > maxEntries {
		start = len(messages) - maxEntries
	}
	recent := messages[start:]

	var lines []string
	for _, m := range recent {
		lines = append(lines, fmt.Sprintf("  %s: %s", m.SenderName, m.Message))
	}

	return "=== RECENT CHAT ===\n" + strings.Join(lines, "\n")
}
