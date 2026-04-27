package process

import (
	"testing"

	"github.com/multiversx/mx-chain-proxy-go/data"
	"github.com/multiversx/mx-chain-proxy-go/process/mock"
	"github.com/stretchr/testify/require"
)

func TestNewStatusProcessor(t *testing.T) {
	t.Parallel()

	t.Run("nil base processor - should error", func(t *testing.T) {
		t.Parallel()

		sp, err := NewStatusProcessor(nil, &mock.StatusMetricsProviderStub{})
		require.Nil(t, sp)
		require.Equal(t, ErrNilCoreProcessor, err)
	})

	t.Run("nil status metric provider - should error", func(t *testing.T) {
		t.Parallel()

		sp, err := NewStatusProcessor(&mock.ProcessorStub{}, nil)
		require.Nil(t, sp)
		require.Equal(t, ErrNilStatusMetricsProvider, err)
	})

	t.Run("should work", func(t *testing.T) {
		t.Parallel()

		sp, err := NewStatusProcessor(&mock.ProcessorStub{}, &mock.StatusMetricsProviderStub{})
		require.NoError(t, err)
		require.NotNil(t, sp)
	})
}

func TestStatusProcessor_GetMetrics(t *testing.T) {
	t.Parallel()

	expectedMetrics := map[string]*data.EndpointMetrics{
		"endpoint0": {NumErrors: 5},
		"endpoint1": {NumErrors: 37},
	}
	statusProvider := &mock.StatusMetricsProviderStub{
		GetAllCalled: func() map[string]*data.EndpointMetrics {
			return expectedMetrics
		},
	}
	sp, err := NewStatusProcessor(&mock.ProcessorStub{}, statusProvider)
	require.NoError(t, err)
	require.NotNil(t, sp)

	metrics := sp.GetMetrics()
	require.NoError(t, err)
	require.Equal(t, expectedMetrics, metrics)
}

func TestStatusProcessor_GetMetricsForPrometheus(t *testing.T) {
	t.Parallel()

	resetProxyDRWAMetrics()
	t.Cleanup(resetProxyDRWAMetrics)

	expectedOutput := "metrics"
	statusProvider := &mock.StatusMetricsProviderStub{
		GetMetricsForPrometheusCalled: func() string {
			return expectedOutput
		},
	}
	sp, err := NewStatusProcessor(&mock.ProcessorStub{}, statusProvider)
	require.NoError(t, err)
	require.NotNil(t, sp)

	metrics := sp.GetMetricsForPrometheus()
	require.NoError(t, err)
	require.Equal(t, expectedOutput, metrics)
}

func TestStatusProcessor_GetDRWAMetrics(t *testing.T) {
	resetProxyDRWAMetrics()
	t.Cleanup(resetProxyDRWAMetrics)
	recordProxyDRWAMetric("drwa_signal_accepted")
	recordProxyDRWAMetric("drwa_signal_accepted")

	sp, err := NewStatusProcessor(&mock.ProcessorStub{}, &mock.StatusMetricsProviderStub{})
	require.NoError(t, err)

	metrics := sp.GetDRWAMetrics()
	require.Equal(t, uint64(2), metrics["drwa_signal_accepted"])
}

func TestStatusProcessor_GetMetricsForPrometheus_AppendsProxyObservabilityCounters(t *testing.T) {
	resetProxyDRWAMetrics()
	t.Cleanup(resetProxyDRWAMetrics)

	recordProxyDRWAMetric("drwa_signal_accepted")

	statusProvider := &mock.StatusMetricsProviderStub{
		GetMetricsForPrometheusCalled: func() string {
			return "base_metric 1\n"
		},
	}
	sp, err := NewStatusProcessor(&mock.ProcessorStub{}, statusProvider)
	require.NoError(t, err)

	metrics := sp.GetMetricsForPrometheus()
	require.Contains(t, metrics, "base_metric 1")
	require.Contains(t, metrics, `proxy_drwa{metric="drwa_signal_accepted"} 1`)
}
