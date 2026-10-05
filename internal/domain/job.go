package domain

import (
	"encoding/json"
	"fmt"
)

type JobKind string

const (
	JobKindBuild  JobKind = "build"
	JobKindTest   JobKind = "test"
	JobKindPush   JobKind = "push"
	JobKindDeploy JobKind = "deploy"
)

func (k JobKind) Validate() error {
	switch k {
	case JobKindBuild, JobKindTest, JobKindPush, JobKindDeploy:
		return nil
	default:
		return invalid("kind", "unsupported_value", "job kind is unsupported")
	}
}

type JobOperation string

const (
	OperationBazelBuild           JobOperation = "bazel_build"
	OperationBazelTest            JobOperation = "bazel_test"
	OperationKubernetesValidation JobOperation = "kubernetes_validation"
	OperationTerragruntPlan       JobOperation = "terragrunt_plan"
	OperationPublishOCI           JobOperation = "publish_oci"
	OperationDeploy               JobOperation = "deploy"
)

type ExecutionClass string

const (
	ExecutionClassValidation ExecutionClass = "validation"
	ExecutionClassPublishing ExecutionClass = "publishing"
	ExecutionClassDeployment ExecutionClass = "deployment"
	ExecutionClassAnalysis   ExecutionClass = "analysis"
)

type ExecutorFamily string

const (
	ExecutorFamilyBazel      ExecutorFamily = "bazel"
	ExecutorFamilyKubernetes ExecutorFamily = "kubernetes"
	ExecutorFamilyTerragrunt ExecutorFamily = "terragrunt"
	ExecutorFamilyRegistry   ExecutorFamily = "registry"
	ExecutorFamilyDeployment ExecutorFamily = "deployment"
)

func (f ExecutorFamily) Validate() error {
	switch f {
	case ExecutorFamilyBazel, ExecutorFamilyKubernetes, ExecutorFamilyTerragrunt,
		ExecutorFamilyRegistry, ExecutorFamilyDeployment:
		return nil
	default:
		return invalid("family", "unsupported_value", "executor family is unsupported")
	}
}

type ExecutorReference struct {
	Family  ExecutorFamily `json:"family"`
	ID      string         `json:"id"`
	Version string         `json:"version"`
}

func (e ExecutorReference) Validate() error {
	if err := e.Family.Validate(); err != nil {
		return prefixError("family", err)
	}
	if err := validateID("id", e.ID); err != nil {
		return prefixError("id", err)
	}
	return validateVersion("version", e.Version)
}

type ProfileReference struct {
	ID      ProfileID     `json:"id"`
	Version string        `json:"version"`
	Digest  ContentDigest `json:"digest"`
}

func (p ProfileReference) Validate() error {
	if err := p.ID.Validate(); err != nil {
		return prefixError("id", err)
	}
	if err := validateVersion("version", p.Version); err != nil {
		return err
	}
	return prefixError("digest", validateContractDigest("digest", string(p.Digest)))
}

type ResourceProfileReference = ProfileReference
type TimeoutProfileReference = ProfileReference
type RetryProfileReference = ProfileReference

type OutputKind string

const (
	OutputKindOCI        OutputKind = "oci_image"
	OutputKindTestReport OutputKind = "test_report"
	OutputKindPlan       OutputKind = "infrastructure_plan"
)

func (k OutputKind) Validate() error {
	switch k {
	case OutputKindOCI, OutputKindTestReport, OutputKindPlan:
		return nil
	default:
		return invalid("kind", "unsupported_value", "output kind is unsupported")
	}
}

type OutputDeclaration struct {
	ID   OutputID   `json:"id"`
	Kind OutputKind `json:"kind"`
}

func (o OutputDeclaration) Validate() error {
	if err := o.ID.Validate(); err != nil {
		return prefixError("id", err)
	}
	return prefixError("kind", o.Kind.Validate())
}

type DestinationReference struct {
	ID      DestinationID `json:"id"`
	Version string        `json:"version"`
	Digest  ContentDigest `json:"digest"`
}

func (d DestinationReference) Validate() error {
	if err := d.ID.Validate(); err != nil {
		return prefixError("id", err)
	}
	if err := validateVersion("version", d.Version); err != nil {
		return err
	}
	return prefixError("digest", validateContractDigest("digest", string(d.Digest)))
}

