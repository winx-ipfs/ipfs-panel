package pins

import (
	"bytes"
	"math/big"
	"strings"
	"testing"
)

func TestParseTarget(t *testing.T) {
	v0 := sampleCIDv0()
	if !strings.HasPrefix(v0, "Qm") {
		t.Fatalf("v0 %s", v0)
	}
	v1 := sampleCIDv1()
	if !strings.HasPrefix(v1, "b") {
		t.Fatalf("v1 %s", v1)
	}
	upper := "B" + strings.ToUpper(v1[1:])
	for _, tc := range []struct{ in, want string }{
		{v0, v0},
		{"  " + v0 + "\n", v0},
		{v1, v1},
		{upper, v1},
		{"/ipfs/" + v0, "/ipfs/" + v0},
		{"/ipfs/" + v1 + "/docs/a.txt", "/ipfs/" + v1 + "/docs/a.txt"},
		{"f" + hexOf(mustV1Bytes()), "f" + hexOf(mustV1Bytes())},
	} {
		got, err := ParseTarget(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("in %q got %q err %v want %q", tc.in, got, err, tc.want)
		}
	}
	for _, bad := range []string{
		"",
		"hello",
		"Qm",
		"bafy",
		"/ipns/" + v0,
		"http://127.0.0.1/ipfs/" + v0,
		"/ipfs/" + v0 + "/../x",
		"/ipfs/" + v0 + "/./x",
		"/ipfs/" + v0 + "//x",
		"b" + v1[1:len(v1)-2],
	} {
		if _, err := ParseTarget(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	if RootCID("/ipfs/"+v1+"/docs") != v1 {
		t.Fatal(RootCID("/ipfs/" + v1 + "/docs"))
	}
}

func TestInspect(t *testing.T) {
	v0, err := Inspect(sampleCIDv0())
	if err != nil || v0.Version != 0 || v0.Codec != 0x70 || v0.Hash != 0x12 {
		t.Fatalf("v0 %+v err %v", v0, err)
	}
	v1, err := Inspect(sampleCIDv1())
	if err != nil || v1.Version != 1 || v1.Codec != 0x55 || v1.Hash != 0x12 {
		t.Fatalf("v1 %+v err %v", v1, err)
	}
	dagpb := "b" + encodeBase32(append([]byte{0x01, 0x70, 0x12, 0x20}, bytes.Repeat([]byte{0x11}, 32)...))
	pb, err := Inspect(dagpb)
	if err != nil || pb.Version != 1 || pb.Codec != 0x70 || pb.Hash != 0x12 {
		t.Fatalf("dag-pb %+v err %v", pb, err)
	}
	// The inlined "hello world" CID from ipfs-webui's explore test.
	id, err := Inspect("bafkqac3imvwgy3zao5xxe3de")
	if err != nil || id.Version != 1 || id.Codec != 0x55 || id.Hash != 0x00 {
		t.Fatalf("identity %+v err %v", id, err)
	}
	if _, err := Inspect("/ipfs/" + sampleCIDv1() + "/docs"); err != nil {
		t.Fatal(err)
	}
}

func TestValidName(t *testing.T) {
	if err := ValidName(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidName("会议记录"); err != nil {
		t.Fatal(err)
	}
	if err := ValidName("my notes"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{" notes", "notes ", "a/b", "a\\b", "say \"hi\"", "line\nbreak", strings.Repeat("a", 256)} {
		if err := ValidName(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func sampleCIDv0() string {
	raw := append([]byte{0x12, 0x20}, bytes.Repeat([]byte{0x11}, 32)...)
	return encodeBase58(raw)
}

func sampleCIDv1() string {
	return "b" + encodeBase32(mustV1Bytes())
}

func mustV1Bytes() []byte {
	raw := []byte{0x01, 0x55, 0x12, 0x20}
	return append(raw, bytes.Repeat([]byte{0x22}, 32)...)
}

func hexOf(raw []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(raw)*2)
	for i, b := range raw {
		out[i*2] = hexdigits[b>>4]
		out[i*2+1] = hexdigits[b&0x0f]
	}
	return string(out)
}

func encodeBase58(raw []byte) string {
	zeros := 0
	for zeros < len(raw) && raw[zeros] == 0 {
		zeros++
	}
	n := new(big.Int).SetBytes(raw)
	base := big.NewInt(58)
	zero := big.NewInt(0)
	rem := new(big.Int)
	var digits []byte
	for n.Cmp(zero) > 0 {
		n.QuoRem(n, base, rem)
		digits = append(digits, b58Alphabet[rem.Int64()])
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return strings.Repeat("1", zeros) + string(digits)
}

func encodeBase32(raw []byte) string {
	var out []byte
	var buf uint64
	var bits uint
	for _, b := range raw {
		buf = (buf << 8) | uint64(b)
		bits += 8
		for bits >= 5 {
			bits -= 5
			out = append(out, b32Alphabet[(buf>>bits)&31])
		}
	}
	if bits > 0 {
		out = append(out, b32Alphabet[(buf<<(5-bits))&31])
	}
	return string(out)
}
