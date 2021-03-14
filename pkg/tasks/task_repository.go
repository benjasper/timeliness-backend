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

// TaskService does everything related to storing and finding tasks
type TaskService struct {
	DB     *mongo.Collection
	Logger logger.Interface
}

// Add adds a task
func (s TaskService) Add(ctx context.Context, task *Task) error {
	task.CreatedAt = time.Now()
	task.LastModifiedAt = time.Now()
	task.ID = primitive.NewObjectID()

	for index, unit := range task.WorkUnits {
		if unit.ID.IsZero() {
			task.WorkUnits[index].ID = primitive.NewObjectID()
		}
	}

	_, err := s.DB.InsertOne(ctx, task)
	return err
}

// Update updates a task
func (s TaskService) Update(ctx context.Context, taskID string, userID string, task *TaskUpdate) error {
	task.LastModifiedAt = time.Now()

	for index, unit := range task.WorkUnits {
		if unit.ID.IsZero() {
			task.WorkUnits[index].ID = primitive.NewObjectID()
		}
	}

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

// FindAll finds all task paginated
func (s TaskService) FindAll(ctx context.Context, userID string, page int, pageSize int) ([]Task, int, error) {
	t := []Task{}

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, 0, err
	}

	offset := page * pageSize

	findOptions := options.Find()
	findOptions.SetSort(bson.M{"dueAt.date.start": 1})
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

// FindAllByWorkUnits finds all task paginated, but unwound by their work units
func (s TaskService) FindAllByWorkUnits(ctx context.Context, userID string, page int, pageSize int) ([]TaskUnwound, int, error) {
	var t []TaskUnwound

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

	matchStage := bson.D{{Key: "$match", Value: bson.M{"userId": userObjectID}}}
	addFieldsStage := bson.D{{Key: "$addFields", Value: bson.M{"workUnitsCount": bson.M{"$size": "$workUnits"}}}}
	addFieldStage2 := bson.D{{Key: "$addFields", Value: bson.M{"workUnit": "$workUnits"}}}
	unwindStage := bson.D{{Key: "$unwind", Value: bson.M{"path": "$workUnit", "includeArrayIndex": "workUnitsIndex"}}}
	sortStage := bson.D{{Key: "$sort", Value: bson.M{"workUnits.scheduledAt.date": 1}}}

	cursor, err := s.DB.Aggregate(ctx, mongo.Pipeline{matchStage, addFieldsStage, addFieldStage2, unwindStage, sortStage})
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

// FindByID finds a specific task by ID
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

// FindUpdatableByID Finds a task and returns the TaskUpdate view of the model
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

// Delete deletes a task
func (s TaskService) Delete(ctx context.Context, taskID string, userID string) error {
	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}
	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}

	_, err = s.DB.DeleteOne(ctx, bson.M{"userId": userObjectID, "_id": taskObjectID})
	if err != nil {
		return err
	}

	return nil
}
