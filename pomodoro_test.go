package main

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestTaskStoreAddsTasksInMemoryAndSelectsFirstTask(t *testing.T) {
	store := newTaskStore()

	if index, ok := store.Add("   "); ok || index != -1 {
		t.Fatalf("blank task should not be added, got index=%d ok=%v", index, ok)
	}

	firstIndex, ok := store.Add("  Kabinet IP  ")
	if !ok {
		t.Fatal("expected first task to be added")
	}
	if firstIndex != 0 {
		t.Fatalf("expected first index 0, got %d", firstIndex)
	}
	if store.Selected() != 0 {
		t.Fatalf("expected first task to be selected, got %d", store.Selected())
	}

	secondIndex, ok := store.Add("Write tests")
	if !ok {
		t.Fatal("expected second task to be added")
	}
	if secondIndex != 1 {
		t.Fatalf("expected second index 1, got %d", secondIndex)
	}
	if store.Selected() != 0 {
		t.Fatalf("adding another task should keep current selection, got %d", store.Selected())
	}

	firstTask, ok := store.Task(0)
	if !ok || firstTask.title != "Kabinet IP" {
		t.Fatalf("expected trimmed first task title, got task=%+v ok=%v", firstTask, ok)
	}
	if firstTask.remaining != workDuration || firstTask.running {
		t.Fatalf("new task should start idle at 25 minutes, got %+v", firstTask)
	}
}

func TestTaskStoreSelectsWithoutTouchingRunningTimer(t *testing.T) {
	store := newTaskStore()
	store.Add("First")
	store.Add("Second")

	if !store.Start(0) {
		t.Fatal("expected first task to start")
	}
	if _, finished := store.AdvanceActive(5 * time.Second); finished {
		t.Fatal("timer should not finish after five seconds")
	}
	if !store.Select(1) {
		t.Fatal("expected second task to be selectable")
	}

	firstTask, _ := store.Task(0)
	if !firstTask.running || firstTask.remaining != workDuration-5*time.Second {
		t.Fatalf("selecting another task must not reset running timer, got %+v", firstTask)
	}
	if store.Active() != 0 {
		t.Fatalf("active timer should stay on first task, got %d", store.Active())
	}
}

func TestStartingAnotherTaskPausesPreviousTimer(t *testing.T) {
	store := newTaskStore()
	store.Add("First")
	store.Add("Second")

	store.Start(0)
	store.AdvanceActive(time.Minute)
	if !store.Start(1) {
		t.Fatal("expected second task to start")
	}

	firstTask, _ := store.Task(0)
	if firstTask.running || firstTask.remaining != workDuration-time.Minute {
		t.Fatalf("starting another task should pause previous timer, got %+v", firstTask)
	}

	secondTask, _ := store.Task(1)
	if !secondTask.running || secondTask.remaining != workDuration {
		t.Fatalf("second task should be running from 25 minutes, got %+v", secondTask)
	}
	if store.Active() != 1 {
		t.Fatalf("expected active timer on second task, got %d", store.Active())
	}

	if _, finished := store.AdvanceActive(30 * time.Second); finished {
		t.Fatal("second timer should not finish after thirty seconds")
	}
	firstTask, _ = store.Task(0)
	if firstTask.running || firstTask.remaining != workDuration-time.Minute {
		t.Fatalf("paused previous timer should not advance, got %+v", firstTask)
	}

	if !store.Start(0) {
		t.Fatal("expected first task to resume")
	}
	firstTask, _ = store.Task(0)
	if !firstTask.running || firstTask.remaining != workDuration-time.Minute {
		t.Fatalf("first task should resume from saved time, got %+v", firstTask)
	}
}

