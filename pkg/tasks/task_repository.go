package tasks

import (
	"context"
	"errors"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
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

func (s TaskService) Add(ctx context.Context, task *Task) error {
	task.CreatedAt = time.Now()
	task.LastModifiedAt = time.Now()
	task.ID = primitive.NewObjectID()
	_, err := s.DB.InsertOne(ctx, task)
	return err
}

func (s TaskService) Update(ctx context.Context, taskID string, userID string, task *TaskUpdate) error {
	task.LastModifiedAt = time.Now()

	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}
	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}

	result, err := s.DB.UpdateOne(ctx, bson.M{"userId": userObjectID, "_id": taskObjectID}, bson.M{"$set": task})
	if err != nil {
		return err
	}

	if result.MatchedCount != 1 {
		return errors.New("updated count != 1")
	}

	return nil
}

func (s TaskService) FindAll(ctx context.Context, userID string, page int, pageSize int) ([]Task, int, error) {
	t := []Task{}

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, 0, err
	}

	offset := page * pageSize

	findOptions := options.Find()
	findOptions.SetSort(bson.M{"dueAt": 1})
	findOptions.SetSkip(int64(offset))
	findOptions.SetLimit(int64(pageSize))

	filter := bson.M{"userId": userObjectID}

	cursor, err := s.DB.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.DB.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	err = cursor.All(ctx, &t)
	if err != nil {
		return nil, 0, err
	}

	return t, int(count), nil
}

func (s TaskService) FindByID(ctx context.Context, taskID string, userID string) (Task, error) {
	t := Task{}

	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return t, err
	}
	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return t, err
	}

	result := s.DB.FindOne(ctx, bson.M{"userId": userObjectID, "_id": taskObjectID})

	if result.Err() != nil {
		return t, result.Err()
	}

	err = result.Decode(&t)
	if err != nil {
		return t, err
	}

	return t, nil
}

func (s TaskService) FindUpdatableByID(ctx context.Context, taskID string, userID string) (TaskUpdate, error) {
	t := TaskUpdate{}

	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return t, err
	}
	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return t, err
	}

	result := s.DB.FindOne(ctx, bson.M{"userId": userObjectID, "_id": taskObjectID})

	if result.Err() != nil {
		return t, result.Err()
	}

	err = result.Decode(&t)
	if err != nil {
		return t, err
	}

	return t, nil
}

func (s TaskService) Delete(ctx context.Context, id string) error {
	panic("implement me")
}
