package seed

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testLogger(buf *bytes.Buffer) *slog.Logger {
	if buf == nil {
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return slog.New(slog.NewJSONHandler(buf, nil))
}

// writeStorage lays down a .storage file the way HA leaves it.
func writeStorage(t *testing.T, dir, name string, doc any) string {
	t.Helper()
	path := filepath.Join(dir, name)
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// readStorage reads one back.
func readStorage(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return doc
}

// list reads one array out of a storage document's data.
func list(t *testing.T, doc map[string]any, key string) []any {
	t.Helper()
	data, ok := doc["data"].(map[string]any)
	if !ok {
		t.Fatalf("data is %T", doc["data"])
	}
	items, ok := data[key].([]any)
	if !ok {
		t.Fatalf("%s is %T", key, data[key])
	}
	return items
}

// --- expandvars ---

// Python's expandvars leaves an unset variable exactly as written. Blanking
// it instead would write an empty password into a config entry, and the
// integration would fail to authenticate with no hint why.
func TestExpandVarsMatchesPython(t *testing.T) {
	lookup := func(name string) (string, bool) {
		if name == "SET_VAR" {
			return "value", true
		}
		return "", false
	}

	cases := map[string]string{
		"$SET_VAR":       "value",
		"${SET_VAR}":     "value",
		"$UNSET_VAR":     "$UNSET_VAR",
		"${UNSET_VAR}":   "${UNSET_VAR}",
		"a$SET_VAR-b":    "avalue-b",
		"$$":             "$$",
		"no vars":        "no vars",
		"${UNSET}x":      "${UNSET}x",
		"${unterminated": "${unterminated",
	}
	for in, want := range cases {
		if got := ExpandVars(in, lookup); got != want {
			t.Errorf("ExpandVars(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReadSeedExpandedSubstitutesSecrets(t *testing.T) {
	lookup := func(name string) (string, bool) {
		if name == "HA_TOKEN" {
			return "s3cret", true
		}
		return "", false
	}

	var entries []Entry
	in := strings.NewReader(`[{"entry_id":"abc","data":{"token":"$HA_TOKEN"}}]`)
	if err := ReadSeedExpanded(in, lookup, &entries); err != nil {
		t.Fatalf("ReadSeedExpanded: %v", err)
	}

	data, _ := entries[0]["data"].(map[string]any)
	if data["token"] != "s3cret" {
		t.Errorf("token = %v, want s3cret", data["token"])
	}
}

func TestReadSeedToleratesEmptyInput(t *testing.T) {
	var areas []Area
	if err := ReadSeed(strings.NewReader("  \n"), &areas); err != nil {
		t.Errorf("ReadSeed: %v", err)
	}
	if len(areas) != 0 {
		t.Errorf("areas = %v", areas)
	}
}

func TestReadSeedReportsBadJSON(t *testing.T) {
	var areas []Area
	if err := ReadSeed(strings.NewReader("{not json"), &areas); err == nil {
		t.Error("ReadSeed accepted malformed JSON")
	}
}

// --- areas ---

func TestAreasCreatesTheRegistry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "core.area_registry")
	icon := "mdi:sofa"

	err := Areas(testLogger(nil), path, []Area{
		{AreaID: "living_room", Name: "Living Room", Icon: &icon},
		{AreaID: "garage", Name: "Garage"},
	})
	if err != nil {
		t.Fatalf("Areas: %v", err)
	}

	doc := readStorage(t, path)
	if doc["key"] != "core.area_registry" || doc["version"] != float64(3) {
		t.Errorf("header = %v", doc)
	}

	areas := list(t, doc, "areas")
	if len(areas) != 2 {
		t.Fatalf("areas = %v", areas)
	}
	first, _ := areas[0].(map[string]any)
	if first["id"] != "living_room" || first["name"] != "Living Room" || first["icon"] != "mdi:sofa" {
		t.Errorf("first area = %v", first)
	}
	// HA writes these fields on every area; a missing one upsets its
	// schema migration.
	for _, key := range []string{"aliases", "floor_id", "labels", "picture"} {
		if _, ok := first[key]; !ok {
			t.Errorf("%s missing from a seeded area: %v", key, first)
		}
	}
	// An area with no icon carries null, not "".
	second, _ := areas[1].(map[string]any)
	if second["icon"] != nil {
		t.Errorf("icon = %v, want null", second["icon"])
	}
}

// Append-only: a person may have renamed an area or moved it to a floor,
// and none of that is in the declaration.
func TestAreasLeavesExistingAreasAlone(t *testing.T) {
	dir := t.TempDir()
	path := writeStorage(t, dir, "core.area_registry", map[string]any{
		"version": 3, "minor_version": 7, "key": "core.area_registry",
		"data": map[string]any{"areas": []any{
			map[string]any{
				"id": "living_room", "name": "Lounge", "floor_id": "ground",
				"aliases": []any{"snug"}, "labels": []any{}, "picture": nil, "icon": nil,
			},
		}},
	})

	err := Areas(testLogger(nil), path, []Area{
		{AreaID: "living_room", Name: "Living Room"},
		{AreaID: "garage", Name: "Garage"},
	})
	if err != nil {
		t.Fatalf("Areas: %v", err)
	}

	areas := list(t, readStorage(t, path), "areas")
	if len(areas) != 2 {
		t.Fatalf("areas = %v", areas)
	}
	existing, _ := areas[0].(map[string]any)
	if existing["name"] != "Lounge" || existing["floor_id"] != "ground" {
		t.Errorf("an existing area was overwritten: %v", existing)
	}
}

// Nothing to add means nothing written, so HA does not see its file change
// on every boot.
func TestAreasDoesNotRewriteWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := writeStorage(t, dir, "core.area_registry", map[string]any{
		"version": 3, "minor_version": 7, "key": "core.area_registry",
		"data": map[string]any{"areas": []any{
			map[string]any{"id": "garage", "name": "Garage"},
		}},
	})
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := Areas(testLogger(nil), path, []Area{{AreaID: "garage", Name: "Garage"}}); err != nil {
		t.Fatalf("Areas: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("the registry was rewritten with nothing to add")
	}
}

func TestAreasDoesNothingWithoutDeclaration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "core.area_registry")

	if err := Areas(testLogger(nil), path, nil); err != nil {
		t.Fatalf("Areas: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("a registry was created with nothing declared")
	}
}

// --- persons ---

// A person registry that does not exist means HA has not onboarded. Writing
// one would create a person with no credentials.
func TestPersonsNeverCreatesTheRegistry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "person")

	err := Persons(testLogger(nil), path, []Person{
		{ID: "abc", DeviceTrackers: []string{"device_tracker.phone"}},
	})
	if err != nil {
		t.Fatalf("Persons: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("a person registry was created")
	}
}

// A declared person HA does not know is reported, not invented.
func TestPersonsSkipsUnknownPeople(t *testing.T) {
	dir := t.TempDir()
	path := writeStorage(t, dir, "person", map[string]any{
		"version": 2, "key": "person",
		"data": map[string]any{"items": []any{
			map[string]any{"id": "known", "name": "Known"},
		}},
	})

	var logs bytes.Buffer
	err := Persons(testLogger(&logs), path, []Person{
		{ID: "stranger", DeviceTrackers: []string{"device_tracker.phone"}},
	})
	if err != nil {
		t.Fatalf("Persons: %v", err)
	}

	items := list(t, readStorage(t, path), "items")
	if len(items) != 1 {
		t.Errorf("a person was created: %v", items)
	}
	if !strings.Contains(logs.String(), "person not found") {
		t.Errorf("logs = %s", logs.String())
	}
}

// Trackers are merged, never replaced: HA may have added one through the
// mobile app and losing it would stop presence working.
func TestPersonsMergesTrackers(t *testing.T) {
	dir := t.TempDir()
	path := writeStorage(t, dir, "person", map[string]any{
		"version": 2, "key": "person",
		"data": map[string]any{"items": []any{
			map[string]any{
				"id":              "zms",
				"name":            "zms",
				"user_id":         "auth-user",
				"device_trackers": []any{"device_tracker.app"},
			},
		}},
	})

	err := Persons(testLogger(nil), path, []Person{
		{ID: "zms", DeviceTrackers: []string{"device_tracker.app", "device_tracker.phone"}},
	})
	if err != nil {
		t.Fatalf("Persons: %v", err)
	}

	items := list(t, readStorage(t, path), "items")
	person, _ := items[0].(map[string]any)
	trackers, _ := person["device_trackers"].([]any)
	if len(trackers) != 2 || trackers[0] != "device_tracker.app" || trackers[1] != "device_tracker.phone" {
		t.Errorf("trackers = %v", trackers)
	}
	// Everything else about the person survives.
	if person["user_id"] != "auth-user" {
		t.Errorf("the person's auth link was lost: %v", person)
	}
}

func TestPersonsDoesNotRewriteWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := writeStorage(t, dir, "person", map[string]any{
		"version": 2, "key": "person",
		"data": map[string]any{"items": []any{
			map[string]any{"id": "zms", "device_trackers": []any{"device_tracker.app"}},
		}},
	})
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	err = Persons(testLogger(nil), path, []Person{
		{ID: "zms", DeviceTrackers: []string{"device_tracker.app"}},
	})
	if err != nil {
		t.Fatalf("Persons: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("the registry was rewritten with nothing to add")
	}
}

// A person with no trackers yet carries null rather than an array.
func TestPersonsHandlesNullTrackers(t *testing.T) {
	dir := t.TempDir()
	path := writeStorage(t, dir, "person", map[string]any{
		"version": 2, "key": "person",
		"data": map[string]any{"items": []any{
			map[string]any{"id": "zms", "device_trackers": nil},
		}},
	})

	err := Persons(testLogger(nil), path, []Person{
		{ID: "zms", DeviceTrackers: []string{"device_tracker.phone"}},
	})
	if err != nil {
		t.Fatalf("Persons: %v", err)
	}

	items := list(t, readStorage(t, path), "items")
	person, _ := items[0].(map[string]any)
	trackers, _ := person["device_trackers"].([]any)
	if len(trackers) != 1 || trackers[0] != "device_tracker.phone" {
		t.Errorf("trackers = %v", trackers)
	}
}

// --- config entries ---

// A time with microseconds, because that is what the timestamp format has
// to carry: HA's own Python writes isoformat(), not an RFC3339 Z.
var fixedNow = time.Date(2026, 9, 30, 12, 0, 0, 123456000, time.UTC)

// The stamp written into a config entry has to look like the one HA writes
// itself, down to the +00:00 rather than a Z.
func TestTimestampMatchesPythonIsoformat(t *testing.T) {
	if got := Timestamp(fixedNow); got != "2026-09-30T12:00:00.123456+00:00" {
		t.Errorf("Timestamp = %q", got)
	}
	whole := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	if got := Timestamp(whole); got != "2026-09-30T12:00:00+00:00" {
		t.Errorf("Timestamp on a whole second = %q", got)
	}
}

func TestEntriesSeedsNewEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "core.config_entries")

	err := Entries(testLogger(nil), path, []Entry{{
		"entry_id": "abc", "domain": "mqtt", "title": "Mosquitto",
		"data": map[string]any{"broker": "10.10.30.7"},
	}}, fixedNow)
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}

	entries := list(t, readStorage(t, path), "entries")
	entry, _ := entries[0].(map[string]any)

	// HA writes these on every entry it creates; a seeded one has to look
	// the same or HA's own migration trips over it.
	for _, key := range []string{"created_at", "modified_at", "discovery_keys", "subentries"} {
		if _, ok := entry[key]; !ok {
			t.Errorf("%s missing: %v", key, entry)
		}
	}
	if entry["created_at"] != Timestamp(fixedNow) {
		t.Errorf("created_at = %v", entry["created_at"])
	}
}

