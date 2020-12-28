package calendar

import (
	"time"
)

// Timespan is a simple timespan between to times/dates
type Timespan struct {
	Start time.Time
	End   time.Time
}

// Duration simply get the duration of a Timespan
func (t *Timespan) Duration() time.Duration {
	return t.End.Sub(t.Start)
}

// TimeWindow is a window equally to timespan, with additional data about busy and free timeslots
type TimeWindow struct {
	Start time.Time
	End   time.Time
	Busy  []Timespan
	Free  []Timespan
}

// AddToBusy adds a single Timespan to the sorted busy timespan array in a TimeWindow
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

// ComputeFree computes the free times, that are the inverse of busy times in the specified window
func (w *TimeWindow) ComputeFree() []Timespan {
	var free = make([]Timespan, 0)

	for index, busy := range w.Busy {
		if index == 0 {
			if w.Start.Before(busy.Start) {
				free = append(free, Timespan{Start: w.Start, End: busy.Start})
			}
		}

		if index == len(w.Busy)-1 {
			free = append(free, Timespan{Start: busy.End, End: w.End})
			continue
		}

		free = append(free, Timespan{Start: busy.End, End: w.Busy[index+1].Start})
	}

	w.Free = free

	return free
}

// FindTimeSlot finds one or multiple time slots that comply with the specified rules
func (w *TimeWindow) FindTimeSlot() []Timespan {
	// TODO: Write actual logic
	var free []Timespan

	return free
}
