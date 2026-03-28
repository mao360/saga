package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	commandTotal      *prometheus.CounterVec
	commandDuration   *prometheus.HistogramVec
	kafkaConsumeTotal *prometheus.CounterVec
	kafkaConsumeDur   *prometheus.HistogramVec
	kafkaPublishTotal *prometheus.CounterVec
	kafkaPublishDur   *prometheus.HistogramVec
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		commandTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "payment",
				Name:      "command_total",
				Help:      "Total number of processed commands.",
			},
			[]string{"type", "status"},
		),
		commandDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "payment",
				Name:      "command_duration_seconds",
				Help:      "Command processing duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"type"},
		),
		kafkaConsumeTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "payment",
				Name:      "kafka_consume_total",
				Help:      "Total number of consumed Kafka messages.",
			},
			[]string{"topic", "status"},
		),
		kafkaConsumeDur: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "payment",
				Name:      "kafka_consume_duration_seconds",
				Help:      "Kafka consume handler duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"topic"},
		),
		kafkaPublishTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "payment",
				Name:      "kafka_publish_total",
				Help:      "Total number of produced Kafka messages.",
			},
			[]string{"topic", "status"},
		),
		kafkaPublishDur: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "payment",
				Name:      "kafka_publish_duration_seconds",
				Help:      "Kafka publish duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"topic"},
		),
	}

	reg.MustRegister(
		m.commandTotal,
		m.commandDuration,
		m.kafkaConsumeTotal,
		m.kafkaConsumeDur,
		m.kafkaPublishTotal,
		m.kafkaPublishDur,
	)
	return m
}

func (m *Metrics) ObserveCommand(commandType string, err error, d time.Duration) {
	status := statusFromErr(err)
	m.commandTotal.WithLabelValues(commandType, status).Inc()
	m.commandDuration.WithLabelValues(commandType).Observe(d.Seconds())
}

func (m *Metrics) ObserveKafkaConsume(topic string, err error, d time.Duration) {
	status := statusFromErr(err)
	m.kafkaConsumeTotal.WithLabelValues(topic, status).Inc()
	m.kafkaConsumeDur.WithLabelValues(topic).Observe(d.Seconds())
}

func (m *Metrics) ObserveKafkaPublish(topic string, err error, d time.Duration) {
	status := statusFromErr(err)
	m.kafkaPublishTotal.WithLabelValues(topic, status).Inc()
	m.kafkaPublishDur.WithLabelValues(topic).Observe(d.Seconds())
}

func statusFromErr(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