// A declaration that states created_at keeps it, rather than being stamped
// with now.
func TestEntriesKeepsDeclaredTimestamps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "core.config_entries")

	err := Entries(testLogger(nil), path, []Entry{{
		"entry_id": "abc", "domain": "mqtt", "created_at": "2020-01-01T00:00:00Z",
	}}, fixedNow)
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}

	entries := list(t, readStorage(t, path), "entries")
	entry, _ := entries[0].(map[string]any)
	if entry["created_at"] != "2020-01-01T00:00:00Z" {
		t.Errorf("created_at = %v", entry["created_at"])
	}
}

// Reconciliation is what lets a re-seed repair an entry: declared data keys
// win, and keys HA added are left alone.
func TestEntriesUpdatesDataKeys(t *testing.T) {
	dir := t.TempDir()
	path := writeStorage(t, dir, "core.config_entries", map[string]any{
		"version": 1, "minor_version": 1, "key": "core.config_entries",
		"data": map[string]any{"entries": []any{
			map[string]any{
				"entry_id": "abc", "domain": "mqtt",
				"data":        map[string]any{"broker": "old-host", "port": float64(1883)},
				"modified_at": "2020-01-01T00:00:00Z",
			},
		}},
	})

	err := Entries(testLogger(nil), path, []Entry{{
		"entry_id": "abc", "domain": "mqtt",
		"data": map[string]any{"broker": "10.10.30.7"},
	}}, fixedNow)
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}

	entries := list(t, readStorage(t, path), "entries")
	entry, _ := entries[0].(map[string]any)
	data, _ := entry["data"].(map[string]any)

	if data["broker"] != "10.10.30.7" {
		t.Errorf("broker = %v", data["broker"])
	}
	if data["port"] != float64(1883) {
		t.Errorf("an undeclared key was lost: %v", data)
	}
	if entry["modified_at"] != Timestamp(fixedNow) {
		t.Errorf("modified_at was not stamped: %v", entry["modified_at"])
	}
}

