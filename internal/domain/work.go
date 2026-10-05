package domain

import (
	"bytes"
	"encoding/json"
	"reflect"
)

type AnalysisKind string

const (
	AnalysisKindRevisionResolution AnalysisKind = "revision_resolution"
	AnalysisKindGraph              AnalysisKind = "graph_analysis"
	AnalysisKindPath               AnalysisKind = "path_analysis"
	AnalysisKindHybrid             AnalysisKind = "hybrid_analysis"
)

func (k AnalysisKind) Validate() error {
	switch k {
	case AnalysisKindRevisionResolution, AnalysisKindGraph, AnalysisKindPath, AnalysisKindHybrid:
		return nil
	default:
		return invalid("kind", "unsupported_value", "analysis kind is unsupported")
	}
}

type AnalysisSpecInput struct {
	SchemaVersion uint16                   `json:"schema_version"`
	ID            AnalysisID               `json:"analysis_id"`
	Run           RunID                    `json:"run_id"`
	Repository    RepositoryBinding        `json:"repository"`
	Revision      RevisionContext          `json:"revision"`
	Configuration ConfigurationReference   `json:"configuration"`
	Policy        PolicyReference          `json:"policy"`
	Trust         TrustReference           `json:"trust"`
	Kind          AnalysisKind             `json:"kind"`
	Mode          SelectionMode            `json:"mode"`
	Evidence      []EvidenceReference      `json:"evidence"`
	Executor      ExecutorReference        `json:"executor"`
	Resources     ResourceProfileReference `json:"resources"`
	Timeout       TimeoutProfileReference  `json:"timeout"`
	Retry         RetryProfileReference    `json:"retry"`
	Class         ExecutionClass           `json:"class"`
}

func (a AnalysisSpecInput) Validate() error {
	if a.SchemaVersion != AnalysisSchemaVersion {
		return invalid("schema_version", "unsupported_value", "analysis schema version is unsupported")
	}
	if err := a.ID.Validate(); err != nil {
		return prefixError("analysis_id", err)
	}
	if err := a.Run.Validate(); err != nil {
		return prefixError("run_id", err)
	}
	if err := a.Repository.Validate(); err != nil {
		return prefixError("repository", err)
	}
	if err := a.Revision.Validate(); err != nil {
		return prefixError("revision", err)
	}
	if a.Revision.State == RevisionStateFailed {
		return invalid("revision.state", "not_ready", "failed revision cannot become executable analysis")
	}
	if err := a.Configuration.ValidateWithDigest(); err != nil {
		return prefixError("configuration", err)
	}
	if err := a.Policy.Validate(); err != nil {
		return prefixError("policy", err)
	}
	if err := a.Trust.Validate(); err != nil {
		return prefixError("trust", err)
	}
	if err := a.Kind.Validate(); err != nil {
		return prefixError("kind", err)
	}
	if err := a.Mode.Validate(); err != nil {
		return prefixError("mode", err)
	}
	if a.Kind == AnalysisKindRevisionResolution && a.Revision.State != RevisionStatePending {
		return invalid("kind", "inconsistent_value", "revision resolution analysis requires a pending revision")
	}
	if a.Kind != AnalysisKindRevisionResolution && a.Revision.State != RevisionStateResolved {
		return invalid("revision.state", "not_ready", "graph or path analysis requires a resolved revision")
	}
	if len(a.Evidence) == 0 || len(a.Evidence) > maxListEntries {
		return invalid("evidence", "invalid_count", "analysis must contain bounded evidence references")
	}
	seen := make(map[string]struct{}, len(a.Evidence))
	for index, evidence := range a.Evidence {
		if err := evidence.Validate(); err != nil {
			return prefixError("evidence", prefixError(indexPath(index), err))
		}
		key := string(evidence.ID) + "\x00" + evidence.Version
		if _, exists := seen[key]; exists {
			return invalid("evidence", "duplicate_value", "analysis contains duplicate evidence")
		}
		seen[key] = struct{}{}
	}
	if err := a.Executor.Validate(); err != nil {
		return prefixError("executor", err)
	}
	if a.Executor.Family == ExecutorFamilyRegistry || a.Executor.Family == ExecutorFamilyDeployment {
		return invalid("executor.family", "unsupported_value", "analysis cannot use a privileged executor family")
	}
	if a.Class != ExecutionClassAnalysis {
		return invalid("class", "derived_mismatch", "analysis class is fixed")
	}
	if err := a.Resources.Validate(); err != nil {
		return prefixError("resources", err)
	}
	if err := a.Timeout.Validate(); err != nil {
		return prefixError("timeout", err)
	}
	return prefixError("retry", a.Retry.Validate())
}

