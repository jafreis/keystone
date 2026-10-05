package domain

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func testSelection(revision RevisionContext, configuration ConfigurationReference, reason SelectionReason) SelectionProvenance {
	return SelectionProvenance{
		Mode: SelectionModeHybrid, Complete: true, Analysis: "analysis-1",
		AnalysisEvidence: EvidenceReference{ID: "analysis-evidence", Version: "v1"},
		DetectorVersion:  "detector-v1", MappingVersion: "mapping-v1", RevisionID: revision.ID,
		Configuration: configuration,
		GraphEvidence: &EvidenceReference{ID: "graph-evidence", Version: "v1"},
		PathEvidence:  &EvidenceReference{ID: "path-evidence", Version: "v1"}, Reason: reason,
	}
}

func testPlanInput() (ExecutionPlanInput, JobSpecInput, JobSpecInput) {
	build := testJobInput(OperationBazelBuild)
	build.ID = "job-build"
	push := testJobInput(OperationPublishOCI)
	push.ID = "job-push"
	push.Dependencies = []JobID{"job-build"}
	push.Input.PublishOCI.Artifact = ArtifactInput{Producer: &ProducerOutputReference{Producer: build.ID, Output: "image", Kind: OutputKindOCI, RevisionID: build.Revision.ID, Configuration: build.Configuration}}
	configuration := build.Configuration
	input := ExecutionPlanInput{
		SchemaVersion: PlanSchemaVersion, ID: "plan-1", Run: build.Run, Repository: build.Repository,
		Revision: build.Revision, Configuration: ConfigurationSnapshot{Profile: configuration, Matrix: []ConfigurationReference{configuration}},
		Policy: build.Policy, Trust: build.Trust,
		Provenance: testSelection(build.Revision, configuration, SelectionReasonWorkRequired),
		Jobs:       []JobSpecInput{push, build},
	}
	return input, build, push
}

