// SPDX-License-Identifier: MPL-2.0
package domain

import (
	"encoding/json"
	"reflect"
	"strings"
	"unicode/utf8"
)

const (
	EvidenceSchemaVersion        uint16 = 1
	SelectionHeaderSchemaVersion uint16 = 1
	TelemetryHeaderSchemaVersion uint16 = 1
	EvidenceHeaderSchemaVersion  uint16 = 1
	maxEvidenceBytes                    = 16 * 1024
	maxEvidenceEnvelopeBytes            = 16 * 1024
	maxMediaTypeLength                  = 256
)

type StorageReference struct {
	StoreID  string `json:"store_id"`
	ObjectID string `json:"object_id"`
	Version  string `json:"version"`
}

func (r StorageReference) Validate() error {
	if err := validateID("store_id", r.StoreID); err != nil {
		return err
	}
	if err := validateStorageObject("object_id", r.ObjectID); err != nil {
		return err
	}
	return validateVersion("version", r.Version)
}

func validateStorageObject(path, value string) error {
	if value == "" {
		return invalid(path, "missing", "storage object reference is required")
	}
	if len(value) > maxRefLength || !utf8.ValidString(value) {
		return invalid(path, "invalid_ref", "storage object reference is unsupported")
	}
	if strings.HasPrefix(value, "/") || strings.Contains(value, "://") || strings.Contains(value, "?") || strings.Contains(value, "#") || strings.Contains(value, "@") || strings.ContainsRune(value, '\\') || strings.Contains(value, "..") {
		return invalid(path, "invalid_ref", "storage object reference contains an unsupported sequence")
	}
	for _, char := range value {
		if char <= 0x20 || char == 0x7f || char > 0x7f {
			return invalid(path, "invalid_ref", "storage object reference contains an unsupported character")
		}
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '.' || char == '_' || char == '-' || char == '/' || char == ':') {
			return invalid(path, "invalid_ref", "storage object reference contains an unsupported character")
		}
	}
	for _, component := range strings.Split(value, "/") {
		if component == "" || component == "." || component == ".." {
			return invalid(path, "invalid_ref", "storage object reference contains an invalid component")
		}
	}
	return nil
}

type ArtifactVerificationStatus string

const (
	ArtifactVerificationUnverified ArtifactVerificationStatus = "unverified"
	ArtifactVerificationReported   ArtifactVerificationStatus = "reported"
	ArtifactVerificationVerified   ArtifactVerificationStatus = "verified"
	ArtifactVerificationFailed     ArtifactVerificationStatus = "failed"
)

func (s ArtifactVerificationStatus) Validate() error {
	switch s {
	case ArtifactVerificationUnverified, ArtifactVerificationReported,
		ArtifactVerificationVerified, ArtifactVerificationFailed:
		return nil
	default:
		return invalid("verification", "unsupported_value", "artifact verification status is unsupported")
	}
}

type ProducedArtifactEvidence struct {
	SchemaVersion uint16                     `json:"schema_version"`
	Producer      AttemptIdentity            `json:"producer"`
	Scope         WorkScope                  `json:"scope"`
	Artifact      ArtifactReference          `json:"artifact"`
	Output        *OutputDeclaration         `json:"output,omitempty"`
	Storage       StorageReference           `json:"storage"`
	MediaType     string                     `json:"media_type"`
	Verification  ArtifactVerificationStatus `json:"verification,omitempty"`
}

