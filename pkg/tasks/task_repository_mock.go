package tasks

import (
	"context"
	"github.com/pkg/errors"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

// MockTaskRepository is a task repository for testing
type MockTaskRepository struct {
	Tasks []*Task
}

// Add adds a task
func (m *MockTaskRepository) Add(_ context.Context, task *Task) error {
	task.CreatedAt = time.Now()
	task.LastModifiedAt = time.Now()
	task.ID = primitive.NewObjectID()

	for index, unit := range task.WorkUnits {
		if unit.ID.IsZero() {
			task.WorkUnits[index].ID = primitive.NewObjectID()
		}
	}

	m.Tasks = append(m.Tasks, task)
	return nil
}

// Update updates a task
func (m *MockTaskRepository) Update(_ context.Context, task *Task, deleted bool) error {
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

// FindAll finds all tasks. Filters are not yet implemented.
func (m *MockTaskRepository) FindAll(ctx context.Context, userID string, page int, pageSize int, _ []Filter, _ time.Time, _ bool) ([]Task, int, error) {
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

// FindAllByWorkUnits outputs tasks by WorkUnits and is not implemented yet
func (m *MockTaskRepository) FindAllByWorkUnits(_ context.Context, _ string, _ int, _ int, _ []Filter, _ bool, _ time.Time) ([]TaskUnwound, int, error) {
	panic("not implemented")
}

// FindByID finds a task
func (m *MockTaskRepository) FindByID(_ context.Context, taskID string, userID string, _ bool) (*Task, error) {
	taskObjectID, _ := primitive.ObjectIDFromHex(taskID)
	userObjectID, _ := primitive.ObjectIDFromHex(userID)
	for _, t := range m.Tasks {
		if t.ID == taskObjectID && (t.UserID == userObjectID || t.Collaborators.IncludesUser(userID)) {
			return t, nil
		}
	}

	return nil, errors.New("no task found")
}

// FindByCalendarEventID finds a task by its calendar event ID
func (m *MockTaskRepository) FindByCalendarEventID(ctx context.Context, calendarEventID string, userID string, isDeleted bool) (*Task, error) {
	userObjectID, _ := primitive.ObjectIDFromHex(userID)
	for _, t := range m.Tasks {
		calendarEventDue := t.DueAt.CalendarEvents.FindByCalendarID(calendarEventID)
		_, workUnit := t.WorkUnits.FindByCalendarID(calendarEventID)
		if (calendarEventDue != nil && calendarEventDue.CalendarEventID == calendarEventID || workUnit != nil) &&
			(t.UserID == userObjectID || t.Collaborators.IncludesUser(userID)) {
			return (*Task)(t), nil
		}
	}

	return nil, errors.New("no task found")
}

// Delete deletes a task
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

	if !found {
		return errors.New("no task found")
	}

	return nil
}

// FindUnscheduledTasks finds all tasks by user ID, not implemented
func (m *MockTaskRepository) FindUnscheduledTasks(ctx context.Context, userID string, page int, pageSize int) ([]Task, int, error) {
	return nil, 0, nil
}

// FindIntersectingWithEvent is not implemented yet
func (m *MockTaskRepository) FindIntersectingWithEvent(ctx context.Context, userID string, event *calendar.Event, ignoreTaskID *primitive.ObjectID, isDeleted bool) ([]Task, error) {
	return []Task{}, nil
}

// DeleteFinally is not implemented yet
func (m *MockTaskRepository) DeleteFinally(ctx context.Context, taskID string, userID string) error {
	panic("implement me")
}

// DeleteTag is not implemented yet
func (m *MockTaskRepository) DeleteTag(ctx context.Context, tagID string, userID string) error {
	panic("implement me")
}

// FindAllByDate finds all task, combining work units and due dates
func (m *MockTaskRepository) FindAllByDate(ctx context.Context, userID string, page int, pageSize int, filters []Filter, date time.Time, sort int) ([]TaskAgenda, int, error) {
	panic("not implemented")
}

// CountTasksBetween counts tasks between dates
func (m *MockTaskRepository) CountTasksBetween(ctx context.Context, userID string, from time.Time, to time.Time, isDone bool) (int64, error) {
	panic("not implemented")
}

// CountWorkUnitsBetween counts work units between dates
func (m *MockTaskRepository) CountWorkUnitsBetween(ctx context.Context, userID string, from time.Time, to time.Time, isDone bool) (int64, error) {
	panic("not implemented")
}
