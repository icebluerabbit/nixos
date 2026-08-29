# 👍 on an unsubscribable card in #inbox fires the matrix_command `mail_unsub`,
# handled by automations/_mail-unsubscribe.nix. Same reaction glyph as the autobrr
# grab — 👍 (U+1F44D) + U+FE0F, the exact string Element sends — but the two never
# collide because commands are room-scoped and this one only listens in #inbox.
{
  name = "mail_unsub";
  reaction = "👍️";
  rooms = [ "#inbox:matrix.services.internal" ];
}
