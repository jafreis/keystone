package domain

import (
	"bytes"
	"encoding/json"
	"reflect"
)

type SelectionMode string

const (
	SelectionModeHybrid    SelectionMode = "hybrid"
	SelectionModeGraphOnly SelectionMode = "graph_only"
	SelectionModePathOnly  SelectionMode = "path_only"
)

func (m SelectionMode) Validate() error {
	switch m {
	case SelectionModeHybrid, SelectionModeGraphOnly, SelectionModePathOnly:
		return nil
	default:
		return invalid("mode", "unsupported_value", "selection mode is unsupported")
	}
}

type SelectionReason string

const (
	SelectionReasonWorkRequired   SelectionReason = "work_required"
	SelectionReasonCompleteNoWork SelectionReason = "complete_no_work"
)

func (r SelectionReason) Validate() error {
	switch r {
	case SelectionReasonWorkRequired, SelectionReasonCompleteNoWork:
		return nil
	default:
		return invalid("reason", "unsupported_value", "selection reason is unsupported")
	}
}

type SelectionProvenance struct {
	Mode             SelectionMode          `json:"mode"`
	Complete         bool                   `json:"complete"`
	Analysis         AnalysisID             `json:"analysis_id"`
	AnalysisEvidence EvidenceReference      `json:"analysis_evidence"`
	DetectorVersion  string                 `json:"detector_version"`
	MappingVersion   string                 `json:"mapping_version"`
	RevisionID       RevisionID             `json:"revision_id"`
	Configuration    ConfigurationReference `json:"configuration"`
	GraphEvidence    *EvidenceReference     `json:"graph_evidence,omitempty"`
	PathEvidence     *EvidenceReference     `json:"path_evidence,omitempty"`
	Reason           SelectionReason        `json:"reason"`
}

func (p SelectionProvenance) Validate() error {
	if err := p.Mode.Validate(); err != nil {
		return err
	}
	if !p.Complete {
		return invalid("complete", "incomplete", "executable plan provenance must be complete")
	}
	if err := p.Analysis.Validate(); err != nil {
		return prefixError("analysis_id", err)
	}
	if err := p.AnalysisEvidence.Validate(); err != nil {
		return prefixError("analysis_evidence", err)
	}
	if err := validateVersion("detector_version", p.DetectorVersion); err != nil {
		return err
	}
	if err := validateVersion("mapping_version", p.MappingVersion); err != nil {
		return err
	}
	if err := p.RevisionID.Validate(); err != nil {
		return prefixError("revision_id", err)
	}
	if err := p.Configuration.ValidateWithDigest(); err != nil {
		return prefixError("configuration", err)
	}
	if p.Mode == SelectionModeHybrid || p.Mode == SelectionModeGraphOnly {
		if p.GraphEvidence == nil {
			return invalid("graph_evidence", "missing", "selection mode requires graph evidence")
		}
	}
	if p.Mode == SelectionModeHybrid || p.Mode == SelectionModePathOnly {
		if p.PathEvidence == nil {
			return invalid("path_evidence", "missing", "selection mode requires path evidence")
		}
	}
	if p.GraphEvidence != nil {
		if err := p.GraphEvidence.Validate(); err != nil {
			return prefixError("graph_evidence", err)
		}
	}
	if p.PathEvidence != nil {
		if err := p.PathEvidence.Validate(); err != nil {
			return prefixError("path_evidence", err)
		}
	}
	return p.Reason.Validate()
}

type ExecutionPlanInput struct {
	SchemaVersion uint16                `json:"schema_version"`
	ID            PlanID                `json:"plan_id"`
	Run           RunID                 `json:"run_id"`
	Repository    RepositoryBinding     `json:"repository"`
	Revision      RevisionContext       `json:"revision"`
	Configuration ConfigurationSnapshot `json:"configuration"`
	Policy        PolicyReference       `json:"policy"`
	Trust         TrustReference        `json:"trust"`
	Provenance    SelectionProvenance   `json:"provenance"`
	RequiredGates []GateReference       `json:"required_gates"`
	Jobs          []JobSpecInput        `json:"jobs"`
}

