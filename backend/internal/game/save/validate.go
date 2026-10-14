package save

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"openmars/internal/game"
	"openmars/internal/game/award"
	"openmars/internal/game/board"
	"openmars/internal/game/cards"
	"openmars/internal/game/colony"
	"openmars/internal/game/datastore"
	"openmars/internal/game/milestone"
	"openmars/internal/game/mode"
	"openmars/internal/game/projectfunding"
	"openmars/internal/game/shared"
	"openmars/internal/game/standardproject"
)

// Catalog supplies immutable definitions used to validate a portable position.
type Catalog struct {
	rawCards             map[string]*cards.Card
	effectiveCards       map[[2]string]*cards.Card
	allowUnselectedCards bool
	StandardProjects     standardproject.StandardProjectRegistry
	Cards                cards.CardRegistry
	Maps                 *board.MapRegistry
	Colonies             colony.ColonyRegistry
	Awards               award.AwardRegistry
	Milestones           milestone.MilestoneRegistry
	Projects             projectfunding.ProjectFundingRegistry
}

// Fingerprint includes all loaded rule assets with deterministic project ordering.
func (c Catalog) Fingerprint() (string, error) {
	maps := []*board.MapDefinition{}
	for _, info := range c.Maps.ListMaps() {
		m, _ := c.Maps.GetMap(info.ID)
		maps = append(maps, m)
	}
	projects := c.Projects.GetAll()
	slices.SortFunc(projects, func(a, b projectfunding.ProjectDefinition) int { return strings.Compare(a.ID, b.ID) })
	return Fingerprint(struct {
		Cards            []cards.Card
		Modes            []mode.Definition
		Maps             []*board.MapDefinition
		Colonies         []colony.ColonyDefinition
		Awards           []award.AwardDefinition
		Milestones       []milestone.MilestoneDefinition
		Projects         []projectfunding.ProjectDefinition
		StandardProjects []standardproject.StandardProjectDefinition
	}{c.Cards.GetAll(), cards.ModeDefinitions(c.Cards), maps, c.Colonies.GetAll(), c.Awards.GetAll(), c.Milestones.GetAll(), projects, c.StandardProjects.GetAll()})
}

// Validate checks current gameplay invariants and the complete historical archive.
func Validate(doc *Document, catalog Catalog, fingerprint string) (err error) {
	defer func() {
		if err != nil {
			var classified *Error
			if !errors.As(err, &classified) {
				err = Failure("invalid_save", "This save contains invalid game data. Select another export.", err)
			}
		}
	}()
	if doc == nil || doc.State == nil {
		return fmt.Errorf("state: required")
	}
	if doc.FormatVersion != FormatVersion || doc.RulesVersion != RulesVersion {
		return Failure("incompatible_save", "This save uses a different game version. Open it on a matching server.", nil)
	}
	if doc.ContentFingerprint != fingerprint {
		return Failure("incompatible_save", "This save uses different game content. Open it on a matching server.", nil)
	}
	if doc.SavedAt.IsZero() {
		return fmt.Errorf("savedAt: required")
	}
	if doc.State.Status != shared.GameStatusActive {
		return fmt.Errorf("only in-progress games can be loaded")
	}
	// Lookups are immutable during validation; archive snapshots reuse these local copies.
	catalog.rawCards = make(map[string]*cards.Card)
	catalog.effectiveCards = make(map[[2]string]*cards.Card)
	if err := catalog.validateState(doc.State); err != nil {
		return fmt.Errorf("state: %w", err)
	}
	if len(doc.History) > MaxEntries || len(doc.Log) > MaxEntries {
		return fmt.Errorf("too many history entries")
	}
	// Archive snapshots include intermediate mutations (for example, assigning a
	// host before inserting that player). Resolve their references across the archive.
	archive := *doc.State
	archive.Players = map[string]*datastore.PlayerState{}
	archive.Settings = doc.State.Settings
	archiveCatalog := catalog
	archiveCatalog.allowUnselectedCards = true
	for id, p := range doc.State.Players {
		archive.Players[id] = p
	}
	for _, h := range doc.History {
		if h != nil && h.State != nil {
			for id, p := range h.State.Players {
				archive.Players[id] = p
			}
		}
	}
	var seq int64
	for i, h := range doc.History {
		if h == nil || h.State == nil || h.GameID != doc.State.ID || h.State.ID != doc.State.ID || h.Sequence <= seq || h.Timestamp.IsZero() {
			return fmt.Errorf("history[%d]: invalid entry or sequence", i)
		}
		for id, p := range h.State.Players {
			if p == nil || p.ID != id {
				return fmt.Errorf("history[%d]: invalid player", i)
			}
		}
		historyPosition := archive
		historyPosition.Settings = h.State.Settings
		if err := archiveCatalog.walk(reflect.ValueOf(h.State), &historyPosition, "history"); err != nil {
			return fmt.Errorf("history[%d]: %w", i, err)
		}
		seq = h.Sequence
	}
	seq = 0
	for i, entry := range doc.Log {
		if entry.GameID != doc.State.ID || entry.SequenceNumber <= seq || entry.Timestamp.IsZero() {
			return fmt.Errorf("log[%d]: invalid entry or sequence", i)
		}
		if entry.PlayerID != "" && archive.Players[entry.PlayerID] == nil {
			return fmt.Errorf("log[%d]: unknown player", i)
		}
		if err := archiveCatalog.walk(reflect.ValueOf(entry), &archive, "log"); err != nil {
			return err
		}
		seq = entry.SequenceNumber
	}
	if len(doc.Log) > 0 && doc.LogBaseline == nil {
		return fmt.Errorf("logBaseline: required when log is present")
	}
	if doc.LogBaseline != nil {
		for id, p := range doc.LogBaseline.Players {
			if p == nil || archive.Players[id] == nil {
				return fmt.Errorf("logBaseline: invalid player")
			}
		}
		if err := archiveCatalog.walk(reflect.ValueOf(doc.LogBaseline), &archive, "logBaseline"); err != nil {
			return err
		}
	}
	return nil
}

