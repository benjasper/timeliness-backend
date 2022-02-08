package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/timeliness-app/timeliness-backend/internal/google"
	"github.com/timeliness-app/timeliness-backend/pkg/auth"
	"github.com/timeliness-app/timeliness-backend/pkg/auth/encryption"
	"github.com/timeliness-app/timeliness-backend/pkg/communication"
	"github.com/timeliness-app/timeliness-backend/pkg/locking"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"math"
	"net/http"
	"os"
	"time"
)

// CalendarHandler handles all calendar related API calls
type CalendarHandler struct {
	UserRepository            users.UserRepositoryInterface
	TaskRepository            *MongoDBTaskRepository
	Logger                    logger.Interface
	ResponseManager           *communication.ResponseManager
	PlanningService           *PlanningService
	Locker                    locking.LockerInterface
	CalendarRepositoryManager *CalendarRepositoryManager
}

// GoogleConnectionWithCalendars is the type the calendar handler works with
type GoogleConnectionWithCalendars struct {
	Connection users.GoogleCalendarConnection `json:"connection"`
	Calendars  []*calendar.Calendar           `json:"calendars"`
}

// GetCalendarsFromConnection responds with all calendars the user can register for busy time information
func (handler *CalendarHandler) GetCalendarsFromConnection(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	connectionID := mux.Vars(request)["connectionID"]

	u, err := handler.UserRepository.FindByID(request.Context(), userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not find user", err)
		return
	}

	var googleConnections GoogleConnectionWithCalendars

	// TODO: check which sources have a connection
	for _, connection := range u.GoogleCalendarConnections {
		if connection.ID != connectionID {
			continue
		}

		googleRepo, err := handler.CalendarRepositoryManager.GetCalendarRepositoryForUserByConnectionID(request.Context(), u, connection.ID)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
				"Error while using Google Calendar connection", err)
			return
		}

		googleCalendarMap, err := googleRepo.GetAllCalendarsOfInterest()
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
				"Could not retrieve Google Calendar calendars", err)
			return
		}

		for _, calendarSync := range connection.CalendarsOfInterest {
			if googleCalendarMap[calendarSync.CalendarID] != nil {
				googleCalendarMap[calendarSync.CalendarID].IsActive = true
			}
		}

		googleCalendars := make([]*calendar.Calendar, 0, len(googleCalendarMap))

		for _, c := range googleCalendarMap {
			googleCalendars = append(googleCalendars, c)
		}

		googleConnections = GoogleConnectionWithCalendars{Connection: connection, Calendars: googleCalendars}
	}

	handler.ResponseManager.Respond(writer, googleConnections)
}

// PatchCalendars sets the active calendars used for busy time calculation
func (handler *CalendarHandler) PatchCalendars(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	connectionID := mux.Vars(request)["connectionID"]

	u, err := handler.UserRepository.FindByID(request.Context(), userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not find user", err)
		return
	}

	var requestBody []calendar.Calendar

	err = json.NewDecoder(request.Body).Decode(&requestBody)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err)
		return
	}

	connection, index, err := u.GoogleCalendarConnections.FindByConnectionID(connectionID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Could not find connection", err)
		return
	}

	// TODO: check which sources have a connection
	googleRepo, err := handler.CalendarRepositoryManager.GetCalendarRepositoryForUserByConnectionID(request.Context(), u, connectionID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusServiceUnavailable,
			"Error while using Google Calendar connection", err)
		return
	}

	googleCalendars, err := googleRepo.GetAllCalendarsOfInterest()
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not retrieve Google Calendar calendars", err)
		return
	}

	connection.CalendarsOfInterest, u = handler.matchNewGoogleCalendars(request.Context(), u, requestBody, googleCalendars, connection)

	u.GoogleCalendarConnections[index] = *connection

	err = handler.UserRepository.Update(request.Context(), u)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Error trying to persist user", err)
		return
	}

	err = handler.syncGoogleCalendars(writer, request, u)
	if err != nil {
		// We don't have to print error messages because the sub routine already took care of it
	}

	writer.WriteHeader(http.StatusAccepted)
}

func (handler *CalendarHandler) syncGoogleCalendars(writer http.ResponseWriter, request *http.Request, u *users.User) error {
	var err error

	for i, connection := range u.GoogleCalendarConnections {
		u, err = handler.CalendarRepositoryManager.CheckIfGoogleTaskCalendarIsSet(request.Context(), u, &connection, i)
		if err != nil {
			handler.Logger.Error("Could not check if Google Task Calendar is set", errors.Wrap(err, "could not check if Google Task Calendar is set"))
			return err
		}

		calendarRepository, err := handler.CalendarRepositoryManager.GetCalendarRepositoryForUserByConnectionID(request.Context(), u, connection.ID)
		if err != nil {
			handler.Logger.Error("Error while processing user for sync renewal", err)
			return err
		}

		for _, sync := range connection.CalendarsOfInterest {
			u, err = calendarRepository.WatchCalendar(sync.CalendarID, u)
			if err != nil {
				_ = handler.UserRepository.Update(request.Context(), u)
				handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
					"Error while registering for calendar notifications", err)
				return err
			}
		}
	}

	err = handler.UserRepository.Update(request.Context(), u)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Error trying to persist user", err)
		return err
	}

	return nil
}

