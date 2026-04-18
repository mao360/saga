package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HTTPPort string `yaml:"http_port"`

	KafkaBrokers  []string `yaml:"kafka_brokers"`
	KafkaClientID string   `yaml:"kafka_client_id"`
	KafkaGroupID  string   `yaml:"kafka_group_id"`

	TopicOrderEvents     string `yaml:"topic_order_events"`
	TopicPaymentEvents   string `yaml:"topic_payment_events"`
	TopicInventoryEvents string `yaml:"topic_inventory_events"`
	TopicCommands        string `yaml:"topic_commands"`
	TopicDLQ             string `yaml:"topic_dlq"`

	DatabaseDSN string `yaml:"database_dsn"`

	DatabaseConnectTimeout time.Duration `yaml:"database_connect_timeout"`
	ShutdownTimeout        time.Duration `yaml:"shutdown_timeout"`

	OutboxPollInterval      time.Duration `yaml:"outbox_poll_interval"`
	OutboxBatchSize         int           `yaml:"outbox_batch_size"`
	OutboxCleanerInterval   time.Duration `yaml:"outbox_cleaner_interval"`
	OutboxRetention         time.Duration `yaml:"outbox_retention"`
}

func Load() (Config, error) {
	cfg := defaultConfig()

	path := strings.TrimSpace(getOr("APP_CONFIG_PATH", "internal/platform/config/config.yaml"))
	if err := cfg.loadYAML(path); err != nil {
		return Config{}, err
	}
	cfg.applyEnv()
	cfg.normalize()

	return cfg, nil
}

func defaultConfig() Config {
	return Config{
		HTTPPort: "8080",

		KafkaBrokers:  []string{"localhost:9092"},
		KafkaClientID: "order-service",
		KafkaGroupID:  "order-service-v1",

		TopicOrderEvents:     "saga.order.events",
		TopicPaymentEvents:   "saga.payment.events",
		TopicInventoryEvents: "saga.inventory.events",
		TopicCommands:        "saga.commands",
		TopicDLQ:             "saga.dlq",

		DatabaseDSN: "",

		DatabaseConnectTimeout: 10 * time.Second,
		ShutdownTimeout:        5 * time.Second,

		OutboxPollInterval:    200 * time.Millisecond,
		OutboxBatchSize:       50,
		OutboxCleanerInterval: 10 * time.Minute,
		OutboxRetention:       7 * 24 * time.Hour,
	}
}

func (c *Config) loadYAML(path string) error {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, c)
}

func (c *Config) applyEnv() {
	c.HTTPPort = getOr("HTTP_PORT", c.HTTPPort)

	c.KafkaBrokers = splitCSV(getOr("KAFKA_BROKERS", strings.Join(c.KafkaBrokers, ",")))
	c.KafkaClientID = getOr("KAFKA_CLIENT_ID", c.KafkaClientID)
	c.KafkaGroupID = getOr("KAFKA_GROUP_ID", c.KafkaGroupID)

	c.TopicOrderEvents = getOr("KAFKA_TOPIC_ORDER_EVENTS", c.TopicOrderEvents)
	c.TopicPaymentEvents = getOr("KAFKA_TOPIC_PAYMENT_EVENTS", c.TopicPaymentEvents)
	c.TopicInventoryEvents = getOr("KAFKA_TOPIC_INVENTORY_EVENTS", c.TopicInventoryEvents)
	c.TopicCommands = getOr("KAFKA_TOPIC_COMMANDS", c.TopicCommands)
	c.TopicDLQ = getOr("KAFKA_TOPIC_DLQ", c.TopicDLQ)

	c.DatabaseDSN = getOr("DATABASE_DSN", c.DatabaseDSN)

	c.DatabaseConnectTimeout = parseDurationOr(getOr("DATABASE_CONNECT_TIMEOUT", c.DatabaseConnectTimeout.String()), c.DatabaseConnectTimeout)
	c.ShutdownTimeout = parseDurationOr(getOr("SHUTDOWN_TIMEOUT", c.ShutdownTimeout.String()), c.ShutdownTimeout)

	c.OutboxPollInterval = parseDurationOr(getOr("OUTBOX_POLL_INTERVAL", c.OutboxPollInterval.String()), c.OutboxPollInterval)
	c.OutboxBatchSize = parseIntOr(getOr("OUTBOX_BATCH_SIZE", strconv.Itoa(c.OutboxBatchSize)), c.OutboxBatchSize)
	c.OutboxCleanerInterval = parseDurationOr(getOr("OUTBOX_CLEANER_INTERVAL", c.OutboxCleanerInterval.String()), c.OutboxCleanerInterval)
	c.OutboxRetention = parseDurationOr(getOr("OUTBOX_RETENTION", c.OutboxRetention.String()), c.OutboxRetention)
}

func (c *Config) normalize() {
	if len(c.KafkaBrokers) == 0 {
		c.KafkaBrokers = []string{"localhost:9092"}
	}
	if c.DatabaseConnectTimeout <= 0 {
		c.DatabaseConnectTimeout = 10 * time.Second
	}
	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = 5 * time.Second
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

func parseDurationOr(v string, def time.Duration) time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return d
}

func parseIntOr(v string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return n
}
