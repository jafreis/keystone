// SPDX-License-Identifier: MPL-2.0
package domain

import (
	"bytes"
	"encoding/json"
	"math"
	"time"
)

const (
	AttemptSchemaVersion             uint16 = 1
	CapacityReservationSchemaVersion uint16 = 1
	maxAttemptBytes                         = 64 * 1024
	maxReservationBytes                     = 64 * 1024
)

type WorkerID string
type FenceToken string
type ReservationID string

func (v WorkerID) Validate() error {
	return validateID("worker", string(v))
}

func (v FenceToken) Validate() error {
	return validateID("fence", string(v))
}

func (v ReservationID) Validate() error {
	return validateID("reservation", string(v))
}

func (c ExecutionClass) Validate() error {
	switch c {
	case ExecutionClassValidation, ExecutionClassPublishing, ExecutionClassDeployment, ExecutionClassAnalysis:
		return nil
	default:
		return invalid("class", "unsupported_value", "execution class is unsupported")
	}
}

type AttemptStage string

const (
	AttemptStageClaimed    AttemptStage = "claimed"
	AttemptStagePreparing  AttemptStage = "preparing"
	AttemptStageExecuting  AttemptStage = "executing"
	AttemptStageCollecting AttemptStage = "collecting_evidence"
	AttemptStageReporting  AttemptStage = "reporting"
	AttemptStageFinished   AttemptStage = "finished"
)

func (s AttemptStage) Validate() error {
	switch s {
	case AttemptStageClaimed, AttemptStagePreparing, AttemptStageExecuting,
		AttemptStageCollecting, AttemptStageReporting, AttemptStageFinished:
		return nil
	default:
		return invalid("stage", "unsupported_value", "attempt stage is unsupported")
	}
}

// AttemptIdentity is the stable identity of one execution try. Worker, fence,
// reservation and timestamps are deliberately excluded so retries retain the
// same logical work identity while each try remains distinguishable.
type AttemptIdentity struct {
	Work       WorkKind   `json:"work"`
	Run        RunID      `json:"run_id"`
	JobID      JobID      `json:"job_id,omitempty"`
	AnalysisID AnalysisID `json:"analysis_id,omitempty"`
	Number     uint64     `json:"attempt_number"`
}

func (i AttemptIdentity) Validate() error {
	if err := i.Work.Validate(); err != nil {
		return prefixError("work", err)
	}
	if err := i.Run.Validate(); err != nil {
		return prefixError("run_id", err)
	}
	if i.Number == 0 {
		return invalid("attempt_number", "invalid_value", "attempt number must be positive")
	}
	jobPresent := i.JobID != ""
	analysisPresent := i.AnalysisID != ""
	if jobPresent == analysisPresent {
		return invalid("identity", "inconsistent_variant", "attempt identity must contain exactly one work identifier")
	}
	if jobPresent {
		if i.Work != WorkKindJob {
			return invalid("work", "mismatch", "job identifier requires job work")
		}
		return prefixError("job_id", i.JobID.Validate())
	}
	if i.Work != WorkKindAnalysis {
		return invalid("work", "mismatch", "analysis identifier requires analysis work")
	}
	return prefixError("analysis_id", i.AnalysisID.Validate())
}

func (i AttemptIdentity) Equal(other AttemptIdentity) bool {
	return i.Work == other.Work && i.Run == other.Run && i.JobID == other.JobID &&
		i.AnalysisID == other.AnalysisID && i.Number == other.Number
}

// WorkScope contains the loaded, immutable contracts that an attempt is
// allowed to execute. It is constructed from those contracts rather than from
// queue or worker fields.
type WorkScope struct {
	Work          WorkKind                 `json:"work"`
	Run           RunID                    `json:"run_id"`
	JobID         JobID                    `json:"job_id,omitempty"`
	AnalysisID    AnalysisID               `json:"analysis_id,omitempty"`
	Class         ExecutionClass           `json:"class"`
	Repository    RepositoryBinding        `json:"repository"`
	Revision      RevisionContext          `json:"revision"`
	Configuration ConfigurationReference   `json:"configuration"`
	Policy        PolicyReference          `json:"policy"`
	Trust         TrustReference           `json:"trust"`
	Resource      ResourceProfileReference `json:"resource"`
	TargetVersion string                   `json:"target_version"`
	Plan          *PlanReference           `json:"plan,omitempty"`
	TargetDigest  ContentDigest            `json:"target_digest"`
	OperationKey  OperationKey             `json:"operation_key,omitempty"`
	Outputs       []OutputDeclaration      `json:"outputs,omitempty"`
}

