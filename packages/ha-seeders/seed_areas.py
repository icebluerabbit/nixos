import json
import os
import pathlib
import sys

seed_areas = json.load(sys.stdin)

if not seed_areas:
    sys.exit(0)

storage_path = "/var/lib/hass/.storage/core.area_registry"

if not os.path.exists(storage_path):
    storage = {
        "version": 3,
        "minor_version": 7,
        "key": "core.area_registry",
        "data": {"areas": []},
    }
else:
    with open(storage_path) as f:
        storage = json.load(f)

existing_ids = {a["id"] for a in storage["data"]["areas"]}
changed = False
for area in seed_areas:
    if area["area_id"] not in existing_ids:
        storage["data"]["areas"].append(
            {
                "aliases": [],
                "id": area["area_id"],
                "floor_id": None,
                "icon": area.get("icon"),
                "labels": [],
                "name": area["name"],
                "picture": None,
            }
        )
        changed = True
        print(f"Seeded area: {area['name']} ({area['area_id']})")

if changed:
    pathlib.Path(storage_path).parent.mkdir(parents=True, exist_ok=True)
    with open(storage_path, "w") as f:
        json.dump(storage, f, indent=2)
