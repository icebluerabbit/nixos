# laptop — configuration profile for the MacBook Pro (nix-darwin, aarch64-darwin)
{ den, ... }:
{
  den.aspects.hosts._.laptop = {
    aspect-name = "hosts.laptop";
    includes = [
      den.provides.hostname
      den.aspects.security._.sops
      den.aspects.infra._.homelab
    ];

    darwin = _: {
      nixpkgs.hostPlatform = "aarch64-darwin";

      programs.zsh.enable = true;

      # nix-darwin release baseline; bump deliberately.
      system.stateVersion = 6;
    };
  };
}
