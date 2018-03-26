package store_test

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"github.com/tomyl/mort/store"
	"github.com/tomyl/xl"
	"github.com/tomyl/xl/logger"
)

func TestTimesheet(t *testing.T) {
	xl.SetLogger(logger.Test(t))

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
