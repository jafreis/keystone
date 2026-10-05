// SPDX-License-Identifier: MPL-2.0
package domain

import (
	"bytes"
	"encoding/json"
	"time"
)

type EventKind string

const (
	EventKindPush        EventKind = "push"
	EventKindPullRequest EventKind = "pull_request"
)

func (k EventKind) Validate() error {
	switch k {
	case EventKindPush, EventKindPullRequest:
		return nil
	default:
		return invalid("kind", "unsupported_value", "event kind is unsupported")
	}
}

type EventAction string

const (
	EventActionCreated          EventAction = "created"
	EventActionUpdated          EventAction = "updated"
	EventActionDeleted          EventAction = "deleted"
	EventActionForced           EventAction = "forced"
	EventActionOpened           EventAction = "opened"
	EventActionReopened         EventAction = "reopened"
	EventActionSynchronize      EventAction = "synchronize"
	EventActionClosed           EventAction = "closed"
	EventActionReadyForReview   EventAction = "ready_for_review"
	EventActionConvertedToDraft EventAction = "converted_to_draft"
)

func (a EventAction) ValidateFor(kind EventKind) error {
	var supported bool
	switch kind {
	case EventKindPush:
		switch a {
		case EventActionCreated, EventActionUpdated, EventActionDeleted, EventActionForced:
			supported = true
		}
	case EventKindPullRequest:
		switch a {
		case EventActionOpened, EventActionReopened, EventActionSynchronize, EventActionClosed,
			EventActionUpdated, EventActionReadyForReview, EventActionConvertedToDraft:
			supported = true
		}
	default:
		return invalid("action", "unsupported_value", "event kind is unsupported")
	}
	if !supported {
		return invalid("action", "unsupported_value", "event action is unsupported for this event kind")
	}
	return nil
}

// ComparisonIntent is a normalized policy input. Its vocabulary is owned by
// the adapter/resolver decision, so the domain contract bounds it without
// treating a provider ref or URL as an intent.
type ComparisonIntent string

func (c ComparisonIntent) Validate() error {
	return validateID("comparison_intent", string(c))
}

type CandidateState string

const (
	CandidateAbsent  CandidateState = "absent"
	CandidateNull    CandidateState = "null"
	CandidatePresent CandidateState = "present"
)

// CommitCandidate preserves the distinction between an omitted candidate,
// provider-explicit null and a supplied commit object. Its object form keeps
// that distinction stable through normalized JSON serialization.
type CommitCandidate struct {
	State  CandidateState `json:"state"`
	Commit *CommitID      `json:"commit,omitempty"`
}

func AbsentCommitCandidate() CommitCandidate {
	return CommitCandidate{State: CandidateAbsent}
}

func NullCommitCandidate() CommitCandidate {
	return CommitCandidate{State: CandidateNull}
}

func PresentCommitCandidate(commit CommitID) CommitCandidate {
	return CommitCandidate{State: CandidatePresent, Commit: &commit}
}

func (c CommitCandidate) Validate() error {
	switch c.State {
	case CandidateAbsent, CandidateNull:
		if c.Commit != nil {
			return invalid("commit", "contradictory_value", "absent or null candidate cannot contain a commit")
		}
	case CandidatePresent:
		if c.Commit == nil {
			return invalid("commit", "missing", "present candidate requires a commit")
		}
		if err := c.Commit.Validate(); err != nil {
			return prefixError("commit", err)
		}
	default:
		return invalid("state", "unsupported_value", "candidate state is unsupported")
	}
	return nil
}

func (c CommitCandidate) Equal(other CommitCandidate) bool {
	if c.State != other.State {
		return false
	}
	if c.Commit == nil || other.Commit == nil {
		return c.Commit == nil && other.Commit == nil
	}
	return c.Commit.Equal(*other.Commit)
}

func (c CommitCandidate) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	type candidate CommitCandidate
	return json.Marshal(candidate(c))
}

func (c *CommitCandidate) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*c = NullCommitCandidate()
		return nil
	}
	type candidate CommitCandidate
	var value candidate
	if err := decodeStrict(data, &value); err != nil {
		return err
	}
	parsed := CommitCandidate(value)
	if err := parsed.Validate(); err != nil {
		return err
	}
	*c = parsed
	return nil
}

type DeliveryProvenance struct {
	ID     DeliveryID       `json:"id"`
	Source ProvenanceSource `json:"source"`
}

func (d DeliveryProvenance) Validate() error {
	if err := d.ID.Validate(); err != nil {
		return prefixError("id", err)
	}
	switch d.Source {
	case ProvenanceProviderPayload, ProvenanceAdapterDerived:
		return nil
	default:
		return invalid("source", "unsupported_value", "delivery identity cannot come from a privileged binding")
	}
}

