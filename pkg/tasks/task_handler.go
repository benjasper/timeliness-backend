package tasks

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/timeliness-app/timeliness-backend/pkg/auth"
	"github.com/timeliness-app/timeliness-backend/pkg/communication"
	"github.com/timeliness-app/timeliness-backend/pkg/date"
	"github.com/timeliness-app/timeliness-backend/pkg/locking"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/sync/errgroup"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Handler handles all task related API calls
type Handler struct {
	TaskRepository  TaskRepositoryInterface
	UserRepository  users.UserRepositoryInterface
	Logger          logger.Interface
	Locker          locking.LockerInterface
	ResponseManager *communication.ResponseManager
	PlanningService *PlanningService
}

// TaskAdd is the route for adding a task
func (handler *Handler) TaskAdd(writer http.ResponseWriter, request *http.Request) {
	parsedTask := TaskUpdate{}

	err := json.NewDecoder(request.Body).Decode(&parsedTask)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err, request, parsedTask)
		return
	}

	task := Task(parsedTask)

	userID, err := primitive.ObjectIDFromHex(request.Context().Value(auth.KeyUserID).(string))
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "UserID malformed", err, request, parsedTask)
		return
	}

	task.UserID = userID

	v := validator.New()
	err = v.Struct(task)
	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, e.Error(), e, request, parsedTask)
			return
		}
	}

	err = task.Validate()
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "task invalid", err, request, parsedTask)
		return
	}

	err = handler.TaskRepository.Add(request.Context(), &task)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Persisting task in database did not work", err, request, parsedTask)
		return
	}

	scheduledTask, err := handler.PlanningService.ScheduleTask(request.Context(), &task, false)
	if err != nil {
		err2 := handler.TaskRepository.Delete(request.Context(), task.ID.Hex(), userID.Hex())
		if err2 != nil {
			handler.Logger.Error("Error while deleting task", err2)
			return
		}

		handler.ResponseManager.RespondWithErrorAndErrorType(writer, http.StatusInternalServerError, "Error while creating calendar events", err, request, communication.Calendar, parsedTask)
		return
	}

	handler.ResponseManager.Respond(writer, &scheduledTask)
}

// TaskUpdate is the route for updating a Task
func (handler *Handler) TaskUpdate(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	taskID := mux.Vars(request)["taskID"]

	isValid := primitive.IsValidObjectID(taskID)
	if !isValid {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Invalid taskID", errors.New("Invalid taskID"), request, nil)
		return
	}

	lock, err := handler.Locker.Acquire(request.Context(), taskID, time.Second*10, false, 10*time.Second)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, fmt.Sprintf("Could not acquire lock for %s", taskID), err, request, nil)
		return
	}

	defer func(lock locking.LockInterface, ctx context.Context) {
		err := lock.Release(ctx)
		if err != nil {
			handler.Logger.Error("error releasing lock", err)
		}
	}(lock, request.Context())

	original, err := handler.TaskRepository.FindByID(request.Context(), taskID, userID, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err, request, nil)
		return
	}
	parsedTask := (TaskUpdate)(*original)

	err = json.NewDecoder(request.Body).Decode(&parsedTask)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err, request, parsedTask)
		return
	}

	task := (*Task)(&parsedTask)

	err = task.Validate()
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "task invalid", err, request, parsedTask)
		return
	}

	// If the tasks' workload was changed or if we have unscheduled time we want to schedule the task
	if original.WorkloadOverall != task.WorkloadOverall || task.NotScheduled > 0 {
		task, err = handler.PlanningService.ScheduleTask(request.Context(), task, false)
		if err != nil {
			handler.ResponseManager.RespondWithErrorAndErrorType(writer, http.StatusInternalServerError, fmt.Sprintf("Error scheduling task %s", taskID), err, request, communication.Calendar, parsedTask)
			return
		}
	}

	if original.DueAt.Date != task.DueAt.Date {
		task, err = handler.PlanningService.DueDateChanged(request.Context(), task, true)
		if err != nil {
			handler.ResponseManager.RespondWithErrorAndErrorType(writer, http.StatusInternalServerError, fmt.Sprintf("Error updating due date for task %s", taskID), err, request, communication.Calendar, parsedTask)
			return
		}
	}

	if original.IsDone != task.IsDone {
		if task.IsDone {
			// TODO task was switched to done so we should remove all workunits left and remove the time from workload
		}
	}

	if original.Name != task.Name {
		err = handler.PlanningService.UpdateTaskTitle(request.Context(), task, true)
		if err != nil {
			handler.ResponseManager.RespondWithErrorAndErrorType(writer, http.StatusInternalServerError, "Error updating event", err, request, communication.Calendar, parsedTask)
			return
		}
	}

	err = handler.TaskRepository.Update(request.Context(), task, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Could not persist task", err, request, parsedTask)
		return
	}

	handler.ResponseManager.Respond(writer, task)
}

