package main

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/tomyl/gocui"
	"github.com/tomyl/mort/store"
	"github.com/tomyl/xui"
)

const (
	timeFormat = "15:04"
	helptext   = `mört - a simple task manager and time tracker

Global keybindings
==================

F1      This help screen.
F2      Task screen.
F3      Timesheet screen.
Ctrl-C  Exit mort.
Ctrl-G  Cancel current operation.
Ctrl-L  Redraw screen.

Task view keybindings
=====================

Ctrl-N  Create new task.
Enter   Edit selected task.
/       Search task titles.
s       Search task bodies.

w       Toggle between day and week view.
Left    Go to previous day/week.
Right   Go to next day/week.

Ctrl-T  Toggle todo state of selected task.
t       Toggle display of todo tasks.
Ctrl-X  Toggle archive status of selected task.
x       Toggle display of archived tasks.
p       Toggle project filter.
q       Reset filters.

Ctrl-I  Clock in on selected task.
Ctrl-O  Clock out from currently active task.
i       Jump to active task.

Timesheet view keybindings
==========================

w       Toggle between day and week view.
Left    Go to previous day/week.
Right   Go to next day/week.

i       Edit clockin time.
o       Edit clockout time.
`
)

var todoStates = []string{"TODO", "WAIT", "DONE"}
var todoColors = []string{"\033[31m", "\033[33m", "\033[32m"}

func getStateIndex(state string) int {
	for i := range todoStates {
		if todoStates[i] == state {
			return i
		}
	}
	return -1
}

type restart struct {
	f              func(restart)
	task           *store.Task
	parentID       int64
	tmpl           string
	focus          string
	timesheetIndex int
}

func (r restart) Error() string {
	return "task command"
}

type mortApp struct {
	db *store.Store
	gx *xui.Xui

	help      *xui.ListWidget
	tasks     *tasksWidget
	timesheet *timesheetWidget
	status    *xui.TextWidget
	prompt    *xui.TextWidget

	Range          store.TimeRange
	FilterArchived bool
	FilterTitle    string
	FilterBody     string
	FilterRange    bool
	FilterProject  string
	FilterParentID int64
	FilterTodo     bool
}

func newMortApp(db *store.Store) *mortApp {

	app := &mortApp{
		db:        db,
		help:      &xui.ListWidget{},
		tasks:     &tasksWidget{},
		timesheet: &timesheetWidget{},
		status: &xui.TextWidget{
			FgColor: gocui.ColorWhite,
			BgColor: gocui.ColorBlue,
		},
		prompt: &xui.TextWidget{},
	}

	app.Range.Today()
	app.help.SetModel(strings.Split(helptext, "\n"))

	return app
}

func (app *mortApp) Run() error {
	var state restart

	for {
		err := app.runOnce(state)

		if r, ok := err.(restart); ok {
			r.f(r)
			state = r
			continue
		}

		return err
	}
}

func (app *mortApp) runOnce(state restart) error {
	g, err := gocui.NewGui(gocui.Output256)

	if err != nil {
		return err
	}

	defer g.Close()

	app.gx = xui.New(g)

	g.SetManagerFunc(xui.ResizeLayout(app.layout))

	if err := app.layout(g); err != nil {
		return err
	}

	if err := app.registerKeys(g); err != nil {
		return err
	}

	app.gx.SetPreActionHandler(func() {
		app.prompt.SetText("")
	})

	app.gx.SetPostActionHandler(func(err error) error {
		if err != nil {
			if err == gocui.ErrQuit {
				return err
			}
			if r, ok := err.(restart); ok {
				return r
			}
			log.Println(err)
			app.prompt.SetText(err.Error())
		}
		return nil
	})

	if state.focus == "" {
		app.loadTasks()
		if len(app.tasks.Model()) == 0 {
			app.gx.Focus(app.help.View())
			app.status.SetText("Help")
		} else {
			app.gx.Focus(app.tasks.View())
			if state.task != nil {
				app.tasks.SetCurrentByTaskID(state.task.ID)
			}
			app.setMessage("Press F1 for help.")
		}
	} else {
		app.gx.FocusName(state.focus)
		app.timesheet.SetCurrent(state.timesheetIndex)
	}

	return g.MainLoop()
}

