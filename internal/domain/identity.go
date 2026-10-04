package domain

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
)

const (
	// Each top-level contract owns its schema version. Keeping the constants
	// separate prevents a nested record from silently inheriting a different
	// contract's compatibility policy.
	RepositoryEventSchemaVersion uint16 = 1
	RevisionContextSchemaVersion uint16 = 1
	RunSchemaVersion             uint16 = 1

	FingerprintVersion = "keystone/fingerprint/v1"

	maxOpaqueIDLength = 256
	maxVersionLength  = 64
	maxRefLength      = 512
	maxListEntries    = 128
)

// ValidationError is deliberately small and value-oriented. Its message does
// not include input values, request bodies, signatures, credentials or URLs.
type ValidationError struct {
	Path    string
	Code    string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Path == "" {
		return e.Message
	}
	return e.Path + ": " + e.Message
}

func invalid(path, code, message string) error {
	return &ValidationError{Path: path, Code: code, Message: message}
}

func prefixError(prefix string, err error) error {
	if err == nil || prefix == "" {
		return err
	}
	var validation *ValidationError
	if errors.As(err, &validation) {
		copy := *validation
		if copy.Path == "" {
			copy.Path = prefix
		} else {
			copy.Path = prefix + "." + copy.Path
		}
		return &copy
	}
	return invalid(prefix, "invalid", "value is invalid")
}

func validateID(path, value string) error {
	if value == "" {
		return invalid(path, "missing", "identifier is required")
	}
	if len(value) > maxOpaqueIDLength {
		return invalid(path, "too_long", "identifier exceeds the supported length")
	}
	for index := 0; index < len(value); index++ {
		char := value[index]
		if char > 0x7f {
			return invalid(path, "invalid_identifier", "identifier must use the supported ASCII grammar")
		}
		isLetter := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
		isDigit := char >= '0' && char <= '9'
		if index == 0 {
			if !isLetter && !isDigit {
				return invalid(path, "invalid_identifier", "identifier has an invalid first character")
			}
			continue
		}
		if !isLetter && !isDigit && char != '.' && char != '_' && char != ':' && char != '-' {
			return invalid(path, "invalid_identifier", "identifier has an invalid character")
		}
	}
	return nil
}

func validateVersion(path, value string) error {
	if value == "" {
		return invalid(path, "missing", "version is required")
	}
	if len(value) > maxVersionLength {
		return invalid(path, "too_long", "version exceeds the supported length")
	}
	return validateID(path, value)
}

func validateOptionalDigest(path, value string) error {
	if value == "" {
		return nil
	}
	return validateID(path, value)
}

func validateRef(path, value string) error {
	if value == "" {
		return invalid(path, "missing", "ref is required")
	}
	if len(value) > maxRefLength {
		return invalid(path, "too_long", "ref exceeds the supported length")
	}
	if !strings.HasPrefix(value, "refs/") {
		return invalid(path, "invalid_ref", "ref must be a full refs/ name")
	}
	if strings.Contains(value, "//") || strings.Contains(value, "..") || strings.Contains(value, "@{") || strings.ContainsRune(value, '\\') {
		return invalid(path, "invalid_ref", "ref contains an unsupported sequence")
	}
	for _, char := range value {
		if char <= 0x20 || char == 0x7f {
			return invalid(path, "invalid_ref", "ref contains whitespace or a control character")
		}
	}
	return nil
}

// ProviderKind is intentionally open to registered provider adapters while
// retaining the bounded opaque-ID grammar.
type ProviderKind string

// IntegrationID identifies a configured provider instance, not merely a
// provider family such as GitHub or GitLab.
type IntegrationID string

type RegisteredRepositoryID string
type ProviderRepositoryID string
type DeliveryID string
type PullRequestID string
type RevisionID string
type RunID string
type RerunRequestID string
type AuthorityID string
type PolicyID string
type ConfigurationID string
type TrustID string
type StrategyID string
type EvidenceID string
type ResolutionAttemptID string
type PlanID string
type ContentDigest string

