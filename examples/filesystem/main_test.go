package main

import (
	"os"
	"path/filepath"
	"testing"
)

// BUG EX1: safePath used strings.HasPrefix without a trailing separator
// on root, so for root "/tmp/a" the value "/tmp/aa/evil" would pass
// the check despite being outside the root ("/tmp/aa" is a sibling,
// not a subdirectory). The fix normalises root with filepath.Abs and
// only accepts either root itself or paths prefixed with root + OS
// separator.
func TestSafePathRejectsSiblingPrefix(t *testing.T) {
	// Create two real directories: /.../gomcp-fs-a and /.../gomcp-fs-aa.
	base := t.TempDir()
	rootA := filepath.Join(base, "a")
	sibling := filepath.Join(base, "aa")
	if err := os.MkdirAll(rootA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}

	// Point the example's package-level root to our test dir.
	root = rootA

	// Relative path that resolves to sibling via filepath.Join's
	// path cleaning: "../aa/evil" -> "/.../aa/evil", which starts
	// with rootA as a string but is *not* under it.
	if _, err := safePath("../aa/evil"); err == nil {
		t.Fatal("sibling-prefix path must be rejected — HasPrefix-without-separator regressed")
	}

	// A legitimate in-root file still resolves cleanly.
	got, err := safePath("subdir/file.txt")
	if err != nil {
		t.Fatalf("in-root path was rejected: %v", err)
	}
	want := filepath.Join(rootA, "subdir", "file.txt")
	if got != want {
		t.Fatalf("safePath returned %q, want %q", got, want)
	}

	// An explicit traversal that escapes root still errors.
	if _, err := safePath("../../etc/passwd"); err == nil {
		t.Fatal("traversal out of root must be rejected")
	}
}
