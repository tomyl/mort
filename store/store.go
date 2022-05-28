package store

import (
	"database/sql"
	"log"
	"os"
	"strings"
	"time"

	xdg "github.com/queria/golang-go-xdg"
	"github.com/tomyl/xl"
)

// Schema in SQLite dialect used by mort.
var Schema = `
CREATE TABLE IF NOT EXISTS task (
	id		     INTEGER PRIMARY KEY,
	created_at   TIMESTAMP NOT NULL DEFAULT current_timestamp,
	updated_at   TIMESTAMP NOT NULL DEFAULT current_timestamp,
	scheduled_at TIMESTAMP,
	clockin_at   TIMESTAMP,
	paused_at    TIMESTAMP,
	archived_at  TIMESTAMP,
	parent_id 	 INTEGER,
	project      TEXT NOT NULL,
	title        TEXT NOT NULL,
	body	     TEXT,
	state        TEXT,
	state_idx    INTEGER,

	FOREIGN KEY (parent_id) REFERENCES task (id)
);

CREATE TABLE IF NOT EXISTS timesheet (
	id		    INTEGER PRIMARY KEY,
	task_id		INTEGER NOT NULL,
	clockin_at  TIMESTAMP NOT NULL DEFAULT current_timestamp,
	clockout_at TIMESTAMP,

	FOREIGN KEY (task_id) REFERENCES task (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS timesheet_1 ON timesheet (task_id);

CREATE TABLE IF NOT EXISTS migrate (
    schema      TEXT PRIMARY KEY,
	version     INTEGER NOT NULL
);
`

// InitSchema creates the database schema.
func InitSchema(db *xl.DB) {
	statements := strings.Split(Schema, ";")
	for _, statement := range statements {
		statement = strings.TrimSpace(statement)
		if statement != "" {
			if _, err := db.Exec(statement); err != nil {
				log.Fatal(err)
			}
		}
	}
}

