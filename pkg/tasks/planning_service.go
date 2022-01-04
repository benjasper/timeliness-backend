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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/sync/errgroup"
	"sort"
	"sync"
	"time"
)

// now is the current time and is globally available to override it in tests
var now = time.Now

// WorkUnitDurationMin is the minimum duration of a work unit
const WorkUnitDurationMin = time.Hour * 2

// WorkUnitDurationMax is the maximum duration of a work unit
const WorkUnitDurationMax = time.Hour * 6

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
		collaborator := collaborator

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

func (s *PlanningService) initializeTimeWindow(task *Task, relevantUsers []*users.User) (*date.TimeWindow, *date.FreeConstraint, error) {
	// TODO: This is random(first user) for now, this has to be changed when multi user support is implemented
	location, err := time.LoadLocation(relevantUsers[0].Settings.Scheduling.TimeZone)
	if err != nil {
		return nil, nil, err
	}

	// TODO merge these? or only take owners constraints?; Also move this into its own function, so we can called it when needed
	constraint := &date.FreeConstraint{
		Location:         location,
		AllowedTimeSpans: relevantUsers[0].Settings.Scheduling.AllowedTimespans,
	}

	var spacing time.Duration
	for _, user := range relevantUsers {
		spacing = maxDuration(user.Settings.Scheduling.BusyTimeSpacing, spacing)
	}

	nowRound := now().Add(time.Minute * 15).Round(time.Minute * 15)
	return &date.TimeWindow{Start: nowRound.UTC(), End: task.DueAt.Date.Start.UTC(), BusyPadding: spacing}, constraint, nil
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
func (s *PlanningService) ScheduleTask(ctx context.Context, t *Task, withLock bool) (*Task, error) {
	if !t.ID.IsZero() && withLock == true {
		lock, err := s.locker.Acquire(ctx, t.ID.Hex(), time.Second*30, false)
		if err != nil {
			return nil, err
		}

		defer func(lock locking.LockInterface, ctx context.Context) {
			err := lock.Release(ctx)
			if err != nil {
				s.logger.Error("problem releasing lock", err)
			}
		}(lock, ctx)

		t, err = s.taskRepository.FindByID(ctx, t.ID.Hex(), t.UserID.Hex(), false)
		if err != nil {
			return nil, err
		}
	}

	relevantUsers, err := s.getAllRelevantUsers(ctx, t)
	if err != nil {
		return nil, err
	}

	windowTotal, constraint, err := s.initializeTimeWindow(t, relevantUsers)
	if err != nil {
		return nil, err
	}

	workloadToSchedule := t.WorkloadOverall
	for _, unit := range t.WorkUnits {
		workloadToSchedule -= unit.Workload
	}
	t.NotScheduled = 0

	taskCalendarRepositories := make(map[string]calendar.RepositoryInterface)
	var availabilityRepositories []calendar.RepositoryInterface

	// TODO make TimeWindow thread safe and make this parallel
	for _, user := range relevantUsers {
		taskRepository, err := s.calendarRepositoryManager.GetTaskCalendarRepositoryForUser(ctx, user)
		if err != nil {
			return nil, err
		}

		taskCalendarRepositories[user.ID.Hex()] = taskRepository

		// Repositories for availability
		availabilityRepositoriesForUser, err := s.calendarRepositoryManager.GetAllCalendarRepositoriesForUser(ctx, user)
		if err != nil {
			return nil, err
		}

		availabilityRepositories = append(availabilityRepositories, availabilityRepositoriesForUser...)
	}

	targetTime := s.getTargetTimeForUser(relevantUsers[0], windowTotal, workloadToSchedule)
	windowTotal, err = s.computeAvailabilityForTimeWindow(targetTime, workloadToSchedule, windowTotal, availabilityRepositories, constraint)
	if err != nil {
		return nil, err
	}

	if workloadToSchedule > 0 {
		workUnits := t.WorkUnits

		foundWorkUnits := s.findWorkUnitTimes(windowTotal, workloadToSchedule)

		for _, workUnit := range foundWorkUnits {
			workUnit.ScheduledAt.Blocking = true
			workUnit.ScheduledAt.Title = s.taskTextRenderer.RenderWorkUnitEventTitle(t)
			workUnit.ScheduledAt.Description = ""

			var workEvent *calendar.Event
			for _, user := range relevantUsers {
				workEvent, err = taskCalendarRepositories[user.ID.Hex()].NewEvent(&workUnit.ScheduledAt, t.ID.Hex())
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
				return nil, errors.New("workload can't be less than all not done work units combined")
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

		err := s.taskRepository.Update(ctx, t, false)
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
				err = taskCalendarRepositories[user.ID.Hex()].UpdateEvent(&unit.ScheduledAt, t.ID.Hex())
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
			dueEvent, err = taskCalendarRepositories[user.ID.Hex()].NewEvent(&t.DueAt, t.ID.Hex())
			if err != nil {
				return nil, err
			}
		}

		t.DueAt = *dueEvent
	}

	if !t.ID.IsZero() {
		err := s.taskRepository.Update(ctx, t, false)
		if err != nil {
			return nil, err
		}
	}

	return t, nil
}

// RescheduleWorkUnit takes a work unit and reschedules it to a time between now and the task due end, updates task
func (s *PlanningService) RescheduleWorkUnit(ctx context.Context, t *Task, w *WorkUnit, withLock bool) (*Task, error) {
	if withLock == true {
		lock, err := s.locker.Acquire(ctx, t.ID.Hex(), time.Second*30, false)
		if err != nil {
			return nil, err
		}

		defer func(lock locking.LockInterface, ctx context.Context) {
			err := lock.Release(ctx)
			if err != nil {
				s.logger.Error("problem releasing lock", err)
			}
		}(lock, ctx)

		// Refresh task, after potential change
		t, err = s.taskRepository.FindByID(ctx, t.ID.Hex(), t.UserID.Hex(), false)
		if err != nil {
			return nil, err
		}
	}

	index, _ := t.WorkUnits.FindByID(w.ID.Hex())
	if index < 0 {
		return nil, fmt.Errorf("could not find workunit %s in task %s", w.ID.Hex(), t.ID.Hex())
	}

	relevantUsers, err := s.getAllRelevantUsers(ctx, t)
	if err != nil {
		return nil, err
	}

	windowTotal, constraint, err := s.initializeTimeWindow(t, relevantUsers)
	if err != nil {
		return nil, err
	}

	taskRepositories := make(map[string]calendar.RepositoryInterface)
	var availabilityRepositories []calendar.RepositoryInterface

	// TODO Make parallel
	for _, user := range relevantUsers {
		taskRepository, err := s.calendarRepositoryManager.GetTaskCalendarRepositoryForUser(ctx, user)
		if err != nil {
			return nil, err
		}

		taskRepositories[user.ID.Hex()] = taskRepository

		// Repositories for availability
		availabilityRepositoriesForUser, err := s.calendarRepositoryManager.GetAllCalendarRepositoriesForUser(ctx, user)
		if err != nil {
			return nil, err
		}

		availabilityRepositories = append(availabilityRepositories, availabilityRepositoriesForUser...)
	}

	targetTime := s.getTargetTimeForUser(relevantUsers[0], windowTotal, w.Workload)
	windowTotal, err = s.computeAvailabilityForTimeWindow(targetTime, w.Workload, windowTotal, availabilityRepositories, constraint)
	if err != nil {
		return nil, err
	}

	workloadToSchedule := w.Workload

	foundWorkUnits := s.findWorkUnitTimes(windowTotal, workloadToSchedule)

	if len(foundWorkUnits) == 0 {
		t.WorkUnits = t.WorkUnits.RemoveByIndex(index)

		for _, user := range relevantUsers {
			err = taskRepositories[user.ID.Hex()].DeleteEvent(&w.ScheduledAt)
			if err != nil {
				return nil, err
			}
		}
	} else if len(foundWorkUnits) > 0 {
		t.WorkUnits[index].ScheduledAt.Date = foundWorkUnits[0].ScheduledAt.Date
		t.WorkUnits[index].Workload = foundWorkUnits[0].Workload

		for _, user := range relevantUsers {
			err = taskRepositories[user.ID.Hex()].UpdateEvent(&t.WorkUnits[index].ScheduledAt, t.ID.Hex())
			if err != nil {
				return nil, err
			}
		}

		t.WorkUnits.Sort()

		foundWorkUnits = foundWorkUnits.RemoveByIndex(0)
		workloadToSchedule -= workloadToSchedule
	}

	for _, workUnit := range foundWorkUnits {
		workUnit.ScheduledAt.Blocking = true
		workUnit.ScheduledAt.Title = s.taskTextRenderer.RenderWorkUnitEventTitle(t)
		workUnit.ScheduledAt.Description = ""

		var workEvent *calendar.Event
		for _, user := range relevantUsers {
			workEvent, err = taskRepositories[user.ID.Hex()].NewEvent(&workUnit.ScheduledAt, t.ID.Hex())
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

// SuggestTimespansForWorkUnit returns a list of timespans that can be used to reschedule a work unit
func (s *PlanningService) SuggestTimespansForWorkUnit(ctx context.Context, t *Task, w *WorkUnit) ([][]date.Timespan, error) {
	relevantUsers, err := s.getAllRelevantUsers(ctx, t)
	if err != nil {
		return nil, err
	}

	windowTotal, constraint, err := s.initializeTimeWindow(t, relevantUsers)
	if err != nil {
		return nil, err
	}

	taskCalendarRepositories := make(map[string]calendar.RepositoryInterface)
	var availabilityRepositories []calendar.RepositoryInterface

	// TODO make this parallel
	for _, user := range relevantUsers {
		taskRepository, err := s.calendarRepositoryManager.GetTaskCalendarRepositoryForUser(ctx, user)
		if err != nil {
			return nil, err
		}

		taskCalendarRepositories[user.ID.Hex()] = taskRepository

		// Repositories for availability
		availabilityRepositoriesForUser, err := s.calendarRepositoryManager.GetAllCalendarRepositoriesForUser(ctx, user)
		if err != nil {
			return nil, err
		}

		availabilityRepositories = append(availabilityRepositories, availabilityRepositoriesForUser...)
	}

	var iterations time.Duration = 5

	targetTime := s.getTargetTimeForUser(relevantUsers[0], windowTotal, w.Workload)
	windowTotal, err = s.computeAvailabilityForTimeWindow(targetTime, w.Workload*iterations, windowTotal, availabilityRepositories, constraint)
	if err != nil {
		return nil, err
	}

	return s.findWorkUnitTimesForExactWorkload(windowTotal, w.Workload, int(iterations)), nil
}

func (s *PlanningService) findWorkUnitTimes(w *date.TimeWindow, durationToFind time.Duration) WorkUnits {
	var workUnits WorkUnits
	if w.FreeDuration == 0 {
		return workUnits
	}

	minDuration := 1 * time.Hour
	if durationToFind < 1*time.Hour {
		minDuration = durationToFind
	}
	maxDuration := WorkUnitDurationMax

	for w.FreeDuration >= 0 && durationToFind > 0 {
		if durationToFind < WorkUnitDurationMax {
			if durationToFind < WorkUnitDurationMin {
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

	return workUnits
}

func (s *PlanningService) findWorkUnitTimesForExactWorkload(w *date.TimeWindow, durationToFindPerIteration time.Duration, iterations int) [][]date.Timespan {
	var timespanGroups [][]date.Timespan

	if w.FreeDuration == 0 || w.FreeDuration < durationToFindPerIteration {
		return timespanGroups
	}

	for i := 0; i < iterations; i++ {
		durationToFind := durationToFindPerIteration

		minDuration := 1 * time.Hour
		if durationToFind < 1*time.Hour {
			minDuration = durationToFind
		}
		maxDuration := WorkUnitDurationMax

		timespanGroup := make([]date.Timespan, 0)

		for w.FreeDuration >= 0 && durationToFind > 0 {
			if durationToFind < WorkUnitDurationMax {
				if durationToFind < WorkUnitDurationMin {
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

			timespanGroup = append(timespanGroup, *slot)
		}

		if durationToFind != 0 {
			break
		}

		sort.Slice(timespanGroup, func(i, j int) bool {
			return timespanGroup[i].Start.Before(timespanGroup[j].Start)
		})

		timespanGroups = append(timespanGroups, timespanGroup)
	}

	sort.Slice(timespanGroups, func(i, j int) bool {
		return timespanGroups[i][0].Start.Before(timespanGroups[j][0].Start)
	})

	return timespanGroups
}

// CreateNewSpecificWorkUnits creates new work units for a specific task and times (won't save the task)
func (s *PlanningService) CreateNewSpecificWorkUnits(ctx context.Context, task *Task, timespans []date.Timespan) (*Task, error) {
	relevantUsers, err := s.getAllRelevantUsers(ctx, task)
	if err != nil {
		return nil, err
	}

	for _, timespan := range timespans {
		workUnit := WorkUnit{
			ID:       primitive.NewObjectID(),
			Workload: timespan.Duration(),
			ScheduledAt: calendar.Event{
				Title:    s.taskTextRenderer.RenderWorkUnitEventTitle(task),
				Blocking: true,
				Date:     timespan,
			},
		}

		for _, user := range relevantUsers {
			repository, err := s.calendarRepositoryManager.GetTaskCalendarRepositoryForUser(ctx, user)
			if err != nil {
				return nil, err
			}

			event, err := repository.NewEvent(&workUnit.ScheduledAt, task.ID.Hex())
			if err != nil {
				return nil, err
			}

			workUnit.ScheduledAt = *event
		}

		task.WorkloadOverall += workUnit.Workload
		task.WorkUnits = task.WorkUnits.Add(&workUnit)
	}

	return task, nil
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

		err = repository.UpdateEvent(event, task.ID.Hex())
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

		err = repository.UpdateEvent(&task.DueAt, task.ID.Hex())
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
			err = repositories[user.ID.Hex()].UpdateEvent(&unit.ScheduledAt, task.ID.Hex())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// UpdateWorkUnitTitle updates the event title of a work unit
func (s *PlanningService) UpdateWorkUnitTitle(ctx context.Context, task *Task, unit *WorkUnit) error {
	unit.ScheduledAt.Title = s.taskTextRenderer.RenderWorkUnitEventTitle(task)

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

		err = repository.UpdateEvent(&unit.ScheduledAt, task.ID.Hex())
		if err != nil {
			return err
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
			return nil, errors.WithStack(err)
		case <-ctx.Done():
			return nil, errors.Wrap(err, "context canceled")
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
		_ = s.checkForIntersectingWorkUnits(ctx, userID, event, nil)
		s.lookForUnscheduledTasks(ctx, userID)

		return
	}

	lock, err := s.locker.Acquire(ctx, task.ID.Hex(), time.Second*10, false)
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
	}(lock, ctx)

	// Refresh task, after potential change
	task, err = s.taskRepository.FindByCalendarEventID(ctx, calendarEvent.CalendarEventID, userID, false)
	if err != nil {
		s.logger.Info("could not find task with event id after refresh, event id has passed apparently")
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
			err := s.DeleteTask(ctx, task)
			if err != nil {
				s.logger.Error("problem with deleting task", err)
				return
			}

			s.lookForUnscheduledTasks(ctx, userID)
			return
		}

		// If the event is not deleted, we update the task
		task.DueAt.Date = event.Date
		err = s.updateCalendarEventForOtherCollaborators(ctx, task, userID, &task.DueAt)
		if err != nil {
			s.logger.Error(fmt.Sprintf("problem updating other collaborators workUnit event %s", task.ID.Hex()), err)
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
			if unit.ScheduledAt.Date.End.After(task.DueAt.Date.Start) && unit.IsDone == false {
				toReschedule = append(toReschedule, unit)
			}
		}

		for _, unit := range toReschedule {
			task, err = s.RescheduleWorkUnit(ctx, task, &unit, false)
			if err != nil {
				s.logger.Error(fmt.Sprintf("Problem rescheduling work unit %s", unit.ID.Hex()), err)
				return
			}
		}

		s.lookForUnscheduledTasks(ctx, userID)
		return
	}

	index, workUnit := task.WorkUnits.FindByCalendarID(calendarEvent.CalendarEventID)
	if workUnit == nil {
		s.logger.Error("event not found", errors.Errorf("could not find work unit for calendar event %s", calendarEvent.CalendarEventID))
		return
	}

	if workUnit.ScheduledAt.Date.Start == event.Date.Start && workUnit.ScheduledAt.Date.End == event.Date.End {
		return
	}

	task.WorkloadOverall -= workUnit.Workload

	// If the event is deleted we delete the work unit
	if event.Deleted {
		relevantUsers, err := s.getAllRelevantUsers(ctx, task)
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

			err = calendarRepository.DeleteEvent(&workUnit.ScheduledAt)
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

		s.lookForUnscheduledTasks(ctx, userID)
		return
	}

	// If the work unit event is not deleted, we update the work unit
	workUnit.ScheduledAt.Date = event.Date
	err = s.updateCalendarEventForOtherCollaborators(ctx, task, userID, &workUnit.ScheduledAt)
	if err != nil {
		s.logger.Error(fmt.Sprintf("problem updating other collaborators workUnit event %s", task.ID.Hex()), err)
		// We don't return here, because we still need to update the task
	}

	workUnit.Workload = workUnit.ScheduledAt.Date.Duration()

	task.WorkloadOverall += workUnit.Workload

	task.WorkUnits[index] = *workUnit

	task.WorkUnits = task.WorkUnits.RemoveByIndex(index)
	task.WorkUnits = task.WorkUnits.Add(workUnit)

	task = s.checkForMergingWorkUnits(ctx, task)

	err = s.taskRepository.Update(ctx, task, false)
	if err != nil {
		s.logger.Error("problem with updating task", err)
		return
	}

	_ = s.checkForIntersectingWorkUnits(ctx, userID, event, &task.ID)

	s.lookForUnscheduledTasks(ctx, userID)
}

// checkForMergingWorkUnits looks for work units that are scheduled right after one another and merges them
func (s *PlanningService) checkForMergingWorkUnits(ctx context.Context, task *Task) *Task {
	lastDate := date.Timespan{}
	var relevantUsers []*users.User

	for i, unit := range task.WorkUnits {
		if unit.ScheduledAt.Date.IntersectsWith(lastDate) || unit.ScheduledAt.Date.Start.Equal(lastDate.End) {
			if len(relevantUsers) == 0 {
				relevantUsers, _ = s.getAllRelevantUsers(ctx, task)
			}

			var spacing time.Duration
			for _, user := range relevantUsers {
				spacing = maxDuration(user.Settings.Scheduling.BusyTimeSpacing, spacing)
			}

			// In case the users consent on no busy padding we don't want to merge them because this could be wanted behaviour
			if unit.ScheduledAt.Date.Start.Equal(lastDate.End) && spacing == 0 {
				return task
			}

			// Reduce of both work units
			task.WorkloadOverall -= unit.Workload
			task.WorkloadOverall -= task.WorkUnits[i-1].Workload

			// Recalculate the workload of the merged work unit, based on the new end date
			task.WorkUnits[i-1].ScheduledAt.Date.End = unit.ScheduledAt.Date.End
			task.WorkUnits[i-1].Workload = task.WorkUnits[i-1].ScheduledAt.Date.Duration()

			// Reapply the workload of the merged work unit
			task.WorkloadOverall += task.WorkUnits[i-1].Workload

			for _, user := range relevantUsers {
				calendarRepository, err := s.calendarRepositoryManager.GetTaskCalendarRepositoryForUser(ctx, user)
				if err != nil {
					s.logger.Error(fmt.Sprintf("could not get calendar repository for user %s", user.ID.Hex()), err)
					continue
				}

				err = calendarRepository.DeleteEvent(&unit.ScheduledAt)
				if err != nil {
					s.logger.Error(fmt.Sprintf("could not delete event for user %s in task %s", user.ID.Hex(), task.ID.Hex()), err)
					// Try the other action
				}

				err = calendarRepository.UpdateEvent(&task.WorkUnits[i-1].ScheduledAt, task.ID.Hex())
				if err != nil {
					s.logger.Error(fmt.Sprintf("could not update event for user %s in task %s", user.ID.Hex(), task.ID.Hex()), err)
					continue
				}
			}

			task.WorkUnits = task.WorkUnits.RemoveByIndex(i)

			// We only want to merge one work unit max
			break
		}

		lastDate = unit.ScheduledAt.Date
	}

	return task
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

		err = calendarRepository.UpdateEvent(event, task.ID.Hex())
		if err != nil {
			s.logger.Error(fmt.Sprintf("could not update event for user %s in task %s", user.ID.Hex(), task.ID.Hex()), err)
			continue
		}
	}

	return nil
}

// checkForIntersectingWorkUnits checks if the given work unit or event intersects with any other work unit
func (s *PlanningService) checkForIntersectingWorkUnits(ctx context.Context, userID string, event *calendar.Event, ignoreTaskID *primitive.ObjectID) int {
	intersectingTasks, err := s.taskRepository.FindIntersectingWithEvent(ctx, userID, event, ignoreTaskID, false)
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
			updatedTask, err := s.RescheduleWorkUnit(ctx, &intersection.Task, &unit, true)
			if err != nil {
				s.logger.Error(fmt.Sprintf(
					"Could not reschedule work unit %d for task %s",
					intersection.IntersectingWorkUnitIndices[i], intersection.Task.ID.Hex()), err)
				continue
			}

			intersection.Task = *updatedTask
		}
	}

	return len(intersectingTasks)
}

// lookForUnscheduledTasks looks for tasks that have unscheduled time
func (s *PlanningService) lookForUnscheduledTasks(ctx context.Context, userID string) {
	lock, err := s.locker.Acquire(ctx, fmt.Sprintf("lookForUnscheduledTasks-%s", userID), time.Minute*1, true)
	if err != nil {
		// This is fine, and we don't want to spam the logs
		return
	}

	defer func() {
		err := lock.Release(ctx)
		if err != nil {
			s.logger.Error("could not release lock", errors.Wrap(err, fmt.Sprintf("could not release lock for looking for unscheduled tasks for user %s", userID)))
			return
		}
	}()

	// We max this to 10 tasks per run for now to not overload the system
	tasks, _, err := s.taskRepository.FindUnscheduledTasks(ctx, userID, 0, 10)
	if err != nil {
		s.logger.Error("problem while trying to find unscheduled tasks", err)
		return
	}

	for _, task := range tasks {
		_, err := s.ScheduleTask(ctx, &task, true)
		if err != nil {
			s.logger.Error(fmt.Sprintf("problem scheduling task %s", task.ID.Hex()), errors.Wrap(err, "problem scheduling task while looking for unscheduled tasks"))
			return
		}
	}
}

// computeAvailabilityForTimeWindow traverses the given time interval by two weeks and returns when it found enough free time or traversed the whole interval
func (s *PlanningService) computeAvailabilityForTimeWindow(target time.Time, timeToSchedule time.Duration, window *date.TimeWindow, repositories []calendar.RepositoryInterface, constraint *date.FreeConstraint) (*date.TimeWindow, error) {
	s.generateTimespansBasedOnTargetDate(target, window, func(timespans []date.Timespan) bool {
		for _, timespan := range timespans {
			wg := errgroup.Group{}

			for _, repository := range repositories {
				repository := repository

				wg.Go(func() error {
					err := repository.AddBusyToWindow(window, timespan.Start, timespan.End)
					if err != nil {
						return errors.Wrap(err, "problem while adding busy time to window")
					}

					return nil
				})
			}

			err := wg.Wait()
			if err != nil {
				s.logger.Error("problem while adding busy time to window", err)
				return true
			}

			window.ComputeFree(constraint, target, timespan)

			if window.FreeDuration >= timeToSchedule {
				return true
			}
		}

		return false
	})

	return window, nil
}

func absoluteOfDuration(duration time.Duration) time.Duration {
	if duration < 0 {
		return -duration
	}

	return duration
}

// generateTimespansBasedOnTargetDate generates a list of timespans based on a target date and expands the list from the target date outwards
// the yieldFunc is called for each generated timespan and should return true if it should stop generating timespans
func (s *PlanningService) generateTimespansBasedOnTargetDate(target time.Time, window *date.TimeWindow, yieldFunc func(timespans []date.Timespan) bool) {
	leftBoundDone := false
	rightBoundDone := false

	// TODO Check the time on these to prevent cutting into free time
	leftPointer := target.AddDate(0, 0, -7)
	rightPointer := target.AddDate(0, 0, 7)

	if leftPointer.Before(window.Start) || leftPointer.Equal(window.Start) {
		leftPointer = window.Start
		leftBoundDone = true
	}

	if rightPointer.After(window.End) || rightPointer.Equal(window.End) {
		rightPointer = window.End
		rightBoundDone = true
	}

	stop := yieldFunc([]date.Timespan{
		{
			Start: leftPointer,
			End:   rightPointer,
		},
	})
	if stop {
		return
	}

	for !leftBoundDone || !rightBoundDone {
		var newTimespans []date.Timespan

		if !leftBoundDone {
			lastLeftPointer := leftPointer
			// TODO Check the time on these to prevent cutting into free time
			leftPointer = leftPointer.AddDate(0, 0, -14)

			if leftPointer.Before(window.Start) || leftPointer.Equal(window.Start) || absoluteOfDuration(window.Start.Sub(leftPointer)) < time.Hour*24*7 {
				leftPointer = window.Start
				leftBoundDone = true
			}

			newTimespans = append(newTimespans, date.Timespan{
				Start: leftPointer,
				End:   lastLeftPointer,
			})
		}

		if !rightBoundDone {
			lastRightPointer := rightPointer
			// TODO Check the time on these to prevent cutting into free time
			rightPointer = rightPointer.AddDate(0, 0, 14)

			if rightPointer.After(window.End) || rightPointer.Equal(window.End) || absoluteOfDuration(rightPointer.Sub(window.End)) < time.Hour*24*7 {
				rightPointer = window.End
				rightBoundDone = true
			}

			newTimespans = append(newTimespans, date.Timespan{
				Start: lastRightPointer,
				End:   rightPointer,
			})
		}

		stop = yieldFunc(newTimespans)
		if stop {
			return
		}
	}
}

func (s *PlanningService) getTargetTimeForUser(user *users.User, window *date.TimeWindow, workloadToSchedule time.Duration) time.Time {
	if window.End.Before(now().Add(time.Hour * 24 * 2)) {
		if window.End.Before(now().Add(time.Hour * 24)) {
			return window.Start
		}

		return window.Start.Add(time.Hour * 2)
	}

	switch user.Settings.Scheduling.TimingPreference {
	case users.TimingPreferenceVeryEarly:
		return window.Start.Add(time.Hour * 6)
	default:
	case users.TimingPreferenceEarly:
		return window.Start.Add(time.Hour * 24 * 2)
	case users.TimingPreferenceLate:
		offset := time.Hour*24*7 + workloadToSchedule
		if window.End.Before(now().Add(offset)) {
			return window.End.Add(time.Hour * -24 * 2)
		}
		return window.End.Add(-offset)
	case users.TimingPreferenceVeryLate:
		offset := time.Hour*6 + workloadToSchedule
		return window.End.Add(-offset)
	}
	return window.Start
}
