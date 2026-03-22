package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/mao360/saga/order/internal/domain"
	"github.com/mao360/saga/order/internal/usecase"
)

type OrderCreator interface {
	CreateTestOrder(ctx context.Context) (domain.Order, error)
	CreateOrder(ctx context.Context, input usecase.CreateOrderInput) (domain.Order, error)
	GetOrderByID(ctx context.Context, id string) (domain.Order, error)
}

type ReadinessChecker interface {
	Ping(ctx context.Context) error
}

type HTTPHandler struct {
	creator OrderCreator
	ready   ReadinessChecker
	log     *slog.Logger
}

func NewHTTPHandler(creator OrderCreator, ready ReadinessChecker, log *slog.Logger) *HTTPHandler {
	return &HTTPHandler{
		creator: creator,
		ready:   ready,
		log:     log,
	}
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", h.health)
	mux.HandleFunc("/readyz", h.readyz)
	mux.HandleFunc("/orders", h.orders)
	mux.HandleFunc("/orders/", h.orderByID)
	mux.HandleFunc("/orders/test", h.createTestOrder)
}

func (h *HTTPHandler) health(w http.ResponseWriter, _ *http.Request) {
	start := time.Now()
	h.log.Info("http request started", "method", http.MethodGet, "path", "/healthz")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
	h.log.Info("http request completed", "method", http.MethodGet, "path", "/healthz", "status", http.StatusOK, "duration_ms", time.Since(start).Milliseconds())
}

func (h *HTTPHandler) readyz(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.log.Info("http request started", "method", r.Method, "path", r.URL.Path)
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	if err := h.ready.Ping(ctx); err != nil {
		h.log.Error("readiness check failed", "err", err)
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		h.log.Info("http request completed", "method", r.Method, "path", r.URL.Path, "status", http.StatusServiceUnavailable, "duration_ms", time.Since(start).Milliseconds())
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
	h.log.Info("http request completed", "method", r.Method, "path", r.URL.Path, "status", http.StatusOK, "duration_ms", time.Since(start).Milliseconds())
}

func (h *HTTPHandler) createTestOrder(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.log.Info("http request started", "method", r.Method, "path", r.URL.Path)
	if r.Method != http.MethodPost {
		h.log.Warn("method not allowed", "method", r.Method, "path", r.URL.Path)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		h.log.Info("http request completed", "method", r.Method, "path", r.URL.Path, "status", http.StatusMethodNotAllowed, "duration_ms", time.Since(start).Milliseconds())
		return
	}

	order, err := h.creator.CreateTestOrder(r.Context())
	if err != nil {
		h.log.Error("create test order failed", "err", err)
		if errors.Is(err, domain.ErrInvalidOrder) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			h.log.Info("http request completed", "method", r.Method, "path", r.URL.Path, "status", http.StatusBadRequest, "duration_ms", time.Since(start).Milliseconds())
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		h.log.Info("http request completed", "method", r.Method, "path", r.URL.Path, "status", http.StatusInternalServerError, "duration_ms", time.Since(start).Milliseconds())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(order)
	h.log.Info("http request completed", "method", r.Method, "path", r.URL.Path, "status", http.StatusCreated, "duration_ms", time.Since(start).Milliseconds())
}

func (h *HTTPHandler) orders(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.log.Info("http request started", "method", r.Method, "path", r.URL.Path)
	if r.Method != http.MethodPost {
		h.log.Warn("method not allowed", "method", r.Method, "path", r.URL.Path)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		h.log.Info("http request completed", "method", r.Method, "path", r.URL.Path, "status", http.StatusMethodNotAllowed, "duration_ms", time.Since(start).Milliseconds())
		return
	}

	var input usecase.CreateOrderInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&input); err != nil {
		h.log.Warn("decode order request failed", "err", err, "path", r.URL.Path)
		http.Error(w, "bad request", http.StatusBadRequest)
		h.log.Info("http request completed", "method", r.Method, "path", r.URL.Path, "status", http.StatusBadRequest, "duration_ms", time.Since(start).Milliseconds())
		return
	}

	order, err := h.creator.CreateOrder(r.Context(), input)
	if err != nil {
		status := h.writeOrderError(w, err)
		h.log.Info("http request completed", "method", r.Method, "path", r.URL.Path, "status", status, "duration_ms", time.Since(start).Milliseconds())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(order)
	h.log.Info("http request completed", "method", r.Method, "path", r.URL.Path, "status", http.StatusCreated, "duration_ms", time.Since(start).Milliseconds())
}

func (h *HTTPHandler) orderByID(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.log.Info("http request started", "method", r.Method, "path", r.URL.Path)
	if r.Method != http.MethodGet {
		h.log.Warn("method not allowed", "method", r.Method, "path", r.URL.Path)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		h.log.Info("http request completed", "method", r.Method, "path", r.URL.Path, "status", http.StatusMethodNotAllowed, "duration_ms", time.Since(start).Milliseconds())
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/orders/")
	if id == "" || strings.Contains(id, "/") {
		h.log.Warn("invalid order id path", "path", r.URL.Path)
		http.NotFound(w, r)
		h.log.Info("http request completed", "method", r.Method, "path", r.URL.Path, "status", http.StatusNotFound, "duration_ms", time.Since(start).Milliseconds())
		return
	}

	order, err := h.creator.GetOrderByID(r.Context(), id)
	if err != nil {
		status := h.writeOrderError(w, err)
		h.log.Info("http request completed", "method", r.Method, "path", r.URL.Path, "status", status, "duration_ms", time.Since(start).Milliseconds())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(order)
	h.log.Info("http request completed", "method", r.Method, "path", r.URL.Path, "status", http.StatusOK, "duration_ms", time.Since(start).Milliseconds())
}

func (h *HTTPHandler) writeOrderError(w http.ResponseWriter, err error) int {
	h.log.Error("order request failed", "err", err)
	switch {
	case errors.Is(err, domain.ErrInvalidOrder):
		http.Error(w, err.Error(), http.StatusBadRequest)
		return http.StatusBadRequest
	case errors.Is(err, domain.ErrOrderNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
		return http.StatusNotFound
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
		return http.StatusInternalServerError
	}
}
