package store_test

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"github.com/tomyl/mort/store"
	"github.com/tomyl/xl"
	"github.com/tomyl/xl/testlogger"
)

func TestTimesheet(t *testing.T) {
	xl.SetLogger(testlogger.Simple(t))

	backenddb, err := xl.Open("sqlite3", ":memory:")

	if err != nil {
		t.Fatal(err)
	}

	store.InitSchema(backenddb)

	db := store.New(backenddb)
	var taskId int64

	{
		var payload store.Task
		payload.Body = "test"

		taskId, err = db.CreateTask(payload)

		if err != nil {
			t.Fatal(err)
		}
	}

	if err := db.Clockin(taskId); err != nil {
		t.Fatal(err)
	}

	{
		activeId, err := db.GetActiveTaskID()

		if err != nil {
			t.Fatal(err)
		}

		if activeId != taskId {
			t.Fatalf("wrong active task")
		}
	}

	if err := db.Clockout(); err != nil {
		t.Fatal(err)
	}

	r := store.TimeRange{}
	r.Today()

	{
		entries, err := db.GetTimesheet(r)

		if err != nil {
			t.Fatal(err)
		}

		if len(entries) == 0 {
			//t.Fatalf("no entries")
		}
	}

	r.Next()

	{
		entries, err := db.GetTimesheet(r)

		if err != nil {
			t.Fatal(err)
		}

		if len(entries) == 0 {
			//t.Fatalf("no entries")
		}
	}

	r.Next()

	{
		entries, err := db.GetTimesheet(r)

		if err != nil {
			t.Fatal(err)
		}

		if len(entries) == 0 {
			//t.Fatalf("no entries")
		}
	}

	{
		var payload store.Task
		payload.Body = "child body"
		payload.Title = "child title"
		payload.ParentID = &taskId

		_, err = db.CreateTask(payload)

		if err != nil {
			t.Fatal(err)
		}
	}

	{
		query := store.TaskQuery{}
		tasks, err := db.GetTasks(query)
		require.Nil(t, err)
		require.Equal(t, 2, len(tasks))
	}
}

func TestState(t *testing.T) {
	xl.SetLogger(testlogger.Simple(t))

	backenddb, err := xl.Open("sqlite3", ":memory:")
	require.Nil(t, err)

	store.InitSchema(backenddb)

	db := store.New(backenddb)
	var taskId int64

	{
		var payload store.Task
		payload.Body = "test"

		taskId, err = db.CreateTask(payload)
		require.Nil(t, err)
	}

	require.Nil(t, db.SetTodoState(taskId, 42, "TODO"))

	{
		task, err := db.GetTaskByID(taskId)
		require.Nil(t, err)
		require.NotNil(t, task.State)
		require.Equal(t, "TODO", *task.State)
	}

	require.Nil(t, db.SetTodoState(taskId, -1, ""))

	{
		task, err := db.GetTaskByID(taskId)
		require.Nil(t, err)
		require.Nil(t, task.State)
	}
}
