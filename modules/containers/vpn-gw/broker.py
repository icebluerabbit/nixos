#!/usr/bin/env python3
"""
vpn-gw tunnel broker — pool registry + health + provider port-forward + inbound DNAT.

Simple + trusted (internal download VLAN): no authz. A client claims a free healthy tunnel,
tells the broker its own bridge IP to DNAT inbound traffic to, and keeps the claim alive. The
broker owns selection, health (pushed by probe timers + inline natpmp failures), the provider
port lease, and the mutable nftables dnat map. Reactive: no poll loop; periodic work is pushed
in (POST /health) or poked (POST /reap).

Claims are keyed by source IP purely for idempotency (a restarting client re-converges to its
tunnel) and to give the client a clean "you lost it, re-claim" signal (409) — not for security.

FORCED ROTATION: POST /rotate {client_name} frees that client's claim and records a short-lived
exclusion (owner IP -> tunnel) so the client's NEXT claim skips the tunnel it just held (and
ignores its saved preference for it). This is pure selection steering — it does NOT mark the
tunnel unhealthy or touch flap state. The client re-claims on its own keepalive tick.

Two locks: TABLE (fast claim/health state) + LEASE (serializes route-replace + natpmpc, since
all Proton tunnels share one natpmp gateway and natpmpc can't bind an interface — we steer it
by replacing the route onto the target wg device before each lease).

WIPE ON START: a daemon-only crash leaves nft rules while memory is gone; startup flushes the
dnat map + clears state so nft and memory agree, and clients re-claim.

Config JSON ($BROKER_CONFIG): { control_port, listen_ip, ttl_seconds, fail_threshold,
  flap_window_seconds, exclude_seconds, tunnels: { wg1: {port_forward: {type, gateway?/port?}}, ... } }
Binaries via env: BROKER_NFT, BROKER_NATPMPC, BROKER_IP.
"""
import json
import os
import re
import subprocess
import threading
import time

from flask import Flask, jsonify, request

CONFIG = json.load(open(os.environ["BROKER_CONFIG"]))
NFT = os.environ.get("BROKER_NFT", "nft")
NATPMPC = os.environ.get("BROKER_NATPMPC", "natpmpc")
IP = os.environ.get("BROKER_IP", "ip")

TTL = CONFIG.get("ttl_seconds", 120)
FAIL_THRESHOLD = CONFIG.get("fail_threshold", 3)
FLAP_WINDOW = CONFIG.get("flap_window_seconds", 86400)   # rolling window for flap scoring
EXCLUDE_SECONDS = CONFIG.get("exclude_seconds", 120)     # forced-rotation exclusion lifetime
TUNNELS = CONFIG["tunnels"]   # name -> {port_forward: {...}}

# claims: tunnel -> {owner, label, ts, port, ip}   (owner = source IP; one tunnel per owner)
# health: tunnel -> {healthy, fails, exit_ip, flaps, latency}
#   flaps   = timestamps of down-transitions (rolling flap-score window)
#   latency = last probed tunnel RTT in ms (None until first probe); selection tie-breaker
claims = {}
health = {
    t: {"healthy": False, "fails": 0, "exit_ip": "", "flaps": [], "latency": None} for t in TUNNELS
}
# excludes: owner IP -> {tunnel, expiry}  — forced-rotation steering (NOT a health signal).
# Set by /rotate, honored + pruned lazily in /claim; the client's next tick lands elsewhere.
excludes = {}

TABLE = threading.Lock()   # fast: claim/health state
LEASE = threading.Lock()   # slow: route-replace + natpmpc (shared natpmp gateway)

app = Flask(__name__)


# ── nftables: mutable `dnat_map` (ifname -> ipv4) referenced by the nat prerouting hook ──
def nft(*args):
    subprocess.run([NFT, *args], check=False)


def dnat_set(tunnel, client_ip):
    nft("add", "element", "ip", "nat", "dnat_map", '{ "%s" : %s }' % (tunnel, client_ip))


def dnat_clear(tunnel):
    nft("delete", "element", "ip", "nat", "dnat_map", '{ "%s" }' % tunnel)


def dnat_flush():
    nft("flush", "map", "ip", "nat", "dnat_map")


# ── provider-specific port forward. Returns opened public port as str, or "" on failure. ──
def lease(tunnel):
    pf = TUNNELS[tunnel]["port_forward"]
    kind = pf["type"]
    if kind == "static":
        return str(pf["port"])
    if kind == "none":
        return ""
    # natmap (Proton): steer the shared natpmp gateway onto THIS wg device, then lease.
    # Serialized by LEASE so concurrent leases can't fight over the single gateway route.
    gw = pf["gateway"]
    with LEASE:
        subprocess.run([IP, "route", "replace", gw, "dev", tunnel], check=False)
        port = ""
        for proto in ("udp", "tcp"):   # BitTorrent needs both; Proton maps the same port
            try:
                out = subprocess.run(
                    [NATPMPC, "-a", "1", "0", proto, "60", "-g", gw],
                    capture_output=True, text=True, timeout=15,
                ).stdout
                m = re.search(r"Mapped public port (\d+)", out)
                if m:
                    port = m.group(1)
            except Exception:
                pass
    return port


