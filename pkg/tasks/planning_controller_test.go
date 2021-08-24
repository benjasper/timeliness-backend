package tasks

import (
	"context"
	lru "github.com/hashicorp/golang-lru"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestNewPlanningController(t *testing.T) {
	type args struct {
		ctx            context.Context
		owner          *users.User
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
			got, err := NewPlanningController(tt.args.ctx, tt.args.owner, tt.args.userService, tt.args.taskRepository, tt.args.logger)
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
		calendarRepositories map[string]calendar.RepositoryInterface
		userRepository       users.UserRepositoryInterface
		taskRepository       TaskRepositoryInterface
		ctx                  context.Context
		logger               logger.Interface
		constraint           *calendar.FreeConstraint
		taskMutexMap         sync.Map
		owner                *users.User
		userCache            *lru.Cache
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
				calendarRepositories: tt.fields.calendarRepositories,
				userRepository:       tt.fields.userRepository,
				taskRepository:       tt.fields.taskRepository,
				ctx:                  tt.fields.ctx,
				logger:               tt.fields.logger,
				constraint:           tt.fields.constraint,
				taskMutexMap:         tt.fields.taskMutexMap,
				owner:                tt.fields.owner,
				userCache:            tt.fields.userCache,
			}
			if err := c.DeleteTask(tt.args.task); (err != nil) != tt.wantErr {
				t.Errorf("DeleteTask() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPlanningController_RescheduleWorkUnit(t *testing.T) {
	type fields struct {
		calendarRepositories map[string]calendar.RepositoryInterface
		userRepository       users.UserRepositoryInterface
		taskRepository       TaskRepositoryInterface
		ctx                  context.Context
		logger               logger.Interface
		constraint           *calendar.FreeConstraint
		taskMutexMap         sync.Map
		owner                *users.User
		userCache            *lru.Cache
	}
	type args struct {
		t *TaskUpdate
		w *WorkUnit
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *TaskUpdate
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &PlanningController{
				calendarRepositories: tt.fields.calendarRepositories,
				userRepository:       tt.fields.userRepository,
				taskRepository:       tt.fields.taskRepository,
				ctx:                  tt.fields.ctx,
				logger:               tt.fields.logger,
				constraint:           tt.fields.constraint,
				taskMutexMap:         tt.fields.taskMutexMap,
				owner:                tt.fields.owner,
				userCache:            tt.fields.userCache,
			}
			got, err := c.RescheduleWorkUnit(tt.args.t, tt.args.w)
			if (err != nil) != tt.wantErr {
				t.Errorf("RescheduleWorkUnit() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RescheduleWorkUnit() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanningController_ScheduleTask(t *testing.T) {
	type fields struct {
		calendarRepositories map[string]calendar.RepositoryInterface
		userRepository       users.UserRepositoryInterface
		taskRepository       TaskRepositoryInterface
		ctx                  context.Context
		logger               logger.Interface
		constraint           *calendar.FreeConstraint
		taskMutexMap         sync.Map
		owner                *users.User
		userCache            *lru.Cache
	}
	type args struct {
		t *Task
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *Task
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &PlanningController{
				calendarRepositories: tt.fields.calendarRepositories,
				userRepository:       tt.fields.userRepository,
				taskRepository:       tt.fields.taskRepository,
				ctx:                  tt.fields.ctx,
				logger:               tt.fields.logger,
				constraint:           tt.fields.constraint,
				taskMutexMap:         tt.fields.taskMutexMap,
				owner:                tt.fields.owner,
				userCache:            tt.fields.userCache,
			}
			got, err := c.ScheduleTask(tt.args.t)
			if (err != nil) != tt.wantErr {
				t.Errorf("ScheduleTask() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ScheduleTask() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanningController_SyncCalendar(t *testing.T) {
	type fields struct {
		calendarRepositories map[string]calendar.RepositoryInterface
		userRepository       users.UserRepositoryInterface
		taskRepository       TaskRepositoryInterface
		ctx                  context.Context
		logger               logger.Interface
		constraint           *calendar.FreeConstraint
		taskMutexMap         sync.Map
		owner                *users.User
		userCache            *lru.Cache
	}
	type args struct {
		user       *users.User
		calendarID string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *users.User
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &PlanningController{
				calendarRepositories: tt.fields.calendarRepositories,
				userRepository:       tt.fields.userRepository,
				taskRepository:       tt.fields.taskRepository,
				ctx:                  tt.fields.ctx,
				logger:               tt.fields.logger,
				constraint:           tt.fields.constraint,
				taskMutexMap:         tt.fields.taskMutexMap,
				owner:                tt.fields.owner,
				userCache:            tt.fields.userCache,
			}
			got, err := c.SyncCalendar(tt.args.user, tt.args.calendarID)
			if (err != nil) != tt.wantErr {
				t.Errorf("SyncCalendar() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SyncCalendar() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanningController_UpdateEvent(t *testing.T) {
	type fields struct {
		calendarRepositories map[string]calendar.RepositoryInterface
		userRepository       users.UserRepositoryInterface
		taskRepository       TaskRepositoryInterface
		ctx                  context.Context
		logger               logger.Interface
		constraint           *calendar.FreeConstraint
		taskMutexMap         sync.Map
		owner                *users.User
		userCache            *lru.Cache
	}
	type args struct {
		task  *Task
		event *calendar.Event
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
				calendarRepositories: tt.fields.calendarRepositories,
				userRepository:       tt.fields.userRepository,
				taskRepository:       tt.fields.taskRepository,
				ctx:                  tt.fields.ctx,
				logger:               tt.fields.logger,
				constraint:           tt.fields.constraint,
				taskMutexMap:         tt.fields.taskMutexMap,
				owner:                tt.fields.owner,
				userCache:            tt.fields.userCache,
			}
			if err := c.UpdateEvent(tt.args.task, tt.args.event); (err != nil) != tt.wantErr {
				t.Errorf("UpdateEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPlanningController_UpdateTaskTitle(t *testing.T) {
	type fields struct {
		calendarRepositories map[string]calendar.RepositoryInterface
		userRepository       users.UserRepositoryInterface
		taskRepository       TaskRepositoryInterface
		ctx                  context.Context
		logger               logger.Interface
		constraint           *calendar.FreeConstraint
		taskMutexMap         sync.Map
		owner                *users.User
		userCache            *lru.Cache
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
				calendarRepositories: tt.fields.calendarRepositories,
				userRepository:       tt.fields.userRepository,
				taskRepository:       tt.fields.taskRepository,
				ctx:                  tt.fields.ctx,
				logger:               tt.fields.logger,
				constraint:           tt.fields.constraint,
				taskMutexMap:         tt.fields.taskMutexMap,
				owner:                tt.fields.owner,
				userCache:            tt.fields.userCache,
			}
			if err := c.UpdateTaskTitle(tt.args.task, tt.args.updateWorkUnits); (err != nil) != tt.wantErr {
				t.Errorf("UpdateTaskTitle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPlanningController_checkForIntersectingWorkUnits(t *testing.T) {
	type fields struct {
		calendarRepositories map[string]calendar.RepositoryInterface
		userRepository       users.UserRepositoryInterface
		taskRepository       TaskRepositoryInterface
		ctx                  context.Context
		logger               logger.Interface
		constraint           *calendar.FreeConstraint
		taskMutexMap         sync.Map
		owner                *users.User
		userCache            *lru.Cache
	}
	type args struct {
		userID     string
		event      *calendar.Event
		workUnitID string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &PlanningController{
				calendarRepositories: tt.fields.calendarRepositories,
				userRepository:       tt.fields.userRepository,
				taskRepository:       tt.fields.taskRepository,
				ctx:                  tt.fields.ctx,
				logger:               tt.fields.logger,
				constraint:           tt.fields.constraint,
				taskMutexMap:         tt.fields.taskMutexMap,
				owner:                tt.fields.owner,
				userCache:            tt.fields.userCache,
			}
			if got := c.checkForIntersectingWorkUnits(tt.args.userID, tt.args.event, tt.args.workUnitID); got != tt.want {
				t.Errorf("checkForIntersectingWorkUnits() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanningController_getAllRelevantUsers(t *testing.T) {
	type fields struct {
		calendarRepositories map[string]calendar.RepositoryInterface
		userRepository       users.UserRepositoryInterface
		taskRepository       TaskRepositoryInterface
		ctx                  context.Context
		logger               logger.Interface
		constraint           *calendar.FreeConstraint
		taskMutexMap         sync.Map
		owner                *users.User
		userCache            *lru.Cache
	}
	type args struct {
		task *Task
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*users.User
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &PlanningController{
				calendarRepositories: tt.fields.calendarRepositories,
				userRepository:       tt.fields.userRepository,
				taskRepository:       tt.fields.taskRepository,
				ctx:                  tt.fields.ctx,
				logger:               tt.fields.logger,
				constraint:           tt.fields.constraint,
				taskMutexMap:         tt.fields.taskMutexMap,
				owner:                tt.fields.owner,
				userCache:            tt.fields.userCache,
			}
			got, err := c.getAllRelevantUsers(tt.args.task)
			if (err != nil) != tt.wantErr {
				t.Errorf("getAllRelevantUsers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getAllRelevantUsers() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanningController_getRepositoryForUser(t *testing.T) {
	type fields struct {
		calendarRepositories map[string]calendar.RepositoryInterface
		userRepository       users.UserRepositoryInterface
		taskRepository       TaskRepositoryInterface
		ctx                  context.Context
		logger               logger.Interface
		constraint           *calendar.FreeConstraint
		taskMutexMap         sync.Map
		owner                *users.User
		userCache            *lru.Cache
	}
	type args struct {
		u *users.User
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    calendar.RepositoryInterface
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &PlanningController{
				calendarRepositories: tt.fields.calendarRepositories,
				userRepository:       tt.fields.userRepository,
				taskRepository:       tt.fields.taskRepository,
				ctx:                  tt.fields.ctx,
				logger:               tt.fields.logger,
				constraint:           tt.fields.constraint,
				taskMutexMap:         tt.fields.taskMutexMap,
				owner:                tt.fields.owner,
				userCache:            tt.fields.userCache,
			}
			got, err := c.getRepositoryForUser(tt.args.u)
			if (err != nil) != tt.wantErr {
				t.Errorf("getRepositoryForUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getRepositoryForUser() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanningController_processTaskEventChange(t *testing.T) {
	type fields struct {
		calendarRepositories map[string]calendar.RepositoryInterface
		userRepository       users.UserRepositoryInterface
		taskRepository       TaskRepositoryInterface
		ctx                  context.Context
		logger               logger.Interface
		constraint           *calendar.FreeConstraint
		taskMutexMap         sync.Map
		owner                *users.User
		userCache            *lru.Cache
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
			_ = &PlanningController{
				calendarRepositories: tt.fields.calendarRepositories,
				userRepository:       tt.fields.userRepository,
				taskRepository:       tt.fields.taskRepository,
				ctx:                  tt.fields.ctx,
				logger:               tt.fields.logger,
				constraint:           tt.fields.constraint,
				taskMutexMap:         tt.fields.taskMutexMap,
				owner:                tt.fields.owner,
				userCache:            tt.fields.userCache,
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderDueEventTitle(tt.args.task); got != tt.want {
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderWorkUnitEventTitle(tt.args.task); got != tt.want {
				t.Errorf("renderWorkUnitEventTitle() = %v, want %v", got, tt.want)
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