// WorkUnitUpdate updates a WorkUnit inside a task
func (handler *Handler) WorkUnitUpdate(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	taskID := mux.Vars(request)["taskID"]
	workUnitID := mux.Vars(request)["workUnitID"]

	isValid := primitive.IsValidObjectID(taskID)
	if !isValid {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Invalid taskID", errors.New("Invalid taskID"), request, nil)
		return
	}

	lock, err := handler.Locker.Acquire(request.Context(), taskID, time.Second*10, false, 2*time.Second)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, fmt.Sprintf("Could not acquire lock for %s", taskID), err, request, nil)
		return
	}

	defer func(lock locking.LockInterface, ctx context.Context) {
		err := lock.Release(ctx)
		if err != nil {
			handler.Logger.Error("error releasing lock", err)
		}
	}(lock, request.Context())

	task, err := handler.TaskRepository.FindByID(request.Context(), taskID, userID, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err, request, nil)
		return
	}

	index, _ := task.WorkUnits.FindByID(workUnitID)
	if index == -1 {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find work unit with id %s", errors.Errorf("Invalid work unit id %s", workUnitID), request, nil)
		return
	}

	workUnit := task.WorkUnits[index]
	original := workUnit
	err = json.NewDecoder(request.Body).Decode(&workUnit)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err, request, workUnit)
		return
	}

	if workUnit.Workload != original.Workload {
		// TODO Reschedule this work unit
		task.WorkloadOverall -= original.Workload
		task.WorkloadOverall += workUnit.Workload

		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Not supported to modify workload", errors.New("Not supported"), request, workUnit)
		return
	}

	if workUnit.ScheduledAt.Date != original.ScheduledAt.Date {
		if original.Workload != workUnit.ScheduledAt.Date.Duration() {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Not supported to modify scheduledAt", errors.New("Not supported"), request, workUnit)
			return
		}

		err = handler.PlanningService.UpdateWorkUnitEvent(request.Context(), task, &workUnit)
		if err != nil {
			handler.ResponseManager.RespondWithErrorAndErrorType(writer, http.StatusInternalServerError, "Error updating the task", err, request, communication.Calendar, workUnit)
			return
		}
	}

	if original.IsDone != workUnit.IsDone {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Not supported to modify isDone", errors.New("Not supported"), request, workUnit)
		return
	}

	task.WorkUnits[index] = workUnit

	err = handler.TaskRepository.Update(request.Context(), task, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Could not persist task", err, request, workUnit)
		return
	}

	handler.ResponseManager.Respond(writer, *task)
}

// MarkWorkUnitAsDoneRequest is the request body for marking a work unit as done
type MarkWorkUnitAsDoneRequest struct {
	IsDone   bool          `json:"isDone"`
	TimeLeft time.Duration `json:"timeLeft"`
}

