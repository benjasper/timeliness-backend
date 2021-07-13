package tasks

import (
	"context"
	"errors"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

// TaskRepositoryInterface is an interface for a MongoDBTaskRepository
type TaskRepositoryInterface interface {
	Add(ctx context.Context, task *Task) error
	Update(ctx context.Context, taskID string, userID string, task *TaskUpdate) error
	FindAll(ctx context.Context, userID string, page int, pageSize int, filters []Filter) ([]Task, int, error)
	FindAllByWorkUnits(ctx context.Context, userID string, page int, pageSize int, filters []Filter) ([]TaskUnwound, int, error)
	FindByID(ctx context.Context, taskID string, userID string) (Task, error)
	FindByCalendarEventID(ctx context.Context, calendarEventID string, userID string) (*TaskUpdate, error)
	FindUpdatableByID(ctx context.Context, taskID string, userID string) (*TaskUpdate, error)
	FindIntersectingWithEvent(ctx context.Context, userID string, event *calendar.Event) ([]Task, error)
	Delete(ctx context.Context, taskID string, userID string) error
}

// MongoDBTaskRepository does everything related to storing and finding tasks
type MongoDBTaskRepository struct {
	DB     *mongo.Collection
	Logger logger.Interface
}

// Add adds a task
func (s MongoDBTaskRepository) Add(ctx context.Context, task *Task) error {
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
func (s MongoDBTaskRepository) Update(ctx context.Context, taskID string, userID string, task *TaskUpdate) error {
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
func (s MongoDBTaskRepository) FindAll(ctx context.Context, userID string, page int, pageSize int, filters []Filter) ([]Task, int, error) {
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

	queryFilter := bson.D{{Key: "userId", Value: userObjectID}}
	for _, filter := range filters {
		if filter.Operator != "" {
			queryFilter = append(queryFilter, bson.E{Key: filter.Field, Value: bson.M{filter.Operator: filter.Value}})
			continue
		}
		queryFilter = append(queryFilter, bson.E{Key: filter.Field, Value: filter.Value})
	}

	cursor, err := s.DB.Find(ctx, queryFilter, findOptions)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.DB.CountDocuments(ctx, queryFilter)
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
func (s MongoDBTaskRepository) FindAllByWorkUnits(ctx context.Context, userID string, page int, pageSize int, filters []Filter) ([]TaskUnwound, int, error) {

	var results []struct {
		AllResults []TaskUnwound

		TotalCount struct {
			Count int
		}
	}

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, 0, err
	}

	offset := page * pageSize

	queryFilters := bson.D{{Key: "userId", Value: userObjectID}}

	matchStage := bson.D{{Key: "$match", Value: queryFilters}}
	addFieldsStage := bson.D{{Key: "$addFields", Value: bson.M{"workUnitsCount": bson.M{"$size": "$workUnits"}}}}
	addFieldStage2 := bson.D{{Key: "$addFields", Value: bson.M{"workUnit": "$workUnits"}}}
	unwindStage := bson.D{{Key: "$unwind", Value: bson.M{"path": "$workUnit", "includeArrayIndex": "workUnitsIndex"}}}

	queryWorkUnitFilters := bson.D{}
	for _, filter := range filters {
		queryWorkUnitFilters = append(queryWorkUnitFilters, bson.E{Key: filter.Field, Value: filter.Value})
	}
	matchStage2 := bson.D{{Key: "$match", Value: queryWorkUnitFilters}}

	facetStage := bson.D{
		{
			Key: "$facet",
			Value: bson.M{
				"allResults": bson.A{bson.D{{Key: "$skip", Value: offset}}, bson.D{{Key: "$limit", Value: pageSize}},
					bson.D{{Key: "$sort", Value: bson.M{"workUnit.scheduledAt.date": 1}}}},
				"totalCount": bson.A{bson.D{{Key: "$count", Value: "count"}}},
			},
		},
	}

	unwindCountStage := bson.D{{Key: "$unwind", Value: bson.M{"path": "$totalCount"}}}

	cursor, err := s.DB.Aggregate(ctx, mongo.Pipeline{matchStage, addFieldsStage, addFieldStage2, unwindStage, matchStage2, facetStage, unwindCountStage})
	if err != nil {
		return nil, 0, err
	}

	err = cursor.All(ctx, &results)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return nil, 0, err
	}

	return results[0].AllResults, results[0].TotalCount.Count, nil
}

// FindByID finds a specific task by ID
func (s MongoDBTaskRepository) FindByID(ctx context.Context, taskID string, userID string) (Task, error) {
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

// FindByCalendarEventID finds a specific task by a calendar event ID in workUnits or dueAt
func (s MongoDBTaskRepository) FindByCalendarEventID(ctx context.Context, calendarEventID string, userID string) (*TaskUpdate, error) {
	t := TaskUpdate{}

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}

	result := s.DB.FindOne(ctx, bson.D{
		{Key: "userId", Value: userObjectID},
		{Key: "$or", Value: bson.A{
			bson.M{"workUnits.scheduledAt.calendarEventID": calendarEventID},
			bson.M{"dueAt.calendarEventID": calendarEventID},
		}},
	})

	if result.Err() != nil {
		return nil, result.Err()
	}

	err = result.Decode(&t)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

// FindUpdatableByID Finds a task and returns the TaskUpdate view of the model
func (s MongoDBTaskRepository) FindUpdatableByID(ctx context.Context, taskID string, userID string) (*TaskUpdate, error) {
	task, err := s.FindByID(ctx, taskID, userID)
	if err != nil {
		return nil, err
	}

	return (*TaskUpdate)(&task), nil
}

// FindIntersectingWithEvent finds tasks whose WorkUnits are scheduled so that they intersect with a given Event
func (s MongoDBTaskRepository) FindIntersectingWithEvent(ctx context.Context, userID string, event *calendar.Event) ([]Task, error) {
	var t []Task

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}

	findOptions := options.Find()
	findOptions.SetSort(bson.M{"dueAt.date.start": 1})

	queryFilter := bson.D{
		{Key: "userId", Value: userObjectID},
		{Key: "workUnits.scheduledAt.date.start", Value: bson.M{"$lt": event.Date.End}},
		{Key: "workUnits.scheduledAt.date.end", Value: bson.M{"$gt": event.Date.Start}},
	}

	cursor, err := s.DB.Find(ctx, queryFilter, findOptions)
	if err != nil {
		return nil, err
	}

	err = cursor.All(ctx, &t)
	if err != nil {
		return nil, err
	}

	return t, nil
}

// Delete deletes a task
func (s MongoDBTaskRepository) Delete(ctx context.Context, taskID string, userID string) error {
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
