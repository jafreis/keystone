package domain

import (
	"encoding/json"
	"reflect"
)

type ConfigurationIntent struct {
	Profile ConfigurationReference   `json:"profile"`
	Matrix  []ConfigurationReference `json:"matrix"`
}

func (c ConfigurationIntent) Validate() error {
	if err := c.Profile.Validate(); err != nil {
		return prefixError("profile", err)
	}
	if len(c.Matrix) == 0 || len(c.Matrix) > maxListEntries {
		return invalid("matrix", "invalid_count", "configuration intent must contain a bounded matrix")
	}
	for index, entry := range c.Matrix {
		if err := entry.Validate(); err != nil {
			return prefixError("matrix", prefixError(indexPath(index), err))
		}
	}
	return nil
}

type ConfigurationSnapshot struct {
	Profile ConfigurationReference   `json:"profile"`
	Matrix  []ConfigurationReference `json:"matrix"`
}

func (c ConfigurationSnapshot) Validate() error {
	if err := c.Profile.ValidateWithDigest(); err != nil {
		return prefixError("profile", err)
	}
	if len(c.Matrix) == 0 || len(c.Matrix) > maxListEntries {
		return invalid("matrix", "invalid_count", "recorded configuration must contain a bounded matrix")
	}
	for index, entry := range c.Matrix {
		if err := entry.ValidateWithDigest(); err != nil {
			return prefixError("matrix", prefixError(indexPath(index), err))
		}
	}
	return nil
}

type RerunRequestKey struct {
	Authority              AuthorityReference     `json:"authority"`
	RegisteredRepositoryID RegisteredRepositoryID `json:"registered_repository_id"`
	RequestID              RerunRequestID         `json:"request_id"`
}

func (r RerunRequestKey) Validate() error {
	if err := r.Authority.Validate(); err != nil {
		return prefixError("authority", err)
	}
	if err := r.RegisteredRepositoryID.Validate(); err != nil {
		return prefixError("registered_repository_id", err)
	}
	if err := r.RequestID.Validate(); err != nil {
		return prefixError("request_id", err)
	}
	return nil
}

func (r RerunRequestKey) Equal(other RerunRequestKey) bool {
	return r.Authority == other.Authority &&
		r.RegisteredRepositoryID == other.RegisteredRepositoryID &&
		r.RequestID == other.RequestID
}

type OriginalDeliveryOrigin struct {
	Delivery DeliveryKey `json:"delivery"`
}

func (o OriginalDeliveryOrigin) Validate() error {
	return prefixError("delivery", o.Delivery.Validate())
}

type RerunOrigin struct {
	Request        RerunRequestKey `json:"request"`
	SourceRunID    RunID           `json:"source_run_id"`
	SourceDelivery DeliveryKey     `json:"source_delivery"`
}

func (o RerunOrigin) Validate() error {
	if err := o.Request.Validate(); err != nil {
		return prefixError("request", err)
	}
	if err := o.SourceRunID.Validate(); err != nil {
		return prefixError("source_run_id", err)
	}
	return prefixError("source_delivery", o.SourceDelivery.Validate())
}

type RunOriginKind string

const (
	RunOriginOriginalDelivery RunOriginKind = "original_delivery"
	RunOriginDeliberateRerun  RunOriginKind = "deliberate_rerun"
)

type RunOrigin struct {
	Kind     RunOriginKind           `json:"kind"`
	Original *OriginalDeliveryOrigin `json:"original,omitempty"`
	Rerun    *RerunOrigin            `json:"rerun,omitempty"`
}

func (o RunOrigin) Validate() error {
	switch o.Kind {
	case RunOriginOriginalDelivery:
		if o.Original == nil || o.Rerun != nil {
			return invalid("origin", "inconsistent_variant", "original origin must contain only original delivery data")
		}
		return prefixError("original", o.Original.Validate())
	case RunOriginDeliberateRerun:
		if o.Rerun == nil || o.Original != nil {
			return invalid("origin", "inconsistent_variant", "rerun origin must contain only rerun data")
		}
		return prefixError("rerun", o.Rerun.Validate())
	default:
		return invalid("kind", "unsupported_value", "run origin kind is unsupported")
	}
}

