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

	controller.ctx = ctx
	controller.userService = userService
	// TODO: Figure out which repository to use
	oldAccessToken := u.GoogleCalendarConnection.Token.AccessToken
	calendarRepository, err := calendar.NewGoogleCalendarRepository(ctx, u)
	if err != nil {
		return nil, err
	}

	if oldAccessToken != u.GoogleCalendarConnection.Token.AccessToken {
		println("Refreshed Google access Token")
		err := userService.Update(context.TODO(), u)
		if err != nil {
			return nil, err
		}
	}

	controller.repository = calendarRepository
	return &controller, nil
}

func (c *PlanningController) SuggestTimeslot(u *users.User, window *calendar.TimeWindow) (*[]calendar.Timespan, error) {
	err := c.repository.AddBusyToWindow(window)
	if err != nil {
		return nil, err
	}

	free := window.ComputeFree()
	return &free, nil
}
