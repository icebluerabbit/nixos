"""Merge declared IRC networks into thelounge per-user JSON.

Only the ``.networks`` key is touched — sessions, clientSettings and any other
per-user state are preserved. Network UUIDs are derived deterministically from
(user, network name) so re-running never re-creates networks (keeps scrollback,
which thelounge keys by uuid). Nicks are read from bind-mounted sops secret
files at runtime, so they never enter the Nix store.

Env:
  THELOUNGE_SEED_FILE  JSON list of {name, networks:[{name,host,port,tls,
                       rejectUnauthorized,nickPath,channels,sasl,
                       saslAccountPath,saslPasswordPath}]}
  THELOUNGE_USERS_DIR  users dir (default /var/lib/thelounge/users)
"""

import json
import os
import uuid

users = json.load(open(os.environ["THELOUNGE_SEED_FILE"]))
users_dir = os.environ.get("THELOUNGE_USERS_DIR", "/var/lib/thelounge/users")
os.makedirs(users_dir, exist_ok=True)

for u in users:
    name = u["name"]
    nets = []
    for n in u.get("networks", []):
        net = {
            "uuid": str(
                uuid.uuid5(uuid.NAMESPACE_URL, f"thelounge/{name}/{n['name']}")
            ),
            "name": n["name"],
            "host": n["host"],
            "port": n.get("port", 6697),
            "tls": n.get("tls", True),
            "rejectUnauthorized": n.get("rejectUnauthorized", True),
            "channels": [{"name": c} for c in n.get("channels", [])],
        }
        if n.get("nickPath"):
            net["nick"] = open(n["nickPath"]).read().strip()
        if n.get("passwordPath"):
            net["password"] = open(n["passwordPath"]).read().strip()
        if n.get("sasl"):
            net["sasl"] = n["sasl"]
            if n.get("saslAccountPath"):
                net["saslAccount"] = open(n["saslAccountPath"]).read().strip()
            if n.get("saslPasswordPath"):
                net["saslPassword"] = (
                    open(n["saslPasswordPath"]).read().strip()
                )
        cmd_secrets = {
            k: open(v).read().strip()
            for k, v in n.get("commandsSecretPaths", {}).items()
        }
        cmds = [c.format(**cmd_secrets) for c in n.get("commands", [])]
        if cmds:
            net["commands"] = cmds
        nets.append(net)
    path = os.path.join(users_dir, f"{name}.json")
    try:
        data = json.load(open(path))
    except FileNotFoundError:
        data = {"password": "", "log": True}
    data["networks"] = nets
    cs = u.get("clientSettings")
    if cs:
        data.setdefault("clientSettings", {}).update(cs)
    tmp = path + ".tmp"
    json.dump(data, open(tmp, "w"), indent=2)
    os.replace(tmp, path)
    print(f"seeded {name}: {len(nets)} networks")
