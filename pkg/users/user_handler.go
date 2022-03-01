package users

import (
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/timeliness-app/timeliness-backend/internal/google"
	"github.com/timeliness-app/timeliness-backend/pkg/auth"
	"github.com/timeliness-app/timeliness-backend/pkg/auth/jwt"
	"github.com/timeliness-app/timeliness-backend/pkg/communication"
	"github.com/timeliness-app/timeliness-backend/pkg/date"
	"github.com/timeliness-app/timeliness-backend/pkg/email"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"
)

// Handler is the handler for user API calls
type Handler struct {
	UserRepository  UserRepositoryInterface
	Logger          logger.Interface
	ResponseManager *communication.ResponseManager
	Secret          string
	EmailService    email.Mailer
}

// UserRegister is the route for registering a user
func (handler *Handler) UserRegister(writer http.ResponseWriter, request *http.Request) {
	user := User{}
	body := map[string]interface{}{}

	decoder := json.NewDecoder(request.Body)

	err := decoder.Decode(&body)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err, request, nil)
		return
	}

	user.Firstname = body["firstname"].(string)
	user.Lastname = body["lastname"].(string)
	user.Email = body["email"].(string)
	user.Settings.Scheduling.TimeZone = "Europe/Berlin"
	user.Settings.Scheduling.BusyTimeSpacing = time.Minute * 15
	user.Settings.Scheduling.TimingPreference = TimingPreferenceEarly

	presentUser, err := handler.UserRepository.FindByEmail(request.Context(), user.Email)
	if presentUser != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusConflict, "User with email "+presentUser.Email+" already exists", err, request, nil)
		return
	}

	password := body["password"].(string)

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Error hashing password", err, request, nil)
		return
	}
	user.Password = string(hashedPassword)

	v := validator.New()
	err = v.Struct(user)
	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, e.Error(), e, request, nil)
			return
		}
	}

	user.EmailVerificationToken = uuid.New().String()

	err = handler.UserRepository.Add(request.Context(), &user)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "User couldn't be persisted in the database", err, request, nil)
		return
	}

	err = handler.EmailService.SendEmail(request.Context(), &email.Email{
		ReceiverName:    fmt.Sprintf("%s %s", user.Firstname, user.Lastname),
		ReceiverAddress: user.Email,
		Template:        "1",
		Parameters: map[string]interface{}{
			"verifyLink": fmt.Sprintf("%s/v1/auth/register/verify?token=%s", os.Getenv("BASE_URL"), user.EmailVerificationToken),
		},
	})
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Could not send registration confirmation mail", err, request, nil)
		return
	}

	err = handler.EmailService.AddToList(request.Context(), user.Email, email.AppUsersListID)
	if err != nil {
		handler.Logger.Error("Could not add user to app users list", err)
	}

	handler.generateAndRespondWithTokens(&user, request, writer)
}

// UserAddDevice upserts a DeviceToken
func (handler *Handler) UserAddDevice(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)

	body := map[string]string{}

	decoder := json.NewDecoder(request.Body)

	err := decoder.Decode(&body)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err, request, body)
		return
	}

	deviceToken := body["deviceToken"]

	if deviceToken == "" {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Must provide deviceToken", nil, request, body)
		return
	}

	u, err := handler.UserRepository.FindByID(request.Context(), userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "User wasn't found", err, request, body)
		return
	}

	found := false
	for i, token := range u.DeviceTokens {
		if token.Token == deviceToken {
			u.DeviceTokens[i].LastRegistered = time.Now()
			found = true
			break
		}
	}

	if !found {
		if len(u.DeviceTokens) >= 10 {
			handler.ResponseManager.RespondWithError(writer, http.StatusTooManyRequests, "Too many registered devices", err, request, body)
			return
		}

		u.DeviceTokens = append(u.DeviceTokens, DeviceToken{Token: deviceToken, LastRegistered: time.Now()})
	}

	err = handler.UserRepository.Update(request.Context(), u)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Could not update user", err, request, body)
		return
	}

	handler.ResponseManager.RespondWithNoContent(writer)
}

