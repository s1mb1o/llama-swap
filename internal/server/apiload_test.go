package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mostlygeek/llama-swap/internal/config"
	"github.com/mostlygeek/llama-swap/internal/process"
	"github.com/mostlygeek/llama-swap/internal/shared"
)

// The load endpoint must report process state, not whatever the upstream
// answered: llama-server serves GET / but vLLM returns 404 there, so an
// upstream status code is not a load signal.
func TestServer_HandleAPILoadModel(t *testing.T) {
	newServer := func(upstreamStatus int, endState process.ProcessState) *Server {
		local := newStubRouter([]string{"m1"}, "")
		local.serveHTTP = func(w http.ResponseWriter, r *http.Request) {
			local.running = map[string]process.ProcessState{"m1": endState}
			w.WriteHeader(upstreamStatus)
		}
		s := newTestServer(local, newStubRouter(nil, ""))
		s.cfg = config.Config{Models: map[string]config.ModelConfig{"m1": {}}}
		return s
	}

	t.Run("ready upstream answering 404 still reports 200", func(t *testing.T) {
		s := newServer(http.StatusNotFound, process.StateReady)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/models/load/m1", nil))
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
	})

	t.Run("not ready reports 503", func(t *testing.T) {
		s := newServer(http.StatusOK, process.StateStopped)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/models/load/m1", nil))
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", w.Code)
		}
	})

	t.Run("unknown model 404", func(t *testing.T) {
		s := newServer(http.StatusOK, process.StateReady)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/models/load/nope", nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", w.Code)
		}
	})

	t.Run("probe is a pinned GET /health", func(t *testing.T) {
		var seen *http.Request
		local := newStubRouter([]string{"m1"}, "")
		local.serveHTTP = func(w http.ResponseWriter, r *http.Request) {
			seen = r
			local.running = map[string]process.ProcessState{"m1": process.StateReady}
			w.WriteHeader(http.StatusOK)
		}
		s := newTestServer(local, newStubRouter(nil, ""))
		s.cfg = config.Config{Models: map[string]config.ModelConfig{"m1": {}}}

		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/models/load/m1", nil))

		if seen == nil {
			t.Fatal("router never saw the probe request")
		}
		if seen.Method != http.MethodGet {
			t.Errorf("probe method = %s, want GET", seen.Method)
		}
		if seen.URL.Path != apiLoadProbePath {
			t.Errorf("probe path = %s, want %s", seen.URL.Path, apiLoadProbePath)
		}
		data, err := shared.FetchContext(seen, config.Config{})
		if err != nil {
			t.Fatalf("probe carried no pinned model: %v", err)
		}
		if data.ModelID != "m1" {
			t.Errorf("probe ModelID = %q, want m1", data.ModelID)
		}
	})
}
