package communication

import (
	"encoding/json"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"net/http"
)

type ErrorResponseManager struct {
	Logger logger.Interface
}

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
