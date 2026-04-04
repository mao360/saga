package app

import (
	"context"
	"encoding/json"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/mao360/saga/order/internal/domain"
	"github.com/mao360/saga/order/internal/observability"
	"github.com/mao360/saga/order/internal/platform/postgres"
	"github.com/mao360/saga/order/internal/repository"
	"github.com/mao360/saga/order/internal/transport"
	"github.com/mao360/saga/order/internal/usecase"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	c, err := NewContainer()
	if err != nil {
		return err
	}
	defer c.Close()

	c.Log.Info("starting order service")
	c.Log.Info("running migrations", "dir", "migrations")
	if err := postgres.RunMigrations(c.Cfg.DatabaseDSN, "migrations"); err != nil {
		c.Log.Error("migrations failed", "err", err)
		return err
	}
	c.Log.Info("migrations completed")

	orderRepo := repository.NewOrderRepository(c.Database)
	sagaRepo := repository.NewSagaRepository(c.Database)

	orderUseCase := usecase.NewOrderUseCase(orderRepo, sagaRepo, c.KafkaProducer, c.Cfg.TopicOrderEvents, c.Cfg.TopicCommands)
	processEventUseCase := usecase.NewProcessEventUseCase(orderRepo, sagaRepo, c.KafkaProducer, c.Cfg.TopicCommands)

	httpHandler := transport.NewHTTPHandler(orderUseCase, orderRepo, c.Log)
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	httpHandler.Register(mux)

	httpSrv := &http.Server{
		Addr:    ":" + c.Cfg.HTTPPort,
		Handler: observability.Middleware(c.Tel, mux),
	}

	go func() {
		c.Log.Info("http server starting", "addr", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			c.Log.Error("http server failed", "err", err)
			stop()
		}
	}()

	go func() {
		c.Log.Info("kafka consumer starting")
		err := c.KafkaConsumer.Run(ctx, func(ctx context.Context, rec *kgo.Record) error {
			start := time.Now()
			span := trace.SpanFromContext(ctx)
			if c.Tel != nil {
				ctx = observability.ExtractKafkaHeaders(ctx, c.Tel.Propagator, rec.Headers)
				ctx, span = c.Tel.Tracer.Start(ctx, "order.kafka.consume",
					trace.WithAttributes(
						attribute.String("kafka.topic", rec.Topic),
						attribute.Int64("kafka.offset", rec.Offset),
					),
				)
				defer span.End()
			}

			var event domain.SagaEvent
			if err := json.Unmarshal(rec.Value, &event); err != nil {
				c.Log.Error("invalid event payload", "err", err, "topic", rec.Topic, "offset", rec.Offset)
				if c.Tel != nil {
					c.Tel.Metrics.ObserveKafkaConsume(rec.Topic, err, time.Since(start))
					span.SetStatus(codes.Error, err.Error())
				}
				return err
			}

			c.Log.Info("event received",
				"type", event.Type,
				"saga_id", event.SagaID,
				"order_id", event.OrderID,
				"topic", rec.Topic,
			)

			if err := processEventUseCase.ProcessEvent(ctx, event); err != nil {
				c.Log.Error("event processing failed", "err", err, "type", event.Type, "saga_id", event.SagaID)
				if c.Tel != nil {
					c.Tel.Metrics.ObserveKafkaConsume(rec.Topic, err, time.Since(start))
					span.SetStatus(codes.Error, err.Error())
				}
				return err
			}

			if c.Tel != nil {
				c.Tel.Metrics.ObserveKafkaConsume(rec.Topic, nil, time.Since(start))
				span.SetStatus(codes.Ok, "processed")
			}
			return nil
		})
		if err != nil {
			c.Log.Error("consumer stopped", "err", err)
			stop()
			return
		}
		c.Log.Info("kafka consumer stopped")
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), c.Cfg.ShutdownTimeout)
	defer cancel()

	c.Log.Info("shutting down http server")
	return httpSrv.Shutdown(shutdownCtx)
}