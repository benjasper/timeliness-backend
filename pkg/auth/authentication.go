package auth

import (
	"errors"
	"github.com/benjasper/project-tasks/pkg/communication"
	"net/http"
	"strings"
)

type AuthenticationMiddleware struct {
	ErrorManager *communication.ErrorResponseManager
}

func (m *AuthenticationMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {

	})
}

func extractTokenStringFromHeader(r *http.Request) (string, error) {
	unformatted := r.Header.Get("Authorization")
	if strings.TrimSpace(unformatted) == "" {
		return "", errors.New("no authorization token specified")
	}

	tokenParts := strings.Fields(unformatted)
	if tokenParts[0] != "Bearer" {
		return "", errors.New("token must be a bearer token")
	}

	if strings.TrimSpace(tokenParts[1]) == "" {
		return "", errors.New("no authorization token specified")
	}

	return tokenParts[1], nil
}
