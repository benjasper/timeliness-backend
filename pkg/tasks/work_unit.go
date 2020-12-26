package tasks

import "time"

// WorkUnit is an appointment where the user works on completing the tasks this model is embedded in
type WorkUnit struct {
	// TODO: Validation
	IsDone       bool      `json:"isDone"`
	MarkedDoneAt time.Time `json:"markedDoneAt"`

	ScheduledAt time.Time     `json:"scheduledAt"`
	Workload    time.Duration `json:"workload"`
}