func TestTaskStorePauseResetAndFinish(t *testing.T) {
	store := newTaskStore()
	store.Add("Current")
	store.Add("Next")

	store.Start(0)
	if _, finished := store.AdvanceActive(time.Second); finished {
		t.Fatal("timer should not finish after one second")
	}
	current, _ := store.Task(0)
	if current.remaining != workDuration-time.Second || !current.running {
		t.Fatalf("expected running timer at 24:59, got %+v", current)
	}

	store.Pause(0)
	if _, finished := store.AdvanceActive(time.Minute); finished {
		t.Fatal("paused timer should not finish")
	}
	current, _ = store.Task(0)
	if current.remaining != workDuration-time.Second || current.running {
		t.Fatalf("pause should freeze timer, got %+v", current)
	}

	store.Start(0)
	title, finished := store.AdvanceActive(current.remaining)
	if !finished || title != "Current" {
		t.Fatalf("timer should finish current task, title=%q finished=%v", title, finished)
	}
	current, _ = store.Task(0)
	if current.remaining != 0 || current.running || current.done {
		t.Fatalf("finished timer should stop at zero and not complete task, got %+v", current)
	}
	next, _ := store.Task(1)
	if next.running || next.remaining != workDuration {
		t.Fatalf("finished timer should not start next task, got %+v", next)
	}

	store.Start(0)
	current, _ = store.Task(0)
	if current.remaining != workDuration || !current.running {
		t.Fatalf("starting finished task should create fresh timer, got %+v", current)
	}

	store.Reset(0)
	current, _ = store.Task(0)
	if current.remaining != workDuration || current.running || store.Active() != -1 {
		t.Fatalf("reset should restore idle task and clear active timer, task=%+v active=%d", current, store.Active())
	}
}

func TestTaskStoreMarksTasksDoneManually(t *testing.T) {
	store := newTaskStore()
	store.Add("First")
	store.Add("Second")

	if !store.Select(1) {
		t.Fatal("expected second task to be selectable")
	}
	if store.Select(2) {
		t.Fatal("out-of-range task should not be selectable")
	}
	if !store.SetDone(1, true) {
		t.Fatal("expected selected task to be marked done")
	}

	current, ok := store.Current()
	if !ok {
		t.Fatal("expected current task")
	}
	if current.title != "Second" || !current.done {
		t.Fatalf("expected done second task, got %+v", current)
	}

	firstTask, _ := store.Task(0)
	if firstTask.done {
		t.Fatal("marking second task done should not complete first task")
	}
}

func TestMarkingRunningTaskDoneStopsTimer(t *testing.T) {
	store := newTaskStore()
	store.Add("Running")

	store.Start(0)
	store.AdvanceActive(10 * time.Second)
	if !store.SetDone(0, true) {
		t.Fatal("expected running task to be marked done")
	}

	task, _ := store.Task(0)
	if !task.done {
		t.Fatal("task should be done")
	}
	if task.running {
		t.Fatal("done task timer should be stopped")
	}
	if task.remaining != workDuration-10*time.Second {
		t.Fatalf("done task should keep stopped remaining time, got %s", task.remaining)
	}
	if store.Active() != -1 {
		t.Fatalf("done running task should clear active timer, got %d", store.Active())
	}
	if store.Start(0) {
		t.Fatal("done task should not start")
	}
}

func TestAddAndDeleteOtherTasksWhileTimerRuns(t *testing.T) {
	store := newTaskStore()
	store.Add("Running")
	store.Add("Delete me")

	store.Start(0)
	store.AdvanceActive(2 * time.Second)
	store.Add("New task")

	if !store.Delete(1) {
		t.Fatal("expected non-running task to be deleted")
	}

	runningTask, _ := store.Task(0)
	if runningTask.title != "Running" || !runningTask.running || runningTask.remaining != workDuration-2*time.Second {
		t.Fatalf("deleting another task must not interrupt running timer, got %+v", runningTask)
	}
	if store.Len() != 2 {
		t.Fatalf("expected two tasks after delete, got %d", store.Len())
	}
	if store.Active() != 0 {
		t.Fatalf("expected active index to stay 0, got %d", store.Active())
	}
}

