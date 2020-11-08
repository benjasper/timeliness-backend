package tasks

import (
	"encoding/json"
	"github.com/benjasper/project-tasks/pkg/communication"
	"github.com/benjasper/project-tasks/pkg/logger"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
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

	userID, err := primitive.ObjectIDFromHex("5fa8158a47b7ff4422a5a407")
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
	userID := "5fa8158a47b7ff4422a5a407"
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
