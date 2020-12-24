package google

import (
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gcalendar "google.golang.org/api/calendar/v3"
	"io/ioutil"
	"log"
)

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
