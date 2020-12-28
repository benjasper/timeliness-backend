package tasks

import (
	"context"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"log"
	"time"
)

// The PlanningController combines the calendar and task implementations
type PlanningController struct {
	repository  calendar.RepositoryInterface
	userService *users.UserService
	taskService *TaskService
	ctx         context.Context
}

// NewPlanningController constructs a PlanningController that is specific for a user
func NewPlanningController(ctx context.Context, u *users.User, userService *users.UserService, taskService *TaskService) (*PlanningController, error) {
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
	controller.taskService = taskService

	return &controller, nil
}

// setupGoogleRepository manages token refreshing and calendar creation
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

	if u.GoogleCalendarConnection.CalendarID == "" {
		calendarID, err := calendarRepository.CreateCalendar()
		if err != nil {
			return nil, err
		}

		u.GoogleCalendarConnection.CalendarID = calendarID
		err = userService.Update(ctx, u)
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

	loc, err := time.LoadLocation("Local")
	if err != nil {
		log.Panic(err)
	}

	constraint := calendar.FreeConstraint{
		AllowedTimeSpans: []calendar.Timespan{{
			Start: time.Date(0, 0, 0, 8, 0, 0, 0, loc),
			End:   time.Date(0, 0, 0, 16, 30, 0, 0, loc),
		}}}
	free := window.ComputeFree(&constraint)
	return &free, nil
}

func (c *PlanningController) ScheduleNewTask(t *Task) error {
	window := calendar.TimeWindow{Start: time.Now(), End: t.DueAt.Start}
	err := c.repository.AddBusyToWindow(&window)
	if err != nil {
		return err
	}

	return nil
}
