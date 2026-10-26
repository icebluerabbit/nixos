/*
  gh — GitHub CLI
  https://cli.github.com
*/
_:
let
  # The GitHub login that owns this repo, and the account
  # github-access-token-nixos must be issued from. Deliberately not
  # config.home.username: the local account is `zms`, which made `gh auth
  # status` report "account zms" while the token actually belonged to a
  # different GitHub user — gh believed a login that GitHub had never heard
  # of. There is more than one account behind this email, so the two names
  # cannot be assumed equal.
  githubUser = "icebluerabbit";
in
{
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
              user: ${githubUser}
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
