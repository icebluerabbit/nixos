/*
  git — distributed version control system
  https://git-scm.com
*/
_: {
  den.aspects.cli._.git = {
    aspect-name = "cli.git";
    homeManager =
      { config, pkgs, ... }:
      let
        github-helper = pkgs.writeShellScript "github-credential-helper" ''
          echo username=oauth2
          echo password=$(cat ${config.sops.secrets.github-access-token-nixos.path})
        '';
      in
      {
        sops.secrets.github-access-token-nixos = { };

        programs.git = {
          enable = true;
          settings = {
            # The empty first entry is load-bearing: git accumulates
            # credential.helper across config scopes, and an empty value
            # resets the list built up so far. nixpkgs' git on darwin ships
            # `credential.helper = osxkeychain` in its own /etc/gitconfig,
            # which is system scope and therefore read first — a stale
            # keychain entry answered for github.com and the push was
            # rejected as the wrong account, without this helper ever being
            # consulted. Linux git has no such entry, so only darwin saw it.
            credential."https://github.com".helper = [
              ""
              "${github-helper}"
            ];
          };
        };

        home.file.".gitconfig".text = ''
          [include]
            path = ~/.config/git/config
        '';
      };
  };
}
