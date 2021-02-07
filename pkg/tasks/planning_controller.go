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

	if u.GoogleCalendarConnection.TaskCalendar.CalendarID == "" {
		calendarID, err := calendarRepository.CreateCalendar()
		if err != nil {
			return nil, err
		}

		u.GoogleCalendarConnection.TaskCalendar.CalendarID = calendarID
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

// ScheduleNewTask takes a new task a non existent task and creates workunits and pushes events to the calendar
func (c *PlanningController) ScheduleNewTask(t *Task, u *users.User) error {
	window := calendar.TimeWindow{Start: time.Now().Add(time.Minute * 15).Round(time.Minute * 15), End: t.DueAt.Date.Start}
	err := c.repository.AddBusyToWindow(&window)
	if err != nil {
		return err
	}

	loc, err := time.LoadLocation("")
	if err != nil {
		return err
	}

	constraint := calendar.FreeConstraint{
		AllowedTimeSpans: []calendar.Timespan{{
			Start: time.Date(0, 0, 0, 7, 0, 0, 0, loc),
			End:   time.Date(0, 0, 0, 15, 30, 0, 0, loc),
		}}}
	window.ComputeFree(&constraint)

	var workunits []WorkUnit

	dueEvent, err := c.repository.NewEvent(t.Name+" is due", "", false, &t.DueAt)
	if err != nil {
		return err
	}

	t.DueAt = *dueEvent

	for i := t.WorkloadOverall; i > 0; {
		minDuration := 2 * time.Hour
		maxDuration := 6 * time.Hour
		if i < 6*time.Hour {
			if i < 2*time.Hour {
				minDuration = i
			}
			maxDuration = i
		}

		var rules = []calendar.RuleInterface{&calendar.RuleDuration{Minimum: minDuration, Maximum: maxDuration}}
		timeslot := window.FindTimeSlot(&rules)
		if timeslot == nil {
			log.Panic("Found timeslot is nil")
		}

		workunit := WorkUnit{
			Workload:    timeslot.Duration(),
			ScheduledAt: calendar.Event{Date: *timeslot},
		}

		event, err := c.repository.NewEvent("Working at "+t.Name, "", true,
			&workunit.ScheduledAt)
		if err != nil {
			return err
		}

		workunit.ScheduledAt = *event
		i -= workunit.Workload

		workunits = append(workunits, workunit)
	}

	t.WorkUnits = workunits

	return nil
}
