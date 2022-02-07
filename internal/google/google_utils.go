package google

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gcalendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/idtoken"
	oauth22 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

// AllScopes is a list of all scopes that will be requested
var AllScopes = []string{gcalendar.CalendarReadonlyScope, oauth22.UserinfoProfileScope, oauth22.UserinfoEmailScope, "https://www.googleapis.com/auth/calendar.app.created", "https://www.googleapis.com/auth/calendar.calendarlist"}

// UserInfo is the sign in information about the user
type UserInfo struct {
	Email         string `json:"email"`
	Firstname     string `json:"firstname"`
	Lastname      string `json:"lastname"`
	ID            string `json:"id"`
	EmailVerified bool   `json:"emailVerified"`
}

// ReadGoogleConfig reads and parses the json file where google credentials are stored
func ReadGoogleConfig(withCustomScopes bool) (*oauth2.Config, error) {
	b := []byte(os.Getenv("GCP_AUTH_CREDENTIALS"))

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b)
	if err != nil {
		return nil, err
	}

	if withCustomScopes {
		config.Scopes = AllScopes
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
	config, err := ReadGoogleConfig(true)
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
	config, err := ReadGoogleConfig(true)
	if err != nil {
		return "", "", err
	}

	stateToken := uuid.New().String()

	url := config.AuthCodeURL(stateToken, oauth2.AccessTypeOffline)

	return url, stateToken, nil
}

// GetUserInfo gets the matching user id to a token from the Google API
func GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	config, err := ReadGoogleConfig(true)
	if err != nil {
		return nil, err
	}

	client := config.Client(ctx, token)

	service, err := oauth22.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	userinfo, err := service.Userinfo.Get().Do()
	if err != nil {
		return nil, err
	}

	return &UserInfo{
		Email:         userinfo.Email,
		Firstname:     userinfo.GivenName,
		Lastname:      userinfo.FamilyName,
		ID:            userinfo.Id,
		EmailVerified: *userinfo.VerifiedEmail,
	}, nil
}

// GetUserInfoFromIDToken validates a Google ID Token and returns the user info
func GetUserInfoFromIDToken(ctx context.Context, idToken string) (*UserInfo, error) {
	validate, err := idtoken.Validate(ctx, idToken, "")
	if err != nil {
		return nil, err
	}

	return &UserInfo{
		Email:         validate.Claims["email"].(string),
		Firstname:     validate.Claims["given_name"].(string),
		Lastname:      validate.Claims["family_name"].(string),
		ID:            validate.Subject,
		EmailVerified: validate.Claims["email_verified"].(bool),
	}, nil
}

// CheckTokenForCorrectScopes checks if the token is valid and has the correct scopes
func CheckTokenForCorrectScopes(ctx context.Context, token *oauth2.Token) error {
	config, err := ReadGoogleConfig(false)
	if err != nil {
		return err
	}

	client := config.Client(ctx, token)

	service, err := oauth22.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return err
	}

	result, err := service.Tokeninfo().Do()
	if err != nil {
		return err
	}

	scopes := strings.Trim(result.Scope, " ")

	// Check if AllScopes are in the scopes
	for _, scope := range AllScopes {
		if !strings.Contains(scopes, scope) {
			return errors.Errorf("missing scope %s", scope)
		}
	}

	return nil
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
