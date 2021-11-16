package calendar

import (
	"github.com/timeliness-app/timeliness-backend/pkg/date"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Type declares in which calendar implementation an event is persisted
type Type string

const (
	// PersistedCalendarTypeGoogleCalendar is different calendar implementation enum
	PersistedCalendarTypeGoogleCalendar Type = "google_calendar"
)

// Event represents a simple calendar event
type Event struct {
	Date        date.Timespan `json:"date" bson:"date" validate:"required"`
	Title       string        `json:"-" bson:"title"`
	Description string        `json:"-" bson:"-"`
	IsOriginal  bool          `json:"-" bson:"-"`
	Blocking    bool          `json:"-" bson:"blocking"`
	Deleted     bool          `json:"-" bson:"deleted"`

	CalendarEvents PersistedEvents `json:"-" bson:"calendarEvents"`
}

// AgendaEvent represents an agenda view calendar event
type AgendaEvent struct {
	Date        date.Timespan `json:"date" bson:"date" validate:"required"`
	Title       string        `json:"-" bson:"title"`
	Description string        `json:"-" bson:"-"`
	IsOriginal  bool          `json:"-" bson:"-"`
	Blocking    bool          `json:"-" bson:"blocking"`
	Deleted     bool          `json:"-" bson:"deleted"`
	Type        string        `json:"type" bson:"type"`

	CalendarEvents PersistedEvents `json:"-" bson:"calendarEvents"`
}

// PersistedEvent represents an event persistent in a users calendar
type PersistedEvent struct {
	CalendarType    Type               `json:"-" bson:"calendarType"`
	CalendarEventID string             `json:"-" bson:"calendarEventID"`
	UserID          primitive.ObjectID `json:"-" bson:"userID"`
}

// PersistedEvents is a slice of PersistedEvents
type PersistedEvents []PersistedEvent

// FindByCalendarID finds a PersistedEvent by its CalendarEventID
func (c PersistedEvents) FindByCalendarID(ID string) *PersistedEvent {
	for _, event := range c {
		if event.CalendarEventID == ID {
			return &event
		}
	}

	return nil
}

// FindByUserID finds a PersistedEvent by its UserID
func (c PersistedEvents) FindByUserID(ID string) *PersistedEvent {
	userID, _ := primitive.ObjectIDFromHex(ID)

	for _, event := range c {
		if event.UserID == userID {
			return &event
		}
	}

	return nil
}

// Calendar represents a calendar of any source
type Calendar struct {
	CalendarID string `json:"calendarId"`
	Name       string `json:"name"`
	IsActive   bool   `json:"isActive"`
}
