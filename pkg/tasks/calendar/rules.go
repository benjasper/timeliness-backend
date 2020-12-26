package calendar

import "time"

// RuleInterface is the interface all rules have to implement
type RuleInterface interface {
	Test(timespan Timespan) bool
}

// RuleDistanceToBusytimes is the minimum distance a time slot has to have to a busy time slot
type RuleDistanceToBusytimes struct {
	Distance int
}

// RuleDuration sets minimum and maximum times
type RuleDuration struct {
	Minimum time.Duration
	Maximum time.Duration
}

// Test against RuleDuration
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

// Test against RuleDistanceToBusytimes
func (r *RuleDistanceToBusytimes) Test(timespan *Timespan) bool {
	return true
}
