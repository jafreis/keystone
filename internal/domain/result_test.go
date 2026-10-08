// SPDX-License-Identifier: MPL-2.0
package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func testResultInput(t *testing.T, execution ExecutionObservation, exitCode *int32, outcome AttemptOutcome, failure ResultFailureClass) (Attempt, AttemptResultInput) {
	t.Helper()
	attemptInput := testAttemptInput(t)
	attemptInput.Stage = AttemptStageFinished
	attemptInput.EndTime = timePtr(attemptInput.HeartbeatTime)
	attempt, err := NewAttempt(attemptInput)
	if err != nil {
		t.Fatal(err)
	}
	gate := testGateEvidence(t, attempt, GateStateUnknown)
	input := AttemptResultInput{
		SchemaVersion: AttemptResultSchemaVersion,
		Attempt:       attempt.Identity(),
		Scope:         attempt.Scope(),
		Worker:        attempt.Worker(),
		Class:         attempt.Class(),
		Fence:         attempt.Fence(),
		Reservation:   attempt.Reservation(),
		StartTime:     *attemptInput.StartTime,
		EndTime:       *attemptInput.EndTime,
		Stage:         AttemptStageFinished,
		Outcome:       outcome,
		Execution:     execution,
		ExitCode:      exitCode,
		Failure:       failure,
		Gates:         []GateEvidence{gate},
		Telemetry: &TelemetryObservation{
			Completeness: TelemetryCompletenessIncomplete,
			InvocationID: "invocation-1",
			Reason:       "optional stream ended early",
		},
	}
	return attempt, input
}

func int32Ptr(value int32) *int32 { return &value }

