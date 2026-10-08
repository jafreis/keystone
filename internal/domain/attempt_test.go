// SPDX-License-Identifier: MPL-2.0
package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func testAttemptTimes() (time.Time, time.Time, time.Time) {
	claim := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	return claim, claim.Add(2 * time.Minute), claim.Add(10 * time.Minute)
}

func testJobAttemptScope(t *testing.T) (ExecutionPlan, JobSpec, WorkScope) {
	t.Helper()
	planInput, _, _ := testPlanInput()
	plan, err := NewExecutionPlan(planInput)
	if err != nil {
		t.Fatalf("construct plan: %v", err)
	}
	job := plan.Jobs()[1]
	scope, err := NewJobAttemptScope(plan, job)
	if err != nil {
		t.Fatalf("construct job scope: %v", err)
	}
	return plan, job, scope
}

func testAttemptInput(t *testing.T) AttemptInput {
	t.Helper()
	_, job, scope := testJobAttemptScope(t)
	claim, heartbeat, expiry := testAttemptTimes()
	return AttemptInput{
		SchemaVersion: AttemptSchemaVersion,
		Identity: AttemptIdentity{
			Work: WorkKindJob, Run: job.RunID(), JobID: job.ID(), Number: 1,
		},
		Scope:               scope,
		Worker:              WorkerID("worker-1"),
		Class:               job.Class(),
		Fence:               FenceToken("fence-1"),
		Reservation:         ReservationID("reservation-1"),
		ClaimTime:           claim,
		StartTime:           timePtr(claim.Add(time.Minute)),
		HeartbeatTime:       heartbeat,
		ExpiryTime:          expiry,
		Stage:               AttemptStageExecuting,
		RecordVersion:       1,
		CancellationVersion: 0,
	}
}

func timePtr(value time.Time) *time.Time { return &value }

func testReservationInput(t *testing.T, attempt *AttemptIdentity) CapacityReservationInput {
	t.Helper()
	_, job, scope := testJobAttemptScope(t)
	claim, heartbeat, expiry := testAttemptTimes()
	return CapacityReservationInput{
		SchemaVersion:       CapacityReservationSchemaVersion,
		ID:                  ReservationID("reservation-1"),
		Repository:          scope.Repository,
		Class:               job.Class(),
		Resource:            job.Input().Resources,
		Controller:          WorkerID("controller-1"),
		RecordVersion:       1,
		CancellationVersion: 0,
		CreatedAt:           claim,
		HeartbeatAt:         heartbeat,
		ExpiresAt:           expiry,
		State:               CapacityReservationReserved,
		Attempt:             attempt,
	}
}

func TestAttemptScopeBindsAuthoritativeJobAndAnalysisContracts(t *testing.T) {
	plan, job, scope := testJobAttemptScope(t)
	if err := scope.Validate(); err != nil {
		t.Fatalf("job scope invalid: %v", err)
	}
	if scope.OperationKey != job.OperationKey() || scope.TargetDigest != job.Digest() {
		t.Fatal("job scope lost immutable job identities")
	}
	if scope.Plan == nil || scope.Plan.Digest != plan.Digest() {
		t.Fatal("job scope lost authoritative plan reference")
	}

	analysisInput := testAnalysisInput()
	analysis, err := NewAnalysisSpec(analysisInput)
	if err != nil {
		t.Fatalf("construct analysis: %v", err)
	}
	analysisScope, err := NewAnalysisAttemptScope(analysis)
	if err != nil {
		t.Fatalf("construct analysis scope: %v", err)
	}
	if err := analysisScope.Validate(); err != nil {
		t.Fatalf("analysis scope invalid: %v", err)
	}
	if analysisScope.Plan != nil || analysisScope.OperationKey != "" || analysisScope.AnalysisID != analysis.ID() {
		t.Fatal("analysis scope acquired job-only authority")
	}
	pendingInput := testAnalysisInput()
	pendingInput.Kind = AnalysisKindRevisionResolution
	pendingInput.Revision = testRevision(RevisionStatePending)
	pending, err := NewAnalysisSpec(pendingInput)
	if err != nil {
		t.Fatalf("construct pending analysis: %v", err)
	}
	pendingScope, err := NewAnalysisAttemptScope(pending)
	if err != nil {
		t.Fatalf("construct pending analysis scope: %v", err)
	}
	if pendingScope.Revision.State != RevisionStatePending {
		t.Fatal("pending analysis scope was rewritten")
	}
}

