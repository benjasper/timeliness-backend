package users

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/oauth2"
	"time"
)

// User represents the user
type User struct {
	ID             primitive.ObjectID `json:"id" bson:"_id"`
	Firstname      string             `json:"firstname" validate:"required"`
	Lastname       string             `json:"lastname" validate:"required"`
	Password       string             `json:"-" bson:"password" validate:"required"`
	Email          string             `json:"email" validate:"required,email"`
	CreatedAt      time.Time          `json:"createdAt" bson:"createdAt" validate:"isdefault"`
	LastModifiedAt time.Time          `json:"lastModifiedAt" bson:"lastModifiedAt" validate:"isdefault"`

	GoogleCalendarConnection GoogleCalendarConnection `json:"-" bson:"googleCalendarConnection,omitempty"`
}

// UserLogin is the view for users logging in
type UserLogin struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" bson:"password" validate:"required"`
}

// GoogleCalendarConnection stores everything related to Google Calendar
type GoogleCalendarConnection struct {
	Token               oauth2.Token
	StateToken          string               `json:"stateToken,omitempty" bson:"stateToken,omitempty"`
	TaskCalendar        GoogleCalendarSync   `json:"calendarId,omitempty" bson:"calendarId,omitempty"`
	CalendarsOfInterest []GoogleCalendarSync `json:"calendarsOfInterest" bson:"calendarsOfInterest,omitempty"`
}

// GoogleCalendarSync holds information about a calendar that will be used to determine busy times
type GoogleCalendarSync struct {
	CalendarID     string
	SyncResourceID string
	SyncToken      string
}
