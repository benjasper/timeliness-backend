package tasks

import "time"

type WorkUnit struct {
	IsDone       bool      `json:"isDone"`
	MarkedDoneAt time.Time `json:"markedDoneAt"`

	ScheduledAt time.Time `json:"scheduledAt"`
	Workload    int       `json:"workload"`
}
