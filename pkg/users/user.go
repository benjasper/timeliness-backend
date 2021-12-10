package users

import (
	"fmt"
	"github.com/timeliness-app/timeliness-backend/pkg/date"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/oauth2"
	"time"
)

// CalendarConnectionStatusActive marks an active calendar connection
const CalendarConnectionStatusActive = "active"

// CalendarConnectionStatusInactive marks an inactive calendar connection
const CalendarConnectionStatusInactive = ""

// CalendarConnectionStatusExpired marks an active calendar connection
const CalendarConnectionStatusExpired = "expired"

// CalendarConnectionStatusUnverified marks a calendar connection in progress
const CalendarConnectionStatusUnverified = "unverified"

// User represents the user
type User struct {
	ID             primitive.ObjectID `json:"id" bson:"_id"`
	Firstname      string             `json:"firstname" validate:"required"`
	Lastname       string             `json:"lastname" validate:"required"`
	Password       string             `json:"-" bson:"password" validate:"required"`
	Email          string             `json:"email" validate:"required,email"`
	CreatedAt      time.Time          `json:"createdAt" bson:"createdAt" validate:"isdefault"`
	LastModifiedAt time.Time          `json:"lastModifiedAt" bson:"lastModifiedAt" validate:"isdefault"`
	DeviceTokens   []DeviceToken      `json:"-" bson:"deviceTokens" validate:"isdefault"`
	IsDeactivated  bool               `json:"-" bson:"isDeactivated"`
	Contacts       []Contact          `json:"contacts" bson:"contacts"`

	GoogleCalendarConnections GoogleCalendarConnections `json:"googleCalendarConnection" bson:"googleCalendarConnection"`
	Settings                  UserSettings              `json:"settings" bson:"settings"`
	EmailVerified             bool                      `json:"emailVerified" bson:"emailVerified"`
	EmailVerificationToken    string                    `json:"-" bson:"emailVerificationToken"`
}

// UserLogin is the view for users logger in
type UserLogin struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" bson:"password" validate:"required"`
}

// Contact is a reference to another User
type Contact struct {
	UserID             primitive.ObjectID
	ContactRequestedAt time.Time
}

// DeviceToken is a struct for keeping a Firebase Cloud Messaging registration token
type DeviceToken struct {
	Token          string
	LastRegistered time.Time
}

type GoogleCalendarConnections []GoogleCalendarConnection

// FindByConnectionID finds a connection by it ID
func (g GoogleCalendarConnections) FindByConnectionID(connectionID string) (*GoogleCalendarConnection, int, error) {
	for i, connection := range g {
		if connectionID == connection.ID.Hex() {
			return &connection, i, nil
		}
	}

	return nil, 0, fmt.Errorf("could not find connection with id %s", connectionID)
}

// GoogleCalendarConnection stores everything related to Google Calendar
type GoogleCalendarConnection struct {
	ID                       primitive.ObjectID  `json:"id" bson:"_id"`
	IsTaskCalendarConnection bool                `json:"isTaskCalendarConnection" bson:"isTaskCalendarConnection"`
	Status                   string              `json:"status" bson:"status"`
	Token                    oauth2.Token        `json:"-" bson:"token,omitempty"`
	StateToken               string              `json:"-" bson:"stateToken,omitempty"`
	TaskCalendarID           string              `json:"-" bson:"taskCalendarID,omitempty"`
	CalendarsOfInterest      GoogleCalendarSyncs `json:"-" bson:"calendarsOfInterest,omitempty"`
}

type GoogleCalendarSyncs []GoogleCalendarSync

func (s GoogleCalendarSyncs) HasCalendarWithID(ID string) bool {
	for _, sync := range s {
		if sync.CalendarID == ID {
			return true
		}
	}

	return false
}

// GoogleCalendarSync holds information about a calendar that will be used to determine busy times
type GoogleCalendarSync struct {
	CalendarID     string    `json:"-" bson:"calendarId,omitempty"`
	SyncResourceID string    `json:"-" bson:"syncResourceId,omitempty"`
	ChannelID      string    `json:"-" bson:"channelId,omitempty"`
	SyncToken      string    `json:"-" bson:"syncToken,omitempty"`
	Expiration     time.Time `json:"-" bson:"expiration,omitempty"`
}

// UserSettings hold different settings roughly separated by topics
type UserSettings struct {
	OnboardingCompleted bool `json:"onboardingCompleted" bson:"onboardingCompleted"`
	Scheduling          struct {
		TimeZone         string          `json:"timeZone" bson:"timeZone" validate:"required"`
		AllowedTimespans []date.Timespan `json:"allowedTimespans" bson:"allowedTimespans"`
	} `json:"scheduling" bson:"scheduling"`
}
