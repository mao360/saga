package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/mao360/saga/payment/internal/domain"
	"github.com/mao360/saga/payment/internal/repository"
)

type PaymentRepository interface {
	Charge(ctx context.Context, cmd domain.Command) (string, error)
	Refund(ctx context.Context, cmd domain.Command) (string, error)
	SetBalance(ctx context.Context, cmd domain.Command) (string, error)
}

type EventPublisher interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
}

type PaymentUseCase struct {
	repo      PaymentRepository
	publisher EventPublisher
	topic     string
}

func NewPaymentUseCase(repo PaymentRepository, publisher EventPublisher, topic string) *PaymentUseCase {
	return &PaymentUseCase{
		repo:      repo,
		publisher: publisher,
		topic:     topic,
	}
}

func (u *PaymentUseCase) ProcessCommand(ctx context.Context, cmd domain.Command) error {
	if strings.TrimSpace(cmd.CommandID) == "" || strings.TrimSpace(cmd.AccountID) == "" || cmd.Amount <= 0 {
		return domain.ErrInvalidCommand
	}

	switch cmd.Type {
	case "charge_payment":
		status, err := u.repo.Charge(ctx, cmd)
		if err != nil {
			return err
		}
		if status == repository.ChargeStatusProcessed {
			return nil
		}
		event := u.baseEvent(cmd)
		if status == repository.ChargeStatusCharged {
			event.Type = "payment_charged"
		} else {
			event.Type = "payment_rejected"
			event.Reason = "insufficient_funds"
		}
		return u.publishEvent(ctx, event)
	case "refund_payment":
		status, err := u.repo.Refund(ctx, cmd)
		if err != nil {
			return err
		}
		if status == repository.RefundStatusProcessed {
			return nil
		}
		event := u.baseEvent(cmd)
		if status == repository.RefundStatusRefunded {
			event.Type = "payment_refunded"
		} else {
			event.Type = "payment_refund_rejected"
			event.Reason = "charge_not_found"
		}
		return u.publishEvent(ctx, event)
	case "set_balance":
		status, err := u.repo.SetBalance(ctx, cmd)
		if err != nil {
			return err
		}
		if status == repository.BalanceStatusProcessed {
			return nil
		}
		event := u.baseEvent(cmd)
		event.Type = "payment_balance_set"
		return u.publishEvent(ctx, event)
	default:
		return nil
	}
}

func (u *PaymentUseCase) baseEvent(cmd domain.Command) domain.Event {
	return domain.Event{
		CommandID:  cmd.CommandID,
		SagaID:     cmd.SagaID,
		OrderID:    cmd.OrderID,
		AccountID:  cmd.AccountID,
		Amount:     cmd.Amount,
		OccurredAt: time.Now().UTC(),
	}
}

func (u *PaymentUseCase) publishEvent(ctx context.Context, event domain.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	key := event.OrderID
	if strings.TrimSpace(key) == "" {
		key = event.AccountID
	}
	return u.publisher.Publish(ctx, u.topic, []byte(key), payload)
}
