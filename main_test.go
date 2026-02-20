package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/tasks/v1"
)

type fakeTasks struct {
	taskLists     []TaskListItem
	taskItems     []TaskItem
	err           error
	created       *tasks.Task
	updated       *tasks.Task
	completed     *tasks.Task
	deleteErr     error
	lastTasklist  string
	lastTaskID    string
	lastCompleted bool
}

func (f *fakeTasks) ListTaskLists(_ context.Context) ([]TaskListItem, error) {
	return f.taskLists, f.err
}

func (f *fakeTasks) ListTasks(_ context.Context, tasklistID string, showCompleted bool) ([]TaskItem, error) {
	f.lastTasklist = tasklistID
	f.lastCompleted = showCompleted
	return f.taskItems, f.err
}

func (f *fakeTasks) CreateTask(_ context.Context, tasklistID, title, notes, due string) (*tasks.Task, error) {
	f.lastTasklist = tasklistID
	return f.created, f.err
}

func (f *fakeTasks) UpdateTask(_ context.Context, tasklistID, taskID string, updates TaskUpdates) (*tasks.Task, error) {
	f.lastTasklist = tasklistID
	f.lastTaskID = taskID
	return f.updated, f.err
}

func (f *fakeTasks) CompleteTask(_ context.Context, tasklistID, taskID string) (*tasks.Task, error) {
	f.lastTasklist = tasklistID
	f.lastTaskID = taskID
	return f.completed, f.err
}

func (f *fakeTasks) DeleteTask(_ context.Context, tasklistID, taskID string) error {
	f.lastTasklist = tasklistID
	f.lastTaskID = taskID
	return f.deleteErr
}

func newTestServer(fake *fakeTasks) *Server {
	return &Server{tasks: fake, loc: time.UTC}
}

func TestHandleInitialize(t *testing.T) {
	s := newTestServer(&fakeTasks{})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1), Method: "initialize"}

	resp := s.handleRequest(req)
	if resp == nil {
		t.Fatal("expected response")
	}

	result := resp.Result.(map[string]interface{})
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("expected protocol version 2024-11-05, got %v", result["protocolVersion"])
	}

	serverInfo := result["serverInfo"].(map[string]string)
	if serverInfo["name"] != "google-tasks" {
		t.Errorf("expected server name google-tasks, got %s", serverInfo["name"])
	}
}

func TestHandleInitialized(t *testing.T) {
	s := newTestServer(&fakeTasks{})
	resp := s.handleRequest(JSONRPCRequest{JSONRPC: "2.0", Method: "initialized"})
	if resp != nil {
		t.Error("expected nil response for initialized notification")
	}
}

func TestHandleUnknownMethod(t *testing.T) {
	s := newTestServer(&fakeTasks{})
	resp := s.handleRequest(JSONRPCRequest{JSONRPC: "2.0", ID: float64(1), Method: "unknown"})
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Error("expected method not found error")
	}
}

func TestHandleToolsList(t *testing.T) {
	s := newTestServer(&fakeTasks{})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1), Method: "tools/list"}

	resp := s.handleRequest(req)
	result := resp.Result.(map[string]interface{})
	tools := result["tools"].([]map[string]interface{})

	expected := []string{"list_task_lists", "list_tasks", "create_task", "update_task", "complete_task", "delete_task"}
	if len(tools) != len(expected) {
		t.Fatalf("expected %d tools, got %d", len(expected), len(tools))
	}
	for i, name := range expected {
		if tools[i]["name"] != name {
			t.Errorf("tool %d: expected %q, got %q", i, name, tools[i]["name"])
		}
	}
}

func TestCallUnknownTool(t *testing.T) {
	s := newTestServer(&fakeTasks{})
	params, _ := json.Marshal(map[string]interface{}{"name": "nonexistent"})
	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1), Method: "tools/call", Params: params}

	resp := s.handleRequest(req)
	if resp.Error == nil || resp.Error.Code != -32602 {
		t.Error("expected invalid params error for unknown tool")
	}
}

