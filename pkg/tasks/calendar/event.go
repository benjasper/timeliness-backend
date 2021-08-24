package calendar

import "go.mongodb.org/mongo-driver/bson/primitive"

// Type declares in which calendar implementation an event is persisted
type Type string

const (
	// CalendarTypeGoogleCalendar is different calendar implementation enum
	CalendarTypeGoogleCalendar Type = "google_calendar"
)

// Event represents a simple calendar event
type Event struct {
	Date        Timespan `json:"date" bson:"date" validate:"required"`
	Title       string   `json:"-" bson:"-"`
	Description string   `json:"-" bson:"-"`
	IsOriginal  bool     `json:"-" bson:"-"`
	Blocking    bool     `json:"-" bson:"blocking"`
	Deleted     bool     `json:"-" bson:"deleted"`

	CalendarEvents CalendarEvents `json:"-" bson:"calendarEvents"`
}

type CalendarEvent struct {
	CalendarType    Type               `json:"-" bson:"calendarType"`
	CalendarEventID string             `json:"-" bson:"calendarEventID"`
	UserID          primitive.ObjectID `json:"-" bson:"userID"`
}

// CalendarEvents is a slice of CalendarEvents
type CalendarEvents []CalendarEvent

// FindByCalendarID finds a CalendarEvent by its CalendarEventID
func (c CalendarEvents) FindByCalendarID(ID string) *CalendarEvent {
	for _, event := range c {
		if event.CalendarEventID == ID {
			return &event
		}
	}

	return nil
}

// FindByUserID finds a CalendarEvent by its UserID
func (c CalendarEvents) FindByUserID(ID string) *CalendarEvent {
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
