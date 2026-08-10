"""
soularr reclaim — hand stranded Soulseek album folders back to Lidarr, one
at a time.

A soularr run only imports what it grabbed in that same run: it tags each
downloaded album with the Lidarr metadata it already knows, then calls
DownloadedAlbumsScan on that one folder. Any album stranded by a crash, an
NFS stall, or an earlier whole-list sweep therefore sits in the download dir
forever — real music that nothing will ever look at again.

This walks the download dir and replays the second half of that flow (the
scan) per album folder. It cannot replay the first half: the album identity
soularr had at grab time is gone, so Lidarr must recognise each folder from
its existing tags and name alone. Well-tagged rips import; the rest stay put
and are reported, not moved or deleted.

The parent dir is NOT a valid scan target — Lidarr finds no audio files
directly inside it and returns "Failed to import" in ~13ms. One POST per
album folder is the only shape that works.

Safety rails:
  - failed_imports/ is skipped (soularr already denylisted those; nothing
    retries them).
  - Folders touched within RECLAIM_MIN_AGE_MINUTES are skipped — a
    multi-file album may still be landing, and slskd writes completed files
    into this dir one by one.
  - Folders with no audio file are skipped (artwork/nfo leftovers).
  - Read-only with respect to the filesystem: this process never moves or
    deletes anything. Lidarr does the moving, under its own import rules.

Env:
  LIDARR_URL               base URL, e.g. http://10.10.120.5:8686
  LIDARR_KEY_FILE          path to the raw API key (bound sops secret)
  DOWNLOAD_DIR             dir holding one subfolder per album
  RECLAIM_MIN_AGE_MINUTES  skip folders modified more recently than this
                           (default 120)
  RECLAIM_MAX_FOLDERS      stop after this many scan attempts, 0 = no cap
                           (default 0)
  RECLAIM_COMMAND_TIMEOUT  seconds to wait for one Lidarr command to settle
                           (default 120)
"""

import json
import os
import sys
import time
import urllib.error
import urllib.request

AUDIO_SUFFIXES = frozenset(
    {
        ".flac",
        ".mp3",
        ".m4a",
        ".ogg",
        ".opus",
        ".wav",
        ".aac",
        ".wma",
        ".alac",
        ".ape",
    }
)
POLL_INTERVAL_SECONDS = 2
SETTLED_STATUSES = frozenset({"completed", "failed", "aborted", "cancelled"})


def log(message):
    """One line per event — Vector ships these to Loki, so keep greppable."""
    print(message, flush=True)


def env_int(name, default):
    raw = os.environ.get(name, "").strip()
    if not raw:
        return default
    try:
        return int(raw)
    except ValueError:
        log(f"reclaim: ignoring non-integer {name}={raw!r}, using {default}")
        return default


def api_request(url, key, payload=None, timeout=30):
    """POST when given a payload, else GET. Returns the decoded JSON body."""
    data = json.dumps(payload).encode() if payload is not None else None
    request = urllib.request.Request(
        url, data=data, method="POST" if data else "GET"
    )
    request.add_header("X-Api-Key", key)
    if data:
        request.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(request, timeout=timeout) as response:
        return json.load(response)


def has_audio(folder):
    """True if the folder holds an audio file at any depth.

    Multi-disc rips nest their tracks in per-disc subfolders.
    """
    for _root, _dirs, files in os.walk(folder):
        for name in files:
            if os.path.splitext(name)[1].lower() in AUDIO_SUFFIXES:
                return True
    return False


def candidates(download_dir, min_age_seconds):
    """Album folders worth handing to Lidarr, longest-stranded first."""
    now = time.time()
    found = []
    with os.scandir(download_dir) as entries:
        for entry in entries:
            if not entry.is_dir() or entry.name == "failed_imports":
                continue
            age = now - entry.stat().st_mtime
            if age < min_age_seconds:
                log(f"reclaim: skip (too fresh, {int(age)}s) {entry.name}")
                continue
            if not has_audio(entry.path):
                log(f"reclaim: skip (no audio) {entry.name}")
                continue
            found.append((entry.stat().st_mtime, entry.path))
    return [path for _mtime, path in sorted(found)]


def scan_folder(base_url, key, folder, command_timeout):
    """
    Fire DownloadedAlbumsScan for one folder and wait for it to settle.

    Returns (imported, detail). Lidarr reports a scan that matched nothing
    as a completed command with result "unsuccessful" and message "Failed to
    import" — a normal outcome here, not an error, so it is counted rather
    than raised.
    """
    command = api_request(
        f"{base_url}/api/v1/command",
        key,
        {"name": "DownloadedAlbumsScan", "path": folder},
    )
    command_id = command["id"]

    deadline = time.time() + command_timeout
    status = command.get("status", "queued")
    while status not in SETTLED_STATUSES:
        if time.time() > deadline:
            return (
                False,
                f"timed out after {command_timeout}s (command {command_id})",
            )
        time.sleep(POLL_INTERVAL_SECONDS)
        command = api_request(f"{base_url}/api/v1/command/{command_id}", key)
        status = command.get("status", "queued")

    result = command.get("result", "")
    message = command.get("message", "")
    imported = status == "completed" and result == "successful"
    return (
        imported,
        f"{status}/{result or 'no-result'}"
        f"{f' — {message}' if message else ''}",
    )


def main():
    base_url = os.environ["LIDARR_URL"].rstrip("/")
    download_dir = os.environ["DOWNLOAD_DIR"]
    with open(os.environ["LIDARR_KEY_FILE"], encoding="utf-8") as handle:
        key = handle.read().strip()

    min_age_seconds = env_int("RECLAIM_MIN_AGE_MINUTES", 120) * 60
    max_folders = env_int("RECLAIM_MAX_FOLDERS", 0)
    command_timeout = env_int("RECLAIM_COMMAND_TIMEOUT", 120)

    folders = candidates(download_dir, min_age_seconds)
    if max_folders > 0 and len(folders) > max_folders:
        log(
            f"reclaim: capping at {max_folders} of {len(folders)} "
            "eligible folders this run"
        )
        folders = folders[:max_folders]

    log(
        f"reclaim: {len(folders)} folder(s) to hand to Lidarr "
        f"from {download_dir}"
    )

    imported_count = 0
    failed_count = 0
    for folder in folders:
        name = os.path.basename(folder)
        try:
            imported, detail = scan_folder(
                base_url, key, folder, command_timeout
            )
        except (urllib.error.URLError, OSError, KeyError, ValueError) as error:
            failed_count += 1
            log(f"reclaim: ERROR {name}: {error}")
            continue
        if imported:
            imported_count += 1
            log(f"reclaim: imported {name}")
        else:
            failed_count += 1
            log(f"reclaim: no match {name} ({detail})")

    log(
        f"reclaim: done — {imported_count} imported, "
        f"{failed_count} left in place, {len(folders)} attempted"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