func (p ExecutionPlanInput) Validate() error {
	if p.SchemaVersion != PlanSchemaVersion {
		return invalid("schema_version", "unsupported_value", "plan schema version is unsupported")
	}
	if err := p.ID.Validate(); err != nil {
		return prefixError("plan_id", err)
	}
	if err := p.Run.Validate(); err != nil {
		return prefixError("run_id", err)
	}
	if err := p.Repository.Validate(); err != nil {
		return prefixError("repository", err)
	}
	if err := p.Revision.Validate(); err != nil {
		return prefixError("revision", err)
	}
	if p.Revision.State != RevisionStateResolved {
		return invalid("revision.state", "not_ready", "executable plan requires a resolved revision")
	}
	if err := p.Configuration.Validate(); err != nil {
		return prefixError("configuration", err)
	}
	if err := p.Policy.Validate(); err != nil {
		return prefixError("policy", err)
	}
	if err := p.Trust.Validate(); err != nil {
		return prefixError("trust", err)
	}
	if err := p.Provenance.Validate(); err != nil {
		return prefixError("provenance", err)
	}
	if p.Provenance.RevisionID != p.Revision.ID {
		return invalid("provenance.revision_id", "mismatch", "selection provenance uses another revision")
	}
	if !p.Provenance.Configuration.Equal(p.Configuration.Profile) {
		return invalid("provenance.configuration", "mismatch", "selection provenance uses another configuration")
	}
	if len(p.RequiredGates) > maxListEntries {
		return invalid("required_gates", "invalid_count", "required gates exceed the supported count")
	}
	for index, gate := range p.RequiredGates {
		if err := gate.Validate(); err != nil {
			return prefixError("required_gates", prefixError(indexPath(index), err))
		}
	}
	if len(p.Jobs) > maxListEntries {
		return invalid("jobs", "invalid_count", "jobs exceed the supported count")
	}
	if len(p.Jobs) == 0 && p.Provenance.Reason != SelectionReasonCompleteNoWork {
		return invalid("provenance.reason", "missing", "an empty plan requires an explicit complete-no-work reason")
	}
	if len(p.Jobs) != 0 && p.Provenance.Reason == SelectionReasonCompleteNoWork {
		return invalid("provenance.reason", "inconsistent_value", "nonempty plan cannot claim complete-no-work")
	}
	return nil
}

type ExecutionPlan struct {
	input  ExecutionPlanInput
	jobs   []JobSpec
	digest ContentDigest
}

func NewExecutionPlan(input ExecutionPlanInput) (ExecutionPlan, error) {
	if err := input.Validate(); err != nil {
		return ExecutionPlan{}, err
	}
	jbos := make([]JobSpec, 0, len(input.Jobs))
	byID := make(map[JobID]JobSpec, len(input.Jobs))
	byKey := make(map[OperationKey]struct{}, len(input.Jobs))
	for index, jobInput := range input.Jobs {
		job, err := NewJobSpec(jobInput)
		if err != nil {
			return ExecutionPlan{}, prefixError("jobs", prefixError(indexPath(index), err))
		}
		if job.PlanID() != input.ID {
			return ExecutionPlan{}, invalid("jobs", "scope_mismatch", "job belongs to another plan")
		}
		if job.RunID() != input.Run {
			return ExecutionPlan{}, invalid("jobs", "scope_mismatch", "job belongs to another run")
		}
		if !job.input.Repository.Equal(input.Repository) {
			return ExecutionPlan{}, invalid("jobs", "scope_mismatch", "job belongs to another repository")
		}
		if !reflect.DeepEqual(job.input.Revision, input.Revision) {
			return ExecutionPlan{}, invalid("jobs", "scope_mismatch", "job uses another revision context")
		}
		if !job.input.Configuration.Equal(input.Configuration.Profile) {
			return ExecutionPlan{}, invalid("jobs", "scope_mismatch", "job uses another configuration")
		}
		if job.input.Policy != input.Policy || job.input.Trust != input.Trust {
			return ExecutionPlan{}, invalid("jobs", "scope_mismatch", "job uses another policy or trust scope")
		}
		if _, exists := byID[job.ID()]; exists {
			return ExecutionPlan{}, invalid("jobs", "duplicate_value", "plan contains duplicate job identity")
		}
		if _, exists := byKey[job.OperationKey()]; exists {
			return ExecutionPlan{}, invalid("jobs", "duplicate_value", "plan contains duplicate logical operation")
		}
		byID[job.ID()] = job
		byKey[job.OperationKey()] = struct{}{}
		jbos = append(jbos, job)
	}
	for _, job := range jbos {
		for _, dependency := range job.Dependencies() {
			if _, exists := byID[dependency]; !exists {
				return ExecutionPlan{}, invalid("jobs.dependencies", "missing_reference", "job dependency is not in the plan")
			}
		}
	}
	if err := validateAcyclic(jbos, byID); err != nil {
		return ExecutionPlan{}, err
	}
	for _, job := range jbos {
		if err := validateJobBindings(job, byID); err != nil {
			return ExecutionPlan{}, err
		}
	}
	if err := validatePlanGateBindings(input.RequiredGates, byID, jbos); err != nil {
		return ExecutionPlan{}, err
	}
	result := ExecutionPlan{input: cloneValue(input), jobs: cloneValue(jbos)}
	result.input.Jobs = nil
	result.digest = sha256ContractDigest(planProjection(result))
	return result, nil
}

