package calendar

import "time"

type RuleInterface interface {
	Test(timespan Timespan) bool
}

type RuleDistanceToBusytimes struct {
	Distance int
}

type RuleDuration struct {
	Minimum time.Duration
	Maximum time.Duration
}

func (r *RuleDuration) Test(timespan *Timespan) bool {
	diff := timespan.End.Sub(timespan.Start)
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

func (r *RuleDistanceToBusytimes) Test(timespan *Timespan) bool {
	return true
}
