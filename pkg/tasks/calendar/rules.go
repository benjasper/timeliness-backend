package calendar

import "time"

// RuleInterface is the interface all rules have to implement
type RuleInterface interface {
	Test(timespan Timespan) []Timespan
}

// ConstraintInterface is for constraints that a single timespan has to comply with
type ConstraintInterface interface {
	Test(timespan Timespan) bool
}

// ConstraintDistanceToBusy is the minimum distance a time slot has to have to a busy time slot
type ConstraintDistanceToBusy struct {
	Distance int
}

// Test against RuleDistanceToBusytimes
func (r *ConstraintDistanceToBusy) Test(timespan *Timespan) bool {
	return true
}

// RuleDuration sets minimum and maximum times
type RuleDuration struct {
	Minimum time.Duration
	Maximum time.Duration
}

// Test against RuleDuration
func (r *RuleDuration) Test(timespan *Timespan) []Timespan {
	var timeslots []Timespan
	diff := timespan.Duration()
	if r.Minimum != 0 && diff < r.Minimum {
		return timeslots
	}

	if r.Maximum != 0 && diff > r.Maximum {
		return timeslots
	}
	return timeslots
}

// TODO Rule Min Max day/nighttime
// TODO Rule Weekdays/Weekends