func (handler *CalendarHandler) matchNewGoogleCalendars(ctx context.Context, u *users.User, requestCalendars []calendar.Calendar, googleCalendars map[string]*calendar.Calendar, connection *users.GoogleCalendarConnection) ([]users.GoogleCalendarSync, *users.User) {
	var newGoogleCalendars []users.GoogleCalendarSync

	for _, c := range requestCalendars {
		if googleCalendars[c.CalendarID] == nil {
			continue
		}

		var foundPresentCalendar *users.GoogleCalendarSync = nil
		for _, userCalendar := range connection.CalendarsOfInterest {
			if userCalendar.CalendarID == c.CalendarID && c.IsActive {
				foundPresentCalendar = &userCalendar
				break
			}
		}

		if !c.IsActive && foundPresentCalendar != nil {
			repository, err := handler.CalendarRepositoryManager.GetCalendarRepositoryForUserByConnectionID(ctx, u, connection.ID)
			if err != nil {
				return nil, u
			}

			u, err = repository.StopWatchingCalendar(foundPresentCalendar.CalendarID, u)
			if err != nil {
				return nil, u
			}
			continue
		}

		if !c.IsActive {
			continue
		}

		if foundPresentCalendar != nil {
			newGoogleCalendars = append(newGoogleCalendars, *foundPresentCalendar)
			continue
		}

		newGoogleCalendars = append(newGoogleCalendars, users.GoogleCalendarSync{CalendarID: c.CalendarID})
	}

	for _, sync := range connection.CalendarsOfInterest {
		if sync.CalendarID == connection.TaskCalendarID {
			newGoogleCalendars = append(newGoogleCalendars, sync)
		}
	}

	return newGoogleCalendars, u
}

// GoogleCalendarSyncRenewal is hit by a scheduler to renew sync that are about to expire
func (handler *CalendarHandler) GoogleCalendarSyncRenewal(writer http.ResponseWriter, request *http.Request) {
	pageSize := 25
	schedulerSecret := os.Getenv("SCHEDULER_SECRET")
	if schedulerSecret == "" {
		schedulerSecret = "local"
	}

	if request.Header.Get("scheduler-secret") != schedulerSecret {
		handler.ResponseManager.RespondWithError(writer, http.StatusForbidden, "Invalid secret", fmt.Errorf("%s != the scheduler secret", request.Header.Get("scheduler-secret")))
		return
	}

	now := time.Now().Add(calendar.GoogleNotificationExpirationOffset)

	_, count, err := handler.UserRepository.FindBySyncExpiration(request.Context(), now, 0, pageSize)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Error finding users for renewal", err)
		return
	}

	pages := int(math.Ceil(float64(count) / float64(pageSize)))

	for i := 0; i < pages; i++ {
		u, _, err := handler.UserRepository.FindBySyncExpiration(request.Context(), now, i, pageSize)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Error finding users for renewal", err)
			return
		}

		for _, user := range u {
			go handler.processUserForSyncRenewal(user, now)
		}
	}

	response := map[string]interface{}{
		"usersProcessed": count,
	}

	handler.Logger.Info(fmt.Sprintf("Processed %d users for google sync renewal", count))

	handler.ResponseManager.Respond(writer, response)
}

func (handler *CalendarHandler) processUserForSyncRenewal(user *users.User, time time.Time) {
	// Google Calendar
	for _, connection := range user.GoogleCalendarConnections {
		calendarRepository, err := handler.CalendarRepositoryManager.GetCalendarRepositoryForUserByConnectionID(context.Background(), user, connection.ID)
		if err != nil {
			handler.Logger.Error("Error while processing user for sync renewal", err)
			return
		}

		for _, sync := range connection.CalendarsOfInterest {
			if !sync.Expiration.Before(time) {
				continue
			}

			// TODO: change when multiple repositories are allowed
			user, err := calendarRepository.WatchCalendar(sync.CalendarID, user)
			if err != nil {
				handler.Logger.Error("Error while trying to renew sync", err)
				return
			}

			err = handler.UserRepository.Update(context.Background(), user)
			if err != nil {
				handler.Logger.Error("Error while trying to update user", err)
				return
			}
		}
	}
}

