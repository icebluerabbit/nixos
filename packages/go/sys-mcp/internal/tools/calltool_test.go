package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

// callTool invokes a registered tool the way a client does — a tools/call
// frame through the server — and returns the text content of the result plus
// whether the tool reported an error.
//
// Driving the server rather than the handler closure is deliberate: the
// handlers are closures created inside each Register function, so this is the
// only path that exercises the registration, the input schema and the handler
// together. That triple is exactly what the tool catalog promises a client.
func callTool(t *testing.T, s *server.MCPServer, name string, args map[string]any) (text string, isError bool) {
	t.Helper()

	if args == nil {
		args = map[string]any{}
	}
	frame, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	raw, err := json.Marshal(s.HandleMessage(context.Background(), frame))
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var resp struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("decode response: %v (raw %s)", err, raw)
	}
	if resp.Error != nil {
		t.Fatalf("%s: protocol error: %s", name, resp.Error.Message)
	}

	for _, c := range resp.Result.Content {
		text += c.Text
	}
	return text, resp.Result.IsError
}

// toolNames lists what a server has registered, which is how the conditional
// registrations are asserted.
func toolNames(t *testing.T, s *server.MCPServer) []string {
	t.Helper()

	frame, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	raw, err := json.Marshal(s.HandleMessage(context.Background(), frame))
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var resp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("decode response: %v (raw %s)", err, raw)
	}

	names := make([]string, 0, len(resp.Result.Tools))
	for _, tool := range resp.Result.Tools {
		names = append(names, tool.Name)
	}
	return names
}

// has reports whether name is in names.
func has(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}
