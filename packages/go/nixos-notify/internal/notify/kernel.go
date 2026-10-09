package notify

import (
	"os"
	"strings"
)

// osReleasePath is what uname(2) reports as the release, without needing a
// syscall wrapper or a cgo dependency for one string.
const osReleasePath = "/proc/sys/kernel/osrelease"

// kernelRelease is the running kernel's version, or "" when it cannot be
// read — a boot report with a blank kernel is still worth sending.
func kernelRelease() string {
	raw, err := os.ReadFile(osReleasePath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}
