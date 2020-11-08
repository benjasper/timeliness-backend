package tasks

import (
	"encoding/json"
	"github.com/benjasper/project-tasks/pkg/communication"
	"github.com/benjasper/project-tasks/pkg/logger"
	"github.com/go-playground/validator/v10"
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

	userID, err := primitive.ObjectIDFromHex("5fa7f298d797ff101643145d")
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
