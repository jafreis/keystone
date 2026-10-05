// SPDX-License-Identifier: MPL-2.0
package domain

import (
	"reflect"
	"strings"
	"testing"
)

func testTrust() TrustReference {
	return TrustReference{ID: "trust-ci", Version: "v2", Digest: "sha256-trust-v2"}
}

func testRequestedConfiguration() ConfigurationIntent {
	return ConfigurationIntent{
		Profile: testConfiguration(false),
		Matrix: []ConfigurationReference{
			{ID: "linux-arm64", Version: "v1"},
			{ID: "linux-amd64", Version: "v1"},
		},
	}
}

func testApprovedConfiguration() ConfigurationSnapshot {
	return ConfigurationSnapshot{
		Profile: testConfiguration(true),
		Matrix: []ConfigurationReference{
			{ID: "linux-arm64", Version: "v1", Digest: "sha256-linux-arm64"},
			{ID: "linux-amd64", Version: "v1", Digest: "sha256-linux-amd64"},
		},
	}
}

func testOriginalRun(event RepositoryEvent, revision RevisionContext) Run {
	return Run{
		SchemaVersion: RunSchemaVersion,
		ID:            "run-original",
		Repository:    event.Binding,
		EventKey:      event.DeliveryKey(),
		EventKind:     event.Kind,
		Revision:      revision,
		Requested:     testRequestedConfiguration(),
		Approved:      testApprovedConfiguration(),
		Policy:        testPolicy(),
		Trust:         testTrust(),
		Origin: RunOrigin{
			Kind:     RunOriginOriginalDelivery,
			Original: &OriginalDeliveryOrigin{Delivery: event.DeliveryKey()},
		},
		Lifecycle: LifecycleAdmitted,
	}
}

func testTrustedRunBinding(event RepositoryEvent) TrustedRunBinding {
	authority := AuthorityReference{ID: "operator-1", Version: "v1"}
	return TrustedRunBinding{
		Repository:      event.Binding,
		ApprovedProfile: testApprovedConfiguration().Profile,
		ApprovedMatrix:  testApprovedConfiguration().Matrix,
		Policy:          testPolicy(),
		Trust:           testTrust(),
		RerunAuthority:  &authority,
	}
}

func TestRunRoundTripsOriginalAndDeliberateRerunOrigins(t *testing.T) {
	event := testPushEvent(NullCommitCandidate())
	revision := testRevision(RevisionStatePending)
	original := testOriginalRun(event, revision)
	trusted := testTrustedRunBinding(event)
	if err := original.ValidateAgainst(event, revision, trusted); err != nil {
		t.Fatalf("original run rejected: %v", err)
	}

	encoded, err := EncodeRun(original)
	if err != nil {
		t.Fatalf("encode original: %v", err)
	}
	decoded, err := DecodeRun(encoded)
	if err != nil {
		t.Fatalf("decode original: %v", err)
	}
	if !reflect.DeepEqual(original, decoded) {
		t.Fatalf("original round trip changed semantic content")
	}

	rerun := original
	rerun.ID = "run-rerun"
	rerun.Origin = RunOrigin{
		Kind: RunOriginDeliberateRerun,
		Rerun: &RerunOrigin{
			Request: RerunRequestKey{
				Authority:              *trusted.RerunAuthority,
				RegisteredRepositoryID: event.Binding.RegisteredRepositoryID,
				RequestID:              "rerun-request-1",
			},
			SourceRunID:    original.ID,
			SourceDelivery: event.DeliveryKey(),
		},
	}
	if err := rerun.ValidateAgainst(event, revision, trusted, &original); err != nil {
		t.Fatalf("rerun rejected: %v", err)
	}
	if rerun.ID == original.ID || !rerun.EventKey.Equal(original.EventKey) {
		t.Fatal("rerun must have a distinct run identity while retaining source delivery identity")
	}
	if rerun.Origin.Rerun.Request.Equal(original.Origin.RerunRequestKey()) {
		t.Fatal("original and rerun origins must remain distinguishable")
	}
}

