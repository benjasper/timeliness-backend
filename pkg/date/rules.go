package date

import (
	"time"
)

// RuleInterface is the interface all rules have to implement
type RuleInterface interface {
	Test(timespan Timespan) *Timespan
}

// FreeConstraint is for constraints that a single timespan has to comply with
// AllowedTimeSpans can only contain []Timespan with dates 0 and times that don't cross the dateline (0:00)
type FreeConstraint struct {
	DistanceToBusy   time.Duration
	AllowedTimeSpans []Timespan
	Location         *time.Location
}

// Test tests multiple constrains and cuts free timeslots to these constraints
func (r *FreeConstraint) Test(timespan Timespan) []Timespan {
	var result []Timespan

	if len(r.AllowedTimeSpans) == 0 {
		return append(result, timespan)
	}

	timespan = timespan.In(r.Location)
	p := timespan.Start

	for p.Before(timespan.End) {

		for _, span := range r.AllowedTimeSpans {
			localAllowedTimespan := span.In(r.Location)
			allowedDuration := span.Duration()

			if span.ContainsClock(p) {
				end := time.Date(p.Year(), p.Month(), p.Day(), localAllowedTimespan.End.Hour(), localAllowedTimespan.End.Minute(), 0, 0, r.Location)
				if timespan.End.Before(end) {
					end = timespan.End
				}

				if p.Equal(end) {
					continue
				}

				result = append(result, Timespan{p, end})
			} else {
				start := time.Date(p.Year(), p.Month(), p.Day(), localAllowedTimespan.Start.Hour(), localAllowedTimespan.Start.Minute(), 0, 0, r.Location)

				newEnd := start.Add(allowedDuration)
				if timespan.End.Before(newEnd) {
					newEnd = timespan.End
				}

				newTimespan := Timespan{start, newEnd}

				if start.After(newEnd) || start.Before(timespan.Start) || start.Equal(newEnd) || !timespan.Contains(newTimespan) {
					continue
				}

				result = append(result, newTimespan)
			}
		}

		nextDay := p.Add(24 * time.Hour)
		p = time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 0, 0, 0, 0, r.Location)
	}

	return result
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
func (r *RuleDuration) Test(timespan Timespan) *Timespan {
	diff := timespan.Duration()
	if r.Minimum != 0 && diff < r.Minimum {
		return nil
	}

	if r.Maximum != 0 && diff > r.Maximum {
		timespan.End = timespan.End.Add((diff - r.Maximum) * -1)
	}

	return &timespan
}

// TODO Rule Weekdays/Weekends
