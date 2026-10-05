// SPDX-License-Identifier: MPL-2.0
package domain

import "testing"

func TestTypedIdentityValidation(t *testing.T) {
	valid := "repo-1"
	cases := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "valid", value: valid, valid: true},
		{name: "empty", value: "", valid: false},
		{name: "whitespace", value: "repo name", valid: false},
		{name: "control", value: "repo\nname", valid: false},
		{name: "leading punctuation", value: "-repo", valid: false},
		{name: "slash", value: "repo/name", valid: false},
		{name: "oversized", value: string(make([]byte, maxOpaqueIDLength+1)), valid: false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := RegisteredRepositoryID(test.value).Validate() == nil
			if got != test.valid {
				t.Fatalf("valid=%v, want %v", got, test.valid)
			}
		})
	}
}

func TestDeliveryKeyKeepsProviderAndRepositoryNamespaces(t *testing.T) {
	first := DeliveryKey{
		Provider:               ProviderIdentity{Kind: "github", Integration: "integration-a"},
		RegisteredRepositoryID: "repo-a",
		DeliveryID:             "delivery-1",
	}
	second := first
	second.Provider.Integration = "integration-b"
	third := first
	third.RegisteredRepositoryID = "repo-b"

	for name, key := range map[string]DeliveryKey{"first": first, "second": second, "third": third} {
		if err := key.Validate(); err != nil {
			t.Fatalf("%s key should validate: %v", name, err)
		}
	}
	if first.Equal(second) || first.Equal(third) {
		t.Fatal("provider and registered repository namespaces must remain distinct")
	}
	if !first.Equal(first) {
		t.Fatal("a delivery key must equal itself")
	}
}

func TestCommitIDRequiresAFullNonZeroObject(t *testing.T) {
	valid := CommitID{Format: ObjectFormatSHA1, Hash: "0123456789abcdef0123456789abcdef01234567"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid commit rejected: %v", err)
	}

	cases := []CommitID{
		{Format: ObjectFormatSHA1, Hash: "0123456789abcdef"},
		{Format: ObjectFormatSHA256, Hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"[:40]},
		{Format: ObjectFormatSHA1, Hash: "0000000000000000000000000000000000000000"},
		{Format: ObjectFormatSHA1, Hash: "0123456789abcdef0123456789abcdef0123456z"},
	}
	for _, candidate := range cases {
		if err := candidate.Validate(); err == nil {
			t.Fatalf("invalid commit accepted: %#v", candidate)
		}
	}
}
