package user

import (
	"encoding/json"
	"github.com/benjasper/project-tasks/pkg/logger"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"net/http"
)

type Handler struct {
	UserService ServiceInterface
	Logger      logger.Interface
}

func (handler *Handler) HandleUserAdd(writer http.ResponseWriter, request *http.Request) {
	user := User{}
	body := map[string]string{}

	decoder := json.NewDecoder(request.Body)

	err := decoder.Decode(&body)
	if err != nil {
		handler.respondWithError(writer, http.StatusBadRequest,
			"Wrong format", err)
		return
	}

	user.Firstname = body["firstName"]
	user.Lastname = body["lastName"]
	user.Email = body["email"]

	password := body["password"]
	// TODO: Hash Password
	user.Password = password

	v := validator.New()
	err = v.Struct(user)
	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			handler.respondWithError(writer, http.StatusBadRequest, e.Error(), e)
			return
		}
	}

	// TODO: Check whether email exists already

	err = handler.UserService.Add(request.Context(), &user)
	if err != nil {
		handler.respondWithError(writer, http.StatusInternalServerError,
			"User couldn't be persisted in the database", err)
	}

	binary, err := json.Marshal(user)
	if err != nil {
		handler.respondWithError(writer, http.StatusInternalServerError,
			"Parsing user didn't work", err)
	}

	_, err = writer.Write(binary)
	if err != nil {
		handler.respondWithError(writer, http.StatusInternalServerError,
			"Writing response data didn't work", err)
	}
}

func (handler *Handler) HandleUserGet(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	u, err := handler.UserService.FindByID(request.Context(), vars["id"])
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
