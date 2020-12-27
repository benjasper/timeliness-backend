package calendar

import "time"

// Type declares in which calendar implementation an event is persisted
type Type string

const (
	CalendarTypeGoogleCalendar Type = "google_calendar"
)

// Event represents a simple calendar event
type Event struct {
	Start time.Time `json:"start" bson:"start" validate:"required"`
	End   time.Time `json:"end" bson:"end" validate:"required"`

	CalendarType    Type   `json:"calendarType" bson:"calendarType"`
	CalendarEventID string `json:"calendarEventID" bson:"calendarEventID"`
}