func NewJobAttemptScope(plan ExecutionPlan, job JobSpec) (WorkScope, error) {
	if err := plan.Validate(); err != nil {
		return WorkScope{}, prefixError("plan", err)
	}
	if err := job.Validate(); err != nil {
		return WorkScope{}, prefixError("job", err)
	}
	if !planContainsJob(plan, job) {
		return WorkScope{}, invalid("job", "mismatch", "job is not recorded in the authoritative plan")
	}
	input := job.Input()
	scope := WorkScope{
		Work:          WorkKindJob,
		Run:           job.RunID(),
		JobID:         job.ID(),
		Class:         job.Class(),
		Repository:    input.Repository,
		Revision:      plan.Revision(),
		Configuration: input.Configuration,
		Policy:        input.Policy,
		Trust:         input.Trust,
		Resource:      input.Resources,
		TargetVersion: "v1",
		Plan:          clonePointer(plan.Reference()),
		TargetDigest:  job.Digest(),
		OperationKey:  job.OperationKey(),
		Outputs:       jobOutputs(input),
	}
	if err := scope.Validate(); err != nil {
		return WorkScope{}, err
	}
	return scope, nil
}

func NewAnalysisAttemptScope(analysis AnalysisSpec) (WorkScope, error) {
	if err := analysis.Validate(); err != nil {
		return WorkScope{}, prefixError("analysis", err)
	}
	input := analysis.Input()
	scope := WorkScope{
		Work:          WorkKindAnalysis,
		Run:           analysis.RunID(),
		AnalysisID:    analysis.ID(),
		Class:         ExecutionClassAnalysis,
		Repository:    input.Repository,
		Revision:      input.Revision,
		Configuration: input.Configuration,
		Policy:        input.Policy,
		Trust:         input.Trust,
		Resource:      input.Resources,
		TargetVersion: "v1",
		TargetDigest:  analysis.Digest(),
	}
	if err := scope.Validate(); err != nil {
		return WorkScope{}, err
	}
	return scope, nil
}

