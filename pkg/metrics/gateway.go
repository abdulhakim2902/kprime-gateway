package metrics

import (
	"fmt"
	"sync"
	"time"

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

var (
	kafkaDurations      map[string]uint64
	kafkaDurationsMutex sync.RWMutex
)

func genKafkaDurationKey(userId, clOrdID string) string {
	return fmt.Sprintf("%s-%s", clOrdID, userId)
}

func cleanUpDuration(key string) {
	kafkaDurationsMutex.RLock()
	defer kafkaDurationsMutex.RUnlock()

	delete(kafkaDurations, key)
}

func StartKafkaDuration(userId, clOrdID string) {
	if kafkaDurations == nil {
		kafkaDurations = make(map[string]uint64)
	}

	key := genKafkaDurationKey(userId, clOrdID)
	start := uint64(time.Now().UnixMicro())

	// Add duration
	kafkaDurationsMutex.RLock()
	defer kafkaDurationsMutex.RUnlock()

	kafkaDurations[key] = start
}

func EndKafkaDuration(userId, clOrdID string) {
	key := genKafkaDurationKey(userId, clOrdID)
	start, ok := kafkaDurations[key]
	if !ok {
		return
	}

	end := uint64(time.Now().UnixMicro())

	go func(diff float64) {
		GatewayKafkaDurationHistogram.Observe(diff)
	}(float64(end - start))

	// Release duration
	cleanUpDuration(key)
}
