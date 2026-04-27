package process

import "sync"

type drwaProcessObservability struct {
	mut      sync.Mutex
	counters map[string]uint64
}

var proxyDRWAMetrics = newDRWAProcessObservability()

func newDRWAProcessObservability() *drwaProcessObservability {
	return &drwaProcessObservability{
		counters: make(map[string]uint64),
	}
}

func (d *drwaProcessObservability) increment(metric string) {
	d.mut.Lock()
	d.counters[metric]++
	d.mut.Unlock()
}

func (d *drwaProcessObservability) snapshot() map[string]uint64 {
	d.mut.Lock()
	defer d.mut.Unlock()

	snapshot := make(map[string]uint64, len(d.counters))
	for key, value := range d.counters {
		snapshot[key] = value
	}

	return snapshot
}

func (d *drwaProcessObservability) reset() {
	d.mut.Lock()
	d.counters = make(map[string]uint64)
	d.mut.Unlock()
}

func recordProxyDRWAMetric(metric string) {
	proxyDRWAMetrics.increment(metric)
}

func snapshotProxyDRWAMetrics() map[string]uint64 {
	return proxyDRWAMetrics.snapshot()
}

func resetProxyDRWAMetrics() {
	proxyDRWAMetrics.reset()
}
