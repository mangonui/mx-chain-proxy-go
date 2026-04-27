package process

import (
	"encoding/hex"
	"regexp"
	"sort"
	"strings"

	coredrwa "github.com/multiversx/mx-chain-core-go/data/drwa"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-chain-proxy-go/data"
)

// drwaCanonicalEvents is the exact set of event identifiers emitted by DRWA
// contracts and the protocol gate.  Prefix matching ("drwa*") is intentionally
// avoided: it would misclassify unrelated application events whose identifiers
// happen to start with "drwa", producing false-positive compliance signals.
var drwaCanonicalEvents = map[string]struct{}{
	"drwatokenpolicy":            {},
	"drwaassetregistered":        {},
	"drwaassetupdated":           {},
	"drwaholdercompliance":       {},
	"drwatransferdenied":         {},
	"drwatransferallowed":        {},
	"drwaglobalpause":            {},
	"drwametadataprotection":     {},
	"drwawhitepapercidset":       {},
	"drwaregistrationstatusset":  {},
	"drwaidentityregistered":     {},
	"drwacomplianceupdated":      {},
	"drwaidentitydeactivated":    {},
	"drwaidentityerased":         {},
	"drwawinddowninitiated":      {},
	"drwaauditorproposed":        {},
	"drwaauditoraccepted":        {},
	"drwaauditorrevoked":         {},
	"drwaattestationoverwritten": {},
	"drwaattestationrecorded":    {},
	"drwagovernanceproposed":     {},
	"drwagovernanceaccepted":     {},
	"drwagovernancerevoked":      {},
}

func isDRWACanonicalEvent(identifier string) bool {
	if identifier == "" {
		return false
	}
	_, ok := drwaCanonicalEvents[strings.ToLower(identifier)]
	return ok
}

var drwaDenialPattern = regexp.MustCompile(`\bDRWA_[A-Z0-9_]+\b`)

var drwaKnownFunctions = map[string]struct{}{
	"settokenpolicy":         {},
	"deactivatetokenpolicy":  {},
	"setwhitepapercid":       {},
	"setregistrationstatus":  {},
	"registerasset":          {},
	"updateasset":            {},
	"initiatewinddown":       {},
	"syncholdercompliance":   {},
	"registeridentity":       {},
	"updatecompliancestatus": {},
	"deactivateidentity":     {},
	"eraseidentity":          {},
	"setauditor":             {},
	"acceptauditor":          {},
	"revokeauditor":          {},
	"recordattestation":      {},
	"revokeattestation":      {},
	"setgovernance":          {},
	"acceptgovernance":       {},
	"revokegovernance":       {},
	"manageddrwasyncmirror":  {},
	"drwa":                   {},
}

func isDRWAKnownFunction(function string) bool {
	if function == "" {
		return false
	}
	_, ok := drwaKnownFunctions[strings.ToLower(function)]
	return ok
}

func materializeDRWADetails(
	failReason string,
	scrs map[string]*transaction.ApiSmartContractResult,
	rootFunction string,
) *data.DrwaDetails {
	return materializeDRWADetailsWithTrustedEmitters(failReason, scrs, rootFunction, nil)
}

func materializeDRWADetailsWithTrustedEmitters(
	failReason string,
	scrs map[string]*transaction.ApiSmartContractResult,
	rootFunction string,
	trustedEmitters map[string]struct{},
) *data.DrwaDetails {
	result := &data.DrwaDetails{}

	if denialIdentifier := extractDRWADenialCode(failReason); denialIdentifier != "" {
		recordProxyDRWAMetric("drwa_denial_detected")
		result.IsDrwa = true
		result.DenialCode = denialIdentifier
		result.DenialMessage = failReason
	}

	for _, key := range sortedDRWASCRKeys(scrs) {
		scr := scrs[key]
		if scr == nil {
			continue
		}
		denialIdentifier := extractDRWADenialCode(scr.ReturnMessage)
		if denialIdentifier == "" {
			continue
		}

		recordProxyDRWAMetric("drwa_denial_detected")
		result.IsDrwa = true
		if result.DenialCode == "" {
			result.DenialCode = denialIdentifier
			result.DenialMessage = scr.ReturnMessage
		}
	}

	for _, key := range sortedDRWASCRKeys(scrs) {
		scr := scrs[key]
		if scr == nil || scr.Logs == nil {
			continue
		}
		if !isDRWAKnownFunction(rootFunction) && !isDRWAKnownFunction(scr.Function) {
			continue
		}

		for _, event := range scr.Logs.Events {
			if !isDRWACanonicalEvent(event.Identifier) {
				continue
			}
			if !isTrustedDRWAEmitter(event.Address, trustedEmitters) {
				recordProxyDRWAMetric("drwa_signal_rejected")
				continue
			}

			recordProxyDRWAMetric("drwa_signal_accepted")
			result.IsDrwa = true
			result.HasComplianceSignal = true
			for _, topic := range event.Topics {
				if result.DenialCode == "" {
					topicText := string(topic)
					if denialIdentifier := extractDRWADenialCode(topicText); denialIdentifier != "" {
						recordProxyDRWAMetric("drwa_denial_detected")
						result.DenialCode = denialIdentifier
						result.DenialMessage = topicText
					}
				}
				result.DenialTopics = append(result.DenialTopics, hex.EncodeToString(topic))
			}
		}
	}

	if !result.IsDrwa && !result.HasComplianceSignal {
		return nil
	}

	return result
}

func newDRWATrustedEmitterSet(emitters ...string) map[string]struct{} {
	trustedEmitters := make(map[string]struct{}, len(emitters))
	for _, emitter := range emitters {
		if emitter == "" {
			continue
		}
		trustedEmitters[emitter] = struct{}{}
	}
	return trustedEmitters
}

func isTrustedDRWAEmitter(address string, trustedEmitters map[string]struct{}) bool {
	if address == "" || len(trustedEmitters) == 0 {
		return false
	}
	_, ok := trustedEmitters[address]
	return ok
}

func sortedDRWASCRKeys(scrs map[string]*transaction.ApiSmartContractResult) []string {
	keys := make([]string, 0, len(scrs))
	for key := range scrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func extractDRWADenialCode(message string) string {
	match := drwaDenialPattern.FindString(message)
	if match == "" {
		return ""
	}

	normalized := coredrwa.NormalizeDenialCode(match)
	if normalized != "" && normalized != coredrwa.DenialUnknown {
		return string(normalized)
	}

	// The legacy family-level code is lossy relative to the canonical sender /
	// receiver denial codes, so expose the explicit unknown sentinel instead of
	// inventing direction from text. Preserve older concrete proxy codes that
	// are already part of the materialized API contract until core-go grows a
	// shared constant for them.
	if match == "DRWA_KYC_REQUIRED" {
		return string(coredrwa.DenialUnknown)
	}
	if match == "DRWA_GLOBAL_PAUSE" {
		return match
	}
	return string(coredrwa.DenialUnknown)
}
