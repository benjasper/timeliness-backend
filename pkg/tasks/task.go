package tasks

import (
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sort"
	"time"
)

// RoleGuest guest role can only view the task and not edit it
const RoleGuest = "guest"

// RoleEditor can edit the description and title
const RoleEditor = "editor"

// RoleMaintainer can edit everything
const RoleMaintainer = "maintainer"

// Role is the constants starting with role
type Role string

// AgendaWorkUnit is a type to keep apart dates
const AgendaWorkUnit = "WORK_UNIT"

// AgendaDueAt is a type to keep apart dates
const AgendaDueAt = "DUE_AT"

// Task is the model for a task
type Task struct {
	// TODO: More validation
	ID             primitive.ObjectID   `bson:"_id" json:"id"`
	UserID         primitive.ObjectID   `json:"userId" bson:"userId" validate:"required"`
	CreatedAt      time.Time            `json:"createdAt" bson:"createdAt"`
	LastModifiedAt time.Time            `json:"lastModifiedAt" bson:"lastModifiedAt"`
	Deleted        bool                 `json:"deleted" bson:"deleted"`
	Name           string               `json:"name" bson:"name" validate:"required"`
	Description    string               `json:"description" bson:"description"`
	IsDone         bool                 `json:"isDone" bson:"isDone"`
	Tags           []primitive.ObjectID `json:"tags" bson:"tags"`
	Collaborators  Collaborators        `json:"collaborators" bson:"collaborators"`

	WorkloadOverall time.Duration  `json:"workloadOverall" bson:"workloadOverall"`
	NotScheduled    time.Duration  `json:"notScheduled" bson:"notScheduled"`
	DueAt           calendar.Event `json:"dueAt" bson:"dueAt" validate:"required"`
	WorkUnits       WorkUnits      `json:"workUnits" bson:"workUnits"`
}

// TaskAgenda is the model for the agenda task view
type TaskAgenda struct {
	ID             primitive.ObjectID   `bson:"_id" json:"id"`
	UserID         primitive.ObjectID   `json:"userId" bson:"userId" validate:"required"`
	CreatedAt      time.Time            `json:"createdAt" bson:"createdAt"`
	LastModifiedAt time.Time            `json:"lastModifiedAt" bson:"lastModifiedAt"`
	Deleted        bool                 `json:"deleted" bson:"deleted"`
	Name           string               `json:"name" bson:"name" validate:"required"`
	Description    string               `json:"description" bson:"description"`
	IsDone         bool                 `json:"isDone" bson:"isDone"`
	Tags           []primitive.ObjectID `json:"tags" bson:"tags"`
	Collaborators  Collaborators        `json:"collaborators" bson:"collaborators"`

	WorkloadOverall time.Duration  `json:"workloadOverall" bson:"workloadOverall"`
	NotScheduled    time.Duration  `json:"notScheduled" bson:"notScheduled"`
	DueAt           calendar.Event `json:"dueAt" bson:"dueAt" validate:"required"`
	WorkUnits       WorkUnits      `json:"workUnits" bson:"workUnits"`

	Date          calendar.AgendaEvent `json:"date" bson:"date"`
	WorkUnitIndex int                  `json:"workUnitIndex" bson:"workUnitIndex"`
}

// TaskUnwound is the model for a task that only has a single work unit extracted
type TaskUnwound struct {
	ID             primitive.ObjectID   `bson:"_id" json:"id"`
	UserID         primitive.ObjectID   `json:"userId" bson:"userId" validate:"required"`
	CreatedAt      time.Time            `json:"createdAt" bson:"createdAt"`
	LastModifiedAt time.Time            `json:"lastModifiedAt" bson:"lastModifiedAt"`
	Deleted        bool                 `json:"deleted" bson:"deleted"`
	Name           string               `json:"name" bson:"name" validate:"required"`
	Description    string               `json:"description" bson:"description"`
	IsDone         bool                 `json:"isDone" bson:"isDone"`
	Tags           []primitive.ObjectID `json:"tags" bson:"tags"`
	Collaborators  Collaborators        `json:"collaborators" bson:"collaborators"`

	WorkloadOverall time.Duration  `json:"workloadOverall" bson:"workloadOverall"`
	NotScheduled    time.Duration  `json:"notScheduled" bson:"notScheduled"`
	DueAt           calendar.Event `json:"dueAt" bson:"dueAt" validate:"required"`
	WorkUnit        WorkUnit       `json:"workUnit" bson:"workUnit"`
	WorkUnits       WorkUnits      `json:"workUnits" bson:"workUnits"`
	WorkUnitsIndex  int            `json:"workUnitsIndex" bson:"workUnitsIndex"`
	WorkUnitsCount  int            `json:"workUnitsCount" bson:"workUnitsCount"`
}

