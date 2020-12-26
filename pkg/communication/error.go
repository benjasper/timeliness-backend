package communication

import (
	"encoding/json"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"net/http"
)

// ErrorResponseManager handles errors that have to be returned to the user
type ErrorResponseManager struct {
	Logger logger.Interface
}

// RespondWithError takes several arguments to return an error to the user and logs the error as well
func (m *ErrorResponseManager) RespondWithError(writer http.ResponseWriter, status int, message string, err error) {
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
