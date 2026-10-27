package reap

import (
	"path"
	"strings"
)

// entryUnder returns the top-level entry of root that contains p.
//
// Both trees this walks are laid out two levels below their root —
// complete/<category>/<entry> and links/<tracker>/<entry> — so an entry is the
// first two components under root, and anything deeper belongs to it.
//
// THIS FUNCTION IS THE WHOLE CORRECTNESS ARGUMENT. Doing the comparison at the
// directory level instead, by hand, reported 669 orphaned entries holding 2.8TB
// when the real answer was 31 entries and 104GB — because qBittorrent's
// content_path for a torrent holding ONE file inside a folder is the FILE, not
// the folder. Every such torrent looked unclaimed while it was actively
// seeding, and deleting on that basis would have destroyed live data.
//
// Reported paths are therefore never matched literally against disk entries.
// They are folded up to the entry that contains them, and the comparison
// happens there.
func entryUnder(root, p string) (string, bool) {
	root = path.Clean(root)
	p = path.Clean(p)
	if p == root || !strings.HasPrefix(p, root+"/") {
		return "", false
	}

	rest := strings.TrimPrefix(p, root+"/")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		// A file sitting directly in a category directory with no entry of its
		// own. Not addressable as an entry, so nothing can claim or reap it.
		return "", false
	}
	return path.Join(root, parts[0], parts[1]), true
}

// claimedEntries folds every reported path up to the entry that contains it.
//
// The result is the set of entries something still holds. Anything on disk and
// not in here is unclaimed — which is a necessary condition for reaping it, not
// a sufficient one; see Scanner.Run for the rest.
func claimedEntries(root string, reported []string) map[string]struct{} {
	out := make(map[string]struct{}, len(reported))
	for _, p := range reported {
		if e, ok := entryUnder(root, p); ok {
			out[e] = struct{}{}
		}
	}
	return out
}

// withinRoots reports whether p lies inside one of the roots.
//
// Plain containment, unlike entryUnder: this answers "could this process see
// the path at all", which has nothing to do with the two-level entry layout. A
// path one component below a root is inside it even though it is not an entry.
//
// The separator is appended so a sibling root sharing a prefix — /mnt/media and
// /mnt/media-old — does not match.
func withinRoots(roots []string, p string) bool {
	p = path.Clean(p)
	for _, root := range roots {
		root = path.Clean(root)
		if p == root || strings.HasPrefix(p, root+"/") {
			return true
		}
	}
	return false
}

// categoryOf returns the first component under root, which is the category for
// the download tree and the tracker for the cross-seed tree.
func categoryOf(root, entry string) string {
	root = path.Clean(root)
	rest := strings.TrimPrefix(path.Clean(entry), root+"/")
	if i := strings.Index(rest, "/"); i >= 0 {
		return rest[:i]
	}
	return rest
}
