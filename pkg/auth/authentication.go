package auth

import (
	"context"
	"errors"
	"github.com/benjasper/project-tasks/pkg/auth/jwt"
	"github.com/benjasper/project-tasks/pkg/communication"
	"net/http"
	"strings"
)

type AuthenticationMiddleware struct {
	ErrorManager *communication.ErrorResponseManager
}

func (m *AuthenticationMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		extractedToken, err := extractTokenStringFromHeader(r)
		if err != nil {
			m.ErrorManager.RespondWithError(writer, http.StatusUnauthorized, "No authorization", err)
			return
		}
		claims := jwt.Claims{}
		token, err := jwt.Verify(extractedToken, "secret", jwt.AlgHS256, claims)
		if err != nil {
			m.ErrorManager.RespondWithError(writer, http.StatusUnauthorized, "Token invalid", err)
			return
		}

		newContext := context.WithValue(r.Context(), "userID", token.Payload.Subject)
		next.ServeHTTP(writer, r.WithContext(newContext))
	})
}

func extractTokenStringFromHeader(r *http.Request) (string, error) {
	nonformatted := r.Header.Get("Authorization")
	if strings.TrimSpace(nonformatted) == "" {
		return "", errors.New("no authorization token specified")
	}

	tokenParts := strings.Fields(nonformatted)
	if tokenParts[0] != "Bearer" {
		return "", errors.New("token must be a bearer token")
	}

	if strings.TrimSpace(tokenParts[1]) == "" {
		return "", errors.New("no authorization token specified")
	}

	return tokenParts[1], nil
}