type AuthenticationStatus string

const AuthenticationVerified AuthenticationStatus = "verified"

type AuthenticationEvidence struct {
	Status          AuthenticationStatus `json:"status"`
	VerifierVersion string               `json:"verifier_version"`
	Evidence        EvidenceReference    `json:"evidence"`
	Fields          []FieldProvenance    `json:"fields"`
	RawBodyDigest   ContentDigest        `json:"raw_body_digest,omitempty"`
}

func (a AuthenticationEvidence) Validate() error {
	if a.Status != AuthenticationVerified {
		return invalid("status", "unsupported_value", "normalized admission requires a verified authentication result")
	}
	if err := validateVersion("verifier_version", a.VerifierVersion); err != nil {
		return err
	}
	if err := a.Evidence.Validate(); err != nil {
		return prefixError("evidence", err)
	}
	if len(a.Fields) == 0 || len(a.Fields) > maxListEntries {
		return invalid("fields", "invalid_count", "authentication field provenance has an invalid count")
	}
	seen := make(map[string]struct{}, len(a.Fields))
	for index, field := range a.Fields {
		if err := field.Validate(); err != nil {
			return prefixError("fields", prefixError(indexPath(index), err))
		}
		if _, exists := seen[field.Field]; exists {
			return invalid("fields", "duplicate_value", "authentication field provenance contains a duplicate field")
		}
		seen[field.Field] = struct{}{}
	}
	if err := a.RawBodyDigest.Validate(); err != nil {
		return prefixError("raw_body_digest", err)
	}
	return nil
}

type ReceiptEvidence struct {
	ReceivedAt     time.Time              `json:"received_at"`
	Authentication AuthenticationEvidence `json:"authentication"`
}

func (r ReceiptEvidence) Validate() error {
	if r.ReceivedAt.IsZero() {
		return invalid("received_at", "missing", "receipt time is required")
	}
	if err := r.Authentication.Validate(); err != nil {
		return prefixError("authentication", err)
	}
	return nil
}

type ImmutableFact struct {
	Name       string           `json:"name"`
	Value      string           `json:"value"`
	Provenance ProvenanceSource `json:"provenance"`
}

func (f ImmutableFact) Validate() error {
	if err := validateID("name", f.Name); err != nil {
		return err
	}
	if err := validateID("value", f.Value); err != nil {
		return prefixError("value", err)
	}
	return f.Provenance.Validate()
}

type PushContext struct {
	Ref     string          `json:"ref"`
	Before  CommitCandidate `json:"before"`
	Created bool            `json:"created"`
	Deleted bool            `json:"deleted"`
	Forced  bool            `json:"forced"`
}

func (p PushContext) Validate() error {
	if err := validateRef("ref", p.Ref); err != nil {
		return err
	}
	if err := p.Before.Validate(); err != nil {
		return prefixError("before", err)
	}
	return nil
}

type PullRequestContext struct {
	ID        PullRequestID              `json:"id"`
	Number    uint64                     `json:"number"`
	Source    ProviderRepositoryIdentity `json:"source"`
	Target    ProviderRepositoryIdentity `json:"target"`
	SourceRef string                     `json:"source_ref"`
	TargetRef string                     `json:"target_ref"`
}

func (p PullRequestContext) Validate() error {
	if err := p.ID.Validate(); err != nil {
		return prefixError("id", err)
	}
	if p.Number == 0 {
		return invalid("number", "missing", "pull request number is required")
	}
	if err := p.Source.Validate(); err != nil {
		return prefixError("source", err)
	}
	if err := p.Target.Validate(); err != nil {
		return prefixError("target", err)
	}
	if err := validateRef("source_ref", p.SourceRef); err != nil {
		return err
	}
	return validateRef("target_ref", p.TargetRef)
}

type RepositoryEvent struct {
	SchemaVersion  uint16              `json:"schema_version"`
	AdapterVersion string              `json:"adapter_version"`
	Provider       ProviderIdentity    `json:"provider"`
	Binding        RepositoryBinding   `json:"binding"`
	Delivery       DeliveryProvenance  `json:"delivery"`
	Kind           EventKind           `json:"kind"`
	Action         EventAction         `json:"action"`
	Receipt        ReceiptEvidence     `json:"receipt"`
	Head           CommitID            `json:"head"`
	Base           CommitCandidate     `json:"base"`
	Comparison     ComparisonIntent    `json:"comparison"`
	Facts          []ImmutableFact     `json:"facts,omitempty"`
	Push           *PushContext        `json:"push,omitempty"`
	PullRequest    *PullRequestContext `json:"pull_request,omitempty"`
}

func indexPath(index int) string {
	// The index is generated locally and cannot contain request data.
	if index < 0 {
		return "item"
	}
	if index < 10 {
		return "item_" + string(rune('0'+index))
	}
	return "item"
}

