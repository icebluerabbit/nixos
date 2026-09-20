package reconcile

import (
	"testing"
	"time"

	"homelab/clients/slskd"
)

// TestSearchTimeMatchesPostgresPrecision is the trap this normalisation exists
// for.
//
// slskd sends seven fractional digits — "2026-09-06T09:30:11.1648898Z" — and
// PostgreSQL's timestamptz holds six, rounding to fit. Handing the database a
// value it has to round means the time that comes back out is not the one that
// went in, the preloaded id map never matches, every search looks new, and
// every result ends up pointing at a search row that does not exist.
func TestSearchTimeMatchesPostgresPrecision(t *testing.T) {
	// The real value, taken from this homelab's slskd.
	in, err := time.Parse(time.RFC3339Nano, "2026-09-06T09:30:11.1648898Z")
	if err != nil {
		t.Fatal(err)
	}
	got := searchTime(in)

	if got.Nanosecond()%1000 != 0 {
		t.Errorf("nanosecond = %d; not microsecond-precise", got.Nanosecond())
	}
	// Rounding, not truncation: Postgres rounds, so .1648898 goes UP to
	// .164890 rather than down to .164889.
	if want := 164890000; got.Nanosecond() != want {
		t.Errorf("nanosecond = %d, want %d", got.Nanosecond(), want)
	}
	if got.Location() != time.UTC {
		t.Errorf("location = %s, want UTC", got.Location())
	}
}

// TestSearchTimeIsIdempotent is the property the map depends on.
//
// A value read back out of the database goes through this again when its key is
// rebuilt. If a second application moved it, the key would differ from the one
// stored on the first pass.
func TestSearchTimeIsIdempotent(t *testing.T) {
	in, err := time.Parse(time.RFC3339Nano, "2026-09-06T09:30:11.1648898Z")
	if err != nil {
		t.Fatal(err)
	}
	once := searchTime(in)
	twice := searchTime(once)
	if !once.Equal(twice) {
		t.Fatalf("not idempotent: %s then %s", once, twice)
	}
}

// TestSearchKeyDistinguishesQueryAndTime guards the separator.
//
// A naive concatenation would make ("ab", t) and ("a", "b"+t) the same key. The
// separator is a NUL because it cannot occur in a Soulseek query.
func TestSearchKeyDistinguishesQueryAndTime(t *testing.T) {
	at := searchTime(time.Date(2026, 9, 6, 9, 30, 11, 0, time.UTC))

	if searchKey("a", at) == searchKey("b", at) {
		t.Error("different queries collided")
	}
	later := at.Add(time.Microsecond)
	if searchKey("a", at) == searchKey("a", later) {
		t.Error("different times collided")
	}
	// Bound to variables rather than compared inline: the point is that two
	// separate calls agree, and staticcheck reads the inline form as one
	// expression compared with itself.
	first, second := searchKey("a", at), searchKey("a", at)
	if first != second {
		t.Error("the same search produced two keys")
	}

	// The same query issued twice is two searches, not one — repeating a
	// search is the normal way to find a peer who has since come online.
	if searchKey("Q&A", at) == searchKey("Q&A", later) {
		t.Error("a repeated query collided with its earlier run")
	}
}

// TestFileKeyIsThePeerAndPath pins the match rule.
//
// A Soulseek file has no hash, no id and no catalogue entry — its only identity
// is which peer holds it and where. That pair is also the natural key of
// soulseek_downloads, which is what makes the result-to-download match exact
// rather than inferred.
func TestFileKeyIsThePeerAndPath(t *testing.T) {
	const path = `music\Artist\Album\01 Track.flac`

	// Two separate calls, bound rather than compared inline — see searchKey's
	// determinism check above.
	first, second := fileKey("peer", path), fileKey("peer", path)
	if first != second {
		t.Error("the same file produced two keys")
	}
	// The same path on two peers is two different files: one may be online and
	// the other not, and only one of them was downloaded.
	if fileKey("a", path) == fileKey("b", path) {
		t.Error("two peers collided")
	}
	// And the separator must stop a concatenation collision.
	if fileKey("ab", path) == fileKey("a", "b"+path) {
		t.Error("peer and path ran together")
	}
}

// TestWriteResultsKeepsOnlyDownloadedFiles is the whole cut.
//
// A search is answered by up to a hundred peers offering a thousand files. One
// of them was taken. Recording the rest would be a million rows describing
// files that existed only while their peer was online and cannot be retrieved
// now — the row answers a question nobody can act on.
func TestWriteResultsKeepsOnlyDownloadedFiles(t *testing.T) {
	responses := []slskd.Response{
		{Username: "peerA", Files: []slskd.ResponseFile{
			{Filename: `music\Album\01 Taken.flac`, Size: 100, Extension: "flac",
				BitRate: 1411, BitDepth: 24, SampleRate: 96000, Length: 245},
			{Filename: `music\Album\02 Passed.flac`, Size: 200},
		}},
		{Username: "peerB", Files: []slskd.ResponseFile{
			// Same path as the taken file, different peer: not the same file.
			{Filename: `music\Album\01 Taken.flac`, Size: 100},
		}},
	}
	downloaded := map[string]struct{}{
		fileKey("peerA", `music\Album\01 Taken.flac`): {},
	}

	kept := filterResults(responses, downloaded)
	if len(kept) != 1 {
		t.Fatalf("kept %d files, want 1: %+v", len(kept), kept)
	}
	got := kept[0]
	if got.peer != "peerA" || got.file.Filename != `music\Album\01 Taken.flac` {
		t.Errorf("kept the wrong file: %s / %s", got.peer, got.file.Filename)
	}
	// The audio attributes are the reason the row is worth keeping at all:
	// soulseek_downloads has size and nothing else.
	if got.file.BitDepth != 24 || got.file.SampleRate != 96000 {
		t.Errorf("audio attributes lost: %+v", got.file)
	}
}

