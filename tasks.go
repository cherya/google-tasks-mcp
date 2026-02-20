package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/tasks/v1"
)

type TasksClient struct {
	service *tasks.Service
	loc     *time.Location
}

type TaskItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Notes     string `json:"notes,omitempty"`
	Due       string `json:"due,omitempty"`
	Status    string `json:"status"`
	Completed string `json:"completed,omitempty"`
}

type TaskListItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// NewTasksClientOAuth creates a client using OAuth2 token
func NewTasksClientOAuth(httpClient *http.Client, loc *time.Location) (*TasksClient, error) {
	ctx := context.Background()

	srv, err := tasks.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}

	return &TasksClient{
		service: srv,
		loc:     loc,
	}, nil
}

// parseDue converts user input (YYYY-MM-DD or YYYY-MM-DDTHH:MM) to RFC3339 in the configured timezone.
func parseDue(due string, loc *time.Location) (string, error) {
	if loc == nil {
		loc = time.UTC
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04", due, loc); err == nil {
		return t.Format(time.RFC3339), nil
	}
	if t, err := time.ParseInLocation("2006-01-02", due, loc); err == nil {
		return t.Format(time.RFC3339), nil
	}
	return "", fmt.Errorf("invalid due format %q, expected YYYY-MM-DD or YYYY-MM-DDTHH:MM", due)
}

// formatDue converts RFC3339 from Google API to a human-readable string in the configured timezone.
func formatDue(rfc3339 string, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		// Google API may return fractional seconds (e.g. "...T00:00:00.000Z")
		t, err = time.Parse(time.RFC3339Nano, rfc3339)
		if err != nil {
			return rfc3339
		}
	}
	t = t.In(loc)
	if t.Hour() == 0 && t.Minute() == 0 {
		return t.Format("2006-01-02")
	}
	return t.Format("2006-01-02 15:04")
}

// ListTaskLists returns all task lists
func (c *TasksClient) ListTaskLists(ctx context.Context) ([]TaskListItem, error) {
	lists, err := c.service.Tasklists.List().Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	result := make([]TaskListItem, 0, len(lists.Items))
	for _, l := range lists.Items {
		result = append(result, TaskListItem{
			ID:    l.Id,
			Title: l.Title,
		})
	}

	return result, nil
}

// ListTasks returns tasks from a specific task list
func (c *TasksClient) ListTasks(ctx context.Context, tasklistID string, showCompleted bool) ([]TaskItem, error) {
	call := c.service.Tasks.List(tasklistID).
		MaxResults(100).
		ShowCompleted(showCompleted).
		ShowHidden(false)

	taskList, err := call.Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	result := make([]TaskItem, 0, len(taskList.Items))
	for _, t := range taskList.Items {
		completed := ""
		if t.Completed != nil {
			completed = *t.Completed
		}
		result = append(result, TaskItem{
			ID:        t.Id,
			Title:     t.Title,
			Notes:     t.Notes,
			Due:       t.Due,
			Status:    t.Status,
			Completed: completed,
		})
	}

	return result, nil
}

// CreateTask creates a new task in the specified task list
func (c *TasksClient) CreateTask(ctx context.Context, tasklistID, title, notes, due string) (*tasks.Task, error) {
	task := &tasks.Task{
		Title: title,
		Notes: notes,
	}

	if due != "" {
		parsed, err := parseDue(due, c.loc)
		if err != nil {
			return nil, err
		}
		task.Due = parsed
	}

	return c.service.Tasks.Insert(tasklistID, task).Context(ctx).Do()
}

// TaskUpdates contains optional fields to update
type TaskUpdates struct {
	Title  *string
	Notes  *string
	Due    *string
	Status *string
}

// UpdateTask updates an existing task
func (c *TasksClient) UpdateTask(ctx context.Context, tasklistID, taskID string, updates TaskUpdates) (*tasks.Task, error) {
	// Get existing task
	existing, err := c.service.Tasks.Get(tasklistID, taskID).Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	// Apply updates
	if updates.Title != nil {
		existing.Title = *updates.Title
	}
	if updates.Notes != nil {
		existing.Notes = *updates.Notes
	}
	if updates.Due != nil {
		if *updates.Due == "" {
			existing.Due = ""
		} else {
			parsed, err := parseDue(*updates.Due, c.loc)
			if err != nil {
				return nil, err
			}
			existing.Due = parsed
		}
	}
	if updates.Status != nil {
		existing.Status = *updates.Status
		if *updates.Status == "completed" {
			completedTime := time.Now().UTC().Format(time.RFC3339)
			existing.Completed = &completedTime
		} else {
			existing.Completed = nil
		}
	}

	return c.service.Tasks.Update(tasklistID, taskID, existing).Context(ctx).Do()
}

// CompleteTask marks a task as completed
func (c *TasksClient) CompleteTask(ctx context.Context, tasklistID, taskID string) (*tasks.Task, error) {
	status := "completed"
	return c.UpdateTask(ctx, tasklistID, taskID, TaskUpdates{Status: &status})
}

// DeleteTask deletes a task
func (c *TasksClient) DeleteTask(ctx context.Context, tasklistID, taskID string) error {
	return c.service.Tasks.Delete(tasklistID, taskID).Context(ctx).Do()
}
