package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockSSE struct {
	lastPublished []byte
}

func (m *mockSSE) ServeHTTP(w http.ResponseWriter, r *http.Request) {}
func (m *mockSSE) Publish(data []byte, channel string) {
	m.lastPublished = data
}

func TestTinywasmHTTP_ActionRoute(t *testing.T) {
	actionCalled := ""
	actionValue := ""
	s := NewTinywasmHTTP("3030", nil, nil, "1.0.0")
	s.OnAction(func(key, value string) {
		actionCalled = key
		actionValue = value
	})

	req := httptest.NewRequest("POST", "/action?key=stop&value=now", nil)
	w := httptest.NewRecorder()

	s.handleActionPOST(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if actionCalled != "stop" {
		t.Errorf("Expected action 'stop', got '%s'", actionCalled)
	}
	if actionValue != "now" {
		t.Errorf("Expected value 'now', got '%s'", actionValue)
	}
}

func TestTinywasmHTTP_StateRoute(t *testing.T) {
	s := NewTinywasmHTTP("3030", nil, nil, "1.0.0")
	s.OnState(func() []byte {
		return []byte(`{"status":"ok"}`)
	})

	req := httptest.NewRequest("GET", "/state", nil)
	w := httptest.NewRecorder()

	s.handleStateGET(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}
	body, _ := io.ReadAll(w.Body)
	if string(body) != `{"status":"ok"}` {
		t.Errorf("Expected body '{\"status\":\"ok\"}', got '%s'", string(body))
	}
}

func TestTinywasmHTTP_VersionRoute(t *testing.T) {
	s := NewTinywasmHTTP("3030", nil, nil, "1.2.3")

	req := httptest.NewRequest("GET", "/version", nil)
	w := httptest.NewRecorder()

	s.handleVersion(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if !strings.Contains(string(body), `"version":"1.2.3"`) {
		t.Errorf("Expected version '1.2.3' in body, got '%s'", string(body))
	}
}

func TestPublishTabLog_JSONFieldNames(t *testing.T) {
	sse := &mockSSE{}
	s := NewTinywasmHTTP("3030", nil, sse, "1.0.0")

	s.PublishTabLog("BUILD", "MCP", "#orange", "hello")

	if sse.lastPublished == nil {
		t.Fatal("Expected log to be published")
	}

	var entry LogEntry
	if err := json.Unmarshal(sse.lastPublished, &entry); err != nil {
		t.Fatalf("Failed to unmarshal log entry: %v", err)
	}

	// JSON field names MUST match devtui/sse_client.go expectations
	// We verify them by checking the LogEntry struct tags and the unmarshaled content
	if entry.TabTitle != "BUILD" {
		t.Errorf("Expected TabTitle 'BUILD', got '%s'", entry.TabTitle)
	}
	if entry.HandlerName != "MCP" {
		t.Errorf("Expected HandlerName 'MCP', got '%s'", entry.HandlerName)
	}
	if entry.Content != "hello" {
		t.Errorf("Expected Content 'hello', got '%s'", entry.Content)
	}

	// Verify the actual JSON keys for devtui compatibility
	var raw map[string]interface{}
	json.Unmarshal(sse.lastPublished, &raw)
	requiredKeys := []string{"id", "timestamp", "content", "type", "tab_title", "handler_name", "handler_color", "handler_type"}
	for _, key := range requiredKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("Missing required JSON key: %s", key)
		}
	}
}

func TestTinywasmHTTP_Name(t *testing.T) {
	s := NewTinywasmHTTP("3030", nil, nil, "1.0.0")
	if s.Name() != "MCP" {
		t.Errorf("Expected Name() to return 'MCP', got '%s'", s.Name())
	}
}
