package config

import (
	"os"
	"strings"
)

type Config struct {
	HTTPPort string

	KafkaBrokers  []string
	KafkaClientID string
	KafkaGroupID  string

	TopicOrderEvents     string
	TopicPaymentEvents   string
	TopicInventoryEvents string
	TopicCommands        string
	TopicDLQ             string
}

func Load() Config {
	return Config{
		HTTPPort: getOr("HTTP_PORT", "8080"),

		KafkaBrokers:  splitCSV(getOr("KAFKA_BROKERS", "localhost:9092")),
		KafkaClientID: getOr("KAFKA_CLIENT_ID", "order-service"),
		KafkaGroupID:  getOr("KAFKA_GROUP_ID", "order-service-v1"),

		TopicOrderEvents:     getOr("KAFKA_TOPIC_ORDER_EVENTS", "saga.order.events"),
		TopicPaymentEvents:   getOr("KAFKA_TOPIC_PAYMENT_EVENTS", "saga.payment.events"),
		TopicInventoryEvents: getOr("KAFKA_TOPIC_INVENTORY_EVENTS", "saga.inventory.events"),
		TopicCommands:        getOr("KAFKA_TOPIC_COMMANDS", "saga.commands"),
		TopicDLQ:             getOr("KAFKA_TOPIC_DLQ", "saga.dlq"),
	}
}

func getOr(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"localhost:9092"}
	}
	return out
}
