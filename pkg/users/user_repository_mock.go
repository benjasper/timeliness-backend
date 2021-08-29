package users

import (
	"context"
	"errors"
	"time"
)

// MockUserRepository is a user repository for testing
type MockUserRepository struct {
	Users []*User
}

// Add adds a user
func (r *MockUserRepository) Add(ctx context.Context, user *User) error {
	r.Users = append(r.Users, user)
	return nil
}

// FindByID finds a user
func (r *MockUserRepository) FindByID(ctx context.Context, id string) (*User, error) {
	for _, user := range r.Users {
		if user.ID.Hex() == id {
			return user, nil
		}
	}

	return nil, errors.New("user not found")
}

// FindByEmail finds a user by email
func (r *MockUserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	for _, user := range r.Users {
		if user.Email == email {
			return user, nil
		}
	}

	return nil, errors.New("user not found")
}

// FindByGoogleStateToken finds a user by a GoogleStateToken
func (r *MockUserRepository) FindByGoogleStateToken(ctx context.Context, stateToken string) (*User, error) {
	for _, user := range r.Users {
		if user.GoogleCalendarConnection.StateToken == stateToken {
			return user, nil
		}
	}

	return nil, errors.New("user not found")
}

// FindBySyncExpiration is not implemented yet
func (r *MockUserRepository) FindBySyncExpiration(ctx context.Context, greaterThan time.Time, page int, pageSize int) ([]*User, int, error) {
	panic("implement me")
}

// Update updates a user
func (r *MockUserRepository) Update(ctx context.Context, user *User) error {
	for i, user := range r.Users {
		if user.ID == user.ID {
			r.Users[i] = user
			return nil
		}
	}

	return errors.New("user not found")
}

// Remove removes a user
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
