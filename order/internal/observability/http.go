package observability

import (
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func Middleware(t *Telemetry, next http.Handler) http.Handler {
	if t == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		path := normalizePath(r.URL.Path)
		ctx, span := t.Tracer.Start(r.Context(), "order.http.request",
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.route", path),
				attribute.String("http.target", r.URL.Path),
			),
		)

		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r.WithContext(ctx))
		d := time.Since(start)

		t.Metrics.ObserveHTTPRequest(r.Method, path, rw.status, d)
		if rw.status >= 500 {
			span.SetStatus(codes.Error, "http_server_error")
		} else {
			span.SetStatus(codes.Ok, "ok")
		}
		span.SetAttributes(attribute.Int("http.status_code", rw.status))
		span.End()
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func normalizePath(path string) string {
	if strings.HasPrefix(path, "/orders/") {
		return "/orders/:id"
	}
	if path == "" {
		return "/"
	}
	return path
}
