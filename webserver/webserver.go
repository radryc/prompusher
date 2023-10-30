package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/radryc/prompusher/metrics"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "prompusher_http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)
)

type RequestHandler struct {
	mux         *http.ServeMux
	metricStore *metrics.MetricStore
}

// New http handler
func New(s *http.ServeMux, m *metrics.MetricStore) *RequestHandler {
	h := RequestHandler{s, m}
	p := m.GetMetricsRegistry()
	p.MustRegister(httpRequestsTotal)
	h.registerRoutes(p)
	return &h
}

func (h *RequestHandler) registerRoutes(p *prometheus.Registry) {
	h.mux.HandleFunc("/register", h.Register)
	h.mux.HandleFunc("/store", h.Store)
	h.mux.Handle("/metrics", promhttp.HandlerFor(p, promhttp.HandlerOpts{
		// Opt into OpenMetrics to support exemplars.
		EnableOpenMetrics: true,
	}))
}

func (h *RequestHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var reg metrics.RegistrationRequest
		err := json.NewDecoder(r.Body).Decode(&reg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		m := metrics.Metric{
			Name:   reg.MetricsName,
			Prefix: reg.Prefix,
			Labels: reg.Labels,
			Type:   reg.Type,
			Help:   reg.Help,
		}
		err = h.metricStore.RegisterMetric(m)
		if err != nil {
			httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", http.StatusInternalServerError)).Inc()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(reg.CheckSchedule) > 0 {
			err = h.metricStore.RegisterPrefixTicker(reg.Prefix, reg.CheckSchedule)
			if err != nil {
				httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", http.StatusInternalServerError)).Inc()
				http.Error(w, err.Error(), http.StatusInternalServerError)
				h.metricStore.UnregisterMetric(m)
				return
			}
		}
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", http.StatusOK)).Inc()
		w.WriteHeader(http.StatusOK)
		return
	}
	httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", http.StatusOK)).Inc()
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (h *RequestHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var store metrics.StoreRequest
		err := json.NewDecoder(r.Body).Decode(&store)
		if err != nil {
			httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", http.StatusBadRequest)).Inc()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err = h.metricStore.StoreMetric(metrics.Metric{
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
}