func (s WorkScope) Validate() error {
	if err := s.Work.Validate(); err != nil {
		return prefixError("work", err)
	}
	if err := s.Run.Validate(); err != nil {
		return prefixError("run_id", err)
	}
	if err := s.Class.Validate(); err != nil {
		return prefixError("class", err)
	}
	if err := s.Repository.Validate(); err != nil {
		return prefixError("repository", err)
	}
	if err := s.Revision.Validate(); err != nil {
		return prefixError("revision", err)
	}
	if !s.Revision.Binding.Equal(s.Repository) {
		return invalid("revision.binding", "mismatch", "work scope revision uses another repository")
	}
	if err := s.Configuration.ValidateWithDigest(); err != nil {
		return prefixError("configuration", err)
	}
	if !referencesAgree(string(s.Revision.Configuration.ID), s.Revision.Configuration.Version, s.Revision.Configuration.Digest, string(s.Configuration.ID), s.Configuration.Version, s.Configuration.Digest) {
		return invalid("configuration", "mismatch", "work scope configuration differs from revision")
	}
	if err := s.Policy.Validate(); err != nil {
		return prefixError("policy", err)
	}
	if err := s.Trust.Validate(); err != nil {
		return prefixError("trust", err)
	}
	if err := s.Resource.Validate(); err != nil {
		return prefixError("resource", err)
	}
	if !referencesAgree(string(s.Revision.Policy.ID), s.Revision.Policy.Version, s.Revision.Policy.Digest, string(s.Policy.ID), s.Policy.Version, s.Policy.Digest) {
		return invalid("policy", "mismatch", "work scope policy differs from revision")
	}
	if err := validateVersion("target_version", s.TargetVersion); err != nil {
		return err
	}
	if err := validateContractDigest("target_digest", string(s.TargetDigest)); err != nil {
		return err
	}
	if len(s.Outputs) > maxListEntries {
		return invalid("outputs", "invalid_count", "work scope outputs exceed the supported count")
	}
	seenOutputs := make(map[OutputID]struct{}, len(s.Outputs))
	for index, output := range s.Outputs {
		if err := output.Validate(); err != nil {
			return prefixError("outputs", prefixError(indexPath(index), err))
		}
		if _, exists := seenOutputs[output.ID]; exists {
			return invalid("outputs", "duplicate_value", "work scope contains duplicate outputs")
		}
		seenOutputs[output.ID] = struct{}{}
	}
	jobPresent := s.JobID != ""
	analysisPresent := s.AnalysisID != ""
	if jobPresent == analysisPresent {
		return invalid("work", "inconsistent_variant", "work scope must contain exactly one work identifier")
	}
	if jobPresent {
		if s.Work != WorkKindJob {
			return invalid("work", "mismatch", "job identifier requires job work")
		}
		if err := s.JobID.Validate(); err != nil {
			return prefixError("job_id", err)
		}
		if s.Plan == nil {
			return invalid("plan", "missing", "job scope requires an authoritative plan")
		}
		if err := s.Plan.Validate(); err != nil {
			return prefixError("plan", err)
		}
		if err := s.OperationKey.Validate(); err != nil {
			return prefixError("operation_key", err)
		}
		if s.Revision.State != RevisionStateResolved {
			return invalid("revision.state", "not_ready", "job work requires a resolved revision")
		}
		if s.Class == ExecutionClassAnalysis {
			return invalid("class", "mismatch", "job work cannot use analysis class")
		}
		return nil
	}
	if s.Work != WorkKindAnalysis {
		return invalid("work", "mismatch", "analysis identifier requires analysis work")
	}
	if err := s.AnalysisID.Validate(); err != nil {
		return prefixError("analysis_id", err)
	}
	if s.Class != ExecutionClassAnalysis {
		return invalid("class", "mismatch", "analysis work requires analysis class")
	}
	if s.Plan != nil || s.OperationKey != "" {
		return invalid("analysis", "inconsistent_variant", "analysis scope cannot contain job-only fields")
	}
	if len(s.Outputs) != 0 {
		return invalid("outputs", "inconsistent_variant", "analysis scope cannot contain job outputs")
	}
	return nil
}

func jobOutputs(input JobSpecInput) []OutputDeclaration {
	if input.Input.BazelBuild == nil {
		return nil
	}
	return cloneValue(input.Input.BazelBuild.Outputs)
}

type AttemptInput struct {
	SchemaVersion       uint16          `json:"schema_version"`
	Identity            AttemptIdentity `json:"identity"`
	Scope               WorkScope       `json:"scope"`
	Worker              WorkerID        `json:"worker"`
	Class               ExecutionClass  `json:"class"`
	Fence               FenceToken      `json:"fence"`
	Reservation         ReservationID   `json:"reservation_id"`
	ClaimTime           time.Time       `json:"claim_time"`
	StartTime           *time.Time      `json:"start_time,omitempty"`
	HeartbeatTime       time.Time       `json:"heartbeat_time"`
	ExpiryTime          time.Time       `json:"expiry_time"`
	EndTime             *time.Time      `json:"end_time,omitempty"`
	Stage               AttemptStage    `json:"stage"`
	RecordVersion       uint64          `json:"record_version"`
	CancellationVersion uint64          `json:"cancellation_version"`
}

