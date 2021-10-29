package users

import (
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

	GoogleCalendarConnection GoogleCalendarConnection `json:"googleCalendarConnection" bson:"googleCalendarConnection"`
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

// GoogleCalendarConnection stores everything related to Google Calendar
type GoogleCalendarConnection struct {
	Status              string               `json:"status" bson:"status"`
	Token               oauth2.Token         `json:"-" bson:"token,omitempty"`
	StateToken          string               `json:"-" bson:"stateToken,omitempty"`
	TaskCalendarID      string               `json:"-" bson:"taskCalendarID,omitempty"`
	CalendarsOfInterest []GoogleCalendarSync `json:"-" bson:"calendarsOfInterest,omitempty"`
}

// GoogleCalendarSync holds information about a calendar that will be used to determine busy times
type GoogleCalendarSync struct {
	CalendarID     string    `json:"-" bson:"calendarId,omitempty"`
	SyncResourceID string    `json:"-" bson:"syncResourceId,omitempty"`
	ChannelID      string    `json:"-" bson:"channelId,omitempty"`
	SyncToken      string    `json:"-" bson:"syncToken,omitempty"`
	Expiration     time.Time `json:"-" bson:"expiration,omitempty"`
}
