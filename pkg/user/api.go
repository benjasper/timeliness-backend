package user

import (
	"encoding/json"
	"github.com/benjasper/project-tasks/pkg/logger"
	"github.com/gorilla/mux"
	"net/http"
	"time"
)

type Handler struct {
	UserService ServiceInterface
	Logger      logger.Interface
}

func (handler *Handler) HandleUserAdd(writer http.ResponseWriter, request *http.Request) {
	user := User{
		Firstname:      "John",
		Lastname:       "Doe",
		Password:       "",
		Email:          "",
		CreatedAt:      time.Now(),
		LastModifiedAt: time.Now(),
	}

	err := handler.UserService.Add(&user, request.Context())
	if err != nil {
		handler.respondWithError(writer, http.StatusInternalServerError,
			"User couldn't be persisted in the database", err)
	}
}

func (handler *Handler) HandleUserGet(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	u, err := handler.UserService.FindById(vars["id"], request.Context())
	if err != nil {
		handler.respondWithError(writer, http.StatusNotFound,
			"User wasn't found", err)
		return
	}

	binary, err := json.Marshal(u)
	if err != nil {
		handler.Logger.Fatal(err)
		return
	}

	_, err = writer.Write(binary)
	if err != nil {
		handler.Logger.Fatal(err)
	}
}

func (handler *Handler) respondWithError(writer http.ResponseWriter, status int, message string, err error) {
	if status >= 500 {
		handler.Logger.Error(message, err)
	}

	writer.WriteHeader(status)
	var response = map[string]interface{}{
		"status": status,
		"error": map[string]interface{}{
			"message": message,
			"error":   err.Error(),
		},
	}
	binary, err := json.Marshal(response)
	if err != nil {
		handler.Logger.Fatal(err)
	}

	_, err = writer.Write(binary)
	if err != nil {
		handler.Logger.Fatal(err)
	}
}
