package tasks

import "time"

type WorkUnit struct {
	// TODO: Validation
	IsDone       bool      `json:"isDone"`
	MarkedDoneAt time.Time `json:"markedDoneAt"`

	ScheduledAt time.Time     `json:"scheduledAt"`
	Workload    time.Duration `json:"workload"`
}
