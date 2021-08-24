package tasks

import (
	"context"
	"errors"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type MockTaskRepository struct {
	Tasks []*Task
}

func (m *MockTaskRepository) Add(ctx context.Context, task *Task) error {
	m.Tasks = append(m.Tasks, task)
	return nil
}

func (m *MockTaskRepository) Update(ctx context.Context, task *TaskUpdate, deleted bool) error {
	taskObjectID := task.ID
	userObjectID := task.UserID
	for i, t := range m.Tasks {
		if t.ID == taskObjectID && t.UserID == userObjectID {
			m.Tasks[i] = (*Task)(task)
			return nil
		}
	}

	return nil
}

func (m *MockTaskRepository) FindAll(ctx context.Context, userID string, page int, pageSize int, filters []Filter, includeIsNotDone bool, includeDeleted bool) ([]Task, int, error) {
	userObjectID, _ := primitive.ObjectIDFromHex(userID)

	var tasks []Task

	for _, t := range m.Tasks {
		if t.UserID == userObjectID {
			tasks = append(tasks, *t)
		}
	}

	endIndex := page * pageSize

	var filteredTasks []Task

	for i, task := range tasks {
		if i < page {
			continue
		}

		if i > endIndex {
			break
		}

		filteredTasks = append(filteredTasks, task)
	}

	return tasks, len(m.Tasks), nil
}

func (m *MockTaskRepository) FindAllByWorkUnits(ctx context.Context, userID string, page int, pageSize int, filters []Filter, includeDeleted bool, isDoneAndScheduledAt time.Time) ([]TaskUnwound, int, error) {
	panic("not implemented")
}

func (m *MockTaskRepository) FindByID(ctx context.Context, taskID string, userID string, isDeleted bool) (Task, error) {
	taskObjectID, _ := primitive.ObjectIDFromHex(taskID)
	userObjectID, _ := primitive.ObjectIDFromHex(userID)
	for _, t := range m.Tasks {
		if t.ID == taskObjectID && t.UserID == userObjectID {
			return *t, nil
		}
	}

	return Task{}, errors.New("no task found")
}

func (m *MockTaskRepository) FindByCalendarEventID(ctx context.Context, calendarEventID string, userID string, isDeleted bool) (*TaskUpdate, error) {
	userObjectID, _ := primitive.ObjectIDFromHex(userID)
	for _, t := range m.Tasks {
		calendarEventDue := t.DueAt.CalendarEvents.FindByCalendarID(calendarEventID)
		_, workUnit := t.WorkUnits.FindByCalendarID(calendarEventID)
		if (calendarEventDue != nil && calendarEventDue.CalendarEventID == calendarEventID || workUnit != nil) &&
			t.UserID == userObjectID {
			return (*TaskUpdate)(t), nil
		}
	}

	return nil, errors.New("no task found")
}

func (m *MockTaskRepository) FindUpdatableByID(ctx context.Context, taskID string, userID string, isDeleted bool) (*TaskUpdate, error) {
	task, err := m.FindByID(ctx, taskID, userID, isDeleted)
	if err != nil {
		return nil, errors.New("no task found")
	}

	return (*TaskUpdate)(&task), nil
}

func (m *MockTaskRepository) Delete(ctx context.Context, taskID string, userID string) error {
	taskObjectID, _ := primitive.ObjectIDFromHex(taskID)
	userObjectID, _ := primitive.ObjectIDFromHex(userID)

	found := false
	for i, t := range m.Tasks {
		if t.ID == taskObjectID && t.UserID == userObjectID {
			found = true
			m.Tasks = append(m.Tasks[:i], m.Tasks[i+1:]...)
			break
		}
	}

	if found == false {
		return errors.New("no task found")
	}

	return nil
}

func (m *MockTaskRepository) FindIntersectingWithEvent(ctx context.Context, userID string, event *calendar.Event, ignoreWorkUnitByID string, isDeleted bool) ([]Task, error) {
	panic("implement me")
}

func (m *MockTaskRepository) DeleteFinally(ctx context.Context, taskID string, userID string) error {
	panic("implement me")
}

func (m *MockTaskRepository) DeleteTag(ctx context.Context, tagID string, userID string) error {
	panic("implement me")
}
