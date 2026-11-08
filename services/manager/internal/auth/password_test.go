package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	encoded, err := Hash("correct-horse-battery", TestingParams())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$") {
		t.Fatalf("phc %s", encoded)
	}
	if !Verify("correct-horse-battery", encoded) {
		t.Fatal("verify")
	}
	if Verify("correct-horse-batteries", encoded) {
		t.Fatal("accepted a different password")
	}
	if Verify("correct-horse-battery", encoded+"x") {
		t.Fatal("accepted a damaged hash")
	}
	tampered := strings.Replace(encoded, "m=", "m=999999999,", 1)
	if Verify("correct-horse-battery", tampered) {
		t.Fatal("accepted a huge memory parameter")
	}
}

func TestDefaultParamsRoundTrip(t *testing.T) {
	encoded, err := Hash("correct-horse-battery", DefaultParams())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(encoded, "m=19456,t=2,p=1$") {
		t.Fatalf("params %s", encoded)
	}
	if !Verify("correct-horse-battery", encoded) || Verify("nope-nope-nope", encoded) {
		t.Fatal("default verify")
	}
}

func TestPasswordAndUsernameRules(t *testing.T) {
	if err := ValidatePassword("short"); err == nil {
		t.Fatal("short")
	}
	if err := ValidatePassword(strings.Repeat("a", 12)); err == nil {
		t.Fatal("repeated")
	}
	if err := ValidatePassword("password1234"); err == nil {
		t.Fatal("denylist")
	}
	if err := ValidatePassword("correct horse"); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePassword(strings.Repeat("a", 129)); err == nil {
		t.Fatal("long")
	}
	if err := ValidateUsername(""); err == nil {
		t.Fatal("empty user")
	}
	if err := ValidateUsername("has space"); err == nil {
		t.Fatal("space")
	}
	if err := ValidateUsername(".hidden"); err == nil {
		t.Fatal("dot")
	}
	if err := ValidateUsername("ada"); err != nil {
		t.Fatal(err)
	}
	if constUsername("Ada", "ada") {
		t.Fatal("username compare is case-sensitive")
	}
	if !constUsername("ada", "ada") {
		t.Fatal("same username")
	}
}
