package process

import (
	"testing"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/stretchr/testify/require"
)

func TestMaterializeDRWADetailsFromFailReason(t *testing.T) {
	resetProxyDRWAMetrics()
	t.Cleanup(resetProxyDRWAMetrics)

	result := materializeDRWADetails("execution failed: DRWA_KYC_REQUIRED receiver not eligible", nil, "")
	require.NotNil(t, result)
	require.True(t, result.IsDrwa)
	require.Equal(t, "DRWA_UNKNOWN", result.DenialCode)
	metrics := snapshotProxyDRWAMetrics()
	require.Equal(t, uint64(1), metrics["drwa_denial_detected"])
}

func TestMaterializeDRWADetailsFromSCRReturnMessage(t *testing.T) {
	t.Parallel()

	result := materializeDRWADetails("", map[string]*transaction.ApiSmartContractResult{
		"scr": {ReturnMessage: "DRWA_TOKEN_PAUSED token is paused"},
	}, "")
	require.NotNil(t, result)
	require.Equal(t, "DRWA_TOKEN_PAUSED", result.DenialCode)
}

func TestMaterializeDRWADetailsFromLogs(t *testing.T) {
	resetProxyDRWAMetrics()
	t.Cleanup(resetProxyDRWAMetrics)

	result := materializeDRWADetailsWithTrustedEmitters("", map[string]*transaction.ApiSmartContractResult{
		"scr": {
			Function: "setTokenPolicy",
			RcvAddr:  "erd1policy",
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Address:    "erd1policy",
						Identifier: "drwaTransferDenied",
						Topics:     [][]byte{[]byte("DRWA_JURISDICTION_BLOCKED")},
					},
				},
			},
		},
	}, "", newDRWATrustedEmitterSet("erd1policy"))
	require.NotNil(t, result)
	require.True(t, result.HasComplianceSignal)
	require.Len(t, result.DenialTopics, 1)
	metrics := snapshotProxyDRWAMetrics()
	require.Equal(t, uint64(1), metrics["drwa_signal_accepted"])
}

func TestMaterializeDRWADetailsRejectsLogSignalsWithoutTrustedEmitters(t *testing.T) {
	resetProxyDRWAMetrics()
	t.Cleanup(resetProxyDRWAMetrics)

	result := materializeDRWADetails("", map[string]*transaction.ApiSmartContractResult{
		"scr": {
			Function: "setTokenPolicy",
			RcvAddr:  "erd1policy",
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Address:    "erd1policy",
						Identifier: "drwaTransferDenied",
						Topics:     [][]byte{[]byte("DRWA_JURISDICTION_BLOCKED")},
					},
				},
			},
		},
	}, "")

	require.Nil(t, result)
	metrics := snapshotProxyDRWAMetrics()
	require.Equal(t, uint64(1), metrics["drwa_signal_rejected"])
}

func TestMaterializeDRWADetailsIgnoresTransportFailures(t *testing.T) {
	t.Parallel()

	result := materializeDRWADetails("gateway timeout while fetching SCR", map[string]*transaction.ApiSmartContractResult{
		"scr": {ReturnMessage: "upstream unavailable"},
	}, "")
	require.Nil(t, result)
}

func TestMaterializeDRWADetailsPrefersFailReasonOverSCRDenial(t *testing.T) {
	t.Parallel()

	result := materializeDRWADetails("execution failed: DRWA_KYC_REQUIRED source holder missing approval", map[string]*transaction.ApiSmartContractResult{
		"scr": {ReturnMessage: "DRWA_TOKEN_PAUSED token is paused"},
	}, "")

	require.NotNil(t, result)
	require.Equal(t, "DRWA_UNKNOWN", result.DenialCode)
	require.Contains(t, result.DenialMessage, "DRWA_KYC_REQUIRED")
}

func TestMaterializeDRWADetailsUsesDeterministicSCROrder(t *testing.T) {
	t.Parallel()

	result := materializeDRWADetails("", map[string]*transaction.ApiSmartContractResult{
		"z-scr": {ReturnMessage: "DRWA_TOKEN_PAUSED token is paused"},
		"a-scr": {ReturnMessage: "DRWA_KYC_REQUIRED holder is not eligible"},
	}, "")

	require.NotNil(t, result)
	require.Equal(t, "DRWA_UNKNOWN", result.DenialCode)
	require.Contains(t, result.DenialMessage, "DRWA_KYC_REQUIRED")
}

func TestMaterializeDRWADetailsIgnoresSubstringFalsePositive(t *testing.T) {
	t.Parallel()

	result := materializeDRWADetails("execution failed: XDRWA_KYC_REQUIREDY", nil, "")
	require.Nil(t, result)
}