type AnalysisSpec struct {
	input  AnalysisSpecInput
	digest ContentDigest
}

func NewAnalysisSpec(input AnalysisSpecInput) (AnalysisSpec, error) {
	if err := input.Validate(); err != nil {
		return AnalysisSpec{}, err
	}
	return AnalysisSpec{input: cloneValue(input), digest: sha256ContractDigest(analysisProjection(input))}, nil
}

func (a AnalysisSpec) Input() AnalysisSpecInput { return cloneValue(a.input) }
func (a AnalysisSpec) ID() AnalysisID           { return a.input.ID }
func (a AnalysisSpec) AnalysisID() AnalysisID   { return a.input.ID }
func (a AnalysisSpec) RunID() RunID             { return a.input.Run }
func (a AnalysisSpec) Class() ExecutionClass    { return a.input.Class }
func (a AnalysisSpec) Digest() ContentDigest    { return a.digest }
func (a AnalysisSpec) CanonicalBytes() []byte {
	return append([]byte(nil), analysisProjection(a.input)...)
}

func (a AnalysisSpec) Validate() error {
	if err := a.input.Validate(); err != nil {
		return err
	}
	return verifyContractDigest("digest", a.digest, analysisProjection(a.input))
}

func (a AnalysisSpec) MarshalJSON() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(analysisSpecWire{SchemaVersion: a.input.SchemaVersion, ID: a.input.ID, Run: a.input.Run, Repository: a.input.Repository, Revision: a.input.Revision, Configuration: a.input.Configuration, Policy: a.input.Policy, Trust: a.input.Trust, Kind: a.input.Kind, Mode: a.input.Mode, Evidence: a.input.Evidence, Executor: a.input.Executor, Resources: a.input.Resources, Timeout: a.input.Timeout, Retry: a.input.Retry, Class: a.input.Class, Digest: a.digest})
}

func (a *AnalysisSpec) UnmarshalJSON(data []byte) error {
	var wire analysisSpecWire
	if err := decodeStrict(data, &wire); err != nil {
		return err
	}
	input := AnalysisSpecInput{SchemaVersion: wire.SchemaVersion, ID: wire.ID, Run: wire.Run, Repository: wire.Repository, Revision: wire.Revision, Configuration: wire.Configuration, Policy: wire.Policy, Trust: wire.Trust, Kind: wire.Kind, Mode: wire.Mode, Evidence: wire.Evidence, Executor: wire.Executor, Resources: wire.Resources, Timeout: wire.Timeout, Retry: wire.Retry, Class: wire.Class}
	parsed, err := NewAnalysisSpec(input)
	if err != nil {
		return err
	}
	if err := verifyContractDigest("digest", wire.Digest, parsed.CanonicalBytes()); err != nil {
		return err
	}
	*a = parsed
	return nil
}

type analysisSpecWire struct {
	SchemaVersion uint16                   `json:"schema_version"`
	ID            AnalysisID               `json:"analysis_id"`
	Run           RunID                    `json:"run_id"`
	Repository    RepositoryBinding        `json:"repository"`
	Revision      RevisionContext          `json:"revision"`
	Configuration ConfigurationReference   `json:"configuration"`
	Policy        PolicyReference          `json:"policy"`
	Trust         TrustReference           `json:"trust"`
	Kind          AnalysisKind             `json:"kind"`
	Mode          SelectionMode            `json:"mode"`
	Evidence      []EvidenceReference      `json:"evidence"`
	Executor      ExecutorReference        `json:"executor"`
	Resources     ResourceProfileReference `json:"resources"`
	Timeout       TimeoutProfileReference  `json:"timeout"`
	Retry         RetryProfileReference    `json:"retry"`
	Class         ExecutionClass           `json:"class"`
	Digest        ContentDigest            `json:"digest"`
}