// list_task_lists

func TestCallListTaskLists(t *testing.T) {
	fake := &fakeTasks{
		taskLists: []TaskListItem{
			{ID: "list1", Title: "My Tasks"},
			{ID: "list2", Title: "Work"},
		},
	}
	s := newTestServer(fake)

	resp := s.callListTaskLists(context.Background(), float64(1))
	text := getResponseText(t, resp)
	if !strings.Contains(text, "My Tasks") || !strings.Contains(text, "Work") {
		t.Errorf("expected both task lists in response, got: %s", text)
	}
}

func TestCallListTaskLists_Empty(t *testing.T) {
	s := newTestServer(&fakeTasks{})
	resp := s.callListTaskLists(context.Background(), float64(1))
	text := getResponseText(t, resp)
	if text != "No task lists found." {
		t.Errorf("expected 'No task lists found.', got %q", text)
	}
}

// list_tasks

func TestCallListTasks_DefaultTasklist(t *testing.T) {
	fake := &fakeTasks{
		taskItems: []TaskItem{
			{ID: "t1", Title: "Buy milk", Status: "needsAction"},
		},
	}
	s := newTestServer(fake)

	resp := s.callListTasks(context.Background(), float64(1), nil)
	if fake.lastTasklist != "@default" {
		t.Errorf("expected default tasklist, got %s", fake.lastTasklist)
	}
	text := getResponseText(t, resp)
	if !strings.Contains(text, "Buy milk") {
		t.Errorf("expected task title in response, got: %s", text)
	}
}

func TestCallListTasks_WithNotesAndDue(t *testing.T) {
	fake := &fakeTasks{
		taskItems: []TaskItem{
			{ID: "t1", Title: "Task", Status: "needsAction", Notes: "Some notes", Due: "2026-03-01T00:00:00Z"},
		},
	}
	s := newTestServer(fake)

	resp := s.callListTasks(context.Background(), float64(1), nil)
	text := getResponseText(t, resp)
	if !strings.Contains(text, "Some notes") || !strings.Contains(text, "2026-03-01") {
		t.Errorf("expected notes and due in response, got: %s", text)
	}
}

func TestCallListTasks_WithDueTime(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Tbilisi")
	fake := &fakeTasks{
		taskItems: []TaskItem{
			{ID: "t1", Title: "Task", Status: "needsAction", Due: "2026-03-01T10:30:00+04:00"},
		},
	}
	s := &Server{tasks: fake, loc: loc}

	resp := s.callListTasks(context.Background(), float64(1), nil)
	text := getResponseText(t, resp)
	if !strings.Contains(text, "2026-03-01 10:30") {
		t.Errorf("expected formatted due with time, got: %s", text)
	}
}

func TestCallListTasks_CompletedCheckbox(t *testing.T) {
	fake := &fakeTasks{
		taskItems: []TaskItem{
			{ID: "t1", Title: "Done task", Status: "completed"},
		},
	}
	s := newTestServer(fake)

	resp := s.callListTasks(context.Background(), float64(1), nil)
	text := getResponseText(t, resp)
	if !strings.Contains(text, "[x]") {
		t.Errorf("expected [x] checkbox for completed task, got: %s", text)
	}
}

func TestCallListTasks_Empty(t *testing.T) {
	s := newTestServer(&fakeTasks{})
	resp := s.callListTasks(context.Background(), float64(1), nil)
	text := getResponseText(t, resp)
	if text != "No tasks found." {
		t.Errorf("expected 'No tasks found.', got %q", text)
	}
}

// create_task

func TestCallCreateTask(t *testing.T) {
	fake := &fakeTasks{
		created: &tasks.Task{Id: "new-1", Title: "New task", Due: "2026-03-15T00:00:00Z"},
	}
	s := newTestServer(fake)

	args, _ := json.Marshal(map[string]string{"title": "New task", "due": "2026-03-15"})
	resp := s.callCreateTask(context.Background(), float64(1), args)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	text := getResponseText(t, resp)
	if !strings.Contains(text, "new-1") || !strings.Contains(text, "2026-03-15") {
		t.Errorf("expected task ID and due date in response, got: %s", text)
	}
}

