package calendar

import (
	"reflect"
	"testing"
	"time"
)

func timeDate(year int, month time.Month, day int, hour int, min int, seconds int) time.Time {
	loc, _ := time.LoadLocation("Local")
	return time.Date(year, month, day, hour, min, seconds, 0, loc)
}
func getLocation() *time.Location {
	loc, _ := time.LoadLocation("Local")
	return loc
}

var timeWindowTests = []struct {
	in         TimeWindow
	constraint FreeConstraint
	out        []Timespan
}{
	{
		// Case single busy time
		TimeWindow{Start: timeDate(2020, 6, 10, 12, 30, 0), End: timeDate(2020, 6, 18, 12, 30, 0),
			Busy: []Timespan{{Start: timeDate(2020, 6, 10, 13, 0, 0), End: timeDate(2020, 6, 10, 14, 0, 0)}}},
		FreeConstraint{},
		[]Timespan{
			{Start: timeDate(2020, 6, 10, 12, 30, 0), End: timeDate(2020, 6, 10, 13, 0, 0)},
			{Start: timeDate(2020, 6, 10, 14, 0, 0), End: timeDate(2020, 6, 18, 12, 30, 0)}},
	},
	{
		// Case 2 busy time
		TimeWindow{Start: timeDate(2020, 6, 10, 12, 30, 0), End: timeDate(2020, 6, 18, 12, 30, 0),
			Busy: []Timespan{
				{Start: timeDate(2020, 6, 10, 13, 0, 0), End: timeDate(2020, 6, 10, 14, 0, 0)},
				{Start: timeDate(2020, 6, 10, 14, 30, 0), End: timeDate(2020, 6, 10, 15, 0, 0)}}},
		FreeConstraint{},
		[]Timespan{
			{Start: timeDate(2020, 6, 10, 12, 30, 0), End: timeDate(2020, 6, 10, 13, 0, 0)},
			{Start: timeDate(2020, 6, 10, 14, 0, 0), End: timeDate(2020, 6, 10, 14, 30, 0)},
			{Start: timeDate(2020, 6, 10, 15, 0, 0), End: timeDate(2020, 6, 18, 12, 30, 0)}},
	},
	{
		// Case 3 busy time
		TimeWindow{Start: timeDate(2020, 6, 10, 12, 30, 0), End: timeDate(2020, 6, 18, 12, 30, 0),
			Busy: []Timespan{
				{Start: timeDate(2020, 6, 10, 13, 0, 0), End: timeDate(2020, 6, 10, 14, 0, 0)},
				{Start: timeDate(2020, 6, 10, 14, 30, 0), End: timeDate(2020, 6, 10, 15, 0, 0)},
				{Start: timeDate(2020, 6, 12, 14, 30, 0), End: timeDate(2020, 6, 13, 15, 0, 0)},
			},
		},
		FreeConstraint{},
		[]Timespan{
			{Start: timeDate(2020, 6, 10, 12, 30, 0), End: timeDate(2020, 6, 10, 13, 0, 0)},
			{Start: timeDate(2020, 6, 10, 14, 0, 0), End: timeDate(2020, 6, 10, 14, 30, 0)},
			{Start: timeDate(2020, 6, 10, 15, 0, 0), End: timeDate(2020, 6, 12, 14, 30, 0)},
			{Start: timeDate(2020, 6, 13, 15, 0, 0), End: timeDate(2020, 6, 18, 12, 30, 0)}},
	},
	{
		// Case windows start is in busy time
		TimeWindow{Start: timeDate(2020, 6, 10, 12, 30, 0), End: timeDate(2020, 6, 18, 12, 30, 0),
			Busy: []Timespan{
				{Start: timeDate(2020, 6, 10, 12, 0, 0), End: timeDate(2020, 6, 10, 14, 0, 0)},
				{Start: timeDate(2020, 6, 10, 14, 30, 0), End: timeDate(2020, 6, 10, 15, 0, 0)},
				{Start: timeDate(2020, 6, 12, 14, 30, 0), End: timeDate(2020, 6, 13, 15, 0, 0)},
			},
		},
		FreeConstraint{},
		[]Timespan{
			{Start: timeDate(2020, 6, 10, 14, 0, 0), End: timeDate(2020, 6, 10, 14, 30, 0)},
			{Start: timeDate(2020, 6, 10, 15, 0, 0), End: timeDate(2020, 6, 12, 14, 30, 0)},
			{Start: timeDate(2020, 6, 13, 15, 0, 0), End: timeDate(2020, 6, 18, 12, 30, 0)}},
	},
	{
		// Case busy == 0
		TimeWindow{Start: timeDate(2020, 6, 10, 12, 30, 0), End: timeDate(2020, 6, 18, 12, 30, 0),
			Busy: nil,
		},
		FreeConstraint{},
		[]Timespan{{
			Start: timeDate(2020, 6, 10, 12, 30, 0), End: timeDate(2020, 6, 18, 12, 30, 0),
		}},
	},
	{
		// Case with free constraint
		TimeWindow{Start: timeDate(2021, 3, 1, 8, 30, 0), End: timeDate(2021, 3, 7, 17, 00, 0),
			Busy: []Timespan{
				{Start: timeDate(2021, 3, 1, 8, 30, 0), End: timeDate(2021, 3, 4, 23, 59, 0)},
				{Start: timeDate(2021, 3, 5, 8, 0, 0), End: timeDate(2021, 3, 5, 16, 30, 0)},
				{Start: timeDate(2021, 3, 6, 8, 0, 0), End: timeDate(2021, 3, 6, 9, 30, 0)},
			},
		},
		FreeConstraint{
			AllowedTimeSpans: []Timespan{{
				Start: time.Date(0, 0, 0, 8, 0, 0, 0, getLocation()),
				End:   time.Date(0, 0, 0, 16, 30, 0, 0, getLocation()),
			}}},
		[]Timespan{
			{Start: timeDate(2021, 3, 6, 9, 30, 0), End: timeDate(2021, 3, 6, 16, 30, 0)},
			{Start: timeDate(2021, 3, 7, 8, 0, 0), End: timeDate(2021, 3, 7, 16, 30, 0)},
		},
	},
}

