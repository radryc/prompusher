package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/radryc/prompusher/metrics"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)
)

func main() {
	port := flag.Int("port", 8080, "port number to listen on")
	flag.Parse()

	metricStore := metrics.NewMetricStore()
	promRegistry := metricStore.GetMetricsRegistry()
	promRegistry.MustRegister(httpRequestsTotal)

	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var reg metrics.RegistrationRequest
			err := json.NewDecoder(r.Body).Decode(&reg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			err = metricStore.RegisterMetric(metrics.Metric{
				Name:   reg.MetricsName,
				Prefix: reg.Prefix,
				Labels: reg.Labels,
				Type:   reg.Type,
			})
			if err != nil {
				httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", http.StatusInternalServerError)).Inc()
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", http.StatusOK)).Inc()
			w.WriteHeader(http.StatusOK)
			return
		}
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", http.StatusOK)).Inc()
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	http.HandleFunc("/store", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var store metrics.StoreRequest
			err := json.NewDecoder(r.Body).Decode(&store)
			if err != nil {
				httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", http.StatusBadRequest)).Inc()
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			err = metricStore.StoreMetric(metrics.Metric{
				Name:   store.MetricsName,
				Prefix: store.Prefix,
				Labels: store.Labels,
				Value:  store.Value,
			})
			if err != nil {
				httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", http.StatusInternalServerError)).Inc()
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", http.StatusOK)).Inc()
			w.WriteHeader(http.StatusOK)
			return
		}
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", http.StatusMethodNotAllowed)).Inc()
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	http.Handle("/metrics", promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{
		// Opt into OpenMetrics to support exemplars.
		EnableOpenMetrics: true,
	}))

	log.Printf("Listening on port %d...", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
