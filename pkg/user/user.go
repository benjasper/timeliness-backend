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
	Add(user *User, ctx context.Context) error
	FindById(id string, ctx context.Context) (*User, error)
	Update(user *User, ctx context.Context) error
	Remove(id string, ctx context.Context) error
}
