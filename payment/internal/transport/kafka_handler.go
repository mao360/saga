package transport

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/mao360/saga/payment/internal/domain"
	"github.com/mao360/saga/payment/internal/observability"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type CommandProcessor interface {
	ProcessCommand(ctx context.Context, cmd domain.Command) error
}

type KafkaHandler struct {
	processor CommandProcessor
	log       *slog.Logger
	telemetry *observability.Telemetry
}

func NewKafkaHandler(processor CommandProcessor, log *slog.Logger, telemetry *observability.Telemetry) *KafkaHandler {
	return &KafkaHandler{
		processor: processor,
		log:       log,
		telemetry: telemetry,
	}
}

func (h *KafkaHandler) Handle(ctx context.Context, rec *kgo.Record) error {
	start := time.Now()
	commandType := "unknown"
	span := trace.SpanFromContext(ctx)

	if h.telemetry != nil {
		ctx = observability.ExtractKafkaHeaders(ctx, h.telemetry.Propagator, rec.Headers)
		ctx, span = h.telemetry.Tracer.Start(ctx, "payment.kafka.consume",
			trace.WithAttributes(
				attribute.String("kafka.topic", rec.Topic),
				attribute.Int64("kafka.offset", rec.Offset),
			),
		)
		defer span.End()
	}

	var cmd domain.Command
	if err := json.Unmarshal(rec.Value, &cmd); err != nil {
		h.log.Error("invalid command payload", "err", err, "topic", rec.Topic, "offset", rec.Offset)
		if h.telemetry != nil {
			h.telemetry.Metrics.ObserveCommand(commandType, err, time.Since(start))
			h.telemetry.Metrics.ObserveKafkaConsume(rec.Topic, err, time.Since(start))
			span.SetStatus(codes.Error, err.Error())
		}
		return err
	}
	commandType = cmd.Type

	h.log.Info("command received",
		"command_id", cmd.CommandID,
		"type", cmd.Type,
		"order_id", cmd.OrderID,
		"account_id", cmd.AccountID,
		"amount", cmd.Amount,
	)

	if err := h.processor.ProcessCommand(ctx, cmd); err != nil {
		h.log.Error("command processing failed", "err", err, "command_id", cmd.CommandID)
		if h.telemetry != nil {
			h.telemetry.Metrics.ObserveCommand(commandType, err, time.Since(start))
			h.telemetry.Metrics.ObserveKafkaConsume(rec.Topic, err, time.Since(start))
			span.SetStatus(codes.Error, err.Error())
		}
		return err
	}

	if h.telemetry != nil {
		h.telemetry.Metrics.ObserveCommand(commandType, nil, time.Since(start))
		h.telemetry.Metrics.ObserveKafkaConsume(rec.Topic, nil, time.Since(start))
		span.SetStatus(codes.Ok, "processed")
	}

	h.log.Info("command processed", "command_id", cmd.CommandID, "type", cmd.Type)
	return nil
}
