# Hand one finished album from beets to Lidarr for import.
#
# beets calls this once it has written an album into its output directory.
# Lidarr does the rest: it links the files into the artist folder, renames
# them to the configured format and records the extra files alongside. That
# is what keeps a CD rip and a torrent landing identically, with Lidarr as
# the only writer into the library.
#
# This asks Lidarr what it found and then tells it to import that, rather
# than asking it to scan a folder and decide for itself.
#
# The difference matters. An unattended DownloadedAlbumsScan scores the files
# against the releases Lidarr holds and refuses anything under 80%, and the
# release MBID carries weight 5.0 of roughly 23 — ten times any other field
# (CloseAlbumMatchSpecification, DistanceCalculator). A rip tagged with a
# real MusicBrainz release that Lidarr's metadata mirror happens not to hold
# therefore fails every time, however correct the rip is. The ManualImport
# command builds its decisions directly and never runs that specification, so
# identification still comes from Lidarr — it is acted on rather than
# second-guessed.
#
# Usage: beets-lidarr-import <album-directory>
# Environment: LIDARR_URL, LIDARR_API_KEY, and optionally
#              LIDARR_IMPORT_MODE, LIDARR_IMPORT_TIMEOUT, LIDARR_IMPORT_POLL
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

NAME = "beets-lidarr-import"

# Copy leaves the rip where beets put it and, with Lidarr's
# copyUsingHardlinks enabled on the same filesystem, links rather than
# duplicates it — so the rip survives as its own tree at no extra disk cost.
# That mirrors how a torrent stays in the download directory while Lidarr
# links it into the library.
DEFAULT_MODE = "Copy"
DEFAULT_TIMEOUT = 300.0
DEFAULT_POLL = 2.0


class Failure(Exception):
    """Something went wrong that the operator has to see."""


def env(key):
    value = os.environ.get(key)
    if not value:
        raise Failure(f"{key} is not set")
    return value


def number(key, default):
    raw = os.environ.get(key)
    if not raw:
        return default
    try:
        return float(raw)
    except ValueError:
        raise Failure(f"{key} is not a number: {raw!r}") from None


def call(api, key, path, params=None, body=None):
    """One Lidarr API call. POSTs when given a body, GETs otherwise."""
    url = api + path
    if params:
        url += "?" + urllib.parse.urlencode(params)

    data = json.dumps(body).encode() if body is not None else None
    request = urllib.request.Request(
        url,
        data=data,
        method="POST" if data is not None else "GET",
        headers={"X-Api-Key": key, "Content-Type": "application/json"},
    )
    with urllib.request.urlopen(request, timeout=30) as response:
        raw = response.read()
    return json.loads(raw) if raw else None


def placed(item):
    """Whether Lidarr resolved everything needed to import one file."""
    return bool(
        (item.get("artist") or {}).get("id")
        and (item.get("album") or {}).get("id")
        and item.get("albumReleaseId")
        and item.get("tracks")
    )


def unplaced_report(album, items, total):
    """Explain which files Lidarr could not place, and why."""
    lines = [
        f"Lidarr could not place {len(items)} of {total} files in "
        f"{album}; not imported"
    ]
    for item in items:
        reasons = [
            rejection.get("reason", "no reason given")
            for rejection in item.get("rejections") or []
        ] or ["no reason given"]
        name = os.path.basename(item.get("path", "?"))
        lines.append(f"  {name}: {'; '.join(reasons)}")
    return "\n".join(lines)


def import_payload(items, mode):
    """The ManualImport command for exactly what Lidarr identified.

    disableReleaseSwitching is left off so Lidarr may still adopt the release
    the files actually are, which is the point of tagging them from the disc
    in the first place.
    """
    return {
        "name": "ManualImport",
        "importMode": mode,
        "replaceExistingFiles": False,
        "files": [
            {
                "path": item["path"],
                "artistId": item["artist"]["id"],
                "albumId": item["album"]["id"],
                "albumReleaseId": item["albumReleaseId"],
                "trackIds": [track["id"] for track in item["tracks"]],
                "quality": item["quality"],
                "disableReleaseSwitching": False,
            }
            for item in items
        ],
    }


def identify(api, key, album):
    """Ask Lidarr what it makes of the folder.

    This is the same identification an unattended scan performs: it returns
    the artist, album, release and tracks resolved for every file, alongside
    any reason it would have refused to act on them.
    """
    try:
        found = call(
            api,
            key,
            "/manualimport",
            params={"folder": album, "filterExistingFiles": "false"},
        )
    except (urllib.error.URLError, OSError) as err:
        raise Failure(f"could not reach Lidarr at {api}: {err}") from None

    found = found or []
    if not found:
        raise Failure(f"Lidarr found no importable files in {album}")
    return found


def await_command(api, key, album, command, timeout, poll, now, sleep):
    """Wait for an import to finish, and say what it did.

    The command is asynchronous and completes successfully even when nothing
    was linked — that shows up as an unsuccessful result rather than as a
    failure. Polling for it is the difference between knowing an album was
    imported and assuming it was.
    """
    deadline = now() + timeout
    while now() < deadline:
        try:
            body = call(api, key, f"/command/{command}") or {}
        except (urllib.error.URLError, OSError) as err:
            raise Failure(
                f"lost contact with Lidarr while awaiting {command}: {err}"
            ) from None

        status = body.get("status", "unknown")
        if status == "completed":
            if body.get("result") == "successful":
                return f"imported {album}"
            # Nothing was linked, so the album stays visible in the output
            # directory as one Lidarr could not place.
            raise Failure(f"Lidarr imported nothing from {album}")
        if status in ("failed", "aborted"):
            detail = body.get("exception") or body.get(
                "message", "no detail given"
            )
            raise Failure(f"import of {album} {status}: {detail}")

        sleep(poll)

    raise Failure(
        f"gave up waiting {timeout:g}s for command {command} ({album})"
    )


def run(argv, out, now=time.monotonic, sleep=time.sleep):
    """Import one album, returning what happened."""
    if len(argv) < 2:
        raise Failure("usage: beets-lidarr-import <album-directory>")
    album = argv[1]

    api = env("LIDARR_URL").rstrip("/") + "/api/v1"
    key = env("LIDARR_API_KEY")
    mode = os.environ.get("LIDARR_IMPORT_MODE") or DEFAULT_MODE
    timeout = number("LIDARR_IMPORT_TIMEOUT", DEFAULT_TIMEOUT)
    poll = number("LIDARR_IMPORT_POLL", DEFAULT_POLL)

    found = identify(api, key, album)

    # Every file has to be placed. A file Lidarr could not resolve has no
    # track to import it as, and importing the rest would leave the album
    # silently short. Left where beets put it rather than imported somewhere
    # wrong: an album Lidarr does not track is a decision for a person.
    unresolved = [item for item in found if not placed(item)]
    if unresolved:
        raise Failure(unplaced_report(album, unresolved, len(found)))

    try:
        queued = call(api, key, "/command", body=import_payload(found, mode))
    except (urllib.error.URLError, OSError) as err:
        raise Failure(f"Lidarr refused the import of {album}: {err}") from None

    command = (queued or {}).get("id")
    if not command:
        raise Failure(f"Lidarr rejected the request for {album}: {queued}")

    print(
        f"{NAME}: queued command {command} to import "
        f"{len(found)} files from {album}",
        file=out,
    )
    return await_command(api, key, album, command, timeout, poll, now, sleep)


def main(argv=None):
    try:
        print(f"{NAME}: {run(argv or sys.argv, sys.stdout)}")
    except Failure as err:
        print(f"{NAME}: {err}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