type EnvironmentReference struct {
	ID      string        `json:"id"`
	Version string        `json:"version"`
	Digest  ContentDigest `json:"digest"`
}

func (e EnvironmentReference) Validate() error {
	if err := validateID("id", e.ID); err != nil {
		return prefixError("id", err)
	}
	if err := validateVersion("version", e.Version); err != nil {
		return err
	}
	return prefixError("digest", validateContractDigest("digest", string(e.Digest)))
}

type ArtifactReference struct {
	ID            string                 `json:"id"`
	Digest        ContentDigest          `json:"digest"`
	Kind          OutputKind             `json:"kind"`
	RevisionID    RevisionID             `json:"revision_id"`
	Configuration ConfigurationReference `json:"configuration"`
}

func (a ArtifactReference) Validate() error {
	if err := validateID("id", a.ID); err != nil {
		return prefixError("id", err)
	}
	if err := validateContractDigest("digest", string(a.Digest)); err != nil {
		return err
	}
	if err := a.Kind.Validate(); err != nil {
		return prefixError("kind", err)
	}
	if err := a.RevisionID.Validate(); err != nil {
		return prefixError("revision_id", err)
	}
	return prefixError("configuration", a.Configuration.ValidateWithDigest())
}

type ProducerOutputReference struct {
	Producer      JobID                  `json:"producer"`
	Output        OutputID               `json:"output"`
	Kind          OutputKind             `json:"kind"`
	RevisionID    RevisionID             `json:"revision_id"`
	Configuration ConfigurationReference `json:"configuration"`
}

func (p ProducerOutputReference) Validate() error {
	if err := p.Producer.Validate(); err != nil {
		return prefixError("producer", err)
	}
	if err := p.Output.Validate(); err != nil {
		return prefixError("output", err)
	}
	if err := p.Kind.Validate(); err != nil {
		return prefixError("kind", err)
	}
	if err := p.RevisionID.Validate(); err != nil {
		return prefixError("revision_id", err)
	}
	return prefixError("configuration", p.Configuration.ValidateWithDigest())
}

type ArtifactInput struct {
	Retained *ArtifactReference       `json:"retained,omitempty"`
	Producer *ProducerOutputReference `json:"producer,omitempty"`
}

func (a ArtifactInput) Validate() error {
	if (a.Retained == nil) == (a.Producer == nil) {
		return invalid("artifact", "inconsistent_variant", "artifact input must contain exactly one variant")
	}
	if a.Retained != nil {
		return prefixError("retained", a.Retained.Validate())
	}
	return prefixError("producer", a.Producer.Validate())
}

type GateReference struct {
	ID       GateID             `json:"id"`
	Version  string             `json:"version"`
	Digest   ContentDigest      `json:"digest"`
	Producer *JobID             `json:"producer,omitempty"`
	Evidence *EvidenceReference `json:"evidence,omitempty"`
}

func (g GateReference) Validate() error {
	if err := g.ID.Validate(); err != nil {
		return prefixError("id", err)
	}
	if err := validateVersion("version", g.Version); err != nil {
		return err
	}
	if err := validateContractDigest("digest", string(g.Digest)); err != nil {
		return err
	}
	if g.Producer != nil {
		if err := g.Producer.Validate(); err != nil {
			return prefixError("producer", err)
		}
	}
	if g.Evidence != nil {
		if err := g.Evidence.Validate(); err != nil {
			return prefixError("evidence", err)
		}
	}
	return nil
}

type BazelBuildInput struct {
	Labels        []string               `json:"labels"`
	Configuration ConfigurationReference `json:"configuration"`
	Outputs       []OutputDeclaration    `json:"outputs"`
}

func (b BazelBuildInput) Validate() error {
	if err := validateLabels("labels", b.Labels); err != nil {
		return err
	}
	if err := b.Configuration.ValidateWithDigest(); err != nil {
		return prefixError("configuration", err)
	}
	if len(b.Outputs) == 0 || len(b.Outputs) > maxListEntries {
		return invalid("outputs", "invalid_count", "build input must declare bounded outputs")
	}
	seen := make(map[OutputID]struct{}, len(b.Outputs))
	for index, output := range b.Outputs {
		if err := output.Validate(); err != nil {
			return prefixError("outputs", prefixError(indexPath(index), err))
		}
		if _, exists := seen[output.ID]; exists {
			return invalid("outputs", "duplicate_value", "build input contains duplicate outputs")
		}
		seen[output.ID] = struct{}{}
	}
	return nil
}