// unique_id is what the integration keys off; syncing it is what repairs an
// entry that went wrong.
func TestEntriesSyncsIdentityFields(t *testing.T) {
	dir := t.TempDir()
	path := writeStorage(t, dir, "core.config_entries", map[string]any{
		"version": 1, "minor_version": 1, "key": "core.config_entries",
		"data": map[string]any{"entries": []any{
			map[string]any{"entry_id": "abc", "domain": "mqtt", "unique_id": "wrong", "title": "Old"},
		}},
	})

	err := Entries(testLogger(nil), path, []Entry{{
		"entry_id": "abc", "domain": "mqtt", "unique_id": "right", "title": "New",
	}}, fixedNow)
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}

	entries := list(t, readStorage(t, path), "entries")
	entry, _ := entries[0].(map[string]any)
	if entry["unique_id"] != "right" || entry["title"] != "New" {
		t.Errorf("identity fields = %v", entry)
	}
}

// Subentries are declared wholesale, so a removed one disappears.
func TestEntriesReplacesSubentriesWholesale(t *testing.T) {
	dir := t.TempDir()
	path := writeStorage(t, dir, "core.config_entries", map[string]any{
		"version": 1, "minor_version": 1, "key": "core.config_entries",
		"data": map[string]any{"entries": []any{
			map[string]any{
				"entry_id": "abc", "domain": "mqtt",
				"subentries": []any{
					map[string]any{"subentry_id": "keep", "title": "Keep"},
					map[string]any{"subentry_id": "drop", "title": "Drop"},
				},
			},
		}},
	})

	err := Entries(testLogger(nil), path, []Entry{{
		"entry_id": "abc", "domain": "mqtt",
		"subentries": []any{
			map[string]any{"subentry_id": "keep", "title": "Keep"},
		},
	}}, fixedNow)
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}

	entries := list(t, readStorage(t, path), "entries")
	entry, _ := entries[0].(map[string]any)
	subs, _ := entry["subentries"].([]any)
	if len(subs) != 1 {
		t.Fatalf("subentries = %v", subs)
	}
	sub, _ := subs[0].(map[string]any)
	if sub["subentry_id"] != "keep" {
		t.Errorf("the wrong subentry survived: %v", sub)
	}
}

