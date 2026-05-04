package main

import (
	"os/exec"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type pomodoroUI struct {
	app fyne.App

	content fyne.CanvasObject

	store *taskStore

	taskEntry *widget.Entry
	taskList  *widget.List
	addButton *widget.Button
}

type taskRowParts struct {
	check  *widget.Check
	title  *widget.Label
	timer  *widget.Label
	start  *widget.Button
	pause  *widget.Button
	reset  *widget.Button
	delete *widget.Button
}

func main() {
	a := app.New()
	w := a.NewWindow("Pomodoro Timer")
	w.Resize(fyne.NewSize(900, 560))

	ui := newPomodoroUI(a)
	w.SetContent(ui.content)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	go ui.runTimer(ticker)

	w.ShowAndRun()
}

func newPomodoroUI(a fyne.App) *pomodoroUI {
	ui := &pomodoroUI{
		app:   a,
		store: newTaskStore(),
	}

	ui.taskEntry = widget.NewEntry()
	ui.taskEntry.SetPlaceHolder("New task")

	ui.taskList = ui.newTaskList()
	ui.taskList.OnSelected = ui.selectTask

	ui.taskEntry.OnSubmitted = func(_ string) {
		ui.addTask()
	}

	ui.addButton = widget.NewButtonWithIcon("", theme.ContentAddIcon(), ui.addTask)

	taskControls := container.NewBorder(nil, nil, nil, ui.addButton, ui.taskEntry)
	content := container.NewBorder(
		nil,
		taskControls,
		nil,
		nil,
		ui.taskList,
	)
	ui.content = content

	return ui
}

func (ui *pomodoroUI) newTaskList() *widget.List {
	return widget.NewList(
		func() int {
			return ui.store.Len()
		},
		func() fyne.CanvasObject {
			check := widget.NewCheck("", nil)
			title := widget.NewLabel("")
			title.Truncation = fyne.TextTruncateEllipsis

			timer := widget.NewLabelWithStyle(formatDuration(workDuration), fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

			startButton := widget.NewButtonWithIcon("", theme.MediaPlayIcon(), nil)
			pauseButton := widget.NewButtonWithIcon("", theme.MediaPauseIcon(), nil)
			resetButton := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), nil)
			deleteButton := widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)

			taskText := container.NewBorder(nil, nil, nil, timer, title)
			controls := container.NewHBox(startButton, pauseButton, resetButton, deleteButton)
			return container.NewBorder(nil, nil, check, controls, taskText)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			row, ok := findTaskRowParts(item)
			if !ok {
				return
			}

			index := int(id)
			task, ok := ui.store.Task(index)
			if !ok {
				return
			}

			row.check.OnChanged = nil
			row.check.SetChecked(task.done)
			row.check.OnChanged = func(done bool) {
				ui.store.SetDone(index, done)
				ui.refreshTasks()
			}

			title := task.title
			if !task.done && index == ui.store.Selected() {
				title = "" + title
			}

			row.title.SetText(title)
			row.title.TextStyle = fyne.TextStyle{Italic: task.done}
			row.title.Refresh()
			row.timer.SetText(formatDuration(task.remaining))

			row.start.OnTapped = func() {
				ui.startTask(index)
			}
			row.pause.OnTapped = func() {
				ui.pauseTask(index)
			}
			row.reset.OnTapped = func() {
				ui.resetTask(index)
			}
			row.delete.OnTapped = func() {
				ui.deleteTask(index)
			}

			updateTaskRowVisibility(row, task, index == ui.store.Active())
		},
	)
}

func updateTaskRowVisibility(row taskRowParts, task task, active bool) {
	switch {
	case task.done:
		row.timer.Hide()
		row.start.Hide()
		row.pause.Hide()
		row.reset.Hide()
		row.delete.Show()
	case task.running:
		row.timer.Show()
		row.start.Hide()
		row.pause.Show()
		row.reset.Show()
		row.delete.Show()
		row.pause.Enable()
		row.reset.Enable()
	case active:
		row.timer.Show()
		row.start.Show()
		row.pause.Hide()
		row.reset.Show()
		row.delete.Show()
		row.start.Enable()
		row.reset.Enable()
	default:
		row.timer.Hide()
		row.start.Show()
		row.pause.Hide()
		row.reset.Hide()
		row.delete.Show()
		row.start.Enable()
	}
}