func TestCallCreateTask_MissingTitle(t *testing.T) {
	s := newTestServer(&fakeTasks{})
	args, _ := json.Marshal(map[string]string{"notes": "no title"})
	resp := s.callCreateTask(context.Background(), float64(1), args)
	if resp.Error == nil {
		t.Error("expected error for missing title")
	}
}

// update_task

func TestCallUpdateTask(t *testing.T) {
	fake := &fakeTasks{
		updated: &tasks.Task{Id: "t1", Title: "Updated"},
	}
	s := newTestServer(fake)

	args, _ := json.Marshal(map[string]interface{}{"task_id": "t1", "title": "Updated"})
	resp := s.callUpdateTask(context.Background(), float64(1), args)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if fake.lastTaskID != "t1" {
		t.Errorf("expected task ID t1, got %s", fake.lastTaskID)
	}
}

func TestCallUpdateTask_MissingTaskID(t *testing.T) {
	s := newTestServer(&fakeTasks{})
	args, _ := json.Marshal(map[string]string{"title": "No ID"})
	resp := s.callUpdateTask(context.Background(), float64(1), args)
	if resp.Error == nil {
		t.Error("expected error for missing task_id")
	}
	if !strings.Contains(resp.Error.Message, "list_tasks") {
		t.Errorf("expected hint about list_tasks in error, got: %s", resp.Error.Message)
	}
}

// complete_task

func TestCallCompleteTask(t *testing.T) {
	fake := &fakeTasks{
		completed: &tasks.Task{Id: "t1", Title: "Done"},
	}
	s := newTestServer(fake)

	args, _ := json.Marshal(map[string]string{"task_id": "t1"})
	resp := s.callCompleteTask(context.Background(), float64(1), args)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if fake.lastTaskID != "t1" {
		t.Errorf("expected task ID t1, got %s", fake.lastTaskID)
	}
}

func TestCallCompleteTask_MissingTaskID(t *testing.T) {
	s := newTestServer(&fakeTasks{})
	args, _ := json.Marshal(map[string]string{})
	resp := s.callCompleteTask(context.Background(), float64(1), args)
	if resp.Error == nil {
		t.Error("expected error for missing task_id")
	}
}

// delete_task

func TestCallDeleteTask(t *testing.T) {
	fake := &fakeTasks{}
	s := newTestServer(fake)

	args, _ := json.Marshal(map[string]string{"task_id": "t-del"})
	resp := s.callDeleteTask(context.Background(), float64(1), args)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if fake.lastTaskID != "t-del" {
		t.Errorf("expected task ID t-del, got %s", fake.lastTaskID)
	}
}

func TestCallDeleteTask_MissingTaskID(t *testing.T) {
	s := newTestServer(&fakeTasks{})
	args, _ := json.Marshal(map[string]string{})
	resp := s.callDeleteTask(context.Background(), float64(1), args)
	if resp.Error == nil {
		t.Error("expected error for missing task_id")
	}
}

// helpers

func TestSuccessResponse(t *testing.T) {
	s := newTestServer(&fakeTasks{})
	resp := s.successResponse(float64(1), "hello")
	text := getResponseText(t, resp)
	if text != "hello" {
		t.Errorf("expected 'hello', got %q", text)
	}
}

func TestErrorResponse(t *testing.T) {
	s := newTestServer(&fakeTasks{})
	resp := s.errorResponse(float64(1), context.DeadlineExceeded)
	result := resp.Result.(map[string]interface{})
	if !result["isError"].(bool) {
		t.Error("expected isError to be true")
	}
}

func getResponseText(t *testing.T, resp *JSONRPCResponse) string {
	t.Helper()
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	result := resp.Result.(map[string]interface{})
	content := result["content"].([]map[string]string)
	return content[0]["text"]
}