func (e ProducedArtifactEvidence) Validate() error {
	if e.SchemaVersion != EvidenceSchemaVersion {
		return invalid("schema_version", "unsupported_value", "evidence schema version is unsupported")
	}
	if err := e.Producer.Validate(); err != nil {
		return prefixError("producer", err)
	}
	if err := e.Scope.Validate(); err != nil {
		return prefixError("scope", err)
	}
	if !e.Producer.Equal(scopeAttemptIdentity(e.Scope, e.Producer.Number)) {
		return invalid("producer", "mismatch", "producer identity does not match evidence scope")
	}
	if err := e.Artifact.Validate(); err != nil {
		return prefixError("artifact", err)
	}
	if e.Artifact.RevisionID != e.Scope.Revision.ID {
		return invalid("artifact.revision_id", "mismatch", "artifact revision does not match producer scope")
	}
	if !e.Artifact.Configuration.Equal(e.Scope.Configuration) {
		return invalid("artifact.configuration", "mismatch", "artifact configuration does not match producer scope")
	}
	if e.Output != nil {
		if err := e.Output.Validate(); err != nil {
			return prefixError("output", err)
		}
		if e.Output.ID != OutputID(e.Artifact.ID) || e.Output.Kind != e.Artifact.Kind {
			return invalid("output", "mismatch", "artifact output declaration does not match artifact")
		}
	}
	if err := e.Storage.Validate(); err != nil {
		return prefixError("storage", err)
	}
	if err := validateMediaType(e.MediaType); err != nil {
		return err
	}
	if e.Verification != "" {
		return prefixError("verification", e.Verification.Validate())
	}
	return nil
}

func scopeAttemptIdentity(scope WorkScope, number uint64) AttemptIdentity {
	return AttemptIdentity{Work: scope.Work, Run: scope.Run, JobID: scope.JobID, AnalysisID: scope.AnalysisID, Number: number}
}

func validateMediaType(value string) error {
	if value == "" || len(value) > maxMediaTypeLength || !utf8.ValidString(value) {
		return invalid("media_type", "invalid_value", "media type is unsupported")
	}
	for _, char := range value {
		if char <= 0x20 || char == 0x7f || char > 0x7f {
			return invalid("media_type", "invalid_value", "media type contains an unsupported character")
		}
	}
	return nil
}

func (e ProducedArtifactEvidence) ValidateAgainstAttempt(attempt Attempt) error {
	if err := e.Validate(); err != nil {
		return err
	}
	if err := attempt.Validate(); err != nil {
		return prefixError("attempt", err)
	}
	if !e.Producer.Equal(attempt.Identity()) || !reflect.DeepEqual(e.Scope, attempt.Scope()) {
		return invalid("producer", "mismatch", "artifact evidence does not belong to the supplied attempt")
	}
	return nil
}

func (e ProducedArtifactEvidence) MarshalJSON() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	type wire ProducedArtifactEvidence
	data, err := json.Marshal(wire(e))
	if err != nil {
		return nil, err
	}
	if len(data) > maxEvidenceBytes {
		return nil, invalid("evidence", "too_large", "evidence exceeds the supported size")
	}
	return data, nil
}

func (e *ProducedArtifactEvidence) UnmarshalJSON(data []byte) error {
	if len(data) > maxEvidenceBytes {
		return invalid("evidence", "too_large", "evidence exceeds the supported size")
	}
	type wire ProducedArtifactEvidence
	var value wire
	if err := decodeStrict(data, &value); err != nil {
		return err
	}
	parsed := ProducedArtifactEvidence(value)
	if err := parsed.Validate(); err != nil {
		return err
	}
	*e = parsed
	return nil
}

type ProducedReportEvidence struct {
	SchemaVersion uint16            `json:"schema_version"`
	Evidence      EvidenceReference `json:"evidence"`
	Producer      AttemptIdentity   `json:"producer"`
	Scope         WorkScope         `json:"scope"`
	ContentDigest ContentDigest     `json:"content_digest"`
	Storage       StorageReference  `json:"storage"`
	MediaType     string            `json:"media_type"`
}

func (e ProducedReportEvidence) Validate() error {
	if e.SchemaVersion != EvidenceSchemaVersion {
		return invalid("schema_version", "unsupported_value", "evidence schema version is unsupported")
	}
	if err := e.Evidence.Validate(); err != nil {
		return prefixError("evidence", err)
	}
	if err := e.Producer.Validate(); err != nil {
		return prefixError("producer", err)
	}
	if err := e.Scope.Validate(); err != nil {
		return prefixError("scope", err)
	}
	if !e.Producer.Equal(scopeAttemptIdentity(e.Scope, e.Producer.Number)) {
		return invalid("producer", "mismatch", "producer identity does not match evidence scope")
	}
	if err := validateContractDigest("content_digest", string(e.ContentDigest)); err != nil {
		return err
	}
	if err := e.Storage.Validate(); err != nil {
		return prefixError("storage", err)
	}
	return validateMediaType(e.MediaType)
}

