package process

import "testing"

func TestIsDRWACanonicalEvent_AcceptsKnownIdentifiers(t *testing.T) {
	cases := []string{
		"drwaTokenPolicy",
		"drwaAssetRegistered",
		"drwaAssetUpdated",
		"drwaHolderCompliance",
		"drwaTransferDenied",
		"drwaTransferAllowed",
		"drwaGlobalPause",
		"drwaMetadataProtection",
		"drwaWhitePaperCidSet",
		"drwaRegistrationStatusSet",
		"drwaIdentityRegistered",
		"drwaComplianceUpdated",
		"drwaIdentityDeactivated",
		"drwaIdentityErased",
		"drwaWindDownInitiated",
		"drwaAuditorProposed",
		"drwaAuditorAccepted",
		"drwaAuditorRevoked",
		"drwaAttestationOverwritten",
		"drwaAttestationRecorded",
		"drwaGovernanceProposed",
		"drwaGovernanceAccepted",
		"drwaGovernanceRevoked",
	}
	for _, identifier := range cases {
		if !isDRWACanonicalEvent(identifier) {
			t.Fatalf("expected identifier %q to be accepted", identifier)
		}
	}
}

func TestIsDRWACanonicalEvent_RejectsUnknown(t *testing.T) {
	if isDRWACanonicalEvent("drwaUnknownEvent") {
		t.Fatalf("unexpected acceptance of unknown DRWA event")
	}
	if isDRWACanonicalEvent("") {
		t.Fatalf("unexpected acceptance of empty identifier")
	}
}
