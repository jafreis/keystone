package domain

import "testing"

func TestConcreteLabelAndRepositoryPathBoundaries(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		valid bool
	}{
		{name: "root label", value: "//:all", valid: true},
		{name: "package label", value: "//services/api:binary", valid: true},
		{name: "external label", value: "@rules_oci//oci:tar", valid: true},
		{name: "recursive label", value: "//services/...", valid: false},
		{name: "negative label", value: "-//services:api", valid: false},
		{name: "empty package", value: "//:", valid: false},
		{name: "path traversal", value: "services/../api/main.tf", valid: false},
		{name: "absolute path", value: "/services/api/main.tf", valid: false},
		{name: "backslash path", value: "services\\api\\main.tf", valid: false},
		{name: "normalized path", value: "services/api/main.tf", valid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var err error
			if len(test.value) > 2 && (test.value[:2] == "//" || test.value[0] == '@') {
				err = ValidateConcreteLabel(test.value)
			} else {
				err = ValidateRepositoryPath(test.value)
			}
			if (err == nil) != test.valid {
				t.Fatalf("validation result=%v, want valid=%v", err, test.valid)
			}
		})
	}
}
