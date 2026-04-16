package gomcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/istarshine/gomcp/internal/uid"
)

// Task states
const (
	TaskRunning   = "running"
	TaskCompleted = "completed"
	TaskFailed    = "failed"
	TaskCancelled = "cancelled"
)

type task struct {
	mu        sync.Mutex
	ID        string          `json:"id"`
	Tool      string          `json:"tool"`
	Status    string          `json:"status"`
	Result    *CallToolResult `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
	cancel    context.CancelFunc
}

type taskManager struct {
	tasks sync.Map
	sem   chan struct{} // concurrency limiter
}

func newTaskManager(maxConcurrent int) *taskManager {
	if maxConcurrent <= 0 {
		maxConcurrent = 100
	}
	return &taskManager{sem: make(chan struct{}, maxConcurrent)}
}

func (tm *taskManager) submit(toolName string, ctx context.Context, handler func(context.Context) (*CallToolResult, error)) string {
	id := uid.New()
	taskCtx, cancel := context.WithCancel(ctx)
	t := &task{ID: id, Tool: toolName, Status: TaskRunning, CreatedAt: time.Now(), cancel: cancel}
	tm.tasks.Store(id, t)

	go func() {
		// acquire semaphore
		select {
		case tm.sem <- struct{}{}:
			defer func() { <-tm.sem }()
		case <-taskCtx.Done():
			t.mu.Lock()
			t.Status = TaskCancelled
			t.mu.Unlock()
			return
		}

		result, err := handler(taskCtx)

		t.mu.Lock()
		defer t.mu.Unlock()
		if taskCtx.Err() != nil {
			t.Status = TaskCancelled
			return
		}
		if err != nil {
			t.Status = TaskFailed
			t.Error = err.Error()
			return
		}
		t.Status = TaskCompleted
		t.Result = result
	}()

	return id
}

func (tm *taskManager) get(id string) (*task, bool) {
	v, ok := tm.tasks.Load(id)
	if !ok {
		return nil, false
	}
	return v.(*task), true
}

func (tm *taskManager) cancel(id string) bool {
	t, ok := tm.get(id)
	if !ok {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Status == TaskRunning {
		t.cancel()
		t.Status = TaskCancelled
	}
	return true
}

// --- Server integration ---

// AsyncTool registers an async tool that returns a task ID immediately.
func (s *Server) AsyncTool(name, description string, handler HandlerFunc, opts ...ToolOption) {
	s.ensureTaskManager()
	wrapper := func(ctx *Context) (*CallToolResult, error) {
		// capture args and logger for the async goroutine
		args := ctx.Args()
		logger := ctx.Logger()
		id := s.taskMgr.submit(name, ctx.Context(), func(taskCtx context.Context) (*CallToolResult, error) {
			asyncCtx := newContext(taskCtx, args, logger)
			return handler(asyncCtx)
		})
		return TextResult(fmt.Sprintf(`{"taskId":"%s"}`, id)), nil
	}
	s.Tool(name, description, wrapper, opts...)
}

// AsyncToolFunc registers an async typed tool.
func (s *Server) AsyncToolFunc(name, description string, fn any, opts ...ToolOption) {
	s.ensureTaskManager()
	// Register normally first to get the schema-aware handler
	s.ToolFunc(name, description, fn, opts...)

	// Find the registered entry and wrap its handler
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, entry := range s.tools {
		baseName := key
		if i := len(name); len(key) > i && key[i] == '@' {
			baseName = key[:i]
		}
		if baseName == name {
			original := entry.handler
			entry.handler = func(ctx *Context) (*CallToolResult, error) {
				args := ctx.Args()
				logger := ctx.Logger()
				id := s.taskMgr.submit(name, ctx.Context(), func(taskCtx context.Context) (*CallToolResult, error) {
					asyncCtx := newContext(taskCtx, args, logger)
					return original(asyncCtx)
				})
				return TextResult(fmt.Sprintf(`{"taskId":"%s"}`, id)), nil
			}
			s.tools[key] = entry
		}
	}
}

func (s *Server) ensureTaskManager() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.taskMgr == nil {
		s.taskMgr = newTaskManager(100)
	}
}

// SetMaxConcurrentTasks sets the max concurrent async tasks.
func (s *Server) SetMaxConcurrentTasks(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskMgr = newTaskManager(n)
}

// --- Protocol handlers ---

type taskGetParams struct {
	TaskID string `json:"taskId"`
}

type taskResult struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Result *CallToolResult `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func (s *Server) handleTasksGet(msg *jsonrpcMessage) *jsonrpcMessage {
	if s.taskMgr == nil {
		return newErrorResponse(msg.ID, -32001, "tasks not enabled")
	}
	var params taskGetParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return newErrorResponse(msg.ID, -32602, "invalid params")
	}
	t, ok := s.taskMgr.get(params.TaskID)
	if !ok {
		return newErrorResponse(msg.ID, -32001, "task not found: "+params.TaskID)
	}
	t.mu.Lock()
	tr := taskResult{ID: t.ID, Status: t.Status, Result: t.Result, Error: t.Error}
	t.mu.Unlock()
	return newResponse(msg.ID, tr)
}

func (s *Server) handleTasksCancel(msg *jsonrpcMessage) *jsonrpcMessage {
	if s.taskMgr == nil {
		return newErrorResponse(msg.ID, -32001, "tasks not enabled")
	}
	var params taskGetParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return newErrorResponse(msg.ID, -32602, "invalid params")
	}
	if !s.taskMgr.cancel(params.TaskID) {
		return newErrorResponse(msg.ID, -32001, "task not found: "+params.TaskID)
	}
	return newResponse(msg.ID, map[string]string{"status": "cancelled"})
}
