//go:build darwin

package preflight

import "golang.org/x/sys/unix"

// com.apple.quarantine marks files that arrived from the internet.
// kMDItemWhereFroms holds the source URL (as a binary plist).
var originAttrs = []string{"com.apple.quarantine", "com.apple.metadata:kMDItemWhereFroms"}

func readOrigins(path string) []Origin {
	return readAttrs(path, originAttrs, unix.Getxattr)
}
