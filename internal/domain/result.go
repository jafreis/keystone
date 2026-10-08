// SPDX-License-Identifier: MPL-2.0
package domain

import (
	"bytes"
	"encoding/json"
	"reflect"
	"time"
	"unicode/utf8"
)

const (
	AttemptResultSchemaVersion uint16 = 1
	AttemptResultDigestVersion        = "keystone/attempt-result/v1"
	MaxAttemptResultBytes             = 256 * 1024
	MaxDiagnosticEntries              = 16
	MaxDiagnosticTextBytes            = 4 * 1024
	MaxDiagnosticTotalBytes           = 16 * 1024
	maxAttemptResultBytes             = MaxAttemptResultBytes
)

type AttemptOutcome string

const (
	AttemptOutcomeSucceeded           AttemptOutcome = "succeeded"
	AttemptOutcomeFailed              AttemptOutcome = "failed"
	AttemptOutcomeCanceled            AttemptOutcome = "canceled"
	AttemptOutcomeTimedOut            AttemptOutcome = "timed_out"
	AttemptOutcomeLeaseLost           AttemptOutcome = "lease_lost"
	AttemptOutcomeNeedsReconciliation AttemptOutcome = "needs_reconciliation"
)

func (o AttemptOutcome) Validate() error {
	switch o {
	case AttemptOutcomeSucceeded, AttemptOutcomeFailed, AttemptOutcomeCanceled,
		AttemptOutcomeTimedOut, AttemptOutcomeLeaseLost, AttemptOutcomeNeedsReconciliation:
		return nil
	default:
		return invalid("outcome", "unsupported_value", "attempt outcome is unsupported")
	}
}

type ExecutionObservation string

const (
	ExecutionNotStarted         ExecutionObservation = "not_started"
	ExecutionCompletedNoProcess ExecutionObservation = "completed_without_process"
	ExecutionProcessExited      ExecutionObservation = "process_exited"
	ExecutionInterrupted        ExecutionObservation = "process_interrupted"
)

func (o ExecutionObservation) Validate() error {
	switch o {
	case ExecutionNotStarted, ExecutionCompletedNoProcess, ExecutionProcessExited, ExecutionInterrupted:
		return nil
	default:
		return invalid("execution", "unsupported_value", "execution observation is unsupported")
	}
}

type ResultFailureClass string

const (
	FailureClassNone           ResultFailureClass = ""
	FailureClassCommand        ResultFailureClass = "command"
	FailureClassAnalysis       ResultFailureClass = "analysis"
	FailureClassInfrastructure ResultFailureClass = "infrastructure"
	FailureClassInvalidInput   ResultFailureClass = "invalid_input"
	FailureClassEvidence       ResultFailureClass = "evidence"
	FailureClassCancellation   ResultFailureClass = "cancellation"
	FailureClassTimeout        ResultFailureClass = "timeout"
	FailureClassLease          ResultFailureClass = "lease"
)

func (c ResultFailureClass) Validate() error {
	switch c {
	case FailureClassNone, FailureClassCommand, FailureClassAnalysis, FailureClassInfrastructure,
		FailureClassInvalidInput, FailureClassEvidence, FailureClassCancellation,
		FailureClassTimeout, FailureClassLease:
		return nil
	default:
		return invalid("failure", "unsupported_value", "result failure class is unsupported")
	}
}

type TelemetryCompleteness = TelemetryHeaderCompleteness

const (
	TelemetryCompletenessUnknown    TelemetryCompleteness = TelemetryHeaderUnknown
	TelemetryCompletenessIncomplete TelemetryCompleteness = TelemetryHeaderIncomplete
	TelemetryCompletenessComplete   TelemetryCompleteness = TelemetryHeaderComplete
)

type TelemetryObservation struct {
	Completeness TelemetryCompleteness `json:"completeness"`
	InvocationID string                `json:"invocation_id,omitempty"`
	Reason       string                `json:"reason,omitempty"`
	Evidence     []EvidenceReference   `json:"evidence,omitempty"`
}