# ── flap scoring: count of down-transitions in the last FLAP_WINDOW (under TABLE) ──
# In-memory only (resets on broker/container restart — by design). Prunes lazily on read,
# so it's a true rolling window with no decay timer. Steady state (no flaps) → 0 for all.
def flap_score(tunnel):
    cutoff = time.time() - FLAP_WINDOW
    f = health[tunnel]["flaps"]
    f[:] = [ts for ts in f if ts >= cutoff]
    return len(f)


# ── forced-rotation exclusion (under TABLE): is `tunnel` off-limits for `ip` right now? ──
# Prunes lazily on read (expired → gone), so a single-shot rotation self-clears with no timer.
def excluded(ip, tunnel):
    e = excludes.get(ip)
    if not e:
        return False
    if time.time() >= e["expiry"]:
        del excludes[ip]
        return False
    return e["tunnel"] == tunnel


# ── health folding (under TABLE) ──────────────────────────────────────────────
def mark_up(tunnel, exit_ip):
    h = health[tunnel]
    h["fails"] = 0
    h["healthy"] = True
    if exit_ip:
        h["exit_ip"] = exit_ip


def mark_down(tunnel):
    # Hysteresis: cross to unhealthy only after FAIL_THRESHOLD consecutive fails (absorbs probe
    # blips + wg-watchdog restarts). On crossing, void the claim so the client fails over.
    h = health[tunnel]
    h["fails"] += 1
    if h["fails"] >= FAIL_THRESHOLD and h["healthy"]:
        h["healthy"] = False
        h["flaps"].append(time.time())   # one flap per outage episode (the crossing), not per probe
        if tunnel in claims:
            dnat_clear(tunnel)
            del claims[tunnel]


def _lease_and_reply(tunnel, ip):
    port = lease(tunnel)   # slow path, LEASE-serialized inside
    with TABLE:
        c = claims.get(tunnel)
        if not c or c["owner"] != ip:   # voided/stolen while we were leasing
            return ("claim lost", 409)
        if not port:
            mark_down(tunnel)   # natpmp failure is itself a health signal
            return ("port-forward failed", 502)
        mark_up(tunnel, health[tunnel]["exit_ip"])
        c["port"] = port
        c["ts"] = time.time()   # dnat + renew both refresh the claim TTL
        return jsonify(port=port, exit_ip=health[tunnel]["exit_ip"])


# ── RPC (no authz; internal trusted VLAN) ─────────────────────────────────────
@app.post("/claim")
def claim():
    ip = request.remote_addr
    body = request.json or {}
    label = body.get("client_name", "") if request.is_json else ""
    pref = body.get("prefer", "") if request.is_json else ""
    with TABLE:
        for t, c in claims.items():   # idempotent per source IP → restart re-converges
            if c["owner"] == ip:
                return jsonify(tunnel=t)
        # soft preference: after a broker WIPE-ON-START (or host reboot) all claims are gone
        # and clients re-converge. A client persists its last tunnel and re-requests it here so
        # its exit IP survives the wipe — no needless rotation (which for IP/ASN-locked trackers
        # costs a re-lock). Best-effort: granted only if the tunnel is healthy + free right now;
        # otherwise fall through to normal selection. Never yanks another owner's claim.
        if (pref in TUNNELS and health[pref]["healthy"] and pref not in claims
                and not excluded(ip, pref)):
            claims[pref] = {"owner": ip, "label": label, "ts": time.time(), "port": "", "ip": ""}
            return jsonify(tunnel=pref)
        # soft flap-avoidance: among healthy + unclaimed, pick the least-flappy tunnel. Candidates
        # are built in TUNNELS order, so min() breaks score ties by that order (steady state, all
        # scores 0 → first-in-order, identical to the old behavior). A claimed flappy tunnel is
        # never yanked — flaps only steer the NEXT claim away from it.
        candidates = [
            t for t in TUNNELS
            if health[t]["healthy"] and t not in claims and not excluded(ip, t)
        ]
        if not candidates:
            # exclusion would strand the client (no other healthy+free tunnel) — a tunnel beats
            # none (killswitch), so drop the exclusion for this pick rather than return 503.
            candidates = [t for t in TUNNELS if health[t]["healthy"] and t not in claims]
        if candidates:
            # primary: fewest flaps; tie-break: lowest RTT (unprobed → inf, sorts last).
            def rank(t):
                lat = health[t]["latency"]
                return (flap_score(t), lat if lat is not None else float("inf"))

            t = min(candidates, key=rank)
            claims[t] = {"owner": ip, "label": label, "ts": time.time(), "port": "", "ip": ""}
            return jsonify(tunnel=t)
    return ("no tunnel available", 503)


