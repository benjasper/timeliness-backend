package date

import (
	"reflect"
	"testing"
	"time"
)

func TestFreeConstraint_Test(t *testing.T) {
	var constraintTests = []struct {
		in      Timespan
		allowed []Timespan
		out     []Timespan
	}{
		// Case one allowed, overflows start
		{
			Timespan{
				Start: timeDate(0, 0, 0, 7, 0, 0),
				End:   timeDate(0, 0, 0, 9, 0, 0)},
			[]Timespan{
				{
					Start: timeDate(0, 0, 0, 8, 00, 0),
					End:   timeDate(0, 0, 0, 16, 30, 0),
				},
			},
			[]Timespan{
				{
					Start: timeDate(0, 0, 0, 8, 0, 0),
					End:   timeDate(0, 0, 0, 9, 0, 0),
				},
			},
		},
		// Case one allowed, overflows end
		{
			Timespan{
				Start: timeDate(0, 0, 0, 15, 0, 0),
				End:   timeDate(0, 0, 0, 17, 15, 0)},
			[]Timespan{
				{
					Start: timeDate(0, 0, 0, 8, 00, 0),
					End:   timeDate(0, 0, 0, 16, 30, 0),
				},
			},
			[]Timespan{
				{
					Start: timeDate(0, 0, 0, 15, 0, 0),
					End:   timeDate(0, 0, 0, 16, 30, 0),
				},
			},
		},
		// Case one allowed, is contained in allowed
		{
			Timespan{
				Start: timeDate(0, 0, 0, 14, 0, 0),
				End:   timeDate(0, 0, 0, 16, 15, 0)},
			[]Timespan{
				{
					Start: timeDate(0, 0, 0, 8, 00, 0),
					End:   timeDate(0, 0, 0, 16, 30, 0),
				},
			},
			[]Timespan{
				{
					Start: timeDate(0, 0, 0, 14, 0, 0),
					End:   timeDate(0, 0, 0, 16, 15, 0),
				},
			},
		},
		// Case one allowed, not contained in allowed at all
		{
			Timespan{
				Start: timeDate(0, 0, 0, 6, 0, 0),
				End:   timeDate(0, 0, 0, 7, 15, 0)},
			[]Timespan{
				{
					Start: timeDate(0, 0, 0, 8, 00, 0),
					End:   timeDate(0, 0, 0, 16, 30, 0),
				},
			},
			nil,
		},
		// Case 2 allowed,
		{
			Timespan{
				Start: timeDate(0, 0, 0, 16, 0, 0),
				End:   timeDate(0, 0, 0, 19, 0, 0)},
			[]Timespan{
				{
					Start: timeDate(0, 0, 0, 8, 00, 0),
					End:   timeDate(0, 0, 0, 16, 30, 0),
				},
				{
					Start: timeDate(0, 0, 0, 18, 00, 0),
					End:   timeDate(0, 0, 0, 20, 00, 0),
				},
			},
			[]Timespan{
				{
					Start: timeDate(0, 0, 0, 16, 00, 0),
					End:   timeDate(0, 0, 0, 16, 30, 0),
				},
				{
					Start: timeDate(0, 0, 0, 18, 0, 0),
					End:   timeDate(0, 0, 0, 19, 0, 0),
				},
			},
		},
		// Case Spanning whole day,
		{
			Timespan{
				Start: timeDate(2020, 12, 12, 0, 1, 0),
				End:   timeDate(2020, 12, 12, 23, 59, 0)},
			[]Timespan{
				{
					Start: timeDate(0, 0, 0, 8, 00, 0),
					End:   timeDate(0, 0, 0, 16, 30, 0),
				},
				{
					Start: timeDate(0, 0, 0, 18, 00, 0),
					End:   timeDate(0, 0, 0, 20, 00, 0),
				},
			},
			[]Timespan{
				{
					Start: timeDate(2020, 12, 12, 8, 00, 0),
					End:   timeDate(2020, 12, 12, 16, 30, 0),
				},
				{
					Start: timeDate(2020, 12, 12, 18, 00, 0),
					End:   timeDate(2020, 12, 12, 20, 00, 0),
				},
			},
		},
		// Case Spanning containing one allowed and part of second allowed,
		{
			Timespan{
				Start: timeDate(2020, 12, 12, 8, 0, 0),
				End:   timeDate(2020, 12, 12, 19, 0, 0)},
			[]Timespan{
				{
					Start: timeDate(0, 0, 0, 8, 00, 0),
					End:   timeDate(0, 0, 0, 16, 30, 0),
				},
				{
					Start: timeDate(0, 0, 0, 18, 00, 0),
					End:   timeDate(0, 0, 0, 20, 00, 0),
				},
			},
			[]Timespan{
				{
					Start: timeDate(2020, 12, 12, 8, 00, 0),
					End:   timeDate(2020, 12, 12, 16, 30, 0),
				},
				{
					Start: timeDate(2020, 12, 12, 18, 00, 0),
					End:   timeDate(2020, 12, 12, 19, 00, 0),
				},
			},
		},
		// Timespan spans multiple days
		{
			Timespan{
				Start: timeDate(2020, 12, 12, 8, 0, 0),
				End:   timeDate(2020, 12, 13, 16, 0, 0)},
			[]Timespan{
				{
					Start: timeDate(0, 0, 0, 8, 00, 0),
					End:   timeDate(0, 0, 0, 16, 30, 0),
				},
				{
					Start: timeDate(0, 0, 0, 18, 00, 0),
					End:   timeDate(0, 0, 0, 20, 00, 0),
				},
			},
			[]Timespan{
				{
					Start: timeDate(2020, 12, 12, 8, 00, 0),
					End:   timeDate(2020, 12, 12, 16, 30, 0),
				},
				{
					Start: timeDate(2020, 12, 12, 18, 0, 0),
					End:   timeDate(2020, 12, 12, 20, 0, 0),
				},
				{
					Start: timeDate(2020, 12, 13, 8, 00, 0),
					End:   timeDate(2020, 12, 13, 16, 0, 0),
				},
			},
		},
		{
			Timespan{
				Start: timeDate(2021, 1, 1, 14, 0, 0),
				End:   timeDate(2021, 1, 3, 13, 0, 0),
			},
			[]Timespan{
				{
					Start: timeDate(0, 0, 0, 8, 00, 0),
					End:   timeDate(0, 0, 0, 16, 30, 0),
				},
			},
			[]Timespan{
				{
					Start: timeDate(2021, 1, 1, 14, 0, 0),
					End:   timeDate(2021, 1, 1, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 1, 2, 8, 0, 0),
					End:   timeDate(2021, 1, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 1, 3, 8, 0, 0),
					End:   timeDate(2021, 1, 3, 13, 0, 0),
				},
			},
		},
	}

	for index, tt := range constraintTests {
		t.Run("Case "+string(rune(index)), func(t *testing.T) {
			constraint := FreeConstraint{AllowedTimeSpans: tt.allowed, Location: getLocation()}
			result := constraint.Test(tt.in)
			if !reflect.DeepEqual(result, tt.out) {
				t.Errorf("got %v, want %v", result, tt.out)
			}
		})
	}
}