// MarkWorkUnitAsDone marks a work unit as done
func (handler *Handler) MarkWorkUnitAsDone(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	taskID := mux.Vars(request)["taskID"]
	workUnitID := mux.Vars(request)["workUnitID"]

	isValid := primitive.IsValidObjectID(taskID)
	if !isValid {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Invalid taskID", errors.New("Invalid taskID"), request, nil)
		return
	}

	lock, err := handler.Locker.Acquire(request.Context(), taskID, time.Second*10, false, 2*time.Second)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, fmt.Sprintf("Could not acquire lock for %s", taskID), err, request, nil)
		return
	}

	defer func(lock locking.LockInterface, ctx context.Context) {
		err := lock.Release(ctx)
		if err != nil {
			handler.Logger.Error("error releasing lock", err)
		}
	}(lock, request.Context())

	task, err := handler.TaskRepository.FindByID(request.Context(), taskID, userID, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err, request, nil)
		return
	}

	index, _ := task.WorkUnits.FindByID(workUnitID)
	if index == -1 {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find work unit", errors.Errorf("Invalid work unit id %s", workUnitID), request, nil)
		return
	}

	workUnit := &task.WorkUnits[index]

	requestBody := MarkWorkUnitAsDoneRequest{
		IsDone: true,
	}

	err = json.NewDecoder(request.Body).Decode(&requestBody)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err, request, requestBody)
		return
	}

	if requestBody.TimeLeft < 0 || requestBody.TimeLeft == workUnit.Workload {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Time left must be positive and not equal to units workload", errors.New("Invalid time left"), request, requestBody)
		return
	}

	if requestBody.TimeLeft > task.WorkUnits[index].Workload {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Time left must be smaller than the scheduled time", errors.New("Invalid time left"), request, requestBody)
		return
	}

	// No need to update because, it's already set correctly
	if requestBody.IsDone == workUnit.IsDone && requestBody.TimeLeft == 0 {
		handler.ResponseManager.Respond(writer, task)
		return
	}

	workUnit.IsDone = requestBody.IsDone

	if workUnit.IsDone && requestBody.TimeLeft > 0 {
		workUnit.ScheduledAt.Date.End = workUnit.ScheduledAt.Date.End.Add(requestBody.TimeLeft * -1)
		workUnit.Workload = workUnit.ScheduledAt.Date.Duration()

		// We let the planning service create new work units at its own discretion
		task, err = handler.PlanningService.ScheduleTask(request.Context(), task, false)
		if err != nil {
			handler.ResponseManager.RespondWithErrorAndErrorType(writer, http.StatusInternalServerError, "Error scheduling the task", err, request, communication.Calendar, requestBody)
			return
		}
	}

	taskDoneChanged := false

	// Check if we should mark the task as done
	if !workUnit.IsDone {
		task.IsDone = false
		taskDoneChanged = true
	} else {
		allAreDone := true
		for _, unit := range task.WorkUnits {
			if !unit.IsDone && unit.ID != workUnit.ID {
				allAreDone = false
				break
			}
		}

		if allAreDone {
			task.IsDone = true
			taskDoneChanged = true
		}
	}

	err = handler.TaskRepository.Update(request.Context(), task, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Could not persist task", err, request, requestBody)
		return
	}

	if taskDoneChanged {
		err = handler.PlanningService.UpdateTaskTitle(request.Context(), task, false)
		if err != nil {
			handler.ResponseManager.RespondWithErrorAndErrorType(writer, http.StatusInternalServerError, "Error while communicating with calendar", err, request, communication.Calendar, requestBody)
			return
		}
	}

	err = handler.PlanningService.UpdateWorkUnitTitle(request.Context(), task, workUnit)
	if err != nil {
		handler.ResponseManager.RespondWithErrorAndErrorType(writer, http.StatusInternalServerError, "Error while communicating with calendar", err, request, communication.Calendar, requestBody)
		return
	}

	handler.ResponseManager.Respond(writer, task)
}

