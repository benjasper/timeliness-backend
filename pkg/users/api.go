package users

import (
	"encoding/json"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/timeliness-app/timeliness-backend/pkg/auth/jwt"
	"github.com/timeliness-app/timeliness-backend/pkg/communication"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"time"
)

type Handler struct {
	UserService  ServiceInterface
	Logger       logger.Interface
	ErrorManager *communication.ErrorResponseManager
}

func (handler *Handler) UserRegister(writer http.ResponseWriter, request *http.Request) {
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

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem hashing password", err)
		return
	}
	user.Password = string(hashedPassword)

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
		return
	}

	binary, err := json.Marshal(user)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Parsing users didn't work", err)
		return
	}

	_, err = writer.Write(binary)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Writing response data didn't work", err)
		return
	}
}

func (handler *Handler) UserGet(writer http.ResponseWriter, request *http.Request) {
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
		return
	}
}

func (handler *Handler) UserLogin(writer http.ResponseWriter, request *http.Request) {
	userLogin := UserLogin{}
	err := json.NewDecoder(request.Body).Decode(&userLogin)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusBadRequest,
			"Wrong format", err)
		return
	}

	v := validator.New()
	err = v.Struct(userLogin)
	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			handler.ErrorManager.RespondWithError(writer, http.StatusBadRequest, e.Error(), e)
			return
		}
	}

	user, err := handler.UserService.FindByEmail(request.Context(), userLogin.Email)
	if err != nil || user == nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusBadRequest,
			"Wrong credentials", err)
		return
	}

	hashedPassword := []byte(user.Password)
	inputPassword := []byte(userLogin.Password)
	err = bcrypt.CompareHashAndPassword(hashedPassword, inputPassword)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusBadRequest,
			"Wrong credentials", err)
		return
	}

	claims := jwt.Claims{
		Subject:        user.ID.Hex(),
		Issuer:         "project-tasks",
		IssuedAt:       time.Now().Unix(),
		ExpirationTime: time.Now().AddDate(0, 0, 1).Unix(),
		TokenType:      "access_token",
	}
	accessToken := jwt.New(jwt.AlgHS256, claims)

	// TODO change to secret to env var
	accessTokenString, err := accessToken.Sign("secret")
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusBadRequest,
			"Problem signing access token", err)
		return
	}

	var response = map[string]interface{}{
		"result":      user,
		"accessToken": accessTokenString,
	}

	binary, err := json.Marshal(response)
	if err != nil {
		handler.Logger.Fatal(err)
		return
	}

	_, err = writer.Write(binary)
	if err != nil {
		handler.Logger.Fatal(err)
		return
	}
}

func (handler *Handler) UserRefresh(writer http.ResponseWriter, request *http.Request) {

}
