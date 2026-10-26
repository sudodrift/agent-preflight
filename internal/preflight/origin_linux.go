//go:build linux

package preflight

import "golang.org/x/sys/unix"

// Extended attributes that record where a file was downloaded from on Linux.
// curl --xattr and wget --xattr write them. Chromium used to and stopped.
var originAttrs = []string{"user.xdg.origin.url", "user.xdg.referrer.url"}

func readOrigins(path string) []Origin {
	return readAttrs(path, originAttrs, unix.Getxattr)
}
