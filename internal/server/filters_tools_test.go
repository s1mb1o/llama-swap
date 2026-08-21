package server

import (
	"encoding/json"
	"testing"

	"github.com/mostlygeek/llama-swap/internal/config"
	"github.com/tidwall/gjson"
)

// toolNames returns the function names present in a request body, in order.
func toolNames(t *testing.T, body []byte) []string {
	t.Helper()
	var out []string
	for _, tool := range gjson.GetBytes(body, "tools").Array() {
		name := tool.Get("function.name").String()
		if name == "" {
			name = tool.Get("name").String()
		}
		out = append(out, name)
	}
	return out
}

func TestApplyFilters_AllowTools(t *testing.T) {
	openAIBody := []byte(`{"model":"m","tools":[
		{"type":"function","function":{"name":"web_search","parameters":{}}},
		{"type":"function","function":{"name":"run_shell","parameters":{}}},
		{"type":"function","function":{"name":"write_file","parameters":{}}}
	]}`)

	t.Run("keeps only the allowed tool", func(t *testing.T) {
		got, err := applyFilters(openAIBody, "m", "", config.Filters{AllowTools: []string{"web_search"}})
		if err != nil {
			t.Fatalf("applyFilters: %v", err)
		}
		names := toolNames(t, got)
		if len(names) != 1 || names[0] != "web_search" {
			t.Errorf("tools = %v, want [web_search]", names)
		}
		if !json.Valid(got) {
			t.Error("filtered body is not valid JSON")
		}
	})

	t.Run("matching is case-insensitive", func(t *testing.T) {
		got, err := applyFilters(openAIBody, "m", "", config.Filters{AllowTools: []string{"WEB_Search"}})
		if err != nil {
			t.Fatalf("applyFilters: %v", err)
		}
		if names := toolNames(t, got); len(names) != 1 {
			t.Errorf("tools = %v, want exactly one", names)
		}
	})

	t.Run("no allowlist leaves tools untouched", func(t *testing.T) {
		got, err := applyFilters(openAIBody, "m", "", config.Filters{})
		if err != nil {
			t.Fatalf("applyFilters: %v", err)
		}
		if names := toolNames(t, got); len(names) != 3 {
			t.Errorf("tools = %v, want all three kept", names)
		}
	})

	t.Run("nothing allowed removes tools and tool_choice entirely", func(t *testing.T) {
		body := []byte(`{"model":"m","tool_choice":"auto","tools":[
			{"type":"function","function":{"name":"run_shell","parameters":{}}}
		]}`)
		got, err := applyFilters(body, "m", "", config.Filters{AllowTools: []string{"web_search"}})
		if err != nil {
			t.Fatalf("applyFilters: %v", err)
		}
		// An empty tools array is rejected by some upstreams, so the keys must
		// be gone rather than present-but-empty.
		if gjson.GetBytes(got, "tools").Exists() {
			t.Errorf("tools key still present: %s", got)
		}
		if gjson.GetBytes(got, "tool_choice").Exists() {
			t.Errorf("tool_choice still present: %s", got)
		}
	})

	t.Run("tool_choice pinned to a removed tool falls back to auto", func(t *testing.T) {
		body := []byte(`{"model":"m","tool_choice":{"type":"function","function":{"name":"run_shell"}},"tools":[
			{"type":"function","function":{"name":"web_search","parameters":{}}},
			{"type":"function","function":{"name":"run_shell","parameters":{}}}
		]}`)
		got, err := applyFilters(body, "m", "", config.Filters{AllowTools: []string{"web_search"}})
		if err != nil {
			t.Fatalf("applyFilters: %v", err)
		}
		if v := gjson.GetBytes(got, "tool_choice").String(); v != "auto" {
			t.Errorf("tool_choice = %q, want auto", v)
		}
	})

	t.Run("tool_choice pinned to a kept tool is preserved", func(t *testing.T) {
		body := []byte(`{"model":"m","tool_choice":{"type":"function","function":{"name":"web_search"}},"tools":[
			{"type":"function","function":{"name":"web_search","parameters":{}}},
			{"type":"function","function":{"name":"run_shell","parameters":{}}}
		]}`)
		got, err := applyFilters(body, "m", "", config.Filters{AllowTools: []string{"web_search"}})
		if err != nil {
			t.Fatalf("applyFilters: %v", err)
		}
		if v := gjson.GetBytes(got, "tool_choice.function.name").String(); v != "web_search" {
			t.Errorf("tool_choice.function.name = %q, want web_search", v)
		}
	})

	t.Run("anthropic-style top-level tool names", func(t *testing.T) {
		body := []byte(`{"model":"m","tools":[
			{"name":"web_search","input_schema":{}},
			{"name":"bash","input_schema":{}}
		]}`)
		got, err := applyFilters(body, "m", "", config.Filters{AllowTools: []string{"web_search"}})
		if err != nil {
			t.Fatalf("applyFilters: %v", err)
		}
		names := toolNames(t, got)
		if len(names) != 1 || names[0] != "web_search" {
			t.Errorf("tools = %v, want [web_search]", names)
		}
	})

	t.Run("request without tools is untouched", func(t *testing.T) {
		body := []byte(`{"model":"m","messages":[]}`)
		got, err := applyFilters(body, "m", "", config.Filters{AllowTools: []string{"web_search"}})
		if err != nil {
			t.Fatalf("applyFilters: %v", err)
		}
		if string(got) != string(body) {
			t.Errorf("body changed: %s", got)
		}
	})
}
