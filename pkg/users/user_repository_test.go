package users

import (
	"context"
	"time"
)

type MockUserRepository struct {
	Users []*User
}

func (MockUserRepository) Add(ctx context.Context, user *User) error {
	panic("implement me")
}

func (MockUserRepository) FindByID(ctx context.Context, id string) (*User, error) {
	panic("implement me")
}

func (MockUserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	panic("implement me")
}

func (MockUserRepository) FindByGoogleStateToken(ctx context.Context, stateToken string) (*User, error) {
	panic("implement me")
}

func (MockUserRepository) FindBySyncExpiration(ctx context.Context, greaterThan time.Time, page int, pageSize int) ([]*User, int, error) {
	panic("implement me")
}

func (MockUserRepository) Update(ctx context.Context, user *User) error {
	panic("implement me")
}

func (MockUserRepository) Remove(ctx context.Context, id string) error {
	panic("implement me")
}
