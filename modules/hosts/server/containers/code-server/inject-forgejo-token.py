#!/usr/bin/env python3
"""Splice the forgejo API token into code-server's settings.json at preStart.

The token comes from a systemd credential (LoadCredential), so the plaintext
never enters the Nix store or the committed settings.json — it is written into
the container's own appdata copy at runtime.

It goes onto each `forgejo.instances[].token` (NOT the top-level `forgejo.token`):
forgejo-vscode only consumes the top-level key via its legacy single-instance
migration, which is skipped whenever `forgejo.instances` is already populated —
as it is here. The path that actually runs (`migrateTokensToSecretStorage`)
reads the per-instance `token` field and stores it in SecretStorage keyed by the
instance id. So the token must ride on the instance object.

Idempotent: safe to run on every start. No-ops (exit 0) if the settings file,
the credential, or a non-empty instances array is missing, so a first boot
before the secret exists still comes up cleanly.

Usage: inject-forgejo-token.py <settings.json> <token-file>
"""
import json
import os
import sys
import tempfile


def main() -> int:
    settings_path, token_path = sys.argv[1], sys.argv[2]

    if not os.path.isfile(settings_path) or not os.path.isfile(token_path):
        return 0

    with open(token_path, encoding="utf-8") as f:
        token = f.read().strip()
    if not token:
        return 0

    with open(settings_path, encoding="utf-8") as f:
        settings = json.load(f)

    instances = settings.get("forgejo.instances")
    if not isinstance(instances, list) or not instances:
        return 0  # nothing to attach the token to

    changed = False
    for inst in instances:
        if isinstance(inst, dict) and inst.get("token") != token:
            inst["token"] = token
            changed = True
    if not changed:
        return 0  # already current, nothing to write

    # Atomic replace so a crash mid-write can't leave truncated JSON. Preserve the
    # original file's mode (mkstemp defaults to 0600) so the seeded 0644 — the mode
    # the code-server service user reads it under — is not narrowed.
    mode = os.stat(settings_path).st_mode
    d = os.path.dirname(settings_path)
    fd, tmp = tempfile.mkstemp(dir=d, prefix=".settings.", suffix=".json")
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as f:
            json.dump(settings, f, indent=2)
            f.write("\n")
        os.chmod(tmp, mode)
        os.replace(tmp, settings_path)
    except BaseException:
        os.unlink(tmp)
        raise
    return 0


if __name__ == "__main__":
    sys.exit(main())
