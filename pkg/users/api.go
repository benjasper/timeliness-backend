package users

import (
	"encoding/json"
	"github.com/benjasper/project-tasks/pkg/communication"
	"github.com/benjasper/project-tasks/pkg/logger"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"net/http"
)

type Handler struct {
	UserService  ServiceInterface
	Logger       logger.Interface
	ErrorManager *communication.ErrorResponseManager
}

func (handler *Handler) HandleUserAdd(writer http.ResponseWriter, request *http.Request) {
	user := User{}
	body := map[string]string{}

	decoder := json.NewDecoder(request.Body)

	err := decoder.Decode(&body)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusBadRequest,
			"Wrong format", err)
		return
	}

	user.Firstname = body["firstname"]
	user.Lastname = body["lastname"]
	user.Email = body["email"]

	presentUser, err := handler.UserService.FindByEmail(request.Context(), user.Email)
	if presentUser != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusBadRequest,
			"User with email "+presentUser.Email+" already exists", err)
		return
	}

	password := body["password"]
	// TODO: Hash Password
	user.Password = password

	v := validator.New()
	err = v.Struct(user)
	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			handler.ErrorManager.RespondWithError(writer, http.StatusBadRequest, e.Error(), e)
			return
		}
	}

	err = handler.UserService.Add(request.Context(), &user)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"User couldn't be persisted in the database", err)
	}

	binary, err := json.Marshal(user)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Parsing users didn't work", err)
	}

	_, err = writer.Write(binary)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Writing response data didn't work", err)
	}
}

func (handler *Handler) HandleUserGet(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	u, err := handler.UserService.FindByID(request.Context(), vars["id"])
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusNotFound,
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