// TaskDelete deletes a task
func (handler *Handler) TaskDelete(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	taskID := mux.Vars(request)["taskID"]

	isValid := primitive.IsValidObjectID(taskID)
	if !isValid {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Invalid taskID", errors.New("Invalid taskID"), request, nil)
		return
	}

	lock, err := handler.Locker.Acquire(request.Context(), taskID, time.Second*10, false, 2*time.Second)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, fmt.Sprintf("Could not acquire lock for %s", taskID), err, request, nil)
		return
	}

	defer func(lock locking.LockInterface, ctx context.Context) {
		err := lock.Release(ctx)
		if err != nil {
			handler.Logger.Error("error releasing lock", err)
		}
	}(lock, request.Context())

	task, err := handler.TaskRepository.FindByID(request.Context(), taskID, userID, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err, request, nil)
		return
	}

	err = handler.PlanningService.DeleteTask(request.Context(), task)
	if err != nil {
		handler.ResponseManager.RespondWithErrorAndErrorType(writer, http.StatusInternalServerError, "Could not delete task events", err, request, communication.Calendar, nil)
		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

// GetAllTasks is the route for getting all tasks
func (handler *Handler) GetAllTasks(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)

	var page = 0
	var pageSize = 10
	var err error

	queryPage := request.URL.Query().Get("page")
	queryPageSize := request.URL.Query().Get("pageSize")
	queryDueAt := request.URL.Query().Get("dueAt.date.start")
	lastModifiedAt := request.URL.Query().Get("lastModifiedAt")
	includeDeletedQuery := request.URL.Query().Get("includeDeleted")
	queryIsDoneAndDueAt := request.URL.Query().Get("isDoneAndDueAt")
	queryIsDone := request.URL.Query().Get("isDone")

	includeDeleted := false
	if includeDeletedQuery != "" {
		includeDeleted, err = strconv.ParseBool(includeDeletedQuery)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Bad value for includeDeleted", err, request, nil)
			return
		}
	}

	if queryPage != "" {
		page, err = strconv.Atoi(queryPage)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Bad query parameter page", err, request, nil)
			return
		}
	}

	if queryPageSize != "" {
		pageSize, err = strconv.Atoi(queryPageSize)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Bad query parameter pageSize", err, request, nil)
			return
		}

		if pageSize > 25 {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Page size can't be more than 25", nil, request, nil)
			return
		}
	}

	andFilter := ConcatFilter{Operator: "$and"}
	tagsFilter, err := handler.buildTagFilter(request, writer)
	if err != nil {
		// No need to respond with an error, the error has already been handled
		return
	}

	if queryDueAt != "" {
		timeValue, err := time.Parse(time.RFC3339, queryDueAt)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string", err, request, nil)
			return
		}
		andFilter.Filters = append(andFilter.Filters, Filter{Field: "dueAt.date.start", Operator: "$gte", Value: timeValue})
	}

	if queryIsDone != "" {
		queryIsDoneParts := strings.Split(queryIsDone, ":")
		queryIsDonePart := ""
		operator := "$eq"
		if len(queryIsDoneParts) == 1 {
			queryIsDonePart = queryIsDoneParts[0]
		} else if len(queryIsDoneParts) == 2 {
			operator = queryIsDoneParts[0]
			queryIsDonePart = queryIsDoneParts[1]
		} else {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong query parameter isDone", nil, request, nil)
			return
		}

		isDone, err := strconv.ParseBool(queryIsDonePart)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Bad value for isDone", err, request, nil)
			return
		}

		andFilter.Filters = append(andFilter.Filters, Filter{Field: "isDone", Operator: operator, Value: isDone})
	}

	if lastModifiedAt != "" {
		timeValue, err := time.Parse(time.RFC3339, lastModifiedAt)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string", err, request, nil)
			return
		}
		andFilter.Filters = append(andFilter.Filters, Filter{Field: "lastModifiedAt", Operator: "$gte", Value: timeValue})
	}

	isDoneAndDueAt := time.Time{}
	if queryIsDoneAndDueAt != "" {
		isDoneAndDueAt, err = time.Parse(time.RFC3339, queryIsDoneAndDueAt)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Bad query parameter isDoneAndDueAt value", err, request, nil)
			return
		}
	}

	tasks, count, err := handler.TaskRepository.FindAll(request.Context(), userID, page, pageSize, []ConcatFilter{andFilter, tagsFilter}, isDoneAndDueAt, includeDeleted)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Error in query", err, request, nil)
		return
	}

	pages := float64(count) / float64(pageSize)

	var response = map[string]interface{}{
		"results": tasks,
		"pagination": map[string]interface{}{
			"resultCount": count,
			"pageSize":    pageSize,
			"pageIndex":   page,
			"pages":       int(math.Ceil(pages)),
		},
	}

	handler.ResponseManager.Respond(writer, response)
}

// TaskGet get a single task
func (handler *Handler) TaskGet(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	taskID := mux.Vars(request)["taskID"]

	task, err := handler.TaskRepository.FindByID(request.Context(), taskID, userID, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Could not find task", err, request, nil)
		return
	}

	handler.ResponseManager.Respond(writer, task)
}

