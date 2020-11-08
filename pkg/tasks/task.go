package tasks

import (
	"context"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type Task struct {
	// TODO: More validation
	ID             primitive.ObjectID `bson:"_id" json:"id"`
	UserID         primitive.ObjectID `json:"userId" validate:"required"`
	Name           string             `json:"name" validate:"required"`
	Description    string             `json:"description"`
	IsDone         bool               `json:"isDone"`
	CreatedAt      time.Time          `json:"createdAt"`
	LastModifiedAt time.Time          `json:"lastModifiedAt"`

	Priority        int        `json:"priority" validate:"required"`
	WorkloadOverall int        `json:"workloadOverall"`
	DueAt           time.Time  `json:"dueAt" validate:"required"`
	WorkUnits       []WorkUnit `json:"workUnits"`

	Tags []string `json:"tags"`
}

type TaskServiceInterface interface {
	Add(ctx context.Context, task *Task) error
	FindAll(ctx context.Context, userID string) ([]Task, error)
	FindByID(ctx context.Context, taskID string, userID string) (Task, error)
	Update(ctx context.Context, task *Task) error
	Delete(ctx context.Context, id string) error
}
