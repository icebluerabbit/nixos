# formatter — repository-wide code formatter configuration
_: {
  perSystem =
    { pkgs, ... }:
    {
      formatter = pkgs.nixfmt-tree;
    };
}