// GetAllTasksByWorkUnits is the route for getting all tasks, but by TaskUnwound
func (handler *Handler) GetAllTasksByWorkUnits(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)

	var page = 0
	var pageSize = 10
	var err error

	queryPage := request.URL.Query().Get("page")
	queryPageSize := request.URL.Query().Get("pageSize")
	queryWorkUnitIsDone := request.URL.Query().Get("workUnit.isDone")
	lastModifiedAt := request.URL.Query().Get("lastModifiedAt")
	includeDeletedQuery := request.URL.Query().Get("includeDeleted")
	queryIsDoneAndScheduledAt := request.URL.Query().Get("isDoneAndScheduledAt")
	isDoneAndScheduledAt := time.Time{}

	includeDeleted := false
	if includeDeletedQuery != "" {
		includeDeleted, err = strconv.ParseBool(includeDeletedQuery)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Bad value for includeDeleted", err, request, nil)
			return
		}
	}

	andFilter := ConcatFilter{Operator: "$and"}
	tagsFilter, err := handler.buildTagFilter(request, writer)
	if err != nil {
		// No need to respond with an error, the error has already been handled
		return
	}

	if queryPage != "" {
		page, err = strconv.Atoi(queryPage)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Bad query parameter page", err, request, nil)
			return
		}
	}

	if queryPageSize != "" {
		pageSize, err = strconv.Atoi(queryPageSize)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Bad query parameter pageSize", err, request, nil)
			return
		}

		if pageSize > 25 {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Page size can't be more than 25", nil, request, nil)
			return
		}
	}

	if queryIsDoneAndScheduledAt != "" {
		isDoneAndScheduledAt, err = time.Parse(time.RFC3339, queryIsDoneAndScheduledAt)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string", err, request, nil)
			return
		}
	}

	if queryWorkUnitIsDone != "" && queryIsDoneAndScheduledAt == "" {
		value := false
		value, err = strconv.ParseBool(queryWorkUnitIsDone)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Bad query parameter workUnit.isDone value", nil, request, nil)
			return
		}

		andFilter.Filters = append(andFilter.Filters, Filter{Field: "workUnit.isDone", Value: value})
	}

	if lastModifiedAt != "" {
		timeValue, err := time.Parse(time.RFC3339, lastModifiedAt)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string", err, request, nil)
			return
		}
		andFilter.Filters = append(andFilter.Filters, Filter{Field: "lastModifiedAt", Operator: "$gte", Value: timeValue})
	}

	tasks, count, err := handler.TaskRepository.FindAllByWorkUnits(request.Context(), userID, page, pageSize, []ConcatFilter{andFilter, tagsFilter}, includeDeleted, isDoneAndScheduledAt)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Error in query", err, request, nil)
		return
	}

	if tasks == nil {
		tasks = make([]TaskUnwound, 0)
	}

	pages := float64(count) / float64(pageSize)

	var response = map[string]interface{}{
		"results": tasks,
		"pagination": map[string]interface{}{
			"resultCount": count,
			"pageSize":    pageSize,
			"pageIndex":   page,
			"pages":       int(math.Ceil(pages)),
		},
	}

	handler.ResponseManager.Respond(writer, response)
}

// GetTaskBetween is the endpoint for getting counts for the statistic
func (handler *Handler) GetTaskBetween(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)

	queryFrom := request.URL.Query().Get("from")
	queryTo := request.URL.Query().Get("to")

	from := time.Time{}
	from, err := time.Parse(time.RFC3339, queryFrom)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string from", err, request, nil)
		return
	}

	to := time.Time{}
	to, err = time.Parse(time.RFC3339, queryTo)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string to", err, request, nil)
		return
	}

	count, err := handler.TaskRepository.CountTasksBetween(request.Context(), userID, from, to, true)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Error while db connection", err, request, nil)
		return
	}

	response := map[string]int64{
		"count": count,
	}

	handler.ResponseManager.Respond(writer, response)
}

// GetWorkUnitsBetween is the endpoint for getting counts for the statistic
func (handler *Handler) GetWorkUnitsBetween(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)

	queryFrom := request.URL.Query().Get("from")
	queryTo := request.URL.Query().Get("to")

	from := time.Time{}
	from, err := time.Parse(time.RFC3339, queryFrom)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string from", err, request, nil)
		return
	}

	to := time.Time{}
	to, err = time.Parse(time.RFC3339, queryTo)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string to", err, request, nil)
		return
	}

	count, err := handler.TaskRepository.CountWorkUnitsBetween(request.Context(), userID, from, to, true)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Error while db connection", err, request, nil)
		return
	}

	response := map[string]int64{
		"count": count,
	}

	handler.ResponseManager.Respond(writer, response)
}

