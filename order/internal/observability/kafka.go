package observability

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/propagation"
)

type kafkaHeaderCarrier struct {
	headers *[]kgo.RecordHeader
}

func (c kafkaHeaderCarrier) Get(key string) string {
	if c.headers == nil {
		return ""
	}
	for _, h := range *c.headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c kafkaHeaderCarrier) Set(key, value string) {
	if c.headers == nil {
		return
	}
	for i, h := range *c.headers {
		if h.Key == key {
			(*c.headers)[i].Value = []byte(value)
			return
		}
	}
	*c.headers = append(*c.headers, kgo.RecordHeader{
		Key:   key,
		Value: []byte(value),
	})
}

func (c kafkaHeaderCarrier) Keys() []string {
	if c.headers == nil {
		return nil
	}
	out := make([]string, 0, len(*c.headers))
	for _, h := range *c.headers {
		out = append(out, h.Key)
	}
	return out
}

func InjectKafkaHeaders(ctx context.Context, propagator propagation.TextMapPropagator, headers *[]kgo.RecordHeader) {
	if propagator == nil || headers == nil {
		return
	}
	propagator.Inject(ctx, kafkaHeaderCarrier{headers: headers})
}

func ExtractKafkaHeaders(ctx context.Context, propagator propagation.TextMapPropagator, headers []kgo.RecordHeader) context.Context {
	if propagator == nil {
		return ctx
	}
	local := headers
	return propagator.Extract(ctx, kafkaHeaderCarrier{headers: &local})
}