type ConsumedArtifactReference struct {
	SchemaVersion uint16            `json:"schema_version"`
	Artifact      ArtifactReference `json:"artifact"`
	Producer      AttemptIdentity   `json:"producer"`
	Scope         WorkScope         `json:"scope"`
}

func (r ConsumedArtifactReference) Validate() error {
	if r.SchemaVersion != EvidenceSchemaVersion {
		return invalid("schema_version", "unsupported_value", "evidence schema version is unsupported")
	}
	if err := r.Artifact.Validate(); err != nil {
		return prefixError("artifact", err)
	}
	if err := r.Producer.Validate(); err != nil {
		return prefixError("producer", err)
	}
	if err := r.Scope.Validate(); err != nil {
		return prefixError("scope", err)
	}
	if !r.Producer.Equal(scopeAttemptIdentity(r.Scope, r.Producer.Number)) {
		return invalid("producer", "mismatch", "consumed artifact producer does not match scope")
	}
	if r.Artifact.RevisionID != r.Scope.Revision.ID || !r.Artifact.Configuration.Equal(r.Scope.Configuration) {
		return invalid("artifact", "mismatch", "consumed artifact is outside producer scope")
	}
	return nil
}

func (r ConsumedArtifactReference) ValidateAgainstProducer(attempt Attempt) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if err := attempt.Validate(); err != nil {
		return prefixError("attempt", err)
	}
	if !r.Producer.Equal(attempt.Identity()) || !reflect.DeepEqual(r.Scope, attempt.Scope()) {
		return invalid("producer", "mismatch", "consumed artifact is not pinned to the supplied producer")
	}
	return nil
}

type GateState string

const (
	GateStatePassed  GateState = "passed"
	GateStateFailed  GateState = "failed"
	GateStateUnknown GateState = "unknown"
)

func (s GateState) Validate() error {
	switch s {
	case GateStatePassed, GateStateFailed, GateStateUnknown:
		return nil
	default:
		return invalid("state", "unsupported_value", "gate state is unsupported")
	}
}

type GateEvidence struct {
	SchemaVersion  uint16              `json:"schema_version"`
	Gate           GateReference       `json:"gate"`
	Producer       AttemptIdentity     `json:"producer"`
	Scope          WorkScope           `json:"scope"`
	State          GateState           `json:"state"`
	Classification string              `json:"classification,omitempty"`
	Evidence       []EvidenceReference `json:"evidence,omitempty"`
}

func (g GateEvidence) Validate() error {
	if g.SchemaVersion != EvidenceSchemaVersion {
		return invalid("schema_version", "unsupported_value", "evidence schema version is unsupported")
	}
	if err := g.Gate.Validate(); err != nil {
		return prefixError("gate", err)
	}
	if err := g.Producer.Validate(); err != nil {
		return prefixError("producer", err)
	}
	if err := g.Scope.Validate(); err != nil {
		return prefixError("scope", err)
	}
	if !g.Producer.Equal(scopeAttemptIdentity(g.Scope, g.Producer.Number)) {
		return invalid("producer", "mismatch", "gate producer does not match scope")
	}
	if g.Gate.Producer != nil && (g.Producer.Work != WorkKindJob || *g.Gate.Producer != g.Producer.JobID) {
		return invalid("gate.producer", "mismatch", "gate producer does not match evidence producer")
	}
	if err := g.State.Validate(); err != nil {
		return err
	}
	if len(g.Evidence) > maxListEntries {
		return invalid("evidence", "invalid_count", "gate evidence list exceeds the supported count")
	}
	for index, evidence := range g.Evidence {
		if err := evidence.Validate(); err != nil {
			return prefixError("evidence", prefixError(indexPath(index), err))
		}
	}
	if g.State == GateStatePassed && len(g.Evidence) == 0 {
		return invalid("evidence", "missing", "passed gate requires retained evidence")
	}
	if g.State != GateStatePassed {
		if err := validateClassification("classification", g.Classification); err != nil {
			return err
		}
	}
	return nil
}

