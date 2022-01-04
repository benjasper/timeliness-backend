package tasks

import (
	"context"
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
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"math"
	"net/http"
	"strconv"
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
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err)
		return
	}

	task := Task(parsedTask)

	userID, err := primitive.ObjectIDFromHex(request.Context().Value(auth.KeyUserID).(string))
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"UserID malformed", err)
		return
	}

	task.UserID = userID

	v := validator.New()
	err = v.Struct(task)
	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, e.Error(), e)
			return
		}
	}

	err = task.Validate()
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "task invalid", err)
		return
	}

	err = handler.TaskRepository.Add(request.Context(), &task)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Persisting task in database did not work", err)
		return
	}

	scheduledTask, err := handler.PlanningService.ScheduleTask(request.Context(), &task, false)
	if err != nil {
		err2 := handler.TaskRepository.Delete(request.Context(), task.ID.Hex(), userID.Hex())
		if err2 != nil {
			handler.Logger.Error("Error while deleting task", err2)
			return
		}

		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem with creating calendar events", err)
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
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Invalid taskID", errors.New("Invalid taskID"))
		return
	}

	lock, err := handler.Locker.Acquire(request.Context(), taskID, time.Second*10, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			fmt.Sprintf("Could not acquire lock for %s", taskID), err)
		return
	}

	defer func(lock locking.LockInterface, ctx context.Context) {
		err := lock.Release(ctx)
		if err != nil {
			handler.Logger.Error("problem releasing lock", err)
		}
	}(lock, request.Context())

	original, err := handler.TaskRepository.FindByID(request.Context(), taskID, userID, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err)
		return
	}
	parsedTask := (TaskUpdate)(*original)

	err = json.NewDecoder(request.Body).Decode(&parsedTask)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err)
		return
	}

	task := (*Task)(&parsedTask)

	err = task.Validate()
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "task invalid", err)
		return
	}

	// If the tasks' workload was changed or if we have unscheduled time we want to schedule the task
	if original.WorkloadOverall != task.WorkloadOverall || task.NotScheduled > 0 {
		task, err = handler.PlanningService.ScheduleTask(request.Context(), task, false)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
				fmt.Sprintf("Problem scheduling task %s", taskID), err)
			return
		}
	}

	if original.DueAt.Date != task.DueAt.Date {
		task.DueAt.Date.End = task.DueAt.Date.Start.Add(15 * time.Minute)

		err = handler.PlanningService.UpdateEvent(request.Context(), task, &task.DueAt)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
				"Problem updating the task", err)
			return
		}

		// In case there are work units now after the deadline
		var toReschedule []WorkUnit
		for _, unit := range task.WorkUnits {
			if unit.ScheduledAt.Date.End.After(task.DueAt.Date.Start) && unit.IsDone == false {
				toReschedule = append(toReschedule, unit)
			}
		}

		for _, unit := range toReschedule {
			task, err = handler.PlanningService.RescheduleWorkUnit(request.Context(), task, &unit, false)
			if err != nil {
				handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
					fmt.Sprintf("Problem rescheduling work unit %s", unit.ID.Hex()), err)
				return
			}
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
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
				"Problem updating event", err)
			return
		}
	}

	err = handler.TaskRepository.Update(request.Context(), task, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Could not persist task", err)
		return
	}

	handler.ResponseManager.Respond(writer, task)
}

// WorkUnitUpdate updates a WorkUnit inside a task
func (handler *Handler) WorkUnitUpdate(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	taskID := mux.Vars(request)["taskID"]
	indexString := mux.Vars(request)["index"]
	index, err := strconv.Atoi(indexString)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "No int as index", err)
		return
	}

	isValid := primitive.IsValidObjectID(taskID)
	if !isValid {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Invalid taskID", errors.New("Invalid taskID"))
		return
	}

	lock, err := handler.Locker.Acquire(request.Context(), taskID, time.Second*10, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			fmt.Sprintf("Could not acquire lock for %s", taskID), err)
		return
	}

	defer func(lock locking.LockInterface, ctx context.Context) {
		err := lock.Release(ctx)
		if err != nil {
			handler.Logger.Error("problem releasing lock", err)
		}
	}(lock, request.Context())

	task, err := handler.TaskRepository.FindByID(request.Context(), taskID, userID, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err)
		return
	}

	if index > len(task.WorkUnits)-1 || index < 0 {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, fmt.Sprintf("Index %d does not exist", index), err)
		return
	}

	workUnit := task.WorkUnits[index]
	original := workUnit
	err = json.NewDecoder(request.Body).Decode(&workUnit)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err)
		return
	}

	if workUnit.Workload != original.Workload {
		// TODO Reschedule this work unit
		task.WorkloadOverall -= original.Workload
		task.WorkloadOverall += workUnit.Workload

		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Not supported to modify workload", errors.New("Not supported"))
		return
	}

	if workUnit.ScheduledAt.Date != original.ScheduledAt.Date {
		if original.Workload != workUnit.ScheduledAt.Date.Duration() {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Not supported to modify workload", errors.New("Not supported"))
			return
		}

		err = handler.PlanningService.UpdateEvent(request.Context(), task, &workUnit.ScheduledAt)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
				"Problem updating the task", err)
			return
		}
	}

	shouldUpdateTitle := false

	if original.IsDone != workUnit.IsDone {
		shouldUpdateTitle = true
		if !workUnit.IsDone {
			if original.IsDone {
				task.IsDone = false
			}
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
			}
		}

		err = handler.PlanningService.UpdateTaskTitle(request.Context(), task, false)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
				"Problem with calendar communication", err)
			return
		}
	}

	task.WorkUnits[index] = workUnit

	if shouldUpdateTitle {
		err = handler.PlanningService.UpdateWorkUnitTitle(request.Context(), task, &workUnit)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
				"Problem with calendar communication", err)
			return
		}
	}

	err = handler.TaskRepository.Update(request.Context(), task, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Could not persist task", err)
		return
	}

	handler.ResponseManager.Respond(writer, *task)
}