// UserRemoveDevice deletes a DeviceToken
func (handler *Handler) UserRemoveDevice(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)

	deviceToken := mux.Vars(request)["deviceToken"]

	if deviceToken == "" {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Must provide deviceToken", nil, request, nil)
		return
	}

	u, err := handler.UserRepository.FindByID(request.Context(), userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "User wasn't found", err, request, nil)
		return
	}

	found := false
	for index, token := range u.DeviceTokens {
		if token.Token == deviceToken {
			u.DeviceTokens = append(u.DeviceTokens[:index], u.DeviceTokens[index+1:]...)
			found = true
			break
		}
	}

	if !found {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "device token not registered", err, request, nil)
		return
	}

	err = handler.UserRepository.Update(request.Context(), u)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Could not update user", err, request, nil)
		return
	}

	handler.ResponseManager.RespondWithNoContent(writer)
}

// UserGet retrieves a single user
func (handler *Handler) UserGet(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)

	u, err := handler.UserRepository.FindByID(request.Context(), userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "User wasn't found", err, request, nil)
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

// UserLogin is the route for user authentication
func (handler *Handler) UserLogin(writer http.ResponseWriter, request *http.Request) {
	userLogin := UserLogin{}
	err := json.NewDecoder(request.Body).Decode(&userLogin)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err, request, nil)
		return
	}

	v := validator.New()
	err = v.Struct(userLogin)
	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, e.Error(), e, request, nil)
			return
		}
	}

	user, err := handler.UserRepository.FindByEmail(request.Context(), userLogin.Email)
	if err != nil || user == nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong credentials", err, request, nil)
		return
	}

	hashedPassword := []byte(user.Password)
	inputPassword := []byte(userLogin.Password)
	err = bcrypt.CompareHashAndPassword(hashedPassword, inputPassword)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong credentials", err, request, nil)
		return
	}

	handler.generateAndRespondWithTokens(user, request, writer)
}

// UserLoginWithGoogle is the endpoint that gets called when a user signs in with a Google token
func (handler *Handler) UserLoginWithGoogle(writer http.ResponseWriter, request *http.Request) {
	userLogin := UserLoginGoogle{}
	err := json.NewDecoder(request.Body).Decode(&userLogin)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err, request, userLogin)
		return
	}

	v := validator.New()
	err = v.Struct(userLogin)
	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, e.Error(), e, request, userLogin)
			return
		}
	}

	userInfo, err := google.GetUserInfoFromIDToken(request.Context(), userLogin.Token)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Invalid Google ID Token", err, request, userLogin)
		return
	}

	// Check if user exists, or if we have to register them
	user, err := handler.UserRepository.FindByIdentityProvider(request.Context(), userInfo.Email, userInfo.ID)
	if err != nil || user == nil {
		// Create a new user
		user = &User{}

		user.Firstname = userInfo.Firstname
		user.Lastname = userInfo.Lastname
		user.Email = userInfo.Email
		user.Settings.Scheduling.TimeZone = "Europe/Berlin"
		user.Settings.Scheduling.BusyTimeSpacing = time.Minute * 15
		user.Settings.Scheduling.TimingPreference = TimingPreferenceEarly
		user.Settings.Scheduling.MaxWorkUnitDuration = time.Hour * 4
		user.Settings.Scheduling.AllowedTimespans = make([]date.Timespan, 0)

		presentUser, err := handler.UserRepository.FindByEmail(request.Context(), user.Email)
		if presentUser != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusConflict, "User with email "+presentUser.Email+" already exists", err, request, userLogin)
			return
		}

		v := validator.New()
		err = v.Struct(user)
		if err != nil {
			for _, e := range err.(validator.ValidationErrors) {
				handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, e.Error(), e, request, userLogin)
				return
			}
		}

		if !userInfo.EmailVerified {
			user.EmailVerificationToken = uuid.New().String()
		}

		user.EmailVerified = userInfo.EmailVerified

		err = handler.UserRepository.Add(request.Context(), user)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "User couldn't be persisted in the database", err, request, userLogin)
			return
		}

		if !userInfo.EmailVerified {
			err = handler.EmailService.SendEmail(request.Context(), &email.Email{
				ReceiverName:    fmt.Sprintf("%s %s", user.Firstname, user.Lastname),
				ReceiverAddress: user.Email,
				Template:        "1",
				Parameters: map[string]interface{}{
					"verifyLink": fmt.Sprintf("%s/v1/auth/register/verify?token=%s", os.Getenv("BASE_URL"), user.EmailVerificationToken),
				},
			})
			if err != nil {
				handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Could not send registration confirmation mail", err, request, userLogin)
				return
			}
		}

		err = handler.EmailService.AddToList(request.Context(), user.Email, email.AppUsersListID)
		if err != nil {
			handler.Logger.Error("Could not add user to app users list", err)
		}

		user.GoogleCalendarConnections = append(user.GoogleCalendarConnections, GoogleCalendarConnection{
			ID:                       userInfo.ID,
			Email:                    userInfo.Email,
			IsTaskCalendarConnection: true,
			Status:                   CalendarConnectionStatusInactive,
		})
	} else {
		// Check if the user already has this Google calendar connection
		_, _, err = user.GoogleCalendarConnections.FindByConnectionID(userInfo.ID)
		if err != nil {
			// If they don't, add it
			user.GoogleCalendarConnections = append(user.GoogleCalendarConnections, GoogleCalendarConnection{
				ID:                       userInfo.ID,
				Email:                    userInfo.Email,
				IsTaskCalendarConnection: true,
				Status:                   CalendarConnectionStatusInactive,
			})
		}
	}

	handler.generateAndRespondWithTokens(user, request, writer)
}

