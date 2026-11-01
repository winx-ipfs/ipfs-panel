package pins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLabelsRoundTripAndCorrupt(t *testing.T) {
	dir := t.TempDir()
	s := OpenLabels(dir)
	cid := sampleCIDv1()
	if err := s.Set(cid, "notes"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, labelFileName)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}
	again := OpenLabels(dir)
	if again.Get(cid) != "notes" {
		t.Fatalf("got %q", again.Get(cid))
	}
	pins := again.Apply([]Pin{{CID: cid, Type: "recursive"}, {CID: cid, Name: "kubo", Type: "direct"}})
	if pins[0].Name != "notes" || pins[1].Name != "kubo" {
		t.Fatalf("%#v", pins)
	}
	if err := again.Delete(cid); err != nil {
		t.Fatal(err)
	}
	if OpenLabels(dir).Get(cid) != "" {
		t.Fatal("label remained")
	}

	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	frozen := OpenLabels(dir)
	if err := frozen.Set(cid, "nope"); err == nil {
		t.Fatal("wrote over a corrupt file")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "{" {
		t.Fatalf("file %q %v", got, err)
	}
}
