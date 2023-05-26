package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type metrics struct{}

func ListenAndServeMetrics() error {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		GatewayIncomingRequest,
		GatewaySuccessResponse,
		GatewayValidation,
		GatewayError,
		GatewayOutgoingKafka,
		GatewayIncomingKafka,
		GatewayRequestDuration,
		GatewayKafkaDuration,
	)

	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Ok"))
	}))

	return http.ListenAndServe(":2112", nil)
}
