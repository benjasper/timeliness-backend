package tasks

import (
	"context"
	"errors"
	"fmt"
	"github.com/timeliness-app/timeliness-backend/pkg/locking"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
)

// now is the current time and is globally available to override it in tests
var now = time.Now

// The PlanningService combines the calendar and task implementations
type PlanningService struct {
	userRepository users.UserRepositoryInterface
	taskRepository TaskRepositoryInterface
	logger         logger.Interface
	constraint     *calendar.FreeConstraint
	locker         locking.LockerInterface
	userCache      UserDataCacheInterface
}

// NewPlanningController constructs a PlanningService that is specific for a user
func NewPlanningController(userService users.UserRepositoryInterface,
	taskRepository TaskRepositoryInterface,
	logger logger.Interface, cache UserDataCacheInterface, locker locking.LockerInterface) *PlanningService {
	controller := PlanningService{}

	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		logger.Fatal(err)
		return nil
	}

	controller.userRepository = userService
	controller.taskRepository = taskRepository
	controller.logger = logger
	controller.userCache = cache
	controller.locker = locker

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

	return &controller
}

// setupGoogleRepository manages token refreshing and calendar creation
func setupGoogleRepository(ctx context.Context, u *users.User, userService users.UserRepositoryInterface, logger logger.Interface) (*calendar.GoogleCalendarRepository, error) {
	oldAccessToken := u.GoogleCalendarConnection.Token.AccessToken
	calendarRepository, err := calendar.NewGoogleCalendarRepository(context.Background(), u, logger)
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

func (c *PlanningService) getRepositoryForUser(ctx context.Context, u *users.User) (calendar.RepositoryInterface, error) {
	// TODO: Figure out which calendarRepository to use
	repository, err := setupGoogleRepository(ctx, u, c.userRepository, c.logger)
	if err != nil {
		return nil, err
	}

	return repository, nil
}

func (c *PlanningService) getAllRelevantUsersWithOwner(ctx context.Context, task *Task, initializeWithOwner *users.User) ([]*users.User, error) {
	relevantUsers := []*users.User{initializeWithOwner}

	mutex := sync.Mutex{}
	wg, ctx := errgroup.WithContext(ctx)

	for _, collaborator := range task.Collaborators {
		wg.Go(func() error {
			var collaboratorUser *users.User
			var err error

			cacheResult, err := c.userCache.Get(ctx, collaborator.UserID.Hex())
			collaboratorUser = cacheResult.User
			if err != nil {
				collaboratorUser, err = c.userRepository.FindByID(ctx, collaborator.UserID.Hex())
				if err != nil {
					return err
				}

				found := false
				for _, contact := range initializeWithOwner.Contacts {
					if contact.UserID == collaboratorUser.ID {
						found = true
						break
					}
				}

				if !found {
					return fmt.Errorf("user %s is not part of %s's contacts",
						collaboratorUser.ID.Hex(), initializeWithOwner.ID.Hex())
				}

				repository, err := c.getRepositoryForUser(ctx, collaboratorUser)
				if err != nil {
					return err
				}

				err = c.userCache.Add(ctx, collaboratorUser.ID.Hex(), &UserDataCacheEntry{
					User:               collaboratorUser,
					CalendarRepository: repository,
				})
				if err != nil {
					return err
				}
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

func (c *PlanningService) getAllRelevantUsers(ctx context.Context, task *Task) ([]*users.User, error) {
	var initializeWithOwner *users.User
	userData, err := c.userCache.Get(ctx, task.UserID.Hex())
	if err != nil {
		initializeWithOwner, err = c.userRepository.FindByID(ctx, task.UserID.Hex())
		if err != nil {
			return nil, err
		}

		repository, err := c.getRepositoryForUser(ctx, initializeWithOwner)
		if err != nil {
			return nil, err
		}

		err = c.userCache.Add(ctx, initializeWithOwner.ID.Hex(), &UserDataCacheEntry{
			User:               initializeWithOwner,
			CalendarRepository: repository,
		})
		if err != nil {
			return nil, err
		}
	} else {
		initializeWithOwner = userData.User
	}

	return c.getAllRelevantUsersWithOwner(ctx, task, initializeWithOwner)
}

// InvalidateUserData should be called in case user data changes that might influence the calendar connection
func (c *PlanningService) InvalidateUserData(ctx context.Context, u *users.User) {
	_ = c.userCache.Invalidate(ctx, u.ID.Hex())
}

// ScheduleTask takes a task and schedules it according to workloadOverall by creating or removing WorkUnits
// and pushes or removes events to and from the calendar
func (c *PlanningService) ScheduleTask(ctx context.Context, t *Task) (*Task, error) {
	if !t.ID.IsZero() {
		lock, _ := c.locker.Acquire(ctx, t.ID.Hex(), time.Second*30)

		defer func(lock locking.LockInterface, ctx context.Context) {
			err := lock.Release(ctx)
			if err != nil {
				c.logger.Error("problem releasing lock", err)
			}
		}(lock, ctx)
	}

	nowRound := now().Add(time.Minute * 15).Round(time.Minute * 15)
	windowTotal := calendar.TimeWindow{Start: nowRound.UTC(), End: t.DueAt.Date.Start.UTC(), BusyPadding: 15 * time.Minute}

	relevantUsers, err := c.getAllRelevantUsers(ctx, t)
	if err != nil {
		return nil, err
	}

	// TODO make TimeWindow thread safe and make this parallel
	for _, user := range relevantUsers {
		userData, err := c.userCache.Get(ctx, user.ID.Hex())
		if err != nil {
			return nil, err
		}

		err = userData.CalendarRepository.AddBusyToWindow(&windowTotal)
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
				userData, err := c.userCache.Get(ctx, user.ID.Hex())
				if err != nil {
					return nil, err
				}

				workEvent, err = userData.CalendarRepository.NewEvent(&workUnit.ScheduledAt)
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

		err := c.taskRepository.Update(ctx, (*TaskUpdate)(t), false)
		if err != nil {
			return nil, err
		}

		for _, user := range relevantUsers {
			userData, err := c.userCache.Get(ctx, user.ID.Hex())
			if err != nil {
				return nil, err
			}

			for _, unit := range shouldDelete {
				err = userData.CalendarRepository.DeleteEvent(&unit.ScheduledAt)
				if err != nil {
					return nil, err
				}
			}
		}

		for _, user := range relevantUsers {
			userData, err := c.userCache.Get(ctx, user.ID.Hex())
			if err != nil {
				return nil, err
			}

			for _, unit := range shouldUpdate {
				err = userData.CalendarRepository.UpdateEvent(&unit.ScheduledAt)
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
			userData, err := c.userCache.Get(ctx, user.ID.Hex())
			if err != nil {
				return nil, err
			}

			dueEvent, err = userData.CalendarRepository.NewEvent(&t.DueAt)
			if err != nil {
				return nil, err
			}
		}

		t.DueAt = *dueEvent
	}

	if !t.ID.IsZero() {
		err := c.taskRepository.Update(ctx, (*TaskUpdate)(t), false)
		if err != nil {
			return nil, err
		}
	}

	return t, nil
}

// RescheduleWorkUnit takes a work unit and reschedules it to a time between now and the task due end, updates task
func (c *PlanningService) RescheduleWorkUnit(ctx context.Context, t *TaskUpdate, w *WorkUnit) (*TaskUpdate, error) {
	lock, _ := c.locker.Acquire(ctx, t.ID.Hex(), time.Second*30)

	defer func(lock locking.LockInterface, ctx context.Context) {
		err := lock.Release(ctx)
		if err != nil {
			c.logger.Error("problem releasing lock", err)
		}
	}(lock, ctx)

	// Refresh task, after potential change
	t, err := c.taskRepository.FindUpdatableByID(ctx, t.ID.Hex(), t.UserID.Hex(), false)
	if err != nil {
		return nil, err
	}

	nowRound := now().Add(time.Minute * 15).Round(time.Minute * 15)
	windowTotal := calendar.TimeWindow{Start: nowRound.UTC(), End: t.DueAt.Date.Start.UTC(), BusyPadding: 15 * time.Minute}

	index, _ := t.WorkUnits.FindByID(w.ID.Hex())
	if index < 0 {
		return nil, fmt.Errorf("could not find workunit %s in task %s", w.ID.Hex(), t.ID.Hex())
	}

	t.WorkUnits = t.WorkUnits.RemoveByIndex(index)
	err = c.taskRepository.Update(ctx, t, false)
	if err != nil {
		return nil, err
	}

	relevantUsers, err := c.getAllRelevantUsers(ctx, (*Task)(t))
	if err != nil {
		return nil, err
	}

	// TODO Make parallel
	for _, user := range relevantUsers {
		userData, err := c.userCache.Get(ctx, user.ID.Hex())
		if err != nil {
			return nil, err
		}

		err = userData.CalendarRepository.DeleteEvent(&w.ScheduledAt)
		if err != nil {
			return nil, err
		}
	}

	// TODO Make parallel
	for _, user := range relevantUsers {
		userData, err := c.userCache.Get(ctx, user.ID.Hex())
		if err != nil {
			return nil, err
		}

		err = userData.CalendarRepository.AddBusyToWindow(&windowTotal)
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
			userData, err := c.userCache.Get(ctx, user.ID.Hex())
			if err != nil {
				return nil, err
			}

			workEvent, err = userData.CalendarRepository.NewEvent(&workUnit.ScheduledAt)
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

	err = c.taskRepository.Update(ctx, t, false)
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
func (c *PlanningService) UpdateEvent(ctx context.Context, task *Task, event *calendar.Event) error {
	relevantUsers, err := c.getAllRelevantUsers(ctx, task)
	if err != nil {
		return err
	}

	for _, user := range relevantUsers {
		userData, err := c.userCache.Get(ctx, user.ID.Hex())
		if err != nil {
			return err
		}

		err = userData.CalendarRepository.UpdateEvent(event)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateTaskTitle updates the events of the tasks and work units
func (c *PlanningService) UpdateTaskTitle(ctx context.Context, task *Task, updateWorkUnits bool) error {
	task.DueAt.Title = renderDueEventTitle(task)

	relevantUsers, err := c.getAllRelevantUsers(ctx, task)
	if err != nil {
		return err
	}

	for _, user := range relevantUsers {
		userData, err := c.userCache.Get(ctx, user.ID.Hex())
		if err != nil {
			return err
		}

		err = userData.CalendarRepository.UpdateEvent(&task.DueAt)
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
			userData, err := c.userCache.Get(ctx, user.ID.Hex())
			if err != nil {
				return err
			}

			err = userData.CalendarRepository.UpdateEvent(&unit.ScheduledAt)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DeleteTask deletes all events that are connected to a task
func (c *PlanningService) DeleteTask(ctx context.Context, task *Task) error {
	err := c.taskRepository.Delete(ctx, task.ID.Hex(), task.UserID.Hex())
	if err != nil {
		return err
	}

	relevantUsers, err := c.getAllRelevantUsers(ctx, task)
	if err != nil {
		return err
	}

	// TODO make these parallel
	for _, user := range relevantUsers {
		userData, err := c.userCache.Get(ctx, user.ID.Hex())
		if err != nil {
			return err
		}

		for _, unit := range task.WorkUnits {

			err := userData.CalendarRepository.DeleteEvent(&unit.ScheduledAt)
			if err != nil {
				return err
			}
		}
	}

	for _, user := range relevantUsers {
		userData, err := c.userCache.Get(ctx, user.ID.Hex())
		if err != nil {
			return err
		}

		err = userData.CalendarRepository.DeleteEvent(&task.DueAt)
		if err != nil {
			return err
		}
	}

	return nil
}

// SyncCalendar triggers a sync on a single calendar
func (c *PlanningService) SyncCalendar(ctx context.Context, user *users.User, calendarID string) (*users.User, error) {
	eventChannel := make(chan *calendar.Event)
	errorChannel := make(chan error)
	userChannel := make(chan *users.User)

	calendarRepository, err := c.getRepositoryForUser(ctx, user)
	if err != nil {
		return nil, err
	}

	go calendarRepository.SyncEvents(calendarID, user, &eventChannel, &errorChannel, &userChannel)

	for {
		select {
		case user := <-userChannel:
			return user, nil
		case event := <-eventChannel:
			go c.processTaskEventChange(ctx, event, user.ID.Hex())
		case err := <-errorChannel:
			return nil, err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *PlanningService) processTaskEventChange(ctx context.Context, event *calendar.Event, userID string) {
	calendarEvent := event.CalendarEvents.FindByUserID(userID)
	task, err := c.taskRepository.FindByCalendarEventID(ctx, calendarEvent.CalendarEventID, userID, false)
	if err != nil {
		if event.Deleted || event.IsOriginal {
			return
		}
		_ = c.checkForIntersectingWorkUnits(ctx, userID, event, "")

		return
	}

	lock, _ := c.locker.Acquire(ctx, task.ID.Hex(), time.Second*10)

	defer func(lock locking.LockInterface, ctx context.Context) {
		err := lock.Release(ctx)
		if err != nil {
			c.logger.Error("problem releasing lock", err)
		}
	}(lock, ctx)

	// Refresh task, after potential change
	task, err = c.taskRepository.FindUpdatableByID(ctx, task.ID.Hex(), userID, false)
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
			err := c.DeleteTask(ctx, (*Task)(task))
			if err != nil {
				c.logger.Error("problem with deleting task", err)
				return
			}
		}

		err = c.taskRepository.Update(ctx, task, false)
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
		err = c.taskRepository.Update(ctx, task, false)
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

	err = c.taskRepository.Update(ctx, task, false)
	if err != nil {
		c.logger.Error("problem with updating task", err)
		return
	}
}

func (c *PlanningService) checkForIntersectingWorkUnits(ctx context.Context, userID string, event *calendar.Event, workUnitID string) int {
	intersectingTasks, err := c.taskRepository.FindIntersectingWithEvent(ctx, userID, event, workUnitID, false)
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
			updatedTask, err := c.RescheduleWorkUnit(ctx, (*TaskUpdate)(&intersection.Task), &unit)
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
