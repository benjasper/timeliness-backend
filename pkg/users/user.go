package users

import (
	"context"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type User struct {
	ID             primitive.ObjectID `json:"id" bson:"_id"`
	Firstname      string             `json:"firstname" validate:"required"`
	Lastname       string             `json:"lastname" validate:"required"`
	Password       string             `json:"-" bson:"password" validate:"required"`
	Email          string             `json:"email"  validate:"required,email"`
	CreatedAt      time.Time          `json:"createdAt" bson:"createdAt" validate:"isdefault"`
	LastModifiedAt time.Time          `json:"lastModifiedAt" bson:"lastModifiedAt" validate:"isdefault"`
}

type UserLogin struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" bson:"password" validate:"required"`
}

type ServiceInterface interface {
	Add(ctx context.Context, user *User) error
	FindByID(ctx context.Context, id string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	Remove(ctx context.Context, id string) error
}