func (v ProviderKind) Validate() error  { return validateID("provider_kind", string(v)) }
func (v IntegrationID) Validate() error { return validateID("integration_id", string(v)) }
func (v RegisteredRepositoryID) Validate() error {
	return validateID("registered_repository_id", string(v))
}
func (v ProviderRepositoryID) Validate() error {
	return validateID("provider_repository_id", string(v))
}
func (v DeliveryID) Validate() error          { return validateID("delivery_id", string(v)) }
func (v PullRequestID) Validate() error       { return validateID("pull_request_id", string(v)) }
func (v RevisionID) Validate() error          { return validateID("revision_id", string(v)) }
func (v RunID) Validate() error               { return validateID("run_id", string(v)) }
func (v RerunRequestID) Validate() error      { return validateID("rerun_request_id", string(v)) }
func (v AuthorityID) Validate() error         { return validateID("authority_id", string(v)) }
func (v PolicyID) Validate() error            { return validateID("policy_id", string(v)) }
func (v ConfigurationID) Validate() error     { return validateID("configuration_id", string(v)) }
func (v TrustID) Validate() error             { return validateID("trust_id", string(v)) }
func (v StrategyID) Validate() error          { return validateID("strategy_id", string(v)) }
func (v EvidenceID) Validate() error          { return validateID("evidence_id", string(v)) }
func (v ResolutionAttemptID) Validate() error { return validateID("resolution_attempt_id", string(v)) }
func (v PlanID) Validate() error              { return validateID("plan_id", string(v)) }
func (v ContentDigest) Validate() error       { return validateOptionalDigest("digest", string(v)) }

type ProvenanceSource string

const (
	ProvenanceProviderPayload ProvenanceSource = "provider_payload"
	ProvenanceTrustedBinding  ProvenanceSource = "trusted_binding"
	ProvenanceAdapterDerived  ProvenanceSource = "adapter_derived"
)

func (p ProvenanceSource) Validate() error {
	switch p {
	case ProvenanceProviderPayload, ProvenanceTrustedBinding, ProvenanceAdapterDerived:
		return nil
	default:
		return invalid("provenance", "unsupported_value", "provenance source is unsupported")
	}
}

// FieldProvenance records where a normalized fact came from without retaining
// the raw request or an authentication secret.
type FieldProvenance struct {
	Field  string           `json:"field"`
	Source ProvenanceSource `json:"source"`
}

func (p FieldProvenance) Validate() error {
	if err := validateID("field", p.Field); err != nil {
		return err
	}
	return p.Source.Validate()
}

// ProviderIdentity is the namespace for provider-local identities.
type ProviderIdentity struct {
	Kind        ProviderKind  `json:"kind"`
	Integration IntegrationID `json:"integration"`
}

func (p ProviderIdentity) Validate() error {
	if err := p.Kind.Validate(); err != nil {
		return prefixError("kind", err)
	}
	if err := p.Integration.Validate(); err != nil {
		return prefixError("integration", err)
	}
	return nil
}

func (p ProviderIdentity) Equal(other ProviderIdentity) bool {
	return p.Kind == other.Kind && p.Integration == other.Integration
}

type ProviderRepositoryIdentity struct {
	Provider ProviderIdentity     `json:"provider"`
	ID       ProviderRepositoryID `json:"id"`
}

func (p ProviderRepositoryIdentity) Validate() error {
	if err := p.Provider.Validate(); err != nil {
		return prefixError("provider", err)
	}
	if err := p.ID.Validate(); err != nil {
		return prefixError("id", err)
	}
	return nil
}

func (p ProviderRepositoryIdentity) Equal(other ProviderRepositoryIdentity) bool {
	return p.Provider.Equal(other.Provider) && p.ID == other.ID
}

// RepositoryBinding is the target registration used for policy and trust. A
// provider payload may be compared against it, but cannot create or replace it.
type RepositoryBinding struct {
	RegisteredRepositoryID RegisteredRepositoryID     `json:"registered_repository_id"`
	ProviderRepository     ProviderRepositoryIdentity `json:"provider_repository"`
}

func (r RepositoryBinding) Validate() error {
	if err := r.RegisteredRepositoryID.Validate(); err != nil {
		return prefixError("registered_repository_id", err)
	}
	if err := r.ProviderRepository.Validate(); err != nil {
		return prefixError("provider_repository", err)
	}
	return nil
}

func (r RepositoryBinding) Equal(other RepositoryBinding) bool {
	return r.RegisteredRepositoryID == other.RegisteredRepositoryID && r.ProviderRepository.Equal(other.ProviderRepository)
}

type DeliveryKey struct {
	Provider               ProviderIdentity       `json:"provider"`
	RegisteredRepositoryID RegisteredRepositoryID `json:"registered_repository_id"`
	DeliveryID             DeliveryID             `json:"delivery_id"`
}

