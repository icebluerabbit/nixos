# Vendor hashes, one per Go module, keyed by its directory under packages/go.
#
# Kept apart from default.nix because `just go-update` rewrites this file
# mechanically: it sets each entry to a known-wrong hash, builds, and copies the
# hash Nix reports back in. A generated table has no business living inside
# hand-written logic.
#
# A module with no external dependencies uses null rather than a hash.
{
  lib = "sha256-Rfb3GREwtnzo/mdkiu6UklcprBB6fUTV1r2LmKwD1Yg=";
  cleanuparr-setup = "sha256-S6w4p2a5cZQfBjnMy7ZRB3ZZCcuTws2uVO8bmscorZU=";
}
