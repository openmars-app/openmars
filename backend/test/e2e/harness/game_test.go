package harness

import (
	"slices"
	"testing"

	"openmars/internal/delivery/dto"
)

func TestCardsToTakeSkipsHandPlayOnlyWhenPlayingPreludes(t *testing.T) {
	t.Parallel()
	cards := []dto.PlayerCardDto{
		{ID: "hand-play", Behaviors: []dto.CardBehaviorDto{{Outputs: []any{map[string]any{"type": "card-play"}}}}},
		{ID: "first-supported"},
		{ID: "second-supported"},
	}
	for _, tc := range []struct {
		name    string
		prelude bool
		count   int
		want    []string
	}{
		{"prelude", true, 1, []string{"first-supported"}},
		{"multiple preludes", true, 2, []string{"first-supported", "second-supported"}},
		{"ordinary draw", false, 1, []string{"hand-play"}},
		{"no free cards", true, 0, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selection := &dto.PendingCardDrawSelectionDto{AvailableCards: cards, FreeTakeCount: tc.count, PlayAsPrelude: tc.prelude}
			if got := cardsToTake(selection); !slices.Equal(got, tc.want) {
				t.Fatalf("cards to take = %v, want %v", got, tc.want)
			}
		})
	}
	if cards[0].ID != "hand-play" {
		t.Fatal("selection reordered the offered cards")
	}
}
