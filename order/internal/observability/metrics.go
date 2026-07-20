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
	sagaDuration      *prometheus.HistogramVec
	sagaPending       prometheus.Gauge
	sagaOldestPending prometheus.Gauge
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
		// Ключевая бизнес-метрика: сколько живёт распределённая транзакция
		// целиком — от создания заказа до терминального статуса. HTTP-латентность
		// этого не показывает, потому что POST /orders отвечает сразу после
		// записи в outbox, когда сага ещё даже не началась.
		sagaDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "order",
				Name:      "saga_duration_seconds",
				Help:      "End-to-end saga duration from order creation to terminal status.",
				// 5ms .. ~82s: под нагрузкой хвост уезжает далеко за DefBuckets.
				Buckets: prometheus.ExponentialBuckets(0.005, 2, 15),
			},
			[]string{"status"},
		),
		sagaPending: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "order",
				Name:      "saga_pending_orders",
				Help:      "Number of orders currently in non-terminal status.",
			},
		),
		sagaOldestPending: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "order",
				Name:      "saga_oldest_pending_age_seconds",
				Help:      "Age of the oldest order still in non-terminal status.",
			},
		),
	}

	reg.MustRegister(
		m.httpRequestsTotal,
		m.httpRequestDur,
		m.kafkaConsumeTotal,
		m.kafkaConsumeDur,
		m.kafkaPublishTotal,
		m.kafkaPublishDur,
		m.sagaDuration,
		m.sagaPending,
		m.sagaOldestPending,
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

// ObserveSagaFinished вызывается после коммита транзакции, которая перевела
// заказ в терминальный статус. status — completed или failed.
func (m *Metrics) ObserveSagaFinished(status string, d time.Duration) {
	m.sagaDuration.WithLabelValues(status).Observe(d.Seconds())
}

// SetSagaPending обновляет снимок незавершённых саг. Заполняется фоновым
// сборщиком: застрявшая сага не порождает событий и иначе была бы не видна.
func (m *Metrics) SetSagaPending(count int64, oldestAge time.Duration) {
	m.sagaPending.Set(float64(count))
	m.sagaOldestPending.Set(oldestAge.Seconds())
}

func statusFromErr(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
