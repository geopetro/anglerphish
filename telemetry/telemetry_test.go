package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	otelapi "go.opentelemetry.io/otel"

	"github.com/gophish/gophish/config"
	"github.com/gophish/gophish/logger"
)

func resetForTest() {
	shutdownFuncs = nil
}

func TestSetupDisabled(t *testing.T) {
	resetForTest()
	if err := Setup(&config.Telemetry{Enable: false}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(shutdownFuncs) != 0 {
		t.Errorf("expected no shutdown funcs, got %d", len(shutdownFuncs))
	}
	if otelapi.GetTracerProvider() == nil {
		t.Error("expected tracer provider to be set")
	}
	if !hasBridgeHook() {
		t.Error("expected BridgeHook to be installed")
	}
}

func TestSetupEnabledWithoutEndpoint(t *testing.T) {
	resetForTest()
	defer Shutdown(context.Background())
	cfg := &config.Telemetry{Enable: true, ServiceName: "gophish-test"}
	if err := Setup(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(shutdownFuncs) != 2 {
		t.Errorf("expected tracer and meter providers registered, got %d", len(shutdownFuncs))
	}
	if !hasBridgeHook() {
		t.Error("expected BridgeHook to be installed")
	}
}

func TestSetupNilConfig(t *testing.T) {
	resetForTest()
	defer Shutdown(context.Background())
	if err := Setup(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMetricsMiddleware(t *testing.T) {
	h := Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics-test", nil))
	if rec.Code != http.StatusTeapot {
		t.Errorf("expected status %d, got %d", http.StatusTeapot, rec.Code)
	}
}

func TestRoutePattern(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test/123", nil)
	if got := RoutePattern(r); got != "/test/123" {
		t.Errorf("expected path fallback, got %q", got)
	}
}

func hasBridgeHook() bool {
	for _, levelHooks := range logger.Logger.Hooks {
		for _, h := range levelHooks {
			if _, ok := h.(BridgeHook); ok {
				return true
			}
		}
	}
	return false
}