func (o TelemetryObservation) Validate() error {
	if err := o.Completeness.Validate(); err != nil {
		return prefixError("completeness", err)
	}
	if o.InvocationID != "" {
		if err := validateID("invocation_id", o.InvocationID); err != nil {
			return err
		}
	}
	if o.Completeness == TelemetryCompletenessIncomplete {
		if err := validateClassification("reason", o.Reason); err != nil {
			return err
		}
	} else if o.Reason != "" {
		if err := validateClassification("reason", o.Reason); err != nil {
			return err
		}
	}
	if len(o.Evidence) > maxListEntries {
		return invalid("evidence", "invalid_count", "telemetry evidence exceeds the supported count")
	}
	for index, evidence := range o.Evidence {
		if err := evidence.Validate(); err != nil {
			return prefixError("evidence", prefixError(indexPath(index), err))
		}
	}
	return nil
}

type DiagnosticEntry struct {
	Code      string `json:"code,omitempty"`
	Text      string `json:"text"`
	Truncated bool   `json:"truncated,omitempty"`
}

func (d DiagnosticEntry) Validate() error {
	if d.Code != "" {
		if err := validateID("code", d.Code); err != nil {
			return err
		}
	}
	if d.Text == "" || len(d.Text) > MaxDiagnosticTextBytes || !utf8.ValidString(d.Text) {
		return invalid("text", "invalid_value", "diagnostic text is missing or too large")
	}
	for _, char := range d.Text {
		if char < 0x20 && char != '\n' && char != '\t' || char == 0x7f {
			return invalid("text", "invalid_value", "diagnostic text contains an unsupported control")
		}
	}
	return nil
}

type Diagnostics struct {
	Producer        WorkerID          `json:"producer,omitempty"`
	ContractVersion string            `json:"contract_version,omitempty"`
	Redacted        bool              `json:"redacted,omitempty"`
	Entries         []DiagnosticEntry `json:"entries,omitempty"`
}

func (d Diagnostics) Validate() error {
	if len(d.Entries) == 0 {
		if d.Producer != "" || d.ContractVersion != "" || d.Redacted {
			if err := d.Producer.Validate(); err != nil {
				return prefixError("producer", err)
			}
			if d.ContractVersion != "" {
				return prefixError("contract_version", validateVersion("contract_version", d.ContractVersion))
			}
		}
		return nil
	}
	if err := d.Producer.Validate(); err != nil {
		return prefixError("producer", err)
	}
	if err := validateVersion("contract_version", d.ContractVersion); err != nil {
		return err
	}
	if !d.Redacted {
		return invalid("redacted", "required", "diagnostics require an approved redaction contract")
	}
	if len(d.Entries) > MaxDiagnosticEntries {
		return invalid("entries", "invalid_count", "diagnostic entries exceed the supported count")
	}
	total := 0
	for index, entry := range d.Entries {
		if err := entry.Validate(); err != nil {
			return prefixError("entries", prefixError(indexPath(index), err))
		}
		total += len(entry.Text)
	}
	if total > MaxDiagnosticTotalBytes {
		return invalid("entries", "too_large", "diagnostic text exceeds the supported aggregate size")
	}
	return nil
}

type AttemptResultInput struct {
	SchemaVersion uint16                      `json:"schema_version"`
	Attempt       AttemptIdentity             `json:"attempt"`
	Scope         WorkScope                   `json:"scope"`
	Worker        WorkerID                    `json:"worker"`
	Class         ExecutionClass              `json:"class"`
	Fence         FenceToken                  `json:"fence"`
	Reservation   ReservationID               `json:"reservation_id"`
	StartTime     time.Time                   `json:"start_time"`
	EndTime       time.Time                   `json:"end_time"`
	Stage         AttemptStage                `json:"stage"`
	Outcome       AttemptOutcome              `json:"outcome"`
	Execution     ExecutionObservation        `json:"execution"`
	ExitCode      *int32                      `json:"exit_code,omitempty"`
	Failure       ResultFailureClass          `json:"failure,omitempty"`
	Artifacts     []ProducedArtifactEvidence  `json:"artifacts,omitempty"`
	Reports       []ProducedReportEvidence    `json:"reports,omitempty"`
	Consumed      []ConsumedArtifactReference `json:"consumed,omitempty"`
	Gates         []GateEvidence              `json:"gates,omitempty"`
	Telemetry     *TelemetryObservation       `json:"telemetry,omitempty"`
	Diagnostics   Diagnostics                 `json:"diagnostics,omitempty"`
}