type BazelTestInput struct {
	Labels        []string               `json:"labels"`
	Configuration ConfigurationReference `json:"configuration"`
}

func (b BazelTestInput) Validate() error {
	if err := validateLabels("labels", b.Labels); err != nil {
		return err
	}
	return prefixError("configuration", b.Configuration.ValidateWithDigest())
}

type ValidationMode string

const (
	ValidationModeClientServer ValidationMode = "client_server"
	ValidationModeClientOnly   ValidationMode = "client_only"
	ValidationModeServerOnly   ValidationMode = "server_only"
)

func (m ValidationMode) Validate() error {
	switch m {
	case ValidationModeClientServer, ValidationModeClientOnly, ValidationModeServerOnly:
		return nil
	default:
		return invalid("mode", "unsupported_value", "validation mode is unsupported")
	}
}

type KubernetesValidationInput struct {
	Paths         []string               `json:"paths"`
	Configuration ConfigurationReference `json:"configuration"`
	Mode          ValidationMode         `json:"mode"`
	Destination   *DestinationReference  `json:"destination,omitempty"`
}

func (k KubernetesValidationInput) Validate() error {
	if err := validatePaths("paths", k.Paths); err != nil {
		return err
	}
	if err := k.Configuration.ValidateWithDigest(); err != nil {
		return prefixError("configuration", err)
	}
	if err := k.Mode.Validate(); err != nil {
		return err
	}
	if k.Mode == ValidationModeServerOnly || k.Mode == ValidationModeClientServer {
		if k.Destination == nil {
			return invalid("destination", "missing", "server validation requires a destination")
		}
	}
	if k.Destination != nil {
		return prefixError("destination", k.Destination.Validate())
	}
	return nil
}

type TerragruntPlanInput struct {
	Directories   []string               `json:"directories"`
	Configuration ConfigurationReference `json:"configuration"`
	Environment   EnvironmentReference   `json:"environment"`
	StatePolicy   PolicyReference        `json:"state_policy"`
}

func (t TerragruntPlanInput) Validate() error {
	if err := validatePaths("directories", t.Directories); err != nil {
		return err
	}
	if err := t.Configuration.ValidateWithDigest(); err != nil {
		return prefixError("configuration", err)
	}
	if err := t.Environment.Validate(); err != nil {
		return prefixError("environment", err)
	}
	return prefixError("state_policy", t.StatePolicy.Validate())
}

type PublishOCIInput struct {
	Artifact    ArtifactInput        `json:"artifact"`
	Destination DestinationReference `json:"destination"`
	Tags        []string             `json:"tags"`
}

func (p PublishOCIInput) Validate() error {
	if err := p.Artifact.Validate(); err != nil {
		return prefixError("artifact", err)
	}
	if err := p.Destination.Validate(); err != nil {
		return prefixError("destination", err)
	}
	if len(p.Tags) == 0 || len(p.Tags) > maxListEntries {
		return invalid("tags", "invalid_count", "publish input must contain bounded tag aliases")
	}
	for index, tag := range p.Tags {
		if err := validateID(indexPath(index), tag); err != nil {
			return prefixError("tags", err)
		}
	}
	return nil
}

type DeployInput struct {
	Artifact      ArtifactInput        `json:"artifact"`
	Destination   DestinationReference `json:"destination"`
	Environment   EnvironmentReference `json:"environment"`
	ConcurrencyID string               `json:"concurrency_id"`
}

func (d DeployInput) Validate() error {
	if err := d.Artifact.Validate(); err != nil {
		return prefixError("artifact", err)
	}
	if err := d.Destination.Validate(); err != nil {
		return prefixError("destination", err)
	}
	if err := d.Environment.Validate(); err != nil {
		return prefixError("environment", err)
	}
	return validateID("concurrency_id", d.ConcurrencyID)
}

