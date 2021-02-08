package communication

import (
	"encoding/json"
	"errors"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"net/http"
)

// ErrorResponseManager handles errors that have to be returned to the user
type ErrorResponseManager struct {
	Logger logger.Interface
}

// CalendarAuthInvalid is an error thrown if the auth token is invalid
var CalendarAuthInvalid = errors.New("calendar auth is invalid")

// RespondWithError takes several arguments to return an error to the user and logs the error as well
func (m *ErrorResponseManager) RespondWithError(writer http.ResponseWriter, status int, message string, err error) {
	if errors.Is(err, CalendarAuthInvalid) {
		status = http.StatusUnauthorized
		message = "Calendar connection authentication is invalid"
	}

	if status >= 500 {
		m.Logger.Error(message, err)
	}

	writer.WriteHeader(status)
	var response = map[string]interface{}{
		"status": status,
		"error": map[string]interface{}{
			"message": message,
		},
	}

	if err != nil {
		response["err"] = err.Error()
	}

	binary, err := json.Marshal(response)
	if err != nil {
		m.Logger.Fatal(err)
	}

	_, err = writer.Write(binary)
	if err != nil {
		m.Logger.Fatal(err)
	}
}