func (c Catalog) validateState(s *datastore.GameState) error {
	if s.ID == "" || len(s.ID) > 128 {
		return fmt.Errorf("id: required and at most 128 characters")
	}
	if s.CreatedAt.IsZero() || s.UpdatedAt.IsZero() {
		return fmt.Errorf("timestamps: required")
	}
	if s.Generation < 1 || s.Generation > 10000 {
		return fmt.Errorf("generation: out of range")
	}
	if s.Settings.MaxPlayers < 1 || s.Settings.MaxPlayers > 10 || len(s.Players) > s.Settings.MaxPlayers {
		return fmt.Errorf("players: invalid player count")
	}
	if s.Players == nil {
		return fmt.Errorf("players: required")
	}
	if len(s.Players) == 0 || s.Players[s.HostPlayerID] == nil {
		return fmt.Errorf("hostPlayerId: unknown player")
	}
	def, ok := c.Maps.GetMap(s.Settings.MapID)
	if !ok {
		return fmt.Errorf("settings.mapId: unknown map")
	}
	settings, definition, err := mode.Normalize(cards.ModeDefinitions(c.Cards), s.Settings)
	if err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	if settings.GameModeID != s.Settings.GameModeID || !maps.Equal(settings.GameModeParameters, s.Settings.GameModeParameters) || !slices.Equal(settings.Features, s.Settings.Features) || !slices.Equal(settings.Systems, s.Settings.Systems) {
		return fmt.Errorf("settings: incomplete or inconsistent mode selection")
	}
	if err := c.validateModeRuntime(s, definition); err != nil {
		return err
	}
	phases := []shared.GamePhase{shared.GamePhaseWaitingForGameStart, shared.GamePhaseStartingSelection, shared.GamePhaseInitApplyCorp, shared.GamePhaseInitApplyPrelude, shared.GamePhaseAction, shared.GamePhaseProductionAndCardDraw, shared.GamePhaseFinalPhase, shared.GamePhaseComplete}
	if !slices.Contains(phases, s.CurrentPhase) {
		return fmt.Errorf("currentPhase: unknown phase")
	}
	if !slices.Contains([]shared.GameStatus{shared.GameStatusLobby, shared.GameStatusActive, shared.GameStatusCompleted}, s.Status) {
		return fmt.Errorf("status: unknown status")
	}
	if s.CurrentPhase == shared.GamePhaseWaitingForGameStart || s.CurrentPhase == shared.GamePhaseComplete {
		return fmt.Errorf("currentPhase: not an active phase")
	}
	if s.Temperature < -30 || s.Temperature > 8 || s.Temperature%2 != 0 || s.Oxygen < 0 || s.Oxygen > 14 || s.Oceans < 0 || s.Oceans > s.MaxOceans || s.MaxOceans > 9 || s.Venus < 0 || s.Venus > 30 || s.Venus%2 != 0 {
		return fmt.Errorf("global parameters: out of range")
	}
	if s.CurrentTurnActions < -1 || s.CurrentTurnTotalActions < -1 || s.GlobalActionCounter < 0 || s.ShuffleCount < 0 || s.DrawnCardCount < 0 {
		return fmt.Errorf("turn/deck counters: out of range")
	}
	if err := checkOrder(s.PlayerOrder, s.Players); err != nil {
		return fmt.Errorf("playerOrder: %w", err)
	}
	if err := checkOrder(s.TurnOrder, s.Players); err != nil {
		return fmt.Errorf("turnOrder: %w", err)
	}
	if s.Players[s.CurrentTurnPlayerID] == nil {
		return fmt.Errorf("currentTurnPlayerId: unknown player")
	}
	expected := board.GenerateBoardFromMap(def, s.Settings.HasVenus())
	hexes := map[string]board.Tile{}
	for _, tile := range expected {
		hexes[tile.Coordinates.String()] = tile
	}
	seen := map[string]bool{}
	if len(s.Tiles) != len(expected) {
		return fmt.Errorf("tiles: board does not match map")
	}
	for _, tile := range s.Tiles {
		hex := tile.Coordinates.String()
		expectedTile, known := hexes[hex]
		if !known || seen[hex] {
			return fmt.Errorf("tiles: unknown or duplicate hex %q", hex)
		}
		seen[hex] = true
		if tile.Type != expectedTile.Type || tile.Location != expectedTile.Location {
			return fmt.Errorf("tiles: terrain does not match map at %s", hex)
		}
		if tile.OccupiedBy != nil && !board.ValidPlaceableTileType(strings.TrimSuffix(string(tile.OccupiedBy.Type), "-tile")) {
			return fmt.Errorf("tiles: unknown tile type %q", tile.OccupiedBy.Type)
		}
	}
	locations := map[string]string{}
	register := func(ids []string, where string) error {
		for _, id := range ids {
			if id == "" {
				return fmt.Errorf("%s: empty card ID", where)
			}
			if prev, ok := locations[id]; ok {
				return fmt.Errorf("card %s occurs in both %s and %s", id, prev, where)
			}
			locations[id] = where
		}
		return nil
	}
	for _, zone := range []struct {
		ids  []string
		name string
	}{{s.ProjectCards, "deck"}, {s.Corporations, "corporations"}, {s.PreludeCards, "preludes"}, {s.DiscardPile, "discard"}, {s.RemovedCards, "removed"}} {
		if err := register(zone.ids, zone.name); err != nil {
			return err
		}
	}
	for _, phase := range s.SelectCorporationPhases {
		if phase != nil {
			if err := register(phase.AvailableCorporations, "corporation selection"); err != nil {
				return err
			}
		}
	}
	for _, phase := range s.SelectStartingCardsPhases {
		if phase != nil && !phase.SelectionComplete {
			if err := register(phase.AvailableCards, "starting cards"); err != nil {
				return err
			}
		}
	}
	for _, phase := range s.SelectPreludeCardsPhases {
		if phase != nil {
			if err := register(phase.AvailablePreludes, "prelude selection"); err != nil {
				return err
			}
		}
	}
	for _, phase := range s.ProductionPhases {
		if phase != nil && !phase.SelectionComplete {
			if err := register(phase.AvailableCards, "production cards"); err != nil {
				return err
			}
		}
	}
	for _, choices := range s.DeferredStartingChoices {
		if choices == nil {
			continue
		}
		if !choices.CorpApplied {
			if err := register(choices.CardIDs, "chosen starting cards"); err != nil {
				return err
			}
		}
		if choices.PreludesAppliedCount >= 0 && choices.PreludesAppliedCount <= len(choices.PreludeIDs) {
			if err := register(choices.PreludeIDs[choices.PreludesAppliedCount:], "chosen preludes"); err != nil {
				return err
			}
		}
	}

	for id, p := range s.Players {
		if p == nil || p.ID != id || id == "" {
			return fmt.Errorf("players: invalid identity")
		}
		if strings.TrimSpace(p.Name) == "" || utf8.RuneCountInString(p.Name) > game.MaxPlayerNameLength {
			return fmt.Errorf("players.%s.name: invalid name", id)
		}
		if !slices.Contains(shared.PlayerColors, p.Color) {
			return fmt.Errorf("players.%s.color: unknown color", id)
		}
		if p.TerraformRating < 0 {
			return fmt.Errorf("players.%s.terraformRating: negative rating", id)
		}
		if p.PlayerType != "human" && p.PlayerType != "bot" {
			return fmt.Errorf("players.%s.playerType: invalid", id)
		}
		if pending := p.PendingCardPlaySelection; pending != nil {
			source, err := c.rawCard(pending.SourceCardID)
			if err != nil || pending.CostReduction < 0 || !slices.Contains(p.PlayedCardIDs, pending.SourceCardID) {
				return fmt.Errorf("players.%s.pendingCardPlaySelection: invalid source", id)
			}
			valid := false
			for _, b := range source.Behaviors {
				for _, output := range b.Outputs {
					if op, ok := output.(*shared.CardOperationCondition); ok && op.ResourceType == shared.ResourceCardPlay && op.CostReduction == pending.CostReduction && reflect.DeepEqual(op.Selectors, pending.Selectors) {
						valid = true
					}
				}
			}
			if !valid {
				return fmt.Errorf("players.%s.pendingCardPlaySelection: invalid operation", id)
			}
		}
		if pending := p.PendingCardDrawSelection; pending != nil {
			if err := register(pending.AvailableCards, "pending draw "+id); err != nil {
				return err
			}
		}
		if err := register(p.HandCardIDs, "hand "+id); err != nil {
			return err
		}
		if err := register(p.PlayedCardIDs, "played "+id); err != nil {
			return err
		}
		if p.CorporationID != "" {
			card, err := c.rawCard(p.CorporationID)
			if err != nil || card.Type != cards.CardTypeCorporation {
				return fmt.Errorf("players.%s.corporationId: invalid corporation", id)
			}
			if err := register([]string{p.CorporationID}, "corporation "+id); err != nil {
				return err
			}
		}
		if p.Resources.Credits < 0 || p.Resources.Steel < 0 || p.Resources.Titanium < 0 || p.Resources.Plants < 0 || p.Resources.Energy < 0 || p.Resources.Heat < 0 {
			return fmt.Errorf("players.%s.resources: negative resource", id)
		}
		if !validSavedProduction(p) {
			return fmt.Errorf("players.%s.production: out of range", id)
		}
		for _, a := range p.Actions {
			if a.TimesUsedThisTurn < 0 || a.TimesUsedThisGeneration < 0 {
				return fmt.Errorf("players.%s.actions: negative usage", id)
			}
		}
	}
	for id, f := range s.TradeFleets {
		if s.Players[id] == nil || f.Used < 0 || f.Capacity < 0 || f.Used > f.Capacity {
			return fmt.Errorf("tradeFleets: invalid fleet")
		}
	}
	for _, state := range s.ColonyStates {
		if state == nil {
			return fmt.Errorf("colonyStates: null colony")
		}
		def, err := c.Colonies.GetByID(state.DefinitionID)
		if err != nil || state.MarkerPosition < -1 || state.MarkerPosition >= len(def.Steps) || len(state.PlayerColonies) > len(def.Colonies) {
			return fmt.Errorf("colonyStates: invalid colony")
		}
	}
	for _, id := range s.SelectedAwards {
		if _, err := c.Awards.GetByID(id); err != nil {
			return err
		}
	}
	for _, id := range s.SelectedMilestones {
		if _, err := c.Milestones.GetByID(id); err != nil {
			return err
		}
	}
	for id, first := range s.ForcedFirstActions {
		if s.Players[id] == nil || first == nil || !slices.Contains([]string{"queued", "resolving"}, first.State) {
			return fmt.Errorf("forcedFirstActions: invalid continuation")
		}
		card, err := c.cardForState(first.CorporationID, s)
		if err != nil {
			return err
		}
		for _, index := range first.BehaviorIndices {
			if index < 0 || index >= len(card.Behaviors) {
				return fmt.Errorf("forcedFirstActions: invalid behavior index")
			}
		}
	}
	if err := c.validatePosition(s); err != nil {
		return err
	}
	return c.walk(reflect.ValueOf(s), s, "state")
}

