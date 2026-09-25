# Vendor hashes, one per Go module, keyed by its directory under packages/go.
#
# Kept apart from default.nix because `just go-update` rewrites this file
# mechanically: it sets each entry to a known-wrong hash, builds, and copies the
# hash Nix reports back in. A generated table has no business living inside
# hand-written logic.
#
# A module with no external dependencies uses null rather than a hash.
{
  lib = "sha256-QoDnrjl6wZAyaFxJb/f3/AwaxjzGf2f/O82YLy3eNG8=";
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
  slskd-prune = "sha256-3ytzSjsWUrIpsqUWLZqNFGSgSJO5xpFMcT89ItiKdrw=";
}