func analysisProjection(input AnalysisSpecInput) []byte {
	encoder := newCanonicalEncoder("analysis", AnalysisDigestVersion)
	encodeCanonicalField(encoder, "schema_version", input.SchemaVersion)
	encodeCanonicalField(encoder, "analysis_id", input.ID)
	encodeCanonicalField(encoder, "run_id", input.Run)
	encodeCanonicalField(encoder, "repository", input.Repository)
	encodeCanonicalField(encoder, "revision", input.Revision)
	encodeCanonicalField(encoder, "configuration", input.Configuration)
	encodeCanonicalField(encoder, "policy", input.Policy)
	encodeCanonicalField(encoder, "trust", input.Trust)
	encodeCanonicalField(encoder, "kind", input.Kind)
	encodeCanonicalField(encoder, "mode", input.Mode)
	encodeCanonicalField(encoder, "evidence", input.Evidence)
	encodeCanonicalField(encoder, "executor", input.Executor)
	encodeCanonicalField(encoder, "resources", input.Resources)
	encodeCanonicalField(encoder, "timeout", input.Timeout)
	encodeCanonicalField(encoder, "retry", input.Retry)
	encodeCanonicalField(encoder, "class", input.Class)
	return encoder.Bytes()
}

type WorkKind string

const (
	WorkKindJob      WorkKind = "job"
	WorkKindAnalysis WorkKind = "analysis"
)

func (k WorkKind) Validate() error {
	switch k {
	case WorkKindJob, WorkKindAnalysis:
		return nil
	default:
		return invalid("kind", "unsupported_value", "work kind is unsupported")
	}
}

const maxWorkReferenceBytes = 16 * 1024

type WorkReference struct {
	schemaVersion  uint16
	messageID      MessageID
	id             WorkReferenceID
	runID          RunID
	kind           WorkKind
	class          ExecutionClass
	targetVersion  string
	plan           *PlanReference
	jobID          JobID
	jobDigest      ContentDigest
	analysisID     AnalysisID
	analysisDigest ContentDigest
}

func NewJobWorkReference(messageID MessageID, id WorkReferenceID, plan ExecutionPlan, job JobSpec) (WorkReference, error) {
	if err := plan.Validate(); err != nil {
		return WorkReference{}, prefixError("plan", err)
	}
	if err := job.Validate(); err != nil {
		return WorkReference{}, prefixError("job", err)
	}
	if job.PlanID() != plan.ID() || job.RunID() != plan.RunID() {
		return WorkReference{}, invalid("job", "scope_mismatch", "job does not belong to plan")
	}
	if !planContainsJob(plan, job) {
		return WorkReference{}, invalid("job", "mismatch", "job is not recorded in the authoritative plan")
	}
	if err := messageID.Validate(); err != nil {
		return WorkReference{}, prefixError("message_id", err)
	}
	if err := id.Validate(); err != nil {
		return WorkReference{}, prefixError("work_reference_id", err)
	}
	return WorkReference{schemaVersion: WorkReferenceSchemaVersion, messageID: messageID, id: id, runID: plan.RunID(), kind: WorkKindJob, class: job.Class(), targetVersion: "v1", plan: clonePointer(plan.Reference()), jobID: job.ID(), jobDigest: job.Digest()}, nil
}

