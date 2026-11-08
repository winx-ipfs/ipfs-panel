package pins

import (
	"context"
	"testing"
	"time"
)

func TestJobsCancelProgressAndCap(t *testing.T) {
	book := NewBook(2, 1)
	started := make(chan struct{})
	job, err := book.Start(sampleCIDv1(), "notes", true, time.Second, func(ctx context.Context, progress func(int)) (string, error) {
		progress(3)
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	got, err := book.Get(job.ID)
	if err != nil || got.Progress != 3 || got.State != "running" || got.CID != sampleCIDv1() {
		t.Fatalf("%#v %v", got, err)
	}
	cancelled, err := book.Cancel(job.ID)
	if err != nil || cancelled.State != "cancelled" || cancelled.Error != "cancelled" {
		t.Fatalf("%#v %v", cancelled, err)
	}
	again, err := book.Cancel(job.ID)
	if err != nil || again.State != "cancelled" {
		t.Fatalf("second %#v %v", again, err)
	}

	hold := make(chan struct{})
	t.Cleanup(func() { close(hold) })
	if _, err := book.Start(sampleCIDv0(), "", true, time.Second, func(ctx context.Context, _ func(int)) (string, error) {
		<-hold
		return sampleCIDv0(), nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := book.Start(sampleCIDv0(), "", false, time.Second, func(ctx context.Context, _ func(int)) (string, error) {
		<-hold
		return sampleCIDv0(), nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := book.Start(sampleCIDv0(), "", true, time.Second, func(context.Context, func(int)) (string, error) {
		t.Fatal("ran over the cap")
		return "", nil
	}); CodeOf(err) != "too_many" {
		t.Fatalf("cap %v", err)
	}
}

func TestJobsFinishAndPrune(t *testing.T) {
	book := NewBook(4, 1)
	var first string
	for i := 0; i < 2; i++ {
		job, err := book.Start(sampleCIDv1(), "", true, time.Second, func(context.Context, func(int)) (string, error) {
			return sampleCIDv0(), nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if first == "" {
			first = job.ID
		}
		waitJob(t, book, job.ID, "succeeded")
	}
	if _, err := book.Get(first); CodeOf(err) != "not_found" {
		t.Fatalf("pruned %v", err)
	}
	list := book.List()
	if len(list) != 1 || list[0].State != "succeeded" || list[0].CID != sampleCIDv0() {
		t.Fatalf("%#v", list)
	}
}

func waitJob(t *testing.T, book *Book, id, state string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := book.Get(id)
		if err == nil && job.State == state {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s did not reach %s", id, state)
}
