package contentplans

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestBuildPlanScheduleCreatesSharedItemsAndAccountVariants(t *testing.T) {
	from := time.Date(2026, time.July, 6, 0, 0, 0, 0, time.FixedZone("CEST", 2*60*60))
	to := time.Date(2026, time.July, 12, 23, 59, 0, 0, from.Location())

	result, err := BuildPlanSchedule(PreviewInput{
		From:     from,
		To:       to,
		Timezone: "Europe/Madrid",
		Blocks: []BlockInput{{
			AccountIDs:     []string{"acc_linkedin", "acc_instagram"},
			Weekdays:       []time.Weekday{time.Monday, time.Wednesday},
			Slots:          []string{"09:00", "17:30"},
			GenerateImages: true,
		}},
	})
	if err != nil {
		t.Fatalf("build schedule: %v", err)
	}
	if result.ItemCount != 4 {
		t.Fatalf("expected four shared editorial items, got %d", result.ItemCount)
	}
	if result.VariantCount != 8 || result.ImageCount != 8 {
		t.Fatalf("expected eight account variants and images, got variants=%d images=%d", result.VariantCount, result.ImageCount)
	}
	if got := result.Items[0].PlannedAt.Format(time.RFC3339); got != "2026-07-06T09:00:00+02:00" {
		t.Fatalf("unexpected first planned time %s", got)
	}
	if len(result.Items[0].AccountIDs) != 2 {
		t.Fatalf("expected both account variants on the shared item, got %#v", result.Items[0].AccountIDs)
	}
}

func TestBuildPlanScheduleRejectsMoreThanNinetyDays(t *testing.T) {
	from := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	_, err := BuildPlanSchedule(PreviewInput{
		From: from,
		To:   from.AddDate(0, 0, 90),
		Blocks: []BlockInput{{
			AccountIDs: []string{"acc_x"},
			Weekdays:   []time.Weekday{time.Monday},
			Slots:      []string{"09:00"},
		}},
	})
	if !errors.Is(err, ErrDateRangeTooLong) {
		t.Fatalf("expected ErrDateRangeTooLong, got %v", err)
	}
}

func TestBuildPlanScheduleRejectsMoreThanFiveHundredVariants(t *testing.T) {
	from := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	accounts := make([]string, 11)
	for i := range accounts {
		accounts[i] = fmt.Sprintf("account_%d", i)
	}
	_, err := BuildPlanSchedule(PreviewInput{
		From: from,
		To:   from.AddDate(0, 0, 60),
		Blocks: []BlockInput{{
			AccountIDs: accounts,
			Weekdays: []time.Weekday{
				time.Sunday, time.Monday, time.Tuesday, time.Wednesday,
				time.Thursday, time.Friday, time.Saturday,
			},
			Slots: []string{"09:00"},
		}},
	})
	if !errors.Is(err, ErrTooManyVariants) {
		t.Fatalf("expected ErrTooManyVariants, got %v", err)
	}
}

func TestBuildPlanScheduleRejectsCadenceWithNoDatesInRange(t *testing.T) {
	from := time.Date(2026, time.July, 6, 0, 0, 0, 0, time.UTC) // Monday.
	_, err := BuildPlanSchedule(PreviewInput{From: from, To: from.Add(23 * time.Hour), Blocks: []BlockInput{{AccountIDs: []string{"acc_x"}, Weekdays: []time.Weekday{time.Tuesday}, Slots: []string{"09:00"}}}})
	if !errors.Is(err, ErrNoPlannedItems) {
		t.Fatalf("expected ErrNoPlannedItems, got %v", err)
	}
}
