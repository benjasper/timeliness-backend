package tasks

import (
	"context"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
)

// The PlanningController combines the calendar and task implementations
type PlanningController struct {
	repository  calendar.RepositoryInterface
	userService *users.UserService
	ctx         context.Context
}

// NewPlanningController constructs a PlanningController that is specific for a user
func NewPlanningController(ctx context.Context, u *users.User, userService *users.UserService) (*PlanningController, error) {
	controller := PlanningController{}
	var repository calendar.RepositoryInterface

	// TODO: Figure out which repository to use
	repository, err := setupGoogleRepository(ctx, u, userService)
	if err != nil {
		return nil, err
	}

	controller.repository = repository
	controller.ctx = ctx
	controller.userService = userService

	if u.GoogleCalendarConnection.CalendarID == "" {
		calendarID, err := controller.repository.CreateCalendar()
		if err != nil {
			return nil, err
		}
		u.GoogleCalendarConnection.CalendarID = calendarID
		err = userService.Update(ctx, u)
		if err != nil {
			return nil, err
		}
	}

	return &controller, nil
}

func setupGoogleRepository(ctx context.Context, u *users.User, userService *users.UserService) (*calendar.GoogleCalendarRepository, error) {
	oldAccessToken := u.GoogleCalendarConnection.Token.AccessToken
	calendarRepository, err := calendar.NewGoogleCalendarRepository(ctx, u)
	if err != nil {
		return nil, err
	}

	if oldAccessToken != u.GoogleCalendarConnection.Token.AccessToken {
		err := userService.Update(ctx, u)
		if err != nil {
			return nil, err
		}
	}

	return calendarRepository, nil
}

// SuggestTimeslot finds a free timeslot
func (c *PlanningController) SuggestTimeslot(u *users.User, window *calendar.TimeWindow) (*[]calendar.Timespan, error) {
	err := c.repository.AddBusyToWindow(window)
	if err != nil {
		return nil, err
	}

	free := window.ComputeFree()
	return &free, nil
}
