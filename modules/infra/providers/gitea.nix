# Gitea provider configuration
{ den, ... }:
(import ../_mk-provider.nix { inherit den; }) {
  id = "gitea";
  includes = [ den.aspects.infra._.clients._.container-gitea ];

  secrets = {
    gitea-api-token = { };
    github-access-token-nixos = { }; # reused: migration + push-mirror auth
  };

  terranix =
    { config, ... }:
    let
      gitea = config.infra.clients.container-gitea;
    in
    {
      terraform.required_providers = {
        gitea = {
          source = "go-gitea/gitea";
          version = "0.6.0";
        };
        null = {
          source = "hashicorp/null";
          version = "3.2.3";
        };
      };

      # token read from GITEA_TOKEN env (.envrc) at apply time; hit the
      # container directly to bypass Caddy/Authelia (API token auth, not a
      # forward_auth session)
      provider.gitea.base_url = "http://${gitea.ip}:${toString gitea.ports.web}";

      # github PAT, fed via TF_VAR_github_token (.envrc)
      variable.github_token = {
        type = "string";
        sensitive = true;
      };
    };
}