func (i AttemptInput) Validate() error {
	if i.SchemaVersion != AttemptSchemaVersion {
		return invalid("schema_version", "unsupported_value", "attempt schema version is unsupported")
	}
	if err := i.Identity.Validate(); err != nil {
		return prefixError("identity", err)
	}
	if err := i.Scope.Validate(); err != nil {
		return prefixError("scope", err)
	}
	if i.Identity.Run != i.Scope.Run || i.Identity.Work != i.Scope.Work || i.Identity.JobID != i.Scope.JobID || i.Identity.AnalysisID != i.Scope.AnalysisID {
		return invalid("identity", "mismatch", "attempt identity does not match work scope")
	}
	if err := i.Worker.Validate(); err != nil {
		return prefixError("worker", err)
	}
	if err := i.Class.Validate(); err != nil {
		return prefixError("class", err)
	}
	if i.Class != i.Scope.Class {
		return invalid("class", "mismatch", "attempt class does not match work scope")
	}
	if err := i.Fence.Validate(); err != nil {
		return prefixError("fence", err)
	}
	if err := i.Reservation.Validate(); err != nil {
		return prefixError("reservation_id", err)
	}
	if i.RecordVersion == 0 {
		return invalid("record_version", "invalid_value", "record version must be positive")
	}
	if err := i.Stage.Validate(); err != nil {
		return err
	}
	if err := validateInstant("claim_time", i.ClaimTime); err != nil {
		return err
	}
	if err := validateInstant("heartbeat_time", i.HeartbeatTime); err != nil {
		return err
	}
	if err := validateInstant("expiry_time", i.ExpiryTime); err != nil {
		return err
	}
	if i.ClaimTime.After(i.HeartbeatTime) || !i.HeartbeatTime.Before(i.ExpiryTime) {
		return invalid("time", "inconsistent_value", "attempt heartbeat must be between claim and expiry")
	}
	if i.StartTime != nil {
		if err := validateInstant("start_time", *i.StartTime); err != nil {
			return err
		}
		if i.StartTime.Before(i.ClaimTime) {
			return invalid("start_time", "inconsistent_value", "attempt start precedes claim")
		}
	}
	if i.EndTime != nil {
		if i.StartTime == nil {
			return invalid("end_time", "inconsistent_value", "attempt end requires a start")
		}
		if err := validateInstant("end_time", *i.EndTime); err != nil {
			return err
		}
		if i.EndTime.Before(*i.StartTime) || i.EndTime.Before(i.HeartbeatTime) {
			return invalid("end_time", "inconsistent_value", "attempt end precedes an execution timestamp")
		}
	}
	if i.Stage == AttemptStageFinished && i.EndTime == nil {
		return invalid("end_time", "missing", "finished attempt requires an end time")
	}
	if i.Stage != AttemptStageFinished && i.EndTime != nil {
		return invalid("end_time", "inconsistent_value", "non-terminal attempt cannot have an end time")
	}
	return nil
}

func validateInstant(path string, value time.Time) error {
	if value.IsZero() {
		return invalid(path, "missing", "UTC instant is required")
	}
	return nil
}

type Attempt struct{ input AttemptInput }

func NewAttempt(input AttemptInput) (Attempt, error) {
	if err := input.Validate(); err != nil {
		return Attempt{}, err
	}
	return Attempt{input: cloneValue(input)}, nil
}

func (a Attempt) Input() AttemptInput        { return cloneValue(a.input) }
func (a Attempt) Identity() AttemptIdentity  { return a.input.Identity }
func (a Attempt) Scope() WorkScope           { return cloneValue(a.input.Scope) }
func (a Attempt) Worker() WorkerID           { return a.input.Worker }
func (a Attempt) Class() ExecutionClass      { return a.input.Class }
func (a Attempt) Fence() FenceToken          { return a.input.Fence }
func (a Attempt) Reservation() ReservationID { return a.input.Reservation }
func (a Attempt) Validate() error            { return a.input.Validate() }

func (a Attempt) ValidateAgainstScope(expected WorkScope) error {
	if err := a.Validate(); err != nil {
		return err
	}
	if err := expected.Validate(); err != nil {
		return prefixError("expected_scope", err)
	}
	if !workScopesEqual(a.input.Scope, expected) {
		return invalid("scope", "mismatch", "attempt scope does not match the expected work scope")
	}
	return nil
}

func workScopesEqual(left, right WorkScope) bool {
	leftBytes, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rightBytes, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return bytes.Equal(leftBytes, rightBytes)
}

func (a Attempt) MarshalJSON() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(a.input)
	if err != nil {
		return nil, err
	}
	if len(data) > maxAttemptBytes {
		return nil, invalid("attempt", "too_large", "attempt exceeds the supported size")
	}
	return data, nil
}

func (a *Attempt) UnmarshalJSON(data []byte) error {
	parsed, err := DecodeAttempt(data)
	if err != nil {
		return err
	}
	*a = parsed
	return nil
}

func EncodeAttempt(attempt Attempt) ([]byte, error) { return json.Marshal(attempt) }

