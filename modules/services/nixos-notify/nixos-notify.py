#!/usr/bin/env python3
"""Report a host lifecycle event to #nixos by publishing to MQTT; HA consumes it
via an mqtt trigger. Publishing to the broker (up across HA restarts) is what
fixes the old webhook race.
Usage: nixos-notify.py <mosquitto_pub> <host> <port> <topic> <event> [arg] [arg2]"""
import json, os, subprocess, sys, time

pub, mhost, mport, topic = sys.argv[1:5]
event = sys.argv[5]
arg = sys.argv[6] if len(sys.argv) > 6 else None  # switch: flake-rev; scrub: mount
arg2 = sys.argv[7] if len(sys.argv) > 7 else None  # switch: new system path
host = os.uname().nodename
STATE = "/var/lib/nixos-notify/last-system"


def post(payload):
    body = json.dumps(payload)
    for _ in range(30):  # tolerate the broker still starting after a boot
        r = subprocess.run(
            [pub, "-h", mhost, "-p", mport, "-q", "1", "-t", topic, "-m", body]
        )
        if r.returncode == 0:
            return
        time.sleep(2)
    print("nixos-notify: broker unreachable, giving up", file=sys.stderr)


def run(*cmd):
    return subprocess.run(
        cmd, capture_output=True, text=True, env={**os.environ, "NO_COLOR": "1"}
    ).stdout.strip()


def boot():
    failed = run("systemctl", "--failed", "--no-legend", "--plain")
    return {
        "event": "boot", "host": host, "kernel": os.uname().release,
        "degraded": len(failed.splitlines()) if failed else 0,
    }


def switch():
    # The activation script passes the incoming system path explicitly ($systemConfig);
    # /run/current-system is not swapped until later, so it can't be trusted here.
    new = arg2 or os.path.realpath("/run/current-system")
    old = open(STATE).read().strip() if os.path.exists(STATE) else ""
    if not old or old == new:  # first run, or a plain boot: seed + skip
        os.makedirs(os.path.dirname(STATE), exist_ok=True)
        open(STATE, "w").write(new)
        return None

    gen = os.readlink("/nix/var/nix/profiles/system").removeprefix("system-").removesuffix("-link")
    nixos = open(f"{new}/nixos-version").read().strip()
    kernel = os.listdir(f"{new}/kernel-modules/lib/modules")[0]
    reboot = any(
        os.readlink(f"/run/booted-system/{f}") != os.readlink(f"{new}/{f}")
        for f in ("initrd", "kernel", "kernel-modules")
    )
    diff = run("nvd", "--color", "never", "diff", old, new)
    open(STATE, "w").write(new)
    return {
        "event": "switch", "host": host, "rev": arg or "dirty",
        "gen": gen, "nixos": nixos, "kernel": kernel,
        "reboot": reboot, "diff": diff,
    }


def scrub():
    mount = arg or "/"
    text = run("btrfs", "scrub", "status", mount)
    return {"event": "scrub", "host": host, "mount": mount,
            "errors": "no errors found" not in text, "text": text}


def gc():
    log = run("journalctl", "-u", "nix-gc.service", "-o", "cat", "-n", "50")
    freed = next((ln for ln in reversed(log.splitlines()) if "store paths deleted" in ln), "")
    return {"event": "gc", "host": host, "freed": freed}


payload = {"boot": boot, "switch": switch, "scrub": scrub, "gc": gc}[event]()
if payload:
    post(payload)
