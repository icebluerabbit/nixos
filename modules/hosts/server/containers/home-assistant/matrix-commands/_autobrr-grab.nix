# 👍 on a card in #autobrr fires the matrix_command `autobrr_grab`, handled by the
# grab branch in automations/_autobrr-webhook.nix. The reaction key must match the
# EXACT string the client sends — Element appends a U+FE0F variation selector, so
# this is 👍 (U+1F44D) + U+FE0F, not the bare 👍.
{
  name = "autobrr_grab";
  reaction = "👍️";
  rooms = [ "#autobrr:matrix.services.internal" ];
}
