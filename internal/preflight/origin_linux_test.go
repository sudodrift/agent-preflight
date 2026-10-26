//go:build linux

package preflight

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// TestOriginXattr sets user.xdg.origin.url on a real file, the way
// curl --xattr and wget --xattr do, and checks that it is reported.
func TestOriginXattr(t *testing.T) {
	root := copyTree(t, "clean")
	home := filepath.Join(root, "home")
	start := filepath.Join(home, "code", "app")
	target := filepath.Join(start, "CLAUDE.md")
	url := "https://files.example.invalid/CLAUDE.md"
	if err := unix.Setxattr(target, "user.xdg.origin.url", []byte(url), 0); err != nil {
		if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EPERM) {
			t.Skipf("user xattrs not supported on this filesystem: %v", err)
		}
		t.Fatal(err)
	}
	r := scan(t, start, home, noEnv)
	if !hasFinding(r, "origin", "app/CLAUDE.md", "downloaded from the internet (user.xdg.origin.url)") {
		dump(t, r)
		t.Fatal("origin xattr not reported")
	}
	f := fileByPath(r, "app/CLAUDE.md")
	if len(f.Origins) != 1 || f.Origins[0].Value != url {
		t.Errorf("origins = %+v", f.Origins)
	}
	// A file without the attribute has no origin.
	other := filepath.Join(start, ".claude", "settings.json")
	if got := readOrigins(other); len(got) != 0 {
		t.Errorf("unexpected origins %+v", got)
	}
	_ = os.Remove
}
