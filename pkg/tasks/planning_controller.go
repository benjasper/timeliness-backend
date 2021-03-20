package tasks

import (
	"context"
	"fmt"
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
	now := time.Now().Add(time.Minute * 15).Round(time.Minute * 15)
	windowTotal := calendar.TimeWindow{Start: now, End: t.DueAt.Date.Start}
	err := c.repository.AddBusyToWindow(&windowTotal)
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

	windowTotal.ComputeFree(&constraint)

	var workUnits []WorkUnit
	for _, workUnit := range findWorkUnitTimes(&windowTotal, t.WorkloadOverall) {
		workUnit.ScheduledAt.Blocking = true
		workUnit.ScheduledAt.Title = fmt.Sprintf("⚙️ Working on %s", t.Name)
		workUnit.ScheduledAt.Description = ""

		workEvent, err := c.repository.NewEvent(&workUnit.ScheduledAt)
		if err != nil {
			return err
		}

		workUnit.ScheduledAt = *workEvent
		workUnits = append(workUnits, workUnit)
	}

	t.DueAt.Blocking = false
	t.DueAt.Title = fmt.Sprintf("📅 %s is due", t.Name)
	t.DueAt.Date.End = t.DueAt.Date.Start.Add(time.Minute * 15)
	t.DueAt.Description = ""

	dueEvent, err := c.repository.NewEvent(&t.DueAt)
	if err != nil {
		return err
	}

	t.DueAt = *dueEvent

	t.WorkUnits = workUnits

	return nil
}

func findWorkUnitTimes(w *calendar.TimeWindow, durationToFind time.Duration) []WorkUnit {
	var workUnits []WorkUnit
	if w.FreeDuration == 0 {
		return workUnits
	}

	if w.Duration() < 24*time.Hour*7 {
		minDuration := 2 * time.Hour
		maxDuration := 6 * time.Hour

		for w.FreeDuration >= minDuration && durationToFind != 0 {
			if durationToFind < 6*time.Hour {
				if durationToFind < 2*time.Hour {
					minDuration = durationToFind
				}
				maxDuration = durationToFind
			}

			var rules = []calendar.RuleInterface{&calendar.RuleDuration{Minimum: minDuration, Maximum: maxDuration}}
			slot := w.FindTimeSlot(&rules)
			if slot == nil {
				break
			}
			durationToFind -= slot.Duration()
			workUnits = append(workUnits, WorkUnit{ScheduledAt: calendar.Event{Date: *slot}, Workload: slot.Duration()})
		}
	}

	durationThird := w.Duration() / 3

	windowMiddle := w.GetPreferredTimeWindow(w.Start.Add(durationThird), w.Start.Add(durationThird*2))
	if windowMiddle.FreeDuration > 0 && durationToFind != 0 {
		found := findWorkUnitTimes(windowMiddle, durationToFind)
		for _, unit := range found {
			durationToFind -= unit.Workload
		}
		if len(found) > 0 {
			workUnits = append(workUnits, found...)
		}
	}

	windowRight := w.GetPreferredTimeWindow(w.Start.Add(durationThird*2), w.End)
	if windowRight.FreeDuration > 0 && durationToFind != 0 {
		found := findWorkUnitTimes(windowRight, durationToFind)
		for _, unit := range found {
			durationToFind -= unit.Workload
		}
		if len(found) > 0 {
			workUnits = append(workUnits, found...)
		}
	}

	windowLeft := w.GetPreferredTimeWindow(w.Start, w.Start.Add(durationThird))
	if windowLeft.FreeDuration > 0 && durationToFind != 0 {
		found := findWorkUnitTimes(windowLeft, durationToFind)
		for _, unit := range found {
			durationToFind -= unit.Workload
		}
		if len(found) > 0 {
			workUnits = append(workUnits, found...)
		}
	}

	return workUnits
}

// DeleteTask deletes all events that are connected to a task
func (c *PlanningController) DeleteTask(task *Task) error {
	for _, unit := range task.WorkUnits {
		err := c.repository.DeleteEvent(&unit.ScheduledAt)
		if err != nil {
			return err
		}
	}

	err := c.repository.DeleteEvent(&task.DueAt)
	if err != nil {
		return err
	}

	return nil
}
