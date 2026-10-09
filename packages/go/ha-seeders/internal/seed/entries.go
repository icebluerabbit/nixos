package seed

import (
	"log/slog"
	"reflect"
	"time"
)

// EntryStoragePath is HA's config entry registry.
const EntryStoragePath = "/var/lib/hass/.storage/core.config_entries"

// The registry's schema version, used only when the file does not exist yet.
const (
	entryVersion      = 1
	entryMinorVersion = 1
	entryKey          = "core.config_entries"
)

// Entry is one declared config entry. It stays an open map because HA owns
// the schema: a field this does not know about has to survive.
type Entry = map[string]any

// Entries merges declared config entries into the registry.
//
// Unlike areas, these are reconciled rather than appended: an existing
// entry has its data keys, its identity fields and its subentries brought
// back in line with the declaration, so a re-seed repairs an entry that
// drifted.
func Entries(logger *slog.Logger, path string, declared []Entry, now time.Time) error {
	if len(declared) == 0 {
		return nil
	}

	storage, missing, err := ReadStorage(path)
	if err != nil {
		return err
	}
	if missing {
		storage = Storage{
			"version":       entryVersion,
			"minor_version": entryMinorVersion,
			"key":           entryKey,
			"data":          map[string]any{"entries": []any{}},
		}
	}

	stamp := Timestamp(now)
	entries := rows(storage, "entries")

	byID := make(map[string]map[string]any, len(entries))
	for _, row := range entries {
		if m, ok := asObject(row); ok {
			byID[str(m, "entry_id")] = m
		}
	}

	changed := false
	for _, entry := range declared {
		id, _ := entry["entry_id"].(string)
		existing, found := byID[id]
		if !found {
			entries = append(entries, withDefaults(entry, stamp))
			changed = true
			logger.Info("seeded config entry",
				slog.String("domain", str(entry, "domain")),
				slog.String("entry_id", id),
			)
			continue
		}
		if reconcile(existing, entry, stamp) {
			changed = true
			logger.Info("updated config entry",
				slog.String("domain", str(entry, "domain")),
				slog.String("entry_id", id),
			)
		}
	}
	if !changed {
		return nil
	}

	storage.Data()["entries"] = entries
	return WriteStorage(path, storage)
}

// withDefaults fills in the fields HA writes itself on a new entry, so one
// seeded here is shaped like one HA created.
func withDefaults(entry Entry, stamp string) Entry {
	out := make(Entry, len(entry)+4)
	for k, v := range entry {
		out[k] = v
	}
	for key, value := range map[string]any{
		"created_at":     stamp,
		"modified_at":    stamp,
		"discovery_keys": map[string]any{},
		"subentries":     []any{},
	} {
		if _, ok := out[key]; !ok {
			out[key] = value
		}
	}
	return out
}

// reconcile brings one existing entry in line with its declaration,
// reporting whether anything changed.
func reconcile(existing, declared Entry, stamp string) bool {
	changed := false

	// Declared data keys win; keys HA added that the declaration does not
	// mention are left alone.
	if data, ok := declared["data"].(map[string]any); ok && len(data) > 0 {
		current, _ := existing["data"].(map[string]any)
		if current == nil {
			current = map[string]any{}
		}
		for key, value := range data {
			if !reflect.DeepEqual(current[key], value) {
				changed = true
			}
		}
		if changed {
			for key, value := range data {
				current[key] = value
			}
			existing["data"] = current
		}
	}

	// Identity fields the integration keys off. Syncing these is what lets
	// a re-seed repair an entry whose unique_id went wrong.
	for _, field := range []string{"unique_id", "title", "minor_version"} {
		value, ok := declared[field]
		if !ok || reflect.DeepEqual(existing[field], value) {
			continue
		}
		existing[field] = value
		changed = true
	}

	// Subentries are declared wholesale, so the declaration is
	// authoritative: added, changed and removed ones all land here.
	//
	// Removal only drops the subentry. The device and entity registry rows
	// HA would clear through async_remove_subentry stay behind and show up
	// as stale unavailable entities until deleted in the UI. Integrations
	// key entity unique_ids off their own ids rather than subentry_id, so
	// re-declaring a subentry reuses them.
	if declaredSubs, ok := declared["subentries"]; ok {
		if !sameSubentries(existing["subentries"], declaredSubs) {
			existing["subentries"] = declaredSubs
			changed = true
		}
	}

	if changed {
		existing["modified_at"] = stamp
	}
	return changed
}

// sameSubentries compares two subentry lists by subentry_id, which is how
// the original compared them: order is not significant, identity is.
func sameSubentries(current, declared any) bool {
	return reflect.DeepEqual(bySubentryID(current), bySubentryID(declared))
}

// bySubentryID indexes a subentry list by its id.
func bySubentryID(v any) map[string]any {
	list, _ := v.([]any)
	out := make(map[string]any, len(list))
	for _, item := range list {
		if m, ok := asObject(item); ok {
			out[str(m, "subentry_id")] = m
		}
	}
	return out
}
