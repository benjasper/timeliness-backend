package tasks

import (
	"context"
	"errors"
	"fmt"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"log"
	"time"
)

// The PlanningController combines the calendar and task implementations
type PlanningController struct {
	calendarRepository calendar.RepositoryInterface
	userRepository     users.UserRepositoryInterface
	taskRepository     TaskRepositoryInterface
	ctx                context.Context
	logger             logger.Interface
}

// NewPlanningController constructs a PlanningController that is specific for a user
func NewPlanningController(ctx context.Context, u *users.User, userService users.UserRepositoryInterface, taskRepository TaskRepositoryInterface,
	logger logger.Interface) (*PlanningController, error) {
	controller := PlanningController{}
	var repository calendar.RepositoryInterface

	// TODO: Figure out which calendarRepository to use
	repository, err := setupGoogleRepository(ctx, u, userService, logger)
	if err != nil {
		return nil, err
	}

	controller.calendarRepository = repository
	controller.ctx = ctx
	controller.userRepository = userService
	controller.taskRepository = taskRepository
	controller.logger = logger

	return &controller, nil
}

// setupGoogleRepository manages token refreshing and calendar creation
func setupGoogleRepository(ctx context.Context, u *users.User, userService users.UserRepositoryInterface, logger logger.Interface) (*calendar.GoogleCalendarRepository, error) {
	oldAccessToken := u.GoogleCalendarConnection.Token.AccessToken
	calendarRepository, err := calendar.NewGoogleCalendarRepository(ctx, u, logger)
	if err != nil {
		return nil, err
	}

	if oldAccessToken != u.GoogleCalendarConnection.Token.AccessToken {
		err := userService.Update(ctx, u)
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
		err = userService.Update(ctx, u)
		if err != nil {
			return nil, err
		}
	}

	return calendarRepository, nil
}

