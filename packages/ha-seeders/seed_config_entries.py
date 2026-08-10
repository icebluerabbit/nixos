import json
import os
import pathlib
import sys
from datetime import datetime, timezone

seed_entries = json.loads(os.path.expandvars(sys.stdin.read()))

if not seed_entries:
    sys.exit(0)

storage_path = "/var/lib/hass/.storage/core.config_entries"

if not os.path.exists(storage_path):
    storage = {
        "version": 1,
        "minor_version": 1,
        "key": "core.config_entries",
        "data": {"entries": []},
    }
else:
    with open(storage_path) as f:
        storage = json.load(f)

existing_ids = {e["entry_id"] for e in storage["data"]["entries"]}
now = datetime.now(timezone.utc).isoformat()
changed = False
for entry in seed_entries:
    if entry["entry_id"] not in existing_ids:
        entry.setdefault("created_at", now)
        entry.setdefault("modified_at", now)
        entry.setdefault("discovery_keys", {})
        entry.setdefault("subentries", [])
        storage["data"]["entries"].append(entry)
        changed = True
        print(f"Seeded config entry: {entry['domain']} ({entry['entry_id']})")
    else:
        seed_data = entry.get("data", {})
        for existing in storage["data"]["entries"]:
            if existing["entry_id"] == entry["entry_id"]:
                entry_changed = False
                if seed_data and any(
                    existing.get("data", {}).get(k) != v
                    for k, v in seed_data.items()
                ):
                    existing["data"].update(seed_data)
                    entry_changed = True
                # Sync top-level identity fields the integration relies on
                # (e.g. unique_id) so a re-seed can repair a bad existing
                # entry.
                for field in ("unique_id", "title", "minor_version"):
                    if field in entry and existing.get(field) != entry[field]:
                        existing[field] = entry[field]
                        entry_changed = True
                # Subentries are declared wholesale, so the declaration is
                # authoritative: added, changed and removed ones all land here.
                # Removal only drops the subentry — the device/entity registry
                # rows HA would clear via async_remove_subentry are left behind
                # and show up as stale `unavailable` entities until deleted in
                # the UI. Integrations key entity unique_ids off their own ids
                # (not subentry_id), so re-declaring a subentry reuses them.
                if "subentries" in entry:
                    declared = {
                        s["subentry_id"]: s for s in entry["subentries"]
                    }
                    current = {
                        s["subentry_id"]: s
                        for s in existing.get("subentries", [])
                    }
                    if declared != current:
                        existing["subentries"] = entry["subentries"]
                        entry_changed = True
                if entry_changed:
                    existing["modified_at"] = now
                    changed = True
                    print(
                        "Updated config entry: "
                        f"{entry['domain']} ({entry['entry_id']})"
                    )
                break

if changed:
    pathlib.Path(storage_path).parent.mkdir(parents=True, exist_ok=True)
    with open(storage_path, "w") as f:
        json.dump(storage, f, indent=2)
