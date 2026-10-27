package seed

import (
	"log/slog"
)

// PersonStoragePath is HA's person registry.
const PersonStoragePath = "/var/lib/hass/.storage/person"

// Person is one declared person's device trackers.
type Person struct {
	ID             string   `json:"id"`
	DeviceTrackers []string `json:"device_trackers"`
}

// Persons merges declared device trackers into existing people.
//
// People are never created here: onboarding and SSO own them, and a person
// fabricated in this file would have no credentials and no way to log in.
// A missing registry therefore means HA has not onboarded yet, and the
// right answer is to do nothing rather than write one.
func Persons(logger *slog.Logger, path string, declared []Person) error {
	if len(declared) == 0 {
		return nil
	}

	storage, missing, err := ReadStorage(path)
	if err != nil {
		return err
	}
	if missing {
		return nil
	}

	items := rows(storage, "items")
	byID := make(map[string]map[string]any, len(items))
	for _, row := range items {
		if m, ok := asObject(row); ok {
			byID[str(m, "id")] = m
		}
	}

	changed := false
	for _, want := range declared {
		person, ok := byID[want.ID]
		if !ok {
			logger.Info("person not found, skipping", slog.String("id", want.ID))
			continue
		}

		existing := stringList(person["device_trackers"])
		merged := append([]string(nil), existing...)
		for _, tracker := range want.DeviceTrackers {
			if !contains(merged, tracker) {
				merged = append(merged, tracker)
			}
		}
		if len(merged) == len(existing) {
			continue
		}

		// Assigned back as []any so the document stays uniformly decoded
		// JSON, whatever else HA put in this row.
		trackers := make([]any, 0, len(merged))
		for _, t := range merged {
			trackers = append(trackers, t)
		}
		person["device_trackers"] = trackers
		changed = true
		logger.Info("updated person device_trackers", slog.String("id", want.ID))
	}
	if !changed {
		return nil
	}

	return WriteStorage(path, storage)
}

// stringList reads a JSON array of strings, tolerating a null or an absent
// field — a person with no trackers yet has either.
func stringList(v any) []string {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
