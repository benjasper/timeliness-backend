package tasks

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

// Task is the model for a task
type Task struct {
	// TODO: More validation
	ID             primitive.ObjectID `bson:"_id" json:"id"`
	UserID         primitive.ObjectID `json:"userId" bson:"userId" validate:"required"`
	CreatedAt      time.Time          `json:"createdAt" bson:"createdAt"`
	LastModifiedAt time.Time          `json:"lastModifiedAt" bson:"lastModifiedAt"`
	Name           string             `json:"name" bson:"name" validate:"required"`
	Description    string             `json:"description" bson:"description"`
	IsDone         bool               `json:"isDone" bson:"isDone"`
	Tags           []string           `json:"tags" bson:"tags"`

	Priority        int        `json:"priority" bson:"priority" validate:"required"`
	WorkloadOverall int        `json:"workloadOverall" bson:"workloadOverall"`
	DueAt           time.Time  `json:"dueAt" bson:"dueAt" validate:"required"`
	WorkUnits       []WorkUnit `json:"workUnits" bson:"workUnits"`
}

// TaskUpdate is the view of a task for an update
type TaskUpdate struct {
	ID             primitive.ObjectID `bson:"_id" json:"-"`
	UserID         primitive.ObjectID `bson:"userId" json:"-"`
	CreatedAt      time.Time          `bson:"createdAt" json:"-"`
	LastModifiedAt time.Time          `bson:"lastModifiedAt" json:"-"`
	Name           string             `json:"name" bson:"name" validate:"required"`
	Description    string             `json:"description" bson:"description"`
	IsDone         bool               `json:"isDone" bson:"isDone"`
	Tags           []string           `json:"tags" bson:"tags"`

	Priority        int        `json:"priority" bson:"priority" validate:"required"`
	WorkloadOverall int        `json:"workloadOverall" bson:"workloadOverall"`
	DueAt           time.Time  `json:"dueAt" bson:"dueAt" validate:"required"`
	WorkUnits       []WorkUnit `json:"workUnits" bson:"workUnits"`
}
