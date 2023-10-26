package metrics

import (
	"fmt"
	"log"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	mainLabel = "job_name"
)

type Metric struct {
	Name   string              `json:"name"`
	Prefix string              `json:"prefix"`
	Labels []map[string]string `json:"labels"`
	Value  float64             `json:"value"`
	Type   string              `json:"type"`
}

type MetricCollector struct {
	PromMetric prometheus.Collector
	Type       string
}

type MetricStore struct {
	PromColectors   map[string]MetricCollector
	metricsRegistry *prometheus.Registry
	metricsMutex    sync.Mutex
}

func NewMetricStore() *MetricStore {
	return &MetricStore{
		PromColectors:   make(map[string]MetricCollector),
		metricsRegistry: prometheus.NewRegistry(),
		metricsMutex:    sync.Mutex{},
	}
}

func (m *MetricStore) RegisterMetric(metric Metric) error {
	m.metricsMutex.Lock()
	defer m.metricsMutex.Unlock()
	if err := metric.ValidateMetrics(true); err != nil {
		return err
	}
	metricFullName := metric.Prefix + "_" + metric.Name
	if _, ok := m.PromColectors[metricFullName]; ok {
		return fmt.Errorf("metric %s already registered", metricFullName)
	}

	labelKeys := []string{mainLabel}
	if len(metric.Labels) > 0 {
		for _, label := range metric.Labels {
			for key := range label {
				labelKeys = append(labelKeys, key)
			}
		}
	}
	switch metric.Type {
	case "counter":
		m.PromColectors[metricFullName] = MetricCollector{
			PromMetric: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: metric.Name,
				Help: fmt.Sprintf("Counter for metric %s", metric.Name),
			}, labelKeys),
			Type: metric.Type,
		}
	case "gauge":
		m.PromColectors[metricFullName] = MetricCollector{
			PromMetric: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: metric.Name,
				Help: fmt.Sprintf("Gauge for metric %s", metric.Name),
			}, labelKeys),
			Type: metric.Type,
		}
	default:
		return fmt.Errorf("unknown metric type: %s", metric.Type)
	}
	m.metricsRegistry.MustRegister(m.PromColectors[metricFullName].PromMetric)
	return nil
}

func (m *MetricStore) StoreMetric(metric Metric) error {
	m.metricsMutex.Lock()
	defer m.metricsMutex.Unlock()
	if err := metric.ValidateMetrics(false); err != nil {
		return err
	}
	log.Println("STORE")
	metricFullName := metric.Prefix + "_" + metric.Name
	if _, ok := m.PromColectors[metricFullName]; !ok {
		return fmt.Errorf("metric %s not registered", metric.Name)
	}

	labelKeys := []string{mainLabel}
	labelValues := []string{metric.Prefix}
	if len(metric.Labels) > 0 {
		for _, label := range metric.Labels {
			for key, val := range label {
				labelKeys = append(labelKeys, key)
				labelValues = append(labelValues, val)
			}
		}
	}
	switch m.PromColectors[metricFullName].Type {
	case "counter":
		m.PromColectors[metricFullName].PromMetric.(*prometheus.CounterVec).WithLabelValues(labelValues...).Add(metric.Value)
	case "gauge":
		m.PromColectors[metricFullName].PromMetric.(*prometheus.GaugeVec).WithLabelValues(labelValues...).Set(metric.Value)
	default:
		return fmt.Errorf("unknown metric type: %s", metric.Type)
	}
	return nil
}

func (m *MetricStore) GetMetricsRegistry() *prometheus.Registry {
	return m.metricsRegistry
}

// ValidateMetrics validates if metrics fields are not empty
func (m Metric) ValidateMetrics(withType bool) error {
	if m.Name == "" {
		return fmt.Errorf("name field cannot be empty")
	}
	if m.Prefix == "" {
		return fmt.Errorf("prefix field cannot be empty")
	}
	if withType && m.Type == "" {
		return fmt.Errorf("type field cannot be empty")
	}
	return nil
}
