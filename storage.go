package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var tasksCSVHeader = []string{"title", "done"}

type persistedTasks struct {
	tasks []task
}

func defaultTasksFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}

	return filepath.Join(home, "Downloads", "pomodoro-tasks.csv"), nil
}

func loadTasksCSV(path string) (persistedTasks, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return persistedTasks{}, nil
	}
	if err != nil {
		return persistedTasks{}, fmt.Errorf("open tasks file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	header, err := reader.Read()
	if errors.Is(err, io.EOF) {
		return persistedTasks{}, nil
	}
	if err != nil {
		return persistedTasks{}, fmt.Errorf("read tasks header: %w", err)
	}
	if !equalCSVRecord(header, tasksCSVHeader) {
		return persistedTasks{}, fmt.Errorf("unexpected tasks header: expected %s", strings.Join(tasksCSVHeader, ","))
	}

	result := persistedTasks{}
	for line := 2; ; line++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return persistedTasks{}, fmt.Errorf("read tasks line %d: %w", line, err)
		}

		loadedTask, err := parseTaskCSVRecord(record)
		if err != nil {
			return persistedTasks{}, fmt.Errorf("read tasks line %d: %w", line, err)
		}
		result.tasks = append(result.tasks, loadedTask)
	}

	return result, nil
}

func parseTaskCSVRecord(record []string) (task, error) {
	if len(record) != len(tasksCSVHeader) {
		return task{}, fmt.Errorf("expected %d columns, got %d", len(tasksCSVHeader), len(record))
	}

	title := strings.TrimSpace(record[0])
	if title == "" {
		return task{}, errors.New("task title is empty")
	}
	done, err := strconv.ParseBool(record[1])
	if err != nil {
		return task{}, fmt.Errorf("invalid done value %q", record[1])
	}

	return task{
		title:     title,
		done:      done,
		remaining: workDuration,
	}, nil
}

func saveTasksCSV(path string, snapshot persistedTasks) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create tasks directory: %w", err)
	}

	temporary, err := os.CreateTemp(filepath.Dir(path), ".pomodoro-tasks-*.csv")
	if err != nil {
		return fmt.Errorf("create temporary tasks file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	writer := csv.NewWriter(temporary)
	if err := writer.Write(tasksCSVHeader); err != nil {
		temporary.Close()
		return fmt.Errorf("write tasks header: %w", err)
	}
	for index, savedTask := range snapshot.tasks {
		record := []string{savedTask.title, strconv.FormatBool(savedTask.done)}
		if err := writer.Write(record); err != nil {
			temporary.Close()
			return fmt.Errorf("write task %d: %w", index+1, err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		temporary.Close()
		return fmt.Errorf("flush tasks file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync tasks file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close tasks file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace tasks file: %w", err)
	}

	return nil
}

func equalCSVRecord(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
