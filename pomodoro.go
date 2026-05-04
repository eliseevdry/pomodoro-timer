package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const workDuration = 25 * time.Minute

type task struct {
	title     string
	done      bool
	remaining time.Duration
	running   bool
}

type taskStore struct {
	mu       sync.Mutex
	tasks    []task
	selected int
	active   int
}

func newTaskStore() *taskStore {
	return &taskStore{
		selected: -1,
		active:   -1,
	}
}

func (s *taskStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.tasks)
}

func (s *taskStore) Add(title string) (int, bool) {
	title = strings.TrimSpace(title)
	if title == "" {
		return -1, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks = append(s.tasks, task{
		title:     title,
		remaining: workDuration,
	})

	index := len(s.tasks) - 1
	if s.selected == -1 {
		s.selected = index
	}

	return index, true
}

func (s *taskStore) DeleteSelected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.delete(s.selected)
}

func (s *taskStore) Delete(index int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.delete(index)
}

func (s *taskStore) delete(index int) bool {
	if index < 0 || index >= len(s.tasks) {
		return false
	}

	removed := index
	s.tasks = append(s.tasks[:removed], s.tasks[removed+1:]...)

	if len(s.tasks) == 0 {
		s.selected = -1
		s.active = -1
		return true
	}

	if s.active == removed {
		s.active = -1
	} else if s.active > removed {
		s.active--
	}

	switch {
	case s.selected == removed:
		if removed >= len(s.tasks) {
			s.selected = len(s.tasks) - 1
		} else {
			s.selected = removed
		}
	case s.selected > removed:
		s.selected--
	case s.selected >= len(s.tasks):
		s.selected = removed
	}

	return true
}

func (s *taskStore) Select(index int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 0 || index >= len(s.tasks) {
		return false
	}

	s.selected = index
	return true
}

func (s *taskStore) Selected() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.selected
}

func (s *taskStore) Active() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.active
}

func (s *taskStore) Task(index int) (task, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.task(index)
}

func (s *taskStore) Current() (task, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.task(s.selected)
}

func (s *taskStore) SetDone(index int, done bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 0 || index >= len(s.tasks) {
		return false
	}

	s.tasks[index].done = done
	if done {
		s.tasks[index].running = false
		if s.active == index {
			s.active = -1
		}
	}
	return true
}

func (s *taskStore) Start(index int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 0 || index >= len(s.tasks) {
		return false
	}
	if s.tasks[index].done {
		return false
	}

	if s.active >= 0 && s.active < len(s.tasks) && s.active != index {
		s.tasks[s.active].running = false
	}

	s.active = index
	if s.tasks[index].remaining <= 0 {
		s.tasks[index].remaining = workDuration
	}
	s.tasks[index].running = true
	return true
}

func (s *taskStore) Pause(index int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 0 || index >= len(s.tasks) {
		return false
	}

	s.tasks[index].running = false
	return true
}

func (s *taskStore) Reset(index int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 0 || index >= len(s.tasks) {
		return false
	}

	s.tasks[index].remaining = workDuration
	s.tasks[index].running = false
	if s.active == index {
		s.active = -1
	}

	return true
}

func (s *taskStore) AdvanceActive(step time.Duration) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if step <= 0 || s.active < 0 || s.active >= len(s.tasks) || !s.tasks[s.active].running {
		return "", false
	}

	active := &s.tasks[s.active]
	active.remaining -= step
	if active.remaining > 0 {
		return "", false
	}

	active.remaining = 0
	active.running = false
	return active.title, true
}

func (s *taskStore) task(index int) (task, bool) {
	if index < 0 || index >= len(s.tasks) {
		return task{}, false
	}

	return s.tasks[index], true
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