func (d DeliveryKey) Validate() error {
	if err := d.Provider.Validate(); err != nil {
		return prefixError("provider", err)
	}
	if err := d.RegisteredRepositoryID.Validate(); err != nil {
		return prefixError("registered_repository_id", err)
	}
	if err := d.DeliveryID.Validate(); err != nil {
		return prefixError("delivery_id", err)
	}
	return nil
}

func (d DeliveryKey) Equal(other DeliveryKey) bool {
	return d.Provider.Equal(other.Provider) &&
		d.RegisteredRepositoryID == other.RegisteredRepositoryID &&
		d.DeliveryID == other.DeliveryID
}

type CommitObjectFormat string

const (
	ObjectFormatSHA1   CommitObjectFormat = "sha1"
	ObjectFormatSHA256 CommitObjectFormat = "sha256"
)

// CommitID is an immutable object identity. It is never a branch, tag or
// abbreviated hash, and constructing one does not resolve anything in Git.
type CommitID struct {
	Format CommitObjectFormat `json:"format"`
	Hash   string             `json:"hash"`
}

func (c CommitID) Validate() error {
	var expectedLength int
	switch c.Format {
	case ObjectFormatSHA1:
		expectedLength = 40
	case ObjectFormatSHA256:
		expectedLength = 64
	default:
		return invalid("format", "unsupported_value", "commit object format is unsupported")
	}
	if len(c.Hash) != expectedLength {
		return invalid("hash", "invalid_commit", "commit hash has the wrong length")
	}
	decoded, err := hex.DecodeString(c.Hash)
	if err != nil || len(decoded) == 0 {
		return invalid("hash", "invalid_commit", "commit hash is not hexadecimal")
	}
	allZero := true
	for _, b := range decoded {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return invalid("hash", "invalid_commit", "all-zero hash is not a usable commit")
	}
	return nil
}

func (c CommitID) Equal(other CommitID) bool {
	return c.Format == other.Format && strings.EqualFold(c.Hash, other.Hash)
}

func (c CommitID) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	type commitID CommitID
	return json.Marshal(commitID(c))
}

func (c *CommitID) UnmarshalJSON(data []byte) error {
	type commitID CommitID
	var value commitID
	if err := decodeStrict(data, &value); err != nil {
		return err
	}
	if err := CommitID(value).Validate(); err != nil {
		return err
	}
	*c = CommitID(value)
	return nil
}

type EvidenceReference struct {
	ID      EvidenceID    `json:"id"`
	Version string        `json:"version"`
	Digest  ContentDigest `json:"digest,omitempty"`
}

func (r EvidenceReference) Validate() error {
	if err := r.ID.Validate(); err != nil {
		return prefixError("id", err)
	}
	if err := validateVersion("version", r.Version); err != nil {
		return err
	}
	if err := r.Digest.Validate(); err != nil {
		return prefixError("digest", err)
	}
	return nil
}

type PolicyReference struct {
	ID      PolicyID      `json:"id"`
	Version string        `json:"version"`
	Digest  ContentDigest `json:"digest,omitempty"`
}

func (r PolicyReference) Validate() error {
	if err := r.ID.Validate(); err != nil {
		return prefixError("id", err)
	}
	if err := validateVersion("version", r.Version); err != nil {
		return err
	}
	if err := r.Digest.Validate(); err != nil {
		return prefixError("digest", err)
	}
	return nil
}

type ConfigurationReference struct {
	ID      ConfigurationID `json:"id"`
	Version string          `json:"version"`
	Digest  ContentDigest   `json:"digest,omitempty"`
}

func (r ConfigurationReference) Validate() error {
	if err := r.ID.Validate(); err != nil {
		return prefixError("id", err)
	}
	if err := validateVersion("version", r.Version); err != nil {
		return err
	}
	if err := r.Digest.Validate(); err != nil {
		return prefixError("digest", err)
	}
	return nil
}

func (r ConfigurationReference) ValidateWithDigest() error {
	if err := r.Validate(); err != nil {
		return err
	}
	if r.Digest == "" {
		return invalid("digest", "missing", "configuration digest is required")
	}
	return nil
}

func (r ConfigurationReference) Equal(other ConfigurationReference) bool {
	return r.ID == other.ID && r.Version == other.Version && r.Digest == other.Digest
}

type TrustReference struct {
	ID      TrustID       `json:"id"`
	Version string        `json:"version"`
	Digest  ContentDigest `json:"digest,omitempty"`
}

func (r TrustReference) Validate() error {
	if err := r.ID.Validate(); err != nil {
		return prefixError("id", err)
	}
	if err := validateVersion("version", r.Version); err != nil {
		return err
	}
	if err := r.Digest.Validate(); err != nil {
		return prefixError("digest", err)
	}
	return nil
}

