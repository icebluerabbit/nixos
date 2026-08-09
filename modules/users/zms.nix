# zms — configuration for the primary system user profile
{ den, ... }:
{
  den.aspects.users._.zms = {
    aspect-name = "users.zms";
    includes = [
      den.provides.define-user
      den.provides.primary-user
      (den.provides.user-shell "zsh")
      den.aspects.security._.sops
      den.aspects.cli._.btop
      den.aspects.cli._.bat
      den.aspects.cli._.claude
      den.aspects.cli._.mcp
      den.aspects.cli._.direnv
      den.aspects.cli._.eza
      den.aspects.cli._.fzf
      den.aspects.cli._.gh
      den.aspects.cli._.git
      den.aspects.cli._.ssh
      den.aspects.cli._.just
      den.aspects.cli._.lazygit
      den.aspects.cli._.macchina
      den.aspects.cli._.neovim
      den.aspects.cli._.nh
      den.aspects.cli._.oh-my-posh
      den.aspects.cli._.ripgrep
      den.aspects.cli._.television
      den.aspects.cli._.tmux
      den.aspects.cli._.yazi
      den.aspects.cli._.zoxide
      den.aspects.cli._.zsh
      den.aspects.groups._.nixos-config
      den.aspects.dev._.nixd
      den.aspects.dev._.manix
      den.aspects.style._.stylix
    ];

    nixos =
      { config, lib, ... }:
      {
        sops.secrets.zms-password = {
          neededForUsers = true;
        };

        users.users.zms = {
          uid = 1000;
          hashedPasswordFile = config.sops.secrets.zms-password.path;
          linger = true;
          extraGroups = [
            "wheel"
            "nixos-config"
          ]
          ++ lib.optionals config.hardware.openrazer.enable [ "openrazer" ];
        };
      };

    homeManager =
      { config, ... }:
      {
        programs.mcp.servers.filesystem.args = [ config.home.homeDirectory ];

        programs.git = {
          settings = {
            user = {
              name = "icebluerabbit";
              email = "314762863+icebluerabbit@users.noreply.github.com";
            };
          };
        };
      };
  };
}