func (o RunOrigin) ValidateFor(runID RunID, eventKey DeliveryKey, repository RepositoryBinding) error {
	if err := o.Validate(); err != nil {
		return err
	}
	switch o.Kind {
	case RunOriginOriginalDelivery:
		if !o.Original.Delivery.Equal(eventKey) {
			return invalid("original.delivery", "mismatch", "original delivery does not match run event")
		}
	case RunOriginDeliberateRerun:
		rerun := o.Rerun
		if rerun.Request.RegisteredRepositoryID != repository.RegisteredRepositoryID {
			return invalid("rerun.request.registered_repository_id", "mismatch", "rerun request is scoped to another repository")
		}
		if rerun.SourceDelivery.RegisteredRepositoryID != repository.RegisteredRepositoryID {
			return invalid("rerun.source_delivery", "mismatch", "rerun source delivery is scoped to another repository")
		}
		if !rerun.SourceDelivery.Equal(eventKey) {
			return invalid("rerun.source_delivery", "mismatch", "rerun source delivery does not match run event")
		}
		if rerun.SourceRunID == runID {
			return invalid("rerun.source_run_id", "self_link", "rerun cannot link to itself")
		}
	}
	return nil
}

type LifecycleState string

const (
	LifecycleAdmitted            LifecycleState = "admitted"
	LifecyclePending             LifecycleState = "pending"
	LifecycleQueued              LifecycleState = "queued"
	LifecycleRunning             LifecycleState = "running"
	LifecycleSucceeded           LifecycleState = "succeeded"
	LifecycleFailed              LifecycleState = "failed"
	LifecycleCanceled            LifecycleState = "canceled"
	LifecycleNeedsReconciliation LifecycleState = "needs_reconciliation"
)

func (s LifecycleState) Validate() error {
	switch s {
	case LifecycleAdmitted, LifecyclePending, LifecycleQueued, LifecycleRunning,
		LifecycleSucceeded, LifecycleFailed, LifecycleCanceled, LifecycleNeedsReconciliation:
		return nil
	default:
		return invalid("lifecycle", "unsupported_value", "lifecycle state is unsupported")
	}
}

// TrustedRunBinding represents values supplied by a trusted caller or
// registration layer. It is an input to comparison, not an authority granted
// by the Run payload itself.
type TrustedRunBinding struct {
	Repository      RepositoryBinding
	ApprovedProfile ConfigurationReference
	Policy          PolicyReference
	Trust           TrustReference
	RerunAuthority  *AuthorityReference
}

func (b TrustedRunBinding) Validate() error {
	if err := b.Repository.Validate(); err != nil {
		return prefixError("repository", err)
	}
	if err := b.ApprovedProfile.ValidateWithDigest(); err != nil {
		return prefixError("approved_profile", err)
	}
	if err := b.Policy.Validate(); err != nil {
		return prefixError("policy", err)
	}
	if err := b.Trust.Validate(); err != nil {
		return prefixError("trust", err)
	}
	if b.RerunAuthority != nil {
		if err := b.RerunAuthority.Validate(); err != nil {
			return prefixError("rerun_authority", err)
		}
	}
	return nil
}

