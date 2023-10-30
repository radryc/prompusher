package prefix

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Prefix struct {
	Checked      map[string]time.Time
	Updated      map[string]time.Time
	mute         sync.RWMutex
	prefixFailed *prometheus.CounterVec
}

func New(c *prometheus.CounterVec) *Prefix {
	p := Prefix{
		make(map[string]time.Time),
		make(map[string]time.Time),
		sync.RWMutex{},
		c,
	}
	return &p
}

func (p *Prefix) Add(prefix string) error {
	p.mute.Lock()
	defer p.mute.Unlock()
	if _, ok := p.Checked[prefix]; ok {
		return nil
	}
	p.Checked[prefix] = time.Now()
	p.Updated[prefix] = time.Now()
	return nil
}

func (p *Prefix) Check(prefix string) func() {
	p.mute.Lock()
	defer p.mute.Unlock()
	if _, ok := p.Checked[prefix]; !ok {
		return func() {}
	}
	return func() {
		if p.Updated[prefix].Before(p.Checked[prefix]) {
			func() prometheus.Counter {
				c, err := p.prefixFailed.GetMetricWithLabelValues([]string{prefix}...)
				if err != nil {
					panic(err)
				}
				return c
			}().Inc()
		}
		p.Checked[prefix] = time.Now()
	}
}

func (p *Prefix) Update(prefix string) {
	p.mute.Lock()
	defer p.mute.Unlock()
	p.Updated[prefix] = time.Now()
}

func (p *Prefix) Delete(prefix string) {
	p.mute.Lock()
	defer p.mute.Unlock()
	delete(p.Checked, prefix)
	delete(p.Updated, prefix)
}
