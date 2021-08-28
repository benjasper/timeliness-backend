package tasks

import (
	"context"
	"encoding/json"
	"fmt"
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
	UserService     users.UserRepositoryInterface
	TaskService     *MongoDBTaskRepository
	Logger          logger.Interface
	ResponseManager *communication.ResponseManager
	PlanningService *PlanningService
	Locker          locking.LockerInterface
}

type calendarsPost struct {
	GoogleCalendar []calendar.Calendar `json:"googleCalendar"`
}

// GetAllCalendars responds with all calendars the user can register for busy time information
func (handler *CalendarHandler) GetAllCalendars(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	u, err := handler.UserService.FindByID(request.Context(), userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not find user", err)
		return
	}

	// TODO: check which sources have a connection
	googleRepo, err := calendar.NewGoogleCalendarRepository(request.Context(), u, handler.Logger)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem with Google Calendar connection", err)
		return
	}

	googleCalendarMap, err := googleRepo.GetAllCalendarsOfInterest()
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not retrieve Google Calendar calendars", err)
		return
	}

	for _, calendarSync := range u.GoogleCalendarConnection.CalendarsOfInterest {
		if googleCalendarMap[calendarSync.CalendarID] != nil {
			googleCalendarMap[calendarSync.CalendarID].IsActive = true
		}
	}

	googleCalendars := make([]*calendar.Calendar, 0, len(googleCalendarMap))

	for _, c := range googleCalendarMap {
		googleCalendars = append(googleCalendars, c)
	}

	var response = map[string][]*calendar.Calendar{
		"googleCalendar": googleCalendars,
	}

	binary, err := json.Marshal(&response)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem while marshalling tasks into json", err)
		return
	}

	_, err = writer.Write(binary)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem writing response", err)
		return
	}
}

// PostCalendars sets the active calendars used for busy time calculation
func (handler *CalendarHandler) PostCalendars(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	u, err := handler.UserService.FindByID(request.Context(), userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not find user", err)
		return
	}

	requestBody := calendarsPost{}

	err = json.NewDecoder(request.Body).Decode(&requestBody)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err)
		return
	}

	// TODO: check which sources have a connection
	googleRepo, err := calendar.NewGoogleCalendarRepository(request.Context(), u, handler.Logger)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusServiceUnavailable,
			"Problem with Google Calendar connection", err)
		return
	}

	googleCalendars, err := googleRepo.GetAllCalendarsOfInterest()
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not retrieve Google Calendar calendars", err)
		return
	}

	u.GoogleCalendarConnection.CalendarsOfInterest = matchNewGoogleCalendars(requestBody, googleCalendars, u)

	err = handler.UserService.Update(request.Context(), u)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Problem trying to persist user", err)
		return
	}

	googleCalendarRepository, err := calendar.NewGoogleCalendarRepository(request.Context(), u, handler.Logger)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem with calendar communication", err)
		return
	}

	env := os.Getenv("APP_ENV")
	for _, sync := range u.GoogleCalendarConnection.CalendarsOfInterest {
		if env != "prod" {
			continue
		}
		u, err = googleCalendarRepository.WatchCalendar(sync.CalendarID, u)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
				"Problem with calendar notification registration", err)
			return
		}
	}

	err = handler.UserService.Update(request.Context(), u)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Problem trying to persist user", err)
		return
	}

	writer.WriteHeader(http.StatusAccepted)
}