func (i AttemptResultInput) Validate() error {
	if i.SchemaVersion != AttemptResultSchemaVersion {
		return invalid("schema_version", "unsupported_value", "attempt result schema version is unsupported")
	}
	if err := i.Attempt.Validate(); err != nil {
		return prefixError("attempt", err)
	}
	if err := i.Scope.Validate(); err != nil {
		return prefixError("scope", err)
	}
	if !i.Attempt.Equal(scopeAttemptIdentity(i.Scope, i.Attempt.Number)) {
		return invalid("attempt", "mismatch", "result attempt does not match result scope")
	}
	if err := i.Worker.Validate(); err != nil {
		return prefixError("worker", err)
	}
	if err := i.Class.Validate(); err != nil {
		return prefixError("class", err)
	}
	if i.Class != i.Scope.Class {
		return invalid("class", "mismatch", "result class does not match result scope")
	}
	if err := i.Fence.Validate(); err != nil {
		return prefixError("fence", err)
	}
	if err := i.Reservation.Validate(); err != nil {
		return prefixError("reservation_id", err)
	}
	if err := validateInstant("start_time", i.StartTime); err != nil {
		return err
	}
	if err := validateInstant("end_time", i.EndTime); err != nil {
		return err
	}
	if i.EndTime.Before(i.StartTime) {
		return invalid("time", "inconsistent_value", "result end precedes start")
	}
	if i.Stage != AttemptStageFinished {
		return invalid("stage", "inconsistent_value", "attempt result requires a finished stage")
	}
	if err := i.Outcome.Validate(); err != nil {
		return err
	}
	if err := i.Execution.Validate(); err != nil {
		return err
	}
	if err := i.Failure.Validate(); err != nil {
		return err
	}
	if i.ExitCode != nil {
		// int32 is the wire representation; accepting the pointer preserves
		// the complete signed process exit value without Unix status clamping.
	}
	if err := validateExecutionShape(i); err != nil {
		return err
	}
	if len(i.Artifacts) > maxListEntries || len(i.Reports) > maxListEntries || len(i.Consumed) > maxListEntries || len(i.Gates) > maxListEntries {
		return invalid("evidence", "invalid_count", "result evidence exceeds the supported count")
	}
	for index, artifact := range i.Artifacts {
		if err := artifact.Validate(); err != nil {
			return prefixError("artifacts", prefixError(indexPath(index), err))
		}
		if err := validateResultProducer(artifact.Producer, artifact.Scope, i.Attempt, i.Scope); err != nil {
			return prefixError("artifacts", prefixError(indexPath(index), err))
		}
	}
	for index, report := range i.Reports {
		if err := report.Validate(); err != nil {
			return prefixError("reports", prefixError(indexPath(index), err))
		}
		if err := validateResultProducer(report.Producer, report.Scope, i.Attempt, i.Scope); err != nil {
			return prefixError("reports", prefixError(indexPath(index), err))
		}
	}
	for index, consumed := range i.Consumed {
		if err := consumed.Validate(); err != nil {
			return prefixError("consumed", prefixError(indexPath(index), err))
		}
	}
	seenGates := make(map[string]struct{}, len(i.Gates))
	for index, gate := range i.Gates {
		if err := gate.Validate(); err != nil {
			return prefixError("gates", prefixError(indexPath(index), err))
		}
		key := gateEvidenceKey(gate.Gate)
		if _, exists := seenGates[key]; exists {
			return invalid("gates", "duplicate_value", "result contains duplicate gate evidence")
		}
		seenGates[key] = struct{}{}
	}
	if i.Telemetry != nil {
		if err := i.Telemetry.Validate(); err != nil {
			return prefixError("telemetry", err)
		}
	}
	if err := i.Diagnostics.Validate(); err != nil {
		return prefixError("diagnostics", err)
	}
	if len(i.Diagnostics.Entries) > 0 && i.Diagnostics.Producer != i.Worker {
		return invalid("diagnostics.producer", "mismatch", "diagnostics producer does not match result worker")
	}
	return nil
}