func validateClassification(path, value string) error {
	if value == "" || len(value) > maxInputValueLength || !utf8.ValidString(value) {
		return invalid(path, "invalid_value", "classification is required and bounded")
	}
	for _, char := range value {
		if char < 0x20 || char == 0x7f || char > 0x7f {
			return invalid(path, "invalid_value", "classification contains an unsupported character")
		}
	}
	return nil
}

type GateObservation struct {
	Gate           GateReference       `json:"gate"`
	State          GateState           `json:"state"`
	Producer       *AttemptIdentity    `json:"producer,omitempty"`
	Evidence       []EvidenceReference `json:"evidence,omitempty"`
	Classification string              `json:"classification,omitempty"`
}

type GateRequirement struct {
	Gate  GateReference `json:"gate"`
	Scope WorkScope     `json:"scope"`
}

func ObserveRequiredGates(required []GateReference, available []GateEvidence) ([]GateObservation, error) {
	if len(required) > maxListEntries || len(available) > maxListEntries {
		return nil, invalid("gates", "invalid_count", "gate observation exceeds the supported count")
	}
	requirements := make([]GateRequirement, len(required))
	for index, gate := range required {
		if err := gate.Validate(); err != nil {
			return nil, prefixError("required", prefixError(indexPath(index), err))
		}
		requirements[index] = GateRequirement{Gate: gate}
	}
	return observeGateRequirements(requirements, available)
}

func ObserveGateRequirements(required []GateRequirement, available []GateEvidence) ([]GateObservation, error) {
	return observeGateRequirements(required, available)
}

func observeGateRequirements(required []GateRequirement, available []GateEvidence) ([]GateObservation, error) {
	if len(required) > maxListEntries || len(available) > maxListEntries {
		return nil, invalid("gates", "invalid_count", "gate observation exceeds the supported count")
	}
	requiredSeen := make(map[string]struct{}, len(required))
	requiredByID := make(map[GateID]struct{}, len(required))
	for index, item := range required {
		if err := item.Gate.Validate(); err != nil {
			return nil, prefixError("required", prefixError(indexPath(index), err))
		}
		if err := item.Scope.Validate(); err != nil && (item.Scope.Work != "" || item.Scope.Run != "") {
			return nil, prefixError("required", prefixError(indexPath(index), err))
		}
		key := gateEvidenceKey(item.Gate)
		if _, exists := requiredSeen[key]; exists {
			return nil, invalid("required", "duplicate_value", "required gates contain duplicates")
		}
		requiredSeen[key] = struct{}{}
		requiredByID[item.Gate.ID] = struct{}{}
	}
	availableByKey := make(map[string]GateEvidence, len(available))
	for index, evidence := range available {
		if err := evidence.Validate(); err != nil {
			return nil, prefixError("available", prefixError(indexPath(index), err))
		}
		key := gateEvidenceKey(evidence.Gate)
		if _, requiredID := requiredByID[evidence.Gate.ID]; requiredID {
			if _, exact := requiredSeen[key]; !exact {
				return nil, invalid("available", "mismatch", "available gate evidence does not match the required gate version")
			}
		}
		if _, exists := availableByKey[key]; exists {
			return nil, invalid("available", "duplicate_value", "available gate evidence contains duplicates")
		}
		availableByKey[key] = evidence
	}
	observed := make([]GateObservation, len(required))
	for index, item := range required {
		observation := GateObservation{Gate: item.Gate, State: GateStateUnknown, Classification: "required evidence missing"}
		if evidence, exists := availableByKey[gateEvidenceKey(item.Gate)]; exists {
			if item.Scope.Work != "" && !reflect.DeepEqual(item.Scope, evidence.Scope) {
				return nil, invalid("available", "mismatch", "available gate evidence does not match the required scope")
			}
			observation.State = evidence.State
			observation.Evidence = cloneValue(evidence.Evidence)
			observation.Classification = evidence.Classification
			producer := evidence.Producer
			observation.Producer = &producer
		}
		observed[index] = observation
	}
	return observed, nil
}

func gateEvidenceKey(gate GateReference) string {
	return string(gate.ID) + "\x00" + gate.Version + "\x00" + string(gate.Digest)
}

