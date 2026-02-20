package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/api/tasks/v1"
)

const (
	serverName    = "google-tasks"
	serverVersion = "1.0.0"

	toolListTaskLists = "list_task_lists"
	toolListTasks     = "list_tasks"
	toolCreateTask    = "create_task"
	toolUpdateTask    = "update_task"
	toolCompleteTask  = "complete_task"
	toolDeleteTask    = "delete_task"

	defaultTasklistID = "@default"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type TasksService interface {
	ListTaskLists(ctx context.Context) ([]TaskListItem, error)
	ListTasks(ctx context.Context, tasklistID string, showCompleted bool) ([]TaskItem, error)
	CreateTask(ctx context.Context, tasklistID, title, notes, due string) (*tasks.Task, error)
	UpdateTask(ctx context.Context, tasklistID, taskID string, updates TaskUpdates) (*tasks.Task, error)
	CompleteTask(ctx context.Context, tasklistID, taskID string) (*tasks.Task, error)
	DeleteTask(ctx context.Context, tasklistID, taskID string) error
}

type Server struct {
	tasks TasksService
	loc   *time.Location
}

func main() {
	credentialsFile := os.Getenv("GOOGLE_OAUTH_CREDENTIALS")
	tokenFile := os.Getenv("GOOGLE_TOKEN_FILE")

	if credentialsFile == "" {
		log.Fatal("GOOGLE_OAUTH_CREDENTIALS environment variable must be set")
	}

	if tokenFile == "" {
		tokenFile = filepath.Join(filepath.Dir(credentialsFile), "tasks-token.json")
	}

	// Check for --auth flag (get URL)
	if len(os.Args) > 1 && os.Args[1] == "--auth" {
		if err := runAuthFlow(credentialsFile, tokenFile); err != nil {
			log.Fatalf("Authorization failed: %v", err)
		}
		return
	}

	// Check for --token flag (exchange code)
	if len(os.Args) > 2 && os.Args[1] == "--token" {
		code := os.Args[2]
		if err := exchangeCode(credentialsFile, tokenFile, code); err != nil {
			log.Fatalf("Token exchange failed: %v", err)
		}
		return
	}

	config, err := getOAuthConfig(credentialsFile)
	if err != nil {
		log.Fatalf("Failed to get OAuth config: %v", err)
	}

	httpClient, err := getClient(config, tokenFile)
	if err != nil {
		log.Fatalf("Failed to get HTTP client: %v", err)
	}

	timezone := os.Getenv("TIMEZONE")
	if timezone == "" {
		timezone = "UTC"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		log.Fatalf("Invalid TIMEZONE %q: %v", timezone, err)
	}

	tasksClient, err := NewTasksClientOAuth(httpClient, loc)
	if err != nil {
		log.Fatalf("Failed to create tasks client: %v", err)
	}

	server := &Server{tasks: tasksClient, loc: loc}
	server.run()
}

func (s *Server) run() {
	scanner := bufio.NewScanner(os.Stdin)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, -32700, "Parse error", err.Error())
			continue
		}

		response := s.handleRequest(req)
		if response != nil {
			s.sendResponse(response)
		}
	}
}

func (s *Server) sendResponse(resp *JSONRPCResponse) {
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func (s *Server) sendError(id interface{}, code int, message string, data interface{}) {
	resp := &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	s.sendResponse(resp)
}

func (s *Server) handleRequest(req JSONRPCRequest) *JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		return nil
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}
}

func (s *Server) handleInitialize(req JSONRPCRequest) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]string{
				"name":    serverName,
				"version": serverVersion,
			},
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
		},
	}
}