func TestAttemptRejectsIdentityAndLeaseContradictions(t *testing.T) {
	input := testAttemptInput(t)
	valid, err := NewAttempt(input)
	if err != nil {
		t.Fatalf("construct attempt: %v", err)
	}
	if valid.Identity().Number != 1 || valid.Scope().TargetDigest == "" {
		t.Fatal("attempt did not retain its identity")
	}
	wrongScope := input.Scope
	wrongScope.TargetDigest = strictDigest('b')
	if err := valid.ValidateAgainstScope(wrongScope); err == nil {
		t.Fatal("attempt accepted a mismatched target digest")
	}

	cases := []struct {
		name   string
		mutate func(*AttemptInput)
	}{
		{name: "zero ordinal", mutate: func(candidate *AttemptInput) { candidate.Identity.Number = 0 }},
		{name: "missing fence", mutate: func(candidate *AttemptInput) { candidate.Fence = "" }},
		{name: "heartbeat at expiry", mutate: func(candidate *AttemptInput) { candidate.HeartbeatTime = candidate.ExpiryTime }},
		{name: "start before claim", mutate: func(candidate *AttemptInput) { candidate.StartTime = timePtr(candidate.ClaimTime.Add(-time.Second)) }},
		{name: "wrong scope", mutate: func(candidate *AttemptInput) { candidate.Scope.Run = "other-run" }},
		{name: "finished without end", mutate: func(candidate *AttemptInput) { candidate.Stage = AttemptStageFinished }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneValue(input)
			test.mutate(&candidate)
			if _, err := NewAttempt(candidate); err == nil {
				t.Fatalf("invalid attempt accepted: %#v", test.name)
			}
		})
	}

	claim, heartbeat, expiry := testAttemptTimes()
	if _, err := NewAttempt(AttemptInput{
		SchemaVersion: AttemptSchemaVersion,
		Identity:      AttemptIdentity{Work: WorkKindJob, Run: input.Identity.Run, JobID: input.Identity.JobID, Number: 1},
		Scope:         input.Scope, Worker: input.Worker, Class: input.Class, Fence: input.Fence,
		Reservation: input.Reservation, ClaimTime: claim, HeartbeatTime: heartbeat,
		ExpiryTime: expiry, Stage: AttemptStageFinished, StartTime: timePtr(claim), EndTime: timePtr(heartbeat), RecordVersion: 1,
	}); err != nil {
		t.Fatalf("finished attempt should validate: %v", err)
	}
}

func TestAttemptNumberHelpersRejectRegressionAndOverflow(t *testing.T) {
	if next, err := NextAttemptNumber(1); err != nil || next != 2 {
		t.Fatalf("next attempt: %d, %v", next, err)
	}
	if _, err := NextAttemptNumber(0); err == nil {
		t.Fatal("zero previous ordinal accepted")
	}
	if _, err := NextAttemptNumber(^uint64(0)); err == nil {
		t.Fatal("overflowing ordinal accepted")
	}
	if err := ValidateAttemptSequence(2, 1); err == nil {
		t.Fatal("regressing ordinal accepted")
	}
	if err := ValidateAttemptSequence(1, 2); err != nil {
		t.Fatalf("monotonic ordinal rejected: %v", err)
	}
}