type ResolverStrategyReference struct {
	ID      StrategyID `json:"id"`
	Version string     `json:"version"`
}

func (r ResolverStrategyReference) Validate() error {
	if err := r.ID.Validate(); err != nil {
		return prefixError("id", err)
	}
	return validateVersion("version", r.Version)
}

func (r ResolverStrategyReference) Equal(other ResolverStrategyReference) bool {
	return r.ID == other.ID && r.Version == other.Version
}

type AuthorityReference struct {
	ID      AuthorityID `json:"id"`
	Version string      `json:"version"`
}

func (r AuthorityReference) Validate() error {
	if err := r.ID.Validate(); err != nil {
		return prefixError("id", err)
	}
	return validateVersion("version", r.Version)
}

type PlanReference struct {
	ID      PlanID        `json:"id"`
	Version string        `json:"version"`
	Digest  ContentDigest `json:"digest"`
}

func (r PlanReference) Validate() error {
	if err := r.ID.Validate(); err != nil {
		return prefixError("id", err)
	}
	if err := validateVersion("version", r.Version); err != nil {
		return err
	}
	if r.Digest == "" {
		return invalid("digest", "missing", "plan digest is required")
	}
	return r.Digest.Validate()
}

// decodeStrict performs a duplicate-key scan before DisallowUnknownFields so
// nested duplicate fields cannot be silently overwritten by encoding/json.
func decodeStrict(data []byte, destination any) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return invalid("", "invalid_json", "normalized JSON is empty")
	}
	structure := json.NewDecoder(bytes.NewReader(data))
	if err := scanJSONValue(structure); err != nil {
		return invalid("", "invalid_json", "normalized JSON is malformed or contains duplicate fields")
	}
	if _, err := structure.Token(); err != io.EOF {
		return invalid("", "invalid_json", "normalized JSON contains trailing data")
	}
	if err := validateExactJSONFields(data, reflect.TypeOf(destination)); err != nil {
		return invalid("", "invalid_json", "normalized JSON does not match the contract")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return invalid("", "invalid_json", "normalized JSON does not match the contract")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return invalid("", "invalid_json", "normalized JSON contains trailing data")
	}
	return nil
}

var jsonUnmarshalerType = reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()

// encoding/json matches struct field names case-insensitively. Normalized
// contracts use fixed field names, so perform an exact tag check before the
// standard decoder to reject aliases such as "ID" for "id".
func validateExactJSONFields(data []byte, valueType reflect.Type) error {
	if valueType == nil {
		return nil
	}
	if valueType.Kind() == reflect.Pointer {
		if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
			return nil
		}
		return validateExactJSONFields(data, valueType.Elem())
	}
	if valueType.Implements(jsonUnmarshalerType) || reflect.PointerTo(valueType).Implements(jsonUnmarshalerType) {
		return nil
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}

	switch valueType.Kind() {
	case reflect.Struct:
		fields := make(map[string]json.RawMessage)
		decoder := json.NewDecoder(bytes.NewReader(data))
		if err := decoder.Decode(&fields); err != nil {
			return err
		}
		allowed := make(map[string]reflect.Type, valueType.NumField())
		for index := 0; index < valueType.NumField(); index++ {
			field := valueType.Field(index)
			if field.PkgPath != "" {
				continue
			}
			name := field.Tag.Get("json")
			if name == "-" {
				continue
			}
			if comma := strings.IndexByte(name, ','); comma >= 0 {
				name = name[:comma]
			}
			if name == "" {
				name = field.Name
			}
			allowed[name] = field.Type
		}
		for name, raw := range fields {
			fieldType, ok := allowed[name]
			if !ok {
				return errors.New("unknown normalized field")
			}
			if err := validateExactJSONFields(raw, fieldType); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		var values []json.RawMessage
		decoder := json.NewDecoder(bytes.NewReader(data))
		if err := decoder.Decode(&values); err != nil {
			return err
		}
		for _, raw := range values {
			if err := validateExactJSONFields(raw, valueType.Elem()); err != nil {
				return err
			}
		}
	case reflect.Map, reflect.Interface:
		return nil
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok {
				return errors.New("invalid object key")
			}
			if _, exists := seen[name]; exists {
				return errors.New("duplicate object key")
			}
			seen[name] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return errors.New("invalid object")
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return errors.New("invalid array")
		}
	default:
		return errors.New("invalid delimiter")
	}
	return nil
}
