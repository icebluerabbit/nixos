# FlareSolverr indexer proxy — applied to indexers sharing the "cloudflare"
# tag (e.g. TorrentLeech) to solve CF challenges.
{ config, ... }:
{
  name = "flaresolverr";
  implementation = "FlareSolverr";
  config_contract = "FlareSolverrSettings";
  tags = [ "\${ref.tag.cloudflare}" ];
  fields = {
    host = "http://${config.infra.clients.container-flaresolverr.ip}:${toString config.infra.clients.container-flaresolverr.ports.default}";
    requestTimeout = 60;
  };
}
