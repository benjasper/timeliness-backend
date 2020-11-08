package users

import (
	"context"
	"errors"
	"github.com/benjasper/project-tasks/pkg/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

type UserService struct {
	DB     *mongo.Collection
	Logger logger.Interface
}

func (s UserService) Add(ctx context.Context, user *User) error {
	user.CreatedAt = time.Now()
	user.LastModifiedAt = time.Now()
	user.ID = primitive.NewObjectID()
	_, err := s.DB.InsertOne(ctx, user)
	return err
}

func (s UserService) FindByID(ctx context.Context, id string) (*User, error) {
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

func (s UserService) FindByEmail(ctx context.Context, email string) (*User, error) {
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

func (s UserService) Update(ctx context.Context, user *User) error {
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

func (s UserService) Remove(ctx context.Context, id string) error {
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