func TestDeletingTaskBeforeActiveTimerAdjustsActiveIndex(t *testing.T) {
	store := newTaskStore()
	store.Add("Delete me")
	store.Add("Running")

	store.Start(1)
	store.AdvanceActive(3 * time.Second)
	if !store.Delete(0) {
		t.Fatal("expected first task to be deleted")
	}

	runningTask, _ := store.Task(0)
	if runningTask.title != "Running" || !runningTask.running || runningTask.remaining != workDuration-3*time.Second {
		t.Fatalf("active timer should survive index shift, got %+v", runningTask)
	}
	if store.Active() != 0 {
		t.Fatalf("expected active index to shift to 0, got %d", store.Active())
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[time.Duration]string{
		25 * time.Minute:                "25:00",
		24*time.Minute + 59*time.Second: "24:59",
		0:                               "00:00",
		-time.Second:                    "00:00",
	}

	for duration, expected := range cases {
		if actual := formatDuration(duration); actual != expected {
			t.Fatalf("formatDuration(%s) = %s, expected %s", duration, actual, expected)
		}
	}
}

func TestUIAddFirstTaskDoesNotPanicAndShowsTaskTitle(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	w := test.NewWindow(widget.NewLabel(""))
	defer w.Close()

	ui := newPomodoroUI(a)
	w.SetContent(ui.content)
	ui.taskEntry.SetText("Kabinet IP")

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("adding first task should not panic, recovered=%v", recovered)
		}
	}()
	ui.addTask()

	if ui.store.Len() != 1 {
		t.Fatalf("expected one task, got %d", ui.store.Len())
	}
	current, ok := ui.store.Current()
	if !ok || current.title != "Kabinet IP" {
		t.Fatalf("expected current task title to be visible in state, got task=%+v ok=%v", current, ok)
	}
}

func TestTaskListTemplateRendersTimerAndControls(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	w := test.NewWindow(widget.NewLabel(""))
	defer w.Close()

	ui := newPomodoroUI(a)
	w.SetContent(ui.content)
	ui.store.Add("Посмотреть ИП")
	if ui.addButton.Text != "" {
		t.Fatalf("add button should be icon-only, got %q", ui.addButton.Text)
	}

	item := ui.taskList.CreateItem()
	if _, ok := item.(*fyne.Container); !ok {
		t.Fatalf("list template must be a Fyne container, got %T", item)
	}

	row, ok := findTaskRowParts(item)
	if !ok {
		t.Fatalf("expected checkbox, title, timer, and controls in list template, got item=%T", item)
	}
	if !row.check.Visible() || !row.title.Visible() {
		t.Fatal("checkbox and title should be visible")
	}

	ui.taskList.UpdateItem(0, item)
	if row.title.Text != "Посмотреть ИП" {
		t.Fatalf("expected rendered task title, got %q", row.title.Text)
	}
	if row.start.Text != "" || row.pause.Text != "" || row.reset.Text != "" || row.delete.Text != "" {
		t.Fatalf("task controls should be icon-only, got %q/%q/%q/%q", row.start.Text, row.pause.Text, row.reset.Text, row.delete.Text)
	}
	if !row.start.Visible() || !row.delete.Visible() || row.timer.Visible() || row.pause.Visible() || row.reset.Visible() {
		t.Fatal("idle task should show only start and delete icons")
	}

	row.start.OnTapped()
	task, _ := ui.store.Task(0)
	if !task.running {
		t.Fatal("start button should start task timer")
	}
	ui.taskList.UpdateItem(0, item)
	if !row.timer.Visible() || row.start.Visible() || !row.pause.Visible() || !row.reset.Visible() || !row.delete.Visible() {
		t.Fatal("running task should show timer, pause, reset, and delete only")
	}
	if row.timer.Text != "25:00" {
		t.Fatalf("expected rendered task timer, got %q", row.timer.Text)
	}

	row.pause.OnTapped()
	task, _ = ui.store.Task(0)
	if task.running {
		t.Fatal("pause button should pause task timer")
	}
	ui.taskList.UpdateItem(0, item)
	if !row.start.Visible() || !row.timer.Visible() || row.pause.Visible() || !row.reset.Visible() || !row.delete.Visible() {
		t.Fatal("paused task should show timer, start, reset, and delete")
	}

	row.check.OnChanged(true)
	task, _ = ui.store.Task(0)
	if !task.done {
		t.Fatal("checkbox should mark task done")
	}
	ui.taskList.UpdateItem(0, item)
	if row.title.Text != "Посмотреть ИП" {
		t.Fatalf("done task should show only original title, got %q", row.title.Text)
	}
	if !row.delete.Visible() || row.timer.Visible() || row.start.Visible() || row.pause.Visible() || row.reset.Visible() {
		t.Fatal("done task should hide timer controls and keep delete")
	}

	row.delete.OnTapped()
	if ui.store.Len() != 0 {
		t.Fatalf("delete button should remove task, got len=%d", ui.store.Len())
	}
}
