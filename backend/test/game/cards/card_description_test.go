package cards_test

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"openmars/internal/delivery/dto"
	"openmars/internal/game/cards"
	"openmars/test/testutil"
)

func descriptionErrors(description cards.CardDescription) []error {
	card := &cards.Card{ID: "test", Type: cards.CardTypeAutomated, Description: description}
	var errs []error
	for _, err := range cards.ValidateCardJSON(card) {
		if strings.Contains(err.Error(), "description") {
			errs = append(errs, err)
		}
	}
	return errs
}

func TestCardDescriptionValidationAcceptsAllSectionTypes(t *testing.T) {
	description := cards.CardDescription{
		{Type: cards.DescriptionSectionGeneric, Text: "Gain 1 plant."},
		{Type: cards.DescriptionSectionEffect, Text: "Pay 2 M€ less for space cards."},
		{Type: cards.DescriptionSectionAction, Text: "Draw a card."},
		{Type: cards.DescriptionSectionRequirement, Text: "2 science tags."},
	}
	testutil.AssertEqual(t, 0, len(descriptionErrors(description)), "all section types should be valid")
	testutil.AssertEqual(t, 0, len(descriptionErrors(cards.CardDescription{})), "empty description should be valid")
}

func TestCardDescriptionValidationRejectsInvalidSections(t *testing.T) {
	testutil.AssertEqual(t, 1, len(descriptionErrors(cards.CardDescription{{Type: "flavor", Text: "Hello."}})), "unknown type should be rejected")
	testutil.AssertEqual(t, 1, len(descriptionErrors(cards.CardDescription{{Type: cards.DescriptionSectionEffect, Text: ""}})), "empty text should be rejected")
	testutil.AssertEqual(t, 1, len(descriptionErrors(cards.CardDescription{{Type: cards.DescriptionSectionAction, Text: "  \n"}})), "whitespace text should be rejected")
}

func TestCardDescriptionRejectsStringFormat(t *testing.T) {
	var card cards.Card
	err := json.Unmarshal([]byte(`{"id":"x","name":"X","type":"automated","description":"**Effect:** Old format."}`), &card)
	testutil.AssertErrorContains(t, err, "cannot unmarshal string", "string description should fail to unmarshal")
}

func TestCardDescriptionRoundTripsThroughDto(t *testing.T) {
	raw := `{"id":"x","name":"X","type":"active","description":[` +
		`{"type":"effect","text":"First **bold** effect."},` +
		`{"type":"action","text":"An action."},` +
		`{"type":"effect","text":"Second effect."},` +
		`{"type":"requirement","text":"3 oceans."}]}`
	var card cards.Card
	testutil.AssertNoError(t, json.Unmarshal([]byte(raw), &card), "unmarshal card")

	out, err := json.Marshal(dto.ToCardDto(card))
	testutil.AssertNoError(t, err, "marshal dto")
	var decoded struct {
		Description []dto.CardDescriptionSectionDto `json:"description"`
	}
	testutil.AssertNoError(t, json.Unmarshal(out, &decoded), "unmarshal dto")

	expected := []dto.CardDescriptionSectionDto{
		{Type: "effect", Text: "First **bold** effect."},
		{Type: "action", Text: "An action."},
		{Type: "effect", Text: "Second effect."},
		{Type: "requirement", Text: "3 oceans."},
	}
	testutil.AssertTrue(t, reflect.DeepEqual(expected, decoded.Description), "description should keep order and repeated types")
}

func TestCardDescriptionDtoIsEmptyArrayNotNull(t *testing.T) {
	out, err := json.Marshal(dto.ToCardDto(cards.Card{ID: "x", Description: cards.CardDescription{}}))
	testutil.AssertNoError(t, err, "marshal dto")
	testutil.AssertTrue(t, strings.Contains(string(out), `"description":[]`), "empty description should serialize as []")
}

func TestCardDescriptionPlainText(t *testing.T) {
	zoo := cards.CardDescription{
		{Type: cards.DescriptionSectionEffect, Text: "Place an animal here when you play an Earth tag."},
		{Type: cards.DescriptionSectionAction, Text: "Gain 1 M€ per animal here."},
		{Type: cards.DescriptionSectionRequirement, Text: "2 cities in play."},
	}
	testutil.AssertEqual(t,
		"Effect: Place an animal here when you play an Earth tag.\nAction: Gain 1 M€ per animal here.\nRequirement: 2 cities in play.",
		zoo.PlainText(), "labelled sections")

	generic := cards.CardDescription{{Type: cards.DescriptionSectionGeneric, Text: "Place an ocean **on an area not reserved for ocean**."}}
	testutil.AssertEqual(t, "Place an ocean **on an area not reserved for ocean**.", generic.PlainText(), "generic has no prefix")

	repeated := cards.CardDescription{
		{Type: cards.DescriptionSectionAction, Text: "A."},
		{Type: cards.DescriptionSectionAction, Text: "B."},
	}
	testutil.AssertEqual(t, "Action: A.\nAction: B.", repeated.PlainText(), "repeated types")
	testutil.AssertEqual(t, "", cards.CardDescription{}.PlainText(), "empty description")

	dtoSections := []dto.CardDescriptionSectionDto{{Type: "requirement", Text: "2 cities in play."}, {Type: "generic", Text: "Gain 1 plant."}}
	testutil.AssertEqual(t, "Requirement: 2 cities in play.\nGain 1 plant.", dto.CardDescriptionPlainText(dtoSections), "dto helper")
}