// GetTasksByAgenda is the route for the agenda view
func (handler *Handler) GetTasksByAgenda(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)

	var page = 0
	var pageSize = 10
	var err error

	queryPage := request.URL.Query().Get("page")
	queryPageSize := request.URL.Query().Get("pageSize")
	queryDate := request.URL.Query().Get("date")
	querySort := request.URL.Query().Get("sort")
	queryIsDone := request.URL.Query().Get("isDone")
	queryEventType := request.URL.Query().Get("date.type")

	d := time.Time{}
	sort := 1

	andFilter := ConcatFilter{Operator: "$and"}
	tagsFilter, err := handler.buildTagFilter(request, writer)
	if err != nil {
		// No need to respond with an error, the error has already been handled
		return
	}

	if queryIsDone != "" {
		queryIsDoneParts := strings.Split(queryIsDone, ":")
		queryIsDonePart := ""
		operator := "$eq"
		if len(queryIsDoneParts) == 1 {
			queryIsDonePart = queryIsDoneParts[0]
		} else if len(queryIsDoneParts) == 2 {
			operator = queryIsDoneParts[0]
			queryIsDonePart = queryIsDoneParts[1]
		} else {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong query parameter isDone", nil, request, nil)
			return
		}

		isDone, err := strconv.ParseBool(queryIsDonePart)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Bad value for isDone", err, request, nil)
			return
		}

		andFilter.Filters = append(andFilter.Filters, Filter{Field: "isDone", Operator: operator, Value: isDone})
	}

	if queryEventType != "" {
		operator, value, err := extractOperatorAndValue(queryEventType)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong query parameter date.type", err, request, nil)
			return
		}

		andFilter.Filters = append(andFilter.Filters, Filter{Field: "date.type", Operator: operator, Value: value})
	}

	if queryPage != "" {
		page, err = strconv.Atoi(queryPage)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Bad query parameter page", err, request, nil)
			return
		}
	}

	if queryPageSize != "" {
		pageSize, err = strconv.Atoi(queryPageSize)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Bad query parameter pageSize", err, request, nil)
			return
		}

		if pageSize > 25 {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Page size can't be more than 25", nil, request, nil)
			return
		}
	}

	if querySort != "" {
		sort, err = strconv.Atoi(querySort)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Bad query parameter sort", err, request, nil)
			return
		}
	}

	d, err = time.Parse(time.RFC3339, queryDate)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string", err, request, nil)
		return
	}

	tasks, count, err := handler.TaskRepository.FindAllByDate(request.Context(), userID, page, pageSize, []ConcatFilter{andFilter, tagsFilter}, d, sort)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Error in query", err, request, nil)
		return
	}

	if tasks == nil {
		tasks = make([]TaskAgenda, 0)
	}

	pages := float64(count) / float64(pageSize)

	var response = map[string]interface{}{
		"results": tasks,
		"pagination": map[string]interface{}{
			"resultCount": count,
			"pageSize":    pageSize,
			"pageIndex":   page,
			"pages":       int(math.Ceil(pages)),
		},
	}

	handler.ResponseManager.Respond(writer, response)
}

// OptionalTimespans is the optional body for a rescheduling post request
type OptionalTimespans struct {
	ChosenTimespans []date.Timespan `json:"chosenTimespans"`
}

// RescheduleWorkUnitPost is the endpoint implementation for rescheduling workunits
func (handler *Handler) RescheduleWorkUnitPost(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	taskID := mux.Vars(request)["taskID"]
	workUnitID := mux.Vars(request)["workUnitID"]

	isValid := primitive.IsValidObjectID(taskID)
	if !isValid {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Invalid taskID", errors.New("Invalid taskID"), request, nil)
		return
	}

	lock, err := handler.Locker.Acquire(request.Context(), taskID, time.Second*10, false, 2*time.Second)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, fmt.Sprintf("Could not acquire lock for %s", taskID), err, request, nil)
		return
	}

	defer func(lock locking.LockInterface, ctx context.Context) {
		err := lock.Release(ctx)
		if err != nil {
			handler.Logger.Error("error releasing lock", err)
		}
	}(lock, request.Context())

	task, err := handler.TaskRepository.FindByID(request.Context(), taskID, userID, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err, request, nil)
		return
	}

	index, _ := task.WorkUnits.FindByID(workUnitID)
	if index == -1 {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find work unit with id %s", errors.Errorf("Invalid work unit id %s", workUnitID), request, nil)
		return
	}

	workUnit := task.WorkUnits[index]

	requestBody := &OptionalTimespans{}
	err = json.NewDecoder(request.Body).Decode(&requestBody)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, fmt.Sprintf("Invalid body format"), err, request, requestBody)
		return
	}

	if len(requestBody.ChosenTimespans) > 0 {
		var summedDuration time.Duration = 0
		for _, timespan := range requestBody.ChosenTimespans {
			summedDuration += timespan.Duration()
		}

		if summedDuration != workUnit.Workload {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, fmt.Sprintf("Sum of chosen timespans does not match workunit duration"), err, request, requestBody)
			return
		}

		// Reduce the workload of the og work unit
		task.WorkloadOverall -= workUnit.Workload

		// Override the old work unit's specifications
		task.WorkUnits[index].ScheduledAt.Date = requestBody.ChosenTimespans[0]
		task.WorkUnits[index].Workload = requestBody.ChosenTimespans[0].Duration()

		// Add the new work unit's workload back in
		task.WorkloadOverall += requestBody.ChosenTimespans[0].Duration()

		err = handler.PlanningService.UpdateWorkUnitEvent(request.Context(), task, &task.WorkUnits[index])
		if err != nil {
			handler.ResponseManager.RespondWithErrorAndErrorType(writer, http.StatusInternalServerError, "Couldn't update event", err, request, communication.Calendar, requestBody)
			return
		}

		task.WorkUnits.Sort()

		if len(requestBody.ChosenTimespans) > 1 {
			task, err = handler.PlanningService.CreateNewSpecificWorkUnits(request.Context(), task, requestBody.ChosenTimespans[1:])
			if err != nil {
				handler.ResponseManager.RespondWithErrorAndErrorType(writer, http.StatusInternalServerError, "Couldn't create new work units", err, request, communication.Calendar, requestBody)
				return
			}
		}

		err = handler.TaskRepository.Update(request.Context(), task, false)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Couldn't update task", err, request, requestBody)
			return
		}

		task = handler.PlanningService.CheckForMergingWorkUnits(request.Context(), task)
	} else {
		task, err = handler.PlanningService.RescheduleWorkUnit(request.Context(), task, &workUnit, true, false)
		if err != nil {
			handler.ResponseManager.RespondWithErrorAndErrorType(writer, http.StatusInternalServerError, "Error rescheduling the task", err, request, communication.Calendar, requestBody)
			return
		}
	}

	handler.ResponseManager.Respond(writer, *task)
}