type JobInput struct {
	BazelBuild           *BazelBuildInput           `json:"bazel_build,omitempty"`
	BazelTest            *BazelTestInput            `json:"bazel_test,omitempty"`
	KubernetesValidation *KubernetesValidationInput `json:"kubernetes_validation,omitempty"`
	TerragruntPlan       *TerragruntPlanInput       `json:"terragrunt_plan,omitempty"`
	PublishOCI           *PublishOCIInput           `json:"publish_oci,omitempty"`
	Deploy               *DeployInput               `json:"deploy,omitempty"`
}

func (i JobInput) variantCount() int {
	count := 0
	if i.BazelBuild != nil {
		count++
	}
	if i.BazelTest != nil {
		count++
	}
	if i.KubernetesValidation != nil {
		count++
	}
	if i.TerragruntPlan != nil {
		count++
	}
	if i.PublishOCI != nil {
		count++
	}
	if i.Deploy != nil {
		count++
	}
	return count
}

func (i JobInput) ValidateFor(operation JobOperation) error {
	if i.variantCount() != 1 {
		return invalid("input", "inconsistent_variant", "job input must contain exactly one typed variant")
	}
	switch operation {
	case OperationBazelBuild:
		if i.BazelBuild == nil {
			return invalid("input", "operation_mismatch", "operation requires bazel build input")
		}
		return prefixError("input.bazel_build", i.BazelBuild.Validate())
	case OperationBazelTest:
		if i.BazelTest == nil {
			return invalid("input", "operation_mismatch", "operation requires bazel test input")
		}
		return prefixError("input.bazel_test", i.BazelTest.Validate())
	case OperationKubernetesValidation:
		if i.KubernetesValidation == nil {
			return invalid("input", "operation_mismatch", "operation requires Kubernetes validation input")
		}
		return prefixError("input.kubernetes_validation", i.KubernetesValidation.Validate())
	case OperationTerragruntPlan:
		if i.TerragruntPlan == nil {
			return invalid("input", "operation_mismatch", "operation requires Terragrunt input")
		}
		return prefixError("input.terragrunt_plan", i.TerragruntPlan.Validate())
	case OperationPublishOCI:
		if i.PublishOCI == nil {
			return invalid("input", "operation_mismatch", "operation requires publish input")
		}
		return prefixError("input.publish_oci", i.PublishOCI.Validate())
	case OperationDeploy:
		if i.Deploy == nil {
			return invalid("input", "operation_mismatch", "operation requires deploy input")
		}
		return prefixError("input.deploy", i.Deploy.Validate())
	default:
		return invalid("operation", "unsupported_value", "job operation is unsupported")
	}
}

type JobSpecInput struct {
	SchemaVersion uint16                   `json:"schema_version"`
	ID            JobID                    `json:"job_id"`
	Plan          PlanID                   `json:"plan_id"`
	Run           RunID                    `json:"run_id"`
	Repository    RepositoryBinding        `json:"repository"`
	Revision      RevisionContext          `json:"revision"`
	Configuration ConfigurationReference   `json:"configuration"`
	Policy        PolicyReference          `json:"policy"`
	Trust         TrustReference           `json:"trust"`
	Kind          JobKind                  `json:"kind"`
	Operation     JobOperation             `json:"operation"`
	Executor      ExecutorReference        `json:"executor"`
	Input         JobInput                 `json:"input"`
	Dependencies  []JobID                  `json:"dependencies"`
	Gates         []GateReference          `json:"gates"`
	Resources     ResourceProfileReference `json:"resources"`
	Timeout       TimeoutProfileReference  `json:"timeout"`
	Retry         RetryProfileReference    `json:"retry"`
	Class         ExecutionClass           `json:"class"`
}

func operationContract(operation JobOperation) (JobKind, ExecutorFamily, ExecutionClass, bool) {
	switch operation {
	case OperationBazelBuild:
		return JobKindBuild, ExecutorFamilyBazel, ExecutionClassValidation, true
	case OperationBazelTest:
		return JobKindTest, ExecutorFamilyBazel, ExecutionClassValidation, true
	case OperationKubernetesValidation:
		return JobKindTest, ExecutorFamilyKubernetes, ExecutionClassValidation, true
	case OperationTerragruntPlan:
		return JobKindTest, ExecutorFamilyTerragrunt, ExecutionClassValidation, true
	case OperationPublishOCI:
		return JobKindPush, ExecutorFamilyRegistry, ExecutionClassPublishing, true
	case OperationDeploy:
		return JobKindDeploy, ExecutorFamilyDeployment, ExecutionClassDeployment, true
	default:
		return "", "", "", false
	}
}