type Run struct {
	SchemaVersion       uint16                `json:"schema_version"`
	ID                  RunID                 `json:"run_id"`
	Repository          RepositoryBinding     `json:"repository"`
	EventKey            DeliveryKey           `json:"event_key"`
	EventKind           EventKind             `json:"event_kind"`
	Revision            RevisionContext       `json:"revision"`
	Requested           ConfigurationIntent   `json:"requested_configuration"`
	Approved            ConfigurationSnapshot `json:"approved_configuration"`
	Policy              PolicyReference       `json:"policy"`
	Trust               TrustReference        `json:"trust"`
	Origin              RunOrigin             `json:"origin"`
	Lifecycle           LifecycleState        `json:"lifecycle"`
	CancellationVersion uint64                `json:"cancellation_version"`
	Plan                *PlanReference        `json:"plan,omitempty"`
}

func referencesAgree(recordedID, recordedVersion string, recordedDigest ContentDigest, expectedID, expectedVersion string, expectedDigest ContentDigest) bool {
	if recordedID != expectedID || recordedVersion != expectedVersion {
		return false
	}
	return recordedDigest == "" || expectedDigest == "" || recordedDigest == expectedDigest
}

func (r Run) Validate() error {
	if r.SchemaVersion != RunSchemaVersion {
		return invalid("schema_version", "unsupported_value", "run schema version is unsupported")
	}
	if err := r.ID.Validate(); err != nil {
		return prefixError("run_id", err)
	}
	if err := r.Repository.Validate(); err != nil {
		return prefixError("repository", err)
	}
	if err := r.EventKey.Validate(); err != nil {
		return prefixError("event_key", err)
	}
	if r.EventKey.RegisteredRepositoryID != r.Repository.RegisteredRepositoryID ||
		!r.EventKey.Provider.Equal(r.Repository.ProviderRepository.Provider) {
		return invalid("event_key", "inconsistent_value", "run event key does not match repository binding")
	}
	if err := r.EventKind.Validate(); err != nil {
		return prefixError("event_kind", err)
	}
	if err := r.Revision.Validate(); err != nil {
		return prefixError("revision", err)
	}
	if !r.Revision.Binding.Equal(r.Repository) {
		return invalid("revision.binding", "mismatch", "run revision binding does not match run repository")
	}
	if r.Revision.EventKind != r.EventKind {
		return invalid("revision.event_kind", "mismatch", "run revision kind does not match run event kind")
	}
	if err := r.Requested.Validate(); err != nil {
		return prefixError("requested_configuration", err)
	}
	if err := r.Approved.Validate(); err != nil {
		return prefixError("approved_configuration", err)
	}
	if !referencesAgree(
		string(r.Revision.Configuration.ID),
		r.Revision.Configuration.Version,
		r.Revision.Configuration.Digest,
		string(r.Approved.Profile.ID),
		r.Approved.Profile.Version,
		r.Approved.Profile.Digest,
	) {
		return invalid("approved_configuration.profile", "mismatch", "approved profile does not match revision configuration")
	}
	if !referencesAgree(
		string(r.Revision.Policy.ID), r.Revision.Policy.Version, r.Revision.Policy.Digest,
		string(r.Policy.ID), r.Policy.Version, r.Policy.Digest,
	) {
		return invalid("policy", "mismatch", "run policy does not match revision policy")
	}
	if err := r.Policy.Validate(); err != nil {
		return prefixError("policy", err)
	}
	if err := r.Trust.Validate(); err != nil {
		return prefixError("trust", err)
	}
	if err := r.Origin.ValidateFor(r.ID, r.EventKey, r.Repository); err != nil {
		return prefixError("origin", err)
	}
	if err := r.Lifecycle.Validate(); err != nil {
		return err
	}
	if r.Plan != nil {
		if err := r.Plan.Validate(); err != nil {
			return prefixError("plan", err)
		}
	}
	return nil
}

