package tasks

import (
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
)

type PlanningController struct {
	repository calendar.RepositoryInterface
}

func NewPlanningController(u *users.User) (*PlanningController, error) {
	controller := PlanningController{}
	// TODO: Figure out which repository to use
	calendarRepository, err := calendar.NewCalendarRepository(u)
	if err != nil {
		return nil, err
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
