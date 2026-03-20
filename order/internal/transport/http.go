package transport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/mao360/saga/order/internal/domain"
)

type OrderCreator interface {
	CreateTestOrder(ctx context.Context) (domain.Order, error)
}

type ReadinessChecker interface {
	Ping(ctx context.Context) error
}

type HTTPHandler struct {
	creator OrderCreator
	ready   ReadinessChecker
}

func NewHTTPHandler(creator OrderCreator, ready ReadinessChecker) *HTTPHandler {
	return &HTTPHandler{
		creator: creator,
		ready:   ready,
	}
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", h.health)
	mux.HandleFunc("/readyz", h.readyz)
	mux.HandleFunc("/orders/test", h.createTestOrder)
}

func (h *HTTPHandler) health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *HTTPHandler) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	if err := h.ready.Ping(ctx); err != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

func (h *HTTPHandler) createTestOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	order, err := h.creator.CreateTestOrder(r.Context())
	if err != nil {
		if errors.Is(err, domain.ErrInvalidOrder) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(order)
}
