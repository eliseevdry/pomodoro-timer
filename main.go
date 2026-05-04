package main

import (
	"os/exec"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type pomodoroUI struct {
	app    fyne.App
	window fyne.Window

	content fyne.CanvasObject

	store *taskStore
	timer *pomodoroTimer

	taskEntry       *widget.Entry
	taskList        *widget.List
	currentTaskText *widget.Label
	timerText       *widget.Label
	statusText      *widget.Label
	progress        *widget.ProgressBar
	startButton     *widget.Button
	pauseButton     *widget.Button
	resetButton     *widget.Button
	deleteButton    *widget.Button
}

func main() {
	a := app.New()
	w := a.NewWindow("Pomodoro Timer")
	w.Resize(fyne.NewSize(900, 560))

	ui := newPomodoroUI(a, w)
	w.SetContent(ui.content)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	go ui.runTimer(ticker)

	w.ShowAndRun()
}

func newPomodoroUI(a fyne.App, w fyne.Window) *pomodoroUI {
	ui := &pomodoroUI{
		app:    a,
		window: w,
		store:  newTaskStore(),
		timer:  newPomodoroTimer(),
	}

	ui.taskEntry = widget.NewEntry()
	ui.taskEntry.SetPlaceHolder("Новая задача")

	ui.currentTaskText = widget.NewLabel("Текущая задача: не выбрана")
	ui.currentTaskText.Wrapping = fyne.TextWrapWord

	ui.timerText = widget.NewLabelWithStyle(formatDuration(workDuration), fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	ui.statusText = widget.NewLabel("Добавьте задачу и нажмите Старт.")
	ui.statusText.Wrapping = fyne.TextWrapWord

	ui.progress = widget.NewProgressBar()
	ui.progress.Min = 0
	ui.progress.Max = 1

	ui.taskList = ui.newTaskList()
	ui.taskList.OnSelected = ui.selectTask

	ui.taskEntry.OnSubmitted = func(_ string) {
		ui.addTask()
	}

	addButton := widget.NewButtonWithIcon("Добавить", theme.ContentAddIcon(), ui.addTask)
	ui.deleteButton = widget.NewButtonWithIcon("Удалить", theme.DeleteIcon(), ui.deleteSelectedTask)
	ui.startButton = widget.NewButtonWithIcon("Старт", theme.MediaPlayIcon(), ui.startTimer)
	ui.pauseButton = widget.NewButtonWithIcon("Пауза", theme.MediaPauseIcon(), ui.pauseTimer)
	ui.resetButton = widget.NewButtonWithIcon("Сброс", theme.ViewRefreshIcon(), ui.resetTimer)

	taskControls := container.NewBorder(nil, nil, nil, addButton, ui.taskEntry)
	leftPanel := container.NewBorder(
		widget.NewLabelWithStyle("Задачи", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewVBox(taskControls, ui.deleteButton),
		nil,
		nil,
		ui.taskList,
	)

	timerControls := container.NewGridWithColumns(3, ui.startButton, ui.pauseButton, ui.resetButton)
	rightPanel := container.NewVBox(
		widget.NewLabelWithStyle("Таймер", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		ui.currentTaskText,
		ui.timerText,
		ui.progress,
		timerControls,
		ui.statusText,
	)

	content := container.NewHSplit(leftPanel, container.NewPadded(rightPanel))
	content.SetOffset(0.55)
	ui.content = content

	ui.deleteButton.Disable()
	ui.refreshTimer()

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
			return container.NewBorder(nil, nil, check, nil, title)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			check, title, ok := taskRowParts(item)
			if !ok {
				return
			}

			index := int(id)
			task, ok := ui.store.Task(index)
			if !ok {
				return
			}

			check.OnChanged = nil
			check.SetChecked(task.done)
			check.OnChanged = func(done bool) {
				ui.store.SetDone(index, done)
				ui.taskList.Refresh()
			}

			text := task.title
			if index == ui.store.Selected() {
				text = "-> " + text
			}
			if task.done {
				text += " (выполнена)"
			}

			title.SetText(text)
			title.TextStyle = fyne.TextStyle{Italic: task.done}
			title.Refresh()
		},
	)
}

func taskRowParts(item fyne.CanvasObject) (*widget.Check, *widget.Label, bool) {
	row, ok := item.(*fyne.Container)
	if !ok {
		return nil, nil, false
	}

	var check *widget.Check
	var title *widget.Label
	for _, object := range row.Objects {
		switch typed := object.(type) {
		case *widget.Check:
			check = typed
		case *widget.Label:
			title = typed
		}
	}

	return check, title, check != nil && title != nil
}

func (ui *pomodoroUI) addTask() {
	hadSelection := ui.store.Selected() != -1
	index, ok := ui.store.Add(ui.taskEntry.Text)
	if !ok {
		return
	}

	ui.taskEntry.SetText("")
	ui.taskList.Refresh()

	if !hadSelection {
		ui.taskList.Select(widget.ListItemID(index))
		return
	}

	ui.refreshCurrentTask()
}

func (ui *pomodoroUI) deleteSelectedTask() {
	if !ui.store.DeleteSelected() {
		return
	}

	ui.timer.reset()
	ui.statusText.SetText("Выберите задачу для нового таймера.")
	ui.taskList.Refresh()

	if selected := ui.store.Selected(); selected >= 0 {
		ui.taskList.Select(widget.ListItemID(selected))
		return
	}

	ui.refreshCurrentTask()
}

func (ui *pomodoroUI) selectTask(id widget.ListItemID) {
	if !ui.store.Select(int(id)) {
		return
	}

	_, running := ui.timer.snapshot()
	if !running {
		ui.timer.reset()
		ui.statusText.SetText("Готово к запуску 25-минутного таймера.")
	}

	ui.taskList.Refresh()
	ui.refreshCurrentTask()
}

func (ui *pomodoroUI) startTimer() {
	if ui.store.Selected() == -1 {
		dialog.ShowInformation("Нет выбранной задачи", "Сначала добавьте и выберите задачу.", ui.window)
		return
	}

	ui.timer.start()
	ui.statusText.SetText("Таймер запущен.")
	ui.refreshTimer()
}

func (ui *pomodoroUI) pauseTimer() {
	ui.timer.pause()
	ui.statusText.SetText("Таймер на паузе.")
	ui.refreshTimer()
}

func (ui *pomodoroUI) resetTimer() {
	ui.timer.reset()
	ui.statusText.SetText("Таймер сброшен на 25:00.")
	ui.refreshTimer()
}

func (ui *pomodoroUI) refreshCurrentTask() {
	current, ok := ui.store.Current()
	if !ok {
		ui.currentTaskText.SetText("Текущая задача: не выбрана")
		ui.deleteButton.Disable()
		ui.refreshTimer()
		return
	}

	ui.currentTaskText.SetText("Текущая задача: " + current.title)
	ui.deleteButton.Enable()
	ui.refreshTimer()
}

func (ui *pomodoroUI) refreshTimer() {
	remaining, running := ui.timer.snapshot()
	ui.timerText.SetText(formatDuration(remaining))
	ui.progress.SetValue(progressForRemaining(remaining))

	if ui.store.Selected() == -1 || running {
		ui.startButton.Disable()
	} else {
		ui.startButton.Enable()
	}

	if running {
		ui.pauseButton.Enable()
	} else {
		ui.pauseButton.Disable()
	}

	if remaining == workDuration && !running {
		ui.resetButton.Disable()
	} else {
		ui.resetButton.Enable()
	}
}

func (ui *pomodoroUI) runTimer(ticker *time.Ticker) {
	for range ticker.C {
		if !ui.timer.tick() {
			fyne.Do(ui.refreshTimer)
			continue
		}

		ringAlarm()
		fyne.Do(func() {
			ui.app.SendNotification(fyne.NewNotification(
				"Pomodoro завершен",
				"25 минут истекли. Задача не отмечена выполненной автоматически.",
			))
			ui.statusText.SetText("Время вышло. Отметьте задачу выполненной вручную, если она завершена.")
			ui.refreshTimer()
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
