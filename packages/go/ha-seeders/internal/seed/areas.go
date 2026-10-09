package seed

import (
	"log/slog"
)

// AreaStoragePath is HA's area registry.
const AreaStoragePath = "/var/lib/hass/.storage/core.area_registry"

// The registry's schema version, used only when the file does not exist yet
// — HA owns it from then on.
const (
	areaVersion      = 3
	areaMinorVersion = 7
	areaKey          = "core.area_registry"
)

// Area is one declared area.
type Area struct {
	AreaID string  `json:"area_id"`
	Name   string  `json:"name"`
	Icon   *string `json:"icon"`
}

// Areas merges declared areas into the registry.
//
// Append-only: an area already there is left exactly as it is, because a
// person may have renamed it, given it a floor or moved devices into it,
// and none of that is expressed in the declaration.
func Areas(logger *slog.Logger, path string, declared []Area) error {
	if len(declared) == 0 {
		return nil
	}

	storage, missing, err := ReadStorage(path)
	if err != nil {
		return err
	}
	if missing {
		storage = Storage{
			"version":       areaVersion,
			"minor_version": areaMinorVersion,
			"key":           areaKey,
			"data":          map[string]any{"areas": []any{}},
		}
	}

	areas := rows(storage, "areas")
	existing := make(map[string]bool, len(areas))
	for _, row := range areas {
		if m, ok := asObject(row); ok {
			existing[str(m, "id")] = true
		}
	}

	changed := false
	for _, area := range declared {
		if existing[area.AreaID] {
			continue
		}
		areas = append(areas, map[string]any{
			"aliases":  []any{},
			"id":       area.AreaID,
			"floor_id": nil,
			"icon":     iconValue(area.Icon),
			"labels":   []any{},
			"name":     area.Name,
			"picture":  nil,
		})
		changed = true
		logger.Info("seeded area",
			slog.String("name", area.Name),
			slog.String("area_id", area.AreaID),
		)
	}
	if !changed {
		return nil
	}

	storage.Data()["areas"] = areas
	return WriteStorage(path, storage)
}

// iconValue keeps an unset icon as JSON null rather than an empty string,
// which is what HA writes for an area with no icon.
func iconValue(icon *string) any {
	if icon == nil {
		return nil
	}
	return *icon
}
