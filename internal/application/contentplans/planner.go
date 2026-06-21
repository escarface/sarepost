package contentplans

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	MaxPlanDays     = 90
	MaxPlanVariants = 500
)

var (
	ErrDateRangeTooLong = errors.New("content plan date range exceeds 90 days")
	ErrTooManyVariants  = errors.New("content plan exceeds 500 variants")
	ErrNoPlannedItems   = errors.New("content plan cadence produces no items in the selected range")
)

type BlockInput struct {
	BrandProfileID string
	CampaignID     string
	AccountIDs     []string
	Instructions   string
	Weekdays       []time.Weekday
	Slots          []string
	GenerateImages bool
	ImagePrompt    string
	ForceWebSearch bool
}

type PreviewInput struct {
	From     time.Time
	To       time.Time
	Timezone string
	Blocks   []BlockInput
}

type PlannedItem struct {
	BlockIndex int
	PlannedAt  time.Time
	AccountIDs []string
}

type Preview struct {
	ItemCount    int           `json:"item_count"`
	VariantCount int           `json:"variant_count"`
	ImageCount   int           `json:"image_count"`
	Items        []PlannedItem `json:"items,omitempty"`
}

func BuildPlanSchedule(in PreviewInput) (Preview, error) {
	if in.From.IsZero() || in.To.IsZero() || in.To.Before(in.From) {
		return Preview{}, errors.New("valid from and to dates are required")
	}
	location := in.From.Location()
	if name := strings.TrimSpace(in.Timezone); name != "" {
		loaded, err := time.LoadLocation(name)
		if err != nil {
			return Preview{}, fmt.Errorf("invalid timezone %q", name)
		}
		location = loaded
	}
	from := in.From.In(location)
	to := in.To.In(location)
	fromDate := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, location)
	toDate := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, location)
	days := 0
	for date := fromDate; !date.After(toDate); date = date.AddDate(0, 0, 1) {
		days++
		if days > MaxPlanDays {
			break
		}
	}
	if days > MaxPlanDays {
		return Preview{}, ErrDateRangeTooLong
	}

	var out Preview
	for blockIndex, block := range in.Blocks {
		accounts := uniqueNonEmpty(block.AccountIDs)
		if len(accounts) == 0 {
			return Preview{}, fmt.Errorf("block %d requires at least one account", blockIndex+1)
		}
		weekdays := make(map[time.Weekday]bool, len(block.Weekdays))
		for _, weekday := range block.Weekdays {
			weekdays[weekday] = true
		}
		if len(weekdays) == 0 {
			return Preview{}, fmt.Errorf("block %d requires at least one weekday", blockIndex+1)
		}
		slots, err := normalizeSlots(block.Slots)
		if err != nil {
			return Preview{}, fmt.Errorf("block %d: %w", blockIndex+1, err)
		}
		for date := fromDate; !date.After(toDate); date = date.AddDate(0, 0, 1) {
			if !weekdays[date.Weekday()] {
				continue
			}
			for _, slot := range slots {
				plannedAt := time.Date(date.Year(), date.Month(), date.Day(), slot.Hour(), slot.Minute(), 0, 0, location)
				if plannedAt.Before(from) || plannedAt.After(to) {
					continue
				}
				out.Items = append(out.Items, PlannedItem{BlockIndex: blockIndex, PlannedAt: plannedAt, AccountIDs: append([]string(nil), accounts...)})
				out.ItemCount++
				out.VariantCount += len(accounts)
				if block.GenerateImages {
					out.ImageCount += len(accounts)
				}
				if out.VariantCount > MaxPlanVariants {
					return Preview{}, ErrTooManyVariants
				}
			}
		}
	}
	if out.ItemCount == 0 {
		return Preview{}, ErrNoPlannedItems
	}
	return out, nil
}

func normalizeSlots(raw []string) ([]time.Time, error) {
	seen := make(map[string]bool, len(raw))
	values := make([]string, 0, len(raw))
	for _, value := range raw {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		if _, err := time.Parse("15:04", value); err != nil {
			return nil, fmt.Errorf("invalid slot %q", value)
		}
		seen[value] = true
		values = append(values, value)
	}
	if len(values) == 0 {
		return nil, errors.New("at least one slot is required")
	}
	sort.Strings(values)
	out := make([]time.Time, 0, len(values))
	for _, value := range values {
		parsed, _ := time.Parse("15:04", value)
		out = append(out, parsed)
	}
	return out, nil
}

func uniqueNonEmpty(raw []string) []string {
	seen := make(map[string]bool, len(raw))
	out := make([]string, 0, len(raw))
	for _, value := range raw {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
