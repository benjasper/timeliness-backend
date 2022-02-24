package date

import (
	"fmt"
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

func TestTimeWindow_ComputeFree(t *testing.T) {
	var timeWindowTests = []struct {
		in         *TimeWindow
		constraint FreeConstraint
		out        []Timespan
	}{
		{
			// Case single busy time
			&TimeWindow{Start: timeDate(2020, 6, 10, 12, 30, 0), End: timeDate(2020, 6, 18, 12, 30, 0),
				busy: []Timespan{{Start: timeDate(2020, 6, 10, 13, 0, 0), End: timeDate(2020, 6, 10, 14, 0, 0)}}},
			FreeConstraint{},
			[]Timespan{
				{Start: timeDate(2020, 6, 10, 12, 30, 0), End: timeDate(2020, 6, 10, 13, 0, 0)},
				{Start: timeDate(2020, 6, 10, 14, 0, 0), End: timeDate(2020, 6, 18, 12, 30, 0)}},
		},
		{
			// Case 2 busy time
			&TimeWindow{Start: timeDate(2020, 6, 10, 12, 30, 0), End: timeDate(2020, 6, 18, 12, 30, 0),
				busy: []Timespan{
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
			&TimeWindow{Start: timeDate(2020, 6, 10, 12, 30, 0), End: timeDate(2020, 6, 18, 12, 30, 0),
				busy: []Timespan{
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
			&TimeWindow{Start: timeDate(2020, 6, 10, 12, 30, 0), End: timeDate(2020, 6, 18, 12, 30, 0),
				busy: []Timespan{
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
			&TimeWindow{Start: timeDate(2020, 6, 10, 12, 30, 0), End: timeDate(2020, 6, 18, 12, 30, 0),
				busy: nil,
			},
			FreeConstraint{},
			[]Timespan{{
				Start: timeDate(2020, 6, 10, 12, 30, 0), End: timeDate(2020, 6, 18, 12, 30, 0),
			}},
		},
		{
			// Case with free constraint
			&TimeWindow{Start: timeDate(2021, 3, 1, 8, 30, 0), End: timeDate(2021, 3, 7, 17, 00, 0),
				busy: []Timespan{
					{Start: timeDate(2021, 3, 1, 8, 30, 0), End: timeDate(2021, 3, 4, 23, 59, 0)},
					{Start: timeDate(2021, 3, 5, 8, 0, 0), End: timeDate(2021, 3, 5, 16, 30, 0)},
					{Start: timeDate(2021, 3, 6, 8, 0, 0), End: timeDate(2021, 3, 6, 9, 30, 0)},
				},
			},
			FreeConstraint{
				Location: getLocation(),
				AllowedTimeSpans: []Timespan{{
					Start: time.Date(0, 0, 0, 8, 0, 0, 0, getLocation()),
					End:   time.Date(0, 0, 0, 16, 30, 0, 0, getLocation()),
				}}},
			[]Timespan{
				{Start: timeDate(2021, 3, 6, 9, 30, 0), End: timeDate(2021, 3, 6, 16, 30, 0)},
				{Start: timeDate(2021, 3, 7, 8, 0, 0), End: timeDate(2021, 3, 7, 16, 30, 0)},
			},
		},
		{
			// Case with two free constraints
			&TimeWindow{Start: timeDate(2021, 3, 5, 8, 30, 0), End: timeDate(2021, 3, 7, 18, 00, 0),
				busy: []Timespan{
					{Start: timeDate(2021, 3, 5, 8, 0, 0), End: timeDate(2021, 3, 5, 16, 30, 0)},
					{Start: timeDate(2021, 3, 6, 8, 0, 0), End: timeDate(2021, 3, 6, 9, 30, 0)},
				},
			},
			FreeConstraint{
				Location: getLocation(),
				AllowedTimeSpans: []Timespan{
					{
						Start: time.Date(0, 0, 0, 8, 0, 0, 0, getLocation()),
						End:   time.Date(0, 0, 0, 16, 30, 0, 0, getLocation()),
					},
					{
						Start: time.Date(0, 0, 0, 17, 0, 0, 0, getLocation()),
						End:   time.Date(0, 0, 0, 18, 0, 0, 0, getLocation()),
					},
				}},
			[]Timespan{
				{Start: timeDate(2021, 3, 5, 17, 0, 0), End: timeDate(2021, 3, 5, 18, 0, 0)},
				{Start: timeDate(2021, 3, 6, 9, 30, 0), End: timeDate(2021, 3, 6, 16, 30, 0)},
				{Start: timeDate(2021, 3, 6, 17, 0, 0), End: timeDate(2021, 3, 6, 18, 0, 0)},
				{Start: timeDate(2021, 3, 7, 8, 0, 0), End: timeDate(2021, 3, 7, 16, 30, 0)},
				{Start: timeDate(2021, 3, 7, 17, 0, 0), End: timeDate(2021, 3, 7, 18, 0, 0)},
			},
		},
	}

	for index, tt := range timeWindowTests {
		t.Run("Case "+string(rune(index)), func(t *testing.T) {
			free := tt.in.ComputeFree(&tt.constraint, tt.in.Start, Timespan{Start: tt.in.Start, End: tt.in.End})
			if !reflect.DeepEqual(free, tt.out) {
				t.Errorf("got (%d)%q, want (%d)%q", len(free), free, len(tt.out), tt.out)
			}
		})
	}
}

func TestTimeWindow_AddToBusy(t *testing.T) {
	var timespanAddToBusyTests = []struct {
		input  Timespan
		window *TimeWindow
		output []Timespan
	}{
		{
			// Case 0 new is before all old ones
			input: Timespan{
				Start: timeDate(2021, 2, 28, 9, 30, 0),
				End:   timeDate(2021, 2, 28, 16, 30, 0),
			},
			window: &TimeWindow{busy: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			}},
			output: []Timespan{
				{
					Start: timeDate(2021, 2, 28, 9, 30, 0),
					End:   timeDate(2021, 2, 28, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			},
		},
		{
			// Case 1 new is after all old ones
			input: Timespan{
				Start: timeDate(2021, 3, 7, 9, 30, 0),
				End:   timeDate(2021, 3, 7, 16, 30, 0),
			},
			window: &TimeWindow{busy: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			}},
			output: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 7, 9, 30, 0),
					End:   timeDate(2021, 3, 7, 16, 30, 0),
				},
			},
		},
		{
			// Case 2 new is contained by existing timeslot
			input: Timespan{
				Start: timeDate(2021, 3, 1, 9, 30, 0),
				End:   timeDate(2021, 3, 1, 16, 30, 0),
			},
			window: &TimeWindow{busy: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			}},
			output: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			},
		},
		{
			// Case 3 existing is contained by new without other intersections
			input: Timespan{
				Start: timeDate(2021, 3, 2, 17, 30, 0),
				End:   timeDate(2021, 3, 4, 17, 30, 0),
			},
			window: &TimeWindow{busy: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			}},
			output: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 2, 17, 30, 0),
					End:   timeDate(2021, 3, 4, 17, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			},
		},
		{
			// Case 4 new timeslot between two existing ones
			input: Timespan{
				Start: timeDate(2021, 3, 2, 17, 30, 0),
				End:   timeDate(2021, 3, 3, 7, 30, 0),
			},
			window: &TimeWindow{busy: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			}},
			output: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 2, 17, 30, 0),
					End:   timeDate(2021, 3, 3, 7, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			},
		},
		{
			// Case 5 overlaps multiple whole timeslot and intersects with two others
			input: Timespan{
				Start: timeDate(2021, 3, 1, 10, 30, 0),
				End:   timeDate(2021, 3, 5, 10, 30, 0),
			},
			window: &TimeWindow{busy: []Timespan{
				{
					Start: timeDate(2021, 2, 6, 9, 30, 0),
					End:   timeDate(2021, 2, 7, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 4, 17, 30, 0),
					End:   timeDate(2021, 3, 5, 7, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 7, 9, 30, 0),
					End:   timeDate(2021, 3, 8, 16, 30, 0),
				},
			},
			},
			output: []Timespan{
				{
					Start: timeDate(2021, 2, 6, 9, 30, 0),
					End:   timeDate(2021, 2, 7, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 7, 9, 30, 0),
					End:   timeDate(2021, 3, 8, 16, 30, 0),
				},
			},
		},
		{
			// Case 6 intersects with 2 timeslots
			input: Timespan{
				Start: timeDate(2021, 3, 2, 10, 30, 0),
				End:   timeDate(2021, 3, 3, 10, 30, 0),
			},
			window: &TimeWindow{busy: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			}},
			output: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			},
		},
		{
			// Case 7 timeslot overlaps all others
			input: Timespan{
				Start: timeDate(2021, 2, 2, 10, 30, 0),
				End:   timeDate(2021, 4, 3, 10, 30, 0),
			},
			window: &TimeWindow{busy: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			}},
			output: []Timespan{
				{
					Start: timeDate(2021, 2, 2, 10, 30, 0),
					End:   timeDate(2021, 4, 3, 10, 30, 0),
				},
			},
		},
		{
			// Case 8 timeslot overlaps all others except the last
			input: Timespan{
				Start: timeDate(2021, 3, 1, 9, 30, 0),
				End:   timeDate(2021, 4, 3, 10, 30, 0),
			},
			window: &TimeWindow{busy: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			}},
			output: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 4, 3, 10, 30, 0),
				},
			},
		},
		{
			// Case 9 timeslot overlaps all others except the first one
			input: Timespan{
				Start: timeDate(2021, 2, 2, 10, 30, 0),
				End:   timeDate(2021, 3, 6, 16, 30, 0),
			},
			window: &TimeWindow{busy: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			}},
			output: []Timespan{
				{
					Start: timeDate(2021, 2, 2, 10, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			},
		},
		{
			// Case 10 timeslot fits exactly into gap
			input: Timespan{
				Start: timeDate(2021, 3, 2, 16, 30, 0),
				End:   timeDate(2021, 3, 3, 9, 30, 0),
			},
			window: &TimeWindow{busy: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			}},
			output: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			},
		},
		{
			// Case 11 timeslot fits exactly into gap but overlaps next one
			input: Timespan{
				Start: timeDate(2021, 3, 2, 16, 30, 0),
				End:   timeDate(2021, 3, 3, 12, 30, 0),
			},
			window: &TimeWindow{busy: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			}},
			output: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			},
		},
		{
			// Case 12 timeslot fits exactly into gap but overlaps at the start
			input: Timespan{
				Start: timeDate(2021, 3, 2, 15, 30, 0),
				End:   timeDate(2021, 3, 3, 9, 30, 0),
			},
			window: &TimeWindow{busy: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			}},
			output: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			},
		},
		{
			// Case 13 timeslot extends the start
			input: Timespan{
				Start: timeDate(2021, 2, 2, 15, 30, 0),
				End:   timeDate(2021, 3, 1, 9, 30, 0),
			},
			window: &TimeWindow{busy: []Timespan{
				{
					Start: timeDate(2021, 3, 1, 9, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			}},
			output: []Timespan{
				{
					Start: timeDate(2021, 2, 2, 15, 30, 0),
					End:   timeDate(2021, 3, 2, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 3, 9, 30, 0),
					End:   timeDate(2021, 3, 4, 16, 30, 0),
				},
				{
					Start: timeDate(2021, 3, 5, 9, 30, 0),
					End:   timeDate(2021, 3, 6, 16, 30, 0),
				},
			},
		},
	}

	for index, tt := range timespanAddToBusyTests {
		t.Run(fmt.Sprintf("Case %d", index), func(t *testing.T) {
			tt := tt
			t.Parallel()

			tt.window.AddToBusy(tt.input)
			if !reflect.DeepEqual(tt.output, tt.window.busy) {
				t.Errorf("got %v, want %v", tt.window.busy, tt.output)
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
		t.Run(fmt.Sprintf("Case %d", index), func(t *testing.T) {
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
			result := tt.container.OverflowsStartClock(tt.contained)
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
			result := tt.container.OverflowsEndClock(tt.contained)
			if result != tt.out {
				t.Errorf("got %v, want %v", result, tt.out)
			}
		})
	}
}

func TestTimespan_IntersectsWithClock(t *testing.T) {
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
			result := tt.container.IntersectsWithClock(tt.contained)
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
				Start: timeDate(2021, 7, 14, 14, 0, 0),
				End:   timeDate(2021, 7, 14, 15, 0, 0)},
			Timespan{
				Start: timeDate(2021, 7, 14, 14, 30, 0),
				End:   timeDate(2021, 7, 14, 14, 45, 0)},
			true,
		},
		{
			// Case overflows both
			Timespan{
				Start: timeDate(2021, 7, 14, 14, 0, 0),
				End:   timeDate(2021, 7, 14, 15, 0, 0)},
			Timespan{
				Start: timeDate(2021, 7, 14, 13, 30, 0),
				End:   timeDate(2021, 7, 14, 16, 45, 0)},
			true,
		},
		{
			// Overflows left
			Timespan{
				Start: timeDate(2021, 7, 14, 14, 0, 0),
				End:   timeDate(2021, 7, 14, 15, 0, 0)},
			Timespan{
				Start: timeDate(2021, 7, 14, 12, 30, 0),
				End:   timeDate(2021, 7, 14, 14, 45, 0)},
			true,
		},
		{
			// Overflows right
			Timespan{
				Start: timeDate(2021, 7, 14, 14, 0, 0),
				End:   timeDate(2021, 7, 14, 15, 0, 0)},
			Timespan{
				Start: timeDate(2021, 7, 14, 14, 30, 0),
				End:   timeDate(2021, 7, 14, 14, 45, 0)},
			true,
		},
		{
			// not contained
			Timespan{
				Start: timeDate(2021, 7, 14, 14, 0, 0),
				End:   timeDate(2021, 7, 14, 15, 0, 0)},
			Timespan{
				Start: timeDate(2021, 7, 15, 14, 0, 0),
				End:   timeDate(2021, 7, 15, 15, 0, 0)},
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

func TestTimeWindow_FindTimeSlot(t *testing.T) {
	var ruleTests = []struct {
		in   []Timespan
		rule *RuleDuration
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
				free:              tt.in,
				MaxWorkUnitLength: time.Hour * 6,
			}

			result := window.FindTimeSlot(tt.rule)
			if !reflect.DeepEqual(result, tt.out) {
				t.Errorf("got %v, want %v", result, tt.out)
			}
		})
	}
}
