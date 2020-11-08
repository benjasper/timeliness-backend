package tasks

import (
	"context"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

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

type TaskServiceInterface interface {
	Add(ctx context.Context, task *Task) error
	FindAll(ctx context.Context, userID string) ([]Task, error)
	FindByID(ctx context.Context, taskID string, userID string) (Task, error)
	FindUpdatableByID(ctx context.Context, taskID string, userID string) (TaskUpdate, error)
	Update(ctx context.Context, taskID string, userID string, task *TaskUpdate) error
	Delete(ctx context.Context, id string) error
}
