package gomcp

import "encoding/json"

// CompletionHandler returns completion suggestions for a partial input.
type CompletionHandler func(partial string) []string

type completionEntry struct {
	refType string // "ref/prompt" or "ref/resource"
	refName string
	argName string
	handler CompletionHandler
}

// Completion registers an auto-complete handler for a prompt or resource argument.
// refType is "prompt" or "resource", refName is the prompt/resource name, argName is the argument.
func (s *Server) Completion(refType, refName, argName string, handler CompletionHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completions = append(s.completions, completionEntry{
		refType: "ref/" + refType,
		refName: refName,
		argName: argName,
		handler: handler,
	})
}

// protocol types

type completeParams struct {
	Ref      completeRef      `json:"ref"`
	Argument completeArgument `json:"argument"`
}

type completeRef struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type completeArgument struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type completeResult struct {
	Completion completeValues `json:"completion"`
}

type completeValues struct {
	Values  []string `json:"values"`
	HasMore bool     `json:"hasMore,omitempty"`
	Total   int      `json:"total,omitempty"`
}

func (s *Server) handleComplete(msg *jsonrpcMessage) *jsonrpcMessage {
	var params completeParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return newErrorResponse(msg.ID, -32602, "invalid params")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, c := range s.completions {
		if c.refType == params.Ref.Type && c.refName == params.Ref.Name && c.argName == params.Argument.Name {
			values := c.handler(params.Argument.Value)
			if len(values) > 100 {
				return newResponse(msg.ID, completeResult{
					Completion: completeValues{Values: values[:100], HasMore: true, Total: len(values)},
				})
			}
			return newResponse(msg.ID, completeResult{
				Completion: completeValues{Values: values, Total: len(values)},
			})
		}
	}

	return newResponse(msg.ID, completeResult{Completion: completeValues{Values: []string{}}})
}
