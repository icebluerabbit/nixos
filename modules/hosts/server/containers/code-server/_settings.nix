/*
  code-server User settings.json, expressed as nix so the font name and
  cross-container URLs come from the same sources as the rest of the config.
  `_` prefix: import-tree must not pick this up as an aspect module — it is
  imported explicitly by code-server.nix and rendered to JSON there.
*/
{ config }:
let
  gitea = config.infra.clients.container-gitea;
  opencode = config.infra.clients.container-opencode;
  monospace = "'${config.stylix.fonts.monospace.name}', monospace";
in
{
  "telemetry.telemetryLevel" = "off";
  # extension HTTP (forgejo clone) hangs ~60s without this
  "http.proxySupport" = "off";
  "workbench.colorTheme" = "Catppuccin Mocha";
  "workbench.iconTheme" = "material-icon-theme";
  "editor.fontFamily" = monospace;
  "editor.fontSize" = 16;
  "editor.minimap.enabled" = false;
  "terminal.integrated.defaultProfile.linux" = "zsh";
  "terminal.integrated.fontFamily" = monospace;
  "terminal.integrated.fontSize" = 16;
  "direnv.restart.automatic" = true;
  "nix.enableLanguageServer" = true;
  "nix.serverPath" = "nixd";
  "files.watcherExclude" = {
    "**/node_modules/**" = true;
    "**/target/**" = true;
    "**/.direnv/**" = true;
  };
  "files.exclude" = {
    "**/.direnv" = true;
  };
  "forgejo.instances" = [
    {
      id = "homelab-gitea";
      name = "Homelab Gitea";
      instanceUrl = "http://${gitea.ip}:${toString gitea.ports.web}";
      sshPort = gitea.ports.ssh;
    }
  ];
  "openchamber.apiUrl" = "http://${opencode.ip}:${toString opencode.ports.default}";
}
