package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/mao360/saga/inventory/internal/platform/postgres"
	"github.com/mao360/saga/inventory/internal/repository"
	"github.com/mao360/saga/inventory/internal/transport"
	"github.com/mao360/saga/inventory/internal/usecase"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/twmb/franz-go/pkg/kgo"
)

func Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	c, err := NewContainer()
	if err != nil {
		return err
	}
	defer c.Close()

	c.Log.Info("starting inventory service")
	if err := postgres.RunMigrations(c.Cfg.DatabaseDSN, "migrations"); err != nil {
		c.Log.Error("migrations failed", "err", err)
		return err
	}
	c.Log.Info("migrations completed")

	inventoryRepo := repository.NewInventoryRepository(c.Database)
	inventoryUseCase := usecase.NewInventoryUseCase(inventoryRepo, c.KafkaProducer, c.Cfg.TopicInventoryEvents)
	kafkaHandler := transport.NewKafkaHandler(inventoryUseCase, c.Log, c.Tel)

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	metricsSrv := &http.Server{
		Addr:    ":" + c.Cfg.HTTPPort,
		Handler: metricsMux,
	}
	go func() {
		c.Log.Info("metrics server starting", "addr", metricsSrv.Addr)
		if serveErr := metricsSrv.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			c.Log.Error("metrics server failed", "err", serveErr)
			stop()
		}
	}()

	c.Log.Info("kafka consumer starting", "topic", c.Cfg.TopicCommands)
	go func() {
		err := c.KafkaConsumer.Run(ctx, func(ctx context.Context, rec *kgo.Record) error {
			return kafkaHandler.Handle(ctx, rec)
		})
		if err != nil {
			c.Log.Error("consumer stopped", "err", err)
			stop()
		}
	}()

	// Example accepted command payloads in saga.commands:
	// reserve: {"command_id":"cmd-1","type":"reserve_inventory","saga_id":"saga-1","order_id":"order-1","sku":"sku-1","qty":1}
	// release: {"command_id":"cmd-2","type":"release_inventory","saga_id":"saga-1","order_id":"order-1","sku":"sku-1","qty":1}
	// set stock: {"command_id":"cmd-3","type":"set_stock","sku":"sku-1","qty":10}
	sample, _ := json.Marshal(map[string]any{
		"topic": c.Cfg.TopicCommands,
		"types": []string{"reserve_inventory", "release_inventory", "set_stock"},
	})
	c.Log.Info("inventory command contract", "example", string(sample))

	<-ctx.Done()
	_ = metricsSrv.Shutdown(context.Background())
	c.Log.Info("inventory service stopped")

	return nil
}