func (i JobSpecInput) Validate() error {
	if i.SchemaVersion != JobSchemaVersion {
		return invalid("schema_version", "unsupported_value", "job schema version is unsupported")
	}
	if err := i.ID.Validate(); err != nil {
		return prefixError("job_id", err)
	}
	if err := i.Plan.Validate(); err != nil {
		return prefixError("plan_id", err)
	}
	if err := i.Run.Validate(); err != nil {
		return prefixError("run_id", err)
	}
	if err := i.Repository.Validate(); err != nil {
		return prefixError("repository", err)
	}
	if err := i.Revision.Validate(); err != nil {
		return prefixError("revision", err)
	}
	if i.Revision.State != RevisionStateResolved {
		return invalid("revision.state", "not_ready", "executable jobs require a resolved revision")
	}
	if err := i.Configuration.ValidateWithDigest(); err != nil {
		return prefixError("configuration", err)
	}
	if err := i.Policy.Validate(); err != nil {
		return prefixError("policy", err)
	}
	if err := i.Trust.Validate(); err != nil {
		return prefixError("trust", err)
	}
	if err := i.Kind.Validate(); err != nil {
		return prefixError("kind", err)
	}
	expectedKind, expectedExecutor, expectedClass, supported := operationContract(i.Operation)
	if !supported {
		return invalid("operation", "unsupported_value", "job operation is unsupported")
	}
	if i.Kind != expectedKind {
		return invalid("kind", "operation_mismatch", "job kind does not match operation")
	}
	if err := i.Executor.Validate(); err != nil {
		return prefixError("executor", err)
	}
	if i.Executor.Family != expectedExecutor {
		return invalid("executor.family", "operation_mismatch", "executor family does not match operation")
	}
	if i.Class != expectedClass {
		return invalid("class", "derived_mismatch", "execution class is derived from operation")
	}
	if err := i.Input.ValidateFor(i.Operation); err != nil {
		return err
	}
	if len(i.Dependencies) > maxListEntries {
		return invalid("dependencies", "invalid_count", "dependencies exceed the supported count")
	}
	seen := make(map[JobID]struct{}, len(i.Dependencies))
	for index, dependency := range i.Dependencies {
		if err := dependency.Validate(); err != nil {
			return prefixError("dependencies", prefixError(indexPath(index), err))
		}
		if _, exists := seen[dependency]; exists {
			return invalid("dependencies", "duplicate_value", "job contains duplicate dependencies")
		}
		seen[dependency] = struct{}{}
		if dependency == i.ID {
			return invalid("dependencies", "self_link", "job cannot depend on itself")
		}
	}
	if len(i.Gates) > maxListEntries {
		return invalid("gates", "invalid_count", "gates exceed the supported count")
	}
	for index, gate := range i.Gates {
		if err := gate.Validate(); err != nil {
			return prefixError("gates", prefixError(indexPath(index), err))
		}
	}
	if err := i.Resources.Validate(); err != nil {
		return prefixError("resources", err)
	}
	if err := i.Timeout.Validate(); err != nil {
		return prefixError("timeout", err)
	}
	return prefixError("retry", i.Retry.Validate())
}

func validateLabels(path string, values []string) error {
	if len(values) == 0 || len(values) > maxListEntries {
		return invalid(path, "invalid_count", "label list must be bounded and non-empty")
	}
	for index, value := range values {
		if err := ValidateConcreteLabel(value); err != nil {
			return prefixError(path, prefixError(indexPath(index), err))
		}
	}
	return nil
}

func validatePaths(path string, values []string) error {
	if len(values) == 0 || len(values) > maxListEntries {
		return invalid(path, "invalid_count", "path list must be bounded and non-empty")
	}
	for index, value := range values {
		if err := ValidateRepositoryPath(value); err != nil {
			return prefixError(path, prefixError(indexPath(index), err))
		}
	}
	return nil
}

