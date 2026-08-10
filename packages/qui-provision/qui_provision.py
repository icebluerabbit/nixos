# Idempotently register the qBittorrent instance in qui via its REST API.
#
# qui keeps instances in its SQLite DB — there is NO declarative config for
# them, so we provision over the API at startup. This works because qui runs
# with authDisabled=true: IsAuthenticated injects a synthetic "admin" and
# RequireSetup is skipped, so POST /api/instances needs no login/session. The
# call originates from 127.0.0.1, which must be in authDisabledAllowedCIDRs
# (the allowlist is checked against the direct TCP peer, before RealIP).
#
# qbittorrent's WebUI has no auth for the apps subnet (authSubnetWhitelist), so
# the instance is created with empty credentials.
#
# Usage: qui_provision.py <api-base> <qbt-url> <name>
import json
import sys
import time
import urllib.error
import urllib.request

api, qbt_url, name = sys.argv[1:4]


def call(method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(
        api + path,
        data=data,
        method=method,
        headers={"Content-Type": "application/json"},
    )
    with urllib.request.urlopen(req, timeout=10) as resp:
        raw = resp.read()
        return resp.status, (json.loads(raw) if raw else None)


# Poll the real endpoint until qui's listener is up (qui.service is
# Type=simple).
instances = None
for _ in range(60):
    try:
        _, instances = call("GET", "/api/instances")
        break
    except Exception:
        time.sleep(2)

if instances is None:
    print(
        "qui-provision: API never became ready; skipping (fail-open)",
        flush=True,
    )
    sys.exit(0)

# Idempotent: bail if an instance already targets this host.
for inst in instances or []:
    if inst.get("host") == qbt_url:
        print(
            f"qui-provision: instance for {qbt_url} already present, "
            "nothing to do",
            flush=True,
        )
        sys.exit(0)

try:
    status, _ = call(
        "POST",
        "/api/instances",
        {"name": name, "host": qbt_url, "username": "", "password": ""},
    )
    print(
        f"qui-provision: registered '{name}' -> {qbt_url} (HTTP {status})",
        flush=True,
    )
except urllib.error.HTTPError as e:
    # fail-open: a provisioning hiccup must not wedge the container
    print(
        f"qui-provision: create failed HTTP {e.code}: "
        f"{e.read().decode(errors='replace')}",
        flush=True,
    )
