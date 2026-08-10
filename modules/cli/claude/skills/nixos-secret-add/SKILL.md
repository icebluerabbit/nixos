---
name: nixos-secret-add
description: Add or wire a secret in this NixOS homelab using sops-nix — API keys, passwords, tokens, env files for containers or host services. Use whenever the user asks to add a secret, store an API key, create an env file with credentials, set up SOPS, or pass a credential into a container. Covers secrets vs templates, host vs user secrets, and referencing them from container host configs. Pairs with nixos-container-create and nixos-web-expose when a new or web-facing service needs credentials.
---

# Add a Secret (sops-nix)

All secrets are sops-nix, age keys (not PGP). Two stores:

| Store | File                        | Edit with                       | Decrypted by                           |
| :---- | :-------------------------- | :------------------------------ | :------------------------------------- |
| Host  | `secrets/host-secrets.yaml` | `just sops-host-secrets` (sudo) | host key `/var/lib/sops-nix/key.txt`   |
| User  | `secrets/user-secrets.yaml` | `just sops-user-secrets`        | user key `~/.config/sops/age/keys.txt` |

Container/service secrets are almost always **host** secrets.

## `secrets.*` vs `templates.*` — pick first

| Use                     | When                                                 | Reference at runtime                                     |
| :---------------------- | :--------------------------------------------------- | :------------------------------------------------------- |
| `sops.secrets.<name>`   | a single raw value (password file, key file)         | `config.sops.secrets.<name>.path` → file path            |
| `sops.templates.<name>` | a rendered env/config file mixing secrets + literals | `config.sops.templates.<name>.path` → rendered file path |

Most container `envFile`s are **templates** (multiple `KEY=value` lines).

## Procedure

### 1. Store the raw secret

```bash
just sops-host-secrets        # opens decrypted YAML in $EDITOR
```

Add e.g. `radarr-apikey: <value>`. Save & close — sops re-encrypts.

### 2. Declare it — ALL declarations live in `modules/hosts/server/security.nix`

Do not scatter secret declarations across container files.

Raw secret:

```nix
sops.secrets.radarr-apikey = { };
```

Or a rendered template (placeholder interpolation):

```nix
sops.templates."radarr-env".content = ''
  RADARR__AUTH__APIKEY=${config.sops.placeholder.radarr-apikey}
'';
```

`config.sops.placeholder.<secret-name>` is replaced at activation with the decrypted value — the literal never enters the Nix store.

### 3. Reference from the container host config (`modules/hosts/server/containers/<name>.nix`)

```nix
containerConfigs.radarr.envFile = config.sops.templates."radarr-env".path;
# or a raw file option:
someOption = config.sops.secrets.radarr-apikey.path;
```

Make sure the host container aspect `includes` `den.aspects.hosts._.server._.security`.

### 4. Passing secrets INTO an nspawn container

`envFile` is the usual path (read by the service). For files that must appear inside the container, prefer systemd credentials over plain bind mounts:

```nix
# host side
extraFlags = [ "--load-credential=pgadmin-password:${cfg.initialPasswordFile}" ];
# container reads:  /run/host/credentials/pgadmin-password
```

This avoids leaking raw secret strings into store closures.

### 5. Apply & verify

```bash
nix fmt
just eval-server
```

After deploy, confirm the rendered file exists (without printing the secret):

```
container_exec <name> "test -f <path-in-container> && echo present"
```

## Gotchas

- Editing the YAML requires your age key be a recipient (already configured for host/user keys).
- `validateConfig = false` on a Vector/logging sidecar when secrets arrive via runtime env vars (build-time validation can't see them).
- Never `cat` a decrypted secret into the transcript. Test existence, not contents.
