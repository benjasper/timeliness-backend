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
	Date       date.Timespan `json:"date" bson:"date" validate:"required"`
	IsOriginal bool          `json:"-" bson:"-"`
	Blocking   bool          `json:"-" bson:"blocking"`
	Deleted    bool          `json:"-" bson:"deleted"`

	CalendarEvents PersistedEvents `json:"-" bson:"calendarEvents"`
}

// AgendaEvent represents an agenda view calendar event
type AgendaEvent struct {
	Date       date.Timespan `json:"date" bson:"date" validate:"required"`
	IsOriginal bool          `json:"-" bson:"-"`
	Blocking   bool          `json:"-" bson:"blocking"`
	Deleted    bool          `json:"-" bson:"deleted"`
	Type       string        `json:"type" bson:"type"`

	CalendarEvents PersistedEvents `json:"-" bson:"calendarEvents"`
}

// PersistedEvent represents an event persistent in a users calendar
type PersistedEvent struct {
	CalendarType    Type               `json:"calendarType" bson:"calendarType"`
	CalendarEventID string             `json:"calendarEventId" bson:"calendarEventID"`
	UserID          primitive.ObjectID `json:"userId" bson:"userID"`
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

// IsEmpty returns true if the slice is empty
func (c PersistedEvents) IsEmpty() bool {
	return len(c) == 0
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

// RemoveByUserID removes a PersistedEvent by its UserID
func (c PersistedEvents) RemoveByUserID(ID string) PersistedEvents {
	userID, _ := primitive.ObjectIDFromHex(ID)

	for i, event := range c {
		if event.UserID == userID {
			c = append(c[:i], c[i+1:]...)
			return c
		}
	}

	return c
}

// Calendar represents a calendar of any source
type Calendar struct {
	CalendarID string `json:"calendarId"`
	Name       string `json:"name"`
	IsActive   bool   `json:"isActive"`
}