// TaskDelete deletes a task
func (handler *Handler) TaskDelete(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	taskID := mux.Vars(request)["taskID"]

	isValid := primitive.IsValidObjectID(taskID)
	if !isValid {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Invalid taskID", errors.New("Invalid taskID"))
		return
	}

	lock, err := handler.Locker.Acquire(request.Context(), taskID, time.Second*10, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			fmt.Sprintf("Could not acquire lock for %s", taskID), err)
		return
	}

	defer func(lock locking.LockInterface, ctx context.Context) {
		err := lock.Release(ctx)
		if err != nil {
			handler.Logger.Error("problem releasing lock", err)
		}
	}(lock, request.Context())

	task, err := handler.TaskRepository.FindByID(request.Context(), taskID, userID, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err)
		return
	}

	err = handler.PlanningService.DeleteTask(request.Context(), task)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not delete task events", err)
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

	includeDeleted := false
	if includeDeletedQuery != "" {
		includeDeleted, err = strconv.ParseBool(includeDeletedQuery)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Bad value for includeDeleted", err)
			return
		}
	}

	if queryPage != "" {
		page, err = strconv.Atoi(queryPage)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Bad query parameter page", err)
			return
		}
	}

	if queryPageSize != "" {
		pageSize, err = strconv.Atoi(queryPageSize)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Bad query parameter pageSize", err)
			return
		}

		if pageSize > 25 {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Page size can't be more than 25", nil)
			return
		}
	}

	var filters []Filter

	if queryDueAt != "" {
		timeValue, err := time.Parse(time.RFC3339, queryDueAt)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string", err)
			return
		}
		filters = append(filters, Filter{Field: "dueAt.date.start", Operator: "$gte", Value: timeValue})
	}

	if lastModifiedAt != "" {
		timeValue, err := time.Parse(time.RFC3339, lastModifiedAt)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string", err)
			return
		}
		filters = append(filters, Filter{Field: "lastModifiedAt", Operator: "$gte", Value: timeValue})
	}

	isDoneAndDueAt := time.Time{}
	if queryIsDoneAndDueAt != "" {
		isDoneAndDueAt, err = time.Parse(time.RFC3339, queryIsDoneAndDueAt)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Bad query parameter isDoneAndDueAt value", err)
			return
		}
	}

	tasks, count, err := handler.TaskRepository.FindAll(request.Context(), userID, page, pageSize, filters, isDoneAndDueAt, includeDeleted)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Problem in query", err)
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
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Could not find task", err)
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
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Bad value for includeDeleted", err)
			return
		}
	}

	var filters []Filter

	if queryPage != "" {
		page, err = strconv.Atoi(queryPage)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Bad query parameter page", err)
			return
		}
	}

	if queryPageSize != "" {
		pageSize, err = strconv.Atoi(queryPageSize)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Bad query parameter pageSize", err)
			return
		}

		if pageSize > 25 {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Page size can't be more than 25", nil)
			return
		}
	}

	if queryIsDoneAndScheduledAt != "" {
		isDoneAndScheduledAt, err = time.Parse(time.RFC3339, queryIsDoneAndScheduledAt)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string", err)
			return
		}
	}

	if queryWorkUnitIsDone != "" && queryIsDoneAndScheduledAt == "" {
		value := false
		value, err = strconv.ParseBool(queryWorkUnitIsDone)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Bad query parameter workUnit.isDone value", nil)
			return
		}

		filters = append(filters, Filter{Field: "workUnit.isDone", Value: value})
	}

	if lastModifiedAt != "" {
		timeValue, err := time.Parse(time.RFC3339, lastModifiedAt)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string", err)
			return
		}
		filters = append(filters, Filter{Field: "lastModifiedAt", Operator: "$gte", Value: timeValue})
	}

	tasks, count, err := handler.TaskRepository.FindAllByWorkUnits(request.Context(), userID, page, pageSize, filters, includeDeleted, isDoneAndScheduledAt)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Problem in query", err)
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
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string from", err)
		return
	}

	to := time.Time{}
	to, err = time.Parse(time.RFC3339, queryTo)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string to", err)
		return
	}

	count, err := handler.TaskRepository.CountTasksBetween(request.Context(), userID, from, to, true)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Problem with db connection", err)
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
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string from", err)
		return
	}

	to := time.Time{}
	to, err = time.Parse(time.RFC3339, queryTo)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string to", err)
		return
	}

	count, err := handler.TaskRepository.CountWorkUnitsBetween(request.Context(), userID, from, to, true)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Problem with db connection", err)
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
	date := time.Time{}
	sort := 1

	var filters []Filter

	if queryPage != "" {
		page, err = strconv.Atoi(queryPage)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Bad query parameter page", err)
			return
		}
	}

	if queryPageSize != "" {
		pageSize, err = strconv.Atoi(queryPageSize)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Bad query parameter pageSize", err)
			return
		}

		if pageSize > 25 {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Page size can't be more than 25", nil)
			return
		}
	}

	if querySort != "" {
		sort, err = strconv.Atoi(querySort)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Bad query parameter sort", err)
			return
		}
	}

	date, err = time.Parse(time.RFC3339, queryDate)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string", err)
		return
	}

	tasks, count, err := handler.TaskRepository.FindAllByDate(request.Context(), userID, page, pageSize, filters, date, sort)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Problem in query", err)
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
	indexString := mux.Vars(request)["index"]
	index, err := strconv.Atoi(indexString)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "No int as index", err)
		return
	}

	isValid := primitive.IsValidObjectID(taskID)
	if !isValid {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Invalid taskID", errors.New("Invalid taskID"))
		return
	}

	lock, err := handler.Locker.Acquire(request.Context(), taskID, time.Second*10, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			fmt.Sprintf("Could not acquire lock for %s", taskID), err)
		return
	}

	defer func(lock locking.LockInterface, ctx context.Context) {
		err := lock.Release(ctx)
		if err != nil {
			handler.Logger.Error("problem releasing lock", err)
		}
	}(lock, request.Context())

	task, err := handler.TaskRepository.FindByID(request.Context(), taskID, userID, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err)
		return
	}

	if index > len(task.WorkUnits)-1 || index < 0 {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, fmt.Sprintf("Index %d does not exist", index), err)
		return
	}

	requestBody := &OptionalTimespans{}
	err = json.NewDecoder(request.Body).Decode(&requestBody)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, fmt.Sprintf("Invalid body format"), err)
		return
	}

	workUnit := task.WorkUnits[index]
	if len(requestBody.ChosenTimespans) > 0 {
		var summedDuration time.Duration = 0
		for _, timespan := range requestBody.ChosenTimespans {
			summedDuration += timespan.Duration()
		}

		if summedDuration != workUnit.Workload {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, fmt.Sprintf("Sum of chosen timespans does not match workunit duration"), err)
			return
		}

		// Reduce the workload of the og work unit
		task.WorkloadOverall -= workUnit.Workload

		// Override the old work unit's specifications
		task.WorkUnits[index].ScheduledAt.Date = requestBody.ChosenTimespans[0]
		task.WorkUnits[index].Workload = requestBody.ChosenTimespans[0].Duration()

		// Add the new work unit's workload back in
		task.WorkloadOverall += requestBody.ChosenTimespans[0].Duration()

		err = handler.PlanningService.UpdateEvent(request.Context(), task, &task.WorkUnits[index].ScheduledAt)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Couldn't update event", err)
			return
		}

		task.WorkUnits.Sort()

		if len(requestBody.ChosenTimespans) > 1 {
			task, err = handler.PlanningService.CreateNewSpecificWorkUnits(request.Context(), task, requestBody.ChosenTimespans[1:])
			if err != nil {
				handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Couldn't create new work units", err)
				return
			}
		}

		err = handler.TaskRepository.Update(request.Context(), task, false)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Couldn't update task", err)
			return
		}
	} else {
		task, err = handler.PlanningService.RescheduleWorkUnit(request.Context(), task, &workUnit, false)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Problem rescheduling the task", err)
			return
		}
	}

	handler.ResponseManager.Respond(writer, *task)
}

// RescheduleWorkUnitGet is the endpoint for requesting time suggestions when rescheduling a work unit
func (handler *Handler) RescheduleWorkUnitGet(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	taskID := mux.Vars(request)["taskID"]
	indexString := mux.Vars(request)["index"]
	index, err := strconv.Atoi(indexString)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "No int as index", err)
		return
	}

	task, err := handler.TaskRepository.FindByID(request.Context(), taskID, userID, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err)
		return
	}

	if index > len(task.WorkUnits)-1 || index < 0 {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, fmt.Sprintf("Index %d does not exist", index), err)
		return
	}

	workUnit := task.WorkUnits[index]

	timespans, err := handler.PlanningService.SuggestTimespansForWorkUnit(request.Context(), task, &workUnit)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Problem rescheduling the task", err)
		return
	}

	handler.ResponseManager.Respond(writer, timespans)
}
