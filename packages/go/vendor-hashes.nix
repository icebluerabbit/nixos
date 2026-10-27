# Vendor hashes, one per Go module, keyed by its directory under packages/go.
#
# Kept apart from default.nix because `just go-update` rewrites this file
# mechanically: it sets each entry to a known-wrong hash, builds, and copies the
# hash Nix reports back in. A generated table has no business living inside
# hand-written logic.
#
# A module with no external dependencies uses null rather than a hash.
{
  lib = "sha256-00RWBbMAixtLDGiUN5udSKqmqvFKGN0CGxxCWfEzV+o=";
  managarr-crossseed = "sha256-8Amk4GqRINo4dWWcExCCdjnviLvPEP23Z8QkZh1za6w=";
  managarr-lidarr = "sha256-MwQqGbb6mYovPuw5RRZVcFdPXLbk7kuHUqD2QzfNBQc=";
  managarr-prowlarr = "sha256-Pb4pXSK45hmAV1E0dl931sYw1fTNHDUJO3AdFXY3H38=";
  managarr-radarr = "sha256-MwQqGbb6mYovPuw5RRZVcFdPXLbk7kuHUqD2QzfNBQc=";
  managarr-sonarr = "sha256-MwQqGbb6mYovPuw5RRZVcFdPXLbk7kuHUqD2QzfNBQc=";
  managarr-autobrr = "sha256-IxLl9GyaQLVKYRyV1v7roPoNXUJxrvf6603YO+hIVFM=";
  managarr-bazarr = "sha256-KixuQcssM6AV0Y6fRtsRXDfo8yur1NEKExbcX7WEXVk=";
  managarr-beets = "sha256-T5h80jP2VwS1wQ7UDL9k+PlW/0mM7KFZypvhK3c1Sdw=";
  managarr-qbittorrent = "sha256-ssoJoUygVMKYq0AQr3NofyID5O5KrG2mZzY1Waii03c=";
  managarr-seasonpackarr = "sha256-fekADesJbaWpfCU5WMcR55P67MJq9rOSWi4970yu3+Y=";
  managarr-slskd = "sha256-Wbee4MeHGNJH5KAOXZuFP3cRGYzdThPzne0gzkAnni8=";
  managarr-soularr = "sha256-fekADesJbaWpfCU5WMcR55P67MJq9rOSWi4970yu3+Y=";
  managarr-unpackerr = "sha256-fekADesJbaWpfCU5WMcR55P67MJq9rOSWi4970yu3+Y=";
  managarr = "sha256-oTavRptRfMkbEbcgagMmxrXUXueyIzyAKhcbZeQmo3U=";
  beets-lidarr-import = "sha256-RM3rU5wCl1BeiScBtYQ1zFbyCPjfH3P++kiarJIUlG8=";
  flac-util = "sha256-l5WiHLJ0O8AIQSQ989v7mVIKUvvmSRb4jUr5gW/BrOQ=";
  forgejo-token-inject = null;
  ha-seeders = "sha256-X9zdC6jZU5JtJmNiHKaZ3vHZVAafkzUpgJDoqT3WTM8=";
  hermes-resolve-rooms = null;
  jellyfin-refresh = "sha256-nF89xcNeGhZ9AL+GFHyAv61TOrDYx/aNNaDNHz3w+uE=";
  mousetrap = "sha256-5SONJxLMKijEPN8CGs67L5it9cYMzaBJidV5BTAO/m8=";
  nixos-notify = null;
  qbt-scripts = "sha256-FWzrYXHHQaJU2Lb1ErCYqhRu0wY1B/C8LYklnEOi9ck=";
  qui-provision = null;
  slskd-prune = "sha256-3ytzSjsWUrIpsqUWLZqNFGSgSJO5xpFMcT89ItiKdrw=";
  soularr-reclaim = "sha256-X9zdC6jZU5JtJmNiHKaZ3vHZVAafkzUpgJDoqT3WTM8=";
  sys-mcp = "sha256-eLYqsHfexYojdAmJoTPXS/OkZZzMtUmxdAdQo2f8vuE=";
  thelounge-seed = "sha256-AkXkwaOdeG6anuiVPCeOcXE/U87gAW1YBNzv0TxEYHI=";
  topology = "sha256-X9zdC6jZU5JtJmNiHKaZ3vHZVAafkzUpgJDoqT3WTM8=";
  tuwunel-init = null;
}
