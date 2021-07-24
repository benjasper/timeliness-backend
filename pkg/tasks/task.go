package tasks

import (
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sort"
	"time"
)

// Task is the model for a task
type Task struct {
	// TODO: More validation
	ID             primitive.ObjectID `bson:"_id" json:"id"`
	UserID         primitive.ObjectID `json:"userId" bson:"userId" validate:"required"`
	CreatedAt      time.Time          `json:"createdAt" bson:"createdAt"`
	LastModifiedAt time.Time          `json:"lastModifiedAt" bson:"lastModifiedAt"`
	Deleted        bool               `json:"deleted" bson:"deleted"`
	Name           string             `json:"name" bson:"name" validate:"required"`
	Description    string             `json:"description" bson:"description"`
	IsDone         bool               `json:"isDone" bson:"isDone"`
	Tags           []string           `json:"tags" bson:"tags"`

	Priority        int            `json:"priority" bson:"priority" validate:"required"`
	WorkloadOverall time.Duration  `json:"workloadOverall" bson:"workloadOverall"`
	NotScheduled    time.Duration  `json:"notScheduled" bson:"notScheduled"`
	DueAt           calendar.Event `json:"dueAt" bson:"dueAt" validate:"required"`
	WorkUnits       WorkUnits      `json:"workUnits" bson:"workUnits"`
}

// TaskUnwound is the model for a task that only has a single work unit extracted
type TaskUnwound struct {
	ID             primitive.ObjectID `bson:"_id" json:"id"`
	UserID         primitive.ObjectID `json:"userId" bson:"userId" validate:"required"`
	CreatedAt      time.Time          `json:"createdAt" bson:"createdAt"`
	LastModifiedAt time.Time          `json:"lastModifiedAt" bson:"lastModifiedAt"`
	Deleted        bool               `json:"deleted" bson:"deleted"`
	Name           string             `json:"name" bson:"name" validate:"required"`
	Description    string             `json:"description" bson:"description"`
	IsDone         bool               `json:"isDone" bson:"isDone"`
	Tags           []string           `json:"tags" bson:"tags"`

	Priority        int            `json:"priority" bson:"priority" validate:"required"`
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
	ID             primitive.ObjectID `bson:"_id" json:"-"`
	UserID         primitive.ObjectID `bson:"userId" json:"-"`
	CreatedAt      time.Time          `bson:"createdAt" json:"-"`
	LastModifiedAt time.Time          `bson:"lastModifiedAt" json:"-"`
	Deleted        bool               `json:"deleted" bson:"deleted"`
	Name           string             `json:"name" bson:"name" validate:"required"`
	Description    string             `json:"description" bson:"description"`
	IsDone         bool               `json:"isDone" bson:"isDone"`
	Tags           []string           `json:"tags" bson:"tags"`

	Priority        int            `json:"priority" bson:"priority" validate:"required"`
	WorkloadOverall time.Duration  `json:"workloadOverall" bson:"workloadOverall"`
	NotScheduled    time.Duration  `json:"notScheduled" bson:"notScheduled"`
	DueAt           calendar.Event `json:"dueAt" bson:"dueAt" validate:"required"`
	WorkUnits       WorkUnits      `json:"-" bson:"workUnits"`
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
		if unit.ScheduledAt.CalendarEventID == calendarID {
			return i, &unit
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