func checkOrder(order []string, players map[string]*datastore.PlayerState) error {
	seen := map[string]bool{}
	for _, id := range order {
		if seen[id] || players[id] == nil {
			return fmt.Errorf("unknown or duplicate player")
		}
		seen[id] = true
	}
	if len(seen) != len(players) {
		return fmt.Errorf("missing player")
	}
	return nil
}

func (c Catalog) cardID(id string, s *datastore.GameState) error {
	if id == "" {
		return nil
	}
	card, err := c.cardForState(id, s)
	if err != nil {
		return err
	}
	if !c.allowUnselectedCards && (!s.Settings.AllowsContent(card.GameModes, card.RequiresSystems, card.RequiresFeatures) || card.Type == cards.CardTypePrelude && !s.Settings.HasPrelude()) {
		return fmt.Errorf("card %s is unavailable in this mode selection", id)
	}
	return nil
}

func (c Catalog) walk(v reflect.Value, s *datastore.GameState, path string) error {
	if !v.IsValid() {
		return nil
	}
	if v.CanInterface() {
		if condition, ok := v.Interface().(shared.BehaviorCondition); ok && !v.IsNil() {
			if !slices.Contains(shared.AllResourceTypes, condition.GetResourceType()) {
				return fmt.Errorf("%s: unknown resource type", path)
			}
			if condition.GetAmount() < -10000 || condition.GetAmount() > 10000 {
				return fmt.Errorf("%s: effect amount out of range", path)
			}
			if problems := shared.ValidateResourceCondition(condition, strings.Contains(path, ".Inputs[")); len(problems) > 0 {
				return fmt.Errorf("%s: %s", path, strings.Join(problems, "; "))
			}
		}
	}
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.CanInterface() {
		if per, ok := v.Interface().(shared.PerCondition); ok && per.Amount <= 0 {
			return fmt.Errorf("%s: per divisor must be positive", path)
		}
		if callback, ok := v.Interface().(shared.TileCompletionCallback); ok {
			if !slices.Contains([]string{"convert-plants-to-greenery", "standard-project-greenery", "standard-project-aquifer", "adjacent-removal"}, callback.Type) {
				return fmt.Errorf("%s: unknown tile callback", path)
			}
			if callback.Type == "adjacent-removal" && callback.Output == nil {
				return fmt.Errorf("%s: missing removal output", path)
			}
		}
	}
	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		cardID := ""
		for _, name := range []string{"CardID", "SourceCardID"} {
			if f := v.FieldByName(name); f.IsValid() && f.Kind() == reflect.String && f.String() != "" {
				cardID = f.String()
				break
			}
		}
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() || field.Tag.Get("json") == "-" {
				continue
			}
			value := v.Field(i)
			if field.Name == "BehaviorIndex" || field.Name == "SourceBehaviorIndex" {
				if cardID != "" {
					card, err := c.cardForState(cardID, s)
					if err != nil {
						return err
					}
					if value.Int() < 0 || int(value.Int()) >= len(card.Behaviors) {
						return fmt.Errorf("%s.%s: invalid behavior index", path, field.Name)
					}
				}
			}
			if err := c.reference(value, field.Name, s, path); err != nil {
				return err
			}
			if err := c.walk(value, s, path+"."+field.Name); err != nil {
				return err
			}
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			entry := iter.Value()
			if (entry.Kind() == reflect.Pointer || entry.Kind() == reflect.Interface) && entry.IsNil() {
				return fmt.Errorf("%s: null map entry", path)
			}
			if err := c.walk(entry, s, path); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		if v.Len() > MaxEntries {
			return fmt.Errorf("%s: too many entries", path)
		}
		for i := 0; i < v.Len(); i++ {
			entry := v.Index(i)
			if (entry.Kind() == reflect.Pointer || entry.Kind() == reflect.Interface) && entry.IsNil() {
				return fmt.Errorf("%s[%d]: null entry", path, i)
			}
			if err := c.walk(entry, s, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case reflect.Int, reflect.Int64:
		if v.Int() < -1000000000 || v.Int() > 1000000000 {
			return fmt.Errorf("%s: numeric value out of range", path)
		}
	case reflect.String:
		if len(v.String()) > MaxStringBytes {
			return fmt.Errorf("%s: string too long", path)
		}
	}
	return nil
}

func (c Catalog) reference(v reflect.Value, name string, s *datastore.GameState, path string) error {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			if err := c.reference(v.Index(i), strings.TrimSuffix(name, "s"), s, path); err != nil {
				return err
			}
		}
		return nil
	}
	if v.Kind() != reflect.String || v.String() == "" {
		return nil
	}
	id := v.String()
	switch name {
	case "CardID", "CorporationID", "SourceCardID", "TriggeringCardID", "HandCardID", "PlayedCardID", "ProjectCard", "Corporation", "PreludeCard", "PreludeID", "Card", "DiscardPile", "RemovedCard", "AvailableCard", "AvailableCorporation", "AvailablePrelude", "Cards":
		if err := c.cardID(id, s); err != nil {
			return fmt.Errorf("%s.%s: %w", path, name, err)
		}
	case "PlayerID", "HostPlayerID", "CurrentTurnPlayerID", "CurrentTurn", "TargetPlayerID", "TriggeringPlayerID", "FundedByPlayer", "OwnerID", "ReservedBy", "TraderID", "EligiblePlayerID", "PlayerColonie", "SeatOwner":
		if s.Players[id] == nil {
			return fmt.Errorf("%s.%s: unknown player %q", path, name, id)
		}
	}
	return nil
}

