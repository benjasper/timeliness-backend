package tasks

import (
	"encoding/json"
	"github.com/benjasper/project-tasks/pkg/communication"
	"github.com/benjasper/project-tasks/pkg/logger"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"strconv"
)

type Handler struct {
	TaskService  TaskServiceInterface
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

	userID, err := primitive.ObjectIDFromHex("5faa9f5fa77b8287e3f31277")
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"UserID malformed", err)
		return
	}

	// TODO: Change to userID from Middleware(?)
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
	// TODO: Change to userID from Middleware(?)
	userID := "5faa9f5fa77b8287e3f31277"
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
	userID := "5faa9f5fa77b8287e3f31277"

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
				"Bad query parameter page", err)
			return
		}
	}

	tasks, err := handler.TaskService.FindAll(request.Context(), userID, page, pageSize)

	binary, err := json.Marshal(&tasks)
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
