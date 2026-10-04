package domain

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testProvider() ProviderIdentity {
	return ProviderIdentity{Kind: "github", Integration: "integration-a"}
}

func testBinding() RepositoryBinding {
	provider := testProvider()
	return RepositoryBinding{
		RegisteredRepositoryID: "registered-repo",
		ProviderRepository: ProviderRepositoryIdentity{
			Provider: provider,
			ID:       "provider-repo",
		},
	}
}

func testCommit(hash string) CommitID {
	return CommitID{Format: ObjectFormatSHA1, Hash: hash}
}

func testReceipt() ReceiptEvidence {
	return ReceiptEvidence{
		ReceivedAt: time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC),
		Authentication: AuthenticationEvidence{
			Status:          AuthenticationVerified,
			VerifierVersion: "verifier-v1",
			Evidence:        EvidenceReference{ID: "auth-evidence", Version: "v1"},
			Fields: []FieldProvenance{{
				Field:  "delivery_id",
				Source: ProvenanceProviderPayload,
			}},
		},
	}
}

func testPushEvent(base CommitCandidate) RepositoryEvent {
	binding := testBinding()
	return RepositoryEvent{
		SchemaVersion:  RepositoryEventSchemaVersion,
		AdapterVersion: "adapter-v1",
		Provider:       binding.ProviderRepository.Provider,
		Binding:        binding,
		Delivery:       DeliveryProvenance{ID: "delivery-1", Source: ProvenanceProviderPayload},
		Kind:           EventKindPush,
		Action:         EventActionCreated,
		Receipt:        testReceipt(),
		Head:           testCommit("0123456789abcdef0123456789abcdef01234567"),
		Base:           base,
		Comparison:     "push-base",
		Push: &PushContext{
			Ref:     "refs/heads/main",
			Before:  NullCommitCandidate(),
			Created: true,
		},
	}
}

func testPullRequestEvent(source ProviderRepositoryIdentity) RepositoryEvent {
	binding := testBinding()
	return RepositoryEvent{
		SchemaVersion:  RepositoryEventSchemaVersion,
		AdapterVersion: "adapter-v1",
		Provider:       binding.ProviderRepository.Provider,
		Binding:        binding,
		Delivery:       DeliveryProvenance{ID: "delivery-pr-1", Source: ProvenanceProviderPayload},
		Kind:           EventKindPullRequest,
		Action:         EventActionSynchronize,
		Receipt:        testReceipt(),
		Head:           testCommit("abcdef0123456789abcdef0123456789abcdef01"),
		Base:           PresentCommitCandidate(testCommit("1234567890abcdef1234567890abcdef12345678")),
		Comparison:     "merge-base",
		PullRequest: &PullRequestContext{
			ID:        "pr-42",
			Number:    42,
			Source:    source,
			Target:    binding.ProviderRepository,
			SourceRef: "refs/heads/feature",
			TargetRef: "refs/heads/main",
		},
	}
}

func TestRepositoryEventRoundTripsPushAndPullRequestVariants(t *testing.T) {
	provider := testProvider()
	fork := ProviderRepositoryIdentity{Provider: provider, ID: "fork-repo"}
	cases := []struct {
		name  string
		event RepositoryEvent
	}{
		{name: "push", event: testPushEvent(AbsentCommitCandidate())},
		{name: "same repository pull request", event: testPullRequestEvent(testBinding().ProviderRepository)},
		{name: "fork pull request", event: testPullRequestEvent(fork)},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := EncodeRepositoryEvent(test.event)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			decoded, err := DecodeRepositoryEvent(encoded)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if !reflect.DeepEqual(test.event, decoded) {
				t.Fatalf("round trip changed semantic content:\nwant %#v\n got %#v", test.event, decoded)
			}
			if !decoded.DeliveryKey().Equal(test.event.DeliveryKey()) {
				t.Fatal("round trip changed delivery identity")
			}
		})
	}
}

func TestRepositoryEventPreservesCandidatePresence(t *testing.T) {
	cases := []CommitCandidate{
		AbsentCommitCandidate(),
		NullCommitCandidate(),
		PresentCommitCandidate(testCommit("0123456789abcdef0123456789abcdef01234567")),
	}
	for _, candidate := range cases {
		event := testPushEvent(candidate)
		encoded, err := EncodeRepositoryEvent(event)
		if err != nil {
			t.Fatalf("encode %q: %v", candidate.State, err)
		}
		decoded, err := DecodeRepositoryEvent(encoded)
		if err != nil {
			t.Fatalf("decode %q: %v", candidate.State, err)
		}
		if !candidate.Equal(decoded.Base) {
			t.Fatalf("candidate state collapsed for %q: got %#v", candidate.State, decoded.Base)
		}
	}
}

func TestRepositoryEventRejectsMissingContextAndBindingMismatches(t *testing.T) {
	missingHead := testPushEvent(AbsentCommitCandidate())
	missingHead.Head = CommitID{}
	if err := missingHead.Validate(); err == nil {
		t.Fatal("missing immutable head was accepted")
	}

	mixed := testPushEvent(AbsentCommitCandidate())
	mixed.PullRequest = &PullRequestContext{}
	if err := mixed.Validate(); err == nil {
		t.Fatal("mixed push and pull request variants were accepted")
	}

	missingPRContext := testPullRequestEvent(testBinding().ProviderRepository)
	missingPRContext.PullRequest.Source = ProviderRepositoryIdentity{}
	if err := missingPRContext.Validate(); err == nil {
		t.Fatal("pull request with missing source identity was accepted")
	}

	wrongBinding := testBinding()
	wrongBinding.RegisteredRepositoryID = "other-repo"
	if err := testPushEvent(AbsentCommitCandidate()).ValidateAgainst(wrongBinding); err == nil {
		t.Fatal("event with a different trusted target binding was accepted")
	}
}

func TestRepositoryEventStrictDecodeRejectsUnsupportedAndAmbiguousJSON(t *testing.T) {
	valid, err := EncodeRepositoryEvent(testPushEvent(AbsentCommitCandidate()))
	if err != nil {
		t.Fatal(err)
	}
	cases := []string{
		string(valid[:len(valid)-1]) + `,"fetch_url":"https://example.invalid"}`,
		string(valid[:len(valid)-1]) + `,"credential":"secret"}`,
		string(valid[:len(valid)-1]) + `,"signature":"secret"}`,
		string(valid[:len(valid)-1]) + `,"raw_body":"payload"}`,
		strings.Replace(string(valid), `"schema_version":1`, `"schema_version":99`, 1),
		strings.Replace(string(valid), `"delivery":{"id":"delivery-1"`, `"delivery":{"id":"delivery-1","id":"delivery-2"`, 1),
		strings.Replace(string(valid), `"delivery":{"id":"delivery-1"`, `"delivery":{"id":"delivery-1","ID":"delivery-2"`, 1),
		string(valid) + ` {"trailing":true}`,
		`{"schema_version":1}`,
	}
	for index, input := range cases {
		if _, err := DecodeRepositoryEvent([]byte(input)); err == nil {
			t.Fatalf("case %d was accepted", index)
		} else if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "example.invalid") {
			t.Fatalf("case %d echoed unsafe input: %v", index, err)
		}
	}

	var direct RepositoryEvent
	if err := json.Unmarshal(valid, &direct); err != nil {
		t.Fatalf("direct normalized unmarshal: %v", err)
	}
	if !bytes.Contains(valid, []byte(`"raw_body_digest"`)) && direct.Receipt.Authentication.RawBodyDigest != "" {
		t.Fatal("unexpected raw body evidence")
	}
}
