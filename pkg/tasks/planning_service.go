package tasks

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/timeliness-app/timeliness-backend/pkg/date"
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
	userRepository            users.UserRepositoryInterface
	taskRepository            TaskRepositoryInterface
	logger                    logger.Interface
	locker                    locking.LockerInterface
	calendarRepositoryManager *CalendarRepositoryManager
	taskTextRenderer          *TaskTextRenderer
}

// NewPlanningController constructs a PlanningService that is specific for a user
func NewPlanningController(userService users.UserRepositoryInterface,
	taskRepository TaskRepositoryInterface,
	logger logger.Interface, locker locking.LockerInterface,
	calendarRepositoryManager *CalendarRepositoryManager) *PlanningService {
	controller := PlanningService{}

	controller.userRepository = userService
	controller.taskRepository = taskRepository
	controller.logger = logger
	controller.locker = locker
	controller.calendarRepositoryManager = calendarRepositoryManager
	controller.taskTextRenderer = &TaskTextRenderer{}

	return &controller
}

func (s *PlanningService) getAllRelevantUsersWithOwner(ctx context.Context, task *Task, initializeWithOwner *users.User) ([]*users.User, error) {
	relevantUsers := []*users.User{initializeWithOwner}

	mutex := sync.Mutex{}
	wg, ctx := errgroup.WithContext(ctx)

	for _, collaborator := range task.Collaborators {
		wg.Go(func() error {
			var collaboratorUser *users.User
			var err error

			collaboratorUser, err = s.userRepository.FindByID(ctx, collaborator.UserID.Hex())
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

func (s *PlanningService) getAllRelevantUsers(ctx context.Context, task *Task) ([]*users.User, error) {
	var initializeWithOwner *users.User

	initializeWithOwner, err := s.userRepository.FindByID(ctx, task.UserID.Hex())
	if err != nil {
		return nil, err
	}

	return s.getAllRelevantUsersWithOwner(ctx, task, initializeWithOwner)
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

// ScheduleTask takes a task and schedules it according to workloadOverall by creating or removing WorkUnits
// and pushes or removes events to and from the calendar
func (s *PlanningService) ScheduleTask(ctx context.Context, t *Task) (*Task, error) {
	if !t.ID.IsZero() {
		lock, err := s.locker.Acquire(ctx, t.ID.Hex(), time.Second*30)
		if err != nil {
			return nil, err
		}

		defer func(lock locking.LockInterface, ctx context.Context) {
			err := lock.Release(ctx)
			if err != nil {
				s.logger.Error("problem releasing lock", err)
			}
		}(lock, ctx)
	}

	relevantUsers, err := s.getAllRelevantUsers(ctx, t)
	if err != nil {
		return nil, err
	}

	var spacing time.Duration
	for _, user := range relevantUsers {
		spacing = maxDuration(user.Settings.Scheduling.BusyTimeSpacing, spacing)
	}

	nowRound := now().Add(time.Minute * 15).Round(time.Minute * 15)
	windowTotal := date.TimeWindow{Start: nowRound.UTC(), End: t.DueAt.Date.Start.UTC(), BusyPadding: spacing}

	taskCalendarRepositories := make(map[string]calendar.RepositoryInterface)

	// TODO make TimeWindow thread safe and make this parallel
	for _, user := range relevantUsers {
		taskRepository, err := s.calendarRepositoryManager.GetTaskCalendarRepositoryForUser(ctx, user)
		if err != nil {
			return nil, err
		}

		taskCalendarRepositories[user.ID.Hex()] = taskRepository

		// Repositories for availability
		repositories, err := s.calendarRepositoryManager.GetAllCalendarRepositoriesForUser(ctx, user)
		if err != nil {
			return nil, err
		}

		for _, repository := range repositories {
			err = repository.AddBusyToWindow(&windowTotal)
			if err != nil {
				return nil, err
			}
		}
	}

	// TODO: This is random(first user) for now, this has to be changed when multi user support is implemented
	location, err := time.LoadLocation(relevantUsers[0].Settings.Scheduling.TimeZone)
	if err != nil {
		return nil, err
	}

	// TODO merge these? or only take owners constraints?; Also move this into its own function, so we can called it when needed
	constraint := &date.FreeConstraint{
		Location:         location,
		AllowedTimeSpans: relevantUsers[0].Settings.Scheduling.AllowedTimespans,
	}

	windowTotal.ComputeFree(constraint)

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
			workUnit.ScheduledAt.Title = s.taskTextRenderer.RenderWorkUnitEventTitle(t)
			workUnit.ScheduledAt.Description = ""

			var workEvent *calendar.Event
			for _, user := range relevantUsers {
				workEvent, err = taskCalendarRepositories[user.ID.Hex()].NewEvent(&workUnit.ScheduledAt)
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

		err := s.taskRepository.Update(ctx, (*TaskUpdate)(t), false)
		if err != nil {
			return nil, err
		}

		for _, user := range relevantUsers {
			for _, unit := range shouldDelete {
				err = taskCalendarRepositories[user.ID.Hex()].DeleteEvent(&unit.ScheduledAt)
				if err != nil {
					return nil, err
				}
			}
		}

		for _, user := range relevantUsers {
			for _, unit := range shouldUpdate {
				err = taskCalendarRepositories[user.ID.Hex()].UpdateEvent(&unit.ScheduledAt)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if len(t.DueAt.CalendarEvents) != len(relevantUsers) {
		t.DueAt.Blocking = false
		t.DueAt.Title = s.taskTextRenderer.RenderDueEventTitle(t)
		t.DueAt.Date.End = t.DueAt.Date.Start.Add(time.Minute * 15)
		t.DueAt.Description = ""

		var dueEvent *calendar.Event
		for _, user := range relevantUsers {
			if persistedEvent := t.DueAt.CalendarEvents.FindByUserID(user.ID.Hex()); persistedEvent != nil {
				continue
			}
			dueEvent, err = taskCalendarRepositories[user.ID.Hex()].NewEvent(&t.DueAt)
			if err != nil {
				return nil, err
			}
		}

		t.DueAt = *dueEvent
	}

	if !t.ID.IsZero() {
		err := s.taskRepository.Update(ctx, (*TaskUpdate)(t), false)
		if err != nil {
			return nil, err
		}
	}

	return t, nil
}

// RescheduleWorkUnit takes a work unit and reschedules it to a time between now and the task due end, updates task
func (s *PlanningService) RescheduleWorkUnit(ctx context.Context, t *TaskUpdate, w *WorkUnit) (*TaskUpdate, error) {
	lock, err := s.locker.Acquire(ctx, t.ID.Hex(), time.Second*30)
	if err != nil {
		return nil, err
	}

	defer func(lock locking.LockInterface, ctx context.Context) {
		err := lock.Release(ctx)
		if err != nil {
			s.logger.Error("problem releasing lock", err)
		}
	}(lock, ctx)

	return s.rescheduleWorkUnitWithoutLock(ctx, t, w)
}

// rescheduleWorkUnitWithoutLock takes a work unit and reschedules it to a time between now and the task due end, updates task
func (s *PlanningService) rescheduleWorkUnitWithoutLock(ctx context.Context, t *TaskUpdate, w *WorkUnit) (*TaskUpdate, error) {
	// Refresh task, after potential change
	t, err := s.taskRepository.FindUpdatableByID(ctx, t.ID.Hex(), t.UserID.Hex(), false)
	if err != nil {
		return nil, err
	}

	index, _ := t.WorkUnits.FindByID(w.ID.Hex())
	if index < 0 {
		return nil, fmt.Errorf("could not find workunit %s in task %s", w.ID.Hex(), t.ID.Hex())
	}

	t.WorkUnits = t.WorkUnits.RemoveByIndex(index)

	relevantUsers, err := s.getAllRelevantUsers(ctx, (*Task)(t))
	if err != nil {
		return nil, err
	}

	var spacing time.Duration
	for _, user := range relevantUsers {
		spacing = maxDuration(user.Settings.Scheduling.BusyTimeSpacing, spacing)
	}

	nowRound := now().Add(time.Minute * 15).Round(time.Minute * 15)
	windowTotal := date.TimeWindow{Start: nowRound.UTC(), End: t.DueAt.Date.Start.UTC(), BusyPadding: spacing}

	repositories := make(map[string]calendar.RepositoryInterface)

	// TODO Make parallel
	for _, user := range relevantUsers {
		taskRepository, err := s.calendarRepositoryManager.GetTaskCalendarRepositoryForUser(ctx, user)
		if err != nil {
			return nil, err
		}

		repositories[user.ID.Hex()] = taskRepository

		err = taskRepository.DeleteEvent(&w.ScheduledAt)
		if err != nil {
			return nil, err
		}

		// Repositories for availability
		repositories, err := s.calendarRepositoryManager.GetAllCalendarRepositoriesForUser(ctx, user)
		if err != nil {
			return nil, err
		}

		for _, repository := range repositories {
			err = repository.AddBusyToWindow(&windowTotal)
			if err != nil {
				return nil, err
			}
		}
	}

	// TODO: This is random(first user) for now, this has to be changed when multi user support is implemented
	location, err := time.LoadLocation(relevantUsers[0].Settings.Scheduling.TimeZone)
	if err != nil {
		return nil, err
	}

	// TODO merge these? or only take owners constraints?; Also move this into its own function, so we can called it when needed
	constraint := &date.FreeConstraint{
		Location:         location,
		AllowedTimeSpans: relevantUsers[0].Settings.Scheduling.AllowedTimespans,
	}

	windowTotal.ComputeFree(constraint)

	workloadToSchedule := w.Workload

	for _, workUnit := range findWorkUnitTimes(&windowTotal, workloadToSchedule) {
		workUnit.ScheduledAt.Blocking = true
		workUnit.ScheduledAt.Title = s.taskTextRenderer.RenderWorkUnitEventTitle((*Task)(t))
		workUnit.ScheduledAt.Description = ""

		var workEvent *calendar.Event
		for _, user := range relevantUsers {
			workEvent, err = repositories[user.ID.Hex()].NewEvent(&workUnit.ScheduledAt)
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

	err = s.taskRepository.Update(ctx, t, false)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func findWorkUnitTimes(w *date.TimeWindow, durationToFind time.Duration) WorkUnits {
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

			var rules = []date.RuleInterface{&date.RuleDuration{Minimum: minDuration, Maximum: maxDuration}}
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

// UpdateEvent updates an any calendar event
func (s *PlanningService) UpdateEvent(ctx context.Context, task *Task, event *calendar.Event) error {
	relevantUsers, err := s.getAllRelevantUsers(ctx, task)
	if err != nil {
		return err
	}

	for _, user := range relevantUsers {
		repository, err := s.calendarRepositoryManager.GetTaskCalendarRepositoryForUser(ctx, user)
		if err != nil {
			return err
		}

		err = repository.UpdateEvent(event)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateTaskTitle updates the events of the tasks and work units
func (s *PlanningService) UpdateTaskTitle(ctx context.Context, task *Task, updateWorkUnits bool) error {
	task.DueAt.Title = s.taskTextRenderer.RenderDueEventTitle(task)

	relevantUsers, err := s.getAllRelevantUsers(ctx, task)
	if err != nil {
		return err
	}

	repositories := make(map[string]calendar.RepositoryInterface)

	for _, user := range relevantUsers {
		repository, err := s.calendarRepositoryManager.GetTaskCalendarRepositoryForUser(ctx, user)
		if err != nil {
			return err
		}

		repositories[user.ID.Hex()] = repository

		err = repository.UpdateEvent(&task.DueAt)
		if err != nil {
			return err
		}
	}

	if !updateWorkUnits {
		return nil
	}

	for _, unit := range task.WorkUnits {
		unit.ScheduledAt.Title = s.taskTextRenderer.RenderWorkUnitEventTitle(task)

		for _, user := range relevantUsers {
			err = repositories[user.ID.Hex()].UpdateEvent(&unit.ScheduledAt)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DeleteTask deletes all events that are connected to a task
func (s *PlanningService) DeleteTask(ctx context.Context, task *Task) error {
	err := s.taskRepository.Delete(ctx, task.ID.Hex(), task.UserID.Hex())
	if err != nil {
		return err
	}

	relevantUsers, err := s.getAllRelevantUsers(ctx, task)
	if err != nil {
		return err
	}

	repositories := make(map[string]calendar.RepositoryInterface)

	// TODO make these parallel
	for _, user := range relevantUsers {
		repository, err := s.calendarRepositoryManager.GetTaskCalendarRepositoryForUser(ctx, user)
		if err != nil {
			return err
		}

		repositories[user.ID.Hex()] = repository

		for _, unit := range task.WorkUnits {

			err := repository.DeleteEvent(&unit.ScheduledAt)
			if err != nil {
				return err
			}
		}
	}

	for _, user := range relevantUsers {
		err = repositories[user.ID.Hex()].DeleteEvent(&task.DueAt)
		if err != nil {
			return err
		}
	}

	return nil
}

// SyncCalendar triggers a sync on a single calendar
func (s *PlanningService) SyncCalendar(ctx context.Context, user *users.User, calendarID string) (*users.User, error) {
	eventChannel := make(chan *calendar.Event)
	errorChannel := make(chan error)
	userChannel := make(chan *users.User)

	calendarRepository, err := s.calendarRepositoryManager.GetCalendarRepositoryForUserByCalendarID(ctx, user, calendarID)
	if err != nil {
		return nil, err
	}

	go calendarRepository.SyncEvents(calendarID, user, &eventChannel, &errorChannel, &userChannel)

	wg := sync.WaitGroup{}
	for {
		select {
		case user := <-userChannel:
			wg.Wait()
			return user, nil
		case event := <-eventChannel:
			wg.Add(1)
			go func(wg *sync.WaitGroup) {
				s.processTaskEventChange(ctx, event, user.ID.Hex())
				wg.Done()
			}(&wg)
		case err := <-errorChannel:
			return nil, err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// processTaskEventChange processes a single event change and updates the task accordingly
func (s *PlanningService) processTaskEventChange(ctx context.Context, event *calendar.Event, userID string) {
	calendarEvent := event.CalendarEvents.FindByUserID(userID)
	if calendarEvent == nil {
		s.logger.Error("no persisted event", fmt.Errorf("could not find calendar event for user %s", userID))
		return
	}

	task, err := s.taskRepository.FindByCalendarEventID(ctx, calendarEvent.CalendarEventID, userID, false)
	if err != nil {
		if event.Deleted || event.IsOriginal {
			s.lookForUnscheduledTasks(ctx, userID)
			return
		}
		_ = s.checkForIntersectingWorkUnits(ctx, userID, event, "")
		s.lookForUnscheduledTasks(ctx, userID)

		return
	}

	lock, err := s.locker.Acquire(ctx, task.ID.Hex(), time.Second*10)
	if err != nil {
		s.logger.Error(fmt.Sprintf("problem acquiring lock for task %s", task.ID.Hex()), err)
		return
	}

	defer func(lock locking.LockInterface, ctx context.Context) {
		err := lock.Release(ctx)
		if err != nil {
			s.logger.Error("problem releasing lock", err)
			return
		}

		s.lookForUnscheduledTasks(ctx, userID)
	}(lock, ctx)

	// Refresh task, after potential change
	task, err = s.taskRepository.FindUpdatableByID(ctx, task.ID.Hex(), userID, false)
	if err != nil {
		s.logger.Error("could not refresh already loaded task", err)
		return
	}

	dueAtCalendarEvent := task.DueAt.CalendarEvents.FindByUserID(userID)
	if dueAtCalendarEvent != nil && dueAtCalendarEvent.CalendarEventID == calendarEvent.CalendarEventID {
		// If there is no change we do nothing
		if task.DueAt.Date == event.Date {
			return
		}

		// If the event is deleted we delete the task
		if event.Deleted {
			err := s.DeleteTask(ctx, (*Task)(task))
			if err != nil {
				s.logger.Error("problem with deleting task", err)
				return
			}
			return
		}

		// If the event is not deleted, we update the task
		task.DueAt.Date = event.Date
		err = s.updateCalendarEventForOtherCollaborators(ctx, (*Task)(task), userID, &task.DueAt)
		if err != nil {
			s.logger.Error(fmt.Sprintf("problem updating other collaborators workunit event %s", task.ID.Hex()), err)
			return
		}

		err = s.taskRepository.Update(ctx, task, false)
		if err != nil {
			s.logger.Error("problem with updating task", err)
			return
		}

		// Also check if we need to reschedule work units
		var toReschedule []WorkUnit
		for _, unit := range task.WorkUnits {
			if unit.ScheduledAt.Date.End.After(task.DueAt.Date.Start) {
				toReschedule = append(toReschedule, unit)
			}
		}

		for _, unit := range toReschedule {
			task, err = s.rescheduleWorkUnitWithoutLock(ctx, task, &unit)
			if err != nil {
				s.logger.Error(fmt.Sprintf("Problem rescheduling work unit %s", unit.ID.Hex()), err)
				return
			}
		}

		return
	}

	index, workunit := task.WorkUnits.FindByCalendarID(calendarEvent.CalendarEventID)
	if workunit == nil {
		s.logger.Error("there was an event id that could not be found inside a task", nil)
		return
	}

	if workunit.ScheduledAt.Date.Start == event.Date.Start && workunit.ScheduledAt.Date.End == event.Date.End {
		return
	}

	task.WorkloadOverall -= workunit.Workload

	// If the event is deleted we delete the work unit
	if event.Deleted {
		relevantUsers, err := s.getAllRelevantUsers(ctx, (*Task)(task))
		if err != nil {
			s.logger.Error(fmt.Sprintf("could not get all relevant users for task %s", task.ID.Hex()), err)
			return
		}

		// Delete work unit event for all relevant users
		for _, user := range relevantUsers {
			if user.ID.Hex() == userID {
				// We don't need to delete the already deleted event
				continue
			}

			calendarRepository, err := s.calendarRepositoryManager.GetTaskCalendarRepositoryForUser(ctx, user)
			if err != nil {
				s.logger.Error(fmt.Sprintf("could not get calendar repository for user %s", user.ID.Hex()), err)
				continue
			}

			err = calendarRepository.DeleteEvent(&workunit.ScheduledAt)
			if err != nil {
				s.logger.Error(fmt.Sprintf("could not delete event for user %s in task %s", user.ID.Hex(), task.ID.Hex()), err)
				continue
			}
		}

		// Delete the work unit
		task.WorkUnits = task.WorkUnits.RemoveByIndex(index)
		err = s.taskRepository.Update(ctx, task, false)
		if err != nil {
			s.logger.Error("problem with updating task", err)
			return
		}
		return
	}

	// If the work unit event is not deleted, we update the work unit
	workunit.ScheduledAt.Date = event.Date
	err = s.updateCalendarEventForOtherCollaborators(ctx, (*Task)(task), userID, &workunit.ScheduledAt)
	if err != nil {
		s.logger.Error(fmt.Sprintf("problem updating other collaborators workunit event %s", task.ID.Hex()), err)
		// We don't return here, because we still need to update the task
	}

	workunit.Workload = workunit.ScheduledAt.Date.Duration()

	task.WorkloadOverall += workunit.Workload

	task.WorkUnits[index] = *workunit

	task.WorkUnits = task.WorkUnits.RemoveByIndex(index)
	task.WorkUnits = task.WorkUnits.Add(workunit)

	err = s.taskRepository.Update(ctx, task, false)
	if err != nil {
		s.logger.Error("problem with updating task", err)
		return
	}
}

func (s *PlanningService) updateCalendarEventForOtherCollaborators(ctx context.Context, task *Task, userID string, event *calendar.Event) error {
	relevantUsers, err := s.getAllRelevantUsers(ctx, task)
	if err != nil {
		return err
	}

	for _, user := range relevantUsers {
		if user.ID.Hex() == userID {
			// We don't need to delete the already deleted event
			continue
		}

		calendarRepository, err := s.calendarRepositoryManager.GetTaskCalendarRepositoryForUser(ctx, user)
		if err != nil {
			s.logger.Error(fmt.Sprintf("could not get calendar repository for user %s", user.ID.Hex()), err)
			continue
		}

		err = calendarRepository.UpdateEvent(event)
		if err != nil {
			s.logger.Error(fmt.Sprintf("could not update event for user %s in task %s", user.ID.Hex(), task.ID.Hex()), err)
			continue
		}
	}

	return nil
}

// checkForIntersectingWorkUnits checks if the given work unit or event intersects with any other work unit
func (s *PlanningService) checkForIntersectingWorkUnits(ctx context.Context, userID string, event *calendar.Event, workUnitID string) int {
	intersectingTasks, err := s.taskRepository.FindIntersectingWithEvent(ctx, userID, event, workUnitID, false)
	if err != nil {
		s.logger.Error("problem while trying to find tasks intersecting with an event", err)
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
			updatedTask, err := s.RescheduleWorkUnit(ctx, (*TaskUpdate)(&intersection.Task), &unit)
			if err != nil {
				s.logger.Error(fmt.Sprintf(
					"Could not reschedule work unit %d for task %s",
					intersection.IntersectingWorkUnitIndices[i], intersection.Task.ID.Hex()), err)
				continue
			}

			intersection.Task = Task(*updatedTask)
		}
	}

	return len(intersectingTasks)
}

// lookForUnscheduledTasks looks for tasks that have unscheduled time
func (s *PlanningService) lookForUnscheduledTasks(ctx context.Context, userID string) {
	lock, err := s.locker.Acquire(ctx, fmt.Sprintf("lookForUnscheduledTasks-%s", userID), time.Minute*1)
	if err != nil {
		s.logger.Warning("could not acquire lock", errors.Wrap(err, fmt.Sprintf("could not acquire lock for looking for unscheduled tasks for user %s", userID)))
		return
	}

	defer func() {
		err := lock.Release(ctx)
		if err != nil {
			s.logger.Error("could not release lock", errors.Wrap(err, fmt.Sprintf("could not release lock for looking for unscheduled tasks for user %s", userID)))
			return
		}
	}()

	tasks, _, err := s.taskRepository.FindUnscheduledTasks(ctx, userID, 0, 10)
	if err != nil {
		s.logger.Error("problem while trying to find unscheduled tasks", err)
		return
	}

	for _, task := range tasks {
		_, err := s.ScheduleTask(ctx, &task)
		if err != nil {
			s.logger.Error(fmt.Sprintf("problem scheduling task %s", task.ID.Hex()), err)
			return
		}
	}
}
