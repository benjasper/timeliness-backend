package calendar

import (
	"encoding/json"
	"github.com/timeliness-app/timeliness-backend/pkg/auth"
	"github.com/timeliness-app/timeliness-backend/pkg/communication"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"net/http"
)

// Handler handles all calendar related API calls
type Handler struct {
	UserService  *users.UserService
	Logger       logger.Interface
	ErrorManager *communication.ErrorResponseManager
}

type calendarsPost struct {
	GoogleCalendar []Calendar `json:"googleCalendar"`
}

// GetAllCalendars responds with all calendars the user can register for busy time information
func (handler *Handler) GetAllCalendars(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	u, err := handler.UserService.FindByID(request.Context(), userID)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not find user", err)
		return
	}

	// TODO: check which sources have a connection
	googleRepo, err := NewGoogleCalendarRepository(request.Context(), u)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusServiceUnavailable,
			"Problem with Google Calendar connection", err)
		return
	}

	googleCalendarMap, err := googleRepo.GetAllCalendarsOfInterest()
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not retrieve Google Calendar calendars", err)
		return
	}

	for _, calendar := range u.GoogleCalendarConnection.CalendarsOfInterest {
		if googleCalendarMap[calendar.CalendarID] != nil {
			googleCalendarMap[calendar.CalendarID].IsActive = true
		}
	}

	googleCalendars := make([]*Calendar, 0, len(googleCalendarMap))

	for _, c := range googleCalendarMap {
		googleCalendars = append(googleCalendars, c)
	}

	var response = map[string][]*Calendar{
		"googleCalendar": googleCalendars,
	}

	binary, err := json.Marshal(&response)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem while marshalling tasks into json", err)
		return
	}

	_, err = writer.Write(binary)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Problem writing response", err)
		return
	}
}

// GetAllCalendars responds with all calendars the user can register for busy time information
func (handler *Handler) PostCalendars(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	u, err := handler.UserService.FindByID(request.Context(), userID)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not find user", err)
		return
	}

	requestBody := calendarsPost{}

	err = json.NewDecoder(request.Body).Decode(&requestBody)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err)
		return
	}

	// TODO: check which sources have a connection
	googleRepo, err := NewGoogleCalendarRepository(request.Context(), u)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusServiceUnavailable,
			"Problem with Google Calendar connection", err)
		return
	}

	googleCalendars, err := googleRepo.GetAllCalendarsOfInterest()
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not retrieve Google Calendar calendars", err)
		return
	}

	u.GoogleCalendarConnection.CalendarsOfInterest = matchNewGoogleCalendars(requestBody, googleCalendars, u)

	err = handler.UserService.Update(request.Context(), u)
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusBadRequest, "Problem trying to persist user", err)
		return
	}

	writer.WriteHeader(http.StatusAccepted)
}

func matchNewGoogleCalendars(request calendarsPost, googleCalendars map[string]*Calendar, u *users.User) []users.GoogleCalendarSync {
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

	return newGoogleCalendars
}
