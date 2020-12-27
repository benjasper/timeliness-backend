package tasks

import (
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"time"
)

// WorkUnit is an appointment where the user works on completing the tasks this model is embedded in
type WorkUnit struct {
	// TODO: Validation
	IsDone       bool      `json:"isDone" bson:"isDone"`
	MarkedDoneAt time.Time `json:"markedDoneAt" bson:"markedDoneAt"`

	ScheduledAt calendar.Event `json:"scheduledAt" bson:"scheduledAt"`
	Workload    time.Duration  `json:"workload" bson:"workload"`
}
