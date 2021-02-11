package calendar

import (
	"time"
)

// TimeBeforeOrEquals returns whether t1 is before or equal t2
func TimeBeforeOrEquals(t1 time.Time, t2 time.Time) bool {
	ts := t1.UnixNano()
	us := t2.UnixNano()
	return ts <= us
}

// TimeAfterOrEquals returns whether t1 is after or equal t2
func TimeAfterOrEquals(t1 time.Time, t2 time.Time) bool {
	ts := t1.UnixNano()
	us := t2.UnixNano()
	return ts >= us
}

// Timespan is a simple timespan between to times/dates
type Timespan struct {
	Start time.Time `json:"start" bson:"start" validate:"required"`
	End   time.Time `json:"end" bson:"end" validate:"required"`
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
	if t.Start.Day() != t.End.Day() {
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

// RemoveFromTimespanSlice removes a Timespan from a Timespan slice
func RemoveFromTimespanSlice(slice []Timespan, s int) []Timespan {
	return append(slice[:s], slice[s+1:]...)
}

// TimeWindow is a window equally to timespan, with additional data about busy and free timeslots
type TimeWindow struct {
	Start        time.Time
	End          time.Time
	FreeDuration time.Duration
	Busy         []Timespan
	Free         []Timespan
}

// Duration simply get the duration of a Timespan
func (w *TimeWindow) Duration() time.Duration {
	return w.End.Sub(w.Start)
}

/*
// AddToBusy adds a single Timespan to the sorted busy timespan array in a TimeWindow
func (w *TimeWindow) AddToBusy(timespan Timespan) {
	// TODO: Improve and test this function
	for index, busy := range w.Busy {
		// Old timespan is completely enclosing new one
		if timespan.Start.After(busy.Start) && timespan.End.Before(busy.End) {
			return
		}

		// New timespan is completely enclosing one old timespan
		if timespan.Start.Before(busy.Start) && timespan.End.After(busy.End) {
			if index-1 >= 0 && w.Busy[index-1] == timespan {
				copy(w.Busy[index:], w.Busy[index+1:])
				w.Busy[len(w.Busy)-1] = Timespan{}
				w.Busy = w.Busy[:len(w.Busy)-1]
				continue
			}
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
*/

// AddToBusy adds a single Timespan to the sorted busy timespan array in a TimeWindow
func (w *TimeWindow) AddToBusy(timespan Timespan) {
	if len(w.Busy) == 0 {
		w.Busy = append(w.Busy, timespan)
		return
	}

	isOverlapping := false
	overlappingIndex := 0
	for index, busy := range w.Busy {
		if isOverlapping {
			if w.Busy[overlappingIndex].End.Before(timespan.End) && overlappingIndex == len(w.Busy)-1 {
				w.Busy = append(w.Busy[:overlappingIndex], w.Busy[overlappingIndex+1:]...)
				overlappingIndex--
				w.Busy[overlappingIndex].End = timespan.End
				return
			}

			if TimeBeforeOrEquals(w.Busy[overlappingIndex].Start, timespan.End) && TimeAfterOrEquals(w.Busy[overlappingIndex].End, timespan.End) {
				if overlappingIndex == 0 {
					return
				}

				end := w.Busy[overlappingIndex].End
				if len(w.Busy)-1 == overlappingIndex {
					w.Busy = w.Busy[:len(w.Busy)-1]
				} else {
					w.Busy = append(w.Busy[:overlappingIndex], w.Busy[overlappingIndex+1:]...)
				}
				overlappingIndex--
				w.Busy[overlappingIndex].End = end
				return
			}

			if overlappingIndex < len(w.Busy)-1 && w.Busy[overlappingIndex].End.Before(timespan.End) && w.Busy[overlappingIndex+1].Start.After(timespan.End) {
				w.Busy[overlappingIndex].End = timespan.End
				return
			}

			if w.Busy[overlappingIndex].End.Before(timespan.End) {
				w.Busy = append(w.Busy[:overlappingIndex], w.Busy[overlappingIndex+1:]...)
				continue
			}
		}

		// Case: new timespan is contained by existing timespan
		if TimeBeforeOrEquals(busy.Start, timespan.Start) && TimeAfterOrEquals(busy.End, timespan.End) {
			return
		}

		// Case: timespan is before all others
		if index == 0 && timespan.End.Before(busy.Start) {
			w.Busy = append(w.Busy, Timespan{})
			copy(w.Busy[index+1:], w.Busy[index:])
			w.Busy[index] = timespan
			return
		}

		// Case: timespan is after all others
		if index == len(w.Busy)-1 && timespan.End.After(busy.End) {
			w.Busy = append(w.Busy, timespan)
			return
		}

		// Case: new timespan is in the middle of two existing timeslots
		if index < len(w.Busy)-1 && busy.End.Before(timespan.Start) && w.Busy[index+1].Start.After(timespan.End) {
			w.Busy = append(w.Busy, Timespan{})
			copy(w.Busy[index+2:], w.Busy[index+1:])
			w.Busy[index+1] = timespan
			return
		}

		// Case the start of the timeslot is inside an existing busy timeslot and overlaps it in the end
		if TimeBeforeOrEquals(busy.Start, timespan.Start) && TimeAfterOrEquals(busy.End, timespan.Start) {
			isOverlapping = true
			overlappingIndex = index + 1
			continue
		}

		// Case the start of the timeslot is before an existing busy timeslot and overlaps the next one
		if busy.Start.After(timespan.Start) {
			w.Busy[index].Start = timespan.Start
			isOverlapping = true
			if index < len(w.Busy)-1 && w.Busy[index+1].Start.After(timespan.End) {
				overlappingIndex = index
				continue
			}
			overlappingIndex = index + 1
			continue
		}
	}
}

// ComputeFree computes the free times, that are the inverse of busy times in the specified window
func (w *TimeWindow) ComputeFree(constraint *FreeConstraint) []Timespan {
	w.Free = nil
	w.FreeDuration = 0

	if len(w.Busy) == 0 {
		w.Free = append(w.Free, constraint.Test(Timespan{Start: w.Start, End: w.End})...)
	}

	for index, busy := range w.Busy {
		if index == 0 {
			if w.Start.Before(busy.Start) {
				constrained := constraint.Test(Timespan{Start: w.Start, End: busy.Start})
				for _, timespan := range constrained {
					w.FreeDuration += timespan.Duration()
				}
				w.Free = append(w.Free, constrained...)
			}
		}

		if index == len(w.Busy)-1 {
			constrained := constraint.Test(Timespan{Start: busy.End, End: w.End})
			for _, timespan := range constrained {
				w.FreeDuration += timespan.Duration()
			}
			w.Free = append(w.Free, constrained...)
			continue
		}

		constrained := constraint.Test(Timespan{Start: busy.End, End: w.Busy[index+1].Start})
		for _, timespan := range constrained {
			w.FreeDuration += timespan.Duration()
		}
		w.Free = append(w.Free, constrained...)
	}

	return w.Free
}

// FindTimeSlot finds one or multiple time slots that comply with the specified rules
func (w *TimeWindow) FindTimeSlot(rules *[]RuleInterface) *Timespan {
	for index, timespan := range w.Free {
		foundFlag := false

		if len(*rules) == 0 {
			tmp := timespan
			w.Free = RemoveFromTimespanSlice(w.Free, index)
			return &tmp
		}

		for _, rule := range *rules {
			foundFlag = false
			result := rule.Test(timespan)
			if result == nil {
				break
			}
			foundFlag = true
			timespan = *result
		}

		if foundFlag {
			tmp := timespan
			// TODO: This implementation only works if the cut timeslot is at the start of the whole timeslot
			if w.Free[index].Duration() != tmp.Duration() {
				w.Free[index].Start = tmp.End
				return &tmp
			}
			w.Free = RemoveFromTimespanSlice(w.Free, index)
			return &tmp
		}
	}

	return nil
}

// GetPreferredTimeWindow returns a TimeWindow that was cut to the specified times
func (w *TimeWindow) GetPreferredTimeWindow(from time.Time, to time.Time) *TimeWindow {
	preferred := TimeWindow{Start: from, End: to}
	for _, timespan := range w.Free {
		if timespan.Start.Before(from) {
			continue
		}

		if timespan.End.After(to) || timespan.Start.After(to) {
			break
		}

		preferred.Free = append(preferred.Free, timespan)
		preferred.FreeDuration += timespan.Duration()
	}

	return &preferred
}