func validateResultProducer(producer AttemptIdentity, scope WorkScope, expectedProducer AttemptIdentity, expectedScope WorkScope) error {
	if !producer.Equal(expectedProducer) || !reflect.DeepEqual(scope, expectedScope) {
		return invalid("producer", "mismatch", "result evidence belongs to another producer")
	}
	return nil
}

func validateExecutionShape(i AttemptResultInput) error {
	if i.Execution == ExecutionProcessExited {
		if i.ExitCode == nil {
			return invalid("exit_code", "missing", "exited process requires an exit code")
		}
		if *i.ExitCode != 0 && i.Outcome == AttemptOutcomeSucceeded {
			return invalid("outcome", "inconsistent_value", "successful result cannot contain a nonzero exit")
		}
		if *i.ExitCode != 0 && i.Failure == FailureClassNone {
			return invalid("failure", "missing", "nonzero exit requires a failure classification")
		}
		if *i.ExitCode == 0 && i.Outcome == AttemptOutcomeFailed && i.Failure == FailureClassNone {
			return invalid("failure", "missing", "failed result requires a failure classification")
		}
	} else {
		if i.ExitCode != nil {
			return invalid("exit_code", "inconsistent_value", "only an exited process may contain an exit code")
		}
		if i.Outcome == AttemptOutcomeSucceeded && i.Execution != ExecutionCompletedNoProcess {
			return invalid("outcome", "inconsistent_value", "successful result requires a successful execution observation")
		}
		if i.Outcome != AttemptOutcomeSucceeded && i.Failure == FailureClassNone {
			return invalid("failure", "missing", "non-success result requires a failure classification")
		}
	}
	if i.Outcome == AttemptOutcomeSucceeded && i.Failure != FailureClassNone {
		return invalid("failure", "inconsistent_value", "successful result cannot contain a failure classification")
	}
	return nil
}

type AttemptResult struct {
	input  AttemptResultInput
	digest ContentDigest
}

func NewAttemptResult(input AttemptResultInput) (AttemptResult, error) {
	if err := input.Validate(); err != nil {
		return AttemptResult{}, err
	}
	return AttemptResult{input: cloneValue(input), digest: sha256ContractDigest(attemptResultProjection(input))}, nil
}

func (r AttemptResult) Input() AttemptResultInput { return cloneValue(r.input) }
func (r AttemptResult) Digest() ContentDigest     { return r.digest }
func (r AttemptResult) CanonicalBytes() []byte {
	return append([]byte(nil), attemptResultProjection(r.input)...)
}

func (r AttemptResult) Validate() error {
	if err := r.input.Validate(); err != nil {
		return err
	}
	return verifyContractDigest("digest", r.digest, attemptResultProjection(r.input))
}

func (r AttemptResult) ValidateAgainstAttempt(attempt Attempt) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if err := attempt.Validate(); err != nil {
		return prefixError("attempt", err)
	}
	if !r.input.Attempt.Equal(attempt.Identity()) || !reflect.DeepEqual(r.input.Scope, attempt.Scope()) || r.input.Worker != attempt.Worker() || r.input.Class != attempt.Class() || r.input.Fence != attempt.Fence() || r.input.Reservation != attempt.Reservation() {
		return invalid("producer", "mismatch", "result does not belong to the supplied attempt")
	}
	return nil
}

