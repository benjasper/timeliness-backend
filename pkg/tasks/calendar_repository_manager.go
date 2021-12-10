package tasks

import (
	"context"
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

	calendarRepository, err := m.setupGoogleRepository(ctx, user)
	if err != nil {
		return nil, err
	}

	return []calendar.RepositoryInterface{calendarRepository}, nil
}

// GetTaskCalendarRepositoryForUser gets the task calendar repository for a user
func (m *CalendarRepositoryManager) GetTaskCalendarRepositoryForUser(ctx context.Context, user *users.User) (calendar.RepositoryInterface, error) {
	if len(m.overriddenRepos) > 0 && m.overriddenRepos[user.ID.Hex()] != nil {
		return m.overriddenRepos[user.ID.Hex()], nil
	}

	calendarRepository, err := m.setupGoogleRepository(ctx, user)
	if err != nil {
		return nil, err
	}

	return calendarRepository, nil
}

// GetCalendarRepositoryForUserAndCalendarID gets a specific calendar repository for a user
func (m *CalendarRepositoryManager) GetCalendarRepositoryForUserAndCalendarID(ctx context.Context, user *users.User, calendarID string) (calendar.RepositoryInterface, error) {
	// TODO: return the calendar repository which contains the given calendar id
	if len(m.overriddenRepos) > 0 && m.overriddenRepos[user.ID.Hex()] != nil {
		return m.overriddenRepos[user.ID.Hex()], nil
	}

	calendarRepository, err := m.setupGoogleRepository(ctx, user)
	if err != nil {
		return nil, err
	}

	return calendarRepository, nil
}

// setupGoogleRepository manages token refreshing and calendar creation
func (m *CalendarRepositoryManager) setupGoogleRepository(ctx context.Context, u *users.User) (*calendar.GoogleCalendarRepository, error) {
	oldAccessToken := u.GoogleCalendarConnection.Token.AccessToken

	calendarRepository, err := calendar.NewGoogleCalendarRepository(ctx, u.ID, &u.GoogleCalendarConnection, m.logger)
	if err != nil {
		return nil, err
	}

	if oldAccessToken != u.GoogleCalendarConnection.Token.AccessToken {
		err := m.userRepository.Update(ctx, u)
		if err != nil {
			return nil, err
		}
	}

	if u.GoogleCalendarConnection.TaskCalendarID == "" {
		calendarID, err := calendarRepository.CreateCalendar()
		if err != nil {
			return nil, err
		}

		u.GoogleCalendarConnection.TaskCalendarID = calendarID
		u.GoogleCalendarConnection.CalendarsOfInterest = append(u.GoogleCalendarConnection.CalendarsOfInterest,
			users.GoogleCalendarSync{CalendarID: calendarID})
		err = m.userRepository.Update(ctx, u)
		if err != nil {
			return nil, err
		}
	}

	return calendarRepository, nil
}
