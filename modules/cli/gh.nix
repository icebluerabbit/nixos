/*
  gh — GitHub CLI
  https://cli.github.com
*/
_: {
  den.aspects.cli._.gh = {
    aspect-name = "cli.gh";
    homeManager =
      { config, ... }:
      {
        sops.secrets.github-access-token-nixos = { };

        sops.templates."gh-hosts.yml" = {
          mode = "0400";
          content = ''
            github.com:
              oauth_token: ${config.sops.placeholder.github-access-token-nixos}
              user: ${config.home.username}
              git_protocol: https
          '';
          path = "${config.home.homeDirectory}/.config/gh/hosts.yml";
        };

        programs.gh = {
          enable = true;
          gitCredentialHelper.enable = false;
          settings = {
            git_protocol = "https";
            prompt = "enabled";
          };
        };
      };
  };
}