func TestRuleDuration_Test(t *testing.T) {
	var ruleTests = []struct {
		in   Timespan
		rule RuleDuration
		out  *Timespan
	}{
		// Case fits minimum
		{
			Timespan{
				Start: timeDate(2020, 12, 12, 7, 0, 0),
				End:   timeDate(2020, 12, 12, 9, 0, 0)},
			RuleDuration{Minimum: time.Hour * 2, Maximum: time.Hour * 6},
			&Timespan{
				Start: timeDate(2020, 12, 12, 7, 0, 0),
				End:   timeDate(2020, 12, 12, 9, 0, 0)},
		},
		// Case fits maximum
		{
			Timespan{
				Start: timeDate(2020, 12, 12, 7, 0, 0),
				End:   timeDate(2020, 12, 12, 13, 0, 0)},
			RuleDuration{Minimum: time.Hour * 2, Maximum: time.Hour * 6},
			&Timespan{
				Start: timeDate(2020, 12, 12, 7, 0, 0),
				End:   timeDate(2020, 12, 12, 13, 0, 0)},
		},
		// Case fits in between
		{
			Timespan{
				Start: timeDate(2020, 12, 12, 7, 0, 0),
				End:   timeDate(2020, 12, 12, 10, 0, 0)},
			RuleDuration{Minimum: time.Hour * 2, Maximum: time.Hour * 6},
			&Timespan{
				Start: timeDate(2020, 12, 12, 7, 0, 0),
				End:   timeDate(2020, 12, 12, 10, 0, 0)},
		},
		// Case maximum overflows
		{
			Timespan{
				Start: timeDate(2020, 12, 12, 7, 0, 0),
				End:   timeDate(2020, 12, 12, 16, 0, 0)},
			RuleDuration{Minimum: time.Hour * 2, Maximum: time.Hour * 6},
			&Timespan{
				Start: timeDate(2020, 12, 12, 7, 0, 0),
				End:   timeDate(2020, 12, 12, 13, 0, 0)},
		},
		// Case too small
		{
			Timespan{
				Start: timeDate(2020, 12, 12, 7, 0, 0),
				End:   timeDate(2020, 12, 12, 8, 0, 0)},
			RuleDuration{Minimum: time.Hour * 2, Maximum: time.Hour * 6},
			nil,
		},
	}

	for index, tt := range ruleTests {
		t.Run("Case "+string(rune(index)), func(t *testing.T) {
			tt := tt
			t.Parallel()
			result := tt.rule.Test(tt.in)
			if !reflect.DeepEqual(result, tt.out) {
				t.Errorf("got %v, want %v", result, tt.out)
			}
		})
	}
}