func (handler *Handler) generateAndRespondWithTokens(user *User, request *http.Request, writer http.ResponseWriter) {
	accessClaims := jwt.Claims{
		Subject:        user.ID.Hex(),
		Issuer:         "timeliness",
		IssuedAt:       time.Now().Unix(),
		ExpirationTime: time.Now().AddDate(0, 0, 1).Unix(),
		TokenType:      jwt.TokenTypeAccess,
	}
	accessToken := jwt.New(jwt.AlgHS256, accessClaims)

	refreshTokenClaims := jwt.Claims{
		Subject:   user.ID.Hex(),
		Issuer:    "timeliness",
		IssuedAt:  time.Now().Unix(),
		TokenType: jwt.TokenTypeRefresh,
	}
	refreshToken := jwt.New(jwt.AlgHS256, refreshTokenClaims)

	accessTokenString, err := accessToken.Sign(handler.Secret)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Error signing access token", err, request, nil)
		return
	}

	refreshTokenString, err := refreshToken.Sign(handler.Secret)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Error signing refresh token", err, request, nil)
		return
	}

	user.LastRefreshAt = time.Now()
	err = handler.UserRepository.Update(request.Context(), user)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Could not update user", err, request, nil)
		return
	}

	var response = map[string]interface{}{
		"result":       user,
		"accessToken":  accessTokenString,
		"refreshToken": refreshTokenString,
	}

	handler.ResponseManager.Respond(writer, response)
}

// UserSettingsPatch updates specific values of a user
func (handler *Handler) UserSettingsPatch(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)

	user, err := handler.UserRepository.FindByID(request.Context(), userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, fmt.Sprintf("Could not find user %s", userID), err, request, nil)
		return
	}

	userSettings := user.Settings
	originalSettings := userSettings

	err = json.NewDecoder(request.Body).Decode(&userSettings)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err, request, userSettings)
		return
	}

	if userSettings.Scheduling.TimeZone != originalSettings.Scheduling.TimeZone {
		_, err := time.LoadLocation(userSettings.Scheduling.TimeZone)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, 400, fmt.Sprintf("Timezone %s does not exist", userSettings.Scheduling.TimeZone), err, request, userSettings)
			return
		}
	}

	if !reflect.DeepEqual(userSettings.Scheduling.AllowedTimespans, originalSettings.Scheduling.AllowedTimespans) {
		for _, timespan := range userSettings.Scheduling.AllowedTimespans {
			if !timespan.IsStartBeforeEnd() || timespan.Duration() == 0 {
				handler.ResponseManager.RespondWithError(writer, 400, fmt.Sprintf("Allowed Timespan %s is invalid", timespan), nil, request, userSettings)
				return
			}
		}

		userSettings.Scheduling.AllowedTimespans = date.MergeTimespans(userSettings.Scheduling.AllowedTimespans)
	}

	if userSettings.Scheduling.BusyTimeSpacing != originalSettings.Scheduling.BusyTimeSpacing {
		if userSettings.Scheduling.BusyTimeSpacing > time.Hour*2 {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, fmt.Sprintf("BusyTimeSpacing is invalid"), nil, request, userSettings)
			return
		}
	}

	if userSettings.Scheduling.TimingPreference != originalSettings.Scheduling.TimingPreference {
		if userSettings.Scheduling.TimingPreference != TimingPreferenceEarly &&
			userSettings.Scheduling.TimingPreference != TimingPreferenceVeryEarly &&
			userSettings.Scheduling.TimingPreference != TimingPreferenceLate &&
			userSettings.Scheduling.TimingPreference != TimingPreferenceVeryLate {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, fmt.Sprintf("TimingPreference is invalid"), nil, request, userSettings)
			return
		}
	}

	if userSettings.Scheduling.MaxWorkUnitDuration != originalSettings.Scheduling.MaxWorkUnitDuration {
		if userSettings.Scheduling.MaxWorkUnitDuration < time.Hour || userSettings.Scheduling.MaxWorkUnitDuration > time.Hour*8 {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, fmt.Sprintf("MaxWorkUnitDuration is invalid"), nil, request, userSettings)
			return
		}
	}

	v := validator.New()
	err = v.Struct(userSettings)
	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, e.Error(), e, request, userSettings)
			return
		}
	}

	user.Settings = userSettings
	err = handler.UserRepository.UpdateSettings(request.Context(), user)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, fmt.Sprintf("Couldn't update user settings for %s", userID), err, request, userSettings)
		return
	}

	handler.ResponseManager.Respond(writer, &user)
}

