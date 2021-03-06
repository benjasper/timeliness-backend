package tasks

import (
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

// WorkUnit is an appointment where the user works on completing the tasks this model is embedded in
type WorkUnit struct {
	// TODO: Validation
	ID             primitive.ObjectID `json:"id" bson:"_id"`
	TaskID         primitive.ObjectID `json:"taskId" bson:"taskId"`
	UserID         primitive.ObjectID `json:"userId" bson:"userId"`
	CreatedAt      time.Time          `json:"createdAt" bson:"createdAt"`
	LastModifiedAt time.Time          `json:"lastModifiedAt" bson:"lastModifiedAt"`
	IsDone         bool               `json:"isDone" bson:"isDone"`
	MarkedDoneAt   time.Time          `json:"markedDoneAt" bson:"markedDoneAt"`

	ScheduledAt calendar.Event `json:"scheduledAt" bson:"scheduledAt"`
	Workload    time.Duration  `json:"workload" bson:"workload"`
}

// WorkUnitUpdate is a view on the WorkUnit that can be modified by the user
type WorkUnitUpdate struct {
	// TODO: Validation
	ID             primitive.ObjectID `json:"_" bson:"_id"`
	TaskID         primitive.ObjectID `json:"-" bson:"taskId"`
	UserID         primitive.ObjectID `json:"-" bson:"userId"`
	CreatedAt      time.Time          `json:"-" bson:"createdAt"`
	LastModifiedAt time.Time          `json:"-" bson:"lastModifiedAt"`
	IsDone         bool               `json:"isDone" bson:"isDone"`
	MarkedDoneAt   time.Time          `json:"markedDoneAt" bson:"markedDoneAt"`

	ScheduledAt calendar.Event `json:"scheduledAt" bson:"scheduledAt"`
	Workload    time.Duration  `json:"workload" bson:"workload"`
}