func TestExecutionPlanAcceptsForwardDependenciesAndPreservesStoredOrder(t *testing.T) {
	input, build, push := testPlanInput()
	plan, err := NewExecutionPlan(input)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	jobs := plan.Jobs()
	if len(jobs) != 2 || jobs[0].ID() != push.ID || jobs[1].ID() != build.ID {
		t.Fatalf("job order changed: %#v", jobs)
	}
	if plan.Digest() == "" || plan.Reference().Version != "v1" {
		t.Fatal("plan identity was not computed")
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("validate snapshot: %v", err)
	}

	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeExecutionPlan(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(plan.Jobs()[0].Input(), decoded.Jobs()[0].Input()) || plan.Digest() != decoded.Digest() {
		t.Fatal("plan round trip changed order or identity")
	}
}

func TestExecutionPlanRejectsMissingCyclesConflictsAndProducerViolations(t *testing.T) {
	input, build, push := testPlanInput()
	cases := []struct {
		name   string
		mutate func(*ExecutionPlanInput)
	}{
		{name: "missing dependency", mutate: func(candidate *ExecutionPlanInput) { candidate.Jobs[0].Dependencies = []JobID{"missing"} }},
		{name: "cycle", mutate: func(candidate *ExecutionPlanInput) { candidate.Jobs[1].Dependencies = []JobID{push.ID} }},
		{name: "duplicate job id", mutate: func(candidate *ExecutionPlanInput) { candidate.Jobs[1].ID = candidate.Jobs[0].ID }},
		{name: "duplicate operation", mutate: func(candidate *ExecutionPlanInput) { candidate.Jobs[1].ID = "job-other" }},
		{name: "non-prerequisite producer", mutate: func(candidate *ExecutionPlanInput) { candidate.Jobs[0].Dependencies = nil }},
		{name: "wrong producer output", mutate: func(candidate *ExecutionPlanInput) {
			candidate.Jobs[0].Dependencies = []JobID{build.ID}
			candidate.Jobs[0].Input.PublishOCI.Artifact.Producer.Output = "missing-output"
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			candidate := input
			candidate.Jobs = append([]JobSpecInput(nil), input.Jobs...)
			candidate.Jobs[0] = cloneValue(candidate.Jobs[0])
			candidate.Jobs[1] = cloneValue(candidate.Jobs[1])
			test.mutate(&candidate)
			if test.name == "duplicate operation" {
				candidate.Jobs[1].Input = cloneValue(candidate.Jobs[0].Input)
				candidate.Jobs[1].Operation = candidate.Jobs[0].Operation
				candidate.Jobs[1].Kind = candidate.Jobs[0].Kind
				candidate.Jobs[1].Executor = candidate.Jobs[0].Executor
				candidate.Jobs[1].Class = candidate.Jobs[0].Class
			}
			if _, err := NewExecutionPlan(candidate); err == nil {
				t.Fatal("invalid graph was accepted")
			}
		})
	}
}

func TestExecutionPlanRequiresExplicitCompleteEmptyProvenance(t *testing.T) {
	input, _, _ := testPlanInput()
	input.Jobs = nil
	input.Provenance.Reason = SelectionReasonWorkRequired
	if _, err := NewExecutionPlan(input); err == nil {
		t.Fatal("empty plan without no-work provenance was accepted")
	}
	input.Provenance.Reason = SelectionReasonCompleteNoWork
	plan, err := NewExecutionPlan(input)
	if err != nil {
		t.Fatalf("complete empty plan rejected: %v", err)
	}
	if len(plan.Jobs()) != 0 {
		t.Fatal("empty plan acquired jobs")
	}

	input.Provenance.Complete = false
	if _, err := NewExecutionPlan(input); err == nil {
		t.Fatal("incomplete provenance was accepted")
	}
	input.Provenance.Complete = true
	input.Provenance.Mode = SelectionModeGraphOnly
	input.Provenance.PathEvidence = nil
	if _, err := NewExecutionPlan(input); err != nil {
		t.Fatalf("graph-only provenance should be valid: %v", err)
	}
}

func TestExecutionPlanSameIdentityConflictsAndFailedDecodePreservesReceiver(t *testing.T) {
	input, _, _ := testPlanInput()
	first, err := NewExecutionPlan(input)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := NewExecutionPlan(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := CompareExecutionPlans(first, replay); err != nil {
		t.Fatalf("identical replay conflicted: %v", err)
	}
	changed := input
	changed.Jobs = append([]JobSpecInput(nil), input.Jobs...)
	changed.Jobs[0] = cloneValue(changed.Jobs[0])
	changed.Jobs[0].Dependencies = nil
	if _, err := NewExecutionPlan(changed); err == nil {
		t.Fatal("changed plan unexpectedly remained valid")
	}

	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	var receiver = first
	tampered := strings.Replace(string(encoded), string(first.Digest()), string(strictDigest('a')), 1)
	if err := json.Unmarshal([]byte(tampered), &receiver); err == nil {
		t.Fatal("tampered plan digest was accepted")
	}
	if receiver.Digest() != first.Digest() {
		t.Fatal("failed plan decode partially replaced receiver")
	}
}

func TestExecutionPlanValidateAgainstSeparateTrustedBinding(t *testing.T) {
	input, _, _ := testPlanInput()
	plan, err := NewExecutionPlan(input)
	if err != nil {
		t.Fatal(err)
	}
	event := testPushEvent(NullCommitCandidate())
	run := testOriginalRun(event, input.Revision)
	run.ID = input.Run
	run.Approved = input.Configuration
	run.Plan = clonePointer(plan.Reference())
	if err := run.Validate(); err != nil {
		t.Fatalf("trusted fixture run: %v", err)
	}
	trusted := TrustedPlanBinding{Repository: run.Repository, Revision: run.Revision, Configuration: run.Approved, Policy: run.Policy, Trust: run.Trust}
	for _, job := range plan.Jobs() {
		trusted.ApprovedExecutors = append(trusted.ApprovedExecutors, job.Executor())
		trusted.ApprovedProfiles = append(trusted.ApprovedProfiles, job.input.Resources, job.input.Timeout, job.input.Retry)
		trusted.ApprovedDestinations = append(trusted.ApprovedDestinations, jobDestinations(job)...)
	}
	if err := plan.ValidateAgainst(run, trusted); err != nil {
		t.Fatalf("trusted plan rejected: %v", err)
	}

	tampered := trusted
	tampered.Revision = cloneValue(trusted.Revision)
	tampered.Revision.Resolved.Base = testCommit("abcdefabcdefabcdefabcdefabcdefabcdefabcd")
	tampered.Revision.Resolved.BaseEvidence = testObjectEvidence(event.Binding.ProviderRepository, tampered.Revision.Resolved.Base, "tampered-base")
	if err := plan.ValidateAgainst(run, tampered); err == nil {
		t.Fatal("same revision ID with changed base passed trusted comparison")
	}
}