// SuggestTimeslot finds a free timeslot
func (c *PlanningController) SuggestTimeslot(window *calendar.TimeWindow) (*[]calendar.Timespan, error) {
	err := c.calendarRepository.AddBusyToWindow(window)
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

// ScheduleTask takes a task and schedules it according to workloadOverall by creating or removing WorkUnits
// and pushes or removes events to and from the calendar
func (c *PlanningController) ScheduleTask(t *Task) error {
	now := time.Now().Add(time.Minute * 15).Round(time.Minute * 15)
	windowTotal := calendar.TimeWindow{Start: now.UTC(), End: t.DueAt.Date.Start.UTC()}
	err := c.calendarRepository.AddBusyToWindow(&windowTotal)
	if err != nil {
		return err
	}

	loc, err := time.LoadLocation("")
	if err != nil {
		return err
	}

	constraint := calendar.FreeConstraint{
		AllowedTimeSpans: []calendar.Timespan{{
			Start: time.Date(0, 0, 0, 6, 0, 0, 0, loc),
			End:   time.Date(0, 0, 0, 14, 30, 0, 0, loc),
		}},
	}

	windowTotal.ComputeFree(&constraint)

	workloadToSchedule := t.WorkloadOverall
	for _, unit := range t.WorkUnits {
		workloadToSchedule -= unit.Workload
	}
	t.NotScheduled = 0

	if workloadToSchedule > 0 {
		workUnits := t.WorkUnits

		foundWorkUnits := findWorkUnitTimes(&windowTotal, workloadToSchedule)

		for _, workUnit := range foundWorkUnits {
			workUnit.ScheduledAt.Blocking = true
			workUnit.ScheduledAt.Title = renderWorkUnitEventTitle(t)
			workUnit.ScheduledAt.Description = ""

			workEvent, err := c.calendarRepository.NewEvent(&workUnit.ScheduledAt)
			if err != nil {
				return err
			}

			workUnit.ScheduledAt = *workEvent
			workloadToSchedule -= workUnit.Workload
			workUnits = workUnits.Add(&workUnit)
		}

		if workloadToSchedule > 0 {
			t.NotScheduled += workloadToSchedule
		}

		t.WorkUnits = workUnits
		t.IsDone = false

	} else {
		var workUnits = WorkUnits{}
		for index := len(t.WorkUnits) - 1; index >= 0; index-- {
			if index < 0 {
				return errors.New("workload can't be less than all not done workunits combined")
			}

			unit := t.WorkUnits[index]

			if workloadToSchedule == 0 {
				workUnits = workUnits.Add(&t.WorkUnits[index])
				continue
			}

			if unit.IsDone {
				workUnits = workUnits.Add(&t.WorkUnits[index])
				t.WorkUnits[index].Workload += workloadToSchedule
				continue
			}

			// If we can cut off time of an existing WorkUnit we do that
			if -workloadToSchedule < unit.Workload {
				t.WorkUnits[index].Workload += workloadToSchedule
				t.WorkUnits[index].ScheduledAt.Date.End = unit.ScheduledAt.Date.End.Add(workloadToSchedule)
				err := c.calendarRepository.UpdateEvent(&t.WorkUnits[index].ScheduledAt)
				if err != nil {
					return err
				}

				workUnits = workUnits.Add(&t.WorkUnits[index])
				workloadToSchedule = 0
				continue
			}

			err := c.calendarRepository.DeleteEvent(&unit.ScheduledAt)
			if err != nil {
				return err
			}

			workloadToSchedule += unit.Workload
		}

		t.WorkUnits = workUnits
	}

	if t.DueAt.CalendarEventID == "" {
		t.DueAt.Blocking = false
		t.DueAt.Title = renderDueEventTitle(t)
		t.DueAt.Date.End = t.DueAt.Date.Start.Add(time.Minute * 15)
		t.DueAt.Description = ""

		dueEvent, err := c.calendarRepository.NewEvent(&t.DueAt)
		if err != nil {
			return err
		}

		t.DueAt = *dueEvent
	}

	return nil
}

// RescheduleWorkUnit takes a work unit and reschedules it to a time between now and the task due end
func (c *PlanningController) RescheduleWorkUnit(t *TaskUpdate, w *WorkUnit, index int) error {
	now := time.Now().Add(time.Minute * 15).Round(time.Minute * 15)
	windowTotal := calendar.TimeWindow{Start: now.UTC(), End: t.DueAt.Date.Start.UTC()}

	err := c.calendarRepository.DeleteEvent(&w.ScheduledAt)
	if err != nil {
		return err
	}

	t.WorkUnits = t.WorkUnits.RemoveByIndex(index)

	err = c.calendarRepository.AddBusyToWindow(&windowTotal)
	if err != nil {
		return err
	}

	loc, err := time.LoadLocation("")
	if err != nil {
		return err
	}

	constraint := calendar.FreeConstraint{
		AllowedTimeSpans: []calendar.Timespan{{
			Start: time.Date(0, 0, 0, 6, 0, 0, 0, loc),
			End:   time.Date(0, 0, 0, 14, 30, 0, 0, loc),
		}},
	}

	windowTotal.ComputeFree(&constraint)

	workloadToSchedule := w.Workload

	for _, workUnit := range findWorkUnitTimes(&windowTotal, workloadToSchedule) {
		workUnit.ScheduledAt.Blocking = true
		workUnit.ScheduledAt.Title = renderWorkUnitEventTitle((*Task)(t))
		workUnit.ScheduledAt.Description = ""

		workEvent, err := c.calendarRepository.NewEvent(&workUnit.ScheduledAt)
		if err != nil {
			return err
		}

		workUnit.ScheduledAt = *workEvent
		workloadToSchedule -= workloadToSchedule

		t.WorkUnits = t.WorkUnits.Add(&workUnit)
	}

	if workloadToSchedule > 0 {
		t.NotScheduled += workloadToSchedule
	}

	return nil
}

func findWorkUnitTimes(w *calendar.TimeWindow, durationToFind time.Duration) WorkUnits {
	var workUnits WorkUnits
	if w.FreeDuration == 0 {
		return workUnits
	}

	if w.Duration() < 24*time.Hour*7 {
		minDuration := 2 * time.Hour
		if durationToFind < 2*time.Hour {
			minDuration = durationToFind
		}
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

func renderDueEventTitle(task *Task) string {
	var icon = "📅"

	if task.IsDone {
		icon = "✔️"
	}

	return fmt.Sprintf("%s %s is due", icon, task.Name)
}

func renderWorkUnitEventTitle(task *Task) string {
	return fmt.Sprintf("⚙️ Working on %s", task.Name)
}

// UpdateTaskTitle updates the events of the tasks and work units
func (c *PlanningController) UpdateTaskTitle(task *Task, updateWorkUnits bool) error {
	task.DueAt.Title = renderDueEventTitle(task)
	err := c.calendarRepository.UpdateEvent(&task.DueAt)
	if err != nil {
		return err
	}

	if !updateWorkUnits {
		return nil
	}

	for _, unit := range task.WorkUnits {
		unit.ScheduledAt.Title = renderWorkUnitEventTitle(task)
		err := c.calendarRepository.UpdateEvent(&unit.ScheduledAt)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteTask deletes all events that are connected to a task
func (c *PlanningController) DeleteTask(task *Task) error {
	for _, unit := range task.WorkUnits {
		err := c.calendarRepository.DeleteEvent(&unit.ScheduledAt)
		if err != nil {
			return err
		}
	}

	err := c.calendarRepository.DeleteEvent(&task.DueAt)
	if err != nil {
		return err
	}

	return nil
}

// SyncCalendar triggers a sync on a single calendar
func (c *PlanningController) SyncCalendar(userID string, calendarID string) error {
	eventChannel := make(chan *calendar.Event)
	errorChannel := make(chan error)
	userChannel := make(chan *users.User)
	go c.calendarRepository.SyncEvents(calendarID, &eventChannel, &errorChannel, &userChannel)

	for {
		select {
		case user := <-userChannel:
			err := c.userRepository.Update(c.ctx, user)
			if err != nil {
				return err
			}
			return nil
		case event := <-eventChannel:
			go c.processTaskEventChange(event, userID)
		case err := <-errorChannel:
			return err
		}
	}
}

func (c *PlanningController) processTaskEventChange(event *calendar.Event, userID string) {
	task, err := c.taskRepository.FindByCalendarEventID(c.ctx, event.CalendarEventID, userID)
	if err != nil {
		// TODO: check work unit date intersections with tasks
		return
	}

	if task.DueAt.CalendarEventID == event.CalendarEventID {
		if task.DueAt == *event {
			return
		}

		task.DueAt = *event
		// TODO: do other actions based on due date change
		if event.Deleted {
			err := c.DeleteTask((*Task)(task))
			if err != nil {
				c.logger.Error("problem with deleting task", err)
				return
			}
		}

		err = c.taskRepository.Update(c.ctx, task.ID.Hex(), userID, task)
		if err != nil {
			c.logger.Error("problem with updating task", err)
			return
		}
		return
	}

	index, workunit := task.WorkUnits.FindByCalendarID(event.CalendarEventID)
	if workunit == nil {
		c.logger.Error("there was a event id that could not be found inside a task", nil)
		return
	}

	if workunit.ScheduledAt == *event {
		return
	}

	task.WorkloadOverall -= workunit.Workload

	if event.Deleted {
		task.WorkUnits = task.WorkUnits.RemoveByIndex(index)
		err = c.taskRepository.Update(c.ctx, task.ID.Hex(), userID, task)
		if err != nil {
			c.logger.Error("problem with updating task", err)
			return
		}
		return
	}

	workunit.ScheduledAt = *event
	workunit.Workload = workunit.ScheduledAt.Date.Duration()

	// Remove and re-add to sort it naively for now
	task.WorkUnits = task.WorkUnits.RemoveByIndex(index)
	task.WorkUnits = task.WorkUnits.Add(workunit)

	task.WorkloadOverall += workunit.Workload

	task.WorkUnits[index] = *workunit
	err = c.taskRepository.Update(c.ctx, task.ID.Hex(), userID, task)
	if err != nil {
		c.logger.Error("problem with updating task", err)
		return
	}
}
