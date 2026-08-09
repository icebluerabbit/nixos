/*
  Firefox — Mozilla's open-source web browser
  https://github.com/mozilla/gecko-dev
  Extensions sourced from rycee's NUR expressions (gitlab:rycee/nur-expressions).
*/
{ inputs, ... }:
{
  flake-file.inputs = {
    firefox-addons = {
      url = "gitlab:rycee/nur-expressions?dir=pkgs/firefox-addons";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  den.aspects.apps._.firefox = {
    aspect-name = "apps.firefox";
    homeManager =
      { pkgs, ... }:
      let
        system = pkgs.stdenv.hostPlatform.system;
      in
      {
        stylix.targets.firefox.profileNames = [ "default" ];
        home.sessionVariables = {
          MOZ_USE_XINPUT2 = "1";
          MOZ_DBUS_REMOTE = "1";
          MOZ_ENABLE_WAYLAND = "1";
        };
        programs.firefox = {
          enable = true;
          profiles = {
            default = {
              extensions.packages = with inputs.firefox-addons.packages.${system}; [
                clearurls
                containerise
                darkreader
                privacy-badger
                refined-github
                sponsorblock
                ublock-origin
                return-youtube-dislikes
                watchmarker-for-youtube
                # enhancer-for-youtube
              ];

              settings = {
                "accessibility.force_disabled" = 1;
                "browser.ping-centre.telemetry" = false;
                "browser.search.suggest.enabled" = false;
                "datareporting.healthreport.uploadEnabled" = false;
                "datareporting.policy.dataSubmissionEnabled" = false;
                "privacy.trackingprotection.enabled" = true;
                "toolkit.telemetry.archive.enabled" = false;
                "toolkit.telemetry.bhrPing.enabled" = false;
                "toolkit.telemetry.coverage.opt-out" = true;
                "toolkit.telemetry.enabled" = false;
                "toolkit.telemetry.firstShutdownPing.enabled" = false;
                "toolkit.telemetry.newProfilePing.enabled" = false;
                "toolkit.telemetry.server" = "data:,";
                "toolkit.telemetry.shutdownPingSender.enabled" = false;
                "toolkit.telemetry.unified" = false;
                "toolkit.telemetry.updatePing.enabled" = false;
                "browser.startup.page" = 3;
                "media.ffmpeg.vaapi.enabled" = true;
                "media.hardware-video-decoding.force-enabled" = true;
                "media.av1.enabled" = true;
                "media.eme.enabled" = true;
                "media.gmp-widevinecdm.enabled" = true;
                "media.gmp-manager.updateEnabled" = true;
                # Wayland DMA-BUF — GPU buffer sharing, reduces copies
                "widget.dmabuf.force-enabled" = true;
                # GPU process + content process parallelism
                "layers.gpu-process.enabled" = true;
                "dom.ipc.processCount" = 16;
                # memory cache (disabled by default in LibreWolf)
                "browser.cache.memory.enable" = true;
                "browser.cache.disk.enable" = false; # memory-only cache (you have RAM)
                "browser.cache.memory.capacity" = 1048576; # 1GB memory cache
                # WebRender (GPU compositor) — should be auto on NVIDIA but explicit is safe
                "gfx.webrender.all" = true;
                "gfx.webrender.compositor" = true;
                # WebGL
                "webgl.disabled" = false;
                # DNS over HTTPS
                "network.trr.mode" = 2; # prefer DoH, fallback to system
                # Parallel connections
                "network.http.max-persistent-connections-per-server" = 10;
                # Speculation/preloading
                "network.prefetch-next" = true;
                "network.predictor.enabled" = true;
                "security.enterprise_roots.enabled" = true;
              };
            };
          };
        };
      };
  };
}
