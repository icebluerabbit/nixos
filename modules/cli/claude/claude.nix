/*
  Claude Code — Anthropic's agentic CLI; installed via claude-code-nix flake overlay with cachix binary cache
  https://github.com/anthropics/claude-code
*/
{ den, inputs, ... }:
{
  flake-file.inputs = {
    claude-code.url = "github:sadjow/claude-code-nix";
    claude-plugins = {
      url = "github:anthropics/claude-plugins-official";
      flake = false;
    };
    caveman = {
      url = "github:JuliusBrussee/caveman";
      flake = false;
    };
    anthropics-skills = {
      url = "github:anthropics/skills";
      flake = false;
    };
    matt-pocock-skills = {
      url = "github:mattpocock/skills";
      flake = false;
    };
  };

  den.aspects.cli._.claude = {
    aspect-name = "cli.claude";
    includes = [ den.aspects.dev._.nodejs ];

    nixos = {
      nixpkgs.overlays = [
        inputs.claude-code.overlays.default
        # Spec Kit (`specify`) — spec-driven development CLI paired with this
        # agent. nixpkgs is stuck on 0.0.90 while upstream is past 1.0, so the
        # version, source hash and dependency set are pinned here instead of
        # waiting on the nixpkgs bump. Refresh all three together.
        # https://github.com/github/spec-kit
        (final: prev: {
          spec-kit =
            let
              version = "1.0.8";
            in
            prev.spec-kit.overridePythonAttrs {
              inherit version;
              src = final.fetchFromGitHub {
                owner = "github";
                repo = "spec-kit";
                tag = "v${version}";
                hash = "sha256-VnbzJ1EoWze1haoVZQAHXc1QQgUFFKZkPVvmQbS+m5Q=";
              };
              dependencies = with final.python3Packages; [
                typer
                click
                rich
                platformdirs
                readchar
                pyyaml
                packaging
                pathspec
                json5
              ];
            };
        })
      ];

      nix.settings = {
        substituters = [ "https://claude-code.cachix.org" ];
        trusted-public-keys = [ "claude-code.cachix.org-1:YeXf2aNu7UTX8Vwrze0za1WEDS+4DuI2kVeWEE4fsRk=" ];
      };
    };

    homeManager =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        skills = ./skills;
        hooks = ./hooks;
        notificationHook = pkgs.writeShellApplication {
          name = "claude-notification";
          runtimeInputs = [
            pkgs.jq
            pkgs.libnotify
          ];
          text = builtins.readFile "${hooks}/notification.sh";
        };
        stopHook = pkgs.writeShellApplication {
          name = "claude-stop";
          runtimeInputs = [
            pkgs.jq
            pkgs.libnotify
          ];
          text = builtins.readFile "${hooks}/stop.sh";
        };
        # Tints the session's kitty tab; a no-op outside kitty.
        kittyTabHook = pkgs.writeShellApplication {
          name = "claude-kitty-tab";
          runtimeInputs = [ pkgs.kitty ];
          # The state colours and the text colour drawn on them.
          runtimeEnv = {
            CLAUDE_TAB_BUSY = config.lib.stylix.colors.withHashtag.base0A;
            CLAUDE_TAB_ATTENTION = config.lib.stylix.colors.withHashtag.base08;
            CLAUDE_TAB_IDLE = config.lib.stylix.colors.withHashtag.base0D;
            CLAUDE_TAB_DONE = config.lib.stylix.colors.withHashtag.base0B;
            CLAUDE_TAB_FAILED = config.lib.stylix.colors.withHashtag.base09;
            CLAUDE_TAB_COMPACTING = config.lib.stylix.colors.withHashtag.base0C;
            CLAUDE_TAB_FG = config.lib.stylix.colors.withHashtag.base00;
          };
          text = builtins.readFile "${hooks}/kitty-tab.sh";
        };
        kittyTabFor = matcher: state: {
          inherit matcher;
          hooks = [
            {
              type = "command";
              command = "${kittyTabHook}/bin/claude-kitty-tab ${state}";
            }
          ];
        };
        kittyTab = kittyTabFor "";
      in
      {
        home.packages = [ pkgs.spec-kit ];

        programs.claude-code = {
          enable = true;
          enableMcpIntegration = true;
          package = pkgs.claude-code;
          marketplaces = {
            claude-plugins-official = inputs.claude-plugins;
            inherit (inputs) caveman;
          };
          agents = { };
          plugins = [ ];
          rules = { };
          skills = {
            grill-me = "${inputs.matt-pocock-skills}/grill-me";
            skill-creator = "${inputs.anthropics-skills}/skills/skill-creator";

            # Local homelab skills — procedures + knowledge references.
            # Markdown only (SKILL.md); import-tree ignores non-.nix so these are never auto-imported.
            # Procedures (workflows):
            nixos-container-create = "${skills}/nixos-container-create";
            nixos-container-debug = "${skills}/nixos-container-debug";
            nixos-secret-add = "${skills}/nixos-secret-add";
            nixos-infra-provision = "${skills}/nixos-infra-provision";
            nixos-web-expose = "${skills}/nixos-web-expose";
            homelab-observability = "${skills}/homelab-observability";
            # Knowledge (reference, no tool required):
            homelab-topology = "${skills}/homelab-topology";
            homelab-networking = "${skills}/homelab-networking";
            nixos-container-conventions = "${skills}/nixos-container-conventions";
            den-aspects = "${skills}/den-aspects";
            nixos-vector-logging = "${skills}/nixos-vector-logging";
            nix-style = "${skills}/nix-style";
          };
          lspServers = {
            nix = {
              command = lib.getExe pkgs.nixd;
              extensionToLanguage = {
                ".nix" = "nix";
              };
            };
          };
          settings = {
            # Caveman mode badge. Script reads ~/.claude/.caveman-active and
            # renders nothing when the flag is absent, so this is inert if the
            # plugin is disabled.
            statusLine = {
              type = "command";
              command = "${lib.getExe pkgs.bash} ${inputs.caveman}/src/hooks/caveman-statusline.sh";
            };
            hooks = {
              UserPromptSubmit = [ (kittyTab "busy") ];
              # Repaints busy after every tool call. Approving a permission
              # prompt fires no hook of its own, so without this the
              # attention colour would sit there until the turn ended.
              PostToolUse = [ (kittyTab "busy") ];
              Notification = [
                {
                  matcher = "";
                  hooks = [
                    {
                      type = "command";
                      command = "${notificationHook}/bin/claude-notification";
                    }
                  ];
                }
                # Notification matchers name the notification type, so the
                # two cases split here rather than by sniffing the payload
                # inside the hook script.
                (kittyTabFor "permission_prompt" "attention")
                (kittyTabFor "idle_prompt" "idle")
              ];
              Stop = [
                {
                  matcher = "";
                  hooks = [
                    {
                      type = "command";
                      command = "${stopHook}/bin/claude-stop";
                    }
                  ];
                }
                (kittyTab "done")
              ];
              # A turn that died on an API error rather than finishing.
              StopFailure = [ (kittyTab "failed") ];
              # Compaction is slow and silent; the colour says the session
              # is busy rather than wedged.
              PreCompact = [ (kittyTab "compacting") ];
              # Both ends of a session start from no tint, so a resumed one
              # does not inherit whatever colour it died with.
              SessionStart = [ (kittyTab "") ];
              SessionEnd = [ (kittyTab "") ];
            };
            env = {
              CLAUDE_CODE_ENABLE_TELEMETRY = "1";
              OTEL_METRICS_EXPORTER = "otlp";
              OTEL_EXPORTER_OTLP_METRICS_ENDPOINT = "http://127.0.0.1:4318/v1/metrics";
              OTEL_EXPORTER_OTLP_PROTOCOL = "http/protobuf";
              CLAUDE_CODE_ENHANCED_TELEMETRY_BETA = "1";
              OTEL_METRIC_EXPORT_INTERVAL = "30000";
            };
            permissions = {
              allow = [
                # git
                "Bash(git status *)"
                "Bash(git diff *)"
                "Bash(git log *)"
                "Bash(git show *)"
                "Bash(git blame *)"
                "Bash(git branch *)"
                "Bash(git stash *)"
                "Bash(git fetch *)"
                "Bash(git rev-parse *)"
                "Bash(git remote *)"
                "Bash(git add *)"

                # nix
                "Bash(nix build *)"
                "Bash(nix eval *)"
                "Bash(nix flake *)"
                "Bash(nixfmt *)"

                # utilities
                "Bash(jq *)"
                "Bash(which *)"
                "Bash(ls *)"
                "Bash(pwd)"
                "Bash(cat *)"
                "Bash(head *)"
                "Bash(tail *)"
                "Bash(find *)"
                "Bash(grep *)"
                "Bash(rg *)"
                "Bash(wc *)"
                "Bash(sort *)"
                "Bash(mkdir *)"
                "Bash(echo *)"
                "Bash(journalctl *)"
              ];
              deny = [
                "Bash(rm -rf /)"
                "Bash(rm -rf /*)"
                "Bash(sudo *)"
              ];
            };
            enabledPlugins = {
              #"context7@claude-plugins-official" = true;
              #"github@claude-plugins-official" = true;
              "code-simplifier@claude-plugins-official" = true;
              #"superpowers@claude-plugins-official" = true;
              #"security-guidance@claude-plugins-official" = true;
              "claude-md-management@claude-plugins-official" = true;
              "claude-code-setup@claude-plugins-official" = true;
              "commit-commands@claude-plugins-official" = true;
              "rust-analyzer-lsp@claude-plugins-official" = true;
              "lua-lsp@claude-plugins-official" = true;
              "caveman@caveman" = true;
            };
          };
        };
      };
  };
}
