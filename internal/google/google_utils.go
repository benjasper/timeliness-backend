package google

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gcalendar "google.golang.org/api/calendar/v3"
	oauth22 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

// ReadGoogleConfig reads and parses the json file where google credentials are stored
func ReadGoogleConfig() (*oauth2.Config, error) {
	b := []byte(os.Getenv("GCP_AUTH_CREDENTIALS"))

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, gcalendar.CalendarReadonlyScope, "https://www.googleapis.com/auth/calendar.app.created", "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		return nil, err
	}

	apiBaseURL := "http://localhost"
	envBaseURL, ok := os.LookupEnv("BASE_URL")
	if ok {
		apiBaseURL = envBaseURL
	}

	config.RedirectURL = fmt.Sprintf("%s/v1/auth/google", apiBaseURL)

	return config, nil
}

// GetGoogleToken gets a Google OAuth Token with an auth code
func GetGoogleToken(context context.Context, authCode string) (*oauth2.Token, error) {
	config, err := ReadGoogleConfig()
	if err != nil {
		return nil, err
	}

	tok, err := config.Exchange(context, authCode, oauth2.AccessTypeOffline)
	if err != nil {
		return nil, err
	}
	return tok, nil
}

// GetGoogleAuthURL returns the URL where the user can allow Timeliness access to the calendar
func GetGoogleAuthURL() (string, string, error) {
	config, err := ReadGoogleConfig()
	if err != nil {
		return "", "", err
	}

	stateToken := uuid.New().String()

	url := config.AuthCodeURL(stateToken, oauth2.AccessTypeOffline)

	return url, stateToken, nil
}

// GetUserID gets the matching user id to a token from the Google API
func GetUserID(ctx context.Context, token *oauth2.Token) (string, error) {
	config, err := ReadGoogleConfig()
	if err != nil {
		return "", err
	}

	client := config.Client(ctx, token)

	service, err := oauth22.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return "", err
	}

	userinfo, err := service.Userinfo.Get().Do()
	if err != nil {
		return "", err
	}

	if strings.Trim(userinfo.Email, " ") != "" {
		return userinfo.Email, nil
	}

	if strings.Trim(userinfo.Id, " ") != "" {
		return userinfo.Id, nil
	}

	return "", errors.Errorf("neither email nor user ID exists")
}

// RevokeToken revokes a google access token
func RevokeToken(ctx context.Context, token *oauth2.Token) error {
	tokenToRevoke := token.AccessToken
	if token.RefreshToken != "" {
		tokenToRevoke = token.RefreshToken
	}

	url := fmt.Sprintf("https://oauth2.googleapis.com/revoke?token=%s", tokenToRevoke)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		// We don't care if this worked or not
		_ = Body.Close()
	}(resp.Body)

	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("google revoke: status %d: %s", resp.StatusCode, body)
	}

	return nil
}
