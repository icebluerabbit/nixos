_: {
  perSystem = { pkgs, ... }: {
    packages.sys-mcp = pkgs.callPackage ../../packages/sys-mcp { };
  };
}
