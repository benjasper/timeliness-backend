package calendar

import (
	"reflect"
	"testing"
	"time"
)

func timeDate(year int, month time.Month, day int, hour int, min int) time.Time {
	loc, _ := time.LoadLocation("Local")
	return time.Date(year, month, day, hour, min, 0, 0, loc)
}

var freetimetests = []struct {
	in  TimeWindow
	out []Timespan
}{
	{
		// Case single busy time
		TimeWindow{Start: timeDate(2020, 6, 10, 12, 30), End: timeDate(2020, 6, 18, 12, 30),
			Busy: []Timespan{{Start: timeDate(2020, 6, 10, 13, 0), End: timeDate(2020, 6, 10, 14, 0)}}},
		[]Timespan{
			{Start: timeDate(2020, 6, 10, 12, 30), End: timeDate(2020, 6, 10, 13, 0)},
			{Start: timeDate(2020, 6, 10, 14, 0), End: timeDate(2020, 6, 18, 12, 30)}},
	},
	{
		// Case 2 busy time
		TimeWindow{Start: timeDate(2020, 6, 10, 12, 30), End: timeDate(2020, 6, 18, 12, 30),
			Busy: []Timespan{
				{Start: timeDate(2020, 6, 10, 13, 0), End: timeDate(2020, 6, 10, 14, 0)},
				{Start: timeDate(2020, 6, 10, 14, 30), End: timeDate(2020, 6, 10, 15, 0)}}},
		[]Timespan{
			{Start: timeDate(2020, 6, 10, 12, 30), End: timeDate(2020, 6, 10, 13, 0)},
			{Start: timeDate(2020, 6, 10, 14, 0), End: timeDate(2020, 6, 10, 14, 30)},
			{Start: timeDate(2020, 6, 10, 15, 0), End: timeDate(2020, 6, 18, 12, 30)}},
	},
	{
		// Case 3 busy time
		TimeWindow{Start: timeDate(2020, 6, 10, 12, 30), End: timeDate(2020, 6, 18, 12, 30),
			Busy: []Timespan{
				{Start: timeDate(2020, 6, 10, 13, 0), End: timeDate(2020, 6, 10, 14, 0)},
				{Start: timeDate(2020, 6, 10, 14, 30), End: timeDate(2020, 6, 10, 15, 0)},
				{Start: timeDate(2020, 6, 12, 14, 30), End: timeDate(2020, 6, 13, 15, 0)},
			},
		},
		[]Timespan{
			{Start: timeDate(2020, 6, 10, 12, 30), End: timeDate(2020, 6, 10, 13, 0)},
			{Start: timeDate(2020, 6, 10, 14, 0), End: timeDate(2020, 6, 10, 14, 30)},
			{Start: timeDate(2020, 6, 10, 15, 0), End: timeDate(2020, 6, 12, 14, 30)},
			{Start: timeDate(2020, 6, 13, 15, 0), End: timeDate(2020, 6, 18, 12, 30)}},
	},
	{
		// Case windows start is in busy time
		TimeWindow{Start: timeDate(2020, 6, 10, 12, 30), End: timeDate(2020, 6, 18, 12, 30),
			Busy: []Timespan{
				{Start: timeDate(2020, 6, 10, 12, 0), End: timeDate(2020, 6, 10, 14, 0)},
				{Start: timeDate(2020, 6, 10, 14, 30), End: timeDate(2020, 6, 10, 15, 0)},
				{Start: timeDate(2020, 6, 12, 14, 30), End: timeDate(2020, 6, 13, 15, 0)},
			},
		},
		[]Timespan{
			{Start: timeDate(2020, 6, 10, 14, 0), End: timeDate(2020, 6, 10, 14, 30)},
			{Start: timeDate(2020, 6, 10, 15, 0), End: timeDate(2020, 6, 12, 14, 30)},
			{Start: timeDate(2020, 6, 13, 15, 0), End: timeDate(2020, 6, 18, 12, 30)}},
	},
}

func TestTimeWindow_ComputeFree(t *testing.T) {
	for index, tt := range freetimetests {
		t.Run("Case "+string(rune(index)), func(t *testing.T) {
			free := tt.in.ComputeFree()
			if !reflect.DeepEqual(free, tt.out) {
				t.Errorf("got (%d)%q, want (%d)%q", len(free), free, len(tt.out), tt.out)
			}
		})
	}
}
