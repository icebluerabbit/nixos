/*
  Vesktop — Electron Discord client with Vencord built in
  https://github.com/Vencord/Vesktop
  Wayland + NVIDIA, PipeWire screen share.

  Worth having next to the official client for one specific reason: Vencord's
  WebScreenShareFixes plugin (bundled, enabled by default) rewrites the
  negotiated SDP ceiling from Discord's 2500 kbps to 80000. It is gated to
  browser/Vesktop clients, so the official desktop app can never load it.
  Vesktop also ships a current Electron — the official Linux client lags, and
  that costs it hardware decode paths and AV1 support.

  Installed alongside apps.discord rather than replacing it; the official one
  stays as the fallback.
*/
_: {
  den.aspects.apps._.vesktop = {
    aspect-name = "apps.vesktop";
    includes = [ ];

    homeManager =
      { pkgs, lib, ... }:
      let
        # Kept deliberately in step with apps.discord — same GPU, same
        # compositor, so the same Chromium workarounds apply. See
        # modules/apps/discord.nix for the reasoning behind each entry.
        features = {
          # --- Wayland ---
          UseOzonePlatform = true;
          WaylandWindowDecorations = true;
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

          # ANGLE/Vulkan disabled — GPU process crashes (exit 8704) on
          # niri+NVIDIA. Use native EGL path (--use-gl=egl) instead.
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
        disabledFeatures = builtins.attrNames (lib.attrsets.filterAttrs (_: v: !v) features);
      in
      {
        home.packages = [
          (pkgs.symlinkJoin {
            name = "vesktop";
            # Vencord is left for Vesktop to manage at runtime (the nixpkgs
            # default) rather than pinned via withSystemVencord — it then
            # self-updates, which matters because Discord ships breaking
            # client changes often and a pinned Vencord silently loses plugins.
            paths = [ pkgs.vesktop ];
            buildInputs = [ pkgs.makeWrapper ];
            # Vesktop's own wrapper only adds ozone flags when NIXOS_OZONE_WL
            # and WAYLAND_DISPLAY are both set, and never the NVIDIA ones —
            # add them unconditionally on top.
            postBuild = ''
              wrapProgram $out/bin/vesktop \
                --add-flags "--ozone-platform=wayland" \
                --add-flags "--use-gl=egl" \
                --add-flags "--disable-gpu-sandbox" \
                --add-flags "--ignore-gpu-blocklist" \
                --add-flags "--enable-gpu-rasterization" \
                --add-flags "--enable-features=${builtins.concatStringsSep "," enabledFeatures}" \
                --add-flags "--disable-features=${builtins.concatStringsSep "," disabledFeatures}"
            '';
          })
        ];
      };
  };
}