func findTaskRowParts(item fyne.CanvasObject) (taskRowParts, bool) {
	var parts taskRowParts
	collectTaskRowParts(item, &parts)

	ok := parts.check != nil &&
		parts.title != nil &&
		parts.timer != nil &&
		parts.start != nil &&
		parts.pause != nil &&
		parts.reset != nil &&
		parts.delete != nil

	return parts, ok
}

func collectTaskRowParts(item fyne.CanvasObject, parts *taskRowParts) {
	switch object := item.(type) {
	case *widget.Check:
		parts.check = object
	case *widget.Label:
		if object.TextStyle.Bold {
			parts.timer = object
		} else if parts.title == nil {
			parts.title = object
		}
	case *widget.Button:
		switch iconName(object.Icon) {
		case "media-play":
			parts.start = object
		case "media-pause":
			parts.pause = object
		case "view-refresh":
			parts.reset = object
		case "delete":
			parts.delete = object
		}
	case *fyne.Container:
		for _, child := range object.Objects {
			collectTaskRowParts(child, parts)
		}
	}
}

func iconName(resource fyne.Resource) string {
	if resource == nil {
		return ""
	}

	name := strings.TrimPrefix(resource.Name(), "foreground_")
	name = strings.TrimSuffix(name, ".svg")
	return name
}

func (ui *pomodoroUI) addTask() {
	hadSelection := ui.store.Selected() != -1
	index, ok := ui.store.Add(ui.taskEntry.Text)
	if !ok {
		return
	}

	ui.taskEntry.SetText("")
	ui.refreshTasks()

	if !hadSelection {
		ui.taskList.Select(widget.ListItemID(index))
		return
	}
}

func (ui *pomodoroUI) selectTask(id widget.ListItemID) {
	if !ui.store.Select(int(id)) {
		return
	}

	ui.refreshTasks()
}

func (ui *pomodoroUI) startTask(index int) {
	ui.store.Start(index)
	ui.refreshTasks()
}

func (ui *pomodoroUI) pauseTask(index int) {
	ui.store.Pause(index)
	ui.refreshTasks()
}

func (ui *pomodoroUI) resetTask(index int) {
	ui.store.Reset(index)
	ui.refreshTasks()
}

func (ui *pomodoroUI) deleteTask(index int) {
	if !ui.store.Delete(index) {
		return
	}

	ui.refreshTasks()
	if selected := ui.store.Selected(); selected >= 0 {
		ui.taskList.Select(widget.ListItemID(selected))
	}
}

func (ui *pomodoroUI) refreshTasks() {
	ui.taskList.Refresh()
}

func (ui *pomodoroUI) runTimer(ticker *time.Ticker) {
	for range ticker.C {
		title, finished := ui.store.AdvanceActive(time.Second)
		fyne.Do(ui.refreshTasks)
		if !finished {
			continue
		}

		ringAlarm()
		fyne.Do(func() {
			ui.app.SendNotification(fyne.NewNotification(
				"Pomodoro завершен",
				"25 минут истекли: "+title,
			))
		})
	}
}

func ringAlarm() {
	switch runtime.GOOS {
	case "darwin":
		runFirstAvailable([]string{"afplay", "/System/Library/Sounds/Glass.aiff"})
	case "windows":
		runFirstAvailable([]string{"powershell", "-NoProfile", "-Command", "[console]::beep(880,700)"})
	default:
		runFirstAvailable(
			[]string{"paplay", "/usr/share/sounds/freedesktop/stereo/alarm-clock-elapsed.oga"},
			[]string{"canberra-gtk-play", "-i", "alarm-clock-elapsed"},
			[]string{"aplay", "/usr/share/sounds/alsa/Front_Center.wav"},
		)
	}
}

func runFirstAvailable(commands ...[]string) {
	for _, command := range commands {
		if len(command) == 0 {
			continue
		}

		if _, err := exec.LookPath(command[0]); err != nil {
			continue
		}

		_ = exec.Command(command[0], command[1:]...).Start()
		return
	}
}
