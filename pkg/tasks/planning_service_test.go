package tasks

import (
	"context"
	"errors"
	"fmt"
	"github.com/timeliness-app/timeliness-backend/pkg/locking"
	logger "github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"testing"
	"time"
)

var location, _ = time.LoadLocation("Europe/Berlin")

var secondaryUser = users.User{
	ID: primitive.NewObjectIDFromTimestamp(time.Date(2021, 2, 1, 1, 1, 1, 1, location)),
}

var primaryUser = users.User{
	ID: primitive.NewObjectIDFromTimestamp(time.Date(2021, 1, 1, 1, 1, 1, 1, location)),
	Contacts: []users.Contact{
		{
			UserID:             secondaryUser.ID,
			ContactRequestedAt: time.Date(2021, 1, 1, 1, 1, 1, 1, location),
		},
	},
}

var userRepo = users.MockUserRepository{
	Users: []*users.User{
		&primaryUser,
		&secondaryUser,
	},
}

var log = logger.Logger{}

var locker = locking.NewLockerMemory()

func TestPlanningService_ScheduleTask(t *testing.T) {
	now = func() time.Time { return time.Date(2021, 1, 1, 12, 0, 0, 0, location) }

	taskRepo := &MockTaskRepository{Tasks: []*Task{}}

	var calendarRepositoryManager = CalendarRepositoryManager{
		userRepository: &userRepo,
		logger:         log,
		overridenRepos: make(map[string]calendar.RepositoryInterface),
	}

	calendarRepositoryManager.overridenRepos[primaryUser.ID.Hex()] = &calendar.MockCalendarRepository{Events: []*calendar.Event{}, User: &primaryUser}
	calendarRepositoryManager.overridenRepos[secondaryUser.ID.Hex()] = &calendar.MockCalendarRepository{Events: []*calendar.Event{}, User: &secondaryUser}

	service := PlanningService{
		userRepository:            &userRepo,
		taskRepository:            taskRepo,
		calendarRepositoryManager: &calendarRepositoryManager,
		logger:                    log, locker: locker, constraint: &calendar.FreeConstraint{
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
		}}

	type repoEntry struct {
		userID             primitive.ObjectID
		calendarRepository calendar.RepositoryInterface
	}

	tests := []struct {
		name                 string
		task                 Task
		calendarRepositories []repoEntry
	}{
		{
			name: "Task: 4h, lots of free time",
			task: Task{
				UserID:          primaryUser.ID,
				CreatedAt:       time.Now(),
				LastModifiedAt:  time.Now(),
				Deleted:         false,
				Name:            "Testtask",
				Description:     "",
				WorkloadOverall: time.Hour * 4,
				DueAt: calendar.Event{
					Date: calendar.Timespan{
						Start: time.Date(2021, 2, 1, 18, 0, 0, 0, location),
						End:   time.Date(2021, 2, 1, 18, 15, 0, 0, location),
					},
				},
			},
			calendarRepositories: []repoEntry{
				{
					userID:             primaryUser.ID,
					calendarRepository: &calendar.MockCalendarRepository{Events: []*calendar.Event{}, User: &primaryUser},
				},
			},
		},
		{
			name: "Task: 4h, lots of free time one collaborator",
			task: Task{
				UserID:          primaryUser.ID,
				CreatedAt:       time.Now(),
				LastModifiedAt:  time.Now(),
				Deleted:         false,
				Name:            "Testtask 2",
				Description:     "",
				WorkloadOverall: time.Hour * 4,
				DueAt: calendar.Event{
					Date: calendar.Timespan{
						Start: time.Date(2021, 2, 5, 18, 0, 0, 0, location),
						End:   time.Date(2021, 2, 1, 18, 15, 0, 0, location),
					},
				},
				Collaborators: []Collaborator{
					{
						UserID: secondaryUser.ID,
						Role:   RoleEditor,
					},
				},
			},
			calendarRepositories: []repoEntry{
				{
					userID:             primaryUser.ID,
					calendarRepository: &calendar.MockCalendarRepository{Events: []*calendar.Event{}, User: &primaryUser},
				},
				{
					userID:             secondaryUser.ID,
					calendarRepository: &calendar.MockCalendarRepository{Events: []*calendar.Event{}, User: &secondaryUser},
				},
			},
		},
		{
			name: "Task: 4h, only one fitting timeslot available",
			task: Task{
				UserID:          primaryUser.ID,
				CreatedAt:       time.Now(),
				LastModifiedAt:  time.Now(),
				Deleted:         false,
				Name:            "Testtask 2",
				Description:     "",
				WorkloadOverall: time.Hour * 4,
				DueAt: calendar.Event{
					Date: calendar.Timespan{
						Start: time.Date(2021, 1, 5, 18, 0, 0, 0, location),
						End:   time.Date(2021, 1, 5, 18, 15, 0, 0, location),
					},
				},
			},
			calendarRepositories: []repoEntry{
				{
					userID: primaryUser.ID,
					calendarRepository: &calendar.MockCalendarRepository{Events: []*calendar.Event{
						{
							Date: calendar.Timespan{
								Start: time.Date(2021, 1, 1, 9, 0, 0, 0, location),
								End:   time.Date(2021, 1, 3, 18, 0, 0, 0, location),
							},
							Blocking: true,
						},
						{
							Date: calendar.Timespan{
								Start: time.Date(2021, 1, 4, 8, 0, 0, 0, location),
								End:   time.Date(2021, 1, 4, 13, 0, 0, 0, location),
							},
							Blocking: true,
						},
						{
							Date: calendar.Timespan{
								Start: time.Date(2021, 1, 5, 8, 0, 0, 0, location),
								End:   time.Date(2021, 1, 5, 18, 0, 0, 0, location),
							},
							Blocking: true,
						},
					}, User: &primaryUser},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.TODO())
			defer cancel()

			for _, repo := range tt.calendarRepositories {
				service.calendarRepositoryManager.overridenRepos[repo.userID.Hex()] = repo.calendarRepository
			}

			err := taskRepo.Add(ctx, &tt.task)
			if err != nil {
				t.Error(err)
			}

			scheduledTask, err := service.ScheduleTask(ctx, &tt.task)
			if err != nil {
				t.Error(err)
			}

			err = testScheduledTask(scheduledTask)
			if err != nil {
				t.Error(err)
			}
		})
	}
}