func TestMaterializeDRWADetailsIgnoresSubstringFalsePositiveInTopic(t *testing.T) {
	t.Parallel()

	result := materializeDRWADetailsWithTrustedEmitters("", map[string]*transaction.ApiSmartContractResult{
		"scr": {
			Function: "recordAttestation",
			RcvAddr:  "erd1attestation",
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Address:    "erd1attestation",
						Identifier: "drwaAttestationRecorded",
						Topics:     [][]byte{[]byte("XDRWA_GLOBAL_PAUSEY")},
					},
				},
			},
		},
	}, "", newDRWATrustedEmitterSet("erd1attestation"))

	require.NotNil(t, result)
	require.True(t, result.IsDrwa)
	require.True(t, result.HasComplianceSignal)
	require.Equal(t, "", result.DenialCode)
}

func TestMaterializeDRWADetailsExtractsDenialFromTopic(t *testing.T) {
	t.Parallel()

	result := materializeDRWADetailsWithTrustedEmitters("", map[string]*transaction.ApiSmartContractResult{
		"scr": {
			Function: "setTokenPolicy",
			RcvAddr:  "erd1policy",
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Address:    "erd1policy",
						Identifier: "drwaTransferDenied",
						Topics:     [][]byte{[]byte("DRWA_GLOBAL_PAUSE")},
					},
				},
			},
		},
	}, "", newDRWATrustedEmitterSet("erd1policy"))

	require.NotNil(t, result)
	require.Equal(t, "DRWA_GLOBAL_PAUSE", result.DenialCode)
	require.Equal(t, "DRWA_GLOBAL_PAUSE", result.DenialMessage)
}

func TestMaterializeDRWADetailsPreservesTopicMessageContext(t *testing.T) {
	t.Parallel()

	result := materializeDRWADetailsWithTrustedEmitters("", map[string]*transaction.ApiSmartContractResult{
		"scr": {
			Function: "setTokenPolicy",
			RcvAddr:  "erd1policy",
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Address:    "erd1policy",
						Identifier: "drwaTransferDenied",
						Topics:     [][]byte{[]byte("DRWA_GLOBAL_PAUSE token paused by governance")},
					},
				},
			},
		},
	}, "", newDRWATrustedEmitterSet("erd1policy"))

	require.NotNil(t, result)
	require.Equal(t, "DRWA_GLOBAL_PAUSE", result.DenialCode)
	require.Equal(t, "DRWA_GLOBAL_PAUSE token paused by governance", result.DenialMessage)
}

func TestMaterializeDRWADetailsRecognizesAttestationSignalWithoutDenial(t *testing.T) {
	t.Parallel()

	result := materializeDRWADetailsWithTrustedEmitters("", map[string]*transaction.ApiSmartContractResult{
		"scr": {
			Function: "recordAttestation",
			RcvAddr:  "erd1attestation",
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Address:    "erd1attestation",
						Identifier: "drwaAttestationRecorded",
						Topics:     [][]byte{[]byte("HOTEL-1234"), []byte("erd1subject")},
					},
				},
			},
		},
	}, "", newDRWATrustedEmitterSet("erd1attestation"))

	require.NotNil(t, result)
	require.True(t, result.HasComplianceSignal)
	require.Equal(t, "", result.DenialCode)
}

func TestMaterializeDRWADetailsIgnoresCanonicalEventsWithoutTrustedFunctionContext(t *testing.T) {
	t.Parallel()

	result := materializeDRWADetailsWithTrustedEmitters("", map[string]*transaction.ApiSmartContractResult{
		"scr": {
			Function: "customPing",
			RcvAddr:  "erd1spoof",
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Address:    "erd1spoof",
						Identifier: "drwaTransferAllowed",
						Topics:     [][]byte{[]byte("DRWA_TRANSFER_LOCKED")},
					},
				},
			},
		},
	}, "", newDRWATrustedEmitterSet("erd1spoof"))

	require.Nil(t, result)
}

func TestMaterializeDRWADetailsIgnoresCanonicalEventsFromUnexpectedEmitter(t *testing.T) {
	resetProxyDRWAMetrics()
	t.Cleanup(resetProxyDRWAMetrics)

	result := materializeDRWADetailsWithTrustedEmitters("", map[string]*transaction.ApiSmartContractResult{
		"scr": {
			Function: "setTokenPolicy",
			RcvAddr:  "erd1policy",
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Address:    "erd1spoof",
						Identifier: "drwaTransferAllowed",
						Topics:     [][]byte{[]byte("DRWA_TRANSFER_LOCKED")},
					},
				},
			},
		},
	}, "", newDRWATrustedEmitterSet("erd1policy"))

	require.Nil(t, result)
	metrics := snapshotProxyDRWAMetrics()
	require.Equal(t, uint64(1), metrics["drwa_signal_rejected"])
}