func (app *mortApp) layout(g *gocui.Gui) error {
	center := xui.Region{0, 0, -1, -3}
	status := xui.Region{0, -2, -1, -2}
	prompt := xui.Region{0, -1, -1, -1}

	app.help.SetView(app.gx.SetRegionView("help", center))
	app.tasks.SetView(app.gx.SetRegionView("tasks", center))
	app.timesheet.SetView(app.gx.SetRegionView("timesheet", center))
	app.status.SetView(app.gx.SetRegionView("status", status))
	app.prompt.SetView(app.gx.SetRegionView("prompt", prompt))

	return app.gx.Err()
}

func (app *mortApp) registerKeys(g *gocui.Gui) error {
	// Global
	app.gx.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, xui.ErrorHandler(gocui.ErrQuit))

	app.gx.SetKeybinding("", gocui.KeyCtrlG, gocui.ModNone, xui.Handler(func() {
	}))

	app.gx.SetKeybinding("", gocui.KeyCtrlL, gocui.ModNone, xui.Handler(func() {
		app.layout(g)
	}))

	app.gx.SetKeybinding("", gocui.KeyF1, gocui.ModNone, xui.Handler(app.showHelpView))
	app.gx.SetKeybinding("", '1', gocui.ModNone, xui.Handler(app.showHelpView))

	app.gx.SetKeybinding("", gocui.KeyF2, gocui.ModNone, xui.Handler(app.showTasksView))
	app.gx.SetKeybinding("", '2', gocui.ModNone, xui.Handler(app.showTasksView))

	app.gx.SetKeybinding("", gocui.KeyF3, gocui.ModNone, xui.Handler(app.showTimesheetView))
	app.gx.SetKeybinding("", '3', gocui.ModNone, xui.Handler(app.showTimesheetView))

	// Tasks
	app.gx.SetWidgetAction(app.tasks, gocui.KeyArrowUp, gocui.ModNone, xui.ActionPreviousLine)
	app.gx.SetWidgetAction(app.tasks, gocui.KeyArrowDown, gocui.ModNone, xui.ActionNextLine)
	app.gx.SetWidgetAction(app.tasks, gocui.KeyPgup, gocui.ModNone, xui.ActionPreviousPage)
	app.gx.SetWidgetAction(app.tasks, gocui.KeyPgdn, gocui.ModNone, xui.ActionNextPage)

	app.gx.SetKeybinding("tasks", gocui.KeyArrowLeft, gocui.ModNone, xui.Handler(func() {
		app.Range.Prev()
		app.loadTasks()
	}))

	app.gx.SetKeybinding("tasks", gocui.KeyArrowRight, gocui.ModNone, xui.Handler(func() {
		app.Range.Next()
		app.loadTasks()
	}))

	app.gx.SetKeybinding("tasks", gocui.KeyEnter, gocui.ModNone, app.editCurrentTask)

	app.gx.SetKeybinding("tasks", 'd', gocui.ModNone, xui.Handler(func() {
		app.toggleParentFilter()
		app.loadTasks()
	}))

	app.gx.SetKeybinding("tasks", gocui.KeyCtrlI, gocui.ModNone, xui.Handler(func() {
		app.clockIn()
	}))

	app.gx.SetKeybinding("tasks", 'i', gocui.ModNone, xui.Handler(func() {
		app.goToActive()
	}))

	app.gx.SetKeybinding("tasks", gocui.KeyCtrlL, gocui.ModNone, xui.Handler(func() {
		app.loadTasks()
	}))

	app.gx.SetKeybinding("tasks", gocui.KeyCtrlN, gocui.ModNone, app.newTask)

	app.gx.SetKeybinding("tasks", gocui.KeyCtrlO, gocui.ModNone, xui.Handler(func() {
		app.clockOut()
	}))

	app.gx.SetKeybinding("tasks", 'p', gocui.ModNone, xui.Handler(func() {
		app.toggleProjectFilter()
		app.loadTasks()
	}))

	app.gx.SetKeybinding("tasks", 'q', gocui.ModNone, xui.Handler(func() {
		app.resetFilters()
	}))

	app.gx.SetKeybinding("tasks", 's', gocui.ModNone, xui.Handler(func() {
		callback := func(success bool, response string) {
			if success {
				app.setBodyFilter(response)
			} else {
				app.setMessage("Cancelled.")
			}
		}

		app.prompt.SetPrompt(g, "Search body: ", "", callback)
	}))

	app.gx.SetKeybinding("tasks", gocui.KeyCtrlT, gocui.ModNone, xui.Handler(func() {
		app.toggleTodoState()
	}))

	app.gx.SetKeybinding("tasks", 't', gocui.ModNone, xui.Handler(func() {
		app.FilterTodo = !app.FilterTodo
		app.loadTasks()
	}))

	app.gx.SetKeybinding("tasks", 'v', gocui.ModNone, xui.Handler(func() {
		app.openCurrentTask()
	}))

	app.gx.SetKeybinding("tasks", 'w', gocui.ModNone, xui.Handler(func() {
		app.toggleTasksDateRange()
		app.loadTasks()
	}))

	app.gx.SetWidgetKeybinding(app.tasks, gocui.KeyCtrlX, gocui.ModNone, app.toggleArchived)

	app.gx.SetKeybinding("tasks", 'x', gocui.ModNone, xui.Handler(func() {
		app.FilterArchived = !app.FilterArchived
		app.loadTasks()
	}))

	app.gx.SetKeybinding("tasks", '/', gocui.ModNone, xui.Handler(func() {
		callback := func(success bool, response string) {
			if success {
				app.setTitleFilter(response)
			} else {
				app.setMessage("Cancelled.")
			}
		}

		app.prompt.SetPrompt(g, "Search title: ", "", callback)
	}))

	// Timesheet
	app.gx.SetWidgetAction(app.timesheet, gocui.KeyArrowUp, gocui.ModNone, xui.ActionPreviousLine)
	app.gx.SetWidgetAction(app.timesheet, gocui.KeyArrowDown, gocui.ModNone, xui.ActionNextLine)
	app.gx.SetWidgetAction(app.timesheet, gocui.KeyPgup, gocui.ModNone, xui.ActionPreviousPage)
	app.gx.SetWidgetAction(app.timesheet, gocui.KeyPgdn, gocui.ModNone, xui.ActionNextPage)

	app.gx.SetKeybinding("timesheet", gocui.KeyArrowLeft, gocui.ModNone, xui.Handler(func() {
		app.Range.Prev()
		app.loadTimesheet()
	}))

	app.gx.SetKeybinding("timesheet", gocui.KeyArrowRight, gocui.ModNone, xui.Handler(func() {
		app.Range.Next()
		app.loadTimesheet()
	}))

	app.gx.SetKeybinding("timesheet", gocui.KeyEnter, gocui.ModNone, app.editCurrentTimesheetTask)

	app.gx.SetKeybinding("timesheet", 'i', gocui.ModNone, xui.Handler(func() {
		app.editClockin(g)
	}))

	app.gx.SetKeybinding("timesheet", 'o', gocui.ModNone, xui.Handler(func() {
		app.editClockout(g)
	}))

	app.gx.SetKeybinding("timesheet", gocui.KeyCtrlL, gocui.ModNone, xui.Handler(func() {
		app.loadTimesheet()
	}))

	app.gx.SetKeybinding("timesheet", 'v', gocui.ModNone, xui.Handler(func() {
		app.openCurrentTimesheetTask()
	}))

	app.gx.SetKeybinding("timesheet", 'w', gocui.ModNone, xui.Handler(func() {
		app.toggleTimesheetDateRange()
		app.loadTimesheet()
	}))

	return app.gx.Err()
}

