package main

import (
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
)

func TestTasksCSVStoresTitlesAndDoneStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pomodoro-tasks.csv")
	store := newTaskStore()
	store.Add("Plan, execute")
	store.Add("Finished task")
	store.SetDone(1, true)
	store.Start(0)
	store.AdvanceActive(10 * time.Second)

	if err := saveTasksCSV(path, store.Snapshot()); err != nil {
		t.Fatalf("save tasks: %v", err)
	}

	loaded, err := loadTasksCSV(path)
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(loaded.tasks) != 2 {
		t.Fatalf("expected two loaded tasks, got %d", len(loaded.tasks))
	}

	restored := newTaskStore()
	restored.Restore(loaded)
	first, _ := restored.Task(0)
	if first.title != "Plan, execute" {
		t.Fatalf("expected CSV title with comma to survive, got %q", first.title)
	}
	if first.done || first.running || first.remaining != workDuration {
		t.Fatalf("timer state should reset after loading, got %+v", first)
	}
	second, _ := restored.Task(1)
	if second.title != "Finished task" || !second.done || second.running || second.remaining != workDuration {
		t.Fatalf("title and done status should be restored without timer state, got %+v", second)
	}
	if restored.Active() != -1 {
		t.Fatalf("timer should not be active after loading, got %d", restored.Active())
	}
}

func TestUIPersistsTaskChangesImmediately(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	path := filepath.Join(t.TempDir(), "pomodoro-tasks.csv")
	ui := newPomodoroUI(a)
	if err := ui.enablePersistence(path); err != nil {
		t.Fatalf("enable persistence: %v", err)
	}

	ui.taskEntry.SetText("First task")
	ui.addTask()
	assertPersistedTasks(t, path, persistedTaskExpectation{title: "First task"})

	ui.editing = 0
	ui.saveTaskTitle(0, "Renamed task", false)
	assertPersistedTasks(t, path, persistedTaskExpectation{title: "Renamed task"})

	item := ui.taskList.CreateItem()
	ui.taskList.UpdateItem(0, item)
	row, ok := findTaskRowParts(item)
	if !ok {
		t.Fatal("expected task row controls")
	}
	row.check.OnChanged(true)
	assertPersistedTasks(t, path, persistedTaskExpectation{title: "Renamed task", done: true})

	ui.deleteTask(0)
	assertPersistedTasks(t, path)
}

type persistedTaskExpectation struct {
	title string
	done  bool
}

func assertPersistedTasks(t *testing.T, path string, expected ...persistedTaskExpectation) {
	t.Helper()

	loaded, err := loadTasksCSV(path)
	if err != nil {
		t.Fatalf("load persisted titles: %v", err)
	}
	if len(loaded.tasks) != len(expected) {
		t.Fatalf("expected %d persisted titles, got %d", len(expected), len(loaded.tasks))
	}
	for index, expectedTask := range expected {
		if loaded.tasks[index].title != expectedTask.title || loaded.tasks[index].done != expectedTask.done {
			t.Fatalf("unexpected task %d: got %+v, expected %+v", index, loaded.tasks[index], expectedTask)
		}
	}
}
