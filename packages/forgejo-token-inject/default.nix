/*
  forgejo-token-inject — splice the forgejo API token into code-server's
  settings.json at preStart.

  The token comes from a systemd credential, so the plaintext never enters
  the Nix store or the committed settings.json — it is written into the
  container's own appdata copy at runtime. Idempotent and fail-open: exits 0
  if the settings file, the credential or the instances array is missing.

  Usage: forgejo-token-inject <settings.json> <token-file>
*/
{ writers }:
writers.writePython3Bin "forgejo-token-inject" { } (builtins.readFile ./inject-forgejo-token.py)