func DecodeAttempt(data []byte) (Attempt, error) {
	if len(data) > maxAttemptBytes {
		return Attempt{}, invalid("attempt", "too_large", "attempt exceeds the supported size")
	}
	var input AttemptInput
	if err := decodeStrict(data, &input); err != nil {
		return Attempt{}, err
	}
	return NewAttempt(input)
}

type AttemptOwnershipBinding struct {
	Identity            AttemptIdentity
	Attempt             *Attempt
	Scope               *WorkScope
	Worker              WorkerID
	Class               ExecutionClass
	Fence               FenceToken
	Reservation         ReservationID
	RecordVersion       uint64
	CancellationVersion uint64
	Now                 time.Time
}

func (a Attempt) ValidateActive(binding AttemptOwnershipBinding) error {
	if err := a.Validate(); err != nil {
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
	if !a.Identity().Equal(binding.Identity) || a.Worker() != binding.Worker || a.Class() != binding.Class || a.Fence() != binding.Fence || a.Reservation() != binding.Reservation {
		return invalid("ownership", "mismatch", "active ownership binding does not match attempt")
	}
	if binding.Scope != nil {
		if err := a.ValidateAgainstScope(*binding.Scope); err != nil {
			return prefixError("ownership.scope", err)
		}
	}
	if a.input.RecordVersion != binding.RecordVersion || a.input.CancellationVersion != binding.CancellationVersion {
		return invalid("ownership", "stale", "active ownership versions do not match attempt")
	}
	if binding.Now.Before(a.input.ClaimTime) || !binding.Now.Before(a.input.ExpiryTime) {
		return invalid("ownership", "expired", "attempt ownership is not active at the supplied time")
	}
	return nil
}

func NextAttemptNumber(previous uint64) (uint64, error) {
	if previous == 0 {
		return 0, invalid("attempt_number", "invalid_value", "previous attempt number must be positive")
	}
	if previous == math.MaxUint64 {
		return 0, invalid("attempt_number", "overflow", "attempt number cannot advance")
	}
	return previous + 1, nil
}

func ValidateAttemptSequence(previous, current uint64) error {
	if previous == 0 || current == 0 {
		return invalid("attempt_number", "invalid_value", "attempt numbers must be positive")
	}
	if current <= previous {
		return invalid("attempt_number", "regression", "attempt number must increase")
	}
	return nil
}

type CapacityReservationState string

const (
	CapacityReservationReserved  CapacityReservationState = "reserved"
	CapacityReservationStarting  CapacityReservationState = "starting"
	CapacityReservationIdle      CapacityReservationState = "idle"
	CapacityReservationActive    CapacityReservationState = "active"
	CapacityReservationReleasing CapacityReservationState = "releasing"
	CapacityReservationReleased  CapacityReservationState = "released"
	CapacityReservationExpired   CapacityReservationState = "expired"
	CapacityReservationFailed    CapacityReservationState = "failed"
)

func (s CapacityReservationState) Validate() error {
	switch s {
	case CapacityReservationReserved, CapacityReservationStarting, CapacityReservationIdle,
		CapacityReservationActive, CapacityReservationReleasing, CapacityReservationReleased,
		CapacityReservationExpired, CapacityReservationFailed:
		return nil
	default:
		return invalid("state", "unsupported_value", "capacity reservation state is unsupported")
	}
}

type CapacityReservationInput struct {
	SchemaVersion       uint16                   `json:"schema_version"`
	ID                  ReservationID            `json:"reservation_id"`
	Repository          RepositoryBinding        `json:"repository"`
	Class               ExecutionClass           `json:"class"`
	Resource            ResourceProfileReference `json:"resource"`
	Controller          WorkerID                 `json:"controller"`
	RecordVersion       uint64                   `json:"record_version"`
	CancellationVersion uint64                   `json:"cancellation_version"`
	CreatedAt           time.Time                `json:"created_at"`
	HeartbeatAt         time.Time                `json:"heartbeat_at"`
	ExpiresAt           time.Time                `json:"expires_at"`
	State               CapacityReservationState `json:"state"`
	Worker              *WorkerID                `json:"worker,omitempty"`
	Attempt             *AttemptIdentity         `json:"attempt,omitempty"`
}

func (i CapacityReservationInput) Validate() error {
	if i.SchemaVersion != CapacityReservationSchemaVersion {
		return invalid("schema_version", "unsupported_value", "capacity reservation schema version is unsupported")
	}
	if err := i.ID.Validate(); err != nil {
		return prefixError("reservation_id", err)
	}
	if err := i.Repository.Validate(); err != nil {
		return prefixError("repository", err)
	}
	if err := i.Class.Validate(); err != nil {
		return prefixError("class", err)
	}
	if err := i.Resource.Validate(); err != nil {
		return prefixError("resource", err)
	}
	if err := i.Controller.Validate(); err != nil {
		return prefixError("controller", err)
	}
	if i.RecordVersion == 0 {
		return invalid("record_version", "invalid_value", "record version must be positive")
	}
	if err := i.State.Validate(); err != nil {
		return err
	}
	if err := validateInstant("created_at", i.CreatedAt); err != nil {
		return err
	}
	if err := validateInstant("heartbeat_at", i.HeartbeatAt); err != nil {
		return err
	}
	if err := validateInstant("expires_at", i.ExpiresAt); err != nil {
		return err
	}
	if i.CreatedAt.After(i.HeartbeatAt) || !i.HeartbeatAt.Before(i.ExpiresAt) {
		return invalid("time", "inconsistent_value", "reservation heartbeat must be between creation and expiry")
	}
	if i.Worker != nil {
		if err := i.Worker.Validate(); err != nil {
			return prefixError("worker", err)
		}
	}
	if i.Attempt != nil {
		if err := i.Attempt.Validate(); err != nil {
			return prefixError("attempt", err)
		}
	}
	if i.State == CapacityReservationActive && (i.Worker == nil || i.Attempt == nil) {
		return invalid("state", "inconsistent_value", "active reservation requires worker and attempt correlation")
	}
	return nil
}

type CapacityReservation struct{ input CapacityReservationInput }

func NewCapacityReservation(input CapacityReservationInput) (CapacityReservation, error) {
	if err := input.Validate(); err != nil {
		return CapacityReservation{}, err
	}
	return CapacityReservation{input: cloneValue(input)}, nil
}

func (r CapacityReservation) Input() CapacityReservationInput { return cloneValue(r.input) }
func (r CapacityReservation) ID() ReservationID               { return r.input.ID }
func (r CapacityReservation) State() CapacityReservationState { return r.input.State }
func (r CapacityReservation) Validate() error                 { return r.input.Validate() }

func (r CapacityReservation) ValidateAgainstAttempt(attempt Attempt) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if err := attempt.Validate(); err != nil {
		return prefixError("attempt", err)
	}
	if r.input.State != CapacityReservationActive {
		return nil
	}
	if r.input.Attempt == nil || r.input.Worker == nil || !r.input.Attempt.Equal(attempt.Identity()) || *r.input.Worker != attempt.Worker() || r.input.ID != attempt.Reservation() || r.input.Class != attempt.Class() || !r.input.Repository.Equal(attempt.Scope().Repository) || r.input.Resource != attempt.Scope().Resource {
		return invalid("attempt", "mismatch", "active reservation does not match attempt ownership")
	}
	return nil
}

func (r CapacityReservation) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(r.input)
	if err != nil {
		return nil, err
	}
	if len(data) > maxReservationBytes {
		return nil, invalid("reservation", "too_large", "capacity reservation exceeds the supported size")
	}
	return data, nil
}

func (r *CapacityReservation) UnmarshalJSON(data []byte) error {
	parsed, err := DecodeCapacityReservation(data)
	if err != nil {
		return err
	}
	*r = parsed
	return nil
}

func EncodeCapacityReservation(reservation CapacityReservation) ([]byte, error) {
	return json.Marshal(reservation)
}

func DecodeCapacityReservation(data []byte) (CapacityReservation, error) {
	if len(data) > maxReservationBytes {
		return CapacityReservation{}, invalid("reservation", "too_large", "capacity reservation exceeds the supported size")
	}
	var input CapacityReservationInput
	if err := decodeStrict(data, &input); err != nil {
		return CapacityReservation{}, err
	}
	return NewCapacityReservation(input)
}
