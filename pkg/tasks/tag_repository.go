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

// Tag tags tasks to categorize them
type Tag struct {
	ID             primitive.ObjectID `json:"id" bson:"_id"`
	UserID         primitive.ObjectID `json:"-" bson:"userId" validate:"required"`
	Value          string             `json:"value" bson:"value" validate:"required"`
	Color          string             `json:"color" bson:"color" validate:"required"`
	CreatedAt      time.Time          `json:"createdAt" bson:"createdAt"`
	LastModifiedAt time.Time          `json:"lastModifiedAt" bson:"lastModifiedAt"`
	Deleted        bool               `json:"deleted" bson:"deleted"`
}

// TagUpdate is an update view for a tag
type TagUpdate struct {
	ID             primitive.ObjectID `json:"-" bson:"_id"`
	UserID         primitive.ObjectID `json:"-" bson:"userId" validate:"required"`
	Value          string             `json:"value" bson:"value" validate:"required"`
	Color          string             `json:"color" bson:"color" validate:"required"`
	CreatedAt      time.Time          `json:"-" bson:"createdAt"`
	LastModifiedAt time.Time          `json:"-" bson:"lastModifiedAt"`
	Deleted        bool               `json:"deleted" bson:"deleted"`
}

// TagRepository manages the tags of tasks
type TagRepository struct {
	DB     *mongo.Collection
	Logger logger.Interface
}

// Add adds a tag
func (s *TagRepository) Add(ctx context.Context, tag *Tag) error {
	tag.CreatedAt = time.Now()
	tag.LastModifiedAt = time.Now()
	tag.ID = primitive.NewObjectID()

	_, err := s.DB.InsertOne(ctx, tag)
	if err != nil {
		return err
	}

	return nil
}

// Update updates a tag
func (s *TagRepository) Update(ctx context.Context, tag *Tag) error {
	tag.LastModifiedAt = time.Now()

	result, err := s.DB.UpdateOne(ctx, bson.M{"userId": tag.UserID, "_id": tag.ID}, bson.M{"$set": tag})
	if err != nil {
		return err
	}

	if result.MatchedCount != 1 {
		return errors.New("updated count != 1")
	}

	return nil
}

// FindByID finds a specific tag by ID
func (s *TagRepository) FindByID(ctx context.Context, tagID string, userID string, isDeleted bool) (*Tag, error) {
	t := Tag{}

	tagObjectID, err := primitive.ObjectIDFromHex(tagID)
	if err != nil {
		return nil, err
	}

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}

	result := s.DB.FindOne(ctx, bson.M{"userId": userObjectID, "_id": tagObjectID, "deleted": isDeleted})

	if result.Err() != nil {
		return nil, result.Err()
	}

	err = result.Decode(&t)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

// FindByValue finds a specific tag by value
func (s *TagRepository) FindByValue(ctx context.Context, value string, userID string, isDeleted bool) (*Tag, error) {
	t := Tag{}

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}

	result := s.DB.FindOne(ctx, bson.M{"userId": userObjectID, "value": value, "deleted": isDeleted})

	if result.Err() != nil {
		return nil, result.Err()
	}

	err = result.Decode(&t)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

// FindAll finds all tags paginated
func (s *TagRepository) FindAll(ctx context.Context, userID string, page int, pageSize int, filters []Filter, isDeleted bool) ([]Tag, int, error) {
	t := []Tag{}

	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, 0, err
	}

	offset := page * pageSize

	findOptions := options.Find()
	findOptions.SetSort(bson.M{"value": 1})
	findOptions.SetSkip(int64(offset))
	findOptions.SetLimit(int64(pageSize))

	queryFilter := bson.D{{Key: "userId", Value: userObjectID}, {Key: "deleted", Value: isDeleted}}
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

// Delete deletes a tag
func (s *TagRepository) Delete(ctx context.Context, tagID string, userID string) error {
	tagObjectID, err := primitive.ObjectIDFromHex(tagID)
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
		"_id":    tagObjectID,
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

	return nil
}

// DeleteFinally deletes a tag unrecoverable from the database
func (s *TagRepository) DeleteFinally(ctx context.Context, tagID string, userID string) error {
	tagObjectID, err := primitive.ObjectIDFromHex(tagID)
	if err != nil {
		return err
	}
	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}

	_, err = s.DB.DeleteOne(ctx, bson.M{"userId": userObjectID, "_id": tagObjectID})
	if err != nil {
		return err
	}

	return nil
}
