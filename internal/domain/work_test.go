package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func testAnalysisInput() AnalysisSpecInput {
	revision := testRevision(RevisionStateResolved)
	configuration := ConfigurationReference{ID: "profile-default", Version: "v1", Digest: "sha256-profile-v1"}
	return AnalysisSpecInput{
		SchemaVersion: AnalysisSchemaVersion, ID: "analysis-1", Run: "run-1", Repository: revision.Binding,
		Revision: revision, Configuration: configuration, Policy: testPolicy(), Trust: testTrust(), Kind: AnalysisKindGraph,
		Mode: SelectionModeHybrid, Evidence: []EvidenceReference{{ID: "candidate-evidence", Version: "v1"}},
		Executor:  ExecutorReference{Family: ExecutorFamilyBazel, ID: "bazel-analysis", Version: "v1"},
		Resources: ProfileReference{ID: "cpu-small", Version: "v1", Digest: strictDigest('4')},
		Timeout:   ProfileReference{ID: "timeout-analysis", Version: "v1", Digest: strictDigest('5')},
		Retry:     ProfileReference{ID: "retry-analysis", Version: "v1", Digest: strictDigest('6')}, Class: ExecutionClassAnalysis,
	}
}

func TestAnalysisSpecIsSeparateFromPublicJobKinds(t *testing.T) {
	analysis, err := NewAnalysisSpec(testAnalysisInput())
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	encoded, err := json.Marshal(analysis)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var decoded AnalysisSpec
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Class() != ExecutionClassAnalysis || decoded.Digest() != analysis.Digest() {
		t.Fatal("analysis round trip changed class or identity")
	}
	if _, err := NewJobSpec(JobSpecInput{SchemaVersion: JobSchemaVersion, Kind: JobKindTest, Operation: JobOperation(AnalysisKindGraph), Class: ExecutionClassAnalysis}); err == nil {
		t.Fatal("analysis was accepted as a public job")
	}
}

func TestAnalysisSpecRejectsFailedResolutionAndInvalidClass(t *testing.T) {
	input := testAnalysisInput()
	input.Revision = testRevision(RevisionStateFailed)
	if _, err := NewAnalysisSpec(input); err == nil {
		t.Fatal("failed revision became analysis work")
	}
	input = testAnalysisInput()
	input.Class = ExecutionClassValidation
	if _, err := NewAnalysisSpec(input); err == nil {
		t.Fatal("analysis accepted validation class")
	}
	input = testAnalysisInput()
	input.Evidence = nil
	if _, err := NewAnalysisSpec(input); err == nil {
		t.Fatal("analysis without explicit evidence was accepted")
	}
}

func TestWorkReferencesAreBoundedAndCompareLoadedContracts(t *testing.T) {
	planInput, _, _ := testPlanInput()
	plan, err := NewExecutionPlan(planInput)
	if err != nil {
		t.Fatal(err)
	}
	job := plan.Jobs()[1]
	jobReference, err := NewJobWorkReference("message-1", "work-1", plan, job)
	if err != nil {
		t.Fatalf("job reference: %v", err)
	}
	if err := jobReference.ValidateAgainstJob(plan, job); err != nil {
		t.Fatalf("job reference comparison: %v", err)
	}

	encoded, err := EncodeWorkReference(jobReference)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeWorkReference(encoded)
	if err != nil {
		t.Fatalf("decode job reference: %v", err)
	}
	if err := decoded.ValidateAgainstJob(plan, job); err != nil {
		t.Fatalf("decoded comparison: %v", err)
	}
	if decoded.Plan() == nil || decoded.Plan().Digest != plan.Digest() {
		t.Fatal("job reference lost plan identity")
	}

	tampered := strings.Replace(string(encoded), `"class":"validation"`, `"class":"publishing"`, 1)
	tamperedReference, err := DecodeWorkReference([]byte(tampered))
	if err != nil {
		t.Fatal(err)
	}
	if err := tamperedReference.ValidateAgainstJob(plan, job); err == nil {
		t.Fatal("tampered queue class passed loaded-contract comparison")
	}

	oversized := append([]byte(`{"schema_version":1,"padding":"`), []byte(strings.Repeat("x", maxWorkReferenceBytes))...)
	oversized = append(oversized, []byte(`"}`)...)
	if _, err := DecodeWorkReference(oversized); err == nil {
		t.Fatal("oversized work reference was parsed")
	}
	if strings.Contains(string(encoded), "command") || strings.Contains(string(encoded), "credential") {
		t.Fatal("work reference contains execution authority fields")
	}
}

func TestAnalysisWorkReferenceHasNoJobOrPlanVariant(t *testing.T) {
	analysis, err := NewAnalysisSpec(testAnalysisInput())
	if err != nil {
		t.Fatal(err)
	}
	reference, err := NewAnalysisWorkReference("message-analysis", "work-analysis", analysis)
	if err != nil {
		t.Fatal(err)
	}
	if reference.Kind() != WorkKindAnalysis || reference.Class() != ExecutionClassAnalysis {
		t.Fatal("analysis reference was classified as executable job work")
	}
	if err := reference.ValidateAgainstAnalysis(analysis); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(reference)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "plan") || strings.Contains(string(encoded), "job_id") {
		t.Fatal("analysis reference contains job variant fields")
	}
}
