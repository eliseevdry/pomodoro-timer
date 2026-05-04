package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const workDuration = 25 * time.Minute

type task struct {
	title string
	done  bool
}

type taskStore struct {
	tasks    []task
	selected int
}

func newTaskStore() *taskStore {
	return &taskStore{selected: -1}
}

func (s *taskStore) Len() int {
	return len(s.tasks)
}

func (s *taskStore) Add(title string) (int, bool) {
	title = strings.TrimSpace(title)
	if title == "" {
		return -1, false
	}

	s.tasks = append(s.tasks, task{title: title})
	index := len(s.tasks) - 1
	if s.selected == -1 {
		s.selected = index
	}

	return index, true
}

func (s *taskStore) DeleteSelected() bool {
	if s.selected < 0 || s.selected >= len(s.tasks) {
		return false
	}

	index := s.selected
	s.tasks = append(s.tasks[:index], s.tasks[index+1:]...)
	if len(s.tasks) == 0 {
		s.selected = -1
		return true
	}

	if index >= len(s.tasks) {
		s.selected = len(s.tasks) - 1
	} else {
		s.selected = index
	}

	return true
}

func (s *taskStore) Select(index int) bool {
	if index < 0 || index >= len(s.tasks) {
		return false
	}

	s.selected = index
	return true
}

func (s *taskStore) Selected() int {
	return s.selected
}

func (s *taskStore) Task(index int) (task, bool) {
	if index < 0 || index >= len(s.tasks) {
		return task{}, false
	}

	return s.tasks[index], true
}

func (s *taskStore) Current() (task, bool) {
	return s.Task(s.selected)
}

func (s *taskStore) SetDone(index int, done bool) bool {
	if index < 0 || index >= len(s.tasks) {
		return false
	}

	s.tasks[index].done = done
	return true
}

type pomodoroTimer struct {
	mu        sync.Mutex
	remaining time.Duration
	running   bool
}

func newPomodoroTimer() *pomodoroTimer {
	return &pomodoroTimer{remaining: workDuration}
}

func (t *pomodoroTimer) snapshot() (time.Duration, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.remaining, t.running
}

func (t *pomodoroTimer) start() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.remaining <= 0 {
		t.remaining = workDuration
	}
	t.running = true
}

func (t *pomodoroTimer) pause() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.running = false
}

func (t *pomodoroTimer) reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.remaining = workDuration
	t.running = false
}

func (t *pomodoroTimer) tick() bool {
	return t.advance(time.Second)
}

func (t *pomodoroTimer) advance(step time.Duration) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running || step <= 0 {
		return false
	}

	t.remaining -= step
	if t.remaining > 0 {
		return false
	}

	t.remaining = 0
	t.running = false
	return true
}

func formatDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}

	totalSeconds := int(duration.Seconds())
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60

	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func progressForRemaining(remaining time.Duration) float64 {
	if remaining <= 0 {
		return 1
	}
	if remaining >= workDuration {
		return 0
	}

	return 1 - float64(remaining)/float64(workDuration)
}
