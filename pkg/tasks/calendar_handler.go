package tasks

import (
	"encoding/json"
	"github.com/timeliness-app/timeliness-backend/pkg/auth"
	"github.com/timeliness-app/timeliness-backend/pkg/communication"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"net/http"
)

// CalendarHandler handles all calendar related API calls
type CalendarHandler struct {
	UserService     *users.UserService
	TaskService     *TaskService
	Logger          logger.Interface
	ResponseManager *communication.ResponseManager
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
	googleRepo, err := calendar.NewGoogleCalendarRepository(request.Context(), u)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusServiceUnavailable,
			"Problem with Google Calendar connection", err)
		return
	}

	googleCalendarMap, err := googleRepo.GetAllCalendarsOfInterest()
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not retrieve Google Calendar calendars", err)
		return
	}

	for _, calendar := range u.GoogleCalendarConnection.CalendarsOfInterest {
		if googleCalendarMap[calendar.CalendarID] != nil {
			googleCalendarMap[calendar.CalendarID].IsActive = true
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
	googleRepo, err := calendar.NewGoogleCalendarRepository(request.Context(), u)
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

	planning, err := NewPlanningController(request.Context(), u, handler.UserService, handler.TaskService, handler.Logger)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem with calendar communication", err)
		return
	}

	for _, sync := range u.GoogleCalendarConnection.CalendarsOfInterest {
		err := planning.SyncCalendar(userID, sync.CalendarID)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
				"Problem with calendar sync", err)
			return
		}
	}

	writer.WriteHeader(http.StatusAccepted)
}

func matchNewGoogleCalendars(request calendarsPost, googleCalendars map[string]*calendar.Calendar, u *users.User) []users.GoogleCalendarSync {
	var newGoogleCalendars []users.GoogleCalendarSync
	for _, calendar := range request.GoogleCalendar {
		if googleCalendars[calendar.CalendarID] == nil {
			continue
		}

		var foundPresentCalendar *users.GoogleCalendarSync = nil
		for _, userCalendar := range u.GoogleCalendarConnection.CalendarsOfInterest {
			if userCalendar.CalendarID == calendar.CalendarID && calendar.IsActive {
				foundPresentCalendar = &userCalendar
				break
			}
		}

		if !calendar.IsActive && foundPresentCalendar != nil {
			// TODO: deregister calendar notifications gracefully
			continue
		}

		if !calendar.IsActive {
			continue
		}

		if foundPresentCalendar != nil {
			newGoogleCalendars = append(newGoogleCalendars, *foundPresentCalendar)
			continue
		}

		newGoogleCalendars = append(newGoogleCalendars, users.GoogleCalendarSync{CalendarID: calendar.CalendarID})
	}

	for _, sync := range u.GoogleCalendarConnection.CalendarsOfInterest {
		if sync.CalendarID == u.GoogleCalendarConnection.TaskCalendarID {
			newGoogleCalendars = append(newGoogleCalendars, sync)
		}
	}

	return newGoogleCalendars
}