func TestAttemptResultPreservesIndependentExecutionEvidenceAndTelemetryFacts(t *testing.T) {
	attempt, input := testResultInput(t, ExecutionProcessExited, int32Ptr(17), AttemptOutcomeFailed, FailureClassCommand)
	result, err := NewAttemptResult(input)
	if err != nil {
		t.Fatalf("construct result: %v", err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("encode result: %v", err)
	}
	var decoded AttemptResult
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	decodedInput := decoded.Input()
	if decodedInput.ExitCode == nil || *decodedInput.ExitCode != 17 || decodedInput.Gates[0].State != GateStateUnknown || decodedInput.Telemetry == nil || decodedInput.Telemetry.Completeness != TelemetryCompletenessIncomplete {
		t.Fatal("result collapsed independent execution, gate, or telemetry facts")
	}
	if err := decoded.ValidateAgainstAttempt(attempt); err != nil {
		t.Fatalf("decoded result producer mismatch: %v", err)
	}

	zeroAttempt, zeroInput := testResultInput(t, ExecutionProcessExited, int32Ptr(0), AttemptOutcomeSucceeded, FailureClassNone)
	status, err := NewAttemptResult(zeroInput)
	if err != nil {
		t.Fatalf("zero result: %v", err)
	}
	pipeline, err := status.PipelineStatus([]GateReference{zeroInput.Gates[0].Gate})
	if err != nil || pipeline != PipelineStatusUnknown {
		t.Fatalf("unknown gate changed pipeline status: %v %v", pipeline, err)
	}
	if err := status.ValidateAgainstAttempt(zeroAttempt); err != nil {
		t.Fatalf("zero result producer mismatch: %v", err)
	}
}

func TestAttemptResultOutcomeMatrixRejectsContradictoryShapes(t *testing.T) {
	_, valid := testResultInput(t, ExecutionProcessExited, int32Ptr(17), AttemptOutcomeFailed, FailureClassCommand)
	cases := []struct {
		name   string
		mutate func(*AttemptResultInput)
	}{
		{name: "succeeded nonzero", mutate: func(candidate *AttemptResultInput) {
			candidate.Outcome = AttemptOutcomeSucceeded
			candidate.Failure = FailureClassNone
		}},
		{name: "exited without code", mutate: func(candidate *AttemptResultInput) { candidate.ExitCode = nil }},
		{name: "unstarted with code", mutate: func(candidate *AttemptResultInput) { candidate.Execution = ExecutionNotStarted }},
		{name: "interrupted with success", mutate: func(candidate *AttemptResultInput) {
			candidate.Execution = ExecutionInterrupted
			candidate.ExitCode = nil
			candidate.Outcome = AttemptOutcomeSucceeded
			candidate.Failure = FailureClassNone
		}},
		{name: "reversed times", mutate: func(candidate *AttemptResultInput) { candidate.EndTime = candidate.StartTime.Add(-time.Second) }},
		{name: "nonterminal stage", mutate: func(candidate *AttemptResultInput) { candidate.Stage = AttemptStageReporting }},
		{name: "zero exit evidence failure omitted", mutate: func(candidate *AttemptResultInput) {
			candidate.ExitCode = int32Ptr(0)
			candidate.Outcome = AttemptOutcomeFailed
			candidate.Failure = FailureClassNone
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneValue(valid)
			test.mutate(&candidate)
			if _, err := NewAttemptResult(candidate); err == nil {
				t.Fatal("contradictory result accepted")
			}
		})
	}

	_, interrupted := testResultInput(t, ExecutionInterrupted, nil, AttemptOutcomeCanceled, FailureClassCancellation)
	if _, err := NewAttemptResult(interrupted); err != nil {
		t.Fatalf("interrupted cancellation rejected: %v", err)
	}
}

func TestAttemptResultPreservesZeroExitWhenArtifactCollectionFails(t *testing.T) {
	_, input := testResultInput(t, ExecutionProcessExited, int32Ptr(0), AttemptOutcomeFailed, FailureClassEvidence)
	result, err := NewAttemptResult(input)
	if err != nil {
		t.Fatalf("artifact collection failure rejected: %v", err)
	}
	if result.Input().ExitCode == nil || *result.Input().ExitCode != 0 || result.Input().Failure != FailureClassEvidence {
		t.Fatal("artifact collection failure lost the observed zero exit")
	}
}

func TestAttemptResultDiagnosticsRequireRedactionContractAndStayBounded(t *testing.T) {
	_, input := testResultInput(t, ExecutionProcessExited, int32Ptr(1), AttemptOutcomeFailed, FailureClassCommand)
	input.Diagnostics = Diagnostics{
		Producer: input.Worker, ContractVersion: "redaction-v1", Redacted: true,
		Entries: []DiagnosticEntry{{Code: "command", Text: "sanitized output\nline", Truncated: false}},
	}
	if _, err := NewAttemptResult(input); err != nil {
		t.Fatalf("redacted diagnostics rejected: %v", err)
	}
	for _, mutate := range []func(*AttemptResultInput){
		func(candidate *AttemptResultInput) { candidate.Diagnostics.Redacted = false },
		func(candidate *AttemptResultInput) { candidate.Diagnostics.Producer = "other-worker" },
		func(candidate *AttemptResultInput) { candidate.Diagnostics.Entries[0].Text = "bad\x00control" },
		func(candidate *AttemptResultInput) {
			candidate.Diagnostics.Entries = make([]DiagnosticEntry, MaxDiagnosticEntries+1)
		},
	} {
		candidate := cloneValue(input)
		mutate(&candidate)
		if _, err := NewAttemptResult(candidate); err == nil {
			t.Fatal("invalid diagnostics accepted")
		}
	}
	oversized := cloneValue(input)
	oversized.Diagnostics.Entries[0].Text = strings.Repeat("x", MaxDiagnosticTextBytes+1)
	if _, err := NewAttemptResult(oversized); err == nil {
		t.Fatal("oversized diagnostic entry accepted")
	}
	exactEntry := cloneValue(input)
	exactEntry.Diagnostics.Entries[0].Text = strings.Repeat("x", MaxDiagnosticTextBytes)
	if _, err := NewAttemptResult(exactEntry); err != nil {
		t.Fatalf("diagnostic entry at its limit rejected: %v", err)
	}
	exactAggregate := cloneValue(input)
	exactAggregate.Diagnostics.Entries = make([]DiagnosticEntry, MaxDiagnosticEntries)
	for index := range exactAggregate.Diagnostics.Entries {
		exactAggregate.Diagnostics.Entries[index] = DiagnosticEntry{Code: "entry", Text: strings.Repeat("x", MaxDiagnosticTotalBytes/MaxDiagnosticEntries)}
	}
	if _, err := NewAttemptResult(exactAggregate); err != nil {
		t.Fatalf("diagnostic aggregate at its limit rejected: %v", err)
	}
	aboveAggregate := cloneValue(exactAggregate)
	aboveAggregate.Diagnostics.Entries[0].Text += "x"
	if _, err := NewAttemptResult(aboveAggregate); err == nil {
		t.Fatal("diagnostic aggregate over its limit accepted")
	}
}

func TestAttemptResultReplayIsExactAndDoesNotApplyActiveExpiry(t *testing.T) {
	attempt, input := testResultInput(t, ExecutionProcessExited, int32Ptr(17), AttemptOutcomeFailed, FailureClassCommand)
	first, err := NewAttemptResult(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewAttemptResult(cloneValue(input))
	if err != nil {
		t.Fatal(err)
	}
	if err := CompareAttemptResultReplay(first, second); err != nil {
		t.Fatalf("equal replay conflicted: %v", err)
	}
	altered := cloneValue(input)
	altered.Diagnostics = Diagnostics{Producer: input.Worker, ContractVersion: "redaction-v1", Redacted: true, Entries: []DiagnosticEntry{{Code: "changed", Text: "other"}}}
	changed, err := NewAttemptResult(altered)
	if err != nil {
		t.Fatal(err)
	}
	if err := CompareAttemptResultReplay(first, changed); err == nil {
		t.Fatal("altered replay was accepted")
	}
	if first.Digest() == changed.Digest() {
		t.Fatal("altered replay reused the accepted digest")
	}
	if err := first.ValidateAgainstAttempt(attempt); err != nil {
		t.Fatalf("accepted history changed during replay comparison: %v", err)
	}
}

func TestAttemptResultStrictDecodeLeavesReceiverUnchangedAndBoundsEncodedSize(t *testing.T) {
	_, input := testResultInput(t, ExecutionProcessExited, int32Ptr(17), AttemptOutcomeFailed, FailureClassCommand)
	result, err := NewAttemptResult(input)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	receiver := result
	unknown := strings.Replace(string(encoded), `"digest":`, `"unknown":`, 1)
	if err := json.Unmarshal([]byte(unknown), &receiver); err == nil {
		t.Fatal("unknown result field accepted")
	}
	if receiver.Digest() != result.Digest() {
		t.Fatal("failed decode mutated result receiver")
	}
	if _, err := DecodeAttemptResult([]byte(strings.Repeat("x", MaxAttemptResultBytes+1))); err == nil {
		t.Fatal("oversized result accepted")
	}
}
