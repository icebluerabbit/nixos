"""Resolve declared Matrix room aliases to room IDs for the Hermes gateway.

Usage: resolve_rooms.py <homeserver> <env_file> <home_alias|-> [free_alias ...]

Matrix mints room IDs server-side at createRoom, so there is nothing to declare
in Nix — only the alias is knowable at eval time. Hermes takes room IDs (never
aliases) in MATRIX_HOME_ROOM and MATRIX_FREE_RESPONSE_ROOMS, so resolve them
here and append to the .env the hermes-agent activation script just wrote.

Runs on every container start, after the .env is rebuilt from scratch, so a
recreated room heals itself and nothing accumulates. Unresolved aliases are not
fatal: an absent MATRIX_HOME_ROOM only means unattended output has nowhere to
go, and an absent MATRIX_FREE_RESPONSE_ROOMS only means @mention stays
required.
"""

import json
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

RETRIES = 60
RETRY_DELAY_SECONDS = 2

homeserver, env_file, home_alias = sys.argv[1:4]
free_aliases = sys.argv[4:]


def resolve(alias):
    """Return the room ID for an alias, retrying while tuwunel creates it.

    Returns None once the retry budget is spent. The directory endpoint
    needs no auth — the same tokenless lookup tuwunel's own init.py makes.
    """
    url = (
        f"{homeserver}/_matrix/client/v3/directory/room/"
        + urllib.parse.quote(alias, safe="")
    )

    for _ in range(RETRIES):
        try:
            with urllib.request.urlopen(url) as resp:
                room_id = json.loads(resp.read() or "null").get("room_id")
            if room_id:
                return room_id
        except (urllib.error.URLError, json.JSONDecodeError, AttributeError):
            pass
        time.sleep(RETRY_DELAY_SECONDS)

    return None


def append(key, value):
    with open(env_file, "a") as f:
        f.write(f"{key}={value}\n")
    print(f"{key}={value}")


if home_alias != "-":
    room_id = resolve(home_alias)
    if room_id:
        append("MATRIX_HOME_ROOM", room_id)
    else:
        print(
            f"could not resolve {home_alias}; starting without a home room "
            "(cron and notification output will be dropped)",
            file=sys.stderr,
        )

if free_aliases:
    resolved = [room_id for a in free_aliases if (room_id := resolve(a))]
    if resolved:
        append("MATRIX_FREE_RESPONSE_ROOMS", ",".join(resolved))
    else:
        print(
            "could not resolve any free-response room; "
            "@mention stays required in every room",
            file=sys.stderr,
        )
