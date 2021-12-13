package tasks

import (
	"context"
	"fmt"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
)

// CalendarRepositoryManager manages calendar repositories. It decided which user needs which repository.
type CalendarRepositoryManager struct {
	userRepository  users.UserRepositoryInterface
	logger          logger.Interface
	overriddenRepos map[string]calendar.RepositoryInterface
}

// NewCalendarRepositoryManager creates a new CalendarRepositoryManager
func NewCalendarRepositoryManager(_ int, userRepository users.UserRepositoryInterface, logger logger.Interface) (*CalendarRepositoryManager, error) {
	manager := CalendarRepositoryManager{userRepository: userRepository, logger: logger}

	return &manager, nil
}

// GetAllCalendarRepositoriesForUser gets all calendar repositories for a user
func (m *CalendarRepositoryManager) GetAllCalendarRepositoriesForUser(ctx context.Context, user *users.User) ([]calendar.RepositoryInterface, error) {
	// TODO: Figure out which calendarRepository to use
	if len(m.overriddenRepos) > 0 && m.overriddenRepos[user.ID.Hex()] != nil {
		return []calendar.RepositoryInterface{m.overriddenRepos[user.ID.Hex()]}, nil
	}

	var repos []calendar.RepositoryInterface

	// TODO: Make parallel
	for i, connection := range user.GoogleCalendarConnections {
		if connection.Status != users.CalendarConnectionStatusActive {
			continue
		}
		calendarRepository, err := m.setupGoogleRepository(ctx, user, &connection, i)
		if err != nil {
			return nil, err
		}

		repos = append(repos, calendarRepository)
	}

	return repos, nil
}

// GetTaskCalendarRepositoryForUser gets the task calendar repository for a user
func (m *CalendarRepositoryManager) GetTaskCalendarRepositoryForUser(ctx context.Context, user *users.User) (calendar.RepositoryInterface, error) {
	if len(m.overriddenRepos) > 0 && m.overriddenRepos[user.ID.Hex()] != nil {
		return m.overriddenRepos[user.ID.Hex()], nil
	}

	for i, connection := range user.GoogleCalendarConnections {
		if connection.IsTaskCalendarConnection {
			calendarRepository, err := m.setupGoogleRepository(ctx, user, &connection, i)
			if err != nil {
				return nil, err
			}

			return calendarRepository, nil
		}
	}

	return nil, fmt.Errorf("could not find a connection that has a task calendar connection for user %s", user.ID.Hex())
}

// GetCalendarRepositoryForUserByCalendarID gets a specific calendar repository for a user
func (m *CalendarRepositoryManager) GetCalendarRepositoryForUserByCalendarID(ctx context.Context, user *users.User, calendarID string) (calendar.RepositoryInterface, error) {
	if len(m.overriddenRepos) > 0 && m.overriddenRepos[user.ID.Hex()] != nil {
		return m.overriddenRepos[user.ID.Hex()], nil
	}

	for i, connection := range user.GoogleCalendarConnections {
		if connection.CalendarsOfInterest.HasCalendarWithID(calendarID) {
			calendarRepository, err := m.setupGoogleRepository(ctx, user, &connection, i)
			if err != nil {
				return nil, err
			}

			return calendarRepository, nil
		}
	}

	return nil, fmt.Errorf("could not find a connection that contains the given calendar %s for user %s", calendarID, user.ID.Hex())
}

// GetCalendarRepositoryForUserByConnectionID gets a specific calendar repository for a connection
func (m *CalendarRepositoryManager) GetCalendarRepositoryForUserByConnectionID(ctx context.Context, user *users.User, connectionID string) (calendar.RepositoryInterface, error) {
	if len(m.overriddenRepos) > 0 && m.overriddenRepos[user.ID.Hex()] != nil {
		return m.overriddenRepos[user.ID.Hex()], nil
	}

	for i, connection := range user.GoogleCalendarConnections {
		if connection.ID == connectionID {
			calendarRepository, err := m.setupGoogleRepository(ctx, user, &connection, i)
			if err != nil {
				return nil, err
			}

			return calendarRepository, nil
		}
	}

	return nil, fmt.Errorf("could not find a connection that has the given id %s for user %s", connectionID, user.ID.Hex())
}

// CheckIfGoogleTaskCalendarIsSet checks if a task calendar is set already
func (m *CalendarRepositoryManager) CheckIfGoogleTaskCalendarIsSet(ctx context.Context, u *users.User, connection *users.GoogleCalendarConnection, connectionIndex int) (*users.User, error) {
	if connection.IsTaskCalendarConnection {
		calendarRepository, err := m.setupGoogleRepository(ctx, u, connection, connectionIndex)
		if err != nil {
			return nil, err
		}

		u, err = calendarRepository.TestTaskCalendarExistence(u)
		if err != nil {
			return nil, err
		}

		err = m.userRepository.Update(ctx, u)
		if err != nil {
			return nil, err
		}
	}

	return u, nil
}

// setupGoogleRepository manages token refreshing and calendar creation
func (m *CalendarRepositoryManager) setupGoogleRepository(ctx context.Context, u *users.User, connection *users.GoogleCalendarConnection, connectionIndex int) (*calendar.GoogleCalendarRepository, error) {
	oldAccessToken := connection.Token.AccessToken

	calendarRepository, err := calendar.NewGoogleCalendarRepository(ctx, u.ID, connection, m.logger)
	if err != nil {
		return nil, err
	}

	if oldAccessToken != connection.Token.AccessToken {
		u.GoogleCalendarConnections[connectionIndex] = *connection

		err := m.userRepository.Update(ctx, u)
		if err != nil {
			return nil, err
		}
	}

	return calendarRepository, nil
}
