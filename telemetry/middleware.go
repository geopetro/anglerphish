package telemetry

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Telemetry meters for the HTTP servers.
var (
	requestsCounter   metric.Int64Counter
	durationHistogram metric.Float64Histogram
)

func init() {
	m := otelapi.Meter("gophish.http")
	var err error
	requestsCounter, err = m.Int64Counter("gophish.http.requests_total",
		metric.WithDescription("Total number of HTTP requests served"),
		metric.WithUnit("{request}"))
	if err != nil {
		return
	}
	durationHistogram, _ = m.Float64Histogram("gophish.http.request_duration_seconds",
		metric.WithDescription("Duration of HTTP requests"),
		metric.WithUnit("s"))
}

// statusWriter captures the response status code so metrics can record it.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// RoutePattern returns the gorilla/mux path template for the request's route,
// falling back to the request path when the route is not a matched path
// route. It is exported so the HTTP servers can use it for telemetry labels.
func RoutePattern(r *http.Request) string {
	if rt := mux.CurrentRoute(r); rt != nil {
		if t, err := rt.GetPathTemplate(); err == nil {
			return t
		}
	}
	return r.URL.Path
}

// Metrics is an HTTP middleware that records per-request metrics: a request
// counter and a duration histogram, both tagged with method, route and
// status code.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		attrs := []attribute.KeyValue{
			attribute.String("method", r.Method),
			attribute.String("route", RoutePattern(r)),
			attribute.String("status", strconv.Itoa(sw.status)),
		}
		ctx := r.Context()
		requestsCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		durationHistogram.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
	})
}
