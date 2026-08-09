/*
  nh — command-line Nix helper for simplified system building and updates
  https://github.com/viperML/nh
*/
{ den, ... }:
{
  perSystem =
    { pkgs, ... }:
    {
      packages = den.lib.nh.denPackages { fromFlake = true; } pkgs;
    };
}