func NewAnalysisWorkReference(messageID MessageID, id WorkReferenceID, analysis AnalysisSpec) (WorkReference, error) {
	if err := analysis.Validate(); err != nil {
		return WorkReference{}, prefixError("analysis", err)
	}
	if err := messageID.Validate(); err != nil {
		return WorkReference{}, prefixError("message_id", err)
	}
	if err := id.Validate(); err != nil {
		return WorkReference{}, prefixError("work_reference_id", err)
	}
	return WorkReference{schemaVersion: WorkReferenceSchemaVersion, messageID: messageID, id: id, runID: analysis.RunID(), kind: WorkKindAnalysis, class: ExecutionClassAnalysis, targetVersion: "v1", analysisID: analysis.ID(), analysisDigest: analysis.Digest()}, nil
}

func clonePointer[T any](value T) *T { copy := value; return &copy }

func (w WorkReference) Validate() error {
	if w.schemaVersion != WorkReferenceSchemaVersion {
		return invalid("schema_version", "unsupported_value", "work reference schema version is unsupported")
	}
	if err := w.messageID.Validate(); err != nil {
		return prefixError("message_id", err)
	}
	if err := w.id.Validate(); err != nil {
		return prefixError("work_reference_id", err)
	}
	if err := w.runID.Validate(); err != nil {
		return prefixError("run_id", err)
	}
	if err := w.kind.Validate(); err != nil {
		return err
	}
	if err := validateContractVersion("target_version", w.targetVersion, "v1"); err != nil {
		return err
	}
	switch w.kind {
	case WorkKindJob:
		if w.plan == nil || w.analysisID != "" || w.analysisDigest != "" {
			return invalid("job", "inconsistent_variant", "job reference contains the wrong variant")
		}
		if err := w.plan.Validate(); err != nil {
			return prefixError("plan", err)
		}
		if err := w.jobID.Validate(); err != nil {
			return prefixError("job_id", err)
		}
		if err := validateContractDigest("job_digest", string(w.jobDigest)); err != nil {
			return err
		}
		if w.class != ExecutionClassValidation && w.class != ExecutionClassPublishing && w.class != ExecutionClassDeployment {
			return invalid("class", "derived_mismatch", "job reference class is unsupported")
		}
	case WorkKindAnalysis:
		if w.plan != nil || w.jobID != "" || w.jobDigest != "" {
			return invalid("analysis", "inconsistent_variant", "analysis reference contains the wrong variant")
		}
		if err := w.analysisID.Validate(); err != nil {
			return prefixError("analysis_id", err)
		}
		if err := validateContractDigest("analysis_digest", string(w.analysisDigest)); err != nil {
			return err
		}
		if w.class != ExecutionClassAnalysis {
			return invalid("class", "derived_mismatch", "analysis reference class is fixed")
		}
	}
	return nil
}

func (w WorkReference) MessageID() MessageID             { return w.messageID }
func (w WorkReference) ID() WorkReferenceID              { return w.id }
func (w WorkReference) WorkReferenceID() WorkReferenceID { return w.id }
func (w WorkReference) RunID() RunID                     { return w.runID }
func (w WorkReference) Kind() WorkKind                   { return w.kind }
func (w WorkReference) Class() ExecutionClass            { return w.class }
func (w WorkReference) TargetVersion() string            { return w.targetVersion }
func (w WorkReference) Plan() *PlanReference {
	if w.plan == nil {
		return nil
	}
	return clonePointer(*w.plan)
}
func (w WorkReference) JobID() JobID                  { return w.jobID }
func (w WorkReference) JobDigest() ContentDigest      { return w.jobDigest }
func (w WorkReference) AnalysisID() AnalysisID        { return w.analysisID }
func (w WorkReference) AnalysisDigest() ContentDigest { return w.analysisDigest }

func (w WorkReference) ValidateAgainstJob(plan ExecutionPlan, job JobSpec) error {
	if err := w.Validate(); err != nil {
		return err
	}
	if w.kind != WorkKindJob || w.plan == nil {
		return invalid("kind", "mismatch", "work reference is not job work")
	}
	if !planContainsJob(plan, job) {
		return invalid("job", "mismatch", "job is not recorded in the authoritative plan")
	}
	if w.runID != plan.RunID() || w.jobID != job.ID() || w.jobDigest != job.Digest() || w.class != job.Class() {
		return invalid("reference", "mismatch", "work reference does not match the loaded job")
	}
	if !reflect.DeepEqual(*w.plan, plan.Reference()) {
		return invalid("plan", "mismatch", "work reference does not match the loaded plan")
	}
	return nil
}

