# devshell — default development shell environment and git pre-commit setup
_: {
  perSystem =
    {
      config,
      pkgs,
      ...
    }:
    {
      devShells.default = pkgs.mkShell {
        inputsFrom = if config ? pre-commit then [ config.pre-commit.devShell ] else [ ];

        # The per-language toolchain treefmt drives (see modules/formatter.nix),
        # on PATH so editors and one-off runs use the same binaries the
        # formatter and the pre-commit hook do.
        packages = [
          pkgs.go
          pkgs.gopls
          pkgs.ruff
          pkgs.shfmt
          pkgs.shellcheck
          pkgs.prettier
        ]
        ++ (if config ? treefmt then [ config.treefmt.build.wrapper ] else [ ]);

        shellHook =
          if config ? pre-commit then
            ''
              # Link the nix-generated pre-commit config into the workspace.
              ln -sf ${config.pre-commit.settings.configFile} .pre-commit-config.yaml

              # Run pre-commit install to write the executable script in .git/hooks/pre-commit
              if [ -d .git ]; then
                pre-commit install
              fi
            ''
          else
            "";
      };
    };
}