// TaskUpdate is the view of a task for an update
type TaskUpdate struct {
	ID             primitive.ObjectID   `bson:"_id" json:"-"`
	UserID         primitive.ObjectID   `bson:"userId" json:"-"`
	CreatedAt      time.Time            `bson:"createdAt" json:"-"`
	LastModifiedAt time.Time            `bson:"lastModifiedAt" json:"-"`
	Deleted        bool                 `json:"-" bson:"deleted"`
	Name           string               `json:"name" bson:"name" validate:"required"`
	Description    string               `json:"description" bson:"description"`
	IsDone         bool                 `json:"isDone" bson:"isDone"`
	Tags           []primitive.ObjectID `json:"tags" bson:"tags"`
	Collaborators  Collaborators        `json:"-" bson:"collaborators"`

	WorkloadOverall time.Duration  `json:"workloadOverall" bson:"workloadOverall"`
	NotScheduled    time.Duration  `json:"notScheduled" bson:"notScheduled"`
	DueAt           calendar.Event `json:"dueAt" bson:"dueAt" validate:"required"`
	WorkUnits       WorkUnits      `json:"-" bson:"workUnits"`
}

// Collaborator is a contact that is part of a task
type Collaborator struct {
	UserID primitive.ObjectID `json:"userId" bson:"userId"`
	Role   Role               `json:"role" bson:"role"`
}

// Collaborators is a slice of multiple Collaborator
type Collaborators []Collaborator

// IncludesUser checks if Collaborators includes a User
func (c Collaborators) IncludesUser(userID string) bool {
	for _, collaborator := range c {
		if collaborator.UserID.Hex() == userID {
			return true
		}
	}

	return false
}

// WorkUnits represents an array of WorkUnit
type WorkUnits []WorkUnit

// Add adds a WorkUnit to the slice
func (w WorkUnits) Add(unit *WorkUnit) WorkUnits {
	if len(w) == 0 {
		return append(w, *unit)
	}

	i := sort.Search(len(w), func(i int) bool {
		return w[i].ScheduledAt.Date.Start.After(unit.ScheduledAt.Date.Start)
	})

	array := append(w, WorkUnit{})
	copy(array[i+1:], array[i:])
	array[i] = *unit

	return array
}

// RemoveByIndex removes an entry by index
func (w WorkUnits) RemoveByIndex(index int) WorkUnits {
	return append(w[:index], w[index+1:]...)
}

// FindByCalendarID finds a single work unit by its calendar event ID
func (w WorkUnits) FindByCalendarID(calendarID string) (int, *WorkUnit) {
	for i, unit := range w {
		for _, cEvent := range unit.ScheduledAt.CalendarEvents {
			if cEvent.CalendarEventID == calendarID {
				return i, &unit
			}
		}
	}

	return -1, nil
}

// FindByID finds a single work unit by its ID
func (w WorkUnits) FindByID(ID string) (int, *WorkUnit) {
	for i, unit := range w {
		if unit.ID.Hex() == ID {
			return i, &unit
		}
	}

	return -1, nil
}

// FindByEventIntersection finds a single work unit by it
func (w WorkUnits) FindByEventIntersection(event *calendar.Event) ([]int, []WorkUnit) {
	var workUnits []WorkUnit
	var indices []int

	for i, unit := range w {
		if unit.ScheduledAt.Date.IntersectsWith(event.Date) {
			indices = append(indices, i)
			workUnits = append(workUnits, unit)
		}
	}

	return indices, workUnits
}

// Tags is an array of ObjectIDs
type Tags []primitive.ObjectID

// Add adds a WorkUnit to the slice
func (tags Tags) Add(tag primitive.ObjectID) Tags {
	return append(tags, tag)
}

// FindByID finds a ObjectID
func (tags Tags) FindByID(ID primitive.ObjectID) (int, *primitive.ObjectID) {
	for i, tag := range tags {
		if tag == ID {
			return i, &tag
		}
	}

	return -1, nil
}

// RemoveByIndex removes an entry by index
func (tags Tags) RemoveByIndex(index int) Tags {
	return append(tags[:index], tags[index+1:]...)
}
