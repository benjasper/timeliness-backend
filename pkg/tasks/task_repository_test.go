package tasks

import "context"

type MockTaskRepository struct {
	Tasks []Task
}

func (m MockTaskRepository) Add(ctx context.Context, task *Task) error {
	panic("implement me")
}

func (m MockTaskRepository) Update(ctx context.Context, taskID string, userID string, task *TaskUpdate) error {
	panic("implement me")
}

func (m MockTaskRepository) FindAll(ctx context.Context, userID string, page int, pageSize int, filters []Filter) ([]Task, int, error) {
	panic("implement me")
}

func (m MockTaskRepository) FindAllByWorkUnits(ctx context.Context, userID string, page int, pageSize int, filters []Filter) ([]TaskUnwound, int, error) {
	panic("implement me")
}

func (m MockTaskRepository) FindByID(ctx context.Context, taskID string, userID string) (Task, error) {
	panic("implement me")
}

func (m MockTaskRepository) FindByCalendarEventID(ctx context.Context, calendarEventID string, userID string) (*TaskUpdate, error) {
	panic("implement me")
}

func (m MockTaskRepository) FindUpdatableByID(ctx context.Context, taskID string, userID string) (TaskUpdate, error) {
	panic("implement me")
}

func (m MockTaskRepository) Delete(ctx context.Context, taskID string, userID string) error {
	panic("implement me")
}
