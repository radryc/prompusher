package metrics

import (
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/radryc/prompusher/prefix"
	cron "github.com/robfig/cron/v3"
)

const (
	mainLabel = "job_name"
)

var (
	PrefixFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "prompusher_prefix_failed",
			Help: "Total number of failed prefix checks.",
		},
		[]string{"prefix"},
	)
)

type Metric struct {
	Name   string              `json:"name"`
	Prefix string              `json:"prefix"`
	Labels []map[string]string `json:"labels"`
	Value  float64             `json:"value"`
	Type   string              `json:"type"`
	Help   string              `json:"help"`
}

type MetricCollector struct {
	PromMetric prometheus.Collector
	Type       string
}

type MetricStore struct {
	PromColectors   map[string]MetricCollector // all prometheus metrics
	metricsRegistry *prometheus.Registry       // prometheus registry
	metricsMutex    sync.Mutex                 // mutex for metricsRegistry
	Cron            *cron.Cron                 // cron
	prefix          *prefix.Prefix             // Prefix checker
}

func NewMetricStore() *MetricStore {
	m := &MetricStore{
		PromColectors:   make(map[string]MetricCollector),
		metricsRegistry: prometheus.NewRegistry(),
		metricsMutex:    sync.Mutex{},
		Cron:            cron.New(),
		prefix:          prefix.New(PrefixFailed),
	}
	m.metricsRegistry.MustRegister(PrefixFailed)
	return m
}

func (m *MetricStore) RegisterMetric(metric Metric) error {
	if err := metric.ValidateMetrics(true); err != nil {
		return err
	}
	labelKeys := []string{mainLabel}
	if len(metric.Labels) > 0 {
		for _, label := range metric.Labels {
			for key := range label {
				labelKeys = append(labelKeys, key)
			}
		}
	}
	if metric.Help == "" {
		metric.Help = fmt.Sprintf("%s for metric %s", metric.Type, metric.Name)
	}

	m.metricsMutex.Lock()
	defer m.metricsMutex.Unlock()
	metricFullName := metric.Prefix + "_" + metric.Name
	if _, ok := m.PromColectors[metricFullName]; ok {
		return fmt.Errorf("metric %s already registered", metricFullName)
	}
	switch metric.Type {
	case "counter":
		m.PromColectors[metricFullName] = MetricCollector{
			PromMetric: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: metric.Name,
				Help: metric.Help,
			}, labelKeys),
			Type: metric.Type,
		}
	case "gauge":
		m.PromColectors[metricFullName] = MetricCollector{
			PromMetric: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: metric.Name,
				Help: metric.Help,
			}, labelKeys),
			Type: metric.Type,
		}
	default:
		return fmt.Errorf("unknown metric type: %s", metric.Type)
	}
	m.metricsRegistry.MustRegister(m.PromColectors[metricFullName].PromMetric)
	return nil
}

func (m *MetricStore) UnregisterMetric(metric Metric) error {
	m.metricsMutex.Lock()
	defer m.metricsMutex.Unlock()
	metricFullName := metric.Prefix + "_" + metric.Name
	if _, ok := m.PromColectors[metricFullName]; !ok {
		return nil
	}
	m.metricsRegistry.Unregister(m.PromColectors[metricFullName].PromMetric)
	delete(m.PromColectors, metricFullName)
	return nil
}

func (m *MetricStore) RegisterPrefix(prefix string, cronEntry string) error {
	if err := m.prefix.Add(prefix); err != nil {
		return err
	}
	cid, err := m.Cron.AddFunc(cronEntry, m.prefix.Check(prefix))
	if err != nil {
		return err
	}
	m.prefix.UpdateID(prefix, cid)
	return nil
}

func (m *MetricStore) UnregisterPrefix(prefix string) error {
	cid, err := m.prefix.GetID(prefix)
	if err != nil {
		return err
	}
	m.Cron.Remove(cid)
	m.prefix.Delete(prefix)
	return nil
}

func (m *MetricStore) StoreMetric(metric Metric) error {
	if err := metric.ValidateMetrics(false); err != nil {
		return err
	}
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
	m.metricsMutex.Lock()
	defer m.metricsMutex.Unlock()
	switch m.PromColectors[metricFullName].Type {
	case "counter":
		m.PromColectors[metricFullName].PromMetric.(*prometheus.CounterVec).WithLabelValues(labelValues...).Add(metric.Value)
		m.prefix.Update(metric.Prefix)
	case "gauge":
		m.PromColectors[metricFullName].PromMetric.(*prometheus.GaugeVec).WithLabelValues(labelValues...).Set(metric.Value)
		m.prefix.Update(metric.Prefix)
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

// StartCron starts cron
func (m *MetricStore) StartCron() {
	m.Cron.Start()
}

// StopCron stops cron
func (m *MetricStore) StopCron() {
	m.Cron.Stop()
}
