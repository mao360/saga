package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/mao360/saga/payment/internal/platform/postgres"
	"github.com/mao360/saga/payment/internal/repository"
	"github.com/mao360/saga/payment/internal/transport"
	"github.com/mao360/saga/payment/internal/usecase"
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

	c.Log.Info("starting payment service")
	if err := postgres.RunMigrations(c.Cfg.DatabaseDSN, "migrations"); err != nil {
		c.Log.Error("migrations failed", "err", err)
		return err
	}
	c.Log.Info("migrations completed")

	paymentRepo := repository.NewPaymentRepository(c.Database)
	paymentUseCase := usecase.NewPaymentUseCase(paymentRepo, c.KafkaProducer, c.Cfg.TopicPaymentEvents)
	kafkaHandler := transport.NewKafkaHandler(paymentUseCase, c.Log, c.Tel)

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

	sample, _ := json.Marshal(map[string]any{
		"topic": c.Cfg.TopicCommands,
		"types": []string{"charge_payment", "refund_payment", "set_balance"},
	})
	c.Log.Info("payment command contract", "example", string(sample))

	<-ctx.Done()
	_ = metricsSrv.Shutdown(context.Background())
	c.Log.Info("payment service stopped")
	return nil
}