func TestTimeWindow_ComputeFree(t *testing.T) {
	for index, tt := range timeWindowTests {
		t.Run("Case "+string(rune(index)), func(t *testing.T) {
			free := tt.in.ComputeFree(&tt.constraint)
			if !reflect.DeepEqual(free, tt.out) {
				t.Errorf("got (%d)%q, want (%d)%q", len(free), free, len(tt.out), tt.out)
			}
		})
	}
}

func TestTimespan_ContainsInClock_Test(t *testing.T) {
	var timespanContainsTests = []struct {
		container Timespan
		contained Timespan
		out       bool
	}{
		{
			// Case is contained
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 8, 12, 0),
				End:   timeDate(0, 0, 0, 10, 0, 0)},
			true,
		},
		{
			// Case is contained (2)
			Timespan{
				Start: timeDate(0, 0, 0, 8, 14, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 9, 12, 0),
				End:   timeDate(0, 0, 0, 10, 0, 0)},
			true,
		},
		{
			// Equal container
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 8, 00, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			true,
		},
		{
			// Start not contained
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 4, 00, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			false,
		},
		{
			// End not contained
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 9, 00, 0),
				End:   timeDate(0, 0, 0, 19, 0, 0)},
			false,
		},
		{
			// Both not contained
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 4, 00, 0),
				End:   timeDate(0, 0, 0, 19, 0, 0)},
			false,
		},
	}

	for index, tt := range timespanContainsTests {
		t.Run("Case "+string(rune(index)), func(t *testing.T) {
			tt := tt
			t.Parallel()
			result := tt.container.ContainsByClock(tt.contained)
			if result != tt.out {
				t.Errorf("got %v, want %v", result, tt.out)
			}
		})
	}
}