// TestFilterResultsIgnoresUnusableRows keeps the foreign keys satisfiable.
func TestFilterResultsIgnoresUnusableRows(t *testing.T) {
	responses := []slskd.Response{
		{Username: "", Files: []slskd.ResponseFile{{Filename: "x"}}},
		{Username: "peer", Files: []slskd.ResponseFile{{Filename: ""}}},
	}
	downloaded := map[string]struct{}{
		fileKey("", "x"):    {},
		fileKey("peer", ""): {},
	}
	if kept := filterResults(responses, downloaded); len(kept) != 0 {
		t.Errorf("kept %d unusable rows: %+v", len(kept), kept)
	}
}

// TestFilterResultsEmptyDownloadSet is the normal state of an old search.
//
// Nearly every search in the history produced no download at all. Those must
// write nothing — and, separately, still be marked read, or the pass would
// re-fetch a couple of hundred kilobytes per search every hour forever.
func TestFilterResultsEmptyDownloadSet(t *testing.T) {
	responses := []slskd.Response{
		{Username: "peer", Files: []slskd.ResponseFile{{Filename: "a"}, {Filename: "b"}}},
	}
	if kept := filterResults(responses, nil); len(kept) != 0 {
		t.Errorf("kept %d files with nothing downloaded", len(kept))
	}
}

// TestFileCompletePeerPrefersTheTransfer covers where the username actually is.
//
// slskd puts it on the nested transfer, not at the top level of the payload —
// so a reader that only looked at the outer object would drop every event for
// want of a peer, and the peer is half the key of a download.
func TestFileCompletePeerPrefersTheTransfer(t *testing.T) {
	nested := slskd.FileComplete{Transfer: slskd.DownloadFile{Username: "peer"}}
	if got := nested.Peer(); got != "peer" {
		t.Errorf("nested username = %q", got)
	}

	// Some versions also set it at the top level; that one wins when present.
	both := slskd.FileComplete{
		Username: "outer",
		Transfer: slskd.DownloadFile{Username: "inner"},
	}
	if got := both.Peer(); got != "outer" {
		t.Errorf("top-level username = %q", got)
	}

	if got := (slskd.FileComplete{}).Peer(); got != "" {
		t.Errorf("empty payload = %q", got)
	}
}

// TestUploadEventDecodesALibraryPath pins what an upload actually says.
//
// The local path of an upload is a LIBRARY path — slskd shares the library
// directly — which is why an upload can be tied to the file that was taken at
// all. On a download the same field means the opposite: where slskd put a file
// it fetched.
func TestUploadEventDecodesALibraryPath(t *testing.T) {
	// Verbatim from this homelab's slskd, trimmed.
	const data = `{"version":0,` +
		`"localFilename":"/mnt/media/library/music/Mudvayne/By the People, for the People (2007)/07 - Dull Boy.flac",` +
		`"remoteFilename":"music\\Mudvayne\\By the People, for the People (2007)\\07 - Dull Boy.flac",` +
		`"transfer":{"id":"f36e53d1-932c-4ed2-a728-8a25b65427b3","username":"n11byt33",` +
		`"direction":"Upload","size":33217422,"state":"Completed, Succeeded",` +
		`"requestedAt":"2026-09-14T03:52:36.0560226","startedAt":"2026-09-14T03:52:42.1432551Z",` +
		`"endedAt":"2026-09-14T03:52:49.5017591Z","bytesTransferred":33217422}}`

	ev := slskd.Event{Type: slskd.EventUploadFileComplete, Data: data}
	done, err := ev.AsFileComplete()
	if err != nil {
		t.Fatal(err)
	}
	if done.LocalFilename == "" || done.LocalFilename[:20] != "/mnt/media/library/m" {
		t.Errorf("local = %q; an upload's local path is a library path", done.LocalFilename)
	}
	// The transfer id is the primary key of an upload: unique across every one
	// slskd has recorded, so re-reading the log cannot duplicate a row.
	if done.Transfer.ID != "f36e53d1-932c-4ed2-a728-8a25b65427b3" {
		t.Errorf("transfer id = %q", done.Transfer.ID)
	}
	if done.Peer() != "n11byt33" {
		t.Errorf("peer = %q", done.Peer())
	}
	if done.Transfer.BytesTransferred != done.Transfer.Size {
		t.Errorf("a completed upload should have moved every byte: %+v", done.Transfer)
	}
}
