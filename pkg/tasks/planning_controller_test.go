package tasks

import (
	"context"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNewPlanningController(t *testing.T) {
	type args struct {
		ctx            context.Context
		u              *users.User
		userService    users.UserRepositoryInterface
		taskRepository TaskRepositoryInterface
		logger         logger.Interface
	}
	tests := []struct {
		name    string
		args    args
		want    *PlanningController
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewPlanningController(tt.args.ctx, tt.args.u, tt.args.userService, tt.args.taskRepository, tt.args.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPlanningController() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewPlanningController() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanningController_DeleteTask(t *testing.T) {
	type fields struct {
		calendarRepository calendar.RepositoryInterface
		userRepository     users.UserRepositoryInterface
		taskRepository     TaskRepositoryInterface
		ctx                context.Context
		logger             logger.Interface
	}
	type args struct {
		task *Task
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &PlanningController{
				calendarRepository: tt.fields.calendarRepository,
				userRepository:     tt.fields.userRepository,
				taskRepository:     tt.fields.taskRepository,
				ctx:                tt.fields.ctx,
				logger:             tt.fields.logger,
			}
			if err := c.DeleteTask(tt.args.task); (err != nil) != tt.wantErr {
				t.Errorf("DeleteTask() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPlanningController_RescheduleWorkUnit(t *testing.T) {
	type fields struct {
		calendarRepository calendar.RepositoryInterface
		userRepository     users.UserRepositoryInterface
		taskRepository     TaskRepositoryInterface
		ctx                context.Context
		logger             logger.Interface
	}
	type args struct {
		t     *TaskUpdate
		w     *WorkUnit
		index int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &PlanningController{
				calendarRepository: tt.fields.calendarRepository,
				userRepository:     tt.fields.userRepository,
				taskRepository:     tt.fields.taskRepository,
				ctx:                tt.fields.ctx,
				logger:             tt.fields.logger,
			}
			if _, err := c.RescheduleWorkUnit(tt.args.t, tt.args.w); (err != nil) != tt.wantErr {
				t.Errorf("RescheduleWorkUnit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPlanningController_ScheduleTask(t *testing.T) {
	type fields struct {
		calendarRepository calendar.RepositoryInterface
		userRepository     users.UserRepositoryInterface
		taskRepository     TaskRepositoryInterface
		ctx                context.Context
		logger             logger.Interface
	}
	type args struct {
		t *Task
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &PlanningController{
				calendarRepository: tt.fields.calendarRepository,
				userRepository:     tt.fields.userRepository,
				taskRepository:     tt.fields.taskRepository,
				ctx:                tt.fields.ctx,
				logger:             tt.fields.logger,
			}
			if _, err := c.ScheduleTask(tt.args.t); (err != nil) != tt.wantErr {
				t.Errorf("ScheduleTask() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPlanningController_SuggestTimeslot(t *testing.T) {
	type fields struct {
		calendarRepository calendar.RepositoryInterface
		userRepository     users.UserRepositoryInterface
		taskRepository     TaskRepositoryInterface
		ctx                context.Context
		logger             logger.Interface
	}
	type args struct {
		window *calendar.TimeWindow
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *[]calendar.Timespan
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &PlanningController{
				calendarRepository: tt.fields.calendarRepository,
				userRepository:     tt.fields.userRepository,
				taskRepository:     tt.fields.taskRepository,
				ctx:                tt.fields.ctx,
				logger:             tt.fields.logger,
			}
			got, err := c.SuggestTimeslot(tt.args.window)
			if (err != nil) != tt.wantErr {
				t.Errorf("SuggestTimeslot() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SuggestTimeslot() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanningController_SyncCalendar(t *testing.T) {
	type fields struct {
		calendarRepository calendar.RepositoryInterface
		userRepository     users.UserRepositoryInterface
		taskRepository     TaskRepositoryInterface
		ctx                context.Context
		logger             logger.Interface
	}
	type args struct {
		user       *users.User
		calendarID string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &PlanningController{
				calendarRepository: tt.fields.calendarRepository,
				userRepository:     tt.fields.userRepository,
				taskRepository:     tt.fields.taskRepository,
				ctx:                tt.fields.ctx,
				logger:             tt.fields.logger,
			}
			if _, err := c.SyncCalendar(tt.args.user, tt.args.calendarID); (err != nil) != tt.wantErr {
				t.Errorf("SyncCalendar() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPlanningController_UpdateTaskTitle(t *testing.T) {
	type fields struct {
		calendarRepository calendar.RepositoryInterface
		userRepository     users.UserRepositoryInterface
		taskRepository     TaskRepositoryInterface
		ctx                context.Context
		logger             logger.Interface
	}
	type args struct {
		task            *Task
		updateWorkUnits bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &PlanningController{
				calendarRepository: tt.fields.calendarRepository,
				userRepository:     tt.fields.userRepository,
				taskRepository:     tt.fields.taskRepository,
				ctx:                tt.fields.ctx,
				logger:             tt.fields.logger,
			}
			if err := c.UpdateTaskTitle(tt.args.task, tt.args.updateWorkUnits); (err != nil) != tt.wantErr {
				t.Errorf("UpdateTaskTitle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPlanningController_processTaskEventChange(t *testing.T) {
	type fields struct {
		calendarRepository calendar.RepositoryInterface
		userRepository     users.UserRepositoryInterface
		taskRepository     TaskRepositoryInterface
		ctx                context.Context
		logger             logger.Interface
	}
	type args struct {
		event  *calendar.Event
		userID string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &PlanningController{
				calendarRepository: tt.fields.calendarRepository,
				userRepository:     tt.fields.userRepository,
				taskRepository:     tt.fields.taskRepository,
				ctx:                tt.fields.ctx,
				logger:             tt.fields.logger,
			}

			task, err := c.taskRepository.FindByCalendarEventID(context.Background(), tt.args.event.CalendarEventID, tt.args.userID)
			if err != nil {
				return
			}

			if !reflect.DeepEqual(task.DueAt, tt.args.event) {
				t.Errorf("dueAt event %s doesn't equal input event %s", task.DueAt.Date, tt.args.event.Date)
			}
		})
	}
}

func Test_findWorkUnitTimes(t *testing.T) {
	type args struct {
		w              *calendar.TimeWindow
		durationToFind time.Duration
	}
	tests := []struct {
		name string
		args args
		want WorkUnits
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findWorkUnitTimes(tt.args.w, tt.args.durationToFind); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findWorkUnitTimes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_renderDueEventTitle(t *testing.T) {
	type args struct {
		task *Task
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"Should include name", args{&Task{Name: "Testtask 1"}}, "Testtask 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderDueEventTitle(tt.args.task); !strings.Contains(got, tt.want) {
				t.Errorf("renderDueEventTitle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_renderWorkUnitEventTitle(t *testing.T) {
	type args struct {
		task *Task
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"Should include name", args{&Task{Name: "Testtask 1"}}, "Testtask 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderWorkUnitEventTitle(tt.args.task); !strings.Contains(got, tt.want) {
				t.Errorf("renderWorkUnitEventTitle() = %v, should include %v", got, tt.want)
			}
		})
	}
}

func Test_setupGoogleRepository(t *testing.T) {
	type args struct {
		ctx         context.Context
		u           *users.User
		userService users.UserRepositoryInterface
		logger      logger.Interface
	}
	tests := []struct {
		name    string
		args    args
		want    *calendar.GoogleCalendarRepository
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := setupGoogleRepository(tt.args.ctx, tt.args.u, tt.args.userService, tt.args.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("setupGoogleRepository() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("setupGoogleRepository() got = %v, want %v", got, tt.want)
			}
		})
	}
}
