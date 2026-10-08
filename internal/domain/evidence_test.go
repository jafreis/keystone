// SPDX-License-Identifier: MPL-2.0
package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func testProducedArtifact(t *testing.T, attempt Attempt) ProducedArtifactEvidence {
	t.Helper()
	scope := attempt.Scope()
	return ProducedArtifactEvidence{
		SchemaVersion: EvidenceSchemaVersion,
		Producer:      attempt.Identity(),
		Scope:         scope,
		Artifact: ArtifactReference{
			ID: "image", Digest: strictDigest('a'), Kind: OutputKindOCI,
			RevisionID: scope.Revision.ID, Configuration: scope.Configuration,
		},
		Output:    &OutputDeclaration{ID: "image", Kind: OutputKindOCI},
		Storage:   StorageReference{StoreID: "artifact-store", ObjectID: "objects/image-1", Version: "v1"},
		MediaType: "application/vnd.oci.image.manifest.v1+json",
	}
}

func testGateEvidence(t *testing.T, attempt Attempt, state GateState) GateEvidence {
	t.Helper()
	return GateEvidence{
		SchemaVersion: EvidenceSchemaVersion,
		Gate:          GateReference{ID: "gate-tests", Version: "v1", Digest: strictDigest('7')},
		Producer:      attempt.Identity(),
		Scope:         attempt.Scope(),
		State:         state,
		Classification: func() string {
			if state == GateStatePassed {
				return ""
			}
			return "required evidence unavailable"
		}(),
		Evidence: func() []EvidenceReference {
			if state == GateStatePassed {
				return []EvidenceReference{{ID: "gate-report", Version: "v1", Digest: strictDigest('r')}}
			}
			return nil
		}(),
	}
}

func TestProducedArtifactBindsDigestAndProducerScope(t *testing.T) {
	input := testAttemptInput(t)
	attempt, err := NewAttempt(input)
	if err != nil {
		t.Fatal(err)
	}
	artifact := testProducedArtifact(t, attempt)
	if err := artifact.Validate(); err != nil {
		t.Fatalf("artifact invalid: %v", err)
	}
	if err := artifact.ValidateAgainstAttempt(attempt); err != nil {
		t.Fatalf("producer comparison: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*ProducedArtifactEvidence)
	}{
		{name: "wrong attempt", mutate: func(candidate *ProducedArtifactEvidence) { candidate.Producer.Number++ }},
		{name: "wrong run", mutate: func(candidate *ProducedArtifactEvidence) { candidate.Scope.Run = "run-other" }},
		{name: "wrong repository", mutate: func(candidate *ProducedArtifactEvidence) {
			candidate.Scope.Repository.RegisteredRepositoryID = "repo-other"
		}},
		{name: "wrong configuration", mutate: func(candidate *ProducedArtifactEvidence) { candidate.Artifact.Configuration.ID = "configuration-other" }},
		{name: "wrong revision", mutate: func(candidate *ProducedArtifactEvidence) { candidate.Artifact.RevisionID = "revision-other" }},
		{name: "wrong output", mutate: func(candidate *ProducedArtifactEvidence) { candidate.Output.ID = "other-output" }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneValue(artifact)
			test.mutate(&candidate)
			if err := candidate.ValidateAgainstAttempt(attempt); err == nil {
				t.Fatal("mismatched producer evidence accepted")
			}
		})
	}
}

func TestConsumedArtifactRetainsOriginalProducerAcrossRetries(t *testing.T) {
	firstInput := testAttemptInput(t)
	first, err := NewAttempt(firstInput)
	if err != nil {
		t.Fatal(err)
	}
	secondInput := cloneValue(firstInput)
	secondInput.Identity.Number = 2
	secondInput.Worker = "worker-2"
	secondInput.Fence = "fence-2"
	second, err := NewAttempt(secondInput)
	if err != nil {
		t.Fatal(err)
	}
	produced := testProducedArtifact(t, first)
	consumed := ConsumedArtifactReference{
		SchemaVersion: EvidenceSchemaVersion,
		Artifact:      produced.Artifact,
		Producer:      first.Identity(),
		Scope:         first.Scope(),
	}
	if err := consumed.ValidateAgainstProducer(first); err != nil {
		t.Fatalf("consumed artifact rejected: %v", err)
	}
	if err := consumed.ValidateAgainstProducer(second); err == nil {
		t.Fatal("consumed artifact was relabeled to a retry")
	}
	if produced.Artifact.Digest != testProducedArtifact(t, second).Artifact.Digest {
		t.Fatal("identical retry content changed artifact digest")
	}
}

