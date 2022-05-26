package users

import (
	"fmt"
	"github.com/pkg/errors"
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

// CalendarConnectionStatusMissingScopes marks that a calendar connection is missing scopes
const CalendarConnectionStatusMissingScopes = "missing_scopes"

// User represents the user
type User struct {
	ID             primitive.ObjectID `json:"id" bson:"_id"`
	Firstname      string             `json:"firstname" validate:"required"`
	Lastname       string             `json:"lastname" validate:"required"`
	Password       string             `json:"-" bson:"password"`
	Email          string             `json:"email" validate:"required,email"`
	CreatedAt      time.Time          `json:"createdAt" bson:"createdAt" validate:"isdefault"`
	LastModifiedAt time.Time          `json:"lastModifiedAt" bson:"lastModifiedAt" validate:"isdefault"`
	LastRefreshAt  time.Time          `json:"-" bson:"lastRefreshAt" validate:"isdefault"`
	DeviceTokens   []DeviceToken      `json:"-" bson:"deviceTokens" validate:"isdefault"`
	IsDeactivated  bool               `json:"-" bson:"isDeactivated"`
	Contacts       []Contact          `json:"contacts" bson:"contacts"`
	Billing        Billing            `json:"billing" bson:"billing"`

	GoogleCalendarConnections GoogleCalendarConnections `json:"googleCalendarConnections" bson:"googleCalendarConnections"`
	Settings                  UserSettings              `json:"settings" bson:"settings"`
	EmailVerified             bool                      `json:"emailVerified" bson:"emailVerified"`
	EmailVerificationToken    string                    `json:"-" bson:"emailVerificationToken"`
}

// UserLogin is the view for users logger in
type UserLogin struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" bson:"password" validate:"required"`
}

