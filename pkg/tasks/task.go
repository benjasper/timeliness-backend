package tasks

import (
	"context"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type Task struct {
	ID             primitive.ObjectID `bson:"_id" json:"id"`
	UserID         primitive.ObjectID `json:"userId"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	IsDone         bool               `json:"isDone"`
	CreatedAt      time.Time          `json:"createdAt"`
	LastModifiedAt time.Time          `json:"lastModifiedAt"`

	WorkloadOverall int        `json:"workloadOverall"`
	DueAt           time.Time  `json:"dueAt"`
	WorkUnits       []WorkUnit `json:"workUnits"`

	Tags []string `json:"tags"`
}

type TaskServiceInterface interface {
	Add(ctx context.Context, task *Task) error
	FindAll(ctx context.Context, userId string) ([]Task, error)
	Update(ctx context.Context, task *Task) error
	Delete(ctx context.Context, id string) error
}
