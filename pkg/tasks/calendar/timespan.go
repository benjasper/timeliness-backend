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

// ContainsByClock checks if a timespan is contained by the source timespan only by time and not by date
func (t *Timespan) ContainsByClock(timespan Timespan) bool {
	cStart := calcSecondsFromClock(t.Start.Clock())
	cEnd := calcSecondsFromClock(t.End.Clock())

	tStart := calcSecondsFromClock(timespan.Start.Clock())
	tEnd := calcSecondsFromClock(timespan.End.Clock())

	if cStart <= tStart && cEnd >= tEnd {
		return true
	}
	return false
}

// IntersectsWith checks if a timespan even intersects
func (t *Timespan) IntersectsWith(timespan Timespan) bool {
	cStart := calcSecondsFromClock(t.Start.Clock())
	cEnd := calcSecondsFromClock(t.End.Clock())

	tStart := calcSecondsFromClock(timespan.Start.Clock())
	tEnd := calcSecondsFromClock(timespan.End.Clock())

	if tStart > cEnd || tEnd < cStart {
		return false
	}
	return true
}

// OverflowsStart checks if the given timespan overflows at end
func (t *Timespan) OverflowsStart(timespan Timespan) bool {
	cStart := calcSecondsFromClock(t.Start.Clock())
	cEnd := calcSecondsFromClock(t.End.Clock())

	tStart := calcSecondsFromClock(timespan.Start.Clock())
	tEnd := calcSecondsFromClock(timespan.End.Clock())

	if cStart >= tStart && cEnd >= tEnd {
		return true
	}
	return false
}

// OverflowsEnd checks if the given timespan overflows at start
func (t *Timespan) OverflowsEnd(timespan Timespan) bool {
	cStart := calcSecondsFromClock(t.Start.Clock())
	cEnd := calcSecondsFromClock(t.End.Clock())

	tStart := calcSecondsFromClock(timespan.Start.Clock())
	tEnd := calcSecondsFromClock(timespan.End.Clock())

	if cEnd <= tEnd && cStart <= tStart {
		return true
	}
	return false
}

func calcSecondsFromClock(hours int, minutes int, seconds int) int {
	secondsTotal := 0
	secondsTotal += hours * 3600
	secondsTotal += minutes * 60
	secondsTotal += seconds
	return secondsTotal
}

// SplitByDays splits a timespan that's longer than one day into multiple timespan with the size of one day
func (t *Timespan) SplitByDays() []Timespan {
	var splitted []Timespan
	if t.Duration() > 24*time.Hour {
		timespan1 := Timespan{Start: t.Start, End: time.Date(
			t.Start.Year(), t.Start.Month(), t.Start.Day(), 23, 59, 59, 0, t.Start.Location())}
		splitted = append(splitted, timespan1)

		timespan2 := Timespan{
			Start: newTimeFromDateAndTime(t.Start.AddDate(0, 0, 1),
				time.Date(0, 0, 0, 0, 0, 0, 0, t.Start.Location())),
			End: t.End,
		}
		splitted = append(splitted, timespan2.SplitByDays()...)
	} else {
		splitted = append(splitted, *t)
	}
	return splitted
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
func (w *TimeWindow) ComputeFree(constraint *FreeConstraint) []Timespan {
	w.Free = nil

	for index, busy := range w.Busy {
		if index == 0 {
			if w.Start.Before(busy.Start) {
				w.Free = append(w.Free, constraint.Test(Timespan{Start: w.Start, End: busy.Start})...)
			}
		}

		if index == len(w.Busy)-1 {
			w.Free = append(w.Free, constraint.Test(Timespan{Start: busy.End, End: w.End})...)
			continue
		}

		w.Free = append(w.Free, constraint.Test(Timespan{Start: busy.End, End: w.Busy[index+1].Start})...)
	}

	return w.Free
}

// FindTimeSlot finds one or multiple time slots that comply with the specified rules
func (w *TimeWindow) FindTimeSlot(rules *[]RuleInterface) *Timespan {
	for _, timespan := range w.Free {
		for _, rule := range *rules {
			result := rule.Test(timespan)
			if result == nil {
				break
			}
			timespan = *result
		}

		return &timespan
	}

	return nil
}
