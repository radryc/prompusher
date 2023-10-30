package prefix

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/robfig/cron/v3"
)

type Prefix struct {
	Checked      map[string]time.Time
	Updated      map[string]time.Time
	mute         sync.RWMutex
	prefixFailed *prometheus.CounterVec
	cronIDs      map[string]cron.EntryID
}

func New(c *prometheus.CounterVec) *Prefix {
	p := Prefix{
		make(map[string]time.Time),
		make(map[string]time.Time),
		sync.RWMutex{},
		c,
		make(map[string]cron.EntryID),
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

func (p *Prefix) UpdateID(prefix string, i cron.EntryID) {
	p.mute.Lock()
	defer p.mute.Unlock()
	p.cronIDs[prefix] = i
}

func (p *Prefix) GetID(prefix string) (cron.EntryID, error) {
	p.mute.RLock()
	defer p.mute.RUnlock()
	if i, ok := p.cronIDs[prefix]; ok {
		return i, nil
	}
	return cron.EntryID(0), fmt.Errorf("cron id for prefix %s not found", prefix)
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
	delete(p.cronIDs, prefix)
}
