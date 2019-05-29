package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tomyl/gocui"
	"github.com/tomyl/mort/store"
	"github.com/tomyl/xui"
)

type tasksWidget struct {
	base  xui.ScrollWidget
	model []store.Task
}

func (w *tasksWidget) View() *gocui.View {
	return w.base.View()
}

func (w *tasksWidget) SetView(view *gocui.View) {
	w.base.Highlight = true
	w.base.SetView(view)
	w.render()
}

func (w *tasksWidget) Model() []store.Task {
	return w.model
}

func (w *tasksWidget) SetModel(model []store.Task) {
	w.base.SetMax(len(model))
	w.model = model
	w.render()
}

func (w *tasksWidget) Current() int {
	return w.base.Current()
}

func (w *tasksWidget) SetCurrent(idx int) error {
	return w.base.SetCurrent(idx)
}

func (w *tasksWidget) CurrentTask() *store.Task {
	if len(w.model) > 0 {
		current := w.base.Current()
		if current >= 0 && current < len(w.model) {
			return &w.model[current]
		}
	}
	return nil
}

func (w *tasksWidget) GetTaskIndexByID(id int64) int {
	for idx, task := range w.model {
		if task.ID == id {
			return idx
		}
	}

	return -1
}

func (w *tasksWidget) SetTaskAt(idx int, task *store.Task) {
	if len(w.model) > 0 && idx < len(w.model) {
		w.model[idx] = *task
	}
}

func (w *tasksWidget) render() {
	view := w.base.View()

	if view != nil {
		view.Clear()
		sx, _ := view.Size()
		now := time.Now()

		for i, task := range w.model {
			if i > 0 {
				fmt.Fprintf(view, "\n")
			}

			ts := ""
			age := now.Sub(task.UpdatedAt)

			if age > 6*24*time.Hour {
				ts = task.UpdatedAt.Local().Format("Jan 02")
			} else if age > 23*time.Hour {
				ts = task.UpdatedAt.Local().Format("Mon   ")
			} else {
				ts = task.UpdatedAt.Local().Format("15:04 ")
			}

			prefix := "  "
			color := ""
			reset := "\033[0m"
			//color = "\033[0;37m"

			if task.ClockinAt != nil {
				color = "\033[1m"
				prefix = "A "
			} else if task.ArchivedAt != nil {
				// color = "\033[9m"
				// color = "\033[34m"
				color = "\033[38;5;240m"
				prefix = "X "
			}

			line := color + prefix + ts + " " + _escape(task.Title) + reset
			fmt.Fprintf(view, xui.Pad(line, sx))
		}
	}
}

func _escape(s string) string {
	return strings.ReplaceAll(s, "%", "%%")
}

func (w *tasksWidget) HandleAction(action string) error {
	return w.base.HandleAction(action)
}

type timesheetWidget struct {
	base    xui.ListWidget
	entries []store.TimesheetEntry
}

func (w *timesheetWidget) View() *gocui.View {
	return w.base.View()
}

func (w *timesheetWidget) SetView(view *gocui.View) {
	w.base.Highlight = true
	w.base.SetView(view)
}

func (w *timesheetWidget) Model() []store.TimesheetEntry {
	return w.entries
}

func (w *timesheetWidget) SetModel(entries []store.TimesheetEntry) {
	w.entries = entries

	lines := make([]string, 0)
	m := make(map[string]time.Duration, 0)
	var total time.Duration

	for _, e := range entries {
		day := e.ClockinAt.Local().Format("Jan 02 Mon")
		start := e.ClockinAt.Local().Format("15:04")
		end := ""
		diff := ""

		if e.ClockoutAt != nil {
			end = e.ClockoutAt.Local().Format("15:04")
			delta := e.ClockoutAt.Sub(e.ClockinAt)
			diff = formatDuration(delta)
			m[e.Project] += delta
			total += delta
		} else {
			delta := time.Now().Sub(e.ClockinAt)
			diff = formatDuration(delta)
			m[e.Project] += delta
			total += delta
		}

		line := fmt.Sprintf("%s %5s - %5s = %5s %s", day, start, end, diff, e.Title)
		lines = append(lines, line)
	}

	if len(m) > 0 {
		lines = append(lines, "\n")

		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, project := range keys {
			tot := m[project]
			line := fmt.Sprintf("%-15s %s", project, formatDuration(tot))
			lines = append(lines, line)
		}
		line := fmt.Sprintf("--------------- %s", formatDuration(total))
		lines = append(lines, line)
	}

	w.base.SetModel(lines)
}

func (w *timesheetWidget) Current() int {
	return w.base.Current()
}

func (w *timesheetWidget) SetCurrent(idx int) error {
	return w.base.SetCurrent(idx)
}

func (w *timesheetWidget) CurrentEntry() *store.TimesheetEntry {
	if len(w.entries) > 0 {
		current := w.base.Current()
		if current >= 0 && current < len(w.entries) {
			return &w.entries[current]
		}
	}
	return nil
}

func (w *timesheetWidget) HandleAction(action string) error {
	return w.base.HandleAction(action)
}

func formatDuration(d time.Duration) string {
	hour := d / time.Hour
	nsec := d % time.Hour
	min := nsec / time.Minute

	return fmt.Sprintf("%02d:%02d", hour, min)
}
