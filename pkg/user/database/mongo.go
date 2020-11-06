package database

import (
	"context"
	"github.com/benjasper/project-tasks/pkg/logger"
	"github.com/benjasper/project-tasks/pkg/user"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserService struct {
	DB     *mongo.Collection
	Logger logger.Interface
}

func (s UserService) Add(user *user.User, ctx context.Context) error {
	_, err := s.DB.InsertOne(ctx, user)
	return err
}

func (s UserService) FindById(id string, ctx context.Context) (*user.User, error) {
	var u = user.User{}
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

func (s UserService) Update(user *user.User, ctx context.Context) error {
	panic("implement me")
}

func (s UserService) Remove(id string, ctx context.Context) error {
	panic("implement me")
}
