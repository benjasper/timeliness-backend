package date

import (
	"fmt"
	"sort"
	"sync"
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
	End   time.Time `json:"end" bson:"end"`
}

// Duration simply get the duration of a Timespan
func (t *Timespan) Duration() time.Duration {
	return t.End.Sub(t.Start)
}

// IsStartBeforeEnd checks if start is earlier than end
func (t *Timespan) IsStartBeforeEnd() bool {
	return t.Start.Before(t.End)
}

// String prints a timespan string
func (t *Timespan) String() string {
	return fmt.Sprintf("%s - %s", t.Start, t.End)
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

// ContainsClock checks if a time is contained in a Timespan
func (t *Timespan) ContainsClock(clock time.Time) bool {
	cStart := calcSecondsFromClock(t.Start.Clock())
	cEnd := calcSecondsFromClock(t.End.Clock())

	clockSeconds := calcSecondsFromClock(clock.Clock())

	if cStart <= clockSeconds && clockSeconds <= cEnd {
		return true
	}
	return false
}

// In changes the location on a Timespan
func (t *Timespan) In(location *time.Location) Timespan {
	t.Start = t.Start.In(location)
	t.End = t.End.In(location)

	return *t
}

// IntersectsWithClock checks if a timespan even intersects
func (t *Timespan) IntersectsWithClock(timespan Timespan) bool {
	cStart := calcSecondsFromClock(t.Start.Clock())
	cEnd := calcSecondsFromClock(t.End.Clock())

	tStart := calcSecondsFromClock(timespan.Start.Clock())
	tEnd := calcSecondsFromClock(timespan.End.Clock())

	if tStart > cEnd || tEnd < cStart {
		return false
	}
	return true
}

// IntersectsWith checks if one timespan intersects with another
func (t *Timespan) IntersectsWith(timespan Timespan) bool {
	if t.Start.Before(timespan.End) && t.End.After(timespan.Start) {
		return true
	}

	return false
}

// Contains checks if one timespan t contains another Timespan timespan
func (t *Timespan) Contains(timespan Timespan) bool {
	if TimeAfterOrEquals(timespan.Start, t.Start) &&
		TimeBeforeOrEquals(timespan.End, t.End) {
		return true
	}

	return false
}

// OverflowsStartClock checks if the given timespan overflows at end
func (t *Timespan) OverflowsStartClock(timespan Timespan) bool {
	cStart := calcSecondsFromClock(t.Start.Clock())
	cEnd := calcSecondsFromClock(t.End.Clock())

	tStart := calcSecondsFromClock(timespan.Start.Clock())
	tEnd := calcSecondsFromClock(timespan.End.Clock())

	if cStart >= tStart && cEnd >= tEnd {
		return true
	}
	return false
}

// OverflowsEndClock checks if the given timespan overflows at start
func (t *Timespan) OverflowsEndClock(timespan Timespan) bool {
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
	BusyPadding  time.Duration
	busyMutex    sync.Mutex
	freeMutex    sync.Mutex
}

// Duration simply get the duration of a Timespan
func (w *TimeWindow) Duration() time.Duration {
	w.busyMutex.Lock()
	defer w.busyMutex.Unlock()

	return w.End.Sub(w.Start)
}

// AddToBusy adds a single Timespan to the sorted busy timespan array in a TimeWindow
func (w *TimeWindow) AddToBusy(timespan Timespan) {
	timespan.Start = timespan.Start.Add(w.BusyPadding * -1)
	timespan.End = timespan.End.Add(w.BusyPadding)

	w.busyMutex.Lock()
	defer w.busyMutex.Unlock()

	w.Busy = append(w.Busy, timespan)

	if len(w.Busy) == 0 {
		return
	}

	w.Busy = MergeTimespans(w.Busy)
}

func min(a, b time.Time) time.Time {
	if a.Unix() < b.Unix() {
		return a
	}
	return b
}

func max(a, b time.Time) time.Time {
	if a.Unix() > b.Unix() {
		return a
	}
	return b
}

// MergeTimespans merges Timespan structs together in case they overlap, they don't have to be presorted
func MergeTimespans(timespans []Timespan) []Timespan {
	if len(timespans) == 0 {
		return nil
	}

	sort.Slice(timespans, func(i, j int) bool {
		return timespans[i].Start.Before(timespans[j].Start)
	})

	index := 0

	for i := 1; i < len(timespans); i++ {
		if timespans[index].End.Unix() >= timespans[i].Start.Unix() {
			timespans[index].End = max(timespans[index].End, timespans[i].End)
			timespans[index].Start = min(timespans[index].Start, timespans[i].Start)
		} else {
			index++
			timespans[index] = timespans[i]
		}
	}

	var mergedTimespans []Timespan
	for i := 0; i <= index; i++ {
		mergedTimespans = append(mergedTimespans, timespans[i])
	}

	return mergedTimespans
}

// ComputeFree computes the free times, that are the inverse of busy times in the specified window
func (w *TimeWindow) ComputeFree(constraint *FreeConstraint, target time.Time, timeInterval Timespan) []Timespan {
	w.freeMutex.Lock()
	defer w.freeMutex.Unlock()

	w.busyMutex.Lock()
	defer w.busyMutex.Unlock()

	var relevantBusyEntries []Timespan

	for _, busy := range w.Busy {
		// Check if the busy timespan is in the time interval we are viewing
		if !timeInterval.Contains(busy) {
			// If it isn't contained but does intersect we can inspect it further
			if busy.IntersectsWith(timeInterval) {
				busy := busy
				if busy.Start.Before(timeInterval.Start) {
					busy.Start = timeInterval.Start
				} else {
					busy.End = timeInterval.End
				}
			} else {
				continue
			}
		}

		relevantBusyEntries = append(relevantBusyEntries, busy)
	}

	if len(relevantBusyEntries) == 0 {
		w.Free = append(w.Free, constraint.Test(Timespan{Start: timeInterval.Start, End: timeInterval.End})...)
		for _, timespan := range w.Free {
			w.FreeDuration += timespan.Duration()
		}
	}

	for index, busy := range relevantBusyEntries {
		if index == 0 {
			if timeInterval.Start.Before(busy.Start) {
				constrained := constraint.Test(Timespan{Start: timeInterval.Start, End: busy.Start})
				for _, timespan := range constrained {
					w.FreeDuration += timespan.Duration()
				}
				w.Free = append(w.Free, constrained...)
			}
		}

		if index == len(relevantBusyEntries)-1 {
			constrained := constraint.Test(Timespan{Start: busy.End, End: timeInterval.End})
			for _, timespan := range constrained {
				w.FreeDuration += timespan.Duration()
			}
			w.Free = append(w.Free, constrained...)
			continue
		}

		constrained := constraint.Test(Timespan{Start: busy.End, End: relevantBusyEntries[index+1].Start})
		for _, timespan := range constrained {
			w.FreeDuration += timespan.Duration()
		}
		w.Free = append(w.Free, constrained...)
	}

	w.Free = MergeTimespans(w.Free)

	sort.Slice(w.Free, func(i, j int) bool {
		return absoluteOfDuration(w.Free[i].Start.Sub(target)) < absoluteOfDuration(w.Free[j].Start.Sub(target))
	})

	return w.Free
}

func absoluteOfDuration(duration time.Duration) time.Duration {
	if duration < 0 {
		return duration * -1
	}
	return duration
}

// FindTimeSlot finds one or multiple time slots that comply with the specified rules
func (w *TimeWindow) FindTimeSlot(rules *[]RuleInterface) *Timespan {
	w.freeMutex.Lock()
	defer w.freeMutex.Unlock()

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
	w.freeMutex.Lock()
	defer w.freeMutex.Unlock()

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
