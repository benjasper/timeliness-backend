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

// WorkUnitService does everything related to storing and finding the work units
type WorkUnitService struct {
	DB     *mongo.Collection
	Logger logger.Interface
}

// Add adds a work unit
func (s WorkUnitService) Add(ctx context.Context, workUnit *WorkUnit, userID primitive.ObjectID, taskID primitive.ObjectID) error {

	workUnit.CreatedAt = time.Now()
	workUnit.LastModifiedAt = time.Now()
	workUnit.ID = primitive.NewObjectID()
	workUnit.UserID = userID
	workUnit.TaskID = taskID
	_, err := s.DB.InsertOne(ctx, workUnit)
	return err
}

// AddMultiple adds multiple work units
func (s WorkUnitService) AddMultiple(ctx context.Context, workUnits []WorkUnit, userID primitive.ObjectID, taskID primitive.ObjectID) error {
	var docs []interface{}

	for _, workUnit := range workUnits {
		workUnit.CreatedAt = time.Now()
		workUnit.LastModifiedAt = time.Now()
		workUnit.ID = primitive.NewObjectID()
		workUnit.UserID = userID
		workUnit.TaskID = taskID
		docs = append(docs, workUnit)
	}

	_, err := s.DB.InsertMany(ctx, docs)
	return err
}

// Update updates a work unit
func (s WorkUnitService) Update(ctx context.Context, workUnitID string, userID string, workUnit *WorkUnitUpdate) error {
	workUnit.LastModifiedAt = time.Now()

	workUnitObjectID, err := primitive.ObjectIDFromHex(workUnitID)
	if err != nil {
		return err
	}

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}

	result, err := s.DB.UpdateOne(ctx, bson.M{"_id": workUnitObjectID, "userId": userObjectID}, bson.M{"$set": workUnit})
	if err != nil {
		return err
	}

	if result.MatchedCount != 1 {
		return errors.New("updated count != 1")
	}

	return nil
}

// FindAll finds all work unit paginated
func (s WorkUnitService) FindAll(ctx context.Context, userID string, page int, pageSize int) ([]WorkUnit, int, error) {
	var t []WorkUnit

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, 0, err
	}

	offset := page * pageSize

	findOptions := options.Find()
	findOptions.SetSort(bson.M{"scheduledAt": 1})
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

// FindByID finds a specific work unit by ID
func (s WorkUnitService) FindByID(ctx context.Context, workUnitID string, userID string) (WorkUnit, error) {
	t := WorkUnit{}

	workUnitObjectID, err := primitive.ObjectIDFromHex(workUnitID)
	if err != nil {
		return t, err
	}

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return t, err
	}

	result := s.DB.FindOne(ctx, bson.M{"_id": workUnitObjectID, "userId": userObjectID})

	if result.Err() != nil {
		return t, result.Err()
	}

	err = result.Decode(&t)
	if err != nil {
		return t, err
	}

	return t, nil
}

// FindByTask finds multiple work units for a specific task
func (s WorkUnitService) FindByTask(ctx context.Context, taskID string, userID string) ([]WorkUnit, error) {
	var t []WorkUnit

	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return t, err
	}

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return t, err
	}

	findOptions := options.Find()
	findOptions.SetSort(bson.M{"scheduledAt": 1})

	cursor, err := s.DB.Find(ctx, bson.M{"taskId": taskObjectID, "userId": userObjectID}, findOptions)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &t)
	if err != nil {
		return nil, err
	}

	return t, nil
}

// FindUpdatableByID Finds a work unit and returns the WorkUnitUpdate view of the model
func (s WorkUnitService) FindUpdatableByID(ctx context.Context, workUnitID string, userID string) (WorkUnitUpdate, error) {
	t := WorkUnitUpdate{}

	workUnitObjectID, err := primitive.ObjectIDFromHex(workUnitID)
	if err != nil {
		return t, err
	}

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return t, err
	}

	result := s.DB.FindOne(ctx, bson.M{"_id": workUnitObjectID, "userId": userObjectID})

	if result.Err() != nil {
		return t, result.Err()
	}

	err = result.Decode(&t)
	if err != nil {
		return t, err
	}

	return t, nil
}

// Delete deletes a work unit
func (s WorkUnitService) Delete(ctx context.Context, workUnitID string, userID string) error {
	workUnitObjectID, err := primitive.ObjectIDFromHex(workUnitID)
	if err != nil {
		return err
	}

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}

	_, err = s.DB.DeleteOne(ctx, bson.M{"_id": workUnitObjectID, "userId": userObjectID})
	if err != nil {
		return err
	}

	return nil
}

// DeleteMultiple deletes multiple work units
func (s WorkUnitService) DeleteMultiple(ctx context.Context, taskID string, userID string) error {
	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}

	_, err = s.DB.DeleteMany(ctx, bson.M{"taskId": taskObjectID, "userId": userObjectID})
	if err != nil {
		return err
	}

	return nil
}
