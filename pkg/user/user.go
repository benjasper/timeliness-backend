package user

import (
	"context"
	"time"
)

type User struct {
	ID             string    `bson:"_id" json:"id"`
	Firstname      string    `json:"firstname"`
	Lastname       string    `json:"lastname"`
	Password       string    `password:"password,omit"`
	Email          string    `json:"email"`
	CreatedAt      time.Time `json:"createdAt"`
	LastModifiedAt time.Time `json:"lastModifiedAt"`
}

type ServiceInterface interface {
	Add(ctx context.Context, user *User) error
	FindById(ctx context.Context, id string) (*User, error)
	Update(ctx context.Context, user *User) error
	Remove(ctx context.Context, id string) error
}
