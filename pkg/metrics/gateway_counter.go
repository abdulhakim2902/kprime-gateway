package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	labels = []string{"protocol", "method"}

	GatewayIncomingRequest = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "incoming_counter",
		Help: "The total number of incoming request",
	}, labels)

	GatewaySuccessResponse = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "success_counter",
		Help: "The total number of success response",
	}, labels)

	GatewayValidation = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "validation_counter",
		Help: "The total number of validation",
	}, labels)

	GatewayError = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "error_counter",
		Help: "The total number of error",
	}, labels)

	GatewayOutgoingKafka = promauto.NewCounter(prometheus.CounterOpts{
		Name: "outgoing_kafka",
		Help: "The total number of outgoing kafka",
	})

	GatewayIncomingKafka = promauto.NewCounter(prometheus.CounterOpts{
		Name: "incoming_kafka",
		Help: "The total number of incoming kafka",
	})

	GatewayRequestDuration = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "request_duration",
		Help: "The total number of request duration",
	}, []string{"success"})

	GatewayKafkaDuration = promauto.NewCounter(prometheus.CounterOpts{
		Name: "kafka_duration",
		Help: "The total number of kafka duration",
	})
)