type SelectionEvidenceHeader struct {
	SchemaVersion   uint16            `json:"schema_version"`
	Producer        AttemptIdentity   `json:"producer"`
	Scope           WorkScope         `json:"scope"`
	DetectorVersion string            `json:"detector_version"`
	PayloadID       EvidenceID        `json:"payload_id"`
	PayloadVersion  string            `json:"payload_version"`
	Evidence        EvidenceReference `json:"evidence"`
}

func (h SelectionEvidenceHeader) Validate() error {
	if h.SchemaVersion != SelectionHeaderSchemaVersion {
		return invalid("schema_version", "unsupported_value", "selection header schema version is unsupported")
	}
	if err := h.Producer.Validate(); err != nil {
		return prefixError("producer", err)
	}
	if h.Producer.Work != WorkKindAnalysis {
		return invalid("producer.work", "mismatch", "selection evidence requires analysis work")
	}
	if err := h.Scope.Validate(); err != nil {
		return prefixError("scope", err)
	}
	if h.Scope.Work != WorkKindAnalysis || h.Scope.Plan != nil || h.Scope.OperationKey != "" || !h.Producer.Equal(scopeAttemptIdentity(h.Scope, h.Producer.Number)) {
		return invalid("scope", "mismatch", "selection header has an invalid analysis scope")
	}
	if err := validateVersion("detector_version", h.DetectorVersion); err != nil {
		return err
	}
	if err := h.PayloadID.Validate(); err != nil {
		return prefixError("payload_id", err)
	}
	if err := validateVersion("payload_version", h.PayloadVersion); err != nil {
		return err
	}
	return prefixError("evidence", h.Evidence.Validate())
}

func (h SelectionEvidenceHeader) MarshalJSON() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}
	type wire SelectionEvidenceHeader
	data, err := json.Marshal(wire(h))
	if err != nil {
		return nil, err
	}
	if len(data) > maxEvidenceEnvelopeBytes {
		return nil, invalid("header", "too_large", "evidence header exceeds the supported size")
	}
	return data, nil
}

func (h *SelectionEvidenceHeader) UnmarshalJSON(data []byte) error {
	parsed, err := DecodeSelectionEvidenceHeader(data)
	if err != nil {
		return err
	}
	*h = parsed
	return nil
}

func EncodeSelectionEvidenceHeader(header SelectionEvidenceHeader) ([]byte, error) {
	return json.Marshal(header)
}

func DecodeSelectionEvidenceHeader(data []byte) (SelectionEvidenceHeader, error) {
	if len(data) > maxEvidenceEnvelopeBytes {
		return SelectionEvidenceHeader{}, invalid("header", "too_large", "evidence header exceeds the supported size")
	}
	type wire SelectionEvidenceHeader
	var value wire
	if err := decodeStrict(data, &value); err != nil {
		return SelectionEvidenceHeader{}, err
	}
	parsed := SelectionEvidenceHeader(value)
	if err := parsed.Validate(); err != nil {
		return SelectionEvidenceHeader{}, err
	}
	return parsed, nil
}

type TelemetryHeaderCompleteness string

const (
	TelemetryHeaderUnknown    TelemetryHeaderCompleteness = "unknown"
	TelemetryHeaderIncomplete TelemetryHeaderCompleteness = "incomplete"
	TelemetryHeaderComplete   TelemetryHeaderCompleteness = "complete"
)

func (s TelemetryHeaderCompleteness) Validate() error {
	switch s {
	case TelemetryHeaderUnknown, TelemetryHeaderIncomplete, TelemetryHeaderComplete:
		return nil
	default:
		return invalid("completion.state", "unsupported_value", "telemetry completion state is unsupported")
	}
}

type TelemetryHeaderCompletion struct {
	State  TelemetryHeaderCompleteness `json:"state"`
	Reason string                      `json:"reason,omitempty"`
}

func (c TelemetryHeaderCompletion) Validate() error {
	if err := c.State.Validate(); err != nil {
		return err
	}
	if c.State == TelemetryHeaderIncomplete {
		return validateClassification("completion.reason", c.Reason)
	}
	if c.Reason != "" {
		return validateClassification("completion.reason", c.Reason)
	}
	return nil
}

