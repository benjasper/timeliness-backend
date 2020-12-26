package google

import (
	"context"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gcalendar "google.golang.org/api/calendar/v3"
	"io/ioutil"
	"log"
)

// ReadGoogleConfig reads and parses the json file where google credentials are stored
func ReadGoogleConfig() (*oauth2.Config, error) {
	b, err := ioutil.ReadFile("./keys/credentials.json")
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, gcalendar.CalendarReadonlyScope, gcalendar.CalendarScope)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return config, nil
}

// GetGoogleToken gets a Google OAuth Token with an auth code
func GetGoogleToken(context context.Context, authCode string) (*oauth2.Token, error) {
	config, _ := ReadGoogleConfig()

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