func (app *mortApp) showHelpView() {
	app.gx.Focus(app.help.View())
	app.status.SetText("Help")
}

func (app *mortApp) showTasksView() {
	app.gx.Focus(app.tasks.View())
	app.loadTasks()
}

func (app *mortApp) showTimesheetView() {
	app.gx.Focus(app.timesheet.View())
	app.loadTimesheet()
}

func (app *mortApp) setMessage(pat string, params ...interface{}) {
	msg := fmt.Sprintf(pat, params...)
	app.prompt.SetText(msg)
}

func (app *mortApp) loadTasks() error {
	query := store.TaskQuery{
		Archived:    app.FilterArchived,
		SearchTitle: app.FilterTitle,
		SearchBody:  app.FilterBody,
		Project:     app.FilterProject,
		ParentID:    app.FilterParentID,
		Todo:        app.FilterTodo,
	}

	if app.FilterRange {
		query.Range = &app.Range
	}

	current := app.tasks.CurrentTask()
	tasks, err := app.db.GetTasks(query)

	if err != nil {
		return err
	}

	app.tasks.SetModel(tasks)

	if current != nil {
		app.tasks.SetCurrentByTaskID(current.ID)
	}

	app.showTasksQuery()

	return nil
}

func (app *mortApp) resetFilters() {
	app.FilterArchived = false
	app.FilterTitle = ""
	app.FilterBody = ""
	app.FilterRange = false
	app.FilterProject = ""
	app.FilterParentID = 0
	app.loadTasks()
}

