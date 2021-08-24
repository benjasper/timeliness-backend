package tasks

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/golang-lru"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
)

// The PlanningController combines the calendar and task implementations
type PlanningController struct {
	calendarRepositories map[string]calendar.RepositoryInterface
	userRepository       users.UserRepositoryInterface
	taskRepository       TaskRepositoryInterface
	ctx                  context.Context
	logger               logger.Interface
	constraint           *calendar.FreeConstraint
	taskMutexMap         sync.Map
	owner                *users.User
	userCache            *lru.Cache
}

// NewPlanningController constructs a PlanningController that is specific for a user
func NewPlanningController(ctx context.Context, owner *users.User, userService users.UserRepositoryInterface, taskRepository TaskRepositoryInterface,
	logger logger.Interface) (*PlanningController, error) {
	controller := PlanningController{}

	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return nil, err
	}

	controller.ctx = ctx
	controller.userRepository = userService
	controller.taskRepository = taskRepository
	controller.logger = logger
	controller.taskMutexMap = sync.Map{}
	controller.taskMutexMap = sync.Map{}
	controller.owner = owner

	controller.userCache, err = lru.New(5)
	if err != nil {
		return nil, err
	}

	// TODO merge these? or only take owners constraints?; Also move this into its own function, so we can called it when needed
	controller.constraint = &calendar.FreeConstraint{
		Location: location,
		AllowedTimeSpans: []calendar.Timespan{
			{
				Start: time.Date(0, 0, 0, 9, 0, 0, 0, location),
				End:   time.Date(0, 0, 0, 12, 0, 0, 0, location),
			},
			{
				Start: time.Date(0, 0, 0, 13, 0, 0, 0, location),
				End:   time.Date(0, 0, 0, 18, 00, 0, 0, location),
			},
		},
	}

	// Initialize repository for owner
	_, err = controller.getRepositoryForUser(owner)
	if err != nil {
		return nil, err
	}

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

func (c *PlanningController) getRepositoryForUser(u *users.User) (calendar.RepositoryInterface, error) {
	if repo, ok := c.calendarRepositories[u.ID.Hex()]; ok {
		return repo, nil
	} else {
		// TODO: Figure out which calendarRepository to use
		repository, err := setupGoogleRepository(c.ctx, u, c.userRepository, c.logger)
		if err != nil {
			return nil, err
		}

		c.calendarRepositories[u.ID.Hex()] = repository
		return repository, nil
	}
}

func (c *PlanningController) getAllRelevantUsers(task *Task) ([]*users.User, error) {
	relevantUsers := []*users.User{c.owner}
	for _, collaborator := range task.Collaborators {

		mutex := sync.Mutex{}
		wg, ctx := errgroup.WithContext(c.ctx)

		wg.Go(func() error {
			// TODO Check if it actually is a contact
			var collaboratorUser *users.User
			var err error

			cacheResult, ok := c.userCache.Get(collaborator.UserID.Hex())
			collaboratorUser = cacheResult.(*users.User)
			if !ok {
				collaboratorUser, err = c.userRepository.FindByID(ctx, collaborator.UserID.Hex())
				if err != nil {
					return err
				}
				c.userCache.Add(collaboratorUser.ID.Hex(), collaboratorUser)
			}

			mutex.Lock()
			relevantUsers = append(relevantUsers, collaboratorUser)
			mutex.Unlock()

			return err
		})

		err := wg.Wait()
		if err != nil {
			return relevantUsers, err
		}
	}

	return relevantUsers, nil
}

