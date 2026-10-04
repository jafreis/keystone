package domain

import (
	"reflect"
	"testing"
)

func testConfiguration(withDigest bool) ConfigurationReference {
	configuration := ConfigurationReference{ID: "profile-default", Version: "v1"}
	if withDigest {
		configuration.Digest = "sha256-profile-v1"
	}
	return configuration
}

func testPolicy() PolicyReference {
	return PolicyReference{ID: "policy-ci", Version: "v3", Digest: "sha256-policy-v3"}
}

func testStrategy() ResolverStrategyReference {
	return ResolverStrategyReference{ID: "merge-base", Version: "v2"}
}

func testRevision(state RevisionState) RevisionContext {
	event := testPushEvent(NullCommitCandidate())
	revision := RevisionContext{
		SchemaVersion:       RevisionContextSchemaVersion,
		ID:                  "revision-1",
		Binding:             event.Binding,
		EventKind:           event.Kind,
		Head:                event.Head,
		Base:                event.Base,
		Comparison:          event.Comparison,
		Strategy:            testStrategy(),
		Configuration:       testConfiguration(state == RevisionStateResolved),
		Policy:              testPolicy(),
		ResolutionAttemptID: "attempt-1",
		State:               state,
	}
	switch state {
	case RevisionStatePending:
		revision.Pending = &PendingResolution{
			Reason:           "base_unavailable",
			RequiredEvidence: []EvidenceReference{{ID: "resolver-request", Version: "v1"}},
		}
	case RevisionStateFailed:
		revision.Failed = &FailedResolution{
			Code:           "object_unavailable",
			Classification: ResolutionRetryable,
			Evidence:       EvidenceReference{ID: "resolver-failure", Version: "v1"},
		}
	case RevisionStateResolved:
		base := testCommit("fedcba9876543210fedcba9876543210fedcba98")
		revision.Resolved = &ResolvedRevision{
			Base: base,
			Head: event.Head,
			BaseEvidence: testObjectEvidence(
				event.Binding.ProviderRepository,
				base,
				"base-object-evidence",
			),
			HeadEvidence: testObjectEvidence(
				event.Binding.ProviderRepository,
				event.Head,
				"head-object-evidence",
			),
		}
	}
	return revision
}

func testObjectEvidence(repository ProviderRepositoryIdentity, object CommitID, id EvidenceID) ObjectVerificationEvidence {
	return ObjectVerificationEvidence{
		Reference:  EvidenceReference{ID: id, Version: "v1"},
		Repository: repository,
		Object:     object,
		Strategy:   testStrategy(),
		Verified:   true,
	}
}

func TestRevisionContextRoundTripsPendingFailedAndResolvedStates(t *testing.T) {
	for _, state := range []RevisionState{RevisionStatePending, RevisionStateFailed, RevisionStateResolved} {
		t.Run(string(state), func(t *testing.T) {
			original := testRevision(state)
			encoded, err := EncodeRevisionContext(original)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			decoded, err := DecodeRevisionContext(encoded)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if !reflect.DeepEqual(original, decoded) {
				t.Fatalf("round trip changed state:\nwant %#v\n got %#v", original, decoded)
			}
		})
	}
}

func TestRevisionContextRequiresCompleteResolvedEvidence(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*RevisionContext)
	}{
		{name: "missing base", mutate: func(revision *RevisionContext) {
			revision.Resolved.Base = CommitID{}
		}},
		{name: "missing head", mutate: func(revision *RevisionContext) {
			revision.Resolved.Head = CommitID{}
		}},
		{name: "missing base evidence", mutate: func(revision *RevisionContext) {
			revision.Resolved.BaseEvidence = ObjectVerificationEvidence{}
		}},
		{name: "missing head evidence", mutate: func(revision *RevisionContext) {
			revision.Resolved.HeadEvidence = ObjectVerificationEvidence{}
		}},
		{name: "unverified object evidence", mutate: func(revision *RevisionContext) {
			revision.Resolved.BaseEvidence.Verified = false
		}},
		{name: "missing strategy", mutate: func(revision *RevisionContext) {
			revision.Strategy = ResolverStrategyReference{}
		}},
		{name: "missing profile digest", mutate: func(revision *RevisionContext) {
			revision.Configuration.Digest = ""
		}},
		{name: "missing policy", mutate: func(revision *RevisionContext) {
			revision.Policy = PolicyReference{}
		}},
		{name: "wrong head evidence object", mutate: func(revision *RevisionContext) {
			revision.Resolved.HeadEvidence.Object = testCommit("abcdefabcdefabcdefabcdefabcdefabcdefabcd")
		}},
		{name: "wrong head repository", mutate: func(revision *RevisionContext) {
			revision.Resolved.HeadEvidence.Repository.ID = "fork-repo"
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			revision := testRevision(RevisionStateResolved)
			test.mutate(&revision)
			if err := revision.Validate(); err == nil {
				t.Fatal("invalid resolved revision was accepted")
			}
		})
	}
}

func TestPendingAndFailedRevisionsRetainUnresolvedBase(t *testing.T) {
	for _, state := range []RevisionState{RevisionStatePending, RevisionStateFailed} {
		revision := testRevision(state)
		revision.Base = AbsentCommitCandidate()
		if err := revision.Validate(); err != nil {
			t.Fatalf("%s revision with absent candidate base rejected: %v", state, err)
		}
		if revision.Resolved != nil {
			t.Fatalf("%s revision unexpectedly contains resolved content", state)
		}
	}
}

func TestPendingResolutionDistinguishesEvidenceIdentityTuples(t *testing.T) {
	pending := PendingResolution{
		Reason: "base_unavailable",
		RequiredEvidence: []EvidenceReference{
			{ID: "resolver:request", Version: "v1"},
			{ID: "resolver", Version: "request:v1"},
		},
	}
	if err := pending.Validate(); err != nil {
		t.Fatalf("distinct evidence tuples rejected: %v", err)
	}

	pending.RequiredEvidence[1] = pending.RequiredEvidence[0]
	pending.RequiredEvidence[1].Digest = "different-digest"
	if err := pending.Validate(); err == nil {
		t.Fatal("duplicate evidence identity accepted with a different digest")
	}
}

func TestRevisionContextBindsToEventAndPullRequestSource(t *testing.T) {
	event := testPullRequestEvent(ProviderRepositoryIdentity{Provider: testProvider(), ID: "fork-repo"})
	revision := testRevision(RevisionStatePending)
	revision.Binding = event.Binding
	revision.EventKind = event.Kind
	source := event.PullRequest.Source
	revision.SourceRepository = &source
	revision.Head = event.Head
	revision.Base = event.Base
	revision.Comparison = event.Comparison
	if err := revision.ValidateAgainstEvent(event); err != nil {
		t.Fatalf("matching event rejected: %v", err)
	}

	revision.SourceRepository.ID = "other-fork"
	if err := revision.ValidateAgainstEvent(event); err == nil {
		t.Fatal("revision with a different PR source was accepted")
	}

	pushRevision := testRevision(RevisionStatePending)
	pushRevision.Head = testCommit("abcdefabcdefabcdefabcdefabcdefabcdefabcd")
	if err := pushRevision.ValidateAgainstEvent(testPushEvent(NullCommitCandidate())); err == nil {
		t.Fatal("revision with a different immutable head was accepted")
	}
}