// GetWorkUnitCalendarData is the endpoint implementation for getting user specific calendar data
func (handler *Handler) GetWorkUnitCalendarData(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	taskID := mux.Vars(request)["taskID"]
	workUnitID := mux.Vars(request)["workUnitID"]

	isValid := primitive.IsValidObjectID(taskID)
	if !isValid {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Invalid taskID", errors.New("Invalid taskID"), request, nil)
		return
	}

	w, _ := errgroup.WithContext(request.Context())

	var user *users.User
	var task *Task

	w.Go(func() error {
		var err error
		user, err = handler.UserRepository.FindByID(request.Context(), userID)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find user", err, request, nil)
			return err
		}

		return nil
	})

	w.Go(func() error {
		var err error
		task, err = handler.TaskRepository.FindByID(request.Context(), taskID, userID, false)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err, request, nil)
			return err
		}

		return nil
	})

	err := w.Wait()
	if err != nil {
		// We already responded with an error
		return
	}

	index, workUnit := task.WorkUnits.FindByID(workUnitID)
	if index == -1 {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find work unit with id %s", errors.Errorf("Invalid work unit id %s", workUnitID), request, nil)
		return
	}

	persistedEvent := workUnit.ScheduledAt.CalendarEvents.FindByUserID(userID)
	if persistedEvent == nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find calendar event data", errors.Errorf("no calendar event for userID %s", userID), request, nil)
		return
	}

	// We need to calculate a custom id for Google Calendar
	if persistedEvent.CalendarType == calendar.PersistedCalendarTypeGoogleCalendar {
		customID, err := handler.getGoogleCalendarEventID(persistedEvent, user)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Couldn't get google calendar event id", err, request, nil)
			return
		}

		persistedEvent.CalendarEventID = customID
	}

	handler.ResponseManager.Respond(writer, persistedEvent)
}

// GetTaskDueDateCalendarData is the endpoint implementation for getting user specific calendar data
func (handler *Handler) GetTaskDueDateCalendarData(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	taskID := mux.Vars(request)["taskID"]

	isValid := primitive.IsValidObjectID(taskID)
	if !isValid {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Invalid taskID", errors.New("Invalid taskID"), request, nil)
		return
	}

	w, _ := errgroup.WithContext(request.Context())

	var user *users.User
	var task *Task

	w.Go(func() error {
		var err error
		user, err = handler.UserRepository.FindByID(request.Context(), userID)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find user", err, request, nil)
			return err
		}

		return nil
	})

	w.Go(func() error {
		var err error
		task, err = handler.TaskRepository.FindByID(request.Context(), taskID, userID, false)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err, request, nil)
			return err
		}

		return nil
	})

	err := w.Wait()
	if err != nil {
		// We already responded with an error
		return
	}

	persistedEvent := task.DueAt.CalendarEvents.FindByUserID(userID)
	if persistedEvent == nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find calendar event data", errors.Errorf("no calendar event for userID %s", userID), request, nil)
		return
	}

	// We need to calculate a custom id for Google Calendar
	if persistedEvent.CalendarType == calendar.PersistedCalendarTypeGoogleCalendar {
		customID, err := handler.getGoogleCalendarEventID(persistedEvent, user)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Couldn't get google calendar event id", err, request, nil)
			return
		}

		persistedEvent.CalendarEventID = customID
	}

	handler.ResponseManager.Respond(writer, persistedEvent)
}

