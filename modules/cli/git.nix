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
            credential."https://github.com".helper = "${github-helper}";
          };
        };

        home.file.".gitconfig".text = ''
          [include]
            path = ~/.config/git/config
        '';
      };
  };
}