type JobSpec struct {
	input        JobSpecInput
	digest       ContentDigest
	operationKey OperationKey
}

func NewJobSpec(input JobSpecInput) (JobSpec, error) {
	if err := input.Validate(); err != nil {
		return JobSpec{}, err
	}
	key := operationKeyFor(input)
	projection := jobProjection(input, key)
	return JobSpec{input: cloneValue(input), digest: sha256ContractDigest(projection), operationKey: key}, nil
}

func (j JobSpec) Input() JobSpecInput         { return cloneValue(j.input) }
func (j JobSpec) ID() JobID                   { return j.input.ID }
func (j JobSpec) PlanID() PlanID              { return j.input.Plan }
func (j JobSpec) RunID() RunID                { return j.input.Run }
func (j JobSpec) Kind() JobKind               { return j.input.Kind }
func (j JobSpec) Operation() JobOperation     { return j.input.Operation }
func (j JobSpec) Class() ExecutionClass       { return j.input.Class }
func (j JobSpec) Executor() ExecutorReference { return j.input.Executor }
func (j JobSpec) Dependencies() []JobID       { return append([]JobID(nil), j.input.Dependencies...) }
func (j JobSpec) Gates() []GateReference      { return cloneValue(j.input.Gates) }
func (j JobSpec) Digest() ContentDigest       { return j.digest }
func (j JobSpec) OperationKey() OperationKey  { return j.operationKey }
func (j JobSpec) CanonicalBytes() []byte {
	return append([]byte(nil), jobProjection(j.input, j.operationKey)...)
}

func (j JobSpec) Validate() error {
	if err := j.input.Validate(); err != nil {
		return err
	}
	key := operationKeyFor(j.input)
	if j.operationKey != key {
		return invalid("operation_key", "digest_mismatch", "operation key does not match canonical content")
	}
	return verifyContractDigest("digest", j.digest, jobProjection(j.input, key))
}

func (j JobSpec) MarshalJSON() ([]byte, error) {
	if err := j.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(jobSpecWireFrom(j))
}

func (j *JobSpec) UnmarshalJSON(data []byte) error {
	var wire jobSpecWire
	if err := decodeStrict(data, &wire); err != nil {
		return err
	}
	input := wire.input()
	parsed, err := NewJobSpec(input)
	if err != nil {
		return err
	}
	if wire.OperationKey != parsed.operationKey {
		return invalid("operation_key", "digest_mismatch", "operation key does not match canonical content")
	}
	if err := verifyContractDigest("digest", wire.Digest, parsed.CanonicalBytes()); err != nil {
		return err
	}
	*j = parsed
	return nil
}

type jobSpecWire struct {
	SchemaVersion uint16                   `json:"schema_version"`
	ID            JobID                    `json:"job_id"`
	Plan          PlanID                   `json:"plan_id"`
	Run           RunID                    `json:"run_id"`
	Repository    RepositoryBinding        `json:"repository"`
	Revision      RevisionContext          `json:"revision"`
	Configuration ConfigurationReference   `json:"configuration"`
	Policy        PolicyReference          `json:"policy"`
	Trust         TrustReference           `json:"trust"`
	Kind          JobKind                  `json:"kind"`
	Operation     JobOperation             `json:"operation"`
	Executor      ExecutorReference        `json:"executor"`
	Input         JobInput                 `json:"input"`
	Dependencies  []JobID                  `json:"dependencies"`
	Gates         []GateReference          `json:"gates"`
	Resources     ResourceProfileReference `json:"resources"`
	Timeout       TimeoutProfileReference  `json:"timeout"`
	Retry         RetryProfileReference    `json:"retry"`
	Class         ExecutionClass           `json:"class"`
	OperationKey  OperationKey             `json:"operation_key"`
	Digest        ContentDigest            `json:"digest"`
}

