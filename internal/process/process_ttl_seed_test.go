package process

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/mostlygeek/llama-swap/internal/config"
)

// TestProcessCommand_TTL_NotFiredBeforeFirstRequest verifies that the TTL is
// measured from readiness, not from a lastUse timestamp that no request has
// ever written.
//
// lastUse is only written when a proxied request completes. A process that has
// just become ready without serving anything therefore carries either a zero
// timestamp or a stale one left by a previous load, and an unseeded TTL
// goroutine unloads the model on its very first tick — throwing away a load
// that may have taken minutes. The existing TTL tests never caught this
// because they each issue a request to "prime the last-use timestamp" before
// waiting. This one deliberately issues none.
func TestProcessCommand_TTL_NotFiredBeforeFirstRequest(t *testing.T) {
	skipIfNoSimpleResponder(t)

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(mock.Close)

	cmd, _ := simpleResponderCmd(t, "-silent")

	cfg := config.ModelConfig{
		Cmd:                cmd,
		Proxy:              mock.URL,
		CheckEndpoint:      "/health",
		HealthCheckTimeout: 10,
		// Long enough that it must not fire during this test. An unseeded
		// lastUse makes time.Since(lastUse) ~56 years, so any TTL fires on
		// the first 1-second tick regardless of how large it is.
		UnloadAfter: 3600,
	}
	if runtime.GOOS == "windows" {
		cfg.CmdStop = "taskkill /f /t /pid ${PID}"
	}

	p := newProcessCommand(t, cfg)

	runErr := runAsync(t, p)
	defer func() {
		if p.State() == StateReady {
			p.Stop(testStopTimeout)
		}
	}()

	if got := p.State(); got != StateReady {
		t.Fatalf("expected StateReady, got %s", got)
	}

	// Span several ticks of the TTL goroutine's 1-second ticker without
	// serving a single request.
	for i := 0; i < 3; i++ {
		time.Sleep(time.Second)
		if got := p.State(); got != StateReady {
			t.Fatalf("model was unloaded %ds after becoming ready with a %ds TTL "+
				"and no requests served (state=%s)", i+1, cfg.UnloadAfter, got)
		}
	}

	if err := p.Stop(testStopTimeout); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return after Stop()")
	}
}
