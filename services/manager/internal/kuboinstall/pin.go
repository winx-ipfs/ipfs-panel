package kuboinstall

// Pinned is the release Install downloads. It is the same pin as the harness.
const Pinned = "0.43.1"

// SupportedMin is inclusive. SupportedMax is exclusive.
// The cap is a panel limit: the RPC surface is tested at the pin, and one
// further minor may be offered. Raising it is a pull request.
const (
	SupportedMin   = "0.43.1"
	SupportedMax   = "0.45.0"
	SupportedLabel = ">=0.43.1 <0.45.0"
)

// pinnedSHA512 is the published hash of each archive, copied from the
// GitHub release sidecars for v0.43.1 on 2026-11-27. linux-amd64 matches
// .github/scripts/fetch-kubo.sh. A sidecar that disagrees with this table
// is a failed verification, not a new pin.
var pinnedSHA512 = map[string]string{
	"linux-amd64":   "ff53b2428794fc8cca39505d28c15b4cceaef4b90a09284f291f611fcdc8c06399690575fc89cbd4d85ef71f3652581296c6f2e710386f887c8edf36b6e89d71",
	"linux-arm64":   "70f082584651ef78fb5b07448be53bb0adf141aadcb7eea15ffa59bd9ca78fed46781c2cf5915b3528c591052f97486067fedb0a7a1415abd5bfd44942921501",
	"darwin-amd64":  "efe0c1561595fbf7b5bdc3fc651863697c2bf388eed5f8c94c163f60d9575201808538abba84f652718f015ad1af6e3c01ced1321ddb2082f934ba8952e4a4ab",
	"darwin-arm64":  "bc282d781d856736051ce6d216ba47d52bec703f1a54ab5fa20fba26a92e4af3e8fb131e220cdf01f20205f1effb1a5b7bedbdeff15e8edbbe8e5b01923a7cbb",
	"windows-amd64": "e611cecef25cddb10b7f907145383608d9b31219ad7120041aeacfe2313882da1944f2897fda29bc2d959dd357e4950089ec8f2cbd52b29f2fc472bcae4356d0",
	"windows-arm64": "a6f2578ad3386a567c437eeebac1646293276b5fb44a26a33195f4310ed7b06cab9dfe0e02e17f33ce8ba1beb4f1ccef45375a861c8cc60192f46da7e0c02166",
}

// PinnedDigest returns the compiled SHA-512 for one platform, or "" when
// this panel has no pin for it.
func PinnedDigest(goos, goarch string) string {
	return pinnedSHA512[goos+"-"+goarch]
}