func planContainsJob(plan ExecutionPlan, candidate JobSpec) bool {
	for _, recorded := range plan.Jobs() {
		if recorded.ID() == candidate.ID() && recorded.Digest() == candidate.Digest() && bytes.Equal(recorded.CanonicalBytes(), candidate.CanonicalBytes()) {
			return true
		}
	}
	return false
}

func (w WorkReference) ValidateAgainstAnalysis(analysis AnalysisSpec) error {
	if err := w.Validate(); err != nil {
		return err
	}
	if w.kind != WorkKindAnalysis || w.analysisID != analysis.ID() || w.analysisDigest != analysis.Digest() || w.runID != analysis.RunID() || w.class != ExecutionClassAnalysis {
		return invalid("reference", "mismatch", "work reference does not match the loaded analysis")
	}
	return nil
}

type workReferenceWire struct {
	SchemaVersion  uint16          `json:"schema_version"`
	MessageID      MessageID       `json:"message_id"`
	ID             WorkReferenceID `json:"work_reference_id"`
	RunID          RunID           `json:"run_id"`
	Kind           WorkKind        `json:"kind"`
	Class          ExecutionClass  `json:"class"`
	TargetVersion  string          `json:"target_version"`
	Plan           *PlanReference  `json:"plan,omitempty"`
	JobID          JobID           `json:"job_id,omitempty"`
	JobDigest      ContentDigest   `json:"job_digest,omitempty"`
	AnalysisID     AnalysisID      `json:"analysis_id,omitempty"`
	AnalysisDigest ContentDigest   `json:"analysis_digest,omitempty"`
}

func (w WorkReference) wire() workReferenceWire {
	return workReferenceWire{SchemaVersion: w.schemaVersion, MessageID: w.messageID, ID: w.id, RunID: w.runID, Kind: w.kind, Class: w.class, TargetVersion: w.targetVersion, Plan: w.plan, JobID: w.jobID, JobDigest: w.jobDigest, AnalysisID: w.analysisID, AnalysisDigest: w.analysisDigest}
}

func (w workReferenceWire) value() WorkReference {
	return WorkReference{schemaVersion: w.SchemaVersion, messageID: w.MessageID, id: w.ID, runID: w.RunID, kind: w.Kind, class: w.Class, targetVersion: w.TargetVersion, plan: w.Plan, jobID: w.JobID, jobDigest: w.JobDigest, analysisID: w.AnalysisID, analysisDigest: w.AnalysisDigest}
}

func (w WorkReference) MarshalJSON() ([]byte, error) {
	if err := w.Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(w.wire())
	if err != nil {
		return nil, err
	}
	if len(data) > maxWorkReferenceBytes {
		return nil, invalid("work_reference", "too_large", "work reference exceeds the supported size")
	}
	return data, nil
}

func (w *WorkReference) UnmarshalJSON(data []byte) error {
	if len(data) > maxWorkReferenceBytes {
		return invalid("work_reference", "too_large", "work reference exceeds the supported size")
	}
	var wire workReferenceWire
	if err := decodeStrict(data, &wire); err != nil {
		return err
	}
	parsed := wire.value()
	if err := parsed.Validate(); err != nil {
		return err
	}
	*w = parsed
	return nil
}

func EncodeWorkReference(reference WorkReference) ([]byte, error) { return json.Marshal(reference) }

func DecodeWorkReference(data []byte) (WorkReference, error) {
	if len(data) > maxWorkReferenceBytes {
		return WorkReference{}, invalid("work_reference", "too_large", "work reference exceeds the supported size")
	}
	var reference WorkReference
	if err := json.Unmarshal(data, &reference); err != nil {
		return WorkReference{}, err
	}
	return reference, nil
}
