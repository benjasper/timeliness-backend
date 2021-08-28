package users

import (
	"context"
	"errors"
	"time"
)

type MockUserRepository struct {
	Users []*User
}

func (r *MockUserRepository) Add(ctx context.Context, user *User) error {
	r.Users = append(r.Users, user)
	return nil
}

func (r *MockUserRepository) FindByID(ctx context.Context, id string) (*User, error) {
	for _, user := range r.Users {
		if user.ID.Hex() == id {
			return user, nil
		}
	}

	return nil, errors.New("user not found")
}

func (r *MockUserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	for _, user := range r.Users {
		if user.Email == email {
			return user, nil
		}
	}

	return nil, errors.New("user not found")
}

func (r *MockUserRepository) FindByGoogleStateToken(ctx context.Context, stateToken string) (*User, error) {
	for _, user := range r.Users {
		if user.GoogleCalendarConnection.StateToken == stateToken {
			return user, nil
		}
	}

	return nil, errors.New("user not found")
}

func (r *MockUserRepository) FindBySyncExpiration(ctx context.Context, greaterThan time.Time, page int, pageSize int) ([]*User, int, error) {
	panic("implement me")
}

func (r *MockUserRepository) Update(ctx context.Context, user *User) error {
	for i, user := range r.Users {
		if user.ID == user.ID {
			r.Users[i] = user
			return nil
		}
	}

	return errors.New("user not found")
}

func (r *MockUserRepository) Remove(ctx context.Context, id string) error {
	var found = false

	for i, u := range r.Users {
		if u.ID.Hex() == id {
			found = true
			r.Users = append(r.Users[:i], r.Users[i+1:]...)
			break
		}
	}

	if !found {
		return errors.New("no task found")
	}

	return nil
}