// InitiateGoogleCalendarAuth responds with the Google Auth URL
func (handler *CalendarHandler) InitiateGoogleCalendarAuth(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	connectionID, ok := mux.Vars(request)["connectionID"]
	if !ok {
		connectionID = ""
	}

	u, err := handler.UserRepository.FindByID(request.Context(), userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Could not find user", err)
		return
	}

	var foundConnectionIndex = -1
	if connectionID == "" {
		for i, connection := range u.GoogleCalendarConnections {
			if connection.Status == users.CalendarConnectionStatusUnverified {
				foundConnectionIndex = i
				break
			}
		}
	} else {
		_, foundConnectionIndex, err = u.GoogleCalendarConnections.FindByConnectionID(connectionID)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Could not find calendar connection", err)
			return
		}
	}

	url, stateToken, err := google.GetGoogleAuthURL()
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Could not get Google config", err)
		return
	}

	if foundConnectionIndex == -1 {
		u.GoogleCalendarConnections = append(u.GoogleCalendarConnections, users.GoogleCalendarConnection{
			ID:         "",
			StateToken: stateToken,
			Status:     users.CalendarConnectionStatusUnverified,
		})
	} else {
		u.GoogleCalendarConnections[foundConnectionIndex].StateToken = stateToken
		u.GoogleCalendarConnections[foundConnectionIndex].Status = users.CalendarConnectionStatusUnverified
	}

	err = handler.UserRepository.Update(request.Context(), u)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Could not update user", err)
		return
	}

	var response = map[string]interface{}{
		"url": url,
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

// DeleteGoogleConnection deletes a Google connection and stops calendar notifications
func (handler *CalendarHandler) DeleteGoogleConnection(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	connectionID := mux.Vars(request)["connectionID"]

	u, err := handler.UserRepository.FindByID(request.Context(), userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Could not find user", err)
		return
	}

	connection, _, err := u.GoogleCalendarConnections.FindByConnectionID(connectionID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, fmt.Sprintf("Could not find connection id %s", connectionID), err)
		return
	}

	repository, err := handler.CalendarRepositoryManager.GetCalendarRepositoryForUserByConnectionID(request.Context(), u, connectionID)
	if err != nil {
		u.GoogleCalendarConnections = u.GoogleCalendarConnections.RemoveConnection(connectionID)

		err = handler.UserRepository.Update(request.Context(), u)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Could not update user", err)
			return
		}

		return
	}

	for _, sync := range connection.CalendarsOfInterest {
		u, err = repository.StopWatchingCalendar(sync.CalendarID, u)
		if err != nil {
			handler.Logger.Warning("Calendar notifications could not be stopped", err)
			continue
		}
	}

	err = google.RevokeToken(request.Context(), &connection.Token)
	if err != nil {
		handler.Logger.Info(fmt.Sprintf("Could not revoke token: %s", err))
	}

	u.GoogleCalendarConnections = u.GoogleCalendarConnections.RemoveConnection(connectionID)

	err = handler.UserRepository.Update(request.Context(), u)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Could not update user", err)
		return
	}

	writer.WriteHeader(http.StatusAccepted)
}