func TestAttemptActivePreconditionRequiresCurrentFenceAndUnexpiredTime(t *testing.T) {
	input := testAttemptInput(t)
	attempt, err := NewAttempt(input)
	if err != nil {
		t.Fatal(err)
	}
	_, heartbeat, expiry := testAttemptTimes()
	binding := AttemptOwnershipBinding{
		Identity: input.Identity, Scope: func() *WorkScope { scope := input.Scope; return &scope }(), Worker: input.Worker, Class: input.Class,
		Fence: input.Fence, Reservation: input.Reservation,
		RecordVersion: input.RecordVersion, CancellationVersion: input.CancellationVersion,
		Now: heartbeat,
	}
	if err := attempt.ValidateActive(binding); err != nil {
		t.Fatalf("current ownership rejected: %v", err)
	}
	binding.Now = expiry
	if err := attempt.ValidateActive(binding); err == nil {
		t.Fatal("ownership at expiry accepted")
	}
	binding.Now = heartbeat
	binding.Fence = "stale-fence"
	if err := attempt.ValidateActive(binding); err == nil {
		t.Fatal("stale fence accepted")
	}
}

func TestAttemptScopeComparisonUsesCanonicalRepresentation(t *testing.T) {
	input := testAttemptInput(t)
	input.Scope.Outputs = []OutputDeclaration{}
	attempt, err := NewAttempt(input)
	if err != nil {
		t.Fatalf("construct attempt: %v", err)
	}
	if err := attempt.ValidateAgainstScope(input.Scope); err != nil {
		t.Fatalf("equivalent empty and nil output representations differ: %v", err)
	}
}

func TestCapacityReservationPreservesPreClaimAndCorrelatesActiveState(t *testing.T) {
	input := testReservationInput(t, nil)
	reservation, err := NewCapacityReservation(input)
	if err != nil {
		t.Fatalf("construct pre-claim reservation: %v", err)
	}
	if reservation.State() != CapacityReservationReserved {
		t.Fatal("reserved capacity lost its state")
	}

	attemptInput := testAttemptInput(t)
	attempt, err := NewAttempt(attemptInput)
	if err != nil {
		t.Fatal(err)
	}
	active := cloneValue(input)
	active.State = CapacityReservationActive
	active.Attempt = &attemptInput.Identity
	active.Worker = timeWorker(attemptInput.Worker)
	activeReservation, err := NewCapacityReservation(active)
	if err != nil {
		t.Fatalf("construct active reservation: %v", err)
	}
	if err := activeReservation.ValidateAgainstAttempt(attempt); err != nil {
		t.Fatalf("active reservation mismatch: %v", err)
	}
	active.Attempt.Number++
	wrongReservation, err := NewCapacityReservation(active)
	if err != nil {
		t.Fatalf("construct structurally valid reservation: %v", err)
	}
	if err := wrongReservation.ValidateAgainstAttempt(attempt); err == nil {
		t.Fatal("invalid active attempt identity accepted")
	}
}

func timeWorker(value WorkerID) *WorkerID { return &value }

func TestAttemptStrictRoundTripAndDefensiveSnapshot(t *testing.T) {
	input := testAttemptInput(t)
	attempt, err := NewAttempt(input)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(attempt)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var decoded Attempt
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Identity() != attempt.Identity() || decoded.Scope().TargetDigest != attempt.Scope().TargetDigest {
		t.Fatal("attempt round trip changed identity")
	}
	view := decoded.Input()
	view.Scope.Repository.RegisteredRepositoryID = "tampered"
	if decoded.Scope().Repository.RegisteredRepositoryID == "tampered" {
		t.Fatal("attempt accessor exposed mutable state")
	}
	if _, err := DecodeAttempt(append(encoded, []byte("{}")...)); err == nil {
		t.Fatal("trailing JSON accepted")
	}
	if _, err := DecodeAttempt([]byte(strings.Repeat("x", maxAttemptBytes+1))); err == nil {
		t.Fatal("oversized attempt accepted")
	}
}
