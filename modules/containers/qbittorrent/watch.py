#!/usr/bin/env python3
"""Import .torrent files from watch/<category>/ into qBittorrent, tagged by category."""
import argparse
import glob
import json
import os
import shutil
import sys
import urllib.request
import uuid


def add_torrent(api, torrent_path, category):
    boundary = uuid.uuid4().hex
    with open(torrent_path, "rb") as f:
        content = f.read()

    def field(name, value):
        return (
            f"--{boundary}\r\n"
            f'Content-Disposition: form-data; name="{name}"\r\n\r\n'
            f"{value}\r\n"
        ).encode()

    body = field("category", category) + field("autoTMM", "true")
    body += (
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="torrents"; '
        f'filename="{os.path.basename(torrent_path)}"\r\n'
        f"Content-Type: application/x-bittorrent\r\n\r\n"
    ).encode()
    body += content + b"\r\n"
    body += f"--{boundary}--\r\n".encode()

    req = urllib.request.Request(
        f"{api}/api/v2/torrents/add",
        data=body,
        method="POST",
        headers={"Content-Type": f"multipart/form-data; boundary={boundary}"},
    )
    with urllib.request.urlopen(req, timeout=15) as resp:
        raw = resp.read().decode().strip()
    if raw.lower() == "ok.":  # qBittorrent < 5.1
        return True
    try:
        return json.loads(raw).get("success_count", 0) >= 1
    except Exception:
        return False


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--api", required=True)
    p.add_argument("--watch-dir", required=True)
    p.add_argument("--backup-dir", required=True)
    p.add_argument("--categories", required=True)
    args = p.parse_args()

    cats = [c for c in args.categories.split(",") if c]
    os.makedirs(args.backup_dir, exist_ok=True)

    for cat in cats:
        for tf in glob.glob(os.path.join(args.watch_dir, cat, "*.torrent")):
            try:
                ok = add_torrent(args.api, tf, cat)
            except Exception as e:
                print(f"watch: add failed {tf}: {e}", file=sys.stderr)
                continue
            if not ok:
                print(f"watch: add rejected {tf}", file=sys.stderr)
                continue
            shutil.move(tf, os.path.join(args.backup_dir, os.path.basename(tf)))
            print(f"watch: added {os.path.basename(tf)} category={cat}")


if __name__ == "__main__":
    main()