func (handler *Handler) getGoogleCalendarEventID(event *calendar.PersistedEvent, user *users.User) (string, error) {
	connection, _, err := user.GoogleCalendarConnections.GetTaskCalendarConnection()
	if err != nil {
		return "", err
	}

	var calendarID string

	splitString := strings.Split(connection.TaskCalendarID, "@")
	if splitString == nil || len(splitString) != 2 {
		return "", errors.New(fmt.Sprintf("Could not split calendar id %s", connection.TaskCalendarID))
	}

	if strings.Contains(connection.TaskCalendarID, "@group.calendar.google.com") {
		calendarID = splitString[0] + "@g"
	} else if strings.Contains(connection.TaskCalendarID, "@gmail.com") {
		calendarID = splitString[0] + "@m"
	} else {
		return "", errors.New(fmt.Sprintf("Could not determine calendar id type for %s", connection.TaskCalendarID))
	}

	toEncode := fmt.Sprintf("%s %s", event.CalendarEventID, calendarID)
	result := base64.StdEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte(toEncode))

	return result, nil
}

// RescheduleWorkUnitGet is the endpoint for requesting time suggestions when rescheduling a work unit
func (handler *Handler) RescheduleWorkUnitGet(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	taskID := mux.Vars(request)["taskID"]
	workUnitID := mux.Vars(request)["workUnitID"]

	var body struct {
		IgnoreTimespans []date.Timespan `json:"ignoreTimespans"`
	}

	requestBody, _ := ioutil.ReadAll(request.Body)

	if len(requestBody) > 0 {
		err := json.Unmarshal(requestBody, &body)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, fmt.Sprintf("Invalid body format"), err, request, requestBody)
			return
		}
	}

	task, err := handler.TaskRepository.FindByID(request.Context(), taskID, userID, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err, request, nil)
		return
	}

	index, workUnit := task.WorkUnits.FindByID(workUnitID)
	if index == -1 {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find work unit with id %s", errors.Errorf("Invalid work unit id %s", workUnitID), request, nil)
		return
	}

	timespans, err := handler.PlanningService.SuggestTimespansForWorkUnit(request.Context(), task, workUnit, body.IgnoreTimespans)
	if err != nil {
		handler.ResponseManager.RespondWithErrorAndErrorType(writer, http.StatusInternalServerError, "Error rescheduling the task", err, request, communication.Calendar, nil)
		return
	}

	handler.ResponseManager.Respond(writer, timespans)
}

func (handler *Handler) buildTagFilter(request *http.Request, writer http.ResponseWriter) (ConcatFilter, error) {
	// query filters in format of "operator:value,operator:value" or "value,value"
	tagsFilter := ConcatFilter{Operator: "$or"}
	queryTags := request.URL.Query().Get("tags")

	if queryTags != "" {
		for _, tagFilter := range strings.Split(queryTags, ",") {
			queryTagsParts := strings.Split(tagFilter, ":")
			queryTagID := ""
			operator := "$eq"
			if len(queryTagsParts) == 1 {
				queryTagID = queryTagsParts[0]
			} else if len(queryTagsParts) == 2 {
				operator = queryTagsParts[0]
				queryTagID = queryTagsParts[1]
			} else {
				var err = errors.New(fmt.Sprintf("Invalid tag filter %s", tagFilter))
				handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong query parameter tags", err, request, nil)
				return ConcatFilter{}, err
			}

			tagObjectID, err := primitive.ObjectIDFromHex(queryTagID)
			if err != nil {
				var err = errors.New(fmt.Sprintf("Invalid tag id %s", queryTagID))
				handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong query parameter tags", err, request, nil)
				return ConcatFilter{}, err
			}

			tagsFilter.Filters = append(tagsFilter.Filters, Filter{Field: "tags", Operator: operator, Value: tagObjectID})
		}
	}

	return tagsFilter, nil
}

func extractOperatorAndValue(filter string) (string, string, error) {
	parts := strings.Split(filter, ":")
	part := ""
	operator := "$eq"

	if len(parts) == 1 {
		part = parts[0]
	} else if len(parts) == 2 {
		operator = parts[0]
		part = parts[1]
	} else {
		return "", "", errors.Errorf("Invalid filter %s", filter)
	}

	return operator, part, nil
}