func TestPlanningService_SyncCalendar(t *testing.T) {
	now = func() time.Time { return time.Date(2021, 1, 1, 12, 0, 0, 0, location) }

	taskID := primitive.NewObjectID()
	taskRepo := &MockTaskRepository{Tasks: []*Task{
		{
			ID:              taskID,
			UserID:          primaryUser.ID,
			CreatedAt:       time.Now(),
			LastModifiedAt:  time.Now(),
			Deleted:         false,
			Name:            "Testtask",
			Description:     "",
			WorkloadOverall: time.Hour * 4,
			DueAt: calendar.Event{
				Date: calendar.Timespan{
					Start: time.Date(2021, 2, 1, 18, 0, 0, 0, location),
					End:   time.Date(2021, 2, 1, 18, 15, 0, 0, location),
				},
				CalendarEvents: calendar.PersistedEvents{
					calendar.PersistedEvent{
						CalendarEventID: "test-123",
						UserID:          primaryUser.ID,
						CalendarType:    "mock_calendar",
					},
				},
			},
		},
	}}

	var calendarRepositoryManager = CalendarRepositoryManager{
		userRepository: &userRepo,
		logger:         log,
		overridenRepos: make(map[string]calendar.RepositoryInterface),
	}

	calendarRepositoryManager.overridenRepos[primaryUser.ID.Hex()] = &calendar.MockCalendarRepository{Events: []*calendar.Event{}, User: &primaryUser}
	calendarRepositoryManager.overridenRepos[secondaryUser.ID.Hex()] = &calendar.MockCalendarRepository{Events: []*calendar.Event{}, User: &secondaryUser}

	service := PlanningService{
		userRepository:            &userRepo,
		taskRepository:            taskRepo,
		calendarRepositoryManager: &calendarRepositoryManager,
		logger:                    log, locker: locker, constraint: &calendar.FreeConstraint{
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
		}}

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*30)
	defer cancel()
	_, err := service.SyncCalendar(ctx, &primaryUser, "test")
	if err != nil {
		t.Error(err)
	}
}

func testScheduledTask(task *Task) error {
	if len(task.WorkUnits) == 0 {
		return errors.New("no workunits were scheduled")
	}

	var collaborators = []primitive.ObjectID{
		task.UserID,
	}

	for _, collaborator := range task.Collaborators {
		collaborators = append(collaborators, collaborator.UserID)
	}

	for _, collaborator := range collaborators {
		event := task.DueAt.CalendarEvents.FindByUserID(collaborator.Hex())
		if event == nil {
			return errors.New("could not find scheduled event for due date for collaborator")
		}
	}

	var workload time.Duration = 0
	for _, unit := range task.WorkUnits {
		workload += unit.Workload

		if unit.ScheduledAt.Date.Start.After(task.DueAt.Date.Start) {
			return errors.New("work unit is scheduled after due date")
		}

		for _, collaborator := range collaborators {
			event := unit.ScheduledAt.CalendarEvents.FindByUserID(collaborator.Hex())
			if event == nil {
				return errors.New("could not find scheduled event for work unit for collaborator")
			}
		}
	}

	if task.WorkloadOverall != workload {
		return fmt.Errorf("only %s of %s of the workload was scheduled", workload.String(), task.WorkloadOverall.String())
	}

	return nil
}