func TestCardDatabaseDescriptions(t *testing.T) {
	allCards, err := cards.LoadCardsFromJSON("../../../assets/cards.json")
	if err != nil {
		t.Fatalf("Failed to load cards: %v", err)
	}

	prefix := regexp.MustCompile(`^\**(Effect|Action|Requirements?|Requires)\b:?`)
	for _, card := range allCards {
		for _, err := range descriptionErrors(card.Description) {
			t.Errorf("Card %s (%s): %v", card.ID, card.Name, err)
		}
		if card.Description == nil {
			t.Errorf("Card %s (%s) has a null description; use []", card.ID, card.Name)
		}
		for i, section := range card.Description {
			if prefix.MatchString(section.Text) {
				t.Errorf("Card %s (%s) description[%d] repeats a generated prefix: %q", card.ID, card.Name, i, section.Text)
			}
			if strings.Count(section.Text, "**")%2 != 0 {
				t.Errorf("Card %s (%s) description[%d] has unbalanced emphasis: %q", card.ID, card.Name, i, section.Text)
			}
		}
		if card.Name == "Martian Zoo" {
			expected := cards.CardDescription{
				{Type: cards.DescriptionSectionEffect, Text: "Place an animal here when you play an Earth tag."},
				{Type: cards.DescriptionSectionAction, Text: "Gain 1 M€ per animal here."},
				{Type: cards.DescriptionSectionRequirement, Text: "2 cities in play."},
			}
			testutil.AssertTrue(t, reflect.DeepEqual(expected, card.Description), "Martian Zoo description should match the canonical example")
		}
	}
}

func TestCardDatabaseRequirementTextMatchesInEveryMode(t *testing.T) {
	allCards, err := cards.LoadCardsFromJSON("../../../assets/cards.json")
	if err != nil {
		t.Fatal(err)
	}
	prefix := regexp.MustCompile(`(?i)^(requires|requirements?)\b`)
	for _, card := range allCards {
		for _, modeID := range append([]string{""}, card.GameModes...) {
			t.Run(card.ID+"/"+modeID, func(t *testing.T) {
				effective, err := cards.ApplyModeModifiers(card, modeID)
				if err != nil {
					t.Fatal(err)
				}
				var sections []string
				for _, section := range effective.Description {
					if section.Type == cards.DescriptionSectionRequirement {
						sections = append(sections, section.Text)
					}
				}
				if effective.Requirements == nil {
					if len(sections) != 0 {
						t.Fatalf("requirement description has no requirement box: %q", sections)
					}
					return
				}
				text := effective.Requirements.Description
				if len(sections) != 1 || sections[0] != text {
					t.Fatalf("requirement box %q must match exactly one description section: %q", text, sections)
				}
				if text == "" || text != strings.TrimSpace(text) || !strings.HasSuffix(text, ".") {
					t.Errorf("requirement text must be nonempty, trimmed and end with a period: %q", text)
				}
				if prefix.MatchString(text) {
					t.Errorf("requirement text repeats a generated prefix: %q", text)
				}
			})
		}
	}
}

func TestCardDatabaseBehaviorDescriptionsStayScoped(t *testing.T) {
	allCards, err := cards.LoadCardsFromJSON("../../../assets/cards.json")
	testutil.AssertNoError(t, err, "Load cards")
	prefix := regexp.MustCompile(`(?i)^\**(effect|action)\b\**:`)
	payment := regexp.MustCompile(`(?i)\b(lose|spend|pay|remove)\b`)
	for _, card := range allCards {
		for _, modeID := range append([]string{""}, card.GameModes...) {
			effective, err := cards.ApplyModeModifiers(card, modeID)
			testutil.AssertNoError(t, err, "Resolve card mode")
			for _, section := range effective.Description {
				if section.Type == cards.DescriptionSectionRequirement && payment.MatchString(section.Text) {
					t.Errorf("%s/%s: payment instruction in requirement badge: %q", card.ID, modeID, section.Text)
				}
			}
			for _, behavior := range effective.Behaviors {
				if prefix.MatchString(behavior.Description) {
					t.Errorf("%s/%s/%s: redundant action/effect prefix in behavior tooltip", card.ID, modeID, behavior.ID)
				}
				manual := false
				for _, trigger := range behavior.Triggers {
					manual = manual || trigger.Type == "manual"
				}
				if !manual {
					continue
				}
				for _, section := range effective.Description {
					if section.Type != cards.DescriptionSectionGeneric && section.Type != cards.DescriptionSectionEffect {
						continue
					}
					text := strings.TrimSuffix(section.Text, ".")
					if len(text) < 20 || !strings.Contains(behavior.Description, text) {
						continue
					}
					alsoAction := false
					for _, action := range effective.Description {
						alsoAction = alsoAction || (action.Type == cards.DescriptionSectionAction && strings.Contains(action.Text, text))
					}
					if !alsoAction {
						t.Errorf("%s/%s/%s: manual tooltip repeats non-action text: %q", card.ID, modeID, behavior.ID, text)
					}
				}
			}
		}
	}
}
