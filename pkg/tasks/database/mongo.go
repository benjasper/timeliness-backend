package database

import (
	"context"
	"errors"
	"github.com/benjasper/project-tasks/pkg/logger"
	"github.com/benjasper/project-tasks/pkg/tasks"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type TaskService struct {
	DB     *mongo.Collection
	Logger logger.Interface
}

func (s TaskService) Add(ctx context.Context, task *tasks.Task) error {
	task.CreatedAt = time.Now()
	task.ID = primitive.NewObjectID()
	_, err := s.DB.InsertOne(ctx, task)
	return err
}

func (s TaskService) Update(ctx context.Context, user *tasks.Task) error {
	user.LastModifiedAt = time.Now()

	result, err := s.DB.UpdateOne(ctx, bson.M{"_id": user.ID}, user)
	if err != nil {
		return err
	}

	if result.MatchedCount != 1 {
		return errors.New("updated count != 1")
	}

	return nil
}

func (s TaskService) FindAll(ctx context.Context, userId string) ([]tasks.Task, error) {
	var t []tasks.Task

	findOptions := options.Find()
	findOptions.SetSort(bson.M{"dueAt": 1})

	cursor, err := s.DB.Find(ctx, bson.M{"userId": userId}, findOptions)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &t)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (s TaskService) Delete(ctx context.Context, id string) error {
	panic("implement me")
}