type TelemetryEvidenceHeader struct {
	SchemaVersion  uint16                     `json:"schema_version"`
	Producer       AttemptIdentity            `json:"producer"`
	Scope          WorkScope                  `json:"scope"`
	InvocationID   string                     `json:"invocation_id"`
	PayloadID      EvidenceID                 `json:"payload_id"`
	PayloadType    string                     `json:"payload_type"`
	PayloadVersion string                     `json:"payload_version"`
	Sequence       uint64                     `json:"sequence"`
	Cursor         string                     `json:"cursor,omitempty"`
	Completion     *TelemetryHeaderCompletion `json:"completion,omitempty"`
}

func (h TelemetryEvidenceHeader) Validate() error {
	if h.SchemaVersion != TelemetryHeaderSchemaVersion {
		return invalid("schema_version", "unsupported_value", "telemetry header schema version is unsupported")
	}
	if err := h.Producer.Validate(); err != nil {
		return prefixError("producer", err)
	}
	if err := h.Scope.Validate(); err != nil {
		return prefixError("scope", err)
	}
	if !h.Producer.Equal(scopeAttemptIdentity(h.Scope, h.Producer.Number)) {
		return invalid("producer", "mismatch", "telemetry producer does not match scope")
	}
	if h.Scope.Work == WorkKindJob {
		if h.Scope.Plan == nil {
			return invalid("scope.plan", "missing", "job telemetry requires plan binding")
		}
	} else if h.Scope.Work == WorkKindAnalysis && h.Scope.Plan != nil {
		return invalid("scope.plan", "inconsistent_variant", "analysis telemetry cannot contain a plan")
	}
	if err := validateID("invocation_id", h.InvocationID); err != nil {
		return err
	}
	if err := h.PayloadID.Validate(); err != nil {
		return prefixError("payload_id", err)
	}
	if err := validateVersion("payload_type", h.PayloadType); err != nil {
		return err
	}
	if err := validateVersion("payload_version", h.PayloadVersion); err != nil {
		return err
	}
	if h.Sequence == 0 {
		return invalid("sequence", "invalid_value", "telemetry sequence must be positive")
	}
	if h.Cursor != "" {
		if err := validateStorageObject("cursor", h.Cursor); err != nil {
			return err
		}
	}
	if h.Completion != nil {
		return prefixError("completion", h.Completion.Validate())
	}
	return nil
}

func (h TelemetryEvidenceHeader) MarshalJSON() ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}
	type wire TelemetryEvidenceHeader
	data, err := json.Marshal(wire(h))
	if err != nil {
		return nil, err
	}
	if len(data) > maxEvidenceEnvelopeBytes {
		return nil, invalid("header", "too_large", "evidence header exceeds the supported size")
	}
	return data, nil
}

func (h *TelemetryEvidenceHeader) UnmarshalJSON(data []byte) error {
	if len(data) > maxEvidenceEnvelopeBytes {
		return invalid("header", "too_large", "evidence header exceeds the supported size")
	}
	type wire TelemetryEvidenceHeader
	var value wire
	if err := decodeStrict(data, &value); err != nil {
		return err
	}
	parsed := TelemetryEvidenceHeader(value)
	if err := parsed.Validate(); err != nil {
		return err
	}
	*h = parsed
	return nil
}

func EncodeTelemetryEvidenceHeader(header TelemetryEvidenceHeader) ([]byte, error) {
	return json.Marshal(header)
}

func DecodeTelemetryEvidenceHeader(data []byte) (TelemetryEvidenceHeader, error) {
	if len(data) > maxEvidenceEnvelopeBytes {
		return TelemetryEvidenceHeader{}, invalid("header", "too_large", "evidence header exceeds the supported size")
	}
	type wire TelemetryEvidenceHeader
	var value wire
	if err := decodeStrict(data, &value); err != nil {
		return TelemetryEvidenceHeader{}, err
	}
	parsed := TelemetryEvidenceHeader(value)
	if err := parsed.Validate(); err != nil {
		return TelemetryEvidenceHeader{}, err
	}
	return parsed, nil
}
