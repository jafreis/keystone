package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// canonicalEncoder is deliberately narrow and explicit. It is used only by
// the domain projections below; it is not a reflection or JSON hash format.
type canonicalEncoder struct {
	bytes.Buffer
}

func newCanonicalEncoder(domain, version string) *canonicalEncoder {
	encoder := &canonicalEncoder{}
	encoder.string(domain)
	encoder.string(version)
	return encoder
}

func (e *canonicalEncoder) string(value string) {
	e.WriteByte('s')
	e.WriteString(strconv.Itoa(len([]byte(value))))
	e.WriteByte(':')
	e.WriteString(value)
}

func (e *canonicalEncoder) bytesValue(value []byte) {
	e.WriteByte('b')
	e.WriteString(strconv.Itoa(len(value)))
	e.WriteByte(':')
	e.Write(value)
}

func (e *canonicalEncoder) uint(value uint64) {
	e.WriteByte('u')
	e.WriteString(strconv.FormatUint(value, 10))
	e.WriteByte(';')
}

func (e *canonicalEncoder) boolean(value bool) {
	if value {
		e.WriteString("t;")
		return
	}
	e.WriteString("f;")
}

func (e *canonicalEncoder) list(count int, encode func(index int)) {
	e.WriteByte('l')
	e.WriteString(strconv.Itoa(count))
	e.WriteByte(':')
	for index := 0; index < count; index++ {
		encode(index)
	}
}

func sha256ContractDigest(value []byte) ContentDigest {
	digest := sha256.Sum256(value)
	return ContentDigest("sha256:" + hex.EncodeToString(digest[:]))
}

func verifyContractDigest(path string, expected ContentDigest, projection []byte) error {
	if err := validateContractDigest(path, string(expected)); err != nil {
		return err
	}
	if expected != sha256ContractDigest(projection) {
		return invalid(path, "digest_mismatch", "contract digest does not match canonical content")
	}
	return nil
}

// ComputeContractDigest provides the versioned digest format to future
// contract consumers without exposing the encoder's internal field grammar.
func ComputeContractDigest(projection []byte) ContentDigest {
	return sha256ContractDigest(projection)
}
