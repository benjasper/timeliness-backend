package tasks

import (
	"context"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
)

type PlanningController struct {
	repository  calendar.RepositoryInterface
	userService *users.UserService
	ctx         context.Context
}

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

func (c *PlanningController) SuggestTimeslot(u *users.User, window *calendar.TimeWindow) (*[]calendar.Timespan, error) {
	err := c.repository.AddBusyToWindow(window)
	if err != nil {
		return nil, err
	}

	free := window.ComputeFree()
	return &free, nil
}