func (app *mortApp) showTasksQuery() {
	var msg string
	if app.FilterTodo {
		msg = fmt.Sprintf("%d todo tasks", len(app.tasks.model))
	} else {
		msg = fmt.Sprintf("%d tasks", len(app.tasks.model))
	}
	filters := make([]string, 0)
	if !app.FilterArchived {
		filters = append(filters, "Archived hidden")
	}
	if app.FilterProject != "" {
		filters = append(filters, "Project="+app.FilterProject)
	}
	if app.FilterParentID != 0 {
		filters = append(filters, fmt.Sprintf("Parent=%d", app.FilterParentID))
	}
	if app.FilterRange {
		filters = append(filters, fmt.Sprintf("Date=%s", app.Range.String()))
	}
	if app.FilterTitle != "" {
		filters = append(filters, fmt.Sprintf("Title=*%s*", app.FilterTitle))
	}
	if app.FilterBody != "" {
		filters = append(filters, fmt.Sprintf("Body=*%s*", app.FilterBody))
	}
	if len(filters) > 0 {
		msg += " | " + strings.Join(filters, " ")
	}
	app.status.SetText(msg)
}

func (app *mortApp) toggleArchived() error {
	task := app.tasks.CurrentTask()

	if task == nil {
		return errors.New("no task")
	}

	var err error
	if task.ArchivedAt != nil {
		err = app.db.SetArchived(task.ID, false)
	} else {
		err = app.db.SetArchived(task.ID, true)
	}

	if err != nil {
		return err
	}

	task, err = app.refreshTask(task.ID)

	if err != nil {
		return err
	}

	if task != nil {
		app.tasks.render()
		if task.ArchivedAt != nil {
			app.setMessage("Archived")
		} else {
			app.setMessage("Unarchived")
		}
	}

	return nil
}

func (app *mortApp) refreshTask(id int64) (*store.Task, error) {
	idx := app.tasks.GetTaskIndexByID(id)

	if idx >= 0 {
		task, err := app.db.GetTaskByID(id)

		if err != nil {
			return nil, err
		}

		app.tasks.SetTaskAt(idx, task)

		return task, nil
	}

	return nil, nil
}

func (app *mortApp) setTitleFilter(needle string) error {
	app.FilterTitle = needle
	return app.loadTasks()
}

func (app *mortApp) setBodyFilter(needle string) error {
	app.FilterBody = needle
	return app.loadTasks()
}

func (app *mortApp) newTask(g *gocui.Gui, view *gocui.View) error {
	log.Printf("new task")
	return restart{f: app.createTask}
}

func (app *mortApp) createTask(cmd restart) {
	body, err := store.GetDraft(0, true)

	if err != nil {
		app.setMessage("Failed to get draft: %v", err)
		return
	}

	if body == cmd.tmpl {
		app.setMessage("No content.")
		return
	}

	if body == "" {
		app.setMessage("Empty content.")
		return
	}

	var payload store.Task
	payload.Title = store.GetTitleFromBody(body)
	payload.Project = store.GetProjectFromTitle(payload.Title)
	payload.Body = body

	if cmd.parentID > 0 {
		payload.ParentID = &cmd.parentID
	}

	_, err = app.db.CreateTask(payload)

	if err != nil {
		app.setMessage("Failed to store task: %v", err)
		return
	}

	store.SetDraft(0, "")
	app.resetFilters()
	app.setMessage("Created task.")
}