func TestTimespan_OverflowsStart(t *testing.T) {
	var timespanOverflowStartTests = []struct {
		container Timespan
		contained Timespan
		out       bool
	}{
		{
			// Case is contained
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 8, 12, 0),
				End:   timeDate(0, 0, 0, 10, 0, 0)},
			false,
		},
		{
			// Case is contained (2)
			Timespan{
				Start: timeDate(0, 0, 0, 8, 14, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 9, 12, 0),
				End:   timeDate(0, 0, 0, 10, 0, 0)},
			false,
		},
		{
			// Equal container
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			true,
		},
		{
			// Start not contained
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 4, 00, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			true,
		},
		{
			// End not contained
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 9, 00, 0),
				End:   timeDate(0, 0, 0, 19, 0, 0)},
			false,
		},
		{
			// Both not contained
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 4, 00, 0),
				End:   timeDate(0, 0, 0, 19, 0, 0)},
			false,
		},
	}

	for index, tt := range timespanOverflowStartTests {
		t.Run("Case "+string(rune(index)), func(t *testing.T) {
			tt := tt
			t.Parallel()
			result := tt.container.OverflowsStart(tt.contained)
			if result != tt.out {
				t.Errorf("got %v, want %v", result, tt.out)
			}
		})
	}
}

func TestTimespan_OverflowsEnd(t *testing.T) {
	var timespanOverflowEndTests = []struct {
		container Timespan
		contained Timespan
		out       bool
	}{
		{
			// Case is contained
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 8, 12, 0),
				End:   timeDate(0, 0, 0, 10, 0, 0)},
			false,
		},
		{
			// Case is contained (2)
			Timespan{
				Start: timeDate(0, 0, 0, 8, 14, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 9, 12, 0),
				End:   timeDate(0, 0, 0, 10, 0, 0)},
			false,
		},
		{
			// Equal container
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			true,
		},
		{
			// Start not contained
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 4, 00, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			false,
		},
		{
			// End not contained
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 9, 00, 0),
				End:   timeDate(0, 0, 0, 19, 0, 0)},
			true,
		},
		{
			// Both not contained
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 4, 00, 0),
				End:   timeDate(0, 0, 0, 19, 0, 0)},
			false,
		},
	}

	for index, tt := range timespanOverflowEndTests {
		t.Run("Case "+string(rune(index)), func(t *testing.T) {
			tt := tt
			t.Parallel()
			result := tt.container.OverflowsEnd(tt.contained)
			if result != tt.out {
				t.Errorf("got %v, want %v", result, tt.out)
			}
		})
	}
}

func TestTimespan_IntersectsWith(t *testing.T) {
	var timespanIntersectTests = []struct {
		container Timespan
		contained Timespan
		out       bool
	}{
		{
			// Case is contained
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 8, 12, 0),
				End:   timeDate(0, 0, 0, 10, 0, 0)},
			true,
		},
		{
			// Case is contained (2)
			Timespan{
				Start: timeDate(0, 0, 0, 8, 14, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 9, 12, 0),
				End:   timeDate(0, 0, 0, 10, 0, 0)},
			true,
		},
		{
			// Equal container
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			true,
		},
		{
			// Start not contained
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 4, 00, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			true,
		},
		{
			// End not contained
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 9, 00, 0),
				End:   timeDate(0, 0, 0, 19, 0, 0)},
			true,
		},
		{
			// Both not contained
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 4, 00, 0),
				End:   timeDate(0, 0, 0, 19, 0, 0)},
			true,
		},
		{
			// Outside container before
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 4, 0, 0),
				End:   timeDate(0, 0, 0, 6, 0, 0)},
			false,
		},
		{
			// Outside container after
			Timespan{
				Start: timeDate(0, 0, 0, 8, 0, 0),
				End:   timeDate(0, 0, 0, 18, 0, 0)},
			Timespan{
				Start: timeDate(0, 0, 0, 19, 0, 0),
				End:   timeDate(0, 0, 0, 22, 0, 0)},
			false,
		},
	}

	for index, tt := range timespanIntersectTests {
		t.Run("Case "+string(rune(index)), func(t *testing.T) {
			tt := tt
			t.Parallel()
			result := tt.container.IntersectsWith(tt.contained)
			if result != tt.out {
				t.Errorf("got %v, want %v", result, tt.out)
			}
		})
	}
}

