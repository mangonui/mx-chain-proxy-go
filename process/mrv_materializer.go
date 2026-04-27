package process

import (
	"encoding/binary"
	"sort"
	"strings"

	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-chain-proxy-go/data"
)

var mrvCanonicalEvents = map[string]struct{}{
	"mrvreportanchoredv2": {},
	"mrvreportamendedv2":  {},
}

var mrvKnownFunctions = map[string]struct{}{
	"anchorreportv2": {},
	"amendreportv2":  {},
}

func materializeMRVDetails(
	scrs map[string]*transaction.ApiSmartContractResult,
	rootFunction string,
) *data.MrvDetails {
	return materializeMRVDetailsWithTrustedEmitters(scrs, rootFunction, nil)
}

func materializeMRVDetailsWithTrustedEmitters(
	scrs map[string]*transaction.ApiSmartContractResult,
	rootFunction string,
	trustedEmitters map[string]struct{},
) *data.MrvDetails {
	for _, key := range sortedMRVSCRKeys(scrs) {
		scr := scrs[key]
		if scr == nil || scr.Logs == nil {
			continue
		}
		if !isMRVKnownFunction(rootFunction) && !isMRVKnownFunction(scr.Function) {
			continue
		}

		for _, event := range scr.Logs.Events {
			if event == nil || !isMRVCanonicalEvent(event.Identifier) {
				continue
			}
			if !isTrustedMRVEmitter(event.Address, trustedEmitters) {
				continue
			}
			return materializeMRVAnchoredProof(event)
		}
	}

	return nil
}

func materializeMRVAnchoredProof(event *transaction.Events) *data.MrvDetails {
	result := &data.MrvDetails{
		IsMrv:            true,
		HasAnchoredProof: true,
		SourceEvent:      event.Identifier,
		ReportID:         mrvTopicText(event, 0),
		PublicTenantID:   mrvTopicText(event, 1),
		PublicFarmID:     mrvTopicText(event, 2),
		PublicSeasonID:   mrvTopicText(event, 3),
	}
	result.ReportHash = mrvAdditionalDataText(event, 0)
	result.HashAlgo = mrvAdditionalDataText(event, 1)
	result.Canonicalization = mrvAdditionalDataText(event, 2)
	result.MethodologyVersion = mrvAdditionalDataUint64(event, 3)
	result.AnchoredAt = mrvAdditionalDataUint64(event, 4)
	result.PublicProjectID = mrvAdditionalDataText(event, 5)
	result.EvidenceManifestHash = mrvAdditionalDataText(event, 6)
	return result
}

func newMRVTrustedEmitterSet(emitters ...string) map[string]struct{} {
	trustedEmitters := make(map[string]struct{}, len(emitters))
	for _, emitter := range emitters {
		if emitter == "" {
			continue
		}
		trustedEmitters[emitter] = struct{}{}
	}
	return trustedEmitters
}

func isMRVCanonicalEvent(identifier string) bool {
	if identifier == "" {
		return false
	}
	_, ok := mrvCanonicalEvents[strings.ToLower(identifier)]
	return ok
}

func isMRVKnownFunction(function string) bool {
	if function == "" {
		return false
	}
	_, ok := mrvKnownFunctions[strings.ToLower(function)]
	return ok
}

func isTrustedMRVEmitter(address string, trustedEmitters map[string]struct{}) bool {
	if address == "" || len(trustedEmitters) == 0 {
		return false
	}
	_, ok := trustedEmitters[address]
	return ok
}

func sortedMRVSCRKeys(scrs map[string]*transaction.ApiSmartContractResult) []string {
	keys := make([]string, 0, len(scrs))
	for key := range scrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func mrvTopicText(event *transaction.Events, index int) string {
	if index >= len(event.Topics) {
		return ""
	}
	return string(event.Topics[index])
}

func mrvAdditionalDataText(event *transaction.Events, index int) string {
	if index >= len(event.AdditionalData) {
		return ""
	}
	return string(event.AdditionalData[index])
}

func mrvAdditionalDataUint64(event *transaction.Events, index int) uint64 {
	if index >= len(event.AdditionalData) {
		return 0
	}
	raw := event.AdditionalData[index]
	if len(raw) == 0 {
		return 0
	}
	if len(raw) <= 8 {
		padded := make([]byte, 8)
		copy(padded[8-len(raw):], raw)
		return binary.BigEndian.Uint64(padded)
	}
	var value uint64
	for _, b := range raw {
		if b < '0' || b > '9' {
			return 0
		}
		value = value*10 + uint64(b-'0')
	}
	return value
}
