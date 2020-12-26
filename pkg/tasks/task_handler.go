package tasks

import (
	"encoding/json"
	"errors"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
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

type Handler struct {
	TaskService  TaskServiceInterface
	UserService  *users.UserService
	Logger       logger.Interface
	ErrorManager *communication.ErrorResponseManager
}

func (handler *Handler) TaskAdd(writer http.ResponseWriter, request *http.Request) {
	task := Task{}

	err := json.NewDecoder(request.Body).Decode(&task)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err)
		return
	}

	userID, err := primitive.ObjectIDFromHex(request.Context().Value("userID").(string))
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"UserID malformed", err)
		return
	}

	task.UserID = userID

	v := validator.New()
	err = v.Struct(task)
	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			handler.ErrorManager.RespondWithError(writer, http.StatusBadRequest, e.Error(), e)
			return
		}
	}

	err = handler.TaskService.Add(request.Context(), &task)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Persisting task in database did not work", err)
		return
	}

	binary, err := json.Marshal(&task)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem while marshalling task into json", err)
		return
	}

	_, err = writer.Write(binary)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem writing response", err)
		return
	}
}

func (handler *Handler) TaskUpdate(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value("userID").(string)
	taskID := mux.Vars(request)["taskID"]

	task, err := handler.TaskService.FindUpdatableByID(request.Context(), taskID, userID)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find task", err)
		return
	}

	err = json.NewDecoder(request.Body).Decode(&task)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err)
		return
	}

	err = handler.TaskService.Update(request.Context(), taskID, userID, &task)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError, "Could not persist task", err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) GetAllTasks(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value("userID").(string)

	var page = 0
	var pageSize = 10
	var err error

	queryPage := request.URL.Query().Get("page")
	queryPageSize := request.URL.Query().Get("pageSize")

	if queryPage != "" {
		page, err = strconv.Atoi(queryPage)
		if err != nil {
			handler.ErrorManager.RespondWithError(writer, http.StatusBadRequest,
				"Bad query parameter page", err)
			return
		}
	}

	if queryPageSize != "" {
		pageSize, err = strconv.Atoi(queryPageSize)
		if err != nil {
			handler.ErrorManager.RespondWithError(writer, http.StatusBadRequest,
				"Bad query parameter pageSize", err)
			return
		}

		if pageSize > 25 {
			handler.ErrorManager.RespondWithError(writer, http.StatusBadRequest,
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

	binary, err := json.Marshal(&response)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem while marshalling tasks into json", err)
		return
	}

	_, err = writer.Write(binary)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem writing response", err)
		return
	}
}

func (handler *Handler) Suggest(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value("userID").(string)

	u, err := handler.UserService.FindByID(request.Context(), userID)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"User not found", err)
		return
	}
	window := calendar.TimeWindow{Start: time.Now(), End: time.Now().AddDate(0, 0, 8)}

	planningController, err := NewPlanningController(u, handler.UserService)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"No calendar access", err)
		return
	}

	timeslots, err := planningController.SuggestTimeslot(u, &window)
	if err != nil {
		if errors.Is(err, calendar.ErrorInvalidToken) {
			handler.ErrorManager.RespondWithError(writer, http.StatusMethodNotAllowed,
				"No Google calendar connection", err)
			return
		}
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem while marshalling tasks into json", err)
		return
	}

	binary, err := json.Marshal(&timeslots)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem while marshalling tasks into json", err)
		return
	}

	_, err = writer.Write(binary)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem writing response", err)
		return
	}
}
