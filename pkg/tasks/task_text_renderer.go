package tasks

import "fmt"

// TaskTextRenderer renders event titles based on the tasks state
type TaskTextRenderer struct{}

// RenderDueEventTitle renders a title of a tasks due date
func (t *TaskTextRenderer) RenderDueEventTitle(task *Task) string {
	var icon = "📅"

	if task.IsDone {
		icon += "✅"
	}

	return fmt.Sprintf("%s %s is due", icon, task.Name)
}

// RenderWorkUnitEventTitle renders a title of a work unit event
func (t *TaskTextRenderer) RenderWorkUnitEventTitle(task *Task, workUnit *WorkUnit) string {
	var icon = "⚙️"

	if workUnit.IsDone {
		icon += "✅"
	}

	return fmt.Sprintf("%s %s", icon, task.Name)
}

// HasReminder returns true if the Task or WorkUnit implementing Done is done
func (t *TaskTextRenderer) HasReminder(element Done) bool {
	return !element.CheckDone()
}