func (r AttemptResult) ValidateActive(binding AttemptOwnershipBinding) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if err := binding.Identity.Validate(); err != nil {
		return prefixError("binding.identity", err)
	}
	if err := binding.Worker.Validate(); err != nil {
		return prefixError("binding.worker", err)
	}
	if err := binding.Class.Validate(); err != nil {
		return prefixError("binding.class", err)
	}
	if err := binding.Fence.Validate(); err != nil {
		return prefixError("binding.fence", err)
	}
	if err := binding.Reservation.Validate(); err != nil {
		return prefixError("binding.reservation", err)
	}
	if err := validateInstant("binding.now", binding.Now); err != nil {
		return err
	}
	if !r.input.Attempt.Equal(binding.Identity) || r.input.Worker != binding.Worker || r.input.Class != binding.Class || r.input.Fence != binding.Fence || r.input.Reservation != binding.Reservation {
		return invalid("ownership", "mismatch", "result active precondition does not match current ownership")
	}
	return nil
}

func (r AttemptResult) ValidateForFinalization(attempt Attempt, binding AttemptOwnershipBinding) error {
	if err := attempt.ValidateActive(binding); err != nil {
		return err
	}
	return r.ValidateAgainstAttempt(attempt)
}

type PipelineStatus string

const (
	PipelineStatusSucceeded PipelineStatus = "succeeded"
	PipelineStatusFailed    PipelineStatus = "failed"
	PipelineStatusUnknown   PipelineStatus = "unknown"
)

func (r AttemptResult) PipelineStatus(required []GateReference) (PipelineStatus, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	if r.input.Outcome != AttemptOutcomeSucceeded {
		return PipelineStatusFailed, nil
	}
	observed, err := ObserveRequiredGates(required, r.input.Gates)
	if err != nil {
		return "", err
	}
	for _, gate := range observed {
		if gate.State == GateStateFailed {
			return PipelineStatusFailed, nil
		}
		if gate.State == GateStateUnknown {
			return PipelineStatusUnknown, nil
		}
	}
	return PipelineStatusSucceeded, nil
}

func CompareAttemptResultReplay(existing, candidate AttemptResult) error {
	if err := existing.Validate(); err != nil {
		return prefixError("existing", err)
	}
	if err := candidate.Validate(); err != nil {
		return prefixError("candidate", err)
	}
	if !existing.input.Attempt.Equal(candidate.input.Attempt) {
		return invalid("attempt", "mismatch", "terminal replay uses another attempt identity")
	}
	if bytes.Equal(existing.CanonicalBytes(), candidate.CanonicalBytes()) && existing.digest == candidate.digest {
		return nil
	}
	return invalid("result", "conflict", "terminal replay content differs")
}

type attemptResultWire struct {
	SchemaVersion uint16                      `json:"schema_version"`
	Attempt       AttemptIdentity             `json:"attempt"`
	Scope         WorkScope                   `json:"scope"`
	Worker        WorkerID                    `json:"worker"`
	Class         ExecutionClass              `json:"class"`
	Fence         FenceToken                  `json:"fence"`
	Reservation   ReservationID               `json:"reservation_id"`
	StartTime     time.Time                   `json:"start_time"`
	EndTime       time.Time                   `json:"end_time"`
	Stage         AttemptStage                `json:"stage"`
	Outcome       AttemptOutcome              `json:"outcome"`
	Execution     ExecutionObservation        `json:"execution"`
	ExitCode      *int32                      `json:"exit_code,omitempty"`
	Failure       ResultFailureClass          `json:"failure,omitempty"`
	Artifacts     []ProducedArtifactEvidence  `json:"artifacts,omitempty"`
	Reports       []ProducedReportEvidence    `json:"reports,omitempty"`
	Consumed      []ConsumedArtifactReference `json:"consumed,omitempty"`
	Gates         []GateEvidence              `json:"gates,omitempty"`
	Telemetry     *TelemetryObservation       `json:"telemetry,omitempty"`
	Diagnostics   Diagnostics                 `json:"diagnostics,omitempty"`
	Digest        ContentDigest               `json:"digest"`
}

