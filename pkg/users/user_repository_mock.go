package users

import (
	"context"
	"github.com/pkg/errors"
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

// FindByIdentityProvider finds a by either email or google ID
func (r *MockUserRepository) FindByIdentityProvider(ctx context.Context, email string, ID string) (*User, error) {
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
		for _, connection := range user.GoogleCalendarConnections {
			if connection.StateToken == stateToken {
				return user, nil
			}
		}
	}

	return nil, errors.New("user not found")
}

// FindByBillingCustomerID finds a user by a billing customer ID
func (r *MockUserRepository) FindByBillingCustomerID(ctx context.Context, customerID string) (*User, error) {
	for _, user := range r.Users {
		if user.Billing.CustomerID == customerID {
			return user, nil
		}
	}

	return nil, errors.New("user not found")
}

// FindByVerificationToken finds a user by email verification token
func (r *MockUserRepository) FindByVerificationToken(ctx context.Context, token string) (*User, error) {
	for _, user := range r.Users {
		if user.EmailVerificationToken == token {
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

// UpdateSettings updates a users settings
func (r *MockUserRepository) UpdateSettings(ctx context.Context, user *User) error {
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