type Task struct {
	ID          int64      `db:"id"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	ScheduledAt *time.Time `db:"scheduled_at"`
	ClockinAt   *time.Time `db:"clockin_at"`
	ArchivedAt  *time.Time `db:"archived_at"`
	PausedAt    *time.Time `db:"paused_at"`
	ParentID    *int64     `db:"parent_id"`
	Project     string     `db:"project"`
	Title       string     `db:"title"`
	Body        string     `db:"body"`
	State       *string    `db:"state"`
	StateIdx    *int       `db:"state_idx"`

	ClockinAtOld *time.Time `db:"clockedin_at"`
}

type TaskQuery struct {
	Project     string
	Archived    bool
	Todo        bool
	ParentID    int64
	SearchTitle string
	SearchBody  string
	Range       *TimeRange
}

type TimesheetEntry struct {
	ID         int64      `db:"id"`
	TaskID     int64      `db:"task_id"`
	ClockinAt  time.Time  `db:"clockin_at"`
	ClockoutAt *time.Time `db:"clockout_at"`

	Project string `db:"project"`
	Title   string `db:"title"`
}

type Store struct {
	db *xl.DB
}

func New(db *xl.DB) *Store {
	return &Store{db}
}

func Default() (*Store, error) {
	dbpath := os.Getenv("MORT_DB")

	if dbpath == "" {
		filepath, err := xdg.Data.Ensure("mort/tasks.db")

		if err != nil {
			return nil, err
		}

		dbpath = filepath
	}

	db, err := xl.Connect("sqlite3", dbpath)

	if err != nil {
		return nil, err
	}

	InitSchema(db)

	store := &Store{db}

	return store, nil
}

func (s *Store) GetTasks(query TaskQuery) ([]Task, error) {
	q := xl.Select("*").From("task")

	if query.Todo {
		q.OrderBy("state_idx, updated_at DESC")
	} else {
		q.OrderBy("updated_at DESC")
	}

	if !query.Archived {
		q.Where("archived_at IS NULL")
	}

	if query.Project != "" {
		q.Where("project=?", query.Project)
	}

	if query.ParentID > 0 {
		q.Where("(id=? OR parent_id=?)", query.ParentID, query.ParentID)
	} else if query.ParentID == 0 {
		//q.Where("parent_id IS NULL")
	}

	if query.SearchTitle != "" {
		pattern := "%" + strings.ToLower(query.SearchTitle) + "%"
		q.Where("LOWER(title) LIKE ?", pattern)
	}

	if query.SearchBody != "" {
		pattern := "%" + strings.ToLower(query.SearchBody) + "%"
		q.Where("LOWER(body) LIKE ?", pattern)
	}

	if query.Range != nil {
		q.Where("((created_at >= ? AND created_at < ?) OR (updated_at >= ? AND updated_at < ?))", query.Range.Start, query.Range.End, query.Range.Start, query.Range.End)
	}

	if query.Todo {
		q.Where("state_idx IS NOT NULL")
	}

	tasks := []Task{}
	err := q.All(s.db, &tasks)

	return tasks, err
}

func (s *Store) GetTaskByID(id int64) (*Task, error) {
	var task Task
	err := s.db.Get(&task, "SELECT * FROM task WHERE id=?", id)

	return &task, err
}

func (s *Store) CreateTask(payload Task) (int64, error) {
	q := xl.Insert("task")
	q.Set("project", payload.Project)
	q.Set("title", payload.Title)
	q.Set("body", payload.Body)

	if payload.ParentID != nil && *payload.ParentID > 0 {
		q.Set("parent_id", payload.ParentID)
	}

	return q.ExecId(s.db)
}

func (s *Store) UpdateTaskByID(id int64, payload Task) error {
	q := xl.Update("task")
	q.Where("id=?", id)
	q.SetRaw("updated_at", "current_timestamp")
	q.Set("project", payload.Project)
	q.Set("title", payload.Title)
	q.Set("body", payload.Body)

	return q.ExecOne(s.db)
}

func (s *Store) GetActiveTaskID() (int64, error) {
	var id int64
	q := xl.Select("id")
	q.From("task")
	q.Where("clockin_at IS NOT NULL")

	if err := q.First(s.db, &id); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}

	return id, nil
}

func (s *Store) GetPausedTaskID() (int64, error) {
	var id int64

	q := xl.Select("id")
	q.From("task")
	q.Where("paused_at IS NOT NULL")

	if err := q.First(s.db, &id); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}

	return id, nil
}

func (s *Store) SetTodoState(id int64, idx int, state string) error {
	tx, err := s.db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()

	if idx < 0 || state == "" {
		if _, err := tx.Exec("UPDATE task SET state_idx=NULL, state=NULL WHERE id=:id", id); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec("UPDATE task SET state_idx=?, state=? WHERE id=:id", idx, state, id); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) Clockin(id int64) error {
	tx, err := s.db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()

	if err := s.clockOut(tx); err != nil {
		return err
	}

	if _, err := tx.Exec("UPDATE task SET clockin_at=current_timestamp, updated_at=current_timestamp WHERE id=:id AND clockin_at IS NULL", id); err != nil {
		return err
	}

	_, err = tx.Exec("INSERT INTO timesheet (task_id, clockin_at) VALUES (?, current_timestamp)", id)

	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) Clockout() error {
	tx, err := s.db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()

	if err := s.clockOut(tx); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) clockOut(tx *sql.Tx) error {
	if _, err := tx.Exec("UPDATE task SET clockin_at=NULL WHERE clockin_at IS NOT NULL"); err != nil {
		return err
	}

	if _, err := tx.Exec("UPDATE task SET paused_at=NULL WHERE paused_at IS NOT NULL"); err != nil {
		return err
	}

	if _, err := tx.Exec("UPDATE timesheet SET clockout_at=current_timestamp WHERE clockout_at IS NULL"); err != nil {
		return err
	}

	return nil
}

func (s *Store) Pause() (int64, error) {
	activeID, err := s.GetActiveTaskID()

	if err != nil {
		return 0, err
	}

	if activeID == 0 {
		return 0, nil
	}

	tx, err := s.db.Begin()

	if err != nil {
		return activeID, err
	}

	defer tx.Rollback()

	if err := s.clockOut(tx); err != nil {
		return activeID, err
	}

	if _, err := tx.Exec("UPDATE task SET paused_at=current_timestamp WHERE id=?", activeID); err != nil {
		return activeID, err
	}

	return activeID, tx.Commit()
}

func (s *Store) GetTimesheet(r TimeRange) ([]TimesheetEntry, error) {
	entries := []TimesheetEntry{}

	q := xl.Select("t.*, n.project, n.title")
	q.FromAs("timesheet", "t")
	q.FromAs("task", "n")
	q.Where("t.task_id=n.id")
	q.Where("t.clockin_at >= ?", r.Start)
	q.Where("(t.clockout_at < ? OR (t.clockout_at IS NULL AND t.clockin_at < ?))", r.End, r.End)

	err := q.All(s.db, &entries)

	return entries, err
}

func (s *Store) UpdateTimesheet(id int64, clockinAt, clockoutAt *time.Time) error {
	tx, err := s.db.Begin()

	if err != nil {
		return err
	}

	defer tx.Rollback()

	if clockinAt != nil {
		if _, err := tx.Exec("UPDATE timesheet SET clockin_at=? WHERE id=?", *clockinAt, id); err != nil {
			return err
		}
	}

	if clockoutAt != nil {
		if _, err := tx.Exec("UPDATE timesheet SET clockout_at=? WHERE id=?", clockoutAt, id); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) SetArchived(id int64, archived bool) error {
	if archived {
		_, err := s.db.Exec("UPDATE task SET archived_at=current_timestamp WHERE id=:id", id)
		return err
	}

	_, err := s.db.Exec("UPDATE task SET archived_at=NULL WHERE id=:id", id)
	return err
}
