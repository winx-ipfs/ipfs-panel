package files

import "testing"

func TestClean(t *testing.T) {
	ok := []string{"/", "/notes.txt", "/photos/2026", "/my notes.txt", "/笔记.txt"}
	for _, path := range ok {
		got, err := Clean(path)
		if err != nil || got != path {
			t.Fatalf("Clean(%q) = %q, %v", path, got, err)
		}
	}
	bad := []string{
		"", "notes.txt", "/../etc/passwd", "/./x", "/a//b", "/a/../b",
		"/a/./b", "/.", "/..", `/\a`, "/a\x00b", "/a\"b", "/ ", "/a/ ",
		"/" + string(make([]byte, 256)),
	}
	for _, path := range bad {
		if _, err := Clean(path); CodeOf(err) != "invalid_path" {
			t.Fatalf("Clean(%q) err = %v", path, err)
		}
	}
	long := "/" + string(make([]byte, maxPathBytes))
	if _, err := Clean(long); CodeOf(err) != "invalid_path" {
		t.Fatalf("long path %v", err)
	}
}

func TestJoinParentBase(t *testing.T) {
	got, err := Join("/photos", "a.txt")
	if err != nil || got != "/photos/a.txt" {
		t.Fatalf("%q %v", got, err)
	}
	got, err = Join("/", "a.txt")
	if err != nil || got != "/a.txt" {
		t.Fatalf("root join %q %v", got, err)
	}
	if _, err := Join("/photos", ".."); CodeOf(err) != "invalid_path" {
		t.Fatal(err)
	}
	parent, err := Parent("/photos/a.txt")
	if err != nil || parent != "/photos" {
		t.Fatalf("parent %q %v", parent, err)
	}
	parent, err = Parent("/")
	if err != nil || parent != "/" {
		t.Fatalf("root parent %q %v", parent, err)
	}
	base, err := Base("/photos/a.txt")
	if err != nil || base != "a.txt" {
		t.Fatalf("base %q %v", base, err)
	}
	if _, err := Base("/"); CodeOf(err) != "invalid_path" {
		t.Fatal(err)
	}
	if !Inside("/photos", "/photos/a.txt") || !Inside("/", "/a") || Inside("/photos", "/other") {
		t.Fatal("inside")
	}
}