func validateAcyclic(jobs []JobSpec, byID map[JobID]JobSpec) error {
	state := make(map[JobID]uint8, len(jobs))
	var visit func(JobID) error
	visit = func(id JobID) error {
		switch state[id] {
		case 1:
			return invalid("jobs.dependencies", "cycle", "plan dependency graph contains a cycle")
		case 2:
			return nil
		}
		state[id] = 1
		for _, dependency := range byID[id].Dependencies() {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	for _, job := range jobs {
		if err := visit(job.ID()); err != nil {
			return err
		}
	}
	return nil
}

func isPrerequisite(candidate, dependent JobID, byID map[JobID]JobSpec) bool {
	seen := map[JobID]struct{}{}
	var walk func(JobID) bool
	walk = func(id JobID) bool {
		if id == candidate {
			return true
		}
		if _, exists := seen[id]; exists {
			return false
		}
		seen[id] = struct{}{}
		for _, dependency := range byID[id].Dependencies() {
			if walk(dependency) {
				return true
			}
		}
		return false
	}
	return walk(dependent)
}

func validateJobBindings(job JobSpec, byID map[JobID]JobSpec) error {
	var artifact *ProducerOutputReference
	switch {
	case job.input.Input.PublishOCI != nil:
		artifact = job.input.Input.PublishOCI.Artifact.Producer
	case job.input.Input.Deploy != nil:
		artifact = job.input.Input.Deploy.Artifact.Producer
	}
	if artifact != nil {
		producer, exists := byID[artifact.Producer]
		if !exists {
			return invalid("jobs.input.artifact.producer", "missing_reference", "artifact producer is not in the plan")
		}
		if !isPrerequisite(producer.ID(), job.ID(), byID) {
			return invalid("jobs.input.artifact.producer", "not_prerequisite", "artifact producer is not a prerequisite")
		}
		if producer.input.Revision.ID != artifact.RevisionID || !producer.input.Configuration.Equal(artifact.Configuration) {
			return invalid("jobs.input.artifact", "scope_mismatch", "artifact producer scope does not match job")
		}
		if !producerDeclaresOutput(producer, artifact.Output, artifact.Kind) {
			return invalid("jobs.input.artifact.output", "missing_reference", "producer does not declare the requested output")
		}
	}
	for index, gate := range job.Gates() {
		if gate.Producer == nil {
			continue
		}
		producer, exists := byID[*gate.Producer]
		if !exists {
			return prefixError("jobs.gates", prefixError(indexPath(index), invalid("producer", "missing_reference", "gate producer is not in the plan")))
		}
		if !isPrerequisite(producer.ID(), job.ID(), byID) {
			return invalid("jobs.gates", "not_prerequisite", "gate producer is not a prerequisite")
		}
	}
	return nil
}

func producerDeclaresOutput(job JobSpec, output OutputID, kind OutputKind) bool {
	if job.input.Input.BazelBuild == nil {
		return false
	}
	for _, declaration := range job.input.Input.BazelBuild.Outputs {
		if declaration.ID == output && declaration.Kind == kind {
			return true
		}
	}
	return false
}

func validatePlanGateBindings(gates []GateReference, byID map[JobID]JobSpec, jobs []JobSpec) error {
	seen := make(map[GateID]struct{}, len(gates))
	for index, gate := range gates {
		if _, exists := seen[gate.ID]; exists {
			return invalid("required_gates", "duplicate_value", "plan contains duplicate required gates")
		}
		seen[gate.ID] = struct{}{}
		if gate.Producer != nil {
			if _, exists := byID[*gate.Producer]; !exists {
				return prefixError("required_gates", prefixError(indexPath(index), invalid("producer", "missing_reference", "required gate producer is not in the plan")))
			}
		}
	}
	_ = jobs
	return nil
}

func (p ExecutionPlan) Validate() error {
	checkInput := p.input
	checkInput.Jobs = make([]JobSpecInput, len(p.jobs))
	for index, job := range p.jobs {
		checkInput.Jobs[index] = job.Input()
	}
	if err := checkInput.Validate(); err != nil {
		return err
	}
	if p.input.Jobs == nil {
		if err := validatePlanIdentity(checkInput, p.jobs); err != nil {
			return err
		}
		if err := verifyContractDigest("digest", p.digest, planProjection(p)); err != nil {
			return err
		}
		return nil
	}
	return invalid("jobs", "inconsistent_value", "plan input must be frozen before validation")
}

func validatePlanIdentity(input ExecutionPlanInput, jobs []JobSpec) error {
	if len(input.Jobs) != len(jobs) {
		return invalid("jobs", "inconsistent_value", "plan job count changed")
	}
	for index, job := range jobs {
		if err := job.Validate(); err != nil {
			return prefixError("jobs", prefixError(indexPath(index), err))
		}
		if !reflect.DeepEqual(input.Jobs[index], job.Input()) {
			return invalid("jobs", "inconsistent_value", "plan job content changed")
		}
	}
	return nil
}

func (p ExecutionPlan) ID() PlanID                { return p.input.ID }
func (p ExecutionPlan) RunID() RunID              { return p.input.Run }
func (p ExecutionPlan) Digest() ContentDigest     { return p.digest }
func (p ExecutionPlan) Revision() RevisionContext { return cloneValue(p.input.Revision) }
func (p ExecutionPlan) Configuration() ConfigurationSnapshot {
	return cloneValue(p.input.Configuration)
}
func (p ExecutionPlan) Provenance() SelectionProvenance { return cloneValue(p.input.Provenance) }
func (p ExecutionPlan) Jobs() []JobSpec {
	result := make([]JobSpec, len(p.jobs))
	for index, job := range p.jobs {
		result[index] = JobSpec{input: cloneValue(job.input), digest: job.digest, operationKey: job.operationKey}
	}
	return result
}
func (p ExecutionPlan) RequiredGates() []GateReference { return cloneValue(p.input.RequiredGates) }
func (p ExecutionPlan) CanonicalBytes() []byte         { return append([]byte(nil), planProjection(p)...) }

func (p ExecutionPlan) Reference() PlanReference {
	return PlanReference{ID: p.ID(), Version: "v1", Digest: p.Digest()}
}

func (p ExecutionPlan) Equal(other ExecutionPlan) bool {
	return p.ID() == other.ID() && p.Reference().Version == other.Reference().Version && bytes.Equal(p.CanonicalBytes(), other.CanonicalBytes()) && p.Digest() == other.Digest()
}

func CompareExecutionPlans(existing, candidate ExecutionPlan) error {
	if err := existing.Validate(); err != nil {
		return prefixError("existing", err)
	}
	if err := candidate.Validate(); err != nil {
		return prefixError("candidate", err)
	}
	if existing.ID() != candidate.ID() || existing.Reference().Version != candidate.Reference().Version {
		return nil
	}
	if existing.Equal(candidate) {
		return nil
	}
	return invalid("plan_id", "conflict", "same plan identity has different canonical content")
}

type TrustedPlanBinding struct {
	Repository           RepositoryBinding
	Revision             RevisionContext
	Configuration        ConfigurationSnapshot
	Policy               PolicyReference
	Trust                TrustReference
	ApprovedExecutors    []ExecutorReference
	ApprovedProfiles     []ProfileReference
	ApprovedGates        []GateReference
	ApprovedDestinations []DestinationReference
}

func (b TrustedPlanBinding) Validate() error {
	if err := b.Repository.Validate(); err != nil {
		return prefixError("repository", err)
	}
	if err := b.Revision.Validate(); err != nil {
		return prefixError("revision", err)
	}
	if err := b.Configuration.Validate(); err != nil {
		return prefixError("configuration", err)
	}
	if err := b.Policy.Validate(); err != nil {
		return prefixError("policy", err)
	}
	if err := b.Trust.Validate(); err != nil {
		return prefixError("trust", err)
	}
	if len(b.ApprovedExecutors) > maxListEntries || len(b.ApprovedProfiles) > maxListEntries || len(b.ApprovedGates) > maxListEntries || len(b.ApprovedDestinations) > maxListEntries {
		return invalid("trusted_binding", "invalid_count", "trusted binding contains too many references")
	}
	for index, executor := range b.ApprovedExecutors {
		if err := executor.Validate(); err != nil {
			return prefixError("approved_executors", prefixError(indexPath(index), err))
		}
	}
	for index, profile := range b.ApprovedProfiles {
		if err := profile.Validate(); err != nil {
			return prefixError("approved_profiles", prefixError(indexPath(index), err))
		}
	}
	for index, gate := range b.ApprovedGates {
		if err := gate.Validate(); err != nil {
			return prefixError("approved_gates", prefixError(indexPath(index), err))
		}
	}
	for index, destination := range b.ApprovedDestinations {
		if err := destination.Validate(); err != nil {
			return prefixError("approved_destinations", prefixError(indexPath(index), err))
		}
	}
	return nil
}

func (p ExecutionPlan) ValidateAgainst(run Run, trusted TrustedPlanBinding) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if err := run.Validate(); err != nil {
		return prefixError("run", err)
	}
	if err := trusted.Validate(); err != nil {
		return prefixError("trusted_binding", err)
	}
	if p.RunID() != run.ID || !p.input.Repository.Equal(run.Repository) || !reflect.DeepEqual(p.input.Revision, run.Revision) {
		return invalid("run", "mismatch", "plan does not match the supplied run")
	}
	if !reflect.DeepEqual(p.input.Configuration, run.Approved) || p.input.Policy != run.Policy || p.input.Trust != run.Trust {
		return invalid("trusted_scope", "mismatch", "plan does not match the run's approved scope")
	}
	if !p.input.Repository.Equal(trusted.Repository) || !reflect.DeepEqual(p.input.Revision, trusted.Revision) || !reflect.DeepEqual(p.input.Configuration, trusted.Configuration) || p.input.Policy != trusted.Policy || p.input.Trust != trusted.Trust {
		return invalid("trusted_binding", "mismatch", "plan does not match the trusted binding")
	}
	if run.Plan != nil && !reflect.DeepEqual(*run.Plan, p.Reference()) {
		return invalid("run.plan", "mismatch", "plan does not match the run attachment")
	}
	for index, job := range p.jobs {
		if err := job.validateAgainst(run, trusted); err != nil {
			return prefixError("jobs", prefixError(indexPath(index), err))
		}
	}
	return nil
}

func (j JobSpec) validateAgainst(run Run, trusted TrustedPlanBinding) error {
	if j.RunID() != run.ID || !j.input.Repository.Equal(run.Repository) || !reflect.DeepEqual(j.input.Revision, run.Revision) {
		return invalid("job", "mismatch", "job does not match the supplied run")
	}
	if j.input.Policy != run.Policy || j.input.Trust != run.Trust {
		return invalid("job", "mismatch", "job trust scope does not match the supplied run")
	}
	if len(trusted.ApprovedExecutors) > 0 && !containsExecutor(trusted.ApprovedExecutors, j.input.Executor) {
		return invalid("executor", "unapproved", "job executor is not approved")
	}
	if len(trusted.ApprovedProfiles) > 0 && (!containsProfile(trusted.ApprovedProfiles, j.input.Resources) || !containsProfile(trusted.ApprovedProfiles, j.input.Timeout) || !containsProfile(trusted.ApprovedProfiles, j.input.Retry)) {
		return invalid("profiles", "unapproved", "job profile is not approved")
	}
	for index, gate := range j.input.Gates {
		if len(trusted.ApprovedGates) > 0 && !containsGate(trusted.ApprovedGates, gate) {
			return prefixError("gates", prefixError(indexPath(index), invalid("gate", "unapproved", "job gate is not approved")))
		}
	}
	for _, destination := range jobDestinations(j) {
		if len(trusted.ApprovedDestinations) > 0 && !containsDestination(trusted.ApprovedDestinations, destination) {
			return invalid("destination", "unapproved", "job destination is not approved")
		}
	}
	return nil
}

func containsExecutor(values []ExecutorReference, target ExecutorReference) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func containsProfile(values []ProfileReference, target ProfileReference) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func containsGate(values []GateReference, target GateReference) bool {
	for _, value := range values {
		if reflect.DeepEqual(value, target) {
			return true
		}
	}
	return false
}
func containsDestination(values []DestinationReference, target DestinationReference) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func jobDestinations(job JobSpec) []DestinationReference {
	result := []DestinationReference{}
	if job.input.Input.KubernetesValidation != nil && job.input.Input.KubernetesValidation.Destination != nil {
		result = append(result, *job.input.Input.KubernetesValidation.Destination)
	}
	if job.input.Input.PublishOCI != nil {
		result = append(result, job.input.Input.PublishOCI.Destination)
	}
	if job.input.Input.Deploy != nil {
		result = append(result, job.input.Input.Deploy.Destination)
	}
	return result
}

func planProjection(plan ExecutionPlan) []byte {
	encoder := newCanonicalEncoder("plan", PlanDigestVersion)
	i := plan.input
	encodeCanonicalField(encoder, "schema_version", i.SchemaVersion)
	encodeCanonicalField(encoder, "plan_id", i.ID)
	encodeCanonicalField(encoder, "run_id", i.Run)
	encodeCanonicalField(encoder, "repository", i.Repository)
	encodeCanonicalField(encoder, "revision", i.Revision)
	encodeCanonicalField(encoder, "configuration", i.Configuration)
	encodeCanonicalField(encoder, "policy", i.Policy)
	encodeCanonicalField(encoder, "trust", i.Trust)
	encodeCanonicalField(encoder, "provenance", i.Provenance)
	encodeCanonicalField(encoder, "required_gates", i.RequiredGates)
	encoder.list(len(plan.jobs), func(index int) {
		encoder.bytesValue(plan.jobs[index].CanonicalBytes())
		encodeCanonicalField(encoder, "job_digest", plan.jobs[index].Digest())
	})
	return encoder.Bytes()
}

func (p ExecutionPlan) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	wire := executionPlanWire{SchemaVersion: p.input.SchemaVersion, ID: p.input.ID, Run: p.input.Run, Repository: p.input.Repository, Revision: p.input.Revision, Configuration: p.input.Configuration, Policy: p.input.Policy, Trust: p.input.Trust, Provenance: p.input.Provenance, RequiredGates: p.input.RequiredGates, Jobs: p.jobs, Digest: p.digest}
	return json.Marshal(wire)
}

