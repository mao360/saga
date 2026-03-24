package transport

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/mao360/saga/payment/internal/domain"
	"github.com/twmb/franz-go/pkg/kgo"
)

type CommandProcessor interface {
	ProcessCommand(ctx context.Context, cmd domain.Command) error
}

type KafkaHandler struct {
	processor CommandProcessor
	log       *slog.Logger
}

func NewKafkaHandler(processor CommandProcessor, log *slog.Logger) *KafkaHandler {
	return &KafkaHandler{
		processor: processor,
		log:       log,
	}
}

func (h *KafkaHandler) Handle(ctx context.Context, rec *kgo.Record) error {
	var cmd domain.Command
	if err := json.Unmarshal(rec.Value, &cmd); err != nil {
		h.log.Error("invalid command payload", "err", err, "topic", rec.Topic, "offset", rec.Offset)
		return err
	}

	h.log.Info("command received",
		"command_id", cmd.CommandID,
		"type", cmd.Type,
		"order_id", cmd.OrderID,
		"account_id", cmd.AccountID,
		"amount", cmd.Amount,
	)

	if err := h.processor.ProcessCommand(ctx, cmd); err != nil {
		h.log.Error("command processing failed", "err", err, "command_id", cmd.CommandID)
		return err
	}

	h.log.Info("command processed", "command_id", cmd.CommandID, "type", cmd.Type)
	return nil
}
