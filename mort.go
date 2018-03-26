package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	xdg "github.com/queria/golang-go-xdg"

	"github.com/tomyl/mort/store"
	"github.com/tomyl/xl"
	"github.com/tomyl/xl/logger"
)

func cmdPauseActiveTask(db *store.Store) {
	pausedID, err := db.GetPausedTaskID()

	if err != nil {
		log.Fatalln(err)
	}

	if pausedID > 0 {
		if err := db.Clockin(pausedID); err != nil {
			log.Fatalln(err)
		}
		cmdCheckinDurationActive(db)
	} else {
		activeID, err := db.GetActiveTaskID()
		if err != nil {
			log.Fatalln(err)
		}
		if activeID > 0 {
			if _, err := db.Pause(); err != nil {
				log.Fatalln(err)
			}
			cmdCheckinDurationActive(db)
		}
	}
}

func cmdCheckinDurationActive(db *store.Store) {
	activeID, err := db.GetActiveTaskID()

	if err != nil {
		log.Fatalf("Get active task id: %v", err)
	}

	if activeID > 0 {
		task, err := db.GetTaskByID(activeID)
		if err != nil {
			log.Fatalf("Get active task: %v", err)
		}
		if task.ClockinAt != nil {
			delta := time.Now().Sub(*task.ClockinAt)
			fmt.Printf("%s +%s\n", task.Project, formatDuration(delta))
		}
	} else {
		pausedID, err := db.GetPausedTaskID()
		if err != nil {
			log.Fatalf("Get paused task id: %v", err)
		}
		if pausedID > 0 {
			task, err := db.GetTaskByID(pausedID)
			if err != nil {
				log.Fatalf("Get paused task: %v", err)
			}
			r := store.TimeRange{}
			r.Today()
			entries, err := db.GetTimesheet(r)
			var total time.Duration
			for _, e := range entries {
				if e.Project == task.Project && e.ClockoutAt != nil {
					total += e.ClockoutAt.Sub(e.ClockinAt)
				}
			}
			fmt.Printf("%s %s\n", task.Project, formatDuration(total))
		}
	}
}

func cmdCheckinDurationToday(db *store.Store) {
	r := store.TimeRange{}
	r.Today()
	entries, err := db.GetTimesheet(r)

	if err != nil {
		log.Fatalln(err)
	}

	var total time.Duration
	active := ""
	for _, e := range entries {
		if e.ClockoutAt != nil {
			total += e.ClockoutAt.Sub(e.ClockinAt)
		} else {
			total += time.Now().Sub(e.ClockinAt)
			active = "+"
		}
	}
	fmt.Printf("today %s%s\n", active, formatDuration(total))
}

func cmdRun(db *store.Store) {
	logpath, err := xdg.Data.Ensure("mort/mort.log")

	if err != nil {
		log.Fatalln(err)
	}

	f, err := os.OpenFile(logpath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)

	if err != nil {
		log.Fatalln(err)
	}

	defer f.Close()

	log.SetOutput(f)
	log.Printf("Starting %v", time.Now())

	xl.SetLogger(logger.Plain)

	app := newMortApp(db)

	if err := app.Run(); err != nil {
		log.Fatalln(err)
	}
}

func cmdNewTask(db *store.Store, project, title *string) {
	if project == nil || *project == "" {
		log.Fatalf("Please provide -project")
	}

	if title == nil || *title == "" {
		log.Fatalf("Please provide -title")
	}

	var payload store.Task
	payload.Project = *project
	payload.Title = fmt.Sprintf("%s: %s", *project, *title)
	payload.Body = fmt.Sprintf("%s: %s", *project, *title)

	taskID, err := db.CreateTask(payload)

	if err != nil {
		log.Fatalf("Failed to store task: %v", err)
		return
	}

	log.Printf("Created task %d", taskID)
}

func main() {
	newtask := flag.Bool("new", false, "Create new task")
	project := flag.String("project", "", "Project for new note")
	title := flag.String("title", "", "Title for new note")

	pause := flag.Bool("pause", false, "Pause current task (or clockin again)")
	clock := flag.Bool("clock", false, "Return current checkin duration")
	today := flag.Bool("today", false, "Return total checkin duration today")
	flag.Parse()

	db, err := store.Default()

	if err != nil {
		log.Fatalln(err)
	}

	switch {
	case *pause:
		cmdPauseActiveTask(db)
	case *clock:
		cmdCheckinDurationActive(db)
	case *today:
		cmdCheckinDurationToday(db)
	case *newtask:
		cmdNewTask(db, project, title)
	default:
		cmdRun(db)
	}
}
