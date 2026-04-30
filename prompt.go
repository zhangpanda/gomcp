package gomcp

import "encoding/json"

// PromptHandler returns messages for a prompt.
type PromptHandler func(ctx *Context) ([]PromptMessage, error)

type promptEntry struct {
	info    PromptInfo
	handler PromptHandler
}

// MCP Prompt types

// PromptInfo describes a registered prompt template.
type PromptInfo struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument describes a prompt parameter.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptMessage is a single message in a prompt response.
type PromptMessage struct {
	Role    string       `json:"role"`
	Content PromptContent `json:"content"`
}

// PromptContent holds the content of a prompt message.
type PromptContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// PromptListResult is the response to prompts/list.
type PromptListResult struct {
	Prompts []PromptInfo `json:"prompts"`
}

// GetPromptParams is the request parameters for prompts/get.
type GetPromptParams struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

// GetPromptResult is the response to prompts/get.
type GetPromptResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

// Helper constructors

// UserMsg creates a user-role prompt message.
func UserMsg(text string) PromptMessage {
	return PromptMessage{Role: "user", Content: PromptContent{Type: "text", Text: text}}
}

// AssistantMsg creates an assistant-role prompt message.
func AssistantMsg(text string) PromptMessage {
	return PromptMessage{Role: "assistant", Content: PromptContent{Type: "text", Text: text}}
}

func PromptArg(name, desc string, required bool) PromptArgument {
	return PromptArgument{Name: name, Description: desc, Required: required}
}

// Prompt registers a prompt template.
func (s *Server) Prompt(name, description string, args []PromptArgument, handler PromptHandler) {
	s.mu.Lock()
	s.prompts = append(s.prompts, promptEntry{
		info:    PromptInfo{Name: name, Description: description, Arguments: args},
		handler: handler,
	})
	s.mu.Unlock()
	s.notify("notifications/prompts/list_changed")
}

// handlers

func (s *Server) handlePromptsList(msg *jsonrpcMessage) *jsonrpcMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	prompts := make([]PromptInfo, len(s.prompts))
	for i, p := range s.prompts {
		prompts[i] = p.info
	}
	return newResponse(msg.ID, PromptListResult{Prompts: prompts})
}

func (s *Server) handlePromptsGet(msg *jsonrpcMessage) *jsonrpcMessage {
	var params GetPromptParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return newErrorResponse(msg.ID, -32602, "invalid params")
	}

	s.mu.RLock()
	var found *promptEntry
	for i := range s.prompts {
		if s.prompts[i].info.Name == params.Name {
			p := s.prompts[i]
			found = &p
			break
		}
	}
	s.mu.RUnlock()

	if found == nil {
		return newErrorResponse(msg.ID, -32001, "prompt not found: "+params.Name)
	}

	args := make(map[string]any, len(params.Arguments))
	for k, v := range params.Arguments {
		args[k] = v
	}
	ctx := newContext(s.ctx(), args, s.logger)
	messages, err := found.handler(ctx)
	if err != nil {
		return newErrorResponse(msg.ID, -32603, err.Error())
	}
	return newResponse(msg.ID, GetPromptResult{
		Description: found.info.Description,
		Messages:    messages,
	})
}
