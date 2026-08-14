# modules/infra/networks/media.nix
{ den, lib, ... }:
(import ../_mk-network.nix { inherit den lib; }) {
  id = "media";
  resourceName = "media";
  displayName = "MEDIA";
  vlanId = 120;
  subnet = "10.10.120.0";
  domain = "media.internal";
  staticAddresses = {
    gateway = "10.10.120.1";
    server = "10.10.120.2";
    radarr = "10.10.120.4";
    lidarr = "10.10.120.5";
    readarr = "10.10.120.6";
    lazylibrarian = "10.10.120.3";
    calibre-web = "10.10.120.17";
    sonarr = "10.10.120.7";
    prowlarr = "10.10.120.8";
    youtarr = "10.10.120.9";
    jellyfin = "10.10.120.10";
    unpackerr = "10.10.120.11";
    soularr = "10.10.120.19";
    cross-seed = "10.10.120.15";
    bazarr = "10.10.120.16";
    flaresolverr = "10.10.120.13";
    autobrr = "10.10.120.14";
    beets = "10.10.120.18";
    # ⚠ Do NOT assign 10.10.120.12 to any container — reserved.
    mousetrap = "10.10.120.20";
    cleanuparr = "10.10.120.21";
    seasonpackarr = "10.10.120.22";
  };
}
