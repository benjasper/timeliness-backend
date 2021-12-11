package google

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gcalendar "google.golang.org/api/calendar/v3"
	oauth22 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
	"io/ioutil"
	"os"
)

// ReadGoogleConfig reads and parses the json file where google credentials are stored
func ReadGoogleConfig() (*oauth2.Config, error) {
	b, err := ioutil.ReadFile("./keys/credentials.json")
	if err != nil {
		return nil, err
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, gcalendar.CalendarReadonlyScope, "https://www.googleapis.com/auth/calendar.app.created", "https://www.googleapis.com/auth/userinfo.profile")
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

func GetUserId(ctx context.Context, token *oauth2.Token) (string, error) {
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

	if userinfo.Id == "" {
		if userinfo.Email == "" {
			return "", fmt.Errorf("neither user ID nor email exists")
		}

		return userinfo.Email, nil
	}

	return userinfo.Id, nil
}
