package tasks

import (
	"context"
	"errors"
	"github.com/timeliness-app/timeliness-backend/pkg/locking"
	logger "github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"testing"
	"time"
)

var location, _ = time.LoadLocation("Europe/Berlin")

var primaryUser = users.User{
	ID: primitive.NewObjectIDFromTimestamp(time.Date(2021, 1, 1, 1, 1, 1, 1, location)),
}

var log = logger.Logger{}

var locker = locking.NewLockerMemory()

func TestPlanningService_ScheduleTask(t *testing.T) {
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
			},
		},
	}}

	userRepo := users.MockUserRepository{
		Users: []*users.User{
			&primaryUser,
		},
	}

	calendarRepository := calendar.MockCalendarRepository{Events: []*calendar.Event{}, User: &primaryUser}

	var calendarRepositoryManager = CalendarRepositoryManager{
		userRepository: &userRepo,
		logger:         log,
		overridenRepo:  &calendarRepository,
	}

	task, err := taskRepo.FindByID(context.TODO(), taskID.Hex(), primaryUser.ID.Hex(), false)
	if err != nil {
		t.Error(err)
	}

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

	scheduledTask, err := service.ScheduleTask(context.TODO(), &task)
	if err != nil {
		t.Error(err)
	}

	err = testScheduledTask(scheduledTask)
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
		return errors.New("not all the workload was scheduled")
	}

	return nil
}