func (c Catalog) rawCard(id string) (*cards.Card, error) {
	if card, ok := c.rawCards[id]; ok {
		return card, nil
	}
	card, err := c.Cards.GetByID(id)
	if err == nil && c.rawCards != nil {
		c.rawCards[id] = card
	}
	return card, err
}

func (c Catalog) cardForState(id string, s *datastore.GameState) (*cards.Card, error) {
	key := [2]string{s.Settings.GameModeID, id}
	if card, ok := c.effectiveCards[key]; ok {
		return card, nil
	}
	card, err := c.rawCard(id)
	if err != nil {
		return nil, err
	}
	effective, err := cards.ApplyModeModifiers(*card, s.Settings.GameModeID)
	if err == nil && c.effectiveCards != nil {
		c.effectiveCards[key] = &effective
	}
	return &effective, err
}

func (c Catalog) validateModeRuntime(s *datastore.GameState, definition mode.Definition) error {
	r := s.ModeRuntime
	actual, err := json.Marshal(r.Definition)
	if err != nil {
		return err
	}
	expected, err := json.Marshal(definition.Runtime)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return fmt.Errorf("modeRuntime.definition: does not match the catalog")
	}
	initial := maps.Clone(definition.Runtime.State)
	if initial == nil {
		initial = map[string]int{}
	}
	for _, deadline := range definition.Runtime.Deadlines {
		initial[deadline.State] = s.Settings.GameModeParameters[deadline.Parameter]
	}
	if !maps.Equal(initial, r.InitialValues) || len(r.Values) != len(initial) {
		return fmt.Errorf("modeRuntime: invalid initial values or state keys")
	}
	for state := range r.Values {
		if _, ok := initial[state]; !ok {
			return fmt.Errorf("modeRuntime: unknown state %s", state)
		}
	}
	for _, deadline := range definition.Runtime.Deadlines {
		value := r.Values[deadline.State]
		if value < initial[deadline.State]-deadline.MaxParameterAdjustment || value > initial[deadline.State]+deadline.MaxParameterAdjustment {
			return fmt.Errorf("modeRuntime: deadline outside configured bounds")
		}
		occurred := r.Values[deadline.OccurredState]
		if occurred != 0 && occurred != 1 || occurred == 0 && r.Values[deadline.ActiveState] != deadline.ActiveValue || occurred == 1 && r.Values[deadline.ActiveState] != deadline.InactiveValue {
			return fmt.Errorf("modeRuntime: inconsistent deadline state")
		}
	}
	if r.LastStartedGeneration < 0 || r.LastStartedGeneration > s.Generation || r.IncomeStreak < 0 || r.IncomeStreak > r.LastStartedGeneration {
		return fmt.Errorf("modeRuntime: invalid generation income progress")
	}
	if r.Sequence < 0 {
		return fmt.Errorf("modeRuntime.sequence: negative value")
	}
	for _, count := range r.Changes {
		if count < 0 {
			return fmt.Errorf("modeRuntime.changes: negative usage")
		}
	}
	boundary := func(generation int, name string) int {
		return generation*3 + slices.Index([]string{"research-start", "production-start", "production-end"}, name)
	}
	ids := map[string]bool{}
	for i, window := range r.Windows {
		_, known := initial[window.State]
		if window.ID == "" || ids[window.ID] || !known || window.StartGeneration < 1 || window.EndGeneration < window.StartGeneration || !slices.Contains([]string{"pending", "active", "completed", "cancelled"}, window.Status) || !slices.Contains([]string{"research-start", "production-start", "production-end"}, window.StartBoundary) || !slices.Contains([]string{"research-start", "production-start", "production-end"}, window.EndBoundary) {
			return fmt.Errorf("modeRuntime.windows: invalid interval")
		}
		ids[window.ID] = true
		start, end := boundary(window.StartGeneration, window.StartBoundary), boundary(window.EndGeneration, window.EndBoundary)
		if end <= start {
			return fmt.Errorf("modeRuntime.windows: invalid interval order")
		}
		for _, previous := range r.Windows[:i] {
			if window.State == previous.State && window.Status != "cancelled" && previous.Status != "cancelled" && start < boundary(previous.EndGeneration, previous.EndBoundary) && end > boundary(previous.StartGeneration, previous.StartBoundary) {
				return fmt.Errorf("modeRuntime.windows: overlapping reservations")
			}
		}
	}
	for _, p := range s.Players {
		if p == nil {
			continue
		}
		groups := maps.Clone(p.DependencyGroups)
		if groups == nil {
			groups = map[string]shared.DependencyGroup{}
		}
		for _, contribution := range p.ProductionContributions {
			if !slices.Contains(p.PlayedCardIDs, contribution.CardID) && p.CorporationID != contribution.CardID {
				return fmt.Errorf("productionContributions: source card is not owned")
			}
			if !slices.Contains([]shared.ResourceType{shared.ResourceCreditProduction, shared.ResourceSteelProduction, shared.ResourceTitaniumProduction, shared.ResourcePlantProduction, shared.ResourceEnergyProduction, shared.ResourceHeatProduction}, contribution.Resource) {
				return fmt.Errorf("productionContributions: invalid production resource")
			}
			groups[shared.DependencyKey(contribution.CardID, contribution.Dependency.ID)] = contribution.Dependency
		}
		for _, group := range groups {
			if _, known := initial[group.State]; group.ID == "" || !known {
				return fmt.Errorf("dependencyGroups: unknown state or empty ID")
			}
			if group.RetireOnState != nil {
				if _, known := initial[group.RetireOnState.State]; !known {
					return fmt.Errorf("dependencyGroups: unknown retirement state")
				}
			}
		}
		for key, status := range p.DependencyStatuses {
			group, ok := groups[key]
			if !ok || status.Removed && (status.Retired || !group.Importable) {
				return fmt.Errorf("dependencyStatuses: invalid source or status")
			}
		}
		for _, count := range p.BehaviorUses {
			if count < 0 {
				return fmt.Errorf("behaviorUses: negative usage")
			}
		}
	}
	return nil
}

func validSavedProduction(p *datastore.PlayerState) bool {
	for _, resource := range []shared.ResourceType{shared.ResourceCreditProduction, shared.ResourceSteelProduction, shared.ResourceTitaniumProduction, shared.ResourcePlantProduction, shared.ResourceEnergyProduction, shared.ResourceHeatProduction} {
		minimum := 0
		if resource == shared.ResourceCreditProduction {
			minimum = shared.MinCreditProduction
		}
		// Importing signed upkeep preserves the raw rate; the floor applies to effective production.
		for _, contribution := range p.ProductionContributions {
			if contribution.Resource == resource && contribution.Amount < 0 && p.DependencyStatuses[shared.DependencyKey(contribution.CardID, contribution.Dependency.ID)].Removed {
				minimum += contribution.Amount
			}
		}
		if p.Production.GetAmount(resource) < minimum {
			return false
		}
	}
	return true
}
