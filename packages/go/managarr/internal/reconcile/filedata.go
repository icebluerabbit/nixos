package reconcile

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
)

// statInto records the inode behind a path and returns its identity.
//
// A path that cannot be stat'd yields (nil, nil, nil). That is not an error: a
// file listed by a client but absent from disk, or outside the mounts this
// container can see, is a fact about the filesystem rather than a failure of
// the pass. The caller stores NULL and moves on.
//
// Identity is (inode, size) rather than (device, inode). Inodes are recycled
// after deletion, so size guards against a stale row aliasing a new file; and
// st_dev on an NFS client is the mount's number, not the server's, so it is not
// stable across remounts and is advisory only.
//
// nlink is a snapshot. Anything acting on it must re-stat first.
func statInto(ctx context.Context, d *dao.DAO, path string, observedAt time.Time) (inode, size *int64, err error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, nil, nil
	}
	sys, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, nil, nil
	}

	// Widening conversions on every platform this builds for: Stat_t's fields
	// are unsigned, and the columns are signed. An inode or device number large
	// enough to wrap would be a filesystem this does not run on.
	dev := int64(sys.Dev)   //nolint:gosec // st_dev is 64-bit, stored as-is
	nlink := int(sys.Nlink) //nolint:gosec // link counts are small
	ino := int64(sys.Ino)   //nolint:gosec // st_ino is 64-bit, stored as-is
	sz := fi.Size()

	// Through the DAO rather than an inline clause.OnConflict: the conflict
	// target is the rule this package exists to keep in one place, and an
	// arbiter written at the call site is how the same target drifted from its
	// index before.
	rows := []model.FileData{{
		Inode:          ino,
		SizeBytes:      sz,
		Device:         &dev,
		Nlink:          &nlink,
		LastObservedAt: observedAt.UTC(),
	}}
	err = dao.Upsert(ctx, d, rows,
		[]string{"inode", "size_bytes"},
		[]string{"nlink", "device", "last_observed_at"})
	if err != nil {
		return nil, nil, fmt.Errorf("file_data for %q: %w", path, err)
	}
	return &ino, &sz, nil
}
