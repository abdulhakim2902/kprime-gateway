package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	labels = []string{"protocol", "method"}

	GatewayIncomingCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "incoming_counter",
		Help: "The total number of incoming request",
	}, labels)

	GatewaySuccessCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "success_counter",
		Help: "The total number of success response",
	}, labels)

	GatewayValidationCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "validation_counter",
		Help: "The total number of validation response",
	}, labels)

	GatewayErrorCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "error_counter",
		Help: "The total number of error",
	}, labels)

	GatewayOutgoingKafkaCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "outgoing_kafka",
		Help: "The total number of outgoing kafka",
	})

	GatewayIncomingKafkaCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "incoming_kafka",
		Help: "The total number of incoming kafka",
	})

	GatewayRequestDurationHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "request_duration",
		Help: "The total number of request duration",
	}, []string{"success"})

	GatewayKafkaDurationHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "kafka_duration",
		Help: "The total number of kafka duration",
	})
)
