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

	googleCalendars, err := googleRepo.GetAllCalendars()
	if err != nil {
		handler.ErrorManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not retrieve Google Calendar calendars", err)
		return
	}

	var response = map[string]interface{}{
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