func (app *mortApp) editCurrentTask(g *gocui.Gui, view *gocui.View) error {
	task := app.tasks.CurrentTask()

	if task == nil {
		app.setMessage("No task.")
		return nil
	}

	return restart{f: app.editTask, task: task}
}

func (app *mortApp) getCurrentTask() *store.Task {
	task := app.tasks.CurrentTask()

	if task == nil {
		app.setMessage("No task.")
		return nil
	}

	return task
}

func (app *mortApp) editCurrentTimesheetTask(g *gocui.Gui, view *gocui.View) error {
	task := app.getCurrentTimesheetTask()

	if task != nil {
		return nil
	}

	focus := ""

	if g.CurrentView() != nil {
		focus = g.CurrentView().Name()
	}

	return restart{
		f:              app.editTask,
		task:           task,
		focus:          focus,
		timesheetIndex: app.timesheet.Current(),
	}
}

func (app *mortApp) getCurrentTimesheetTask() *store.Task {
	entry := app.timesheet.CurrentEntry()

	if entry == nil {
		app.setMessage("No timesheet entry.")
		return nil
	}

	if entry.TaskID == 0 {
		app.setMessage("No task.")
		return nil
	}

	task, err := app.db.GetTaskByID(entry.TaskID)

	if err != nil {
		app.setMessage("Failed to load task: %v", err)
		return nil
	}

	return task
}

func (app *mortApp) editTask(r restart) {
	task := r.task
	store.SetDraft(task.ID, task.Body)
	body, err := store.GetDraft(task.ID, false)

	if err != nil {
		app.setMessage("Failed to get draft: %v", err)
		return
	}

	if body == "" {
		app.setMessage("Empty body.")
		return
	}

	if body == task.Body {
		app.setMessage("No change.")
		return
	}

	var payload store.Task
	payload.Title = store.GetTitleFromBody(body)
	payload.Project = store.GetProjectFromTitle(payload.Title)
	payload.Body = body

	if err := app.db.UpdateTaskByID(task.ID, payload); err != nil {
		app.setMessage("Failed to update task: %v", task)
		return
	}

	app.resetFilters()
	app.setMessage("Updated task.")
}

func (app *mortApp) toggleTodoState() {
	task := app.tasks.CurrentTask()

	if task == nil {
		app.setMessage("No task.")
		return
	}

	newStateIdx := 0
	newState := todoStates[newStateIdx]

	if task.State != nil {
		idx := getStateIndex(*task.State)
		if idx >= 0 {
			if idx < len(todoStates)-1 {
				newStateIdx = idx + 1
				newState = todoStates[newStateIdx]
			} else {
				newState = ""
			}
		}
	}

	if err := app.db.SetTodoState(task.ID, newStateIdx, newState); err != nil {
		app.setMessage("Failed to toggle state: %s", err)
		return
	}

	task, err := app.refreshTask(task.ID)

	if err != nil {
		return
	}

	if task != nil {
		app.tasks.render()
	}

	app.setMessage("")
}

func (app *mortApp) clockIn() {
	task := app.tasks.CurrentTask()

	if task == nil {
		app.setMessage("No task.")
		return
	}

	if err := app.db.Clockin(task.ID); err != nil {
		app.setMessage("Failed to clock in: %s", err)
		return
	}

	app.loadTasks()
	app.setMessage("Clocked in.")
}

func (app *mortApp) clockOut() {
	taskID, err := app.db.GetActiveTaskID()

	if err != nil {
		app.setMessage("Failed to get active task: %v", err)
		return
	}

	if taskID == 0 {
		app.setMessage("Not clocked in.")
		return
	}

	if err := app.db.Clockout(); err != nil {
		app.setMessage("Failed to clock out: %s", err)
		return
	}

	app.loadTasks()
	app.setMessage("Clocked out.")
}

func (app *mortApp) goToActive() {
	taskID, err := app.db.GetActiveTaskID()

	if err != nil {
		app.setMessage("Failed to get active task: %s", err)
		return
	}

	if taskID == 0 {
		app.setMessage("Not clocked in.")
		return
	}

	idx := app.tasks.GetTaskIndexByID(taskID)

	if idx < 0 {
		app.setMessage("Active task not visible.")
		return
	}

	app.tasks.SetCurrent(idx)
}