// UserRefresh refreshes a users access token with a new one by providing a refresh token
func (handler *Handler) UserRefresh(writer http.ResponseWriter, request *http.Request) {
	body := struct {
		RefreshToken string `json:"refreshToken"`
	}{}

	err := json.NewDecoder(request.Body).Decode(&body)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err, request, body)
		return
	}

	if body.RefreshToken == "" {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "No refresh refreshToken specified", err, request, body)
		return
	}

	claims := jwt.Claims{}

	refreshToken, err := jwt.Verify(body.RefreshToken, jwt.TokenTypeRefresh, handler.Secret, jwt.AlgHS256, claims)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Token invalid", err, request, body)
		return
	}

	userID := refreshToken.Payload.Subject

	u, err := handler.UserRepository.FindByID(request.Context(), userID)
	if err != nil || u.IsDeactivated {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "User not found", err, request, body)
		return
	}

	accessClaims := jwt.Claims{
		Subject:        u.ID.Hex(),
		Issuer:         "timeliness",
		IssuedAt:       time.Now().Unix(),
		ExpirationTime: time.Now().AddDate(0, 0, 1).Unix(),
		TokenType:      jwt.TokenTypeAccess,
	}
	accessToken := jwt.New(jwt.AlgHS256, accessClaims)

	accessTokenString, err := accessToken.Sign(handler.Secret)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Error signing access refreshToken", err, request, nil)
		return
	}

	var response = map[string]interface{}{
		"accessToken": accessTokenString,
	}

	handler.ResponseManager.Respond(writer, response)
}

// VerifyRegistrationGet is endpoint that gets called when the email verification link gets hit
func (handler *Handler) VerifyRegistrationGet(writer http.ResponseWriter, request *http.Request) {
	success := true
	token := request.URL.Query().Get("token")

	if strings.Trim(token, " ") == "" {
		handler.Logger.Warning("Invalid request", nil)
		success = false
	}

	usr, err := handler.UserRepository.FindByVerificationToken(request.Context(), strings.Trim(token, " "))
	if err != nil {
		handler.Logger.Warning("Invalid request", err)
		success = false
	}

	if !usr.EmailVerified && success == true {
		usr.EmailVerified = true

		err = handler.UserRepository.Update(request.Context(), usr)
		if err != nil {
			handler.Logger.Error("Error updating user", err)
			success = false
		}
	}

	http.Redirect(writer, request, fmt.Sprintf("%s/auth/verify?success=%t", os.Getenv("FRONTEND_BASE_URL"), success), http.StatusFound)
}

// NewsletterRegistration is the request body for the newsletter registration endpoint
type NewsletterRegistration struct {
	Email string `json:"email"`
}

// RegisterForNewsletter proxies a request to mailchimp and return mail chimps response
func (handler *Handler) RegisterForNewsletter(writer http.ResponseWriter, request *http.Request) {
	body := NewsletterRegistration{}

	err := json.NewDecoder(request.Body).Decode(&body)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err, request, body)
		return
	}

	if body.Email == "" {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "No email specified", err, request, body)
		return
	}

	err = handler.EmailService.AddToList(request.Context(), body.Email, email.UnconfirmedListID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Bad email", err, request, body)
		return
	}

	handler.ResponseManager.RespondWithNoContent(writer)
}
