package auth

import (
	"context"
	"errors"
	"github.com/timeliness-app/timeliness-backend/pkg/auth/jwt"
	"github.com/timeliness-app/timeliness-backend/pkg/communication"
	"net/http"
	"strings"
)

// AuthenticationMiddleware checks if the user login token is valid and responds with an error if it's not the case
type AuthenticationMiddleware struct {
	ErrorManager *communication.ResponseManager
}

type key string

const (
	// KeyUserID the key for the request variable for getting the user id
	KeyUserID key = "userID"
)

// Middleware gets called when a request needs to be authenticated
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

		newContext := context.WithValue(r.Context(), KeyUserID, token.Payload.Subject)
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