func TestGateObservationKeepsMissingRequiredEvidenceUnknown(t *testing.T) {
	input := testAttemptInput(t)
	attempt, err := NewAttempt(input)
	if err != nil {
		t.Fatal(err)
	}
	required := []GateReference{{ID: "gate-tests", Version: "v1", Digest: strictDigest('7')}}
	observed, err := ObserveRequiredGates(required, nil)
	if err != nil {
		t.Fatalf("observe missing gate: %v", err)
	}
	if len(observed) != 1 || observed[0].State != GateStateUnknown || observed[0].Producer != nil {
		t.Fatal("missing gate acquired a fabricated passing producer")
	}
	passed := testGateEvidence(t, attempt, GateStatePassed)
	if err := passed.Validate(); err != nil {
		t.Fatalf("passed gate invalid: %v", err)
	}
	observed, err = ObserveRequiredGates(required, []GateEvidence{passed})
	if err != nil || observed[0].State != GateStatePassed {
		t.Fatalf("passed gate observation: %v %#v", err, observed)
	}
	duplicate := cloneValue(passed)
	if _, err := ObserveRequiredGates(required, []GateEvidence{passed, duplicate}); err == nil {
		t.Fatal("duplicate gate evidence accepted")
	}
	wrongVersion := cloneValue(passed)
	wrongVersion.Gate.Version = "v2"
	if _, err := ObserveRequiredGates(required, []GateEvidence{wrongVersion}); err == nil {
		t.Fatal("wrong gate version accepted")
	}
	passed.Evidence = nil
	if err := passed.Validate(); err == nil {
		t.Fatal("passed gate without retained evidence accepted")
	}
}

func TestStorageReferencesRejectURLsTraversalAndControls(t *testing.T) {
	valid := StorageReference{StoreID: "artifact-store", ObjectID: "objects/image-1", Version: "v1"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid storage reference rejected: %v", err)
	}
	for _, candidate := range []StorageReference{
		{StoreID: "https://store", ObjectID: "objects/image", Version: "v1"},
		{StoreID: "store", ObjectID: "signed/object?sig=secret", Version: "v1"},
		{StoreID: "store", ObjectID: "objects/../secret", Version: "v1"},
		{StoreID: "store", ObjectID: "objects/image", Version: "v1\nsecret"},
	} {
		if err := candidate.Validate(); err == nil {
			t.Fatalf("unsafe storage reference accepted: %#v", candidate)
		}
	}
}

func TestSelectionAndTelemetryHeadersAreReferenceOnlyAndVariantStrict(t *testing.T) {
	analysis, err := NewAnalysisSpec(testAnalysisInput())
	if err != nil {
		t.Fatal(err)
	}
	analysisScope, err := NewAnalysisAttemptScope(analysis)
	if err != nil {
		t.Fatal(err)
	}
	selection := SelectionEvidenceHeader{
		SchemaVersion:   EvidenceHeaderSchemaVersion,
		Producer:        AttemptIdentity{Work: WorkKindAnalysis, Run: analysis.RunID(), AnalysisID: analysis.ID(), Number: 1},
		Scope:           analysisScope,
		DetectorVersion: "detector-v1", PayloadID: "selection-payload", PayloadVersion: "v1",
		Evidence: EvidenceReference{ID: "selection-evidence", Version: "v1"},
	}
	encoded, err := json.Marshal(selection)
	if err != nil {
		t.Fatalf("selection encode: %v", err)
	}
	var decoded SelectionEvidenceHeader
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("selection decode: %v", err)
	}
	if decoded.PayloadID != selection.PayloadID || decoded.Scope.TargetDigest != analysis.Digest() {
		t.Fatal("selection header lost identity")
	}

	_, job, jobScope := testJobAttemptScope(t)
	telemetry := TelemetryEvidenceHeader{
		SchemaVersion: EvidenceHeaderSchemaVersion,
		Producer:      AttemptIdentity{Work: WorkKindJob, Run: job.RunID(), JobID: job.ID(), Number: 1},
		Scope:         jobScope, InvocationID: "invocation-1", PayloadID: "telemetry-payload",
		PayloadType: "bep-event", PayloadVersion: "v1", Sequence: 1,
	}
	if err := telemetry.Validate(); err != nil {
		t.Fatalf("telemetry header invalid: %v", err)
	}
	telemetry.Scope.Plan = nil
	if err := telemetry.Validate(); err == nil {
		t.Fatal("job telemetry without plan binding accepted")
	}
	if _, err := DecodeSelectionEvidenceHeader([]byte(strings.Repeat("x", maxEvidenceEnvelopeBytes+1))); err == nil {
		t.Fatal("oversized selection header accepted")
	}
}
