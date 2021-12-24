package tasks

import (
	"context"
	"github.com/pkg/errors"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

// TaskRepositoryInterface is an interface for a *MongoDBTaskRepository
type TaskRepositoryInterface interface {
	Add(ctx context.Context, task *Task) error
	Update(ctx context.Context, task *TaskUpdate, deleted bool) error
	FindAll(ctx context.Context, userID string, page int, pageSize int, filters []Filter, isDoneAndDueAt time.Time, includeDeleted bool) ([]Task, int, error)
	FindAllByWorkUnits(ctx context.Context, userID string, page int, pageSize int, filters []Filter, includeDeleted bool, isDoneAndScheduledAt time.Time) ([]TaskUnwound, int, error)
	FindAllByDate(ctx context.Context, userID string, page int, pageSize int, filters []Filter, date time.Time, sort int) ([]TaskAgenda, int, error)
	FindByID(ctx context.Context, taskID string, userID string, isDeleted bool) (Task, error)
	FindByCalendarEventID(ctx context.Context, calendarEventID string, userID string, isDeleted bool) (*TaskUpdate, error)
	FindUpdatableByID(ctx context.Context, taskID string, userID string, isDeleted bool) (*TaskUpdate, error)
	FindIntersectingWithEvent(ctx context.Context, userID string, event *calendar.Event, ignoreWorkUnitByID string, isDeleted bool) ([]Task, error)
	FindUnscheduledTasks(ctx context.Context, userID string, page int, pageSize int) ([]Task, int, error)
	CountTasksBetween(ctx context.Context, userID string, from time.Time, to time.Time, isDone bool) (int64, error)
	CountWorkUnitsBetween(ctx context.Context, userID string, from time.Time, to time.Time, isDone bool) (int64, error)
	Delete(ctx context.Context, taskID string, userID string) error
	DeleteFinally(ctx context.Context, taskID string, userID string) error
	DeleteTag(ctx context.Context, tagID string, userID string) error
}

// TaskObserver is an Observer
type TaskObserver interface {
	OnNotify(task *Task)
}

// TaskObservable is an Observable
type TaskObservable interface {
	Subscribe(o TaskObserver)
	Unsubscribe(o TaskObserver)
	Publish(task *Task)
}

// MongoDBTaskRepository does everything related to storing and finding tasks
type MongoDBTaskRepository struct {
	DB          *mongo.Collection
	Logger      logger.Interface
	subscribers []TaskObserver
}

// Add adds a task
func (s *MongoDBTaskRepository) Add(ctx context.Context, task *Task) error {
	task.CreatedAt = time.Now()
	task.LastModifiedAt = time.Now()
	task.ID = primitive.NewObjectID()

	for index, unit := range task.WorkUnits {
		if unit.ID.IsZero() {
			task.WorkUnits[index].ID = primitive.NewObjectID()
		}
	}

	_, err := s.DB.InsertOne(ctx, task)
	if err != nil {
		return err
	}

	s.Publish(task)

	return nil
}

// Update updates a task
func (s *MongoDBTaskRepository) Update(ctx context.Context, task *TaskUpdate, deleted bool) error {
	task.LastModifiedAt = time.Now()

	for index, unit := range task.WorkUnits {
		if unit.ID.IsZero() {
			task.WorkUnits[index].ID = primitive.NewObjectID()
		}
	}

	result, err := s.DB.UpdateOne(ctx, bson.M{
		"$or": bson.A{
			bson.D{
				{Key: "userId", Value: task.UserID},
			},
			bson.D{
				{Key: "collaborators.userId", Value: task.UserID},
			},
		},
		"_id": task.ID, "deleted": deleted,
	}, bson.M{"$set": task})
	if err != nil {
		return err
	}

	if result.MatchedCount != 1 {
		return errors.New("updated count != 1")
	}

	s.Publish((*Task)(task))

	return nil
}

// FindAll finds all task paginated
func (s *MongoDBTaskRepository) FindAll(ctx context.Context, userID string, page int, pageSize int, filters []Filter, isDoneAndDueAt time.Time, includeDeleted bool) ([]Task, int, error) {
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

	filter := bson.D{{
		Key: "$or", Value: bson.A{
			bson.D{
				{Key: "userId", Value: userObjectID},
			},
			bson.D{
				{Key: "collaborators.userId", Value: userObjectID},
			},
		},
	}}
	var queryFilter bson.D

	if !includeDeleted {
		queryFilter = append(queryFilter, bson.E{Key: "deleted", Value: false})
	}

	for _, filter := range filters {
		if filter.Operator != "" {
			queryFilter = append(queryFilter, bson.E{Key: filter.Field, Value: bson.M{filter.Operator: filter.Value}})
			continue
		}
		queryFilter = append(queryFilter, bson.E{Key: filter.Field, Value: filter.Value})
	}

	if (isDoneAndDueAt != time.Time{}) {
		filter = append(filter, bson.E{Key: "$or", Value: bson.A{
			bson.M{"isDone": false},
			bson.M{"isDone": true, "dueAt.date.start": bson.M{"$gte": isDoneAndDueAt}},
		}})
	}

	filter = append(filter, queryFilter...)

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
func (s *MongoDBTaskRepository) FindAllByWorkUnits(ctx context.Context, userID string, page int, pageSize int, filters []Filter, includeDeleted bool, isDoneAndScheduledAt time.Time) ([]TaskUnwound, int, error) {

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

	queryFilters := bson.D{{
		Key: "$or", Value: bson.A{
			bson.D{
				{Key: "userId", Value: userObjectID},
			},
			bson.D{
				{Key: "collaborators.userId", Value: userObjectID},
			},
		},
	}}

	if !includeDeleted {
		queryFilters = append(queryFilters, bson.E{Key: "deleted", Value: false})
	}

	matchStage := bson.D{{Key: "$match", Value: queryFilters}}
	addFieldsStage := bson.D{{Key: "$addFields", Value: bson.M{"workUnitsCount": bson.M{"$size": "$workUnits"}}}}
	addFieldStage2 := bson.D{{Key: "$addFields", Value: bson.M{"workUnit": "$workUnits"}}}
	unwindStage := bson.D{{Key: "$unwind", Value: bson.M{"path": "$workUnit", "includeArrayIndex": "workUnitsIndex"}}}

	queryWorkUnitFilters := bson.D{}
	for _, filter := range filters {
		if filter.Operator != "" {
			queryWorkUnitFilters = append(queryWorkUnitFilters, bson.E{Key: filter.Field, Value: bson.M{filter.Operator: filter.Value}})
			continue
		}
		queryWorkUnitFilters = append(queryWorkUnitFilters, bson.E{Key: filter.Field, Value: filter.Value})
	}

	if (isDoneAndScheduledAt != time.Time{}) {
		queryWorkUnitFilters = append(queryWorkUnitFilters, bson.E{Key: "$or", Value: bson.A{
			bson.M{"workUnit.isDone": false},
			bson.M{"workUnit.isDone": true, "workUnit.scheduledAt.date.start": bson.M{"$gte": isDoneAndScheduledAt}},
		}})
	}

	matchStage2 := bson.D{{Key: "$match", Value: queryWorkUnitFilters}}

	facetStage := bson.D{
		{
			Key: "$facet",
			Value: bson.M{
				"allResults": bson.A{
					bson.D{{Key: "$sort", Value: bson.M{"workUnit.scheduledAt.date.start": 1}}},
					bson.D{{Key: "$skip", Value: offset}}, bson.D{{Key: "$limit", Value: pageSize}},
				},
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

// FindUnscheduledTasks finds tasks where task.NotScheduled != 0, paginated
func (s *MongoDBTaskRepository) FindUnscheduledTasks(ctx context.Context, userID string, page int, pageSize int) ([]Task, int, error) {
	var t []Task

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, 0, err
	}

	offset := page * pageSize

	filter := bson.D{
		{
			Key: "$or", Value: bson.A{
				bson.D{
					{Key: "userId", Value: userObjectID},
				},
				bson.D{
					{Key: "collaborators.userId", Value: userObjectID},
				},
			},
		},
		{Key: "deleted", Value: false},
		{Key: "notScheduled", Value: bson.M{"$ne": 0}},
	}

	findOptions := options.Find()
	findOptions.SetSkip(int64(offset))
	findOptions.SetLimit(int64(pageSize))
	findOptions.SetSort(bson.D{{Key: "dueAt.start.date", Value: 1}})

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

// FindAllByDate finds all task, combining work units and due dates
func (s *MongoDBTaskRepository) FindAllByDate(ctx context.Context, userID string, page int, pageSize int, filters []Filter, date time.Time, sort int) ([]TaskAgenda, int, error) {
	var results []struct {
		AllResults []TaskAgenda

		TotalCount struct {
			Count int
		}
	}

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, 0, err
	}

	offset := page * pageSize

	queryFilters := bson.D{{
		Key: "$or", Value: bson.A{
			bson.D{
				{Key: "userId", Value: userObjectID},
			},
			bson.D{
				{Key: "collaborators.userId", Value: userObjectID},
			},
		},
	},
		{
			Key: "deleted", Value: false,
		},
	}

	queryWorkUnitFilters := bson.D{}
	for _, filter := range filters {
		if filter.Operator != "" {
			queryWorkUnitFilters = append(queryWorkUnitFilters, bson.E{Key: filter.Field, Value: bson.M{filter.Operator: filter.Value}})
			continue
		}
		queryWorkUnitFilters = append(queryWorkUnitFilters, bson.E{Key: filter.Field, Value: filter.Value})
	}

	matchStage := bson.D{{Key: "$match", Value: queryFilters}}
	addFieldsStage := bson.D{{
		Key: "$addFields", Value: bson.M{
			"dueAtDate":     bson.A{"$dueAt"},
			"workUnitDates": "$workUnits.scheduledAt",
		},
	}}

	addFieldsStage2 := bson.D{{
		Key: "$addFields", Value: bson.M{
			"dueAtDate.type":     AgendaDueAt,
			"workUnitDates.type": AgendaWorkUnit,
		},
	}}

	addFieldsStage3 := bson.D{{
		Key: "$addFields", Value: bson.M{
			"date": bson.D{{
				Key:   "$concatArrays",
				Value: bson.A{"$dueAtDate", "$workUnitDates"},
			}},
		},
	}}

	unwindStage := bson.D{{
		Key: "$unwind", Value: bson.M{
			"path":              "$date",
			"includeArrayIndex": "workUnitIndex",
		},
	}}

	setStage := bson.D{{
		Key: "$set", Value: bson.M{
			"workUnitIndex": bson.M{"$subtract": bson.A{
				"$workUnitIndex",
				1,
			}},
		},
	}}

	direction := "$gte"
	if sort == -1 {
		direction = "$lte"
	}

	matchStage2 := bson.D{{
		Key: "$match", Value: bson.D{
			{Key: "date.date.start", Value: bson.M{direction: date}},
		},
	}}

	facetStage := bson.D{
		{
			Key: "$facet",
			Value: bson.M{
				"allResults": bson.A{
					bson.D{{Key: "$sort", Value: bson.M{"date.date.start": sort}}},
					bson.D{{Key: "$skip", Value: offset}}, bson.D{{Key: "$limit", Value: pageSize}},
				},
				"totalCount": bson.A{bson.D{{Key: "$count", Value: "count"}}},
			},
		},
	}

	unwindCountStage := bson.D{{Key: "$unwind", Value: bson.M{"path": "$totalCount"}}}

	cursor, err := s.DB.Aggregate(ctx, mongo.Pipeline{matchStage, addFieldsStage, addFieldsStage2, addFieldsStage3, unwindStage, matchStage2, setStage, facetStage, unwindCountStage})
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
func (s *MongoDBTaskRepository) FindByID(ctx context.Context, taskID string, userID string, isDeleted bool) (Task, error) {
	t := Task{}

	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return t, err
	}
	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return t, err
	}

	result := s.DB.FindOne(ctx, bson.D{
		{
			Key: "$or", Value: bson.A{
				bson.D{
					{Key: "userId", Value: userObjectID},
				},
				bson.D{
					{Key: "collaborators.userId", Value: userObjectID},
				},
			},
		}, {Key: "_id", Value: taskObjectID}, {Key: "deleted", Value: isDeleted}})

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
func (s *MongoDBTaskRepository) FindByCalendarEventID(ctx context.Context, calendarEventID string, userID string, isDeleted bool) (*TaskUpdate, error) {
	t := TaskUpdate{}

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}

	result := s.DB.FindOne(ctx, bson.D{
		{
			Key: "$or", Value: bson.A{
				bson.D{
					{Key: "userId", Value: userObjectID},
				},
				bson.D{
					{Key: "collaborators.userId", Value: userObjectID},
				},
			},
		},
		{Key: "deleted", Value: isDeleted},
		{Key: "$or", Value: bson.A{
			bson.M{"workUnits.scheduledAt.calendarEvents.calendarEventID": calendarEventID},
			bson.M{"dueAt.calendarEvents.calendarEventID": calendarEventID},
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
func (s *MongoDBTaskRepository) FindUpdatableByID(ctx context.Context, taskID string, userID string, isDeleted bool) (*TaskUpdate, error) {
	task, err := s.FindByID(ctx, taskID, userID, isDeleted)
	if err != nil {
		return nil, err
	}

	return (*TaskUpdate)(&task), nil
}

// FindIntersectingWithEvent finds tasks whose WorkUnits are scheduled so that they intersect with a given Event
// The ignoreWorkUnitByID Parameter is optional so it can be empty
func (s *MongoDBTaskRepository) FindIntersectingWithEvent(ctx context.Context, userID string, event *calendar.Event, ignoreWorkUnitByID string, isDeleted bool) ([]Task, error) {
	var t []Task

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}

	arrayMatch := bson.D{
		{Key: "scheduledAt.date.start", Value: bson.M{"$lt": event.Date.End}},
		{Key: "scheduledAt.date.end", Value: bson.M{"$gt": event.Date.Start}},
	}

	if ignoreWorkUnitByID != "" {
		workUnitObjectID, err := primitive.ObjectIDFromHex(ignoreWorkUnitByID)
		if err != nil {
			return nil, err
		}

		arrayMatch = append(arrayMatch, bson.E{
			Key: "_id", Value: bson.M{"$ne": workUnitObjectID},
		})
	}

	findOptions := options.Find()
	findOptions.SetSort(bson.M{"dueAt.date.start": 1})

	queryFilter := bson.D{
		{
			Key: "$or", Value: bson.A{
				bson.D{
					{Key: "userId", Value: userObjectID},
				},
				bson.D{
					{Key: "collaborators.userId", Value: userObjectID},
				},
			},
		},
		{Key: "deleted", Value: isDeleted},
		{Key: "workUnits", Value: bson.M{
			"$elemMatch": arrayMatch,
		},
		},
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

// CountTasksBetween counts tasks between dates
func (s *MongoDBTaskRepository) CountTasksBetween(ctx context.Context, userID string, from time.Time, to time.Time, isDone bool) (int64, error) {
	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return 0, err
	}

	queryFilter := bson.D{
		{
			Key: "$or", Value: bson.A{
				bson.D{
					{Key: "userId", Value: userObjectID},
				},
				bson.D{
					{Key: "collaborators.userId", Value: userObjectID},
				}},
		},
		{Key: "deleted", Value: false},
		{Key: "isDone", Value: isDone},
		{Key: "dueAt.date.start", Value: bson.M{"$gte": from}},
		{Key: "dueAt.date.start", Value: bson.M{"$lte": to}},
	}

	result, err := s.DB.CountDocuments(ctx, queryFilter)
	if err != nil {
		return 0, err
	}

	return result, nil
}

// CountWorkUnitsBetween counts work units between dates
func (s *MongoDBTaskRepository) CountWorkUnitsBetween(ctx context.Context, userID string, from time.Time, to time.Time, isDone bool) (int64, error) {
	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return 0, err
	}

	var result []struct {
		Count int64
	}

	matchStage := bson.D{{Key: "$match", Value: bson.D{{
		Key: "$or", Value: bson.A{
			bson.D{
				{Key: "userId", Value: userObjectID},
			},
			bson.D{
				{Key: "collaborators.userId", Value: userObjectID},
			},
		},
	},
		{
			Key: "deleted", Value: false,
		},
	}}}

	unwindStage := bson.D{{Key: "$unwind", Value: bson.M{"path": "$workUnits", "includeArrayIndex": "workUnitsIndex"}}}

	matchStage2 := bson.D{{Key: "$match", Value: bson.D{
		{Key: "workUnits.scheduledAt.date.start", Value: bson.M{"$gte": from}},
		{Key: "workUnits.scheduledAt.date.start", Value: bson.M{"$lte": to}},
		{Key: "workUnits.isDone", Value: isDone},
	}}}

	countStage := bson.D{
		{
			Key:   "$count",
			Value: "count",
		},
	}

	cursor, err := s.DB.Aggregate(ctx, mongo.Pipeline{matchStage, unwindStage, matchStage2, countStage})
	if err != nil {
		return 0, err
	}

	err = cursor.All(ctx, &result)
	if err != nil {
		return 0, err
	}

	if len(result) == 0 {
		return 0, nil
	}

	return result[0].Count, nil
}

// Delete deletes a task
func (s *MongoDBTaskRepository) Delete(ctx context.Context, taskID string, userID string) error {
	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}
	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}

	findOptions := options.FindOneAndUpdate()
	findOptions.SetReturnDocument(options.After)

	result := s.DB.FindOneAndUpdate(ctx, bson.M{
		"_id":    taskObjectID,
		"userId": userObjectID,
	},
		bson.M{
			"$set": bson.M{
				"deleted":        true,
				"lastModifiedAt": time.Now(),
			},
		}, findOptions)
	if result.Err() != nil {
		return result.Err()
	}

	s.Publish(&Task{ID: taskObjectID, UserID: userObjectID, Deleted: true})

	return nil
}

// DeleteFinally deletes a task unrecoverable from the database
func (s *MongoDBTaskRepository) DeleteFinally(ctx context.Context, taskID string, userID string) error {
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

// DeleteTag deletes a tag from tasks
func (s *MongoDBTaskRepository) DeleteTag(ctx context.Context, tagID string, userID string) error {
	tagObjectID, err := primitive.ObjectIDFromHex(tagID)
	if err != nil {
		return err
	}
	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}

	_, err = s.DB.UpdateMany(ctx,
		bson.D{
			{
				Key: "$or", Value: bson.A{
					bson.D{
						{Key: "userId", Value: userObjectID},
					},
					bson.D{
						{Key: "collaborators.userId", Value: userObjectID},
					},
				},
			},
			{Key: "tags", Value: tagObjectID},
		}, bson.M{
			"$set": bson.M{
				"lastModifiedAt": time.Now(),
			},
			"$pull": bson.M{
				"tags": tagObjectID,
			},
		})

	if err != nil {
		return err
	}

	// TODO: Publish event for all changed tasks

	return nil
}

// Subscribe is useful for listening to task changes
func (s *MongoDBTaskRepository) Subscribe(o TaskObserver) {
	s.subscribers = append(s.subscribers, o)
}

// Unsubscribe unsubscribes from a subscription
func (s *MongoDBTaskRepository) Unsubscribe(o TaskObserver) {
	var index int
	for i, subscriber := range s.subscribers {
		if subscriber == o {
			index = i
			break
		}
	}

	s.subscribers = append(s.subscribers[:index], s.subscribers[index+1:]...)
}

// Publish published a task to all subscribers
func (s *MongoDBTaskRepository) Publish(task *Task) {
	for _, subscriber := range s.subscribers {
		go subscriber.OnNotify(task)
	}
}
