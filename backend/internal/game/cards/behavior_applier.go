package cards

import (
	"context"
	"fmt"
	"log/slog"

	"openmars/internal/game"
	"openmars/internal/game/award"
	"openmars/internal/game/board"
	"openmars/internal/game/colony"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// ColonyBonusLookup provides colony definition lookup for colony-bonus output handling.
type ColonyBonusLookup interface {
	GetByID(colonyID string) (*colony.ColonyDefinition, error)
}

// BehaviorApplier handles applying card behavior inputs and outputs
// This is the single source of truth for all input/output application
type BehaviorApplier struct {
	behaviorValidated  bool
	behavior           *shared.CardBehavior
	productionBox      string
	player             *player.Player // Player affected by the behavior (may be nil for game-only effects)
	game               *game.Game     // Game context for global params/tiles (may be nil for player-only effects)
	source             string         // Source identifier for logging (card name, action name, etc.)
	triggeringCardID   string
	triggeringPlayerID string
	sourceCardID       string // Card ID for self-card targeting (optional)
	inputCardIDs       []string
	reservedResources  map[shared.ResourceType]int
	reservedStorage    map[string]int
	targetCardIDs      []string // Card IDs for any-card targeting (positional, one per any-card output)
	anyCardTargetIdx   int      // Index into targetCardIDs, incremented each time an any-card output is processed
	targetPlayerID     string   // Player ID for any-player targeting (optional, set by caller)
	stealSourceCardID  string   // Card ID to steal resources from for steal-from-any-card outputs (optional)
	completeAction     *shared.CardActionRef
	sourceBehaviorIdx  int // Behavior index for card draw selection tracking
	selectedAmount     int // Player-selected amount for variable-amount behaviors (0 = not applicable)
	inputPaymentPlan   PaymentPlan
	actionPayment      *shared.Payment       // Optional payment for action inputs with paymentAllowed (e.g., titanium for Water Import From Europa)
	cardRegistry       CardRegistryInterface // Card registry for tag counting in per conditions (optional)
	sourceType         shared.SourceType     // Source type for triggered effect classification
	colonyBonusLookup  ColonyBonusLookup
	awardRegistry      award.AwardRegistry
	deferredRemoval    *shared.BasicResourceCondition
	logger             *slog.Logger
}

// DeferredRemoval returns the deferred resource removal output, if any (for post-tile-placement processing)
func (a *BehaviorApplier) DeferredRemoval() *shared.BasicResourceCondition {
	return a.deferredRemoval
}

// NewBehaviorApplier creates a new behavior applier
// player and game can be nil if not needed for the specific operations
func NewBehaviorApplier(
	p *player.Player,
	g *game.Game,
	source string,
	logger *slog.Logger,
) *BehaviorApplier {
	return &BehaviorApplier{
		player: p,
		game:   g,
		source: source,
		logger: logger,
	}
}

// WithSourceCardID sets the source card ID for self-card targeting
func (a *BehaviorApplier) WithSourceCardID(cardID string) *BehaviorApplier {
	a.sourceCardID = cardID
	return a
}

// WithTargetCardIDs sets the target card IDs for any-card resource placement (positional, one per any-card output)
func (a *BehaviorApplier) WithTargetCardIDs(cardIDs []string) *BehaviorApplier {
	a.targetCardIDs = cardIDs
	a.anyCardTargetIdx = 0
	return a
}

// nextTargetCardID returns the next target card ID for any-card outputs and advances the index
func (a *BehaviorApplier) nextTargetCardID() string {
	if a.anyCardTargetIdx >= len(a.targetCardIDs) {
		return ""
	}
	id := a.targetCardIDs[a.anyCardTargetIdx]
	a.anyCardTargetIdx++
	return id
}

// WithCardRegistry sets the card registry for tag counting in scaled outputs
func (a *BehaviorApplier) WithCardRegistry(registry CardRegistryInterface) *BehaviorApplier {
	a.cardRegistry = registry
	return a
}

// WithTargetPlayerID sets the target player ID for any-player resource/production removal
func (a *BehaviorApplier) WithTargetPlayerID(playerID string) *BehaviorApplier {
	a.targetPlayerID = playerID
	return a
}

// WithStealSourceCardID sets the source card ID for steal-from-any-card outputs
func (a *BehaviorApplier) WithStealSourceCardID(cardID string) *BehaviorApplier {
	a.stealSourceCardID = cardID
	return a
}

// WithActionCompletion identifies the action consumed when a deferred selection completes.
func (a *BehaviorApplier) WithActionCompletion(ref shared.CardActionRef) *BehaviorApplier {
	a.completeAction = &ref
	return a
}

// WithSourceBehaviorIndex sets the source behavior index for card draw selection tracking
func (a *BehaviorApplier) WithSourceBehaviorIndex(behaviorIndex int) *BehaviorApplier {
	a.sourceBehaviorIdx = behaviorIndex
	return a
}

// WithSelectedAmount sets the player-selected amount for variable-amount behaviors
func (a *BehaviorApplier) WithSelectedAmount(amount int) *BehaviorApplier {
	a.selectedAmount = amount
	return a
}

// WithActionPayment sets the payment for action inputs that have paymentAllowed
// (e.g., Water Import From Europa allows titanium as payment for the 12 M€ action cost)
func (a *BehaviorApplier) WithActionPayment(payment *shared.Payment) *BehaviorApplier {
	a.actionPayment = payment
	return a
}

// WithSourceType sets the source type for triggered effect classification
func (a *BehaviorApplier) WithSourceType(sourceType shared.SourceType) *BehaviorApplier {
	a.sourceType = sourceType
	return a
}

// WithColonyBonusLookup sets the colony definition lookup for colony-bonus outputs
func (a *BehaviorApplier) WithColonyBonusLookup(lookup ColonyBonusLookup) *BehaviorApplier {
	a.colonyBonusLookup = lookup
	return a
}

// ApplyInputs validates player has required resources and deducts them
// Returns error if player is nil or insufficient resources
func (a *BehaviorApplier) ApplyInputs(
	ctx context.Context,
	inputs []shared.BehaviorCondition,
) error {

	if a.player == nil {
		return fmt.Errorf("cannot apply inputs: no player context")
	}

	log := a.logger.With(
		slog.String("source", a.source),
		slog.Int("input_count", len(inputs)),
	)

	log.Debug("Processing behavior inputs")
	if a.behavior != nil {
		if err := BehaviorAvailable(a.game, a.player, a.sourceCardID, *a.behavior, a.cardRegistry); err != nil {
			return err
		}
	}

	if err := a.ValidateInputs(inputs); err != nil {
		return err
	}
	a.behaviorValidated = true
	inputSources, err := a.resolveInputSources(inputs)
	if err != nil {
		return err
	}

	ApplyPayment(a.player, a.inputPaymentPlan)
	for inputIndex, input := range inputs {
		rt := input.GetResourceType()
		effectiveAmount := input.GetAmount()
		if shared.IsVariableAmount(input) {
			effectiveAmount = input.GetAmount() * a.selectedAmount
		}

		if effectiveAmount == 0 {
			continue
		}

		if IsStorageResourceType(rt) {
			a.player.Resources().AddToStorage(inputSources[inputIndex], -effectiveAmount)
			log.Debug("Deducted from card storage",
				slog.String("card_id", inputSources[inputIndex]),
				slog.String("resource_type", string(rt)),
				slog.Int("amount", effectiveAmount))
			continue
		}

		if shared.IsBasicPaymentResource(rt) {
			continue
		}

		if shared.IsProductionResourceType(rt) {
			a.player.Resources().AddProduction(map[shared.ResourceType]int{rt: -effectiveAmount})
			log.Debug("Deducted production", slog.String("type", string(rt)), slog.Int("amount", effectiveAmount))
		} else {
			a.player.Resources().Add(map[shared.ResourceType]int{rt: -effectiveAmount})
			log.Debug("Deducted resource", slog.String("type", string(rt)), slog.Int("amount", effectiveAmount))
		}
	}

	return nil
}

// validateInputAmount checks the player has enough of the given resource type.
func (a *BehaviorApplier) validateInputAmount(rt shared.ResourceType, amount int, resources shared.Resources) error {
	switch rt {
	case shared.ResourceCredit:
		if resources.Credits < amount {
			return fmt.Errorf("insufficient credits: need %d, have %d", amount, resources.Credits)
		}
	case shared.ResourceSteel:
		if resources.Steel < amount {
			return fmt.Errorf("insufficient steel: need %d, have %d", amount, resources.Steel)
		}
	case shared.ResourceTitanium:
		if resources.Titanium < amount {
			return fmt.Errorf("insufficient titanium: need %d, have %d", amount, resources.Titanium)
		}
	case shared.ResourcePlant:
		if resources.Plants < amount {
			return fmt.Errorf("insufficient plants: need %d, have %d", amount, resources.Plants)
		}
	case shared.ResourceEnergy:
		if resources.Energy < amount {
			return fmt.Errorf("insufficient energy: need %d, have %d", amount, resources.Energy)
		}
	case shared.ResourceHeat:
		if resources.Heat < amount {
			return fmt.Errorf("insufficient heat: need %d, have %d", amount, resources.Heat)
		}
	default:
		if shared.IsProductionResourceType(rt) {
			production := a.player.Resources().PermanentProduction()
			available := production.GetAmount(rt)
			if available-amount < shared.ProductionMinimum(rt) {
				return fmt.Errorf("insufficient %s: need %d, have %d", rt, amount, available)
			}
		}
	}
	return nil
}

// isStorageResourceType returns true for resource types that are stored on cards
func IsStorageResourceType(rt shared.ResourceType) bool {
	switch rt {
	case shared.ResourceMicrobe, shared.ResourceAnimal, shared.ResourceFloater,
		shared.ResourceScience, shared.ResourceAsteroid, shared.ResourceFighter, shared.ResourceDisease, shared.ResourceCamp, shared.ResourceCardResource:
		return true
	}
	return false
}

// isEffectOutputType returns true for output types that represent persistent effects
// rather than immediate resource gains (these get their own "Effect:" notification)
func isEffectOutputType(rt shared.ResourceType) bool {
	switch rt {
	case shared.ResourceDiscount, shared.ResourcePaymentSubstitute, shared.ResourceValueModifier,
		shared.ResourceGlobalParameterLenience, shared.ResourceIgnoreGlobalRequirements,
		shared.ResourceOceanAdjacencyBonus,
		shared.ResourceDefense, shared.ResourceActionReuse:
		return true
	}
	return false
}

// ApplyOutputs applies resource gains, production changes, global params, tile placements
// Returns error if required context (player/game) is missing for the operation
func (a *BehaviorApplier) ApplyOutputs(
	ctx context.Context,
	outputs []shared.BehaviorCondition,
) error {
	_, err := a.ApplyOutputsAndGetCalculated(ctx, outputs)
	return err
}

// ApplyOutputsAndGetCalculated applies outputs and returns the calculated values
// This is useful for logging scaled outputs (e.g., "+1 MC per 2 plant tags" becomes "+3 MC")
func (a *BehaviorApplier) ApplyOutputsAndGetCalculated(
	ctx context.Context,
	outputs []shared.BehaviorCondition,
) ([]shared.CalculatedOutput, error) {
	if len(outputs) == 0 {
		return nil, nil
	}
	if err := a.ValidateModeOutputs(outputs); err != nil {
		return nil, err
	}

	if err := a.ValidateResourceOutputs(outputs); err != nil {
		return nil, err
	}

	log := a.logger.With(
		slog.String("source", a.source),
		slog.Int("output_count", len(outputs)),
	)

	log.Debug("Processing behavior outputs")

	options, deferred, err := EffectSelectionOptions(outputs, a.player, a.game, a.cardRegistry, a.colonyBonusLookup)
	if err != nil {
		return nil, err
	}
	if deferred {
		if len(options) == 0 {
			return nil, fmt.Errorf("no legal effect selection")
		}
		a.player.Selection().SetPendingEffectSelection(&shared.PendingEffectSelection{Source: a.source, SourceCardID: a.sourceCardID, Outputs: outputs, Options: options})
		return nil, nil
	}

	if err := ValidateRevealOutputs(outputs, a.game, a.cardRegistry); err != nil {
		return nil, err
	}
	if _, err := a.ApplyCardDrawOutputs(ctx, outputs); err != nil {
		return nil, err
	}
	var calculatedOutputs []shared.CalculatedOutput
	var notificationOutputs []shared.CalculatedOutput

	for _, output := range outputs {
		if reveal, ok := output.(*shared.CardRevealCondition); ok {
			rewards, err := a.applyCardReveal(ctx, reveal)
			if err != nil {
				return nil, err
			}
			calculatedOutputs = append(calculatedOutputs, rewards...)
			continue
		}
		rt := output.GetResourceType()
		baseAmount := output.GetAmount()

		// Calculate the actual amount if this output has a Per condition
		actualAmount := baseAmount
		isScaled := false

		if per := shared.GetPerCondition(output); per != nil && a.player != nil && a.game != nil {
			count := a.countPerCondition(per)
			if per.Amount > 0 {
				actualAmount = shared.CalculateScaledAmount(output, count)
				isScaled = true
				log.Debug("Calculated scaled output",
					slog.String("resource_type", string(rt)),
					slog.Int("base_amount", baseAmount),
					slog.Int("count", count),
					slog.Int("per_amount", per.Amount),
					slog.Int("calculated_amount", actualAmount))
			}
		}

		// Apply variable amount multiplier (player-selected amount)
		if shared.IsVariableAmount(output) {
			actualAmount = baseAmount * a.selectedAmount
			isScaled = true
			log.Debug("Applied variable amount",
				slog.String("resource_type", string(rt)),
				slog.Int("base_amount", baseAmount),
				slog.Int("selected_amount", a.selectedAmount),
				slog.Int("calculated_amount", actualAmount))
		}

		if err := a.applyOutput(ctx, output, actualAmount, log); err != nil {
			return calculatedOutputs, err
		}

		if basic, ok := output.(*shared.BasicResourceCondition); ok && basic.Target == "any-player" && actualAmount < 0 && basic.TargetRestriction != nil {
			continue // The removal is logged only when its selection is confirmed.
		}

		// Colony-bonus outputs expand into the actual resources gained
		if rt == shared.ResourceColonyBonus {
			bonusOutputs := a.collectColonyBonusOutputs(log)
			calculatedOutputs = append(calculatedOutputs, bonusOutputs...)
			notificationOutputs = append(notificationOutputs, bonusOutputs...)
			continue
		}

		// Track for state diff log (existing behavior)
		if isScaled || actualAmount != 0 {
			calculatedOutputs = append(calculatedOutputs, shared.CalculatedOutput{
				ResourceType: string(rt),
				Amount:       actualAmount,
				IsScaled:     isScaled,
			})
		}

		// Track non-zero resource outputs for triggered effect notifications
		// Skip effect-type outputs (discount, payment-substitute, etc.) since they get
		// their own "Effect:" notification via SourceTypeEffectAdded
		if actualAmount != 0 && !isEffectOutputType(rt) {
			resourceType := string(rt)
			if resourceType == string(shared.ResourceCardResource) {
				resourceType = a.resolveCardResourceType(output.GetTarget())
			}
			notificationOutputs = append(notificationOutputs, shared.CalculatedOutput{
				ResourceType: resourceType,
				Amount:       actualAmount,
				IsScaled:     isScaled,
			})
		}
	}

	if a.game != nil && a.player != nil && len(outputs) > 0 {
		effect := shared.TriggeredEffect{
			CardName:          a.source,
			PlayerID:          a.player.ID(),
			SourceType:        a.sourceType,
			Outputs:           outputs,
			CalculatedOutputs: notificationOutputs,
		}
		if a.sourceType == shared.SourceTypePassiveEffect {
			a.game.AddOrMergeTriggeredEffect(effect)
		} else {
			a.game.AddTriggeredEffect(effect)
		}
	}

	if a.behavior != nil && a.player != nil && a.game != nil {
		if a.behavior.MaxUsesPerGeneration > 0 {
			usageID := a.behavior.ID
			if a.behavior.UsageGroup != "" {
				usageID = a.behavior.UsageGroup
			}
			a.player.Resources().RecordBehaviorUse(a.sourceCardID, usageID, a.game.Generation())
		}
		if a.behavior.MaxUsesPerGame > 0 {
			a.player.Resources().RecordBehaviorUse(a.sourceCardID, a.behavior.ID, 0)
		}
	}
	return calculatedOutputs, nil
}

// resolveCardResourceType resolves a generic storage reward to its destination resource type.
func (a *BehaviorApplier) resolveCardResourceType(target string) string {
	if a.cardRegistry == nil {
		return string(shared.ResourceCardResource)
	}
	targetID := a.triggeringCardID
	if target != "triggering-card" {
		if a.anyCardTargetIdx == 0 {
			return string(shared.ResourceCardResource)
		}
		targetID = a.targetCardIDs[a.anyCardTargetIdx-1]
	}
	targetCard, err := a.cardRegistry.GetByID(targetID)
	if err != nil || targetCard.ResourceStorage == nil {
		return string(shared.ResourceCardResource)
	}
	return string(targetCard.ResourceStorage.Type)
}

func (a *BehaviorApplier) tagCountContext() TagCountContext {
	if a.player == nil || (a.game != nil && a.game.CurrentPhase() != shared.GamePhaseAction) {
		return TagCountContext{}
	}
	switch a.sourceType {
	case shared.SourceTypeCardPlay:
		return TagCountContext{ActorID: a.player.ID(), ExcludeWildCardID: a.sourceCardID}
	case shared.SourceTypeCardAction, shared.SourceTypeCorporationFirstAction:
		return TagCountContext{ActorID: a.player.ID()}
	}
	return TagCountContext{}
}

func (a *BehaviorApplier) countPerCondition(per *shared.PerCondition) int {
	var colonies ColonyCounter
	if a.game != nil {
		colonies = a.game.Colonies()
	}
	var b *board.Board
	var allPlayers []*player.Player
	if a.game != nil {
		b = a.game.Board()
		allPlayers = a.game.GetAllPlayers()
	}
	return CountPerCondition(per, a.sourceCardID, a.player, b, a.cardRegistry, allPlayers, colonies, a.tagCountContext())
}

// ApplyCardDrawOutputs processes card-peek/take/buy outputs together
// Returns true if a pending selection was created (caller should defer action consumption)
func (a *BehaviorApplier) ApplyCardDrawOutputs(
	ctx context.Context,
	outputs []shared.BehaviorCondition,
) (bool, error) {
	log := a.logger.With(
		slog.String("source", a.source),
		slog.String("method", "ApplyCardDrawOutputs"),
	)

	// Scan outputs for card-peek, card-take, card-buy
	var peekAmount, takeAmount, buyAmount, minTakeAmount int
	var isPrelude bool
	for _, output := range outputs {
		switch output.GetResourceType() {
		case shared.ResourceCardPeek:
			peekAmount += output.GetAmount()
		case shared.ResourceCardTake:
			takeAmount += output.GetAmount()
			if !shared.IsOptional(output) {
				minTakeAmount += output.GetAmount()
			}
		case shared.ResourceCardBuy:
			buyAmount += output.GetAmount()
		}
		if co, ok := output.(*shared.CardOperationCondition); ok && hasPreludeCardType(co.Selectors) {
			isPrelude = true
		}
	}

	// If no card-peek found, nothing to do
	if peekAmount == 0 {
		return false, nil
	}

	if a.player == nil {
		return false, fmt.Errorf("cannot apply card draw outputs: no player context")
	}
	if a.game == nil {
		return false, fmt.Errorf("cannot apply card draw outputs: no game context")
	}

	if a.player.Selection().GetPendingCardDrawSelection() != nil {
		return false, fmt.Errorf("a card draw selection is already pending")
	}
	// Draw cards from the appropriate deck
	var drawnCards []string
	var err error
	if isPrelude {
		drawnCards, err = a.game.Deck().DrawPreludeCards(ctx, peekAmount)
	} else {
		drawnCards, err = a.game.Deck().DrawProjectCards(ctx, peekAmount)
	}
	if err != nil {
		return false, fmt.Errorf("failed to draw cards: %w", err)
	}

	log.Debug("Drew cards for peek selection",
		slog.Int("peek_amount", peekAmount),
		slog.Int("take_amount", takeAmount),
		slog.Int("buy_amount", buyAmount),
		slog.Bool("is_prelude", isPrelude),
		slog.Any("drawn_cards", drawnCards))

	// Calculate card buy cost (accounts for discounts like Polyphemos)
	cardBuyCost := 3
	if a.player != nil && a.cardRegistry != nil {
		calc := NewRequirementModifierCalculator(a.cardRegistry)
		discounts := calc.CalculateActionDiscounts(a.player, shared.ActionCardBuying)
		cardBuyCost = max(3-discounts[shared.ResourceCredit], 0)
	}

	// Create pending card draw selection
	selection := &shared.PendingCardDrawSelection{
		AvailableCards:      drawnCards,
		FreeTakeCount:       takeAmount,
		MinFreeTakeCount:    minTakeAmount,
		CompleteAction:      a.completeAction,
		MaxBuyCount:         buyAmount,
		CardBuyCost:         cardBuyCost,
		Source:              a.source,
		SourceCardID:        a.sourceCardID,
		SourceBehaviorIndex: a.sourceBehaviorIdx,
		PlayAsPrelude:       isPrelude,
	}

	// Set on player
	a.player.Selection().SetPendingCardDrawSelection(selection)

	log.Debug("Created pending card draw selection",
		slog.String("source", a.source),
		slog.String("source_card_id", a.sourceCardID),
		slog.Int("source_behavior_index", a.sourceBehaviorIdx),
		slog.Int("available_cards", len(drawnCards)),
		slog.Int("free_take", takeAmount),
		slog.Int("max_buy", buyAmount))

	return true, nil
}

// stealAnyPlayerResource removes resources from the target player and adds them to self
func (a *BehaviorApplier) stealAnyPlayerResource(
	resourceType shared.ResourceType,
	amount int,
	log *slog.Logger,
) error {
	if a.targetPlayerID == "" {
		log.Debug("Skipping steal: no target player (solo mode)",
			slog.String("resource_type", string(resourceType)))
		return nil
	}
	if a.game == nil {
		return fmt.Errorf("cannot steal resource: no game context")
	}
	if a.player == nil {
		return fmt.Errorf("cannot steal resource: no player context")
	}
	targetPlayer, err := a.game.GetPlayer(a.targetPlayerID)
	if err != nil {
		return fmt.Errorf("target player not found: %w", err)
	}

	if targetPlayer.HasExited() || IsResourceProtected(a.player, targetPlayer, resourceType, "") {
		return fmt.Errorf("%s are protected or unavailable", resourceType)
	}
	resources := targetPlayer.Resources().Get()
	var current int
	switch resourceType {
	case shared.ResourceCredit:
		current = resources.Credits
	case shared.ResourceSteel:
		current = resources.Steel
	case shared.ResourceTitanium:
		current = resources.Titanium
	case shared.ResourcePlant:
		current = resources.Plants
	case shared.ResourceEnergy:
		current = resources.Energy
	case shared.ResourceHeat:
		current = resources.Heat
	}

	stolenAmount := min(amount, current)

	if stolenAmount > 0 {
		targetPlayer.Resources().Add(map[shared.ResourceType]int{
			resourceType: -stolenAmount,
		})
		a.player.Resources().Add(map[shared.ResourceType]int{
			resourceType: stolenAmount,
		})
	}

	log.Debug("Stole resource from target player",
		slog.String("target_player_id", a.targetPlayerID),
		slog.String("resource_type", string(resourceType)),
		slog.Int("requested", amount),
		slog.Int("stolen", stolenAmount))
	return nil
}

// applyAnyPlayerResource removes resources from the target player (clamped to what they have)
func (a *BehaviorApplier) applyAnyPlayerResource(
	resourceType shared.ResourceType,
	amount int,
	log *slog.Logger,
) error {
	if a.targetPlayerID == "" {
		log.Debug("Skipping any-player resource removal: no target player (solo mode)",
			slog.String("resource_type", string(resourceType)))
		return nil
	}
	if a.game == nil {
		return fmt.Errorf("cannot apply any-player resource: no game context")
	}
	targetPlayer, err := a.game.GetPlayer(a.targetPlayerID)
	if err != nil {
		return fmt.Errorf("target player not found: %w", err)
	}

	if targetPlayer.HasExited() || IsResourceProtected(a.player, targetPlayer, resourceType, "") {
		return fmt.Errorf("%s are protected or unavailable", resourceType)
	}
	resources := targetPlayer.Resources().Get()
	var current int
	switch resourceType {
	case shared.ResourceCredit:
		current = resources.Credits
	case shared.ResourceSteel:
		current = resources.Steel
	case shared.ResourceTitanium:
		current = resources.Titanium
	case shared.ResourcePlant:
		current = resources.Plants
	case shared.ResourceEnergy:
		current = resources.Energy
	case shared.ResourceHeat:
		current = resources.Heat
	}

	// Card data uses negative amounts for removal (e.g., Deimos Down: amount=-8).
	// Normalize to positive for clamping.
	absAmount := amount
	if absAmount < 0 {
		absAmount = -absAmount
	}

	removeAmount := min(absAmount, current)

	if removeAmount > 0 {
		targetPlayer.Resources().Add(map[shared.ResourceType]int{
			resourceType: -removeAmount,
		})
	}

	log.Debug("Removed resource from target player",
		slog.String("target_player_id", a.targetPlayerID),
		slog.String("resource_type", string(resourceType)),
		slog.Int("requested", absAmount),
		slog.Int("removed", removeAmount))
	return nil
}

// applyAnyPlayerProduction applies production changes to the target player.
// Card data uses negative amounts for decreases (e.g., Asteroid Mining Consortium: amount=-1).
// The amount is applied directly via AddProduction (which handles clamping to minimums).
func (a *BehaviorApplier) applyAnyPlayerProduction(
	productionType shared.ResourceType,
	amount int,
	log *slog.Logger,
) error {
	if a.targetPlayerID == "" {
		log.Debug("Skipping any-player production change: no target player (solo mode)",
			slog.String("production_type", string(productionType)))
		return nil
	}
	if a.game == nil {
		return fmt.Errorf("cannot apply any-player production: no game context")
	}
	targetPlayer, err := a.game.GetPlayer(a.targetPlayerID)
	if err != nil {
		return fmt.Errorf("target player not found: %w", err)
	}

	targetPlayer.Resources().AddProduction(map[shared.ResourceType]int{
		productionType: amount,
	})

	log.Debug("Applied production change to target player",
		slog.String("target_player_id", a.targetPlayerID),
		slog.String("production_type", string(productionType)),
		slog.Int("amount", amount))
	return nil
}

// applyOutput dispatches a single output to the appropriate category handler.
// The amount parameter is the pre-calculated value (accounting for Per and VariableAmount).
func (a *BehaviorApplier) applyOutput(
	ctx context.Context,
	output shared.BehaviorCondition,
	amount int,
	log *slog.Logger,
) error {
	switch o := output.(type) {
	case *shared.StateCondition:
		return a.applyStateOutput(ctx, o)
	case *shared.PaymentSubstituteCondition:
		if problems := shared.ValidateResourceCondition(o, false); len(problems) > 0 {
			return fmt.Errorf("invalid payment substitute: %v", problems)
		}
		source := o.Source
		if source.Target == "self-card" {
			if a.cardRegistry == nil {
				return fmt.Errorf("payment source requires card registry")
			}
			card, err := a.cardRegistry.GetByID(a.sourceCardID)
			if err != nil || card.ResourceStorage == nil || card.ResourceStorage.Type != source.Resource {
				return fmt.Errorf("payment source does not match card storage")
			}
			source.CardID = a.sourceCardID
		}
		a.player.Resources().AddPaymentSubstitute(shared.PaymentSubstitute{Source: source, TargetResource: o.TargetResource, ConversionRate: amount, GrantedByCardID: a.sourceCardID, Selectors: shared.CloneSelectors(o.Selectors)})
		return nil
	case *shared.BasicResourceCondition:
		return a.applyBasicResourceOutput(ctx, o, amount, log)
	case *shared.ProductionCondition:
		return a.applyProductionOutput(ctx, o, amount, log)
	case *shared.GlobalParameterCondition:
		return a.applyGlobalParameterOutput(ctx, o, amount, log)
	case *shared.TilePlacementCondition:
		return a.applyTilePlacementOutput(ctx, o, amount, log)
	case *shared.EffectCondition:
		return a.applyEffectOutput(ctx, o, amount, log)
	case *shared.CardStorageCondition:
		return a.applyCardStorageOutput(ctx, o, amount, log)
	case *shared.CardOperationCondition:
		return a.applyCardOperationOutput(ctx, o, amount, log)
	case *shared.ColonyCondition:
		return a.applyColonyOutput(ctx, o, amount, log)
	case *shared.TileModificationCondition:
		return a.applyTileModificationOutput(ctx, o, amount, log)
	case *shared.MiscCondition:
		return a.applyMiscOutput(ctx, o, amount, log)
	default:
		return fmt.Errorf("unknown output condition type: %T", output)
	}
}

// applyColonyBonuses applies all colony bonuses for the player.
// Card-targeted resources (microbe, animal, floater) are queued for player selection.
func (a *BehaviorApplier) applyColonyBonuses(_ context.Context, log *slog.Logger) {
	bonuses := CollectColonyBonuses(a.player.ID(), a.game.Colonies().States(), a.colonyBonusLookup)

	pendingByType := map[string]int{}
	var pendingOrder []string

	for _, b := range bonuses {
		rt := shared.ResourceType(b.ResourceType)
		if IsStorageResourceType(rt) {
			if _, exists := pendingByType[b.ResourceType]; !exists {
				pendingOrder = append(pendingOrder, b.ResourceType)
			}
			pendingByType[b.ResourceType] += b.Amount
		} else {
			a.player.Resources().Add(map[shared.ResourceType]int{rt: b.Amount})
		}
		log.Debug("Applied colony bonus",
			slog.String("type", b.ResourceType),
			slog.Int("amount", b.Amount))
	}

	for _, rt := range pendingOrder {
		amount := pendingByType[rt]
		if !HasEligibleStorageCard(a.player, shared.ResourceType(rt), a.cardRegistry) {
			log.Debug("No eligible storage card for colony bonus, resources lost",
				slog.String("resource_type", rt),
				slog.Int("amount", amount))
			continue
		}
		a.player.Selection().AppendPendingColonyResource(shared.PendingColonyResourceSelection{
			ResourceType: rt,
			Amount:       amount,
			Source:       a.source,
			Reason:       "colony-bonus",
		})
	}
}

func (a *BehaviorApplier) collectColonyBonusOutputs(_ *slog.Logger) []shared.CalculatedOutput {
	if a.game == nil || a.player == nil || a.colonyBonusLookup == nil {
		return nil
	}
	return ColonyBonusesToCalculatedOutputs(
		CollectColonyBonuses(a.player.ID(), a.game.Colonies().States(), a.colonyBonusLookup),
	)
}

// ColonyBonusEntry represents a single colony bonus resource gain.
type ColonyBonusEntry struct {
	ResourceType string
	Amount       int
}

// CollectColonyBonuses iterates colony tile states and returns all bonuses for the given player.
func CollectColonyBonuses(playerID string, tileStates []*colony.ColonyState, lookup ColonyBonusLookup) []ColonyBonusEntry {
	if lookup == nil {
		return nil
	}
	var result []ColonyBonusEntry
	for _, ts := range tileStates {
		colonyCount := 0
		for _, ownerID := range ts.PlayerColonies {
			if ownerID == playerID {
				colonyCount++
			}
		}
		if colonyCount == 0 {
			continue
		}
		def, err := lookup.GetByID(ts.DefinitionID)
		if err != nil {
			continue
		}
		for i := 0; i < colonyCount; i++ {
			for _, bonus := range def.ColonyBonus {
				if bonus.Amount > 0 {
					result = append(result, ColonyBonusEntry{ResourceType: bonus.Type, Amount: bonus.Amount})
				}
			}
		}
	}
	return result
}

// ColonyBonusesToCalculatedOutputs aggregates colony bonus entries into calculated outputs by type.
func ColonyBonusesToCalculatedOutputs(bonuses []ColonyBonusEntry) []shared.CalculatedOutput {
	if len(bonuses) == 0 {
		return []shared.CalculatedOutput{}
	}
	totals := map[string]int{}
	var order []string
	for _, b := range bonuses {
		if _, exists := totals[b.ResourceType]; !exists {
			order = append(order, b.ResourceType)
		}
		totals[b.ResourceType] += b.Amount
	}
	outputs := make([]shared.CalculatedOutput, 0, len(totals))
	for _, rt := range order {
		outputs = append(outputs, shared.CalculatedOutput{ResourceType: rt, Amount: totals[rt]})
	}
	return outputs
}

// HasEligibleStorageCard checks if a player has any played card or corporation
// that can store the given resource type.
func HasEligibleStorageCard(p *player.Player, resourceType shared.ResourceType, cardRegistry CardRegistryInterface) bool {
	if cardRegistry == nil {
		return false
	}
	for _, cardID := range p.PlayedCards().Cards() {
		card, err := cardRegistry.GetByID(cardID)
		if err != nil {
			continue
		}
		if card.ResourceStorage != nil && card.ResourceStorage.Type == resourceType {
			return true
		}
	}
	if corpID := p.CorporationID(); corpID != "" {
		corp, err := cardRegistry.GetByID(corpID)
		if err == nil && corp.ResourceStorage != nil && corp.ResourceStorage.Type == resourceType {
			return true
		}
	}
	return false
}

// WithProductionBox identifies whether to record the resolved production of this behavior.
func (a *BehaviorApplier) WithProductionBox(mode string) *BehaviorApplier {
	a.productionBox = mode
	return a
}

// WithAwardRegistry supplies the available awards for free funding effects.
func (a *BehaviorApplier) WithAwardRegistry(registry award.AwardRegistry) *BehaviorApplier {
	a.awardRegistry = registry
	return a
}
