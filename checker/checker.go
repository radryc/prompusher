package checker

import (
	"net/http"

	"github.com/radryc/prompusher/metrics"
)

type Checker struct {
	metricStore *metrics.MetricStore
}

func New(_ *http.ServeMux, m *metrics.MetricStore) *Checker {
	c := Checker{m}
	return &c
}