func matchNewGoogleCalendars(request calendarsPost, googleCalendars map[string]*calendar.Calendar, u *users.User) []users.GoogleCalendarSync {
	var newGoogleCalendars []users.GoogleCalendarSync
	for _, c := range request.GoogleCalendar {
		if googleCalendars[c.CalendarID] == nil {
			continue
		}

		var foundPresentCalendar *users.GoogleCalendarSync = nil
		for _, userCalendar := range u.GoogleCalendarConnection.CalendarsOfInterest {
			if userCalendar.CalendarID == c.CalendarID && c.IsActive {
				foundPresentCalendar = &userCalendar
				break
			}
		}

		if !c.IsActive && foundPresentCalendar != nil {
			// TODO: deregister c notifications gracefully
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

	for _, sync := range u.GoogleCalendarConnection.CalendarsOfInterest {
		if sync.CalendarID == u.GoogleCalendarConnection.TaskCalendarID {
			newGoogleCalendars = append(newGoogleCalendars, sync)
		}
	}

	return newGoogleCalendars
}

// GoogleCalendarSyncRenewal is hit by a scheduler to renew sync that are about to expire
func (handler *CalendarHandler) GoogleCalendarSyncRenewal(writer http.ResponseWriter, request *http.Request) {
	pageSize := 25
	schedulerSecret := os.Getenv("SCHEDULER_SECRET")
	if schedulerSecret == "" {
		schedulerSecret = "local"
	}

	if request.Header.Get("scheduler-secret") != schedulerSecret {
		handler.ResponseManager.RespondWithError(writer, http.StatusForbidden, "Invalid secret", nil)
		return
	}

	now := time.Now().Add(calendar.GoogleNotificationExpirationOffset)

	_, count, err := handler.UserService.FindBySyncExpiration(request.Context(), now, 0, pageSize)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Problem finding users for renewal", err)
		return
	}

	pages := int(math.Ceil(float64(count) / float64(pageSize)))

	for i := 0; i < pages; i++ {
		u, _, err := handler.UserService.FindBySyncExpiration(request.Context(), now, i, pageSize)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Problem finding users for renewal", err)
			return
		}

		for _, user := range u {
			go handler.processUserForSyncRenewal(user, now)
		}
	}

	response := map[string]interface{}{
		"usersProcessed": count,
	}

	handler.ResponseManager.Respond(writer, response)
}

func (handler *CalendarHandler) processUserForSyncRenewal(user *users.User, time time.Time) {
	calendarRepository, err := calendar.NewGoogleCalendarRepository(context.Background(), user, handler.Logger)
	if err != nil {
		handler.Logger.Error("Problem while processing user for sync renewal", err)
		return
	}

	for _, sync := range user.GoogleCalendarConnection.CalendarsOfInterest {
		if !sync.Expiration.Before(time) {
			continue
		}

		user, err := calendarRepository.WatchCalendar(sync.CalendarID, user)
		if err != nil {
			handler.Logger.Error("Problem while trying to renew sync", err)
			return
		}

		err = handler.UserService.Update(context.Background(), user)
		if err != nil {
			handler.Logger.Error("Problem while trying to update user", err)
			return
		}
	}
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

	user, err := handler.UserService.FindByID(request.Context(), userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
			"Could not find user", err)
		return
	}

	calendarID := ""
	calendarIndex := -1
	for i, sync := range user.GoogleCalendarConnection.CalendarsOfInterest {
		if sync.SyncResourceID == resourceID {
			calendarID = sync.CalendarID
			calendarIndex = i
			break
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

		lock, err := handler.Locker.Acquire(ctx, user.ID.Hex(), time.Minute*1)
		if err != nil {
			handler.Logger.Error(fmt.Sprintf("problem while acquiring lock for user %s", userID), err)
			return
		}

		defer func(lock locking.LockInterface, ctx context.Context) {
			err := lock.Release(ctx)
			if err != nil {
				handler.Logger.Error(fmt.Sprintf("problem while releasing lock for user %s", userID), err)
				return
			}
		}(lock, ctx)

		user, err = handler.PlanningService.SyncCalendar(ctx, user, calendarID)
		if err != nil {
			handler.Logger.Error(fmt.Sprintf("problem while syncing user %s and calendar ID %s", userID, calendarID), err)
			return
		}

		err = handler.UserService.Update(ctx, user)
		if err != nil {
			handler.Logger.Error(fmt.Sprintf("problem updating user %s", userID), err)
			return
		}
	}(user, calendarIndex)
}
