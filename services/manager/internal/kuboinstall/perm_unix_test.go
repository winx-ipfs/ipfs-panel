//go:build unix

package kuboinstall

import (
	"os"
	"testing"
)

func TestInstalledBinaryIsExecutable(t *testing.T) {
	body := linuxArchive(Pinned, script(Pinned, 0, ""))
	sum := sumOf(body)
	d := newDist(t)
	d.add(Pinned, body, "linux", "amd64", ".tar.gz")
	s, data, _ := testService(t, d, &memNode{}, map[string]string{"linux-amd64": sum})
	job, err := s.Install(Actor{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.State != "succeeded" {
		t.Fatalf("job %+v", done)
	}
	info, err := os.Stat(CurrentPath(data))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}
}
