package calendar

// Type declares in which calendar implementation an event is persisted
type Type string

const (
	// CalendarTypeGoogleCalendar is different calendar implementation enum
	CalendarTypeGoogleCalendar Type = "google_calendar"
)

// Event represents a simple calendar event
type Event struct {
	Date Timespan `json:"date" bson:"date" validate:"required"`

	CalendarType    Type   `json:"-" bson:"calendarType"`
	CalendarEventID string `json:"-" bson:"calendarEventID"`
}

// Calendar represents a calendar of any source
type Calendar struct {
	CalendarID string `json:"calendarId"`
	Name       string `json:"name"`
	IsActive   bool   `json:"isActive"`
}
