package tasks

import (
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/timeliness-app/timeliness-backend/pkg/auth"
	"github.com/timeliness-app/timeliness-backend/pkg/communication"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"math"
	"net/http"
	"strconv"
	"time"
)

// Handler handles all task related API calls
type Handler struct {
	TaskService     *TaskService
	UserService     *users.UserService
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

	user, err := handler.UserService.FindByID(request.Context(), userID.Hex())
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

	planning, err := NewPlanningController(request.Context(), user, handler.UserService, handler.TaskService)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem with calendar communication", err)
		return
	}

	err = planning.ScheduleNewTask(&task, user)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem with creating calendar events", err)
		return
	}

	err = handler.TaskService.Add(request.Context(), &task)
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

	task, err := handler.TaskService.FindUpdatableByID(request.Context(), taskID, userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err)
		return
	}
	original := task

	err = json.NewDecoder(request.Body).Decode(&task)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err)
		return
	}

	if original.WorkloadOverall != task.WorkloadOverall {
		// TODO Add or remove workunits and schedule those
	}

	if original.DueAt != task.DueAt {
		// TODO if workunits still fit => don't need to do anything
		// TODO if workunits or part of the don't fit => reschedule them
	}

	if original.Priority != task.Priority {
		// TODO priority algorithm
	}

	if original.IsDone != task.IsDone {
		if task.IsDone {
			// TODO task was switched to done so we should remove all workunits left and remove the time from workload
		}
	}

	err = handler.TaskService.Update(request.Context(), taskID, userID, &task)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Could not persist task", err)
		return
	}

	returnTask := Task(task)

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

	task, err := handler.TaskService.FindUpdatableByID(request.Context(), taskID, userID)
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

	if workUnit.ScheduledAt != original.ScheduledAt {
		// TODO Update the event of the work unit

		workUnit.Workload = workUnit.ScheduledAt.Date.Duration()
	}

	if workUnit.ScheduledAt == original.ScheduledAt && workUnit.Workload != original.Workload {
		// TODO Reschedule this work unit
	}

	if workUnit.Workload != original.Workload {
		task.WorkloadOverall -= original.Workload
		task.WorkloadOverall += workUnit.Workload
	}

	task.WorkUnits[index] = workUnit

	err = handler.TaskService.Update(request.Context(), taskID, userID, &task)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Could not persist task", err)
		return
	}

	handler.ResponseManager.Respond(writer, Task(task))
}

// TaskDelete deletes a task
func (handler *Handler) TaskDelete(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	taskID := mux.Vars(request)["taskID"]

	user, err := handler.UserService.FindByID(request.Context(), userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not find user", err)
		return
	}

	task, err := handler.TaskService.FindByID(request.Context(), taskID, userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err)
		return
	}

	planning, err := NewPlanningController(request.Context(), user, handler.UserService, handler.TaskService)
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

	err = handler.TaskService.Delete(request.Context(), taskID, userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not delete task", err)
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

	tasks, count, _ := handler.TaskService.FindAll(request.Context(), userID, page, pageSize)

	pages := float64(count) / float64(pageSize)

	var response = map[string]interface{}{
		"results": tasks,
		"pagination": map[string]interface{}{
			"pageCount": count,
			"pageSize":  pageSize,
			"pageIndex": page,
			"pages":     int(math.Ceil(pages)),
		},
	}

	handler.ResponseManager.Respond(writer, response)
}

// Suggest is the route for getting suggested free times
func (handler *Handler) Suggest(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)

	u, err := handler.UserService.FindByID(request.Context(), userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"User not found", err)
		return
	}

	window := calendar.TimeWindow{Start: time.Now(), End: time.Now().AddDate(0, 0, 8)}

	planningController, err := NewPlanningController(request.Context(), u, handler.UserService, handler.TaskService)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"No calendar access", err)
		return
	}

	timeslots, err := planningController.SuggestTimeslot(u, &window)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem while marshalling tasks into json", err)
		return
	}

	handler.ResponseManager.Respond(writer, &timeslots)
}
