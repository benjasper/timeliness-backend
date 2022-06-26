package tasks

import (
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/timeliness-app/timeliness-backend/pkg/date"
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

// Done is an interface that allows to check the done status of a Task or WorkUnit
type Done interface {
	CheckDone() bool
}

// Task is the model for a task
type Task struct {
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

// Validate validates the task and checks the bounds of the fields
func (t *Task) Validate() error {
	if t.DueAt.Date.Start.Before(time.Now()) {
		return errors.New("due date can't be in the past")
	}

	if t.DueAt.Date.Start.After(time.Now().AddDate(2, 0, 0)) {
		return errors.New("due date can't be more than two years in the future")
	}

	if t.WorkloadOverall > time.Hour*24 {
		return errors.New("workload can't be more than 24 hours")
	}

	return nil
}

// CheckDone checks if the task is done
func (t *Task) CheckDone() bool {
	return t.IsDone
}

// MarshalJSON marshals the task to json
func (t *Task) MarshalJSON() ([]byte, error) {
	type Alias Task

	a := struct {
		Alias
	}{
		Alias: (Alias)(*t),
	}

	if a.WorkUnits == nil {
		a.WorkUnits = make(WorkUnits, 0)
	}

	return json.Marshal(a)
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
	NotScheduled    time.Duration  `json:"-" bson:"notScheduled"`
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

// RemoveByID removes an entry by its ID
func (w WorkUnits) RemoveByID(id string) WorkUnits {
	for i, unit := range w {
		if unit.ID.Hex() == id {
			return w.RemoveByIndex(i)
		}
	}

	return w
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
func (w WorkUnits) FindByEventIntersection(event *calendar.Event, ignoreWorkUnitID primitive.ObjectID) ([]int, []WorkUnit) {
	var workUnits []WorkUnit
	var indices []int

	for i, unit := range w {
		if !unit.IsDone && unit.ScheduledAt.Date.IntersectsWith(event.Date) && unit.ID != ignoreWorkUnitID {
			indices = append(indices, i)
			workUnits = append(workUnits, unit)
		}
	}

	return indices, workUnits
}

// Sort sorts the WorkUnits by their scheduledAt date
func (w WorkUnits) Sort() {
	sort.Slice(w, func(i, j int) bool {
		return w[i].ScheduledAt.Date.Start.Before(w[j].ScheduledAt.Date.Start)
	})
}

// Timespans gets all the timespans of the WorkUnits
func (w WorkUnits) Timespans() []date.Timespan {
	var timespans []date.Timespan

	for _, unit := range w {
		timespans = append(timespans, unit.ScheduledAt.Date)
	}

	return timespans
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