@app.post("/dnat/<tunnel>")
def dnat(tunnel):
    ip = request.remote_addr
    target = (request.json or {}).get("ip", "") if request.is_json else ""
    with TABLE:
        c = claims.get(tunnel)
        if not c or c["owner"] != ip:
            return ("not your claim", 409)
        if not target:
            return ("missing ip", 400)
        c["ip"] = target
        dnat_set(tunnel, target)   # catch-all inbound wgN -> this client's bridge IP
    return _lease_and_reply(tunnel, ip)


@app.post("/renew/<tunnel>")
def renew(tunnel):
    ip = request.remote_addr
    with TABLE:
        c = claims.get(tunnel)
        if not c or c["owner"] != ip:
            return ("not your claim", 409)   # post-wipe / lost → client re-claims
    return _lease_and_reply(tunnel, ip)


@app.post("/release/<tunnel>")
def release(tunnel):
    ip = request.remote_addr
    with TABLE:
        c = claims.get(tunnel)
        if c and c["owner"] == ip:
            dnat_clear(tunnel)
            del claims[tunnel]
    return ("", 204)


@app.post("/rotate")
def rotate():
    # Force a client onto a fresh tunnel WITHOUT any health signal. Keyed by client_name (the
    # stored label), since the caller (e.g. a Home Assistant automation) knows the client, not
    # its current tunnel or source IP. Frees the claim + records an exclusion so the client's
    # next keepalive tick re-claims elsewhere. Health/flap state is left untouched.
    body = request.json or {}
    name = body.get("client_name", "") if request.is_json else ""
    if not name:
        return ("missing client_name", 400)
    with TABLE:
        target = next((t for t, c in claims.items() if c["label"] == name), None)
        if not target:
            return ("no claim for client", 404)
        owner = claims[target]["owner"]
        dnat_clear(target)
        del claims[target]
        excludes[owner] = {"tunnel": target, "expiry": time.time() + EXCLUDE_SECONDS}
    return jsonify(rotated=target)


@app.post("/health")
def health_push():
    d = request.json or {}
    tunnel, up, exit_ip = d.get("tunnel"), d.get("up"), d.get("exit_ip", "")
    lat = d.get("latency")   # tunnel RTT in ms, or null if the ping failed
    if tunnel not in TUNNELS:
        return ("unknown tunnel", 404)
    with TABLE:
        if up:
            mark_up(tunnel, exit_ip)
        else:
            mark_down(tunnel)
        if lat is not None:
            health[tunnel]["latency"] = lat
    return ("", 204)


@app.post("/reap")
def reap():
    now = time.time()
    with TABLE:
        for t in [t for t, c in claims.items() if now - c["ts"] > TTL]:
            dnat_clear(t)
            del claims[t]
    return ("", 204)


@app.get("/ready")
def ready():
    # block-until-first-probe: healthy stays False until the first successful health push, so
    # qbt's pre-start gate waits out the initial probe instead of claiming a dead tunnel.
    with TABLE:
        ok = any(h["healthy"] for h in health.values())
    return ("ready", 200) if ok else ("not ready", 503)


@app.get("/export")
def export():
    lines = []
    with TABLE:
        for t in TUNNELS:
            h = health[t]
            c = claims.get(t)
            owner = (c["label"] or c["owner"]) if c else ""
            lines.append('vpngw_tunnel_healthy{tunnel="%s"} %d' % (t, 1 if h["healthy"] else 0))
            lines.append('vpngw_tunnel_fail_count{tunnel="%s"} %d' % (t, h["fails"]))
            lines.append('vpngw_tunnel_flap_score{tunnel="%s"} %d' % (t, flap_score(t)))
            lines.append('vpngw_tunnel_provider{tunnel="%s",type="%s"} 1'
                         % (t, TUNNELS[t]["port_forward"]["type"]))
            if h["exit_ip"]:
                lines.append('vpngw_tunnel_exit_ip{tunnel="%s",exit_ip="%s"} 1' % (t, h["exit_ip"]))
            if h["latency"] is not None:
                lines.append('vpngw_tunnel_latency_seconds{tunnel="%s"} %f' % (t, h["latency"] / 1000.0))
            if c:
                lines.append('vpngw_tunnel_claimed{tunnel="%s",owner="%s"} 1' % (t, owner))
                lines.append('vpngw_claim_age_seconds{tunnel="%s"} %d' % (t, time.time() - c["ts"]))
                if c["port"]:
                    lines.append('vpngw_tunnel_forwarded_port{tunnel="%s",owner="%s"} %s'
                                 % (t, owner, c["port"]))
    return "\n".join(lines) + "\n", 200, {"Content-Type": "text/plain"}


def main():
    dnat_flush()   # wipe-on-start: reconcile kernel nft state with our (empty) memory
    from werkzeug.serving import make_server
    srv = make_server(CONFIG.get("listen_ip", "0.0.0.0"), CONFIG["control_port"], app, threaded=True)
    srv.serve_forever()


if __name__ == "__main__":
    main()
