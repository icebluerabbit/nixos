/*
  Discord — voice, video, and text communication platform
  https://discord.com/
  Configured for Wayland + NVIDIA with EGL rendering and PipeWire screen share.
*/
_: {
  den.aspects.apps._.discord = {
    aspect-name = "apps.discord";
    includes = [ ];

    homeManager =
      { pkgs, lib, ... }:
      let
        features = {
          # --- Wayland ---
          UseOzonePlatform = true;
          WaylandWindowDecorations = true;
          # Modern Wayland synchronization (fixes tearing/latency on NVIDIA)
          # niri doesn't support explicit sync — renderer SIGSEGV (exitCode 139)
          # https://github.com/niri-wm/niri/issues/1432
          WaylandExplicitSync = false;

          # --- Hardware Acceleration (NVIDIA + Vulkan) ---
          VaapiVideoDecoder = true;
          VaapiVideoEncoder = true;
          VaapiOnNvidiaGPUs = true;
          VaapiIgnoreDriverChecks = true;
          # Disabled — Vulkan decode conflicts with EGL render path
          VaapiVideoDecodeLinuxVulkan = false;

          # ANGLE/Vulkan disabled — GPU process crashes (exit 8704) on niri+NVIDIA
          # Use native EGL path (--use-gl=egl) instead
          DefaultANGLEVulkan = false;
          VulkanFromANGLE = false;

          # --- Screen Share / WebRTC ---
          WebRTCPipeWireCapturer = true;

          # --- Disabled ---
          # Native Vulkan path — incompatible with NVIDIA/Wayland Ozone
          Vulkan = false;
          # ChromeOS-only decoder path
          UseChromeOSDirectVideoDecoder = false;
        };

        enabledFeatures = builtins.attrNames (lib.attrsets.filterAttrs (_: v: v == true) features);
        disabledFeatures = builtins.attrNames (lib.attrsets.filterAttrs (_: v: v == false) features);
      in
      {
        # Every flag below describes the desktop's Wayland + NVIDIA stack:
        # Ozone platform, VA-API-on-NVIDIA decode, PipeWire screen share and
        # a DISPLAY wrapper for a libX11 null-deref in discord_voice.node.
        # macOS has none of that — Electron there uses the system compositor
        # and VideoToolbox, so the Mac gets the package as shipped.
        home.packages =
          if pkgs.stdenv.hostPlatform.isDarwin then
            [ pkgs.discord ]
          else
            let
              base = pkgs.discord.override {
                commandLineArgs = lib.concatStringsSep " " [
                  # Explicit Wayland — not hint=auto, DISPLAY=:0 is set for libX11 compat
                  # discord_voice.node links libX11; XOpenDisplay(NULL) returns NULL when
                  # DISPLAY is unset → null-deref crash 17s into voice engine init
                  "--ozone-platform=wayland"
                  "--enable-wayland-ime"
                  # NVIDIA: use native EGL (ANGLE/Vulkan causes GPU process crash exit 8704)
                  "--use-gl=egl"
                  # NVIDIA: prevent GPU process sandbox crashes
                  "--disable-gpu-sandbox"
                  # GPU rasterization
                  "--ignore-gpu-blocklist"
                  "--enable-gpu-rasterization"
                  "--enable-oop-rasterization"
                  "--enable-features=${builtins.concatStringsSep "," enabledFeatures}"
                  "--disable-features=${builtins.concatStringsSep "," disabledFeatures}"
                ];
              };
            in
            [
              (pkgs.symlinkJoin {
                name = "discord";
                paths = [ base ];
                buildInputs = [ pkgs.makeWrapper ];
                postBuild = "wrapProgram $out/bin/discord --set DISPLAY :0";
              })
            ];
      };
  };
}
