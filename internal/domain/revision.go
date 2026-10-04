package domain

import "encoding/json"

type RevisionState string

const (
	RevisionStatePending  RevisionState = "pending"
	RevisionStateFailed   RevisionState = "failed"
	RevisionStateResolved RevisionState = "resolved"
)

func (s RevisionState) Validate() error {
	switch s {
	case RevisionStatePending, RevisionStateFailed, RevisionStateResolved:
		return nil
	default:
		return invalid("state", "unsupported_value", "revision state is unsupported")
	}
}

type ResolutionClassification string

const (
	ResolutionRetryable           ResolutionClassification = "retryable"
	ResolutionTerminal            ResolutionClassification = "terminal"
	ResolutionNeedsReconciliation ResolutionClassification = "needs_reconciliation"
)

func (c ResolutionClassification) Validate() error {
	switch c {
	case ResolutionRetryable, ResolutionTerminal, ResolutionNeedsReconciliation:
		return nil
	default:
		return invalid("classification", "unsupported_value", "resolution classification is unsupported")
	}
}

type PendingResolution struct {
	Reason           string              `json:"reason"`
	RequiredEvidence []EvidenceReference `json:"required_evidence"`
}

func (p PendingResolution) Validate() error {
	if err := validateID("reason", p.Reason); err != nil {
		return err
	}
	if len(p.RequiredEvidence) == 0 || len(p.RequiredEvidence) > maxListEntries {
		return invalid("required_evidence", "invalid_count", "pending resolution must name required evidence")
	}
	seen := make(map[string]struct{}, len(p.RequiredEvidence))
	for index, evidence := range p.RequiredEvidence {
		if err := evidence.Validate(); err != nil {
			return prefixError("required_evidence", prefixError(indexPath(index), err))
		}
		key := string(evidence.ID) + ":" + evidence.Version
		if _, exists := seen[key]; exists {
			return invalid("required_evidence", "duplicate_value", "pending resolution contains duplicate evidence")
		}
		seen[key] = struct{}{}
	}
	return nil
}

type FailedResolution struct {
	Code           string                   `json:"code"`
	Classification ResolutionClassification `json:"classification"`
	Evidence       EvidenceReference        `json:"evidence"`
}

func (f FailedResolution) Validate() error {
	if err := validateID("code", f.Code); err != nil {
		return err
	}
	if err := f.Classification.Validate(); err != nil {
		return err
	}
	return prefixError("evidence", f.Evidence.Validate())
}

type ObjectVerificationEvidence struct {
	Reference  EvidenceReference          `json:"reference"`
	Repository ProviderRepositoryIdentity `json:"repository"`
	Object     CommitID                   `json:"object"`
	Strategy   ResolverStrategyReference  `json:"strategy"`
	Verified   bool                       `json:"verified"`
}

func (e ObjectVerificationEvidence) Validate() error {
	if err := e.Reference.Validate(); err != nil {
		return prefixError("reference", err)
	}
	if err := e.Repository.Validate(); err != nil {
		return prefixError("repository", err)
	}
	if err := e.Object.Validate(); err != nil {
		return prefixError("object", err)
	}
	if err := e.Strategy.Validate(); err != nil {
		return prefixError("strategy", err)
	}
	if !e.Verified {
		return invalid("verified", "missing", "object evidence must record a verified result")
	}
	return nil
}

type ResolvedRevision struct {
	Base         CommitID                   `json:"base"`
	Head         CommitID                   `json:"head"`
	BaseEvidence ObjectVerificationEvidence `json:"base_evidence"`
	HeadEvidence ObjectVerificationEvidence `json:"head_evidence"`
}

func (r ResolvedRevision) Validate() error {
	if err := r.Base.Validate(); err != nil {
		return prefixError("base", err)
	}
	if err := r.Head.Validate(); err != nil {
		return prefixError("head", err)
	}
	if err := r.BaseEvidence.Validate(); err != nil {
		return prefixError("base_evidence", err)
	}
	return prefixError("head_evidence", r.HeadEvidence.Validate())
}

