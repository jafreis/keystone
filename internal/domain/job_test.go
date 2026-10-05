package domain

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func strictDigest(seed byte) ContentDigest {
	return ContentDigest("sha256:" + strings.Repeat(string(seed), 64))
}

func testJobInput(operation JobOperation) JobSpecInput {
	revision := testRevision(RevisionStateResolved)
	configuration := ConfigurationReference{ID: "profile-default", Version: "v1", Digest: "sha256-profile-v1"}
	input := JobInput{BazelTest: &BazelTestInput{
		Labels: []string{"//services/api:test"}, Configuration: configuration,
	}}
	kind := JobKindTest
	executor := ExecutorReference{Family: ExecutorFamilyBazel, ID: "bazel-runner", Version: "v7"}
	class := ExecutionClassValidation
	switch operation {
	case OperationBazelBuild:
		kind, executor = JobKindBuild, ExecutorReference{Family: ExecutorFamilyBazel, ID: "bazel-runner", Version: "v7"}
		input = JobInput{BazelBuild: &BazelBuildInput{Labels: []string{"//services/api:image"}, Configuration: configuration, Outputs: []OutputDeclaration{{ID: "image", Kind: OutputKindOCI}}}}
	case OperationKubernetesValidation:
		kind, executor = JobKindTest, ExecutorReference{Family: ExecutorFamilyKubernetes, ID: "kube-validator", Version: "v2"}
		input = JobInput{KubernetesValidation: &KubernetesValidationInput{Paths: []string{"deploy/api.yaml"}, Configuration: configuration, Mode: ValidationModeClientOnly}}
	case OperationTerragruntPlan:
		kind, executor = JobKindTest, ExecutorReference{Family: ExecutorFamilyTerragrunt, ID: "terragrunt", Version: "v1"}
		input = JobInput{TerragruntPlan: &TerragruntPlanInput{Directories: []string{"infra/api"}, Configuration: configuration, Environment: EnvironmentReference{ID: "staging", Version: "v1", Digest: strictDigest('b')}, StatePolicy: PolicyReference{ID: "state-policy", Version: "v1", Digest: "sha256-state"}}}
	case OperationPublishOCI:
		kind, class, executor = JobKindPush, ExecutionClassPublishing, ExecutorReference{Family: ExecutorFamilyRegistry, ID: "registry", Version: "v1"}
		input = JobInput{PublishOCI: &PublishOCIInput{Artifact: ArtifactInput{Retained: &ArtifactReference{ID: "api-image", Digest: strictDigest('c'), Kind: OutputKindOCI, RevisionID: revision.ID, Configuration: configuration}}, Destination: DestinationReference{ID: "registry-prod", Version: "v1", Digest: strictDigest('d')}, Tags: []string{"latest", "release-1"}}}
	case OperationDeploy:
		kind, class, executor = JobKindDeploy, ExecutionClassDeployment, ExecutorReference{Family: ExecutorFamilyDeployment, ID: "operator", Version: "v3"}
		input = JobInput{Deploy: &DeployInput{Artifact: ArtifactInput{Retained: &ArtifactReference{ID: "api-image", Digest: strictDigest('c'), Kind: OutputKindOCI, RevisionID: revision.ID, Configuration: configuration}}, Destination: DestinationReference{ID: "cluster-prod", Version: "v1", Digest: strictDigest('e')}, Environment: EnvironmentReference{ID: "prod", Version: "v1", Digest: strictDigest('f')}, ConcurrencyID: "api-prod"}}
	}
	return JobSpecInput{
		SchemaVersion: JobSchemaVersion, ID: "job-1", Plan: "plan-1", Run: "run-1", Repository: revision.Binding,
		Revision: revision, Configuration: configuration, Policy: testPolicy(), Trust: testTrust(), Kind: kind,
		Operation: operation, Executor: executor, Input: input, Dependencies: []JobID{}, Gates: []GateReference{},
		Resources: ProfileReference{ID: "cpu-medium", Version: "v1", Digest: strictDigest('1')},
		Timeout:   ProfileReference{ID: "timeout-default", Version: "v1", Digest: strictDigest('2')},
		Retry:     ProfileReference{ID: "retry-default", Version: "v1", Digest: strictDigest('3')}, Class: class,
	}
}