func (o RunOrigin) RerunRequestKey() RerunRequestKey {
	if o.Rerun == nil {
		return RerunRequestKey{}
	}
	return o.Rerun.Request
}

func TestRunRejectsMalformedOriginsAndUntrustedIntentChanges(t *testing.T) {
	event := testPushEvent(NullCommitCandidate())
	revision := testRevision(RevisionStatePending)
	trusted := testTrustedRunBinding(event)
	original := testOriginalRun(event, revision)

	cases := []struct {
		name   string
		mutate func(*Run)
	}{
		{name: "missing rerun source", mutate: func(run *Run) {
			run.ID = "run-rerun"
			run.Origin = RunOrigin{Kind: RunOriginDeliberateRerun, Rerun: &RerunOrigin{}}
		}},
		{name: "self linked rerun", mutate: func(run *Run) {
			run.ID = "run-original"
			run.Origin = RunOrigin{
				Kind: RunOriginDeliberateRerun,
				Rerun: &RerunOrigin{
					Request:        RerunRequestKey{Authority: *trusted.RerunAuthority, RegisteredRepositoryID: event.Binding.RegisteredRepositoryID, RequestID: "rerun-request-1"},
					SourceRunID:    "run-original",
					SourceDelivery: event.DeliveryKey(),
				},
			}
		}},
		{name: "changed repository", mutate: func(run *Run) {
			run.Repository.RegisteredRepositoryID = "other-repo"
		}},
		{name: "changed approved profile", mutate: func(run *Run) {
			run.Approved.Profile.ID = "privileged-profile"
		}},
		{name: "changed head", mutate: func(run *Run) {
			run.Revision.Head = testCommit("abcdefabcdefabcdefabcdefabcdefabcdefabcd")
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			candidate := original
			test.mutate(&candidate)
			var source []*Run
			if candidate.Origin.Kind == RunOriginDeliberateRerun {
				source = []*Run{&original}
			}
			if err := candidate.ValidateAgainst(event, revision, trusted, source...); err == nil {
				t.Fatal("untrusted or malformed run was accepted")
			}
		})
	}

	if err := original.ValidateAgainst(event, revision, trusted, &original); err == nil {
		t.Fatal("original delivery accepted an unrelated source run")
	}
}

func TestRunRejectsApprovedConfigurationMatrixTampering(t *testing.T) {
	event := testPushEvent(NullCommitCandidate())
	revision := testRevision(RevisionStatePending)
	run := testOriginalRun(event, revision)
	run.Approved.Matrix[0] = ConfigurationReference{
		ID:      "unapproved-target",
		Version: "v99",
		Digest:  "sha256-attacker",
	}

	if err := run.ValidateAgainst(event, revision, testTrustedRunBinding(event)); err == nil {
		t.Fatal("run with a payload-controlled approved matrix entry was accepted")
	}
}

func TestRerunCannotChangeResolvedBaseUnderSameRevisionID(t *testing.T) {
	event := testPushEvent(NullCommitCandidate())
	prior := testOriginalRun(event, testRevision(RevisionStateResolved))
	trusted := testTrustedRunBinding(event)
	candidate := prior
	candidate.ID = "run-rerun"
	candidate.Revision = testRevision(RevisionStateResolved)
	changedBase := testCommit("abcdefabcdefabcdefabcdefabcdefabcdefabcd")
	candidate.Revision.Resolved.Base = changedBase
	candidate.Revision.Resolved.BaseEvidence = testObjectEvidence(
		event.Binding.ProviderRepository,
		changedBase,
		"replacement-base-evidence",
	)
	candidate.Origin = RunOrigin{
		Kind: RunOriginDeliberateRerun,
		Rerun: &RerunOrigin{
			Request: RerunRequestKey{
				Authority:              *trusted.RerunAuthority,
				RegisteredRepositoryID: event.Binding.RegisteredRepositoryID,
				RequestID:              "rerun-request-base-change",
			},
			SourceRunID:    prior.ID,
			SourceDelivery: event.DeliveryKey(),
		},
	}

	if err := candidate.ValidateAgainst(event, candidate.Revision, trusted, &prior); err == nil {
		t.Fatal("resolved rerun changed its source base under the same revision identity")
	}
}