type RevisionContext struct {
	SchemaVersion       uint16                      `json:"schema_version"`
	ID                  RevisionID                  `json:"revision_id"`
	Binding             RepositoryBinding           `json:"binding"`
	EventKind           EventKind                   `json:"event_kind"`
	SourceRepository    *ProviderRepositoryIdentity `json:"source_repository,omitempty"`
	Head                CommitID                    `json:"head"`
	Base                CommitCandidate             `json:"base"`
	Comparison          ComparisonIntent            `json:"comparison"`
	Strategy            ResolverStrategyReference   `json:"strategy"`
	Configuration       ConfigurationReference      `json:"configuration"`
	Policy              PolicyReference             `json:"policy"`
	ResolutionAttemptID ResolutionAttemptID         `json:"resolution_attempt_id"`
	State               RevisionState               `json:"state"`
	Pending             *PendingResolution          `json:"pending,omitempty"`
	Failed              *FailedResolution           `json:"failed,omitempty"`
	Resolved            *ResolvedRevision           `json:"resolved,omitempty"`
}

func (r RevisionContext) Validate() error {
	if r.SchemaVersion != RevisionContextSchemaVersion {
		return invalid("schema_version", "unsupported_value", "revision context schema version is unsupported")
	}
	if err := r.ID.Validate(); err != nil {
		return prefixError("revision_id", err)
	}
	if err := r.Binding.Validate(); err != nil {
		return prefixError("binding", err)
	}
	if err := r.EventKind.Validate(); err != nil {
		return prefixError("event_kind", err)
	}
	if r.EventKind == EventKindPullRequest {
		if r.SourceRepository == nil {
			return invalid("source_repository", "missing", "pull request revision requires a source repository")
		}
		if err := r.SourceRepository.Validate(); err != nil {
			return prefixError("source_repository", err)
		}
		if !r.SourceRepository.Provider.Equal(r.Binding.ProviderRepository.Provider) {
			return invalid("source_repository.provider", "inconsistent_value", "source repository uses a different provider namespace")
		}
	} else if r.SourceRepository != nil {
		return invalid("source_repository", "inconsistent_value", "push revision cannot contain a source repository")
	}
	if err := r.Head.Validate(); err != nil {
		return prefixError("head", err)
	}
	if err := r.Base.Validate(); err != nil {
		return prefixError("base", err)
	}
	if err := r.Comparison.Validate(); err != nil {
		return prefixError("comparison", err)
	}
	if err := r.Strategy.Validate(); err != nil {
		return prefixError("strategy", err)
	}
	if err := r.Configuration.Validate(); err != nil {
		return prefixError("configuration", err)
	}
	if err := r.Policy.Validate(); err != nil {
		return prefixError("policy", err)
	}
	if err := r.ResolutionAttemptID.Validate(); err != nil {
		return prefixError("resolution_attempt_id", err)
	}
	if err := r.State.Validate(); err != nil {
		return err
	}

	variants := 0
	if r.Pending != nil {
		variants++
	}
	if r.Failed != nil {
		variants++
	}
	if r.Resolved != nil {
		variants++
	}
	if variants != 1 {
		return invalid("state", "inconsistent_variant", "revision context must contain exactly one state payload")
	}
	switch r.State {
	case RevisionStatePending:
		if r.Pending == nil || r.Failed != nil || r.Resolved != nil {
			return invalid("pending", "inconsistent_variant", "pending revision has the wrong state payload")
		}
		return prefixError("pending", r.Pending.Validate())
	case RevisionStateFailed:
		if r.Failed == nil || r.Pending != nil || r.Resolved != nil {
			return invalid("failed", "inconsistent_variant", "failed revision has the wrong state payload")
		}
		return prefixError("failed", r.Failed.Validate())
	case RevisionStateResolved:
		if r.Resolved == nil || r.Pending != nil || r.Failed != nil {
			return invalid("resolved", "inconsistent_variant", "resolved revision has the wrong state payload")
		}
		if err := r.Configuration.ValidateWithDigest(); err != nil {
			return prefixError("configuration", err)
		}
		return r.validateResolved()
	default:
		return invalid("state", "unsupported_value", "revision state is unsupported")
	}
}