func (p *ExecutionPlan) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return invalid("plan", "invalid_json", "plan cannot be null")
	}
	var wire executionPlanWire
	if err := decodeStrict(data, &wire); err != nil {
		return err
	}
	input := ExecutionPlanInput{SchemaVersion: wire.SchemaVersion, ID: wire.ID, Run: wire.Run, Repository: wire.Repository, Revision: wire.Revision, Configuration: wire.Configuration, Policy: wire.Policy, Trust: wire.Trust, Provenance: wire.Provenance, RequiredGates: wire.RequiredGates}
	input.Jobs = make([]JobSpecInput, len(wire.Jobs))
	for index, job := range wire.Jobs {
		input.Jobs[index] = job.Input()
	}
	parsed, err := NewExecutionPlan(input)
	if err != nil {
		return err
	}
	if err := verifyContractDigest("digest", wire.Digest, parsed.CanonicalBytes()); err != nil {
		return err
	}
	*p = parsed
	return nil
}

type executionPlanWire struct {
	SchemaVersion uint16                `json:"schema_version"`
	ID            PlanID                `json:"plan_id"`
	Run           RunID                 `json:"run_id"`
	Repository    RepositoryBinding     `json:"repository"`
	Revision      RevisionContext       `json:"revision"`
	Configuration ConfigurationSnapshot `json:"configuration"`
	Policy        PolicyReference       `json:"policy"`
	Trust         TrustReference        `json:"trust"`
	Provenance    SelectionProvenance   `json:"provenance"`
	RequiredGates []GateReference       `json:"required_gates"`
	Jobs          []JobSpec             `json:"jobs"`
	Digest        ContentDigest         `json:"digest"`
}

func EncodeExecutionPlan(plan ExecutionPlan) ([]byte, error) { return json.Marshal(plan) }
func DecodeExecutionPlan(data []byte) (ExecutionPlan, error) {
	var plan ExecutionPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return ExecutionPlan{}, err
	}
	return plan, nil
}