//UserLoginGoogle is the view for users logging in with google
type UserLoginGoogle struct {
	Token string `json:"token" validate:"required"`
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

// GoogleCalendarConnections is a slice of GoogleCalendarConnection
type GoogleCalendarConnections []GoogleCalendarConnection

// FindByConnectionID finds a connection by it ID
func (g GoogleCalendarConnections) FindByConnectionID(connectionID string) (*GoogleCalendarConnection, int, error) {
	for i, connection := range g {
		if connectionID == connection.ID {
			return &connection, i, nil
		}
	}

	return nil, 0, fmt.Errorf("could not find connection with id %s", connectionID)
}

// GetTaskCalendarConnection finds a connection if its a task calendar connection
func (g GoogleCalendarConnections) GetTaskCalendarConnection() (*GoogleCalendarConnection, int, error) {
	for i, connection := range g {
		if connection.IsTaskCalendarConnection && connection.TaskCalendarID != "" {
			return &connection, i, nil
		}
	}

	return nil, 0, errors.Errorf("could not find task calendar connection")
}

// RemoveConnection removes a connection
func (g GoogleCalendarConnections) RemoveConnection(connectionID string) GoogleCalendarConnections {
	for i, connection := range g {
		if connectionID == connection.ID {
			return append(g[:i], g[i+1:]...)
		}
	}

	return g
}

// GoogleCalendarConnection stores everything related to Google Calendar
type GoogleCalendarConnection struct {
	ID                       string              `json:"id" bson:"_id"`
	Email                    string              `json:"email" bson:"email"`
	IsTaskCalendarConnection bool                `json:"isTaskCalendarConnection" bson:"isTaskCalendarConnection"`
	Status                   string              `json:"status" bson:"status"`
	Token                    oauth2.Token        `json:"-" bson:"token,omitempty"`
	StateToken               string              `json:"-" bson:"stateToken,omitempty"`
	TaskCalendarID           string              `json:"-" bson:"taskCalendarID,omitempty"`
	CalendarsOfInterest      GoogleCalendarSyncs `json:"-" bson:"calendarsOfInterest,omitempty"`
}

// GoogleCalendarSyncs is a slice of GoogleCalendarSync
type GoogleCalendarSyncs []GoogleCalendarSync

// HasCalendarWithID checks if a calendar exists in a sync
func (s GoogleCalendarSyncs) HasCalendarWithID(ID string) bool {
	for _, sync := range s {
		if sync.CalendarID == ID {
			return true
		}
	}

	return false
}

// RemoveCalendar removes a calendar sync
func (s GoogleCalendarSyncs) RemoveCalendar(ID string) GoogleCalendarSyncs {
	for i, sync := range s {
		if sync.CalendarID == ID {
			return append(s[:i], s[i+1:]...)
		}
	}

	return s
}

// GoogleCalendarSync holds information about a calendar that will be used to determine busy times
type GoogleCalendarSync struct {
	CalendarID     string    `json:"-" bson:"calendarId,omitempty"`
	SyncResourceID string    `json:"-" bson:"syncResourceId,omitempty"`
	ChannelID      string    `json:"-" bson:"channelId,omitempty"`
	SyncToken      string    `json:"-" bson:"syncToken,omitempty"`
	Expiration     time.Time `json:"-" bson:"expiration,omitempty"`
	IsNotSyncable  bool      `json:"-" bson:"isNotSyncable,omitempty"`
}

// TimingPreferenceVeryEarly is the very early timing preference
const TimingPreferenceVeryEarly = "veryEarly"

// TimingPreferenceEarly is the early timing preference
const TimingPreferenceEarly = "early"

// TimingPreferenceLate is the late timing preference
const TimingPreferenceLate = "late"

// TimingPreferenceVeryLate is the very late timing preference
const TimingPreferenceVeryLate = "veryLate"

// TimingPreferences represent all possible timing preferences
var TimingPreferences = []string{
	TimingPreferenceEarly,
	TimingPreferenceVeryEarly,
	TimingPreferenceLate,
	TimingPreferenceVeryLate,
}

// UserSettings hold different settings roughly separated by topics
type UserSettings struct {
	OnboardingCompleted bool               `json:"onboardingCompleted" bson:"onboardingCompleted"`
	Scheduling          SchedulingSettings `json:"scheduling" bson:"scheduling"`
}

// SchedulingSettings holds different settings for scheduling
type SchedulingSettings struct {
	TimingPreference     string          `json:"timingPreference" bson:"timingPreference"`
	TimeZone             string          `json:"timeZone" bson:"timeZone" validate:"required"`
	AllowedTimespans     []date.Timespan `json:"allowedTimespans" bson:"allowedTimespans"`
	BusyTimeSpacing      time.Duration   `json:"busyTimeSpacing" bson:"busyTimeSpacing"`
	MinWorkUnitDuration  time.Duration   `json:"minWorkUnitDuration" bson:"minWorkUnitDuration"`
	MaxWorkUnitDuration  time.Duration   `json:"maxWorkUnitDuration" bson:"maxWorkUnitDuration"`
	HideDeadlineWhenDone bool            `json:"hideDeadlineWhenDone" bson:"hideDeadlineWhenDone"`
}

// AppScopeFree is the free scope
var AppScopeFree = "free"

// AppScopePro is the full scope
var AppScopePro = "pro"

// BillingStatusTrial is the trial status
var BillingStatusTrial = "trial"

// BillingStatusPaymentProblem is when a payment failed
var BillingStatusPaymentProblem = "paymentProblem"

// BillingStatusSubscriptionActive is when a subscription is active
var BillingStatusSubscriptionActive = "subscriptionActive"

// BillingStatusSubscriptionCancelled is when a subscription is cancelled
var BillingStatusSubscriptionCancelled = "subscriptionCancelled"

// Billing is the billing information for stripe
type Billing struct {
	Status     string    `json:"status" bson:"status"`
	CustomerID string    `json:"-" bson:"customerId"`
	EndsAt     time.Time `json:"endsAt" bson:"endsAt"`
}

// IsExpired returns true if the billing is expired
func (b *Billing) IsExpired() bool {
	return b.EndsAt.Before(time.Now())
}
