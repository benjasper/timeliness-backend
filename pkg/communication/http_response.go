package communication

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/timeliness-app/timeliness-backend/pkg/environment"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"net/http"
)

// ResponseManager handles errors that have to be returned to the user
type ResponseManager struct {
	Logger      logger.Interface
	Environment string
}

// ErrCalendarAuthInvalid is an error thrown if calendar auth is invalid
var ErrCalendarAuthInvalid = errors.New("calendar auth is invalid")

const (
	// Other is a generic error
	Other = "other"

	// Calendar is an error thrown if calendar auth is invalid
	Calendar = "calendar"
)

// RespondWithError returns an error to the user
func (r *ResponseManager) RespondWithError(writer http.ResponseWriter, status int, message string, err error) {
	errorType := Other

	if errors.Cause(err) == ErrCalendarAuthInvalid {
		errorType = Calendar
		return
	}

	r.RespondWithErrorAndErrorType(writer, status, message, err, errorType)
}

// RespondWithErrorAndErrorType takes several arguments to return an error to the user and logs the error as well
func (r *ResponseManager) RespondWithErrorAndErrorType(writer http.ResponseWriter, status int, message string, err error, errorType string) {
	trackId := uuid.New().String()

	if status >= 500 {
		r.Logger.Error(fmt.Sprintf("%s - trackId: %s", message, trackId), err)
	} else {
		r.Logger.Warning(fmt.Sprintf("%s - trackId: %s", message, trackId), err)
	}

	writer.WriteHeader(status)
	var response = map[string]interface{}{
		"status": status,
		"error": map[string]interface{}{
			"type":    errorType,
			"message": message,
			"trackId": trackId,
		},
	}

	if err != nil && r.Environment != environment.Production {
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
