package gomcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/zhangpanda/gomcp/internal/uid"
)

// Task states
const (
	TaskRunning   = "running"
	TaskCompleted = "completed"
	TaskFailed    = "failed"
	TaskCancelled = "cancelled"
)

// asyncTaskIDResult returns a JSON object with taskId for async tool responses.
func asyncTaskIDResult(id string) *CallToolResult {
	b, err := json.Marshal(map[string]string{"taskId": id})
	if err != nil {
		return TextResult(fmt.Sprintf(`{"taskId":"%s"}`, id))
	}
	return TextResult(string(b))
}

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
	tasks   sync.Map
	sem     chan struct{} // concurrency limiter
	taskTTL time.Duration
	done    chan struct{}
	// rootCtx is the parent of every submitted task's context so that
	// close() cancels all in-flight AsyncTool handlers. Previously tasks
	// used context.Background() which leaked on Server.Close.
	rootCtx    context.Context
	rootCancel context.CancelFunc
}

func newTaskManager(maxConcurrent int) *taskManager {
	if maxConcurrent <= 0 {
		maxConcurrent = 100
	}
	rootCtx, rootCancel := context.WithCancel(context.Background())
	tm := &taskManager{
		sem:        make(chan struct{}, maxConcurrent),
		taskTTL:    10 * time.Minute,
		done:       make(chan struct{}),
		rootCtx:    rootCtx,
		rootCancel: rootCancel,
	}
	go tm.evictLoop()
	return tm
}

// close cancels all in-flight task contexts and stops the eviction loop.
// Idempotent.
func (tm *taskManager) close() {
	tm.rootCancel()
	select {
	case <-tm.done:
		// already closed
	default:
		close(tm.done)
	}
}

func (tm *taskManager) evictLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-tm.done:
			return
		case <-ticker.C:
			now := time.Now()
			tm.tasks.Range(func(key, value any) bool {
				t, ok := value.(*task)
				if !ok {
					return true
				}
				t.mu.Lock()
				done := t.Status != TaskRunning
				age := now.Sub(t.CreatedAt)
				t.mu.Unlock()
				if done && age > tm.taskTTL {
					tm.tasks.Delete(key)
				}
				return true
			})
		}
	}
}

// submit schedules handler to run with a cancellable context derived from
// tm.rootCtx. The ctx argument is retained for API compatibility but
// effectively only contributes values — deadlines/cancellation flow from
// rootCtx so Server.Close can cancel every in-flight task.
func (tm *taskManager) submit(toolName string, _ context.Context, handler func(context.Context) (*CallToolResult, error)) string {
	id := uid.New()
	taskCtx, cancel := context.WithCancel(tm.rootCtx)
	t := &task{ID: id, Tool: toolName, Status: TaskRunning, CreatedAt: time.Now(), cancel: cancel}
	tm.tasks.Store(id, t)

	go func() {
		defer cancel() // ensure we always release the child context
		// recover panics in async handlers
		defer func() {
			if r := recover(); r != nil {
				t.mu.Lock()
				t.Status = TaskFailed
				t.Error = fmt.Sprintf("panic: %v", r)
				t.mu.Unlock()
			}
		}()

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
	t, ok := v.(*task)
	return t, ok
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
	if !s.ensureTaskManager() {
		return // server already closed
	}
	wrapper := func(ctx *Context) (*CallToolResult, error) {
		// capture args and logger for the async goroutine
		args := ctx.Args()
		logger := ctx.Logger()
		id := s.taskMgr.submit(name, ctx.Context(), func(taskCtx context.Context) (*CallToolResult, error) {
			asyncCtx := newContext(taskCtx, args, logger)
			return handler(asyncCtx)
		})
		return asyncTaskIDResult(id), nil
	}
	s.Tool(name, description, wrapper, opts...)
}

// AsyncToolFunc registers an async typed tool.
func (s *Server) AsyncToolFunc(name, description string, fn any, opts ...ToolOption) {
	if !s.ensureTaskManager() {
		return // server already closed
	}
	// Build the schema-aware entry without registering yet, then wrap its
	// handler with the async trampoline. This happens before we acquire
	// s.mu so no window exists where the tool is callable with the raw
	// sync handler — previously ToolFunc and the subsequent rewrite
	// sat across two separate critical sections.
	entry := s.buildTypedToolEntry(name, description, fn)
	sync := entry.handler
	entry.handler = func(ctx *Context) (*CallToolResult, error) {
		args := ctx.Args()
		logger := ctx.Logger()
		id := s.taskMgr.submit(name, ctx.Context(), func(taskCtx context.Context) (*CallToolResult, error) {
			asyncCtx := newContext(taskCtx, args, logger)
			return sync(asyncCtx)
		})
		return asyncTaskIDResult(id), nil
	}
	s.registerTool(name, entry, opts)
}

// ensureTaskManager lazily initialises s.taskMgr. Returns false if the
// server has already been closed, in which case callers must not
// register new async tools (their goroutines would leak because Close
// already fired).
func (s *Server) ensureTaskManager() bool {
	if s.closed.Load() {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return false
	}
	if s.taskMgr == nil {
		s.taskMgr = newTaskManager(100)
	}
	return true
}

// SetMaxConcurrentTasks sets the max concurrent async tasks.
// Must be called before the first [Server.AsyncTool] or [Server.AsyncToolFunc].
// If a task manager already exists, this is a no-op to avoid race conditions
// with in-flight tasks.
func (s *Server) SetMaxConcurrentTasks(n int) {
	if s.closed.Load() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return
	}
	if s.taskMgr == nil {
		s.taskMgr = newTaskManager(n)
	}
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