func (e RepositoryEvent) Validate() error {
	if e.SchemaVersion != RepositoryEventSchemaVersion {
		return invalid("schema_version", "unsupported_value", "repository event schema version is unsupported")
	}
	if err := validateVersion("adapter_version", e.AdapterVersion); err != nil {
		return err
	}
	if err := e.Provider.Validate(); err != nil {
		return prefixError("provider", err)
	}
	if err := e.Binding.Validate(); err != nil {
		return prefixError("binding", err)
	}
	if !e.Binding.ProviderRepository.Provider.Equal(e.Provider) {
		return invalid("binding.provider_repository.provider", "inconsistent_value", "binding provider does not match event provider")
	}
	if err := e.Delivery.Validate(); err != nil {
		return prefixError("delivery", err)
	}
	if err := e.Kind.Validate(); err != nil {
		return err
	}
	if err := e.Action.ValidateFor(e.Kind); err != nil {
		return err
	}
	if err := e.Receipt.Validate(); err != nil {
		return prefixError("receipt", err)
	}
	if err := e.Head.Validate(); err != nil {
		return prefixError("head", err)
	}
	if err := e.Base.Validate(); err != nil {
		return prefixError("base", err)
	}
	if err := e.Comparison.Validate(); err != nil {
		return err
	}
	if len(e.Facts) > maxListEntries {
		return invalid("facts", "invalid_count", "immutable fact list exceeds the supported count")
	}
	seenFacts := make(map[string]struct{}, len(e.Facts))
	for index, fact := range e.Facts {
		if err := fact.Validate(); err != nil {
			return prefixError("facts", prefixError(indexPath(index), err))
		}
		if _, exists := seenFacts[fact.Name]; exists {
			return invalid("facts", "duplicate_value", "immutable fact list contains a duplicate name")
		}
		seenFacts[fact.Name] = struct{}{}
	}

	switch e.Kind {
	case EventKindPush:
		if e.Push == nil || e.PullRequest != nil {
			return invalid("push", "inconsistent_variant", "push event must contain only push context")
		}
		if err := e.Push.Validate(); err != nil {
			return prefixError("push", err)
		}
	case EventKindPullRequest:
		if e.PullRequest == nil || e.Push != nil {
			return invalid("pull_request", "inconsistent_variant", "pull request event must contain only pull request context")
		}
		if err := e.PullRequest.Validate(); err != nil {
			return prefixError("pull_request", err)
		}
		if !e.PullRequest.Source.Provider.Equal(e.Provider) || !e.PullRequest.Target.Provider.Equal(e.Provider) {
			return invalid("pull_request", "inconsistent_value", "pull request repositories use a different provider namespace")
		}
		if !e.PullRequest.Target.Equal(e.Binding.ProviderRepository) {
			return invalid("pull_request.target", "inconsistent_value", "pull request target does not match the normalized binding")
		}
	}
	return nil
}

// ValidateAgainst checks the normalized payload against a separately supplied
// trusted registration. It does not look up or authorize that registration.
func (e RepositoryEvent) ValidateAgainst(trusted RepositoryBinding) error {
	if err := e.Validate(); err != nil {
		return err
	}
	if err := trusted.Validate(); err != nil {
		return prefixError("trusted_binding", err)
	}
	if !e.Binding.Equal(trusted) {
		return invalid("binding", "mismatch", "event binding does not match the trusted registration")
	}
	return nil
}

func (e RepositoryEvent) DeliveryKey() DeliveryKey {
	return DeliveryKey{
		Provider:               e.Provider,
		RegisteredRepositoryID: e.Binding.RegisteredRepositoryID,
		DeliveryID:             e.Delivery.ID,
	}
}

func (e RepositoryEvent) MarshalJSON() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	type event RepositoryEvent
	return json.Marshal(event(e))
}

func (e *RepositoryEvent) UnmarshalJSON(data []byte) error {
	parsed, err := decodeRepositoryEvent(data)
	if err != nil {
		return err
	}
	*e = parsed
	return nil
}

func decodeRepositoryEvent(data []byte) (RepositoryEvent, error) {
	type event RepositoryEvent
	var value event
	if err := decodeStrict(data, &value); err != nil {
		return RepositoryEvent{}, err
	}
	parsed := RepositoryEvent(value)
	if err := parsed.Validate(); err != nil {
		return RepositoryEvent{}, err
	}
	return parsed, nil
}

func EncodeRepositoryEvent(event RepositoryEvent) ([]byte, error) {
	return json.Marshal(event)
}

func DecodeRepositoryEvent(data []byte) (RepositoryEvent, error) {
	return decodeRepositoryEvent(data)
}
