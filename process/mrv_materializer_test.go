package process

import (
	"testing"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/stretchr/testify/require"
)

func TestMaterializeMRVDetailsFromAnchoredProofLog(t *testing.T) {
	t.Parallel()

	result := materializeMRVDetailsWithTrustedEmitters(map[string]*transaction.ApiSmartContractResult{
		"scr": {
			Function: "anchorReportV2",
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Address:    "erd1mrvregistry",
						Identifier: "mrvReportAnchoredV2",
						Topics: [][]byte{
							[]byte("report-1"),
							[]byte("tenant-1"),
							[]byte("farm-1"),
							[]byte("season-1"),
						},
						AdditionalData: [][]byte{
							[]byte("sha256:report"),
							[]byte("sha256"),
							[]byte("json-canonical"),
							[]byte{0x03},
							[]byte{0x67, 0x1a, 0xed, 0x90},
							[]byte("project-1"),
							[]byte("sha256:manifest"),
						},
					},
				},
			},
		},
	}, "", newMRVTrustedEmitterSet("erd1mrvregistry"))

	require.NotNil(t, result)
	require.True(t, result.IsMrv)
	require.True(t, result.HasAnchoredProof)
	require.Equal(t, "mrvReportAnchoredV2", result.SourceEvent)
	require.Equal(t, "report-1", result.ReportID)
	require.Equal(t, "tenant-1", result.PublicTenantID)
	require.Equal(t, "project-1", result.PublicProjectID)
	require.Equal(t, "sha256:manifest", result.EvidenceManifestHash)
	require.Equal(t, uint64(3), result.MethodologyVersion)
	require.Equal(t, uint64(1729818000), result.AnchoredAt)
}

func TestMaterializeMRVDetailsRejectsLogSignalsWithoutTrustedEmitters(t *testing.T) {
	t.Parallel()

	result := materializeMRVDetails(map[string]*transaction.ApiSmartContractResult{
		"scr": {
			Function: "anchorReportV2",
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Address:    "erd1mrvregistry",
						Identifier: "mrvReportAnchoredV2",
						Topics:     [][]byte{[]byte("report-1")},
					},
				},
			},
		},
	}, "")

	require.Nil(t, result)
}

func TestMaterializeMRVDetailsIgnoresNonMRVFunctions(t *testing.T) {
	t.Parallel()

	result := materializeMRVDetailsWithTrustedEmitters(map[string]*transaction.ApiSmartContractResult{
		"scr": {
			Function: "customCall",
			Logs: &transaction.ApiLogs{
				Events: []*transaction.Events{
					{
						Address:    "erd1mrvregistry",
						Identifier: "mrvReportAnchoredV2",
						Topics:     [][]byte{[]byte("report-1")},
					},
				},
			},
		},
	}, "", newMRVTrustedEmitterSet("erd1mrvregistry"))

	require.Nil(t, result)
}
