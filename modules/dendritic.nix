# dendritic — modular system structure and flake inputs integration
{ inputs, ... }:
{
  imports = [
    inputs.flake-file.flakeModules.dendritic
    inputs.den.flakeModules.dendritic
    inputs.denful-networks.flakeModules.default
  ];

  flake-file.inputs = {
    darwin = {
      url = "github:nix-darwin/nix-darwin";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    den.url = "github:vic/den";
    denful-networks.url = "github:icebluerabbit/denful-networks/dev";
    flake-file.url = "github:vic/flake-file";
    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    terranix = {
      url = "github:terranix/terranix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };
}
