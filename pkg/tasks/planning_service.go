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

	if nowRound.Unix() == task.DueAt.Date.Start.Unix() || task.DueAt.Date.Start.Before(nowRound) {
		nowRound = nowRound.Add(time.Minute * 5).Round(time.Minute * 5)
	}

	return &date.TimeWindow{
		Start:             nowRound.UTC(),
		End:               task.DueAt.Date.Start.UTC(),
		BusyPadding:       spacing,
		MaxWorkUnitLength: relevantUsers[0].Settings.Scheduling.MaxWorkUnitDuration,
	}, constraint, nil
}

// getAllRelevantUsers fetches all relevant users for a task, the first one is always the owner
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
// and pushes or removes events to and from the calendar. Also updates the task.
func (s *PlanningService) ScheduleTask(ctx context.Context, t *Task, withLock bool) (*Task, error) {
	if !t.ID.IsZero() && withLock == true {
		lock, err := s.locker.Acquire(ctx, t.ID.Hex(), time.Second*30, false, 32*time.Second)
		if err != nil {
			return nil, err
		}

		defer func(lock locking.LockInterface, ctx context.Context) {
			err := lock.Release(ctx)
			if err != nil {
				s.logger.Error("error releasing lock", errors.Wrap(err, "error releasing lock"))
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
	windowTotal, err = s.computeAvailabilityForTimeWindow(targetTime, workloadToSchedule, windowTotal, availabilityRepositories, constraint, t)
	if err != nil {
		return nil, err
	}

	if workloadToSchedule > 0 {
		workUnits := t.WorkUnits

		foundWorkUnits := s.findWorkUnitTimes(windowTotal, workloadToSchedule, relevantUsers[0])

		for _, workUnit := range foundWorkUnits {
			workUnit.ScheduledAt.Blocking = true

			var workEvent *calendar.Event
			for _, user := range relevantUsers {
				workEvent, err = taskCalendarRepositories[user.ID.Hex()].NewEvent(&workUnit.ScheduledAt, t.ID.Hex(), s.taskTextRenderer.RenderWorkUnitEventTitle(t, &workUnit), "", s.taskTextRenderer.HasReminder(&workUnit))
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

		err = s.taskRepository.Update(ctx, t, false)
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
				err = taskCalendarRepositories[user.ID.Hex()].UpdateEvent(&unit.ScheduledAt, t.ID.Hex(), s.taskTextRenderer.RenderWorkUnitEventTitle(t, &unit), "", s.taskTextRenderer.HasReminder(&unit))
				if err != nil {
					return nil, err
				}
			}
		}
	}

	// Check if all work units are done, and mark the task as done if so
	allDone := true
	for _, unit := range t.WorkUnits {
		if !unit.IsDone {
			allDone = false
		}

		// Check if a user is missing a work unit event
		if len(unit.ScheduledAt.CalendarEvents) != len(relevantUsers) {
			for _, user := range relevantUsers {
				if persistedEvent := unit.ScheduledAt.CalendarEvents.FindByUserID(user.ID.Hex()); persistedEvent != nil {
					continue
				}
				newEvent, err := taskCalendarRepositories[user.ID.Hex()].NewEvent(&unit.ScheduledAt, t.ID.Hex(), s.taskTextRenderer.RenderWorkUnitEventTitle(t, &unit), "", s.taskTextRenderer.HasReminder(&unit))
				if err != nil {
					return nil, err
				}

				unit.ScheduledAt = *newEvent
			}
		}
	}

	if len(t.WorkUnits) == 0 || t.NotScheduled > 0 {
		allDone = false
	}
	t.IsDone = allDone

	// Create due date event if it doesn't exist
	t, err = s.UpdateDueAtEvent(ctx, t, relevantUsers, taskCalendarRepositories, false, true)
	if err != nil {
		return nil, err
	}

	if !t.ID.IsZero() {
		err = s.taskRepository.Update(ctx, t, false)
		if err != nil {
			return nil, err
		}
	}

	t = s.CheckForMergingWorkUnits(ctx, t)

	return t, nil
}

// RescheduleWorkUnit takes a work unit and reschedules it to a time between now and the task due end, updates task
func (s *PlanningService) RescheduleWorkUnit(ctx context.Context, t *Task, w *WorkUnit, withLock bool) (*Task, error) {
	if withLock == true {
		lock, err := s.locker.Acquire(ctx, t.ID.Hex(), time.Second*30, false, 32*time.Second)
		if err != nil {
			return nil, err
		}

		defer func(lock locking.LockInterface, ctx context.Context) {
			err := lock.Release(ctx)
			if err != nil {
				s.logger.Error("error releasing lock", errors.Wrap(err, "error releasing lock"))
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
	windowTotal, err = s.computeAvailabilityForTimeWindow(targetTime, w.Workload, windowTotal, availabilityRepositories, constraint, t)
	if err != nil {
		return nil, err
	}

	workloadToSchedule := w.Workload

	foundWorkUnits := s.findWorkUnitTimes(windowTotal, workloadToSchedule, relevantUsers[0])

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
			err = taskRepositories[user.ID.Hex()].UpdateEvent(&t.WorkUnits[index].ScheduledAt, t.ID.Hex(), s.taskTextRenderer.RenderWorkUnitEventTitle(t, &t.WorkUnits[index]), "", s.taskTextRenderer.HasReminder(&t.WorkUnits[index]))
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

		var workEvent *calendar.Event
		for _, user := range relevantUsers {
			workEvent, err = taskRepositories[user.ID.Hex()].NewEvent(&workUnit.ScheduledAt, t.ID.Hex(), s.taskTextRenderer.RenderWorkUnitEventTitle(t, &workUnit), "", s.taskTextRenderer.HasReminder(&workUnit))
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

	t = s.CheckForMergingWorkUnits(ctx, t)

	return t, nil
}

// SuggestTimespansForWorkUnit returns a list of timespans that can be used to reschedule a work unit
func (s *PlanningService) SuggestTimespansForWorkUnit(ctx context.Context, t *Task, w *WorkUnit, ignoreTimespans []date.Timespan) ([][]date.Timespan, error) {
	relevantUsers, err := s.getAllRelevantUsers(ctx, t)
	if err != nil {
		return nil, err
	}

	windowTotal, constraint, err := s.initializeTimeWindow(t, relevantUsers)
	if err != nil {
		return nil, err
	}

	for _, timespan := range ignoreTimespans {
		windowTotal.AddToBusy(timespan)
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
	windowTotal, err = s.computeAvailabilityForTimeWindow(targetTime, w.Workload*iterations, windowTotal, availabilityRepositories, constraint, t)
	if err != nil {
		return nil, err
	}

	return s.findWorkUnitTimesForExactWorkload(windowTotal, w.Workload, int(iterations), relevantUsers[0]), nil
}

func (s *PlanningService) findWorkUnitTimes(w *date.TimeWindow, durationToFind time.Duration, user *users.User) WorkUnits {
	var workUnits WorkUnits
	if w.FreeDuration() == 0 {
		return workUnits
	}

	minDuration := user.Settings.Scheduling.MinWorkUnitDuration
	if durationToFind < user.Settings.Scheduling.MinWorkUnitDuration {
		minDuration = durationToFind
	}
	maxDuration := user.Settings.Scheduling.MaxWorkUnitDuration

	for w.FreeDuration() >= 0 && durationToFind > 0 {
		if durationToFind < user.Settings.Scheduling.MaxWorkUnitDuration {
			if durationToFind < WorkUnitDurationMin {
				minDuration = durationToFind
			}
			maxDuration = durationToFind
		}

		slot := w.FindTimeSlot(&date.RuleDuration{Minimum: minDuration, Maximum: maxDuration})
		if slot == nil {
			break
		}
		durationToFind -= slot.Duration()
		workUnits = append(workUnits, WorkUnit{ScheduledAt: calendar.Event{Date: *slot}, Workload: slot.Duration()})
	}

	return workUnits
}

func (s *PlanningService) findWorkUnitTimesForExactWorkload(w *date.TimeWindow, durationToFindPerIteration time.Duration, iterations int, user *users.User) [][]date.Timespan {
	var timespanGroups = make([][]date.Timespan, 0)

	if w.FreeDuration() == 0 || w.FreeDuration() < durationToFindPerIteration {
		return timespanGroups
	}

	for i := 0; i < iterations; i++ {
		durationToFind := durationToFindPerIteration

		minDuration := user.Settings.Scheduling.MinWorkUnitDuration
		if durationToFind < user.Settings.Scheduling.MinWorkUnitDuration {
			minDuration = durationToFind
		}
		maxDuration := user.Settings.Scheduling.MaxWorkUnitDuration

		timespanGroup := make([]date.Timespan, 0)

		for w.FreeDuration() >= 0 && durationToFind > 0 {
			if durationToFind < user.Settings.Scheduling.MaxWorkUnitDuration {
				if durationToFind < WorkUnitDurationMin {
					minDuration = durationToFind
				}
				maxDuration = durationToFind
			}

			slot := w.FindTimeSlot(&date.RuleDuration{Minimum: minDuration, Maximum: maxDuration})
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
				Blocking: true,
				Date:     timespan,
			},
		}

		title := s.taskTextRenderer.RenderWorkUnitEventTitle(task, &workUnit)

		for _, user := range relevantUsers {
			repository, err := s.calendarRepositoryManager.GetTaskCalendarRepositoryForUser(ctx, user)
			if err != nil {
				return nil, err
			}

			event, err := repository.NewEvent(&workUnit.ScheduledAt, task.ID.Hex(), title, "", s.taskTextRenderer.HasReminder(&workUnit))
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

// UpdateDueAtEvent updates a due at event, creates missing events and deletes event when necessary
func (s *PlanningService) UpdateDueAtEvent(ctx context.Context, task *Task, relevantUsers []*users.User, taskCalendarRepositories map[string]calendar.RepositoryInterface, needsUpdate bool, ownerNeedsUpdate bool) (*Task, error) {
	// Create a new event for the due at date if it doesn't exist for a user
	if !task.IsDone && len(task.DueAt.CalendarEvents) != len(relevantUsers) {
		task.DueAt.Blocking = false
		task.DueAt.Date.End = task.DueAt.Date.Start.Add(time.Minute * 15)

		var dueEvent *calendar.Event
		for _, user := range relevantUsers {
			if persistedEvent := task.DueAt.CalendarEvents.FindByUserID(user.ID.Hex()); persistedEvent != nil {
				continue
			}

			var err error
			dueEvent, err = taskCalendarRepositories[user.ID.Hex()].NewEvent(&task.DueAt, task.ID.Hex(), s.taskTextRenderer.RenderDueEventTitle(task), "", s.taskTextRenderer.HasReminder(task))
			if err != nil {
				return nil, err
			}
		}

		task.DueAt = *dueEvent

		err := s.taskRepository.Update(ctx, task, false)
		if err != nil {
			return nil, err
		}
	}

	// Remove event when it's not needed anymore
	if relevantUsers[0].Settings.Scheduling.HideDeadlineWhenDone && task.IsDone {
		for _, user := range relevantUsers {
			if persistedEvent := task.DueAt.CalendarEvents.FindByUserID(user.ID.Hex()); persistedEvent == nil {
				continue
			}

			var err error
			err = taskCalendarRepositories[user.ID.Hex()].DeleteEvent(&task.DueAt)
			if err != nil {
				// We ignore the error here, because we don't want to stop the update process
			}

			task.DueAt.CalendarEvents = task.DueAt.CalendarEvents.RemoveByUserID(user.ID.Hex())
		}

		err := s.taskRepository.Update(ctx, task, false)
		if err != nil {
			return nil, err
		}

		return task, nil
	}

	if !needsUpdate {
		return task, nil
	}

	for _, user := range relevantUsers {
		if user.ID.Hex() == task.UserID.Hex() && !ownerNeedsUpdate {
			continue
		}

		repository, err := s.calendarRepositoryManager.GetTaskCalendarRepositoryForUser(ctx, user)
		if err != nil {
			return nil, err
		}

		err = repository.UpdateEvent(&task.DueAt, task.ID.Hex(), s.taskTextRenderer.RenderDueEventTitle(task), "", s.taskTextRenderer.HasReminder(task))
		if err != nil {
			return nil, err
		}
	}

	err := s.taskRepository.Update(ctx, task, false)
	if err != nil {
		return nil, err
	}

	return task, nil
}

// UpdateWorkUnitEvent updates a work unit event
func (s *PlanningService) UpdateWorkUnitEvent(ctx context.Context, task *Task, unit *WorkUnit) error {
	relevantUsers, err := s.getAllRelevantUsers(ctx, task)
	if err != nil {
		return err
	}

	for _, user := range relevantUsers {
		repository, err := s.calendarRepositoryManager.GetTaskCalendarRepositoryForUser(ctx, user)
		if err != nil {
			return err
		}

		err = repository.UpdateEvent(&unit.ScheduledAt, task.ID.Hex(), s.taskTextRenderer.RenderWorkUnitEventTitle(task, unit), "", s.taskTextRenderer.HasReminder(unit))
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateTaskTitle updates the events of the tasks and work units
func (s *PlanningService) UpdateTaskTitle(ctx context.Context, task *Task, updateWorkUnits bool) error {
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
	}

	task, err = s.UpdateDueAtEvent(ctx, task, relevantUsers, repositories, true, true)
	if err != nil {
		return err
	}

	if !updateWorkUnits {
		return nil
	}

	for _, unit := range task.WorkUnits {
		unitTitle := s.taskTextRenderer.RenderWorkUnitEventTitle(task, &unit)

		for _, user := range relevantUsers {
			err = repositories[user.ID.Hex()].UpdateEvent(&unit.ScheduledAt, task.ID.Hex(), unitTitle, "", s.taskTextRenderer.HasReminder(&unit))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// UpdateWorkUnitTitle updates the event title of a work unit
func (s *PlanningService) UpdateWorkUnitTitle(ctx context.Context, task *Task, unit *WorkUnit) error {
	title := s.taskTextRenderer.RenderWorkUnitEventTitle(task, unit)

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

		err = repository.UpdateEvent(&unit.ScheduledAt, task.ID.Hex(), title, "", s.taskTextRenderer.HasReminder(unit))
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
			err = repository.DeleteEvent(&unit.ScheduledAt)
			if err != nil {
				s.logger.Warning(fmt.Sprintf("failed to delete work unit %s event", unit.ID.Hex()), errors.WithStack(err))
				continue
			}
		}
	}

	for _, user := range relevantUsers {
		err = repositories[user.ID.Hex()].DeleteEvent(&task.DueAt)
		if err != nil {
			s.logger.Warning(fmt.Sprintf("failed to delete task %s event", task.ID.Hex()), errors.WithStack(err))
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

// DueDateChanged should be triggered when the due date changes. The task needs to be locked before this is called.
func (s *PlanningService) DueDateChanged(ctx context.Context, task *Task, ownerNeedsUpdate bool) (*Task, error) {
	task.DueAt.Date.End = task.DueAt.Date.Start.Add(15 * time.Minute)

	relevantUsers, err := s.getAllRelevantUsers(ctx, task)
	if err != nil {
		return nil, err
	}

	repositories := make(map[string]calendar.RepositoryInterface)

	for _, user := range relevantUsers {
		repository, err := s.calendarRepositoryManager.GetTaskCalendarRepositoryForUser(ctx, user)
		if err != nil {
			return nil, err
		}

		repositories[user.ID.Hex()] = repository
	}

	task, err = s.UpdateDueAtEvent(ctx, task, relevantUsers, repositories, true, ownerNeedsUpdate)
	if err != nil {
		return nil, err
	}

	// In case there are work units now after the deadline
	var toReschedule []WorkUnit
	for _, unit := range task.WorkUnits {
		if unit.ScheduledAt.Date.End.After(task.DueAt.Date.Start) && unit.IsDone == false {
			toReschedule = append(toReschedule, unit)
		}
	}

	for _, unit := range toReschedule {
		var err error
		task, err = s.RescheduleWorkUnit(ctx, task, &unit, false)
		if err != nil {
			return nil, err
		}
	}

	return task, nil
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
		_ = s.checkForIntersectingWorkUnits(ctx, userID, event, primitive.NilObjectID, primitive.NilObjectID)
		s.lookForUnscheduledTasks(ctx, userID)

		return
	}

	lock, err := s.locker.Acquire(ctx, task.ID.Hex(), time.Second*60, false, 62*time.Second)
	if err != nil {
		s.logger.Error(fmt.Sprintf("error acquiring lock for task %s", task.ID.Hex()), err)
		return
	}

	defer func(lock locking.LockInterface, ctx context.Context, userID string) {
		err := lock.Release(ctx)
		if err != nil {
			s.logger.Error("error releasing lock", errors.Wrap(err, "error releasing lock"))
		}

		s.lookForUnscheduledTasks(ctx, userID)
	}(lock, ctx, userID)

	// Refresh task, after potential change
	task, err = s.taskRepository.FindByCalendarEventID(ctx, calendarEvent.CalendarEventID, userID, false)
	if err != nil {
		s.logger.Info("could not find task with event id after refresh, event id has passed apparently")
		return
	}

	dueAtCalendarEvent := task.DueAt.CalendarEvents.FindByUserID(userID)
	if dueAtCalendarEvent != nil && dueAtCalendarEvent.CalendarEventID == calendarEvent.CalendarEventID {
		// If there is no change we do nothing or if the task was deleted by us (no calendar events left)
		if task.DueAt.Date == event.Date || (event.Deleted && task.DueAt.CalendarEvents.IsEmpty()) {
			return
		}

		// If the event is deleted
		if event.Deleted {
			err = s.DeleteTask(ctx, task)
			if err != nil {
				s.logger.Error(fmt.Sprintf("Error while deleting task %s", task.ID.Hex()), err)
				return
			}

			return
		}

		// If the event is not deleted, we update the task
		task.DueAt.Date = event.Date
		task, err = s.DueDateChanged(ctx, task, false)
		if err != nil {
			s.logger.Error(fmt.Sprintf("Error while updating due date event for task %s", task.ID.Hex()), err)
			return
		}

		// Update the task
		err = s.taskRepository.Update(ctx, task, false)
		if err != nil {
			s.logger.Error(fmt.Sprintf("Error while updating task %s", task.ID.Hex()), err)
			return
		}

		return
	}

	// At this point the event is not the due date event, so we check if it is a work unit event
	index, workUnit := task.WorkUnits.FindByCalendarID(calendarEvent.CalendarEventID)
	if workUnit == nil {
		s.logger.Error("event not found", errors.Errorf("could not find work unit for calendar event %s", calendarEvent.CalendarEventID))
		return
	}

	// If the work unit date is the same as the event date, we do nothing
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
			s.logger.Error("Error while updating task", err)
			return
		}

		return
	}

	// If the work unit event is not deleted, we update the work unit
	workUnit.ScheduledAt.Date = event.Date
	err = s.updateCalendarEventForOtherCollaborators(ctx, task, userID, &workUnit.ScheduledAt, s.taskTextRenderer.RenderWorkUnitEventTitle(task, workUnit), s.taskTextRenderer.HasReminder(workUnit))
	if err != nil {
		s.logger.Error(fmt.Sprintf("error updating other collaborators workUnit event %s", task.ID.Hex()), err)
		// We don't return here, because we still need to update the task
	}

	workUnitIsOutOfBounds := workUnit.ScheduledAt.Date.End.After(task.DueAt.Date.Start)

	workUnit.Workload = workUnit.ScheduledAt.Date.Duration()

	task.WorkloadOverall += workUnit.Workload

	task.WorkUnits[index] = *workUnit

	task.WorkUnits = task.WorkUnits.RemoveByIndex(index)
	task.WorkUnits = task.WorkUnits.Add(workUnit)

	task = s.CheckForMergingWorkUnits(ctx, task)

	err = s.taskRepository.Update(ctx, task, false)
	if err != nil {
		s.logger.Error("Error while updating task", err)
		return
	}

	_, workUnit = task.WorkUnits.FindByID(workUnit.ID.Hex())
	if workUnit == nil || workUnit.ScheduledAt.Date != event.Date {
		// The work unit was either merged and therefore does not exist anymore or
		// the work unit was merged and exists but now has a different date
		// We don't need to check for intersecting work units anymore
		return
	}

	if !workUnitIsOutOfBounds {
		_ = s.checkForIntersectingWorkUnits(ctx, userID, event, workUnit.ID, task.ID)
	}

	// We only want to for other unscheduled tasks here because we currently hold the lock and could cause a deadlock
	if task.NotScheduled == 0 {
		s.lookForUnscheduledTasks(ctx, userID)
	}

	// Maybe the user wanted to make place for another task, so we first accept the wrong work unit and reschedule
	// after we looked for unscheduled tasks
	if workUnitIsOutOfBounds {
		_, err = s.RescheduleWorkUnit(ctx, task, workUnit, false)
		if err != nil {
			s.logger.Error(fmt.Sprintf("Error rescheduling work unit %s", workUnit.ID.Hex()), errors.Wrap(err, "could not reschedule work unit"))
			return
		}
		return
	}
}

// CheckForMergingWorkUnits looks for work units that are scheduled right after one another and merges them
func (s *PlanningService) CheckForMergingWorkUnits(ctx context.Context, task *Task) *Task {
	lastDate := date.Timespan{}
	var relevantUsers []*users.User

	for i, unit := range task.WorkUnits {
		if (unit.ScheduledAt.Date.IntersectsWith(lastDate) || unit.ScheduledAt.Date.Start.Equal(lastDate.End)) && !unit.ScheduledAt.Date.Contains(lastDate) && !lastDate.Contains(unit.ScheduledAt.Date) {
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

				err = calendarRepository.UpdateEvent(&task.WorkUnits[i-1].ScheduledAt, task.ID.Hex(), s.taskTextRenderer.RenderWorkUnitEventTitle(task, &unit), "", s.taskTextRenderer.HasReminder(&task.WorkUnits[i-1]))
				if err != nil {
					s.logger.Error(fmt.Sprintf("could not update event for user %s in task %s", user.ID.Hex(), task.ID.Hex()), err)
					continue
				}
			}

			task.WorkUnits = task.WorkUnits.RemoveByIndex(i)
		}

		lastDate = unit.ScheduledAt.Date
	}

	return task
}

func (s *PlanningService) updateCalendarEventForOtherCollaborators(ctx context.Context, task *Task, userID string, event *calendar.Event, title string, withReminder bool) error {
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

		err = calendarRepository.UpdateEvent(event, task.ID.Hex(), title, "", withReminder)
		if err != nil {
			s.logger.Error(fmt.Sprintf("could not update event for user %s in task %s", user.ID.Hex(), task.ID.Hex()), err)
			continue
		}
	}

	return nil
}

// checkForIntersectingWorkUnits checks if the given work unit or event intersects with any other work unit
func (s *PlanningService) checkForIntersectingWorkUnits(ctx context.Context, userID string, event *calendar.Event, ignoreWorkUnitID primitive.ObjectID, lockForTaskIDHeld primitive.ObjectID) int {
	intersectingTasks, err := s.taskRepository.FindIntersectingWithEvent(ctx, userID, event, ignoreWorkUnitID, false)
	if err != nil {
		s.logger.Error("error while trying to find tasks intersecting with an event", err)
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
		_, workUnits := intersectingTask.WorkUnits.FindByEventIntersection(event, ignoreWorkUnitID)
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
			// It could be that we already have a lock for the task, because there are overlapping work units of the same task
			needsLock := true
			if intersection.Task.ID == lockForTaskIDHeld {
				needsLock = false
			}

			updatedTask, err := s.RescheduleWorkUnit(ctx, &intersection.Task, &unit, needsLock)
			if err != nil {
				s.logger.Error(fmt.Sprintf(
					"Could not reschedule work unit %s for task %s",
					intersection.IntersectingWorkUnits[i].ID.Hex(), intersection.Task.ID.Hex()), err)
				continue
			}

			intersection.Task = *updatedTask
		}
	}

	return len(intersectingTasks)
}

// lookForUnscheduledTasks looks for tasks that have unscheduled time
func (s *PlanningService) lookForUnscheduledTasks(ctx context.Context, userID string) {
	lock, err := s.locker.Acquire(ctx, fmt.Sprintf("lookForUnscheduledTasks-%s", userID), time.Minute*1, true, 2*time.Second)
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
		s.logger.Error("error while trying to find unscheduled tasks", err)
		return
	}

	for _, task := range tasks {
		_, err := s.ScheduleTask(ctx, &task, true)
		if err != nil {
			s.logger.Error(fmt.Sprintf("error scheduling task %s", task.ID.Hex()), errors.Wrap(err, "error scheduling task while looking for unscheduled tasks"))
			return
		}
	}
}

// computeAvailabilityForTimeWindow traverses the given time interval by two weeks and returns when it found enough free time or traversed the whole interval
func (s *PlanningService) computeAvailabilityForTimeWindow(target time.Time, timeToSchedule time.Duration, window *date.TimeWindow, repositories []calendar.RepositoryInterface, constraint *date.FreeConstraint, task *Task) (*date.TimeWindow, error) {
	window.PreferredNeighbors = task.WorkUnits.Timespans()

	s.generateTimespansBasedOnTargetDate(target, window, func(timespans []date.Timespan) bool {
		for _, timespan := range timespans {
			wg := errgroup.Group{}

			for _, repository := range repositories {
				repository := repository

				wg.Go(func() error {
					err := repository.AddBusyToWindow(window, timespan.Start, timespan.End)
					if err != nil {
						return errors.Wrap(err, "error while adding busy time to window")
					}

					return nil
				})
			}

			err := wg.Wait()
			if err != nil {
				s.logger.Error("error while adding busy time to window", err)
				return true
			}

			window.ComputeFree(constraint, target, timespan)

			if window.FreeDuration() >= timeToSchedule {
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
			switch user.Settings.Scheduling.TimingPreference {
			case users.TimingPreferenceEarly:
			case users.TimingPreferenceVeryEarly:
				return window.Start
			case users.TimingPreferenceLate:
			case users.TimingPreferenceVeryLate:
				return window.End
			}
			return window.End
		}

		return window.Start.Add(time.Hour * 24)
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
