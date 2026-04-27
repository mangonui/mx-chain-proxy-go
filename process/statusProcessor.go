package process

import (
	"fmt"
	"sort"
	"strings"

	"github.com/multiversx/mx-chain-core-go/core/check"
	"github.com/multiversx/mx-chain-proxy-go/data"
)

// StatusProcessor is able to process status requests
type StatusProcessor struct {
	proc                  Processor
	statusMetricsProvider StatusMetricsProvider
}

// NewStatusProcessor creates a new instance of AccountProcessor
func NewStatusProcessor(proc Processor, statusMetricsProvider StatusMetricsProvider) (*StatusProcessor, error) {
	if check.IfNil(proc) {
		return nil, ErrNilCoreProcessor
	}
	if check.IfNil(statusMetricsProvider) {
		return nil, ErrNilStatusMetricsProvider
	}

	return &StatusProcessor{
		proc:                  proc,
		statusMetricsProvider: statusMetricsProvider,
	}, nil
}

// GetMetrics returns the metrics for all the endpoints
func (sp *StatusProcessor) GetMetrics() map[string]*data.EndpointMetrics {
	return sp.statusMetricsProvider.GetAll()
}

// GetDRWAMetrics returns the DRWA-specific proxy observability counters.
func (sp *StatusProcessor) GetDRWAMetrics() map[string]uint64 {
	return snapshotProxyDRWAMetrics()
}

// GetMetricsForPrometheus returns the metrics in a prometheus format
func (sp *StatusProcessor) GetMetricsForPrometheus() string {
	metrics := sp.statusMetricsProvider.GetMetricsForPrometheus()
	drwaMetrics := renderProxyCounterMetrics("proxy_drwa", snapshotProxyDRWAMetrics())
	if drwaMetrics == "" {
		return metrics
	}

	var builder strings.Builder
	builder.WriteString(metrics)
	if metrics != "" && !strings.HasSuffix(metrics, "\n") {
		builder.WriteByte('\n')
	}
	builder.WriteString(drwaMetrics)

	return builder.String()
}

func renderProxyCounterMetrics(namespace string, counters map[string]uint64) string {
	if len(counters) == 0 {
		return ""
	}

	keys := make([]string, 0, len(counters))
	for key := range counters {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(fmt.Sprintf("%s{metric=%q} %d\n", namespace, key, counters[key]))
	}

	return builder.String()
}