func (s *Server) handleToolsList(req JSONRPCRequest) *JSONRPCResponse {
	tools := []map[string]interface{}{
		{
			"name":        toolListTaskLists,
			"description": "List all task lists",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        toolListTasks,
			"description": "List tasks from a task list",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"tasklist_id": map[string]interface{}{
						"type":        "string",
						"description": "Task list ID (use list_task_lists to find IDs, or '@default' for the default list)",
						"default":     defaultTasklistID,
					},
					"show_completed": map[string]interface{}{
						"type":        "boolean",
						"description": "Include completed tasks (default: false)",
						"default":     false,
					},
				},
			},
		},
		{
			"name":        toolCreateTask,
			"description": "Create a new task",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"tasklist_id": map[string]interface{}{
						"type":        "string",
						"description": "Task list ID (use list_task_lists to find IDs, or '@default' for the default list)",
						"default":     defaultTasklistID,
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Task title",
					},
					"notes": map[string]interface{}{
						"type":        "string",
						"description": "Task notes/description (optional)",
					},
					"due": map[string]interface{}{
						"type":        "string",
						"description": "Due date in YYYY-MM-DD or YYYY-MM-DDTHH:MM format (optional)",
					},
				},
				"required": []string{"title"},
			},
		},
		{
			"name":        toolUpdateTask,
			"description": "Update an existing task",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"tasklist_id": map[string]interface{}{
						"type":        "string",
						"description": "Task list ID",
						"default":     defaultTasklistID,
					},
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "Task ID to update (use list_tasks to find IDs)",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "New task title (optional)",
					},
					"notes": map[string]interface{}{
						"type":        "string",
						"description": "New task notes (optional)",
					},
					"due": map[string]interface{}{
						"type":        "string",
						"description": "New due date in YYYY-MM-DD or YYYY-MM-DDTHH:MM format (optional)",
					},
				},
				"required": []string{"task_id"},
			},
		},
		{
			"name":        toolCompleteTask,
			"description": "Mark a task as completed",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"tasklist_id": map[string]interface{}{
						"type":        "string",
						"description": "Task list ID",
						"default":     defaultTasklistID,
					},
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "Task ID to complete (use list_tasks to find IDs)",
					},
				},
				"required": []string{"task_id"},
			},
		},
		{
			"name":        toolDeleteTask,
			"description": "Delete a task",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"tasklist_id": map[string]interface{}{
						"type":        "string",
						"description": "Task list ID",
						"default":     defaultTasklistID,
					},
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "Task ID to delete (use list_tasks to find IDs)",
					},
				},
				"required": []string{"task_id"},
			},
		},
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

func (s *Server) handleToolsCall(req JSONRPCRequest) *JSONRPCResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    err.Error(),
			},
		}
	}

	ctx := context.Background()

	switch params.Name {
	case toolListTaskLists:
		return s.callListTaskLists(ctx, req.ID)
	case toolListTasks:
		return s.callListTasks(ctx, req.ID, params.Arguments)
	case toolCreateTask:
		return s.callCreateTask(ctx, req.ID, params.Arguments)
	case toolUpdateTask:
		return s.callUpdateTask(ctx, req.ID, params.Arguments)
	case toolCompleteTask:
		return s.callCompleteTask(ctx, req.ID, params.Arguments)
	case toolDeleteTask:
		return s.callDeleteTask(ctx, req.ID, params.Arguments)
	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    -32602,
				Message: "Unknown tool: " + params.Name,
			},
		}
	}
}

func (s *Server) callListTaskLists(ctx context.Context, id interface{}) *JSONRPCResponse {
	lists, err := s.tasks.ListTaskLists(ctx)
	if err != nil {
		return s.errorResponse(id, err)
	}

	if len(lists) == 0 {
		return s.successResponse(id, "No task lists found.")
	}

	result := fmt.Sprintf("Found %d task list(s):\n\n", len(lists))
	for _, l := range lists {
		result += fmt.Sprintf("- %s\n  ID: %s\n\n", l.Title, l.ID)
	}

	return s.successResponse(id, result)
}

func (s *Server) callListTasks(ctx context.Context, id interface{}, args json.RawMessage) *JSONRPCResponse {
	var input struct {
		TasklistID    string `json:"tasklist_id"`
		ShowCompleted bool   `json:"show_completed"`
	}
	input.TasklistID = defaultTasklistID

	if len(args) > 0 {
		json.Unmarshal(args, &input)
	}

	if input.TasklistID == "" {
		input.TasklistID = defaultTasklistID
	}

	taskItems, err := s.tasks.ListTasks(ctx, input.TasklistID, input.ShowCompleted)
	if err != nil {
		return s.errorResponse(id, err)
	}

	if len(taskItems) == 0 {
		return s.successResponse(id, "No tasks found.")
	}

	result := fmt.Sprintf("Found %d task(s):\n\n", len(taskItems))
	for _, t := range taskItems {
		status := "[ ]"
		if t.Status == "completed" {
			status = "[x]"
		}
		result += fmt.Sprintf("%s %s\n", status, t.Title)
		if t.Notes != "" {
			result += fmt.Sprintf("  Notes: %s\n", t.Notes)
		}
		if t.Due != "" {
			result += fmt.Sprintf("  Due: %s\n", formatDue(t.Due, s.loc))
		}
		result += fmt.Sprintf("  ID: %s\n\n", t.ID)
	}

	return s.successResponse(id, result)
}