// ScheduleTask takes a task and schedules it according to workloadOverall by creating or removing WorkUnits
// and pushes or removes events to and from the calendar
func (c *PlanningController) ScheduleTask(t *Task) (*Task, error) {
	if !t.ID.IsZero() {
		loaded, _ := c.taskMutexMap.LoadOrStore(t.ID.Hex(), &sync.Mutex{})
		mutex := loaded.(*sync.Mutex)

		mutex.Lock()
		defer mutex.Unlock()
	}

	now := time.Now().Add(time.Minute * 15).Round(time.Minute * 15)
	windowTotal := calendar.TimeWindow{Start: now.UTC(), End: t.DueAt.Date.Start.UTC(), BusyPadding: 15 * time.Minute}

	// TODO make TimeWindow threadsafe and make this parallel
	relevantUsers, err := c.getAllRelevantUsers(t)
	if err != nil {
		return nil, err
	}

	for _, user := range relevantUsers {
		err := c.calendarRepositories[user.ID.Hex()].AddBusyToWindow(&windowTotal)
		if err != nil {
			return nil, err
		}
	}

	windowTotal.ComputeFree(c.constraint)

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

			var workEvent *calendar.Event
			for _, user := range relevantUsers {
				var err error
				workEvent, err = c.calendarRepositories[user.ID.Hex()].NewEvent(&workUnit.ScheduledAt)
				if err != nil {
					return nil, err
				}
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
		var shouldDelete = WorkUnits{}
		var shouldUpdate = WorkUnits{}
		var workUnits = WorkUnits{}
		for index := len(t.WorkUnits) - 1; index >= 0; index-- {
			if index < 0 {
				return nil, errors.New("workload can't be less than all not done workunits combined")
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

				shouldUpdate = append(shouldUpdate, t.WorkUnits[index])

				workUnits = workUnits.Add(&t.WorkUnits[index])
				workloadToSchedule = 0
				continue
			}

			shouldDelete = append(shouldDelete, unit)

			workloadToSchedule += unit.Workload
		}

		t.WorkUnits = workUnits

		err := c.taskRepository.Update(c.ctx, (*TaskUpdate)(t), false)
		if err != nil {
			return nil, err
		}

		for _, unit := range shouldDelete {
			for _, user := range relevantUsers {
				err = c.calendarRepositories[user.ID.Hex()].DeleteEvent(&unit.ScheduledAt)
				if err != nil {
					return nil, err
				}
			}
		}

		for _, unit := range shouldUpdate {
			for _, user := range relevantUsers {
				err = c.calendarRepositories[user.ID.Hex()].UpdateEvent(&unit.ScheduledAt)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if len(t.DueAt.CalendarEvents) == 0 {
		t.DueAt.Blocking = false
		t.DueAt.Title = renderDueEventTitle(t)
		t.DueAt.Date.End = t.DueAt.Date.Start.Add(time.Minute * 15)
		t.DueAt.Description = ""

		var dueEvent *calendar.Event
		for _, user := range relevantUsers {
			var err error
			dueEvent, err = c.calendarRepositories[user.ID.Hex()].NewEvent(&t.DueAt)
			if err != nil {
				return nil, err
			}
		}

		t.DueAt = *dueEvent
	}

	if !t.ID.IsZero() {
		err := c.taskRepository.Update(c.ctx, (*TaskUpdate)(t), false)
		if err != nil {
			return nil, err
		}
	}

	return t, nil
}

// RescheduleWorkUnit takes a work unit and reschedules it to a time between now and the task due end, updates task
func (c *PlanningController) RescheduleWorkUnit(t *TaskUpdate, w *WorkUnit) (*TaskUpdate, error) {
	loaded, _ := c.taskMutexMap.LoadOrStore(t.ID.Hex(), &sync.Mutex{})
	mutex := loaded.(*sync.Mutex)

	mutex.Lock()
	defer mutex.Unlock()

	// Refresh task, after potential change
	t, err := c.taskRepository.FindUpdatableByID(c.ctx, t.ID.Hex(), t.UserID.Hex(), false)
	if err != nil {
		return nil, err
	}

	now := time.Now().Add(time.Minute * 15).Round(time.Minute * 15)
	windowTotal := calendar.TimeWindow{Start: now.UTC(), End: t.DueAt.Date.Start.UTC(), BusyPadding: 15 * time.Minute}

	index, _ := t.WorkUnits.FindByID(w.ID.Hex())
	if index < 0 {
		return nil, fmt.Errorf("could not find workunit %s in task %s", w.ID.Hex(), t.ID.Hex())
	}

	t.WorkUnits = t.WorkUnits.RemoveByIndex(index)
	err = c.taskRepository.Update(c.ctx, t, false)
	if err != nil {
		return nil, err
	}

	relevantUsers, err := c.getAllRelevantUsers((*Task)(t))
	if err != nil {
		return nil, err
	}

	// TODO Make parallel
	for _, user := range relevantUsers {
		err = c.calendarRepositories[user.ID.Hex()].DeleteEvent(&w.ScheduledAt)
		if err != nil {
			return nil, err
		}
	}

	// TODO Make parallel
	for _, user := range relevantUsers {
		err = c.calendarRepositories[user.ID.Hex()].AddBusyToWindow(&windowTotal)
		if err != nil {
			return nil, err
		}
	}

	windowTotal.ComputeFree(c.constraint)

	workloadToSchedule := w.Workload

	for _, workUnit := range findWorkUnitTimes(&windowTotal, workloadToSchedule) {
		workUnit.ScheduledAt.Blocking = true
		workUnit.ScheduledAt.Title = renderWorkUnitEventTitle((*Task)(t))
		workUnit.ScheduledAt.Description = ""

		var workEvent *calendar.Event
		for _, user := range relevantUsers {
			workEvent, err = c.calendarRepositories[user.ID.Hex()].NewEvent(&workUnit.ScheduledAt)
			if err != nil {
				return nil, err
			}
		}

		workUnit.ScheduledAt = *workEvent
		workloadToSchedule -= workloadToSchedule

		t.WorkUnits = t.WorkUnits.Add(&workUnit)
	}

	if workloadToSchedule > 0 {
		t.NotScheduled += workloadToSchedule
	}

	err = c.taskRepository.Update(c.ctx, t, false)
	if err != nil {
		return nil, err
	}

	return t, nil
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

// UpdateEvent updates an any calendar event
func (c *PlanningController) UpdateEvent(task *Task, event *calendar.Event) error {
	relevantUsers, err := c.getAllRelevantUsers(task)
	if err != nil {
		return err
	}

	for _, user := range relevantUsers {
		err := c.calendarRepositories[user.ID.Hex()].UpdateEvent(event)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateTaskTitle updates the events of the tasks and work units
func (c *PlanningController) UpdateTaskTitle(task *Task, updateWorkUnits bool) error {
	task.DueAt.Title = renderDueEventTitle(task)

	relevantUsers, err := c.getAllRelevantUsers(task)
	if err != nil {
		return err
	}

	for _, user := range relevantUsers {
		err := c.calendarRepositories[user.ID.Hex()].UpdateEvent(&task.DueAt)
		if err != nil {
			return err
		}
	}

	if !updateWorkUnits {
		return nil
	}

	for _, unit := range task.WorkUnits {
		unit.ScheduledAt.Title = renderWorkUnitEventTitle(task)

		for _, user := range relevantUsers {
			err := c.calendarRepositories[user.ID.Hex()].UpdateEvent(&unit.ScheduledAt)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DeleteTask deletes all events that are connected to a task
func (c *PlanningController) DeleteTask(task *Task) error {
	err := c.taskRepository.Delete(c.ctx, task.ID.Hex(), task.UserID.Hex())
	if err != nil {
		return err
	}

	relevantUsers, err := c.getAllRelevantUsers(task)
	if err != nil {
		return err
	}

	// TODO make these parallel
	for _, unit := range task.WorkUnits {
		for _, user := range relevantUsers {
			err := c.calendarRepositories[user.ID.Hex()].DeleteEvent(&unit.ScheduledAt)
			if err != nil {
				return err
			}
		}
	}

	for _, user := range relevantUsers {
		err = c.calendarRepositories[user.ID.Hex()].DeleteEvent(&task.DueAt)
		if err != nil {
			return err
		}
	}

	return nil
}

// SyncCalendar triggers a sync on a single calendar
func (c *PlanningController) SyncCalendar(user *users.User, calendarID string) (*users.User, error) {
	eventChannel := make(chan *calendar.Event)
	errorChannel := make(chan error)
	userChannel := make(chan *users.User)
	go c.calendarRepositories[user.ID.Hex()].SyncEvents(calendarID, user, &eventChannel, &errorChannel, &userChannel)

	for {
		select {
		case user := <-userChannel:
			return user, nil
		case event := <-eventChannel:
			go c.processTaskEventChange(event, user.ID.Hex())
		case err := <-errorChannel:
			return nil, err
		}
	}
}

func (c *PlanningController) processTaskEventChange(event *calendar.Event, userID string) {
	calendarEvent := event.CalendarEvents.FindByUserID(userID)
	task, err := c.taskRepository.FindByCalendarEventID(c.ctx, calendarEvent.CalendarEventID, userID, false)
	if err != nil {
		if event.Deleted || event.IsOriginal {
			return
		}
		_ = c.checkForIntersectingWorkUnits(userID, event, "")

		return
	}

	loaded, _ := c.taskMutexMap.LoadOrStore(task.ID.Hex(), &sync.Mutex{})
	mutex := loaded.(*sync.Mutex)

	mutex.Lock()
	defer mutex.Unlock()

	// Refresh task, after potential change
	task, err = c.taskRepository.FindUpdatableByID(c.ctx, task.ID.Hex(), userID, false)
	if err != nil {
		c.logger.Error("could not refresh already loaded task", err)
		return
	}

	dueAtCalendarEvent := task.DueAt.CalendarEvents.FindByUserID(userID)
	if dueAtCalendarEvent != nil && dueAtCalendarEvent.CalendarEventID == calendarEvent.CalendarEventID {
		if task.DueAt.Date == event.Date {
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

		err = c.taskRepository.Update(c.ctx, task, false)
		if err != nil {
			c.logger.Error("problem with updating task", err)
			return
		}
		return
	}

	index, workunit := task.WorkUnits.FindByCalendarID(calendarEvent.CalendarEventID)
	if workunit == nil {
		c.logger.Error("there was an event id that could not be found inside a task", nil)
		return
	}

	if workunit.ScheduledAt.Date.Start == event.Date.Start && workunit.ScheduledAt.Date.End == event.Date.End {
		return
	}

	task.WorkloadOverall -= workunit.Workload

	if event.Deleted {
		task.WorkUnits = task.WorkUnits.RemoveByIndex(index)
		err = c.taskRepository.Update(c.ctx, task, false)
		if err != nil {
			c.logger.Error("problem with updating task", err)
			return
		}
		return
	}

	workunit.ScheduledAt = *event
	workunit.Workload = workunit.ScheduledAt.Date.Duration()

	task.WorkloadOverall += workunit.Workload

	task.WorkUnits[index] = *workunit

	task.WorkUnits = task.WorkUnits.RemoveByIndex(index)
	task.WorkUnits = task.WorkUnits.Add(workunit)

	err = c.taskRepository.Update(c.ctx, task, false)
	if err != nil {
		c.logger.Error("problem with updating task", err)
		return
	}
}

func (c *PlanningController) checkForIntersectingWorkUnits(userID string, event *calendar.Event, workUnitID string) int {
	intersectingTasks, err := c.taskRepository.FindIntersectingWithEvent(c.ctx, userID, event, workUnitID, false)
	if err != nil {
		c.logger.Error("problem while trying to find tasks intersecting with an event", err)
		return 0
	}

	if len(intersectingTasks) == 0 {
		return 0
	}

	type Intersection struct {
		IntersectingWorkUnits       WorkUnits
		IntersectingWorkUnitIndices []int
		Task                        Task
	}

	var intersections []Intersection

	for _, intersectingTask := range intersectingTasks {
		_, workUnits := intersectingTask.WorkUnits.FindByEventIntersection(event)
		if len(workUnits) == 0 {
			continue
		}

		intersection := Intersection{
			IntersectingWorkUnits: workUnits,
			Task:                  intersectingTask,
		}

		intersections = append(intersections, intersection)
	}

	for _, intersection := range intersections {
		for i, unit := range intersection.IntersectingWorkUnits {
			updatedTask, err := c.RescheduleWorkUnit((*TaskUpdate)(&intersection.Task), &unit)
			if err != nil {
				c.logger.Error(fmt.Sprintf(
					"Could not reschedule work unit %d for task %s",
					intersection.IntersectingWorkUnitIndices[i], intersection.Task.ID.Hex()), err)
				continue
			}

			intersection.Task = Task(*updatedTask)
		}
	}

	return len(intersectingTasks)
}
