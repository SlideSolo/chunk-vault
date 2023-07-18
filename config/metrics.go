package config

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

type Metrics struct {
	apiRequestsCount *prometheus.CounterVec
}

const appName = "chunk-vault"
const labelApp = "app"

func InitMetrics(promServeAddr string) *Metrics {
	var m Metrics

	m.apiRequestsCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_count",
			Help: "Number of API requests",
		},
		[]string{labelApp, "method", "endpoint", "status"},
	)

	prometheus.MustRegister(m.apiRequestsCount)

	go func() {
		http.Handle("/metrics", promhttp.Handler())

		if err := http.ListenAndServe(promServeAddr, nil); err != nil {
			log.WithError(err).Fatalln("failed to start prometheus server")
		}
	}()

	return &m
}

func (m *Metrics) IncApiRequestsCount(method, endpoint, status string) {
	m.apiRequestsCount.WithLabelValues(appName, method, endpoint, status).Inc()
}
