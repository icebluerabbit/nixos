/*
  sops-nix — NixOS module for Mozilla SOPS encrypted secrets; decrypted at activation time via age or PGP keys
  https://github.com/Mic92/sops-nix
*/
{ inputs, ... }:
{
  flake-file.inputs = {
    sops-nix = {
      url = "github:Mic92/sops-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  den.aspects.security._.sops = {
    aspect-name = "security.sops";
    nixos =
      { pkgs, ... }:
      {
        imports = [ inputs.sops-nix.nixosModules.sops ];

        environment.systemPackages = [
          pkgs.sops
          pkgs.age
        ];

        sops = {
          defaultSopsFile = inputs.self + "/secrets/host-secrets.yaml";
          age.keyFile = "/var/lib/sops-nix/key.txt";
          age.generateKey = true;
          validateSopsFiles = true;
          secrets = {
            fake-password = { };
          };
        };
      };

    homeManager =
      { config, ... }:
      {
        imports = [ inputs.sops-nix.homeManagerModules.sops ];

        sops = {
          defaultSopsFile = inputs.self + "/secrets/user-secrets.yaml";
          age.keyFile = "${config.home.homeDirectory}/.config/sops/age/keys.txt";
          age.generateKey = true;
          validateSopsFiles = true;
          secrets = {
            fake-password = { };
          };
        };
      };
  };
}
