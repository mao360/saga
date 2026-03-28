package observability

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	httpRequestsTotal *prometheus.CounterVec
	httpRequestDur    *prometheus.HistogramVec
	kafkaConsumeTotal *prometheus.CounterVec
	kafkaConsumeDur   *prometheus.HistogramVec
	kafkaPublishTotal *prometheus.CounterVec
	kafkaPublishDur   *prometheus.HistogramVec
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		httpRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "order",
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests.",
			},
			[]string{"method", "path", "status"},
		),
		httpRequestDur: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "order",
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		kafkaConsumeTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "order",
				Name:      "kafka_consume_total",
				Help:      "Total number of consumed Kafka messages.",
			},
			[]string{"topic", "status"},
		),
		kafkaConsumeDur: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "order",
				Name:      "kafka_consume_duration_seconds",
				Help:      "Kafka consume handler duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"topic"},
		),
		kafkaPublishTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "order",
				Name:      "kafka_publish_total",
				Help:      "Total number of produced Kafka messages.",
			},
			[]string{"topic", "status"},
		),
		kafkaPublishDur: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "order",
				Name:      "kafka_publish_duration_seconds",
				Help:      "Kafka publish duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"topic"},
		),
	}

	reg.MustRegister(
		m.httpRequestsTotal,
		m.httpRequestDur,
		m.kafkaConsumeTotal,
		m.kafkaConsumeDur,
		m.kafkaPublishTotal,
		m.kafkaPublishDur,
	)
	return m
}

func (m *Metrics) ObserveHTTPRequest(method, path string, status int, d time.Duration) {
	statusLabel := "0"
	if status > 0 {
		statusLabel = strconv.Itoa(status)
	}
	m.httpRequestsTotal.WithLabelValues(method, path, statusLabel).Inc()
	m.httpRequestDur.WithLabelValues(method, path).Observe(d.Seconds())
}

func (m *Metrics) ObserveKafkaConsume(topic string, err error, d time.Duration) {
	m.kafkaConsumeTotal.WithLabelValues(topic, statusFromErr(err)).Inc()
	m.kafkaConsumeDur.WithLabelValues(topic).Observe(d.Seconds())
}

func (m *Metrics) ObserveKafkaPublish(topic string, err error, d time.Duration) {
	m.kafkaPublishTotal.WithLabelValues(topic, statusFromErr(err)).Inc()
	m.kafkaPublishDur.WithLabelValues(topic).Observe(d.Seconds())
}

func statusFromErr(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
