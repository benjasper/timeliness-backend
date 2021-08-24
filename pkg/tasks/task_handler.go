package tasks

import (
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/timeliness-app/timeliness-backend/pkg/auth"
	"github.com/timeliness-app/timeliness-backend/pkg/communication"
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
	ResponseManager *communication.ResponseManager
}

// TaskAdd is the route for adding a task
func (handler *Handler) TaskAdd(writer http.ResponseWriter, request *http.Request) {
	task := Task{}

	err := json.NewDecoder(request.Body).Decode(&task)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err)
		return
	}

	userID, err := primitive.ObjectIDFromHex(request.Context().Value(auth.KeyUserID).(string))
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"UserID malformed", err)
		return
	}

	user, err := handler.UserRepository.FindByID(request.Context(), userID.Hex())
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not find user", err)
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

	planning, err := NewPlanningController(request.Context(), user, handler.UserRepository, handler.TaskRepository, handler.Logger)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem with calendar communication", err)
		return
	}

	scheduledTask, err := planning.ScheduleTask(&task)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem with creating calendar events", err)
		return
	}

	task = *scheduledTask

	err = handler.TaskRepository.Add(request.Context(), &task)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Persisting task in database did not work", err)
		return
	}

	handler.ResponseManager.Respond(writer, &task)
}

// TaskUpdate is the route for updating a Task
func (handler *Handler) TaskUpdate(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	taskID := mux.Vars(request)["taskID"]

	task, err := handler.TaskRepository.FindUpdatableByID(request.Context(), taskID, userID, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err)
		return
	}
	original := *task

	user, err := handler.UserRepository.FindByID(request.Context(), userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not find user", err)
		return
	}

	err = json.NewDecoder(request.Body).Decode(&task)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err)
		return
	}

	planning, err := NewPlanningController(request.Context(), user, handler.UserRepository, handler.TaskRepository, handler.Logger)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem with calendar communication", err)
		return
	}

	if original.Name != task.Name {
		err = planning.UpdateTaskTitle((*Task)(task), true)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
				"Problem updating event", err)
			return
		}
	}

	if original.WorkloadOverall != task.WorkloadOverall {
		scheduledTask, err := planning.ScheduleTask((*Task)(task))
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
				"Problem scheduling task", err)
			return
		}

		task = (*TaskUpdate)(scheduledTask)
	}

	if original.DueAt.Date != task.DueAt.Date {
		task.DueAt.Date.End = task.DueAt.Date.Start.Add(15 * time.Minute)

		err := planning.UpdateEvent((*Task)(task), &task.DueAt)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
				"Problem updating the task", err)
			return
		}

		var toReschedule []WorkUnit
		for _, unit := range task.WorkUnits {
			if unit.ScheduledAt.Date.End.After(task.DueAt.Date.Start) {
				toReschedule = append(toReschedule, unit)
			}
		}
		// TODO reschedule toReschschedule work units
	}

	if original.Priority != task.Priority {
		// TODO priority algorithm
	}

	if original.IsDone != task.IsDone {
		if task.IsDone {
			// TODO task was switched to done so we should remove all workunits left and remove the time from workload
		}
	}

	err = handler.TaskRepository.Update(request.Context(), task, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Could not persist task", err)
		return
	}

	returnTask := Task(*task)

	handler.ResponseManager.Respond(writer, &returnTask)
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

	user, err := handler.UserRepository.FindByID(request.Context(), userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not find user", err)
		return
	}

	task, err := handler.TaskRepository.FindUpdatableByID(request.Context(), taskID, userID, false)
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

	if workUnit.ScheduledAt.Date != original.ScheduledAt.Date {
		// TODO Update the event of the work unit

		workUnit.Workload = workUnit.ScheduledAt.Date.Duration()
	}

	if workUnit.ScheduledAt.Date == original.ScheduledAt.Date && workUnit.Workload != original.Workload {
		// TODO Reschedule this work unit
	}

	if workUnit.Workload != original.Workload {
		task.WorkloadOverall -= original.Workload
		task.WorkloadOverall += workUnit.Workload
	}

	if original.IsDone != workUnit.IsDone {
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
	}

	task.WorkUnits[index] = workUnit

	planning, err := NewPlanningController(request.Context(), user, handler.UserRepository, handler.TaskRepository, handler.Logger)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem with calendar communication", err)
		return
	}

	err = planning.UpdateTaskTitle((*Task)(task), false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem with calendar communication", err)
		return
	}

	err = handler.TaskRepository.Update(request.Context(), task, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Could not persist task", err)
		return
	}

	handler.ResponseManager.Respond(writer, Task(*task))
}

// TaskDelete deletes a task
func (handler *Handler) TaskDelete(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	taskID := mux.Vars(request)["taskID"]

	user, err := handler.UserRepository.FindByID(request.Context(), userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not find user", err)
		return
	}

	task, err := handler.TaskRepository.FindByID(request.Context(), taskID, userID, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err)
		return
	}

	planning, err := NewPlanningController(request.Context(), user, handler.UserRepository, handler.TaskRepository, handler.Logger)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem with calendar communication", err)
		return
	}

	err = planning.DeleteTask(&task)
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
	queryIncludeIsNotDone := request.URL.Query().Get("includeIsNotDone")

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

	includeIsNotDone := false
	if queryIncludeIsNotDone != "" {
		includeIsNotDone, err = strconv.ParseBool(queryIncludeIsNotDone)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Bad query parameter workUnit.isDone value", nil)
			return
		}
	}

	tasks, count, err := handler.TaskRepository.FindAll(request.Context(), userID, page, pageSize, filters, includeIsNotDone, includeDeleted)
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

// RescheduleWorkUnit is the endpoint implementation for rescheduling workunits
func (handler *Handler) RescheduleWorkUnit(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	taskID := mux.Vars(request)["taskID"]
	indexString := mux.Vars(request)["index"]
	index, err := strconv.Atoi(indexString)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "No int as index", err)
		return
	}

	user, err := handler.UserRepository.FindByID(request.Context(), userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not find user", err)
		return
	}

	task, err := handler.TaskRepository.FindUpdatableByID(request.Context(), taskID, userID, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err)
		return
	}

	if index > len(task.WorkUnits)-1 || index < 0 {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, fmt.Sprintf("Index %d does not exist", index), err)
		return
	}

	workUnit := task.WorkUnits[index]

	err = json.NewDecoder(request.Body).Decode(&workUnit)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err)
		return
	}

	planning, err := NewPlanningController(request.Context(), user, handler.UserRepository, handler.TaskRepository, handler.Logger)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem with calendar communication", err)
		return
	}

	task, err = planning.RescheduleWorkUnit(task, &workUnit)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Problem rescheduling the task", err)
		return
	}

	handler.ResponseManager.Respond(writer, Task(*task))
}
