package users

import (
	"context"
	"github.com/pkg/errors"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

// UserRepositoryInterface is the interface for a UserRepository
type UserRepositoryInterface interface {
	Add(ctx context.Context, user *User) error
	FindByID(ctx context.Context, id string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByGoogleStateToken(ctx context.Context, stateToken string) (*User, error)
	FindByVerificationToken(ctx context.Context, token string) (*User, error)
	FindBySyncExpiration(ctx context.Context, greaterThan time.Time, page int, pageSize int) ([]*User, int, error)
	Update(ctx context.Context, user *User) error
	UpdateSettings(ctx context.Context, user *User) error
	Remove(ctx context.Context, id string) error
}

// UserRepository does everything related to user storing
type UserRepository struct {
	DB     *mongo.Collection
	Logger logger.Interface
}

// Add adds a user
func (s *UserRepository) Add(ctx context.Context, user *User) error {
	user.CreatedAt = time.Now()
	user.LastModifiedAt = time.Now()
	user.ID = primitive.NewObjectID()
	_, err := s.DB.InsertOne(ctx, user)
	return err
}

// FindByID finds a user by ID
func (s *UserRepository) FindByID(ctx context.Context, id string) (*User, error) {
	var u = User{}
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	result := s.DB.FindOne(ctx, bson.M{"_id": objectID})
	if result.Err() != nil {
		return nil, result.Err()
	}

	err = result.Decode(&u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// FindByEmail finds a user by Email
func (s *UserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	var u = User{}

	result := s.DB.FindOne(ctx, bson.M{"email": email})
	if result.Err() != nil {
		return nil, result.Err()
	}

	err := result.Decode(&u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// FindByGoogleStateToken finds a user by its Google state Token
func (s *UserRepository) FindByGoogleStateToken(ctx context.Context, stateToken string) (*User, error) {
	var u = User{}

	result := s.DB.FindOne(ctx, bson.M{"googleCalendarConnections.stateToken": stateToken})
	if result.Err() != nil {
		return nil, result.Err()
	}

	err := result.Decode(&u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// FindByVerificationToken finds a user by its email verification token
func (s *UserRepository) FindByVerificationToken(ctx context.Context, token string) (*User, error) {
	var u = User{}

	result := s.DB.FindOne(ctx, bson.M{"emailVerificationToken": token})
	if result.Err() != nil {
		return nil, result.Err()
	}

	err := result.Decode(&u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// FindBySyncExpiration finds user documents where at least one sync is ready for renewal
func (s *UserRepository) FindBySyncExpiration(ctx context.Context, greaterThan time.Time, page int, pageSize int) ([]*User, int, error) {
	var users []*User
	offset := page * pageSize

	findOptions := options.Find()
	findOptions.SetSort(bson.M{"_id": 1})
	findOptions.SetSkip(int64(offset))
	findOptions.SetLimit(int64(pageSize))

	queryFilter := bson.D{{
		Key:   "googleCalendarConnections.calendarsOfInterest.expiration",
		Value: bson.M{"$lte": greaterThan}},
	}

	cursor, err := s.DB.Find(ctx, queryFilter, findOptions)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.DB.CountDocuments(ctx, queryFilter)
	if err != nil {
		return nil, 0, err
	}

	err = cursor.All(ctx, &users)
	if err != nil {
		return nil, 0, err
	}
	return users, int(count), nil
}

// Update updates a user
func (s *UserRepository) Update(ctx context.Context, user *User) error {
	user.LastModifiedAt = time.Now()

	result, err := s.DB.UpdateOne(ctx, bson.M{"_id": user.ID}, bson.M{"$set": user})
	if err != nil {
		return err
	}

	if result.MatchedCount != 1 {
		return errors.New("updated count != 1")
	}

	return nil
}

// UpdateSettings updates only the user settings
func (s *UserRepository) UpdateSettings(ctx context.Context, user *User) error {
	user.LastModifiedAt = time.Now()

	result, err := s.DB.UpdateOne(ctx, bson.M{"_id": user.ID}, bson.M{"$set": bson.M{"settings": user.Settings, "lastModifiedAt": user.LastModifiedAt}})
	if err != nil {
		return err
	}

	if result.MatchedCount != 1 {
		return errors.New("updated count != 1")
	}

	return nil
}

// Remove Deletes a user
func (s *UserRepository) Remove(ctx context.Context, id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	result, err := s.DB.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return err
	}

	if result.DeletedCount != 1 {
		return err
	}

	return nil
}