// GoogleCalendarAuthCallback is the call the api will redirect to
func (handler *CalendarHandler) GoogleCalendarAuthCallback(writer http.ResponseWriter, request *http.Request) {
	googleError := request.URL.Query().Get("error")
	authCode := request.URL.Query().Get("code")
	stateToken := request.URL.Query().Get("state")

	if googleError != "" {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Access was denied by user", fmt.Errorf(googleError))
		return
	}

	usr, err := handler.UserRepository.FindByGoogleStateToken(request.Context(), stateToken)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Invalid request", err)
		return
	}

	foundConnectionIndex := -1
	for i, connection := range usr.GoogleCalendarConnections {
		if connection.StateToken == stateToken {
			foundConnectionIndex = i
			break
		}
	}

	if foundConnectionIndex == -1 {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Invalid request", err)
		return
	}

	token, err := google.GetGoogleToken(request.Context(), authCode)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Error getting token", err)
		return
	}

	userInfo, err := google.GetUserInfo(request.Context(), token)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Error getting user id", err)
		return
	}

	for i, connection := range usr.GoogleCalendarConnections {
		if connection.ID == userInfo.ID && i != foundConnectionIndex {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Account is already connected", fmt.Errorf("account %s is already connected", userInfo.ID))
			return
		}
	}

	// Edge case: This checks the case when a user updates a connection, but the new user account is different.
	// Then we want to remove all previous calendars of interest and rebuild them later
	if usr.GoogleCalendarConnections[foundConnectionIndex].ID != "" && usr.GoogleCalendarConnections[foundConnectionIndex].ID != userInfo.ID {
		// The following line will probably fail as the user access is probably already gone, but we try anyways
		repo, err := handler.CalendarRepositoryManager.GetCalendarRepositoryForUserByConnectionID(request.Context(), usr, usr.GoogleCalendarConnections[foundConnectionIndex].ID)
		if err == nil {
			for _, sync := range usr.GoogleCalendarConnections[foundConnectionIndex].CalendarsOfInterest {
				// We also don't care if it worked
				usr, _ = repo.StopWatchingCalendar(sync.CalendarID, usr)
			}
		}

		// We empty it, but leave the task calendar id in, because the user will probably want to reuse it, when he sees his mistake
		usr.GoogleCalendarConnections[foundConnectionIndex].CalendarsOfInterest = users.GoogleCalendarSyncs{}
	}

	usr.GoogleCalendarConnections[foundConnectionIndex].ID = userInfo.ID
	usr.GoogleCalendarConnections[foundConnectionIndex].Email = userInfo.Email
	usr.GoogleCalendarConnections[foundConnectionIndex].Token = *token
	usr.GoogleCalendarConnections[foundConnectionIndex].StateToken = ""
	usr.GoogleCalendarConnections[foundConnectionIndex].Status = users.CalendarConnectionStatusActive

	if google.CheckTokenForCorrectScopes(request.Context(), token) != nil {
		usr.GoogleCalendarConnections[foundConnectionIndex].Status = users.CalendarConnectionStatusMissingScopes
	}

	// TODO: needs to check for other calendar sources as well
	if len(usr.GoogleCalendarConnections) == 1 {
		usr.GoogleCalendarConnections[foundConnectionIndex].IsTaskCalendarConnection = true
	}

	err = handler.UserRepository.Update(request.Context(), usr)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Error updating user", err)
		return
	}

	// Let's also set up the calendar repository here and sync it, so we can initialize things like the Timeliness calendar
	err = handler.syncGoogleCalendars(writer, request, usr)
	if err != nil {
		// We don't have to print error messages because the sub routine already took care of it
	}

	http.Redirect(writer, request, fmt.Sprintf("%s/static/google-connected", os.Getenv("FRONTEND_BASE_URL")), http.StatusFound)
}

// GoogleCalendarNotification receives event change notifications from Google Calendar
func (handler *CalendarHandler) GoogleCalendarNotification(writer http.ResponseWriter, request *http.Request) {
	state := request.Header.Get("X-Goog-Resource-State")
	token := request.Header.Get("X-Goog-Channel-Token")
	resourceID := request.Header.Get("X-Goog-Resource-ID")

	if state == "sync" {
		writer.WriteHeader(http.StatusOK)
		return
	}

	if state == "" || token == "" || resourceID == "" {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	userID := encryption.Decrypt(token)

	user, err := handler.UserRepository.FindByID(request.Context(), userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
			"Could not find user", err)
		return
	}

	calendarID := ""
	calendarIndex := -1
	connectionIndex := -1

Loop:
	for cIndex, connection := range user.GoogleCalendarConnections {
		for i, sync := range connection.CalendarsOfInterest {
			if sync.SyncResourceID == resourceID {
				calendarID = sync.CalendarID
				calendarIndex = i
				connectionIndex = cIndex
				break Loop
			}
		}
	}

	if calendarID == "" {
		handler.Logger.Error(fmt.Sprintf("Could not find calendar sync for resourceId %s for user %s", resourceID, userID), nil)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	go func(user *users.User, calendarIndex int) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
		defer cancel()

		lock, err := handler.Locker.Acquire(ctx, user.ID.Hex(), time.Minute*1, false)
		if err != nil {
			handler.Logger.Error(fmt.Sprintf("error while acquiring lock for user %s", userID), err)
			return
		}

		defer func(lock locking.LockInterface, ctx context.Context) {
			err := lock.Release(ctx)
			if err != nil {
				handler.Logger.Error(fmt.Sprintf("error while releasing lock for user %s", userID), err)
				return
			}
		}(lock, ctx)

		connection := user.GoogleCalendarConnections[connectionIndex]

		if connection.Status != users.CalendarConnectionStatusActive {
			return
		}

		user, err = handler.PlanningService.SyncCalendar(ctx, user, calendarID)
		if err != nil {
			handler.Logger.Error(fmt.Sprintf("error while syncing user %s and calendar ID %s", userID, calendarID), err)
			return
		}

		err = handler.UserRepository.Update(ctx, user)
		if err != nil {
			handler.Logger.Error(fmt.Sprintf("error updating user %s", userID), err)
			return
		}
	}(user, calendarIndex)
}
