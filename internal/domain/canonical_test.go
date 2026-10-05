package domain

import "testing"

func TestCanonicalEncoderUsesLengthDelimitedUTF8Fields(t *testing.T) {
	encoder := newCanonicalEncoder("test", "v1")
	encoder.string("a")
	encoder.string("é")
	encoder.string("bc")
	if got, want := encoder.String(), "s4:tests2:v1s1:as2:és2:bc"; got != want {
		t.Fatalf("canonical bytes=%q, want %q", got, want)
	}
	if got, want := sha256ContractDigest([]byte("x")), ContentDigest("sha256:2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881"); got != want {
		t.Fatalf("digest=%q, want %q", got, want)
	}
}
