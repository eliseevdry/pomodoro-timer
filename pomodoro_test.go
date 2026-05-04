package main

import (
	"math"
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
}

func TestTaskStoreSelectsAndMarksTasksDoneManually(t *testing.T) {
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

func TestTaskStoreDeletesSelectedTaskAndAdjustsSelection(t *testing.T) {
	store := newTaskStore()
	store.Add("First")
	store.Add("Second")
	store.Add("Third")

	store.Select(1)
	if !store.DeleteSelected() {
		t.Fatal("expected selected task to be deleted")
	}
	if store.Len() != 2 {
		t.Fatalf("expected two tasks after deletion, got %d", store.Len())
	}
	if store.Selected() != 1 {
		t.Fatalf("expected next task to stay selected at index 1, got %d", store.Selected())
	}
	current, _ := store.Current()
	if current.title != "Third" {
		t.Fatalf("expected third task to become current, got %+v", current)
	}

	if !store.DeleteSelected() {
		t.Fatal("expected last task to be deleted")
	}
	if store.Selected() != 0 {
		t.Fatalf("expected previous task to be selected, got %d", store.Selected())
	}

	if !store.DeleteSelected() {
		t.Fatal("expected final task to be deleted")
	}
	if store.Len() != 0 || store.Selected() != -1 {
		t.Fatalf("expected empty store with no selection, len=%d selected=%d", store.Len(), store.Selected())
	}
}

func TestPomodoroTimerStartPauseResetAndFinish(t *testing.T) {
	timer := newPomodoroTimer()

	remaining, running := timer.snapshot()
	if remaining != workDuration || running {
		t.Fatalf("expected idle timer at 25 minutes, remaining=%s running=%v", remaining, running)
	}

	timer.start()
	if finished := timer.tick(); finished {
		t.Fatal("timer should not finish after one second")
	}
	remaining, running = timer.snapshot()
	if remaining != workDuration-time.Second || !running {
		t.Fatalf("expected running timer at 24:59, remaining=%s running=%v", remaining, running)
	}

	timer.pause()
	if finished := timer.advance(time.Minute); finished {
		t.Fatal("paused timer should not finish")
	}
	pausedRemaining, running := timer.snapshot()
	if pausedRemaining != remaining || running {
		t.Fatalf("pause should freeze timer, remaining=%s running=%v", pausedRemaining, running)
	}

	timer.start()
	if finished := timer.advance(pausedRemaining); !finished {
		t.Fatal("timer should finish when advanced by remaining duration")
	}
	remaining, running = timer.snapshot()
	if remaining != 0 || running {
		t.Fatalf("finished timer should stop at zero, remaining=%s running=%v", remaining, running)
	}
	if finished := timer.tick(); finished {
		t.Fatal("finished timer should not auto-start next pomodoro")
	}

	timer.start()
	remaining, running = timer.snapshot()
	if remaining != workDuration || !running {
		t.Fatalf("starting after finish should create a fresh 25-minute timer, remaining=%s running=%v", remaining, running)
	}

	timer.reset()
	remaining, running = timer.snapshot()
	if remaining != workDuration || running {
		t.Fatalf("reset should restore idle 25-minute timer, remaining=%s running=%v", remaining, running)
	}
}

func TestTimerFinishDoesNotCompleteTaskOrSelectNextTask(t *testing.T) {
	store := newTaskStore()
	store.Add("Current")
	store.Add("Next")

	timer := newPomodoroTimer()
	timer.start()
	if finished := timer.advance(workDuration); !finished {
		t.Fatal("timer should finish")
	}

	current, ok := store.Current()
	if !ok {
		t.Fatal("expected current task")
	}
	if current.done {
		t.Fatal("finished timer must not mark current task done automatically")
	}
	if store.Selected() != 0 {
		t.Fatalf("finished timer must not select next task, got selected=%d", store.Selected())
	}
}

func TestFormatDurationAndProgress(t *testing.T) {
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

	progress := progressForRemaining(workDuration / 2)
	if math.Abs(progress-0.5) > 0.000001 {
		t.Fatalf("expected half progress, got %f", progress)
	}
	if progressForRemaining(workDuration) != 0 {
		t.Fatal("full remaining duration should mean zero progress")
	}
	if progressForRemaining(0) != 1 {
		t.Fatal("zero remaining duration should mean complete progress")
	}
}

func TestUIAddFirstTaskDoesNotPanicAndShowsTaskTitle(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	w := test.NewWindow(widget.NewLabel(""))
	defer w.Close()

	ui := newPomodoroUI(a, w)
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
	if ui.currentTaskText.Text != "Текущая задача: Kabinet IP" {
		t.Fatalf("unexpected current task label: %q", ui.currentTaskText.Text)
	}
}

func TestTaskListTemplateRendersCheckboxAndTitle(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	w := test.NewWindow(widget.NewLabel(""))
	defer w.Close()

	ui := newPomodoroUI(a, w)
	w.SetContent(ui.content)
	ui.store.Add("Посмотреть ИП")

	item := ui.taskList.CreateItem()
	if _, ok := item.(*fyne.Container); !ok {
		t.Fatalf("list template must be a Fyne container, got %T", item)
	}

	check, title, ok := taskRowParts(item)
	if !ok {
		t.Fatalf("expected checkbox and title in list template, got item=%T", item)
	}
	if !check.Visible() || !title.Visible() {
		t.Fatal("checkbox and title should be visible")
	}

	ui.taskList.UpdateItem(0, item)
	if title.Text != "-> Посмотреть ИП" {
		t.Fatalf("expected rendered task title, got %q", title.Text)
	}

	check.OnChanged(true)
	task, _ := ui.store.Task(0)
	if !task.done {
		t.Fatal("checkbox should mark task done")
	}
}