func (s *Server) callCreateTask(ctx context.Context, id interface{}, args json.RawMessage) *JSONRPCResponse {
	var input struct {
		TasklistID string `json:"tasklist_id"`
		Title      string `json:"title"`
		Notes      string `json:"notes"`
		Due        string `json:"due"`
	}
	input.TasklistID = defaultTasklistID

	if err := json.Unmarshal(args, &input); err != nil {
		return s.paramError(id, "Invalid arguments", err.Error())
	}

	if input.Title == "" {
		return s.paramError(id, "title is required", nil)
	}

	if input.TasklistID == "" {
		input.TasklistID = defaultTasklistID
	}

	task, err := s.tasks.CreateTask(ctx, input.TasklistID, input.Title, input.Notes, input.Due)
	if err != nil {
		return s.errorResponse(id, err)
	}

	result := fmt.Sprintf("Task created successfully!\nID: %s\nTitle: %s", task.Id, task.Title)
	if task.Due != "" {
		result += fmt.Sprintf("\nDue: %s", formatDue(task.Due, s.loc))
	}

	return s.successResponse(id, result)
}

func (s *Server) callUpdateTask(ctx context.Context, id interface{}, args json.RawMessage) *JSONRPCResponse {
	var input struct {
		TasklistID string  `json:"tasklist_id"`
		TaskID     string  `json:"task_id"`
		Title      *string `json:"title"`
		Notes      *string `json:"notes"`
		Due        *string `json:"due"`
	}
	input.TasklistID = defaultTasklistID

	if err := json.Unmarshal(args, &input); err != nil {
		return s.paramError(id, "Invalid arguments", err.Error())
	}

	if input.TaskID == "" {
		return s.paramError(id, "task_id is required (use list_tasks to find task IDs)", nil)
	}

	if input.TasklistID == "" {
		input.TasklistID = defaultTasklistID
	}

	updates := TaskUpdates{
		Title: input.Title,
		Notes: input.Notes,
		Due:   input.Due,
	}

	task, err := s.tasks.UpdateTask(ctx, input.TasklistID, input.TaskID, updates)
	if err != nil {
		return s.errorResponse(id, err)
	}

	result := fmt.Sprintf("Task updated successfully!\nID: %s\nTitle: %s", task.Id, task.Title)
	return s.successResponse(id, result)
}

func (s *Server) callCompleteTask(ctx context.Context, id interface{}, args json.RawMessage) *JSONRPCResponse {
	var input struct {
		TasklistID string `json:"tasklist_id"`
		TaskID     string `json:"task_id"`
	}
	input.TasklistID = defaultTasklistID

	if err := json.Unmarshal(args, &input); err != nil {
		return s.paramError(id, "Invalid arguments", err.Error())
	}

	if input.TaskID == "" {
		return s.paramError(id, "task_id is required (use list_tasks to find task IDs)", nil)
	}

	if input.TasklistID == "" {
		input.TasklistID = defaultTasklistID
	}

	task, err := s.tasks.CompleteTask(ctx, input.TasklistID, input.TaskID)
	if err != nil {
		return s.errorResponse(id, err)
	}

	result := fmt.Sprintf("Task completed!\nID: %s\nTitle: %s", task.Id, task.Title)
	return s.successResponse(id, result)
}

func (s *Server) callDeleteTask(ctx context.Context, id interface{}, args json.RawMessage) *JSONRPCResponse {
	var input struct {
		TasklistID string `json:"tasklist_id"`
		TaskID     string `json:"task_id"`
	}
	input.TasklistID = defaultTasklistID

	if err := json.Unmarshal(args, &input); err != nil {
		return s.paramError(id, "Invalid arguments", err.Error())
	}

	if input.TaskID == "" {
		return s.paramError(id, "task_id is required (use list_tasks to find task IDs)", nil)
	}

	if input.TasklistID == "" {
		input.TasklistID = defaultTasklistID
	}

	err := s.tasks.DeleteTask(ctx, input.TasklistID, input.TaskID)
	if err != nil {
		return s.errorResponse(id, err)
	}

	return s.successResponse(id, "Task deleted successfully!")
}

func (s *Server) successResponse(id interface{}, text string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": text},
			},
		},
	}
}

func (s *Server) errorResponse(id interface{}, err error) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
			},
			"isError": true,
		},
	}
}

func (s *Server) paramError(id interface{}, message string, data interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    -32602,
			Message: message,
			Data:    data,
		},
	}
}