func resultWire(input AttemptResultInput, digest ContentDigest) attemptResultWire {
	return attemptResultWire{
		SchemaVersion: input.SchemaVersion, Attempt: input.Attempt, Scope: input.Scope, Worker: input.Worker,
		Class: input.Class, Fence: input.Fence, Reservation: input.Reservation, StartTime: input.StartTime,
		EndTime: input.EndTime, Stage: input.Stage, Outcome: input.Outcome, Execution: input.Execution,
		ExitCode: input.ExitCode, Failure: input.Failure, Artifacts: input.Artifacts, Reports: input.Reports,
		Consumed: input.Consumed, Gates: input.Gates, Telemetry: input.Telemetry, Diagnostics: input.Diagnostics,
		Digest: digest,
	}
}

func (r AttemptResult) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(resultWire(r.input, r.digest))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxAttemptResultBytes {
		return nil, invalid("result", "too_large", "attempt result exceeds the supported size")
	}
	return data, nil
}

func (r *AttemptResult) UnmarshalJSON(data []byte) error {
	parsed, err := DecodeAttemptResult(data)
	if err != nil {
		return err
	}
	*r = parsed
	return nil
}

func EncodeAttemptResult(result AttemptResult) ([]byte, error) { return json.Marshal(result) }

func DecodeAttemptResult(data []byte) (AttemptResult, error) {
	if len(data) > MaxAttemptResultBytes {
		return AttemptResult{}, invalid("result", "too_large", "attempt result exceeds the supported size")
	}
	var wire attemptResultWire
	if err := decodeStrict(data, &wire); err != nil {
		return AttemptResult{}, err
	}
	input := AttemptResultInput{
		SchemaVersion: wire.SchemaVersion, Attempt: wire.Attempt, Scope: wire.Scope, Worker: wire.Worker,
		Class: wire.Class, Fence: wire.Fence, Reservation: wire.Reservation, StartTime: wire.StartTime,
		EndTime: wire.EndTime, Stage: wire.Stage, Outcome: wire.Outcome, Execution: wire.Execution,
		ExitCode: wire.ExitCode, Failure: wire.Failure, Artifacts: wire.Artifacts, Reports: wire.Reports,
		Consumed: wire.Consumed, Gates: wire.Gates, Telemetry: wire.Telemetry, Diagnostics: wire.Diagnostics,
	}
	parsed, err := NewAttemptResult(input)
	if err != nil {
		return AttemptResult{}, err
	}
	if err := verifyContractDigest("digest", wire.Digest, parsed.CanonicalBytes()); err != nil {
		return AttemptResult{}, err
	}
	return parsed, nil
}

func attemptResultProjection(input AttemptResultInput) []byte {
	encoder := newCanonicalEncoder("attempt-result", AttemptResultDigestVersion)
	encodeCanonicalField(encoder, "schema_version", input.SchemaVersion)
	encodeCanonicalField(encoder, "attempt", input.Attempt)
	encodeCanonicalField(encoder, "scope", input.Scope)
	encodeCanonicalField(encoder, "worker", input.Worker)
	encodeCanonicalField(encoder, "class", input.Class)
	encodeCanonicalField(encoder, "fence", input.Fence)
	encodeCanonicalField(encoder, "reservation_id", input.Reservation)
	encodeCanonicalField(encoder, "start_time", input.StartTime)
	encodeCanonicalField(encoder, "end_time", input.EndTime)
	encodeCanonicalField(encoder, "stage", input.Stage)
	encodeCanonicalField(encoder, "outcome", input.Outcome)
	encodeCanonicalField(encoder, "execution", input.Execution)
	encodeCanonicalField(encoder, "exit_code", input.ExitCode)
	encodeCanonicalField(encoder, "failure", input.Failure)
	encodeCanonicalField(encoder, "artifacts", input.Artifacts)
	encodeCanonicalField(encoder, "reports", input.Reports)
	encodeCanonicalField(encoder, "consumed", input.Consumed)
	encodeCanonicalField(encoder, "gates", input.Gates)
	encodeCanonicalField(encoder, "telemetry", input.Telemetry)
	encodeCanonicalField(encoder, "diagnostics", input.Diagnostics)
	return encoder.Bytes()
}
