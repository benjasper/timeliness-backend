package calendar

import "time"

// RuleInterface is the interface all rules have to implement
type RuleInterface interface {
	Test(timespan Timespan) bool
}

// FreeConstraint is for constraints that a single timespan has to comply with
// AllowedTimeSpans can only contain []Timespan with dates 0 and times that don't cross the dateline (0:00)
type FreeConstraint struct {
	DistanceToBusy   time.Duration
	AllowedTimeSpans []Timespan
}

// Test tests multiple constrains and cuts free timeslots to these constraints
func (r *FreeConstraint) Test(timespan Timespan) []Timespan {
	var free []Timespan
	splitTimespans := timespan.SplitByDays()

	if len(r.AllowedTimeSpans) == 0 {
		free = append(free, timespan)
	}

	for _, t := range splitTimespans {
		for _, allowed := range r.AllowedTimeSpans {
			if allowed.ContainsByClock(t) {
				free = append(free, t)
				continue
			} else {
				if !allowed.IntersectsWith(t) {
					continue
				}
				switch {
				case allowed.OverflowsStart(t):
					start := newTimeFromDateAndTime(t.End, allowed.Start)
					free = append(free, Timespan{Start: start, End: t.End})
					continue
				case allowed.OverflowsEnd(t):
					end := newTimeFromDateAndTime(t.Start, allowed.End)
					free = append(free, Timespan{Start: t.Start, End: end})
					continue
				default:
					start := newTimeFromDateAndTime(t.Start, allowed.Start)
					end := newTimeFromDateAndTime(t.Start, allowed.End)
					free = append(free, Timespan{Start: start, End: end})
					continue
				}
			}
		}
	}

	return free
}

func newTimeFromDateAndTime(date time.Time, clock time.Time) time.Time {
	year, month, day := date.Date()
	hour, minute, second := clock.Clock()
	return time.Date(year, month, day, hour, minute, second, 0, date.Location())
}

// RuleDuration sets minimum and maximum times
type RuleDuration struct {
	Minimum time.Duration
	Maximum time.Duration
}

// Test against RuleDuration
func (r *RuleDuration) Test(timespan *Timespan) bool {
	diff := timespan.Duration()
	if r.Minimum != 0 && diff < r.Minimum {
		return false
	}

	if r.Maximum != 0 && diff > r.Maximum {
		return false
	}
	return true
}

// TODO Rule Min Max day/nighttime
// TODO Rule Weekdays/Weekends
