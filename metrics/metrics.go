package metrics

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	cron "github.com/robfig/cron/v3"
)

const (
	mainLabel = "job_name"
)

var (
	PrefixFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "prefix_failed",
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
	prefixTicker    map[string]time.Time       // map of prefix and last time it was checked
	prefixUpdated   map[string]time.Time       // map of prefix and last time it was updated
	metricsRegistry *prometheus.Registry       // prometheus registry
	metricsMutex    sync.Mutex                 //	mutex for metricsRegistry
	Cron            *cron.Cron                 // cron
}

func NewMetricStore() *MetricStore {
	m := &MetricStore{
		PromColectors:   make(map[string]MetricCollector),
		prefixTicker:    make(map[string]time.Time),
		prefixUpdated:   make(map[string]time.Time),
		metricsRegistry: prometheus.NewRegistry(),
		metricsMutex:    sync.Mutex{},
		Cron:            cron.New(),
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
		m.prefixUpdated[metric.Prefix] = time.Now()
	case "gauge":
		m.PromColectors[metricFullName] = MetricCollector{
			PromMetric: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: metric.Name,
				Help: metric.Help,
			}, labelKeys),
			Type: metric.Type,
		}
		m.prefixUpdated[metric.Prefix] = time.Now()
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

func (m *MetricStore) RegisterPrefixTicker(prefix string, cronEntry string) error {
	m.metricsMutex.Lock()
	if _, ok := m.prefixTicker[prefix]; ok {
		m.metricsMutex.Unlock()
		return nil
	}
	m.prefixTicker[prefix] = time.Now()
	m.metricsMutex.Unlock()
	_, err := m.Cron.AddFunc(cronEntry, m.CheckSchedule(prefix))
	return err
}

func (m *MetricStore) UnregisterPrefixTicker(prefix string) error {
	m.metricsMutex.Lock()
	defer m.metricsMutex.Unlock()
	if _, ok := m.prefixTicker[prefix]; !ok {
		return nil
	}
	delete(m.prefixTicker, prefix)
	return nil
}

func (m *MetricStore) CheckSchedule(prefix string) func() {
	m.metricsMutex.Lock()
	defer m.metricsMutex.Unlock()
	if _, ok := m.prefixTicker[prefix]; !ok {
		return func() {}
	}
	return func() {
		// TODO protect with separate rlock
		// check if m.prefixUpdated[prefix] is older than m.prefixTicker[prefix]
		if m.prefixUpdated[prefix].Before(m.prefixTicker[prefix]) {
			// inc prefix failed counter
			PrefixFailed.WithLabelValues(prefix).Inc()
		}
		m.prefixTicker[prefix] = time.Now()
	}
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
		m.prefixUpdated[metric.Prefix] = time.Now()
	case "gauge":
		m.PromColectors[metricFullName].PromMetric.(*prometheus.GaugeVec).WithLabelValues(labelValues...).Set(metric.Value)
		m.prefixUpdated[metric.Prefix] = time.Now()
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