func (app *mortApp) loadTimesheet() {
	app.showTimesheetQuery()
	entries, err := app.db.GetTimesheet(app.Range)

	if err != nil {
		app.setMessage("Failed to load timesheet: %v", err)
		return
	}

	app.timesheet.SetModel(entries)
}

func (app *mortApp) showTimesheetQuery() {
	msg := "Timesheet"
	filters := make([]string, 0)
	filters = append(filters, fmt.Sprintf("range=%s", app.Range.String()))
	if len(filters) > 0 {
		msg += " filter:" + strings.Join(filters, ",")
	}
	app.status.SetText(msg)
}

func (app *mortApp) toggleTasksDateRange() {
	if app.FilterRange {
		if app.Range.Days <= 1 {
			app.Range.Week()
		} else {
			app.FilterRange = false
		}
	} else {
		app.FilterRange = true
		app.Range.Today()
	}
}

func (app *mortApp) toggleTimesheetDateRange() {
	if app.Range.Days > 1 {
		app.Range.Today()
	} else {
		app.Range.Week()
	}
}

func (app *mortApp) editClockin(g *gocui.Gui) {
	entry := app.timesheet.CurrentEntry()

	if entry != nil {
		t := toTime(entry.ClockinAt)

		callback := func(success bool, response string) {
			if success && response != "" {
				app.timesheetEdit(entry, response, true)
				app.loadTimesheet()
			} else {
				app.setMessage("Cancelled.")
			}
		}

		app.prompt.SetPrompt(g, "", t, callback)
	}
}

func (app *mortApp) editClockout(g *gocui.Gui) {
	entry := app.timesheet.CurrentEntry()

	if entry != nil && entry.ClockoutAt != nil {
		t := toTime(*entry.ClockoutAt)

		callback := func(success bool, response string) {
			if success && response != "" {
				app.timesheetEdit(entry, response, false)
				app.loadTimesheet()
			} else {
				app.setMessage("Cancelled.")
			}
		}

		app.prompt.SetPrompt(g, "", t, callback)
	}
}

func (app *mortApp) timesheetEdit(entry *store.TimesheetEntry, s string, clockin bool) {
	t := entry.ClockinAt.Local()

	var h, m int
	fmt.Sscanf(s, "%d:%d", &h, &m)
	t = time.Date(t.Year(), t.Month(), t.Day(), h, m, t.Second(), t.Nanosecond(), t.Location())

	if clockin {
		entry.ClockinAt = t
	} else {
		entry.ClockoutAt = &t
	}

	if err := app.db.UpdateTimesheet(entry.ID, &entry.ClockinAt, entry.ClockoutAt); err != nil {
		app.setMessage("Failed to update timesheet: %v", err)
	}
}

func toTime(t time.Time) string {
	return t.Local().Format(timeFormat)
}

func (app *mortApp) toggleProjectFilter() {
	if app.FilterProject == "" {
		task := app.tasks.CurrentTask()
		if task == nil {
			app.setMessage("No task.")
			return
		}
		if task.Project == "" {
			app.setMessage("No project.")
			return
		}
		app.FilterProject = task.Project
	} else {
		app.FilterProject = ""
	}
}

func (app *mortApp) toggleParentFilter() {
	if app.FilterParentID <= 0 {
		task := app.tasks.CurrentTask()
		if task == nil {
			app.setMessage("No task.")
			return
		}
		if task.ParentID != nil {
			app.FilterParentID = *task.ParentID
		} else {
			app.FilterParentID = task.ID
		}
	} else {
		app.FilterParentID = 0
	}
}

func (app *mortApp) openCurrentTask() {
	task := app.getCurrentTask()

	if task != nil {
		app.openTask(task)
	}
}

func (app *mortApp) openCurrentTimesheetTask() {
	task := app.getCurrentTimesheetTask()

	if task != nil {
		app.openTask(task)
	}
}

func (app *mortApp) openTask(task *store.Task) {
	cmd := exec.Command("open-mort", task.Title)

	if err := cmd.Run(); err != nil {
		app.setMessage("Failed to execute open-mort: %v", err)
		return
	}

	app.setMessage("Executed open-mort.")
}