func TestJobSpecSupportsPublicOperationMatrixAndStrictRoundTrip(t *testing.T) {
	for _, operation := range []JobOperation{OperationBazelBuild, OperationBazelTest, OperationKubernetesValidation, OperationTerragruntPlan, OperationPublishOCI, OperationDeploy} {
		t.Run(string(operation), func(t *testing.T) {
			job, err := NewJobSpec(testJobInput(operation))
			if err != nil {
				t.Fatalf("construct: %v", err)
			}
			encoded, err := json.Marshal(job)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			var decoded JobSpec
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if decoded.Digest() != job.Digest() || decoded.OperationKey() != job.OperationKey() {
				t.Fatal("round trip changed identity")
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			reordered, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			var reorderedJob JobSpec
			if err := json.Unmarshal(append([]byte(" \n"), append(reordered, []byte(" \n")...)...), &reorderedJob); err != nil {
				t.Fatal(err)
			}
			if reorderedJob.Digest() != job.Digest() || reorderedJob.OperationKey() != job.OperationKey() {
				t.Fatal("JSON ordering or whitespace changed identity")
			}
		})
	}
}

func TestJobSpecRejectsOperationInputAndClassPromotion(t *testing.T) {
	input := testJobInput(OperationBazelTest)
	input.Input.PublishOCI = &PublishOCIInput{}
	if _, err := NewJobSpec(input); err == nil {
		t.Fatal("multiple input variants were accepted")
	}

	input = testJobInput(OperationBazelTest)
	input.Class = ExecutionClassPublishing
	if _, err := NewJobSpec(input); err == nil {
		t.Fatal("validation job was promoted to publishing class")
	}

	input = testJobInput(OperationPublishOCI)
	input.Input = JobInput{BazelTest: &BazelTestInput{Labels: []string{"//api:test"}, Configuration: input.Configuration}}
	if _, err := NewJobSpec(input); err == nil {
		t.Fatal("publish operation accepted validation input")
	}

	input = testJobInput(OperationBazelTest)
	gate := GateReference{ID: "gate-tests", Version: "v1", Digest: strictDigest('7')}
	input.Gates = []GateReference{gate, gate}
	if _, err := NewJobSpec(input); err == nil {
		t.Fatal("duplicate gates were accepted")
	}
}

func TestJobSpecSnapshotsDefensivelyCopyInputsAndPreserveOperationSemantics(t *testing.T) {
	input := testJobInput(OperationBazelTest)
	job, err := NewJobSpec(input)
	if err != nil {
		t.Fatal(err)
	}
	digest, key := job.Digest(), job.OperationKey()
	input.Input.BazelTest.Labels[0] = "//changed:test"
	view := job.Input()
	view.Input.BazelTest.Labels[0] = "//changed-again:test"
	if job.Digest() != digest || job.OperationKey() != key || job.Input().Input.BazelTest.Labels[0] != "//services/api:test" {
		t.Fatal("job snapshot was mutable through input aliases")
	}

	changedID := testJobInput(OperationBazelTest)
	changedID.ID = "job-2"
	other, err := NewJobSpec(changedID)
	if err != nil {
		t.Fatal(err)
	}
	if other.OperationKey() != key || other.Digest() == digest {
		t.Fatal("job identity and logical operation identity were not separated")
	}

	changedRun := testJobInput(OperationBazelTest)
	changedRun.Run = "run-2"
	other, err = NewJobSpec(changedRun)
	if err != nil {
		t.Fatal(err)
	}
	if other.OperationKey() == key {
		t.Fatal("rerun run identity did not change operation key")
	}
	if bytes.Equal(job.CanonicalBytes(), other.CanonicalBytes()) {
		t.Fatal("run mutation did not change canonical job projection")
	}

	ordered := testJobInput(OperationBazelTest)
	ordered.Input.BazelTest.Labels = []string{"//services/api:test", "//services/web:test"}
	reordered := cloneValue(ordered)
	reordered.Input.BazelTest.Labels[0], reordered.Input.BazelTest.Labels[1] = reordered.Input.BazelTest.Labels[1], reordered.Input.BazelTest.Labels[0]
	orderedJob, err := NewJobSpec(ordered)
	if err != nil {
		t.Fatal(err)
	}
	reorderedJob, err := NewJobSpec(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if orderedJob.Digest() == reorderedJob.Digest() || orderedJob.OperationKey() == reorderedJob.OperationKey() {
		t.Fatal("semantic list reorder did not change identity")
	}
}

func TestJobSpecDecodeRejectsTamperedDigestAndLeavesReceiver(t *testing.T) {
	job, err := NewJobSpec(testJobInput(OperationBazelTest))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}
	var receiver = job
	tampered := strings.Replace(string(encoded), string(job.Digest()), string(strictDigest('a')), 1)
	if err := json.Unmarshal([]byte(tampered), &receiver); err == nil {
		t.Fatal("tampered digest was accepted")
	}
	if receiver.Digest() != job.Digest() {
		t.Fatal("failed decode partially replaced receiver")
	}
}
