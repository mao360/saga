package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"gopkg.in/yaml.v3"
)

type Config struct {
	HTTPPort string `yaml:"http_port" env:"HTTP_PORT" envDefault:"8083"`

	KafkaBrokers  []string `yaml:"kafka_brokers" env:"KAFKA_BROKERS" envSeparator:","`
	KafkaClientID string   `yaml:"kafka_client_id" env:"KAFKA_CLIENT_ID"`
	KafkaGroupID  string   `yaml:"kafka_group_id" env:"KAFKA_GROUP_ID"`

	TopicOrderEvents     string `yaml:"topic_order_events" env:"KAFKA_TOPIC_ORDER_EVENTS"`
	TopicPaymentEvents   string `yaml:"topic_payment_events" env:"KAFKA_TOPIC_PAYMENT_EVENTS"`
	TopicInventoryEvents string `yaml:"topic_inventory_events" env:"KAFKA_TOPIC_INVENTORY_EVENTS"`
	TopicCommands        string `yaml:"topic_commands" env:"KAFKA_TOPIC_COMMANDS"`
	TopicDLQ             string `yaml:"topic_dlq" env:"KAFKA_TOPIC_DLQ"`

	DatabaseDSN string `yaml:"database_dsn" env:"DATABASE_DSN"`

	DatabaseConnectTimeout time.Duration `yaml:"database_connect_timeout" env:"DATABASE_CONNECT_TIMEOUT" envDefault:"10s"`
	ShutdownTimeout        time.Duration `yaml:"shutdown_timeout" env:"SHUTDOWN_TIMEOUT" envDefault:"5s"`
}

func Load() (*Config, error) {
	cfg := &Config{}

	path := strings.TrimSpace(os.Getenv("APP_CONFIG_PATH"))
	if path == "" {
		path = "internal/platform/config/config.yaml"
	}

	if err := cfg.loadYAML(path); err != nil {
		return nil, err
	}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	cfg.normalize()
	return cfg, nil
}

func (c *Config) loadYAML(path string) error {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, c)
}

func (c *Config) normalize() {
	if len(c.KafkaBrokers) == 0 {
		c.KafkaBrokers = []string{"localhost:9092"}
	}
}
