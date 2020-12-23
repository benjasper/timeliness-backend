package calendar

import (
	"time"
)

type Timespan struct {
	Start time.Time
	End   time.Time
}

type TimeWindow struct {
	Start time.Time
	End   time.Time
	Busy  []Timespan
	Free  []Timespan
}

func (w *TimeWindow) AddToBusy(timespan Timespan) {
	for index, busy := range w.Busy {
		// Old timespan is completely enclosing new one
		if timespan.Start.After(busy.Start) && timespan.End.Before(busy.End) {
			return
		}

		// New timespan is completely enclosing one old timespan
		if timespan.Start.Before(busy.Start) && timespan.End.After(busy.End) {
			w.Busy[index] = timespan
			return
		}

		// Sort by starting
		if busy.Start.After(timespan.Start) {
			w.Busy = append(w.Busy, Timespan{})
			copy(w.Busy[index+1:], w.Busy[index:])
			w.Busy[index] = timespan
			return
		}
	}
	w.Busy = append(w.Busy, timespan)
}

func (w *TimeWindow) ComputeFree() []Timespan {
	var free = make([]Timespan, 0)

	for index, busy := range w.Busy {
		if index == 0 && !busy.End.After(w.Start) {
			free = append(free, Timespan{Start: w.Start, End: busy.Start})
			continue
		}

		if index == len(w.Busy)-1 && busy.Start.Before(w.End) {
			continue
		}

		if index == len(w.Busy)-1 {
			free = append(free, Timespan{Start: busy.End, End: w.End})
			continue
		}

		free = append(free, Timespan{Start: busy.End, End: w.Busy[index+1].Start})
	}

	return free
}

func (w *TimeWindow) FindTimeSlot(rules *[]RuleInterface) []Timespan {
	// TODO: Write actual logic
	var free []Timespan
	for _, timeslot := range w.Free {
		for _, rule := range *rules {
			if !rule.Test(timeslot) {
				break
			}
			free = append(free, timeslot)
			break
		}
	}
	return free
}
