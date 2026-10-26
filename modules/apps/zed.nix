/*
  Zed — high-performance code editor written in Rust with native GPU rendering (GPUI)
  https://github.com/zed-industries/zed
*/
{ den, ... }:
{
  den.aspects.apps._.zed = {
    aspect-name = "apps.zed";
    includes = [
      den.aspects.fonts._.jetbrains-mono
      den.aspects.dev._.nixd
      den.aspects.dev._.lua-language-server
      den.aspects.dev._.yaml-language-server
      den.aspects.dev._.gopls
    ];

    homeManager =
      {
        pkgs,
        lib,
        config,
        ...
      }:
      {
        programs.zed-editor = {
          enable = true;
          mutableUserSettings = false;
          enableMcpIntegration = false;
          extensions = [
            "nix"
            "lua"
            "toml"
            "yaml"
            "jetbrains-new-ui-icons"
          ];
          userSettings = {
            # Where this repo and the scratch tree live, which differs by
            # platform: the Linux hosts bind-mount them at the top level,
            # the Mac keeps a plain checkout under $HOME.
            trusted_directories =
              if pkgs.stdenv.hostPlatform.isDarwin then
                [ "${config.home.homeDirectory}/Projects" ]
              else
                [
                  "/etc/nixos"
                  "/src"
                ];
            telemetry = {
              diagnostics = false;
              metrics = false;
            };

            context_servers = lib.pipe config.programs.mcp.servers [
              (lib.filterAttrs (name: _: name != "github"))
              (lib.mapAttrs (
                _: server:
                let
                  base = lib.filterAttrs (
                    n: _:
                    lib.elem n [
                      "args"
                      "env"
                      "url"
                      "headers"
                    ]
                  ) server;
                in
                if server ? command then base // { inherit (server) command; } else base
              ))
            ];

            lsp = {
              nixd.binary.path = lib.getExe pkgs.nixd;
              lua_ls.binary.path = lib.getExe pkgs.lua-language-server;
              yaml-language-server.binary = {
                path = lib.getExe pkgs.yaml-language-server;
                arguments = [ "--stdio" ];
              };
              # Pinned so Zed uses the toolchain from dev.gopls rather than
              # downloading its own copy into ~/.local/share/zed, outside Nix.
              # Go needs no extension entry — unlike nix, lua, toml and yaml,
              # its support is built into Zed itself.
              gopls.binary.path = lib.getExe pkgs.gopls;
            };

            languages = {
              Nix = {
                formatter = {
                  external = {
                    command = lib.getExe pkgs.nixfmt;
                  };
                };
              };
            };
          };
        };
      };
  };
}