// ValidateAgainst checks the Run against the separately supplied normalized
// event, revision and trusted decision. Optional source is required only for a
// deliberate rerun.
func (r Run) ValidateAgainst(event RepositoryEvent, revision RevisionContext, trusted TrustedRunBinding, source ...*Run) error {
	if len(source) > 1 {
		return invalid("source", "invalid_count", "run validation accepts at most one source run")
	}
	if err := r.Validate(); err != nil {
		return err
	}
	if err := event.Validate(); err != nil {
		return prefixError("event", err)
	}
	if err := revision.ValidateAgainstEvent(event); err != nil {
		return err
	}
	if !r.EventKey.Equal(event.DeliveryKey()) {
		return invalid("event_key", "mismatch", "run event key does not match event")
	}
	if !r.Repository.Equal(event.Binding) {
		return invalid("repository", "mismatch", "run repository does not match event binding")
	}
	if r.EventKind != event.Kind {
		return invalid("event_kind", "mismatch", "run event kind does not match event")
	}
	if !reflect.DeepEqual(r.Revision, revision) {
		return invalid("revision", "mismatch", "run revision differs from supplied revision")
	}
	if err := trusted.Validate(); err != nil {
		return prefixError("trusted_binding", err)
	}
	if !r.Repository.Equal(trusted.Repository) {
		return invalid("repository", "mismatch", "run repository does not match trusted binding")
	}
	if r.Approved.Profile != trusted.ApprovedProfile {
		return invalid("approved_configuration.profile", "mismatch", "run profile does not match trusted configuration")
	}
	if r.Policy != trusted.Policy {
		return invalid("policy", "mismatch", "run policy does not match trusted policy")
	}
	if r.Trust != trusted.Trust {
		return invalid("trust", "mismatch", "run trust reference does not match trusted decision")
	}

	if r.Origin.Kind == RunOriginOriginalDelivery {
		if len(source) != 0 {
			return invalid("source", "unexpected_value", "original delivery cannot include a source run")
		}
		return nil
	}
	if len(source) != 1 || source[0] == nil {
		return invalid("source", "missing", "rerun requires a source run")
	}
	prior := source[0]
	if err := prior.Validate(); err != nil {
		return prefixError("source", err)
	}
	rerun := r.Origin.Rerun
	if rerun.SourceRunID != prior.ID {
		return invalid("origin.rerun.source_run_id", "mismatch", "rerun source run does not match supplied source")
	}
	if prior.ID == r.ID {
		return invalid("run_id", "self_link", "rerun run cannot reuse source run identity")
	}
	if !prior.Repository.Equal(r.Repository) || !prior.EventKey.Equal(r.EventKey) {
		return invalid("origin.rerun", "mismatch", "rerun source is from another repository or delivery")
	}
	if !prior.Approved.Profile.Equal(r.Approved.Profile) {
		return invalid("approved_configuration.profile", "mismatch", "rerun changed the approved profile")
	}
	if !prior.Revision.Head.Equal(r.Revision.Head) {
		return invalid("revision.head", "mismatch", "rerun changed the immutable head")
	}
	if prior.Revision.State != RevisionStateResolved && r.Revision.State == RevisionStateResolved {
		return invalid("revision.state", "invalid_transition", "rerun cannot turn unresolved source context into resolved context")
	}
	if trusted.RerunAuthority == nil || rerun.Request.Authority != *trusted.RerunAuthority {
		return invalid("origin.rerun.request.authority", "mismatch", "rerun authority does not match trusted input")
	}
	return nil
}

func (r Run) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	type run Run
	return json.Marshal(run(r))
}

func (r *Run) UnmarshalJSON(data []byte) error {
	parsed, err := decodeRun(data)
	if err != nil {
		return err
	}
	*r = parsed
	return nil
}

func decodeRun(data []byte) (Run, error) {
	type run Run
	var value run
	if err := decodeStrict(data, &value); err != nil {
		return Run{}, err
	}
	parsed := Run(value)
	if err := parsed.Validate(); err != nil {
		return Run{}, err
	}
	return parsed, nil
}

func EncodeRun(run Run) ([]byte, error) {
	return json.Marshal(run)
}

func DecodeRun(data []byte) (Run, error) {
	return decodeRun(data)
}
