import json, os, sys

seed_persons = json.load(sys.stdin)
if not seed_persons:
    sys.exit(0)

storage_path = "/var/lib/hass/.storage/person"
# Persons are created by onboarding/auth (SSO); never fabricate them here.
if not os.path.exists(storage_path):
    sys.exit(0)

with open(storage_path) as f:
    storage = json.load(f)

by_id = {p["id"]: p for p in storage["data"]["items"]}
changed = False
for seed in seed_persons:
    person = by_id.get(seed["id"])
    if person is None:
        print(f"Person not found, skipping: {seed['id']}")
        continue
    existing = person.get("device_trackers") or []
    merged = existing + [t for t in seed.get("device_trackers", []) if t not in existing]
    if merged != existing:
        person["device_trackers"] = merged
        changed = True
        print(f"Updated person device_trackers: {seed['id']}")

if changed:
    with open(storage_path, "w") as f:
        json.dump(storage, f, indent=2)
