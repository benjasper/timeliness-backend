package communication

import (
	"encoding/json"
	"errors"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"net/http"
)

// ResponseManager handles errors that have to be returned to the user
type ResponseManager struct {
	Logger logger.Interface
}

// ErrCalendarAuthInvalid is an error thrown if calendar auth is invalid
var ErrCalendarAuthInvalid = errors.New("calendar auth is invalid")

// RespondWithError takes several arguments to return an error to the user and logs the error as well
func (r *ResponseManager) RespondWithError(writer http.ResponseWriter, status int, message string, err error) {
	if errors.Is(err, ErrCalendarAuthInvalid) {
		status = http.StatusUnauthorized
		message = "Calendar connection authentication is invalid"
	}

	if status >= 500 {
		r.Logger.Error(message, err)
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
		r.Logger.Fatal(err)
	}

	_, err = writer.Write(binary)
	if err != nil {
		r.Logger.Fatal(err)
	}
}

// RespondWithBinary simply returns binary data
func (r ResponseManager) RespondWithBinary(writer http.ResponseWriter, i []byte, contentType string) {
	_, err := writer.Write(i)
	if err != nil {
		r.RespondWithError(writer, http.StatusInternalServerError,
			"Problem writing response", err)
		return
	}

	writer.Header().Add("Content-Type", contentType)
}

// Respond takes an object and turns it into json and responds with it and a 200 HTTP status
func (r ResponseManager) Respond(writer http.ResponseWriter, i interface{}) {
	binary, err := json.Marshal(i)
	if err != nil {
		r.RespondWithError(writer, http.StatusInternalServerError,
			"Problem while marshalling tasks into json", err)
		return
	}

	_, err = writer.Write(binary)
	if err != nil {
		r.RespondWithError(writer, http.StatusInternalServerError,
			"Problem writing response", err)
		return
	}
}

// RespondWithStatus responds with a specific status code
func (r ResponseManager) RespondWithStatus(writer http.ResponseWriter, i interface{}, status int) {
	binary, err := json.Marshal(i)
	if err != nil {
		r.RespondWithError(writer, http.StatusInternalServerError,
			"Problem while marshalling tasks into json", err)
		return
	}

	writer.WriteHeader(status)
	_, err = writer.Write(binary)
	if err != nil {
		r.RespondWithError(writer, http.StatusInternalServerError,
			"Problem writing response", err)
		return
	}
}

// RespondWithNoContent sends a no content status code
func (r ResponseManager) RespondWithNoContent(writer http.ResponseWriter) {
	writer.WriteHeader(http.StatusNoContent)
}