func TestTimespan_SplitByDays(t *testing.T) {
	timespan := Timespan{Start: timeDate(2020, 12, 12, 12, 30, 0),
		End: timeDate(2020, 12, 13, 15, 30, 0)}

	want1 := []Timespan{
		{
			Start: timeDate(2020, 12, 12, 12, 30, 0),
			End:   timeDate(2020, 12, 12, 23, 59, 59),
		},
		{
			Start: timeDate(2020, 12, 13, 00, 0, 0),
			End:   timeDate(2020, 12, 13, 15, 30, 0),
		},
	}

	result1 := timespan.SplitByDays()
	if !reflect.DeepEqual(result1, want1) {
		t.Errorf("1) %v not equal to %v", result1, want1)
	}

	timespan2 := Timespan{Start: timeDate(2020, 12, 12, 12, 30, 0),
		End: timeDate(2020, 12, 12, 15, 30, 0)}

	want2 := []Timespan{
		{
			Start: timeDate(2020, 12, 12, 12, 30, 0),
			End:   timeDate(2020, 12, 12, 15, 30, 00),
		},
	}

	result2 := timespan2.SplitByDays()
	if !reflect.DeepEqual(result2, want2) {
		t.Errorf("1) %v not equal to %v", result1, want1)
	}
}

func TestTimeWindow_FindTimeSlot(t *testing.T) {
	var ruleTests = []struct {
		in   []Timespan
		rule RuleInterface
		out  *Timespan
	}{
		// Case fits minimum
		{
			[]Timespan{
				{
					Start: timeDate(2020, 12, 12, 8, 0, 0),
					End:   timeDate(2020, 12, 12, 9, 0, 0),
				},
				{
					Start: timeDate(2020, 12, 12, 9, 0, 0),
					End:   timeDate(2020, 12, 12, 12, 0, 0),
				},
				{
					Start: timeDate(2020, 12, 13, 8, 0, 0),
					End:   timeDate(2020, 12, 13, 10, 0, 0),
				},
			},
			&RuleDuration{Minimum: time.Hour * 2, Maximum: time.Hour * 6},
			&Timespan{
				Start: timeDate(2020, 12, 12, 9, 0, 0),
				End:   timeDate(2020, 12, 12, 12, 0, 0),
			},
		},
		// Case no time slot
		{
			[]Timespan{
				{
					Start: timeDate(2020, 12, 12, 8, 0, 0),
					End:   timeDate(2020, 12, 12, 9, 0, 0),
				},
				{
					Start: timeDate(2020, 12, 12, 9, 0, 0),
					End:   timeDate(2020, 12, 12, 9, 30, 0),
				},
			},
			&RuleDuration{Minimum: time.Hour * 2, Maximum: time.Hour * 6},
			nil,
		},
	}

	for index, tt := range ruleTests {
		t.Run("Case "+string(rune(index)), func(t *testing.T) {
			tt := tt
			t.Parallel()
			window := TimeWindow{
				Free: tt.in,
			}
			var rules = []RuleInterface{tt.rule}
			result := window.FindTimeSlot(&rules)
			if !reflect.DeepEqual(result, tt.out) {
				t.Errorf("got %v, want %v", result, tt.out)
			}
		})
	}
}
