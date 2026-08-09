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
        shellHook =
          if config ? pre-commit then
            ''
              # Copy the nix-generated pre-commit config file to the workspace
              # to avoid symlink failures on exfat filesystems where symlinking is not allowed.
              cp -f ${config.pre-commit.settings.configFile} .pre-commit-config.yaml
              chmod +w .pre-commit-config.yaml

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
