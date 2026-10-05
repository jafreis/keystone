package domain

import (
	"strings"
)

const maxInputValueLength = 512

func validateInputText(path, value string) error {
	if value == "" {
		return invalid(path, "missing", "value is required")
	}
	if len(value) > maxInputValueLength {
		return invalid(path, "too_long", "value exceeds the supported length")
	}
	for index := 0; index < len(value); index++ {
		if value[index] > 0x7f || value[index] <= 0x20 || value[index] == 0x7f {
			return invalid(path, "invalid_value", "value contains an unsupported character")
		}
	}
	return nil
}

func validatePathComponent(path, component string) error {
	if component == "" || component == "." || component == ".." {
		return invalid(path, "invalid_path", "path contains an empty or traversal component")
	}
	for index := 0; index < len(component); index++ {
		char := component[index]
		isLetter := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
		isDigit := char >= '0' && char <= '9'
		if !isLetter && !isDigit && char != '.' && char != '_' && char != '-' {
			return invalid(path, "invalid_path", "path contains an unsupported component")
		}
	}
	return nil
}

func validateLabelComponent(path, component string) error {
	if err := validatePathComponent(path, component); err != nil {
		return err
	}
	if strings.Contains(component, "...") {
		return invalid(path, "invalid_label", "label contains a recursive pattern")
	}
	return nil
}

// ValidateConcreteLabel validates the supported concrete-label subset used by
// typed plan inputs. It deliberately excludes recursive and negative patterns.
func ValidateConcreteLabel(value string) error {
	if err := validateInputText("label", value); err != nil {
		return err
	}
	if strings.HasPrefix(value, "-") {
		return invalid("label", "invalid_label", "negative labels are unsupported")
	}
	label := value
	if strings.HasPrefix(label, "@") {
		separator := strings.Index(label, "//")
		if separator <= 1 {
			return invalid("label", "invalid_label", "external label repository prefix is invalid")
		}
		if err := validateID("label", label[1:separator]); err != nil {
			return invalid("label", "invalid_label", "external label repository prefix is invalid")
		}
		label = label[separator:]
	}
	if !strings.HasPrefix(label, "//") {
		return invalid("label", "invalid_label", "label must be an absolute concrete label")
	}
	colon := strings.LastIndexByte(label, ':')
	if colon < 2 || colon == len(label)-1 || strings.IndexByte(label[colon+1:], ':') >= 0 {
		return invalid("label", "invalid_label", "label must contain an explicit target")
	}
	packageName := label[2:colon]
	if packageName != "" {
		for index, component := range strings.Split(packageName, "/") {
			if err := validateLabelComponent(indexPath(index), component); err != nil {
				return prefixError("label", err)
			}
		}
	}
	return validateLabelComponent("label", label[colon+1:])
}

// ValidateRepositoryPath validates a normalized repository-relative path.
func ValidateRepositoryPath(value string) error {
	if err := validateInputText("path", value); err != nil {
		return err
	}
	if strings.HasPrefix(value, "/") || strings.Contains(value, "\\") ||
		strings.Contains(value, "://") || strings.Contains(value, "//") ||
		(len(value) >= 2 && value[1] == ':') {
		return invalid("path", "invalid_path", "path must be repository-relative and normalized")
	}
	for index, component := range strings.Split(value, "/") {
		if err := validatePathComponent(indexPath(index), component); err != nil {
			return prefixError("path", err)
		}
	}
	return nil
}