func TestRerunCannotPromotePendingSourceToResolved(t *testing.T) {
	event := testPushEvent(NullCommitCandidate())
	pending := testRevision(RevisionStatePending)
	original := testOriginalRun(event, pending)
	trusted := testTrustedRunBinding(event)
	resolved := testRevision(RevisionStateResolved)
	rerun := original
	rerun.ID = "run-rerun"
	rerun.Revision = resolved
	rerun.Origin = RunOrigin{
		Kind: RunOriginDeliberateRerun,
		Rerun: &RerunOrigin{
			Request:        RerunRequestKey{Authority: *trusted.RerunAuthority, RegisteredRepositoryID: event.Binding.RegisteredRepositoryID, RequestID: "rerun-request-2"},
			SourceRunID:    original.ID,
			SourceDelivery: event.DeliveryKey(),
		},
	}
	if err := rerun.ValidateAgainst(event, resolved, trusted, &original); err == nil {
		t.Fatal("rerun promoted pending source to resolved context")
	}
}

func TestCrossContractValidationKeepsForkAndTrustedAuthoritySeparate(t *testing.T) {
	fork := ProviderRepositoryIdentity{Provider: testProvider(), ID: "fork-repo"}
	event := testPullRequestEvent(fork)
	revision := testRevision(RevisionStatePending)
	revision.Binding = event.Binding
	revision.EventKind = event.Kind
	source := event.PullRequest.Source
	revision.SourceRepository = &source
	revision.Head = event.Head
	revision.Base = event.Base
	revision.Comparison = event.Comparison
	run := testOriginalRun(event, revision)
	trusted := testTrustedRunBinding(event)
	if err := run.ValidateAgainst(event, revision, trusted); err != nil {
		t.Fatalf("fork event chain rejected: %v", err)
	}
	if run.Repository.ProviderRepository.ID == fork.ID {
		t.Fatal("fork source was promoted to the registered target repository")
	}

	cases := []struct {
		name   string
		mutate func(*Run)
	}{
		{name: "policy reference", mutate: func(candidate *Run) {
			candidate.Policy.ID = "untrusted-policy"
		}},
		{name: "trust reference", mutate: func(candidate *Run) {
			candidate.Trust.ID = "untrusted-trust"
		}},
		{name: "profile reference", mutate: func(candidate *Run) {
			candidate.Approved.Profile.Digest = "sha256-attacker"
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			candidate := run
			test.mutate(&candidate)
			if err := candidate.ValidateAgainst(event, revision, trusted); err == nil {
				t.Fatal("payload-controlled trusted reference was accepted")
			}
		})
	}
}

func TestRunStrictDecodeRejectsNestedSchemaAndTrailingData(t *testing.T) {
	event := testPushEvent(NullCommitCandidate())
	run := testOriginalRun(event, testRevision(RevisionStatePending))
	encoded, err := EncodeRun(run)
	if err != nil {
		t.Fatal(err)
	}
	modified := strings.Replace(string(encoded), `"schema_version":1,"revision_id":"revision-1"`, `"schema_version":99,"revision_id":"revision-1"`, 1)
	if modified == string(encoded) {
		t.Fatal("revision schema fixture did not change")
	}
	if _, err := DecodeRun([]byte(modified)); err == nil {
		t.Fatal("unsupported nested revision schema was accepted")
	}
	if _, err := DecodeRun(append(encoded, []byte(` {"trailing":true}`)...)); err == nil {
		t.Fatal("trailing JSON was accepted")
	}
}
