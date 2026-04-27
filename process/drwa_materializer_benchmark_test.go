package process

import (
	"testing"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
)

func BenchmarkMaterializeDRWADetails_FromFailReason(b *testing.B) {
	failReason := "execution failed: DRWA_KYC_REQUIRED receiver not eligible"

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result := materializeDRWADetails(failReason, nil, "")
		if result == nil || result.DenialCode != "DRWA_UNKNOWN" {
			b.Fatalf("unexpected result: %+v", result)
		}
	}
}

func BenchmarkMaterializeDRWADetails_FromSCRAndLogs(b *testing.B) {
	scrs := map[string]*transaction.ApiSmartContractResult{
		"scr": {
			Function:      "setTokenPolicy",
			ReturnMessage: "DRWA_TOKEN_PAUSED token is paused",
			RcvAddr:       "erd1drwacontract",
			Logs: &transaction.ApiLogs{
				Address: "erd1drwacontract",
				Events: []*transaction.Events{
					{
						Address:    "erd1drwacontract",
						Identifier: "drwaTransferDenied",
						Topics:     [][]byte{[]byte("DRWA_TOKEN_PAUSED")},
					},
				},
			},
		},
	}
	trustedEmitters := newDRWATrustedEmitterSet("erd1drwacontract")

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result := materializeDRWADetailsWithTrustedEmitters("", scrs, "", trustedEmitters)
		if result == nil || result.DenialCode != "DRWA_TOKEN_PAUSED" {
			b.Fatalf("unexpected result: %+v", result)
		}
	}
}