// An entry already matching its declaration is left alone, so modified_at
// does not churn on every boot.
func TestEntriesDoesNotRewriteWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := writeStorage(t, dir, "core.config_entries", map[string]any{
		"version": 1, "minor_version": 1, "key": "core.config_entries",
		"data": map[string]any{"entries": []any{
			map[string]any{
				"entry_id": "abc", "domain": "mqtt", "unique_id": "u",
				"data":        map[string]any{"broker": "10.10.30.7"},
				"subentries":  []any{},
				"modified_at": "2020-01-01T00:00:00Z",
			},
		}},
	})
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	err = Entries(testLogger(nil), path, []Entry{{
		"entry_id": "abc", "domain": "mqtt", "unique_id": "u",
		"data":       map[string]any{"broker": "10.10.30.7"},
		"subentries": []any{},
	}}, fixedNow)
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("the registry was rewritten with nothing to change:\n%s\n---\n%s", before, after)
	}
}

func TestEntriesDoesNothingWithoutDeclaration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "core.config_entries")

	if err := Entries(testLogger(nil), path, nil, fixedNow); err != nil {
		t.Fatalf("Entries: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("a registry was created with nothing declared")
	}
}

// A storage file HA left malformed is reported rather than replaced:
// overwriting it would lose every area, person or integration in it.
func TestSeedersReportUnparsableStorage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Areas(testLogger(nil), path, []Area{{AreaID: "a", Name: "A"}}); err == nil {
		t.Error("Areas accepted a malformed registry")
	}
	if err := Persons(testLogger(nil), path, []Person{{ID: "a"}}); err == nil {
		t.Error("Persons accepted a malformed registry")
	}
	if err := Entries(testLogger(nil), path, []Entry{{"entry_id": "a"}}, fixedNow); err == nil {
		t.Error("Entries accepted a malformed registry")
	}
}