func (r RevisionContext) validateResolved() error {
	resolved := r.Resolved
	if err := resolved.Validate(); err != nil {
		return prefixError("resolved", err)
	}
	if !resolved.Head.Equal(r.Head) {
		return invalid("resolved.head", "mismatch", "resolved head does not match the admitted head")
	}
	if !resolved.HeadEvidence.Object.Equal(resolved.Head) {
		return invalid("resolved.head_evidence.object", "mismatch", "head evidence does not match resolved head")
	}
	if !resolved.BaseEvidence.Object.Equal(resolved.Base) {
		return invalid("resolved.base_evidence.object", "mismatch", "base evidence does not match resolved base")
	}
	if !resolved.HeadEvidence.Strategy.Equal(r.Strategy) || !resolved.BaseEvidence.Strategy.Equal(r.Strategy) {
		return invalid("resolved", "mismatch", "object evidence uses a different resolver strategy")
	}
	expectedHeadRepository := r.Binding.ProviderRepository
	if r.SourceRepository != nil {
		expectedHeadRepository = *r.SourceRepository
	}
	if !resolved.HeadEvidence.Repository.Equal(expectedHeadRepository) {
		return invalid("resolved.head_evidence.repository", "mismatch", "head evidence repository does not match revision source")
	}
	if !resolved.BaseEvidence.Repository.Equal(r.Binding.ProviderRepository) {
		return invalid("resolved.base_evidence.repository", "mismatch", "base evidence repository does not match revision target")
	}
	return nil
}

// ValidateAgainstEvent checks that the revision did not change the normalized
// event's identity or candidate context while moving through resolution.
func (r RevisionContext) ValidateAgainstEvent(event RepositoryEvent) error {
	if err := event.Validate(); err != nil {
		return prefixError("event", err)
	}
	if err := r.Validate(); err != nil {
		return err
	}
	if !r.Binding.Equal(event.Binding) {
		return invalid("binding", "mismatch", "revision binding does not match event binding")
	}
	if r.EventKind != event.Kind {
		return invalid("event_kind", "mismatch", "revision event kind does not match event")
	}
	if !r.Head.Equal(event.Head) {
		return invalid("head", "mismatch", "revision head does not match event head")
	}
	if !r.Base.Equal(event.Base) {
		return invalid("base", "mismatch", "revision base candidate does not match event")
	}
	if r.Comparison != event.Comparison {
		return invalid("comparison", "mismatch", "revision comparison intent does not match event")
	}
	if event.PullRequest != nil {
		if r.SourceRepository == nil || !r.SourceRepository.Equal(event.PullRequest.Source) {
			return invalid("source_repository", "mismatch", "revision source does not match event source")
		}
	}
	return nil
}

func (r RevisionContext) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	type revision RevisionContext
	return json.Marshal(revision(r))
}

func (r *RevisionContext) UnmarshalJSON(data []byte) error {
	parsed, err := decodeRevisionContext(data)
	if err != nil {
		return err
	}
	*r = parsed
	return nil
}

func decodeRevisionContext(data []byte) (RevisionContext, error) {
	type revision RevisionContext
	var value revision
	if err := decodeStrict(data, &value); err != nil {
		return RevisionContext{}, err
	}
	parsed := RevisionContext(value)
	if err := parsed.Validate(); err != nil {
		return RevisionContext{}, err
	}
	return parsed, nil
}

func EncodeRevisionContext(revision RevisionContext) ([]byte, error) {
	return json.Marshal(revision)
}

func DecodeRevisionContext(data []byte) (RevisionContext, error) {
	return decodeRevisionContext(data)
}