func jobSpecWireFrom(j JobSpec) jobSpecWire {
	i := j.input
	return jobSpecWire{SchemaVersion: i.SchemaVersion, ID: i.ID, Plan: i.Plan, Run: i.Run, Repository: i.Repository,
		Revision: i.Revision, Configuration: i.Configuration, Policy: i.Policy, Trust: i.Trust, Kind: i.Kind,
		Operation: i.Operation, Executor: i.Executor, Input: i.Input, Dependencies: i.Dependencies, Gates: i.Gates,
		Resources: i.Resources, Timeout: i.Timeout, Retry: i.Retry, Class: i.Class, OperationKey: j.operationKey, Digest: j.digest}
}

func (w jobSpecWire) input() JobSpecInput {
	return JobSpecInput{SchemaVersion: w.SchemaVersion, ID: w.ID, Plan: w.Plan, Run: w.Run, Repository: w.Repository,
		Revision: w.Revision, Configuration: w.Configuration, Policy: w.Policy, Trust: w.Trust, Kind: w.Kind,
		Operation: w.Operation, Executor: w.Executor, Input: w.Input, Dependencies: w.Dependencies, Gates: w.Gates,
		Resources: w.Resources, Timeout: w.Timeout, Retry: w.Retry, Class: w.Class}
}

func operationKeyFor(input JobSpecInput) OperationKey {
	encoder := newCanonicalEncoder("operation", OperationKeyVersion)
	encodeCanonicalField(encoder, "run", input.Run)
	encodeCanonicalField(encoder, "repository", input.Repository)
	encodeCanonicalField(encoder, "revision", input.Revision)
	encodeCanonicalField(encoder, "configuration", input.Configuration)
	encodeCanonicalField(encoder, "policy", input.Policy)
	encodeCanonicalField(encoder, "trust", input.Trust)
	encodeCanonicalField(encoder, "kind", input.Kind)
	encodeCanonicalField(encoder, "operation", input.Operation)
	encodeCanonicalField(encoder, "executor", input.Executor)
	encodeCanonicalField(encoder, "input", input.Input)
	encodeCanonicalField(encoder, "gates", input.Gates)
	encodeCanonicalField(encoder, "resources", input.Resources)
	encodeCanonicalField(encoder, "timeout", input.Timeout)
	encodeCanonicalField(encoder, "retry", input.Retry)
	encodeCanonicalField(encoder, "class", input.Class)
	return OperationKey(sha256ContractDigest(encoder.Bytes()))
}

func jobProjection(input JobSpecInput, key OperationKey) []byte {
	encoder := newCanonicalEncoder("job", JobDigestVersion)
	encodeCanonicalField(encoder, "schema_version", input.SchemaVersion)
	encodeCanonicalField(encoder, "job_id", input.ID)
	encodeCanonicalField(encoder, "plan_id", input.Plan)
	encodeCanonicalField(encoder, "run_id", input.Run)
	encodeCanonicalField(encoder, "repository", input.Repository)
	encodeCanonicalField(encoder, "revision", input.Revision)
	encodeCanonicalField(encoder, "configuration", input.Configuration)
	encodeCanonicalField(encoder, "policy", input.Policy)
	encodeCanonicalField(encoder, "trust", input.Trust)
	encodeCanonicalField(encoder, "kind", input.Kind)
	encodeCanonicalField(encoder, "operation", input.Operation)
	encodeCanonicalField(encoder, "executor", input.Executor)
	encodeCanonicalField(encoder, "input", input.Input)
	encodeCanonicalField(encoder, "dependencies", input.Dependencies)
	encodeCanonicalField(encoder, "gates", input.Gates)
	encodeCanonicalField(encoder, "resources", input.Resources)
	encodeCanonicalField(encoder, "timeout", input.Timeout)
	encodeCanonicalField(encoder, "retry", input.Retry)
	encodeCanonicalField(encoder, "class", input.Class)
	encodeCanonicalField(encoder, "operation_key", key)
	return encoder.Bytes()
}

func encodeCanonicalField(encoder *canonicalEncoder, name string, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("canonical domain value cannot be encoded: %v", err))
	}
	encoder.string(name)
	encoder.bytesValue(data)
}

func cloneValue[T any](value T) T {
	data, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("validated domain value cannot be copied: %v", err))
	}
	var clone T
	if err := json.Unmarshal(data, &clone); err != nil {
		panic(fmt.Sprintf("validated domain value cannot be copied: %v", err))
	}
	return clone
}
