package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/mao360/saga/order/internal/app"
	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	c, err := app.NewContainer()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// HTTP health endpoints
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	httpSrv := &http.Server{
		Addr:    ":" + c.Cfg.HTTPPort,
		Handler: mux,
	}

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			c.Log.Error("http server failed", "err", err)
			stop()
		}
	}()

	// Kafka consumer loop
	go func() {
		err := c.KafkaConsumer.Run(ctx, func(ctx context.Context, rec *kgo.Record) error {
			// Временный handler. Тут потом роутинг по event_type.
			var payload map[string]any
			if err := json.Unmarshal(rec.Value, &payload); err != nil {
				return err
			}
			c.Log.Info("event received",
				"topic", rec.Topic,
				"key", string(rec.Key),
				"payload", payload,
			)
			return nil
		})
		if err != nil {
			c.Log.Error("consumer stopped", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	_ = httpSrv.Shutdown(context.Background())
}
