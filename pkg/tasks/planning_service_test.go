package tasks

import (
	"context"
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
		userService    users.UserRepositoryInterface
		taskRepository TaskRepositoryInterface
		logger         logger.Interface
		cache          *UserDataCache
	}
	tests := []struct {
		name string
		args args
		want *PlanningService
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewPlanningController(tt.args.userService, tt.args.taskRepository, tt.args.logger, tt.args.cache); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewPlanningController() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanningService_DeleteTask(t *testing.T) {
	type fields struct {
		userRepository users.UserRepositoryInterface
		taskRepository TaskRepositoryInterface
		logger         logger.Interface
		constraint     *calendar.FreeConstraint
		taskMutexMap   sync.Map
		userCache      *UserDataCache
	}
	type args struct {
		ctx  context.Context
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
			c := &PlanningService{
				userRepository: tt.fields.userRepository,
				taskRepository: tt.fields.taskRepository,
				logger:         tt.fields.logger,
				constraint:     tt.fields.constraint,
				taskMutexMap:   tt.fields.taskMutexMap,
				userCache:      tt.fields.userCache,
			}
			if err := c.DeleteTask(tt.args.ctx, tt.args.task); (err != nil) != tt.wantErr {
				t.Errorf("DeleteTask() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPlanningService_RescheduleWorkUnit(t *testing.T) {
	type fields struct {
		userRepository users.UserRepositoryInterface
		taskRepository TaskRepositoryInterface
		logger         logger.Interface
		constraint     *calendar.FreeConstraint
		taskMutexMap   sync.Map
		userCache      *UserDataCache
	}
	type args struct {
		ctx context.Context
		t   *TaskUpdate
		w   *WorkUnit
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
			c := &PlanningService{
				userRepository: tt.fields.userRepository,
				taskRepository: tt.fields.taskRepository,
				logger:         tt.fields.logger,
				constraint:     tt.fields.constraint,
				taskMutexMap:   tt.fields.taskMutexMap,
				userCache:      tt.fields.userCache,
			}
			got, err := c.RescheduleWorkUnit(tt.args.ctx, tt.args.t, tt.args.w)
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

func TestPlanningService_ScheduleTask(t *testing.T) {
	type fields struct {
		userRepository users.UserRepositoryInterface
		taskRepository TaskRepositoryInterface
		logger         logger.Interface
		constraint     *calendar.FreeConstraint
		taskMutexMap   sync.Map
		userCache      *UserDataCache
	}
	type args struct {
		ctx context.Context
		t   *Task
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
			c := &PlanningService{
				userRepository: tt.fields.userRepository,
				taskRepository: tt.fields.taskRepository,
				logger:         tt.fields.logger,
				constraint:     tt.fields.constraint,
				taskMutexMap:   tt.fields.taskMutexMap,
				userCache:      tt.fields.userCache,
			}
			got, err := c.ScheduleTask(tt.args.ctx, tt.args.t)
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

func TestPlanningService_SyncCalendar(t *testing.T) {
	type fields struct {
		userRepository users.UserRepositoryInterface
		taskRepository TaskRepositoryInterface
		logger         logger.Interface
		constraint     *calendar.FreeConstraint
		taskMutexMap   sync.Map
		userCache      *UserDataCache
	}
	type args struct {
		ctx        context.Context
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
			c := &PlanningService{
				userRepository: tt.fields.userRepository,
				taskRepository: tt.fields.taskRepository,
				logger:         tt.fields.logger,
				constraint:     tt.fields.constraint,
				taskMutexMap:   tt.fields.taskMutexMap,
				userCache:      tt.fields.userCache,
			}
			got, err := c.SyncCalendar(tt.args.ctx, tt.args.user, tt.args.calendarID)
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

func TestPlanningService_UpdateEvent(t *testing.T) {
	type fields struct {
		userRepository users.UserRepositoryInterface
		taskRepository TaskRepositoryInterface
		logger         logger.Interface
		constraint     *calendar.FreeConstraint
		taskMutexMap   sync.Map
		userCache      *UserDataCache
	}
	type args struct {
		ctx   context.Context
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
			c := &PlanningService{
				userRepository: tt.fields.userRepository,
				taskRepository: tt.fields.taskRepository,
				logger:         tt.fields.logger,
				constraint:     tt.fields.constraint,
				taskMutexMap:   tt.fields.taskMutexMap,
				userCache:      tt.fields.userCache,
			}
			if err := c.UpdateEvent(tt.args.ctx, tt.args.task, tt.args.event); (err != nil) != tt.wantErr {
				t.Errorf("UpdateEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPlanningService_UpdateTaskTitle(t *testing.T) {
	type fields struct {
		userRepository users.UserRepositoryInterface
		taskRepository TaskRepositoryInterface
		logger         logger.Interface
		constraint     *calendar.FreeConstraint
		taskMutexMap   sync.Map
		userCache      *UserDataCache
	}
	type args struct {
		ctx             context.Context
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
			c := &PlanningService{
				userRepository: tt.fields.userRepository,
				taskRepository: tt.fields.taskRepository,
				logger:         tt.fields.logger,
				constraint:     tt.fields.constraint,
				taskMutexMap:   tt.fields.taskMutexMap,
				userCache:      tt.fields.userCache,
			}
			if err := c.UpdateTaskTitle(tt.args.ctx, tt.args.task, tt.args.updateWorkUnits); (err != nil) != tt.wantErr {
				t.Errorf("UpdateTaskTitle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPlanningService_checkForIntersectingWorkUnits(t *testing.T) {
	type fields struct {
		userRepository users.UserRepositoryInterface
		taskRepository TaskRepositoryInterface
		logger         logger.Interface
		constraint     *calendar.FreeConstraint
		taskMutexMap   sync.Map
		userCache      *UserDataCache
	}
	type args struct {
		ctx        context.Context
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
			c := &PlanningService{
				userRepository: tt.fields.userRepository,
				taskRepository: tt.fields.taskRepository,
				logger:         tt.fields.logger,
				constraint:     tt.fields.constraint,
				taskMutexMap:   tt.fields.taskMutexMap,
				userCache:      tt.fields.userCache,
			}
			if got := c.checkForIntersectingWorkUnits(tt.args.ctx, tt.args.userID, tt.args.event, tt.args.workUnitID); got != tt.want {
				t.Errorf("checkForIntersectingWorkUnits() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanningService_getAllRelevantUsers(t *testing.T) {
	type fields struct {
		userRepository users.UserRepositoryInterface
		taskRepository TaskRepositoryInterface
		logger         logger.Interface
		constraint     *calendar.FreeConstraint
		taskMutexMap   sync.Map
		userCache      *UserDataCache
	}
	type args struct {
		ctx  context.Context
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
			c := &PlanningService{
				userRepository: tt.fields.userRepository,
				taskRepository: tt.fields.taskRepository,
				logger:         tt.fields.logger,
				constraint:     tt.fields.constraint,
				taskMutexMap:   tt.fields.taskMutexMap,
				userCache:      tt.fields.userCache,
			}
			got, err := c.getAllRelevantUsers(tt.args.ctx, tt.args.task)
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

func TestPlanningService_getAllRelevantUsersWithOwner(t *testing.T) {
	type fields struct {
		userRepository users.UserRepositoryInterface
		taskRepository TaskRepositoryInterface
		logger         logger.Interface
		constraint     *calendar.FreeConstraint
		taskMutexMap   sync.Map
		userCache      *UserDataCache
	}
	type args struct {
		ctx                 context.Context
		task                *Task
		initializeWithOwner *users.User
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
			c := &PlanningService{
				userRepository: tt.fields.userRepository,
				taskRepository: tt.fields.taskRepository,
				logger:         tt.fields.logger,
				constraint:     tt.fields.constraint,
				taskMutexMap:   tt.fields.taskMutexMap,
				userCache:      tt.fields.userCache,
			}
			got, err := c.getAllRelevantUsersWithOwner(tt.args.ctx, tt.args.task, tt.args.initializeWithOwner)
			if (err != nil) != tt.wantErr {
				t.Errorf("getAllRelevantUsersWithOwner() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getAllRelevantUsersWithOwner() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanningService_getRepositoryForUser(t *testing.T) {
	type fields struct {
		userRepository users.UserRepositoryInterface
		taskRepository TaskRepositoryInterface
		logger         logger.Interface
		constraint     *calendar.FreeConstraint
		taskMutexMap   sync.Map
		userCache      *UserDataCache
	}
	type args struct {
		ctx context.Context
		u   *users.User
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
			c := &PlanningService{
				userRepository: tt.fields.userRepository,
				taskRepository: tt.fields.taskRepository,
				logger:         tt.fields.logger,
				constraint:     tt.fields.constraint,
				taskMutexMap:   tt.fields.taskMutexMap,
				userCache:      tt.fields.userCache,
			}
			got, err := c.getRepositoryForUser(tt.args.ctx, tt.args.u)
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

func TestPlanningService_getUserData(t *testing.T) {
	type fields struct {
		userRepository users.UserRepositoryInterface
		taskRepository TaskRepositoryInterface
		logger         logger.Interface
		constraint     *calendar.FreeConstraint
		taskMutexMap   sync.Map
		userCache      *UserDataCache
	}
	type args struct {
		ctx    context.Context
		userID string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *UserDataCacheEntry
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &PlanningService{
				userRepository: tt.fields.userRepository,
				taskRepository: tt.fields.taskRepository,
				logger:         tt.fields.logger,
				constraint:     tt.fields.constraint,
				taskMutexMap:   tt.fields.taskMutexMap,
				userCache:      tt.fields.userCache,
			}
			got, err := c.getUserData(tt.args.ctx, tt.args.userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("getUserData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getUserData() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanningService_processTaskEventChange(t *testing.T) {
	type fields struct {
		userRepository users.UserRepositoryInterface
		taskRepository TaskRepositoryInterface
		logger         logger.Interface
		constraint     *calendar.FreeConstraint
		taskMutexMap   sync.Map
		userCache      *UserDataCache
	}
	type args struct {
		ctx    context.Context
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
			/**
			c := &PlanningService{
				userRepository: tt.fields.userRepository,
				taskRepository: tt.fields.taskRepository,
				logger:         tt.fields.logger,
				constraint:     tt.fields.constraint,
				taskMutexMap:   tt.fields.taskMutexMap,
				userCache:      tt.fields.userCache,
			}
			*/
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
